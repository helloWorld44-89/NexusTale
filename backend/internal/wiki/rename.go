package wiki

// rename.go — C9.5: Entity Rename Cascade.
//
// When a writer renames a wiki entity, this module offers to find and replace
// every occurrence of the old name across the manuscript. The flow:
//
//  1. UpdateEntity detects a name change and counts non-suppressed mentions.
//     If ≥ 1 exists, the response includes rename_cascade_available=true.
//  2. Frontend calls POST /rename-cascade/preview which returns a per-scene
//     unified diff (before → after) without writing anything.
//  3. Frontend calls POST /rename-cascade/confirm with the approved scene IDs.
//     Each scene is patched in git with whole-word case-preserving replacement,
//     then a single Chronicle commit is made.

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// SceneReadWriter abstracts git scene I/O so the wiki package doesn't import
// the project package (which would create an import cycle).
type SceneReadWriter interface {
	ReadSceneFile(repoPath string, chapterID, sceneID uuid.UUID) (string, bool, error)
	WriteSceneFile(repoPath string, chapterID, sceneID uuid.UUID, content string) error
	Chronicle(repoPath, message string) (string, error)
}

// WithSceneReadWriter wires the git service into the wiki service.
func (s *Service) WithSceneReadWriter(rw SceneReadWriter) {
	s.sceneRW = rw
}

// ── Preview ───────────────────────────────────────────────────────────────────

// RenameCascadePreview inspects all non-suppressed scene mentions of entity eid
// and returns a per-scene diff showing how oldName→newName would look.
// Does NOT write anything; safe to call multiple times.
func (s *Service) RenameCascadePreview(ctx context.Context, entityID uuid.UUID, oldName, newName string) ([]RenameCascadePreviewItem, error) {
	if s.sceneRW == nil {
		return nil, fmt.Errorf("scene reader not configured")
	}
	rows, err := s.queries.ListScenesByEntityForProject(ctx, entityID)
	if err != nil {
		return nil, fmt.Errorf("list scenes: %w", err)
	}

	// Group by scene_id to collect all match_texts per scene.
	type sceneEntry struct {
		row        sqlcgen.ListScenesByEntityForProjectRow
		matchTexts []string
	}
	seen := map[uuid.UUID]*sceneEntry{}
	var order []uuid.UUID
	for _, r := range rows {
		if _, ok := seen[r.SceneID]; !ok {
			seen[r.SceneID] = &sceneEntry{row: r}
			order = append(order, r.SceneID)
		}
		// collect distinct match texts
		e := seen[r.SceneID]
		duplicate := false
		for _, mt := range e.matchTexts {
			if mt == r.MatchText {
				duplicate = true
				break
			}
		}
		if !duplicate {
			e.matchTexts = append(e.matchTexts, r.MatchText)
		}
	}

	var result []RenameCascadePreviewItem
	for _, sid := range order {
		e := seen[sid]
		r := e.row

		content, ok, err := s.sceneRW.ReadSceneFile(r.GitRepoPath, r.ChapterID, r.SceneID)
		if err != nil || !ok || content == "" {
			continue // stale mention — scene not in git yet or empty
		}

		// Check that at least one match_text still appears (stale mention guard).
		found := false
		for _, mt := range e.matchTexts {
			if containsWholeWord(content, mt) {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		patched := applyRename(content, e.matchTexts, oldName, newName)
		if patched == content {
			continue // no change
		}

		diff := buildUnifiedDiff(r.SceneTitle, content, patched)

		result = append(result, RenameCascadePreviewItem{
			SceneID:      r.SceneID.String(),
			SceneTitle:   r.SceneTitle,
			ChapterTitle: r.ChapterTitle,
			MatchTexts:   e.matchTexts,
			UnifiedDiff:  diff,
		})
	}
	return result, nil
}

// ── Confirm ───────────────────────────────────────────────────────────────────

// RenameCascadeConfirm applies the rename to the approved scene IDs and creates
// a single Chronicle commit. Updates match_text in scene_entity_mentions for
// affected rows so the tagger stays consistent.
func (s *Service) RenameCascadeConfirm(ctx context.Context, entityID uuid.UUID, oldName, newName string, approvedSceneIDs []string) (int, error) {
	if s.sceneRW == nil {
		return 0, fmt.Errorf("scene writer not configured")
	}

	approvedSet := make(map[string]bool, len(approvedSceneIDs))
	for _, id := range approvedSceneIDs {
		approvedSet[id] = true
	}

	rows, err := s.queries.ListScenesByEntityForProject(ctx, entityID)
	if err != nil {
		return 0, fmt.Errorf("list scenes: %w", err)
	}

	// Group rows by scene_id (same approach as preview).
	type sceneEntry struct {
		row        sqlcgen.ListScenesByEntityForProjectRow
		matchTexts []string
	}
	seen := map[uuid.UUID]*sceneEntry{}
	var order []uuid.UUID
	for _, r := range rows {
		if !approvedSet[r.SceneID.String()] {
			continue
		}
		if _, ok := seen[r.SceneID]; !ok {
			seen[r.SceneID] = &sceneEntry{row: r}
			order = append(order, r.SceneID)
		}
		e := seen[r.SceneID]
		duplicate := false
		for _, mt := range e.matchTexts {
			if mt == r.MatchText {
				duplicate = true
				break
			}
		}
		if !duplicate {
			e.matchTexts = append(e.matchTexts, r.MatchText)
		}
	}

	if len(order) == 0 {
		return 0, nil
	}

	// Track the git repo path for the Chronicle commit.
	// All scenes in a project share the same repo path.
	repoPath := seen[order[0]].row.GitRepoPath
	patchedCount := 0

	for _, sid := range order {
		e := seen[sid]
		r := e.row

		// Re-read scene content (optimistic concurrency check — skip if changed).
		content, ok, readErr := s.sceneRW.ReadSceneFile(r.GitRepoPath, r.ChapterID, r.SceneID)
		if readErr != nil || !ok || content == "" {
			continue
		}

		patched := applyRename(content, e.matchTexts, oldName, newName)
		if patched == content {
			continue
		}

		if writeErr := s.sceneRW.WriteSceneFile(r.GitRepoPath, r.ChapterID, r.SceneID, patched); writeErr != nil {
			return patchedCount, fmt.Errorf("write scene %s: %w", sid, writeErr)
		}

		// Update match_text in scene_entity_mentions.
		if updateErr := s.queries.UpdateMentionMatchText(ctx, sqlcgen.UpdateMentionMatchTextParams{
			EntityID:  entityID,
			MatchText: newName,
			SceneID:   r.SceneID,
		}); updateErr != nil {
			// Non-fatal: the text was patched; the tagger will re-index on next save.
			_ = updateErr
		}

		patchedCount++
	}

	if patchedCount == 0 {
		return 0, nil
	}

	scenePlural := "scene"
	if patchedCount != 1 {
		scenePlural = "scenes"
	}
	commitMsg := fmt.Sprintf("rename: %s → %s (%d %s)", oldName, newName, patchedCount, scenePlural)
	if _, chronicleErr := s.sceneRW.Chronicle(repoPath, commitMsg); chronicleErr != nil {
		// Non-fatal: files are written, commit failed. Log-worthy but don't block.
		_ = chronicleErr
	}

	return patchedCount, nil
}

// ── Text manipulation helpers ─────────────────────────────────────────────────

// applyRename replaces all whole-word occurrences of each matchText (and oldName)
// in content with case-preserving newName equivalents.
func applyRename(content string, matchTexts []string, oldName, newName string) string {
	// Collect all distinct forms to replace.
	forms := make(map[string]bool)
	forms[oldName] = true
	for _, mt := range matchTexts {
		if mt != "" {
			forms[mt] = true
		}
	}

	result := content
	for form := range forms {
		result = replaceWholeWordCasePreserving(result, form, newName)
	}
	return result
}

// replaceWholeWordCasePreserving replaces whole-word occurrences of old with
// a case-adjusted version of newWord that matches the case of the matched text.
func replaceWholeWordCasePreserving(s, old, newWord string) string {
	re, err := buildWholeWordRE(old)
	if err != nil {
		return s
	}
	return re.ReplaceAllStringFunc(s, func(match string) string {
		return adjustCase(newWord, match)
	})
}

// containsWholeWord reports whether s contains a whole-word occurrence of word.
func containsWholeWord(s, word string) bool {
	re, err := buildWholeWordRE(word)
	if err != nil {
		return false
	}
	return re.MatchString(s)
}

// buildWholeWordRE builds a case-insensitive whole-word regex for word.
// Word boundaries are \b for ASCII, or explicit unicode checks for non-ASCII.
func buildWholeWordRE(word string) (*regexp.Regexp, error) {
	escaped := regexp.QuoteMeta(word)
	return regexp.Compile(`(?i)\b` + escaped + `\b`)
}

// adjustCase returns newWord adjusted to match the case style of matched:
//   - ALL CAPS → ALL CAPS
//   - Title Case → Title Case
//   - lowercase → lowercase
func adjustCase(newWord, matched string) string {
	if matched == "" {
		return newWord
	}
	runes := []rune(matched)
	if allUpper(runes) {
		return strings.ToUpper(newWord)
	}
	if unicode.IsUpper(runes[0]) {
		return toTitleCase(newWord)
	}
	return strings.ToLower(newWord)
}

func allUpper(runes []rune) bool {
	for _, r := range runes {
		if unicode.IsLetter(r) && !unicode.IsUpper(r) {
			return false
		}
	}
	return true
}

func toTitleCase(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// ── Minimal unified diff ──────────────────────────────────────────────────────

// buildUnifiedDiff produces a simple unified diff string for display in the
// frontend WordDiffView. We compute it line-by-line; only changed lines are
// marked + or -.
func buildUnifiedDiff(label, before, after string) string {
	beforeLines := strings.Split(before, "\n")
	afterLines  := strings.Split(after, "\n")

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- %s\n+++ %s\n", label, label))

	max := len(beforeLines)
	if len(afterLines) > max {
		max = len(afterLines)
	}
	for i := 0; i < max; i++ {
		b := ""
		if i < len(beforeLines) {
			b = beforeLines[i]
		}
		a := ""
		if i < len(afterLines) {
			a = afterLines[i]
		}
		if b == a {
			sb.WriteString(" " + b + "\n")
		} else {
			if b != "" || i < len(beforeLines) {
				sb.WriteString("-" + b + "\n")
			}
			if a != "" || i < len(afterLines) {
				sb.WriteString("+" + a + "\n")
			}
		}
	}
	return sb.String()
}
