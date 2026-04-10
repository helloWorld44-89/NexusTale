package prompts

// Handler exposes Gin route handlers for writing style presets.
// All routes are mounted under /projects/:id/prompts in main.go.
//
//	GET    /                   list all presets for the project
//	POST   /                   create a new preset
//	PUT    /:promptId          replace a preset's fields
//	DELETE /:promptId          delete a preset

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/pkg/apperror"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("", h.List)
	rg.POST("", h.Create)
	rg.PUT("/:promptId", h.Update)
	rg.DELETE("/:promptId", h.Delete)
}

// List returns all writing style presets for the project.
//
// GET /projects/:id/prompts
func (h *Handler) List(c *gin.Context) {
	projectID, ok := parseProjectID(c)
	if !ok {
		return
	}
	prompts, err := h.svc.List(c.Request.Context(), projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, prompts)
}

// Create adds a new writing style preset to the project.
//
// POST /projects/:id/prompts
//
//	{ "name": "Gritty noir", "content": "Dark, sparse prose…", "system_content": "" }
func (h *Handler) Create(c *gin.Context) {
	projectID, ok := parseProjectID(c)
	if !ok {
		return
	}
	var req CreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request", "detail": err.Error()})
		return
	}
	p, err := h.svc.Create(c.Request.Context(), projectID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, p)
}

// Update patches a writing style preset.
//
// PUT /projects/:id/prompts/:promptId
//
//	{ "name": "Updated name", "sort_order": 2 }
func (h *Handler) Update(c *gin.Context) {
	_, ok := parseProjectID(c)
	if !ok {
		return
	}
	promptID, err := uuid.Parse(c.Param("promptId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid prompt id"})
		return
	}
	var req UpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request", "detail": err.Error()})
		return
	}
	p, err := h.svc.Update(c.Request.Context(), promptID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, p)
}

// Delete removes a writing style preset.
//
// DELETE /projects/:id/prompts/:promptId
func (h *Handler) Delete(c *gin.Context) {
	_, ok := parseProjectID(c)
	if !ok {
		return
	}
	promptID, err := uuid.Parse(c.Param("promptId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid prompt id"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), promptID); err != nil {
		handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func parseProjectID(c *gin.Context) (uuid.UUID, bool) {
	claims := auth.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return uuid.Nil, false
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid project id"})
		return uuid.Nil, false
	}
	return id, true
}

func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*apperror.AppError); ok {
		c.JSON(appErr.Code, appErr)
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"message": "internal error"})
}
