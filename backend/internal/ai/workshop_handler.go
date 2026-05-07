package ai

// Workshop handlers — named persistent AI chat sessions per project (C2).
//
// Routes (registered via RegisterRoutes under /projects/:id/ai/workshop):
//   GET    /projects/:id/ai/workshop          → ListWorkshopSessions
//   POST   /projects/:id/ai/workshop          → CreateWorkshopSession
//   GET    /projects/:id/ai/workshop/:sid     → GetWorkshopSession
//   PUT    /projects/:id/ai/workshop/:sid     → UpdateWorkshopSession
//   DELETE /projects/:id/ai/workshop/:sid     → DeleteWorkshopSession
//   POST   /projects/:id/ai/workshop/:sid/chat → WorkshopChat (SSE)

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jconder44/nexustale/internal/ai/adapters"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// defaultWorkshopSystem is used when no workshop-category project prompt exists.
const defaultWorkshopSystem = "You are Nexus in Workshop mode — a focused craft advisor and story analyst. " +
	"Help the writer examine narrative structure, character arcs, plot consistency, theme, pacing, " +
	"and voice. Be specific and constructive. Reference the project's actual content when relevant. " +
	"Ask clarifying questions when the problem isn't clear. Avoid vague encouragement; offer actionable insight."

// workshopSystemForPhase returns a phase-specific craft focus directive to
// prepend to the workshop system prompt. Returns empty string for 'drafting'
// (no change to default behavior).
func workshopSystemForPhase(phase string) string {
	switch phase {
	case "story_pass":
		return "You are a developmental editor focused on structural integrity. For any scene or chapter discussed: " +
			"(1) flag scenes that don't advance character, plot, or world; " +
			"(2) identify promises made to the reader that haven't been paid off; " +
			"(3) call out pacing issues — scenes that rush through moments that need weight, or linger after they've landed. " +
			"Be specific. Reference open story threads and the project's story structure when relevant."
	case "character_pass":
		return "You are a character editor. For any scene discussed: does each character's action flow from their stated motivation? " +
			"Is their voice distinct from others? Are they behaving consistently with their arc position — early, mid, or late in their journey? " +
			"Flag moments where a character acts for the plot's convenience rather than their own authentic logic."
	case "language_pass":
		return "You are a line editor. For any prose shown, identify: passive constructions that could be active; " +
			"filter words ('she saw', 'he felt', 'she noticed') that create distance; weak verbs that could be specific; " +
			"adverbs masking a stronger verb; repeated sentence structure in close proximity; " +
			"and places where a concrete sensory detail would land harder than an abstraction. Suggest specific rewrites."
	case "editorial_pass":
		return "You are a structural editor giving big-picture notes. Does each chapter open with something that earns attention? " +
			"Does it end in a way that makes the next chapter feel necessary? Are there POV inconsistencies? " +
			"Does each act do its work — setup, escalation, payoff? Be direct and organized."
	default:
		return ""
	}
}

// ── wire types ────────────────────────────────────────────────────────────────

// workshopSessionResponse is the wire format for a workshop session.
// Messages are sent as raw JSON so the frontend can parse them without re-serializing.
type workshopSessionResponse struct {
	ID        string          `json:"id"`
	ProjectID string          `json:"project_id"`
	Title     string          `json:"title"`
	Messages  json.RawMessage `json:"messages"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

func sessionFromRow(row sqlcgen.WorkshopSession) workshopSessionResponse {
	msgs := row.Messages
	if msgs == nil {
		msgs = json.RawMessage("[]")
	}
	return workshopSessionResponse{
		ID:        row.ID.String(),
		ProjectID: row.ProjectID.String(),
		Title:     row.Title,
		Messages:  msgs,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}
}

// ── handlers ──────────────────────────────────────────────────────────────────

// ListWorkshopSessions returns all sessions for the project+user, newest first.
//
// GET /projects/:id/ai/workshop
func (h *Handler) ListWorkshopSessions(c *gin.Context) {
	projectID, userID, ok := resolveIDs(c)
	if !ok {
		return
	}

	rows, err := h.svc.queries.ListWorkshopSessions(c.Request.Context(), sqlcgen.ListWorkshopSessionsParams{
		ProjectID: projectID,
		UserID:    userID,
	})
	if err != nil {
		handleError(c, err)
		return
	}

	resp := make([]workshopSessionResponse, 0, len(rows))
	for _, r := range rows {
		resp = append(resp, sessionFromRow(r))
	}
	c.JSON(http.StatusOK, resp)
}

// CreateWorkshopSession creates a new named workshop session.
//
// POST /projects/:id/ai/workshop
//
//	{ "title": "Act II brainstorm" }
func (h *Handler) CreateWorkshopSession(c *gin.Context) {
	projectID, userID, ok := resolveIDs(c)
	if !ok {
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	title := req.Title
	if title == "" {
		title = "New Session"
	}

	row, err := h.svc.queries.CreateWorkshopSession(c.Request.Context(), sqlcgen.CreateWorkshopSessionParams{
		ProjectID: projectID,
		UserID:    userID,
		Title:     title,
	})
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, sessionFromRow(row))
}

// GetWorkshopSession returns a single workshop session including its messages.
//
// GET /projects/:id/ai/workshop/:sid
func (h *Handler) GetWorkshopSession(c *gin.Context) {
	projectID, userID, ok := resolveIDs(c)
	if !ok {
		return
	}

	sid, err := uuid.Parse(c.Param("sid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid session id"})
		return
	}

	row, err := h.svc.queries.GetWorkshopSession(c.Request.Context(), sqlcgen.GetWorkshopSessionParams{
		ID:        sid,
		ProjectID: projectID,
		UserID:    userID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "session not found"})
			return
		}
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, sessionFromRow(row))
}

// UpdateWorkshopSession persists a title change and/or updated message history.
//
// PUT /projects/:id/ai/workshop/:sid
//
//	{ "title": "Renamed session", "messages": [...] }
func (h *Handler) UpdateWorkshopSession(c *gin.Context) {
	projectID, userID, ok := resolveIDs(c)
	if !ok {
		return
	}

	sid, err := uuid.Parse(c.Param("sid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid session id"})
		return
	}

	// Load current session so we can keep unchanged fields.
	current, err := h.svc.queries.GetWorkshopSession(c.Request.Context(), sqlcgen.GetWorkshopSessionParams{
		ID:        sid,
		ProjectID: projectID,
		UserID:    userID,
	})
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "session not found"})
			return
		}
		handleError(c, err)
		return
	}

	var req struct {
		Title    *string         `json:"title"`
		Messages json.RawMessage `json:"messages"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}

	title := current.Title
	if req.Title != nil {
		title = *req.Title
	}
	messages := current.Messages
	if req.Messages != nil {
		messages = req.Messages
	}

	row, err := h.svc.queries.UpdateWorkshopSession(c.Request.Context(), sqlcgen.UpdateWorkshopSessionParams{
		ID:        sid,
		ProjectID: projectID,
		UserID:    userID,
		Title:     title,
		Messages:  messages,
	})
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, sessionFromRow(row))
}

// DeleteWorkshopSession removes a workshop session and its message history.
//
// DELETE /projects/:id/ai/workshop/:sid
func (h *Handler) DeleteWorkshopSession(c *gin.Context) {
	projectID, userID, ok := resolveIDs(c)
	if !ok {
		return
	}

	sid, err := uuid.Parse(c.Param("sid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid session id"})
		return
	}

	if err := h.svc.queries.DeleteWorkshopSession(c.Request.Context(), sqlcgen.DeleteWorkshopSessionParams{
		ID:        sid,
		ProjectID: projectID,
		UserID:    userID,
	}); err != nil {
		handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// WorkshopChat streams a workshop-mode AI response as SSE.
// The session is validated for ownership; messages are passed by the caller
// (same pattern as the regular /ai/chat endpoint). After streaming completes
// the frontend persists the updated message list via PUT.
//
// POST /projects/:id/ai/workshop/:sid/chat
//
//	{ "messages": [{role, content}, ...], "scene_id": "uuid", "provider": "" }
func (h *Handler) WorkshopChat(c *gin.Context) {
	projectID, userID, ok := resolveIDs(c)
	if !ok {
		return
	}

	sid, err := uuid.Parse(c.Param("sid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid session id"})
		return
	}

	// Validate session ownership.
	if _, err := h.svc.queries.GetWorkshopSession(c.Request.Context(), sqlcgen.GetWorkshopSessionParams{
		ID:        sid,
		ProjectID: projectID,
		UserID:    userID,
	}); err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"message": "session not found"})
			return
		}
		handleError(c, err)
		return
	}

	var req struct {
		Messages     []adapters.Message `json:"messages"`
		SceneID      string             `json:"scene_id"`
		Provider     string             `json:"provider"`
		MaxTokens    int                `json:"max_tokens"`
		ToolsEnabled bool               `json:"tools_enabled"` // C2.5: let AI write directly to manuscript
		MaxRounds    int                `json:"max_rounds"`    // 0 → service default (25)
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request", "detail": err.Error()})
		return
	}
	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "messages is required"})
		return
	}

	var sceneID uuid.UUID
	if req.SceneID != "" {
		if id, err := uuid.Parse(req.SceneID); err == nil {
			sceneID = id
		}
	}

	branch := h.svc.ResolveBranch(c.Request.Context(), c.GetHeader("X-NexusTale-Branch"), userID, projectID)

	// Determine system prompt: use workshop-category preset if configured.
	systemPrompt := h.workshopSystemPrompt(c, projectID)

	svcReq := ChatRequest{
		ProjectID:            projectID,
		SceneID:              sceneID,
		BranchName:           branch,
		Messages:             req.Messages,
		Provider:             req.Provider,
		MaxTokens:            req.MaxTokens,
		SystemPromptOverride: systemPrompt,
		WorkshopMode:         true,
	}

	setSSeHeaders(c)

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		var streamErr error
		if req.ToolsEnabled {
			// Agentic mode: AI may call manuscript tools before replying.
			_, streamErr = h.svc.StreamChatWithTools(c.Request.Context(), userID, svcReq, pw, req.MaxRounds)
		} else {
			_, streamErr = h.svc.StreamChat(c.Request.Context(), userID, svcReq, pw)
		}
		if streamErr != nil {
			slog.Error("ai: workshop StreamChat error", "error", streamErr)
			fmt.Fprintf(pw, "data: {\"error\":%q}\n\n", streamErr.Error())
			fmt.Fprintf(pw, "data: [DONE]\n\n")
		}
	}()

	c.Stream(func(w io.Writer) bool {
		buf := make([]byte, 4096)
		n, err := pr.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
		}
		return err == nil
	})
}

// workshopSystemPrompt returns the system prompt for workshop chat.
// If the project has a workshop-category prompt with non-empty system_content,
// that overrides the default workshop persona.
// A non-drafting project phase prepends a phase-specific focus directive.
func (h *Handler) workshopSystemPrompt(c *gin.Context, projectID uuid.UUID) string {
	base := defaultWorkshopSystem
	p, err := h.svc.queries.GetWorkshopPrompt(c.Request.Context(), projectID)
	if err == nil && p.SystemContent != "" {
		base = p.SystemContent
	}

	phase, err := h.svc.queries.GetProjectPhase(c.Request.Context(), projectID)
	if err == nil {
		if directive := workshopSystemForPhase(phase); directive != "" {
			return directive + "\n\n" + base
		}
	}
	return base
}
