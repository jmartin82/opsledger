package models

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

func generateSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type Connector struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	Secret    string    `json:"-"`
	Enabled   bool      `json:"enabled"`
	CreatedBy uint64    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type JiraConnector struct {
	Connector
	JiraURL  string          `json:"jira_url"`
	APIToken string          `json:"-"`
	Mapping  json.RawMessage `json:"mapping"`
}

// JiraMapping is the parsed shape of the mapping JSON column.
type JiraMapping struct {
	TypeMap                map[string]string `json:"type_map"`
	EnvironmentLabelPrefix string            `json:"environment_label_prefix"`
}

func CreateJiraConnector(db *sql.DB, name, jiraURL, apiToken string, mapping json.RawMessage, createdBy uint64) (*JiraConnector, error) {
	id := uuid.New().String()
	secret, err := generateSecret()
	if err != nil {
		return nil, err
	}

	tx, err := db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"INSERT INTO connectors (id, name, type, secret, enabled, created_by) VALUES (?, ?, 'jira', ?, TRUE, ?)",
		id, name, secret, createdBy,
	)
	if err != nil {
		return nil, err
	}

	_, err = tx.Exec(
		"INSERT INTO jira_connectors (connector_id, jira_url, api_token, mapping) VALUES (?, ?, ?, ?)",
		id, jiraURL, apiToken, string(mapping),
	)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return GetJiraConnectorByID(db, id)
}

func ListConnectors(db *sql.DB) ([]Connector, error) {
	rows, err := db.Query(
		"SELECT id, name, type, secret, enabled, created_by, created_at, updated_at FROM connectors ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var connectors []Connector
	for rows.Next() {
		var c Connector
		if err := rows.Scan(&c.ID, &c.Name, &c.Type, &c.Secret, &c.Enabled, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		connectors = append(connectors, c)
	}
	return connectors, rows.Err()
}

func GetJiraConnectorByID(db *sql.DB, id string) (*JiraConnector, error) {
	var jc JiraConnector
	var mapping string
	err := db.QueryRow(`
		SELECT c.id, c.name, c.type, c.secret, c.enabled, c.created_by, c.created_at, c.updated_at,
		       j.jira_url, j.api_token, j.mapping
		FROM connectors c
		JOIN jira_connectors j ON j.connector_id = c.id
		WHERE c.id = ?`, id,
	).Scan(
		&jc.ID, &jc.Name, &jc.Type, &jc.Secret, &jc.Enabled, &jc.CreatedBy, &jc.CreatedAt, &jc.UpdatedAt,
		&jc.JiraURL, &jc.APIToken, &mapping,
	)
	if err != nil {
		return nil, err
	}
	jc.Mapping = json.RawMessage(mapping)
	return &jc, nil
}

func UpdateJiraConnector(db *sql.DB, id, name, jiraURL, apiToken string, mapping json.RawMessage, enabled bool) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		"UPDATE connectors SET name = ?, enabled = ? WHERE id = ?",
		name, enabled, id,
	)
	if err != nil {
		return err
	}

	if apiToken != "" {
		_, err = tx.Exec(
			"UPDATE jira_connectors SET jira_url = ?, api_token = ?, mapping = ? WHERE connector_id = ?",
			jiraURL, apiToken, string(mapping), id,
		)
	} else {
		_, err = tx.Exec(
			"UPDATE jira_connectors SET jira_url = ?, mapping = ? WHERE connector_id = ?",
			jiraURL, string(mapping), id,
		)
	}
	if err != nil {
		return err
	}

	return tx.Commit()
}

func DeleteConnector(db *sql.DB, id string) error {
	_, err := db.Exec("DELETE FROM connectors WHERE id = ?", id)
	return err
}
