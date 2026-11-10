// Package middleware provides HTTP middleware for the API Gateway
package middleware

import (
	"compress/gzip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
	"tigerswap/backend/go/api_gateway/config"
)

var (
	requestIDKey = "request_id"
	startTimeKey = "start_time"
)

// Logger returns a gin middleware for logging
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		c.Set(startTimeKey, startTime)

		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(startTime)
		statusCode := c.Writer.Status()

		if query != "" {
			path = path + "?" + query
		}

		log.Printf("[%d] %s %s %s %v",
			statusCode,
			c.Request.Method,
			path,
			c.ClientIP(),
			latency,
		)
	}
}

// RequestID returns a gin middleware for adding request ID
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = generateRequestID()
		}
		c.Set(requestIDKey, requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// GetRequestID retrieves the request ID from context
func GetRequestID(c *gin.Context) string {
	if id, exists := c.Get(requestIDKey); exists {
		return id.(string)
	}
	return ""
}

// Timeout bounds how long a request may take.
//
// The deadline is attached to the request context so every downstream call the
// handler makes (all of them derive from `c.Request.Context()`) is cancelled
// when it elapses. The handler runs on this goroutine, so its response is never
// written concurrently with the timeout response; the 408 is only emitted when
// the deadline expired and the handler produced no response of its own.
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)
		c.Next()

		if ctx.Err() == context.DeadlineExceeded && !c.Writer.Written() {
			c.AbortWithStatusJSON(http.StatusRequestTimeout, gin.H{
				"error":      "Request timeout",
				"request_id": GetRequestID(c),
			})
		}
	}
}

// RateLimiter handles rate limiting
type RateLimiter struct {
	redis           *redis.Client
	config          *config.RateLimitConfig
	windowSize      time.Duration
	cleanupInterval time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(redisClient *redis.Client, cfg config.RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		redis:           redisClient,
		config:          &cfg,
		windowSize:      time.Second,
		cleanupInterval: 5 * time.Minute,
	}
}

// Middleware returns the rate limiting middleware
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	if !rl.config.Enabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	return func(c *gin.Context) {
		if rl.redis == nil {
			c.Next()
			return
		}

		clientIP := c.ClientIP()
		key := fmt.Sprintf("ratelimit:%s", clientIP)

		ctx := context.Background()

		// Increment counter
		count, err := rl.redis.Incr(ctx, key).Result()
		if err != nil {
			log.Printf("Rate limiter error: %v", err)
			c.Next()
			return
		}

		// Set expiry on first request
		if count == 1 {
			rl.redis.Expire(ctx, key, rl.windowSize)
		}

		// Check if over limit
		if int(count) > rl.config.RequestsPerSecond {
			retryAfter := rl.windowSize.Seconds()
			c.Header("Retry-After", fmt.Sprintf("%.0f", retryAfter))
			c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.config.RequestsPerSecond))
			c.Header("X-RateLimit-Remaining", "0")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error":       "Rate limit exceeded",
				"retry_after": retryAfter,
				"limit":       rl.config.RequestsPerSecond,
				"request_id":  GetRequestID(c),
			})
			return
		}

		// Set rate limit headers
		remaining := rl.config.RequestsPerSecond - int(count)
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.config.RequestsPerSecond))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))

		c.Next()
	}
}

// Auth validates the bearer JWT and injects the caller identity into context.
func Auth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader || token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization format"})
			return
		}

		claims, err := parseJWT(token, cfg.JWT.Secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":      "Invalid or expired token",
				"request_id": GetRequestID(c),
			})
			return
		}

		// The identity lives in the `user_id` claim: that is what the auth
		// service mints and what the REST backend reads. `sub` is only a
		// fallback for a token issued by another component. Without the
		// user_id claim the account-scoped handlers see an empty id and reject
		// every request, so both claims are read here.
		userID := claims.UserID
		if userID == "" {
			userID = claims.Subject
		}
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":      "Invalid or expired token",
				"request_id": GetRequestID(c),
			})
			return
		}

		c.Set("user_id", userID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// jwtClaims is the subset of JWT claims the gateway trusts.
type jwtClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

// parseJWT verifies the signature and expiry of an HS256 token.
func parseJWT(token, secret string) (*jwtClaims, error) {
	if secret == "" {
		return nil, fmt.Errorf("jwt secret is not configured")
	}
	claims := &jwtClaims{}
	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("token is not valid")
	}
	return claims, nil
}

// AdminRequired admits a caller holding either the `admin` or `superadmin`
// role. A valid token is not enough: without this check any authenticated user
// could reach the admin handlers, which is why the admin route group composes
// Auth and AdminRequired.
func AdminRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, exists := c.Get("role")
		if !exists {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error":      "Unauthorized - no role found",
				"request_id": GetRequestID(c),
			})
			return
		}

		roleStr, ok := role.(string)
		if !ok || (roleStr != "admin" && roleStr != "superadmin") {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":      "Forbidden - admin access required",
				"request_id": GetRequestID(c),
			})
			return
		}

		c.Next()
	}
}

// Recovery returns a gin middleware for panic recovery
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("Panic recovered: %v", err)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"error":      "Internal server error",
					"request_id": GetRequestID(c),
				})
			}
		}()
		c.Next()
	}
}

// CORS returns a gin middleware for CORS
func CORS(allowOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Check if origin is allowed
		allowed := false
		for _, ao := range allowOrigins {
			if ao == "*" || ao == origin {
				allowed = true
				break
			}
		}

		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID")
			c.Header("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// Compression gzips responses when the client advertises support.
func Compression() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
			c.Next()
			return
		}

		gz := gzip.NewWriter(c.Writer)
		defer gz.Close()

		c.Writer = &gzipWriter{ResponseWriter: c.Writer, gz: gz}
		c.Header("Content-Encoding", "gzip")
		c.Header("Vary", "Accept-Encoding")
		c.Next()
	}
}

// gzipWriter wraps the gin ResponseWriter with a gzip stream.
type gzipWriter struct {
	gin.ResponseWriter
	gz *gzip.Writer
}

func (g *gzipWriter) Write(b []byte) (int, error) { return g.gz.Write(b) }

func (g *gzipWriter) WriteString(s string) (int, error) { return g.gz.Write([]byte(s)) }

// Metrics returns a gin middleware that records request counters and latency.
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		c.Next()

		duration := time.Since(startTime)
		statusCode := c.Writer.Status()

		metrics.mu.Lock()
		metrics.requestsTotal++
		metrics.requestsByStatus[strconv.Itoa(statusCode)]++
		metrics.requestDurationSum += duration.Seconds()
		metrics.requestDurationCount++
		if metrics.requestDurationMax < duration.Seconds() {
			metrics.requestDurationMax = duration.Seconds()
		}
		metrics.mu.Unlock()

		log.Printf("metrics: method=%s path=%s status=%d duration=%v",
			c.Request.Method, c.Request.URL.Path, statusCode, duration)
	}
}

// metricsRegistry is an in-process Prometheus-compatible metrics registry.
type metricsRegistry struct {
	mu                   sync.Mutex
	requestsTotal        uint64
	requestsByStatus     map[string]uint64
	requestDurationSum   float64
	requestDurationCount uint64
	requestDurationMax   float64
}

var metrics = &metricsRegistry{requestsByStatus: make(map[string]uint64)}

// MetricsHandler exposes the registry in Prometheus text exposition format.
func MetricsHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		metrics.mu.Lock()
		defer metrics.mu.Unlock()

		var b strings.Builder
		b.WriteString("# HELP tigerswap_gateway_requests_total Total HTTP requests handled.\n")
		b.WriteString("# TYPE tigerswap_gateway_requests_total counter\n")
		fmt.Fprintf(&b, "tigerswap_gateway_requests_total %d\n", metrics.requestsTotal)

		b.WriteString("# HELP tigerswap_gateway_responses_total HTTP responses by status code.\n")
		b.WriteString("# TYPE tigerswap_gateway_responses_total counter\n")
		for status, count := range metrics.requestsByStatus {
			fmt.Fprintf(&b, "tigerswap_gateway_responses_total{status=\"%s\"} %d\n", status, count)
		}

		b.WriteString("# HELP tigerswap_gateway_request_duration_seconds Request latency.\n")
		b.WriteString("# TYPE tigerswap_gateway_request_duration_seconds summary\n")
		fmt.Fprintf(&b, "tigerswap_gateway_request_duration_seconds_sum %f\n", metrics.requestDurationSum)
		fmt.Fprintf(&b, "tigerswap_gateway_request_duration_seconds_count %d\n", metrics.requestDurationCount)
		fmt.Fprintf(&b, "tigerswap_gateway_request_duration_seconds_max %f\n", metrics.requestDurationMax)

		c.Data(http.StatusOK, "text/plain; version=0.0.4", []byte(b.String()))
	}
}

// ValidateRequest returns a middleware for request validation
func ValidateRequest() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Validate content type for POST/PUT
		if c.Request.Method == "POST" || c.Request.Method == "PUT" {
			contentType := c.GetHeader("Content-Type")
			if contentType != "" && !strings.Contains(contentType, "application/json") {
				c.AbortWithStatusJSON(http.StatusUnsupportedMediaType, gin.H{
					"error": "Content-Type must be application/json",
				})
				return
			}
		}

		// Validate request body size
		if c.Request.ContentLength > 10*1024*1024 { // 10MB
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": "Request body too large",
			})
			return
		}

		c.Next()
	}
}

// BodyLimiter returns a middleware for limiting request body size
func BodyLimiter(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxSize {
			c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{
				"error":      "Request body too large",
				"max_size":   maxSize,
				"request_id": GetRequestID(c),
			})
			return
		}

		// Limit reader
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)

		c.Next()
	}
}

// Secure returns security-related middleware
// Secure sets the browser-facing hardening headers on every gateway response.
//
// The gateway returns JSON only, so the Content-Security-Policy can be the
// strictest one there is: no resource of any kind may be loaded. A permissive
// policy with 'unsafe-inline' and 'unsafe-eval' would only matter if a response
// were ever rendered as a document, and the point of the header is to make that
// harmless if it happens. The legacy X-XSS-Protection header is deliberately
// omitted: it is deprecated, and in some browsers a value of 1 enables a filter
// that is itself exploitable, so its absence is the safe choice.
func Secure() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Header("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
		c.Header("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
		c.Header("Cross-Origin-Opener-Policy", "same-origin")
		c.Header("Cross-Origin-Resource-Policy", "same-origin")

		// Every gateway response carries account-scoped data, so it must not be
		// retained by a shared or browser cache and replayed to another account.
		// Nothing here is a static asset that benefits from caching.
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private")
		c.Header("Pragma", "no-cache")

		c.Next()
	}
}

// ============ Helper Functions ============

func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return fmt.Sprintf("%d-%s", time.Now().UnixNano(), "fallback")
	}
	return hex.EncodeToString(b)
}
