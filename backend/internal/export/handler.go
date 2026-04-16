package export

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/pkg/apperror"
)

// Handler exposes Gin route handlers for project exports.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts export routes under the provided project group.
// Expects :id to be the project UUID in the group path.
//
//	POST /projects/:id/export           — start an export (markdown streams; epub enqueues)
//	GET  /projects/:id/export           — list recent export jobs
//	GET  /projects/:id/export/:job_id   — poll a specific job + get download URL
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/export", h.Export)
	rg.GET("/export", h.ListJobs)
	rg.GET("/export/:job_id", h.GetJob)
}

type exportRequest struct {
	Format string `json:"format"` // "markdown" | "epub"
}

// Export handles POST /projects/:id/export.
//
// markdown → streams a zip immediately with Content-Disposition: attachment.
// epub     → enqueues an async job, returns {"job_id":"..."} with HTTP 202.
func (h *Handler) Export(c *gin.Context) {
	projectID, userID, ok := resolveIDs(c)
	if !ok {
		return
	}

	var req exportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}
	if req.Format != "markdown" && req.Format != "epub" && req.Format != "docx" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "format must be 'markdown', 'epub', or 'docx'"})
		return
	}

	proj, err := h.svc.queries.GetProject(c.Request.Context(), projectID)
	if err != nil {
		handleError(c, apperror.NotFound("project", projectID.String()))
		return
	}

	switch req.Format {
	case "markdown":
		filename := fmt.Sprintf("%s.zip", slugify(proj.Title))
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
		c.Header("Content-Type", "application/zip")
		c.Status(http.StatusOK)
		if err := h.svc.ExportMarkdown(c.Request.Context(), projectID, c.Writer); err != nil {
			// Headers are already sent; we can't write a JSON error body.
			// The truncated/corrupt zip signals the failure to the client.
			c.Abort()
		}

	case "epub":
		jobID, err := h.svc.EnqueueEPUB(c.Request.Context(), projectID, userID, proj.Title)
		if err != nil {
			handleError(c, err)
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"job_id": jobID})

	case "docx":
		jobID, err := h.svc.EnqueueDOCX(c.Request.Context(), projectID, userID, proj.Title)
		if err != nil {
			handleError(c, err)
			return
		}
		c.JSON(http.StatusAccepted, gin.H{"job_id": jobID})
	}
}

// ListJobs handles GET /projects/:id/export.
func (h *Handler) ListJobs(c *gin.Context) {
	projectID, _, ok := resolveIDs(c)
	if !ok {
		return
	}
	jobs, err := h.svc.ListJobs(c.Request.Context(), projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"jobs": jobs})
}

// GetJob handles GET /projects/:id/export/:job_id.
func (h *Handler) GetJob(c *gin.Context) {
	_, userID, ok := resolveIDs(c)
	if !ok {
		return
	}
	jobID, err := uuid.Parse(c.Param("job_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid job_id"})
		return
	}
	job, err := h.svc.GetJob(c.Request.Context(), jobID, userID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, job)
}

// resolveIDs extracts projectID from :id and userID from JWT claims.
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

// handleError maps apperror to HTTP status codes.
func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*apperror.AppError); ok {
		c.JSON(appErr.Code, appErr)
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"message": "internal error"})
}
