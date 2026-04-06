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

	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/internal/project"
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
	// Wiki tables first (cascade from projects, but explicit is clearer).
	// Entity order matches FK dependencies: children before parents.
	tables := []string{
		"wiki_timeline_events", "wiki_magic_rules", "wiki_relationships", "wiki_entities",
		"scenes", "chapters", "forks", "project_collaborators", "projects",
		"refresh_tokens", "users",
	}
	for _, table := range tables {
		pool.Exec(ctx, fmt.Sprintf("DELETE FROM %s", table))
	}
}

// SetupRouter builds a test router wired with all current services.
// The wiki group mirrors the mounting in cmd/api/main.go.
func SetupRouter(t *testing.T) (*gin.Engine, *sqlcgen.Queries, *auth.Service) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	pool := SetupTestDB(t)
	queries := sqlcgen.New(pool)

	authService := auth.NewService(queries, TestJWTSecret, TestAccessExpiry, TestRefreshExpiry, TestBcryptCost)
	projectService := project.NewService(queries, nil) // No git for tests
	wikiService := wiki.NewService(queries)

	authHandler := auth.NewHandler(authService)
	projectHandler := project.NewHandler(projectService)
	wikiHandler := wiki.NewHandler(wikiService)

	router := gin.New()

	v1 := router.Group("/api/v1")
	{
		authGroup := v1.Group("/auth")
		authHandler.RegisterRoutes(authGroup)

		projectsGroup := v1.Group("/projects", auth.RequireAuth(authService))
		projectHandler.RegisterRoutes(projectsGroup)

		wikiGroup := v1.Group("/projects/:id/wiki", auth.RequireAuth(authService))
		wikiHandler.RegisterRoutes(wikiGroup)
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
