// Package httperr renders service failures as HTTP responses without leaking
// internal detail.
//
// A handler that writes err.Error() to a response body can hand a client a SQL
// fragment, a filesystem path, a request URL or a credential-bearing endpoint.
// The helpers here log the raw error for the operator and put a fixed message
// on the wire, so the status code still tells the caller what happened while
// the detail stays in the server log.
package httperr

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Message logs err and answers with the given status and a caller-safe
// message. Use it when the status is computed and the body needs extra fields
// that the typed helpers do not cover.
func Message(c *gin.Context, status int, message string, err error) {
	log.Printf("error on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(status, gin.H{"error": message})
}

// Internal logs an unexpected failure and returns a generic 500 body.
func Internal(c *gin.Context, err error) {
	log.Printf("internal error on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

// BadRequest logs a caller-correctable failure and returns a generic 400 body.
func BadRequest(c *gin.Context, err error) {
	log.Printf("bad request on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusBadRequest, gin.H{"error": "request could not be processed"})
}

// BadRequestMessage answers a 400 with a fixed, caller-safe message of the
// handler's choosing; the raw error is logged, not returned.
func BadRequestMessage(c *gin.Context, message string, err error) {
	log.Printf("bad request on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusBadRequest, gin.H{"error": message})
}

// Unauthorized answers a failed authentication with a fixed message. The raw
// error can distinguish "no such account" from "wrong password", which is an
// account-enumeration oracle, so only the caller-safe text is returned.
func Unauthorized(c *gin.Context, message string, err error) {
	log.Printf("unauthorized on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusUnauthorized, gin.H{"error": message})
}

// NotFound answers a missing resource with a fixed message.
func NotFound(c *gin.Context, message string, err error) {
	log.Printf("not found on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusNotFound, gin.H{"error": message})
}

// Forbidden answers a permission failure with a fixed message.
func Forbidden(c *gin.Context, message string, err error) {
	log.Printf("forbidden on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusForbidden, gin.H{"error": message})
}

// Conflict answers a state conflict with a fixed message.
func Conflict(c *gin.Context, message string, err error) {
	log.Printf("conflict on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusConflict, gin.H{"error": message})
}

// BadGateway answers an upstream failure. The dialer's error quotes the
// endpoint, which can carry a key in its path or query, so only a fixed
// message is returned.
func BadGateway(c *gin.Context, message string, err error) {
	log.Printf("upstream error on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusBadGateway, gin.H{"error": message})
}

// BadGatewayService answers an upstream transport failure, naming the service
// so the caller can tell which dependency was unavailable.
func BadGatewayService(c *gin.Context, service string, err error) {
	log.Printf("%s upstream error on %s %s: %v", service, c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusBadGateway, gin.H{"error": service + " upstream unavailable"})
}
