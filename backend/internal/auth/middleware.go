package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const (
	contextKeyClaims = "auth_claims"
)

func RequireAuth(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "missing authorization header"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "invalid authorization header format"})
			return
		}

		claims, err := svc.ValidateAccessToken(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "invalid or expired token"})
			return
		}

		c.Set(contextKeyClaims, claims)
		c.Next()
	}
}

func RequireRole(roles ...Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims := GetClaims(c)
		if claims == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"message": "not authenticated"})
			return
		}

		for _, r := range roles {
			if claims.Role == r {
				c.Next()
				return
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"message": "insufficient permissions"})
	}
}

func GetClaims(c *gin.Context) *Claims {
	val, exists := c.Get(contextKeyClaims)
	if !exists {
		return nil
	}
	claims, ok := val.(*Claims)
	if !ok {
		return nil
	}
	return claims
}

func GetUserID(c *gin.Context) uuid.UUID {
	claims := GetClaims(c)
	if claims == nil {
		return uuid.Nil
	}
	return claims.UserID
}
