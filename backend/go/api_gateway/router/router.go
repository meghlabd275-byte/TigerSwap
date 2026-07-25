// Package router provides routing utilities for the API Gateway
package router

import (
	"github.com/gin-gonic/gin"
	"tigerswap/backend/go/api_gateway/middleware"
)

// Group creates a new route group
func Group(prefix string, handlers ...gin.HandlerFunc) gin.RouterGroup {
	engine := gin.New()
	return engine.Group(prefix, handlers...)
}

// RegisterRoutes registers all API routes
func RegisterRoutes(r *gin.Engine) {
	// Health check routes
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// API v1 routes
	v1 := r.Group("/api/v1")
	v1.Use(middleware.Logger())
	v1.Use(middleware.RequestID())
	v1.Use(middleware.Recovery())
	v1.Use(middleware.Secure())
	{
		// Routes will be registered here
	}
}
