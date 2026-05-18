package models

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

var ErrAlreadyExecuted = errors.New("change is already executed")

type Change struct {
	ID          string    `json:"id"`
	System      string    `json:"system"`
	Environment *string   `json:"environment"`
	User        *string   `json:"user"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	Status      string    `json:"status"`
	EventAt     time.Time `json:"timestamp"`
	CreatedAt   time.Time `json:"created_at"`
}

type ChangeFilters struct {
	System      string
	Environment string
	User        string
	Type        string
	Status      string // "executed", "scheduled", "overdue", or ""
	Search      string
	From        *time.Time
	To          *time.Time
	Limit       int
	Offset      int
	SortAsc     bool
}

func CreateChange(db *sql.DB, system string, environment, user *string, changeType, description, status string, eventAt *time.Time) (*Change, error) {
	newID := uuid.New().String()
	if status == "" {
		status = "executed"
	}
	var err error
	if eventAt != nil {
		_, err = db.Exec(
			"INSERT INTO changes (id, system, environment, user, type, description, status, event_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			newID, system, environment, user, changeType, description, status, *eventAt,
		)
	} else {
		_, err = db.Exec(
			"INSERT INTO changes (id, system, environment, user, type, description, status) VALUES (?, ?, ?, ?, ?, ?, ?)",
			newID, system, environment, user, changeType, description, status,
		)
	}
	if err != nil {
		return nil, err
	}
	return GetChangeByID(db, newID)
}

func GetChangeByID(db *sql.DB, id string) (*Change, error) {
	c := &Change{}
	err := db.QueryRow(
		"SELECT id, system, environment, user, type, description, status, event_at, created_at FROM changes WHERE id = ?",
		id,
	).Scan(&c.ID, &c.System, &c.Environment, &c.User, &c.Type, &c.Description, &c.Status, &c.EventAt, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func UpdateChange(db *sql.DB, id string, system string, environment, user *string, changeType, description, status string, eventAt *time.Time) (*Change, error) {
	var res sql.Result
	var err error
	if eventAt != nil {
		res, err = db.Exec(
			"UPDATE changes SET system=?, environment=?, user=?, type=?, description=?, status=?, event_at=? WHERE id=?",
			system, environment, user, changeType, description, status, *eventAt, id,
		)
	} else {
		res, err = db.Exec(
			"UPDATE changes SET system=?, environment=?, user=?, type=?, description=?, status=? WHERE id=?",
			system, environment, user, changeType, description, status, id,
		)
	}
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, sql.ErrNoRows
	}
	return GetChangeByID(db, id)
}

func ConfirmChange(db *sql.DB, id string, executedAt *time.Time) (*Change, error) {
	existing, err := GetChangeByID(db, id)
	if err == sql.ErrNoRows {
		return nil, sql.ErrNoRows
	}
	if err != nil {
		return nil, err
	}
	if existing.Status == "executed" {
		return nil, ErrAlreadyExecuted
	}

	if executedAt != nil {
		_, err = db.Exec(
			"UPDATE changes SET status='executed', event_at=? WHERE id=?",
			*executedAt, id,
		)
	} else {
		_, err = db.Exec(
			"UPDATE changes SET status='executed', event_at=NOW() WHERE id=?",
			id,
		)
	}
	if err != nil {
		return nil, err
	}
	return GetChangeByID(db, id)
}

func DeleteChange(db *sql.DB, id string) error {
	res, err := db.Exec("DELETE FROM changes WHERE id=?", id)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func ListChanges(db *sql.DB, f ChangeFilters) ([]Change, int, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 200 {
		f.Limit = 200
	}

	var where []string
	var args []interface{}

	if f.System != "" {
		where = append(where, "system = ?")
		args = append(args, f.System)
	}
	if f.Environment != "" {
		where = append(where, "environment = ?")
		args = append(args, f.Environment)
	}
	if f.User != "" {
		where = append(where, "user = ?")
		args = append(args, f.User)
	}
	if f.Type != "" {
		where = append(where, "type = ?")
		args = append(args, f.Type)
	}
	if f.Status == "overdue" {
		where = append(where, "status = 'scheduled' AND event_at < NOW()")
	} else if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.Search != "" {
		where = append(where, "(description LIKE ? OR system LIKE ? OR user LIKE ? OR environment LIKE ?)")
		pattern := "%" + f.Search + "%"
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if f.From != nil {
		where = append(where, "event_at >= ?")
		args = append(args, *f.From)
	}
	if f.To != nil {
		where = append(where, "event_at <= ?")
		args = append(args, *f.To)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	orderDir := "DESC"
	if f.SortAsc {
		orderDir = "ASC"
	}

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM changes %s", whereClause)
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf("SELECT id, system, environment, user, type, description, status, event_at, created_at FROM changes %s ORDER BY event_at %s LIMIT ? OFFSET ?", whereClause, orderDir)
	pageArgs := append(args, f.Limit, f.Offset)
	rows, err := db.Query(query, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var changes []Change
	for rows.Next() {
		var c Change
		if err := rows.Scan(&c.ID, &c.System, &c.Environment, &c.User, &c.Type, &c.Description, &c.Status, &c.EventAt, &c.CreatedAt); err != nil {
			return nil, 0, err
		}
		changes = append(changes, c)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if changes == nil {
		changes = []Change{}
	}

	return changes, total, nil
}
