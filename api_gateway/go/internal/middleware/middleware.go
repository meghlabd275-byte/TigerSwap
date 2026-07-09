package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/spf13/viper"
	"github.com/google/uuid"
)

// RequestID adds a unique request ID to each request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Set("request_id", requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

// CORS handles Cross-Origin Resource Sharing
func CORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-ID")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// RateLimiter implements simple rate limiting
func RateLimiter() gin.HandlerFunc {
	// In production, use Redis for distributed rate limiting
	type client struct {
		requests int
		reset    time.Time
	}
	clients := make(map[string]*client)

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		if _, exists := clients[ip]; !exists {
			clients[ip] = &client{requests: 0, reset: now.Add(time.Minute)}
		}

		client := clients[ip]
		if now.After(client.reset) {
			client.requests = 0
			client.reset = now.Add(time.Minute)
		}

		client.requests++

		// Limit: 100 requests per minute
		if client.requests > 100 {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
				"retry_after": client.reset.Unix() - now.Unix(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// AuthRequired validates JWT token
func AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Authorization header required",
			})
			c.Abort()
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid authorization header format",
			})
			c.Abort()
			return
		}

		tokenString := parts[1]

		// Parse and validate token
		claims := &jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(viper.GetString("jwt.secret")), nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		// Check token type
		tokenType := (*claims)["type"].(string)
		if tokenType != "access" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid token type",
			})
			c.Abort()
			return
		}

		// Set user ID in context
		userID := (*claims)["sub"].(string)
		c.Set("user_id", userID)

		c.Next()
	}
}

// Logger logs requests
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		requestID, _ := c.Get("request_id")

		if status >= 400 {
			// Log errors
			gin.DefaultWriter.Write([]byte(
				time.Now().Format("2006/01/02 - 15:04:05") +
					" | " + requestID.(string) +
					" | " + method +
					" | " + path +
					" | " + latency.String() +
					" | " + http.StatusText(status) + "\n",
			))
		}
	}
}
