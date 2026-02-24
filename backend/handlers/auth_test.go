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
	"golang.org/x/crypto/bcrypt"

	mw "ops-ledger/backend/middleware"
	"ops-ledger/backend/models"
)

func setupAuthContext(method, path, body, role string, userID uint64, email string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	claims := &mw.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(userID, 10),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Email: email,
		Role:  role,
	}
	c.Set("claims", claims)
	return c, rec
}

// ---------------------------------------------------------------------------
// Register Tests
// ---------------------------------------------------------------------------

func TestRegister_BlockedWhenUsersExist(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuthHandler{DB: db, JWTSecret: "test-secret"}

	// CountUsers returns 1 (already have users — registration should be blocked)
	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	body := `{"email":"test@example.com","password":"password123","name":"Test User"}`
	c, rec := setupAuthContext(http.MethodPost, "/api/auth/register", body, "", 0, "")

	_ = h.Register(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Registration is disabled") {
		t.Error("expected registration disabled message")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestRegister_FirstUserBecomesAdmin(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuthHandler{DB: db, JWTSecret: "test-secret"}

	// CountUsers returns 0 (first user)
	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// Insert user with admin role
	hashed, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	mock.ExpectExec("INSERT INTO users").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), "admin").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// GetUserByID
	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM users WHERE id").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "name", "password_hash", "role", "status", "last_login", "created_at", "updated_at",
		}).AddRow(1, "admin@example.com", "Admin", string(hashed), "admin", "active", nil, now, now))

	mock.ExpectExec("UPDATE users SET last_login").WillReturnResult(sqlmock.NewResult(0, 0))

	body := `{"email":"admin@example.com","password":"password123","name":"Admin"}`
	c, rec := setupAuthContext(http.MethodPost, "/api/auth/register", body, "", 0, "")

	_ = h.Register(c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp authResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.User == nil || resp.User.Role != "admin" {
		t.Errorf("expected first user to be admin, got role: %v", resp.User)
	}
}

func TestRegister_MissingFields(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuthHandler{DB: db, JWTSecret: "test-secret"}

	tests := []struct {
		name string
		body string
	}{
		{"missing email", `{"password":"password123","name":"Test"}`},
		{"missing password", `{"email":"test@example.com","name":"Test"}`},
		{"missing name", `{"email":"test@example.com","password":"password123"}`},
		{"empty body", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := setupAuthContext(http.MethodPost, "/api/auth/register", tt.body, "", 0, "")
			_ = h.Register(c)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestRegister_PasswordTooShort(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuthHandler{DB: db, JWTSecret: "test-secret"}

	body := `{"email":"test@example.com","password":"short","name":"Test"}`
	c, rec := setupAuthContext(http.MethodPost, "/api/auth/register", body, "", 0, "")

	_ = h.Register(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "at least 8 characters") {
		t.Error("expected password length error message")
	}
}

// Note: Duplicate email during self-registration can no longer happen because
// registration is only allowed when count==0 (first user). Duplicate handling
// is still tested via admin user creation in users_test.go.

func TestRegister_InvalidJSON(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuthHandler{DB: db, JWTSecret: "test-secret"}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", strings.NewReader("not valid json"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.Register(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Login Tests
// ---------------------------------------------------------------------------

func TestLogin_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuthHandler{DB: db, JWTSecret: "test-secret"}

	hashed, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	now := time.Now()

	// GetUserByEmail
	mock.ExpectQuery("SELECT .+ FROM users WHERE email").
		WithArgs("test@example.com").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "name", "password_hash", "role", "status", "last_login", "created_at", "updated_at",
		}).AddRow(1, "test@example.com", "Test User", string(hashed), "viewer", "active", nil, now, now))

	// UpdateLastLogin
	mock.ExpectExec("UPDATE users SET last_login").WillReturnResult(sqlmock.NewResult(0, 0))

	body := `{"email":"test@example.com","password":"password123"}`
	c, rec := setupAuthContext(http.MethodPost, "/api/auth/login", body, "", 0, "")

	err = h.Login(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp authResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Token == "" {
		t.Error("expected token in response")
	}
}

func TestLogin_MissingFields(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuthHandler{DB: db, JWTSecret: "test-secret"}

	tests := []struct {
		name string
		body string
	}{
		{"missing email", `{"password":"password123"}`},
		{"missing password", `{"email":"test@example.com"}`},
		{"empty body", `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, rec := setupAuthContext(http.MethodPost, "/api/auth/login", tt.body, "", 0, "")
			_ = h.Login(c)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuthHandler{DB: db, JWTSecret: "test-secret"}

	// GetUserByEmail returns no rows
	mock.ExpectQuery("SELECT .+ FROM users WHERE email").
		WithArgs("notfound@example.com").
		WillReturnError(sql.ErrNoRows)

	body := `{"email":"notfound@example.com","password":"password123"}`
	c, rec := setupAuthContext(http.MethodPost, "/api/auth/login", body, "", 0, "")

	_ = h.Login(c)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuthHandler{DB: db, JWTSecret: "test-secret"}

	hashed, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.DefaultCost)
	now := time.Now()

	// GetUserByEmail
	mock.ExpectQuery("SELECT .+ FROM users WHERE email").
		WithArgs("test@example.com").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "name", "password_hash", "role", "status", "last_login", "created_at", "updated_at",
		}).AddRow(1, "test@example.com", "Test User", string(hashed), "viewer", "active", nil, now, now))

	body := `{"email":"test@example.com","password":"wrongpassword"}`
	c, rec := setupAuthContext(http.MethodPost, "/api/auth/login", body, "", 0, "")

	_ = h.Login(c)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogin_DisabledUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuthHandler{DB: db, JWTSecret: "test-secret"}

	hashed, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	now := time.Now()

	// GetUserByEmail - user is disabled
	mock.ExpectQuery("SELECT .+ FROM users WHERE email").
		WithArgs("disabled@example.com").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "name", "password_hash", "role", "status", "last_login", "created_at", "updated_at",
		}).AddRow(1, "disabled@example.com", "Disabled User", string(hashed), "viewer", "disabled", nil, now, now))

	body := `{"email":"disabled@example.com","password":"password123"}`
	c, rec := setupAuthContext(http.MethodPost, "/api/auth/login", body, "", 0, "")

	_ = h.Login(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "disabled") {
		t.Error("expected disabled account message")
	}
}

// ---------------------------------------------------------------------------
// Me Tests
// ---------------------------------------------------------------------------

func TestMe_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuthHandler{DB: db, JWTSecret: "test-secret"}

	now := time.Now()
	mock.ExpectQuery("SELECT .+ FROM users WHERE id").
		WithArgs(uint64(1)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "name", "password_hash", "role", "status", "last_login", "created_at", "updated_at",
		}).AddRow(1, "test@example.com", "Test User", "hash", "viewer", "active", nil, now, now))

	c, rec := setupAuthContext(http.MethodGet, "/api/auth/me", "", "viewer", 1, "test@example.com")

	err = h.Me(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var user models.User
	if err := json.Unmarshal(rec.Body.Bytes(), &user); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if user.ID != 1 || user.Email != "test@example.com" {
		t.Errorf("expected user with id=1, email=test@example.com, got %+v", user)
	}
}

func TestMe_UserNotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuthHandler{DB: db, JWTSecret: "test-secret"}

	mock.ExpectQuery("SELECT .+ FROM users WHERE id").
		WithArgs(uint64(999)).
		WillReturnError(sql.ErrNoRows)

	c, rec := setupAuthContext(http.MethodGet, "/api/auth/me", "", "viewer", 999, "test@example.com")

	_ = h.Me(c)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Logout Tests
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// RegistrationStatus Tests
// ---------------------------------------------------------------------------

func TestRegistrationStatus_AllowedWhenNoUsers(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuthHandler{DB: db, JWTSecret: "test-secret"}

	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/registration-status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.RegistrationStatus(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]bool
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if !resp["allowed"] {
		t.Error("expected allowed=true when no users exist")
	}
}

func TestRegistrationStatus_DisallowedWhenUsersExist(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuthHandler{DB: db, JWTSecret: "test-secret"}

	mock.ExpectQuery("SELECT COUNT").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/auth/registration-status", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	_ = h.RegistrationStatus(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]bool
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp["allowed"] {
		t.Error("expected allowed=false when users exist")
	}
}

// ---------------------------------------------------------------------------
// Logout Tests
// ---------------------------------------------------------------------------

func TestLogout_Success(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &AuthHandler{DB: db, JWTSecret: "test-secret"}

	c, rec := setupAuthContext(http.MethodPost, "/api/auth/logout", "", "viewer", 1, "test@example.com")

	err = h.Logout(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}
