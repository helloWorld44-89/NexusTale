package project

import (
	"context"
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
	git     *GitService
}

func NewService(queries *sqlcgen.Queries, git *GitService) *Service {
	return &Service{queries: queries, git: git}
}

// Projects

func (s *Service) CreateProject(ctx context.Context, ownerID uuid.UUID, req CreateProjectRequest) (*ProjectResponse, error) {
	genres := req.Genres
	if genres == nil {
		genres = []string{}
	}

	repoPath := ""
	if s.git != nil {
		var err error
		// Generate a temp ID for the repo path - will be replaced after DB insert
		tempID := uuid.New()
		repoPath, err = s.git.InitRepo(tempID)
		if err != nil {
			return nil, apperror.Internal(fmt.Sprintf("init git repo: %v", err))
		}
	}

	p, err := s.queries.CreateProject(ctx, sqlcgen.CreateProjectParams{
		OwnerID:     ownerID,
		Title:       req.Title,
		Description: req.Description,
		Genres:      genres,
		GitRepoPath: repoPath,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("create project: %v", err))
	}

	return toProjectResponse(p), nil
}

func (s *Service) GetProject(ctx context.Context, id uuid.UUID) (*ProjectResponse, error) {
	p, err := s.queries.GetProject(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("project", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("get project: %v", err))
	}
	return toProjectResponse(p), nil
}

func (s *Service) ListProjects(ctx context.Context, ownerID uuid.UUID) ([]ProjectResponse, error) {
	projects, err := s.queries.ListProjectsByOwner(ctx, ownerID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list projects: %v", err))
	}
	result := make([]ProjectResponse, len(projects))
	for i, p := range projects {
		result[i] = *toProjectResponse(p)
	}
	return result, nil
}

func (s *Service) UpdateProject(ctx context.Context, id uuid.UUID, req UpdateProjectRequest) (*ProjectResponse, error) {
	params := sqlcgen.UpdateProjectParams{ID: id}
	if req.Title != nil {
		params.Title = pgtype.Text{String: *req.Title, Valid: true}
	}
	if req.Description != nil {
		params.Description = pgtype.Text{String: *req.Description, Valid: true}
	}

	p, err := s.queries.UpdateProject(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("project", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("update project: %v", err))
	}
	return toProjectResponse(p), nil
}

func (s *Service) DeleteProject(ctx context.Context, id uuid.UUID) error {
	err := s.queries.DeleteProject(ctx, id)
	if err != nil {
		return apperror.Internal(fmt.Sprintf("delete project: %v", err))
	}
	return nil
}

// Chapters

func (s *Service) CreateChapter(ctx context.Context, projectID uuid.UUID, req CreateChapterRequest) (*ChapterResponse, error) {
	// Verify project exists
	if _, err := s.queries.GetProject(ctx, projectID); errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("project", projectID.String())
	}

	ch, err := s.queries.CreateChapter(ctx, sqlcgen.CreateChapterParams{
		ProjectID: projectID,
		Title:     req.Title,
		Summary:   req.Summary,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("create chapter: %v", err))
	}
	return toChapterResponse(ch), nil
}

func (s *Service) GetChapter(ctx context.Context, id uuid.UUID) (*ChapterResponse, error) {
	ch, err := s.queries.GetChapter(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("chapter", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("get chapter: %v", err))
	}
	return toChapterResponse(ch), nil
}

func (s *Service) ListChapters(ctx context.Context, projectID uuid.UUID) ([]ChapterResponse, error) {
	chapters, err := s.queries.ListChaptersByProject(ctx, projectID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list chapters: %v", err))
	}
	result := make([]ChapterResponse, len(chapters))
	for i, ch := range chapters {
		result[i] = *toChapterResponse(ch)
	}
	return result, nil
}

func (s *Service) UpdateChapter(ctx context.Context, id uuid.UUID, req UpdateChapterRequest) (*ChapterResponse, error) {
	params := sqlcgen.UpdateChapterParams{ID: id}
	if req.Title != nil {
		params.Title = pgtype.Text{String: *req.Title, Valid: true}
	}
	if req.Summary != nil {
		params.Summary = pgtype.Text{String: *req.Summary, Valid: true}
	}
	if req.SortOrder != nil {
		params.SortOrder = pgtype.Int4{Int32: *req.SortOrder, Valid: true}
	}

	ch, err := s.queries.UpdateChapter(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("chapter", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("update chapter: %v", err))
	}
	return toChapterResponse(ch), nil
}

func (s *Service) DeleteChapter(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteChapter(ctx, id)
}

// Scenes

func (s *Service) CreateScene(ctx context.Context, chapterID uuid.UUID, req CreateSceneRequest) (*SceneResponse, error) {
	if _, err := s.queries.GetChapter(ctx, chapterID); errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("chapter", chapterID.String())
	}

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	sc, err := s.queries.CreateScene(ctx, sqlcgen.CreateSceneParams{
		ChapterID: chapterID,
		Title:     req.Title,
		Content:   req.Content,
		Pov:       req.POV,
		Tense:     req.Tense,
		Tags:      tags,
		Summary:   req.Summary,
		SortOrder: req.SortOrder,
		WordCount: countWords(req.Content),
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("create scene: %v", err))
	}
	return toSceneResponse(sc), nil
}

func (s *Service) GetScene(ctx context.Context, id uuid.UUID) (*SceneResponse, error) {
	sc, err := s.queries.GetScene(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("scene", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("get scene: %v", err))
	}
	return toSceneResponse(sc), nil
}

func (s *Service) ListScenes(ctx context.Context, chapterID uuid.UUID) ([]SceneResponse, error) {
	scenes, err := s.queries.ListScenesByChapter(ctx, chapterID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list scenes: %v", err))
	}
	result := make([]SceneResponse, len(scenes))
	for i, sc := range scenes {
		result[i] = *toSceneResponse(sc)
	}
	return result, nil
}

func (s *Service) UpdateScene(ctx context.Context, id uuid.UUID, req UpdateSceneRequest) (*SceneResponse, error) {
	params := sqlcgen.UpdateSceneParams{ID: id}
	if req.Title != nil {
		params.Title = pgtype.Text{String: *req.Title, Valid: true}
	}
	if req.Content != nil {
		params.Content = pgtype.Text{String: *req.Content, Valid: true}
		params.WordCount = pgtype.Int4{Int32: countWords(*req.Content), Valid: true}
	}
	if req.POV != nil {
		params.Pov = pgtype.Text{String: *req.POV, Valid: true}
	}
	if req.Tense != nil {
		params.Tense = pgtype.Text{String: *req.Tense, Valid: true}
	}
	if req.Tags != nil {
		params.Tags = *req.Tags
	}
	if req.Summary != nil {
		params.Summary = pgtype.Text{String: *req.Summary, Valid: true}
	}
	if req.SummaryStale != nil {
		params.SummaryStale = pgtype.Bool{Bool: *req.SummaryStale, Valid: true}
	}
	if req.SortOrder != nil {
		params.SortOrder = pgtype.Int4{Int32: *req.SortOrder, Valid: true}
	}

	sc, err := s.queries.UpdateScene(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("scene", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("update scene: %v", err))
	}
	return toSceneResponse(sc), nil
}

func (s *Service) DeleteScene(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteScene(ctx, id)
}

// ── Git / Chronicle operations ────────────────────────────────────────────────

// Chronicle snapshots all scene content for the project into the git repo and
// creates a new commit (Chronicle) on the current Timeline.
func (s *Service) Chronicle(ctx context.Context, projectID uuid.UUID, req ChronicleRequest) (*ChronicleEntry, error) {
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return nil, apperror.NotFound("project", projectID.String())
	}
	if p.GitRepoPath == "" {
		return nil, apperror.Internal("project has no git repository")
	}

	// Build a snapshot of all scene files: chapters/<chID>/scenes/<scID>.md
	chapters, err := s.queries.ListChaptersByProject(ctx, projectID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list chapters: %v", err))
	}

	files := make(map[string]string)
	for _, ch := range chapters {
		scenes, err := s.queries.ListScenesByChapter(ctx, ch.ID)
		if err != nil {
			return nil, apperror.Internal(fmt.Sprintf("list scenes: %v", err))
		}
		for _, sc := range scenes {
			path := fmt.Sprintf("chapters/%s/scenes/%s.md", ch.ID, sc.ID)
			files[path] = sc.Content
		}
	}

	sha, err := s.git.Chronicle(p.GitRepoPath, req.Note, files)
	if errors.Is(err, ErrNothingToChronicle) {
		// Return current HEAD as the chronicle entry with a flag.
		entries, _ := s.git.Lore(p.GitRepoPath, 1, 1)
		if len(entries) > 0 {
			return &entries[0], ErrNothingToChronicle
		}
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("chronicle: %v", err))
	}

	entries, err := s.git.Lore(p.GitRepoPath, 1, 1)
	if err != nil || len(entries) == 0 {
		return &ChronicleEntry{SHA: sha, ShortSHA: sha[:7], Note: req.Note}, nil
	}
	return &entries[0], nil
}

// Lore returns the paginated commit history for the project's current Timeline.
func (s *Service) Lore(ctx context.Context, projectID uuid.UUID, page, perPage int) ([]ChronicleEntry, error) {
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return nil, apperror.NotFound("project", projectID.String())
	}

	entries, err := s.git.Lore(p.GitRepoPath, page, perPage)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("lore: %v", err))
	}
	return entries, nil
}

// Echo returns a unified diff between two Chronicle SHAs.
func (s *Service) Echo(ctx context.Context, projectID uuid.UUID, fromSHA, toSHA string) (*EchoResponse, error) {
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return nil, apperror.NotFound("project", projectID.String())
	}

	diff, err := s.git.Echo(p.GitRepoPath, fromSHA, toSHA)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("echo: %v", err))
	}
	return &EchoResponse{FromSHA: fromSHA, ToSHA: toSHA, Diff: diff}, nil
}

// GitStatus returns the current Timeline name and the most recent Chronicle.
func (s *Service) GitStatus(ctx context.Context, projectID uuid.UUID) (*GitStatusResponse, error) {
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return nil, apperror.NotFound("project", projectID.String())
	}

	timeline, err := s.git.CurrentTimeline(p.GitRepoPath)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("current timeline: %v", err))
	}

	resp := &GitStatusResponse{CurrentTimeline: timeline}

	entries, err := s.git.Lore(p.GitRepoPath, 1, 1)
	if err == nil && len(entries) > 0 {
		resp.LastChronicle = &entries[0]
	}

	return resp, nil
}

// Timelines lists all branches (Timelines) in the project's git repo.
func (s *Service) Timelines(ctx context.Context, projectID uuid.UUID) ([]TimelineInfo, error) {
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return nil, apperror.NotFound("project", projectID.String())
	}

	timelines, err := s.git.Timelines(p.GitRepoPath)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("timelines: %v", err))
	}
	return timelines, nil
}

// Diverge creates a new Timeline and switches to it.
func (s *Service) Diverge(ctx context.Context, projectID uuid.UUID, req DivergeRequest) (*TimelineInfo, error) {
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return nil, apperror.NotFound("project", projectID.String())
	}
	if req.TimelineName == CanonBranch {
		return nil, apperror.Validation("timeline name 'canon' is reserved")
	}

	if err := s.git.Diverge(p.GitRepoPath, req.TimelineName, req.FromSHA); err != nil {
		return nil, apperror.Internal(fmt.Sprintf("diverge: %v", err))
	}

	timelines, _ := s.git.Timelines(p.GitRepoPath)
	for _, t := range timelines {
		if t.Name == req.TimelineName {
			return &t, nil
		}
	}
	return &TimelineInfo{Name: req.TimelineName, IsActive: true}, nil
}

// TravelTo switches the working tree to an existing Timeline.
func (s *Service) TravelTo(ctx context.Context, projectID uuid.UUID, timelineName string) (*GitStatusResponse, error) {
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return nil, apperror.NotFound("project", projectID.String())
	}

	if err := s.git.TravelTo(p.GitRepoPath, timelineName); err != nil {
		return nil, apperror.Internal(fmt.Sprintf("travel to %s: %v", timelineName, err))
	}

	return s.GitStatus(ctx, projectID)
}

// Canonize merges a Timeline into Canon (fast-forward only in Phase A).
func (s *Service) Canonize(ctx context.Context, projectID uuid.UUID, timelineName string) (*CanonizeResult, error) {
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return nil, apperror.NotFound("project", projectID.String())
	}
	if timelineName == CanonBranch {
		return nil, apperror.Validation("cannot canonize Canon into itself")
	}

	result, err := s.git.Canonize(p.GitRepoPath, timelineName)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("canonize: %v", err))
	}
	return result, nil
}

// Converters

func toProjectResponse(p sqlcgen.Project) *ProjectResponse {
	return &ProjectResponse{
		ID:          p.ID,
		OwnerID:     p.OwnerID,
		Title:       p.Title,
		Description: p.Description,
		Genres:      p.Genres,
		Archived:    p.Archived,
		CreatedAt:   p.CreatedAt.Time,
		UpdatedAt:   p.UpdatedAt.Time,
	}
}

func toChapterResponse(ch sqlcgen.Chapter) *ChapterResponse {
	return &ChapterResponse{
		ID:        ch.ID,
		ProjectID: ch.ProjectID,
		Title:     ch.Title,
		Summary:   ch.Summary,
		SortOrder: ch.SortOrder,
		CreatedAt: ch.CreatedAt.Time,
		UpdatedAt: ch.UpdatedAt.Time,
	}
}

func toSceneResponse(sc sqlcgen.Scene) *SceneResponse {
	return &SceneResponse{
		ID:           sc.ID,
		ChapterID:    sc.ChapterID,
		Title:        sc.Title,
		Content:      sc.Content,
		POV:          sc.Pov,
		Tense:        sc.Tense,
		Tags:         sc.Tags,
		Summary:      sc.Summary,
		SummaryStale: sc.SummaryStale,
		SortOrder:    sc.SortOrder,
		WordCount:    sc.WordCount,
		CreatedAt:    sc.CreatedAt.Time,
		UpdatedAt:    sc.UpdatedAt.Time,
	}
}

// countWords returns the number of whitespace-delimited tokens in s.
func countWords(s string) int32 {
	return int32(len(strings.Fields(s)))
}
