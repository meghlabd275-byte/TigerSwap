package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// AdminRequired returns middleware that requires admin role
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user role from JWT claims
		role, exists := c.Get("role")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized - no role found",
			})
			c.Abort()
			return
		}
		
		roleStr, ok := role.(string)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Unauthorized - invalid role",
			})
			c.Abort()
			return
		}
		
		// Check if admin or super_admin
		if roleStr != "admin" && roleStr != "super_admin" {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Forbidden - admin access required",
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

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
func APIKeyAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check header for API key
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			// Also check query parameter
			apiKey = c.Query("api_key")
		}
		
		if apiKey == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "API key required",
			})
			c.Abort()
			return
		}
		
		// In production, validate API key against database
		// For now, accept any non-empty key (validation should be implemented)
		c.Set("api_key", apiKey)
		c.Next()
	}
}

// CORS middleware for cross-origin requests
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// Allowed origins (configure in production)
		allowedOrigins := []string{
			"https://tigerswap.io",
			"https://www.tigerswap.io",
			"https://app.tigerswap.io",
			"http://localhost:3000",
			"http://localhost:8080",
		}
		
		allowed := false
		for _, o := range allowedOrigins {
			if origin == o || strings.HasSuffix(origin, ".tigerswap.io") {
				allowed = true
				break
			}
		}
		
		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
		}
		
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-API-Key")
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Max-Age", "86400")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		
		c.Next()
	}
}

// SecurityHeaders adds security headers
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self' https: wss:")
		c.Next()
	}
}
