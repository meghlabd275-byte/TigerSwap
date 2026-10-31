// Package router provides routing utilities for the API Gateway.
//
// The gateway's route table lives in main.go's Server.SetupRouter, which wires
// the configured handlers and middleware. This package only exposes the group
// helper.
package router

import (
	"github.com/gin-gonic/gin"
)

// Group creates a new route group on its own engine.
func Group(prefix string, handlers ...gin.HandlerFunc) *gin.RouterGroup {
	engine := gin.New()
	return engine.Group(prefix, handlers...)
}
