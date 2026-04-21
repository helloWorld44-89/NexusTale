package annotations_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jconder44/nexustale/internal/annotations"
	"github.com/jconder44/nexustale/internal/testutil"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func serve(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, method, url, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest(method, url, token, body))
	return w
}

// setupScene creates a project → default act → chapter → scene and returns
// (projectID, sceneID) so annotation tests have a real scene to annotate.
func setupScene(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, token string) (projectID, sceneID string) {
	t.Helper()

	// Project
	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest("POST", "/api/v1/projects", token, map[string]interface{}{
		"title": "Annotation Test Project",
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create project: %d: %s", w.Code, w.Body.String())
	}
	var proj struct {
		ID string `json:"id"`
	}
	json.Unmarshal(w.Body.Bytes(), &proj)
	projectID = proj.ID

	// Default act
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, testutil.AuthRequest("GET", "/api/v1/projects/"+projectID+"/acts", token, nil))
	var acts []struct{ ID string `json:"id"` }
	json.Unmarshal(w2.Body.Bytes(), &acts)
	actID := acts[0].ID

	// Chapter
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, testutil.AuthRequest("POST", "/api/v1/projects/"+projectID+"/acts/"+actID+"/chapters", token, map[string]interface{}{
		"title":      "Chapter One",
		"sort_order": 1,
	}))
	if w3.Code != http.StatusCreated {
		t.Fatalf("create chapter: %d: %s", w3.Code, w3.Body.String())
	}
	var ch struct{ ID string `json:"id"` }
	json.Unmarshal(w3.Body.Bytes(), &ch)

	// Scene
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, testutil.AuthRequest("POST", "/api/v1/chapters/"+ch.ID+"/scenes", token, map[string]interface{}{
		"title":      "Opening Scene",
		"content":    "The stars were silent across the void.",
		"sort_order": 1,
	}))
	if w4.Code != http.StatusCreated {
		t.Fatalf("create scene: %d: %s", w4.Code, w4.Body.String())
	}
	var sc struct{ ID string `json:"id"` }
	json.Unmarshal(w4.Body.Bytes(), &sc)
	sceneID = sc.ID
	return
}

func annotationBase(projectID, sceneID string) string {
	return "/api/v1/projects/" + projectID + "/scenes/" + sceneID + "/annotations"
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestAnnotationCRUD(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok := testutil.RegisterAndGetToken(t, r, "ann-crud@example.com")
	projID, sceneID := setupScene(t, r, tok)
	base := annotationBase(projID, sceneID)

	// List — initially empty
	w := serve(t, r, "GET", base, tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d", w.Code)
	}
	var list []annotations.AnnotationResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatalf("expected empty list, got %d", len(list))
	}

	// Create
	w = serve(t, r, "POST", base, tok, map[string]interface{}{
		"start_char": 4,
		"end_char":   10,
		"body":       "What do stars mean here?",
		"type":       "question",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d: %s", w.Code, w.Body.String())
	}
	var ann annotations.AnnotationResponse
	json.Unmarshal(w.Body.Bytes(), &ann)
	if ann.Body != "What do stars mean here?" {
		t.Errorf("body: got %q", ann.Body)
	}
	if ann.Type != "question" {
		t.Errorf("type: got %q", ann.Type)
	}
	if ann.StartChar != 4 || ann.EndChar != 10 {
		t.Errorf("chars: got %d–%d", ann.StartChar, ann.EndChar)
	}
	if ann.AuthorName == "" {
		t.Error("author_name should be populated")
	}
	if ann.Resolved {
		t.Error("new annotation should not be resolved")
	}

	// List — now has one
	w = serve(t, r, "GET", base, tok, nil)
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list after create: want 1, got %d", len(list))
	}

	// Delete
	w = serve(t, r, "DELETE", base+"/"+ann.ID, tok, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: want 204, got %d", w.Code)
	}

	// Gone after delete
	w = serve(t, r, "GET", base, tok, nil)
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatalf("list after delete: want 0, got %d", len(list))
	}
}

func TestAnnotationTypes(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok := testutil.RegisterAndGetToken(t, r, "ann-types@example.com")
	projID, sceneID := setupScene(t, r, tok)
	base := annotationBase(projID, sceneID)

	types := []string{"note", "suggestion", "question"}
	for _, annType := range types {
		w := serve(t, r, "POST", base, tok, map[string]interface{}{
			"start_char": 0,
			"end_char":   5,
			"body":       "test " + annType,
			"type":       annType,
		})
		if w.Code != http.StatusCreated {
			t.Errorf("type %q: want 201, got %d: %s", annType, w.Code, w.Body.String())
			continue
		}
		var ann annotations.AnnotationResponse
		json.Unmarshal(w.Body.Bytes(), &ann)
		if ann.Type != annType {
			t.Errorf("type %q: roundtripped as %q", annType, ann.Type)
		}
	}
}

func TestAnnotationDefaultType(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok := testutil.RegisterAndGetToken(t, r, "ann-default-type@example.com")
	projID, sceneID := setupScene(t, r, tok)
	base := annotationBase(projID, sceneID)

	// No type in request → defaults to "note"
	w := serve(t, r, "POST", base, tok, map[string]interface{}{
		"start_char": 0,
		"end_char":   3,
		"body":       "interesting word",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d: %s", w.Code, w.Body.String())
	}
	var ann annotations.AnnotationResponse
	json.Unmarshal(w.Body.Bytes(), &ann)
	if ann.Type != "note" {
		t.Errorf("default type: want %q, got %q", "note", ann.Type)
	}
}

func TestAnnotationResolve(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok := testutil.RegisterAndGetToken(t, r, "ann-resolve@example.com")
	projID, sceneID := setupScene(t, r, tok)
	base := annotationBase(projID, sceneID)

	// Create an annotation
	w := serve(t, r, "POST", base, tok, map[string]interface{}{
		"start_char": 0,
		"end_char":   5,
		"body":       "fix this phrasing",
		"type":       "suggestion",
	})
	var ann annotations.AnnotationResponse
	json.Unmarshal(w.Body.Bytes(), &ann)

	// Resolve it
	w = serve(t, r, "PUT", base+"/"+ann.ID, tok, map[string]interface{}{
		"resolved": true,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("resolve: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var resolved annotations.AnnotationResponse
	json.Unmarshal(w.Body.Bytes(), &resolved)
	if !resolved.Resolved {
		t.Error("annotation should be resolved after PUT {resolved:true}")
	}
	if resolved.ResolvedBy == nil {
		t.Error("resolved_by should be populated after resolve")
	}

	// Still in list (resolved annotations are still returned)
	w = serve(t, r, "GET", base, tok, nil)
	var list []annotations.AnnotationResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("list after resolve: want 1, got %d", len(list))
	}
	if !list[0].Resolved {
		t.Error("annotation in list should show resolved=true")
	}
}

func TestAnnotationUpdateBody(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok := testutil.RegisterAndGetToken(t, r, "ann-update@example.com")
	projID, sceneID := setupScene(t, r, tok)
	base := annotationBase(projID, sceneID)

	w := serve(t, r, "POST", base, tok, map[string]interface{}{
		"start_char": 0,
		"end_char":   5,
		"body":       "original note",
	})
	var ann annotations.AnnotationResponse
	json.Unmarshal(w.Body.Bytes(), &ann)

	w = serve(t, r, "PUT", base+"/"+ann.ID, tok, map[string]interface{}{
		"body": "revised note",
	})
	if w.Code != http.StatusOK {
		t.Fatalf("update body: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var updated annotations.AnnotationResponse
	json.Unmarshal(w.Body.Bytes(), &updated)
	if updated.Body != "revised note" {
		t.Errorf("body after update: got %q", updated.Body)
	}
}

func TestAnnotationMissingBody(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok := testutil.RegisterAndGetToken(t, r, "ann-validation@example.com")
	projID, sceneID := setupScene(t, r, tok)
	base := annotationBase(projID, sceneID)

	// Missing body → 400
	w := serve(t, r, "POST", base, tok, map[string]interface{}{
		"start_char": 0,
		"end_char":   5,
	})
	if w.Code != http.StatusBadRequest {
		t.Errorf("missing body: want 400, got %d", w.Code)
	}
}

func TestAnnotationUnauthenticated(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok := testutil.RegisterAndGetToken(t, r, "ann-auth@example.com")
	projID, sceneID := setupScene(t, r, tok)
	base := annotationBase(projID, sceneID)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", base, nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("GET without token: want 401, got %d", w.Code)
	}
}

func TestAnnotationListIsolatedByScene(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok := testutil.RegisterAndGetToken(t, r, "ann-iso@example.com")
	projID, scene1 := setupScene(t, r, tok)
	_, scene2 := setupScene(t, r, tok)

	// Annotate scene 1
	serve(t, r, "POST", annotationBase(projID, scene1), tok, map[string]interface{}{
		"start_char": 0, "end_char": 5, "body": "for scene 1",
	})

	// Scene 2 should have no annotations
	w := serve(t, r, "GET", annotationBase(projID, scene2), tok, nil)
	var list []annotations.AnnotationResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Errorf("scene 2 should have 0 annotations, got %d", len(list))
	}
}

func TestAnnotationListSortedByStartChar(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok := testutil.RegisterAndGetToken(t, r, "ann-sort@example.com")
	projID, sceneID := setupScene(t, r, tok)
	base := annotationBase(projID, sceneID)

	// Create out of order
	serve(t, r, "POST", base, tok, map[string]interface{}{"start_char": 20, "end_char": 25, "body": "second"})
	serve(t, r, "POST", base, tok, map[string]interface{}{"start_char": 0, "end_char": 5, "body": "first"})
	serve(t, r, "POST", base, tok, map[string]interface{}{"start_char": 10, "end_char": 15, "body": "middle"})

	w := serve(t, r, "GET", base, tok, nil)
	var list []annotations.AnnotationResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 3 {
		t.Fatalf("want 3 annotations, got %d", len(list))
	}
	if list[0].StartChar != 0 || list[1].StartChar != 10 || list[2].StartChar != 20 {
		t.Errorf("not sorted by start_char: %d, %d, %d", list[0].StartChar, list[1].StartChar, list[2].StartChar)
	}
}
