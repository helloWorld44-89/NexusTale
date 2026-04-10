package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jconder44/nexustale/pkg/apperror"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	rg.POST("/register", h.Register)
	rg.POST("/login", h.Login)
	rg.POST("/refresh", h.Refresh)
	rg.POST("/logout", RequireAuth(h.svc), h.Logout)

	// Authenticated user routes
	me := rg.Group("/users/me", RequireAuth(h.svc))
	me.GET("", h.GetMe)
	me.DELETE("", h.DeleteMe)
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	resp, err := h.svc.Register(c.Request.Context(), req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	resp, err := h.svc.Login(c.Request.Context(), req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *Handler) Refresh(c *gin.Context) {
	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	tokens, err := h.svc.Refresh(c.Request.Context(), req)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, tokens)
}

func (h *Handler) Logout(c *gin.Context) {
	var req LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "validation error", "detail": err.Error()})
		return
	}

	if err := h.svc.Logout(c.Request.Context(), req); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (h *Handler) GetMe(c *gin.Context) {
	claims := claimsFromCtx(c)
	if claims == nil {
		return
	}

	user, err := h.svc.GetMe(c.Request.Context(), claims.UserID)
	if err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, user)
}

func (h *Handler) DeleteMe(c *gin.Context) {
	claims := claimsFromCtx(c)
	if claims == nil {
		return
	}

	if err := h.svc.DeleteMe(c.Request.Context(), claims.UserID); err != nil {
		handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "account deleted"})
}

func claimsFromCtx(c *gin.Context) *Claims {
	raw, ok := c.Get(contextKeyClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return nil
	}
	claims, ok := raw.(*Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
		return nil
	}
	return claims
}

func handleError(c *gin.Context, err error) {
	if appErr, ok := err.(*apperror.AppError); ok {
		c.JSON(appErr.Code, appErr)
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"message": "internal error"})
}
