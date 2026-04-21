package research_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jconder44/nexustale/internal/research"
	"github.com/jconder44/nexustale/internal/testutil"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func serve(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, method, url, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest(method, url, token, body))
	return w
}

func createResearchProject(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, token string) string {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest("POST", "/api/v1/projects", token, map[string]interface{}{
		"title": "Research Test Project",
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create project: got %d: %s", w.Code, w.Body.String())
	}
	var p struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &p)
	return p.ID
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestResearchNoteCRUD(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok := testutil.RegisterAndGetToken(t, r, "research-crud@example.com")
	projID := createResearchProject(t, r, tok)
	base := "/api/v1/projects/" + projID + "/research-notes"

	// List — initially empty
	w := serve(t, r, "GET", base, tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d", w.Code)
	}
	var list []research.NoteResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d notes", len(list))
	}

	// Create
	w = serve(t, r, "POST", base, tok, map[string]interface{}{
		"title":      "Arcane History",
		"body":       "Notes on the empire's founding",
		"source_url": "https://example.com/ref",
		"tags":       []string{"world", "history"},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d: %s", w.Code, w.Body.String())
	}
	var note research.NoteResponse
	json.Unmarshal(w.Body.Bytes(), &note)
	if note.Title != "Arcane History" {
		t.Errorf("title: got %q", note.Title)
	}
	if note.SourceURL != "https://example.com/ref" {
		t.Errorf("source_url: got %q", note.SourceURL)
	}
	if len(note.Tags) != 2 || note.Tags[0] != "world" {
		t.Errorf("tags: got %v", note.Tags)
	}

	// Get
	w = serve(t, r, "GET", base+"/"+note.ID, tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("get: want 200, got %d", w.Code)
	}

	// List — now has one
	w = serve(t, r, "GET", base, tok, nil)
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list after create: want 1, got %d", len(list))
	}

	// Update — only title
	w = serve(t, r, "PATCH", base+"/"+note.ID, tok, map[string]interface{}{
		"title": "Updated Title",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var updated research.NoteResponse
	json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.Title != "Updated Title" {
		t.Errorf("updated title: got %q", updated.Title)
	}
	if updated.Body != note.Body {
		t.Errorf("body should not change on title-only patch; got %q", updated.Body)
	}

	// Delete
	w = serve(t, r, "DELETE", base+"/"+note.ID, tok, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d", w.Code)
	}

	// Not found after delete
	w = serve(t, r, "GET", base+"/"+note.ID, tok, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("get after delete: want 404, got %d", w.Code)
	}
}

func TestResearchNoteDefaultTitle(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok := testutil.RegisterAndGetToken(t, r, "research-title@example.com")
	projID := createResearchProject(t, r, tok)
	base := "/api/v1/projects/" + projID + "/research-notes"

	// No title provided → service defaults to "Untitled Note"
	w := serve(t, r, "POST", base, tok, map[string]interface{}{
		"body": "some research",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d: %s", w.Code, w.Body.String())
	}
	var note research.NoteResponse
	json.Unmarshal(w.Body.Bytes(), &note)
	if note.Title != "Untitled Note" {
		t.Errorf("default title: want %q, got %q", "Untitled Note", note.Title)
	}
}

func TestResearchNoteTags(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok := testutil.RegisterAndGetToken(t, r, "research-tags@example.com")
	projID := createResearchProject(t, r, tok)
	base := "/api/v1/projects/" + projID + "/research-notes"

	// Empty tags array should roundtrip as empty, not null
	w := serve(t, r, "POST", base, tok, map[string]interface{}{
		"title": "No Tags",
		"body":  "body",
		"tags":  []string{},
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d", w.Code)
	}
	var note research.NoteResponse
	json.Unmarshal(w.Body.Bytes(), &note)
	if note.Tags == nil {
		t.Error("tags should not be nil even when empty")
	}

	// Update tags via PATCH
	w = serve(t, r, "PATCH", base+"/"+note.ID, tok, map[string]interface{}{
		"tags": []string{"magic", "lore", "history"},
	})
	if w.Code != http.StatusOK {
		t.Fatalf("patch tags: want 200, got %d", w.Code)
	}
	var updated research.NoteResponse
	json.Unmarshal(w.Body.Bytes(), &updated)
	if len(updated.Tags) != 3 {
		t.Errorf("tags after patch: want 3, got %d", len(updated.Tags))
	}
}

func TestResearchNoteUnauthenticated(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok := testutil.RegisterAndGetToken(t, r, "research-auth@example.com")
	projID := createResearchProject(t, r, tok)
	base := "/api/v1/projects/" + projID + "/research-notes"

	for _, method := range []string{"GET", "POST"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(method, base, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s without token: want 401, got %d", method, w.Code)
		}
	}
}

func TestResearchNoteIsolatedByProject(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok := testutil.RegisterAndGetToken(t, r, "research-iso@example.com")
	proj1 := createResearchProject(t, r, tok)
	proj2 := createResearchProject(t, r, tok)

	// Create note in project 1
	serve(t, r, "POST", "/api/v1/projects/"+proj1+"/research-notes", tok, map[string]interface{}{
		"title": "Project 1 Note",
		"body":  "belongs to project 1",
	})

	// Project 2 should have no notes
	w := serve(t, r, "GET", "/api/v1/projects/"+proj2+"/research-notes", tok, nil)
	var list []research.NoteResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Errorf("project 2 should have 0 notes, got %d", len(list))
	}
}
