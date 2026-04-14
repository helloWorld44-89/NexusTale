package ai_test

// context_test.go — integration tests for BuildContext and ResolveBranch.
//
// These tests run against a real database (skipped automatically when none is
// reachable). They verify that the context-assembly pipeline inserts the
// correct sections into the system prompt block that every AI call receives.
//
// Run:  make test  (serial, -p 1 to avoid cross-package DB races)
// Env:  NEXUSTALE_TEST_DB_URL (defaults to localhost dev DB)

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jconder44/nexustale/internal/ai"
	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/internal/testutil"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// newAIService builds a minimal ai.Service backed by the test DB.
// BuildContext only uses s.queries, so we pass a zero-config auth service.
func newAIService(t *testing.T, q *sqlcgen.Queries, as *auth.Service) *ai.Service {
	t.Helper()
	return ai.NewService(as, q, ai.AIConfig{})
}

// seedProject inserts a minimal project owned by userID and returns its ID.
func seedProject(t *testing.T, ctx context.Context, q *sqlcgen.Queries, ownerID uuid.UUID, title string, genres []string, aiInstructions string) uuid.UUID {
	t.Helper()
	if genres == nil {
		genres = []string{} // genres is NOT NULL in the schema
	}
	p, err := q.CreateProject(ctx, sqlcgen.CreateProjectParams{
		OwnerID:     ownerID,
		Title:       title,
		Description: "",
		Genres:      genres,
		GitRepoPath: "",
	})
	if err != nil {
		t.Fatalf("seed project: %v", err)
	}
	if aiInstructions != "" {
		if err := q.UpdateAIInstructions(ctx, sqlcgen.UpdateAIInstructionsParams{
			ID:             p.ID,
			AiInstructions: aiInstructions,
		}); err != nil {
			t.Fatalf("set ai_instructions: %v", err)
		}
	}
	return p.ID
}

// seedAct inserts a default act for the project.
func seedAct(t *testing.T, ctx context.Context, q *sqlcgen.Queries, projectID uuid.UUID) uuid.UUID {
	t.Helper()
	act, err := q.CreateAct(ctx, sqlcgen.CreateActParams{
		ProjectID: projectID,
		Title:     "Act 1",
		Summary:   "",
		SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("seed act: %v", err)
	}
	return act.ID
}

// seedChapter inserts a chapter and returns its ID.
func seedChapter(t *testing.T, ctx context.Context, q *sqlcgen.Queries, projectID, actID uuid.UUID, title string) uuid.UUID {
	t.Helper()
	ch, err := q.CreateChapter(ctx, sqlcgen.CreateChapterParams{
		ProjectID: projectID,
		ActID:     actID,
		Title:     title,
		Summary:   "",
		SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("seed chapter %q: %v", title, err)
	}
	return ch.ID
}

// seedScene inserts a scene under a chapter and returns its ID.
func seedScene(t *testing.T, ctx context.Context, q *sqlcgen.Queries, chapterID uuid.UUID, title, content string) uuid.UUID {
	t.Helper()
	sc, err := q.CreateScene(ctx, sqlcgen.CreateSceneParams{
		ChapterID: chapterID,
		Title:     title,
		Content:   content,
		Tags:      []string{}, // tags is NOT NULL in the schema
		SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("seed scene %q: %v", title, err)
	}
	return sc.ID
}

// seedEntity inserts a wiki entity and returns its ID.
func seedEntity(t *testing.T, ctx context.Context, q *sqlcgen.Queries, projectID uuid.UUID, name, entityType, summary string) uuid.UUID {
	t.Helper()
	e, err := q.CreateEntity(ctx, sqlcgen.CreateEntityParams{
		ProjectID:      projectID,
		ParentEntityID: pgtype.UUID{Valid: false},
		Type:           entityType,
		Name:           name,
		Summary:        summary,
		Attributes:     []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("seed entity %q: %v", name, err)
	}
	return e.ID
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestBuildContext_ProjectIdentity(t *testing.T) {
	// Always includes project title and genres, even with no chapters or scenes.
	pool := testutil.SetupTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	authSvc := auth.NewService(q, testutil.TestJWTSecret, testutil.TestAccessExpiry, testutil.TestRefreshExpiry, testutil.TestBcryptCost, make([]byte, 32))
	svc := newAIService(t, q, authSvc)

	user, _ := q.CreateUser(ctx, sqlcgen.CreateUserParams{Email: "ctx-id@test.com", DisplayName: "T", PasswordHash: "x", Role: sqlcgen.UserRoleAuthor})
	projectID := seedProject(t, ctx, q, user.ID, "The Starfall Accord", []string{"sci-fi", "thriller"}, "")

	result := svc.BuildContext(ctx, projectID, "canon", "", uuid.Nil)

	if !strings.Contains(result, "The Starfall Accord") {
		t.Error("context should include project title")
	}
	if !strings.Contains(result, "sci-fi") {
		t.Error("context should include genres")
	}
}

func TestBuildContext_AIBibleIncluded(t *testing.T) {
	// When ai_instructions is set, a Story bible section appears.
	pool := testutil.SetupTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	authSvc := auth.NewService(q, testutil.TestJWTSecret, testutil.TestAccessExpiry, testutil.TestRefreshExpiry, testutil.TestBcryptCost, make([]byte, 32))
	svc := newAIService(t, q, authSvc)

	user, _ := q.CreateUser(ctx, sqlcgen.CreateUserParams{Email: "ctx-bible@test.com", DisplayName: "T", PasswordHash: "x", Role: sqlcgen.UserRoleAuthor})
	projectID := seedProject(t, ctx, q, user.ID, "Ember Worlds", nil, "This story is set in a volcanic archipelago where fire magic is forbidden.")

	result := svc.BuildContext(ctx, projectID, "canon", "", uuid.Nil)

	if !strings.Contains(result, "Story bible") {
		t.Error("context should include Story bible header when ai_instructions is set")
	}
	if !strings.Contains(result, "volcanic archipelago") {
		t.Error("context should include the actual AI bible text")
	}
}

func TestBuildContext_RawSceneFallback(t *testing.T) {
	// When no AI chapter summary exists, raw scene content (truncated) appears.
	pool := testutil.SetupTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	authSvc := auth.NewService(q, testutil.TestJWTSecret, testutil.TestAccessExpiry, testutil.TestRefreshExpiry, testutil.TestBcryptCost, make([]byte, 32))
	svc := newAIService(t, q, authSvc)

	user, _ := q.CreateUser(ctx, sqlcgen.CreateUserParams{Email: "ctx-fallback@test.com", DisplayName: "T", PasswordHash: "x", Role: sqlcgen.UserRoleAuthor})
	projectID := seedProject(t, ctx, q, user.ID, "The Glass Codex", nil, "")
	actID := seedAct(t, ctx, q, projectID)
	chapterID := seedChapter(t, ctx, q, projectID, actID, "Chapter One: The Arrival")
	seedScene(t, ctx, q, chapterID, "Scene 1", "The ship descended through cloud cover. Captain Mira checked her instruments for the third time.")

	result := svc.BuildContext(ctx, projectID, "canon", "", uuid.Nil)

	if !strings.Contains(result, "Story so far") {
		t.Error("context should include Story so far block")
	}
	if !strings.Contains(result, "Chapter One: The Arrival") {
		t.Error("context should include chapter title")
	}
	if !strings.Contains(result, "excerpt") {
		t.Error("fallback content should be labelled as excerpt")
	}
	if !strings.Contains(result, "Captain Mira") {
		t.Error("context should include scene content snippet")
	}
}

func TestBuildContext_RawFallbackTruncatedAt600Runes(t *testing.T) {
	// Scene content beyond 600 runes should be truncated with an ellipsis.
	pool := testutil.SetupTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	authSvc := auth.NewService(q, testutil.TestJWTSecret, testutil.TestAccessExpiry, testutil.TestRefreshExpiry, testutil.TestBcryptCost, make([]byte, 32))
	svc := newAIService(t, q, authSvc)

	user, _ := q.CreateUser(ctx, sqlcgen.CreateUserParams{Email: "ctx-trunc@test.com", DisplayName: "T", PasswordHash: "x", Role: sqlcgen.UserRoleAuthor})
	projectID := seedProject(t, ctx, q, user.ID, "Long Novel", nil, "")
	actID := seedAct(t, ctx, q, projectID)
	chapterID := seedChapter(t, ctx, q, projectID, actID, "Prologue")

	// Build a scene content string that's definitely longer than 600 runes.
	longContent := strings.Repeat("The ancient hall was quiet. ", 30) // ~810 runes
	seedScene(t, ctx, q, chapterID, "Opening", longContent)

	result := svc.BuildContext(ctx, projectID, "canon", "", uuid.Nil)

	if !strings.Contains(result, "…") {
		t.Error("long raw content should be truncated with ellipsis")
	}
}

func TestBuildContext_UsesSummaryWhenPresent(t *testing.T) {
	// When a chapter summary exists, it should be used instead of raw content.
	pool := testutil.SetupTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	authSvc := auth.NewService(q, testutil.TestJWTSecret, testutil.TestAccessExpiry, testutil.TestRefreshExpiry, testutil.TestBcryptCost, make([]byte, 32))
	svc := newAIService(t, q, authSvc)

	user, _ := q.CreateUser(ctx, sqlcgen.CreateUserParams{Email: "ctx-summary@test.com", DisplayName: "T", PasswordHash: "x", Role: sqlcgen.UserRoleAuthor})
	projectID := seedProject(t, ctx, q, user.ID, "The Crystal Throne", nil, "")
	actID := seedAct(t, ctx, q, projectID)
	chapterID := seedChapter(t, ctx, q, projectID, actID, "Chapter Two")
	seedScene(t, ctx, q, chapterID, "Scene", "Raw content that should not appear.")

	const summaryText = "The queen discovers a betrayal among her closest advisors."
	if err := q.UpsertChapterSummary(ctx, sqlcgen.UpsertChapterSummaryParams{
		ChapterID:  chapterID,
		BranchName: "canon",
		AiSummary:  summaryText,
	}); err != nil {
		t.Fatalf("upsert summary: %v", err)
	}

	result := svc.BuildContext(ctx, projectID, "canon", "", uuid.Nil)

	if !strings.Contains(result, summaryText) {
		t.Error("context should use AI chapter summary when available")
	}
	if strings.Contains(result, "Raw content that should not appear") {
		t.Error("context should not include raw content when summary exists")
	}
	if strings.Contains(result, "excerpt") {
		t.Error("context should not label content as excerpt when using a real summary")
	}
}

func TestBuildContext_CurrentSceneBlock(t *testing.T) {
	// When sceneID and sceneContent are provided, a "Current scene" block appears.
	pool := testutil.SetupTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	authSvc := auth.NewService(q, testutil.TestJWTSecret, testutil.TestAccessExpiry, testutil.TestRefreshExpiry, testutil.TestBcryptCost, make([]byte, 32))
	svc := newAIService(t, q, authSvc)

	user, _ := q.CreateUser(ctx, sqlcgen.CreateUserParams{Email: "ctx-scene@test.com", DisplayName: "T", PasswordHash: "x", Role: sqlcgen.UserRoleAuthor})
	projectID := seedProject(t, ctx, q, user.ID, "Current Scene Test", nil, "")
	actID := seedAct(t, ctx, q, projectID)
	chapterID := seedChapter(t, ctx, q, projectID, actID, "Ch1")
	sceneContent := "She pushed open the iron gate and stepped into the garden."
	sceneID := seedScene(t, ctx, q, chapterID, "The Garden Scene", sceneContent)

	result := svc.BuildContext(ctx, projectID, "canon", sceneContent, sceneID)

	if !strings.Contains(result, "Current scene") {
		t.Error("context should include Current scene header")
	}
	if !strings.Contains(result, "The Garden Scene") {
		t.Error("context should include the scene title in the Current scene label")
	}
	if !strings.Contains(result, sceneContent) {
		t.Error("context should include the full current scene content")
	}
}

func TestBuildContext_EntityReferenceInjected(t *testing.T) {
	// @[EntityName] refs in scene content should resolve to wiki entity snippets.
	pool := testutil.SetupTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	authSvc := auth.NewService(q, testutil.TestJWTSecret, testutil.TestAccessExpiry, testutil.TestRefreshExpiry, testutil.TestBcryptCost, make([]byte, 32))
	svc := newAIService(t, q, authSvc)

	user, _ := q.CreateUser(ctx, sqlcgen.CreateUserParams{Email: "ctx-entity@test.com", DisplayName: "T", PasswordHash: "x", Role: sqlcgen.UserRoleAuthor})
	projectID := seedProject(t, ctx, q, user.ID, "Entity Test", nil, "")
	actID := seedAct(t, ctx, q, projectID)
	chapterID := seedChapter(t, ctx, q, projectID, actID, "Ch1")

	const entitySummary = "A rogue starship captain with a cybernetic eye."
	seedEntity(t, ctx, q, projectID, "Captain Rael", "character", entitySummary)

	// Scene references Captain Rael using @[...] syntax.
	sceneContent := "@[Captain Rael] stepped onto the bridge."
	sceneID := seedScene(t, ctx, q, chapterID, "Bridge", sceneContent)

	result := svc.BuildContext(ctx, projectID, "canon", sceneContent, sceneID)

	if !strings.Contains(result, "Referenced entities") {
		t.Error("context should include Referenced entities section")
	}
	if !strings.Contains(result, "Captain Rael") {
		t.Error("context should include the entity name")
	}
	if !strings.Contains(result, entitySummary) {
		t.Error("context should include the entity summary")
	}
}

func TestBuildContext_EmptyProjectID_ReturnsEmpty(t *testing.T) {
	// A nil project ID should return an empty string — no crash, no partial context.
	pool := testutil.SetupTestDB(t)
	q := sqlcgen.New(pool)
	authSvc := auth.NewService(q, testutil.TestJWTSecret, testutil.TestAccessExpiry, testutil.TestRefreshExpiry, testutil.TestBcryptCost, make([]byte, 32))
	svc := newAIService(t, q, authSvc)

	result := svc.BuildContext(context.Background(), uuid.Nil, "canon", "", uuid.Nil)
	if result != "" {
		t.Errorf("expected empty context for nil project, got: %q", result)
	}
}

func TestBuildContext_BranchFallsBackToCanon(t *testing.T) {
	// When the active branch has no summaries, the function falls back to canon summaries.
	pool := testutil.SetupTestDB(t)
	q := sqlcgen.New(pool)
	ctx := context.Background()
	authSvc := auth.NewService(q, testutil.TestJWTSecret, testutil.TestAccessExpiry, testutil.TestRefreshExpiry, testutil.TestBcryptCost, make([]byte, 32))
	svc := newAIService(t, q, authSvc)

	user, _ := q.CreateUser(ctx, sqlcgen.CreateUserParams{Email: "ctx-branch@test.com", DisplayName: "T", PasswordHash: "x", Role: sqlcgen.UserRoleAuthor})
	projectID := seedProject(t, ctx, q, user.ID, "Branch Fallback", nil, "")
	actID := seedAct(t, ctx, q, projectID)
	chapterID := seedChapter(t, ctx, q, projectID, actID, "Opening")
	seedScene(t, ctx, q, chapterID, "S1", "placeholder")

	const canonSummary = "The hero departs the village on the canon branch."
	if err := q.UpsertChapterSummary(ctx, sqlcgen.UpsertChapterSummaryParams{
		ChapterID:  chapterID,
		BranchName: "canon",
		AiSummary:  canonSummary,
	}); err != nil {
		t.Fatalf("upsert canon summary: %v", err)
	}

	// Request context on "dark-timeline" which has no summaries — expect canon fallback.
	result := svc.BuildContext(ctx, projectID, "dark-timeline", "", uuid.Nil)

	if !strings.Contains(result, canonSummary) {
		t.Error("context on branchless timeline should fall back to canon summary")
	}
}

// ── ResolveBranch ─────────────────────────────────────────────────────────────

func TestResolveBranch_HeaderTakesPrecedence(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	q := sqlcgen.New(pool)
	authSvc := auth.NewService(q, testutil.TestJWTSecret, testutil.TestAccessExpiry, testutil.TestRefreshExpiry, testutil.TestBcryptCost, make([]byte, 32))
	svc := newAIService(t, q, authSvc)

	got := svc.ResolveBranch(context.Background(), "my-feature-branch", uuid.New(), uuid.New())
	if got != "my-feature-branch" {
		t.Errorf("expected header branch, got %q", got)
	}
}

func TestResolveBranch_FallsBackToCanon(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	q := sqlcgen.New(pool)
	authSvc := auth.NewService(q, testutil.TestJWTSecret, testutil.TestAccessExpiry, testutil.TestRefreshExpiry, testutil.TestBcryptCost, make([]byte, 32))
	svc := newAIService(t, q, authSvc)

	// No header, no DB row for this user/project — should return "canon".
	got := svc.ResolveBranch(context.Background(), "", uuid.New(), uuid.New())
	if got != "canon" {
		t.Errorf("expected canon fallback, got %q", got)
	}
}
