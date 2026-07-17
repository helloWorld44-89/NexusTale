package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jconder44/nexustale/internal/annotations"
	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/internal/collaboration"
	"github.com/jconder44/nexustale/internal/maps"
	"github.com/jconder44/nexustale/internal/notifications"
	"github.com/jconder44/nexustale/internal/project"
	"github.com/jconder44/nexustale/internal/research"
	"github.com/jconder44/nexustale/internal/wiki"
	"github.com/jconder44/nexustale/pkg/db"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

const (
	TestJWTSecret     = "test-secret-key-for-testing-only"
	TestAccessExpiry  = 15 * time.Minute
	TestRefreshExpiry = 24 * time.Hour
	TestBcryptCost    = 4 // Low cost for fast tests
)

func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dbURL := os.Getenv("NEXUSTALE_TEST_DB_URL")
	if dbURL == "" {
		dbURL = "postgres://nexustale:nexustale@localhost:5432/nexustale?sslmode=disable"
	}

	pool, err := db.NewPool(context.Background(), dbURL, 5, 1)
	if err != nil {
		t.Skipf("skipping integration test: cannot connect to database: %v", err)
	}

	migrationsPath := os.Getenv("NEXUSTALE_TEST_MIGRATIONS_PATH")
	if migrationsPath == "" {
		migrationsPath = "file://../../pkg/db/migrations"
	}

	if err := db.RunMigrations(dbURL, migrationsPath); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	t.Cleanup(func() {
		cleanDB(pool)
		pool.Close()
	})

	return pool
}

func cleanDB(pool *pgxpool.Pool) {
	ctx := context.Background()
	// Order: leaf tables first, then parents. Most have CASCADE FKs on project_id/user_id,
	// but explicit deletes keep the cleanup deterministic and avoid FK violation errors
	// when tests leave partial state.
	tables := []string{
		// Annotations (depend on scenes, projects, users)
		"manuscript_annotations",
		// Wiki (depend on projects)
		"wiki_timeline_events", "wiki_magic_rules", "wiki_relationships", "wiki_entities",
		// Collaboration & merge (depend on projects, users)
		"merge_requests",
		"project_invites", "project_collaborators",
		// AI state (depend on projects, chapters)
		"ai_context_pins", "workshop_sessions", "chapter_summaries", "project_active_branch",
		// Research & notifications (depend on projects, users)
		"research_notes", "notifications",
		// Usage, exports, guide (depend on projects, users)
		"ai_usage", "export_jobs", "guide_steps",
		// API keys (depend on users)
		"user_api_keys",
		// Core content — scenes before chapters before acts before projects
		"scenes", "chapters", "acts", "forks", "projects",
		// Auth
		"refresh_tokens", "users",
	}
	for _, table := range tables {
		pool.Exec(ctx, fmt.Sprintf("DELETE FROM %s", table))
	}
}

// SetupRouter builds a test router wired with all current services.
// Git is disabled (nil) — use SetupRouterWithGit for tests that exercise git routes.
func SetupRouter(t *testing.T) (*gin.Engine, *sqlcgen.Queries, *auth.Service) {
	t.Helper()
	return setupRouter(t, "")
}

// SetupRouterWithGit is like SetupRouter but wires a real GitService using a
// temporary directory. Use this for tests that exercise Chronicle, Lore, Echo,
// Diverge, TravelTo, or Canonize.
func SetupRouterWithGit(t *testing.T) (*gin.Engine, *sqlcgen.Queries, *auth.Service) {
	t.Helper()
	return setupRouter(t, t.TempDir())
}

func setupRouter(t *testing.T, reposPath string) (*gin.Engine, *sqlcgen.Queries, *auth.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	pool := SetupTestDB(t)
	queries := sqlcgen.New(pool)

	// Use a zero key for tests — encryption is functional but not secret.
	testEncKey := make([]byte, 32)
	authService := auth.NewService(queries, TestJWTSecret, TestAccessExpiry, TestRefreshExpiry, TestBcryptCost, testEncKey)

	var gitSvc *project.GitService
	if reposPath != "" {
		gitSvc = project.NewGitService(reposPath)
	}
	projectService := project.NewService(queries, gitSvc)
	// storage is nil in tests — wiki image upload and export routes return 500 if
	// exercised, but existing tests do not call those paths.
	wikiService          := wiki.NewService(queries, nil)
	mapsService          := maps.NewService(queries, gitSvc)
	researchService      := research.NewService(queries)
	annotationService    := annotations.NewService(queries)
	notifService         := notifications.NewService(queries)
	collabService        := collaboration.NewService(queries)
	collabService.WithNotificationService(notifService)

	authHandler        := auth.NewHandler(authService, true) // open in tests
	projectHandler     := project.NewHandler(projectService)
	wikiHandler        := wiki.NewHandler(wikiService)
	mapsHandler        := maps.NewHandler(mapsService)
	researchHandler    := research.NewHandler(researchService)
	annotationHandler  := annotations.NewHandler(annotationService)
	notifHandler       := notifications.NewHandler(notifService)
	collabHandler      := collaboration.NewHandler(collabService)

	router := gin.New()

	v1 := router.Group("/api/v1")
	{
		authGroup := v1.Group("/auth")
		authHandler.RegisterRoutes(authGroup)

		projectsGroup := v1.Group("/projects", auth.RequireAuth(authService))
		chaptersGroup := v1.Group("/chapters", auth.RequireAuth(authService))
		projectHandler.RegisterRoutes(projectsGroup, chaptersGroup)

		wikiGroup := v1.Group("/projects/:id/wiki", auth.RequireAuth(authService))
		wikiHandler.RegisterRoutes(wikiGroup)

		mapsGroup := v1.Group("/projects/:id", auth.RequireAuth(authService))
		mapsHandler.RegisterRoutes(mapsGroup)

		researchGroup := v1.Group("/projects/:id", auth.RequireAuth(authService))
		researchHandler.RegisterRoutes(researchGroup)

		annotationGroup := v1.Group("/projects/:id", auth.RequireAuth(authService))
		annotationHandler.RegisterRoutes(annotationGroup)

		notifGroup := v1.Group("", auth.RequireAuth(authService))
		notifHandler.RegisterRoutes(notifGroup)

		collabHandler.RegisterPublicRoutes(v1)
		collabAuthGroup := v1.Group("", auth.RequireAuth(authService))
		collabHandler.RegisterAuthRoutes(collabAuthGroup)
		collabProjectGroup := v1.Group("/projects/:id", auth.RequireAuth(authService))
		collabHandler.RegisterProjectRoutes(collabProjectGroup)
	}

	return router, queries, authService
}

// RegisterAndGetToken registers a new user and returns their access token.
// Used as a shared helper across test packages.
func RegisterAndGetToken(t *testing.T, router *gin.Engine, email string) string {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"email":        email,
		"display_name": "Test User",
		"password":     "password123",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("register failed: %d: %s", w.Code, w.Body.String())
	}

	var resp auth.AuthResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.Tokens.AccessToken
}

// RegisterUser registers a new user and returns their access token plus user ID.
func RegisterUser(t *testing.T, router *gin.Engine, email, displayName string) (token, userID string) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{
		"email":        email,
		"display_name": displayName,
		"password":     "password123",
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/api/v1/auth/register", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("register failed: %d: %s", w.Code, w.Body.String())
	}

	var resp auth.AuthResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	return resp.Tokens.AccessToken, resp.User.ID.String()
}

// AuthRequest builds an authenticated JSON request.
func AuthRequest(method, url, token string, body interface{}) *http.Request {
	var buf *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		buf = bytes.NewBuffer(b)
	} else {
		buf = &bytes.Buffer{}
	}
	req, _ := http.NewRequest(method, url, buf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	return req
}
