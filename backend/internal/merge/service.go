package merge

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/jconder44/nexustale/internal/project"
	"github.com/jconder44/nexustale/pkg/apperror"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// NotificationWriter matches notifications.Service.Notify so we avoid an
// import cycle between merge ↔ notifications.
type NotificationWriter interface {
	Notify(ctx context.Context, userID, projectID uuid.UUID, notifType string, payload map[string]any)
}

type Service struct {
	queries  *sqlcgen.Queries
	git      *project.GitService
	notifier NotificationWriter
}

func NewService(queries *sqlcgen.Queries, git *project.GitService) *Service {
	return &Service{queries: queries, git: git}
}

func (s *Service) WithNotificationService(n NotificationWriter) {
	s.notifier = n
}

// ── response types ────────────────────────────────────────────────────────────

type MergeRequestResponse struct {
	ID            string  `json:"id"`
	ProjectID     string  `json:"project_id"`
	FromBranch    string  `json:"from_branch"`
	ToBranch      string  `json:"to_branch"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	RequestedBy   string  `json:"requested_by"`
	RequesterName string  `json:"requester_name"`
	Status        string  `json:"status"`
	ReviewerNote  string  `json:"reviewer_note"`
	CreatedAt     string  `json:"created_at"`
	ResolvedAt    *string `json:"resolved_at,omitempty"`
}

type SceneDiff struct {
	GitPath string `json:"git_path"` // e.g. "scenes/uuid.md"
	SceneID string `json:"scene_id"` // UUID extracted from path, empty if not a scene file
	Diff    string `json:"diff"`     // unified diff for this file
	IsNew   bool   `json:"is_new"`
	IsDeleted bool  `json:"is_deleted"`
}

type MRDiffResponse struct {
	MRID       string      `json:"mr_id"`
	FromBranch string      `json:"from_branch"`
	ToBranch   string      `json:"to_branch"`
	SceneDiffs []SceneDiff `json:"scene_diffs"`
}

type ResolveRequest struct {
	Action       string `json:"action"` // "approve" | "reject" | "merge"
	ReviewerNote string `json:"reviewer_note"`
}

// ── OpenMergeRequest ──────────────────────────────────────────────────────────

func (s *Service) OpenMergeRequest(ctx context.Context, callerID, projectID uuid.UUID, fromBranch, title, description string) (MergeRequestResponse, error) {
	// Verify project exists.
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return MergeRequestResponse{}, apperror.NotFound("project", projectID.String())
	}

	// Owner cannot open an MR against their own repo (they commit directly to canon).
	if p.OwnerID == callerID {
		return MergeRequestResponse{}, apperror.Validation("project owner can merge directly — MRs are for collaborators")
	}

	// Caller must be a collaborator, not a reviewer.
	collab, err := s.queries.GetCollaborator(ctx, sqlcgen.GetCollaboratorParams{
		ProjectID: projectID,
		UserID:    callerID,
	})
	if err != nil {
		return MergeRequestResponse{}, apperror.Forbidden("you are not a collaborator on this project")
	}
	if collab.Role == "reviewer" {
		return MergeRequestResponse{}, apperror.Forbidden("reviewers cannot open merge requests")
	}

	// Branch must carry the collaborator's prefix.
	if !strings.HasPrefix(fromBranch, collab.BranchName) {
		return MergeRequestResponse{}, apperror.Forbidden("you may only open merge requests for branches prefixed with: " + collab.BranchName)
	}

	// No open MR may already exist for this branch.
	if _, err := s.queries.GetOpenMRByBranch(ctx, sqlcgen.GetOpenMRByBranchParams{
		ProjectID:  projectID,
		FromBranch: fromBranch,
	}); err == nil {
		return MergeRequestResponse{}, apperror.Validation("an open merge request already exists for branch: " + fromBranch)
	}

	row, err := s.queries.CreateMergeRequest(ctx, sqlcgen.CreateMergeRequestParams{
		ProjectID:   projectID,
		FromBranch:  fromBranch,
		ToBranch:    "canon",
		Title:       title,
		Description: description,
		RequestedBy: callerID,
	})
	if err != nil {
		return MergeRequestResponse{}, fmt.Errorf("create merge request: %w", err)
	}

	// Notify the project owner.
	if s.notifier != nil {
		caller, _ := s.queries.GetUserByID(ctx, callerID)
		go s.notifier.Notify(ctx, p.OwnerID, projectID, "mr_opened", map[string]any{
			"project_title": p.Title,
			"mr_id":         row.ID.String(),
			"from_branch":   fromBranch,
			"requester":     caller.DisplayName,
		})
	}

	return toResponse(row, ""), nil
}

// ── ListMergeRequests ─────────────────────────────────────────────────────────

func (s *Service) ListMergeRequests(ctx context.Context, callerID, projectID uuid.UUID) ([]MergeRequestResponse, error) {
	if err := s.requireAccess(ctx, callerID, projectID); err != nil {
		return nil, err
	}
	rows, err := s.queries.ListMergeRequests(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list merge requests: %w", err)
	}
	out := make([]MergeRequestResponse, 0, len(rows))
	for _, r := range rows {
		mr := MergeRequestResponse{
			ID:            r.ID.String(),
			ProjectID:     r.ProjectID.String(),
			FromBranch:    r.FromBranch,
			ToBranch:      r.ToBranch,
			Title:         r.Title,
			Description:   r.Description,
			RequestedBy:   r.RequestedBy.String(),
			RequesterName: r.RequesterName,
			Status:        r.Status,
			ReviewerNote:  r.ReviewerNote,
			CreatedAt:     r.CreatedAt.Time.Format(time.RFC3339),
		}
		if r.ResolvedAt.Valid {
			s := r.ResolvedAt.Time.Format(time.RFC3339)
			mr.ResolvedAt = &s
		}
		out = append(out, mr)
	}
	return out, nil
}

// ── GetMergeRequest ───────────────────────────────────────────────────────────

func (s *Service) GetMergeRequest(ctx context.Context, callerID, mrID uuid.UUID) (MergeRequestResponse, error) {
	row, err := s.queries.GetMergeRequest(ctx, mrID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return MergeRequestResponse{}, apperror.NotFound("merge request", mrID.String())
		}
		return MergeRequestResponse{}, fmt.Errorf("get merge request: %w", err)
	}
	if err := s.requireAccess(ctx, callerID, row.ProjectID); err != nil {
		return MergeRequestResponse{}, err
	}

	mr := MergeRequestResponse{
		ID:            row.ID.String(),
		ProjectID:     row.ProjectID.String(),
		FromBranch:    row.FromBranch,
		ToBranch:      row.ToBranch,
		Title:         row.Title,
		Description:   row.Description,
		RequestedBy:   row.RequestedBy.String(),
		RequesterName: row.RequesterName,
		Status:        row.Status,
		ReviewerNote:  row.ReviewerNote,
		CreatedAt:     row.CreatedAt.Time.Format(time.RFC3339),
	}
	if row.ResolvedAt.Valid {
		s := row.ResolvedAt.Time.Format(time.RFC3339)
		mr.ResolvedAt = &s
	}
	return mr, nil
}

// ── GetMergeRequestDiff ───────────────────────────────────────────────────────

// GetMergeRequestDiff opens the collaborator's git clone and diffs the canon
// branch tip (as it was at clone time) against the MR's from_branch. Returns a
// structured list of per-scene diffs for C3.3's prose diff viewer.
func (s *Service) GetMergeRequestDiff(ctx context.Context, callerID, mrID uuid.UUID) (MRDiffResponse, error) {
	row, err := s.queries.GetMergeRequest(ctx, mrID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return MRDiffResponse{}, apperror.NotFound("merge request", mrID.String())
		}
		return MRDiffResponse{}, fmt.Errorf("get merge request: %w", err)
	}
	if err := s.requireAccess(ctx, callerID, row.ProjectID); err != nil {
		return MRDiffResponse{}, err
	}

	// Get collaborator's clone path.
	collab, err := s.queries.GetCollaborator(ctx, sqlcgen.GetCollaboratorParams{
		ProjectID: row.ProjectID,
		UserID:    row.RequestedBy,
	})
	if err != nil {
		return MRDiffResponse{}, apperror.Internal("collaborator record not found for merge request author")
	}

	rawDiff, err := s.git.EchoBranches(collab.ClonePath, "canon", row.FromBranch)
	if err != nil {
		return MRDiffResponse{}, apperror.Internal(fmt.Sprintf("compute diff: %v", err))
	}

	return MRDiffResponse{
		MRID:       mrID.String(),
		FromBranch: row.FromBranch,
		ToBranch:   row.ToBranch,
		SceneDiffs: parseDiff(rawDiff),
	}, nil
}

// ── ResolveMergeRequest ───────────────────────────────────────────────────────

// ResolveMergeRequest handles approve/reject/merge actions on a merge request.
// Only the project owner may perform any resolution. On merge, the collaborator's
// branch is fetched into the main repo and canonized via fast-forward.
func (s *Service) ResolveMergeRequest(ctx context.Context, callerID, mrID uuid.UUID, req ResolveRequest) (MergeRequestResponse, error) {
	row, err := s.queries.GetMergeRequest(ctx, mrID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return MergeRequestResponse{}, apperror.NotFound("merge request", mrID.String())
		}
		return MergeRequestResponse{}, fmt.Errorf("get merge request: %w", err)
	}

	// Only the project owner can resolve.
	p, err := s.queries.GetProject(ctx, row.ProjectID)
	if err != nil {
		return MergeRequestResponse{}, apperror.NotFound("project", row.ProjectID.String())
	}
	if p.OwnerID != callerID {
		return MergeRequestResponse{}, apperror.Forbidden("only the project owner can resolve merge requests")
	}

	if row.Status != "open" && row.Status != "approved" {
		return MergeRequestResponse{}, apperror.Validation("merge request is already " + row.Status)
	}

	var newStatus string
	switch req.Action {
	case "approve":
		newStatus = "approved"
	case "reject":
		newStatus = "rejected"
	case "merge":
		newStatus = "merged"
	default:
		return MergeRequestResponse{}, apperror.Validation("action must be approve, reject, or merge")
	}

	// For merge, fetch the collaborator's branch into the main repo and canonize.
	if req.Action == "merge" {
		collab, err := s.queries.GetCollaborator(ctx, sqlcgen.GetCollaboratorParams{
			ProjectID: row.ProjectID,
			UserID:    row.RequestedBy,
		})
		if err != nil {
			return MergeRequestResponse{}, apperror.Internal("collaborator record not found for merge request author")
		}

		if err := s.git.FetchBranchFromClone(p.GitRepoPath, collab.ClonePath, row.FromBranch); err != nil {
			return MergeRequestResponse{}, apperror.Internal(fmt.Sprintf("fetch branch: %v", err))
		}

		result, err := s.git.Canonize(p.GitRepoPath, row.FromBranch)
		if err != nil {
			return MergeRequestResponse{}, apperror.Internal(fmt.Sprintf("canonize: %v", err))
		}
		if result.HasParadox {
			return MergeRequestResponse{}, apperror.Validation("merge conflict detected — " + result.ParadoxNote)
		}
	}

	updated, err := s.queries.UpdateMergeRequestStatus(ctx, sqlcgen.UpdateMergeRequestStatusParams{
		ID:           mrID,
		Status:       newStatus,
		ReviewerNote: req.ReviewerNote,
	})
	if err != nil {
		return MergeRequestResponse{}, fmt.Errorf("update merge request: %w", err)
	}

	// Fire notification to the MR author.
	if s.notifier != nil {
		notifType := "mr_" + req.Action + "d"
		if req.Action == "merge" {
			notifType = "mr_merged"
		}
		go s.notifier.Notify(ctx, row.RequestedBy, row.ProjectID, notifType, map[string]any{
			"project_title": p.Title,
			"mr_id":         mrID.String(),
			"from_branch":   row.FromBranch,
		})
	}

	return toResponse(updated, ""), nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// requireAccess checks that callerID is either the project owner or a collaborator.
func (s *Service) requireAccess(ctx context.Context, callerID, projectID uuid.UUID) error {
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return apperror.NotFound("project", projectID.String())
	}
	if p.OwnerID == callerID {
		return nil
	}
	_, err = s.queries.GetCollaborator(ctx, sqlcgen.GetCollaboratorParams{
		ProjectID: projectID,
		UserID:    callerID,
	})
	if err != nil {
		return apperror.Forbidden("you do not have access to this project")
	}
	return nil
}

// parseDiff splits a unified diff string into per-file SceneDiff entries.
// Files outside of scenes/ are included with an empty SceneID.
func parseDiff(rawDiff string) []SceneDiff {
	if rawDiff == "" {
		return []SceneDiff{}
	}

	// Split on "diff --git" header lines.
	var diffs []SceneDiff
	var currentPath string
	var currentLines []string
	isNew, isDeleted := false, false

	for _, line := range strings.Split(rawDiff, "\n") {
		if strings.HasPrefix(line, "diff --git ") {
			if currentPath != "" {
				diffs = append(diffs, buildSceneDiff(currentPath, strings.Join(currentLines, "\n"), isNew, isDeleted))
			}
			// Parse "diff --git a/path b/path" → extract path
			parts := strings.SplitN(line, " b/", 2)
			if len(parts) == 2 {
				currentPath = parts[1]
			}
			currentLines = []string{line}
			isNew, isDeleted = false, false
		} else {
			if strings.HasPrefix(line, "new file mode") {
				isNew = true
			}
			if strings.HasPrefix(line, "deleted file mode") {
				isDeleted = true
			}
			currentLines = append(currentLines, line)
		}
	}
	if currentPath != "" {
		diffs = append(diffs, buildSceneDiff(currentPath, strings.Join(currentLines, "\n"), isNew, isDeleted))
	}
	return diffs
}

func buildSceneDiff(gitPath, diff string, isNew, isDeleted bool) SceneDiff {
	sd := SceneDiff{GitPath: gitPath, Diff: diff, IsNew: isNew, IsDeleted: isDeleted}
	// Extract scene UUID from paths like "scenes/<uuid>.md"
	if strings.HasPrefix(gitPath, "scenes/") && strings.HasSuffix(gitPath, ".md") {
		id := strings.TrimPrefix(gitPath, "scenes/")
		id = strings.TrimSuffix(id, ".md")
		sd.SceneID = id
	}
	return sd
}

func toResponse(mr sqlcgen.MergeRequest, requesterName string) MergeRequestResponse {
	r := MergeRequestResponse{
		ID:            mr.ID.String(),
		ProjectID:     mr.ProjectID.String(),
		FromBranch:    mr.FromBranch,
		ToBranch:      mr.ToBranch,
		Title:         mr.Title,
		Description:   mr.Description,
		RequestedBy:   mr.RequestedBy.String(),
		RequesterName: requesterName,
		Status:        mr.Status,
		ReviewerNote:  mr.ReviewerNote,
		CreatedAt:     mr.CreatedAt.Time.Format(time.RFC3339),
	}
	if mr.ResolvedAt.Valid {
		s := mr.ResolvedAt.Time.Format(time.RFC3339)
		r.ResolvedAt = &s
	}
	return r
}
