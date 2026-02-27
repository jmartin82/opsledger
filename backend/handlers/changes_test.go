package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	mw "ops-ledger/backend/middleware"
	"ops-ledger/backend/models"
)

const testUUID = "550e8400-e29b-41d4-a716-446655440000"

func setupChangeContext(method, path, body, role string, userID uint64, apiKeyScopes []string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if len(apiKeyScopes) > 0 {
		c.Set("apiKeyScopes", apiKeyScopes)
	} else {
		claims := &mw.JWTClaims{
			RegisteredClaims: jwt.RegisteredClaims{
				Subject: strconv.FormatUint(userID, 10),
			},
			Email: "test@example.com",
			Role:  role,
		}
		c.Set("claims", claims)
	}
	return c, rec
}

// ---------------------------------------------------------------------------
// Create Tests
// ---------------------------------------------------------------------------

func TestChangeCreate_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	// Insert change — UUID is generated at runtime, use AnyArg()
	mock.ExpectExec("INSERT INTO changes").
		WithArgs(sqlmock.AnyArg(), "production", "prod", "ci-bot", "deployment", "Deployed v1.2.0").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// GetChangeByID
	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).AddRow(testUUID, "production", "prod", "ci-bot", "deployment", "Deployed v1.2.0", now))

	env := "prod"
	user := "ci-bot"
	body := `{"system":"production","environment":"prod","user":"ci-bot","type":"deployment","description":"Deployed v1.2.0"}`
	c, rec := setupChangeContext(http.MethodPost, "/api/changes", body, "admin", 1, nil)

	err = h.Create(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var change models.Change
	if err := json.Unmarshal(rec.Body.Bytes(), &change); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if change.System != "production" || change.Type != "deployment" {
		t.Errorf("unexpected change: %+v", change)
	}

	_ = env
	_ = user
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestChangeCreate_ViewerRoleForbidden(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	body := `{"system":"test","type":"deployment","description":"test"}`
	c, rec := setupChangeContext(http.MethodPost, "/api/changes", body, "viewer", 1, nil)

	_ = h.Create(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeCreate_MissingAPIKeyScope(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	// API key with only read scope
	body := `{"system":"test","type":"deployment","description":"test"}`
	c, rec := setupChangeContext(http.MethodPost, "/api/changes", body, "", 0, []string{"changes:read"})

	_ = h.Create(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeCreate_MissingRequiredFields(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	tests := []struct {
		name string
		body string
	}{
		{"missing system", `{"type":"deployment","description":"test"}`},
		{"missing type", `{"system":"test","description":"test"}`},
		{"missing description", `{"system":"test","type":"deployment"}`},
		{"empty body", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := setupChangeContext(http.MethodPost, "/api/changes", tt.body, "admin", 1, nil)
			_ = h.Create(c)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestChangeCreate_InvalidType(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	body := `{"system":"test","type":"invalid","description":"test"}`
	c, rec := setupChangeContext(http.MethodPost, "/api/changes", body, "admin", 1, nil)

	_ = h.Create(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "must be") {
		t.Error("expected type validation error")
	}
}

func TestChangeCreate_EmptyEnvironmentAndUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	// Empty strings should be normalized to nil
	mock.ExpectExec("INSERT INTO changes").
		WithArgs(sqlmock.AnyArg(), "test-system", nil, nil, "deployment", "Test description").
		WillReturnResult(sqlmock.NewResult(0, 1))

	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).AddRow(testUUID, "test-system", nil, nil, "deployment", "Test description", now))

	// Send empty strings
	body := `{"system":"test-system","environment":"","user":"","type":"deployment","description":"Test description"}`
	c, rec := setupChangeContext(http.MethodPost, "/api/changes", body, "admin", 1, nil)

	_ = h.Create(c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var change models.Change
	json.Unmarshal(rec.Body.Bytes(), &change)
	if change.Environment != nil || change.User != nil {
		t.Errorf("expected nil environment and user, got env=%v user=%v", change.Environment, change.User)
	}
}

func TestChangeCreate_WithTimestamp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	// With explicit timestamp, the INSERT includes created_at (7 args: id + 6 fields)
	mock.ExpectExec("INSERT INTO changes").
		WithArgs(sqlmock.AnyArg(), "production", "prod", "ci-bot", "deployment", "Deployed v1.2.0", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).AddRow(testUUID, "production", "prod", "ci-bot", "deployment", "Deployed v1.2.0", now))

	body := `{"system":"production","environment":"prod","user":"ci-bot","type":"deployment","description":"Deployed v1.2.0","timestamp":"2025-01-15T10:30:00Z"}`
	c, rec := setupChangeContext(http.MethodPost, "/api/changes", body, "admin", 1, nil)

	err = h.Create(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestChangeCreate_InvalidTimestamp(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	body := `{"system":"test","type":"deployment","description":"test","timestamp":"not-a-date"}`
	c, rec := setupChangeContext(http.MethodPost, "/api/changes", body, "admin", 1, nil)

	_ = h.Create(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Invalid timestamp") {
		t.Error("expected timestamp validation error")
	}
}

// ---------------------------------------------------------------------------
// List Tests
// ---------------------------------------------------------------------------

func TestChangeList_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	now := time.Now()

	// Count query
	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	// List query
	mock.ExpectQuery("SELECT .+ FROM changes").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).
			AddRow(testUUID, "prod", "prod", "admin", "deployment", "Deployed v1.0", now).
			AddRow("661e8400-e29b-41d4-a716-446655440001", "staging", "staging", "ci", "configuration", "Updated config", now))

	c, rec := setupChangeContext(http.MethodGet, "/api/changes", "", "viewer", 1, nil)

	err = h.List(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp listChangesResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Changes) != 2 {
		t.Errorf("expected 2 changes, got %d", len(resp.Changes))
	}
	if resp.Total != 2 {
		t.Errorf("expected total 2, got %d", resp.Total)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestChangeList_Filters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	// Count with filter
	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// List with filter
	mock.ExpectQuery("SELECT .+ FROM changes").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).AddRow(testUUID, "prod", "prod", "admin", "deployment", "Test", time.Now()))

	c, rec := setupChangeContext(http.MethodGet, "/api/changes?system=prod&type=deployment", "", "viewer", 1, nil)

	_ = h.List(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp listChangesResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Total != 1 {
		t.Errorf("expected 1 change, got %d", resp.Total)
	}
}

func TestChangeList_Pagination(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))
	mock.ExpectQuery("SELECT .+ FROM changes").WillReturnRows(sqlmock.NewRows([]string{
		"id", "system", "environment", "user", "type", "description", "created_at",
	}).AddRow(testUUID, "test", nil, nil, "deployment", "Test", time.Now()))

	c, rec := setupChangeContext(http.MethodGet, "/api/changes?limit=10&offset=5", "", "viewer", 1, nil)

	_ = h.List(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp listChangesResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Limit != 10 || resp.Offset != 5 {
		t.Errorf("expected limit=10 offset=5, got limit=%d offset=%d", resp.Limit, resp.Offset)
	}
}

func TestChangeList_TimeRange(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT .+ FROM changes").WillReturnRows(sqlmock.NewRows([]string{
		"id", "system", "environment", "user", "type", "description", "created_at",
	}))

	// Test explicit from/to
	from := time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	to := time.Now().Format(time.RFC3339)
	c, rec := setupChangeContext(http.MethodGet, "/api/changes?from="+from+"&to="+to, "", "viewer", 1, nil)

	_ = h.List(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestChangeList_TimeRangeShorthand(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT .+ FROM changes").WillReturnRows(sqlmock.NewRows([]string{
		"id", "system", "environment", "user", "type", "description", "created_at",
	}))

	// Test shorthand
	c, rec := setupChangeContext(http.MethodGet, "/api/changes?timeRange=24h", "", "viewer", 1, nil)

	_ = h.List(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestChangeList_EmptyResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT .+ FROM changes").WillReturnRows(sqlmock.NewRows([]string{
		"id", "system", "environment", "user", "type", "description", "created_at",
	}))

	c, rec := setupChangeContext(http.MethodGet, "/api/changes", "", "viewer", 1, nil)

	_ = h.List(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp listChangesResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Changes) != 0 {
		t.Errorf("expected empty slice, got %d changes", len(resp.Changes))
	}
}

func TestChangeList_APIKeyReadScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT .+ FROM changes").WillReturnRows(sqlmock.NewRows([]string{
		"id", "system", "environment", "user", "type", "description", "created_at",
	}))

	// API key with read scope
	c, rec := setupChangeContext(http.MethodGet, "/api/changes", "", "", 0, []string{"changes:read"})

	_ = h.List(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeList_APIKeyMissingReadScope(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	// API key without read scope
	c, rec := setupChangeContext(http.MethodGet, "/api/changes", "", "", 0, []string{"changes:write"})

	_ = h.List(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Update Tests
// ---------------------------------------------------------------------------

func TestChangeUpdate_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	mock.ExpectExec("UPDATE changes SET").
		WithArgs("api-gateway", "staging", "alice", "deployment", "Updated deploy", testUUID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(testUUID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).AddRow(testUUID, "api-gateway", "staging", "alice", "deployment", "Updated deploy", now))

	body := `{"system":"api-gateway","environment":"staging","user":"alice","type":"deployment","description":"Updated deploy"}`
	c, rec := setupChangeContext(http.MethodPut, "/api/changes/"+testUUID, body, "admin", 1, nil)
	c.SetParamNames("id")
	c.SetParamValues(testUUID)

	_ = h.Update(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var change models.Change
	json.Unmarshal(rec.Body.Bytes(), &change)
	if change.System != "api-gateway" || change.Description != "Updated deploy" {
		t.Errorf("unexpected change: %+v", change)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestChangeUpdate_WithTimestamp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	// With explicit timestamp, the UPDATE includes created_at (7 args)
	mock.ExpectExec("UPDATE changes SET").
		WithArgs("api-gateway", "staging", "alice", "deployment", "Updated deploy", sqlmock.AnyArg(), testUUID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(testUUID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).AddRow(testUUID, "api-gateway", "staging", "alice", "deployment", "Updated deploy", now))

	body := `{"system":"api-gateway","environment":"staging","user":"alice","type":"deployment","description":"Updated deploy","timestamp":"2025-01-15T10:30:00Z"}`
	c, rec := setupChangeContext(http.MethodPut, "/api/changes/"+testUUID, body, "admin", 1, nil)
	c.SetParamNames("id")
	c.SetParamValues(testUUID)

	_ = h.Update(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestChangeUpdate_InvalidTimestamp(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	body := `{"system":"test","type":"deployment","description":"test","timestamp":"bad-format"}`
	c, rec := setupChangeContext(http.MethodPut, "/api/changes/"+testUUID, body, "admin", 1, nil)
	c.SetParamNames("id")
	c.SetParamValues(testUUID)

	_ = h.Update(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Invalid timestamp") {
		t.Error("expected timestamp validation error")
	}
}

func TestChangeUpdate_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	mock.ExpectExec("UPDATE changes SET").
		WillReturnResult(sqlmock.NewResult(0, 0))

	body := `{"system":"test","type":"deployment","description":"test"}`
	c, rec := setupChangeContext(http.MethodPut, "/api/changes/"+testUUID, body, "admin", 1, nil)
	c.SetParamNames("id")
	c.SetParamValues(testUUID)

	_ = h.Update(c)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeUpdate_ViewerForbidden(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	body := `{"system":"test","type":"deployment","description":"test"}`
	c, rec := setupChangeContext(http.MethodPut, "/api/changes/"+testUUID, body, "viewer", 1, nil)
	c.SetParamNames("id")
	c.SetParamValues(testUUID)

	_ = h.Update(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeUpdate_APIKeyWithWriteScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	mock.ExpectExec("UPDATE changes SET").
		WillReturnResult(sqlmock.NewResult(0, 1))

	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).AddRow(testUUID, "test", nil, nil, "deployment", "test", now))

	body := `{"system":"test","type":"deployment","description":"test"}`
	c, rec := setupChangeContext(http.MethodPut, "/api/changes/"+testUUID, body, "", 0, []string{"changes:write"})
	c.SetParamNames("id")
	c.SetParamValues(testUUID)

	_ = h.Update(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeUpdate_APIKeyMissingWriteScope(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	body := `{"system":"test","type":"deployment","description":"test"}`
	c, rec := setupChangeContext(http.MethodPut, "/api/changes/"+testUUID, body, "", 0, []string{"changes:read"})
	c.SetParamNames("id")
	c.SetParamValues(testUUID)

	_ = h.Update(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeUpdate_InvalidID(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	body := `{"system":"test","type":"deployment","description":"test"}`
	c, rec := setupChangeContext(http.MethodPut, "/api/changes/abc", body, "admin", 1, nil)
	c.SetParamNames("id")
	c.SetParamValues("abc")

	_ = h.Update(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeUpdate_MissingFields(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	tests := []struct {
		name string
		body string
	}{
		{"missing system", `{"type":"deployment","description":"test"}`},
		{"missing type", `{"system":"test","description":"test"}`},
		{"missing description", `{"system":"test","type":"deployment"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := setupChangeContext(http.MethodPut, "/api/changes/"+testUUID, tt.body, "admin", 1, nil)
			c.SetParamNames("id")
			c.SetParamValues(testUUID)
			_ = h.Update(c)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestChangeUpdate_InvalidType(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	body := `{"system":"test","type":"invalid","description":"test"}`
	c, rec := setupChangeContext(http.MethodPut, "/api/changes/"+testUUID, body, "admin", 1, nil)
	c.SetParamNames("id")
	c.SetParamValues(testUUID)

	_ = h.Update(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Delete Tests
// ---------------------------------------------------------------------------

func TestChangeDelete_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	now := time.Now()
	// Fetch for audit log
	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(testUUID).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).AddRow(testUUID, "prod", "production", "admin", "deployment", "Deploy v1", now))

	// Delete
	mock.ExpectExec("DELETE FROM changes WHERE id").
		WithArgs(testUUID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	c, rec := setupChangeContext(http.MethodDelete, "/api/changes/"+testUUID, "", "admin", 1, nil)
	c.SetParamNames("id")
	c.SetParamValues(testUUID)

	_ = h.Delete(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["message"] != "Change deleted" {
		t.Errorf("unexpected message: %s", resp["message"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestChangeDelete_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WithArgs(testUUID).
		WillReturnError(sql.ErrNoRows)

	c, rec := setupChangeContext(http.MethodDelete, "/api/changes/"+testUUID, "", "admin", 1, nil)
	c.SetParamNames("id")
	c.SetParamValues(testUUID)

	_ = h.Delete(c)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeDelete_ViewerForbidden(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	c, rec := setupChangeContext(http.MethodDelete, "/api/changes/"+testUUID, "", "viewer", 1, nil)
	c.SetParamNames("id")
	c.SetParamValues(testUUID)

	_ = h.Delete(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeDelete_APIKeyWithWriteScope(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM changes WHERE id").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "system", "environment", "user", "type", "description", "created_at",
		}).AddRow(testUUID, "test", nil, nil, "deployment", "test", now))

	mock.ExpectExec("DELETE FROM changes WHERE id").
		WillReturnResult(sqlmock.NewResult(0, 1))

	c, rec := setupChangeContext(http.MethodDelete, "/api/changes/"+testUUID, "", "", 0, []string{"changes:write"})
	c.SetParamNames("id")
	c.SetParamValues(testUUID)

	_ = h.Delete(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeDelete_APIKeyMissingWriteScope(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	c, rec := setupChangeContext(http.MethodDelete, "/api/changes/"+testUUID, "", "", 0, []string{"changes:read"})
	c.SetParamNames("id")
	c.SetParamValues(testUUID)

	_ = h.Delete(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeDelete_InvalidID(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ChangeHandler{DB: db}

	c, rec := setupChangeContext(http.MethodDelete, "/api/changes/abc", "", "admin", 1, nil)
	c.SetParamNames("id")
	c.SetParamValues("abc")

	_ = h.Delete(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}
