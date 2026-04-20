package notifications

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// Service manages user notifications.
type Service struct {
	queries *sqlcgen.Queries
}

func NewService(queries *sqlcgen.Queries) *Service {
	return &Service{queries: queries}
}

// NotificationResponse is the public wire shape for a notification.
type NotificationResponse struct {
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	ProjectID *string         `json:"project_id,omitempty"`
	Payload   json.RawMessage `json:"payload"`
	ReadAt    *string         `json:"read_at"`
	CreatedAt string          `json:"created_at"`
}

// Notify creates a notification for a user. projectID may be uuid.Nil for
// non-project notifications. Designed to be called from a goroutine (fire-and-forget).
func (s *Service) Notify(ctx context.Context, userID, projectID uuid.UUID, notifType string, payload map[string]any) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return
	}

	pgProjectID := pgtype.UUID{}
	if projectID != uuid.Nil {
		pgProjectID = pgtype.UUID{Bytes: [16]byte(projectID), Valid: true}
	}

	s.queries.CreateNotification(ctx, sqlcgen.CreateNotificationParams{ //nolint:errcheck
		UserID:    userID,
		ProjectID: pgProjectID,
		Type:      notifType,
		Payload:   raw,
	})
}

// List returns all unread + recent read notifications for the user.
func (s *Service) List(ctx context.Context, userID uuid.UUID) ([]NotificationResponse, error) {
	rows, err := s.queries.ListNotifications(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list notifications: %w", err)
	}
	out := make([]NotificationResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, toResponse(r))
	}
	return out, nil
}

// MarkRead marks a single notification as read (scoped to userID — no cross-user reads).
func (s *Service) MarkRead(ctx context.Context, userID, notifID uuid.UUID) error {
	return s.queries.MarkNotificationRead(ctx, sqlcgen.MarkNotificationReadParams{
		ID:     notifID,
		UserID: userID,
	})
}

// MarkAllRead marks every unread notification for the user as read.
func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return s.queries.MarkAllNotificationsRead(ctx, userID)
}

func toResponse(n sqlcgen.Notification) NotificationResponse {
	r := NotificationResponse{
		ID:        n.ID.String(),
		Type:      n.Type,
		Payload:   n.Payload,
		CreatedAt: n.CreatedAt.Time.Format(time.RFC3339),
	}
	if n.ProjectID.Valid {
		s := uuid.UUID(n.ProjectID.Bytes).String()
		r.ProjectID = &s
	}
	if n.ReadAt.Valid {
		s := n.ReadAt.Time.Format(time.RFC3339)
		r.ReadAt = &s
	}
	return r
}
