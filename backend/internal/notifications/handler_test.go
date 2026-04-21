package notifications_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jconder44/nexustale/internal/notifications"
	"github.com/jconder44/nexustale/internal/testutil"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func serve(t *testing.T, r interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, method, url, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, testutil.AuthRequest(method, url, token, body))
	return w
}

// insertNotification creates a notification directly in the DB for a given user.
func insertNotification(t *testing.T, queries *sqlcgen.Queries, userID, notifType string) string {
	t.Helper()
	var uid pgtype.UUID
	if err := uid.Scan(userID); err != nil {
		t.Fatalf("parse userID: %v", err)
	}
	n, err := queries.CreateNotification(context.Background(), sqlcgen.CreateNotificationParams{
		UserID:    uid.Bytes,
		ProjectID: pgtype.UUID{Valid: false},
		Type:      notifType,
		Payload:   []byte(`{}`),
	})
	if err != nil {
		t.Fatalf("insert notification: %v", err)
	}
	return n.ID.String()
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestNotificationListEmpty(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok, _ := testutil.RegisterUser(t, r, "notif-empty@example.com", "Empty User")

	w := serve(t, r, "GET", "/api/v1/notifications", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d: %s", w.Code, w.Body.String())
	}
	var list []notifications.NotificationResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 0 {
		t.Fatalf("want empty list, got %d", len(list))
	}
}

func TestNotificationListShowsOwnNotifications(t *testing.T) {
	r, queries, _ := testutil.SetupRouter(t)
	tok1, uid1 := testutil.RegisterUser(t, r, "notif-user1@example.com", "User One")
	_, uid2 := testutil.RegisterUser(t, r, "notif-user2@example.com", "User Two")

	// Insert 2 notifications for user 1, 1 for user 2
	insertNotification(t, queries, uid1, "invite_received")
	insertNotification(t, queries, uid1, "mr_opened")
	insertNotification(t, queries, uid2, "invite_received")

	w := serve(t, r, "GET", "/api/v1/notifications", tok1, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d", w.Code)
	}
	var list []notifications.NotificationResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 2 {
		t.Fatalf("user 1 should see 2 notifications, got %d", len(list))
	}
	// Unread initially
	for _, n := range list {
		if n.ReadAt != nil {
			t.Errorf("notification %s should be unread", n.ID)
		}
	}
}

func TestNotificationMarkRead(t *testing.T) {
	r, queries, _ := testutil.SetupRouter(t)
	tok, uid := testutil.RegisterUser(t, r, "notif-markread@example.com", "Mark User")

	nid := insertNotification(t, queries, uid, "mr_approved")

	// Mark single read
	w := serve(t, r, "PUT", "/api/v1/notifications/"+nid+"/read", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("mark-read: want 200, got %d: %s", w.Code, w.Body.String())
	}

	// List should show it as read
	w = serve(t, r, "GET", "/api/v1/notifications", tok, nil)
	var list []notifications.NotificationResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Fatalf("want 1 notification, got %d", len(list))
	}
	if list[0].ReadAt == nil {
		t.Error("notification should be marked as read")
	}
}

func TestNotificationMarkAllRead(t *testing.T) {
	r, queries, _ := testutil.SetupRouter(t)
	tok, uid := testutil.RegisterUser(t, r, "notif-markall@example.com", "Mark All User")

	insertNotification(t, queries, uid, "invite_received")
	insertNotification(t, queries, uid, "mr_merged")
	insertNotification(t, queries, uid, "annotation_added")

	// Mark all read
	w := serve(t, r, "PUT", "/api/v1/notifications/read-all", tok, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("mark-all-read: want 200, got %d: %s", w.Code, w.Body.String())
	}

	// All should now be read
	w = serve(t, r, "GET", "/api/v1/notifications", tok, nil)
	var list []notifications.NotificationResponse
	json.Unmarshal(w.Body.Bytes(), &list)
	for _, n := range list {
		if n.ReadAt == nil {
			t.Errorf("notification %s should be read after mark-all-read", n.ID)
		}
	}
}

func TestNotificationUnauthenticated(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)

	routes := []struct{ method, url string }{
		{"GET", "/api/v1/notifications"},
		{"PUT", "/api/v1/notifications/00000000-0000-0000-0000-000000000000/read"},
		{"PUT", "/api/v1/notifications/read-all"},
	}
	for _, rt := range routes {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(rt.method, rt.url, nil)
		r.ServeHTTP(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without token: want 401, got %d", rt.method, rt.url, w.Code)
		}
	}
}

func TestNotificationMarkReadUnknownID(t *testing.T) {
	r, _, _ := testutil.SetupRouter(t)
	tok, _ := testutil.RegisterUser(t, r, "notif-unknown@example.com", "Unknown User")

	// Non-existent notification ID → 404
	w := serve(t, r, "PUT", "/api/v1/notifications/00000000-0000-0000-0000-000000000000/read", tok, nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("mark-read unknown: want 404, got %d", w.Code)
	}
}
