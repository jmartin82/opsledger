package models

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type AuditEntry struct {
	ID         uint64    `json:"id,string"`
	Actor      string    `json:"actor"`
	ActorID    *uint64   `json:"actorId,string,omitempty"`
	Action     string    `json:"action"`
	TargetType string    `json:"targetType"`
	TargetID   *uint64   `json:"targetId,string,omitempty"`
	Details    *string   `json:"details,omitempty"`
	IPAddress  *string   `json:"ipAddress,omitempty"`
	CreatedAt  time.Time `json:"timestamp"`
}

type AuditFilters struct {
	Action     string
	Actor      string
	TargetType string
	From       *time.Time
	To         *time.Time
	Limit      int
	Offset     int
}

func CreateAuditEntry(db *sql.DB, actor string, actorID *uint64, action, targetType string, targetID *uint64, details, ipAddress *string) error {
	_, err := db.Exec(
		"INSERT INTO audit_log (actor, actor_id, action, target_type, target_id, details, ip_address) VALUES (?, ?, ?, ?, ?, ?, ?)",
		actor, actorID, action, targetType, targetID, details, ipAddress,
	)
	return err
}

func ListAuditEntries(db *sql.DB, f AuditFilters) ([]AuditEntry, int, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 200 {
		f.Limit = 200
	}

	var where []string
	var args []interface{}

	if f.Action != "" {
		where = append(where, "action = ?")
		args = append(args, f.Action)
	}
	if f.Actor != "" {
		where = append(where, "actor = ?")
		args = append(args, f.Actor)
	}
	if f.TargetType != "" {
		where = append(where, "target_type = ?")
		args = append(args, f.TargetType)
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

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM audit_log %s", whereClause)
	if err := db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf("SELECT id, actor, actor_id, action, target_type, target_id, details, ip_address, created_at FROM audit_log %s ORDER BY created_at DESC LIMIT ? OFFSET ?", whereClause)
	pageArgs := append(args, f.Limit, f.Offset)
	rows, err := db.Query(query, pageArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entries []AuditEntry
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.Actor, &e.ActorID, &e.Action, &e.TargetType, &e.TargetID, &e.Details, &e.IPAddress, &e.CreatedAt); err != nil {
			return nil, 0, err
		}
		entries = append(entries, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if entries == nil {
		entries = []AuditEntry{}
	}

	return entries, total, nil
}
