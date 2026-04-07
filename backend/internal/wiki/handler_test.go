package wiki_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jconder44/nexustale/internal/project"
	"github.com/jconder44/nexustale/internal/testutil"
	"github.com/jconder44/nexustale/internal/wiki"
)

// wikiBase returns the wiki API prefix for a given project ID string.
func wikiBase(projectID string) string {
	return "/api/v1/projects/" + projectID + "/wiki"
}

// setupProjectAndToken creates a user + project and returns the access token and project ID string.
func setupProjectAndToken(t *testing.T, email string) (router interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, token, projectID string) {
	t.Helper()
	r, _, _ := testutil.SetupRouter(t)
	token = testutil.RegisterAndGetToken(t, r, email)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest("POST", "/api/v1/projects", token, map[string]interface{}{
		"title":       "Wiki Test Novel",
		"description": "test",
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create project: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var proj project.ProjectResponse
	json.Unmarshal(w.Body.Bytes(), &proj)
	return r, token, proj.ID.String()
}

// ========================
// Entity CRUD
// ========================

func TestEntityCRUD(t *testing.T) {
	r, token, projID := setupProjectAndToken(t, "entitycrud@example.com")
	base := wikiBase(projID) + "/entities"

	// Create
	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest("POST", base, token, map[string]interface{}{
		"type":    "location",
		"name":    "The Iron Citadel",
		"summary": "A fortress carved into volcanic rock.",
		"attributes": map[string]interface{}{
			"region": "northern wastes",
		},
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create entity: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var entity wiki.EntityResponse
	json.Unmarshal(w.Body.Bytes(), &entity)

	if entity.Type != "location" {
		t.Errorf("expected type 'location', got '%s'", entity.Type)
	}
	if entity.Name != "The Iron Citadel" {
		t.Errorf("expected name 'The Iron Citadel', got '%s'", entity.Name)
	}
	if entity.ParentEntityID != nil {
		t.Error("expected no parent_entity_id on top-level entity")
	}

	var attrs map[string]interface{}
	json.Unmarshal(entity.Attributes, &attrs)
	if attrs["region"] != "northern wastes" {
		t.Errorf("expected attributes.region 'northern wastes', got %v", attrs["region"])
	}

	entityURL := base + "/" + entity.ID.String()

	// Get
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, testutil.AuthRequest("GET", entityURL, token, nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("get entity: expected 200, got %d", w2.Code)
	}
	var fetched wiki.EntityResponse
	json.Unmarshal(w2.Body.Bytes(), &fetched)
	if fetched.ID != entity.ID {
		t.Error("get entity returned wrong ID")
	}

	// List (all types)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, testutil.AuthRequest("GET", base, token, nil))
	if w3.Code != http.StatusOK {
		t.Fatalf("list entities: expected 200, got %d", w3.Code)
	}
	var list []wiki.EntityResponse
	json.Unmarshal(w3.Body.Bytes(), &list)
	if len(list) == 0 {
		t.Error("expected at least 1 entity in list")
	}

	// List filtered by type
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, testutil.AuthRequest("GET", base+"?type=location", token, nil))
	if w4.Code != http.StatusOK {
		t.Fatalf("list by type: expected 200, got %d", w4.Code)
	}
	var byType []wiki.EntityResponse
	json.Unmarshal(w4.Body.Bytes(), &byType)
	for _, e := range byType {
		if e.Type != "location" {
			t.Errorf("type filter returned entity with type '%s'", e.Type)
		}
	}

	// Update name + attributes
	w5 := httptest.NewRecorder()
	r.ServeHTTP(w5, testutil.AuthRequest("PATCH", entityURL, token, map[string]interface{}{
		"name": "The Iron Citadel (Ruined)",
		"attributes": map[string]interface{}{
			"region": "northern wastes",
			"status": "ruined",
		},
	}))
	if w5.Code != http.StatusOK {
		t.Fatalf("update entity: expected 200, got %d: %s", w5.Code, w5.Body.String())
	}
	var updated wiki.EntityResponse
	json.Unmarshal(w5.Body.Bytes(), &updated)
	if updated.Name != "The Iron Citadel (Ruined)" {
		t.Errorf("expected updated name, got '%s'", updated.Name)
	}
	var updatedAttrs map[string]interface{}
	json.Unmarshal(updated.Attributes, &updatedAttrs)
	if updatedAttrs["status"] != "ruined" {
		t.Errorf("expected attributes.status 'ruined', got %v", updatedAttrs["status"])
	}

	// Delete
	w6 := httptest.NewRecorder()
	r.ServeHTTP(w6, testutil.AuthRequest("DELETE", entityURL, token, nil))
	if w6.Code != http.StatusOK {
		t.Fatalf("delete entity: expected 200, got %d", w6.Code)
	}

	// Confirm gone
	w7 := httptest.NewRecorder()
	r.ServeHTTP(w7, testutil.AuthRequest("GET", entityURL, token, nil))
	if w7.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w7.Code)
	}
}

// ========================
// Child entities (Option A: parent_entity_id)
// ========================

func TestChildEntity(t *testing.T) {
	r, token, projID := setupProjectAndToken(t, "childentity@example.com")
	base := wikiBase(projID) + "/entities"

	// Create parent location
	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest("POST", base, token, map[string]interface{}{
		"type": "location",
		"name": "Ashenvale Forest",
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create location: %d: %s", w.Code, w.Body.String())
	}
	var location wiki.EntityResponse
	json.Unmarshal(w.Body.Bytes(), &location)

	// Create lore as child of location
	childURL := base + "/" + location.ID.String() + "/children"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, testutil.AuthRequest("POST", childURL, token, map[string]interface{}{
		"type":    "lore",
		"name":    "The Ancient Pact",
		"summary": "A binding agreement made between the forest spirits and the first settlers.",
	}))
	if w2.Code != http.StatusCreated {
		t.Fatalf("create child entity: %d: %s", w2.Code, w2.Body.String())
	}
	var lore wiki.EntityResponse
	json.Unmarshal(w2.Body.Bytes(), &lore)

	if lore.Type != "lore" {
		t.Errorf("expected type 'lore', got '%s'", lore.Type)
	}
	if lore.ParentEntityID == nil {
		t.Fatal("expected parent_entity_id to be set on child entity")
	}
	if *lore.ParentEntityID != location.ID {
		t.Errorf("expected parent_entity_id to match location ID")
	}

	// List children of location
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, testutil.AuthRequest("GET", childURL, token, nil))
	if w3.Code != http.StatusOK {
		t.Fatalf("list children: %d", w3.Code)
	}
	var children []wiki.EntityResponse
	json.Unmarshal(w3.Body.Bytes(), &children)
	if len(children) != 1 {
		t.Errorf("expected 1 child, got %d", len(children))
	}
	if children[0].ID != lore.ID {
		t.Error("child list returned wrong entity")
	}
}

// ========================
// Relationships
// ========================

func TestRelationshipCRUD(t *testing.T) {
	r, token, projID := setupProjectAndToken(t, "relcrud@example.com")
	entBase := wikiBase(projID) + "/entities"
	relBase := wikiBase(projID) + "/relationships"

	// Create two entities
	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest("POST", entBase, token, map[string]interface{}{"type": "character", "name": "Aria"}))
	var char wiki.EntityResponse
	json.Unmarshal(w.Body.Bytes(), &char)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, testutil.AuthRequest("POST", entBase, token, map[string]interface{}{"type": "faction", "name": "The Silver Order"}))
	var faction wiki.EntityResponse
	json.Unmarshal(w2.Body.Bytes(), &faction)

	// Create relationship
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, testutil.AuthRequest("POST", relBase, token, map[string]interface{}{
		"from_entity_id": char.ID.String(),
		"to_entity_id":   faction.ID.String(),
		"type":           "member_of",
		"description":    "Aria is a senior member of the Silver Order.",
	}))
	if w3.Code != http.StatusCreated {
		t.Fatalf("create relationship: %d: %s", w3.Code, w3.Body.String())
	}
	var rel wiki.RelationshipResponse
	json.Unmarshal(w3.Body.Bytes(), &rel)

	if rel.Type != "member_of" {
		t.Errorf("expected type 'member_of', got '%s'", rel.Type)
	}
	if rel.FromEntityID != char.ID {
		t.Error("from_entity_id mismatch")
	}
	if rel.ToEntityID != faction.ID {
		t.Error("to_entity_id mismatch")
	}

	// List relationships
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, testutil.AuthRequest("GET", relBase, token, nil))
	if w4.Code != http.StatusOK {
		t.Fatalf("list relationships: %d", w4.Code)
	}
	var rels []wiki.RelationshipResponse
	json.Unmarshal(w4.Body.Bytes(), &rels)
	if len(rels) != 1 {
		t.Errorf("expected 1 relationship, got %d", len(rels))
	}

	// Delete relationship
	w5 := httptest.NewRecorder()
	r.ServeHTTP(w5, testutil.AuthRequest("DELETE", relBase+"/"+rel.ID.String(), token, nil))
	if w5.Code != http.StatusOK {
		t.Fatalf("delete relationship: %d", w5.Code)
	}

	// Confirm list is now empty
	w6 := httptest.NewRecorder()
	r.ServeHTTP(w6, testutil.AuthRequest("GET", relBase, token, nil))
	var relsAfter []wiki.RelationshipResponse
	json.Unmarshal(w6.Body.Bytes(), &relsAfter)
	if len(relsAfter) != 0 {
		t.Errorf("expected 0 relationships after delete, got %d", len(relsAfter))
	}
}

// ========================
// Graph endpoint
// ========================

func TestGraphEndpoint(t *testing.T) {
	r, token, projID := setupProjectAndToken(t, "graph@example.com")
	entBase := wikiBase(projID) + "/entities"

	// Create two entities and a relationship
	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest("POST", entBase, token, map[string]interface{}{"type": "character", "name": "Kira"}))
	var e1 wiki.EntityResponse
	json.Unmarshal(w.Body.Bytes(), &e1)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, testutil.AuthRequest("POST", entBase, token, map[string]interface{}{"type": "location", "name": "The Lighthouse"}))
	var e2 wiki.EntityResponse
	json.Unmarshal(w2.Body.Bytes(), &e2)

	r.ServeHTTP(httptest.NewRecorder(), testutil.AuthRequest("POST", wikiBase(projID)+"/relationships", token, map[string]interface{}{
		"from_entity_id": e1.ID.String(),
		"to_entity_id":   e2.ID.String(),
		"type":           "guards",
	}))

	// Get graph
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, testutil.AuthRequest("GET", wikiBase(projID)+"/graph", token, nil))
	if w3.Code != http.StatusOK {
		t.Fatalf("get graph: %d: %s", w3.Code, w3.Body.String())
	}
	var graph wiki.GraphResponse
	json.Unmarshal(w3.Body.Bytes(), &graph)

	if len(graph.Entities) < 2 {
		t.Errorf("expected at least 2 entities in graph, got %d", len(graph.Entities))
	}
	if len(graph.Relationships) < 1 {
		t.Errorf("expected at least 1 relationship in graph, got %d", len(graph.Relationships))
	}
}

// ========================
// Magic Rules
// ========================

func TestMagicRuleCRUD(t *testing.T) {
	r, token, projID := setupProjectAndToken(t, "magicrule@example.com")
	base := wikiBase(projID) + "/magic-rules"

	// Create
	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest("POST", base, token, map[string]interface{}{
		"name":        "Conservation of Essence",
		"description": "Magic drawn from the world must be returned; hoarding causes decay.",
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create magic rule: %d: %s", w.Code, w.Body.String())
	}
	var rule wiki.MagicRuleResponse
	json.Unmarshal(w.Body.Bytes(), &rule)

	if rule.Name != "Conservation of Essence" {
		t.Errorf("expected name 'Conservation of Essence', got '%s'", rule.Name)
	}

	ruleURL := base + "/" + rule.ID.String()

	// List
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, testutil.AuthRequest("GET", base, token, nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("list magic rules: %d", w2.Code)
	}
	var rules []wiki.MagicRuleResponse
	json.Unmarshal(w2.Body.Bytes(), &rules)
	if len(rules) == 0 {
		t.Error("expected at least 1 magic rule")
	}

	// Update
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, testutil.AuthRequest("PATCH", ruleURL, token, map[string]interface{}{
		"description": "Magic drawn from the world must be returned; hoarding causes decay. Violators are marked.",
	}))
	if w3.Code != http.StatusOK {
		t.Fatalf("update magic rule: %d: %s", w3.Code, w3.Body.String())
	}
	var updated wiki.MagicRuleResponse
	json.Unmarshal(w3.Body.Bytes(), &updated)
	if updated.Name != "Conservation of Essence" {
		t.Error("update should not change name when not provided")
	}

	// Delete
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, testutil.AuthRequest("DELETE", ruleURL, token, nil))
	if w4.Code != http.StatusOK {
		t.Fatalf("delete magic rule: %d", w4.Code)
	}
}

// ========================
// Timeline Events
// ========================

func TestTimelineEventCRUD(t *testing.T) {
	r, token, projID := setupProjectAndToken(t, "timeline@example.com")
	base := wikiBase(projID) + "/timeline"

	// Create with full date
	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest("POST", base, token, map[string]interface{}{
		"name":        "The First Sundering",
		"description": "The world was split into three shards.",
		"era":         "Age of Breaking",
		"year":        1,
		"month":       6,
		"day":         21,
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create timeline event: %d: %s", w.Code, w.Body.String())
	}
	var event wiki.TimelineEventResponse
	json.Unmarshal(w.Body.Bytes(), &event)

	if event.Name != "The First Sundering" {
		t.Errorf("expected name 'The First Sundering', got '%s'", event.Name)
	}
	if event.Era != "Age of Breaking" {
		t.Errorf("expected era 'Age of Breaking', got '%s'", event.Era)
	}
	if event.Year == nil || *event.Year != 1 {
		t.Error("expected year 1")
	}
	if event.Month == nil || *event.Month != 6 {
		t.Error("expected month 6")
	}
	if event.Day == nil || *event.Day != 21 {
		t.Error("expected day 21")
	}

	eventURL := base + "/" + event.ID.String()

	// List
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, testutil.AuthRequest("GET", base, token, nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("list timeline events: %d", w2.Code)
	}
	var events []wiki.TimelineEventResponse
	json.Unmarshal(w2.Body.Bytes(), &events)
	if len(events) == 0 {
		t.Error("expected at least 1 timeline event")
	}

	// Create era-only event (no date)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, testutil.AuthRequest("POST", base, token, map[string]interface{}{
		"name": "The Long Silence",
		"era":  "Age of Mending",
	}))
	if w3.Code != http.StatusCreated {
		t.Fatalf("create era-only event: %d", w3.Code)
	}
	var eraOnly wiki.TimelineEventResponse
	json.Unmarshal(w3.Body.Bytes(), &eraOnly)
	if eraOnly.Year != nil {
		t.Error("expected nil year for era-only event")
	}

	// Update description only — name should be preserved
	w4 := httptest.NewRecorder()
	r.ServeHTTP(w4, testutil.AuthRequest("PATCH", eventURL, token, map[string]interface{}{
		"description": "The world was split into three shards by the Void God's final act.",
	}))
	if w4.Code != http.StatusOK {
		t.Fatalf("update timeline event: %d: %s", w4.Code, w4.Body.String())
	}
	var updatedEvent wiki.TimelineEventResponse
	json.Unmarshal(w4.Body.Bytes(), &updatedEvent)
	if updatedEvent.Name != "The First Sundering" {
		t.Error("update should not change name when not provided")
	}

	// Update date fields
	newYear := int32(2)
	newMonth := int32(3)
	newDay := int32(15)
	w4b := httptest.NewRecorder()
	r.ServeHTTP(w4b, testutil.AuthRequest("PATCH", eventURL, token, map[string]interface{}{
		"year":  newYear,
		"month": newMonth,
		"day":   newDay,
	}))
	if w4b.Code != http.StatusOK {
		t.Fatalf("update timeline event date: %d: %s", w4b.Code, w4b.Body.String())
	}
	var dateUpdated wiki.TimelineEventResponse
	json.Unmarshal(w4b.Body.Bytes(), &dateUpdated)
	if dateUpdated.Year == nil || *dateUpdated.Year != 2 {
		t.Errorf("expected year 2 after update, got %v", dateUpdated.Year)
	}
	if dateUpdated.Month == nil || *dateUpdated.Month != 3 {
		t.Errorf("expected month 3 after update, got %v", dateUpdated.Month)
	}
	if dateUpdated.Day == nil || *dateUpdated.Day != 15 {
		t.Errorf("expected day 15 after update, got %v", dateUpdated.Day)
	}
	// Era and name should be unchanged
	if dateUpdated.Era != "Age of Breaking" {
		t.Errorf("era should be unchanged after date update, got '%s'", dateUpdated.Era)
	}

	// Delete
	w5 := httptest.NewRecorder()
	r.ServeHTTP(w5, testutil.AuthRequest("DELETE", eventURL, token, nil))
	if w5.Code != http.StatusOK {
		t.Fatalf("delete timeline event: %d", w5.Code)
	}
}

// ========================
// Autolink
// ========================

func TestAutolink(t *testing.T) {
	r, token, projID := setupProjectAndToken(t, "autolink@example.com")
	entBase := wikiBase(projID) + "/entities"

	// Create entities with names that appear in prose
	r.ServeHTTP(httptest.NewRecorder(), testutil.AuthRequest("POST", entBase, token, map[string]interface{}{"type": "character", "name": "Elder Morath"}))
	r.ServeHTTP(httptest.NewRecorder(), testutil.AuthRequest("POST", entBase, token, map[string]interface{}{"type": "location", "name": "The Sunken Library"}))
	r.ServeHTTP(httptest.NewRecorder(), testutil.AuthRequest("POST", entBase, token, map[string]interface{}{"type": "faction", "name": "The Quill Society"}))

	prose := "Elder Morath descended the steps into The Sunken Library, seeking a scroll the Quill Society had hidden centuries ago."

	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest("GET", wikiBase(projID)+"/autolink?text="+prose, token, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("autolink: %d: %s", w.Code, w.Body.String())
	}

	var resp struct {
		Entities []wiki.EntityResponse `json:"entities"`
	}
	json.Unmarshal(w.Body.Bytes(), &resp)

	names := make(map[string]bool)
	for _, e := range resp.Entities {
		names[e.Name] = true
	}

	if !names["Elder Morath"] {
		t.Error("expected 'Elder Morath' in autolink results")
	}
	if !names["The Sunken Library"] {
		t.Error("expected 'The Sunken Library' in autolink results")
	}
	// "The Quill Society" is not in the prose verbatim but "Quill Society" is — testing case-insensitive substring
	// The entity name is "The Quill Society"; the prose says "Quill Society" — partial match, should NOT appear
	// This is intentional: autolink requires the full entity name to be present.
}

// ========================
// Unauthenticated access
// ========================

func TestWikiUnauthenticated(t *testing.T) {
	r, token, projID := setupProjectAndToken(t, "wikiunauth@example.com")
	_ = token // set up project but do not use token for the test request

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", wikiBase(projID)+"/entities", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for unauthenticated wiki request, got %d", w.Code)
	}
}
