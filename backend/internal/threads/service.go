package threads

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jconder44/nexustale/pkg/apperror"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

type Service struct {
	queries *sqlcgen.Queries
}

func NewService(queries *sqlcgen.Queries) *Service {
	return &Service{queries: queries}
}

// ── DTOs ──────────────────────────────────────────────────────────────────────

type ThreadResponse struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"project_id"`
	Title             string    `json:"title"`
	Type              string    `json:"type"`
	Notes             string    `json:"notes"`
	OpenedAtSceneID   *string   `json:"opened_at_scene_id,omitempty"`
	ClosedAtSceneID   *string   `json:"closed_at_scene_id,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ChapterThreadCount struct {
	ChapterID       string `json:"chapter_id"`
	OpenThreadCount int32  `json:"open_thread_count"`
}

type CreateThreadRequest struct {
	Title           string  `json:"title" binding:"required,min=1,max=300"`
	Type            string  `json:"type"  binding:"required,oneof=world mystery arc conflict"`
	Notes           string  `json:"notes"`
	OpenedAtSceneID *string `json:"opened_at_scene_id"`
}

type UpdateThreadRequest struct {
	Title           *string  `json:"title"`
	Type            *string  `json:"type"  binding:"omitempty,oneof=world mystery arc conflict"`
	Notes           *string  `json:"notes"`
	OpenedAtSceneID *string  `json:"opened_at_scene_id"`
	// ClosedAtSceneID is nullable; send null to re-open a thread.
	ClosedAtSceneID *string `json:"closed_at_scene_id"`
}

// ── Service methods ───────────────────────────────────────────────────────────

func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]ThreadResponse, error) {
	rows, err := s.queries.ListThreadsByProject(ctx, projectID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list threads: %v", err))
	}
	out := make([]ThreadResponse, len(rows))
	for i, r := range rows {
		out[i] = toResponse(r)
	}
	return out, nil
}

func (s *Service) ChapterCounts(ctx context.Context, projectID uuid.UUID) ([]ChapterThreadCount, error) {
	rows, err := s.queries.CountOpenThreadsByChapter(ctx, projectID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("count open threads by chapter: %v", err))
	}
	out := make([]ChapterThreadCount, len(rows))
	for i, r := range rows {
		out[i] = ChapterThreadCount{
			ChapterID:       r.ChapterID.String(),
			OpenThreadCount: r.OpenThreadCount,
		}
	}
	return out, nil
}

func (s *Service) Create(ctx context.Context, projectID uuid.UUID, req CreateThreadRequest) (*ThreadResponse, error) {
	openedID := pgtype.UUID{}
	if req.OpenedAtSceneID != nil {
		id, err := uuid.Parse(*req.OpenedAtSceneID)
		if err != nil {
			return nil, apperror.Validation("invalid opened_at_scene_id")
		}
		openedID = pgtype.UUID{Bytes: [16]byte(id), Valid: true}
	}
	row, err := s.queries.CreateThread(ctx, sqlcgen.CreateThreadParams{
		ProjectID:       projectID,
		Title:           req.Title,
		Type:            req.Type,
		Notes:           req.Notes,
		OpenedAtSceneID: openedID,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("create thread: %v", err))
	}
	r := toResponse(row)
	return &r, nil
}

func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateThreadRequest) (*ThreadResponse, error) {
	params := sqlcgen.UpdateThreadParams{ID: id}
	if req.Title != nil {
		params.Title = pgtype.Text{String: *req.Title, Valid: true}
	}
	if req.Type != nil {
		params.Type = pgtype.Text{String: *req.Type, Valid: true}
	}
	if req.Notes != nil {
		params.Notes = pgtype.Text{String: *req.Notes, Valid: true}
	}
	if req.OpenedAtSceneID != nil {
		id, err := uuid.Parse(*req.OpenedAtSceneID)
		if err != nil {
			return nil, apperror.Validation("invalid opened_at_scene_id")
		}
		params.OpenedAtSceneID = pgtype.UUID{Bytes: [16]byte(id), Valid: true}
	}
	// ClosedAtSceneID is always set (nullable field): null to re-open, UUID to close.
	if req.ClosedAtSceneID != nil && *req.ClosedAtSceneID != "" {
		id, err := uuid.Parse(*req.ClosedAtSceneID)
		if err != nil {
			return nil, apperror.Validation("invalid closed_at_scene_id")
		}
		params.ClosedAtSceneID = pgtype.UUID{Bytes: [16]byte(id), Valid: true}
	}

	row, err := s.queries.UpdateThread(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("thread", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("update thread: %v", err))
	}
	r := toResponse(row)
	return &r, nil
}

func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	if err := s.queries.DeleteThread(ctx, id); err != nil {
		return apperror.Internal(fmt.Sprintf("delete thread: %v", err))
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func toResponse(r sqlcgen.StoryThread) ThreadResponse {
	resp := ThreadResponse{
		ID:        r.ID.String(),
		ProjectID: r.ProjectID.String(),
		Title:     r.Title,
		Type:      r.Type,
		Notes:     r.Notes,
		CreatedAt: r.CreatedAt.Time,
		UpdatedAt: r.UpdatedAt.Time,
	}
	if r.OpenedAtSceneID.Valid {
		s := uuid.UUID(r.OpenedAtSceneID.Bytes).String()
		resp.OpenedAtSceneID = &s
	}
	if r.ClosedAtSceneID.Valid {
		s := uuid.UUID(r.ClosedAtSceneID.Bytes).String()
		resp.ClosedAtSceneID = &s
	}
	return resp
}
