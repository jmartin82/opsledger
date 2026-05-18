package database

import (
	"database/sql"
	"strings"
)

// isMySQLDupErr returns true for MySQL "duplicate column" (1060) or "duplicate key name" (1061).
func isMySQLDupErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "Error 1060") || strings.Contains(s, "Duplicate column") ||
		strings.Contains(s, "Error 1061") || strings.Contains(s, "Duplicate key name")
}

func Migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id            BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			email         VARCHAR(255) NOT NULL UNIQUE,
			name          VARCHAR(255) NOT NULL,
			password_hash VARCHAR(255) NOT NULL,
			role          ENUM('admin', 'editor', 'viewer') NOT NULL DEFAULT 'viewer',
			status        ENUM('active', 'disabled') NOT NULL DEFAULT 'active',
			last_login    DATETIME NULL,
			created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS api_keys (
			id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			name        VARCHAR(255) NOT NULL,
			key_hash    VARCHAR(64) NOT NULL UNIQUE,
			prefix      VARCHAR(12) NOT NULL,
			scopes      VARCHAR(255) NOT NULL,
			status      ENUM('active', 'revoked') NOT NULL DEFAULT 'active',
			created_by  BIGINT UNSIGNED NOT NULL,
			expires_at  DATETIME NULL,
			last_used   DATETIME NULL,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_api_keys_key_hash (key_hash),
			INDEX idx_api_keys_created_by (created_by),
			FOREIGN KEY (created_by) REFERENCES users(id)
		)
	`)
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS changes (
			id          CHAR(36) NOT NULL PRIMARY KEY,
			system      VARCHAR(255) NOT NULL,
			environment VARCHAR(100) NULL,
			user        VARCHAR(255) NULL,
			type        ENUM('infrastructure', 'deployment', 'configuration') NOT NULL,
			description TEXT NOT NULL,
			status      ENUM('executed', 'scheduled') NOT NULL DEFAULT 'executed',
			event_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_changes_event_at   (event_at DESC),
			INDEX idx_changes_created_at (created_at DESC),
			INDEX idx_changes_system     (system),
			INDEX idx_changes_type       (type),
			INDEX idx_changes_status     (status)
		)
	`)
	if err != nil {
		return err
	}

	// Idempotent schema upgrades for existing deployments.
	migrations := []string{
		`ALTER TABLE changes ADD COLUMN status ENUM('executed','scheduled') NULL`,
		`ALTER TABLE changes ADD COLUMN event_at DATETIME NULL`,
		`UPDATE changes SET event_at = created_at WHERE event_at IS NULL`,
		`UPDATE changes SET status = 'executed' WHERE status IS NULL`,
		`ALTER TABLE changes MODIFY COLUMN status ENUM('executed','scheduled') NOT NULL DEFAULT 'executed'`,
		`ALTER TABLE changes MODIFY COLUMN event_at DATETIME NOT NULL`,
		`ALTER TABLE changes ADD INDEX idx_changes_event_at (event_at DESC)`,
		`ALTER TABLE changes ADD INDEX idx_changes_status (status)`,
	}
	for _, m := range migrations {
		if _, merr := db.Exec(m); merr != nil && !isMySQLDupErr(merr) {
			// UPDATE and MODIFY statements won't trigger dup errors; only ADD COLUMN/INDEX can.
			// Re-check: skip dup errors only for structural DDL, propagate data errors.
			if strings.HasPrefix(m, "UPDATE ") || strings.HasPrefix(m, "ALTER TABLE changes MODIFY") {
				return merr
			}
			if !isMySQLDupErr(merr) {
				return merr
			}
		}
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS audit_log (
			id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
			actor       VARCHAR(255) NOT NULL,
			actor_id    BIGINT UNSIGNED NULL,
			action      VARCHAR(100) NOT NULL,
			target_type VARCHAR(50)  NOT NULL,
			target_id   BIGINT UNSIGNED NULL,
			target_uuid CHAR(36) NULL,
			details     TEXT NULL,
			ip_address  VARCHAR(45) NULL,
			created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			INDEX idx_audit_log_created_at (created_at DESC),
			INDEX idx_audit_log_action (action),
			INDEX idx_audit_log_actor (actor)
		)
	`)
	return err
}
