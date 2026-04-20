package collaboration

// Handler exposes REST routes for project collaboration.
//
// Public routes (no auth):
//   GET  /invites/:token          → GetInvitePreview
//
// Authenticated routes:
//   POST /invites/:token/accept   → AcceptInvite
//
// Project-scoped routes (auth + project membership):
//   POST   /projects/:id/invites                → InviteCollaborator
//   GET    /projects/:id/invites                → ListPendingInvites
//   GET    /projects/:id/collaborators          → ListCollaborators
//   DELETE /projects/:id/collaborators/:uid     → RemoveCollaborator

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

// RegisterPublicRoutes mounts the unauthenticated invite preview route.
// Caller supplies the root v1 group.
func (h *Handler) RegisterPublicRoutes(rg *gin.RouterGroup) {
	rg.GET("/invites/:token", h.GetInvitePreview)
}

// RegisterAuthRoutes mounts routes that need auth but no project context.
// Caller supplies an authenticated (RequireAuth) group rooted at the v1 prefix.
func (h *Handler) RegisterAuthRoutes(rg *gin.RouterGroup) {
	rg.POST("/invites/:token/accept", h.AcceptInvite)
}

// RegisterProjectRoutes mounts project-scoped collaboration routes.
// Caller supplies an authenticated group rooted at /projects/:id.
func (h *Handler) RegisterProjectRoutes(rg *gin.RouterGroup) {
	rg.POST("/invites", h.InviteCollaborator)
	rg.GET("/invites", h.ListPendingInvites)
	rg.GET("/collaborators", h.ListCollaborators)
	rg.DELETE("/collaborators/:uid", h.RemoveCollaborator)
}

// ── handlers ──────────────────────────────────────────────────────────────────

// GetInvitePreview returns safe preview info for an invite token.
//
// GET /invites/:token
func (h *Handler) GetInvitePreview(c *gin.Context) {
	token := c.Param("token")
	preview, err := h.svc.GetInvitePreview(c.Request.Context(), token)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, preview)
}

// AcceptInvite validates the token against the authenticated user and sets up
// their collaborator workspace.
//
// POST /invites/:token/accept
func (h *Handler) AcceptInvite(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == uuid.Nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return
	}

	token := c.Param("token")
	collab, err := h.svc.AcceptInvite(c.Request.Context(), userID, token)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, collab)
}

// InviteCollaborator creates a new invite for the given email and role.
//
// POST /projects/:id/invites
//
//	{ "email": "...", "role": "coauthor|editor|reviewer" }
func (h *Handler) InviteCollaborator(c *gin.Context) {
	projectID, ownerID, ok := resolveIDs(c)
	if !ok {
		return
	}

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request body"})
		return
	}
	if req.Email == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "email is required"})
		return
	}

	inv, err := h.svc.InviteCollaborator(c.Request.Context(), ownerID, projectID, req.Email, req.Role)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusCreated, inv)
}

// ListPendingInvites returns unexpired, unaccepted invites for the project.
//
// GET /projects/:id/invites
func (h *Handler) ListPendingInvites(c *gin.Context) {
	projectID, ownerID, ok := resolveIDs(c)
	if !ok {
		return
	}
	invites, err := h.svc.ListPendingInvites(c.Request.Context(), ownerID, projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, invites)
}

// ListCollaborators returns all accepted collaborators for the project.
//
// GET /projects/:id/collaborators
func (h *Handler) ListCollaborators(c *gin.Context) {
	projectID, _, ok := resolveIDs(c)
	if !ok {
		return
	}
	collabs, err := h.svc.ListCollaborators(c.Request.Context(), projectID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, collabs)
}

// RemoveCollaborator removes a collaborator from the project.
//
// DELETE /projects/:id/collaborators/:uid
func (h *Handler) RemoveCollaborator(c *gin.Context) {
	projectID, ownerID, ok := resolveIDs(c)
	if !ok {
		return
	}
	targetID, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid user id"})
		return
	}

	if err := h.svc.RemoveCollaborator(c.Request.Context(), ownerID, projectID, targetID); err != nil {
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
