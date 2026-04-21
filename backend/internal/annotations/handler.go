package annotations

// Handler exposes REST routes for manuscript annotations.
//
// Routes (registered under /projects/:id):
//   GET    /scenes/:sid/annotations        → list annotations for a scene
//   POST   /scenes/:sid/annotations        → create annotation
//   PUT    /scenes/:sid/annotations/:aid   → update body; pass resolved:true to resolve
//   DELETE /scenes/:sid/annotations/:aid   → delete annotation

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

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	base := rg.Group("/scenes/:sid/annotations")
	base.GET("", h.List)
	base.POST("", h.Create)
	base.PUT("/:aid", h.Update)
	base.DELETE("/:aid", h.Delete)
}

// ── handlers ──────────────────────────────────────────────────────────────────

// List returns all annotations for the given scene.
//
// GET /projects/:id/scenes/:sid/annotations
func (h *Handler) List(c *gin.Context) {
	_, _, sceneID, ok := resolveIDs(c)
	if !ok {
		return
	}
	anns, err := h.svc.ListByScene(c.Request.Context(), sceneID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, anns)
}

// Create adds a new annotation to a scene.
//
// POST /projects/:id/scenes/:sid/annotations
//
//	{ "start_char": 0, "end_char": 20, "body": "...", "type": "note|suggestion|question" }
func (h *Handler) Create(c *gin.Context) {
	projectID, claims, sceneID, ok := resolveIDs(c)
	if !ok {
		return
	}

	var req struct {
		StartChar int32  `json:"start_char"`
		EndChar   int32  `json:"end_char"`
		Body      string `json:"body"`
		Type      string `json:"type"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Body == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "body and char offsets are required"})
		return
	}
	annType := req.Type
	if annType == "" {
		annType = "note"
	}

	ann, err := h.svc.Create(
		c.Request.Context(),
		projectID, sceneID, claims.UserID,
		claims.DisplayName,
		req.StartChar, req.EndChar,
		req.Body, annType,
	)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, ann)
}

// Update edits the body of an annotation, or resolves it.
//
// PUT /projects/:id/scenes/:sid/annotations/:aid
//
//	{ "body": "..." }           — update text
//	{ "resolved": true }        — resolve (owner only)
func (h *Handler) Update(c *gin.Context) {
	projectID, claims, _, ok := resolveIDs(c)
	if !ok {
		return
	}
	annotationID, err := uuid.Parse(c.Param("aid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid annotation id"})
		return
	}

	var req struct {
		Body     *string `json:"body"`
		Resolved *bool   `json:"resolved"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}

	if req.Resolved != nil && *req.Resolved {
		ann, err := h.svc.Resolve(c.Request.Context(), projectID, annotationID, claims.UserID)
		if err != nil {
			handleError(c, err)
			return
		}
		c.JSON(http.StatusOK, ann)
		return
	}

	if req.Body != nil {
		ann, err := h.svc.UpdateBody(c.Request.Context(), projectID, annotationID, *req.Body)
		if err != nil {
			handleError(c, err)
			return
		}
		c.JSON(http.StatusOK, ann)
		return
	}

	c.JSON(http.StatusBadRequest, gin.H{"message": "nothing to update"})
}

// Delete removes an annotation.
//
// DELETE /projects/:id/scenes/:sid/annotations/:aid
func (h *Handler) Delete(c *gin.Context) {
	projectID, _, _, ok := resolveIDs(c)
	if !ok {
		return
	}
	annotationID, err := uuid.Parse(c.Param("aid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid annotation id"})
		return
	}
	if err := h.svc.Delete(c.Request.Context(), projectID, annotationID); err != nil {
		handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func resolveIDs(c *gin.Context) (projectID uuid.UUID, claims *auth.Claims, sceneID uuid.UUID, ok bool) {
	claims = auth.GetClaims(c)
	if claims == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}
	var err error
	projectID, err = uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid project id"})
		return
	}
	sceneID, err = uuid.Parse(c.Param("sid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid scene id"})
		return
	}
	ok = true
	return
}

func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*apperror.AppError); ok {
		c.JSON(appErr.Code, appErr)
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"message": "internal error"})
}
