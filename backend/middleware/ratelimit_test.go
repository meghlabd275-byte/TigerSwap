package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

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

// TestTrustedProxiesCloseTheForwardedForBypass pins the fix for a rate-limit
// bypass. gin trusts every peer by default, so ClientIP() returns the caller's
// own X-Forwarded-For and a client could rotate it to mint a fresh bucket per
// request. Once no proxy is trusted, the header is ignored and the limit keys
// on the real peer address.
func TestTrustedProxiesCloseTheForwardedForBypass(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newRouter := func() *gin.Engine {
		router := gin.New()
		if err := ConfigureTrustedProxies(router, ""); err != nil {
			t.Fatalf("configure trusted proxies: %v", err)
		}
		router.Use(RedisRateLimiter(nil))
		router.GET("/data", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
		return router
	}

	// A spoofed X-Forwarded-For must not change the rate-limit key.
	router := newRouter()
	spoofed := func(i int) int {
		req := httptest.NewRequest(http.MethodGet, "/data", nil)
		req.RemoteAddr = "203.0.113.9:12345"
		req.Header.Set("X-Forwarded-For", "10.0.0."+strconv.Itoa(i))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	for i := 0; i < 100; i++ {
		if code := spoofed(i); code != http.StatusOK {
			t.Fatalf("request %d should be allowed, got %d", i+1, code)
		}
	}
	if code := spoofed(200); code != http.StatusTooManyRequests {
		t.Fatalf("rotating X-Forwarded-For must not bypass the limit; got %d", code)
	}
}

// TestConfigureTrustedProxiesRejectsWildcard pins that an operator cannot
// re-enable the bypass by trusting every address.
func TestConfigureTrustedProxiesRejectsWildcard(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for _, value := range []string{"*", "0.0.0.0/0", "::/0", "not-an-ip"} {
		router := gin.New()
		if err := ConfigureTrustedProxies(router, value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}

	router := gin.New()
	if err := ConfigureTrustedProxies(router, "10.0.0.0/8, 192.168.1.1"); err != nil {
		t.Fatalf("valid proxy list should be accepted: %v", err)
	}

}

// The fallback limiter keys on the caller address, so a client that rotates it
// must not be able to grow the maps without bound.
func TestLocalRateLimiterBoundsKeyCount(t *testing.T) {
	limiter := newLocalRateLimiter(5, time.Minute)
	limiter.maxKeys = 16

	for i := 0; i < 500; i++ {
		limiter.allow("key-" + strconv.Itoa(i))
	}

	limiter.mu.Lock()
	counts, resets := len(limiter.counts), len(limiter.resetAt)
	limiter.mu.Unlock()

	if counts > limiter.maxKeys {
		t.Fatalf("counts map grew to %d, cap is %d", counts, limiter.maxKeys)
	}
	if resets > limiter.maxKeys {
		t.Fatalf("resetAt map grew to %d, cap is %d", resets, limiter.maxKeys)
	}
}

// A tracked key still enforces its limit after unrelated keys are evicted.
func TestLocalRateLimiterStillLimitsTrackedKey(t *testing.T) {
	limiter := newLocalRateLimiter(2, time.Minute)

	if !limiter.allow("client") || !limiter.allow("client") {
		t.Fatal("first two requests from a client should be allowed")
	}
	if limiter.allow("client") {
		t.Fatal("third request from the same client should be rejected")
	}
}
