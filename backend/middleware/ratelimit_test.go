package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestRedisRateLimiterFallsBackWhenRedisUnavailable proves the limiter still
// applies a limit when the distributed store is unreachable. Before the
// fallback existed, a Redis outage let every request through.
func TestRedisRateLimiterFallsBackWhenRedisUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RedisRateLimiter(nil)) // nil client models an unreachable Redis
	router.GET("/data", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	status := func() int {
		req := httptest.NewRequest(http.MethodGet, "/data", nil)
		req.RemoteAddr = "203.0.113.7:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	for i := 0; i < 100; i++ {
		if code := status(); code != http.StatusOK {
			t.Fatalf("request %d should be allowed, got %d", i+1, code)
		}
	}
	if code := status(); code != http.StatusTooManyRequests {
		t.Fatalf("request 101 should be rate limited, got %d", code)
	}
}

func TestHealthCheckIsNotRateLimited(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RedisRateLimiter(nil))
	router.GET("/health", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "healthy"}) })

	for i := 0; i < 150; i++ {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		req.RemoteAddr = "203.0.113.9:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("health check must never be rate limited, got %d on request %d", rec.Code, i+1)
		}
	}
}

// TestBodySizeLimitRejectsOversizedBody proves a declared oversized body is
// rejected before a handler can read it into memory.
func TestBodySizeLimitRejectsOversizedBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BodySizeLimit(16))
	router.POST("/data", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	req := httptest.NewRequest(http.MethodPost, "/data", strings.NewReader(strings.Repeat("a", 64)))
	req.ContentLength = 64
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("an oversized body should be rejected, got %d", rec.Code)
	}
}

func TestBodySizeLimitAllowsSmallBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(BodySizeLimit(1024))
	router.POST("/data", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	req := httptest.NewRequest(http.MethodPost, "/data", strings.NewReader(`{"a":1}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("a small body should be accepted, got %d", rec.Code)
	}
}
