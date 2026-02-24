package models

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

type ApiKey struct {
	ID        uint64     `json:"id"`
	Name      string     `json:"name"`
	KeyHash   string     `json:"-"`
	Prefix    string     `json:"prefix"`
	Scopes    []string   `json:"scopes"`
	Status    string     `json:"status"`
	CreatedBy uint64     `json:"createdBy"`
	ExpiresAt *time.Time `json:"expiresAt"`
	LastUsed  *time.Time `json:"lastUsed"`
	CreatedAt time.Time  `json:"createdAt"`
}

const apiKeyPrefix = "ol_live_"

var validScopes = map[string]bool{
	"changes:read":  true,
	"changes:write": true,
}

// GenerateAPIKey creates a new random API key and returns the raw key and its SHA-256 hash.
func GenerateAPIKey() (rawKey, keyHash, prefix string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("failed to generate random bytes: %w", err)
	}

	rawKey = apiKeyPrefix + hex.EncodeToString(b)
	hash := sha256.Sum256([]byte(rawKey))
	keyHash = hex.EncodeToString(hash[:])
	prefix = rawKey[:12]
	return rawKey, keyHash, prefix, nil
}

// HashAPIKey returns the SHA-256 hex digest of a raw API key.
func HashAPIKey(rawKey string) string {
	hash := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(hash[:])
}

// ValidateScopes checks that all provided scopes are valid.
func ValidateScopes(scopes []string) error {
	if len(scopes) == 0 {
		return fmt.Errorf("at least one scope is required")
	}
	for _, s := range scopes {
		if !validScopes[s] {
			return fmt.Errorf("invalid scope: %s", s)
		}
	}
	return nil
}

func CreateApiKey(db *sql.DB, name, keyHash, prefix string, scopes []string, createdBy uint64, expiresAt *time.Time) (*ApiKey, error) {
	scopesStr := strings.Join(scopes, ",")
	res, err := db.Exec(
		"INSERT INTO api_keys (name, key_hash, prefix, scopes, created_by, expires_at) VALUES (?, ?, ?, ?, ?, ?)",
		name, keyHash, prefix, scopesStr, createdBy, expiresAt,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return GetApiKeyByID(db, uint64(id))
}

func GetApiKeyByID(db *sql.DB, id uint64) (*ApiKey, error) {
	k := &ApiKey{}
	var scopesStr string
	err := db.QueryRow(
		"SELECT id, name, key_hash, prefix, scopes, status, created_by, expires_at, last_used, created_at FROM api_keys WHERE id = ?",
		id,
	).Scan(&k.ID, &k.Name, &k.KeyHash, &k.Prefix, &scopesStr, &k.Status, &k.CreatedBy, &k.ExpiresAt, &k.LastUsed, &k.CreatedAt)
	if err != nil {
		return nil, err
	}
	k.Scopes = strings.Split(scopesStr, ",")
	return k, nil
}

func GetApiKeyByHash(db *sql.DB, keyHash string) (*ApiKey, error) {
	k := &ApiKey{}
	var scopesStr string
	err := db.QueryRow(
		"SELECT id, name, key_hash, prefix, scopes, status, created_by, expires_at, last_used, created_at FROM api_keys WHERE key_hash = ? AND status = 'active'",
		keyHash,
	).Scan(&k.ID, &k.Name, &k.KeyHash, &k.Prefix, &scopesStr, &k.Status, &k.CreatedBy, &k.ExpiresAt, &k.LastUsed, &k.CreatedAt)
	if err != nil {
		return nil, err
	}
	k.Scopes = strings.Split(scopesStr, ",")
	return k, nil
}

func ListApiKeysByCreator(db *sql.DB, createdBy uint64) ([]ApiKey, error) {
	rows, err := db.Query(
		"SELECT id, name, prefix, scopes, status, created_by, expires_at, last_used, created_at FROM api_keys WHERE created_by = ? ORDER BY created_at DESC",
		createdBy,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []ApiKey
	for rows.Next() {
		k := ApiKey{}
		var scopesStr string
		if err := rows.Scan(&k.ID, &k.Name, &k.Prefix, &scopesStr, &k.Status, &k.CreatedBy, &k.ExpiresAt, &k.LastUsed, &k.CreatedAt); err != nil {
			return nil, err
		}
		k.Scopes = strings.Split(scopesStr, ",")
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func RevokeApiKey(db *sql.DB, id uint64) error {
	res, err := db.Exec("UPDATE api_keys SET status = 'revoked' WHERE id = ? AND status = 'active'", id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func UpdateApiKeyLastUsed(db *sql.DB, id uint64) error {
	_, err := db.Exec("UPDATE api_keys SET last_used = NOW() WHERE id = ?", id)
	return err
}
