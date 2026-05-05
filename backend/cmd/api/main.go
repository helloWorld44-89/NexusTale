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
	"github.com/jconder44/nexustale/internal/annotations"
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
	"github.com/jconder44/nexustale/internal/threads"
	"github.com/jconder44/nexustale/internal/waitlist"
	"github.com/jconder44/nexustale/internal/wiki"
	"github.com/jconder44/nexustale/pkg/db"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
	"github.com/jconder44/nexustale/pkg/storage"
)

// corsMiddleware sets CORS headers. allowedOrigin is compared against the
// request Origin header; "*" permits any origin (dev only).
func corsMiddleware(allowedOrigin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		allowed := allowedOrigin == "*" || origin == allowedOrigin
		if allowed && origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
		} else if allowedOrigin == "*" {
			c.Header("Access-Control-Allow-Origin", "*")
		}
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-NexusTale-Branch")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	if err := cfg.ValidateProd(); err != nil {
		slog.Error("security configuration error", "error", err)
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

	// Wire git scene file writer into AI service so agent tool writes
	// also dual-write to the working tree (Step 1 — guide wired below after creation).
	aiService.WithSceneWriter(gitService)

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

	authService.WithStorage(storageClient)

	wikiService := wiki.NewService(queries, storageClient)

	// Wire the wiki service as the mention notifier so scene saves trigger
	// async entity-mention indexing (C7.0).
	projectService.WithMentionNotifier(wikiService)

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
	guideService.WithSceneWriter(gitService)
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

	annotationService := annotations.NewService(queries)
	annotationHandler := annotations.NewHandler(annotationService)

	threadsService := threads.NewService(queries)
	threadsHandler := threads.NewHandler(threadsService)

	waitlistService := waitlist.NewService(queries)
	waitlistHandler := waitlist.NewHandler(waitlistService)

	// Router
	gin.SetMode(cfg.Server.Mode)
	router := gin.Default()
	router.MaxMultipartMemory = 5 << 20 // 5 MiB — matches the per-file size limit in UploadEntityImage
	router.Use(corsMiddleware(cfg.Server.AllowedOrigin))

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := router.Group("/api/v1")
	{
		authGroup := v1.Group("/auth")
		authHandler.RegisterRoutes(authGroup)

		authHandler.RegisterAPIKeyRoutes(v1)

		requireAccess := collaboration.RequireProjectAccess(queries)
		requireChapterAccess := collaboration.RequireChapterAccess(queries)

		// projectsGroup serves both collection routes (/projects) and member routes
		// (/projects/:id/...). requireAccess is a no-op when :id is absent.
		projectsGroup := v1.Group("/projects", auth.RequireAuth(authService), requireAccess)
		// chaptersGroup carries RequireChapterAccess so scene routes are protected
		// even though they don't have a project :id in the URL.
		chaptersGroup := v1.Group("/chapters", auth.RequireAuth(authService), requireChapterAccess)
		projectHandler.RegisterRoutes(projectsGroup, chaptersGroup)

		wikiGroup := v1.Group("/projects/:id/wiki", auth.RequireAuth(authService), requireAccess)
		wikiHandler.RegisterRoutes(wikiGroup)

		// Mention routes sit under /projects/:id/scenes/:sid so the frontend can
		// fetch them alongside scene data without navigating to the wiki URL space.
		mentionGroup := v1.Group("/projects/:id/scenes/:sid", auth.RequireAuth(authService), requireAccess)
		wikiHandler.RegisterMentionRoutes(mentionGroup)

		aiGroup := v1.Group("/projects/:id", auth.RequireAuth(authService), requireAccess)
		aiHandler.RegisterRoutes(aiGroup)

		// User-scoped AI routes (no project context required).
		aiUserGroup := v1.Group("", auth.RequireAuth(authService))
		aiHandler.RegisterUserRoutes(aiUserGroup)

		exportGroup := v1.Group("/projects/:id", auth.RequireAuth(authService), requireAccess)
		exportHandler.RegisterRoutes(exportGroup)

		guideHandler.RegisterPublicRoutes(v1)

		guideGroup := v1.Group("/projects/:id", auth.RequireAuth(authService), requireAccess)
		guideHandler.RegisterRoutes(guideGroup)

		promptsGroup := v1.Group("/projects/:id/prompts", auth.RequireAuth(authService), requireAccess)
		promptsHandler.RegisterRoutes(promptsGroup)

		researchGroup := v1.Group("/projects/:id", auth.RequireAuth(authService), requireAccess)
		researchHandler.RegisterRoutes(researchGroup)

		// Collaboration — public preview (no auth), auth-only accept, project-scoped management.
		collabHandler.RegisterPublicRoutes(v1)
		collabAuthGroup := v1.Group("", auth.RequireAuth(authService))
		collabHandler.RegisterAuthRoutes(collabAuthGroup)
		collabProjectGroup := v1.Group("/projects/:id", auth.RequireAuth(authService), requireAccess)
		collabHandler.RegisterProjectRoutes(collabProjectGroup)

		// Merge requests — project-scoped.
		mergeGroup := v1.Group("/projects/:id", auth.RequireAuth(authService), requireAccess)
		mergeHandler.RegisterRoutes(mergeGroup)

		// Notifications — user-scoped, no project context required.
		notifGroup := v1.Group("", auth.RequireAuth(authService))
		notificationHandler.RegisterRoutes(notifGroup)

		// Annotations — project + scene scoped.
		annotationGroup := v1.Group("/projects/:id", auth.RequireAuth(authService), requireAccess)
		annotationHandler.RegisterRoutes(annotationGroup)

		// Story threads — project-scoped.
		threadsGroup := v1.Group("/projects/:id", auth.RequireAuth(authService), requireAccess)
		threadsHandler.RegisterRoutes(threadsGroup)

		// Waitlist — public, no auth.
		waitlistHandler.RegisterRoutes(v1)
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
