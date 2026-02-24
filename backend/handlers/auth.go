package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	mw "ops-ledger/backend/middleware"
	"ops-ledger/backend/models"
)

type AuthHandler struct {
	DB        *sql.DB
	JWTSecret string
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string       `json:"token"`
	User  *models.User `json:"user"`
}

func (h *AuthHandler) Register(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}
	if req.Email == "" || req.Password == "" || req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Email, password, and name are required"})
	}
	if len(req.Password) < 8 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Password must be at least 8 characters"})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to hash password"})
	}

	// Only allow registration when no users exist (first-time setup)
	count, err := models.CountUsers(h.DB)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Database error"})
	}
	if count > 0 {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Registration is disabled. Contact an administrator."})
	}
	role := "admin"

	user, err := models.CreateUser(h.DB, req.Email, req.Name, string(hash), role)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
			return c.JSON(http.StatusConflict, map[string]string{"error": "An account with this email already exists"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create user"})
	}

	_ = models.UpdateLastLogin(h.DB, user.ID)
	auditLog(h.DB, c, "user.register", "user", uint64Ptr(user.ID), strPtr(user.Email))

	token, err := h.generateToken(user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate token"})
	}

	return c.JSON(http.StatusCreated, authResponse{Token: token, User: user})
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}
	if req.Email == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Email and password are required"})
	}

	user, err := models.GetUserByEmail(h.DB, req.Email)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid email or password"})
	}
	if user.Status == "disabled" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Your account has been disabled. Contact an administrator."})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid email or password"})
	}

	_ = models.UpdateLastLogin(h.DB, user.ID)
	auditLog(h.DB, c, "user.login", "user", uint64Ptr(user.ID), strPtr(user.Email))

	token, err := h.generateToken(user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate token"})
	}

	return c.JSON(http.StatusOK, authResponse{Token: token, User: user})
}

func (h *AuthHandler) Me(c echo.Context) error {
	claims := c.Get("claims").(*mw.JWTClaims)

	sub, err := claims.GetSubject()
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token claims"})
	}
	id, err := strconv.ParseUint(sub, 10, 64)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token claims"})
	}

	user, err := models.GetUserByID(h.DB, id)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "User not found"})
	}

	return c.JSON(http.StatusOK, user)
}

func (h *AuthHandler) Logout(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"message": "Logged out"})
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (h *AuthHandler) ChangePassword(c echo.Context) error {
	claims := c.Get("claims").(*mw.JWTClaims)
	sub, err := claims.GetSubject()
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token claims"})
	}
	id, err := strconv.ParseUint(sub, 10, 64)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token claims"})
	}

	var req changePasswordRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Current and new passwords are required"})
	}
	if len(req.NewPassword) < 8 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "New password must be at least 8 characters"})
	}

	user, err := models.GetUserByID(h.DB, id)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "User not found"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Current password is incorrect"})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to hash password"})
	}

	if err := models.UpdateUserPassword(h.DB, id, string(hash)); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update password"})
	}

	auditLog(h.DB, c, "user.password_change", "user", uint64Ptr(id), strPtr(user.Email))

	return c.JSON(http.StatusOK, map[string]string{"message": "Password updated"})
}

func (h *AuthHandler) RegistrationStatus(c echo.Context) error {
	count, err := models.CountUsers(h.DB)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Database error"})
	}
	return c.JSON(http.StatusOK, map[string]bool{"allowed": count == 0})
}

func (h *AuthHandler) generateToken(user *models.User) (string, error) {
	claims := mw.JWTClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatUint(user.ID, 10),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Email: user.Email,
		Role:  user.Role,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(h.JWTSecret))
}
