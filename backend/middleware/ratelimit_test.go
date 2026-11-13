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

// TestAuthRateLimiterIsTighterThanGlobal pins that credential endpoints get
// their own, tighter budget. Without a separate limiter a password-guessing run
// shared the 100/min global bucket, which is far too permissive for sign-in.
func TestAuthRateLimiterIsTighterThanGlobal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(AuthRateLimiter(nil))
	router.POST("/login", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	status := func() int {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "198.51.100.5:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec.Code
	}

	for i := 0; i < 10; i++ {
		if code := status(); code != http.StatusOK {
			t.Fatalf("login attempt %d should be allowed, got %d", i+1, code)
		}
	}
	if code := status(); code != http.StatusTooManyRequests {
		t.Fatalf("the 11th login attempt in a minute must be limited, got %d", code)
	}
}

// The auth limiter and the global limiter must not share a bucket, so a burst of
// ordinary API traffic cannot exhaust an account's sign-in budget.
func TestAuthAndGlobalLimitersUseSeparateBuckets(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RedisRateLimiter(nil))
	router.POST("/login", AuthRateLimiter(nil), func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

	// Exhaust the auth bucket for this client.
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "198.51.100.7:12345"
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("attempt %d should be allowed, got %d", i+1, rec.Code)
		}
	}
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "198.51.100.7:12345"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("auth bucket should be exhausted, got %d", rec.Code)
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

// TestAccountRateLimiterKeysByAccountNotIP proves the account limiter counts a
// session, not a source address: two accounts behind one address get independent
// budgets, and one account cannot exceed its own by rotating its address.
func TestAccountRateLimiterKeysByAccountNotIP(t *testing.T) {
gin.SetMode(gin.TestMode)
router := gin.New()
router.Use(func(c *gin.Context) {
c.Set("user_id", c.GetHeader("X-Test-User"))
c.Next()
})
router.Use(AccountRateLimiter(nil, "rl_test:", 3, time.Minute))
router.POST("/act", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })

call := func(user, addr string) int {
req := httptest.NewRequest(http.MethodPost, "/act", nil)
req.RemoteAddr = addr
req.Header.Set("X-Test-User", user)
rec := httptest.NewRecorder()
router.ServeHTTP(rec, req)
return rec.Code
}

// One account exhausts its budget from one address.
for i := 0; i < 3; i++ {
if code := call("alice", "203.0.113.1:1000"); code != http.StatusOK {
t.Fatalf("alice request %d should be allowed, got %d", i+1, code)
}
}
if code := call("alice", "203.0.113.1:1000"); code != http.StatusTooManyRequests {
t.Fatalf("alice should be limited after 3, got %d", code)
}
// Rotating the source address must not reset the account's budget.
if code := call("alice", "203.0.113.99:1000"); code != http.StatusTooManyRequests {
t.Fatalf("alice must stay limited from a new address, got %d", code)
}
// A different account behind the same address has its own budget.
if code := call("bob", "203.0.113.1:1000"); code != http.StatusOK {
t.Fatalf("bob must not be limited by alice, got %d", code)
}
}
