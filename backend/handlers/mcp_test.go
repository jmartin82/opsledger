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

// unwrapToolResult extracts the JSON payload from content[0].text of a ToolCallResult.
func unwrapToolResult(t *testing.T, raw json.RawMessage) json.RawMessage {
	t.Helper()
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode ToolCallResult: %v", err)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content[0], got empty content")
	}
	return json.RawMessage(result.Content[0].Text)
}

// assertToolError verifies that resp.Result is a ToolCallResult with isError:true.
// wantMsgContains is checked against content[0].text (pass "" to skip message check).
func assertToolError(t *testing.T, raw json.RawMessage, wantMsgContains string) {
	t.Helper()
	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("decode ToolCallResult: %v", err)
	}
	if !result.IsError {
		t.Errorf("expected isError:true, got false")
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content[0], got empty content")
	}
	if wantMsgContains != "" && !strings.Contains(result.Content[0].Text, wantMsgContains) {
		t.Errorf("expected message to contain %q, got: %s", wantMsgContains, result.Content[0].Text)
	}
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
	if len(tools) != 5 {
		t.Fatalf("expected 5 tools, got %d", len(tools))
	}

	wantNames := map[string]bool{
		"register_change": true,
		"update_change":   true,
		"delete_change":   true,
		"confirm_change":  true,
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
	// UUID is generated at runtime — use AnyArg() for first arg
	mock.ExpectExec("INSERT INTO changes").
		WithArgs(sqlmock.AnyArg(), "api-service", nil, nil, "deployment", "Deployed v2.0", "executed").
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(addChangeRow(sqlmock.NewRows(changeColumns),
			testUUID, "api-service", nil, nil, "deployment", "Deployed v2.0", "executed", now, now))

	mock.ExpectExec("INSERT INTO audit_log").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
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
	if err := json.Unmarshal(unwrapToolResult(t, resp.Result), &change); err != nil {
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
			if resp.Error != nil {
				t.Fatalf("expected no RPC error, got: %+v", resp.Error)
			}
			assertToolError(t, resp.Result, "")
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
	if resp.Error != nil {
		t.Fatalf("expected no RPC error, got: %+v", resp.Error)
	}
	assertToolError(t, resp.Result, "infrastructure")
}

func TestMCPCreateChange_WithTimestamp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	now := time.Now()
	// With timestamp, INSERT has 8 args (id + 5 fields + status + event_at)
	mock.ExpectExec("INSERT INTO changes").
		WithArgs(sqlmock.AnyArg(), "api-service", nil, nil, "deployment", "Deployed v2.0", "executed", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(addChangeRow(sqlmock.NewRows(changeColumns),
			testUUID, "api-service", nil, nil, "deployment", "Deployed v2.0", "executed", now, now))

	mock.ExpectExec("INSERT INTO audit_log").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
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
	if resp.Error != nil {
		t.Fatalf("expected no RPC error, got: %+v", resp.Error)
	}
	assertToolError(t, resp.Result, "RFC3339")
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
	// UPDATE without timestamp: 7 args (system, env, user, type, desc, status, id)
	mock.ExpectExec("UPDATE changes SET").
		WithArgs("api-service", nil, nil, "infrastructure", "Updated desc", "executed", testUUID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(testUUID).
		WillReturnRows(addChangeRow(sqlmock.NewRows(changeColumns),
			testUUID, "api-service", nil, nil, "infrastructure", "Updated desc", "executed", now, now))

	mock.ExpectExec("INSERT INTO audit_log").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"update_change","arguments":{"id":"` + testUUID + `","system":"api-service","type":"infrastructure","description":"Updated desc"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	var change map[string]interface{}
	if err := json.Unmarshal(unwrapToolResult(t, resp.Result), &change); err != nil {
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
	if resp.Error != nil {
		t.Fatalf("expected no RPC error, got: %+v", resp.Error)
	}
	assertToolError(t, resp.Result, "id is required")
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

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"update_change","arguments":{"id":"` + testUUID + `","system":"api-service","type":"deployment","description":"test"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("expected no RPC error, got: %+v", resp.Error)
	}
	assertToolError(t, resp.Result, "Change not found")

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
		WithArgs(testUUID).
		WillReturnRows(addChangeRow(sqlmock.NewRows(changeColumns),
			testUUID, "k8s-cluster", nil, nil, "infrastructure", "Updated ingress", "executed", now, now))

	mock.ExpectExec("DELETE FROM changes WHERE id").
		WithArgs(testUUID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec("INSERT INTO audit_log").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"delete_change","arguments":{"id":"` + testUUID + `"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	var result map[string]string
	if err := json.Unmarshal(unwrapToolResult(t, resp.Result), &result); err != nil {
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
	if resp.Error != nil {
		t.Fatalf("expected no RPC error, got: %+v", resp.Error)
	}
	assertToolError(t, resp.Result, "")
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
		WithArgs(testUUID).
		WillReturnRows(sqlmock.NewRows(changeColumns)) // empty rows → sql.ErrNoRows on Scan

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"delete_change","arguments":{"id":"` + testUUID + `"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("expected no RPC error, got: %+v", resp.Error)
	}
	assertToolError(t, resp.Result, "Change not found")

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
		WillReturnRows(
			addChangeRow(
				addChangeRow(sqlmock.NewRows(changeColumns),
					testUUID, "api-service", nil, nil, "deployment", "Deploy v1", "executed", now, now),
				"661e8400-e29b-41d4-a716-446655440001", "db-primary", nil, nil, "infrastructure", "Scale up", "executed", now, now))

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
	if err := json.Unmarshal(unwrapToolResult(t, resp.Result), &result); err != nil {
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
		WillReturnRows(addChangeRow(sqlmock.NewRows(changeColumns),
			testUUID, "api-service", "prod", nil, "deployment", "Deploy v1", "executed", now, now))

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
	if err := json.Unmarshal(unwrapToolResult(t, resp.Result), &result); err != nil {
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
		WillReturnRows(sqlmock.NewRows(changeColumns))

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
	json.Unmarshal(unwrapToolResult(t, resp.Result), &result)

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
		WillReturnRows(sqlmock.NewRows(changeColumns))

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
	json.Unmarshal(unwrapToolResult(t, resp.Result), &result)

	limit, _ := result["limit"].(float64)
	if int(limit) != 50 {
		t.Errorf("expected default limit 50, got %v", limit)
	}
}

// ---------------------------------------------------------------------------
// confirm_change Tests
// ---------------------------------------------------------------------------

func TestMCPConfirmChange_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	future := time.Now().Add(24 * time.Hour)
	now := time.Now()

	// GetChangeByID — returns scheduled
	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(testUUID).
		WillReturnRows(addChangeRow(sqlmock.NewRows(changeColumns),
			testUUID, "api", nil, nil, "deployment", "v2 deploy", "scheduled", future, now))

	// UPDATE
	mock.ExpectExec("UPDATE changes SET status='executed'").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// GetChangeByID after confirm
	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(testUUID).
		WillReturnRows(addChangeRow(sqlmock.NewRows(changeColumns),
			testUUID, "api", nil, nil, "deployment", "v2 deploy", "executed", now, now))

	// Audit log
	mock.ExpectExec("INSERT INTO audit_log").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"confirm_change","arguments":{"id":"` + testUUID + `"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	var change map[string]interface{}
	if err := json.Unmarshal(unwrapToolResult(t, resp.Result), &change); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if change["status"] != "executed" {
		t.Errorf("expected status executed, got %v", change["status"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMCPConfirmChange_MissingID(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"confirm_change","arguments":{}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("expected no RPC error, got: %+v", resp.Error)
	}
	assertToolError(t, resp.Result, "id is required")
}

func TestMCPConfirmChange_AlreadyExecuted(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(testUUID).
		WillReturnRows(addChangeRow(sqlmock.NewRows(changeColumns),
			testUUID, "api", nil, nil, "deployment", "v2 deploy", "executed", now, now))

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"confirm_change","arguments":{"id":"` + testUUID + `"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("expected no RPC error, got: %+v", resp.Error)
	}
	assertToolError(t, resp.Result, "already marked as executed")
}

// ---------------------------------------------------------------------------
// register_change scheduled Tests
// ---------------------------------------------------------------------------

func TestMCPCreateChange_Scheduled(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	future := time.Now().Add(48 * time.Hour)
	now := time.Now()

	mock.ExpectExec("INSERT INTO changes").
		WithArgs(sqlmock.AnyArg(), "api", nil, nil, "deployment", "v3 deploy", "scheduled", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(addChangeRow(sqlmock.NewRows(changeColumns),
			testUUID, "api", nil, nil, "deployment", "v3 deploy", "scheduled", future, now))

	mock.ExpectExec("INSERT INTO audit_log").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	futureStr := future.UTC().Format(time.RFC3339)
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"register_change","arguments":{"system":"api","type":"deployment","description":"v3 deploy","status":"scheduled","timestamp":"` + futureStr + `"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	var change map[string]interface{}
	if err := json.Unmarshal(unwrapToolResult(t, resp.Result), &change); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if change["status"] != "scheduled" {
		t.Errorf("expected status scheduled, got %v", change["status"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestMCPListChanges_StatusFilter(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &MCPHandler{DB: db, Hub: nil}

	future := time.Now().Add(24 * time.Hour)
	now := time.Now()

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	mock.ExpectQuery("SELECT .+ FROM changes").
		WillReturnRows(addChangeRow(sqlmock.NewRows(changeColumns),
			testUUID, "api", nil, nil, "deployment", "v2 deploy", "scheduled", future, now))

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_changes","arguments":{"status":"scheduled"}}}`
	c, rec := setupMCPContext(body)

	if err := h.Handle(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resp := decodeMCPResp(t, rec.Body.Bytes())
	if resp.Error != nil {
		t.Fatalf("unexpected RPC error: %+v", resp.Error)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(unwrapToolResult(t, resp.Result), &result); err != nil {
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
