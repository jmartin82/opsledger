package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-sql-driver/mysql"
	"ops-ledger/backend/models"
)

// userColumns matches the SELECT columns in GetUserByID / ListUsers
var userColumns = []string{"id", "email", "name", "password_hash", "role", "status", "last_login", "created_at", "updated_at"}

func TestListUsers_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &UserHandler{DB: db}
	now := time.Now()

	mock.ExpectQuery("SELECT .+ FROM users ORDER BY created_at").
		WillReturnRows(sqlmock.NewRows(userColumns).
			AddRow(1, "alice@test.com", "Alice", "hash", "admin", "active", now, now, now).
			AddRow(2, "bob@test.com", "Bob", "hash", "viewer", "active", nil, now, now))

	c, rec := setupContext(http.MethodGet, "/api/admin/users", "", "admin", 1)

	_ = h.List(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var users []models.User
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestListUsers_NonAdmin(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &UserHandler{DB: db}
	c, rec := setupContext(http.MethodGet, "/api/admin/users", "", "viewer", 1)

	_ = h.List(c)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateUser_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &UserHandler{DB: db}
	now := time.Now()

	// Expect INSERT
	mock.ExpectExec("INSERT INTO users").
		WithArgs("newuser@test.com", "New User", sqlmock.AnyArg(), "editor").
		WillReturnResult(sqlmock.NewResult(5, 1))

	// Expect GetUserByID
	mock.ExpectQuery("SELECT .+ FROM users WHERE id").
		WithArgs(uint64(5)).
		WillReturnRows(sqlmock.NewRows(userColumns).
			AddRow(5, "newuser@test.com", "New User", "hash", "editor", "active", nil, now, now))

	body := `{"email":"newuser@test.com","name":"New User","role":"editor"}`
	c, rec := setupContext(http.MethodPost, "/api/admin/users", body, "admin", 1)

	_ = h.Create(c)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		User              *models.User `json:"user"`
		TemporaryPassword string       `json:"temporaryPassword"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.User == nil {
		t.Fatal("expected user in response")
	}
	if resp.TemporaryPassword == "" {
		t.Fatal("expected temporaryPassword in response")
	}
	if len(resp.TemporaryPassword) != 24 {
		t.Errorf("expected 24-char temp password, got %d chars", len(resp.TemporaryPassword))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestCreateUser_DuplicateEmail(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &UserHandler{DB: db}

	// go-sql-driver/mysql MySQLError with code 1062
	mock.ExpectExec("INSERT INTO users").
		WithArgs("dup@test.com", "Dup User", sqlmock.AnyArg(), "viewer").
		WillReturnError(&mysql.MySQLError{Number: 1062, Message: "Duplicate entry"})

	body := `{"email":"dup@test.com","name":"Dup User","role":"viewer"}`
	c, rec := setupContext(http.MethodPost, "/api/admin/users", body, "admin", 1)

	_ = h.Create(c)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateUser_MissingFields(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &UserHandler{DB: db}

	body := `{"email":"","name":"","role":"viewer"}`
	c, rec := setupContext(http.MethodPost, "/api/admin/users", body, "admin", 1)

	_ = h.Create(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateUser_InvalidRole(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &UserHandler{DB: db}

	body := `{"email":"test@test.com","name":"Test","role":"superadmin"}`
	c, rec := setupContext(http.MethodPost, "/api/admin/users", body, "admin", 1)

	_ = h.Create(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateRole_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &UserHandler{DB: db}
	now := time.Now()

	mock.ExpectExec("UPDATE users SET role").
		WithArgs("editor", uint64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery("SELECT .+ FROM users WHERE id").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows(userColumns).
			AddRow(2, "bob@test.com", "Bob", "hash", "editor", "active", nil, now, now))

	body := `{"role":"editor"}`
	c, rec := setupContext(http.MethodPut, "/api/admin/users/2/role", body, "admin", 1)
	c.SetParamNames("id")
	c.SetParamValues("2")

	_ = h.UpdateRole(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestUpdateRole_SelfProtection(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &UserHandler{DB: db}

	body := `{"role":"viewer"}`
	c, rec := setupContext(http.MethodPut, "/api/admin/users/1/role", body, "admin", 1)
	c.SetParamNames("id")
	c.SetParamValues("1")

	_ = h.UpdateRole(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateRole_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &UserHandler{DB: db}

	mock.ExpectExec("UPDATE users SET role").
		WithArgs("editor", uint64(99)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	body := `{"role":"editor"}`
	c, rec := setupContext(http.MethodPut, "/api/admin/users/99/role", body, "admin", 1)
	c.SetParamNames("id")
	c.SetParamValues("99")

	_ = h.UpdateRole(c)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestUpdateStatus_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &UserHandler{DB: db}
	now := time.Now()

	mock.ExpectExec("UPDATE users SET status").
		WithArgs("disabled", uint64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectQuery("SELECT .+ FROM users WHERE id").
		WithArgs(uint64(2)).
		WillReturnRows(sqlmock.NewRows(userColumns).
			AddRow(2, "bob@test.com", "Bob", "hash", "viewer", "disabled", nil, now, now))

	body := `{"status":"disabled"}`
	c, rec := setupContext(http.MethodPut, "/api/admin/users/2/status", body, "admin", 1)
	c.SetParamNames("id")
	c.SetParamValues("2")

	_ = h.UpdateStatus(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestUpdateStatus_SelfProtection(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &UserHandler{DB: db}

	body := `{"status":"disabled"}`
	c, rec := setupContext(http.MethodPut, "/api/admin/users/1/status", body, "admin", 1)
	c.SetParamNames("id")
	c.SetParamValues("1")

	_ = h.UpdateStatus(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestResetPassword_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &UserHandler{DB: db}

	mock.ExpectExec("UPDATE users SET password_hash").
		WithArgs(sqlmock.AnyArg(), uint64(2)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	c, rec := setupContext(http.MethodPost, "/api/admin/users/2/reset-password", "", "admin", 1)
	c.SetParamNames("id")
	c.SetParamValues("2")

	_ = h.ResetPassword(c)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["temporaryPassword"] == "" {
		t.Fatal("expected temporaryPassword in response")
	}
	if len(resp["temporaryPassword"]) != 24 {
		t.Errorf("expected 24-char temp password, got %d chars", len(resp["temporaryPassword"]))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unmet expectations: %v", err)
	}
}

func TestResetPassword_SelfProtection(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	h := &UserHandler{DB: db}

	c, rec := setupContext(http.MethodPost, "/api/admin/users/1/reset-password", "", "admin", 1)
	c.SetParamNames("id")
	c.SetParamValues("1")

	_ = h.ResetPassword(c)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

