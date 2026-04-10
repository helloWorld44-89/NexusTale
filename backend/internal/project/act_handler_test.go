package project_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jconder44/nexustale/internal/project"
	"github.com/jconder44/nexustale/internal/testutil"
)

func TestActCRUD(t *testing.T) {
	router, _, _ := testutil.SetupRouter(t)
	token := registerAndGetToken(t, router, "actcrud@example.com")

	proj := createProject(t, router, token, "Act CRUD Project")
	actsURL := "/api/v1/projects/" + proj.ID.String() + "/acts"

	// Create a second act (default Act 1 already exists)
	createBody := map[string]interface{}{
		"title":      "Act 2: The Reckoning",
		"summary":    "The heroes face the consequences.",
		"sort_order": 1,
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", actsURL, token, createBody))

	if w.Code != http.StatusCreated {
		t.Fatalf("create act: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var act project.ActResponse
	json.Unmarshal(w.Body.Bytes(), &act)

	if act.Title != "Act 2: The Reckoning" {
		t.Errorf("expected 'Act 2: The Reckoning', got '%s'", act.Title)
	}
	if act.Summary != "The heroes face the consequences." {
		t.Errorf("expected summary, got '%s'", act.Summary)
	}
	if act.ProjectID != proj.ID {
		t.Errorf("expected project_id %s, got %s", proj.ID, act.ProjectID)
	}

	actURL := actsURL + "/" + act.ID.String()

	// Get act
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, authRequest("GET", actURL, token, nil))

	if w2.Code != http.StatusOK {
		t.Fatalf("get act: expected 200, got %d: %s", w2.Code, w2.Body.String())
	}

	var fetched project.ActResponse
	json.Unmarshal(w2.Body.Bytes(), &fetched)
	if fetched.ID != act.ID {
		t.Errorf("get act: ID mismatch")
	}

	// List acts — should include the auto-created default plus the one we created
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, authRequest("GET", actsURL, token, nil))

	if w3.Code != http.StatusOK {
		t.Fatalf("list acts: expected 200, got %d", w3.Code)
	}

	var acts []project.ActResponse
	json.Unmarshal(w3.Body.Bytes(), &acts)
	if len(acts) < 2 {
		t.Errorf("expected at least 2 acts (default + created), got %d", len(acts))
	}

	// Update act
	updateBody := map[string]interface{}{
		"title":   "Act 2: Consequences",
		"summary": "Updated summary.",
	}
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, authRequest("PATCH", actURL, token, updateBody))

	if w4.Code != http.StatusOK {
		t.Fatalf("update act: expected 200, got %d: %s", w4.Code, w4.Body.String())
	}

	var updated project.ActResponse
	json.Unmarshal(w4.Body.Bytes(), &updated)
	if updated.Title != "Act 2: Consequences" {
		t.Errorf("expected updated title, got '%s'", updated.Title)
	}
	if updated.Summary != "Updated summary." {
		t.Errorf("expected updated summary, got '%s'", updated.Summary)
	}

	// Delete act
	w5 := httptest.NewRecorder()
	router.ServeHTTP(w5, authRequest("DELETE", actURL, token, nil))

	if w5.Code != http.StatusOK {
		t.Fatalf("delete act: expected 200, got %d", w5.Code)
	}

	// Verify act is gone
	w6 := httptest.NewRecorder()
	router.ServeHTTP(w6, authRequest("GET", actURL, token, nil))
	if w6.Code != http.StatusNotFound {
		t.Errorf("expected 404 after deletion, got %d", w6.Code)
	}
}

func TestActDefaultCreatedWithProject(t *testing.T) {
	router, _, _ := testutil.SetupRouter(t)
	token := registerAndGetToken(t, router, "actdefault@example.com")

	proj := createProject(t, router, token, "Default Act Project")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/v1/projects/"+proj.ID.String()+"/acts", token, nil))

	if w.Code != http.StatusOK {
		t.Fatalf("list acts: expected 200, got %d", w.Code)
	}

	var acts []project.ActResponse
	json.Unmarshal(w.Body.Bytes(), &acts)

	if len(acts) != 1 {
		t.Fatalf("expected exactly 1 default act, got %d", len(acts))
	}
	if acts[0].Title != "Act 1" {
		t.Errorf("expected default title 'Act 1', got '%s'", acts[0].Title)
	}
	if acts[0].SortOrder != 0 {
		t.Errorf("expected sort_order 0, got %d", acts[0].SortOrder)
	}
	if acts[0].ProjectID != proj.ID {
		t.Errorf("expected project_id to match")
	}
}

func TestActCascadeDeletesChaptersAndScenes(t *testing.T) {
	router, _, _ := testutil.SetupRouter(t)
	token := registerAndGetToken(t, router, "actcascade@example.com")

	proj := createProject(t, router, token, "Cascade Test Project")
	actID := defaultActID(t, router, token, proj.ID.String())
	actURL := "/api/v1/projects/" + proj.ID.String() + "/acts/" + actID

	// Create a chapter under the default act
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", actChapterURL(proj.ID.String(), actID), token,
		map[string]interface{}{"title": "Doomed Chapter", "sort_order": 1}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create chapter: %d: %s", w.Code, w.Body.String())
	}
	var ch project.ChapterResponse
	json.Unmarshal(w.Body.Bytes(), &ch)

	// Create a scene under that chapter
	scenesURL := "/api/v1/chapters/" + ch.ID.String() + "/scenes"
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, authRequest("POST", scenesURL, token,
		map[string]interface{}{"title": "Doomed Scene", "sort_order": 1}))
	if w2.Code != http.StatusCreated {
		t.Fatalf("create scene: %d: %s", w2.Code, w2.Body.String())
	}
	var sc project.SceneResponse
	json.Unmarshal(w2.Body.Bytes(), &sc)

	// Delete the act
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, authRequest("DELETE", actURL, token, nil))
	if w3.Code != http.StatusOK {
		t.Fatalf("delete act: expected 200, got %d", w3.Code)
	}

	// Chapter should be gone (cascaded)
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, authRequest("GET", "/api/v1/projects/"+proj.ID.String()+"/acts/"+actID+"/chapters/"+ch.ID.String(), token, nil))
	if w4.Code != http.StatusNotFound {
		t.Errorf("expected chapter to be cascade-deleted (404), got %d", w4.Code)
	}

	// Scene should be gone (cascaded through chapter)
	w5 := httptest.NewRecorder()
	router.ServeHTTP(w5, authRequest("GET", scenesURL+"/"+sc.ID.String(), token, nil))
	if w5.Code != http.StatusNotFound {
		t.Errorf("expected scene to be cascade-deleted (404), got %d", w5.Code)
	}
}

func TestActValidation(t *testing.T) {
	router, _, _ := testutil.SetupRouter(t)
	token := registerAndGetToken(t, router, "actvalidation@example.com")

	proj := createProject(t, router, token, "Validation Project")
	actsURL := "/api/v1/projects/" + proj.ID.String() + "/acts"

	// Empty title should be rejected
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", actsURL, token, map[string]interface{}{"title": ""}))
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for empty title, got %d", w.Code)
	}

	// Missing title should be rejected
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, authRequest("POST", actsURL, token, map[string]interface{}{"summary": "no title"}))
	if w2.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing title, got %d", w2.Code)
	}
}

func TestGetActNotFound(t *testing.T) {
	router, _, _ := testutil.SetupRouter(t)
	token := registerAndGetToken(t, router, "actnotfound@example.com")

	proj := createProject(t, router, token, "Not Found Project")
	fakeID := "00000000-0000-0000-0000-000000000000"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("GET", "/api/v1/projects/"+proj.ID.String()+"/acts/"+fakeID, token, nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-existent act, got %d", w.Code)
	}
}
