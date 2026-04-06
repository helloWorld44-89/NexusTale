package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/internal/config"
	"github.com/jconder44/nexustale/internal/project"
	"github.com/jconder44/nexustale/internal/wiki"
	"github.com/jconder44/nexustale/pkg/db"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Database
	ctx := context.Background()
	pool, err := db.NewPool(ctx, cfg.DB.URL, cfg.DB.MaxConns, cfg.DB.MinConns)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Migrations
	if err := db.RunMigrations(cfg.DB.URL, cfg.DB.MigrationsPath); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	queries := sqlcgen.New(pool)

	// Parse durations
	accessExpiry, err := time.ParseDuration(cfg.Auth.AccessTokenExpiry)
	if err != nil {
		slog.Error("invalid access token expiry", "value", cfg.Auth.AccessTokenExpiry, "error", err)
		os.Exit(1)
	}
	refreshExpiry, err := time.ParseDuration(cfg.Auth.RefreshTokenExpiry)
	if err != nil {
		slog.Error("invalid refresh token expiry", "value", cfg.Auth.RefreshTokenExpiry, "error", err)
		os.Exit(1)
	}

	// Services
	authService := auth.NewService(queries, cfg.Auth.JWTSecret, accessExpiry, refreshExpiry, cfg.Auth.BcryptCost)
	gitService := project.NewGitService(cfg.Git.ReposPath)
	projectService := project.NewService(queries, gitService)
	wikiService := wiki.NewService(queries)

	// Handlers
	authHandler := auth.NewHandler(authService)
	projectHandler := project.NewHandler(projectService)
	wikiHandler := wiki.NewHandler(wikiService)

	// Router
	gin.SetMode(cfg.Server.Mode)
	router := gin.Default()

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := router.Group("/api/v1")
	{
		authGroup := v1.Group("/auth")
		authHandler.RegisterRoutes(authGroup)

		projectsGroup := v1.Group("/projects", auth.RequireAuth(authService))
		projectHandler.RegisterRoutes(projectsGroup)

		wikiGroup := v1.Group("/projects/:id/wiki", auth.RequireAuth(authService))
		wikiHandler.RegisterRoutes(wikiGroup)
	}

	// Server with graceful shutdown
	srv := &http.Server{
		Addr:    ":" + cfg.Server.Port,
		Handler: router,
	}

	go func() {
		slog.Info("starting server", "port", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced shutdown", "error", err)
	}

	slog.Info("server stopped")
}
