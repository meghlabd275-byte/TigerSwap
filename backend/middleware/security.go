package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SuperAdminRequired returns middleware that requires the super-admin role. It
// runs after AuthRequired, which is what places `role` in the context.
func SuperAdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized - no role found",
			})
			return
		}

		roleStr, ok := role.(string)
		if !ok || roleStr != RoleSuperAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Forbidden - super admin access required",
			})
			return
		}

		c.Next()
	}
}
