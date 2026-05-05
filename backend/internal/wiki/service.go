package wiki

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jconder44/nexustale/pkg/apperror"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
	"github.com/jconder44/nexustale/pkg/storage"
)

// imageURLExpiry is how long presigned entity image URLs remain valid.
// Short enough to limit exposure; long enough for a typical editing session.
const imageURLExpiry = 4 * time.Hour

type Service struct {
	queries *sqlcgen.Queries
	store   *storage.Client
	tagger  *tagger
}

func NewService(queries *sqlcgen.Queries, store *storage.Client) *Service {
	return &Service{queries: queries, store: store, tagger: newTagger()}
}

// ========================
// Entities
// ========================

func (s *Service) CreateEntity(ctx context.Context, projectID uuid.UUID, req CreateEntityRequest) (*EntityResponse, error) {
	// Default empty attributes to a valid JSON object so the field is never null.
	if len(req.Attributes) == 0 {
		req.Attributes = json.RawMessage("{}")
	}

	// Pre-populate summary with the type template when none was provided.
	if req.Summary == "" {
		req.Summary = entityTemplates[req.Type]
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
	return s.entityWithURL(ctx, e)
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

	return s.entityWithURL(ctx, e)
}

func (s *Service) DeleteEntity(ctx context.Context, id uuid.UUID) error {
	// Remove the stored image (if any) before deleting the entity record.
	e, err := s.queries.GetEntity(ctx, id)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return apperror.Internal(fmt.Sprintf("get entity for delete: %v", err))
	}
	if err == nil && e.ImageKey.Valid && s.store != nil {
		_ = s.store.DeleteObject(ctx, e.ImageKey.String)
	}

	if err := s.queries.DeleteEntity(ctx, id); err != nil {
		return apperror.Internal(fmt.Sprintf("delete entity: %v", err))
	}
	return nil
}

// UploadEntityImage stores r as the entity's portrait image in MinIO and
// persists the object key in the DB. Any previously stored image is deleted.
// key format: wiki/entities/<entityID>/<filename>
func (s *Service) UploadEntityImage(ctx context.Context, entityID uuid.UUID, filename, contentType string, r io.Reader, size int64) (*EntityResponse, error) {
	if s.store == nil {
		return nil, apperror.Internal("storage not configured")
	}

	// Fetch current record to clean up old image if present.
	current, err := s.queries.GetEntity(ctx, entityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("entity", entityID.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("get entity: %v", err))
	}

	key := fmt.Sprintf("wiki/entities/%s/%s", entityID, filename)

	if err := s.store.PutObject(ctx, key, contentType, r, size); err != nil {
		return nil, apperror.Internal(fmt.Sprintf("upload image: %v", err))
	}

	// Delete the old image after successful upload so we don't orphan objects.
	if current.ImageKey.Valid && current.ImageKey.String != key {
		_ = s.store.DeleteObject(ctx, current.ImageKey.String)
	}

	updated, err := s.queries.UpdateEntityImage(ctx, sqlcgen.UpdateEntityImageParams{
		ID:       entityID,
		ImageKey: pgtype.Text{String: key, Valid: true},
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("save image key: %v", err))
	}
	return s.entityWithURL(ctx, updated)
}

// DeleteEntityImage removes the stored image from MinIO and clears the DB key.
func (s *Service) DeleteEntityImage(ctx context.Context, entityID uuid.UUID) (*EntityResponse, error) {
	current, err := s.queries.GetEntity(ctx, entityID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("entity", entityID.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("get entity: %v", err))
	}

	if current.ImageKey.Valid && s.store != nil {
		if err := s.store.DeleteObject(ctx, current.ImageKey.String); err != nil {
			return nil, apperror.Internal(fmt.Sprintf("delete image from storage: %v", err))
		}
	}

	updated, err := s.queries.ClearEntityImage(ctx, entityID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("clear image key: %v", err))
	}
	return toEntityResponse(updated), nil
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
	attrsJSON, err := json.Marshal(req.Attrs)
	if err != nil {
		attrsJSON = json.RawMessage("{}")
	}
	r, err := s.queries.CreateMagicRule(ctx, sqlcgen.CreateMagicRuleParams{
		ProjectID:   projectID,
		Name:        req.Name,
		Description: req.Description,
		Attributes:  attrsJSON,
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
	if req.Attrs != nil {
		b, err := json.Marshal(req.Attrs)
		if err == nil {
			params.Attributes = b
		}
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
	hasAbsolute := req.Year != nil || req.Month != nil || req.Day != nil
	if req.AnchorEventID != nil && hasAbsolute {
		return nil, apperror.Validation("anchor_event_id and absolute year/month/day are mutually exclusive")
	}

	entityID := pgtype.UUID{}
	if req.EntityID != nil {
		entityID = pgtype.UUID{Bytes: [16]byte(*req.EntityID), Valid: true}
	}
	anchorEventID := pgtype.UUID{}
	if req.AnchorEventID != nil {
		anchorEventID = pgtype.UUID{Bytes: [16]byte(*req.AnchorEventID), Valid: true}
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
	offsetYear := pgtype.Int4{}
	if req.AnchorOffsetYear != nil {
		offsetYear = pgtype.Int4{Int32: *req.AnchorOffsetYear, Valid: true}
	}
	offsetMonth := pgtype.Int4{}
	if req.AnchorOffsetMonth != nil {
		offsetMonth = pgtype.Int4{Int32: *req.AnchorOffsetMonth, Valid: true}
	}
	offsetDay := pgtype.Int4{}
	if req.AnchorOffsetDay != nil {
		offsetDay = pgtype.Int4{Int32: *req.AnchorOffsetDay, Valid: true}
	}

	e, err := s.queries.CreateTimelineEvent(ctx, sqlcgen.CreateTimelineEventParams{
		ProjectID:          projectID,
		EntityID:           entityID,
		Name:               req.Name,
		Description:        req.Description,
		Era:                req.Era,
		Year:               year,
		Month:              month,
		Day:                day,
		AnchorEventID:      anchorEventID,
		AnchorOffsetYear:   offsetYear,
		AnchorOffsetMonth:  offsetMonth,
		AnchorOffsetDay:    offsetDay,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("create timeline event: %v", err))
	}

	// Resolve the single event against an empty peer set (no anchor chain needed
	// for a freshly created event that references events already in the DB — the
	// full resolution happens on List). Return raw values for the create response.
	return toTimelineEventResponse(e), nil
}

func (s *Service) ListTimelineEvents(ctx context.Context, projectID uuid.UUID) ([]TimelineEventResponse, error) {
	rows, err := s.queries.ListTimelineEventsByProject(ctx, projectID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list timeline events: %v", err))
	}
	result, err := ResolveEvents(rows)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("resolve timeline anchors: %v", err))
	}
	return result, nil
}

func (s *Service) UpdateTimelineEvent(ctx context.Context, id uuid.UUID, req UpdateTimelineEventRequest) (*TimelineEventResponse, error) {
	hasAbsolute := req.Year != nil || req.Month != nil || req.Day != nil
	if req.AnchorEventID != nil && hasAbsolute {
		return nil, apperror.Validation("anchor_event_id and absolute year/month/day are mutually exclusive")
	}

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
	if req.Year != nil {
		params.Year = pgtype.Int4{Int32: *req.Year, Valid: true}
	}
	if req.Month != nil {
		params.Month = pgtype.Int4{Int32: *req.Month, Valid: true}
	}
	if req.Day != nil {
		params.Day = pgtype.Int4{Int32: *req.Day, Valid: true}
	}
	if req.AnchorEventID != nil {
		params.AnchorEventID = pgtype.UUID{Bytes: [16]byte(*req.AnchorEventID), Valid: true}
	}
	if req.AnchorOffsetYear != nil {
		params.AnchorOffsetYear = pgtype.Int4{Int32: *req.AnchorOffsetYear, Valid: true}
	}
	if req.AnchorOffsetMonth != nil {
		params.AnchorOffsetMonth = pgtype.Int4{Int32: *req.AnchorOffsetMonth, Valid: true}
	}
	if req.AnchorOffsetDay != nil {
		params.AnchorOffsetDay = pgtype.Int4{Int32: *req.AnchorOffsetDay, Valid: true}
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

// toEntityResponse builds an EntityResponse without a presigned image URL.
// Used for list operations where calling MinIO per-row would be too slow.
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

// entityWithURL builds an EntityResponse and, if an image_key is stored,
// calls MinIO to generate a short-lived presigned GET URL for it.
func (s *Service) entityWithURL(ctx context.Context, e sqlcgen.WikiEntity) (*EntityResponse, error) {
	resp := toEntityResponse(e)
	if e.ImageKey.Valid && e.ImageKey.String != "" && s.store != nil {
		url, err := s.store.PresignedGetURL(ctx, e.ImageKey.String, imageURLExpiry)
		if err != nil {
			return nil, apperror.Internal(fmt.Sprintf("presign image url: %v", err))
		}
		resp.ImageURL = &url
	}
	return resp, nil
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
	var attrs MagicRuleAttributes
	if len(r.Attributes) > 0 {
		_ = json.Unmarshal(r.Attributes, &attrs)
	}
	return &MagicRuleResponse{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		Name:        r.Name,
		Description: r.Description,
		Attributes:  attrs,
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
	if e.AnchorEventID.Valid {
		id := uuid.UUID(e.AnchorEventID.Bytes)
		resp.AnchorEventID = &id
		if e.AnchorOffsetYear.Valid {
			v := e.AnchorOffsetYear.Int32
			resp.AnchorOffsetYear = &v
		}
		if e.AnchorOffsetMonth.Valid {
			v := e.AnchorOffsetMonth.Int32
			resp.AnchorOffsetMonth = &v
		}
		if e.AnchorOffsetDay.Valid {
			v := e.AnchorOffsetDay.Int32
			resp.AnchorOffsetDay = &v
		}
	}
	return resp
}

// ========================
// Scene entity mentions
// ========================

// ListSceneMentions returns all active (non-suppressed) mention rows for a
// scene on a given branch, including the entity name and type for display.
func (s *Service) ListSceneMentions(ctx context.Context, sceneID uuid.UUID, branchName string) ([]MentionResponse, error) {
	rows, err := s.queries.ListMentionsByScene(ctx, sqlcgen.ListMentionsBySceneParams{
		SceneID:    sceneID,
		BranchName: branchName,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list mentions: %v", err))
	}

	out := make([]MentionResponse, len(rows))
	for i, r := range rows {
		out[i] = MentionResponse{
			ID:         r.ID,
			SceneID:    r.SceneID,
			EntityID:   r.EntityID,
			EntityName: r.EntityName,
			EntityType: r.EntityType,
			MatchText:  r.MatchText,
			BranchName: r.BranchName,
			CreatedAt:  r.CreatedAt.Time,
		}
	}
	return out, nil
}

// SuppressMention marks a single mention as suppressed so it is excluded
// from the MentionsBar and AI context. The tagger will not re-add it on the
// next detection pass.
func (s *Service) SuppressMention(ctx context.Context, mentionID uuid.UUID) error {
	if err := s.queries.SuppressMention(ctx, mentionID); err != nil {
		return apperror.Internal(fmt.Sprintf("suppress mention: %v", err))
	}
	return nil
}

// SuppressAllMentions suppresses every active mention for a scene on a branch.
func (s *Service) SuppressAllMentions(ctx context.Context, sceneID uuid.UUID, branchName string) error {
	if err := s.queries.SuppressAllMentions(ctx, sqlcgen.SuppressAllMentionsParams{
		SceneID:    sceneID,
		BranchName: branchName,
	}); err != nil {
		return apperror.Internal(fmt.Sprintf("suppress all mentions: %v", err))
	}
	return nil
}

// ListEntityAppearances returns all scenes (with chapter context) that mention
// the given entity on a specific branch (suppressed mentions excluded).
func (s *Service) ListEntityAppearances(ctx context.Context, entityID uuid.UUID, branchName string) ([]EntityAppearance, error) {
	rows, err := s.queries.ListScenesByEntity(ctx, sqlcgen.ListScenesByEntityParams{
		EntityID:   entityID,
		BranchName: branchName,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list appearances: %v", err))
	}

	out := make([]EntityAppearance, len(rows))
	for i, r := range rows {
		out[i] = EntityAppearance{
			SceneID:      r.SceneID,
			SceneTitle:   r.SceneTitle,
			SceneOrder:   r.SceneOrder,
			ChapterID:    r.ChapterID,
			ChapterTitle: r.ChapterTitle,
			ChapterOrder: r.ChapterOrder,
			BranchName:   r.BranchName,
		}
	}
	return out, nil
}
