package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// requireIdentity returns the authenticated caller's user id.
//
// Routes that read or mutate a specific account's data run behind
// middleware.Auth, which validates the bearer token and sets "user_id" in the
// context. This helper fails closed when that identity is missing, so a handler
// can never fall back to an id taken from the path, the query string or the
// body -- which is how one account reads or changes another's records.
func requireIdentity(c *gin.Context) (string, bool) {
	raw, exists := c.Get("user_id")
	if !exists {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return "", false
	}
	id, _ := raw.(string)
	if strings.TrimSpace(id) == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return "", false
	}
	return id, true
}
