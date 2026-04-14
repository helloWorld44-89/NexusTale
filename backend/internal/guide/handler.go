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
//	POST /projects/:id/guide/structure/score    — score answers, return ranked suggestions
//	GET  /projects/:id/structure                — current structure selection
//	PUT  /projects/:id/structure                — set or clear structure selection
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/guide", h.GetProgress)
	rg.POST("/guide/:step", h.SaveStep)
	rg.POST("/guide/:step/complete", h.CompleteStep)
	rg.POST("/guide/structure/score", h.ScoreStructures)
	rg.GET("/structure", h.GetStructure)
	rg.PUT("/structure", h.UpdateStructure)
}

// RegisterPublicRoutes mounts routes that require no authentication.
//
//	GET /novel-structures — full catalog of seeded structure templates
func (h *Handler) RegisterPublicRoutes(rg *gin.RouterGroup) {
	rg.GET("/novel-structures", h.ListStructures)
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

// ListStructures handles GET /novel-structures (no auth required).
func (h *Handler) ListStructures(c *gin.Context) {
	list, err := h.svc.ListStructures(c.Request.Context())
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, list)
}

// ScoreStructures handles POST /projects/:id/guide/structure/score.
// Pure computation — returns ranked suggestions without persisting anything.
func (h *Handler) ScoreStructures(c *gin.Context) {
	var req ScoreRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid JSON body"})
		return
	}
	ranked, err := h.svc.ScoreStructures(c.Request.Context(), req.Answers)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ranked": ranked})
}

// GetStructure handles GET /projects/:id/structure.
func (h *Handler) GetStructure(c *gin.Context) {
	projectID, ok := resolveProjectID(c)
	if !ok {
		return
	}
	resp, err := h.svc.GetStructure(c.Request.Context(), projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateStructure handles PUT /projects/:id/structure.
func (h *Handler) UpdateStructure(c *gin.Context) {
	projectID, ok := resolveProjectID(c)
	if !ok {
		return
	}
	var req UpdateStructureRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid JSON body"})
		return
	}
	resp, err := h.svc.UpdateStructure(c.Request.Context(), projectID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
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
