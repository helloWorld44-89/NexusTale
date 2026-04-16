package ai

// tools.go — Manuscript tool definitions and executor for C2.5 Step 2.
//
// ManuscriptTools is the full set of tools exposed to the AI in "agent mode".
// executeToolCall dispatches a single ToolCall to the right DB operation and
// returns a human-readable result string (or an error ToolResult).

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jconder44/nexustale/internal/ai/adapters"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// ManuscriptTools defines the 5 tools Nexus may call when tools_enabled=true.
// Input schemas follow JSON Schema draft 7 (type:"object").
var ManuscriptTools = []adapters.ToolDefinition{
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
}

// executeToolCall dispatches a single tool invocation and returns a ToolResult.
// All errors are returned as IsError=true results rather than propagated —
// the AI can read the error and decide how to proceed.
func (s *Service) executeToolCall(ctx context.Context, projectID uuid.UUID, tc adapters.ToolCall) adapters.ToolResult {
	content, err := s.runTool(ctx, projectID, tc)
	if err != nil {
		return adapters.ToolResult{ID: tc.ID, Content: "Error: " + err.Error(), IsError: true}
	}
	return adapters.ToolResult{ID: tc.ID, Content: content}
}

func (s *Service) runTool(ctx context.Context, projectID uuid.UUID, tc adapters.ToolCall) (string, error) {
	switch tc.Name {
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
	default:
		return "", fmt.Errorf("unknown tool: %q", tc.Name)
	}
}

// ── tool implementations ───────────────────────────────────────────────────────

func (s *Service) toolAppendToScene(ctx context.Context, input json.RawMessage) (string, error) {
	var args struct {
		SceneID string `json:"scene_id"`
		Text    string `json:"text"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	sceneID, err := uuid.Parse(args.SceneID)
	if err != nil {
		return "", fmt.Errorf("invalid scene_id: %w", err)
	}
	scene, err := s.queries.GetScene(ctx, sceneID)
	if err != nil {
		return "", fmt.Errorf("scene not found: %w", err)
	}

	newContent := scene.Content
	if newContent != "" {
		if !strings.HasSuffix(newContent, "\n") {
			newContent += "\n"
		}
		newContent += "\n"
	}
	newContent += args.Text

	if _, err := s.queries.UpdateScene(ctx, sqlcgen.UpdateSceneParams{
		ID:      sceneID,
		Content: pgtype.Text{String: newContent, Valid: true},
	}); err != nil {
		return "", fmt.Errorf("update scene: %w", err)
	}
	return fmt.Sprintf("Appended %d characters to scene %q (ID: %s).", len(args.Text), scene.Title, scene.ID), nil
}

func (s *Service) toolReplaceSceneContent(ctx context.Context, input json.RawMessage) (string, error) {
	var args struct {
		SceneID string `json:"scene_id"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	sceneID, err := uuid.Parse(args.SceneID)
	if err != nil {
		return "", fmt.Errorf("invalid scene_id: %w", err)
	}
	scene, err := s.queries.GetScene(ctx, sceneID)
	if err != nil {
		return "", fmt.Errorf("scene not found: %w", err)
	}
	if _, err := s.queries.UpdateScene(ctx, sqlcgen.UpdateSceneParams{
		ID:      sceneID,
		Content: pgtype.Text{String: args.Content, Valid: true},
	}); err != nil {
		return "", fmt.Errorf("update scene: %w", err)
	}
	return fmt.Sprintf("Replaced content of scene %q (ID: %s) with %d characters.", scene.Title, scene.ID, len(args.Content)), nil
}

func (s *Service) toolCreateScene(ctx context.Context, input json.RawMessage) (string, error) {
	var args struct {
		ChapterID string `json:"chapter_id"`
		Title     string `json:"title"`
		Content   string `json:"content"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	chapterID, err := uuid.Parse(args.ChapterID)
	if err != nil {
		return "", fmt.Errorf("invalid chapter_id: %w", err)
	}
	existing, _ := s.queries.ListScenesByChapter(ctx, chapterID)
	scene, err := s.queries.CreateScene(ctx, sqlcgen.CreateSceneParams{
		ChapterID: chapterID,
		Title:     args.Title,
		Content:   args.Content,
		SortOrder: int32(len(existing) + 1),
	})
	if err != nil {
		return "", fmt.Errorf("create scene: %w", err)
	}
	return fmt.Sprintf("Created scene %q (ID: %s) in chapter %s.", scene.Title, scene.ID, chapterID), nil
}

func (s *Service) toolCreateChapter(ctx context.Context, projectID uuid.UUID, input json.RawMessage) (string, error) {
	var args struct {
		ActID string `json:"act_id"`
		Title string `json:"title"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	actID, err := uuid.Parse(args.ActID)
	if err != nil {
		return "", fmt.Errorf("invalid act_id: %w", err)
	}
	existing, _ := s.queries.ListChaptersByAct(ctx, actID)
	chapter, err := s.queries.CreateChapter(ctx, sqlcgen.CreateChapterParams{
		ProjectID: projectID,
		ActID:     actID,
		Title:     args.Title,
		SortOrder: int32(len(existing) + 1),
	})
	if err != nil {
		return "", fmt.Errorf("create chapter: %w", err)
	}
	return fmt.Sprintf("Created chapter %q (ID: %s) in act %s.", chapter.Title, chapter.ID, actID), nil
}

func (s *Service) toolCreateAct(ctx context.Context, projectID uuid.UUID, input json.RawMessage) (string, error) {
	var args struct {
		Title string `json:"title"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	existing, _ := s.queries.ListActsByProject(ctx, projectID)
	act, err := s.queries.CreateAct(ctx, sqlcgen.CreateActParams{
		ProjectID: projectID,
		Title:     args.Title,
		SortOrder: int32(len(existing) + 1),
	})
	if err != nil {
		return "", fmt.Errorf("create act: %w", err)
	}
	return fmt.Sprintf("Created act %q (ID: %s) in project %s.", act.Title, act.ID, projectID), nil
}
