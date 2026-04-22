package notifications

// Handler exposes REST routes for user notifications.
//
// Routes (all require RequireAuth, mounted at v1 root):
//   GET  /notifications              → List (unread first, then recent read)
//   PUT  /notifications/:id/read     → MarkRead
//   PUT  /notifications/read-all     → MarkAllRead

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

// RegisterRoutes mounts all notification routes on the provided auth-guarded group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/notifications", h.List)
	// read-all must be registered before /:id/read to avoid route conflict
	rg.PUT("/notifications/read-all", h.MarkAllRead)
	rg.PUT("/notifications/:id/read", h.MarkRead)
}

// List returns all notifications for the authenticated user.
//
// GET /notifications
func (h *Handler) List(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}
	notifs, err := h.svc.List(c.Request.Context(), userID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, notifs)
}

// MarkRead marks a single notification as read.
//
// PUT /notifications/:id/read
func (h *Handler) MarkRead(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}
	notifID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid notification id"})
		return
	}
	if err := h.svc.MarkRead(c.Request.Context(), userID, notifID); err != nil {
		handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// MarkAllRead marks every unread notification for the user as read.
//
// PUT /notifications/read-all
func (h *Handler) MarkAllRead(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}
	if err := h.svc.MarkAllRead(c.Request.Context(), userID); err != nil {
		handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*apperror.AppError); ok {
		c.JSON(appErr.Code, appErr)
		return
	}
	slog.Error("unhandled handler error", "path", c.FullPath(), "error", err)
	c.JSON(http.StatusInternalServerError, gin.H{"message": "internal error"})
}
