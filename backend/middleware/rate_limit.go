package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	requests map[string]*bucket
	mu       sync.RWMutex
	rate     int           // requests per window
	window   time.Duration // time window
}

type bucket struct {
	tokens    int
	lastFill  time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requests int, window time.Duration) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*bucket),
		rate:     requests,
		window:   window,
	}
	
	// Cleanup old entries periodically
	go rl.cleanup()
	
	return rl
}

// cleanup removes old entries periodically
func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, b := range rl.requests {
			if now.Sub(b.lastFill) > rl.window*2 {
				delete(rl.requests, key)
			}
		}
		rl.mu.Unlock()
	}
}

// allow checks if request is allowed
func (rl *RateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	b, exists := rl.requests[key]
	
	if !exists || now.Sub(b.lastFill) > rl.window {
		// New window - reset tokens
		rl.requests[key] = &bucket{
			tokens:   rl.rate - 1,
			lastFill: now,
		}
		return true
	}
	
	if b.tokens > 0 {
		b.tokens--
		return true
	}
	
	return false
}

// RateLimitMiddleware returns Gin middleware for rate limiting
func RateLimitMiddleware(requests int, window time.Duration) gin.HandlerFunc {
	limiter := NewRateLimiter(requests, window)
	
	return func(c *gin.Context) {
		// Get client identifier (IP or user ID)
		key := c.ClientIP()
		if userID, exists := c.Get("user_id"); exists {
			key = userID.(string)
		}
		
		if !limiter.allow(key) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Please try again later.",
				"retry_after": window.Seconds(),
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}

// IPBanList tracks banned IPs
type IPBanList struct {
	banned   map[string]time.Time
	mu       sync.RWMutex
	duration time.Duration
}

var banList = &IPBanList{
	banned:   make(map[string]time.Time),
	duration: time.Hour * 24, // 24 hour ban
}

// BanIP bans an IP address
func BanIP(ip string) {
	banList.mu.Lock()
	defer banList.mu.Unlock()
	banList.banned[ip] = time.Now()
}

// IsBanned checks if IP is banned
func IsBanned(ip string) bool {
	banList.mu.RLock()
	defer banList.mu.RUnlock()
	
	if banTime, exists := banList.banned[ip]; exists {
		if time.Since(banTime) < banList.duration {
			return true
		}
		// Ban expired
		delete(banList.banned, ip)
	}
	return false
}

// BanCheckMiddleware checks if IP is banned
func BanCheckMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		
		if IsBanned(ip) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Your IP has been banned",
			})
			c.Abort()
			return
		}
		
		c.Next()
	}
}
