package security

// ============================================================================
// TigerSwap Security Platform - Rate Limiter
// Token bucket rate limiting with distributed support
// ============================================================================

import (
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// Rate Limiter Types
// ============================================================================

// LimiterConfig configures a rate limiter
type LimiterConfig struct {
	MaxTokens    int           // Maximum tokens in bucket
	RefillRate  int           // Tokens added per interval
	RefillInterval time.Duration // How often to refill
	Burst       int           // Maximum burst allowance
}

// RateLimiter implements token bucket rate limiting
type RateLimiter struct {
	config     LimiterConfig
	tokens     int
	lastFill   time.Time
	mu        sync.Mutex
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(config LimiterConfig) *RateLimiter {
	return &RateLimiter{
		config:   config,
		tokens:   config.MaxTokens,
		lastFill: time.Now(),
	}
}

// Allow checks if a request is allowed
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(rl.lastFill)
	refills := int(elapsed / rl.config.RefillInterval)
	if refills > 0 {
		rl.tokens = min(rl.config.MaxTokens, rl.tokens+refills*rl.config.RefillRate)
		rl.lastFill = now
	}

	// Check if allowed
	if rl.tokens > 0 {
		rl.tokens--
		return true
	}

	return false
}

// AllowN checks if N requests are allowed
func (rl *RateLimiter) AllowN(n int) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Refill tokens
	now := time.Now()
	elapsed := now.Sub(rl.lastFill)
	refills := int(elapsed / rl.config.RefillInterval)
	if refills > 0 {
		rl.tokens = min(rl.config.MaxTokens, rl.tokens+refills*rl.config.RefillRate)
		rl.lastFill = now
	}

	// Check if allowed
	if rl.tokens >= n {
		rl.tokens -= n
		return true
	}

	return false
}

// Tokens returns current token count
func (rl *RateLimiter) Tokens() int {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastFill)
	refills := int(elapsed / rl.config.RefillInterval)
	tokens := rl.tokens
	if refills > 0 {
		tokens = min(rl.config.MaxTokens, tokens+refills*rl.config.RefillRate)
	}

	return tokens
}

// ============================================================================
// Distributed Rate Limiter
// ============================================================================

// DistributedLimiter implements distributed rate limiting
type DistributedLimiter struct {
	mu         sync.RWMutex
	limiters   map[string]*RateLimiter
	slidingWindows map[string]*SlidingWindow
	config    LimiterConfig
}

// NewDistributedLimiter creates a distributed rate limiter
func NewDistributedLimiter(config LimiterConfig) *DistributedLimiter {
	return &DistributedLimiter{
		limiters:       make(map[string]*RateLimiter),
		slidingWindows: make(map[string]*SlidingWindow),
		config:        config,
	}
}

// GetLimiter gets or creates a limiter for a key
func (drl *DistributedLimiter) GetLimiter(key string) *RateLimiter {
	drl.mu.RLock()
	limiter, ok := drl.limiters[key]
	drl.mu.RUnlock()

	if ok {
		return limiter
	}

	drl.mu.Lock()
	defer drl.mu.Unlock()

	// Double-check
	if limiter, ok := drl.limiters[key]; ok {
		return limiter
	}

	limiter = NewRateLimiter(drl.config)
	drl.limiters[key] = limiter
	return limiter
}

// Allow checks if a request is allowed for a key
func (drl *DistributedLimiter) Allow(key string) bool {
	return drl.GetLimiter(key).Allow()
}

// ============================================================================
// Sliding Window Rate Limiter
// ============================================================================

// SlidingWindow implements sliding window rate limiting
type SlidingWindow struct {
	mu         sync.Mutex
	requests   []time.Time
	windowSize time.Duration
	maxRequests int
}

// NewSlidingWindow creates a new sliding window limiter
func NewSlidingWindow(windowSize time.Duration, maxRequests int) *Sw := &SlidingWindow{
	requests:   make([]time.Time, 0, maxRequests),
	windowSize: windowSize,
	maxRequests: maxRequests,
}

// Allow checks if request is allowed
func (sw *SlidingWindow) Allow() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-sw.windowSize)

	// Remove old requests
	newRequests := make([]time.Time, 0, sw.maxRequests)
	for _, t := range sw.requests {
		if t.After(cutoff) {
			newRequests = append(newRequests, t)
		}
	}
	sw.requests = newRequests

	// Check if allowed
	if len(sw.requests) < sw.maxRequests {
		sw.requests = append(sw.requests, now)
		return true
	}

	return false
}

// Reset clears the sliding window
func (sw *SlidingWindow) Reset() {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	sw.requests = sw.requests[:0]
}

// ============================================================================
// IP-based Rate Limiter
// ============================================================================

// IPLimiter limits requests by IP address
type IPLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*RateLimiter
	config  LimiterConfig
}

// NewIPLimiter creates a new IP-based rate limiter
func NewIPLimiter(config LimiterConfig) *IPLimiter {
	return &IPLimiter{
		limiters: make(map[string]*RateLimiter),
		config:  config,
	}
}

// Allow checks if request from IP is allowed
func (ipl *IPLimiter) Allow(ip string) bool {
	ipl.mu.RLock()
	limiter, ok := ipl.limiters[ip]
	ipl.mu.RUnlock()

	if ok {
		return limiter.Allow()
	}

	ipl.mu.Lock()
	defer ipl.mu.Unlock()

	// Double-check
	if limiter, ok := ipl.limiters[ip]; ok {
		return limiter.Allow()
	}

	limiter = NewRateLimiter(ipl.config)
	ipl.limiters[ip] = limiter
	return limiter.Allow()
}

// ============================================================================
// API Key Rate Limiter
// ============================================================================

// APIKeyLimiter limits requests by API key
type APIKeyLimiter struct {
	mu         sync.RWMutex
	limiters   map[string]*RateLimiter
	tierLimits map[string]LimiterConfig // Different limits per tier
}

// NewAPIKeyLimiter creates a new API key rate limiter
func NewAPIKeyLimiter() *APIKeyLimiter {
	return &APIKeyLimiter{
		limiters: make(map[string]*RateLimiter),
		tierLimits: map[string]LimiterConfig{
			"free":   {MaxTokens: 60, RefillRate: 1, RefillInterval: time.Minute, Burst: 10},
			"basic":  {MaxTokens: 300, RefillRate: 5, RefillInterval: time.Minute, Burst: 50},
			"pro":    {MaxTokens: 3000, RefillRate: 50, RefillInterval: time.Minute, Burst: 500},
			"enterprise": {MaxTokens: 30000, RefillRate: 500, RefillInterval: time.Minute, Burst: 5000},
		},
	}
}

// Allow checks if request from API key is allowed
func (akl *APIKeyLimiter) Allow(apiKey, tier string) bool {
	akl.mu.RLock()
	limiter, ok := akl.limiters[apiKey]
	akl.mu.RUnlock()

	if ok {
		return limiter.Allow()
	}

	akl.mu.Lock()
	defer akl.mu.Unlock()

	// Double-check
	if limiter, ok := akl.limiters[apiKey]; ok {
		return limiter.Allow()
	}

	// Get tier config
	config, ok := akl.tierLimits[tier]
	if !ok {
		config = akl.tierLimits["free"]
	}

	limiter = NewRateLimiter(config)
	akl.limiters[apiKey] = limiter
	return limiter.Allow()
}

// ============================================================================
// Helper
// ============================================================================

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ============================================================================
// Export
// ============================================================================

type RateLimitService struct {
	ipLimiter   *IPLimiter
	apiLimiter *APIKeyLimiter
	distLimiter *DistributedLimiter
}

func NewRateLimitService() *RateLimitService {
	return &RateLimitService{
		ipLimiter:   NewIPLimiter(LimiterConfig{MaxTokens: 100, RefillRate: 10, RefillInterval: time.Second, Burst: 20}),
		apiLimiter: NewAPIKeyLimiter(),
		distLimiter: NewDistributedLimiter(LimiterConfig{MaxTokens: 1000, RefillRate: 100, RefillInterval: time.Minute, Burst: 200}),
	}
}

func (rrs *RateLimitService) AllowIP(ip string) bool {
	return rrs.ipLimiter.Allow(ip)
}

func (rrs *RateLimitService) AllowAPIKey(apiKey, tier string) bool {
	return rrs.apiLimiter.Allow(apiKey, tier)
}

func (rrs *RateLimitService) AllowKey(key string) bool {
	return rrs.distLimiter.Allow(key)
}