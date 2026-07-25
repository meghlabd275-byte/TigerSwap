// Package middleware provides HTTP middleware for the API Gateway
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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

// Timeout returns a gin middleware for request timeout
func Timeout(timeout time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), timeout)
		defer cancel()

		c.Request = c.Request.WithContext(ctx)

		<-ctx.Done()
		if ctx.Err() == context.DeadlineExceeded {
			c.AbortWithStatusJSON(http.StatusRequestTimeout, gin.H{
				"error":       "Request timeout",
				"request_id":  GetRequestID(c),
			})
			return
		}
		c.Next()
	}
}

// RateLimiter handles rate limiting
type RateLimiter struct {
	redis        *redis.Client
	config       *config.RateLimitConfig
	windowSize   time.Duration
	cleanupInterval time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(redisClient *redis.Client, cfg config.RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		redis:          redisClient,
		config:         &cfg,
		windowSize:     time.Second,
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
				"error":          "Rate limit exceeded",
				"retry_after":    retryAfter,
				"limit":          rl.config.RequestsPerSecond,
				"request_id":     GetRequestID(c),
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

// Auth returns authentication middleware
func Auth(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if token == authHeader {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization format",
			})
			return
		}

		// In production, validate JWT token
		// For now, accept any token for development
		c.Set("token", token)

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
					"error":       "Internal server error",
					"request_id":  GetRequestID(c),
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

// Compression returns a gin middleware for response compression
func Compression() gin.HandlerFunc {
	return func(c *gin.Context) {
		acceptEncoding := c.GetHeader("Accept-Encoding")
		
		if strings.Contains(acceptEncoding, "gzip") {
			c.Header("Content-Encoding", "gzip")
			// In production, use gzip writer wrapper
		}

		c.Next()
	}
}

// Metrics returns a gin middleware for collecting metrics
func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		c.Next()

		duration := time.Since(startTime)
		statusCode := c.Writer.Status()

		// In production, send to Prometheus
		log.Printf("metrics: method=%s path=%s status=%d duration=%v",
			c.Request.Method,
			c.Request.URL.Path,
			statusCode,
			duration,
		)
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
				"error":       "Request body too large",
				"max_size":    maxSize,
				"request_id":  GetRequestID(c),
			})
			return
		}

		// Limit reader
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)

		c.Next()
	}
}

// Secure returns security-related middleware
func Secure() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")
		c.Header("X-XSS-Protection", "1; mode=block")
		c.Header("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")

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
