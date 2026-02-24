package handlers

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	mw "ops-ledger/backend/middleware"
	"ops-ledger/backend/models"
)

type ApiKeyHandler struct {
	DB *sql.DB
}

type createApiKeyRequest struct {
	Name      string   `json:"name"`
	Scopes    []string `json:"scopes"`
	ExpiresAt *string  `json:"expiresAt"`
}

type createApiKeyResponse struct {
	Key    string         `json:"key"`
	ApiKey *models.ApiKey `json:"apiKey"`
}

func (h *ApiKeyHandler) requireAdmin(c echo.Context) (*mw.JWTClaims, uint64, error) {
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

func (h *ApiKeyHandler) Create(c echo.Context) error {
	_, userID, err := h.requireAdmin(c)
	if err != nil {
		return nil // response already sent by requireAdmin
	}

	var req createApiKeyRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Name is required"})
	}
	if err := models.ValidateScopes(req.Scopes); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid expiresAt format, use RFC3339"})
		}
		expiresAt = &t
	}

	rawKey, keyHash, prefix, err := models.GenerateAPIKey()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate API key"})
	}

	apiKey, err := models.CreateApiKey(h.DB, req.Name, keyHash, prefix, req.Scopes, userID, expiresAt)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create API key"})
	}

	auditLog(h.DB, c, "apikey.create", "api_key", uint64Ptr(apiKey.ID), strPtr(req.Name))
	return c.JSON(http.StatusCreated, createApiKeyResponse{Key: rawKey, ApiKey: apiKey})
}

func (h *ApiKeyHandler) List(c echo.Context) error {
	_, userID, err := h.requireAdmin(c)
	if err != nil {
		return nil
	}

	keys, err := models.ListApiKeysByCreator(h.DB, userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list API keys"})
	}
	if keys == nil {
		keys = []models.ApiKey{}
	}

	return c.JSON(http.StatusOK, keys)
}

func (h *ApiKeyHandler) Revoke(c echo.Context) error {
	_, _, err := h.requireAdmin(c)
	if err != nil {
		return nil
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid key ID"})
	}

	if err := models.RevokeApiKey(h.DB, id); err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "API key not found or already revoked"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to revoke API key"})
	}

	auditLog(h.DB, c, "apikey.revoke", "api_key", uint64Ptr(id), nil)
	return c.JSON(http.StatusOK, map[string]string{"message": "API key revoked"})
}

func (h *ApiKeyHandler) Rotate(c echo.Context) error {
	_, userID, err := h.requireAdmin(c)
	if err != nil {
		return nil
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid key ID"})
	}

	// Get the existing key to copy name and scopes
	existing, err := models.GetApiKeyByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "API key not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to find API key"})
	}
	if existing.CreatedBy != userID {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Cannot rotate another user's API key"})
	}

	// Revoke the old key
	_ = models.RevokeApiKey(h.DB, id)

	// Create a new key with the same name and scopes
	rawKey, keyHash, prefix, err := models.GenerateAPIKey()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to generate API key"})
	}

	apiKey, err := models.CreateApiKey(h.DB, existing.Name, keyHash, prefix, existing.Scopes, userID, existing.ExpiresAt)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create new API key"})
	}

	auditLog(h.DB, c, "apikey.rotate", "api_key", uint64Ptr(apiKey.ID), strPtr(existing.Name))
	return c.JSON(http.StatusCreated, createApiKeyResponse{Key: rawKey, ApiKey: apiKey})
}
