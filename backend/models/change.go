package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Change struct {
	ID          string    `json:"id"`
	System      string    `json:"system"`
	Environment *string   `json:"environment"`
	User        *string   `json:"user"`
	Type        string    `json:"type"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"timestamp"`
}

type ChangeFilters struct {
	System      string
	Environment string
	User        string
	Type        string
	Search      string
	From        *time.Time
	To          *time.Time
	Limit       int
	Offset      int
}

func CreateChange(db *sql.DB, system string, environment, user *string, changeType, description string, timestamp *time.Time) (*Change, error) {
	newID := uuid.New().String()
	var err error
	if timestamp != nil {
		_, err = db.Exec(
			"INSERT INTO changes (id, system, environment, user, type, description, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			newID, system, environment, user, changeType, description, *timestamp,
		)
	} else {
		_, err = db.Exec(
			"INSERT INTO changes (id, system, environment, user, type, description) VALUES (?, ?, ?, ?, ?, ?)",
			newID, system, environment, user, changeType, description,
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
		"SELECT id, system, environment, user, type, description, created_at FROM changes WHERE id = ?",
		id,
	).Scan(&c.ID, &c.System, &c.Environment, &c.User, &c.Type, &c.Description, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func UpdateChange(db *sql.DB, id string, system string, environment, user *string, changeType, description string, timestamp *time.Time) (*Change, error) {
	var res sql.Result
	var err error
	if timestamp != nil {
		res, err = db.Exec(
			"UPDATE changes SET system=?, environment=?, user=?, type=?, description=?, created_at=? WHERE id=?",
			system, environment, user, changeType, description, *timestamp, id,
		)
	} else {
		res, err = db.Exec(
			"UPDATE changes SET system=?, environment=?, user=?, type=?, description=? WHERE id=?",
			system, environment, user, changeType, description, id,
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
	if f.Search != "" {
		where = append(where, "(description LIKE ? OR system LIKE ? OR user LIKE ? OR environment LIKE ?)")
		pattern := "%" + f.Search + "%"
		args = append(args, pattern, pattern, pattern, pattern)
	}
	if f.From != nil {
		where = append(where, "created_at >= ?")
		args = append(args, *f.From)
	}
	if f.To != nil {
		where = append(where, "created_at <= ?")
		args = append(args, *f.To)
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// Count total matching rows
	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM changes %s", whereClause)
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch page
	query := fmt.Sprintf("SELECT id, system, environment, user, type, description, created_at FROM changes %s ORDER BY created_at DESC LIMIT ? OFFSET ?", whereClause)
	pageArgs := append(args, f.Limit, f.Offset)
	rows, err := db.Query(query, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var changes []Change
	for rows.Next() {
		var c Change
		if err := rows.Scan(&c.ID, &c.System, &c.Environment, &c.User, &c.Type, &c.Description, &c.CreatedAt); err != nil {
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
