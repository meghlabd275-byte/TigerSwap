package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"tigerswap/backend/services"
)

// APIKeyRequired authenticates a request with a developer API key and sets the
// same context keys the session middleware sets (`user_id`, `api_key_scopes`),
// so a handler that reads the caller works unchanged behind either scheme.
//
// The key is read from `X-API-Key`, or `Authorization: Bearer <key>`. A request
// without a valid, unexpired key is rejected.
func APIKeyRequired(apiKeys *services.APIKeyService) gin.HandlerFunc {
	return func(c *gin.Context) {
		presented := ParseAPIKey(
			c.GetHeader("X-API-Key"),
			c.GetHeader("Authorization"),
			"",
		)
		if presented == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "API key required"})
			return
		}

		key, err := apiKeys.Authenticate(presented)
		if err != nil {
			// Every failure is reported the same way so a probe cannot tell an
			// unknown key from a revoked or expired one.
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}

		c.Set("user_id", key.UserID)
		c.Set("api_key_id", key.ID.String())
		c.Set("api_key_scopes", key.Scopes)
		c.Next()
	}
}

// RequireAPIKeyScope refuses a request whose key does not hold the scope.
func RequireAPIKeyScope(scope string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := c.GetString("api_key_scopes")
		allowed := false
		for _, held := range strings.Split(raw, ",") {
			held = strings.TrimSpace(held)
			if held == "*" || held == scope {
				allowed = true
				break
			}
		}
		if !allowed {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient scope"})
			return
		}
		c.Next()
	}
}

// APIKeyRateLimiter bounds each key to its own per-minute budget. A key with no
// budget falls back to the supplied default. The limiter is keyed by the key id,
// so one key cannot exhaust another's budget, and it is process-local: behind
// several instances each enforces its own share, which is acceptable because the
// global per-IP limiter still bounds the aggregate.
func APIKeyRateLimiter(apiKeys *services.APIKeyService, defaultPerMinute int) gin.HandlerFunc {
	type bucket struct {
		count   int
		resetAt time.Time
	}
	var mu sync.Mutex
	buckets := map[string]*bucket{}

	return func(c *gin.Context) {
		id := c.GetString("api_key_id")
		if id == "" {
			// Not an API-key request; nothing to bound here.
			c.Next()
			return
		}

		limit := defaultPerMinute
		if key, err := apiKeys.ByID(id); err == nil && key.RateLimit > 0 {
			limit = key.RateLimit
		}
		if limit <= 0 {
			c.Next()
			return
		}

		now := time.Now()
		mu.Lock()
		b, ok := buckets[id]
		if !ok || now.After(b.resetAt) {
			b = &bucket{count: 0, resetAt: now.Add(time.Minute)}
			buckets[id] = b
		}
		b.count++
		over := b.count > limit
		// Opportunistic sweep so the map cannot grow without bound.
		if len(buckets) > 4096 {
			for k, v := range buckets {
				if now.After(v.resetAt) {
					delete(buckets, k)
				}
			}
		}
		mu.Unlock()

		if over {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "api key rate limit exceeded"})
			return
		}
		c.Next()
	}
}
