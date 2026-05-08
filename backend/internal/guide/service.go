package guide

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

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

// SceneFileWriter writes scene content to the git working tree.
// Implemented by *project.GitService; injected via WithSceneWriter.
type SceneFileWriter interface {
	WriteSceneFile(repoPath string, chapterID, sceneID uuid.UUID, content string) error
}

// Service handles guide wizard state and side effects.
type Service struct {
	queries     *sqlcgen.Queries
	sceneWriter SceneFileWriter
}

func NewService(queries *sqlcgen.Queries) *Service {
	return &Service{queries: queries}
}

// WithSceneWriter wires the git scene file writer (called from main after both
// services are constructed — same pattern as WithNotifier on project.Service).
func (s *Service) WithSceneWriter(w SceneFileWriter) {
	s.sceneWriter = w
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
	Logline      string   `json:"logline"`
	Theme        string   `json:"theme"`
	Genres       []string `json:"genres"`
	Audience     string   `json:"audience"`      // middle_grade | young_adult | new_adult | adult
	WritingVoice string   `json:"writing_voice"` // sparse_direct | lyrical | ya_contemporary | epic_sweeping | thriller_pacing | literary
}

// audienceDirectives maps audience keys to content-guideline sentences injected into the AI Bible.
var audienceDirectives = map[string]string{
	"middle_grade":  "Content guidelines: Middle-grade story for ages 8–12. Keep content age-appropriate: danger and adventure are welcome, but avoid graphic violence, explicit romance, and strong language.",
	"young_adult":   "Content guidelines: Young adult story for ages 13–18. Violence should have weight but not be gratuitous. Romance stays tasteful. Avoid strong profanity and explicit content.",
	"new_adult":     "Content guidelines: New adult story for ages 18–25. Mature themes, moderate language, and romantic tension are appropriate. Avoid explicitly graphic content.",
	"adult":         "Content guidelines: Adult fiction. Mature themes and language are appropriate when they serve the story.",
}

// voicePresets maps writing voice keys to the style guidance text stored in the auto-created project_prompt.
var voicePresets = map[string]struct{ name, content string }{
	"sparse_direct":   {name: "Sparse & Direct", content: "Write in a spare, direct style: short sentences, strong verbs, no filler adverbs. Trust the reader to feel what you show. When in doubt, cut."},
	"lyrical":         {name: "Lyrical", content: "Write with a lyrical, sensory voice: rich imagery, varied rhythm, attention to sound and texture. Let sentences breathe between action beats."},
	"ya_contemporary": {name: "YA Contemporary", content: "Write in a YA contemporary voice: conversational, close, with strong interiority. Sentences that feel like how a teenager actually thinks and notices the world."},
	"epic_sweeping":   {name: "Epic / Sweeping", content: "Write in an elevated, sweeping style: longer sentences that carry weight, language that implies history and scope, a sense that the world existed before the story began."},
	"thriller_pacing": {name: "Thriller Pacing", content: "Write for tension: short paragraphs, punchy sentences, white space. End every paragraph with a reason to read the next one."},
	"literary":        {name: "Literary", content: "Write with literary depth: psychological complexity, ambiguous morality, prose that earns close reading. Let character interiority drive the scene."},
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
		// Auto-fill the AI bible when instructions are empty.
		go s.AutoFillAIInstructions(context.Background(), projectID)
	}

	return toStepResponse(row), nil
}

// ── AI Bible ──────────────────────────────────────────────────────────────────

// GenerateAIInstructions builds a story-bible text from the project's completed
// guide steps: premise (logline, theme, genres), characters, and world
// (setting, locations, magic systems).  Returns an empty string when no
// useful guide data exists yet.
func (s *Service) GenerateAIInstructions(ctx context.Context, projectID uuid.UUID) (string, error) {
	rows, err := s.queries.ListGuideSteps(ctx, projectID)
	if err != nil {
		return "", fmt.Errorf("list guide steps: %w", err)
	}

	stored := make(map[string]sqlcgen.GuideStep)
	for _, r := range rows {
		stored[r.StepKey] = r
	}

	var sb strings.Builder

	// Fetch project for title/genres.
	p, err := s.queries.GetProject(ctx, projectID)
	if err == nil && p.Title != "" {
		if len(p.Genres) > 0 {
			sb.WriteString("\"" + p.Title + "\" is a " + strings.Join(p.Genres, "/") + " story.\n")
		} else {
			sb.WriteString("\"" + p.Title + "\" is a story.\n")
		}
	}

	// Premise block.
	if row, ok := stored["premise"]; ok {
		var d premiseData
		if json.Unmarshal(row.Data, &d) == nil {
			if d.Logline != "" {
				sb.WriteString("\nPremise: " + d.Logline + "\n")
			}
			if d.Theme != "" {
				sb.WriteString("Theme: " + d.Theme + "\n")
			}
			if directive, ok := audienceDirectives[d.Audience]; ok {
				sb.WriteString("\n" + directive + "\n")
			}
		}
	}

	// Characters block.
	if row, ok := stored["characters"]; ok {
		var d charactersData
		if json.Unmarshal(row.Data, &d) == nil && len(d.Characters) > 0 {
			sb.WriteString("\nCore characters:\n")
			for _, ch := range d.Characters {
				if ch.Name == "" {
					continue
				}
				line := "- " + ch.Name
				if ch.Role != "" {
					line += " (" + ch.Role + ")"
				}
				if ch.Description != "" {
					line += ": " + ch.Description
				}
				sb.WriteString(line + "\n")
			}
		}
	}

	// World block.
	if row, ok := stored["world"]; ok {
		var d worldData
		if json.Unmarshal(row.Data, &d) == nil {
			if d.Setting != "" {
				sb.WriteString("\nWorld/Setting: " + d.Setting + "\n")
			}
			if len(d.Locations) > 0 {
				sb.WriteString("Key locations:\n")
				for _, loc := range d.Locations {
					if loc.Name == "" {
						continue
					}
					line := "- " + loc.Name
					if loc.Description != "" {
						line += ": " + loc.Description
					}
					sb.WriteString(line + "\n")
				}
			}
			if len(d.MagicSystems) > 0 {
				sb.WriteString("Systems/Magic:\n")
				for _, ms := range d.MagicSystems {
					if ms.Name == "" {
						continue
					}
					line := "- " + ms.Name
					if ms.Description != "" {
						line += ": " + ms.Description
					}
					sb.WriteString(line + "\n")
				}
			}
		}
	}

	out := strings.TrimSpace(sb.String())

	// Cap at ~1,200 characters, trimming at the last sentence boundary so the
	// bible doesn't bloat every AI call for verbose guide wizard entries.
	const maxBibleChars = 1200
	if runes := []rune(out); len(runes) > maxBibleChars {
		trimmed := string(runes[:maxBibleChars])
		// Walk back to the last sentence-ending punctuation so we don't cut mid-word.
		if i := strings.LastIndexAny(trimmed, ".!?"); i > 0 {
			trimmed = trimmed[:i+1]
		}
		out = trimmed
	}

	return out, nil
}

// GetAIInstructionsText returns the stored ai_instructions text for a project.
func (s *Service) GetAIInstructionsText(ctx context.Context, projectID uuid.UUID) (string, error) {
	return s.queries.GetAIInstructions(ctx, projectID)
}

// SaveAIInstructions persists the given text as the project's ai_instructions.
func (s *Service) SaveAIInstructions(ctx context.Context, projectID uuid.UUID, text string) error {
	return s.queries.UpdateAIInstructions(ctx, sqlcgen.UpdateAIInstructionsParams{
		ID:             projectID,
		AiInstructions: text,
	})
}

// AutoFillAIInstructions generates and saves ai_instructions for a project only
// when the field is currently empty.  Called non-blocking after a guide step
// completes so the user always has a usable baseline without manual action.
func (s *Service) AutoFillAIInstructions(ctx context.Context, projectID uuid.UUID) {
	existing, err := s.queries.GetAIInstructions(ctx, projectID)
	if err != nil || strings.TrimSpace(existing) != "" {
		return // already has content — never overwrite user edits
	}
	text, err := s.GenerateAIInstructions(ctx, projectID)
	if err != nil || text == "" {
		return
	}
	if err := s.queries.UpdateAIInstructions(ctx, sqlcgen.UpdateAIInstructionsParams{
		ID:             projectID,
		AiInstructions: text,
	}); err != nil {
		slog.Warn("guide: auto-fill ai_instructions failed", "project_id", projectID, "error", err)
	}
}

// ── side effects ──────────────────────────────────────────────────────────────

func (s *Service) runSideEffects(ctx context.Context, projectID uuid.UUID, stepKey string, data json.RawMessage) error {
	switch stepKey {
	case "premise":
		return s.effectPremise(ctx, projectID, data)
	case "characters":
		return s.effectCharacters(ctx, projectID, data)
	case "world":
		return s.effectWorld(ctx, projectID, data)
	case "outline":
		return s.effectOutline(ctx, projectID, data)
	case "first_scene":
		return s.effectFirstScene(ctx, projectID, data)
	}
	return nil
}

// effectPremise creates a writing-voice project_prompt when the premise step is completed
// and a voice was selected. This gives the SceneMetadataPanel dropdown a pre-populated
// starting style without the user needing to create one manually.
func (s *Service) effectPremise(ctx context.Context, projectID uuid.UUID, raw json.RawMessage) error {
	var d premiseData
	if err := json.Unmarshal(raw, &d); err != nil {
		return fmt.Errorf("decode premise data: %w", err)
	}
	preset, ok := voicePresets[d.WritingVoice]
	if !ok {
		return nil // no voice selected — nothing to create
	}
	if _, err := s.queries.CreateProjectPrompt(ctx, sqlcgen.CreateProjectPromptParams{
		ProjectID:     projectID,
		Name:          preset.name,
		Category:      "prose",
		Content:       preset.content,
		SystemContent: "",
		SortOrder:     0,
	}); err != nil {
		slog.Warn("guide: create voice preset failed", "voice", d.WritingVoice, "error", err)
	}
	return nil
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
	scene, err := s.queries.CreateScene(ctx, sqlcgen.CreateSceneParams{
		ChapterID: chapterID,
		Title:     title,
		SortOrder: sortOrder,
		WordCount: wc,
	})
	if err != nil {
		return fmt.Errorf("create first scene: %w", err)
	}

	// Write content to git working tree (not stored in DB after Step 4).
	if s.sceneWriter != nil {
		if proj, pErr := s.queries.GetProject(ctx, projectID); pErr == nil {
			if wErr := s.sceneWriter.WriteSceneFile(proj.GitRepoPath, scene.ChapterID, scene.ID, d.Content); wErr != nil {
				slog.Warn("guide: git dual-write failed", "scene_id", scene.ID, "error", wErr)
			}
		}
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

