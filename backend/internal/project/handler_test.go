package project_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/internal/project"
	"github.com/jconder44/nexustale/internal/testutil"
)

func registerAndGetToken(t *testing.T, router *gin.Engine, email string) string {
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

func authRequest(method, url, token string, body interface{}) *http.Request {
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

func TestProjectCRUD(t *testing.T) {
	router, _, _ := testutil.SetupRouter(t)
	token := registerAndGetToken(t, router, "projcrud@example.com")

	// Create project
	createBody := map[string]interface{}{
		"title":       "My Novel",
		"description": "A test novel",
		"genres":      []string{"fantasy", "adventure"},
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/v1/projects", token, createBody))

	if w.Code != http.StatusCreated {
		t.Fatalf("create project: expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var created project.ProjectResponse
	json.Unmarshal(w.Body.Bytes(), &created)

	if created.Title != "My Novel" {
		t.Errorf("expected title 'My Novel', got '%s'", created.Title)
	}

	// Get project
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, authRequest("GET", "/api/v1/projects/"+created.ID.String(), token, nil))

	if w2.Code != http.StatusOK {
		t.Fatalf("get project: expected 200, got %d", w2.Code)
	}

	// List projects
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, authRequest("GET", "/api/v1/projects", token, nil))

	if w3.Code != http.StatusOK {
		t.Fatalf("list projects: expected 200, got %d", w3.Code)
	}

	var projects []project.ProjectResponse
	json.Unmarshal(w3.Body.Bytes(), &projects)
	if len(projects) == 0 {
		t.Error("expected at least 1 project in list")
	}

	// Update project
	updateBody := map[string]string{"title": "Updated Novel"}
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, authRequest("PATCH", "/api/v1/projects/"+created.ID.String(), token, updateBody))

	if w4.Code != http.StatusOK {
		t.Fatalf("update project: expected 200, got %d: %s", w4.Code, w4.Body.String())
	}

	var updated project.ProjectResponse
	json.Unmarshal(w4.Body.Bytes(), &updated)
	if updated.Title != "Updated Novel" {
		t.Errorf("expected title 'Updated Novel', got '%s'", updated.Title)
	}

	// Delete project
	w5 := httptest.NewRecorder()
	router.ServeHTTP(w5, authRequest("DELETE", "/api/v1/projects/"+created.ID.String(), token, nil))

	if w5.Code != http.StatusOK {
		t.Fatalf("delete project: expected 200, got %d", w5.Code)
	}
}

func TestChapterCRUD(t *testing.T) {
	router, _, _ := testutil.SetupRouter(t)
	token := registerAndGetToken(t, router, "chaptercrud@example.com")

	// Create project first
	projBody := map[string]interface{}{"title": "Chapter Test Project", "description": ""}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/v1/projects", token, projBody))
	var proj project.ProjectResponse
	json.Unmarshal(w.Body.Bytes(), &proj)
	projectURL := "/api/v1/projects/" + proj.ID.String()

	// Create chapter
	chBody := map[string]interface{}{
		"title":      "Chapter One",
		"summary":    "The beginning",
		"sort_order": 1,
	}
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, authRequest("POST", projectURL+"/chapters", token, chBody))

	if w2.Code != http.StatusCreated {
		t.Fatalf("create chapter: expected 201, got %d: %s", w2.Code, w2.Body.String())
	}

	var chapter project.ChapterResponse
	json.Unmarshal(w2.Body.Bytes(), &chapter)

	if chapter.Title != "Chapter One" {
		t.Errorf("expected 'Chapter One', got '%s'", chapter.Title)
	}

	// List chapters
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, authRequest("GET", projectURL+"/chapters", token, nil))

	if w3.Code != http.StatusOK {
		t.Fatalf("list chapters: expected 200, got %d", w3.Code)
	}

	// Get chapter
	chapterURL := projectURL + "/chapters/" + chapter.ID.String()
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, authRequest("GET", chapterURL, token, nil))

	if w4.Code != http.StatusOK {
		t.Fatalf("get chapter: expected 200, got %d", w4.Code)
	}

	// Update chapter
	updateBody := map[string]string{"title": "Chapter One - Revised"}
	w5 := httptest.NewRecorder()
	router.ServeHTTP(w5, authRequest("PATCH", chapterURL, token, updateBody))

	if w5.Code != http.StatusOK {
		t.Fatalf("update chapter: expected 200, got %d: %s", w5.Code, w5.Body.String())
	}

	// Delete chapter
	w6 := httptest.NewRecorder()
	router.ServeHTTP(w6, authRequest("DELETE", chapterURL, token, nil))

	if w6.Code != http.StatusOK {
		t.Fatalf("delete chapter: expected 200, got %d", w6.Code)
	}
}

func TestSceneCRUD(t *testing.T) {
	router, _, _ := testutil.SetupRouter(t)
	token := registerAndGetToken(t, router, "scenecrud@example.com")

	// Create project
	w := httptest.NewRecorder()
	router.ServeHTTP(w, authRequest("POST", "/api/v1/projects", token, map[string]interface{}{"title": "Scene Test", "description": ""}))
	var proj project.ProjectResponse
	json.Unmarshal(w.Body.Bytes(), &proj)

	// Create chapter
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, authRequest("POST", "/api/v1/projects/"+proj.ID.String()+"/chapters", token,
		map[string]interface{}{"title": "Ch1", "sort_order": 1}))
	var ch project.ChapterResponse
	json.Unmarshal(w2.Body.Bytes(), &ch)

	chapterURL := "/api/v1/projects/" + proj.ID.String() + "/chapters/" + ch.ID.String()

	// Create scene
	sceneBody := map[string]interface{}{
		"title":      "Opening Scene",
		"content":    "It was a dark and stormy night.",
		"pov":        "third",
		"tense":      "past",
		"tags":       []string{"opening", "weather"},
		"sort_order": 1,
	}
	w3 := httptest.NewRecorder()
	router.ServeHTTP(w3, authRequest("POST", chapterURL+"/scenes", token, sceneBody))

	if w3.Code != http.StatusCreated {
		t.Fatalf("create scene: expected 201, got %d: %s", w3.Code, w3.Body.String())
	}

	var scene project.SceneResponse
	json.Unmarshal(w3.Body.Bytes(), &scene)

	if scene.Title != "Opening Scene" {
		t.Errorf("expected 'Opening Scene', got '%s'", scene.Title)
	}
	if scene.Content != "It was a dark and stormy night." {
		t.Errorf("unexpected content: %s", scene.Content)
	}

	sceneURL := chapterURL + "/scenes/" + scene.ID.String()

	// Get scene
	w4 := httptest.NewRecorder()
	router.ServeHTTP(w4, authRequest("GET", sceneURL, token, nil))

	if w4.Code != http.StatusOK {
		t.Fatalf("get scene: expected 200, got %d", w4.Code)
	}

	// List scenes
	w5 := httptest.NewRecorder()
	router.ServeHTTP(w5, authRequest("GET", chapterURL+"/scenes", token, nil))

	if w5.Code != http.StatusOK {
		t.Fatalf("list scenes: expected 200, got %d", w5.Code)
	}

	// Update scene
	updateBody := map[string]string{"content": "The sun rose over the mountains."}
	w6 := httptest.NewRecorder()
	router.ServeHTTP(w6, authRequest("PATCH", sceneURL, token, updateBody))

	if w6.Code != http.StatusOK {
		t.Fatalf("update scene: expected 200, got %d: %s", w6.Code, w6.Body.String())
	}

	var updatedScene project.SceneResponse
	json.Unmarshal(w6.Body.Bytes(), &updatedScene)
	if updatedScene.Content != "The sun rose over the mountains." {
		t.Errorf("expected updated content, got '%s'", updatedScene.Content)
	}

	// Delete scene
	w7 := httptest.NewRecorder()
	router.ServeHTTP(w7, authRequest("DELETE", sceneURL, token, nil))

	if w7.Code != http.StatusOK {
		t.Fatalf("delete scene: expected 200, got %d", w7.Code)
	}
}

func TestUnauthenticatedAccess(t *testing.T) {
	router, _, _ := testutil.SetupRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/projects", nil)
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated request, got %d", w.Code)
	}
}
