package merge

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

// RegisterRoutes mounts merge-request routes under /projects/:id.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/merge-requests", h.openMR)
	rg.GET("/merge-requests", h.listMRs)
	rg.GET("/merge-requests/:mid", h.getMR)
	rg.GET("/merge-requests/:mid/diff", h.getMRDiff)
	rg.PUT("/merge-requests/:mid", h.resolveMR)
}

// POST /projects/:id/merge-requests
func (h *Handler) openMR(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	callerID := auth.GetUserID(c)

	var body struct {
		FromBranch  string `json:"from_branch"  binding:"required"`
		Title       string `json:"title"        binding:"required"`
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mr, err := h.svc.OpenMergeRequest(c.Request.Context(), callerID, projectID, body.FromBranch, body.Title, body.Description)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, mr)
}

// GET /projects/:id/merge-requests
func (h *Handler) listMRs(c *gin.Context) {
	projectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid project id"})
		return
	}
	callerID := auth.GetUserID(c)

	mrs, err := h.svc.ListMergeRequests(c.Request.Context(), callerID, projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, mrs)
}

// GET /projects/:id/merge-requests/:mid
func (h *Handler) getMR(c *gin.Context) {
	mrID, err := uuid.Parse(c.Param("mid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merge request id"})
		return
	}
	callerID := auth.GetUserID(c)

	mr, err := h.svc.GetMergeRequest(c.Request.Context(), callerID, mrID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, mr)
}

// GET /projects/:id/merge-requests/:mid/diff
func (h *Handler) getMRDiff(c *gin.Context) {
	mrID, err := uuid.Parse(c.Param("mid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merge request id"})
		return
	}
	callerID := auth.GetUserID(c)

	diff, err := h.svc.GetMergeRequestDiff(c.Request.Context(), callerID, mrID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, diff)
}

// PUT /projects/:id/merge-requests/:mid
func (h *Handler) resolveMR(c *gin.Context) {
	mrID, err := uuid.Parse(c.Param("mid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid merge request id"})
		return
	}
	callerID := auth.GetUserID(c)

	var body ResolveRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	mr, err := h.svc.ResolveMergeRequest(c.Request.Context(), callerID, mrID, body)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, mr)
}

func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*apperror.AppError); ok {
		c.JSON(appErr.Code, gin.H{"error": appErr.Error()})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}
