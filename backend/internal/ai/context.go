package ai

// context.go — B2 AI memory layer.
//
// ResolveBranch determines which git Timeline (branch) the requesting user is
// currently on.  BuildContext assembles the chapter-summary context block that
// is prepended to every AI system prompt.  ScheduleSummarize debounces
// chapter-summary regeneration so rapid scene saves don't spam the LLM.

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// canonBranch is the default Timeline name; mirrors project.CanonBranch.
const canonBranch = "canon"

// summarizeDebounce is the quiet-period after the last scene save before the
// background goroutine actually calls the LLM to regenerate a summary.
const summarizeDebounce = 30 * time.Second

// debounceKey uniquely identifies a pending debounce timer.
type debounceKey struct {
	chapterID  uuid.UUID
	branchName string
}

// pendingWork holds everything needed to perform the deferred summarization.
type pendingWork struct {
	timer  *time.Timer
	userID uuid.UUID
}

// debouncer serialises access to the in-process timer map.
type debouncer struct {
	mu      sync.Mutex
	pending map[debounceKey]*pendingWork
}

func newDebouncer() *debouncer {
	return &debouncer{pending: make(map[debounceKey]*pendingWork)}
}

// schedule either resets an existing timer or creates a new one. When the
// timer fires the supplied fn is called in a new goroutine.
func (d *debouncer) schedule(key debounceKey, delay time.Duration, userID uuid.UUID, fn func(userID uuid.UUID)) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if p, ok := d.pending[key]; ok {
		// Absorb this save into the running debounce — bump the userID to the
		// latest writer so we use their API key for the eventual summarization.
		p.timer.Reset(delay)
		p.userID = userID
		return
	}

	p := &pendingWork{userID: userID}
	p.timer = time.AfterFunc(delay, func() {
		d.mu.Lock()
		pw := d.pending[key]
		delete(d.pending, key)
		d.mu.Unlock()
		fn(pw.userID)
	})
	d.pending[key] = p
}

// ── ResolveBranch ─────────────────────────────────────────────────────────────

// ResolveBranch returns the active Timeline (branch) for the user in a given
// project.  Resolution order:
//  1. The "X-NexusTale-Branch" request header (frontend sets this on every
//     call once the user has switched timelines).
//  2. The project_active_branch DB row for (projectID, userID).
//  3. "canon" — the default.
func (s *Service) ResolveBranch(ctx context.Context, headerBranch string, userID, projectID uuid.UUID) string {
	if headerBranch != "" {
		return headerBranch
	}

	branch, err := s.queries.GetProjectActiveBranch(ctx, sqlcgen.GetProjectActiveBranchParams{
		ProjectID: projectID,
		UserID:    userID,
	})
	if err == nil && branch != "" {
		return branch
	}

	return canonBranch
}

// ── ScheduleSummarize ─────────────────────────────────────────────────────────

// ScheduleSummarize marks the chapter summary for (chapterID, branchName) as
// stale and schedules a debounced LLM summarization.  Designed to be called
// non-blocking from scene-save handlers.
func (s *Service) ScheduleSummarize(userID, chapterID uuid.UUID, branchName string) {
	// Mark stale immediately so the frontend can show the indicator.
	go func() {
		if err := s.queries.MarkChapterSummaryStale(context.Background(), sqlcgen.MarkChapterSummaryStaleParams{
			ChapterID:  chapterID,
			BranchName: branchName,
		}); err != nil {
			slog.Warn("ai: mark stale failed", "chapter_id", chapterID, "branch", branchName, "error", err)
		}
	}()

	key := debounceKey{chapterID: chapterID, branchName: branchName}
	s.debounce.schedule(key, summarizeDebounce, userID, func(uid uuid.UUID) {
		s.regenerateSummary(uid, chapterID, branchName)
	})
}

// UpsertActiveBranch satisfies project.SummaryNotifier. It records the
// user's active Timeline so ResolveBranch can fall back to it when the
// X-NexusTale-Branch header is absent.
func (s *Service) UpsertActiveBranch(ctx context.Context, projectID, userID uuid.UUID, branchName string) {
	if err := s.queries.UpsertProjectActiveBranch(ctx, sqlcgen.UpsertProjectActiveBranchParams{
		ProjectID:  projectID,
		UserID:     userID,
		BranchName: branchName,
	}); err != nil {
		slog.Warn("ai: upsert active branch failed", "project_id", projectID, "user_id", userID, "branch", branchName, "error", err)
	}
}

// CleanupBranch satisfies project.SummaryNotifier. It removes chapter-summary
// rows and project_active_branch pointers for a Timeline that was just merged
// via Canonize.
func (s *Service) CleanupBranch(ctx context.Context, projectID uuid.UUID, branchName string) {
	if err := s.queries.DeleteChapterSummariesByBranch(ctx, sqlcgen.DeleteChapterSummariesByBranchParams{
		BranchName: branchName,
		ProjectID:  projectID,
	}); err != nil {
		slog.Warn("ai: cleanup branch summaries failed", "project_id", projectID, "branch", branchName, "error", err)
	}
	if err := s.queries.DeleteProjectActiveBranchByBranch(ctx, sqlcgen.DeleteProjectActiveBranchByBranchParams{
		ProjectID:  projectID,
		BranchName: branchName,
	}); err != nil {
		slog.Warn("ai: cleanup active branch rows failed", "project_id", projectID, "branch", branchName, "error", err)
	}
}

// regenerateSummary fetches all scene content for the chapter, calls Summarize,
// then stores the result.  Called by the debounce timer.
func (s *Service) regenerateSummary(userID, chapterID uuid.UUID, branchName string) {
	ctx := context.Background()

	scenes, err := s.queries.ListScenesByChapter(ctx, chapterID)
	if err != nil {
		slog.Warn("ai: regenerate summary — list scenes failed", "chapter_id", chapterID, "error", err)
		return
	}
	if len(scenes) == 0 {
		return
	}

	var sb strings.Builder
	for i, sc := range scenes {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(sc.Content)
	}

	summary, _, err := s.Summarize(ctx, userID, "", sb.String())
	if err != nil {
		slog.Warn("ai: regenerate summary — summarize failed", "chapter_id", chapterID, "error", err)
		return
	}

	if err := s.queries.UpsertChapterSummary(ctx, sqlcgen.UpsertChapterSummaryParams{
		ChapterID:  chapterID,
		BranchName: branchName,
		AiSummary:  summary,
	}); err != nil {
		slog.Warn("ai: regenerate summary — upsert failed", "chapter_id", chapterID, "error", err)
	}
}

// ── BuildContext ──────────────────────────────────────────────────────────────

// entityRefRE matches @[Entity Name] inline references in scene content.
var entityRefRE = regexp.MustCompile(`@\[([^\]]+)\]`)

// BuildContext assembles a context block to prepend to AI system prompts.
//
//   - Chapter summaries for the active branch (falls back to "canon" when the
//     branch diverged recently and has no summary of its own yet).
//   - Wiki entity snippets for any @[Entity Name] references found in
//     sceneContent.
//
// The returned string is empty if there is no useful context yet.
func (s *Service) BuildContext(ctx context.Context, projectID uuid.UUID, branchName, sceneContent string) string {
	var sb strings.Builder

	// ── 1. Chapter summaries ──────────────────────────────────────────────
	rows, err := s.queries.ListChapterSummariesByProject(ctx, sqlcgen.ListChapterSummariesByProjectParams{
		ProjectID:  projectID,
		BranchName: branchName,
	})

	// If the active branch has no summaries yet, fall back to "canon".
	if (err != nil || len(rows) == 0) && branchName != canonBranch {
		rows, err = s.queries.ListChapterSummariesByProject(ctx, sqlcgen.ListChapterSummariesByProjectParams{
			ProjectID:  projectID,
			BranchName: canonBranch,
		})
	}

	if err == nil && len(rows) > 0 {
		sb.WriteString("## Story so far\n")
		for _, r := range rows {
			if r.AiSummary == "" {
				continue
			}
			sb.WriteString(fmt.Sprintf("**%s**: %s\n", r.ChapterTitle, r.AiSummary))
		}
	}

	// ── 2. @[Entity] inline references ───────────────────────────────────
	matches := entityRefRE.FindAllStringSubmatch(sceneContent, -1)
	if len(matches) > 0 {
		seen := make(map[string]bool)
		var entitySnippets []string

		for _, m := range matches {
			name := m[1]
			if seen[name] {
				continue
			}
			seen[name] = true

			entities, err := s.queries.ListEntitiesByProject(ctx, sqlcgen.ListEntitiesByProjectParams{
				ProjectID: projectID,
				Type:      pgtype.Text{}, // no filter → return all types
			})
			if err != nil {
				break
			}
			for _, e := range entities {
				if strings.EqualFold(e.Name, name) && e.Summary != "" {
					entitySnippets = append(entitySnippets,
						fmt.Sprintf("**%s** (%s): %s", e.Name, e.Type, e.Summary))
					break
				}
			}
		}

		if len(entitySnippets) > 0 {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString("## Referenced entities\n")
			for _, s := range entitySnippets {
				sb.WriteString(s + "\n")
			}
		}
	}

	return sb.String()
}
