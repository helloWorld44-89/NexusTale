package ai

// context.go — B2 AI memory layer.
//
// ResolveBranch determines which git Timeline (branch) the requesting user is
// currently on.  BuildContext assembles the chapter-summary context block that
// is prepended to every AI system prompt.  ScheduleSummarize debounces
// chapter-summary regeneration so rapid scene saves don't spam the LLM.

import (
	"context"
	"encoding/json"
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

// cancelForChapter stops and removes all pending timers whose key matches
// chapterID. Called when a chapter is deleted to prevent spurious LLM calls.
func (d *debouncer) cancelForChapter(chapterID uuid.UUID) {
	d.mu.Lock()
	defer d.mu.Unlock()

	for key, pw := range d.pending {
		if key.chapterID == chapterID {
			pw.timer.Stop()
			delete(d.pending, key)
		}
	}
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

// CancelSummarize satisfies project.SummaryNotifier. It cancels any pending
// debounce timers for the given chapter (called when a chapter is deleted).
func (s *Service) CancelSummarize(chapterID uuid.UUID) {
	s.debounce.cancelForChapter(chapterID)
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

// contentFallbackLimit is the maximum rune count of raw scene content included
// per chapter when no AI summary exists for that chapter yet.
const contentFallbackLimit = 600

// BuildContext assembles a context block to prepend to AI system prompts.
//
//  1. Project identity — title and genres, always included.
//  2. Story so far — AI chapter summaries when available; raw scene content
//     (truncated to contentFallbackLimit) for chapters not yet summarised.
//     Falls back to "canon" summaries when the active branch has none.
//  3. Current scene — full text of the scene the user is currently editing,
//     labelled clearly so the model understands which part of the story is in scope.
//  4. Wiki entity snippets — for any @[Entity Name] refs in the current scene.
//  5. Story structure — name + phases when the project has one selected.
//  6. Pinned context — writer-curated entities, chapters, and scenes pinned via
//     the Context Pins panel; injected verbatim so writers control what Nexus knows.
//
// The returned string is never empty: the project identity block is always
// present so the model always knows the project it is working on.
func (s *Service) BuildContext(ctx context.Context, projectID, userID uuid.UUID, branchName, sceneContent string, currentSceneID uuid.UUID) string {
	var sb strings.Builder

	// ── 1. Project identity + AI bible ───────────────────────────────────
	if projectID != uuid.Nil {
		p, err := s.queries.GetProject(ctx, projectID)
		if err == nil {
			sb.WriteString("## Project\n")
			sb.WriteString("**Title**: " + p.Title + "\n")
			if len(p.Genres) > 0 {
				sb.WriteString("**Genre**: " + strings.Join(p.Genres, ", ") + "\n")
			}
			// AI bible — user-editable text that overrides the bare project identity.
			// Auto-populated from guide steps when first saved; always takes
			// precedence over the bare title/genres block above.
			if strings.TrimSpace(p.AiInstructions) != "" {
				sb.WriteString("\n## Story bible\n")
				sb.WriteString(p.AiInstructions + "\n")
			}
		}
	}

	// ── 2. Story so far ───────────────────────────────────────────────────
	// Load AI chapter summaries for the active branch.
	summaryRows, _ := s.queries.ListChapterSummariesByProject(ctx, sqlcgen.ListChapterSummariesByProjectParams{
		ProjectID:  projectID,
		BranchName: branchName,
	})
	if len(summaryRows) == 0 && branchName != canonBranch {
		summaryRows, _ = s.queries.ListChapterSummariesByProject(ctx, sqlcgen.ListChapterSummariesByProjectParams{
			ProjectID:  projectID,
			BranchName: canonBranch,
		})
	}

	// Build a lookup: chapterID → summary text (empty string = no summary yet).
	summaryByChapter := make(map[uuid.UUID]string)
	for _, r := range summaryRows {
		summaryByChapter[r.ChapterID] = r.AiSummary
	}

	// Fetch the chapter list so we can produce a "story so far" block even
	// when summaries don't exist yet (fallback to raw scene content).
	chapters, err := s.queries.ListChaptersByProject(ctx, projectID)
	if err == nil && len(chapters) > 0 {
		var storySoFar strings.Builder
		for _, ch := range chapters {
			if summary, ok := summaryByChapter[ch.ID]; ok && summary != "" {
				storySoFar.WriteString(fmt.Sprintf("**%s**: %s\n", ch.Title, summary))
				continue
			}
			// No AI summary yet — fall back to a raw content snippet.
			scenes, err := s.queries.ListScenesByChapter(ctx, ch.ID)
			if err != nil || len(scenes) == 0 {
				continue
			}
			var combined strings.Builder
			for i, sc := range scenes {
				if i > 0 {
					combined.WriteString(" ")
				}
				combined.WriteString(strings.TrimSpace(sc.Content))
			}
			snippet := []rune(combined.String())
			if len(snippet) > contentFallbackLimit {
				snippet = append(snippet[:contentFallbackLimit], []rune("…")...)
			}
			if len(snippet) > 0 {
				storySoFar.WriteString(fmt.Sprintf("**%s** *(excerpt)*: %s\n", ch.Title, string(snippet)))
			}
		}
		if storySoFar.Len() > 0 {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString("## Story so far\n")
			sb.WriteString(storySoFar.String())
		}
	}

	// ── 3. Current scene ─────────────────────────────────────────────────
	// Include the full text of the scene currently open in the editor so the
	// model can answer specific questions about it.
	if currentSceneID != uuid.Nil && sceneContent != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sc, err := s.queries.GetScene(ctx, currentSceneID)
		label := "Current scene"
		if err == nil && sc.Title != "" {
			label = fmt.Sprintf("Current scene — %s", sc.Title)
		}
		sb.WriteString(fmt.Sprintf("## %s\n%s\n", label, sceneContent))
	}

	// ── 4. @[Entity] inline references ───────────────────────────────────
	matches := entityRefRE.FindAllStringSubmatch(sceneContent, -1)
	if len(matches) > 0 {
		seen := make(map[string]bool)
		var entitySnippets []string

		// Fetch all entities once; filter by name below.
		entities, _ := s.queries.ListEntitiesByProject(ctx, sqlcgen.ListEntitiesByProjectParams{
			ProjectID: projectID,
			Type:      pgtype.Text{},
		})

		for _, m := range matches {
			name := m[1]
			if seen[name] {
				continue
			}
			seen[name] = true
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
			for _, snippet := range entitySnippets {
				sb.WriteString(snippet + "\n")
			}
		}
	}

	// ── 5. Story structure (optional) ─────────────────────────────────────
	if structureCtx := s.buildStructureContext(ctx, projectID); structureCtx != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(structureCtx)
	}

	// ── 6. Pinned context (writer-curated) ────────────────────────────────
	// Only injected when both projectID and userID are known, so that
	// background summarize goroutines (which have no userID) are unaffected.
	if userID != uuid.Nil {
		if pinnedCtx := s.buildPinnedContext(ctx, projectID, userID, branchName); pinnedCtx != "" {
			if sb.Len() > 0 {
				sb.WriteString("\n")
			}
			sb.WriteString(pinnedCtx)
		}
	}

	return sb.String()
}

// ── structure context helper ──────────────────────────────────────────────────

// phaseEntry is the minimal shape of one element in novel_structures.phases JSONB.
type phaseEntry struct {
	Name string `json:"name"`
}

// structureCustomData is the shape written by the frontend freeform path.
type structureCustomData struct {
	Rules []string `json:"rules"`
}

// buildStructureContext returns an optional context block describing the
// project's story structure. Returns "" when no structure is selected or on
// any error so that callers can always safely append it without checking.
//
// Named structure: injects the structure name + ordered phase list.
// Freeform structure: injects the writer's custom rules (if any).
func (s *Service) buildStructureContext(ctx context.Context, projectID uuid.UUID) string {
	row, err := s.queries.GetProjectStructure(ctx, projectID)
	if err != nil {
		return ""
	}

	var out strings.Builder

	if row.StructureID.Valid && row.StructureName.Valid {
		// Named structure selected — emit name and phase overview.
		out.WriteString("## Story structure\n")
		out.WriteString("Structure: " + row.StructureName.String + "\n")

		if len(row.Phases) > 0 {
			var phases []phaseEntry
			if json.Unmarshal(row.Phases, &phases) == nil && len(phases) > 0 {
				names := make([]string, 0, len(phases))
				for _, p := range phases {
					if p.Name != "" {
						names = append(names, p.Name)
					}
				}
				if len(names) > 0 {
					out.WriteString("Phases: " + strings.Join(names, " → ") + "\n")
				}
			}
		}
	} else if len(row.StructureCustom) > 0 {
		// Freeform structure — emit writer's custom rules if present.
		var custom structureCustomData
		if json.Unmarshal(row.StructureCustom, &custom) == nil && len(custom.Rules) > 0 {
			out.WriteString("## Story rules\n")
			for _, rule := range custom.Rules {
				if rule = strings.TrimSpace(rule); rule != "" {
					out.WriteString("- " + rule + "\n")
				}
			}
		}
	}

	return out.String()
}

// ── buildPinnedContext ────────────────────────────────────────────────────────

// pinnedContentLimit is the maximum rune count of raw content included per pin
// when include_mode is "full" (protects against very long scenes bloating the prompt).
const pinnedContentLimit = 2000

// buildPinnedContext returns the "## Pinned context" block for writer-curated pins.
// Returns "" when the user has no pins for this project.
func (s *Service) buildPinnedContext(ctx context.Context, projectID, userID uuid.UUID, branchName string) string {
	pins, err := s.queries.ListContextPins(ctx, sqlcgen.ListContextPinsParams{
		ProjectID: projectID,
		UserID:    userID,
	})
	if err != nil || len(pins) == 0 {
		return ""
	}

	var out strings.Builder
	out.WriteString("## Pinned context\n")

	for _, pin := range pins {
		switch pin.PinType {
		case "entity":
			s.appendPinnedEntity(&out, ctx, pin.RefID, pin.IncludeMode)
		case "chapter":
			s.appendPinnedChapter(&out, ctx, pin.RefID, branchName, pin.IncludeMode)
		case "scene":
			s.appendPinnedScene(&out, ctx, pin.RefID, pin.IncludeMode)
		case "note":
			s.appendPinnedNote(&out, ctx, pin.RefID, pin.IncludeMode)
		}
	}

	if out.Len() == len("## Pinned context\n") {
		return "" // all pins failed to resolve
	}
	return out.String()
}

func (s *Service) appendPinnedEntity(out *strings.Builder, ctx context.Context, id uuid.UUID, mode string) {
	e, err := s.queries.GetEntity(ctx, id)
	if err != nil || e.Name == "" {
		return
	}
	out.WriteString(fmt.Sprintf("**%s** (%s)", e.Name, e.Type))
	if e.Summary != "" {
		out.WriteString(": " + e.Summary)
	}
	if mode == "full" {
		// Append attribute JSON if present and non-trivial.
		if len(e.Attributes) > 2 { // "{}" is 2 bytes
			out.WriteString("\nAttributes: " + string(e.Attributes))
		}
	}
	out.WriteString("\n")
}

func (s *Service) appendPinnedChapter(out *strings.Builder, ctx context.Context, id uuid.UUID, branchName, mode string) {
	ch, err := s.queries.GetChapter(ctx, id)
	if err != nil {
		return
	}

	out.WriteString("**Chapter: " + ch.Title + "**\n")

	if mode == "summary" {
		// Use the AI summary when available; fall back to raw excerpt.
		row, err := s.queries.GetChapterSummary(ctx, sqlcgen.GetChapterSummaryParams{
			ChapterID:  id,
			BranchName: branchName,
		})
		if err == nil && row.AiSummary != "" {
			out.WriteString(row.AiSummary + "\n")
			return
		}
		// No summary — fall through to scene content with the content limit.
	}

	scenes, _ := s.queries.ListScenesByChapter(ctx, id)
	var combined strings.Builder
	for i, sc := range scenes {
		if i > 0 {
			combined.WriteString("\n\n")
		}
		combined.WriteString(sc.Content)
	}
	content := []rune(combined.String())
	if mode == "summary" && len(content) > contentFallbackLimit {
		content = append(content[:contentFallbackLimit], []rune("…")...)
	} else if mode == "full" && len(content) > pinnedContentLimit {
		content = append(content[:pinnedContentLimit], []rune("…")...)
	}
	if len(content) > 0 {
		out.WriteString(string(content) + "\n")
	}
}

func (s *Service) appendPinnedNote(out *strings.Builder, ctx context.Context, id uuid.UUID, mode string) {
	n, err := s.queries.GetResearchNoteByID(ctx, id)
	if err != nil {
		return
	}
	out.WriteString("**Research note: " + n.Title + "**\n")
	body := []rune(n.Body)
	limit := pinnedContentLimit
	if mode == "summary" {
		limit = contentFallbackLimit
	}
	if len(body) > limit {
		body = append(body[:limit], []rune("…")...)
	}
	if len(body) > 0 {
		out.WriteString(string(body) + "\n")
	}
	if n.SourceUrl != "" {
		out.WriteString("Source: " + n.SourceUrl + "\n")
	}
}

func (s *Service) appendPinnedScene(out *strings.Builder, ctx context.Context, id uuid.UUID, mode string) {
	sc, err := s.queries.GetScene(ctx, id)
	if err != nil {
		return
	}
	label := "Scene"
	if sc.Title != "" {
		label = "Scene: " + sc.Title
	}
	out.WriteString("**" + label + "**\n")

	content := []rune(sc.Content)
	if mode == "summary" && len(content) > contentFallbackLimit {
		content = append(content[:contentFallbackLimit], []rune("…")...)
	} else if mode == "full" && len(content) > pinnedContentLimit {
		content = append(content[:pinnedContentLimit], []rune("…")...)
	}
	if len(content) > 0 {
		out.WriteString(string(content) + "\n")
	}
}
