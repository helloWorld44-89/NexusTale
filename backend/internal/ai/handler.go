package ai

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jconder44/nexustale/internal/ai/adapters"
	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/pkg/apperror"
)

// Handler exposes Gin route handlers for AI-assisted writing.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts AI routes under /projects/:id/ai.
// All routes require the auth middleware applied by the caller.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	ai := rg.Group("/ai")
	ai.POST("/complete", h.Complete)
	ai.POST("/chat", h.Chat)
	ai.POST("/summarize", h.Summarize)
	ai.GET("/usage", h.Usage)

	// Chapter summary endpoints (B2): mounted under the project group.
	rg.GET("/chapters/:cid/summary", h.GetChapterSummary)
	rg.POST("/chapters/:cid/summarize", h.RegenerateChapterSummary)
}

// RegisterUserRoutes mounts user-scoped (non-project) AI routes.
// Caller must apply auth middleware.
func (h *Handler) RegisterUserRoutes(rg *gin.RouterGroup) {
	rg.POST("/ai/test-connection", h.TestConnectionHandler)
}

// ── request types ─────────────────────────────────────────────────────────────

type completeRequest struct {
	SceneID     string `json:"scene_id"`
	Mode        string `json:"mode"`        // "continue" | "beat" (default: "continue")
	Beat        string `json:"beat"`        // required when mode=beat
	Instruction string `json:"instruction"` // optional hint for continue
	Provider    string `json:"provider"`    // optional
	MaxTokens   int    `json:"max_tokens"`  // optional
	PromptID    string `json:"prompt_id"`   // optional writing style preset
}

type chatRequest struct {
	SceneID   string             `json:"scene_id"`
	Messages  []adapters.Message `json:"messages"`
	Provider  string             `json:"provider"`
	MaxTokens int                `json:"max_tokens"`
}

type summarizeRequest struct {
	Text     string `json:"text"`      // inline text, or
	SceneID  string `json:"scene_id"`  // resolve from DB if text is empty
	Provider string `json:"provider"`
}

// ── handlers ──────────────────────────────────────────────────────────────────

// Complete streams a scene continuation or beat expansion as SSE.
//
// POST /projects/:id/ai/complete
//
//	{ "scene_id": "uuid", "mode": "beat", "beat": "Jack finds the door open" }
//	{ "scene_id": "uuid", "mode": "continue", "instruction": "darker tone" }
func (h *Handler) Complete(c *gin.Context) {
	projectID, userID, ok := resolveIDs(c)
	if !ok {
		return
	}

	var req completeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request", "detail": err.Error()})
		return
	}

	mode := adapters.CompleteModeContinue
	if req.Mode == string(adapters.CompleteModeBeat) {
		if req.Beat == "" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "beat is required when mode=beat"})
			return
		}
		mode = adapters.CompleteModeBeat
	}

	var sceneID uuid.UUID
	if req.SceneID != "" {
		id, err := uuid.Parse(req.SceneID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid scene_id"})
			return
		}
		sceneID = id
	}

	var promptID uuid.UUID
	if req.PromptID != "" {
		if id, err := uuid.Parse(req.PromptID); err == nil {
			promptID = id
		}
	}

	branch := h.svc.ResolveBranch(c.Request.Context(), c.GetHeader("X-NexusTale-Branch"), userID, projectID)

	svcReq := CompleteRequest{
		ProjectID:   projectID,
		SceneID:     sceneID,
		BranchName:  branch,
		Mode:        mode,
		Beat:        req.Beat,
		Instruction: req.Instruction,
		Provider:    req.Provider,
		MaxTokens:   req.MaxTokens,
		PromptID:    promptID,
	}

	setSSeHeaders(c)

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		if _, err := h.svc.StreamComplete(c.Request.Context(), userID, svcReq, pw); err != nil {
			fmt.Fprintf(pw, "data: {\"error\":%q}\n\n", err.Error())
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

// Chat streams a freeform chat response as SSE.
//
// POST /projects/:id/ai/chat
//
//	{ "messages": [{"role":"user","content":"What if the ending changed?"}], "scene_id": "uuid" }
func (h *Handler) Chat(c *gin.Context) {
	projectID, userID, ok := resolveIDs(c)
	if !ok {
		return
	}

	var req chatRequest
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
		id, err := uuid.Parse(req.SceneID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid scene_id"})
			return
		}
		sceneID = id
	}

	branch := h.svc.ResolveBranch(c.Request.Context(), c.GetHeader("X-NexusTale-Branch"), userID, projectID)

	svcReq := ChatRequest{
		ProjectID:  projectID,
		SceneID:    sceneID,
		BranchName: branch,
		Messages:   req.Messages,
		Provider:   req.Provider,
		MaxTokens:  req.MaxTokens,
	}

	setSSeHeaders(c)

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		if _, err := h.svc.StreamChat(c.Request.Context(), userID, svcReq, pw); err != nil {
			fmt.Fprintf(pw, "data: {\"error\":%q}\n\n", err.Error())
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

// Summarize generates a short summary of the provided text (non-streaming).
//
// POST /projects/:id/ai/summarize
//
//	{ "text": "Full scene content...", "provider": "anthropic" }
//	{ "scene_id": "uuid" }          — fetches scene content from DB
func (h *Handler) Summarize(c *gin.Context) {
	_, userID, ok := resolveIDs(c)
	if !ok {
		return
	}

	var req summarizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request", "detail": err.Error()})
		return
	}

	text := req.Text
	if text == "" && req.SceneID != "" {
		sceneID, err := uuid.Parse(req.SceneID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "invalid scene_id"})
			return
		}
		sc, err := h.svc.queries.GetScene(c.Request.Context(), sceneID)
		if err != nil {
			handleError(c, apperror.NotFound("scene", req.SceneID))
			return
		}
		text = sc.Content
	}

	if text == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "text or scene_id is required"})
		return
	}

	summary, _, err := h.svc.Summarize(c.Request.Context(), userID, req.Provider, text)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"summary": summary})
}

// Usage returns aggregate token/cost stats for the project.
//
// GET /projects/:id/ai/usage
func (h *Handler) Usage(c *gin.Context) {
	projectID, _, ok := resolveIDs(c)
	if !ok {
		return
	}
	summary, err := h.svc.GetUsageSummary(c.Request.Context(), projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, summary)
}

// ── helpers ───────────────────────────────────────────────────────────────────

// resolveIDs extracts projectID from the URL param and userID from JWT claims.
func resolveIDs(c *gin.Context) (projectID, userID uuid.UUID, ok bool) {
	claims := auth.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return uuid.Nil, uuid.Nil, false
	}

	pid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid project id"})
		return uuid.Nil, uuid.Nil, false
	}

	return pid, claims.UserID, true
}

// setSSeHeaders sets the standard headers for Server-Sent Events.
func setSSeHeaders(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no") // disable nginx proxy buffering
}

// TestConnectionHandler tests connectivity for the specified AI provider.
//
// POST /ai/test-connection
//
//	{ "provider": "ollama" }
//	→ { "ok": true, "provider": "ollama", "models": ["llama3:latest", ...] }
//	→ { "ok": false, "provider": "ollama", "error": "cannot reach Ollama …" }
func (h *Handler) TestConnectionHandler(c *gin.Context) {
	claims := auth.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	var req struct {
		Provider string `json:"provider" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "provider is required"})
		return
	}

	result := h.svc.TestConnection(c.Request.Context(), claims.UserID, req.Provider)
	c.JSON(http.StatusOK, result)
}

// handleError maps apperror to HTTP status codes, matching the pattern in other handlers.
func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*apperror.AppError); ok {
		c.JSON(appErr.Code, appErr)
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"message": "internal error"})
}

// writeSSEError writes a terminal error SSE event and is used for deferred cleanup.
func writeSSEError(w io.Writer, msg string) {
	encoded, _ := json.Marshal(map[string]string{"error": msg})
	fmt.Fprintf(w, "data: %s\n\n", encoded)
}

// GetChapterSummary returns the AI-generated summary for a chapter on the
// active Timeline (branch).
//
// GET /projects/:id/chapters/:cid/summary
func (h *Handler) GetChapterSummary(c *gin.Context) {
	projectID, userID, ok := resolveIDs(c)
	if !ok {
		return
	}

	chapterID, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid chapter id"})
		return
	}

	branch := h.svc.ResolveBranch(c.Request.Context(), c.GetHeader("X-NexusTale-Branch"), userID, projectID)

	row, err := h.svc.GetChapterSummary(c.Request.Context(), chapterID, branch)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, row)
}

// RegenerateChapterSummary forces a synchronous re-summarization of all
// scenes in a chapter and stores the result.
//
// POST /projects/:id/chapters/:cid/summarize
func (h *Handler) RegenerateChapterSummary(c *gin.Context) {
	projectID, userID, ok := resolveIDs(c)
	if !ok {
		return
	}

	chapterID, err := uuid.Parse(c.Param("cid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid chapter id"})
		return
	}

	branch := h.svc.ResolveBranch(c.Request.Context(), c.GetHeader("X-NexusTale-Branch"), userID, projectID)

	summary, err := h.svc.RegenerateChapterSummary(c.Request.Context(), userID, chapterID, branch)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"chapter_id":  chapterID,
		"branch_name": branch,
		"ai_summary":  summary,
		"stale":       false,
		"project_id":  projectID,
	})
}
