package handlers

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	mw "ops-ledger/backend/middleware"
	"ops-ledger/backend/models"
)

type ChangeHandler struct {
	DB  *sql.DB
	Hub *EventHub
}

type createChangeRequest struct {
	System      string  `json:"system"`
	Environment *string `json:"environment"`
	User        *string `json:"user"`
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Timestamp   *string `json:"timestamp"`
}

type listChangesResponse struct {
	Changes []models.Change `json:"changes"`
	Total   int             `json:"total"`
	Limit   int             `json:"limit"`
	Offset  int             `json:"offset"`
}

func (h *ChangeHandler) Create(c echo.Context) error {
	if err := h.requireWriteAccess(c); err != nil {
		return err
	}

	var req createChangeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if req.System == "" || req.Type == "" || req.Description == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "system, type, and description are required"})
	}

	validTypes := map[string]bool{"infrastructure": true, "deployment": true, "configuration": true}
	if !validTypes[req.Type] {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "type must be infrastructure, deployment, or configuration"})
	}

	// Normalize empty strings to nil for nullable fields
	env := req.Environment
	if env != nil && *env == "" {
		env = nil
	}
	user := req.User
	if user != nil && *user == "" {
		user = nil
	}

	var ts *time.Time
	if req.Timestamp != nil && *req.Timestamp != "" {
		parsed, err := time.Parse(time.RFC3339, *req.Timestamp)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid timestamp format, use RFC3339 (e.g. 2025-01-15T10:30:00Z)"})
		}
		ts = &parsed
	}

	change, err := models.CreateChange(h.DB, req.System, env, user, req.Type, req.Description, ts)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to create change"})
	}

	auditLog(h.DB, c, "change.create", "change", uint64Ptr(change.ID), strPtr(req.System+": "+req.Description))
	if h.Hub != nil {
		h.Hub.Publish(SSEEvent{Type: "change.created", Data: change})
	}
	return c.JSON(http.StatusCreated, change)
}

func (h *ChangeHandler) Update(c echo.Context) error {
	if err := h.requireWriteAccess(c); err != nil {
		return err
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid change ID"})
	}

	var req createChangeRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid request body"})
	}

	if req.System == "" || req.Type == "" || req.Description == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "system, type, and description are required"})
	}

	validTypes := map[string]bool{"infrastructure": true, "deployment": true, "configuration": true}
	if !validTypes[req.Type] {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "type must be infrastructure, deployment, or configuration"})
	}

	env := req.Environment
	if env != nil && *env == "" {
		env = nil
	}
	user := req.User
	if user != nil && *user == "" {
		user = nil
	}

	var ts *time.Time
	if req.Timestamp != nil && *req.Timestamp != "" {
		parsed, err := time.Parse(time.RFC3339, *req.Timestamp)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid timestamp format, use RFC3339 (e.g. 2025-01-15T10:30:00Z)"})
		}
		ts = &parsed
	}

	change, err := models.UpdateChange(h.DB, id, req.System, env, user, req.Type, req.Description, ts)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Change not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to update change"})
	}

	auditLog(h.DB, c, "change.update", "change", uint64Ptr(change.ID), strPtr(req.System+": "+req.Description))
	if h.Hub != nil {
		h.Hub.Publish(SSEEvent{Type: "change.updated", Data: change})
	}
	return c.JSON(http.StatusOK, change)
}

func (h *ChangeHandler) Delete(c echo.Context) error {
	if err := h.requireWriteAccess(c); err != nil {
		return err
	}

	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid change ID"})
	}

	// Fetch change first for audit log details
	change, err := models.GetChangeByID(h.DB, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "Change not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to fetch change"})
	}

	if err := models.DeleteChange(h.DB, id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to delete change"})
	}

	auditLog(h.DB, c, "change.delete", "change", uint64Ptr(id), strPtr(change.System+": "+change.Description))
	if h.Hub != nil {
		h.Hub.Publish(SSEEvent{Type: "change.deleted", Data: DeletedPayload{ID: fmt.Sprintf("%d", id)}})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "Change deleted"})
}

func (h *ChangeHandler) List(c echo.Context) error {
	if err := h.requireReadAccess(c); err != nil {
		return err
	}

	f := models.ChangeFilters{
		System:      c.QueryParam("system"),
		Environment: c.QueryParam("environment"),
		User:        c.QueryParam("user"),
		Type:        c.QueryParam("type"),
		Search:      c.QueryParam("search"),
	}

	if v := c.QueryParam("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Limit = n
		}
	}
	if v := c.QueryParam("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			f.Offset = n
		}
	}

	// Time range: explicit from/to take precedence over shorthand
	if v := c.QueryParam("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := c.QueryParam("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}

	// timeRange shorthand (only if from/to not set)
	if f.From == nil && f.To == nil {
		if tr := c.QueryParam("timeRange"); tr != "" {
			now := time.Now()
			var cutoff time.Time
			switch tr {
			case "30m":
				cutoff = now.Add(-30 * time.Minute)
			case "1h":
				cutoff = now.Add(-1 * time.Hour)
			case "2h":
				cutoff = now.Add(-2 * time.Hour)
			case "6h":
				cutoff = now.Add(-6 * time.Hour)
			case "24h":
				cutoff = now.Add(-24 * time.Hour)
			case "7d":
				cutoff = now.Add(-7 * 24 * time.Hour)
			}
			if !cutoff.IsZero() {
				f.From = &cutoff
			}
		}
	}

	changes, total, err := models.ListChanges(h.DB, f)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list changes"})
	}

	return c.JSON(http.StatusOK, listChangesResponse{
		Changes: changes,
		Total:   total,
		Limit:   f.Limit,
		Offset:  f.Offset,
	})
}

func (h *ChangeHandler) requireWriteAccess(c echo.Context) error {
	if scopes, ok := c.Get("apiKeyScopes").([]string); ok {
		for _, s := range scopes {
			if s == "changes:write" {
				return nil
			}
		}
		return c.JSON(http.StatusForbidden, map[string]string{"error": "API key missing changes:write scope"})
	}

	claims := c.Get("claims").(*mw.JWTClaims)
	if claims.Role == "viewer" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Insufficient permissions"})
	}
	return nil
}

func (h *ChangeHandler) requireReadAccess(c echo.Context) error {
	if _, ok := c.Get("apiKeyScopes").([]string); ok {
		scopes := c.Get("apiKeyScopes").([]string)
		for _, s := range scopes {
			if s == "changes:read" {
				return nil
			}
		}
		return c.JSON(http.StatusForbidden, map[string]string{"error": "API key missing changes:read scope"})
	}

	// JWT: any authenticated user can read
	return nil
}
