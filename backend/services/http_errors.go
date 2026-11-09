package services

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

// respondInternalError logs the underlying error and returns a generic body.
//
// A 5xx is raised by an internal operation (a database write, an upstream RPC,
// a signer), and the raw error can carry a SQL fragment, a filesystem path, an
// upstream URL or a credential-bearing connection string. None of that is
// useful to the caller, so it is logged for the operator and replaced with a
// fixed message on the wire.
func respondInternalError(c *gin.Context, err error) {
	log.Printf("internal error on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}
