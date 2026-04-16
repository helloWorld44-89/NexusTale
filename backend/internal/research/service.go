package research

import (
	"context"

	"github.com/google/uuid"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// Service provides research-note CRUD for a project.
type Service struct {
	queries *sqlcgen.Queries
}

func NewService(queries *sqlcgen.Queries) *Service {
	return &Service{queries: queries}
}

// NoteResponse is the public wire shape for a research note.
type NoteResponse struct {
	ID        string   `json:"id"`
	ProjectID string   `json:"project_id"`
	UserID    string   `json:"user_id"`
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	SourceURL string   `json:"source_url"`
	Tags      []string `json:"tags"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

func toResponse(row sqlcgen.ResearchNote) NoteResponse {
	tags := row.Tags
	if tags == nil {
		tags = []string{}
	}
	return NoteResponse{
		ID:        row.ID.String(),
		ProjectID: row.ProjectID.String(),
		UserID:    row.UserID.String(),
		Title:     row.Title,
		Body:      row.Body,
		SourceURL: row.SourceUrl,
		Tags:      tags,
		CreatedAt: row.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt: row.UpdatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// List returns all research notes for the project, newest first.
func (s *Service) List(ctx context.Context, projectID uuid.UUID) ([]NoteResponse, error) {
	rows, err := s.queries.ListResearchNotes(ctx, projectID)
	if err != nil {
		return nil, err
	}
	resp := make([]NoteResponse, 0, len(rows))
	for _, r := range rows {
		resp = append(resp, toResponse(r))
	}
	return resp, nil
}

// Create inserts a new research note and returns it.
func (s *Service) Create(ctx context.Context, projectID, userID uuid.UUID, title, body, sourceURL string, tags []string) (NoteResponse, error) {
	if tags == nil {
		tags = []string{}
	}
	row, err := s.queries.CreateResearchNote(ctx, sqlcgen.CreateResearchNoteParams{
		ProjectID: projectID,
		UserID:    userID,
		Title:     title,
		Body:      body,
		SourceUrl: sourceURL,
		Tags:      tags,
	})
	if err != nil {
		return NoteResponse{}, err
	}
	return toResponse(row), nil
}

// Get returns a single research note by ID within the project.
func (s *Service) Get(ctx context.Context, projectID, noteID uuid.UUID) (NoteResponse, error) {
	row, err := s.queries.GetResearchNote(ctx, sqlcgen.GetResearchNoteParams{
		ID:        noteID,
		ProjectID: projectID,
	})
	if err != nil {
		return NoteResponse{}, err
	}
	return toResponse(row), nil
}

// Update replaces the mutable fields of a research note.
func (s *Service) Update(ctx context.Context, projectID, noteID uuid.UUID, title, body, sourceURL string, tags []string) (NoteResponse, error) {
	if tags == nil {
		tags = []string{}
	}
	row, err := s.queries.UpdateResearchNote(ctx, sqlcgen.UpdateResearchNoteParams{
		ID:        noteID,
		ProjectID: projectID,
		Title:     title,
		Body:      body,
		SourceUrl: sourceURL,
		Tags:      tags,
	})
	if err != nil {
		return NoteResponse{}, err
	}
	return toResponse(row), nil
}

// Delete removes a research note.
func (s *Service) Delete(ctx context.Context, projectID, noteID uuid.UUID) error {
	return s.queries.DeleteResearchNote(ctx, sqlcgen.DeleteResearchNoteParams{
		ID:        noteID,
		ProjectID: projectID,
	})
}
