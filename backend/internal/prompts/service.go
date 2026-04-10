package prompts

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

// PromptResponse is the public representation of a writing style preset.
type PromptResponse struct {
	ID            uuid.UUID `json:"id"`
	ProjectID     uuid.UUID `json:"project_id"`
	Name          string    `json:"name"`
	Category      string    `json:"category"`
	Content       string    `json:"content"`
	SystemContent string    `json:"system_content"`
	SortOrder     int32     `json:"sort_order"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Service struct {
	queries *sqlcgen.Queries
}

func NewService(queries *sqlcgen.Queries) *Service {
	return &Service{queries: queries}
}

// List returns all writing style presets for the project, ordered by sort_order.
func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]PromptResponse, error) {
	rows, err := s.queries.ListProjectPrompts(ctx, projectID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list prompts: %v", err))
	}
	out := make([]PromptResponse, len(rows))
	for i, r := range rows {
		out[i] = toResponse(r)
	}
	return out, nil
}

// Get returns a single prompt by ID.
func (s *Service) Get(ctx context.Context, id uuid.UUID) (*PromptResponse, error) {
	r, err := s.queries.GetProjectPrompt(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("prompt", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("get prompt: %v", err))
	}
	p := toResponse(r)
	return &p, nil
}

type CreateRequest struct {
	Name          string `json:"name"`
	Category      string `json:"category"`
	Content       string `json:"content"`
	SystemContent string `json:"system_content"`
	SortOrder     int32  `json:"sort_order"`
}

// Create adds a new writing style preset to the project.
func (s *Service) Create(ctx context.Context, projectID uuid.UUID, req CreateRequest) (*PromptResponse, error) {
	if req.Name == "" {
		return nil, apperror.Validation("name is required")
	}
	cat := req.Category
	if cat == "" {
		cat = "prose"
	}
	r, err := s.queries.CreateProjectPrompt(ctx, sqlcgen.CreateProjectPromptParams{
		ProjectID:     projectID,
		Name:          req.Name,
		Category:      cat,
		Content:       req.Content,
		SystemContent: req.SystemContent,
		SortOrder:     req.SortOrder,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("create prompt: %v", err))
	}
	p := toResponse(r)
	return &p, nil
}

type UpdateRequest struct {
	Name          *string `json:"name"`
	Category      *string `json:"category"`
	Content       *string `json:"content"`
	SystemContent *string `json:"system_content"`
	SortOrder     *int32  `json:"sort_order"`
}

// Update patches a writing style preset by ID.
// Returns 404 if the prompt does not exist.
func (s *Service) Update(ctx context.Context, id uuid.UUID, req UpdateRequest) (*PromptResponse, error) {
	params := sqlcgen.UpdateProjectPromptParams{ID: id}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Category != nil {
		params.Category = pgtype.Text{String: *req.Category, Valid: true}
	}
	if req.Content != nil {
		params.Content = pgtype.Text{String: *req.Content, Valid: true}
	}
	if req.SystemContent != nil {
		params.SystemContent = pgtype.Text{String: *req.SystemContent, Valid: true}
	}
	if req.SortOrder != nil {
		params.SortOrder = pgtype.Int4{Int32: *req.SortOrder, Valid: true}
	}

	r, err := s.queries.UpdateProjectPrompt(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("prompt", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("update prompt: %v", err))
	}
	p := toResponse(r)
	return &p, nil
}

// Delete removes a writing style preset by ID.
func (s *Service) Delete(ctx context.Context, id uuid.UUID) error {
	// Verify existence before delete so we can return 404.
	if _, err := s.queries.GetProjectPrompt(ctx, id); errors.Is(err, pgx.ErrNoRows) {
		return apperror.NotFound("prompt", id.String())
	}
	return s.queries.DeleteProjectPrompt(ctx, id)
}

// ── helpers ────────────────────────────────────────────────────────────────────

func toResponse(r sqlcgen.ProjectPrompt) PromptResponse {
	return PromptResponse{
		ID:            r.ID,
		ProjectID:     r.ProjectID,
		Name:          r.Name,
		Category:      r.Category,
		Content:       r.Content,
		SystemContent: r.SystemContent,
		SortOrder:     r.SortOrder,
		CreatedAt:     r.CreatedAt.Time,
		UpdatedAt:     r.UpdatedAt.Time,
	}
}
