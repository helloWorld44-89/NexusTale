package maps_test

// handler_test.go — integration tests for D-Map M1 (data model + git
// plumbing). Runs against a real DB + git working tree; skipped
// automatically when neither is reachable (see testutil.SetupTestDB).
//
// Run: make test (serial, -p 1 to avoid cross-package DB races)

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jconder44/nexustale/internal/project"
	"github.com/jconder44/nexustale/internal/testutil"
)

// setupMapProject creates a project and returns the router, its base URL,
// and an access token — mirrors internal/project/git_handler_test.go's
// setupGitProject helper.
func setupMapProject(t *testing.T) (router interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, projectURL, token string) {
	t.Helper()

	r, _, _ := testutil.SetupRouterWithGit(t)
	tok := testutil.RegisterAndGetToken(t, r, "maps-test-"+t.Name()+"@example.com")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest("POST", "/api/v1/projects", tok, map[string]interface{}{
		"title": "Map Test Project",
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create project: want 201, got %d: %s", w.Code, w.Body.String())
	}
	var proj project.ProjectResponse
	json.Unmarshal(w.Body.Bytes(), &proj)

	return r, "/api/v1/projects/" + proj.ID.String(), tok
}

type mapResp struct {
	ID             string          `json:"id"`
	ProjectID      string          `json:"project_id"`
	ParentEntityID *string         `json:"parent_entity_id"`
	Name           string          `json:"name"`
	MapType        string          `json:"map_type"`
	Layout         json.RawMessage `json:"layout"`
}

func createMap(t *testing.T, router interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, projectURL, token, name, mapType string) mapResp {
	t.Helper()
	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("POST", projectURL+"/maps", token, map[string]interface{}{
		"name":     name,
		"map_type": mapType,
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create map: want 201, got %d: %s", w.Code, w.Body.String())
	}
	var m mapResp
	json.Unmarshal(w.Body.Bytes(), &m)
	return m
}

func TestCreateMap_DefaultsAndLayout(t *testing.T) {
	router, projectURL, token := setupMapProject(t)

	m := createMap(t, router, projectURL, token, "World Map", "world")

	if m.Name != "World Map" || m.MapType != "world" {
		t.Errorf("unexpected map: %+v", m)
	}
	if string(m.Layout) != `{"regions":[],"symbols":[]}` {
		t.Errorf("expected default empty layout, got %s", m.Layout)
	}
	if m.ParentEntityID != nil {
		t.Errorf("expected no parent, got %v", *m.ParentEntityID)
	}
}

func TestCreateMap_WithParent(t *testing.T) {
	router, projectURL, token := setupMapProject(t)

	world := createMap(t, router, projectURL, token, "World Map", "world")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("POST", projectURL+"/maps", token, map[string]interface{}{
		"name":             "Region Map",
		"map_type":         "region",
		"parent_entity_id": world.ID,
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create child map: want 201, got %d: %s", w.Code, w.Body.String())
	}
	var region mapResp
	json.Unmarshal(w.Body.Bytes(), &region)
	if region.ParentEntityID == nil || *region.ParentEntityID != world.ID {
		t.Errorf("expected parent_entity_id %s, got %v", world.ID, region.ParentEntityID)
	}
}

func TestCreateMap_InvalidMapType(t *testing.T) {
	router, projectURL, token := setupMapProject(t)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("POST", projectURL+"/maps", token, map[string]interface{}{
		"name":     "Bad Map",
		"map_type": "underwater",
	}))
	if w.Code != http.StatusBadRequest {
		t.Errorf("invalid map_type: want 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetMap_LayoutRoundTrips(t *testing.T) {
	router, projectURL, token := setupMapProject(t)
	m := createMap(t, router, projectURL, token, "City Map", "city")

	newLayout := `{"regions":[{"id":"r1","type":"landmass"}],"symbols":[]}`
	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("PUT", projectURL+"/maps/"+m.ID, token, map[string]interface{}{
		"layout": json.RawMessage(newLayout),
	}))
	if w.Code != http.StatusOK {
		t.Fatalf("update layout: want 200, got %d: %s", w.Code, w.Body.String())
	}

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, testutil.AuthRequest("GET", projectURL+"/maps/"+m.ID, token, nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("get map: want 200, got %d: %s", w2.Code, w2.Body.String())
	}
	var got mapResp
	json.Unmarshal(w2.Body.Bytes(), &got)
	if string(got.Layout) != newLayout {
		t.Errorf("layout did not round-trip: want %s, got %s", newLayout, got.Layout)
	}
}

func TestListMaps_OmitsLayout(t *testing.T) {
	router, projectURL, token := setupMapProject(t)
	createMap(t, router, projectURL, token, "Galaxy Map", "galaxy")
	createMap(t, router, projectURL, token, "Planet Map", "planet")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("GET", projectURL+"/maps", token, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("list maps: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var list []mapResp
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 2 {
		t.Fatalf("expected 2 maps, got %d", len(list))
	}
	for _, m := range list {
		if len(m.Layout) != 0 {
			t.Errorf("expected layout omitted in list response, got %s", m.Layout)
		}
	}
}

func TestUpdateMap_NameAndType(t *testing.T) {
	router, projectURL, token := setupMapProject(t)
	m := createMap(t, router, projectURL, token, "Old Name", "world")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("PUT", projectURL+"/maps/"+m.ID, token, map[string]interface{}{
		"name":     "New Name",
		"map_type": "region",
	}))
	if w.Code != http.StatusOK {
		t.Fatalf("update map: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var got mapResp
	json.Unmarshal(w.Body.Bytes(), &got)
	if got.Name != "New Name" || got.MapType != "region" {
		t.Errorf("unexpected map after update: %+v", got)
	}
}

func TestDeleteMap(t *testing.T) {
	router, projectURL, token := setupMapProject(t)
	m := createMap(t, router, projectURL, token, "Doomed Map", "world")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("DELETE", projectURL+"/maps/"+m.ID, token, nil))
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete map: want 204, got %d: %s", w.Code, w.Body.String())
	}

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, testutil.AuthRequest("GET", projectURL+"/maps/"+m.ID, token, nil))
	if w2.Code != http.StatusNotFound {
		t.Errorf("get deleted map: want 404, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestGetMap_WrongProjectRejected(t *testing.T) {
	router, projectURL, token := setupMapProject(t)
	m := createMap(t, router, projectURL, token, "Some Map", "world")

	// A second project owned by the same user; the map belongs to the first.
	w := httptest.NewRecorder()
	router.ServeHTTP(w, testutil.AuthRequest("POST", "/api/v1/projects", token, map[string]interface{}{
		"title": "Other Project",
	}))
	var other project.ProjectResponse
	json.Unmarshal(w.Body.Bytes(), &other)

	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, testutil.AuthRequest("GET", "/api/v1/projects/"+other.ID.String()+"/maps/"+m.ID, token, nil))
	if w2.Code != http.StatusNotFound {
		t.Errorf("cross-project map fetch: want 404, got %d: %s", w2.Code, w2.Body.String())
	}
}
