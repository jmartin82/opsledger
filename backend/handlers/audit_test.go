package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/labstack/echo/v4"

	"ops-ledger/backend/models"
)

func TestAuditList_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuditHandler{DB: db}
	now := time.Now()

	// Count query
	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	// List query
	mock.ExpectQuery("SELECT id, actor, actor_id, action, target_type, target_id, details, ip_address, created_at FROM audit_log").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "actor", "actor_id", "action", "target_type", "target_id", "details", "ip_address", "created_at",
		}).
			AddRow(2, "admin@example.com", 1, "user.login", "user", 1, "admin@example.com", "127.0.0.1", now).
			AddRow(1, "admin@example.com", 1, "user.register", "user", 1, "admin@example.com", "127.0.0.1", now))

	c, rec := setupAuthContext(http.MethodGet, "/api/admin/audit?limit=50&offset=0", "", "admin", 1, "admin@example.com")

	err = h.List(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp listAuditResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(resp.Entries))
	}
	if resp.Total != 2 {
		t.Errorf("expected total=2, got %d", resp.Total)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestAuditList_NonAdmin(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuditHandler{DB: db}

	c, rec := setupAuthContext(http.MethodGet, "/api/admin/audit", "", "viewer", 2, "viewer@example.com")

	_ = h.List(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAuditList_WithFilters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuditHandler{DB: db}
	now := time.Now()

	// Count query with WHERE action = ?
	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// List query with WHERE action = ?
	mock.ExpectQuery("SELECT id, actor, actor_id, action, target_type, target_id, details, ip_address, created_at FROM audit_log").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "actor", "actor_id", "action", "target_type", "target_id", "details", "ip_address", "created_at",
		}).
			AddRow(1, "admin@example.com", 1, "user.login", "user", 1, nil, "127.0.0.1", now))

	c, rec := setupAuthContext(http.MethodGet, "/api/admin/audit?action=user.login&limit=10", "", "admin", 1, "admin@example.com")

	err = h.List(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp listAuditResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(resp.Entries))
	}
	if resp.Entries[0].Action != "user.login" {
		t.Errorf("expected action=user.login, got %s", resp.Entries[0].Action)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestAuditList_EmptyResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuditHandler{DB: db}

	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	mock.ExpectQuery("SELECT id, actor, actor_id, action, target_type, target_id, details, ip_address, created_at FROM audit_log").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "actor", "actor_id", "action", "target_type", "target_id", "details", "ip_address", "created_at",
		}))

	c, rec := setupAuthContext(http.MethodGet, "/api/admin/audit", "", "admin", 1, "admin@example.com")

	err = h.List(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp listAuditResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(resp.Entries))
	}
	if resp.Total != 0 {
		t.Errorf("expected total=0, got %d", resp.Total)
	}
}

// Verify extractActor works with JWT claims
func TestExtractActor_JWT(t *testing.T) {
	c, _ := setupAuthContext(http.MethodGet, "/", "", "admin", 42, "admin@example.com")

	actor, actorID := extractActor(c)
	if actor != "admin@example.com" {
		t.Errorf("expected actor=admin@example.com, got %s", actor)
	}
	if actorID == nil || *actorID != 42 {
		t.Errorf("expected actorID=42, got %v", actorID)
	}
}

// Verify extractActor works with API key context
func TestExtractActor_APIKey(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("apiKeyCreatedBy", uint64(10))

	actor, actorID := extractActor(c)
	if actor != "api_key" {
		t.Errorf("expected actor=api_key, got %s", actor)
	}
	if actorID == nil || *actorID != 10 {
		t.Errorf("expected actorID=10, got %v", actorID)
	}
}

// Suppress unused import warning
var _ = models.AuditEntry{}
