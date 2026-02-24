//go:build integration

package handlers_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

const testJWTSecret = "test-secret"

// testEnv holds shared state for integration tests.
type testEnv struct {
	db      *sql.DB
	handler *handlers.ApiKeyHandler
	userID  uint64
	jwt     string
}

// setupTestEnv connects to MariaDB using environment variables (with docker compose
// defaults), runs migrations, creates a test admin user, and returns a JWT token
// signed with testJWTSecret. It registers a cleanup function that removes all
// api_keys and users rows created during the test run.
func setupTestEnv(t *testing.T) *testEnv {
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

	// Create a test admin user.
	hash, err := bcrypt.GenerateFromPassword([]byte("test-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	email := fmt.Sprintf("admin-test-%d@example.com", time.Now().UnixNano())
	res, err := db.Exec(
		"INSERT INTO users (email, name, password_hash, role, status) VALUES (?, ?, ?, 'admin', 'active')",
		email, "Test Admin", string(hash),
	)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	rawID, _ := res.LastInsertId()
	userID := uint64(rawID)

	// Build a signed JWT for this user.
	claims := &mw.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(userID, 10),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
		Email: email,
		Role:  "admin",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(testJWTSecret))
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}

	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM api_keys WHERE created_by = ?", userID)
		_, _ = db.Exec("DELETE FROM users WHERE id = ?", userID)
		db.Close()
	})

	return &testEnv{
		db:      db,
		handler: &handlers.ApiKeyHandler{DB: db},
		userID:  userID,
		jwt:     signed,
	}
}

// newEchoContext creates an Echo context backed by a test recorder. If jwtToken
// is non-empty the Authorization header is set automatically.
func newEchoContext(e *echo.Echo, method, path, body, jwtToken string) (echo.Context, *httptest.ResponseRecorder) {
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

// injectJWTClaims parses the token and injects claims into the Echo context,
// mimicking what the JWTAuth middleware does. This keeps handler tests
// independent of the middleware under test.
func injectJWTClaims(t *testing.T, c echo.Context, token string) {
	t.Helper()
	claims := &mw.JWTClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (interface{}, error) {
		return []byte(testJWTSecret), nil
	})
	if err != nil {
		t.Fatalf("failed to parse JWT for context injection: %v", err)
	}
	c.Set("claims", claims)
}

func getEnvDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// ---------------------------------------------------------------------------
// Test cases
// ---------------------------------------------------------------------------

// TestCreateRevokeRoundTrip verifies the full lifecycle:
//
//  1. Create an API key via the handler.
//  2. Confirm the key_hash is persisted in the DB with status "active".
//  3. Revoke the key via the handler.
//  4. Confirm the DB row now has status "revoked".
func TestCreateRevokeRoundTrip(t *testing.T) {
	env := setupTestEnv(t)
	e := echo.New()

	// --- Step 1: Create ---
	body := `{"name":"round-trip-key","scopes":["changes:read"]}`
	c, rec := newEchoContext(e, http.MethodPost, "/api/keys", body, env.jwt)
	injectJWTClaims(t, c, env.jwt)

	if err := env.handler.Create(c); err != nil {
		t.Fatalf("Create returned unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("Create: expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var createResp struct {
		Key    string         `json:"key"`
		ApiKey *models.ApiKey `json:"apiKey"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("failed to decode create response: %v", err)
	}
	if createResp.Key == "" {
		t.Fatal("expected raw key in response, got empty string")
	}
	if createResp.ApiKey == nil {
		t.Fatal("expected apiKey in response, got nil")
	}
	keyID := createResp.ApiKey.ID

	// --- Step 2: Verify hash in DB ---
	expectedHash := models.HashAPIKey(createResp.Key)
	var dbHash, dbStatus string
	row := env.db.QueryRow("SELECT key_hash, status FROM api_keys WHERE id = ?", keyID)
	if err := row.Scan(&dbHash, &dbStatus); err != nil {
		t.Fatalf("failed to query api_keys: %v", err)
	}
	if dbHash != expectedHash {
		t.Errorf("key_hash mismatch: got %q, want %q", dbHash, expectedHash)
	}
	if dbStatus != "active" {
		t.Errorf("status after create: got %q, want %q", dbStatus, "active")
	}

	// --- Step 3: Revoke ---
	c2, rec2 := newEchoContext(e, http.MethodDelete, "/api/keys/"+strconv.FormatUint(keyID, 10), "", env.jwt)
	c2.SetParamNames("id")
	c2.SetParamValues(strconv.FormatUint(keyID, 10))
	injectJWTClaims(t, c2, env.jwt)

	if err := env.handler.Revoke(c2); err != nil {
		t.Fatalf("Revoke returned unexpected error: %v", err)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("Revoke: expected 200, got %d; body: %s", rec2.Code, rec2.Body.String())
	}

	// --- Step 4: Verify revoked status in DB ---
	if err := env.db.QueryRow("SELECT status FROM api_keys WHERE id = ?", keyID).Scan(&dbStatus); err != nil {
		t.Fatalf("failed to re-query api_keys: %v", err)
	}
	if dbStatus != "revoked" {
		t.Errorf("status after revoke: got %q, want %q", dbStatus, "revoked")
	}
}

// TestAPIKeyMiddlewareAuthentication verifies that a raw API key obtained from
// the create handler is accepted by the APIKeyOrJWT middleware and that the
// expected context values are set.
func TestAPIKeyMiddlewareAuthentication(t *testing.T) {
	env := setupTestEnv(t)
	e := echo.New()

	// Create a key.
	body := `{"name":"middleware-test-key","scopes":["changes:read","changes:write"]}`
	c, rec := newEchoContext(e, http.MethodPost, "/api/keys", body, env.jwt)
	injectJWTClaims(t, c, env.jwt)

	if err := env.handler.Create(c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("Create: expected 201, got %d", rec.Code)
	}

	var createResp struct {
		Key    string         `json:"key"`
		ApiKey *models.ApiKey `json:"apiKey"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	rawKey := createResp.Key
	keyID := createResp.ApiKey.ID

	// Use the raw key with the APIKeyOrJWT middleware.
	middlewareFn := mw.APIKeyOrJWT(env.db, testJWTSecret)

	var capturedCtx echo.Context
	sentinel := middlewareFn(func(c echo.Context) error {
		capturedCtx = c
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+rawKey)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req, rec2)

	if err := sentinel(c2); err != nil {
		t.Fatalf("middleware returned error: %v", err)
	}
	if rec2.Code != http.StatusOK {
		t.Fatalf("middleware: expected 200, got %d; body: %s", rec2.Code, rec2.Body.String())
	}

	// Verify context values.
	if capturedCtx == nil {
		t.Fatal("handler was not called; middleware blocked the request unexpectedly")
	}

	gotKeyID, ok := capturedCtx.Get("apiKeyID").(uint64)
	if !ok {
		t.Fatalf("apiKeyID not set or wrong type in context: %v", capturedCtx.Get("apiKeyID"))
	}
	if gotKeyID != keyID {
		t.Errorf("apiKeyID: got %d, want %d", gotKeyID, keyID)
	}

	gotCreatedBy, ok := capturedCtx.Get("apiKeyCreatedBy").(uint64)
	if !ok {
		t.Fatalf("apiKeyCreatedBy not set or wrong type in context: %v", capturedCtx.Get("apiKeyCreatedBy"))
	}
	if gotCreatedBy != env.userID {
		t.Errorf("apiKeyCreatedBy: got %d, want %d", gotCreatedBy, env.userID)
	}

	gotScopes, ok := capturedCtx.Get("apiKeyScopes").([]string)
	if !ok {
		t.Fatalf("apiKeyScopes not set or wrong type in context: %v", capturedCtx.Get("apiKeyScopes"))
	}
	expectedScopes := map[string]bool{"changes:read": true, "changes:write": true}
	for _, s := range gotScopes {
		if !expectedScopes[s] {
			t.Errorf("unexpected scope in context: %q", s)
		}
		delete(expectedScopes, s)
	}
	for s := range expectedScopes {
		t.Errorf("missing scope in context: %q", s)
	}
}

// TestExpiredKeyRejection verifies that a key whose expires_at is in the past
// is rejected with HTTP 401.
func TestExpiredKeyRejection(t *testing.T) {
	env := setupTestEnv(t)
	e := echo.New()

	// Create a key with an expiry in the past.
	pastTime := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	body := fmt.Sprintf(`{"name":"expired-key","scopes":["changes:read"],"expiresAt":%q}`, pastTime)
	c, rec := newEchoContext(e, http.MethodPost, "/api/keys", body, env.jwt)
	injectJWTClaims(t, c, env.jwt)

	if err := env.handler.Create(c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("Create: expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var createResp struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &createResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	// Attempt authentication with the expired key.
	middlewareFn := mw.APIKeyOrJWT(env.db, testJWTSecret)
	handler := middlewareFn(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set(echo.HeaderAuthorization, "Bearer "+createResp.Key)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req, rec2)

	// The handler may return the error or write the response directly.
	_ = handler(c2)

	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for expired key, got %d; body: %s", rec2.Code, rec2.Body.String())
	}

	var errResp map[string]string
	if err := json.Unmarshal(rec2.Body.Bytes(), &errResp); err == nil {
		if errResp["error"] == "" {
			t.Error("expected non-empty error field in response body")
		}
	}
}

// TestRotateKey verifies the rotate lifecycle:
//
//  1. Create an original key.
//  2. Rotate it via the handler.
//  3. Confirm the original key is revoked in the DB.
//  4. Confirm the new raw key authenticates successfully via the middleware.
func TestRotateKey(t *testing.T) {
	env := setupTestEnv(t)
	e := echo.New()

	// --- Step 1: Create original key ---
	body := `{"name":"rotate-me","scopes":["changes:read"]}`
	c, rec := newEchoContext(e, http.MethodPost, "/api/keys", body, env.jwt)
	injectJWTClaims(t, c, env.jwt)

	if err := env.handler.Create(c); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("Create: expected 201, got %d; body: %s", rec.Code, rec.Body.String())
	}

	var origResp struct {
		Key    string         `json:"key"`
		ApiKey *models.ApiKey `json:"apiKey"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &origResp); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	origID := origResp.ApiKey.ID
	origRawKey := origResp.Key

	// --- Step 2: Rotate ---
	c2, rec2 := newEchoContext(e, http.MethodPost, "/api/keys/"+strconv.FormatUint(origID, 10)+"/rotate", "", env.jwt)
	c2.SetParamNames("id")
	c2.SetParamValues(strconv.FormatUint(origID, 10))
	injectJWTClaims(t, c2, env.jwt)

	if err := env.handler.Rotate(c2); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if rec2.Code != http.StatusCreated {
		t.Fatalf("Rotate: expected 201, got %d; body: %s", rec2.Code, rec2.Body.String())
	}

	var rotateResp struct {
		Key    string         `json:"key"`
		ApiKey *models.ApiKey `json:"apiKey"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &rotateResp); err != nil {
		t.Fatalf("decode rotate response: %v", err)
	}
	newRawKey := rotateResp.Key
	newKeyID := rotateResp.ApiKey.ID

	if newRawKey == origRawKey {
		t.Error("rotated key must differ from the original key")
	}
	if newKeyID == origID {
		t.Error("rotated key must have a new DB row (different ID)")
	}

	// --- Step 3: Original key is revoked in DB ---
	var origStatus string
	if err := env.db.QueryRow("SELECT status FROM api_keys WHERE id = ?", origID).Scan(&origStatus); err != nil {
		t.Fatalf("query original key: %v", err)
	}
	if origStatus != "revoked" {
		t.Errorf("original key status: got %q, want %q", origStatus, "revoked")
	}

	// --- Step 4: Original key is rejected by middleware ---
	middlewareFn := mw.APIKeyOrJWT(env.db, testJWTSecret)

	okHandler := middlewareFn(func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"ok": "true"})
	})

	reqOld := httptest.NewRequest(http.MethodGet, "/protected", nil)
	reqOld.Header.Set(echo.HeaderAuthorization, "Bearer "+origRawKey)
	recOld := httptest.NewRecorder()
	_ = okHandler(e.NewContext(reqOld, recOld))

	if recOld.Code != http.StatusUnauthorized {
		t.Errorf("old key: expected 401 after rotation, got %d", recOld.Code)
	}

	// --- Step 5: New key authenticates successfully ---
	reqNew := httptest.NewRequest(http.MethodGet, "/protected", nil)
	reqNew.Header.Set(echo.HeaderAuthorization, "Bearer "+newRawKey)
	recNew := httptest.NewRecorder()
	_ = okHandler(e.NewContext(reqNew, recNew))

	if recNew.Code != http.StatusOK {
		t.Errorf("new key: expected 200, got %d; body: %s", recNew.Code, recNew.Body.String())
	}
}
