package security

// ============================================================================
// TigerSwap Security Platform - Circuit Breaker
// Prevents cascading failures in distributed systems
// ============================================================================

import (
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// Circuit Breaker States
// ============================================================================

// State represents the circuit breaker state
type State int

const (
	StateClosed State = iota // Normal operation
	StateOpen           // Failing, reject requests
	StateHalfOpen      // Testing if service recovered
)

// ============================================================================
// Circuit Breaker
// ============================================================================

// Config configures a circuit breaker
type CircuitBreakerConfig struct {
	FailureThreshold  int           // Failures before opening
	SuccessThreshold int           // Successes before closing
	Timeout         time.Duration // Time in open state before half-open
	MaxRequests     int           // Max requests in half-open state
}

// CircuitBreaker implements circuit breaker pattern
type CircuitBreaker struct {
	config     CircuitBreakerConfig
	state      State
	failures   int
	successes  int
	lastFailure time.Time
	mu        sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		config:     config,
		state:      StateClosed,
		failures:   0,
		successes:  0,
		lastFailure: time.Time{},
	}
}

// Execute runs a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if !cb.Allow() {
		return fmt.Errorf("circuit breaker open")
	}

	err := fn()
	cb.RecordResult(err)
	return err
}

// Allow checks if request is allowed
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true

	case StateOpen:
		// Check if timeout elapsed
		if time.Since(cb.lastFailure) > cb.config.Timeout {
			cb.state = StateHalfOpen
			cb.successes = 0
			return true
		}
		return false

	case StateHalfOpen:
		return true

	default:
		return false
	}
}

// RecordResult records the result of a request
func (cb *CircuitBreaker) RecordResult(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if err != nil {
		// Failure
		cb.failures++
		cb.lastFailure = time.Now()

		if cb.failures >= cb.config.FailureThreshold {
			cb.state = StateOpen
		}
	} else {
		// Success
		cb.successes++

		if cb.state == StateHalfOpen && cb.successes >= cb.config.SuccessThreshold {
			cb.state = StateClosed
			cb.failures = 0
		}
	}
}

// State returns the current state
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Failures returns the failure count
func (cb *CircuitBreaker) Failures() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// ============================================================================
// Distributed Circuit Breaker
// ============================================================================

// DistributedCircuitBreaker implements distributed circuit breaking
type DistributedCircuitBreaker struct {
	mu           sync.RWMutex
	breakers    map[string]*CircuitBreaker
	config      CircuitBreakerConfig
}

// NewDistributedCircuitBreaker creates a distributed circuit breaker
func NewDistributedCircuitBreaker(config CircuitBreakerConfig) *DistributedCircuitBreaker {
	return &DistributedCircuitBreaker{
		breakers: make(map[string]*CircuitBreaker),
		config:  config,
	}
}

// GetBreaker gets or creates a breaker for a service
func (dcb *DistributedCircuitBreaker) GetBreaker(service string) *CircuitBreaker {
	dcb.mu.RLock()
	breaker, ok := dcb.breakers[service]
	dcb.mu.RUnlock()

	if ok {
		return breaker
	}

	dcb.mu.Lock()
	defer dcb.mu.Unlock()

	// Double-check
	if breaker, ok := dcb.breakers[service]; ok {
		return breaker
	}

	breaker = NewCircuitBreaker(dcb.config)
	dcb.breakers[service] = breaker
	return breaker
}

// Execute runs a function with circuit breaker protection
func (dcb *DistributedCircuitBreaker) Execute(service string, fn func() error) error {
	return dcb.GetBreaker(service).Execute(fn)
}

// ============================================================================
// Health Check
// ============================================================================

// HealthChecker checks service health
type HealthChecker struct {
	mu       sync.RWMutex
	checks  map[string]*ServiceHealth
}

// ServiceHealth represents service health status
type ServiceHealth struct {
	Service      string    `json:"service"`
	Status      string    `json:"status"` // "healthy", "degraded", "unhealthy"
	Latency     float64   `json:"latency"`
	SuccessRate float64   `json:"successRate"`
	Requests    int      `json:"requests"`
	Failures    int      `json:"failures"`
	LastCheck   time.Time `json:"lastCheck"`
}

// NewHealthChecker creates a new health checker
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks: make(map[string]*ServiceHealth),
	}
}

// RecordSuccess records a successful request
func (hc *HealthChecker) RecordSuccess(service string, latency time.Duration) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	health, ok := hc.checks[service]
	if !ok {
		health = &ServiceHealth{Service: service}
		hc.checks[service] = health
	}

	health.Requests++
	health.Latency = (health.Latency*float64(health.Requests-1) + float64(latency)) / float64(health.Requests)
	health.SuccessRate = float64(health.Requests-health.Failures) / float64(health.Requests)
	health.LastCheck = time.Now()

	if health.SuccessRate > 0.99 {
		health.Status = "healthy"
	} else if health.SuccessRate > 0.95 {
		health.Status = "degraded"
	} else {
		health.Status = "unhealthy"
	}
}

// RecordFailure records a failed request
func (hc *HealthChecker) RecordFailure(service string, latency time.Duration) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	health, ok := hc.checks[service]
	if !ok {
		health = &ServiceHealth{Service: service}
		hc.checks[service] = health
	}

	health.Requests++
	health.Failures++
	health.Latency = (health.Latency*float64(health.Requests-1) + float64(latency)) / float64(health.Requests)
	health.SuccessRate = float64(health.Requests-health.Failures) / float64(health.Requests)
	health.LastCheck = time.Now()

	if health.SuccessRate > 0.99 {
		health.Status = "healthy"
	} else if health.SuccessRate > 0.95 {
		health.Status = "degraded"
	} else {
		health.Status = "unhealthy"
	}
}

// GetHealth returns health status for a service
func (hc *HealthChecker) GetHealth(service string) *ServiceHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return hc.checks[service]
}

// GetAllHealth returns health status for all services
func (hc *HealthChecker) GetAllHealth() map[string]*ServiceHealth {
	hc.mu.RLock()
	defer hc.mu.RUnlock()

	result := make(map[string]*ServiceHealth)
	for k, v := range hc.checks {
		result[k] = v
	}
	return result
}

// ============================================================================
// Helper Functions
// ============================================================================

func timeSince(t time.Time) time.Duration {
	if t.IsZero() {
		return 0
	}
	return time.Since(t)
}

// ============================================================================
// Export
// ============================================================================

type CircuitBreakerService struct {
	distBreaker *DistributedCircuitBreaker
	healthCheck *HealthChecker
}

func NewCircuitBreakerService() *CircuitBreakerService {
	config := CircuitBreakerConfig{
		FailureThreshold:  5,
		SuccessThreshold: 3,
		Timeout:         30 * time.Second,
		MaxRequests:     10,
	}

	return &CircuitBreakerService{
		distBreaker: NewDistributedCircuitBreaker(config),
		healthCheck: NewHealthChecker(),
	}
}

func (cbs *CircuitBreakerService) Execute(service string, fn func() error) error {
	start := time.Now()
	err := cbs.distBreaker.Execute(service, fn)
	latency := time.Since(start)

	if err != nil {
		cbs.healthCheck.RecordFailure(service, latency)
	} else {
		cbs.healthCheck.RecordSuccess(service, latency)
	}

	return err
}

func (cbs *CircuitBreakerService) GetHealth(service string) *ServiceHealth {
	return cbs.healthCheck.GetHealth(service)
}

func (cbs *CircuitBreakerService) GetAllHealth() map[string]*ServiceHealth {
	return cbs.healthCheck.GetAllHealth()
}