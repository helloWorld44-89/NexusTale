package project

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jconder44/nexustale/pkg/apperror"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// SummaryNotifier is implemented by the AI service (internal/ai.Service).
// Using an interface here breaks the import cycle between project ↔ ai.
type SummaryNotifier interface {
	// ScheduleSummarize marks a chapter summary stale and debounces LLM regen.
	ScheduleSummarize(userID, chapterID, projectID uuid.UUID, branchName string)
	// CancelSummarize cancels any pending debounce timers for the given chapter
	// so deleted chapters don't trigger spurious LLM calls.
	CancelSummarize(chapterID uuid.UUID)
	// UpsertActiveBranch records which Timeline a user is on for a project.
	UpsertActiveBranch(ctx context.Context, projectID, userID uuid.UUID, branchName string)
	// CleanupBranch deletes summary rows and active-branch pointers for a
	// merged Timeline (called by Canonize).
	CleanupBranch(ctx context.Context, projectID uuid.UUID, branchName string)
}

type Service struct {
	queries  *sqlcgen.Queries
	git      *GitService
	notifier SummaryNotifier // optional; nil → branch tracking disabled
}

func NewService(queries *sqlcgen.Queries, git *GitService) *Service {
	return &Service{queries: queries, git: git}
}

// WithNotifier wires the AI summary notifier into the project service.
// Called from cmd/api during startup after both services are initialised.
func (s *Service) WithNotifier(n SummaryNotifier) {
	s.notifier = n
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

	// Every project starts with a default act. Writers who don't care about
	// acts will never see it; the frontend hides the act layer when only one
	// act exists with the default title.
	if _, err := s.queries.CreateAct(ctx, sqlcgen.CreateActParams{
		ProjectID: p.ID,
		Title:     "Act 1",
		Summary:   "",
		SortOrder: 0,
	}); err != nil {
		return nil, apperror.Internal(fmt.Sprintf("create default act: %v", err))
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

func (s *Service) ListProjects(ctx context.Context, userID uuid.UUID) ([]ProjectResponse, error) {
	projects, err := s.queries.ListProjectsForUser(ctx, userID)
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

func (s *Service) GetProjectStats(ctx context.Context, id uuid.UUID) (*ProjectStatsResponse, error) {
	row, err := s.queries.GetProjectStats(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("project", "not found")
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("get project stats: %v", err))
	}

	// GREATEST returns interface{} — assert to time.Time (pgx returns pgtype.Timestamptz or time.Time).
	var lastUpdated time.Time
	switch v := row.LastUpdatedAt.(type) {
	case time.Time:
		lastUpdated = v
	default:
		lastUpdated = time.Now()
	}

	return &ProjectStatsResponse{
		SceneCount:     row.SceneCount,
		ChapterCount:   row.ChapterCount,
		TotalWordCount: row.TotalWordCount,
		LastUpdatedAt:  lastUpdated,
	}, nil
}

// Acts

func (s *Service) CreateAct(ctx context.Context, projectID uuid.UUID, req CreateActRequest) (*ActResponse, error) {
	if _, err := s.queries.GetProject(ctx, projectID); errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("project", projectID.String())
	}

	act, err := s.queries.CreateAct(ctx, sqlcgen.CreateActParams{
		ProjectID: projectID,
		Title:     req.Title,
		Summary:   req.Summary,
		SortOrder: req.SortOrder,
	})
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("create act: %v", err))
	}
	return toActResponse(act), nil
}

func (s *Service) GetAct(ctx context.Context, id uuid.UUID) (*ActResponse, error) {
	act, err := s.queries.GetAct(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("act", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("get act: %v", err))
	}
	return toActResponse(act), nil
}

func (s *Service) ListActs(ctx context.Context, projectID uuid.UUID) ([]ActResponse, error) {
	acts, err := s.queries.ListActsByProject(ctx, projectID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list acts: %v", err))
	}
	result := make([]ActResponse, len(acts))
	for i, act := range acts {
		result[i] = *toActResponse(act)
	}
	return result, nil
}

func (s *Service) UpdateAct(ctx context.Context, id uuid.UUID, req UpdateActRequest) (*ActResponse, error) {
	params := sqlcgen.UpdateActParams{ID: id}
	if req.Title != nil {
		params.Title = pgtype.Text{String: *req.Title, Valid: true}
	}
	if req.Summary != nil {
		params.Summary = pgtype.Text{String: *req.Summary, Valid: true}
	}
	if req.SortOrder != nil {
		params.SortOrder = pgtype.Int4{Int32: *req.SortOrder, Valid: true}
	}

	act, err := s.queries.UpdateAct(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("act", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("update act: %v", err))
	}
	return toActResponse(act), nil
}

func (s *Service) DeleteAct(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteAct(ctx, id)
}

// Chapters

func (s *Service) CreateChapter(ctx context.Context, actID uuid.UUID, req CreateChapterRequest) (*ChapterResponse, error) {
	act, err := s.queries.GetAct(ctx, actID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("act", actID.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("get act: %v", err))
	}

	ch, err := s.queries.CreateChapter(ctx, sqlcgen.CreateChapterParams{
		ProjectID: act.ProjectID,
		ActID:     actID,
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

func (s *Service) ListChaptersByAct(ctx context.Context, actID uuid.UUID) ([]ChapterResponse, error) {
	chapters, err := s.queries.ListChaptersByAct(ctx, actID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list chapters: %v", err))
	}
	result := make([]ChapterResponse, len(chapters))
	for i, ch := range chapters {
		result[i] = *toChapterResponse(ch)
	}
	return result, nil
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
	if s.notifier != nil {
		s.notifier.CancelSummarize(id)
	}
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

	// Write content to git working tree (content is no longer stored in Postgres).
	if req.ProjectID != uuid.Nil {
		if repoPath, _, rErr := s.repoPathForUser(ctx, req.ProjectID, req.UserID); rErr == nil {
			if wErr := s.git.WriteSceneFile(repoPath, sc.ChapterID, sc.ID, req.Content); wErr != nil {
				slog.Warn("git scene write failed on create", "scene_id", sc.ID, "error", wErr)
			}
		}
	}

	resp := toSceneResponse(sc)
	resp.Content = req.Content // carry from request; not stored in DB
	return resp, nil
}

func (s *Service) GetScene(ctx context.Context, id, projectID, userID uuid.UUID) (*SceneResponse, error) {
	sc, err := s.queries.GetScene(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("scene", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("get scene: %v", err))
	}
	resp := toSceneResponse(sc)
	if projectID != uuid.Nil && s.git != nil {
		if repoPath, _, rErr := s.repoPathForUser(ctx, projectID, userID); rErr == nil {
			if content, ok, _ := s.git.ReadSceneFile(repoPath, sc.ChapterID, sc.ID); ok {
				resp.Content = content
			}
		}
	}
	return resp, nil
}

func (s *Service) ListScenes(ctx context.Context, chapterID, projectID, userID uuid.UUID) ([]SceneResponse, error) {
	scenes, err := s.queries.ListScenesByChapter(ctx, chapterID)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("list scenes: %v", err))
	}
	var repoPath string
	if projectID != uuid.Nil && s.git != nil {
		if rp, _, rErr := s.repoPathForUser(ctx, projectID, userID); rErr == nil {
			repoPath = rp
		}
	}
	result := make([]SceneResponse, len(scenes))
	for i, sc := range scenes {
		r := toSceneResponse(sc)
		if repoPath != "" {
			if content, ok, _ := s.git.ReadSceneFile(repoPath, sc.ChapterID, sc.ID); ok {
				r.Content = content
			}
		}
		result[i] = *r
	}
	return result, nil
}

func (s *Service) UpdateScene(ctx context.Context, id uuid.UUID, req UpdateSceneRequest) (*SceneResponse, error) {
	params := sqlcgen.UpdateSceneParams{ID: id}
	if req.Title != nil {
		params.Title = pgtype.Text{String: *req.Title, Valid: true}
	}
	if req.Content != nil {
		// Content no longer stored in DB; compute word count from incoming value.
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
	if req.Attributes != nil {
		if b, err := json.Marshal(req.Attributes); err == nil {
			params.Attributes = b
		}
	}

	sc, err := s.queries.UpdateScene(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, apperror.NotFound("scene", id.String())
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("update scene: %v", err))
	}

	// Write updated content to git working tree (not stored in DB after Step 4).
	if req.Content != nil && req.ProjectID != uuid.Nil {
		if repoPath, _, rErr := s.repoPathForUser(ctx, req.ProjectID, req.NotifyUserID); rErr == nil {
			if wErr := s.git.WriteSceneFile(repoPath, sc.ChapterID, sc.ID, *req.Content); wErr != nil {
				slog.Warn("git scene write failed on update", "scene_id", sc.ID, "error", wErr)
			}
		}
	}

	// Notify the AI service to mark the chapter summary stale and schedule
	// re-summarization — only when content was actually updated.
	if req.Content != nil && s.notifier != nil && req.NotifyUserID != uuid.Nil {
		branch := req.NotifyBranch
		if branch == "" {
			branch = CanonBranch
		}
		s.notifier.ScheduleSummarize(req.NotifyUserID, sc.ChapterID, req.ProjectID, branch)
	}

	result := toSceneResponse(sc)
	if req.Content != nil {
		result.Content = *req.Content // carry from request; not stored in DB
	}
	return result, nil
}

func (s *Service) DeleteScene(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteScene(ctx, id)
}

// ── Git / Chronicle operations ────────────────────────────────────────────────

// repoPathForUser resolves the git working-tree path for (projectID, userID).
// Owners use project.GitRepoPath; collaborators use their per-user clone path
// created during AcceptInvite. Also returns the collaborator row (nil for owner)
// so callers can apply branch-prefix enforcement without a second DB round-trip.
func (s *Service) repoPathForUser(ctx context.Context, projectID, userID uuid.UUID) (string, *sqlcgen.ProjectCollaborator, error) {
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return "", nil, apperror.NotFound("project", projectID.String())
	}
	if p.GitRepoPath == "" {
		return "", nil, apperror.Internal("project has no git repository")
	}
	if p.OwnerID == userID {
		return p.GitRepoPath, nil, nil
	}
	collab, err := s.queries.GetCollaborator(ctx, sqlcgen.GetCollaboratorParams{
		ProjectID: projectID,
		UserID:    userID,
	})
	if err != nil {
		return "", nil, apperror.Forbidden("you are not a member of this project")
	}
	return collab.ClonePath, &collab, nil
}

// Chronicle stages all working-tree changes and creates a commit (Chronicle)
// on the current Timeline. Scene files are already current in the working tree
// from Step 1 dual-write; no Postgres snapshot is needed.
// Reviewers have read-only access and cannot create commits.
func (s *Service) Chronicle(ctx context.Context, projectID, userID uuid.UUID, req ChronicleRequest) (*ChronicleEntry, error) {
	repoPath, collab, err := s.repoPathForUser(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	if collab != nil && collab.Role == "reviewer" {
		return nil, apperror.Forbidden("reviewers have read-only access and cannot chronicle")
	}

	sha, err := s.git.Chronicle(repoPath, req.Note)
	if errors.Is(err, ErrNothingToChronicle) {
		entries, _ := s.git.Lore(repoPath, 1, 1)
		if len(entries) > 0 {
			return &entries[0], ErrNothingToChronicle
		}
	}
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("chronicle: %v", err))
	}

	entries, err := s.git.Lore(repoPath, 1, 1)
	if err != nil || len(entries) == 0 {
		return &ChronicleEntry{SHA: sha, ShortSHA: sha[:7], Note: req.Note}, nil
	}
	return &entries[0], nil
}

// Lore returns the paginated commit history for the project's current Timeline.
func (s *Service) Lore(ctx context.Context, projectID, userID uuid.UUID, page, perPage int) ([]ChronicleEntry, error) {
	repoPath, _, err := s.repoPathForUser(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}

	entries, err := s.git.Lore(repoPath, page, perPage)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("lore: %v", err))
	}
	return entries, nil
}

// Echo returns a unified diff between two Chronicle SHAs.
func (s *Service) Echo(ctx context.Context, projectID, userID uuid.UUID, fromSHA, toSHA string) (*EchoResponse, error) {
	repoPath, _, err := s.repoPathForUser(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}

	diff, err := s.git.Echo(repoPath, fromSHA, toSHA)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("echo: %v", err))
	}
	return &EchoResponse{FromSHA: fromSHA, ToSHA: toSHA, Diff: diff}, nil
}

// GitStatus returns the current Timeline name and the most recent Chronicle.
func (s *Service) GitStatus(ctx context.Context, projectID, userID uuid.UUID) (*GitStatusResponse, error) {
	repoPath, _, err := s.repoPathForUser(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}

	timeline, err := s.git.CurrentTimeline(repoPath)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("current timeline: %v", err))
	}

	resp := &GitStatusResponse{CurrentTimeline: timeline}

	entries, err := s.git.Lore(repoPath, 1, 1)
	if err == nil && len(entries) > 0 {
		resp.LastChronicle = &entries[0]
	}

	return resp, nil
}

// Timelines lists all branches (Timelines) in the project's git repo.
func (s *Service) Timelines(ctx context.Context, projectID, userID uuid.UUID) ([]TimelineInfo, error) {
	repoPath, _, err := s.repoPathForUser(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}

	timelines, err := s.git.Timelines(repoPath)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("timelines: %v", err))
	}
	return timelines, nil
}

// Diverge creates a new Timeline and switches to it.
// userID is recorded in project_active_branch so the user's AI calls resolve
// to the new branch automatically.
// Collaborators may only create branches prefixed with their assigned branch name.
func (s *Service) Diverge(ctx context.Context, projectID, userID uuid.UUID, req DivergeRequest) (*TimelineInfo, error) {
	repoPath, collab, err := s.repoPathForUser(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	if req.TimelineName == CanonBranch {
		return nil, apperror.Validation("timeline name 'canon' is reserved")
	}
	if collab != nil && collab.Role == "reviewer" {
		return nil, apperror.Forbidden("reviewers have read-only access and cannot create branches")
	}
	if collab != nil && !strings.HasPrefix(req.TimelineName, collab.BranchName) {
		return nil, apperror.Forbidden("collaborators may only create branches prefixed with their assigned branch name: " + collab.BranchName)
	}

	if err := s.git.Diverge(repoPath, req.TimelineName, req.FromSHA); err != nil {
		return nil, apperror.Internal(fmt.Sprintf("diverge: %v", err))
	}

	if s.notifier != nil {
		s.notifier.UpsertActiveBranch(ctx, projectID, userID, req.TimelineName)
	}

	timelines, _ := s.git.Timelines(repoPath)
	for _, t := range timelines {
		if t.Name == req.TimelineName {
			return &t, nil
		}
	}
	return &TimelineInfo{Name: req.TimelineName, IsActive: true}, nil
}

// TravelTo switches the working tree to an existing Timeline.
// userID is recorded in project_active_branch so AI calls resolve correctly.
// Collaborators may only travel to their own branch prefix or canon.
func (s *Service) TravelTo(ctx context.Context, projectID, userID uuid.UUID, timelineName string) (*GitStatusResponse, error) {
	repoPath, collab, err := s.repoPathForUser(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	if collab != nil && timelineName != CanonBranch && !strings.HasPrefix(timelineName, collab.BranchName) {
		return nil, apperror.Forbidden("collaborators may only travel to their own branch or canon")
	}

	if err := s.git.TravelTo(repoPath, timelineName); err != nil {
		return nil, apperror.Internal(fmt.Sprintf("travel to %s: %v", timelineName, err))
	}

	if s.notifier != nil {
		s.notifier.UpsertActiveBranch(ctx, projectID, userID, timelineName)
	}

	return s.GitStatus(ctx, projectID, userID)
}

// Canonize merges a Timeline into Canon (fast-forward only in Phase A).
// On success it cleans up the merged branch's summary rows and user pointers.
// Reviewers cannot canonize; collaborators can only canonize their own branch.
func (s *Service) Canonize(ctx context.Context, projectID, userID uuid.UUID, timelineName string) (*CanonizeResult, error) {
	repoPath, collab, err := s.repoPathForUser(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	if timelineName == CanonBranch {
		return nil, apperror.Validation("cannot canonize Canon into itself")
	}
	if collab != nil {
		if collab.Role == "reviewer" {
			return nil, apperror.Forbidden("reviewers cannot canonize timelines")
		}
		if !strings.HasPrefix(timelineName, collab.BranchName) {
			return nil, apperror.Forbidden("collaborators may only canonize their own branch")
		}
	}

	result, err := s.git.Canonize(repoPath, timelineName)
	if err != nil {
		return nil, apperror.Internal(fmt.Sprintf("canonize: %v", err))
	}

	if s.notifier != nil && !result.HasParadox {
		s.notifier.CleanupBranch(ctx, projectID, timelineName)
	}

	return result, nil
}

// Converters

func toActResponse(a sqlcgen.Act) *ActResponse {
	return &ActResponse{
		ID:        a.ID,
		ProjectID: a.ProjectID,
		Title:     a.Title,
		Summary:   a.Summary,
		SortOrder: a.SortOrder,
		CreatedAt: a.CreatedAt.Time,
		UpdatedAt: a.UpdatedAt.Time,
	}
}

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
		ActID:     ch.ActID,
		Title:     ch.Title,
		Summary:   ch.Summary,
		SortOrder: ch.SortOrder,
		CreatedAt: ch.CreatedAt.Time,
		UpdatedAt: ch.UpdatedAt.Time,
	}
}

func toSceneResponse(sc sqlcgen.Scene) *SceneResponse {
	var attrs SceneAttributes
	if len(sc.Attributes) > 0 {
		_ = json.Unmarshal(sc.Attributes, &attrs)
	}
	return &SceneResponse{
		ID:           sc.ID,
		ChapterID:    sc.ChapterID,
		Title:        sc.Title,
		Content:      "", // populated from git working tree in GetScene / ListScenes
		POV:          sc.Pov,
		Tense:        sc.Tense,
		Tags:         sc.Tags,
		Summary:      sc.Summary,
		SummaryStale: sc.SummaryStale,
		SortOrder:    sc.SortOrder,
		WordCount:    sc.WordCount,
		Attributes:   attrs,
		CreatedAt:    sc.CreatedAt.Time,
		UpdatedAt:    sc.UpdatedAt.Time,
	}
}

// countWords returns the number of whitespace-delimited tokens in s.
func countWords(s string) int32 {
	return int32(len(strings.Fields(s)))
}
