//go:build integration

package handlers_test

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"

	"ops-ledger/backend/config"
	"ops-ledger/backend/database"
	"ops-ledger/backend/handlers"
	"ops-ledger/backend/models"
)

type mcpTestEnv struct {
	db  *sql.DB
	hub *handlers.EventHub
}

func setupMCPTestEnv(t *testing.T) *mcpTestEnv {
	t.Helper()

	cfg := config.Config{
		DBHost: getEnvDefault("DB_HOST", "localhost"),
		DBPort: getEnvDefault("DB_PORT", "3306"),
		DBUser: getEnvDefault("DB_USER", "tracker"),
		DBPass: getEnvDefault("DB_PASSWORD", "tracker_dev"),
		DBName: getEnvDefault("DB_NAME", "ops_ledger"),
	}

	db, err := database.Connect(cfg)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	if err := database.Migrate(db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	hub := handlers.NewEventHub()

	t.Cleanup(func() {
		db.Close()
	})

	return &mcpTestEnv{db: db, hub: hub}
}

// mcpCall sends a JSON-RPC request to the MCP handler and returns the response.
func mcpCall(t *testing.T, h *handlers.MCPHandler, body string) (int, mcpRespBody) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h.Handle(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	var resp mcpRespBody
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v\nbody: %s", err, rec.Body.String())
	}
	return rec.Code, resp
}

type mcpRespBody struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// ---------------------------------------------------------------------------
// Integration Tests
// ---------------------------------------------------------------------------

func TestIntegrationMCPCreateChange(t *testing.T) {
	env := setupMCPTestEnv(t)
	h := &handlers.MCPHandler{DB: env.db, Hub: env.hub}

	system := fmt.Sprintf("mcp-test-%d", time.Now().UnixNano())

	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"register_change","arguments":{"system":%q,"type":"deployment","description":"Integration test deploy","environment":"test"}}}`, system)

	code, resp := mcpCall(t, h, body)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	var change models.Change
	if err := json.Unmarshal(resp.Result, &change); err != nil {
		t.Fatalf("decode change: %v", err)
	}
	if change.System != system {
		t.Errorf("expected system %s, got %s", system, change.System)
	}
	if change.Type != "deployment" {
		t.Errorf("expected type deployment, got %s", change.Type)
	}
	if change.ID == 0 {
		t.Error("expected non-zero ID")
	}

	// Verify row exists in DB
	var dbSystem string
	err := env.db.QueryRow("SELECT system FROM changes WHERE id = ?", change.ID).Scan(&dbSystem)
	if err != nil {
		t.Fatalf("DB verify failed: %v", err)
	}
	if dbSystem != system {
		t.Errorf("DB has system %s, want %s", dbSystem, system)
	}

	t.Cleanup(func() {
		env.db.Exec("DELETE FROM changes WHERE id = ?", change.ID)
		env.db.Exec("DELETE FROM audit_log WHERE actor = 'mcp' AND target_id = ?", change.ID)
	})
}

func TestIntegrationMCPUpdateChange(t *testing.T) {
	env := setupMCPTestEnv(t)
	h := &handlers.MCPHandler{DB: env.db, Hub: env.hub}

	system := fmt.Sprintf("mcp-update-test-%d", time.Now().UnixNano())

	// Insert a change directly
	res, err := env.db.Exec(
		"INSERT INTO changes (system, environment, user, type, description) VALUES (?, ?, ?, ?, ?)",
		system, "staging", "test-user", "deployment", "Original description",
	)
	if err != nil {
		t.Fatalf("setup insert: %v", err)
	}
	id, _ := res.LastInsertId()

	t.Cleanup(func() {
		env.db.Exec("DELETE FROM changes WHERE id = ?", id)
		env.db.Exec("DELETE FROM audit_log WHERE actor = 'mcp' AND target_id = ?", id)
	})

	// Update via MCP handler
	updatedDesc := "Updated via MCP integration test"
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"update_change","arguments":{"id":%d,"system":%q,"type":"infrastructure","description":%q}}}`,
		id, system, updatedDesc)

	code, resp := mcpCall(t, h, body)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	// Verify update in DB
	var dbType, dbDesc string
	err = env.db.QueryRow("SELECT type, description FROM changes WHERE id = ?", id).Scan(&dbType, &dbDesc)
	if err != nil {
		t.Fatalf("DB verify failed: %v", err)
	}
	if dbType != "infrastructure" {
		t.Errorf("expected type infrastructure, got %s", dbType)
	}
	if dbDesc != updatedDesc {
		t.Errorf("expected description %q, got %q", updatedDesc, dbDesc)
	}
}

func TestIntegrationMCPDeleteChange(t *testing.T) {
	env := setupMCPTestEnv(t)
	h := &handlers.MCPHandler{DB: env.db, Hub: env.hub}

	system := fmt.Sprintf("mcp-delete-test-%d", time.Now().UnixNano())

	// Insert a change directly
	res, err := env.db.Exec(
		"INSERT INTO changes (system, environment, user, type, description) VALUES (?, ?, ?, ?, ?)",
		system, nil, nil, "configuration", "To be deleted",
	)
	if err != nil {
		t.Fatalf("setup insert: %v", err)
	}
	id, _ := res.LastInsertId()

	t.Cleanup(func() {
		// Row should already be deleted; this is a safety net
		env.db.Exec("DELETE FROM changes WHERE id = ?", id)
		env.db.Exec("DELETE FROM audit_log WHERE actor = 'mcp' AND target_id = ?", id)
	})

	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"delete_change","arguments":{"id":%d}}}`, id)

	code, resp := mcpCall(t, h, body)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result["message"] != "Change deleted" {
		t.Errorf("expected 'Change deleted', got %s", result["message"])
	}

	// Verify row is gone
	var count int
	err = env.db.QueryRow("SELECT COUNT(*) FROM changes WHERE id = ?", id).Scan(&count)
	if err != nil {
		t.Fatalf("DB verify count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected row to be deleted, got count %d", count)
	}
}

func TestIntegrationMCPListChanges(t *testing.T) {
	env := setupMCPTestEnv(t)
	h := &handlers.MCPHandler{DB: env.db, Hub: env.hub}

	// Use a unique system name prefix to isolate this test
	uniqueSystem := fmt.Sprintf("mcp-list-test-%d", time.Now().UnixNano())
	otherSystem := fmt.Sprintf("mcp-list-other-%d", time.Now().UnixNano())

	// Insert 2 rows with uniqueSystem, 1 row with otherSystem
	var insertedIDs []int64
	for i := 0; i < 2; i++ {
		res, err := env.db.Exec(
			"INSERT INTO changes (system, environment, user, type, description) VALUES (?, ?, ?, ?, ?)",
			uniqueSystem, "prod", "ci-bot", "deployment", fmt.Sprintf("Deploy %d", i),
		)
		if err != nil {
			t.Fatalf("setup insert %d: %v", i, err)
		}
		id, _ := res.LastInsertId()
		insertedIDs = append(insertedIDs, id)
	}
	res, err := env.db.Exec(
		"INSERT INTO changes (system, environment, user, type, description) VALUES (?, ?, ?, ?, ?)",
		otherSystem, "staging", "dev", "configuration", "Other system change",
	)
	if err != nil {
		t.Fatalf("setup insert other: %v", err)
	}
	otherID, _ := res.LastInsertId()
	insertedIDs = append(insertedIDs, otherID)

	t.Cleanup(func() {
		for _, id := range insertedIDs {
			env.db.Exec("DELETE FROM changes WHERE id = ?", id)
		}
	})

	// List with system filter — should return exactly 2
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_changes","arguments":{"system":%q}}}`, uniqueSystem)

	code, resp := mcpCall(t, h, body)
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d", code)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}

	total, _ := result["total"].(float64)
	if int(total) != 2 {
		t.Errorf("expected total 2 for system filter, got %v", total)
	}

	changes, _ := result["changes"].([]interface{})
	if len(changes) != 2 {
		t.Errorf("expected 2 changes in response, got %d", len(changes))
	}
}
