package wiki

import (
	"context"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

const taggerDelay = 5 * time.Second

type tagDebounceKey struct {
	sceneID    uuid.UUID
	branchName string
}

type tagPendingWork struct {
	timer     *time.Timer
	projectID uuid.UUID
	content   string
}

type tagger struct {
	mu      sync.Mutex
	pending map[tagDebounceKey]*tagPendingWork
}

func newTagger() *tagger {
	return &tagger{pending: make(map[tagDebounceKey]*tagPendingWork)}
}

// schedule debounces entity detection for a scene. If a save arrives before
// the previous timer fires, the timer is reset and content is replaced so
// only the latest version is scanned.
func (t *tagger) schedule(sceneID, projectID uuid.UUID, branchName, content string, fn func(sceneID, projectID uuid.UUID, branchName, content string)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	key := tagDebounceKey{sceneID: sceneID, branchName: branchName}
	if p, ok := t.pending[key]; ok {
		p.timer.Reset(taggerDelay)
		p.content = content
		p.projectID = projectID
		return
	}

	p := &tagPendingWork{projectID: projectID, content: content}
	p.timer = time.AfterFunc(taggerDelay, func() {
		t.mu.Lock()
		pw := t.pending[key]
		delete(t.pending, key)
		t.mu.Unlock()
		fn(sceneID, pw.projectID, branchName, pw.content)
	})
	t.pending[key] = p
}

// IndexSceneMentions schedules a debounced entity detection pass for a scene.
// It is safe to call from the scene-save hot path — it returns immediately.
func (s *Service) IndexSceneMentions(ctx context.Context, projectID, sceneID uuid.UUID, branchName, content string) {
	s.tagger.schedule(sceneID, projectID, branchName, content, s.runDetection)
}

// runDetection is called by the tagger after the debounce delay. It checks
// auto_tag_enabled, runs whole-word case-insensitive detection, then atomically
// replaces the non-suppressed mention rows for (scene_id, branch_name).
func (s *Service) runDetection(sceneID, projectID uuid.UUID, branchName, content string) {
	ctx := context.Background()

	// Respect the per-project auto_tag_enabled flag.
	p, err := s.queries.GetProject(ctx, projectID)
	if err != nil {
		slog.Warn("tagger: get project failed", "project_id", projectID, "error", err)
		return
	}
	if !p.AutoTagEnabled {
		return
	}

	entities, err := s.queries.ListEntitiesByProject(ctx, sqlcgen.ListEntitiesByProjectParams{
		ProjectID: projectID,
	})
	if err != nil {
		slog.Warn("tagger: list entities failed", "project_id", projectID, "error", err)
		return
	}

	type match struct {
		entity    sqlcgen.WikiEntity
		matchText string
	}
	var matches []match
	for _, e := range entities {
		re, reErr := regexp.Compile(`(?i)\b` + regexp.QuoteMeta(e.Name) + `\b`)
		if reErr != nil {
			continue
		}
		found := re.FindString(content)
		if found != "" {
			matches = append(matches, match{entity: e, matchText: found})
		}
	}

	// Delete non-suppressed rows then upsert fresh matches.
	// Suppressed rows are left untouched so author removals persist.
	if err := s.queries.DeleteSceneEntityMentions(ctx, sqlcgen.DeleteSceneEntityMentionsParams{
		SceneID:    sceneID,
		BranchName: branchName,
	}); err != nil {
		slog.Warn("tagger: delete failed", "scene_id", sceneID, "error", err)
		return
	}

	for _, m := range matches {
		if _, err := s.queries.UpsertSceneEntityMention(ctx, sqlcgen.UpsertSceneEntityMentionParams{
			SceneID:    sceneID,
			EntityID:   m.entity.ID,
			ProjectID:  projectID,
			BranchName: branchName,
			MatchText:  m.matchText,
		}); err != nil {
			slog.Warn("tagger: upsert failed", "scene_id", sceneID, "entity", m.entity.Name, "error", err)
		}
	}
}
