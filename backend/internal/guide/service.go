package guide

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/jconder44/nexustale/pkg/apperror"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// stepOrder defines the canonical step sequence.
var stepOrder = []string{"premise", "characters", "world", "outline", "first_scene"}

// stepLabels maps step keys to human-readable labels.
var stepLabels = map[string]string{
	"premise":     "Premise",
	"characters":  "Core Characters",
	"world":       "World Basics",
	"outline":     "Chapter Outline",
	"first_scene": "First Scene",
}

// Service handles guide wizard state and side effects.
type Service struct {
	queries *sqlcgen.Queries
}

func NewService(queries *sqlcgen.Queries) *Service {
	return &Service{queries: queries}
}

// ── response types ────────────────────────────────────────────────────────────

// StepResponse is the API-facing representation of one guide step.
type StepResponse struct {
	StepKey     string          `json:"step_key"`
	Label       string          `json:"label"`
	Data        json.RawMessage `json:"data"`
	IsComplete  bool            `json:"is_complete"`
	CompletedAt string          `json:"completed_at,omitempty"` // RFC3339
}

// ProgressResponse is the full guide state for a project.
type ProgressResponse struct {
	Steps          []*StepResponse `json:"steps"`
	CompletedCount int             `json:"completed_count"`
	TotalCount     int             `json:"total_count"`
}

// ── JSONB data shapes ─────────────────────────────────────────────────────────
// These are decoded only when running side effects on completion.

type premiseData struct {
	Logline string   `json:"logline"`
	Theme   string   `json:"theme"`
	Genres  []string `json:"genres"`
}

type characterEntry struct {
	Name        string `json:"name"`
	Role        string `json:"role"`        // e.g. "protagonist", "antagonist", "supporting"
	Description string `json:"description"`
}

type charactersData struct {
	Characters []characterEntry `json:"characters"`
}

type locationEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type magicEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type worldData struct {
	Setting      string          `json:"setting"`
	Locations    []locationEntry `json:"locations"`
	MagicSystems []magicEntry    `json:"magic_systems"`
}

type chapterEntry struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
}

type outlineData struct {
	Chapters []chapterEntry `json:"chapters"`
}

type firstSceneData struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// ── public API ────────────────────────────────────────────────────────────────

// GetProgress returns the guide state for a project, including steps that haven't
// been started yet (represented with empty data and is_complete=false).
func (s *Service) GetProgress(ctx context.Context, projectID uuid.UUID) (*ProgressResponse, error) {
	rows, err := s.queries.ListGuideSteps(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list guide steps: %w", err)
	}

	// Index DB rows by step key.
	stored := make(map[string]sqlcgen.GuideStep, len(rows))
	for _, r := range rows {
		stored[r.StepKey] = r
	}

	steps := make([]*StepResponse, len(stepOrder))
	completed := 0
	for i, key := range stepOrder {
		sr := &StepResponse{
			StepKey: key,
			Label:   stepLabels[key],
			Data:    json.RawMessage("{}"),
		}
		if row, ok := stored[key]; ok {
			sr.Data = row.Data
			if row.CompletedAt.Valid {
				sr.IsComplete = true
				sr.CompletedAt = row.CompletedAt.Time.Format("2006-01-02T15:04:05Z07:00")
				completed++
			}
		}
		steps[i] = sr
	}

	return &ProgressResponse{
		Steps:          steps,
		CompletedCount: completed,
		TotalCount:     len(stepOrder),
	}, nil
}

// SaveStep persists step data without marking it complete.
// Idempotent — safe to call on every field blur.
func (s *Service) SaveStep(ctx context.Context, projectID uuid.UUID, stepKey string, data json.RawMessage) (*StepResponse, error) {
	if !validStep(stepKey) {
		return nil, apperror.Validation(fmt.Sprintf("unknown step key: %s", stepKey))
	}

	row, err := s.queries.UpsertGuideStep(ctx, sqlcgen.UpsertGuideStepParams{
		ProjectID: projectID,
		StepKey:   stepKey,
		Data:      data,
	})
	if err != nil {
		return nil, fmt.Errorf("upsert guide step: %w", err)
	}
	return toStepResponse(row), nil
}

// CompleteStep marks a step done and runs its side effects (creates wiki entities,
// chapters, scenes, etc.). Completing an already-complete step is a no-op for
// the DB row but side effects are not re-run.
func (s *Service) CompleteStep(ctx context.Context, projectID uuid.UUID, stepKey string, data json.RawMessage) (*StepResponse, error) {
	if !validStep(stepKey) {
		return nil, apperror.Validation(fmt.Sprintf("unknown step key: %s", stepKey))
	}

	// Check if already complete — don't re-run side effects.
	existing, _ := s.queries.GetGuideStep(ctx, sqlcgen.GetGuideStepParams{
		ProjectID: projectID,
		StepKey:   stepKey,
	})
	alreadyComplete := existing.CompletedAt.Valid

	row, err := s.queries.CompleteGuideStep(ctx, sqlcgen.CompleteGuideStepParams{
		ProjectID: projectID,
		StepKey:   stepKey,
		Data:      data,
	})
	if err != nil {
		return nil, fmt.Errorf("complete guide step: %w", err)
	}

	if !alreadyComplete {
		if err := s.runSideEffects(ctx, projectID, stepKey, data); err != nil {
			// Side effects are best-effort: log and continue so the step isn't
			// stuck incomplete just because e.g. a chapter already exists.
			slog.Warn("guide: side effect failed", "step", stepKey, "error", err)
		}
	}

	return toStepResponse(row), nil
}

// ── side effects ──────────────────────────────────────────────────────────────

func (s *Service) runSideEffects(ctx context.Context, projectID uuid.UUID, stepKey string, data json.RawMessage) error {
	switch stepKey {
	case "characters":
		return s.effectCharacters(ctx, projectID, data)
	case "world":
		return s.effectWorld(ctx, projectID, data)
	case "outline":
		return s.effectOutline(ctx, projectID, data)
	case "first_scene":
		return s.effectFirstScene(ctx, projectID, data)
	}
	return nil // premise has no side effects beyond persisting the data
}

func (s *Service) effectCharacters(ctx context.Context, projectID uuid.UUID, raw json.RawMessage) error {
	var d charactersData
	if err := json.Unmarshal(raw, &d); err != nil {
		return fmt.Errorf("decode characters data: %w", err)
	}
	for _, ch := range d.Characters {
		if ch.Name == "" {
			continue
		}
		attrs := json.RawMessage(`{}`)
		if ch.Role != "" {
			b, _ := json.Marshal(map[string]string{"role": ch.Role})
			attrs = b
		}
		if _, err := s.queries.CreateEntity(ctx, sqlcgen.CreateEntityParams{
			ProjectID:  projectID,
			Type:       "character",
			Name:       ch.Name,
			Summary:    ch.Description,
			Attributes: attrs,
		}); err != nil {
			slog.Warn("guide: create character entity failed", "name", ch.Name, "error", err)
		}
	}
	return nil
}

func (s *Service) effectWorld(ctx context.Context, projectID uuid.UUID, raw json.RawMessage) error {
	var d worldData
	if err := json.Unmarshal(raw, &d); err != nil {
		return fmt.Errorf("decode world data: %w", err)
	}
	for _, loc := range d.Locations {
		if loc.Name == "" {
			continue
		}
		if _, err := s.queries.CreateEntity(ctx, sqlcgen.CreateEntityParams{
			ProjectID:  projectID,
			Type:       "location",
			Name:       loc.Name,
			Summary:    loc.Description,
			Attributes: json.RawMessage(`{}`),
		}); err != nil {
			slog.Warn("guide: create location entity failed", "name", loc.Name, "error", err)
		}
	}
	for _, ms := range d.MagicSystems {
		if ms.Name == "" {
			continue
		}
		if _, err := s.queries.CreateMagicRule(ctx, sqlcgen.CreateMagicRuleParams{
			ProjectID:   projectID,
			Name:        ms.Name,
			Description: ms.Description,
		}); err != nil {
			slog.Warn("guide: create magic rule failed", "name", ms.Name, "error", err)
		}
	}
	return nil
}

func (s *Service) effectOutline(ctx context.Context, projectID uuid.UUID, raw json.RawMessage) error {
	var d outlineData
	if err := json.Unmarshal(raw, &d); err != nil {
		return fmt.Errorf("decode outline data: %w", err)
	}
	if len(d.Chapters) == 0 {
		return nil
	}

	// Use the project's default act (first act by sort_order).
	acts, err := s.queries.ListActsByProject(ctx, projectID)
	if err != nil || len(acts) == 0 {
		return fmt.Errorf("list acts: %w", err)
	}
	actID := acts[0].ID

	// Determine starting sort_order by checking existing chapters.
	existing, _ := s.queries.ListChaptersByAct(ctx, actID)
	nextOrder := int32(len(existing) + 1)

	for i, ch := range d.Chapters {
		if ch.Title == "" {
			continue
		}
		if _, err := s.queries.CreateChapter(ctx, sqlcgen.CreateChapterParams{
			ProjectID: projectID,
			ActID:     actID,
			Title:     ch.Title,
			Summary:   ch.Summary,
			SortOrder: nextOrder + int32(i),
		}); err != nil {
			slog.Warn("guide: create chapter failed", "title", ch.Title, "error", err)
		}
	}
	return nil
}

func (s *Service) effectFirstScene(ctx context.Context, projectID uuid.UUID, raw json.RawMessage) error {
	var d firstSceneData
	if err := json.Unmarshal(raw, &d); err != nil {
		return fmt.Errorf("decode first_scene data: %w", err)
	}

	// Find the first chapter across all acts.
	acts, err := s.queries.ListActsByProject(ctx, projectID)
	if err != nil || len(acts) == 0 {
		return fmt.Errorf("list acts: %w", err)
	}

	var chapterID uuid.UUID
	for _, act := range acts {
		chapters, err := s.queries.ListChaptersByAct(ctx, act.ID)
		if err != nil || len(chapters) == 0 {
			continue
		}
		chapterID = chapters[0].ID
		break
	}
	if chapterID == uuid.Nil {
		return fmt.Errorf("no chapters found; complete the outline step first")
	}

	title := d.Title
	if title == "" {
		title = "Opening Scene"
	}

	// Count existing scenes so we don't step on them.
	existing, _ := s.queries.ListScenesByChapter(ctx, chapterID)
	sortOrder := int32(len(existing) + 1)

	wc := int32(len([]rune(d.Content)) / 5) // rough word count estimate
	if _, err := s.queries.CreateScene(ctx, sqlcgen.CreateSceneParams{
		ChapterID: chapterID,
		Title:     title,
		Content:   d.Content,
		SortOrder: sortOrder,
		WordCount: wc,
	}); err != nil {
		return fmt.Errorf("create first scene: %w", err)
	}
	return nil
}

// ── structure methods ─────────────────────────────────────────────────────────

// NovelStructureResponse is the API shape for a single structure template.
type NovelStructureResponse struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Phases      json.RawMessage `json:"phases"`
	Strengths   string          `json:"strengths"`
	Risks       string          `json:"risks"`
}

// ProjectStructureResponse is the API shape for a project's current structure.
type ProjectStructureResponse struct {
	StructureID     *string         `json:"structure_id"`
	StructureName   *string         `json:"structure_name"`
	Phases          json.RawMessage `json:"phases"`
	StructureCustom json.RawMessage `json:"structure_custom"`
}

// UpdateStructureRequest is the body for PUT /projects/:id/structure.
type UpdateStructureRequest struct {
	StructureID     *string         `json:"structure_id"`      // null = clear
	StructureCustom json.RawMessage `json:"structure_custom"`  // null = clear
}

// ListStructures returns all seeded novel structure templates.
func (s *Service) ListStructures(ctx context.Context) ([]NovelStructureResponse, error) {
	rows, err := s.queries.ListNovelStructures(ctx)
	if err != nil {
		return nil, fmt.Errorf("list novel structures: %w", err)
	}
	out := make([]NovelStructureResponse, len(rows))
	for i, r := range rows {
		out[i] = NovelStructureResponse{
			ID:          r.ID.String(),
			Name:        r.Name,
			Description: r.Description,
			Phases:      r.Phases,
			Strengths:   r.Strengths,
			Risks:       r.Risks,
		}
	}
	return out, nil
}

// GetStructure returns the structure selection for a project.
func (s *Service) GetStructure(ctx context.Context, projectID uuid.UUID) (*ProjectStructureResponse, error) {
	row, err := s.queries.GetProjectStructure(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("get project structure: %w", err)
	}
	resp := &ProjectStructureResponse{}
	if row.StructureID.Valid {
		id := row.StructureID.Bytes
		idStr := uuid.UUID(id).String()
		resp.StructureID = &idStr
	}
	if row.StructureName.Valid {
		resp.StructureName = &row.StructureName.String
	}
	if len(row.Phases) > 0 {
		resp.Phases = row.Phases
	} else {
		resp.Phases = json.RawMessage("[]")
	}
	if len(row.StructureCustom) > 0 {
		resp.StructureCustom = row.StructureCustom
	}
	return resp, nil
}

// UpdateStructure sets or clears the structure selection on a project.
func (s *Service) UpdateStructure(ctx context.Context, projectID uuid.UUID, req UpdateStructureRequest) (*ProjectStructureResponse, error) {
	params := sqlcgen.UpdateProjectStructureParams{ID: projectID}

	if req.StructureID != nil {
		sid, err := uuid.Parse(*req.StructureID)
		if err != nil {
			return nil, apperror.Validation("invalid structure_id: not a UUID")
		}
		params.StructureID.Bytes = [16]byte(sid)
		params.StructureID.Valid = true
	}
	if len(req.StructureCustom) > 0 && string(req.StructureCustom) != "null" {
		params.StructureCustom = req.StructureCustom
	}

	if _, err := s.queries.UpdateProjectStructure(ctx, params); err != nil {
		return nil, fmt.Errorf("update project structure: %w", err)
	}
	return s.GetStructure(ctx, projectID)
}

// ScoreStructures runs the scoring matrix and returns ranked structure recommendations.
// It does NOT persist anything — purely a computation.
func (s *Service) ScoreStructures(ctx context.Context, answers map[string][]string) ([]StructureScore, error) {
	rows, err := s.queries.ListNovelStructures(ctx)
	if err != nil {
		return nil, fmt.Errorf("list structures for scoring: %w", err)
	}
	catalog := make([]Structure, len(rows))
	for i, r := range rows {
		catalog[i] = Structure{ID: r.ID.String(), Name: r.Name}
	}
	return Score(answers, catalog), nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func validStep(key string) bool {
	for _, k := range stepOrder {
		if k == key {
			return true
		}
	}
	return false
}

func toStepResponse(row sqlcgen.GuideStep) *StepResponse {
	sr := &StepResponse{
		StepKey: row.StepKey,
		Label:   stepLabels[row.StepKey],
		Data:    row.Data,
	}
	if row.CompletedAt.Valid {
		sr.IsComplete = true
		sr.CompletedAt = row.CompletedAt.Time.Format("2006-01-02T15:04:05Z07:00")
	}
	return sr
}

