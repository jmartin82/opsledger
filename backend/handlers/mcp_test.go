package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/labstack/echo/v4"
)

// mcpResp mirrors the JSON-RPC response envelope for decoding in tests.
type mcpResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func setupMCPContext(body string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func decodeMCPResp(t *testing.T, body []byte) mcpResp {
	t.Helper()
	var resp mcpResp
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("failed to decode MCP response: %v\nbody: %s", err, body)
	}
	return resp
}

// ---------------------------------------------------------------------------
// Protocol Tests (no DB needed)
// ---------------------------------------------------------------------------

func TestMCPHandle_Initialize(t *testing.T) {
	h := &MCPHandler{DB: nil, Hub: nil}

	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocolVersion 2024-11-05, got %v", result["protocolVersion"])
	}
	serverInfo, _ := result["serverInfo"].(map[string]interface{})
	if serverInfo["name"] != "ops-ledger" {
		t.Errorf("expected serverInfo.name ops-ledger, got %v", serverInfo["name"])
	}
}

func TestMCPHandle_ToolsList(t *testing.T) {
	h := &MCPHandler{DB: nil, Hub: nil}

	body := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}

	tools, _ := result["tools"].([]interface{})
	if len(tools) != 4 {
		t.Fatalf("expected 4 tools, got %d", len(tools))
	}

	wantNames := map[string]bool{
		"register_change": true,
		"update_change":   true,
		"delete_change":   true,
		"list_changes":    true,
	}
	for _, tool := range tools {
		m, _ := tool.(map[string]interface{})
		name, _ := m["name"].(string)
		if !wantNames[name] {
			t.Errorf("unexpected tool name: %s", name)
		}
		delete(wantNames, name)
	}
	for name := range wantNames {
		t.Errorf("missing expected tool: %s", name)
	}
}

func TestMCPHandle_InvalidMethod(t *testing.T) {
	h := &MCPHandler{DB: nil, Hub: nil}

	body := `{"jsonrpc":"2.0","id":3,"method":"no/such/method"}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error == nil {
		t.Fatal("expected error, got nil")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected code -32601, got %d", resp.Error.Code)
	}
}

func TestMCPHandle_InvalidJSON(t *testing.T) {
	h := &MCPHandler{DB: nil, Hub: nil}

	c, rec := setupMCPContext("{not valid json}")

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error == nil {
		t.Fatal("expected error in body")
	}
	if resp.Error.Code != -32700 {
		t.Errorf("expected parse error code -32700, got %d", resp.Error.Code)
	}
}

// ---------------------------------------------------------------------------
// register_change Tests
// ---------------------------------------------------------------------------

func TestMCPCreateChange_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	now := time.Now()
	mock.ExpectExec("INSERT INTO changes").
		WithArgs("api-service", nil, nil, "deployment", "Deployed v2.0").
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).AddRow(uint64(1), "api-service", nil, nil, "deployment", "Deployed v2.0", now))

	mock.ExpectExec("INSERT INTO audit_log").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"register_change","arguments":{"system":"api-service","type":"deployment","description":"Deployed v2.0"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	var change map[string]interface{}
	if err := json.Unmarshal(resp.Result, &change); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if change["system"] != "api-service" {
		t.Errorf("expected system api-service, got %v", change["system"])
	}
	if change["type"] != "deployment" {
		t.Errorf("expected type deployment, got %v", change["type"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMCPCreateChange_MissingRequired(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	tests := []struct {
		name string
		args string
	}{
		{"missing system", `{"type":"deployment","description":"test"}`},
		{"missing type", `{"system":"api-service","description":"test"}`},
		{"missing description", `{"system":"api-service","type":"deployment"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"register_change","arguments":` + tt.args + `}}`
			c, rec := setupMCPContext(body)
			if err := h.Handle(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			resp := decodeMCPResp(t, rec.Body.Bytes())
			if resp.Error == nil {
				t.Fatal("expected error, got nil")
			}
			if resp.Error.Code != -32602 {
				t.Errorf("expected -32602, got %d", resp.Error.Code)
			}
		})
	}
}

func TestMCPCreateChange_InvalidType(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"register_change","arguments":{"system":"api-service","type":"rollback","description":"test"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error == nil {
		t.Fatal("expected error for invalid type")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected -32602, got %d", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "infrastructure") {
		t.Errorf("expected message to mention 'infrastructure', got: %s", resp.Error.Message)
	}
}

func TestMCPCreateChange_WithTimestamp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	now := time.Now()
	// With timestamp, INSERT has 6 args (includes created_at)
	mock.ExpectExec("INSERT INTO changes").
		WithArgs("api-service", nil, nil, "deployment", "Deployed v2.0", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).AddRow(uint64(1), "api-service", nil, nil, "deployment", "Deployed v2.0", now))

	mock.ExpectExec("INSERT INTO audit_log").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"register_change","arguments":{"system":"api-service","type":"deployment","description":"Deployed v2.0","timestamp":"2025-01-15T10:30:00Z"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected error: %+v", resp.Error)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMCPCreateChange_InvalidTimestamp(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"register_change","arguments":{"system":"api-service","type":"deployment","description":"test","timestamp":"not-a-date"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error == nil {
		t.Fatal("expected error for invalid timestamp")
	}
	if !strings.Contains(resp.Error.Message, "RFC3339") {
		t.Errorf("expected RFC3339 mention, got: %s", resp.Error.Message)
	}
}

// ---------------------------------------------------------------------------
// update_change Tests
// ---------------------------------------------------------------------------

func TestMCPUpdateChange_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	now := time.Now()
	// UPDATE without timestamp: 6 args (system, env, user, type, desc, id)
	mock.ExpectExec("UPDATE changes SET").
		WithArgs("api-service", nil, nil, "infrastructure", "Updated desc", uint64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(uint64(42)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).AddRow(uint64(42), "api-service", nil, nil, "infrastructure", "Updated desc", now))

	mock.ExpectExec("INSERT INTO audit_log").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"update_change","arguments":{"id":42,"system":"api-service","type":"infrastructure","description":"Updated desc"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	var change map[string]interface{}
	if err := json.Unmarshal(resp.Result, &change); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if change["system"] != "api-service" {
		t.Errorf("expected system api-service, got %v", change["system"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMCPUpdateChange_MissingID(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"update_change","arguments":{"system":"api-service","type":"deployment","description":"test"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error == nil {
		t.Fatal("expected error for missing id")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected -32602, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "id is required" {
		t.Errorf("expected 'id is required', got: %s", resp.Error.Message)
	}
}

func TestMCPUpdateChange_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	mock.ExpectExec("UPDATE changes SET").
		WillReturnResult(sqlmock.NewResult(0, 0))

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"update_change","arguments":{"id":999,"system":"api-service","type":"deployment","description":"test"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error == nil {
		t.Fatal("expected error for not found")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected -32602, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "Change not found" {
		t.Errorf("expected 'Change not found', got: %s", resp.Error.Message)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// delete_change Tests
// ---------------------------------------------------------------------------

func TestMCPDeleteChange_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	now := time.Now()
	// Fetch for audit log
	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).AddRow(uint64(7), "k8s-cluster", nil, nil, "infrastructure", "Updated ingress", now))

	mock.ExpectExec("DELETE FROM changes WHERE id").
		WithArgs(uint64(7)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec("INSERT INTO audit_log").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"delete_change","arguments":{"id":7}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	var result map[string]string
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result["message"] != "Change deleted" {
		t.Errorf("expected 'Change deleted', got: %s", result["message"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMCPDeleteChange_MissingID(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"delete_change","arguments":{}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error == nil {
		t.Fatal("expected error for missing id")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected -32602, got %d", resp.Error.Code)
	}
}

func TestMCPDeleteChange_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	// GetChangeByID returns sql.ErrNoRows — handler should return "Change not found"
	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(uint64(404)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		})) // empty rows → sql.ErrNoRows on Scan

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"delete_change","arguments":{"id":404}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error == nil {
		t.Fatal("expected error for not found")
	}
	if resp.Error.Code != -32602 {
		t.Errorf("expected -32602, got %d", resp.Error.Code)
	}
	if resp.Error.Message != "Change not found" {
		t.Errorf("expected 'Change not found', got: %s", resp.Error.Message)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

// ---------------------------------------------------------------------------
// list_changes Tests
// ---------------------------------------------------------------------------

func TestMCPListChanges_NoFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	now := time.Now()
	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	mock.ExpectQuery("SELECT .+ FROM changes").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).
			AddRow(uint64(1), "api-service", nil, nil, "deployment", "Deploy v1", now).
			AddRow(uint64(2), "db-primary", nil, nil, "infrastructure", "Scale up", now))

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_changes","arguments":{}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}

	total, _ := result["total"].(float64)
	if int(total) != 2 {
		t.Errorf("expected total 2, got %v", total)
	}
	limit, _ := result["limit"].(float64)
	if int(limit) != 50 {
		t.Errorf("expected default limit 50, got %v", limit)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMCPListChanges_WithFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	now := time.Now()
	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery("SELECT .+ FROM changes").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).AddRow(uint64(1), "api-service", "prod", nil, "deployment", "Deploy v1", now))

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_changes","arguments":{"system":"api-service","environment":"prod","type":"deployment"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}

	total, _ := result["total"].(float64)
	if int(total) != 1 {
		t.Errorf("expected total 1, got %v", total)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMCPListChanges_LimitEnforcement(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT .+ FROM changes").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}))

	// Send limit:999 — should be capped at 200
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_changes","arguments":{"limit":999}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	var result map[string]interface{}
	json.Unmarshal(resp.Result, &result)

	limit, _ := result["limit"].(float64)
	if int(limit) != 200 {
		t.Errorf("expected limit capped to 200, got %v", limit)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMCPListChanges_DefaultLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT .+ FROM changes").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}))

	// No limit arg — should default to 50
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_changes","arguments":{}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	var result map[string]interface{}
	json.Unmarshal(resp.Result, &result)

	limit, _ := result["limit"].(float64)
	if int(limit) != 50 {
		t.Errorf("expected default limit 50, got %v", limit)
	}
}
