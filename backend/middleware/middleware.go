package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"tigerswap/backend/internal/netutil"
	"tigerswap/backend/models"
	"tigerswap/backend/services"
)

// Platform role names, re-exported from the model layer so a middleware check
// and the value stored on the User row cannot drift apart.
const (
	RoleUser       = models.RoleUser
	RoleAdmin      = models.RoleAdmin
	RoleSuperAdmin = models.RoleSuperAdmin
)

// hasRole reports whether the token claims carry one of the supplied roles.
func hasRole(claims jwt.MapClaims, roles ...string) bool {
	current, ok := claims["role"].(string)
	if !ok {
		return false
	}
	for _, role := range roles {
		if current == role {
			return true
		}
	}
	return false
}

// authenticate validates the bearer token and returns its claims. On failure it
// writes the 401 response and aborts the context, and the caller must return.
func authenticate(c *gin.Context, authService *services.AuthService) (jwt.MapClaims, bool) {
	authHeader := c.GetHeader("Authorization")
	if authHeader == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
		return nil, false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
		return nil, false
	}

	claims, err := authService.ValidateToken(parts[1])
	if err != nil {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return nil, false
	}
	return claims, true
}

// setAuthContext copies the validated identity into the request context.
func setAuthContext(c *gin.Context, claims jwt.MapClaims) {
	c.Set("user_id", claims["user_id"])
	c.Set("email", claims["email"])
	c.Set("role", claims["role"])
}

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()

		if statusCode >= 400 {
			// Log errors
			println("[ERROR]", method, path, statusCode, latency)
		} else {
			println("[INFO]", method, path, statusCode, latency)
		}
	}
}

// BodySizeLimit caps the size of a request body. A handler that decodes JSON
// from an unbounded reader can be driven out of memory by a single client, so
// every API request is bounded before it reaches a handler. A declared
// Content-Length over the limit is rejected up front; a chunked body is bounded
// by the reader so a lying or absent Content-Length cannot bypass the cap.
func BodySizeLimit(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes > 0 {
			if c.Request.ContentLength > maxBytes {
				c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
					"error":    "request body too large",
					"max_size": maxBytes,
				})
				return
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// SecurityHeaders sets the browser-facing hardening headers on every response.
//
// This process serves JSON only: it renders no HTML and hosts no script. The
// policy is therefore the strictest one that cannot break a client -- nothing
// may be loaded at all -- rather than a permissive policy with 'unsafe-inline'
// and 'unsafe-eval', which would matter if a response body were ever rendered
// as a document. If an HTML surface is ever added here, it must ship its own
// nonce-based policy instead of relaxing this one.
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		c.Header("Referrer-Policy", "no-referrer")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
		c.Header("Cross-Origin-Resource-Policy", "same-origin")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		// Every response carries account-scoped data (balances, positions, keys),
		// so it must not be retained by a shared or browser cache and replayed to
		// another account. Nothing here is a static asset that benefits from caching.
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
		c.Header("Pragma", "no-cache")
		c.Next()
	}
}

// localRateLimiter is a fixed-window per-key limiter used as a fallback when the
// distributed limiter (Redis) is unreachable. It keeps the API from becoming an
// open door during a Redis outage, at the cost of being per-process.
//
// The key is derived from the caller's address, so a client that rotates it
// could otherwise grow the map without bound; the map is capped and expired
// windows are purged so a Redis outage cannot be turned into memory exhaustion.
type localRateLimiter struct {
	mu        sync.Mutex
	counts    map[string]int
	resetAt   map[string]time.Time
	limit     int
	window    time.Duration
	maxKeys   int
	lastPurge time.Time
}

const defaultMaxLocalRateLimitKeys = 100_000

func newLocalRateLimiter(limit int, window time.Duration) *localRateLimiter {
	return &localRateLimiter{
		counts:  make(map[string]int),
		resetAt: make(map[string]time.Time),
		limit:   limit,
		window:  window,
		maxKeys: defaultMaxLocalRateLimitKeys,
	}
}

func (l *localRateLimiter) allow(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	// Purge drained windows once per window so the map holds live keys only.
	if now.Sub(l.lastPurge) >= l.window {
		for existing, expiry := range l.resetAt {
			if now.After(expiry) {
				delete(l.counts, existing)
				delete(l.resetAt, existing)
			}
		}
		l.lastPurge = now
	}

	expiry, tracked := l.resetAt[key]
	if !tracked || now.After(expiry) {
		if !tracked && len(l.counts) >= l.maxKeys {
			l.evictNearestResetLocked()
		}
		l.counts[key] = 0
		l.resetAt[key] = now.Add(l.window)
	}
	l.counts[key]++
	return l.counts[key] <= l.limit
}

// evictNearestResetLocked drops the key whose window resets soonest, which is
// the entry closest to leaving on its own. The caller holds the lock.
func (l *localRateLimiter) evictNearestResetLocked() {
	var victim string
	var victimExpiry time.Time
	for key, expiry := range l.resetAt {
		if victim == "" || expiry.Before(victimExpiry) {
			victim, victimExpiry = key, expiry
		}
	}
	if victim != "" {
		delete(l.counts, victim)
		delete(l.resetAt, victim)
	}
}

// RedisRateLimiter applies a distributed rate limit backed by Redis. When Redis
// is unreachable it falls back to an in-process limiter rather than allowing
// every request through, so a Redis outage cannot be used to bypass the limit.
func RedisRateLimiter(redisClient *redis.Client) gin.HandlerFunc {
	const (
		limit  = 100 // requests per minute per client
		window = time.Minute
	)
	fallback := newLocalRateLimiter(limit, window)

	return func(c *gin.Context) {
		// Skip rate limiting for health check
		if c.Request.URL.Path == "/health" {
			c.Next()
			return
		}

		ip := c.ClientIP()
		key := "rate_limit:" + ip

		if redisClient != nil {
			count, err := redisClient.Incr(c.Request.Context(), key).Result()
			if err == nil {
				if count == 1 {
					redisClient.Expire(c.Request.Context(), key, window)
				}
				if count > int64(limit) {
					c.JSON(http.StatusTooManyRequests, gin.H{
						"error":       "Rate limit exceeded",
						"retry_after": int(window.Seconds()),
					})
					c.Abort()
					return
				}
				c.Next()
				return
			}
			// Redis is unreachable; fall through to the local limiter.
		}

		if !fallback.allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": int(window.Seconds()),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

func AuthRequired(authService *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := authenticate(c, authService)
		if !ok {
			return
		}
		setAuthContext(c, claims)
		c.Next()
	}
}

// AdminRequired admits an operator holding either the `admin` or `superadmin`
// role. A superadmin is the most privileged account and must not be locked out
// of the endpoints an ordinary admin can reach.
func AdminRequired(authService *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := authenticate(c, authService)
		if !ok {
			return
		}
		if !hasRole(claims, RoleAdmin, RoleSuperAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Admin access required"})
			return
		}
		setAuthContext(c, claims)
		c.Next()
	}
}

// ConfigureTrustedProxies tells gin which peers may set the client IP through a
// forwarding header. It delegates to internal/netutil so every service shares
// one implementation; see that package for the rationale.
func ConfigureTrustedProxies(router *gin.Engine, raw string) error {
	return netutil.ConfigureTrustedProxies(router, raw)
}
