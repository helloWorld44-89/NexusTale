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

	"github.com/jconder44/nexustale/internal/ai"
	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/internal/collaboration"
	"github.com/jconder44/nexustale/internal/config"
	"github.com/jconder44/nexustale/internal/export"
	"github.com/jconder44/nexustale/internal/guide"
	"github.com/jconder44/nexustale/internal/merge"
	"github.com/jconder44/nexustale/internal/notifications"
	"github.com/jconder44/nexustale/internal/project"
	"github.com/jconder44/nexustale/internal/prompts"
	"github.com/jconder44/nexustale/internal/research"
	"github.com/jconder44/nexustale/internal/wiki"
	"github.com/jconder44/nexustale/pkg/db"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
	"github.com/jconder44/nexustale/pkg/storage"
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

	// Encryption key for API key storage
	encKey, err := auth.ParseEncryptionKey(cfg.Encryption.Key)
	if err != nil {
		slog.Error("invalid encryption key", "error", err)
		os.Exit(1)
	}

	// Services
	authService := auth.NewService(queries, cfg.Auth.JWTSecret, accessExpiry, refreshExpiry, cfg.Auth.BcryptCost, encKey)
	gitService := project.NewGitService(cfg.Git.ReposPath)
	projectService := project.NewService(queries, gitService)
	aiService := ai.NewService(authService, queries, ai.AIConfig{
		OllamaURL:     cfg.AI.OllamaURL,
		OllamaModel:   cfg.AI.OllamaModel,
		MaxTokens:     cfg.AI.MaxTokens,
		BeatMaxTokens: cfg.AI.BeatMaxTokens,
	})

	// Wire the AI service as the summary notifier so scene saves trigger
	// debounced chapter-summary regeneration (B2).
	projectService.WithNotifier(aiService)

	// MinIO storage client (used by export service for EPUB uploads).
	storageClient, err := storage.New(storage.Config{
		Endpoint:  cfg.Minio.Endpoint,
		AccessKey: cfg.Minio.AccessKey,
		SecretKey: cfg.Minio.SecretKey,
		Bucket:    cfg.Minio.Bucket,
		UseSSL:    cfg.Minio.UseSSL,
	})
	if err != nil {
		slog.Error("failed to connect to MinIO", "error", err)
		os.Exit(1)
	}

	wikiService := wiki.NewService(queries, storageClient)
	exportService := export.NewService(queries, storageClient)
	exportService.StartWorkers(2)

	promptsService := prompts.NewService(queries)

	// Handlers
	authHandler := auth.NewHandler(authService)
	projectHandler := project.NewHandler(projectService)
	wikiHandler := wiki.NewHandler(wikiService)
	aiHandler := ai.NewHandler(aiService)
	exportHandler := export.NewHandler(exportService)
	guideService := guide.NewService(queries)
	guideHandler := guide.NewHandler(guideService)
	promptsHandler := prompts.NewHandler(promptsService)

	researchService := research.NewService(queries)
	researchHandler := research.NewHandler(researchService)

	notificationService := notifications.NewService(queries)
	notificationHandler := notifications.NewHandler(notificationService)

	collabService := collaboration.NewService(queries)
	collabService.WithNotificationService(notificationService)
	collabHandler := collaboration.NewHandler(collabService)

	mergeService := merge.NewService(queries, gitService)
	mergeService.WithNotificationService(notificationService)
	mergeHandler := merge.NewHandler(mergeService)

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

		authHandler.RegisterAPIKeyRoutes(v1)

		projectsGroup := v1.Group("/projects", auth.RequireAuth(authService))
		chaptersGroup := v1.Group("/chapters", auth.RequireAuth(authService))
		projectHandler.RegisterRoutes(projectsGroup, chaptersGroup)

		wikiGroup := v1.Group("/projects/:id/wiki", auth.RequireAuth(authService))
		wikiHandler.RegisterRoutes(wikiGroup)

		aiGroup := v1.Group("/projects/:id", auth.RequireAuth(authService))
		aiHandler.RegisterRoutes(aiGroup)

		// User-scoped AI routes (no project context required).
		aiUserGroup := v1.Group("", auth.RequireAuth(authService))
		aiHandler.RegisterUserRoutes(aiUserGroup)

		exportGroup := v1.Group("/projects/:id", auth.RequireAuth(authService))
		exportHandler.RegisterRoutes(exportGroup)

		guideHandler.RegisterPublicRoutes(v1)

		guideGroup := v1.Group("/projects/:id", auth.RequireAuth(authService))
		guideHandler.RegisterRoutes(guideGroup)

		promptsGroup := v1.Group("/projects/:id/prompts", auth.RequireAuth(authService))
		promptsHandler.RegisterRoutes(promptsGroup)

		researchGroup := v1.Group("/projects/:id", auth.RequireAuth(authService))
		researchHandler.RegisterRoutes(researchGroup)

		// Collaboration — public preview (no auth), auth-only accept, project-scoped management.
		collabHandler.RegisterPublicRoutes(v1)
		collabAuthGroup := v1.Group("", auth.RequireAuth(authService))
		collabHandler.RegisterAuthRoutes(collabAuthGroup)
		collabProjectGroup := v1.Group("/projects/:id", auth.RequireAuth(authService))
		collabHandler.RegisterProjectRoutes(collabProjectGroup)

		// Merge requests — project-scoped.
		mergeGroup := v1.Group("/projects/:id", auth.RequireAuth(authService))
		mergeHandler.RegisterRoutes(mergeGroup)

		// Notifications — user-scoped, no project context required.
		notifGroup := v1.Group("", auth.RequireAuth(authService))
		notificationHandler.RegisterRoutes(notifGroup)
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
