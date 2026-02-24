package handlers

import (
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

func setupContext(method, path, body, role string, userID uint64) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	claims := &mw.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject: strconv.FormatUint(userID, 10),
		},
		Email: "admin@test.com",
		Role:  role,
	}
	c.Set("claims", claims)
	return c, rec
}

func TestCreate_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ApiKeyHandler{DB: db}

	// Expect INSERT
	mock.ExpectExec("INSERT INTO api_keys").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "changes:read,changes:write", uint64(1), nil).
		WillReturnResult(sqlmock.NewResult(1, 1))

	// Expect SELECT for GetApiKeyByID
	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM api_keys WHERE id").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "key_hash", "prefix", "scopes", "status", "created_by", "expires_at", "last_used", "created_at",
		}).AddRow(1, "CI Pipeline", "somehash", "ol_live_xxxx", "changes:read,changes:write", "active", 1, nil, nil, now))

	body := `{"name":"CI Pipeline","scopes":["changes:read","changes:write"]}`
	c, rec := setupContext(http.MethodPost, "/api/admin/api-keys", body, "admin", 1)

	err = h.Create(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Key    string         `json:"key"`
		ApiKey *models.ApiKey `json:"apiKey"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(resp.Key, "ol_live_") {
		t.Errorf("key should start with ol_live_, got %q", resp.Key)
	}
	if resp.ApiKey == nil {
		t.Fatal("expected apiKey in response")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestCreate_MissingName(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ApiKeyHandler{DB: db}

	body := `{"name":"","scopes":["changes:read"]}`
	c, rec := setupContext(http.MethodPost, "/api/admin/api-keys", body, "admin", 1)

	_ = h.Create(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreate_InvalidScopes(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ApiKeyHandler{DB: db}

	body := `{"name":"test","scopes":["invalid:scope"]}`
	c, rec := setupContext(http.MethodPost, "/api/admin/api-keys", body, "admin", 1)

	_ = h.Create(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreate_NonAdmin(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ApiKeyHandler{DB: db}

	body := `{"name":"test","scopes":["changes:read"]}`
	c, rec := setupContext(http.MethodPost, "/api/admin/api-keys", body, "viewer", 1)

	_ = h.Create(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRevoke_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ApiKeyHandler{DB: db}

	mock.ExpectExec("UPDATE api_keys SET status").
		WithArgs(uint64(5)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	c, rec := setupContext(http.MethodPost, "/api/admin/api-keys/5/revoke", "", "admin", 1)
	c.SetParamNames("id")
	c.SetParamValues("5")

	_ = h.Revoke(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRevoke_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ApiKeyHandler{DB: db}

	mock.ExpectExec("UPDATE api_keys SET status").
		WithArgs(uint64(99)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	c, rec := setupContext(http.MethodPost, "/api/admin/api-keys/99/revoke", "", "admin", 1)
	c.SetParamNames("id")
	c.SetParamValues("99")

	_ = h.Revoke(c)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestList_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ApiKeyHandler{DB: db}

	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM api_keys WHERE created_by").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "prefix", "scopes", "status", "created_by", "expires_at", "last_used", "created_at",
		}).
			AddRow(1, "Key 1", "ol_live_aaaa", "changes:read", "active", 1, nil, nil, now).
			AddRow(2, "Key 2", "ol_live_bbbb", "changes:read,changes:write", "active", 1, nil, nil, now))

	c, rec := setupContext(http.MethodGet, "/api/admin/api-keys", "", "admin", 1)

	_ = h.List(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var keys []models.ApiKey
	if err := json.Unmarshal(rec.Body.Bytes(), &keys); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRotate_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &ApiKeyHandler{DB: db}

	now := time.Now()

	// GetApiKeyByID for existing key
	mock.ExpectQuery("SELECT .+ FROM api_keys WHERE id").
		WithArgs(uint64(3)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "key_hash", "prefix", "scopes", "status", "created_by", "expires_at", "last_used", "created_at",
		}).AddRow(3, "Rotate Me", "oldhash", "ol_live_aaaa", "changes:read", "active", 1, nil, nil, now))

	// RevokeApiKey
	mock.ExpectExec("UPDATE api_keys SET status").
		WithArgs(uint64(3)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	// CreateApiKey INSERT
	mock.ExpectExec("INSERT INTO api_keys").
		WithArgs("Rotate Me", sqlmock.AnyArg(), sqlmock.AnyArg(), "changes:read", uint64(1), nil).
		WillReturnResult(sqlmock.NewResult(4, 1))

	// GetApiKeyByID for new key
	mock.ExpectQuery("SELECT .+ FROM api_keys WHERE id").
		WithArgs(uint64(4)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "key_hash", "prefix", "scopes", "status", "created_by", "expires_at", "last_used", "created_at",
		}).AddRow(4, "Rotate Me", "newhash", "ol_live_bbbb", "changes:read", "active", 1, nil, nil, now))

	c, rec := setupContext(http.MethodPost, "/api/admin/api-keys/3/rotate", "", "admin", 1)
	c.SetParamNames("id")
	c.SetParamValues("3")

	_ = h.Rotate(c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Key    string         `json:"key"`
		ApiKey *models.ApiKey `json:"apiKey"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !strings.HasPrefix(resp.Key, "ol_live_") {
		t.Errorf("rotated key should start with ol_live_, got %q", resp.Key)
	}
	if resp.ApiKey.ID != 4 {
		t.Errorf("expected new key ID 4, got %d", resp.ApiKey.ID)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}
