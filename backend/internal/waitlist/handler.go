package waitlist

// Handler exposes the public waitlist endpoint and the admin list endpoint.
//
// Routes (no auth required):
//
//	POST /waitlist   → submit an invite request
//
// Routes (admin only — mount via RegisterAdminRoutes on an auth+role-gated group):
//
//	GET /admin/waitlist  → list all signups

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Handler serves the public waitlist route.
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes mounts the public waitlist route (no auth middleware).
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/waitlist", h.Create)
}

// RegisterAdminRoutes mounts admin-only waitlist routes on rg, which must
// already have RequireAuth + RequireRole(admin) middleware applied.
func (h *Handler) RegisterAdminRoutes(rg *gin.RouterGroup) {
	rg.GET("/waitlist", h.List)
}

// List returns all waitlist signups ordered by most-recent first.
//
// GET /api/v1/admin/waitlist
func (h *Handler) List(c *gin.Context) {
	signups, err := h.svc.List(c.Request.Context())
	if err != nil {
		slog.Error("waitlist list failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "internal error"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"signups": signups,
		"total":   len(signups),
	})
}

// Create processes an invite-request submission.
//
// POST /api/v1/waitlist
//
//	{ "email": "...", "what_they_write": "..." }
func (h *Handler) Create(c *gin.Context) {
	var req struct {
		Email         string `json:"email"`
		WhatTheyWrite string `json:"what_they_write"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}

	email := strings.TrimSpace(strings.ToLower(req.Email))
	whatTheyWrite := strings.TrimSpace(req.WhatTheyWrite)

	if email == "" || !strings.Contains(email, "@") || !strings.Contains(email, ".") {
		c.JSON(http.StatusBadRequest, gin.H{"message": "valid email required"})
		return
	}
	if whatTheyWrite == "" {
		c.JSON(http.StatusBadRequest, gin.H{"message": "please tell us what you write"})
		return
	}
	if len(whatTheyWrite) > 500 {
		c.JSON(http.StatusBadRequest, gin.H{"message": "what_they_write must be under 500 characters"})
		return
	}

	signup, err := h.svc.Create(c.Request.Context(), email, whatTheyWrite)
	if err != nil {
		slog.Error("waitlist signup failed", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"message": "internal error"})
		return
	}
	c.JSON(http.StatusOK, signup)
}
