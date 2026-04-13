package guide

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/pkg/apperror"
)

// Handler exposes Gin route handlers for the novel guide wizard.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts guide routes under the provided project group.
// Expects :id to be the project UUID in the group path.
//
//	GET  /projects/:id/guide                    — full progress (all steps)
//	POST /projects/:id/guide/:step              — save step data (no completion)
//	POST /projects/:id/guide/:step/complete     — complete step + run side effects
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/guide", h.GetProgress)
	rg.POST("/guide/:step", h.SaveStep)
	rg.POST("/guide/:step/complete", h.CompleteStep)
}

// GetProgress handles GET /projects/:id/guide.
func (h *Handler) GetProgress(c *gin.Context) {
	projectID, ok := resolveProjectID(c)
	if !ok {
		return
	}
	progress, err := h.svc.GetProgress(c.Request.Context(), projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, progress)
}

// SaveStep handles POST /projects/:id/guide/:step.
// Persists the step data without marking it complete — safe to call on autosave.
func (h *Handler) SaveStep(c *gin.Context) {
	projectID, ok := resolveProjectID(c)
	if !ok {
		return
	}
	stepKey := c.Param("step")

	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid JSON body"})
		return
	}

	step, err := h.svc.SaveStep(c.Request.Context(), projectID, stepKey, raw)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, step)
}

// CompleteStep handles POST /projects/:id/guide/:step/complete.
// Saves data, marks the step done, and runs side effects (creates entities/chapters/scenes).
func (h *Handler) CompleteStep(c *gin.Context) {
	projectID, ok := resolveProjectID(c)
	if !ok {
		return
	}
	stepKey := c.Param("step")

	var raw json.RawMessage
	if err := c.ShouldBindJSON(&raw); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid JSON body"})
		return
	}

	step, err := h.svc.CompleteStep(c.Request.Context(), projectID, stepKey, raw)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, step)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func resolveProjectID(c *gin.Context) (uuid.UUID, bool) {
	claims := auth.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return uuid.Nil, false
	}
	pid, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid project id"})
		return uuid.Nil, false
	}
	return pid, true
}

func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*apperror.AppError); ok {
		c.JSON(appErr.Code, appErr)
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"message": "internal error"})
}
