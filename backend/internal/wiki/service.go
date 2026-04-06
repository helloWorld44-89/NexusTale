package wiki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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

// ========================
// Entities
// ========================

func (s *Service) CreateEntity(ctx context.Context, projectID uuid.UUID, req CreateEntityRequest) (*EntityResponse, error) {
	// Default empty attributes to a valid JSON object so the field is never null.
	if len(req.Attributes) == 0 {
		req.Attributes = json.RawMessage("{}")
	}

	// Convert the optional *uuid.UUID to pgtype.UUID which pgx understands as nullable.
	parentID := pgtype.UUID{}
	if req.ParentEntityID != nil {
		parentID = pgtype.UUID{Bytes: [16]byte(*req.ParentEntityID), Valid: true}
	}

	e, err := s.queries.CreateEntity(ctx, sqlcgen.CreateEntityParams{
		ProjectID:      projectID,
		ParentEntityID: parentID,
		Type:           req.Type,
		Name:           req.Name,
		Summary:        req.Summary,
		Attributes:     req.Attributes,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("create entity: %v", err))
	}
	return toEntityResponse(e), nil
}

func (s *Service) GetEntity(ctx context.Context, id uuid.UUID) (*EntityResponse, error) {
	e, err := s.queries.GetEntity(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("entity", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("get entity: %v", err))
	}
	return toEntityResponse(e), nil
}

// ListEntities returns entities for a project, optionally filtered by type.
// Pass an empty string to get all types.
func (s *Service) ListEntities(ctx context.Context, projectID uuid.UUID, entityType string) ([]EntityResponse, error) {
	typeFilter := pgtype.Text{}
	if entityType != "" {
		typeFilter = pgtype.Text{String: entityType, Valid: true}
	}

	entities, err := s.queries.ListEntitiesByProject(ctx, sqlcgen.ListEntitiesByProjectParams{
		ProjectID: projectID,
		Type:      typeFilter,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list entities: %v", err))
	}
	return toEntityResponses(entities), nil
}

// ListChildEntities returns all entities whose parent is the given entity ID.
// Typical use: lore entries parented to a location.
func (s *Service) ListChildEntities(ctx context.Context, parentID uuid.UUID) ([]EntityResponse, error) {
	entities, err := s.queries.ListEntitiesByParent(ctx, pgtype.UUID{Bytes: [16]byte(parentID), Valid: true})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list child entities: %v", err))
	}
	return toEntityResponses(entities), nil
}

func (s *Service) UpdateEntity(ctx context.Context, id uuid.UUID, req UpdateEntityRequest) (*EntityResponse, error) {
	params := sqlcgen.UpdateEntityParams{ID: id}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Summary != nil {
		params.Summary = pgtype.Text{String: *req.Summary, Valid: true}
	}

	e, err := s.queries.UpdateEntity(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("entity", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("update entity: %v", err))
	}

	// Attributes are a full replace, handled separately to avoid COALESCE on JSONB.
	if len(req.Attributes) > 0 {
		e, err = s.queries.UpdateEntityAttributes(ctx, sqlcgen.UpdateEntityAttributesParams{
			ID:         id,
			Attributes: req.Attributes,
		})
		if err != nil {
			return nil, apperror.Internal(fmt.Sprintf("update entity attributes: %v", err))
		}
	}

	return toEntityResponse(e), nil
}

func (s *Service) DeleteEntity(ctx context.Context, id uuid.UUID) error {
	if err := s.queries.DeleteEntity(ctx, id); err != nil {
		return apperror.Internal(fmt.Sprintf("delete entity: %v", err))
	}
	return nil
}

// Autolink scans text for entity name mentions and returns the matched entities.
// The scan is case-insensitive and done in Go — acceptable for novel-sized wikis.
func (s *Service) Autolink(ctx context.Context, projectID uuid.UUID, text string) ([]EntityResponse, error) {
	all, err := s.ListEntities(ctx, projectID, "")
	if err != nil {
		return nil, err
	}

	lower := strings.ToLower(text)
	matched := []EntityResponse{}
	for _, e := range all {
		if strings.Contains(lower, strings.ToLower(e.Name)) {
			matched = append(matched, e)
		}
	}
	return matched, nil
}

// ========================
// Relationships
// ========================

func (s *Service) CreateRelationship(ctx context.Context, projectID uuid.UUID, req CreateRelationshipRequest) (*RelationshipResponse, error) {
	r, err := s.queries.CreateRelationship(ctx, sqlcgen.CreateRelationshipParams{
		ProjectID:    projectID,
		FromEntityID: req.FromEntityID,
		ToEntityID:   req.ToEntityID,
		Type:         req.Type,
		Description:  req.Description,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("create relationship: %v", err))
	}
	return toRelationshipResponse(r), nil
}

func (s *Service) ListRelationships(ctx context.Context, projectID uuid.UUID) ([]RelationshipResponse, error) {
	rows, err := s.queries.ListRelationshipsByProject(ctx, projectID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list relationships: %v", err))
	}
	result := make([]RelationshipResponse, len(rows))
	for i, r := range rows {
		result[i] = *toRelationshipResponse(r)
	}
	return result, nil
}

func (s *Service) DeleteRelationship(ctx context.Context, id uuid.UUID) error {
	if err := s.queries.DeleteRelationship(ctx, id); err != nil {
		return apperror.Internal(fmt.Sprintf("delete relationship: %v", err))
	}
	return nil
}

// GetGraph returns all entities and relationships for a project so the frontend
// can render a node-edge diagram without making two separate requests.
func (s *Service) GetGraph(ctx context.Context, projectID uuid.UUID) (*GraphResponse, error) {
	entities, err := s.ListEntities(ctx, projectID, "")
	if err != nil {
		return nil, err
	}
	relationships, err := s.ListRelationships(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return &GraphResponse{Entities: entities, Relationships: relationships}, nil
}

// ========================
// Magic Rules
// ========================

func (s *Service) CreateMagicRule(ctx context.Context, projectID uuid.UUID, req CreateMagicRuleRequest) (*MagicRuleResponse, error) {
	r, err := s.queries.CreateMagicRule(ctx, sqlcgen.CreateMagicRuleParams{
		ProjectID:   projectID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("create magic rule: %v", err))
	}
	return toMagicRuleResponse(r), nil
}

func (s *Service) ListMagicRules(ctx context.Context, projectID uuid.UUID) ([]MagicRuleResponse, error) {
	rows, err := s.queries.ListMagicRulesByProject(ctx, projectID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list magic rules: %v", err))
	}
	result := make([]MagicRuleResponse, len(rows))
	for i, r := range rows {
		result[i] = *toMagicRuleResponse(r)
	}
	return result, nil
}

func (s *Service) UpdateMagicRule(ctx context.Context, id uuid.UUID, req UpdateMagicRuleRequest) (*MagicRuleResponse, error) {
	params := sqlcgen.UpdateMagicRuleParams{ID: id}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}

	r, err := s.queries.UpdateMagicRule(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("magic rule", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("update magic rule: %v", err))
	}
	return toMagicRuleResponse(r), nil
}

func (s *Service) DeleteMagicRule(ctx context.Context, id uuid.UUID) error {
	if err := s.queries.DeleteMagicRule(ctx, id); err != nil {
		return apperror.Internal(fmt.Sprintf("delete magic rule: %v", err))
	}
	return nil
}

// ========================
// Timeline Events
// ========================

func (s *Service) CreateTimelineEvent(ctx context.Context, projectID uuid.UUID, req CreateTimelineEventRequest) (*TimelineEventResponse, error) {
	entityID := pgtype.UUID{}
	if req.EntityID != nil {
		entityID = pgtype.UUID{Bytes: [16]byte(*req.EntityID), Valid: true}
	}

	year := pgtype.Int4{}
	if req.Year != nil {
		year = pgtype.Int4{Int32: *req.Year, Valid: true}
	}
	month := pgtype.Int4{}
	if req.Month != nil {
		month = pgtype.Int4{Int32: *req.Month, Valid: true}
	}
	day := pgtype.Int4{}
	if req.Day != nil {
		day = pgtype.Int4{Int32: *req.Day, Valid: true}
	}

	e, err := s.queries.CreateTimelineEvent(ctx, sqlcgen.CreateTimelineEventParams{
		ProjectID:   projectID,
		EntityID:    entityID,
		Name:        req.Name,
		Description: req.Description,
		Era:         req.Era,
		Year:        year,
		Month:       month,
		Day:         day,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("create timeline event: %v", err))
	}
	return toTimelineEventResponse(e), nil
}

func (s *Service) ListTimelineEvents(ctx context.Context, projectID uuid.UUID) ([]TimelineEventResponse, error) {
	rows, err := s.queries.ListTimelineEventsByProject(ctx, projectID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list timeline events: %v", err))
	}
	result := make([]TimelineEventResponse, len(rows))
	for i, e := range rows {
		result[i] = *toTimelineEventResponse(e)
	}
	return result, nil
}

func (s *Service) UpdateTimelineEvent(ctx context.Context, id uuid.UUID, req UpdateTimelineEventRequest) (*TimelineEventResponse, error) {
	params := sqlcgen.UpdateTimelineEventParams{ID: id}
	if req.Name != nil {
		params.Name = pgtype.Text{String: *req.Name, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}
	if req.Era != nil {
		params.Era = pgtype.Text{String: *req.Era, Valid: true}
	}

	e, err := s.queries.UpdateTimelineEvent(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("timeline event", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("update timeline event: %v", err))
	}
	return toTimelineEventResponse(e), nil
}

func (s *Service) DeleteTimelineEvent(ctx context.Context, id uuid.UUID) error {
	if err := s.queries.DeleteTimelineEvent(ctx, id); err != nil {
		return apperror.Internal(fmt.Sprintf("delete timeline event: %v", err))
	}
	return nil
}

// ========================
// Converters (sqlcgen → response DTOs)
// ========================

func toEntityResponse(e sqlcgen.WikiEntity) *EntityResponse {
	resp := &EntityResponse{
		ID:         e.ID,
		ProjectID:  e.ProjectID,
		Type:       e.Type,
		Name:       e.Name,
		Summary:    e.Summary,
		Attributes: e.Attributes,
		CreatedAt:  e.CreatedAt.Time,
		UpdatedAt:  e.UpdatedAt.Time,
	}
	if e.ParentEntityID.Valid {
		id := uuid.UUID(e.ParentEntityID.Bytes)
		resp.ParentEntityID = &id
	}
	return resp
}

func toEntityResponses(rows []sqlcgen.WikiEntity) []EntityResponse {
	result := make([]EntityResponse, len(rows))
	for i, e := range rows {
		result[i] = *toEntityResponse(e)
	}
	return result
}

func toRelationshipResponse(r sqlcgen.WikiRelationship) *RelationshipResponse {
	return &RelationshipResponse{
		ID:           r.ID,
		ProjectID:    r.ProjectID,
		FromEntityID: r.FromEntityID,
		ToEntityID:   r.ToEntityID,
		Type:         r.Type,
		Description:  r.Description,
		CreatedAt:    r.CreatedAt.Time,
	}
}

func toMagicRuleResponse(r sqlcgen.WikiMagicRule) *MagicRuleResponse {
	return &MagicRuleResponse{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		Name:        r.Name,
		Description: r.Description,
		CreatedAt:   r.CreatedAt.Time,
		UpdatedAt:   r.UpdatedAt.Time,
	}
}

func toTimelineEventResponse(e sqlcgen.WikiTimelineEvent) *TimelineEventResponse {
	resp := &TimelineEventResponse{
		ID:          e.ID,
		ProjectID:   e.ProjectID,
		Name:        e.Name,
		Description: e.Description,
		Era:         e.Era,
		CreatedAt:   e.CreatedAt.Time,
		UpdatedAt:   e.UpdatedAt.Time,
	}
	if e.EntityID.Valid {
		id := uuid.UUID(e.EntityID.Bytes)
		resp.EntityID = &id
	}
	if e.Year.Valid {
		v := e.Year.Int32
		resp.Year = &v
	}
	if e.Month.Valid {
		v := e.Month.Int32
		resp.Month = &v
	}
	if e.Day.Valid {
		v := e.Day.Int32
		resp.Day = &v
	}
	return resp
}
