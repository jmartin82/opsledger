package middleware

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

// SSEAuth is JWT-only middleware with a ?token= query-param fallback.
// Browser EventSource / fetch can't send custom headers, so the JWT is
// passed as a query parameter instead.  API keys are intentionally NOT
// supported here — exposing long-lived keys in URLs and server logs is
// a security risk.
func SSEAuth(secret string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// 1. Try query param first (EventSource / fetch without header support)
			tokenStr := c.QueryParam("token")

			// 2. Fall back to Authorization header
			if tokenStr == "" {
				header := c.Request().Header.Get("Authorization")
				if strings.HasPrefix(header, "Bearer ") {
					tokenStr = strings.TrimPrefix(header, "Bearer ")
				}
			}

			if tokenStr == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Missing token"})
			}

			claims := &JWTClaims{}
			token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "Invalid or expired token"})
			}

			c.Set("claims", claims)
			return next(c)
		}
	}
}
