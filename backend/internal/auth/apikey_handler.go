package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterAPIKeyRoutes mounts the user API key CRUD endpoints under the given
// group. All routes require the caller to be authenticated (RequireAuth).
//
//	POST   /users/me/api-keys           — store or replace a key for a provider
//	GET    /users/me/api-keys           — list key hints (never raw keys)
//	DELETE /users/me/api-keys/:provider — remove a key
func (h *Handler) RegisterAPIKeyRoutes(rg *gin.RouterGroup) {
	me := rg.Group("/users/me/api-keys", RequireAuth(h.svc))
	me.POST("", h.UpsertAPIKey)
	me.GET("", h.ListAPIKeys)
	me.DELETE("/:provider", h.DeleteAPIKey)
}

type upsertAPIKeyRequest struct {
	Provider string `json:"provider" binding:"required"`
	Key      string `json:"key"      binding:"required"`
}

// UpsertAPIKey stores (or replaces) an encrypted API key for the given provider.
func (h *Handler) UpsertAPIKey(c *gin.Context) {
	userID := GetUserID(c)

	var req upsertAPIKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	resp, err := h.svc.UpsertAPIKey(c.Request.Context(), userID, req.Provider, req.Key)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ListAPIKeys returns all stored key hints for the authenticated user.
func (h *Handler) ListAPIKeys(c *gin.Context) {
	userID := GetUserID(c)

	keys, err := h.svc.ListAPIKeys(c.Request.Context(), userID)
	if err != nil {
		handleError(c, err)
		return
	}
	c.JSON(http.StatusOK, keys)
}

// DeleteAPIKey removes the stored key for the given provider.
func (h *Handler) DeleteAPIKey(c *gin.Context) {
	userID := GetUserID(c)
	provider := c.Param("provider")

	if err := h.svc.DeleteAPIKey(c.Request.Context(), userID, provider); err != nil {
		handleError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
