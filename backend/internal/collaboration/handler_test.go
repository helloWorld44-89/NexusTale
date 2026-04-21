package collaboration_test

// Integration tests for the collaboration invite + collaborator management routes.
//
// AcceptInvite is NOT tested here because it requires a real git clone of the
// project's bare repo, which needs SetupRouterWithGit + a Chronicle commit to
// initialize the repo. That path is covered by manual Bruno tests in bruno/10-collaboration/.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jconder44/nexustale/internal/collaboration"
	"github.com/jconder44/nexustale/internal/testutil"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func serve(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, method, url, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest(method, url, token, body))
	return w
}

func createProject(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, token, title string) string {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest("POST", "/api/v1/projects", token, map[string]interface{}{
		"title": title,
	}))
	if w.Code != http.StatusCreated {
		t.Fatalf("create project: %d: %s", w.Code, w.Body.String())
	}
	var p struct{ ID string `json:"id"` }
	json.Unmarshal(w.Body.Bytes(), &p)
	return p.ID
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestListCollaboratorsEmpty(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok, _ := testutil.RegisterUser(t, r, "collab-list@example.com", "Owner")
	projID := createProject(t, r, tok, "Collab Test Project")

	w := serve(t, r, "GET", "/api/v1/projects/"+projID+"/collaborators", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var list []collaboration.CollaboratorResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatalf("want empty collaborator list, got %d", len(list))
	}
}

func TestInviteCollaborator(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	ownerTok, _ := testutil.RegisterUser(t, r, "collab-owner@example.com", "Owner")
	// Register the invitee so InviteCollaborator can look them up by email.
	testutil.RegisterAndGetToken(t, r, "collab-invitee@example.com")
	projID := createProject(t, r, ownerTok, "Invite Test Project")

	// Send invite
	w := serve(t, r, "POST", "/api/v1/projects/"+projID+"/invites", ownerTok, map[string]interface{}{
		"email": "collab-invitee@example.com",
		"role":  "coauthor",
	})
	if w.Code != http.StatusCreated {
		t.Fatalf("invite: want 201, got %d: %s", w.Code, w.Body.String())
	}
	var inv collaboration.InviteResponse
	json.Unmarshal(w.Body.Bytes(), &inv)
	if inv.Token == "" {
		t.Error("invite should have a token")
	}
	if inv.Role != "coauthor" {
		t.Errorf("role: want coauthor, got %s", inv.Role)
	}
	if inv.Email != "collab-invitee@example.com" {
		t.Errorf("email: got %s", inv.Email)
	}

	// Invite preview (public route — no auth required)
	wPublic := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/invites/"+inv.Token, nil)
	r.ServeHTTP(wPublic, req)
	if wPublic.Code != http.StatusOK {
		t.Fatalf("invite preview: want 200, got %d: %s", wPublic.Code, wPublic.Body.String())
	}
	var preview collaboration.InviteResponse
	json.Unmarshal(wPublic.Body.Bytes(), &preview)
	if preview.Token != inv.Token {
		t.Errorf("preview token mismatch")
	}
}

func TestListPendingInvites(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	ownerTok, _ := testutil.RegisterUser(t, r, "pending-owner@example.com", "Owner")
	testutil.RegisterAndGetToken(t, r, "pending-invitee@example.com")
	projID := createProject(t, r, ownerTok, "Pending Invites Project")

	// No invites initially
	w := serve(t, r, "GET", "/api/v1/projects/"+projID+"/invites", ownerTok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list invites: want 200, got %d", w.Code)
	}
	var list []collaboration.InviteResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatalf("want 0 invites, got %d", len(list))
	}

	// Send invite
	serve(t, r, "POST", "/api/v1/projects/"+projID+"/invites", ownerTok, map[string]interface{}{
		"email": "pending-invitee@example.com",
		"role":  "reviewer",
	})

	// Now one pending invite
	w = serve(t, r, "GET", "/api/v1/projects/"+projID+"/invites", ownerTok, nil)
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("want 1 invite, got %d", len(list))
	}
}

func TestInviteValidation(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	ownerTok, _ := testutil.RegisterUser(t, r, "invite-val@example.com", "Owner")
	testutil.RegisterAndGetToken(t, r, "invite-val-invitee@example.com")
	projID := createProject(t, r, ownerTok, "Validation Project")

	// Invalid role → 400
	w := serve(t, r, "POST", "/api/v1/projects/"+projID+"/invites", ownerTok, map[string]interface{}{
		"email": "invite-val-invitee@example.com",
		"role":  "superadmin", // not a valid role
	})
	if w.Code != http.StatusBadRequest && w.Code != http.StatusUnprocessableEntity {
		t.Errorf("invalid role: want 400/422, got %d: %s", w.Code, w.Body.String())
	}

	// Unknown email → 400 (user not found)
	w = serve(t, r, "POST", "/api/v1/projects/"+projID+"/invites", ownerTok, map[string]interface{}{
		"email": "nobody@nowhere.example.com",
		"role":  "coauthor",
	})
	if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
		t.Errorf("unknown email: want 400/404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestInviteNonOwnerForbidden(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	ownerTok, _ := testutil.RegisterUser(t, r, "invite-fo-owner@example.com", "Owner")
	nonOwnerTok, _ := testutil.RegisterUser(t, r, "invite-fo-other@example.com", "Other")
	testutil.RegisterAndGetToken(t, r, "invite-fo-target@example.com")
	projID := createProject(t, r, ownerTok, "Forbidden Invite Project")

	// Non-owner tries to send invite → 403
	w := serve(t, r, "POST", "/api/v1/projects/"+projID+"/invites", nonOwnerTok, map[string]interface{}{
		"email": "invite-fo-target@example.com",
		"role":  "coauthor",
	})
	if w.Code != http.StatusForbidden {
		t.Errorf("non-owner invite: want 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCollaborationUnauthenticated(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	ownerTok, _ := testutil.RegisterUser(t, r, "collab-unauth@example.com", "Owner")
	projID := createProject(t, r, ownerTok, "Auth Test Project")

	authRoutes := []struct{ method, url string }{
		{"POST", "/api/v1/projects/" + projID + "/invites"},
		{"GET", "/api/v1/projects/" + projID + "/invites"},
		{"GET", "/api/v1/projects/" + projID + "/collaborators"},
	}
	for _, rt := range authRoutes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(rt.method, rt.url, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token: want 401, got %d", rt.method, rt.url, w.Code)
		}
	}
}

func TestInvitePreviewInvalidToken(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/invites/not-a-real-token", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("invalid invite token: want 404, got %d", w.Code)
	}
}
