package main

import (
	"log"
	"net/http"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"

	"ops-ledger/backend/config"
	"ops-ledger/backend/database"
	"ops-ledger/backend/handlers"
	"ops-ledger/backend/middleware"
)

func main() {
	cfg := config.Load()

	db, err := database.Connect(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		log.Fatal("Migration failed:", err)
	}

	e := echo.New()

	e.Use(echomw.Logger())
	e.Use(echomw.Recover())
	e.Use(echomw.CORSWithConfig(echomw.CORSConfig{
		AllowOrigins: []string{"http://localhost:8080"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	auth := &handlers.AuthHandler{DB: db, JWTSecret: cfg.JWTSecret}

	api := e.Group("/api")
	api.POST("/auth/register", auth.Register)
	api.POST("/auth/login", auth.Login)
	api.GET("/auth/registration-status", auth.RegistrationStatus)

	protected := api.Group("")
	protected.Use(middleware.JWTAuth(cfg.JWTSecret))
	protected.GET("/auth/me", auth.Me)
	protected.POST("/auth/logout", auth.Logout)
	protected.POST("/auth/change-password", auth.ChangePassword)

	apiKeys := &handlers.ApiKeyHandler{DB: db}
	admin := api.Group("/admin")
	admin.Use(middleware.JWTAuth(cfg.JWTSecret))
	admin.GET("/api-keys", apiKeys.List)
	admin.POST("/api-keys", apiKeys.Create)
	admin.POST("/api-keys/:id/revoke", apiKeys.Revoke)
	admin.POST("/api-keys/:id/rotate", apiKeys.Rotate)

	users := &handlers.UserHandler{DB: db}
	admin.GET("/users", users.List)
	admin.POST("/users", users.Create)
	admin.PUT("/users/:id/role", users.UpdateRole)
	admin.PUT("/users/:id/status", users.UpdateStatus)
	admin.POST("/users/:id/reset-password", users.ResetPassword)

	auditHandler := &handlers.AuditHandler{DB: db}
	admin.GET("/audit", auditHandler.List)

	hub := handlers.NewEventHub()

	changes := &handlers.ChangeHandler{DB: db, Hub: hub}
	changesGroup := api.Group("/changes")
	changesGroup.Use(middleware.APIKeyOrJWT(db, cfg.JWTSecret))
	changesGroup.POST("", changes.Create)
	changesGroup.GET("", changes.List)
	changesGroup.PUT("/:id", changes.Update)
	changesGroup.DELETE("/:id", changes.Delete)
	changesGroup.PATCH("/:id/confirm", changes.Confirm)

	// SSE live events endpoint
	eventHandler := &handlers.EventHandler{Hub: hub}
	api.GET("/events", eventHandler.Stream, middleware.SSEAuth(cfg.JWTSecret))

	// MCP endpoint
	mcpHandler := &handlers.MCPHandler{DB: db, Hub: hub}
	e.POST("/mcp", mcpHandler.Handle)

	e.Logger.Fatal(e.Start(":" + cfg.Port))
}
