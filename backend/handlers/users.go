package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/http"
	"strconv"

	"github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	mw "ops-ledger/backend/middleware"
	"ops-ledger/backend/models"
)

type UserHandler struct {
	DB *sql.DB
}

type createUserRequest struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

type updateRoleRequest struct {
	Role string `json:"role"`
}

type updateStatusRequest struct {
	Status string `json:"status"`
}

func (h *UserHandler) requireAdmin(c echo.Context) (*mw.JWTClaims, uint64, error) {
	claims := c.Get("claims").(*mw.JWTClaims)
	if claims.Role != "admin" {
		return nil, 0, c.JSON(http.StatusForbidden, map[string]string{"error": "Admin access required"})
	}
	sub, err := claims.GetSubject()
	if err != nil {
		return nil, 0, c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token claims"})
	}
	id, err := strconv.ParseUint(sub, 10, 64)
	if err != nil {
		return nil, 0, c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid token claims"})
	}
	return claims, id, nil
}

func generateTempPassword() (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (h *UserHandler) List(c echo.Context) error {
	_, _, err := h.requireAdmin(c)
	if err != nil {
		return nil
	}

	users, err := models.ListUsers(h.DB)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list users"})
	}
	if users == nil {
		users = []models.User{}
	}

	return c.JSON(http.StatusOK, users)
}

func (h *UserHandler) Create(c echo.Context) error {
	_, _, err := h.requireAdmin(c)
	if err != nil {
		return nil
	}

	var req createUserRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}
	if req.Email == "" || req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Email and name are required"})
	}
	if req.Role != "admin" && req.Role != "editor" && req.Role != "viewer" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Role must be admin, editor, or viewer"})
	}

	tempPassword, err := generateTempPassword()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate password"})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(tempPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to hash password"})
	}

	user, err := models.CreateUser(h.DB, req.Email, req.Name, string(hash), req.Role)
	if err != nil {
		if mysqlErr, ok := err.(*mysql.MySQLError); ok && mysqlErr.Number == 1062 {
			return c.JSON(http.StatusConflict, map[string]string{"error": "A user with this email already exists"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create user"})
	}

	auditLog(h.DB, c, "user.create", "user", uint64Ptr(user.ID), nil, strPtr(req.Email))
	return c.JSON(http.StatusCreated, map[string]interface{}{
		"user":              user,
		"temporaryPassword": tempPassword,
	})
}

func (h *UserHandler) UpdateRole(c echo.Context) error {
	_, callerID, err := h.requireAdmin(c)
	if err != nil {
		return nil
	}

	targetID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}
	if targetID == callerID {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Cannot change your own role"})
	}

	var req updateRoleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}
	if req.Role != "admin" && req.Role != "editor" && req.Role != "viewer" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Role must be admin, editor, or viewer"})
	}

	if err := models.UpdateUserRole(h.DB, targetID, req.Role); err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update role"})
	}

	auditLog(h.DB, c, "user.role_change", "user", uint64Ptr(targetID), nil, strPtr("role changed to "+req.Role))

	user, err := models.GetUserByID(h.DB, targetID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch updated user"})
	}

	return c.JSON(http.StatusOK, user)
}

func (h *UserHandler) UpdateStatus(c echo.Context) error {
	_, callerID, err := h.requireAdmin(c)
	if err != nil {
		return nil
	}

	targetID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}
	if targetID == callerID {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Cannot change your own status"})
	}

	var req updateStatusRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}
	if req.Status != "active" && req.Status != "disabled" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Status must be active or disabled"})
	}

	if err := models.UpdateUserStatus(h.DB, targetID, req.Status); err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update status"})
	}

	auditLog(h.DB, c, "user.status_change", "user", uint64Ptr(targetID), nil, strPtr("status changed to "+req.Status))

	user, err := models.GetUserByID(h.DB, targetID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch updated user"})
	}

	return c.JSON(http.StatusOK, user)
}

func (h *UserHandler) ResetPassword(c echo.Context) error {
	_, callerID, err := h.requireAdmin(c)
	if err != nil {
		return nil
	}

	targetID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid user ID"})
	}
	if targetID == callerID {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Cannot reset your own password this way"})
	}

	tempPassword, err := generateTempPassword()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate password"})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(tempPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to hash password"})
	}

	if err := models.UpdateUserPassword(h.DB, targetID, string(hash)); err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "User not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to reset password"})
	}

	auditLog(h.DB, c, "user.password_reset", "user", uint64Ptr(targetID), nil, nil)
	return c.JSON(http.StatusOK, map[string]string{"temporaryPassword": tempPassword})
}
