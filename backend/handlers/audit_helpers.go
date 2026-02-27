package handlers

import (
	"database/sql"
	"log"
	"strconv"

	"github.com/labstack/echo/v4"

	mw "ops-ledger/backend/middleware"
	"ops-ledger/backend/models"
)

func extractActor(c echo.Context) (string, *uint64) {
	if claims, ok := c.Get("claims").(*mw.JWTClaims); ok {
		sub, err := claims.GetSubject()
		if err == nil {
			if id, err := strconv.ParseUint(sub, 10, 64); err == nil {
				return claims.Email, uint64Ptr(id)
			}
		}
		return claims.Email, nil
	}

	if name, ok := c.Get("apiKeyCreatedBy").(uint64); ok {
		return "api_key", uint64Ptr(name)
	}

	return "unknown", nil
}

func auditLog(db *sql.DB, c echo.Context, action, targetType string, targetID *uint64, targetUUID *string, details *string) {
	actor, actorID := extractActor(c)
	ip := c.RealIP()
	go func() {
		if err := models.CreateAuditEntry(db, actor, actorID, action, targetType, targetID, targetUUID, details, &ip); err != nil {
			log.Printf("audit: failed to log %s: %v", action, err)
		}
	}()
}

func uint64Ptr(v uint64) *uint64 {
	return &v
}

func strPtr(v string) *string {
	return &v
}
