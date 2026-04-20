package collaboration

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jconder44/nexustale/internal/auth"
	"github.com/jconder44/nexustale/pkg/db/sqlcgen"
)

const contextKeyCollabRole = "collab_role"

// RequireProjectAccess is a Gin middleware that passes if the authenticated
// user is either the project owner or an accepted collaborator.
// It sets "collab_role" in the context ("owner", "coauthor", "editor", "reviewer")
// so downstream handlers can enforce role-specific restrictions.
//
// Must be applied after RequireAuth so that JWT claims are already in context.
func RequireProjectAccess(queries *sqlcgen.Queries) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := auth.GetUserID(c)
		if userID == uuid.Nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "unauthorized"})
			return
		}

		projectID, err := uuid.Parse(c.Param("id"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"message": "invalid project id"})
			return
		}

		p, err := queries.GetProject(c.Request.Context(), projectID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"message": "project not found"})
			return
		}

		if p.OwnerID == userID {
			c.Set(contextKeyCollabRole, "owner")
			c.Next()
			return
		}

		collab, err := queries.GetCollaborator(c.Request.Context(), sqlcgen.GetCollaboratorParams{
			ProjectID: projectID,
			UserID:    userID,
		})
		if err != nil {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "you are not a member of this project"})
			return
		}

		c.Set(contextKeyCollabRole, collab.Role)
		c.Next()
	}
}

// GetCollabRole returns the resolved role for the current request.
// Returns "owner" for project owners, the collaborator role for members,
// or "" if the middleware was not applied.
func GetCollabRole(c *gin.Context) string {
	v, _ := c.Get(contextKeyCollabRole)
	role, _ := v.(string)
	return role
}

// IsOwner is a convenience check for handlers that are owner-only.
func IsOwner(c *gin.Context) bool {
	return GetCollabRole(c) == "owner"
}
