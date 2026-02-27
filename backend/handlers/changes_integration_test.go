//go:build integration

package handlers_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"ops-ledger/backend/config"
	"ops-ledger/backend/database"
	"ops-ledger/backend/handlers"
	mw "ops-ledger/backend/middleware"
	"ops-ledger/backend/models"
)

const changesTestJWTSecret = "changes-test-secret"

// changesTestEnv holds shared state for changes integration tests.
type changesTestEnv struct {
	db            *sql.DB
	userID        uint64
	userJWT       string
	adminID       uint64
	adminJWT      string
	viewerID      uint64
	viewerJWT     string
	apiKey        string
	readOnlyKey   string
}

// setupChangesTestEnv creates a test environment with admin, user, and viewer.
func setupChangesTestEnv(t *testing.T) *changesTestEnv {
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

	// Create admin user
	adminHash, _ := bcrypt.GenerateFromPassword([]byte("admin-pass"), bcrypt.MinCost)
	adminEmail := fmt.Sprintf("changes-admin-%d@example.com", time.Now().UnixNano())
	resAdmin, _ := db.Exec(
		"INSERT INTO users (email, name, password_hash, role, status) VALUES (?, ?, ?, 'admin', 'active')",
		adminEmail, "Changes Admin", string(adminHash),
	)
	adminID, _ := resAdmin.LastInsertId()

	// Create regular user
	userHash, _ := bcrypt.GenerateFromPassword([]byte("user-pass"), bcrypt.MinCost)
	userEmail := fmt.Sprintf("changes-user-%d@example.com", time.Now().UnixNano())
	resUser, _ := db.Exec(
		"INSERT INTO users (email, name, password_hash, role, status) VALUES (?, ?, ?, 'editor', 'active')",
		userEmail, "Changes User", string(userHash),
	)
	userID, _ := resUser.LastInsertId()

	// Create viewer
	viewerHash, _ := bcrypt.GenerateFromPassword([]byte("viewer-pass"), bcrypt.MinCost)
	viewerEmail := fmt.Sprintf("changes-viewer-%d@example.com", time.Now().UnixNano())
	resViewer, _ := db.Exec(
		"INSERT INTO users (email, name, password_hash, role, status) VALUES (?, ?, ?, 'viewer', 'active')",
		viewerEmail, "Changes Viewer", string(viewerHash),
	)
	viewerID, _ := resViewer.LastInsertId()

	// Generate JWTs
	userJWT := generateChangesTestJWT(t, uint64(userID), userEmail, "editor")
	adminJWT := generateChangesTestJWT(t, uint64(adminID), adminEmail, "admin")
	viewerJWT := generateChangesTestJWT(t, uint64(viewerID), viewerEmail, "viewer")

	// Create API keys
	rawKey, keyHash, prefix, _ := models.GenerateAPIKey()
	_, _ = db.Exec(
		"INSERT INTO api_keys (name, key_hash, prefix, scopes, created_by) VALUES (?, ?, ?, ?, ?)",
		"Full Access Key", keyHash, prefix, "changes:read,changes:write", adminID,
	)
	apiKey := rawKey

	rawKeyRO, keyHashRO, prefixRO, _ := models.GenerateAPIKey()
	_, _ = db.Exec(
		"INSERT INTO api_keys (name, key_hash, prefix, scopes, created_by) VALUES (?, ?, ?, ?, ?)",
		"Read Only Key", keyHashRO, prefixRO, "changes:read", adminID,
	)
	readOnlyKey := rawKeyRO

	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM api_keys WHERE created_by IN (?, ?, ?)", userID, adminID, viewerID)
		_, _ = db.Exec("DELETE FROM users WHERE id IN (?, ?, ?)", userID, adminID, viewerID)
		db.Close()
	})

	return &changesTestEnv{
		db:          db,
		userID:      uint64(userID),
		userJWT:     userJWT,
		adminID:     uint64(adminID),
		adminJWT:    adminJWT,
		viewerID:    uint64(viewerID),
		viewerJWT:   viewerJWT,
		apiKey:      apiKey,
		readOnlyKey: readOnlyKey,
	}
}

func generateChangesTestJWT(t *testing.T, userID uint64, email, role string) string {
	t.Helper()
	claims := &mw.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(userID, 10),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
		Email: email,
		Role:  role,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(changesTestJWTSecret))
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}
	return signed
}

func changesNewContext(e *echo.Echo, method, path, body, jwtToken string) (echo.Context, *httptest.ResponseRecorder) {
	var reqBody *bytes.Reader
	if body != "" {
		reqBody = bytes.NewReader([]byte(body))
	} else {
		reqBody = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	if jwtToken != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+jwtToken)
	}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	return c, rec
}

func changesInjectJWT(t *testing.T, c echo.Context, token string) {
	t.Helper()
	claims := &mw.JWTClaims{}
	_, _ = jwt.ParseWithClaims(token, claims, func(*jwt.Token) (interface{}, error) {
		return []byte(changesTestJWTSecret), nil
	})
	c.Set("claims", claims)
}

func changesInjectAPIKey(t *testing.T, c echo.Context, scopes []string) {
	t.Helper()
	c.Set("apiKeyScopes", scopes)
}

// ---------------------------------------------------------------------------
// Create Integration Tests
// ---------------------------------------------------------------------------

func TestChangeCreateIntegration_Success(t *testing.T) {
	env := setupChangesTestEnv(t)
	e := echo.New()
	h := &handlers.ChangeHandler{DB: env.db}

	body := `{"system":"production","environment":"prod","user":"ci-bot","type":"deployment","description":"Deployed v1.2.0"}`
	c, rec := changesNewContext(e, http.MethodPost, "/api/changes", body, env.adminJWT)
	changesInjectJWT(t, c, env.adminJWT)

	if err := h.Create(c); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var change models.Change
	if err := json.Unmarshal(rec.Body.Bytes(), &change); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if change.System != "production" || change.Type != "deployment" {
		t.Errorf("expected production/deployment, got %s/%s", change.System, change.Type)
	}

	// Cleanup
	_, _ = env.db.Exec("DELETE FROM changes WHERE id = ?", change.ID)
}

func TestChangeCreateIntegration_ViewerForbidden(t *testing.T) {
	env := setupChangesTestEnv(t)
	e := echo.New()
	h := &handlers.ChangeHandler{DB: env.db}

	body := `{"system":"test","type":"deployment","description":"test"}`
	c, rec := changesNewContext(e, http.MethodPost, "/api/changes", body, env.viewerJWT)
	changesInjectJWT(t, c, env.viewerJWT)

	_ = h.Create(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeCreateIntegration_ValidationErrors(t *testing.T) {
	env := setupChangesTestEnv(t)
	e := echo.New()
	h := &handlers.ChangeHandler{DB: env.db}

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"missing system", `{"type":"deployment","description":"test"}`, http.StatusBadRequest},
		{"missing type", `{"system":"test","description":"test"}`, http.StatusBadRequest},
		{"missing description", `{"system":"test","type":"deployment"}`, http.StatusBadRequest},
		{"invalid type", `{"system":"test","type":"invalid","description":"test"}`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := changesNewContext(e, http.MethodPost, "/api/changes", tt.body, env.adminJWT)
			changesInjectJWT(t, c, env.adminJWT)
			_ = h.Create(c)
			if rec.Code != tt.wantStatus {
				t.Errorf("expected %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

func TestChangeCreateIntegration_APIKeyWithWriteScope(t *testing.T) {
	env := setupChangesTestEnv(t)
	e := echo.New()
	h := &handlers.ChangeHandler{DB: env.db}

	body := `{"system":"api-test","type":"configuration","description":"API key test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/changes", bytes.NewReader([]byte(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+env.apiKey)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	changesInjectAPIKey(t, c, []string{"changes:read", "changes:write"})

	if err := h.Create(c); err != nil {
		t.Fatalf("Create error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var change models.Change
	json.Unmarshal(rec.Body.Bytes(), &change)
	_, _ = env.db.Exec("DELETE FROM changes WHERE id = ?", change.ID)
}

func TestChangeCreateIntegration_APIKeyReadOnlyForbidden(t *testing.T) {
	env := setupChangesTestEnv(t)
	e := echo.New()
	h := &handlers.ChangeHandler{DB: env.db}

	body := `{"system":"test","type":"deployment","description":"test"}`
	req := httptest.NewRequest(http.MethodPost, "/api/changes", bytes.NewReader([]byte(body)))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+env.readOnlyKey)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	changesInjectAPIKey(t, c, []string{"changes:read"}) // only read

	_ = h.Create(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// List Integration Tests
// ---------------------------------------------------------------------------

func TestChangeListIntegration_Success(t *testing.T) {
	env := setupChangesTestEnv(t)
	e := echo.New()
	h := &handlers.ChangeHandler{DB: env.db}

	// Create a change first
	res, _ := env.db.Exec(
		"INSERT INTO changes (system, environment, user, type, description) VALUES (?, ?, ?, ?, ?)",
		"test-system", "test", "test-user", "deployment", "Test change",
	)
	changeID, _ := res.LastInsertId()

	c, rec := changesNewContext(e, http.MethodGet, "/api/changes", "", env.userJWT)
	changesInjectJWT(t, c, env.userJWT)

	if err := h.List(c); err != nil {
		t.Fatalf("List error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Changes []models.Change `json:"changes"`
		Total   int             `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Total < 1 {
		t.Error("expected at least 1 change")
	}

	// Cleanup
	_, _ = env.db.Exec("DELETE FROM changes WHERE id = ?", changeID)
}

func TestChangeListIntegration_Filters(t *testing.T) {
	env := setupChangesTestEnv(t)
	e := echo.New()
	h := &handlers.ChangeHandler{DB: env.db}

	// Create changes with different systems/types
	env.db.Exec(
		"INSERT INTO changes (system, environment, user, type, description) VALUES (?, ?, ?, ?, ?)",
		"prod", "prod", "admin", "deployment", "Prod deploy",
	)
	env.db.Exec(
		"INSERT INTO changes (system, environment, user, type, description) VALUES (?, ?, ?, ?, ?)",
		"staging", "staging", "ci", "configuration", "Staging config",
	)

	// Filter by system
	c, rec := changesNewContext(e, http.MethodGet, "/api/changes?system=prod", "", env.userJWT)
	changesInjectJWT(t, c, env.userJWT)
	_ = h.List(c)

	var resp struct {
		Changes []models.Change `json:"changes"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	// Should only get prod changes

	// Cleanup
	_, _ = env.db.Exec("DELETE FROM changes WHERE system IN ('prod', 'staging')")
}

func TestChangeListIntegration_Pagination(t *testing.T) {
	env := setupChangesTestEnv(t)
	e := echo.New()
	h := &handlers.ChangeHandler{DB: env.db}

	// Create 5 changes
	for i := 0; i < 5; i++ {
		env.db.Exec(
			"INSERT INTO changes (system, environment, user, type, description) VALUES (?, ?, ?, ?, ?)",
			fmt.Sprintf("system-%d", i), "test", "test", "deployment", fmt.Sprintf("Change %d", i),
		)
	}

	// Limit to 2
	c, rec := changesNewContext(e, http.MethodGet, "/api/changes?limit=2", "", env.userJWT)
	changesInjectJWT(t, c, env.userJWT)
	_ = h.List(c)

	var resp struct {
		Changes []models.Change `json:"changes"`
		Limit   int            `json:"limit"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Limit != 2 {
		t.Errorf("expected limit 2, got %d", resp.Limit)
	}

	// Cleanup
	_, _ = env.db.Exec("DELETE FROM changes WHERE system LIKE 'system-%'")
}

func TestChangeListIntegration_ViewerCanRead(t *testing.T) {
	env := setupChangesTestEnv(t)
	e := echo.New()
	h := &handlers.ChangeHandler{DB: env.db}

	c, rec := changesNewContext(e, http.MethodGet, "/api/changes", "", env.viewerJWT)
	changesInjectJWT(t, c, env.viewerJWT)

	if err := h.List(c); err != nil {
		t.Fatalf("List error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestChangeListIntegration_APIKeyWithReadScope(t *testing.T) {
	env := setupChangesTestEnv(t)
	e := echo.New()
	h := &handlers.ChangeHandler{DB: env.db}

	req := httptest.NewRequest(http.MethodGet, "/api/changes", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+env.readOnlyKey)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	changesInjectAPIKey(t, c, []string{"changes:read"})

	if err := h.List(c); err != nil {
		t.Fatalf("List error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
