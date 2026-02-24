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

const authTestJWTSecret = "auth-test-secret"

// authTestEnv holds shared state for auth integration tests.
type authTestEnv struct {
	db       *sql.DB
	userID   uint64
	userJWT  string
	adminID  uint64
	adminJWT string
}

// setupAuthTestEnv creates a test environment with a regular user and an admin user.
func setupAuthTestEnv(t *testing.T) *authTestEnv {
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

	// Create regular test user
	userHash, _ := bcrypt.GenerateFromPassword([]byte("user-password"), bcrypt.MinCost)
	userEmail := fmt.Sprintf("user-test-%d@example.com", time.Now().UnixNano())
	res, _ := db.Exec(
		"INSERT INTO users (email, name, password_hash, role, status) VALUES (?, ?, ?, 'viewer', 'active')",
		userEmail, "Test User", string(userHash),
	)
	userID, _ := res.LastInsertId()

	// Create admin test user
	adminHash, _ := bcrypt.GenerateFromPassword([]byte("admin-password"), bcrypt.MinCost)
	adminEmail := fmt.Sprintf("admin-test-%d@example.com", time.Now().UnixNano())
	resAdmin, _ := db.Exec(
		"INSERT INTO users (email, name, password_hash, role, status) VALUES (?, ?, ?, 'admin', 'active')",
		adminEmail, "Test Admin", string(adminHash),
	)
	adminID, _ := resAdmin.LastInsertId()

	// Generate JWTs
	userJWT := generateTestJWT(t, uint64(userID), userEmail, "viewer")
	adminJWT := generateTestJWT(t, uint64(adminID), adminEmail, "admin")

	t.Cleanup(func() {
		_, _ = db.Exec("DELETE FROM api_keys WHERE created_by IN (?, ?)", userID, adminID)
		_, _ = db.Exec("DELETE FROM users WHERE id IN (?, ?)", userID, adminID)
		db.Close()
	})

	return &authTestEnv{
		db:       db,
		userID:   uint64(userID),
		userJWT:  userJWT,
		adminID:  uint64(adminID),
		adminJWT: adminJWT,
	}
}

func generateTestJWT(t *testing.T, userID uint64, email, role string) string {
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
	signed, err := token.SignedString([]byte(authTestJWTSecret))
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}
	return signed
}

func authNewContext(e *echo.Echo, method, path, body, jwtToken string) (echo.Context, *httptest.ResponseRecorder) {
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

func authInjectClaims(t *testing.T, c echo.Context, token string) {
	t.Helper()
	claims := &mw.JWTClaims{}
	_, _ = jwt.ParseWithClaims(token, claims, func(*jwt.Token) (interface{}, error) {
		return []byte(authTestJWTSecret), nil
	})
	c.Set("claims", claims)
}

// ---------------------------------------------------------------------------
// Register Integration Tests
// ---------------------------------------------------------------------------

func TestRegisterIntegration(t *testing.T) {
	env := setupAuthTestEnv(t)
	e := echo.New()
	h := &handlers.AuthHandler{DB: env.db, JWTSecret: authTestJWTSecret}

	// Success case
	body := `{"email":"newuser@example.com","password":"newpassword123","name":"New User"}`
	c, rec := authNewContext(e, http.MethodPost, "/api/auth/register", body, "")

	if err := h.Register(c); err != nil {
		t.Fatalf("Register error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Token string       `json:"token"`
		User  *models.User `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token == "" || resp.User == nil {
		t.Fatal("expected token and user in response")
	}
	if resp.User.Role != "viewer" {
		t.Errorf("expected viewer role, got %s", resp.User.Role)
	}

	// Cleanup
	_, _ = env.db.Exec("DELETE FROM users WHERE id = ?", resp.User.ID)
}

func TestRegisterIntegration_DuplicateEmail(t *testing.T) {
	env := setupAuthTestEnv(t)
	e := echo.New()
	h := &handlers.AuthHandler{DB: env.db, JWTSecret: authTestJWTSecret}

	// Try to register with existing user's email
	body := `{"email":"user-test-%s@example.com","password":"password123","name":"Duplicate"}`
	// We need the actual email from setup, so use the user's email
	body = fmt.Sprintf(body, strconv.FormatInt(int64(env.userID), 10))
	c, rec := authNewContext(e, http.MethodPost, "/api/auth/register", body, "")

	if err := h.Register(c); err != nil {
		t.Fatalf("Register error: %v", err)
	}
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRegisterIntegration_ValidationErrors(t *testing.T) {
	env := setupAuthTestEnv(t)
	e := echo.New()
	h := &handlers.AuthHandler{DB: env.db, JWTSecret: authTestJWTSecret}

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"missing email", `{"password":"password123","name":"Test"}`, http.StatusBadRequest},
		{"missing password", `{"email":"test@example.com","name":"Test"}`, http.StatusBadRequest},
		{"missing name", `{"email":"test@example.com","password":"password123"}`, http.StatusBadRequest},
		{"password too short", `{"email":"test@example.com","password":"short","name":"Test"}`, http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := authNewContext(e, http.MethodPost, "/api/auth/register", tt.body, "")
			_ = h.Register(c)
			if rec.Code != tt.wantStatus {
				t.Errorf("expected %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Login Integration Tests
// ---------------------------------------------------------------------------

func TestLoginIntegration_Success(t *testing.T) {
	env := setupAuthTestEnv(t)
	e := echo.New()
	h := &handlers.AuthHandler{DB: env.db, JWTSecret: authTestJWTSecret}

	body := `{"email":"user-test-%s","password":"user-password"}`
	// Get the actual email from user
	var email string
	env.db.QueryRow("SELECT email FROM users WHERE id = ?", env.userID).Scan(&email)

	body = fmt.Sprintf(`{"email":"%s","password":"user-password"}`, email)
	c, rec := authNewContext(e, http.MethodPost, "/api/auth/login", body, "")

	if err := h.Login(c); err != nil {
		t.Fatalf("Login error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Token string       `json:"token"`
		User  *models.User `json:"user"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected token in response")
	}
}

func TestLoginIntegration_InvalidCredentials(t *testing.T) {
	env := setupAuthTestEnv(t)
	e := echo.New()
	h := &handlers.AuthHandler{DB: env.db, JWTSecret: authTestJWTSecret}

	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{"wrong password", `{"email":"user@example.com","password":"wrongpassword"}`, http.StatusUnauthorized},
		{"nonexistent user", `{"email":"nonexistent@example.com","password":"password123"}`, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := authNewContext(e, http.MethodPost, "/api/auth/login", tt.body, "")
			_ = h.Login(c)
			if rec.Code != tt.wantStatus {
				t.Errorf("expected %d, got %d: %s", tt.wantStatus, rec.Code, rec.Body.String())
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Me Integration Tests
// ---------------------------------------------------------------------------

func TestMeIntegration(t *testing.T) {
	env := setupAuthTestEnv(t)
	e := echo.New()
	h := &handlers.AuthHandler{DB: env.db, JWTSecret: authTestJWTSecret}

	c, rec := authNewContext(e, http.MethodGet, "/api/auth/me", "", env.userJWT)
	authInjectClaims(t, c, env.userJWT)

	if err := h.Me(c); err != nil {
		t.Fatalf("Me error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var user models.User
	if err := json.Unmarshal(rec.Body.Bytes(), &user); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if user.ID != env.userID {
		t.Errorf("expected user ID %d, got %d", env.userID, user.ID)
	}
	if user.Role != "viewer" {
		t.Errorf("expected viewer role, got %s", user.Role)
	}
}

// ---------------------------------------------------------------------------
// Logout Integration Tests
// ---------------------------------------------------------------------------

func TestLogoutIntegration(t *testing.T) {
	env := setupAuthTestEnv(t)
	e := echo.New()
	h := &handlers.AuthHandler{DB: env.db, JWTSecret: authTestJWTSecret}

	c, rec := authNewContext(e, http.MethodPost, "/api/auth/logout", "", env.userJWT)
	authInjectClaims(t, c, env.userJWT)

	if err := h.Logout(c); err != nil {
		t.Fatalf("Logout error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
