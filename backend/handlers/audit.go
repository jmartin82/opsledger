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

type AuditHandler struct {
	DB *sql.DB
}

type listAuditResponse struct {
	Entries []models.AuditEntry `json:"entries"`
	Total   int                 `json:"total"`
	Limit   int                 `json:"limit"`
	Offset  int                 `json:"offset"`
}

func (h *AuditHandler) List(c echo.Context) error {
	claims := c.Get("claims").(*mw.JWTClaims)
	if claims.Role != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "Admin access required"})
	}

	f := models.AuditFilters{
		Action:     c.QueryParam("action"),
		Actor:      c.QueryParam("actor"),
		TargetType: c.QueryParam("targetType"),
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

	entries, total, err := models.ListAuditEntries(h.DB, f)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Failed to list audit entries"})
	}

	return c.JSON(http.StatusOK, listAuditResponse{
		Entries: entries,
		Total:   total,
		Limit:   f.Limit,
		Offset:  f.Offset,
	})
}
