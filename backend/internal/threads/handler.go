package threads

// Handler exposes REST routes for story threads.
//
// Routes (registered under /projects/:id):
//   GET    /story-threads        → list threads
//   POST   /story-threads        → create thread
//   PUT    /story-threads/:tid   → update thread (title, type, notes, open/close)
//   DELETE /story-threads/:tid   → delete thread

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jconder44/nexustale/pkg/apperror"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/story-threads")
	g.GET("", h.List)
	g.POST("", h.Create)
	g.PUT("/:tid", h.Update)
	g.DELETE("/:tid", h.Delete)
}

func (h *Handler) List(c *gin.Context) {
	projectID, ok := parseProjectID(c)
	if !ok {
		return
	}
	threads, err := h.svc.List(c.Request.Context(), projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, threads)
}

func (h *Handler) Create(c *gin.Context) {
	projectID, ok := parseProjectID(c)
	if !ok {
		return
	}
	var req CreateThreadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	thread, err := h.svc.Create(c.Request.Context(), projectID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, thread)
}

func (h *Handler) Update(c *gin.Context) {
	_, ok := parseProjectID(c)
	if !ok {
		return
	}
	threadID, err := uuid.Parse(c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}
	var req UpdateThreadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	thread, err := h.svc.Update(c.Request.Context(), threadID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, thread)
}

func (h *Handler) Delete(c *gin.Context) {
	_, ok := parseProjectID(c)
	if !ok {
		return
	}
	threadID, err := uuid.Parse(c.Param("tid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid thread id"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), threadID); err != nil {
		handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseProjectID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return uuid.Nil, false
	}
	return id, true
}

func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*apperror.AppError); ok {
		c.JSON(appErr.Code, appErr)
		return
	}
	slog.Error("threads: unhandled error", "path", c.FullPath(), "error", err)
	c.JSON(http.StatusInternalServerError, gin.H{"message": "internal error"})
}
