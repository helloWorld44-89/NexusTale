package maps

import (
	"log/slog"
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

// RegisterRoutes mounts map routes under /projects/:id/maps.
// Caller must apply auth + project-access middleware.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	m := rg.Group("/maps")
	m.GET("", h.ListMaps)
	m.POST("", h.CreateMap)
	m.GET("/:mid", h.GetMap)
	m.PUT("/:mid", h.UpdateMap)
	m.DELETE("/:mid", h.DeleteMap)
}

func parseUUID(c *gin.Context, param string) (uuid.UUID, error) {
	id, err := uuid.Parse(c.Param(param))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid " + param})
	}
	return id, err
}

func (h *Handler) CreateMap(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}
	userID := auth.GetUserID(c)

	var req CreateMapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	resp, err := h.svc.CreateMap(c.Request.Context(), projectID, userID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) GetMap(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}
	mapID, err := parseUUID(c, "mid")
	if err != nil {
		return
	}
	userID := auth.GetUserID(c)

	resp, err := h.svc.GetMap(c.Request.Context(), projectID, userID, mapID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) ListMaps(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}

	resp, err := h.svc.ListMaps(c.Request.Context(), projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) UpdateMap(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}
	mapID, err := parseUUID(c, "mid")
	if err != nil {
		return
	}
	userID := auth.GetUserID(c)

	var req UpdateMapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	resp, err := h.svc.UpdateMap(c.Request.Context(), projectID, userID, mapID, req)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

func (h *Handler) DeleteMap(c *gin.Context) {
	projectID, err := parseUUID(c, "id")
	if err != nil {
		return
	}
	mapID, err := parseUUID(c, "mid")
	if err != nil {
		return
	}

	if err := h.svc.DeleteMap(c.Request.Context(), projectID, mapID); err != nil {
		handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*apperror.AppError); ok {
		c.JSON(appErr.Code, appErr)
		return
	}
	slog.Error("maps: unhandled handler error", "path", c.FullPath(), "error", err)
	c.JSON(http.StatusInternalServerError, gin.H{"message": "internal error"})
}
