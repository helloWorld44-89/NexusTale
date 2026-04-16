package research

// Handler exposes REST routes for research notes.
//
// Routes (registered under /projects/:id):
//   GET    /research-notes        → list notes
//   POST   /research-notes        → create note
//   GET    /research-notes/:nid   → get note
//   PATCH  /research-notes/:nid   → update note
//   DELETE /research-notes/:nid   → delete note

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/pkg/apperror"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts all research-note routes under the provided group.
// Caller must supply a group rooted at /projects/:id.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	notes := rg.Group("/research-notes")
	notes.GET("", h.List)
	notes.POST("", h.Create)
	notes.GET("/:nid", h.Get)
	notes.PATCH("/:nid", h.Update)
	notes.DELETE("/:nid", h.Delete)
}

// ── handlers ──────────────────────────────────────────────────────────────────

// List returns all research notes for the project.
//
// GET /projects/:id/research-notes
func (h *Handler) List(c *gin.Context) {
	projectID, _, ok := resolveIDs(c)
	if !ok {
		return
	}
	notes, err := h.svc.List(c.Request.Context(), projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, notes)
}

// Create inserts a new research note.
//
// POST /projects/:id/research-notes
//
//	{ "title": "...", "body": "...", "source_url": "...", "tags": ["..."] }
func (h *Handler) Create(c *gin.Context) {
	projectID, userID, ok := resolveIDs(c)
	if !ok {
		return
	}

	var req struct {
		Title     string   `json:"title"`
		Body      string   `json:"body"`
		SourceURL string   `json:"source_url"`
		Tags      []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}
	title := req.Title
	if title == "" {
		title = "Untitled Note"
	}

	note, err := h.svc.Create(c.Request.Context(), projectID, userID, title, req.Body, req.SourceURL, req.Tags)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, note)
}

// Get returns a single research note.
//
// GET /projects/:id/research-notes/:nid
func (h *Handler) Get(c *gin.Context) {
	projectID, _, ok := resolveIDs(c)
	if !ok {
		return
	}

	noteID, err := uuid.Parse(c.Param("nid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid note id"})
		return
	}

	note, err := h.svc.Get(c.Request.Context(), projectID, noteID)
	if err != nil {
		if err == pgx.ErrNoRows {
			handleError(c, apperror.NotFound("research note", noteID.String()))
			return
		}
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, note)
}

// Update replaces the mutable fields of a research note.
//
// PATCH /projects/:id/research-notes/:nid
//
//	{ "title": "...", "body": "...", "source_url": "...", "tags": [...] }
func (h *Handler) Update(c *gin.Context) {
	projectID, _, ok := resolveIDs(c)
	if !ok {
		return
	}

	noteID, err := uuid.Parse(c.Param("nid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid note id"})
		return
	}

	// Load current state so unchanged fields keep their values.
	current, err := h.svc.Get(c.Request.Context(), projectID, noteID)
	if err != nil {
		if err == pgx.ErrNoRows {
			handleError(c, apperror.NotFound("research note", noteID.String()))
			return
		}
		handleError(c, err)
		return
	}

	var req struct {
		Title     *string  `json:"title"`
		Body      *string  `json:"body"`
		SourceURL *string  `json:"source_url"`
		Tags      []string `json:"tags"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}

	title     := current.Title
	body      := current.Body
	sourceURL := current.SourceURL
	tags      := current.Tags

	if req.Title     != nil { title = *req.Title }
	if req.Body      != nil { body = *req.Body }
	if req.SourceURL != nil { sourceURL = *req.SourceURL }
	if req.Tags      != nil { tags = req.Tags }

	note, err := h.svc.Update(c.Request.Context(), projectID, noteID, title, body, sourceURL, tags)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, note)
}

// Delete removes a research note.
//
// DELETE /projects/:id/research-notes/:nid
func (h *Handler) Delete(c *gin.Context) {
	projectID, _, ok := resolveIDs(c)
	if !ok {
		return
	}

	noteID, err := uuid.Parse(c.Param("nid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid note id"})
		return
	}

	if err := h.svc.Delete(c.Request.Context(), projectID, noteID); err != nil {
		handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ── helpers ───────────────────────────────────────────────────────────────────

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

func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*apperror.AppError); ok {
		c.JSON(appErr.Code, appErr)
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"message": "internal error"})
}
