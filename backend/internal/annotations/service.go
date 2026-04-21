package annotations

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// Service provides CRUD for manuscript annotations.
type Service struct {
	queries *sqlcgen.Queries
}

func NewService(q *sqlcgen.Queries) *Service { return &Service{queries: q} }

// AnnotationResponse is the public wire shape for an annotation.
type AnnotationResponse struct {
	ID         string  `json:"id"`
	ProjectID  string  `json:"project_id"`
	SceneID    string  `json:"scene_id"`
	AuthorID   string  `json:"author_id"`
	AuthorName string  `json:"author_name"`
	StartChar  int32   `json:"start_char"`
	EndChar    int32   `json:"end_char"`
	Body       string  `json:"body"`
	Type       string  `json:"type"`
	Resolved   bool    `json:"resolved"`
	ResolvedBy *string `json:"resolved_by,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

// ── conversions ───────────────────────────────────────────────────────────────

func fromListRow(r sqlcgen.ListAnnotationsBySceneRow) AnnotationResponse {
	return AnnotationResponse{
		ID:         r.ID.String(),
		ProjectID:  r.ProjectID.String(),
		SceneID:    r.SceneID.String(),
		AuthorID:   r.AuthorID.String(),
		AuthorName: r.AuthorName,
		StartChar:  r.StartChar,
		EndChar:    r.EndChar,
		Body:       r.Body,
		Type:       r.Type,
		Resolved:   r.Resolved,
		ResolvedBy: resolvedByStr(r.ResolvedBy),
		CreatedAt:  r.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// fromRow converts a basic ManuscriptAnnotation + a caller-supplied author name.
// Used for Create/Update/Resolve where we already know the author from the request context.
func fromRow(r sqlcgen.ManuscriptAnnotation, authorName string) AnnotationResponse {
	return AnnotationResponse{
		ID:         r.ID.String(),
		ProjectID:  r.ProjectID.String(),
		SceneID:    r.SceneID.String(),
		AuthorID:   r.AuthorID.String(),
		AuthorName: authorName,
		StartChar:  r.StartChar,
		EndChar:    r.EndChar,
		Body:       r.Body,
		Type:       r.Type,
		Resolved:   r.Resolved,
		ResolvedBy: resolvedByStr(r.ResolvedBy),
		CreatedAt:  r.CreatedAt.Time.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func resolvedByStr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	s := uuid.UUID(u.Bytes).String()
	return &s
}

// ── service methods ───────────────────────────────────────────────────────────

func (s *Service) ListByScene(ctx context.Context, sceneID uuid.UUID) ([]AnnotationResponse, error) {
	rows, err := s.queries.ListAnnotationsByScene(ctx, sceneID)
	if err != nil {
		return nil, err
	}
	resp := make([]AnnotationResponse, 0, len(rows))
	for _, r := range rows {
		resp = append(resp, fromListRow(r))
	}
	return resp, nil
}

func (s *Service) Create(
	ctx context.Context,
	projectID, sceneID, authorID uuid.UUID,
	authorName string,
	startChar, endChar int32,
	body, annType string,
) (AnnotationResponse, error) {
	row, err := s.queries.CreateAnnotation(ctx, sqlcgen.CreateAnnotationParams{
		ProjectID: projectID,
		SceneID:   sceneID,
		AuthorID:  authorID,
		StartChar: startChar,
		EndChar:   endChar,
		Body:      body,
		Type:      annType,
	})
	if err != nil {
		return AnnotationResponse{}, err
	}
	return fromRow(row, authorName), nil
}

func (s *Service) UpdateBody(ctx context.Context, projectID, annotationID uuid.UUID, body string) (AnnotationResponse, error) {
	row, err := s.queries.UpdateAnnotationBody(ctx, sqlcgen.UpdateAnnotationBodyParams{
		ID:        annotationID,
		ProjectID: projectID,
		Body:      body,
	})
	if err != nil {
		return AnnotationResponse{}, err
	}
	return fromRow(row, ""), nil
}

func (s *Service) Resolve(ctx context.Context, projectID, annotationID, resolverID uuid.UUID) (AnnotationResponse, error) {
	row, err := s.queries.ResolveAnnotation(ctx, sqlcgen.ResolveAnnotationParams{
		ID:        annotationID,
		ProjectID: projectID,
		ResolvedBy: pgtype.UUID{
			Bytes: [16]byte(resolverID),
			Valid:  true,
		},
	})
	if err != nil {
		return AnnotationResponse{}, err
	}
	return fromRow(row, ""), nil
}

func (s *Service) Delete(ctx context.Context, projectID, annotationID uuid.UUID) error {
	return s.queries.DeleteAnnotation(ctx, sqlcgen.DeleteAnnotationParams{
		ID:        annotationID,
		ProjectID: projectID,
	})
}
