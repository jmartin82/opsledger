package middleware

import (
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"

	"ops-ledger/backend/models"
)

// APIKeyOrJWT returns a middleware that authenticates via API key (ol_live_ prefix)
// or falls back to JWT authentication. This allows both auth methods on the same routes.
func APIKeyOrJWT(db *sql.DB, jwtSecret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			header := c.Request().Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Missing or invalid authorization header"})
			}

			tokenStr := strings.TrimPrefix(header, "Bearer ")

			// API key auth path
			if strings.HasPrefix(tokenStr, "ol_live_") {
				return authenticateAPIKey(c, db, tokenStr, next)
			}

			// JWT auth path (same logic as JWTAuth middleware)
			claims := &JWTClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				return []byte(jwtSecret), nil
			})
			if err != nil || !token.Valid {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid or expired token"})
			}

			c.Set("claims", claims)
			return next(c)
		}
	}
}

func authenticateAPIKey(c echo.Context, db *sql.DB, rawKey string, next echo.HandlerFunc) error {
	keyHash := models.HashAPIKey(rawKey)

	apiKey, err := models.GetApiKeyByHash(db, keyHash)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid API key"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "Authentication error"})
	}

	// Check expiry
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "API key has expired"})
	}

	// Update last used (fire-and-forget)
	go func() { _ = models.UpdateApiKeyLastUsed(db, apiKey.ID) }()

	c.Set("apiKeyID", apiKey.ID)
	c.Set("apiKeyScopes", apiKey.Scopes)
	c.Set("apiKeyCreatedBy", apiKey.CreatedBy)

	return next(c)
}
