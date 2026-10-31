package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// SuperAdminRequired returns middleware that requires super_admin role
func SuperAdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized - no role found",
			})
			c.Abort()
			return
		}

		roleStr, ok := role.(string)
		if !ok || roleStr != "super_admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Forbidden - super admin access required",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// APIKeyAuth returns middleware for API key authentication
// APIKeyAuth returns middleware that authenticates a request against the
// supplied key store. Requests without a valid, unexpired key are rejected.
func APIKeyAuth(store *APIKeyStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		presented := ParseAPIKey(
			c.GetHeader("X-API-Key"),
			c.GetHeader("Authorization"),
			c.Query("api_key"),
		)

		if presented == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
			c.Abort()
			return
		}

		entry, err := store.Validate(presented)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
			c.Abort()
			return
		}

		c.Set("api_key", MaskKey(presented))
		c.Set("api_key_label", entry.Label)
		c.Set("api_key_scopes", entry.Scopes)
		c.Next()
	}
}

// RequireScope returns middleware that requires the authenticated key to hold
// the given scope.
func RequireScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, exists := c.Get("api_key_scopes")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			c.Abort()
			return
		}

		scopes, ok := raw.(map[string]bool)
		if !ok || (!scopes[scope] && !scopes["*"]) {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient scope"})
			c.Abort()
			return
		}

		c.Next()
	}
}
