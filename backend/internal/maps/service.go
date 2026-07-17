package maps

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jconder44/nexustale/internal/project"
	"github.com/jconder44/nexustale/pkg/apperror"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// Service manages map entities: a wiki_entities row (type='map') for
// metadata plus a git-backed layout JSON file for content, mirroring how
// scene prose lives in git while its metadata lives in Postgres.
type Service struct {
	queries *sqlcgen.Queries
	git     *project.GitService
}

func NewService(queries *sqlcgen.Queries, git *project.GitService) *Service {
	return &Service{queries: queries, git: git}
}

// repoPathForUser resolves the git working-tree path for (projectID, userID):
// the project owner writes directly to the canonical repo; a collaborator
// writes to their own clone. Same owner/collaborator split as
// project.Service.repoPathForUser, reimplemented here rather than exported
// from that package — the same pattern internal/merge already uses.
func (s *Service) repoPathForUser(ctx context.Context, projectID, userID uuid.UUID) (string, error) {
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return "", apperror.NotFound("project", projectID.String())
	}
	if p.OwnerID == userID {
		return p.GitRepoPath, nil
	}
	collab, err := s.queries.GetCollaborator(ctx, sqlcgen.GetCollaboratorParams{
		ProjectID: projectID,
		UserID:    userID,
	})
	if err != nil {
		return "", apperror.Forbidden("you do not have access to this project's repository")
	}
	return collab.ClonePath, nil
}

// getMapEntity fetches the entity row and verifies it's a map belonging to
// projectID — defense-in-depth beyond the router-level project access check.
func (s *Service) getMapEntity(ctx context.Context, projectID, mapID uuid.UUID) (sqlcgen.GetEntityRow, error) {
	e, err := s.queries.GetEntity(ctx, mapID)
	if err != nil {
		return sqlcgen.GetEntityRow{}, apperror.NotFound("map", mapID.String())
	}
	if e.ProjectID != projectID || e.Type != "map" {
		return sqlcgen.GetEntityRow{}, apperror.NotFound("map", mapID.String())
	}
	return e, nil
}

func toMapResponse(e sqlcgen.GetEntityRow) MapResponse {
	var attrs mapAttrs
	if len(e.Attributes) > 0 {
		_ = json.Unmarshal(e.Attributes, &attrs)
	}
	resp := MapResponse{
		ID:        e.ID,
		ProjectID: e.ProjectID,
		Name:      e.Name,
		MapType:   attrs.MapType,
		CreatedAt: e.CreatedAt.Time,
		UpdatedAt: e.UpdatedAt.Time,
	}
	if e.ParentEntityID.Valid {
		id := uuid.UUID(e.ParentEntityID.Bytes)
		resp.ParentEntityID = &id
	}
	return resp
}

// CreateMap inserts the wiki_entities row and writes a default empty layout
// to the git working tree.
func (s *Service) CreateMap(ctx context.Context, projectID, userID uuid.UUID, req CreateMapRequest) (*MapResponse, error) {
	repoPath, err := s.repoPathForUser(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}

	attrs, _ := json.Marshal(mapAttrs{MapType: req.MapType})

	var parentID pgtype.UUID
	if req.ParentEntityID != nil {
		id, err := uuid.Parse(*req.ParentEntityID)
		if err != nil {
			return nil, apperror.Validation("invalid parent_entity_id")
		}
		parentID = pgtype.UUID{Bytes: [16]byte(id), Valid: true}
	}

	e, err := s.queries.CreateEntity(ctx, sqlcgen.CreateEntityParams{
		ProjectID:      projectID,
		ParentEntityID: parentID,
		Type:           "map",
		Name:           req.Name,
		Summary:        "",
		Attributes:     attrs,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("create map: %v", err))
	}

	if err := s.git.WriteMapFile(repoPath, e.ID, defaultLayout); err != nil {
		return nil, apperror.Internal(fmt.Sprintf("write map layout: %v", err))
	}

	resp := toMapResponse(sqlcgen.GetEntityRow(e))
	resp.Layout = json.RawMessage(defaultLayout)
	return &resp, nil
}

// GetMap returns map metadata plus its layout JSON read from git. Falls
// back to the default empty layout if the file hasn't been written yet
// (shouldn't normally happen since CreateMap always writes one).
func (s *Service) GetMap(ctx context.Context, projectID, userID, mapID uuid.UUID) (*MapResponse, error) {
	e, err := s.getMapEntity(ctx, projectID, mapID)
	if err != nil {
		return nil, err
	}

	repoPath, err := s.repoPathForUser(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}

	layout, exists, err := s.git.ReadMapFile(repoPath, mapID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("read map layout: %v", err))
	}
	if !exists {
		layout = defaultLayout
	}

	resp := toMapResponse(e)
	resp.Layout = json.RawMessage(layout)
	return &resp, nil
}

// ListMaps returns metadata for every map in the project. Layout is omitted
// — reading it requires a git file read per row, too slow for a list view.
func (s *Service) ListMaps(ctx context.Context, projectID uuid.UUID) ([]MapResponse, error) {
	entities, err := s.queries.ListEntitiesByProject(ctx, sqlcgen.ListEntitiesByProjectParams{
		ProjectID: projectID,
		Type:      pgtype.Text{String: "map", Valid: true},
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list maps: %v", err))
	}

	result := make([]MapResponse, len(entities))
	for i, e := range entities {
		result[i] = toMapResponse(sqlcgen.GetEntityRow(e))
	}
	return result, nil
}

// UpdateMap independently updates name, map_type, and/or layout — each is a
// no-op when its request field is nil.
func (s *Service) UpdateMap(ctx context.Context, projectID, userID, mapID uuid.UUID, req UpdateMapRequest) (*MapResponse, error) {
	e, err := s.getMapEntity(ctx, projectID, mapID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		updated, err := s.queries.UpdateEntity(ctx, sqlcgen.UpdateEntityParams{
			ID:   mapID,
			Name: pgtype.Text{String: *req.Name, Valid: true},
		})
		if err != nil {
			return nil, apperror.Internal(fmt.Sprintf("update map name: %v", err))
		}
		e = sqlcgen.GetEntityRow(updated)
	}

	if req.MapType != nil {
		attrs, _ := json.Marshal(mapAttrs{MapType: *req.MapType})
		updated, err := s.queries.UpdateEntityAttributes(ctx, sqlcgen.UpdateEntityAttributesParams{
			ID:         mapID,
			Attributes: attrs,
		})
		if err != nil {
			return nil, apperror.Internal(fmt.Sprintf("update map type: %v", err))
		}
		e = sqlcgen.GetEntityRow(updated)
	}

	repoPath, err := s.repoPathForUser(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}

	resp := toMapResponse(e)

	if req.Layout != nil {
		if err := s.git.WriteMapFile(repoPath, mapID, string(*req.Layout)); err != nil {
			return nil, apperror.Internal(fmt.Sprintf("write map layout: %v", err))
		}
		resp.Layout = *req.Layout
	} else {
		layout, exists, err := s.git.ReadMapFile(repoPath, mapID)
		if err != nil {
			return nil, apperror.Internal(fmt.Sprintf("read map layout: %v", err))
		}
		if !exists {
			layout = defaultLayout
		}
		resp.Layout = json.RawMessage(layout)
	}

	return &resp, nil
}

// DeleteMap removes the wiki_entities row. The git layout file is left in
// the working tree — harmless (an orphaned file until the next Chronicle),
// and no other content type in this codebase cleans up its git file on
// delete either.
func (s *Service) DeleteMap(ctx context.Context, projectID, mapID uuid.UUID) error {
	if _, err := s.getMapEntity(ctx, projectID, mapID); err != nil {
		return err
	}
	if err := s.queries.DeleteEntity(ctx, mapID); err != nil {
		return apperror.Internal(fmt.Sprintf("delete map: %v", err))
	}
	return nil
}
