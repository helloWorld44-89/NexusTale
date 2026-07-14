package admin

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

// Handler exposes admin-only API routes.
// All routes require RequireRole(auth.RoleAdmin) — applied at registration time.
type Handler struct {
	queries *sqlcgen.Queries
}

func NewHandler(queries *sqlcgen.Queries) *Handler {
	return &Handler{queries: queries}
}

// RegisterRoutes mounts all admin routes under /admin.
// The caller must apply RequireAuth + RequireRole(RoleAdmin) to the group.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	admin := rg.Group("/admin")
	admin.GET("/stats", h.GetStats)
	admin.GET("/users", h.ListUsers)
	admin.PATCH("/users/:uid", h.UpdateUser)
	admin.GET("/ai-usage", h.ListAIUsage)
}

// ── response types ────────────────────────────────────────────────────────────

type statsResponse struct {
	TotalUsers    int32   `json:"total_users"`
	TotalProjects int32   `json:"total_projects"`
	TotalScenes   int32   `json:"total_scenes"`
	TotalAICalls  int32   `json:"total_ai_calls"`
	TotalTokens   int64   `json:"total_tokens"`
	TotalCostUSD  float64 `json:"total_cost_usd"`
}

type adminUserResponse struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	DisplayName  string `json:"display_name"`
	Role         string `json:"role"`
	Plan         string `json:"plan"`
	CreatedAt    string `json:"created_at"`
	ProjectCount int32  `json:"project_count"`
}

type aiUsageResponse struct {
	UserID      string  `json:"user_id"`
	Email       string  `json:"email"`
	DisplayName string  `json:"display_name"`
	CallCount   int32   `json:"call_count"`
	TotalTokens int64   `json:"total_tokens"`
	TotalCost   float64 `json:"total_cost_usd"`
}

// ── handlers ──────────────────────────────────────────────────────────────────

// GetStats returns system-wide aggregate counts and AI cost totals.
// GET /admin/stats
func (h *Handler) GetStats(c *gin.Context) {
	row, err := h.queries.AdminGetStats(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": fmt.Sprintf("stats: %v", err)})
		return
	}
	c.JSON(http.StatusOK, statsResponse{
		TotalUsers:    row.TotalUsers,
		TotalProjects: row.TotalProjects,
		TotalScenes:   row.TotalScenes,
		TotalAICalls:  row.TotalAiCalls,
		TotalTokens:   row.TotalTokens,
		TotalCostUSD:  numericToFloat64(row.TotalCostUsd),
	})
}

// ListUsers returns a paginated list of all users with role, plan, and project count.
// GET /admin/users?limit=50&offset=0
func (h *Handler) ListUsers(c *gin.Context) {
	limit := int32(50)
	offset := int32(0)
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "50")); err == nil && l > 0 && l <= 200 {
		limit = int32(l)
	}
	if o, err := strconv.Atoi(c.DefaultQuery("offset", "0")); err == nil && o >= 0 {
		offset = int32(o)
	}

	rows, err := h.queries.AdminListUsers(c.Request.Context(), sqlcgen.AdminListUsersParams{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": fmt.Sprintf("list users: %v", err)})
		return
	}

	out := make([]adminUserResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, adminUserResponse{
			ID:           r.ID.String(),
			Email:        r.Email,
			DisplayName:  r.DisplayName,
			Role:         string(r.Role),
			Plan:         r.Plan,
			CreatedAt:    r.CreatedAt.Time.Format("2006-01-02T15:04:05Z"),
			ProjectCount: r.ProjectCount,
		})
	}
	c.JSON(http.StatusOK, out)
}

// UpdateUser sets role and/or plan on a user.
// PATCH /admin/users/:uid  { "role": "admin", "plan": "scribe" }
func (h *Handler) UpdateUser(c *gin.Context) {
	uid, err := uuid.Parse(c.Param("uid"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid user id"})
		return
	}

	var req struct {
		Role *string `json:"role"` // "author" | "admin"
		Plan *string `json:"plan"` // "free" | "scribe" | "chronicler" | "studio"
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "invalid request"})
		return
	}

	if req.Role != nil {
		role := sqlcgen.UserRole(*req.Role)
		if role != sqlcgen.UserRoleAuthor && role != sqlcgen.UserRoleAdmin && role != sqlcgen.UserRoleViewer {
			c.JSON(http.StatusBadRequest, gin.H{"message": "role must be author, admin, or viewer"})
			return
		}
		if err := h.queries.AdminSetUserRole(c.Request.Context(), sqlcgen.AdminSetUserRoleParams{
			ID:   uid,
			Role: role,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": fmt.Sprintf("set role: %v", err)})
			return
		}
	}

	if req.Plan != nil {
		plan := *req.Plan
		if plan != "free" && plan != "scribe" && plan != "chronicler" && plan != "studio" {
			c.JSON(http.StatusBadRequest, gin.H{"message": "plan must be free, scribe, chronicler, or studio"})
			return
		}
		if err := h.queries.AdminSetUserPlan(c.Request.Context(), sqlcgen.AdminSetUserPlanParams{
			ID:   uid,
			Plan: plan,
		}); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"message": fmt.Sprintf("set plan: %v", err)})
			return
		}
	}

	c.Status(http.StatusNoContent)
}

// ListAIUsage returns per-user AI call counts and token spend for the last 30 days.
// GET /admin/ai-usage
func (h *Handler) ListAIUsage(c *gin.Context) {
	rows, err := h.queries.AdminListAIUsage(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": fmt.Sprintf("ai usage: %v", err)})
		return
	}

	out := make([]aiUsageResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, aiUsageResponse{
			UserID:      r.UserID.String(),
			Email:       r.Email,
			DisplayName: r.DisplayName,
			CallCount:   r.CallCount,
			TotalTokens: r.TotalTokens,
			TotalCost:   numericToFloat64(r.TotalCostUsd),
		})
	}
	c.JSON(http.StatusOK, out)
}

// numericToFloat64 converts a pgtype.Numeric scanned as interface{} to float64.
// Mirrors the helper in internal/ai/service.go — kept local to avoid import cycle.
func numericToFloat64(v interface{}) float64 {
	if n, ok := v.(pgtype.Numeric); ok {
		f, err := n.Float64Value()
		if err == nil && f.Valid {
			return f.Float64
		}
	}
	return 0
}

// RequireAdmin returns a middleware that enforces RoleAdmin.
// Convenience wrapper so callers don't need to import auth directly.
func RequireAdmin(authSvc *auth.Service) gin.HandlerFunc {
	return auth.RequireRole(auth.RoleAdmin)
}
