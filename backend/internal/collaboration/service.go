package collaboration

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/jconder44/nexustale/pkg/apperror"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// NotificationWriter is implemented by the notifications service.
// Using an interface breaks the import cycle between collaboration ↔ notifications.
type NotificationWriter interface {
	Notify(ctx context.Context, userID, projectID uuid.UUID, notifType string, payload map[string]any)
}

// Service manages project collaborators and invite tokens.
type Service struct {
	queries  *sqlcgen.Queries
	notifier NotificationWriter // optional; nil → notifications disabled
}

func NewService(queries *sqlcgen.Queries) *Service {
	return &Service{queries: queries}
}

// WithNotificationService wires the notification writer. Called from cmd/api during startup.
func (s *Service) WithNotificationService(n NotificationWriter) {
	s.notifier = n
}

// ── response types ────────────────────────────────────────────────────────────

type InviteResponse struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id"`
	Email     string `json:"email"`
	Role      string `json:"role"`
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	CreatedAt string `json:"created_at"`
}

type InvitePreview struct {
	ProjectID    string `json:"project_id"`
	ProjectTitle string `json:"project_title"`
	InviterName  string `json:"inviter_name"`
	Role         string `json:"role"`
	ExpiresAt    string `json:"expires_at"`
}

type CollaboratorResponse struct {
	ProjectID   string `json:"project_id"`
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Role        string `json:"role"`
	BranchName  string `json:"branch_name"`
	JoinedAt    string `json:"joined_at"`
}

// ── InviteCollaborator ────────────────────────────────────────────────────────

// InviteCollaborator creates a 7-day invite token for the given email and role.
// The invitee must already have a NexusTale account; the token is returned so
// the owner can share it via whatever channel they prefer.
//
// TODO(monetization): gate on owner.plan == "studio" before production launch.
func (s *Service) InviteCollaborator(ctx context.Context, ownerID, projectID uuid.UUID, email, role string) (InviteResponse, error) {
	// Validate role.
	if role != "coauthor" && role != "editor" && role != "reviewer" {
		return InviteResponse{}, apperror.Validation("role must be coauthor, editor, or reviewer")
	}

	// Verify the project belongs to the caller.
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return InviteResponse{}, apperror.NotFound("project", projectID.String())
	}
	if p.OwnerID != ownerID {
		return InviteResponse{}, apperror.Forbidden("only the project owner can invite collaborators")
	}

	// Verify the invitee has an existing account.
	invitee, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if err == pgx.ErrNoRows {
			return InviteResponse{}, apperror.Validation("no NexusTale account found for " + email)
		}
		return InviteResponse{}, fmt.Errorf("lookup invitee: %w", err)
	}

	// Check they are not already a collaborator.
	_, err = s.queries.GetCollaborator(ctx, sqlcgen.GetCollaboratorParams{
		ProjectID: projectID,
		UserID:    invitee.ID,
	})
	if err == nil {
		return InviteResponse{}, apperror.Validation("user is already a collaborator on this project")
	}

	// Generate a 32-byte random hex token.
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return InviteResponse{}, fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(b)

	expiresAt := time.Now().UTC().Add(7 * 24 * time.Hour)
	row, err := s.queries.CreateInvite(ctx, sqlcgen.CreateInviteParams{
		ProjectID: projectID,
		InvitedBy: ownerID,
		Email:     email,
		Role:      role,
		Token:     token,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	if err != nil {
		return InviteResponse{}, fmt.Errorf("create invite: %w", err)
	}

	resp := InviteResponse{
		ID:        row.ID.String(),
		ProjectID: row.ProjectID.String(),
		Email:     row.Email,
		Role:      row.Role,
		Token:     row.Token,
		ExpiresAt: row.ExpiresAt.Time.Format(time.RFC3339),
		CreatedAt: row.CreatedAt.Time.Format(time.RFC3339),
	}

	// Notify the invitee non-blocking so invite latency is unaffected.
	if s.notifier != nil {
		inviter, _ := s.queries.GetUserByID(ctx, ownerID)
		go s.notifier.Notify(ctx, invitee.ID, projectID, "invite_received", map[string]any{
			"project_title": p.Title,
			"inviter_name":  inviter.DisplayName,
			"role":          role,
			"invite_token":  token,
		})
	}

	return resp, nil
}

// ── GetInvitePreview ──────────────────────────────────────────────────────────

// GetInvitePreview returns safe, pre-accept information about an invite token.
// Called from the unauthenticated /invites/:token GET route so the user can see
// what they are accepting before logging in.
func (s *Service) GetInvitePreview(ctx context.Context, token string) (InvitePreview, error) {
	inv, err := s.queries.GetInviteByToken(ctx, token)
	if err != nil {
		return InvitePreview{}, apperror.NotFound("invite", token)
	}
	if inv.ExpiresAt.Time.Before(time.Now()) {
		return InvitePreview{}, apperror.Validation("this invite has expired")
	}
	if inv.AcceptedAt.Valid {
		return InvitePreview{}, apperror.Validation("this invite has already been accepted")
	}

	p, err := s.queries.GetProject(ctx, inv.ProjectID)
	if err != nil {
		return InvitePreview{}, apperror.NotFound("project", inv.ProjectID.String())
	}
	inviter, err := s.queries.GetUserByID(ctx, inv.InvitedBy)
	if err != nil {
		return InvitePreview{}, fmt.Errorf("lookup inviter: %w", err)
	}

	return InvitePreview{
		ProjectID:    p.ID.String(),
		ProjectTitle: p.Title,
		InviterName:  inviter.DisplayName,
		Role:         inv.Role,
		ExpiresAt:    inv.ExpiresAt.Time.Format(time.RFC3339),
	}, nil
}

// ── AcceptInvite ──────────────────────────────────────────────────────────────

// AcceptInvite validates the token against the requesting user's email,
// clones the project repo into a per-collaborator working tree, creates the
// collaboration branch, and inserts the project_collaborators row.
func (s *Service) AcceptInvite(ctx context.Context, userID uuid.UUID, token string) (CollaboratorResponse, error) {
	inv, err := s.queries.GetInviteByToken(ctx, token)
	if err != nil {
		return CollaboratorResponse{}, apperror.NotFound("invite", token)
	}
	if inv.ExpiresAt.Time.Before(time.Now()) {
		return CollaboratorResponse{}, apperror.Validation("this invite has expired")
	}
	if inv.AcceptedAt.Valid {
		return CollaboratorResponse{}, apperror.Validation("this invite has already been accepted")
	}

	// Verify the accepting user's email matches the invite.
	user, err := s.queries.GetUserByID(ctx, userID)
	if err != nil {
		return CollaboratorResponse{}, fmt.Errorf("lookup user: %w", err)
	}
	if !strings.EqualFold(user.Email, inv.Email) {
		return CollaboratorResponse{}, apperror.Forbidden("this invite was sent to a different email address")
	}

	// Guard against duplicate acceptance.
	if _, err := s.queries.GetCollaborator(ctx, sqlcgen.GetCollaboratorParams{
		ProjectID: inv.ProjectID,
		UserID:    userID,
	}); err == nil {
		return CollaboratorResponse{}, apperror.Validation("you are already a collaborator on this project")
	}

	// Get the project for the git repo path.
	p, err := s.queries.GetProject(ctx, inv.ProjectID)
	if err != nil {
		return CollaboratorResponse{}, apperror.NotFound("project", inv.ProjectID.String())
	}

	// Build the branch name: role/slug(displayName).
	branchName := inv.Role + "/" + slugify(user.DisplayName)

	// Clone the project repo into a sibling directory for this collaborator.
	// Source:  {reposDir}/{projectId}/
	// Clone:   {reposDir}/{projectId}-collab-{userId}/
	reposDir := filepath.Dir(p.GitRepoPath)
	clonePath := filepath.Join(reposDir, p.ID.String()+"-collab-"+userID.String())

	if err := cloneAndBranch(p.GitRepoPath, clonePath, branchName); err != nil {
		return CollaboratorResponse{}, fmt.Errorf("setup collaborator workspace: %w", err)
	}

	// Insert collaborator row.
	pgInvitedBy := pgtype.UUID{Bytes: [16]byte(inv.InvitedBy), Valid: true}
	collab, err := s.queries.CreateCollaborator(ctx, sqlcgen.CreateCollaboratorParams{
		ProjectID:  inv.ProjectID,
		UserID:     userID,
		Role:       inv.Role,
		BranchName: branchName,
		ClonePath:  clonePath,
		InvitedBy:  pgInvitedBy,
	})
	if err != nil {
		return CollaboratorResponse{}, fmt.Errorf("create collaborator: %w", err)
	}

	// Mark the invite as accepted.
	if err := s.queries.AcceptInvite(ctx, token); err != nil {
		return CollaboratorResponse{}, fmt.Errorf("mark invite accepted: %w", err)
	}

	return CollaboratorResponse{
		ProjectID:   collab.ProjectID.String(),
		UserID:      collab.UserID.String(),
		DisplayName: user.DisplayName,
		Email:       user.Email,
		Role:        collab.Role,
		BranchName:  collab.BranchName,
		JoinedAt:    collab.JoinedAt.Time.Format(time.RFC3339),
	}, nil
}

// ── ListCollaborators ─────────────────────────────────────────────────────────

// ListCollaborators returns all accepted collaborators for a project.
func (s *Service) ListCollaborators(ctx context.Context, projectID uuid.UUID) ([]CollaboratorResponse, error) {
	rows, err := s.queries.ListCollaborators(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list collaborators: %w", err)
	}

	out := make([]CollaboratorResponse, 0, len(rows))
	for _, r := range rows {
		u, err := s.queries.GetUserByID(ctx, r.UserID)
		if err != nil {
			continue // skip rows whose user has been deleted
		}
		out = append(out, CollaboratorResponse{
			ProjectID:   r.ProjectID.String(),
			UserID:      r.UserID.String(),
			DisplayName: u.DisplayName,
			Email:       u.Email,
			Role:        r.Role,
			BranchName:  r.BranchName,
			JoinedAt:    r.JoinedAt.Time.Format(time.RFC3339),
		})
	}
	return out, nil
}

// ── ListPendingInvites ────────────────────────────────────────────────────────

// ListPendingInvites returns unexpired, unaccepted invites for a project.
func (s *Service) ListPendingInvites(ctx context.Context, ownerID, projectID uuid.UUID) ([]InviteResponse, error) {
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return nil, apperror.NotFound("project", projectID.String())
	}
	if p.OwnerID != ownerID {
		return nil, apperror.Forbidden("only the project owner can view invites")
	}

	rows, err := s.queries.ListPendingInvites(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list pending invites: %w", err)
	}

	out := make([]InviteResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, InviteResponse{
			ID:        r.ID.String(),
			ProjectID: r.ProjectID.String(),
			Email:     r.Email,
			Role:      r.Role,
			Token:     r.Token,
			ExpiresAt: r.ExpiresAt.Time.Format(time.RFC3339),
			CreatedAt: r.CreatedAt.Time.Format(time.RFC3339),
		})
	}
	return out, nil
}

// ── RemoveCollaborator ────────────────────────────────────────────────────────

// RemoveCollaborator removes a collaborator from a project. Only the owner may
// remove collaborators. The git clone directory is left on disk — removing it
// is a best-effort cleanup so errors are non-fatal.
func (s *Service) RemoveCollaborator(ctx context.Context, ownerID, projectID, targetUserID uuid.UUID) error {
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		return apperror.NotFound("project", projectID.String())
	}
	if p.OwnerID != ownerID {
		return apperror.Forbidden("only the project owner can remove collaborators")
	}

	if err := s.queries.RemoveCollaborator(ctx, sqlcgen.RemoveCollaboratorParams{
		ProjectID: projectID,
		UserID:    targetUserID,
	}); err != nil {
		return fmt.Errorf("remove collaborator: %w", err)
	}
	return nil
}

// ── GetCollaborator ───────────────────────────────────────────────────────────

// GetCollaborator returns the collaboration row for (projectID, userID).
// Used by the middleware to resolve clone path and role.
func (s *Service) GetCollaborator(ctx context.Context, projectID, userID uuid.UUID) (sqlcgen.ProjectCollaborator, error) {
	return s.queries.GetCollaborator(ctx, sqlcgen.GetCollaboratorParams{
		ProjectID: projectID,
		UserID:    userID,
	})
}

// ── helpers ───────────────────────────────────────────────────────────────────

var nonAlphanumRE = regexp.MustCompile(`[^a-z0-9]+`)

// slugify converts a display name to a branch-safe lowercase slug.
func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumRE.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// cloneAndBranch clones the source repo to destPath (creating it) and
// then creates and checks out branchName inside the clone.
func cloneAndBranch(sourceRepoPath, destPath, branchName string) error {
	if _, err := git.PlainClone(destPath, false, &git.CloneOptions{
		URL: sourceRepoPath,
	}); err != nil {
		return fmt.Errorf("clone repo: %w", err)
	}

	repo, err := git.PlainOpen(destPath)
	if err != nil {
		return fmt.Errorf("open clone: %w", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		return fmt.Errorf("get worktree: %w", err)
	}

	head, err := repo.Head()
	if err != nil {
		return fmt.Errorf("get head: %w", err)
	}

	return wt.Checkout(&git.CheckoutOptions{
		Hash:   head.Hash(),
		Branch: plumbing.NewBranchReferenceName(branchName),
		Create: true,
	})
}
