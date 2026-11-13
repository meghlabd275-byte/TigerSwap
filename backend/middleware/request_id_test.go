package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRequestIDGeneratesWhenAbsent pins that a request with no X-Request-ID is
// given one and the response echoes it, so a client can quote the id when
// reporting an issue and the server log line carries the same value.
func TestRequestIDGeneratesWhenAbsent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"id": RequestIDFromContext(c)})
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	router.ServeHTTP(rec, req)

	header := rec.Header().Get("X-Request-ID")
	if header == "" {
		t.Fatalf("response must carry a generated X-Request-ID")
	}
	if len(header) != 32 {
		t.Fatalf("expected a 32-char hex id, got %q", header)
	}
}

// TestRequestIDPreservesUpstreamID pins that an id an upstream proxy already
// supplied is kept, so a request is traceable across services instead of being
// re-minted at each hop.
func TestRequestIDPreservesUpstreamID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Request-ID", "upstream-abc")
	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got != "upstream-abc" {
		t.Fatalf("upstream id must be preserved: got %q", got)
	}
}

// TestRequestIDRejectsOversizedHeader pins that a client cannot inject an
// arbitrarily long id into every log line; an oversized value is replaced.
func TestRequestIDRejectsOversizedHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RequestID())
	router.GET("/ping", func(c *gin.Context) { c.Status(http.StatusOK) })

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set("X-Request-ID", string(make([]byte, 300)))
	router.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got == "" || len(got) == 300 {
		t.Fatalf("oversized id must be replaced, got len %d", len(got))
	}
}
