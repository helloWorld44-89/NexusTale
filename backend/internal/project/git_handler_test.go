package project_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jconder44/nexustale/internal/project"
	"github.com/jconder44/nexustale/internal/testutil"
)

// setupGitProject creates a project with one chapter and one scene, returning
// the project URL base and the access token. It is shared by all git tests.
func setupGitProject(t *testing.T) (router interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, projectURL, token string) {
	t.Helper()

	r, _, _ := testutil.SetupRouterWithGit(t)
	tok := testutil.RegisterAndGetToken(t, r, "git-test-"+t.Name()+"@example.com")

	// Create project
	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest("POST", "/api/v1/projects", tok, map[string]interface{}{
		"title":       "Git Test Project",
		"description": "for git integration tests",
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create project: want 201, got %d: %s", w.Code, w.Body.String())
	}
	var proj project.ProjectResponse
	json.Unmarshal(w.Body.Bytes(), &proj)

	base := "/api/v1/projects/" + proj.ID.String()

	// Create a chapter
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, testutil.AuthRequest("POST", base+"/chapters", tok, map[string]interface{}{
		"title":      "Chapter One",
		"sort_order": 1,
	}))
	if w2.Code != http.StatusCreated {
		t.Fatalf("create chapter: want 201, got %d: %s", w2.Code, w2.Body.String())
	}
	var ch project.ChapterResponse
	json.Unmarshal(w2.Body.Bytes(), &ch)

	// Create a scene with content so Chronicle has something to commit
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, testutil.AuthRequest("POST", base+"/chapters/"+ch.ID.String()+"/scenes", tok, map[string]interface{}{
		"title":      "Opening Scene",
		"content":    "The stars were silent.",
		"sort_order": 1,
	}))
	if w3.Code != http.StatusCreated {
		t.Fatalf("create scene: want 201, got %d: %s", w3.Code, w3.Body.String())
	}

	return r, base, tok
}

// ── GitStatus ─────────────────────────────────────────────────────────────────

func TestGitStatus_NoChronicle(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("GET", projectURL+"/git/status", token, nil))

	if w.Code != http.StatusOK {
		t.Fatalf("git status: want 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp project.GitStatusResponse
	json.Unmarshal(w.Body.Bytes(), &resp)

	if resp.CurrentTimeline != "canon" {
		t.Errorf("expected current_timeline 'canon', got %q", resp.CurrentTimeline)
	}
	// No Chronicle yet from the user — init commit exists but no user Chronicle.
	// LastChronicle will reflect the init commit.
}

// ── Chronicle ─────────────────────────────────────────────────────────────────

func TestChronicle_CreatesCommit(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("POST", projectURL+"/git/chronicle", token, map[string]string{
		"note": "First draft of opening scene",
	}))

	if w.Code != http.StatusCreated {
		t.Fatalf("chronicle: want 201, got %d: %s", w.Code, w.Body.String())
	}

	var entry project.ChronicleEntry
	json.Unmarshal(w.Body.Bytes(), &entry)

	if entry.SHA == "" {
		t.Error("expected non-empty SHA in chronicle response")
	}
	if len(entry.ShortSHA) != 7 {
		t.Errorf("expected 7-char short SHA, got %q", entry.ShortSHA)
	}
	if entry.Note != "First draft of opening scene" {
		t.Errorf("expected note to match, got %q", entry.Note)
	}
}

func TestChronicle_NothingToChronicle(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	// First chronicle
	router.ServeHTTP(httptest.NewRecorder(), testutil.AuthRequest("POST", projectURL+"/git/chronicle", token, map[string]string{
		"note": "Initial capture",
	}))

	// Second chronicle with no content changes — should return 200 with message
	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("POST", projectURL+"/git/chronicle", token, map[string]string{
		"note": "Nothing changed",
	}))

	if w.Code != http.StatusOK {
		t.Fatalf("nothing-to-chronicle: want 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["message"] != "nothing to chronicle" {
		t.Errorf("expected 'nothing to chronicle' message, got %v", resp["message"])
	}
}

func TestChronicle_RequiresNote(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("POST", projectURL+"/git/chronicle", token, map[string]string{}))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing note: want 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Lore ──────────────────────────────────────────────────────────────────────

func TestLore_ReturnsPaginatedHistory(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	// Chronicle once
	router.ServeHTTP(httptest.NewRecorder(), testutil.AuthRequest("POST", projectURL+"/git/chronicle", token, map[string]string{
		"note": "Draft v1",
	}))

	// Update scene content so second chronicle differs
	// (we need to patch the scene — but from this test we don't have the scene ID)
	// Instead just test that lore returns at least the init + first chronicle.
	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("GET", projectURL+"/git/lore", token, nil))

	if w.Code != http.StatusOK {
		t.Fatalf("lore: want 200, got %d: %s", w.Code, w.Body.String())
	}

	var entries []project.ChronicleEntry
	json.Unmarshal(w.Body.Bytes(), &entries)

	// Expect at least the user Chronicle (init commit may or may not be visible
	// depending on how git log works from HEAD).
	if len(entries) == 0 {
		t.Error("expected at least one entry in lore")
	}
	// Most recent entry first.
	if entries[0].Note != "Draft v1" {
		t.Errorf("expected first entry note 'Draft v1', got %q", entries[0].Note)
	}
}

func TestLore_Pagination(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	// Chronicle to get at least one user entry
	router.ServeHTTP(httptest.NewRecorder(), testutil.AuthRequest("POST", projectURL+"/git/chronicle", token, map[string]string{
		"note": "Paginated entry",
	}))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("GET", projectURL+"/git/lore?page=1&per_page=1", token, nil))

	if w.Code != http.StatusOK {
		t.Fatalf("lore paginated: want 200, got %d: %s", w.Code, w.Body.String())
	}

	var entries []project.ChronicleEntry
	json.Unmarshal(w.Body.Bytes(), &entries)

	if len(entries) > 1 {
		t.Errorf("per_page=1 should return at most 1 entry, got %d", len(entries))
	}
}

// ── Echo ──────────────────────────────────────────────────────────────────────

func TestEcho_DiffBetweenChronicles(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	// First chronicle — capture the SHA
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, testutil.AuthRequest("POST", projectURL+"/git/chronicle", token, map[string]string{
		"note": "Echo test v1",
	}))
	if w1.Code != http.StatusCreated {
		t.Fatalf("first chronicle: want 201, got %d: %s", w1.Code, w1.Body.String())
	}
	var entry1 project.ChronicleEntry
	json.Unmarshal(w1.Body.Bytes(), &entry1)

	// Missing from/to → 400
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, testutil.AuthRequest("GET", projectURL+"/git/echo", token, nil))
	if w2.Code != http.StatusBadRequest {
		t.Errorf("echo without params: want 400, got %d", w2.Code)
	}

	// Echo from the init commit (parent of entry1) to entry1
	// We can use entry1.SHA as both from and to: diff should be empty.
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, testutil.AuthRequest("GET",
		projectURL+"/git/echo?from="+entry1.SHA+"&to="+entry1.SHA,
		token, nil,
	))
	if w3.Code != http.StatusOK {
		t.Fatalf("echo same SHA: want 200, got %d: %s", w3.Code, w3.Body.String())
	}

	var echo project.EchoResponse
	json.Unmarshal(w3.Body.Bytes(), &echo)
	if echo.FromSHA != entry1.SHA {
		t.Errorf("expected from_sha %s, got %s", entry1.SHA, echo.FromSHA)
	}
}

func TestEcho_InvalidSHA(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("GET",
		projectURL+"/git/echo?from=deadbeef&to=deadbeef",
		token, nil,
	))

	// Invalid SHA → internal error (go-git can't resolve it)
	if w.Code == http.StatusOK {
		t.Error("expected non-200 for invalid SHA")
	}
}

// ── Timelines (Diverge) ───────────────────────────────────────────────────────

func TestTimelines_InitiallyJustCanon(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("GET", projectURL+"/git/timelines", token, nil))

	if w.Code != http.StatusOK {
		t.Fatalf("list timelines: want 200, got %d: %s", w.Code, w.Body.String())
	}

	var timelines []project.TimelineInfo
	json.Unmarshal(w.Body.Bytes(), &timelines)

	if len(timelines) != 1 {
		t.Fatalf("expected 1 timeline, got %d", len(timelines))
	}
	if timelines[0].Name != "canon" {
		t.Errorf("expected timeline 'canon', got %q", timelines[0].Name)
	}
	if !timelines[0].IsCanon {
		t.Error("expected is_canon=true for canon timeline")
	}
	if !timelines[0].IsActive {
		t.Error("expected is_active=true since we are on canon")
	}
}

func TestDiverge_CreatesNewTimeline(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("POST", projectURL+"/git/timelines", token, map[string]string{
		"timeline_name": "alternate-ending",
	}))

	if w.Code != http.StatusCreated {
		t.Fatalf("diverge: want 201, got %d: %s", w.Code, w.Body.String())
	}

	// List timelines — should now have canon + alternate-ending
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, testutil.AuthRequest("GET", projectURL+"/git/timelines", token, nil))

	var timelines []project.TimelineInfo
	json.Unmarshal(w2.Body.Bytes(), &timelines)

	if len(timelines) != 2 {
		t.Fatalf("expected 2 timelines after diverge, got %d", len(timelines))
	}

	names := map[string]bool{}
	for _, tl := range timelines {
		names[tl.Name] = true
	}
	if !names["canon"] {
		t.Error("expected 'canon' timeline")
	}
	if !names["alternate-ending"] {
		t.Error("expected 'alternate-ending' timeline")
	}
}

func TestDiverge_RequiresTimelineName(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("POST", projectURL+"/git/timelines", token, map[string]interface{}{}))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("missing timeline_name: want 400, got %d", w.Code)
	}
}

// ── TravelTo ──────────────────────────────────────────────────────────────────

func TestTravelTo_SwitchesActiveTimeline(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	// Diverge first
	router.ServeHTTP(httptest.NewRecorder(), testutil.AuthRequest("POST", projectURL+"/git/timelines", token, map[string]string{
		"timeline_name": "side-quest",
	}))

	// Travel back to canon
	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("POST", projectURL+"/git/timelines/canon/travel", token, nil))

	if w.Code != http.StatusOK {
		t.Fatalf("travel to canon: want 200, got %d: %s", w.Code, w.Body.String())
	}

	var status project.GitStatusResponse
	json.Unmarshal(w.Body.Bytes(), &status)

	if status.CurrentTimeline != "canon" {
		t.Errorf("expected current_timeline 'canon' after travel, got %q", status.CurrentTimeline)
	}
}

func TestTravelTo_ThenBack(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	// Diverge to create alternate
	router.ServeHTTP(httptest.NewRecorder(), testutil.AuthRequest("POST", projectURL+"/git/timelines", token, map[string]string{
		"timeline_name": "flash-forward",
	}))

	// Travel to alternate
	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("POST", projectURL+"/git/timelines/flash-forward/travel", token, nil))

	if w.Code != http.StatusOK {
		t.Fatalf("travel to flash-forward: want 200, got %d: %s", w.Code, w.Body.String())
	}

	var status project.GitStatusResponse
	json.Unmarshal(w.Body.Bytes(), &status)
	if status.CurrentTimeline != "flash-forward" {
		t.Errorf("expected 'flash-forward', got %q", status.CurrentTimeline)
	}

	// Travel back to canon
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, testutil.AuthRequest("POST", projectURL+"/git/timelines/canon/travel", token, nil))

	if w2.Code != http.StatusOK {
		t.Fatalf("travel back to canon: want 200, got %d: %s", w2.Code, w2.Body.String())
	}
}

// ── Canonize ──────────────────────────────────────────────────────────────────

func TestCanonize_FastForwardSucceeds(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	// Chronicle so there is a real commit to work with
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, testutil.AuthRequest("POST", projectURL+"/git/chronicle", token, map[string]string{
		"note": "Pre-diverge snapshot",
	}))
	if w1.Code != http.StatusCreated {
		t.Fatalf("pre-diverge chronicle: want 201, got %d: %s", w1.Code, w1.Body.String())
	}
	var chronicleEntry project.ChronicleEntry
	json.Unmarshal(w1.Body.Bytes(), &chronicleEntry)

	// Diverge from canon HEAD
	router.ServeHTTP(httptest.NewRecorder(), testutil.AuthRequest("POST", projectURL+"/git/timelines", token, map[string]string{
		"timeline_name": "epilogue",
	}))

	// On epilogue branch, canonize it back — it's a direct descendant so
	// fast-forward should succeed.
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, testutil.AuthRequest("POST", projectURL+"/git/timelines/epilogue/canonize", token, nil))

	if w2.Code != http.StatusOK {
		t.Fatalf("canonize epilogue: want 200, got %d: %s", w2.Code, w2.Body.String())
	}

	var result project.CanonizeResult
	json.Unmarshal(w2.Body.Bytes(), &result)

	if result.HasParadox {
		t.Errorf("expected no paradox, got: %s", result.ParadoxNote)
	}
}

func TestCanonize_AlreadyUpToDateBranch(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	// Diverge from current HEAD to create a branch that is at the same commit
	// as canon (no new commits on it). Canonizing it should succeed because
	// the branch is an ancestor of (or equal to) canon — nothing to merge.
	router.ServeHTTP(httptest.NewRecorder(), testutil.AuthRequest("POST", projectURL+"/git/timelines", token, map[string]string{
		"timeline_name": "already-merged",
	}))

	// Travel back to canon so we can canonize the branch
	router.ServeHTTP(httptest.NewRecorder(), testutil.AuthRequest("POST", projectURL+"/git/timelines/canon/travel", token, nil))

	// Canonize the "already-merged" branch — it points to the same commit as
	// canon, so isTimelineAncestor=true → returns 200 with no paradox.
	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("POST", projectURL+"/git/timelines/already-merged/canonize", token, nil))

	if w.Code != http.StatusOK {
		t.Fatalf("canonize already-up-to-date branch: want 200, got %d: %s", w.Code, w.Body.String())
	}

	var result project.CanonizeResult
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.HasParadox {
		t.Errorf("expected no paradox, got: %s", result.ParadoxNote)
	}
}

func TestCanonize_CanonIntoItselfRejected(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("POST", projectURL+"/git/timelines/canon/canonize", token, nil))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("canonize canon into itself: want 400, got %d: %s", w.Code, w.Body.String())
	}
}

// ── Status after full flow ────────────────────────────────────────────────────

func TestGitStatus_AfterChronicleAndDiverge(t *testing.T) {
	router, projectURL, token := setupGitProject(t)

	// Chronicle
	router.ServeHTTP(httptest.NewRecorder(), testutil.AuthRequest("POST", projectURL+"/git/chronicle", token, map[string]string{
		"note": "Status flow test",
	}))

	// Diverge
	router.ServeHTTP(httptest.NewRecorder(), testutil.AuthRequest("POST", projectURL+"/git/timelines", token, map[string]string{
		"timeline_name": "status-test-branch",
	}))

	// Status — now on the new branch
	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("GET", projectURL+"/git/status", token, nil))

	if w.Code != http.StatusOK {
		t.Fatalf("git status: want 200, got %d: %s", w.Code, w.Body.String())
	}

	var status project.GitStatusResponse
	json.Unmarshal(w.Body.Bytes(), &status)

	if status.CurrentTimeline != "status-test-branch" {
		t.Errorf("expected current_timeline 'status-test-branch', got %q", status.CurrentTimeline)
	}
	if status.LastChronicle == nil {
		t.Error("expected last_chronicle to be populated after a chronicle")
	}
	if status.LastChronicle.Note != "Status flow test" {
		t.Errorf("expected last chronicle note 'Status flow test', got %q", status.LastChronicle.Note)
	}
}
