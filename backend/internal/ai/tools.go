package ai

// tools.go — Manuscript tool definitions and executor for C2.5 Step 2.
//
// ManuscriptTools is the full set of tools exposed to the AI in "agent mode".
// executeToolCall dispatches a single ToolCall to the right DB operation and
// returns both an adapters.ToolResult (for the model) and a ToolEvent (for the
// frontend SSE stream, which includes undo metadata).

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jconder44/nexustale/internal/ai/adapters"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// ManuscriptTools defines the tools Nexus may call when tools_enabled=true.
// Input schemas follow JSON Schema draft 7 (type:"object").
var ManuscriptTools = []adapters.ToolDefinition{
	{
		Name: "list_project_structure",
		Description: "Read the current act → chapter → scene tree for this project, including IDs. " +
			"Call this FIRST before any write operation so you know which IDs already exist and can " +
			"target them precisely. Returns act_ids, chapter_ids, and scene_ids with titles and word counts.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {}
		}`),
	},
	{
		Name:        "append_to_scene",
		Description: "Append text to the end of an existing scene's content. Use this to add new paragraphs, dialogue, or description to a scene that already exists.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"scene_id": {"type": "string", "description": "UUID of the scene to append to"},
				"text":     {"type": "string", "description": "Text to append. Will be separated from existing content by a blank line."}
			},
			"required": ["scene_id", "text"]
		}`),
	},
	{
		Name:        "replace_scene_content",
		Description: "Replace the entire content of a scene. Use this when rewriting a scene from scratch rather than extending it.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"scene_id": {"type": "string", "description": "UUID of the scene to replace"},
				"content":  {"type": "string", "description": "The new complete content for the scene"}
			},
			"required": ["scene_id", "content"]
		}`),
	},
	{
		Name:        "create_scene",
		Description: "Create a new scene inside an existing chapter. The scene is appended after any existing scenes in that chapter.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"chapter_id": {"type": "string", "description": "UUID of the chapter to add the scene to"},
				"title":      {"type": "string", "description": "Title of the new scene"},
				"content":    {"type": "string", "description": "Initial prose content for the scene (can be empty string to create a blank scene)"}
			},
			"required": ["chapter_id", "title"]
		}`),
	},
	{
		Name:        "create_chapter",
		Description: "Create a new chapter inside an existing act. The chapter is appended after any existing chapters in that act.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"act_id": {"type": "string", "description": "UUID of the act to add the chapter to"},
				"title":  {"type": "string", "description": "Title of the new chapter"}
			},
			"required": ["act_id", "title"]
		}`),
	},
	{
		Name:        "create_act",
		Description: "Create a new act in the project. The act is appended after any existing acts.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"title": {"type": "string", "description": "Title of the new act (e.g. 'Act I', 'Act Two', 'Prologue')"}
			},
			"required": ["title"]
		}`),
	},
	{
		Name: "list_wiki_entities",
		Description: "List all wiki entities in the project (characters, locations, factions, items, concepts, lore). " +
			"Returns each entity's ID, type, name, and summary. Call this before create_wiki_relationship so you " +
			"have the correct entity IDs.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"type": {"type": "string", "description": "Optional filter: character, location, faction, item, concept, or lore. Omit for all."}
			}
		}`),
	},
	{
		Name: "create_wiki_entity",
		Description: "Create a new wiki entry for a character, location, faction, item, concept, or lore element. " +
			"Use this when the user asks you to add something to the wiki, or when you create a character/place in " +
			"prose and want to record it in the project's world reference.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"type":    {"type": "string", "enum": ["character","location","faction","item","concept","lore"], "description": "Entity type"},
				"name":    {"type": "string", "description": "Name of the entity"},
				"summary": {"type": "string", "description": "One or two sentence description of this entity"}
			},
			"required": ["type", "name"]
		}`),
	},
	{
		Name: "update_wiki_entity",
		Description: "Update the name or summary of an existing wiki entity. " +
			"Call list_wiki_entities first to find the correct entity_id.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"entity_id": {"type": "string", "description": "UUID of the entity to update"},
				"name":      {"type": "string", "description": "New name (omit to keep existing)"},
				"summary":   {"type": "string", "description": "New summary (omit to keep existing)"}
			},
			"required": ["entity_id"]
		}`),
	},
	{
		Name: "create_wiki_relationship",
		Description: "Record a relationship between two wiki entities (e.g. 'mentor of', 'allied with', 'sworn enemy of'). " +
			"Call list_wiki_entities first to find the correct entity IDs.",
		InputSchema: json.RawMessage(`{
			"type": "object",
			"properties": {
				"from_entity_id": {"type": "string", "description": "UUID of the source entity"},
				"to_entity_id":   {"type": "string", "description": "UUID of the target entity"},
				"type":           {"type": "string", "description": "Relationship label, e.g. 'mentor of', 'enemy of', 'located in'"},
				"description":    {"type": "string", "description": "Optional details about the relationship"}
			},
			"required": ["from_entity_id", "to_entity_id", "type"]
		}`),
	},
}

// manuscriptWriteTools is the subset of ManuscriptTools for pure manuscript-write
// tasks (no wiki work). Saves ~400–500 tokens per round when wiki is not needed.
var manuscriptWriteTools = ManuscriptTools[:6] // list_project_structure + 5 manuscript tools

// wikiTools is the subset for pure wiki tasks (no prose write).
var wikiTools = append([]adapters.ToolDefinition{ManuscriptTools[0]}, ManuscriptTools[6:]...) // list_project_structure + 4 wiki tools

// selectToolsForIntent returns the smallest set of tools appropriate for the
// final user message in the conversation. When intent is ambiguous both sets
// are included (i.e. the full ManuscriptTools slice).
func selectToolsForIntent(messages []adapters.Message) []adapters.ToolDefinition {
	// Find the last user message.
	lastUser := ""
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "user" {
			lastUser = strings.ToLower(messages[i].Content)
			break
		}
	}

	wikiKeywords := []string{
		"wiki", "entity", "entities", "character", "location", "faction",
		"relationship", "character sheet", "add to wiki", "update wiki",
	}
	writeKeywords := []string{
		"write", "append", "add prose", "scene", "continue writing",
		"draft", "create chapter", "create act", "create scene", "expand",
	}

	hasWiki := containsAny(lastUser, wikiKeywords)
	hasWrite := containsAny(lastUser, writeKeywords)

	switch {
	case hasWiki && !hasWrite:
		return wikiTools
	case hasWrite && !hasWiki:
		return manuscriptWriteTools
	default:
		return ManuscriptTools
	}
}

func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// ToolEvent carries the result and undo metadata for a single tool execution.
// Emitted as an SSE event so the frontend can display progress and offer Undo.
type ToolEvent struct {
	Tool    string `json:"tool"`
	Result  string `json:"result"`
	IsError bool   `json:"is_error,omitempty"`

	// Scene write ops (append_to_scene, replace_scene_content).
	// BeforeContent lets the frontend restore the previous state without a
	// round-trip.  ChapterID is needed for the PATCH endpoint.
	SceneID       string `json:"scene_id,omitempty"`
	ChapterID     string `json:"chapter_id,omitempty"`
	BeforeContent string `json:"before_content,omitempty"`

	// Create ops (create_scene, create_chapter, create_act).
	// CreatedID + CreatedType identify what was made; ParentID and ProjectID
	// are routing context so the frontend can call the right DELETE endpoint.
	CreatedID   string `json:"created_id,omitempty"`
	CreatedType string `json:"created_type,omitempty"` // "scene"|"chapter"|"act"|"wiki_entity"|"wiki_relationship"
	ActID       string `json:"act_id,omitempty"`        // for chapter delete: /projects/:pid/acts/:aid/chapters/:cid
	ProjectID   string `json:"project_id,omitempty"`    // for act/chapter/wiki delete
}

// executeToolCall dispatches a single tool invocation and returns both the
// model-facing ToolResult and the frontend-facing ToolEvent.
// writeSceneFileIfPossible resolves the git repo path for a scene's project and
// writes the content file. Non-fatal — logs on failure. Called after every
// direct DB write in tool functions so the working tree stays current.
func (s *Service) writeSceneFileIfPossible(ctx context.Context, chapterID, sceneID uuid.UUID, content string) {
	if s.sceneWriter == nil {
		return
	}
	ch, err := s.queries.GetChapter(ctx, chapterID)
	if err != nil {
		slog.Warn("git dual-write: chapter lookup failed", "chapter_id", chapterID, "error", err)
		return
	}
	proj, err := s.queries.GetProject(ctx, ch.ProjectID)
	if err != nil {
		slog.Warn("git dual-write: project lookup failed", "project_id", ch.ProjectID, "error", err)
		return
	}
	if wErr := s.sceneWriter.WriteSceneFile(proj.GitRepoPath, chapterID, sceneID, content); wErr != nil {
		slog.Warn("git dual-write: write failed", "scene_id", sceneID, "error", wErr)
	}
}

func (s *Service) executeToolCall(ctx context.Context, projectID uuid.UUID, tc adapters.ToolCall) (adapters.ToolResult, ToolEvent) {
	evt, err := s.runTool(ctx, projectID, tc)
	evt.Tool = tc.Name
	if err != nil {
		evt.Result = "Error: " + err.Error()
		evt.IsError = true
	}
	return adapters.ToolResult{ID: tc.ID, Content: evt.Result, IsError: evt.IsError}, evt
}

func (s *Service) runTool(ctx context.Context, projectID uuid.UUID, tc adapters.ToolCall) (ToolEvent, error) {
	switch tc.Name {
	case "list_project_structure":
		return s.toolListProjectStructure(ctx, projectID)
	case "append_to_scene":
		return s.toolAppendToScene(ctx, tc.Input)
	case "replace_scene_content":
		return s.toolReplaceSceneContent(ctx, tc.Input)
	case "create_scene":
		return s.toolCreateScene(ctx, tc.Input)
	case "create_chapter":
		return s.toolCreateChapter(ctx, projectID, tc.Input)
	case "create_act":
		return s.toolCreateAct(ctx, projectID, tc.Input)
	case "list_wiki_entities":
		return s.toolListWikiEntities(ctx, projectID, tc.Input)
	case "create_wiki_entity":
		return s.toolCreateWikiEntity(ctx, projectID, tc.Input)
	case "update_wiki_entity":
		return s.toolUpdateWikiEntity(ctx, tc.Input)
	case "create_wiki_relationship":
		return s.toolCreateWikiRelationship(ctx, projectID, tc.Input)
	default:
		return ToolEvent{}, fmt.Errorf("unknown tool: %q", tc.Name)
	}
}

// ── tool implementations ───────────────────────────────────────────────────────

func (s *Service) toolAppendToScene(ctx context.Context, input json.RawMessage) (ToolEvent, error) {
	var args struct {
		SceneID string `json:"scene_id"`
		Text    string `json:"text"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolEvent{}, fmt.Errorf("invalid input: %w", err)
	}
	sceneID, err := uuid.Parse(args.SceneID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("invalid scene_id: %w", err)
	}
	scene, err := s.queries.GetScene(ctx, sceneID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("scene not found: %w", err)
	}

	beforeContent := s.readSceneContent(ctx, scene.ChapterID, scene.ID)

	newContent := beforeContent
	if newContent != "" {
		if !strings.HasSuffix(newContent, "\n") {
			newContent += "\n"
		}
		newContent += "\n"
	}
	newContent += args.Text

	if _, err := s.queries.UpdateScene(ctx, sqlcgen.UpdateSceneParams{
		ID:        sceneID,
		WordCount: pgtype.Int4{Int32: int32(len(strings.Fields(newContent))), Valid: true},
	}); err != nil {
		return ToolEvent{}, fmt.Errorf("update scene: %w", err)
	}
	s.writeSceneFileIfPossible(ctx, scene.ChapterID, scene.ID, newContent)
	return ToolEvent{
		Result:        fmt.Sprintf("Appended %d characters to scene %q (ID: %s).", len(args.Text), scene.Title, scene.ID),
		SceneID:       scene.ID.String(),
		ChapterID:     scene.ChapterID.String(),
		BeforeContent: beforeContent,
	}, nil
}

func (s *Service) toolReplaceSceneContent(ctx context.Context, input json.RawMessage) (ToolEvent, error) {
	var args struct {
		SceneID string `json:"scene_id"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolEvent{}, fmt.Errorf("invalid input: %w", err)
	}
	sceneID, err := uuid.Parse(args.SceneID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("invalid scene_id: %w", err)
	}
	scene, err := s.queries.GetScene(ctx, sceneID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("scene not found: %w", err)
	}

	beforeContent := s.readSceneContent(ctx, scene.ChapterID, scene.ID)

	if _, err := s.queries.UpdateScene(ctx, sqlcgen.UpdateSceneParams{
		ID:        sceneID,
		WordCount: pgtype.Int4{Int32: int32(len(strings.Fields(args.Content))), Valid: true},
	}); err != nil {
		return ToolEvent{}, fmt.Errorf("update scene: %w", err)
	}
	s.writeSceneFileIfPossible(ctx, scene.ChapterID, scene.ID, args.Content)
	return ToolEvent{
		Result:        fmt.Sprintf("Replaced content of scene %q (ID: %s) with %d characters.", scene.Title, scene.ID, len(args.Content)),
		SceneID:       scene.ID.String(),
		ChapterID:     scene.ChapterID.String(),
		BeforeContent: beforeContent,
	}, nil
}

func (s *Service) toolCreateScene(ctx context.Context, input json.RawMessage) (ToolEvent, error) {
	var args struct {
		ChapterID string `json:"chapter_id"`
		Title     string `json:"title"`
		Content   string `json:"content"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolEvent{}, fmt.Errorf("invalid input: %w", err)
	}
	chapterID, err := uuid.Parse(args.ChapterID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("invalid chapter_id: %w", err)
	}
	existing, _ := s.queries.ListScenesByChapter(ctx, chapterID)
	scene, err := s.queries.CreateScene(ctx, sqlcgen.CreateSceneParams{
		ChapterID: chapterID,
		Title:     args.Title,
		SortOrder: int32(len(existing) + 1),
		WordCount: int32(len(strings.Fields(args.Content))),
	})
	if err != nil {
		return ToolEvent{}, fmt.Errorf("create scene: %w", err)
	}
	s.writeSceneFileIfPossible(ctx, scene.ChapterID, scene.ID, args.Content)
	return ToolEvent{
		Result:      fmt.Sprintf("Created scene %q (ID: %s) in chapter %s.", scene.Title, scene.ID, chapterID),
		CreatedID:   scene.ID.String(),
		CreatedType: "scene",
		ChapterID:   chapterID.String(),
	}, nil
}

func (s *Service) toolCreateChapter(ctx context.Context, projectID uuid.UUID, input json.RawMessage) (ToolEvent, error) {
	var args struct {
		ActID string `json:"act_id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolEvent{}, fmt.Errorf("invalid input: %w", err)
	}
	actID, err := uuid.Parse(args.ActID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("invalid act_id: %w", err)
	}
	existing, _ := s.queries.ListChaptersByAct(ctx, actID)
	chapter, err := s.queries.CreateChapter(ctx, sqlcgen.CreateChapterParams{
		ProjectID: projectID,
		ActID:     actID,
		Title:     args.Title,
		SortOrder: int32(len(existing) + 1),
	})
	if err != nil {
		return ToolEvent{}, fmt.Errorf("create chapter: %w", err)
	}
	return ToolEvent{
		Result:      fmt.Sprintf("Created chapter %q (ID: %s) in act %s.", chapter.Title, chapter.ID, actID),
		CreatedID:   chapter.ID.String(),
		CreatedType: "chapter",
		ActID:       actID.String(),
		ProjectID:   projectID.String(),
	}, nil
}

// toolListProjectStructure reads the live act→chapter→scene tree and returns a
// formatted text block the model can use to find IDs before writing.
func (s *Service) toolListProjectStructure(ctx context.Context, projectID uuid.UUID) (ToolEvent, error) {
	acts, err := s.queries.ListActsByProject(ctx, projectID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("list acts: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("Project structure (use these IDs when targeting existing content):\n\n")

	if len(acts) == 0 {
		sb.WriteString("No acts yet. Use create_act to add the first act.\n")
		return ToolEvent{Result: sb.String()}, nil
	}

	for _, act := range acts {
		fmt.Fprintf(&sb, "Act %d: %q  (act_id: %s)\n", act.SortOrder, act.Title, act.ID)

		chapters, err := s.queries.ListChaptersByAct(ctx, act.ID)
		if err != nil {
			fmt.Fprintf(&sb, "  [error loading chapters: %v]\n", err)
			continue
		}
		if len(chapters) == 0 {
			sb.WriteString("  (no chapters)\n")
			continue
		}

		for _, ch := range chapters {
			fmt.Fprintf(&sb, "  Chapter %d: %q  (chapter_id: %s)\n", ch.SortOrder, ch.Title, ch.ID)

			scenes, err := s.queries.ListScenesByChapter(ctx, ch.ID)
			if err != nil {
				fmt.Fprintf(&sb, "    [error loading scenes: %v]\n", err)
				continue
			}
			if len(scenes) == 0 {
				sb.WriteString("    (no scenes)\n")
				continue
			}

			for _, sc := range scenes {
				if sc.WordCount > 0 {
					fmt.Fprintf(&sb, "    Scene %d: %q  (scene_id: %s, %d words)\n",
						sc.SortOrder, sc.Title, sc.ID, sc.WordCount)
				} else {
					fmt.Fprintf(&sb, "    Scene %d: %q  (scene_id: %s, empty)\n",
						sc.SortOrder, sc.Title, sc.ID)
				}
			}
		}
	}

	return ToolEvent{Result: sb.String()}, nil
}

func (s *Service) toolCreateAct(ctx context.Context, projectID uuid.UUID, input json.RawMessage) (ToolEvent, error) {
	var args struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolEvent{}, fmt.Errorf("invalid input: %w", err)
	}
	existing, _ := s.queries.ListActsByProject(ctx, projectID)
	act, err := s.queries.CreateAct(ctx, sqlcgen.CreateActParams{
		ProjectID: projectID,
		Title:     args.Title,
		SortOrder: int32(len(existing) + 1),
	})
	if err != nil {
		return ToolEvent{}, fmt.Errorf("create act: %w", err)
	}
	return ToolEvent{
		Result:      fmt.Sprintf("Created act %q (ID: %s) in project %s.", act.Title, act.ID, projectID),
		CreatedID:   act.ID.String(),
		CreatedType: "act",
		ProjectID:   projectID.String(),
	}, nil
}

// ── wiki tools ────────────────────────────────────────────────────────────────

func (s *Service) toolListWikiEntities(ctx context.Context, projectID uuid.UUID, input json.RawMessage) (ToolEvent, error) {
	var args struct {
		Type string `json:"type"`
	}
	json.Unmarshal(input, &args) // optional — ignore parse errors
	entities, err := s.queries.ListEntitiesByProject(ctx, sqlcgen.ListEntitiesByProjectParams{
		ProjectID:  projectID,
		Type: pgtype.Text{String: args.Type, Valid: args.Type != ""},
	})
	if err != nil {
		return ToolEvent{}, fmt.Errorf("list entities: %w", err)
	}
	if len(entities) == 0 {
		return ToolEvent{Result: "No wiki entities found for this project yet."}, nil
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Wiki entities (%d):\n\n", len(entities)))
	for _, e := range entities {
		summary := e.Summary
		if len(summary) > 120 {
			summary = summary[:120] + "…"
		}
		if summary != "" {
			fmt.Fprintf(&sb, "[%s] %s (ID: %s) — %s\n", e.Type, e.Name, e.ID, summary)
		} else {
			fmt.Fprintf(&sb, "[%s] %s (ID: %s)\n", e.Type, e.Name, e.ID)
		}
	}
	return ToolEvent{Result: sb.String()}, nil
}

func (s *Service) toolCreateWikiEntity(ctx context.Context, projectID uuid.UUID, input json.RawMessage) (ToolEvent, error) {
	var args struct {
		Type    string `json:"type"`
		Name    string `json:"name"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolEvent{}, fmt.Errorf("invalid input: %w", err)
	}
	validTypes := map[string]bool{
		"character": true, "location": true, "faction": true,
		"item": true, "concept": true, "lore": true,
	}
	if !validTypes[args.Type] {
		return ToolEvent{}, fmt.Errorf("invalid entity type %q — must be one of: character, location, faction, item, concept, lore", args.Type)
	}
	if args.Name == "" {
		return ToolEvent{}, fmt.Errorf("name is required")
	}
	entity, err := s.queries.CreateEntity(ctx, sqlcgen.CreateEntityParams{
		ProjectID:  projectID,
		Type:       args.Type,
		Name:       args.Name,
		Summary:    args.Summary,
		Attributes: json.RawMessage(`{}`),
	})
	if err != nil {
		return ToolEvent{}, fmt.Errorf("create entity: %w", err)
	}
	return ToolEvent{
		Result:      fmt.Sprintf("Created %s %q in the wiki (ID: %s).", args.Type, entity.Name, entity.ID),
		CreatedID:   entity.ID.String(),
		CreatedType: "wiki_entity",
		ProjectID:   projectID.String(),
	}, nil
}

func (s *Service) toolUpdateWikiEntity(ctx context.Context, input json.RawMessage) (ToolEvent, error) {
	var args struct {
		EntityID string  `json:"entity_id"`
		Name     *string `json:"name"`
		Summary  *string `json:"summary"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolEvent{}, fmt.Errorf("invalid input: %w", err)
	}
	entityID, err := uuid.Parse(args.EntityID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("invalid entity_id: %w", err)
	}
	params := sqlcgen.UpdateEntityParams{ID: entityID}
	if args.Name != nil {
		params.Name = pgtype.Text{String: *args.Name, Valid: true}
	}
	if args.Summary != nil {
		params.Summary = pgtype.Text{String: *args.Summary, Valid: true}
	}
	entity, err := s.queries.UpdateEntity(ctx, params)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("update entity: %w", err)
	}
	return ToolEvent{
		Result: fmt.Sprintf("Updated %s %q (ID: %s).", entity.Type, entity.Name, entity.ID),
	}, nil
}

func (s *Service) toolCreateWikiRelationship(ctx context.Context, projectID uuid.UUID, input json.RawMessage) (ToolEvent, error) {
	var args struct {
		FromEntityID string `json:"from_entity_id"`
		ToEntityID   string `json:"to_entity_id"`
		Type         string `json:"type"`
		Description  string `json:"description"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return ToolEvent{}, fmt.Errorf("invalid input: %w", err)
	}
	fromID, err := uuid.Parse(args.FromEntityID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("invalid from_entity_id: %w", err)
	}
	toID, err := uuid.Parse(args.ToEntityID)
	if err != nil {
		return ToolEvent{}, fmt.Errorf("invalid to_entity_id: %w", err)
	}
	rel, err := s.queries.CreateRelationship(ctx, sqlcgen.CreateRelationshipParams{
		ProjectID:    projectID,
		FromEntityID: fromID,
		ToEntityID:   toID,
		Type:         args.Type,
		Description:  args.Description,
	})
	if err != nil {
		return ToolEvent{}, fmt.Errorf("create relationship: %w", err)
	}
	return ToolEvent{
		Result:      fmt.Sprintf("Recorded relationship: %s → [%s] → %s (ID: %s).", fromID, args.Type, toID, rel.ID),
		CreatedID:   rel.ID.String(),
		CreatedType: "wiki_relationship",
		ProjectID:   projectID.String(),
	}, nil
}
