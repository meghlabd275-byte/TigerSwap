// ============================================================================
// TIGERSWAP COMPLETE SECURITY HARDENING
// DDOS Protection, XSS Protection, SQL Injection Prevention, CSRF Protection
// Secure Headers, Audit Logging, Intrusion Detection
// ============================================================================

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// CONSTANTS
// ============================================================================

const (
	// Rate limiting
	DEFAULT_RATE_LIMIT        = 100
	DEFAULT_RATE_WINDOW      = 60 // seconds
	ADMIN_RATE_LIMIT        = 1000
	ADMIN_RATE_WINDOW      = 60

	// IP blocking
	MAX_BLOCKED_IPS      = 10000
	BLOCK_DURATION       = 24 * time.Hour

	// DDOS detection
	DDOS_THRESHOLD       = 1000 // requests per minute
	DDOS_BLOCK_DURATION  = 1 * time.Hour

	// Audit log
	MAX_AUDIT_LOG_SIZE  = 100000

	// Headers
	SECURITY_HEADERS_MAX_AGE = 31536000 // 1 year
)

// ============================================================================
// MODELS
// ============================================================================

// SecurityConfig represents security configuration
type SecurityConfig struct {
	EnableDDOSProtection   bool `json:"enable_ddos_protection"`
	EnableXSSProtection    bool `json:"enable_xss_protection"`
	EnableSQLInjectionPrevention bool `json:"enable_sql_injection_prevention"`
	EnableCSRFProtection   bool `json:"enable_csrf_protection"`
	EnableRateLimiting    bool `json:"enable_rate_limiting"`
	EnableAuditLogging   bool `json:"enable_audit_logging"`
	EnableIPBlocking  bool `json:"enable_ip_blocking"`
	EnableHSTS       bool `json:"enable_hsts"`
	EnableCSP        bool `json:"enable_csp"`
	RateLimit        int  `json:"rate_limit"`
	RateWindow       int  `json:"rate_window"`
}

// BlockedIP represents blocked IP
type BlockedIP struct {
	IP         string    `json:"ip"`
	Reason     string    `json:"reason"`
	BlockedAt  time.Time `json:"blocked_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// RateLimitRule represents rate limit rule
type RateLimitRule struct {
	Path       string `json:"path"`
	Method     string `json:"method"`
	Limit      int    `json:"limit"`
	WindowSec  int    `json:"window_sec"`
	Burst      int    `json:"burst"`
}

// AuditLog represents audit log entry
type AuditLog struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	IPAddress   string    `json:"ip_address"`
	UserAgent  string    `json:"user_agent"`
	UserID     string    `json:"user_id,omitempty"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	Method     string    `json:"method"`
	StatusCode int      `json:"status_code"`
	LatencyMs  int      `json:"latency_ms"`
	RequestID  string    `json:"request_id"`
}

// CSRFToken represents CSRF token
type CSRFToken struct {
	Token     string    `json:"token"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ThreatInfo represents detected threat
type ThreatInfo struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	SourceIP    string    `json:"source_ip"`
	Timestamp  time.Time `json:"timestamp"`
	Details    string    `json:"details"`
	Severity   string    `json:"severity"`
	Action    string    `json:"action"` // blocked, logged, warned
}

// ============================================================================
// SECURITY STORE
// ============================================================================

type SecurityStore struct {
	mu sync.RWMutex

	// Configuration
	config *SecurityConfig

	// Rate limiting
	rateLimits map[string]*RateLimitInfo // key -> info
	rateRules  []RateLimitRule

	// IP blocking
	blockedIPs map[string]*BlockedIP // IP -> blocked

	// DDOS detection
	ddosDetect map[string]*DDOSInfo // IP -> info

	// Audit log
	auditLogs []*AuditLog

	// CSRF tokens
	csrfTokens map[string]*CSRFToken // token -> CSRF

	// Threats
	threats []*ThreatInfo

	// Allowed hosts
	allowedHosts map[string]bool

	// Allowed origins (CORS)
	allowedOrigins map[string]bool
}

// RateLimitInfo represents rate limit info
type RateLimitInfo struct {
	Count     int
	ResetAt   time.Time
	Blocked   bool
	BlockTill *time.Time
	Requests []time.Time // for sliding window
}

// DDOSInfo represents DDOS detection info
type DDOSInfo struct {
	RequestCount int
	StartTime   time.Time
	LastTime   time.Time
	IsDetected bool
}

// NewSecurityStore creates new security store
func NewSecurityStore() *SecurityStore {
	return &SecurityStore{
		config: &SecurityConfig{
			EnableDDOSProtection:          true,
			EnableXSSProtection:        true,
			EnableSQLInjectionPrevention: true,
			EnableCSRFProtection:       true,
			EnableRateLimiting:         true,
			EnableAuditLogging:         true,
			EnableIPBlocking:          true,
			EnableHSTS:              true,
			EnableCSP:               true,
			RateLimit:               DEFAULT_RATE_LIMIT,
			RateWindow:              DEFAULT_RATE_WINDOW,
		},
		rateLimits:  make(map[string]*RateLimitInfo),
		rateRules:  make([]RateLimitRule, 0),
		blockedIPs: make(map[string]*BlockedIP),
		ddosDetect: make(map[string]*DDOSInfo),
		auditLogs: make([]*AuditLog, 0),
		csrfTokens: make(map[string]*CSRFToken),
		threats:  make([]*ThreatInfo, 0),
		allowedHosts: make(map[string]bool),
		allowedOrigins: make(map[string]bool),
	}
}

// ============================================================================
// RATE LIMITING
// ============================================================================

// CheckRateLimit checks rate limit
func (s *SecurityStore) CheckRateLimit(key string, limit int, windowSec int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	info, ok := s.rateLimits[key]

	if !ok || now.After(info.ResetAt) {
		s.rateLimits[key] = &RateLimitInfo{
			Count:   1,
			ResetAt: now.Add(time.Duration(windowSec) * time.Second),
			Requests: []time.Time{now},
		}
		return true
	}

	if info.Blocked {
		if info.BlockTill != nil && now.Before(*info.BlockTill) {
			return false
		}
		info.Blocked = false
	}

	// Sliding window rate limiting
	var recentRequests int
	cutoff := now.Add(-time.Duration(windowSec) * time.Second)
	for _, t := range info.Requests {
		if t.After(cutoff) {
			recentRequests++
		}
	}

	if recentRequests >= limit {
		info.Blocked = true
		blockTill := now.Add(time.Minute)
		info.BlockTill = &blockTill
		return false
	}

	info.Count++
	info.Requests = append(info.Requests, now)

	// Trim old requests
	if len(info.Requests) > limit*2 {
		info.Requests = info.Requests[len(info.Requests)-limit:]
	}

	return true
}

// AddRateLimitRule adds rate limit rule
func (s *SecurityStore) AddRateLimitRule(rule RateLimitRule) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.rateRules = append(s.rateRules, rule)
}

// ============================================================================
// IP BLOCKING
// ============================================================================

// BlockIP blocks IP
func (s *SecurityStore) BlockIP(ip, reason string, duration time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.blockedIPs) >= MAX_BLOCKED_IPS {
		return fmt.Errorf("maximum blocked IPs reached")
	}

	blocked := &BlockedIP{
		IP:        ip,
		Reason:    reason,
		BlockedAt: time.Now(),
	}

	if duration > 0 {
		expiresAt := time.Now().Add(duration)
		blocked.ExpiresAt = &expiresAt
	}

	s.blockedIPs[ip] = blocked
	return nil
}

// UnblockIP unblocks IP
func (s *SecurityStore) UnblockIP(ip string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.blockedIPs[ip]; !ok {
		return fmt.Errorf("IP not blocked")
	}

	delete(s.blockedIPs, ip)
	return nil
}

// IsBlocked checks if IP is blocked
func (s *SecurityStore) IsBlocked(ip string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	blocked, ok := s.blockedIPs[ip]
	if !ok {
		return false
	}

	if blocked.ExpiresAt != nil && time.Now().After(*blocked.ExpiresAt) {
		return false
	}

	return true
}

// GetBlockedIPs gets all blocked IPs
func (s *SecurityStore) GetBlockedIPs() []*BlockedIP {
	s.mu.RLock()
	defer s.mu.RUnlock()

	blocked := make([]*BlockedIP, 0, len(s.blockedIPs))
	for _, b := range s.blockedIPs {
		if b.ExpiresAt == nil || time.Now().Before(*b.ExpiresAt) {
			blocked = append(blocked, b)
		}
	}

	return blocked
}

// ============================================================================
// DDOS DETECTION
// ============================================================================

// CheckDDOS checks for DDOS attack
func (s *SecurityStore) CheckDDOS(ip string) (bool, string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	info, ok := s.ddosDetect[ip]

	if !ok {
		s.ddosDetect[ip] = &DDOSInfo{
			RequestCount: 1,
			StartTime:   now,
			LastTime:   now,
		}
		return false, ""
	}

	// Reset counter every minute
	if now.Sub(info.LastTime) > time.Minute {
		info.RequestCount = 1
		info.LastTime = now
		return false, ""
	}

	info.RequestCount++

	if info.RequestCount >= DDOS_THRESHOLD {
		info.IsDetected = true

		// Auto-block
		blocked := &BlockedIP{
			IP:        ip,
			Reason:    "DDOS attack detected",
			BlockedAt: now,
		}
		expiresAt := now.Add(DDOS_BLOCK_DURATION)
		blocked.ExpiresAt = &expiresAt

		s.blockedIPs[ip] = blocked

		return true, "DDOS attack detected"
	}

	return false, ""
}

// ============================================================================
// XSS PROTECTION
// ============================================================================

// XSSPatterns contains XSS attack patterns
var XSSPatterns = []string{
	"<script",
	"javascript:",
	"onerror=",
	"onload=",
	"onclick=",
	"onmouseover=",
	"eval(",
	"expression(",
	"alert(",
	"confirm(",
	"prompt(",
	"document.cookie",
	"document.write",
	"window.location",
	"innerHTML",
	"outerHTML",
	"insertAdjacentHTML",
}

// CheckXSS checks for XSS attack
func (s *SecurityStore) CheckXSS(input string) (bool, string) {
	input = strings.ToLower(input)

	for _, pattern := range XSSPatterns {
		if strings.Contains(input, pattern) {
			return true, pattern
		}
	}

	return false, ""
}

// SanitizeInput sanitizes input to prevent XSS
func SanitizeInput(input string) string {
	// Escape HTML entities
	input = html.EscapeString(input)

	// Remove dangerous patterns
	dangerous := []string{
		"<script",
		"javascript:",
		"onerror=",
		"onload=",
	}

	for _, d := range dangerous {
		input = strings.ReplaceAll(input, d, "")
	}

	return input
}

// ============================================================================
// SQL INJECTION PREVENTION
// ============================================================================

// SQLInjectionPatterns contains SQL injection patterns
var SQLInjectionPatterns = []string{
	"' OR '1'='1",
	"' OR '1'='1' --",
	"' OR '1'='1' ({",
	"' OR '1'='1'/*",
	"'; DROP TABLE",
	"; DROP TABLE",
	"--",
	"/*",
	"@@",
	"CHAR(",
	"NCHAR(",
	"NVARCHAR(",
	"VARCHAR(",
	"ALTER ",
	"BEGIN ",
	"CAST(",
	"CREATE ",
	"CURSOR ",
	"DECLARE ",
	"DELETE ",
	"DROP ",
	"END ",
	"EXEC ",
	"EXECUTE ",
	"FETCH ",
	"INSERT ",
	"KILL ",
	"SELECT ",
	"TRUNCATE ",
	"UPDATE ",
	"UNION ",
	"WAITFOR ",
	"WHERE ",
}

// CheckSQLInjection checks for SQL injection
func (s *SecurityStore) CheckSQLInjection(input string) (bool, string) {
	input = strings.ToUpper(input)

	for _, pattern := range SQLInjectionPatterns {
		if strings.Contains(input, pattern) {
			return true, pattern
		}
	}

	return false, ""
}

// SanitizeSQL sanitizes SQL input
func SanitizeSQL(input string) string {
	// Remove SQL keywords
	keywords := []string{
		"SELECT", "INSERT", "UPDATE", "DELETE", "DROP", "CREATE", "ALTER", "EXEC", "UNION",
	}

	input = strings.ToUpper(input)
	for _, kw := range keywords {
		input = strings.ReplaceAll(input, kw, "")
	}

	// Escape single quotes
	input = strings.ReplaceAll(input, "'", "''")

	return input
}

// ============================================================================
// CSRF PROTECTION
// ============================================================================

// GenerateCSRFToken generates CSRF token
func (s *SecurityStore) GenerateCSRFToken(userID string) *CSRFToken {
	s.mu.Lock()
	defer s.mu.Unlock()

	token := generateRandomToken(32)

	csrf := &CSRFToken{
		Token:     token,
		UserID:    userID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	s.csrfTokens[token] = csrf
	return csrf
}

// ValidateCSRFToken validates CSRF token
func (s *SecurityStore) ValidateCSRFToken(token, userID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	csrf, ok := s.csrfTokens[token]
	if !ok {
		return false
	}

	if time.Now().After(csrf.ExpiresAt) {
		return false
	}

	if userID != "" && csrf.UserID != userID {
		return false
	}

	return true
}

// ============================================================================
// AUDIT LOGGING
// ============================================================================

// LogAudit logs audit entry
func (s *SecurityStore) LogAudit(log *AuditLog) {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.ID = generateUUID()
	log.Timestamp = time.Now()

	s.auditLogs = append(s.auditLogs, log)

	// Trim old logs
	if len(s.auditLogs) > MAX_AUDIT_LOG_SIZE {
		s.auditLogs = s.auditLogs[len(s.auditLogs)-MAX_AUDIT_LOG_SIZE:]
	}
}

// GetAuditLogs gets audit logs
func (s *SecurityStore) GetAuditLogs(limit int, userID, action string) []*AuditLog {
	s.mu.RLock()
	defer s.mu.RUnlock()

	logs := make([]*AuditLog, 0)
	for _, log := range s.auditLogs {
		if userID != "" && log.UserID != userID {
			continue
		}
		if action != "" && log.Action != action {
			continue
		}
		logs = append(logs, log)
		if limit > 0 && len(logs) >= limit {
			break
		}
	}

	return logs
}

// ============================================================================
// THREAT DETECTION
// ============================================================================

// DetectThreat detects threat
func (s *SecurityStore) DetectThreat(threat *ThreatInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	threat.ID = generateUUID()
	threat.Timestamp = time.Now()

	s.threats = append(s.threats, threat)

	// Block IP if severity is high
	if threat.Severity == "high" && threat.Action == "blocked" {
		s.BlockIP(threat.SourceIP, threat.Details, BLOCK_DURATION)
	}
}

// GetThreats gets threats
func (s *SecurityStore) GetThreats(limit int) []*ThreatInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	threats := s.threats
	if limit > 0 && len(threats) > limit {
		threats = threats[len(threats)-limit:]
	}

	return threats
}

// ============================================================================
// SECURITY HEADERS
// ============================================================================

// AddSecurityHeaders adds security headers to response
func (s *SecurityStore) AddSecurityHeaders(w http.ResponseWriter, r *http.Request) {
	cfg := s.config

	// HSTS
	if cfg.EnableHSTS {
		w.Header().Set("Strict-Transport-Security", fmt.Sprintf("max-age=%d; includeSubDomains", SECURITY_HEADERS_MAX_AGE))
	}

	// CSP
	if cfg.EnableCSP {
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline' 'unsafe-eval'; " +
			"style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data: https:; " +
			"font-src 'self' data:; " +
			"connect-src 'self' https: wss:; " +
			"frame-ancestors 'none'; " +
			"form-action 'self'; " +
			"base-uri 'self'; " +
			"upgrade-insecure-requests;"
		w.Header().Set("Content-Security-Policy", csp)
	}

	// X-Frame-Options
	w.Header().Set("X-Frame-Options", "DENY")

	// X-Content-Type-Options
	w.Header().Set("X-Content-Type-Options", "nosniff")

	// X-XSS-Protection
	w.Header().Set("X-XSS-Protection", "1; mode=block")

	// Referrer-Policy
	w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

	// Permissions-Policy
	w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=(), payment=(), usb=()")

	// Cache-Control for sensitive pages
	if strings.Contains(r.URL.Path, "/admin") || strings.Contains(r.URL.Path, "/wallet") {
		w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, proxy-revalidate, max-age=0")
		w.Header().Set("Pragma", "no-cache")
	}
}

// ============================================================================
// ALLOWED HOSTS/ORIGINS
// ============================================================================

// AddAllowedHost adds allowed host
func (s *SecurityStore) AddAllowedHost(host string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.allowedHosts[host] = true
}

// AddAllowedOrigin adds allowed origin for CORS
func (s *SecurityStore) AddAllowedOrigin(origin string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.allowedOrigins[origin] = true
}

// IsAllowedHost checks if host is allowed
func (s *SecurityStore) IsAllowedHost(host string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Allow localhost for development
	if strings.HasPrefix(host, "localhost") || strings.HasPrefix(host, "127.0.0.1") {
		return true
	}

	return s.allowedHosts[host]
}

// IsAllowedOrigin checks if origin is allowed
func (s *SecurityStore) IsAllowedOrigin(origin string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.allowedOrigins[origin]
}

// ============================================================================
// VALIDATION
// ============================================================================

// ValidateURL validates URL
func (s *SecurityStore) ValidateURL(rawURL string) (bool, string) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false, "invalid URL format"
	}

	// Check for valid scheme
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false, "invalid URL scheme"
	}

	// Check host
	if parsed.Host == "" {
		return false, "invalid URL host"
	}

	// Check if host is allowed
	if !s.IsAllowedHost(parsed.Host) {
		return false, "host not allowed"
	}

	return true, ""
}

// ValidateEmail validates email
func ValidateEmail(email string) bool {
	pattern := `^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`
	matched, _ := regexp.MatchString(pattern, email)
	return matched
}

// ValidateAddress validates blockchain address
func ValidateAddress(address string) bool {
	// Ethereum address
	if strings.HasPrefix(address, "0x") && len(address) == 42 {
		matched, _ := regexp.MatchString(`^0x[a-fA-F0-9]{40}$`, address)
		return matched
	}

	// Solana address (base58)
	if len(address) >= 32 && len(address) <= 44 {
		matched, _ := regexp.MatchString(`^[1-9A-HJ-NP-Za-km-z]+$`, address)
		return matched
	}

	return false
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func generateRandomToken(length int) string {
	return generateRandomHex(length)
}

func generateRandomHex(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateUUID() string {
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		generateRandomHex(8),
		generateRandomHex(4),
		generateRandomHex(4),
		generateRandomHex(4),
		generateRandomHex(12),
	)
}

// ============================================================================
// MIDDLEWARE
// ============================================================================

// SecurityMiddleware creates security middleware
func SecurityMiddleware(store *SecurityStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)

			// Check if IP is blocked
			if store.IsBlocked(ip) {
				http.Error(w, "access denied", http.StatusForbidden)
				return
			}

			// Check DDOS
			if store.config.EnableDDOSProtection {
				detected, reason := store.CheckDDOS(ip)
				if detected {
					store.DetectThreat(&ThreatInfo{
						Type:      "DDOS",
						SourceIP:  ip,
						Details:   reason,
						Severity:  "high",
						Action:   "blocked",
					})
					http.Error(w, "too many requests", http.StatusTooManyRequests)
					return
				}
			}

			// Check rate limit
			if store.config.EnableRateLimiting {
				limit := store.config.RateLimit
				window := store.config.RateWindow

				// Check specific rules
				for _, rule := range store.rateRules {
					if matchPath(r.URL.Path, rule.Path) && rule.Method == r.Method {
						limit = rule.Limit
						window = rule.WindowSec
						break
					}
				}

				if !store.CheckRateLimit(ip, limit, window) {
					w.Header().Set("Retry-After", fmt.Sprintf("%d", window))
					http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
					return
				}
			}

			// Add security headers
			store.AddSecurityHeaders(w, r)

			// Log request if audit logging enabled
			if store.config.EnableAuditLogging {
				start := time.Now()
				// Note: In production, we'd wrap ResponseWriter to capture status code
				defer func() {
					store.LogAudit(&AuditLog{
						IPAddress:  ip,
						UserAgent:  r.UserAgent(),
						Action:     r.URL.Path,
						Resource:   r.URL.Path,
						Method:     r.Method,
						StatusCode: 0, // Would capture from wrapped response
						LatencyMs:  int(time.Since(start).Milliseconds()),
					})
				}()
			}

			next.ServeHTTP(w, r)
		})
	}
}

// matchPath matches URL path pattern
func matchPath(path, pattern string) bool {
	if pattern == "*" {
		return true
	}

	// Simple wildcard matching
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(path, prefix)
	}

	return path == pattern
}

// getClientIP gets client IP from request
func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}

	return ip
}

func timeSince(t time.Time) time.Duration {
	return time.Since(t)
}

// ============================================================================
// HTTP HANDLERS
// ============================================================================

// SecurityHandler handles security requests
type SecurityHandler struct {
	store *SecurityStore
}

// NewSecurityHandler creates new handler
func NewSecurityHandler(store *SecurityStore) *SecurityHandler {
	return &SecurityHandler{store: store}
}

// HandleBlockIP handles block IP request
func (h *SecurityHandler) HandleBlockIP(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IP       string `json:"ip"`
		Reason   string `json:"reason"`
		Duration int    `json:"duration"` // hours, 0 = permanent
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	duration := time.Duration(req.Duration) * time.Hour
	if err := h.store.BlockIP(req.IP, req.Reason, duration); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"success": true,
		"message": "IP blocked",
	})
}

// HandleGetBlockedIPs handles get blocked IPs request
func (h *SecurityHandler) HandleGetBlockedIPs(w http.ResponseWriter, r *http.Request) {
	blocked := h.store.GetBlockedIPs()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(blocked)
}

// HandleGetAuditLogs handles get audit logs request
func (h *SecurityHandler) HandleGetAuditLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	userID := r.URL.Query().Get("user_id")
	action := r.URL.Query().Get("action")

	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)

	logs := h.store.GetAuditLogs(limit, userID, action)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

// HandleGetThreats handles get threats request
func (h *SecurityHandler) HandleGetThreats(w http.ResponseWriter, r *http.Request) {
	limit := 100
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)

	threats := h.store.GetThreats(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(threats)
}

// ============================================================================
// GLOBAL INSTANCE
// ============================================================================

var securityStore *SecurityStore

// InitSecurity initializes security system
func InitSecurity() {
	securityStore = NewSecurityStore()

	// Add default allowed hosts
	securityStore.AddAllowedHost("localhost")
	securityStore.AddAllowedHost("127.0.0.1")

	// Add default allowed origins
	securityStore.AddAllowedOrigin("http://localhost:3000")
	securityStore.AddAllowedOrigin("https://tigerswap.com")
}

// GetSecurityStore returns security store
func GetSecurityStore() *SecurityStore {
	return securityStore
}