package ai

// context.go — B2 AI memory layer + C6.6 prompt engineering audit.
//
// ResolveBranch determines which git Timeline (branch) the requesting user is
// currently on.  BuildContext assembles the context block that is prepended to
// every AI system prompt.  ScheduleSummarize debounces chapter-summary
// regeneration so rapid scene saves don't spam the LLM.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// canonBranch is the default Timeline name; mirrors project.CanonBranch.
const canonBranch = "canon"

// summarizeDebounce is the quiet-period after the last scene save before the
// background goroutine actually calls the LLM to regenerate a summary.
const summarizeDebounce = 30 * time.Second

// contextBudgetWarnChars is the character threshold above which BuildContext
// logs a warning (~5,000 tokens at 4 chars/token).
const contextBudgetWarnChars = 20_000

// debounceKey uniquely identifies a pending debounce timer.
type debounceKey struct {
	chapterID  uuid.UUID
	branchName string
}

// pendingWork holds everything needed to perform the deferred summarization.
type pendingWork struct {
	timer     *time.Timer
	userID    uuid.UUID
	projectID uuid.UUID
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
func (d *debouncer) schedule(key debounceKey, delay time.Duration, userID, projectID uuid.UUID, fn func(userID, projectID uuid.UUID)) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if p, ok := d.pending[key]; ok {
		// Absorb this save into the running debounce — bump the userID to the
		// latest writer so we use their API key for the eventual summarization.
		p.timer.Reset(delay)
		p.userID = userID
		p.projectID = projectID
		return
	}

	p := &pendingWork{userID: userID, projectID: projectID}
	p.timer = time.AfterFunc(delay, func() {
		d.mu.Lock()
		pw := d.pending[key]
		delete(d.pending, key)
		d.mu.Unlock()
		fn(pw.userID, pw.projectID)
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
func (s *Service) ScheduleSummarize(userID, chapterID, projectID uuid.UUID, branchName string) {
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
	s.debounce.schedule(key, summarizeDebounce, userID, projectID, func(uid, pid uuid.UUID) {
		s.regenerateSummary(uid, chapterID, pid, branchName)
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
func (s *Service) regenerateSummary(userID, chapterID, projectID uuid.UUID, branchName string) {
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
		sb.WriteString(s.readSceneContent(ctx, chapterID, sc.ID))
	}

	combined := strings.TrimSpace(sb.String())
	if combined == "" {
		// Git files not yet written or sceneWriter not wired — nothing to summarize.
		slog.Warn("ai: regenerate summary — no scene content available", "chapter_id", chapterID)
		return
	}

	summary, _, err := s.Summarize(ctx, userID, projectID, "", combined)
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
// Section order (C6.6 revised):
//  1. Project identity + AI bible
//  2. Story structure (named template or freeform rules)
//  3. Magic systems (always injected when rules exist — world constraints first)
//  4. Story so far (chapter summaries, branch-aware)
//  5. @[Entity] inline references (reformatted by entity type)
//  6. Current scene (full text; suppressed in Beat/Continue — caller passes uuid.Nil)
//  7. Pinned context (writer-curated via Context Pins panel)
//  8. Open story threads (unresolved threads the story owes the reader)
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

	// ── 2. Story structure ────────────────────────────────────────────────
	// World rules before story history — the AI should understand the structural
	// framework it's working within before reading chapter summaries.
	if structureCtx := s.buildStructureContext(ctx, projectID); structureCtx != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(structureCtx)
	}

	// ── 3. Magic systems ─────────────────────────────────────────────────
	// Always injected when the project has magic rules, regardless of @-references
	// in the current scene. Magic systems are world-level constraints the AI must
	// know even when the writer hasn't mentioned them in this scene.
	if magicCtx := s.buildMagicSystemsContext(ctx, projectID); magicCtx != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(magicCtx)
	}

	// ── 4. Story so far ───────────────────────────────────────────────────
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

	// Determine the current chapter's index for arc position hints on character entities.
	currentChapterIdx := -1
	if currentSceneID != uuid.Nil && err == nil {
		sc, scErr := s.queries.GetScene(ctx, currentSceneID)
		if scErr == nil {
			for i, ch := range chapters {
				if ch.ID == sc.ChapterID {
					currentChapterIdx = i
					break
				}
			}
		}
	}

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
			// Skip the current scene's chapter in the fallback path — its full
			// content is already included in section 6, so the excerpt would
			// duplicate it at lower quality.
			if currentSceneID != uuid.Nil {
				isCurrentChapter := false
				for _, sc := range scenes {
					if sc.ID == currentSceneID {
						isCurrentChapter = true
						break
					}
				}
				if isCurrentChapter {
					continue
				}
			}
			var rawSnippet strings.Builder
			for i, sc := range scenes {
				if i > 0 {
					rawSnippet.WriteString(" ")
				}
				rawSnippet.WriteString(strings.TrimSpace(s.readSceneContent(ctx, ch.ID, sc.ID)))
			}
			snippet := []rune(rawSnippet.String())
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

	// ── 5. Entities detected in this scene ───────────────────────────────
	// Read from pre-computed scene_entity_mentions (indexed by the wiki tagger
	// on each save). Suppressed mentions are excluded so the author's removals
	// are respected. Falls back to @[entity] regex when the scene has no indexed
	// mentions yet (e.g. first open before the tagger fires).
	if currentSceneID != uuid.Nil {
		mentionedEntities, mErr := s.queries.ListMentionedEntitiesByScene(ctx, sqlcgen.ListMentionedEntitiesBySceneParams{
			SceneID:    currentSceneID,
			BranchName: branchName,
		})
		if mErr == nil && len(mentionedEntities) > 0 {
			var entitySnippets []string
			for _, e := range mentionedEntities {
				line := buildEntityContextLine(e, currentChapterIdx, len(chapters))
				if line != "" {
					entitySnippets = append(entitySnippets, line)
				}
			}
			if len(entitySnippets) > 0 {
				if sb.Len() > 0 {
					sb.WriteString("\n")
				}
				sb.WriteString("## Entities in this scene\n")
				for _, snippet := range entitySnippets {
					sb.WriteString(snippet + "\n")
				}
			}
		} else {
			// Fallback: @[Entity Name] explicit references in scene content.
			refMatches := entityRefRE.FindAllStringSubmatch(sceneContent, -1)
			if len(refMatches) > 0 {
				seen := make(map[string]bool)
				var names []string
				for _, m := range refMatches {
					lower := strings.ToLower(m[1])
					if !seen[lower] {
						seen[lower] = true
						names = append(names, lower)
					}
				}
				entities, _ := s.queries.GetEntitiesByNames(ctx, sqlcgen.GetEntitiesByNamesParams{
					ProjectID: projectID,
					Names:     names,
				})
				var entitySnippets []string
				for _, e := range entities {
					line := buildEntityContextLine(e, currentChapterIdx, len(chapters))
					if line != "" {
						entitySnippets = append(entitySnippets, line)
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
		}
	}

	// ── 6. Current scene ─────────────────────────────────────────────────
	// Include the full text of the scene currently open in the editor so the
	// model can answer specific questions about it.
	if currentSceneID != uuid.Nil && sceneContent != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sc, scErr := s.queries.GetScene(ctx, currentSceneID)
		label := "Current scene"
		if scErr == nil && sc.Title != "" {
			label = fmt.Sprintf("Current scene — %s", sc.Title)
		}
		sb.WriteString(fmt.Sprintf("## %s\n%s\n", label, sceneContent))
	}

	// ── 7. Pinned context (writer-curated) ────────────────────────────────
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

	// ── 8. Open story threads ─────────────────────────────────────────────
	// Threads are the forward-looking context the AI can have — what the story
	// still owes the reader. Listing them here nudges Beat/Continue to naturally
	// advance open threads rather than drift.
	if threadCtx := s.buildOpenThreadsContext(ctx, projectID); threadCtx != "" {
		if sb.Len() > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(threadCtx)
	}

	if sb.Len() > contextBudgetWarnChars {
		slog.Warn("ai: context budget exceeded",
			"project_id", projectID,
			"chars", sb.Len(),
			"approx_tokens", sb.Len()/4,
		)
	}

	return sb.String()
}

// ── Entity context line formatter ─────────────────────────────────────────────

// charContextAttrs mirrors the character attribute fields stored in wiki_entities.attributes.
type charContextAttrs struct {
	Motivation      string `json:"motivation"`
	ArcStart        string `json:"arc_start"`
	ArcEnd          string `json:"arc_end"`
	CapabilityNotes string `json:"capability_notes"`
}

// buildEntityContextLine formats a single wiki entity as a context line,
// adapting the format to the entity type and available structured fields.
//
// chapterIdx is 0-based index of the current chapter; totalChapters is the full count.
// Both are used to inject an arc position hint for character entities.
func buildEntityContextLine(e sqlcgen.WikiEntity, chapterIdx, totalChapters int) string {
	if e.Name == "" {
		return ""
	}

	switch e.Type {
	case "character":
		return buildCharacterContextLine(e, chapterIdx, totalChapters)
	case "location":
		return buildLocationContextLine(e)
	case "faction":
		desc := truncateRunes(e.Summary, 400)
		if desc == "" {
			return fmt.Sprintf("**%s** (faction)", e.Name)
		}
		return fmt.Sprintf("**%s** (faction) — %s", e.Name, desc)
	default:
		if e.Summary == "" {
			return ""
		}
		return fmt.Sprintf("**%s** (%s): %s", e.Name, e.Type, truncateRunes(e.Summary, 500))
	}
}

func buildCharacterContextLine(e sqlcgen.WikiEntity, chapterIdx, totalChapters int) string {
	var attrs charContextAttrs
	if len(e.Attributes) > 0 {
		_ = json.Unmarshal(e.Attributes, &attrs)
	}

	// If no structured fields are populated, fall back to the generic format.
	if attrs.Motivation == "" && attrs.ArcStart == "" && attrs.ArcEnd == "" {
		if e.Summary == "" {
			return fmt.Sprintf("**%s** (character)", e.Name)
		}
		return fmt.Sprintf("**%s** (character): %s", e.Name, truncateRunes(e.Summary, 500))
	}

	var parts []string
	if attrs.Motivation != "" {
		parts = append(parts, "Motivation: "+attrs.Motivation)
	}
	if attrs.ArcStart != "" && attrs.ArcEnd != "" {
		arcLine := fmt.Sprintf("Arc: %s → %s", attrs.ArcStart, attrs.ArcEnd)
		if hint := arcPositionHint(chapterIdx, totalChapters); hint != "" {
			arcLine += " " + hint
		}
		parts = append(parts, arcLine)
	}
	if attrs.CapabilityNotes != "" {
		parts = append(parts, attrs.CapabilityNotes)
	}
	if desc := truncateRunes(e.Summary, 300); desc != "" {
		parts = append(parts, desc)
	}

	return fmt.Sprintf("**%s** (character) — %s", e.Name, strings.Join(parts, " | "))
}

func buildLocationContextLine(e sqlcgen.WikiEntity) string {
	desc := truncateRunes(e.Summary, 500)
	if desc == "" {
		return fmt.Sprintf("**%s** (location)", e.Name)
	}
	return fmt.Sprintf("**%s** (location) — %s", e.Name, desc)
}

// arcPositionHint returns "(early arc)", "(mid arc)", or "(late arc)" based on
// where the current chapter falls in the story, giving the AI calibration on
// where a character should be in their journey.
func arcPositionHint(chapterIdx, total int) string {
	if total <= 0 || chapterIdx < 0 {
		return ""
	}
	pos := float64(chapterIdx) / float64(total)
	switch {
	case pos < 0.33:
		return "(early arc)"
	case pos < 0.67:
		return "(mid arc)"
	default:
		return "(late arc)"
	}
}

// ── Magic systems context helper ──────────────────────────────────────────────

// magicContextAttrs mirrors MagicRuleAttributes from the wiki package for JSON parsing.
type magicContextAttrs struct {
	Powers       string `json:"powers"`
	Limitations  string `json:"limitations"`
	Cost         string `json:"cost"`
	RulesClarity string `json:"rules_clarity"`
}

// buildMagicSystemsContext returns a `## Magic systems` block listing the 5
// most recently updated magic rules for the project. Returns "" when no rules exist.
// Limitations are listed before Powers so the AI weighs constraints first.
func (s *Service) buildMagicSystemsContext(ctx context.Context, projectID uuid.UUID) string {
	rules, err := s.queries.ListMagicRulesForContext(ctx, projectID)
	if err != nil || len(rules) == 0 {
		return ""
	}

	var out strings.Builder
	out.WriteString("## Magic systems\n")

	for _, r := range rules {
		var attrs magicContextAttrs
		if len(r.Attributes) > 0 {
			_ = json.Unmarshal(r.Attributes, &attrs)
		}

		var parts []string
		if attrs.Limitations != "" {
			parts = append(parts, "Limitations — "+attrs.Limitations)
		}
		if attrs.Powers != "" {
			parts = append(parts, "Powers — "+attrs.Powers)
		}
		if attrs.Cost != "" {
			parts = append(parts, "Cost — "+attrs.Cost)
		}
		if attrs.RulesClarity == "defined" {
			parts = append(parts, "Do not introduce abilities not listed above.")
		}

		if len(parts) > 0 {
			out.WriteString(fmt.Sprintf("%s: %s\n", r.Name, strings.Join(parts, ". ")))
		} else if r.Description != "" {
			// No structured fields — fall back to freeform description.
			out.WriteString(fmt.Sprintf("%s: %s\n", r.Name, truncateRunes(r.Description, 300)))
		} else {
			out.WriteString(r.Name + "\n")
		}
	}

	return out.String()
}

// ── Open story threads context helper ────────────────────────────────────────

// buildOpenThreadsContext returns a `## Open story threads` block listing the
// open (unresolved) story threads for the project, most-recently-opened first,
// capped at 10. Returns "" when no open threads exist.
func (s *Service) buildOpenThreadsContext(ctx context.Context, projectID uuid.UUID) string {
	threads, err := s.queries.ListOpenThreadsByProject(ctx, projectID)
	if err != nil || len(threads) == 0 {
		return ""
	}

	var out strings.Builder
	out.WriteString("## Open story threads\n")

	for _, t := range threads {
		// Capitalise the type label for readability.
		typeLabel := strings.Title(t.Type) //nolint:staticcheck // simple capitalisation
		line := fmt.Sprintf("- %s: %q", typeLabel, t.Title)
		if t.ChapterTitle != "" {
			line += " — opened in " + t.ChapterTitle
		}
		out.WriteString(line + "\n")
	}

	return out.String()
}

// ── Scene attributes for Beat/Continue prompts ────────────────────────────────

// SceneContextAttrs holds the structured scene metadata used to enrich
// Beat and Continue system prompts with scene-specific context.
type SceneContextAttrs struct {
	SceneRole     string `json:"scene_role"`
	SceneGoal     string `json:"scene_goal"`
	SceneConflict string `json:"scene_conflict"`
}

// ParseSceneContextAttrs decodes the attributes JSONB column of a scene.
// Returns an empty struct (all fields "") if the column is empty or invalid JSON.
func ParseSceneContextAttrs(raw []byte) SceneContextAttrs {
	var a SceneContextAttrs
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &a)
	}
	return a
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
		combined.WriteString(s.readSceneContent(ctx, id, sc.ID))
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

	content := []rune(s.readSceneContent(ctx, sc.ChapterID, sc.ID))
	if mode == "summary" && len(content) > contentFallbackLimit {
		content = append(content[:contentFallbackLimit], []rune("…")...)
	} else if mode == "full" && len(content) > pinnedContentLimit {
		content = append(content[:pinnedContentLimit], []rune("…")...)
	}
	if len(content) > 0 {
		out.WriteString(string(content) + "\n")
	}
}

// ── String utilities ─────────────────────────────────────────────────────────

// truncateRunes truncates s to at most n runes, appending "…" if truncated.
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	runes := []rune(s)
	return string(runes[:n]) + "…"
}
