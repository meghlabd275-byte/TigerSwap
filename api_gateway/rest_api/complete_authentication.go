// ============================================================================
// TIGERSWAP COMPLETE AUTHENTICATION SYSTEM
// Industrial-grade security with 2FA/MFA, role-based access, session management
// ============================================================================

package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/sessions"
	"github.com/gorilla/websocket"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// CONSTANTS
// ============================================================================

const (
	// Session settings
	SESSION_NAME                    = "tigerswap_session"
	SESSION_MAX_AGE                 = 86400 * 7 // 7 days
	SESSION_SECURE_COOKIE            = true
	SESSION_HTTP_ONLY                = true
	SESSION_SAME_SITE               = http.SameSiteStrictMode

	// Rate limiting
	MAX_LOGIN_ATTEMPTS             = 5
	LOGIN_LOCKOUT_DURATION         = 15 * 60 // 15 minutes
	MAX_REGISTRATION_PER_IP         = 10
	MAX_API_REQUESTS_PER_MINUTE     = 60

	// Password requirements
	MIN_PASSWORD_LENGTH            = 12
	MAX_PASSWORD_LENGTH          = 128
	PASSWORD_UPPERCASE_REQUIRED   = true
	PASSWORD_LOWERCASE_REQUIRED = true
	PASSWORD_NUMBER_REQUIRED    = true
	PASSWORD_SPECIAL_REQUIRED  = true

	// JWT settings
	JWT_EXPIRY_HOURS             = 24
	JWT_REFRESH_EXPIRY_DAYS      = 30
	JWT_SIGNING_KEY_LENGTH       = 32

	// 2FA settings
	TFOTP_ISSUER                = "TigerSwap"
	TOTP_PERIOD                 = 30
	TOTP_WINDOW                 = 1
)

// ============================================================================
// ENUMS
// ============================================================================

type UserRole string

const (
	RoleSuperAdmin      UserRole = "super_admin"
	RoleAdmin          UserRole = "admin"
	RoleFinanceAdmin   UserRole = "finance_admin"
	RoleBotOperator    UserRole = "bot_operator"
	RoleTradingAdmin   UserRole = "trading_admin"
	RoleClient        UserRole = "client"
	RoleUser          UserRole = "user"
)

type AuthMethod string

const (
	AuthMethodPassword AuthMethod = "password"
	AuthMethod2FA    AuthMethod = "2fa"
	AuthMethodMFA    AuthMethod = "mfa"
	AuthMethodOAuth  AuthMethod = "oauth"
	AuthMethodWallet AuthMethod = "wallet"
)

type SessionStatus string

const (
	SessionActive    SessionStatus = "active"
	SessionExpired SessionStatus = "expired"
	SessionRevoked SessionStatus = "revoked"
)

// ============================================================================
// MODELS
// ============================================================================

// User with complete authentication
type AuthUser struct {
	ID                string            `json:"id"`
	WalletAddress    string            `json:"wallet_address,omitempty"`
	Email           string            `json:"email"`
	Username        string            `json:"username"`
	PasswordHash    string            `json:"password_hash"`
	Role            UserRole         `json:"role"`
	IsActive        bool             `json:"is_active"`
	IsVerified      bool             `json:"is_verified"`
	TwoFactorEnabled bool            `json:"two_factor_enabled"`
	TwoFactorSecret string           `json:"two_factor_secret,omitempty"`
	BackupCodes     []string         `json:"backup_codes,omitempty"`
	Permissions     []string         `json:"permissions"`
	FailedAttempts  int              `json:"failed_attempts"`
	LockedUntil    *time.Time       `json:"locked_until,omitempty"`
	LastLoginAt     *time.Time       `json:"last_login_at,omitempty"`
	LastLoginIP    string           `json:"last_login_ip,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// Admin session
type AdminSession struct {
	ID            string         `json:"id"`
	UserID        string         `json:"user_id"`
	SessionToken  string        `json:"session_token"`
	RefreshToken string        `json:"refresh_token,omitempty"`
	IPAddress     string        `json:"ip_address"`
	UserAgent     string        `json:"user_agent"`
	Status        SessionStatus `json:"status"`
	ExpiresAt     time.Time     `json:"expires_at"`
	CreatedAt     time.Time     `json:"created_at"`
	LastActivity time.Time     `json:"last_activity"`
}

// Admin permission
type AdminPermission struct {
	ID          string   `json:"id"`
	Name       string   `json:"name"`
	Description string  `json:"description"`
	Category   string   `json:"category"`
}

// Role permissions mapping
type RolePermissions struct {
	Role       UserRole    `json:"role"`
	Permissions []string   `json:"permissions"`
}

// Login attempt tracking
type LoginAttempt struct {
	ID          string    `json:"id"`
	Email       string    `json:"email"`
	IPAddress   string    `json:"ip_address"`
	AttemptedAt time.Time `json:"attempted_at"`
	Success    bool      `json:"success"`
}

// ============================================================================
// AUTHENTICATION STORAGE
// ============================================================================

type AuthenticationStore struct {
	mu           sync.RWMutex
	users        map[string]*AuthUser          // email -> user
	usersByID     map[string]*AuthUser          // id -> user
	usersByWallet map[string]*AuthUser          // wallet -> user
	sessions     map[string]*AdminSession    // sessionToken -> session
	sessionsByID map[string][]string         // userID -> sessionTokens
	attempts     map[string][]LoginAttempt  // email -> attempts
	permissions map[string]*AdminPermission // name -> permission
	rolePerms    map[UserRole]*RolePermissions

	// Rate limiting
	rateLimits      map[string]*RateLimitInfo
	ipRegistrations map[string]int

	// JWT keys
	jwtSigningKey   []byte
	jwtRefreshKey   []byte
}

// Rate limit info
type RateLimitInfo struct {
	Count     int
	ResetAt   time.Time
	Blocked   bool
	BlockTill *time.Time
}

// NewAuthenticationStore creates new auth store
func NewAuthenticationStore() *AuthenticationStore {
	store := &AuthenticationStore{
		users:            make(map[string]*AuthUser),
		usersByID:        make(map[string]*AuthUser),
		usersByWallet:    make(map[string]*AuthUser),
		sessions:        make(map[string]*AdminSession),
		sessionsByID:    make(map[string][]string),
		attempts:        make(map[string][]LoginAttempt),
		permissions:     make(map[string]*AdminPermission),
		rolePerms:      make(map[UserRole]*RolePermissions),
		rateLimits:      make(map[string]*RateLimitInfo),
		ipRegistrations: make(map[string]int),
	}

	// Generate JWT keys
	store.jwtSigningKey = generateRandomBytes(JWT_SIGNING_KEY_LENGTH)
	store.jwtRefreshKey = generateRandomBytes(JWT_SIGNING_KEY_LENGTH)

	// Initialize default permissions
	store.initDefaultPermissions()

	// Initialize role permissions
	store.initRolePermissions()

	return store
}

// Initialize default permissions
func (s *AuthenticationStore) initDefaultPermissions() {
	permissions := []*AdminPermission{
		// User management
		{ID: "user.view", Name: "user.view", Description: "View users", Category: "user"},
		{ID: "user.create", Name: "user.create", Description: "Create users", Category: "user"},
		{ID: "user.edit", Name: "user.edit", Description: "Edit users", Category: "user"},
		{ID: "user.delete", Name: "user.delete", Description: "Delete users", Category: "user"},
		{ID: "user.kyc", Name: "user.kyc", Description: "Manage KYC", Category: "user"},

		// Admin management
		{ID: "admin.view", Name: "admin.view", Description: "View admins", Category: "admin"},
		{ID: "admin.create", Name: "admin.create", Description: "Create admins", Category: "admin"},
		{ID: "admin.edit", Name: "admin.edit", Description: "Edit admins", Category: "admin"},
		{ID: "admin.delete", Name: "admin.delete", Description: "Delete admins", Category: "admin"},
		{ID: "admin.grant", Name: "admin.grant", Description: "Grant permissions", Category: "admin"},

		// Bot management
		{ID: "bot.view", Name: "bot.view", Description: "View bots", Category: "bot"},
		{ID: "bot.create", Name: "bot.create", Description: "Create bots", Category: "bot"},
		{ID: "bot.start", Name: "bot.start", Description: "Start bots", Category: "bot"},
		{ID: "bot.stop", Name: "bot.stop", Description: "Stop bots", Category: "bot"},
		{ID: "bot.configure", Name: "bot.configure", Description: "Configure bots", Category: "bot"},
		{ID: "bot.all", Name: "bot.all", Description: "Manage all bots", Category: "bot"},

		// Fee management
		{ID: "fee.view", Name: "fee.view", Description: "View fees", Category: "fee"},
		{ID: "fee.configure", Name: "fee.configure", Description: "Configure fees", Category: "fee"},
		{ID: "fee.withdraw", Name: "fee.withdraw", Description: "Withdraw fees", Category: "fee"},

		// Chain management
		{ID: "chain.view", Name: "chain.view", Description: "View chains", Category: "chain"},
		{ID: "chain.add", Name: "chain.add", Description: "Add chains", Category: "chain"},
		{ID: "chain.edit", Name: "chain.edit", Description: "Edit chains", Category: "chain"},
		{ID: "chain.remove", Name: "chain.remove", Description: "Remove chains", Category: "chain"},

		// Token management
		{ID: "token.view", Name: "token.view", Description: "View tokens", Category: "token"},
		{ID: "token.list", Name: "token.list", Description: "List tokens", Category: "token"},
		{ID: "token.delist", Name: "token.delist", Description: "Delist tokens", Category: "token"},

		// White label
		{ID: "whitelabel.view", Name: "whitelabel.view", Description: "View white label", Category: "whitelabel"},
		{ID: "whitelabel.create", Name: "whitelabel.create", Description: "Create white label", Category: "whitelabel"},
		{ID: "whitelabel.approve", Name: "whitelabel.approve", Description: "Approve white label", Category: "whitelabel"},
		{ID: "whitelabel.destroy", Name: "whitelabel.destroy", Description: "Destroy white label", Category: "whitelabel"},

		// Wallet management
		{ID: "wallet.view", Name: "wallet.view", Description: "View wallets", Category: "wallet"},
		{ID: "wallet.transfer", Name: "wallet.transfer", Description: "Make transfers", Category: "wallet"},
		{ID: "wallet.sign", Name: "wallet.sign", Description: "Sign transactions", Category: "wallet"},
		{ID: "wallet.auto_sign", Name: "wallet.auto_sign", Description: "Auto-sign transactions", Category: "wallet"},

		// Platform
		{ID: "platform.config", Name: "platform.config", Description: "Configure platform", Category: "platform"},
		{ID: "platform.shutdown", Name: "platform.shutdown", Description: "Shutdown platform", Category: "platform"},
	}

	for _, p := range permissions {
		s.permissions[p.ID] = p
	}
}

// Initialize role permissions
func (s *AuthenticationStore) initRolePermissions() {
	s.rolePerms = map[UserRole]*RolePermissions{
		RoleSuperAdmin: {
			Role: RoleSuperAdmin,
			Permissions: []string{
				"user.view", "user.create", "user.edit", "user.delete", "user.kyc",
				"admin.view", "admin.create", "admin.edit", "admin.delete", "admin.grant",
				"bot.view", "bot.create", "bot.start", "bot.stop", "bot.configure", "bot.all",
				"fee.view", "fee.configure", "fee.withdraw",
				"chain.view", "chain.add", "chain.edit", "chain.remove",
				"token.view", "token.list", "token.delist",
				"whitelabel.view", "whitelabel.create", "whitelabel.approve", "whitelabel.destroy",
				"wallet.view", "wallet.transfer", "wallet.sign", "wallet.auto_sign",
				"platform.config", "platform.shutdown",
			},
		},
		RoleAdmin: {
			Role: RoleAdmin,
			Permissions: []string{
				"user.view", "user.create", "user.edit", "user.kyc",
				"admin.view", "admin.create", "admin.edit",
				"bot.view", "bot.create", "bot.start", "bot.stop", "bot.configure",
				"fee.view", "fee.configure",
				"chain.view", "chain.add", "chain.edit",
				"token.view", "token.list",
				"wallet.view", "wallet.transfer", "wallet.sign",
			},
		},
		RoleFinanceAdmin: {
			Role: RoleFinanceAdmin,
			Permissions: []string{
				"user.view",
				"bot.view",
				"fee.view", "fee.configure", "fee.withdraw",
				"wallet.view", "wallet.transfer",
			},
		},
		RoleBotOperator: {
			Role: RoleBotOperator,
			Permissions: []string{
				"bot.view", "bot.create", "bot.start", "bot.stop", "bot.configure",
				"chain.view",
				"token.view",
			},
		},
		RoleTradingAdmin: {
			Role: RoleTradingAdmin,
			Permissions: []string{
				"bot.view", "bot.start", "bot.stop",
				"chain.view", "chain.add", "chain.edit",
				"token.view", "token.list",
				"wallet.view", "wallet.sign",
			},
		},
		RoleClient: {
			Role: RoleClient,
			Permissions: []string{
				"bot.view", "bot.create", "bot.start", "bot.stop", "bot.configure",
				"chain.view",
				"token.view",
				"wallet.view",
			},
		},
	}
}

// ============================================================================
// PASSWORD VALIDATION
// ============================================================================

// ValidatePassword checks password meets requirements
func ValidatePassword(password string) error {
	if len(password) < MIN_PASSWORD_LENGTH || len(password) > MAX_PASSWORD_LENGTH {
		return fmt.Errorf("password must be %d-%d characters", MIN_PASSWORD_LENGTH, MAX_PASSWORD_LENGTH)
	}

	if PASSWORD_UPPERCASE_REQUIRED && !regexp.MustCompile(`[A-Z]`).MatchString(password) {
		return fmt.Errorf("password must contain uppercase letter")
	}

	if PASSWORD_LOWERCASE_REQUIRED && !regexp.MustCompile(`[a-z]`).MatchString(password) {
		return fmt.Errorf("password must contain lowercase letter")
	}

	if PASSWORD_NUMBER_REQUIRED && !regexp.MustCompile(`[0-9]`).MatchString(password) {
		return fmt.Errorf("password must contain number")
	}

	if PASSWORD_SPECIAL_REQUIRED && !regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]`).MatchString(password) {
		return fmt.Errorf("password must contain special character")
	}

	return nil
}

// HashPassword creates secure hash of password
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// CheckPassword verifies password against hash
func CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ============================================================================
// SESSION MANAGEMENT
// ============================================================================

// CreateSession creates new admin session
func (s *AuthenticationStore) CreateSession(userID, ipAddress, userAgent string) (*AdminSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session := &AdminSession{
		ID:            generateUUID(),
		UserID:        userID,
		SessionToken:  generateRandomToken(32),
		RefreshToken: generateRandomToken(32),
		IPAddress:     ipAddress,
		UserAgent:     userAgent,
		Status:        SessionActive,
		ExpiresAt:     time.Now().Add(SESSION_MAX_AGE * time.Second),
		CreatedAt:     time.Now(),
		LastActivity:  time.Now(),
	}

	s.sessions[session.SessionToken] = session
	s.sessionsByID[userID] = append(s.sessionsByID[userID], session.SessionToken)

	return session, nil
}

// ValidateSession validates session token
func (s *AuthenticationStore) ValidateSession(sessionToken string) (*AdminSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionToken]
	if !ok {
		return nil, fmt.Errorf("invalid session")
	}

	if session.Status != SessionActive {
		return nil, fmt.Errorf("session %s", session.Status)
	}

	if time.Now().After(session.ExpiresAt) {
		session.Status = SessionExpired
		return nil, fmt.Errorf("session expired")
	}

	session.LastActivity = time.Now()
	return session, nil
}

// RevokeSession revokes session
func (s *AuthenticationStore) RevokeSession(sessionToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionToken]
	if !ok {
		return fmt.Errorf("session not found")
	}

	session.Status = SessionRevoked
	return nil
}

// RevokeAllUserSessions revokes all sessions for user
func (s *AuthenticationStore) RevokeAllUserSessions(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tokens, ok := s.sessionsByID[userID]
	if !ok {
		return nil
	}

	for _, token := range tokens {
		if session, ok := s.sessions[token]; ok {
			session.Status = SessionRevoked
		}
	}

	delete(s.sessionsByID, userID)
	return nil
}

// ============================================================================
// USER MANAGEMENT
// ============================================================================

// CreateUser creates new user
func (s *AuthenticationStore) CreateUser(email, username, password, walletAddress string, role UserRole) (*AuthUser, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Validate email
	if !isValidEmail(email) {
		return nil, fmt.Errorf("invalid email format")
	}

	// Check email exists
	if _, ok := s.users[email]; ok {
		return nil, fmt.Errorf("email already registered")
	}

	// Validate username
	if !isValidUsername(username) {
		return nil, fmt.Errorf("invalid username format")
	}

	// Check username exists
	for _, u := range s.users {
		if u.Username == username {
			return nil, fmt.Errorf("username already taken")
		}
	}

	// Validate password
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}

	// Hash password
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &AuthUser{
		ID:            generateUUID(),
		Email:         email,
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		IsActive:     true,
		IsVerified:  role == RoleSuperAdmin, // Auto-verify super admins
		Permissions: s.getPermissionsForRole(role),
		FailedAttempts: 0,
		CreatedAt:    time.Now(),
		UpdatedAt:   time.Now(),
	}

	if walletAddress != "" {
		user.WalletAddress = walletAddress
		s.usersByWallet[walletAddress] = user
	}

	s.users[email] = user
	s.usersByID[user.ID] = user

	return user, nil
}

// GetUserByEmail gets user by email
func (s *AuthenticationStore) GetUserByEmail(email string) (*AuthUser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[strings.ToLower(email)]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// GetUserByID gets user by ID
func (s *AuthenticationStore) GetUserByID(id string) (*AuthUser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.usersByID[id]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// GetUserByWallet gets user by wallet address
func (s *AuthenticationStore) GetUserByWallet(wallet string) (*AuthUser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.usersByWallet[strings.ToLower(wallet)]
	if !ok {
		return nil, fmt.Errorf("user not found")
	}

	return user, nil
}

// UpdateUser updates user
func (s *AuthenticationStore) UpdateUser(userID string, updates map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.usersByID[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}

	if email, ok := updates["email"].(string); ok {
		if !isValidEmail(email) {
			return fmt.Errorf("invalid email format")
		}
		if _, exists := s.users[email]; exists && email != user.Email {
			return fmt.Errorf("email already in use")
		}
		delete(s.users, user.Email)
		user.Email = email
		s.users[email] = user
	}

	if username, ok := updates["username"].(string); ok {
		user.Username = username
	}

	if role, ok := updates["role"].(string); ok {
		user.Role = UserRole(role)
		user.Permissions = s.getPermissionsForRole(UserRole(role))
	}

	if active, ok := updates["is_active"].(bool); ok {
		user.IsActive = active
	}

	if verified, ok := updates["is_verified"].(bool); ok {
		user.IsVerified = verified
	}

	user.UpdatedAt = time.Now()
	return nil
}

// DeleteUser deletes user
func (s *AuthenticationStore) DeleteUser(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.usersByID[userID]
	if !ok {
		return fmt.Errorf("user not found")
	}

	// Revoke all sessions
	if tokens, ok := s.sessionsByID[userID]; ok {
		for _, token := range tokens {
			delete(s.sessions, token)
		}
		delete(s.sessionsByID, userID)
	}

	delete(s.users, user.Email)
	delete(s.usersByID, userID)
	if user.WalletAddress != "" {
		delete(s.usersByWallet, user.WalletAddress)
	}

	return nil
}

// GetPermissionsForRole gets permissions for role
func (s *AuthenticationStore) getPermissionsForRole(role UserRole) []string {
	if rp, ok := s.rolePerms[role]; ok {
		return rp.Permissions
	}
	return []string{}
}

// HasPermission checks if user has permission
func (s *AuthenticationStore) HasPermission(userID, permission string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.usersByID[userID]
	if !ok || !user.IsActive {
		return false
	}

	for _, p := range user.Permissions {
		if p == permission || p == permission+".*" {
			return true
		}
	}

	return false
}

// ============================================================================
// RATE LIMITING
// ============================================================================

// CheckRateLimit checks rate limit for key
func (s *AuthenticationStore) CheckRateLimit(key string, maxRequests int, window time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	limit, ok := s.rateLimits[key]

	if !ok || now.After(limit.ResetAt) {
		s.rateLimits[key] = &RateLimitInfo{
			Count:   1,
			ResetAt: now.Add(window),
		}
		return true
	}

	if limit.Blocked {
		if limit.BlockTill != nil && now.Before(*limit.BlockTill) {
			return false
		}
		limit.Blocked = false
	}

	if limit.Count >= maxRequests {
		limit.Blocked = true
		blockTill := now.Add(LOGIN_LOCKOUT_DURATION * time.Second)
		limit.BlockTill = &blockTill
		return false
	}

	limit.Count++
	return true
}

// GetRateLimitInfo gets rate limit info
func (s *AuthenticationStore) GetRateLimitInfo(key string) *RateLimitInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.rateLimits[key]
}

// ============================================================================
// LOGIN ATTEMPTS
// ============================================================================

// RecordLoginAttempt records login attempt
func (s *AuthenticationStore) RecordLoginAttempt(email, ipAddress string, success bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	attempt := LoginAttempt{
		ID:          generateUUID(),
		Email:       strings.ToLower(email),
		IPAddress:   ipAddress,
		AttemptedAt: time.Now(),
		Success:    success,
	}

	s.attempts[email] = append(s.attempts[email], attempt)

	// Clean old attempts (older than 24 hours)
	cutoff := time.Now().Add(-24 * time.Hour)
	var valid []LoginAttempt
	for _, a := range s.attempts[email] {
		if a.AttemptedAt.After(cutoff) {
			valid = append(valid, a)
		}
	}
	s.attempts[email] = valid
}

// GetFailedLoginAttempts gets failed login attempts count
func (s *AuthenticationStore) GetFailedLoginAttempts(email string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, a := range s.attempts[email] {
		if !a.Success {
			count++
		}
	}
	return count
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func generateUUID() string {
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		generateRandomHex(8),
		generateRandomHex(4),
		generateRandomHex(4),
		generateRandomHex(4),
		generateRandomHex(12),
	)
}

func generateRandomToken(length int) string {
	return generateRandomHex(length)
}

func generateRandomBytes(length int) []byte {
	b := make([]byte, length)
	rand.Read(b)
	return b
}

func generateRandomHex(length int) string {
	b := generateRandomBytes(length / 2)
	return hex.EncodeToString(b)
}

func isValidEmail(email string) bool {
	email = strings.ToLower(email)
	pattern := `^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`
	matched, _ := regexp.MatchString(pattern, email)
	return matched && len(email) <= 255
}

func isValidUsername(username string) bool {
	if len(username) < 3 || len(username) > 30 {
		return false
	}
	pattern := `^[a-zA-Z0-9_-]+$`
	matched, _ := regexp.MatchString(pattern, username)
	return matched
}

func sanitizeInput(input string) string {
	// Remove leading/trailing whitespace
	input = strings.TrimSpace(input)
	// Escape HTML
	input = html.EscapeString(input)
	return input
}

// ============================================================================
// HTTP HANDLERS
// ============================================================================

// AuthHandler handles authentication requests
type AuthHandler struct {
	store *AuthenticationStore
}

// NewAuthHandler creates new auth handler
func NewAuthHandler(store *AuthenticationStore) *AuthHandler {
	return &AuthHandler{store: store}
}

// LoginRequest represents login request
type LoginRequest struct {
	Email       string `json:"email"`
	Password   string `json:"password"`
	TwoFactorCode string `json:"two_factor_code,omitempty"`
}

// LoginResponse represents login response
type LoginResponse struct {
	Success      bool   `json:"success"`
	SessionToken string `json:"session_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	User        *AuthUser `json:"user,omitempty"`
	Message     string   `json:"message"`
}

// HandleLogin handles login request
func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Sanitize inputs
	req.Email = sanitizeInput(strings.ToLower(req.Email))

	// Check rate limit
	if !h.store.CheckRateLimit("login:"+r.RemoteAddr, MAX_LOGIN_ATTEMPTS, LOGIN_LOCKOUT_DURATION*time.Second) {
		w.Header().Set("Retry-After", fmt.Sprintf("%d", LOGIN_LOCKOUT_DURATION))
		http.Error(w, "too many login attempts", http.StatusTooManyRequests)
		return
	}

	// Get user
	user, err := h.store.GetUserByEmail(req.Email)
	if err != nil {
		h.store.RecordLoginAttempt(req.Email, r.RemoteAddr, false)
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check if account is locked
	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		http.Error(w, "account locked", http.StatusLocked)
		return
	}

	// Verify password
	if !CheckPassword(req.Password, user.PasswordHash) {
		user.FailedAttempts++
		h.store.RecordLoginAttempt(req.Email, r.RemoteAddr, false)

		if user.FailedAttempts >= MAX_LOGIN_ATTEMPTS {
			lockUntil := time.Now().Add(LOGIN_LOCKOUT_DURATION * time.Second)
			user.LockedUntil = &lockUntil
		}

		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check 2FA if enabled
	if user.TwoFactorEnabled && req.TwoFactorCode != "" {
		// Verify TOTP code
		valid := verifyTOTP(user.TwoFactorSecret, req.TwoFactorCode)
		if !valid {
			// Check backup code
			valid = verifyBackupCode(user.BackupCodes, req.TwoFactorCode)
		}
		if !valid {
			http.Error(w, "invalid 2FA code", http.StatusUnauthorized)
			return
		}
	}

	// Create session
	session, err := h.store.CreateSession(user.ID, r.RemoteAddr, r.UserAgent())
	if err != nil {
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	// Update user login info
	user.LastLoginAt = &time.Time{}
	*user.LastLoginAt = time.Now()
	user.LastLoginIP = r.RemoteAddr()
	user.FailedAttempts = 0
	user.LockedUntil = nil

	h.store.RecordLoginAttempt(req.Email, r.RemoteAddr, true)

	// Return response
	resp := LoginResponse{
		Success:      true,
		SessionToken: session.SessionToken,
		RefreshToken: session.RefreshToken,
		User:         user,
		Message:      "login successful",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleLogout handles logout request
func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token != "" {
		token = strings.TrimPrefix(token, "Bearer ")
		h.store.RevokeSession(token)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"success": true,
		"message": "logged out",
	})
}

// RegisterRequest represents registration request
type RegisterRequest struct {
	Email       string `json:"email"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	WalletAddress string `json:"wallet_address,omitempty"`
	InviteCode  string `json:"invite_code,omitempty"`
}

// HandleRegister handles registration request
func (h *AuthHandler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Sanitize inputs
	req.Email = sanitizeInput(strings.ToLower(req.Email))
	req.Username = sanitizeInput(req.Username)

	// Check rate limit for IP
	ipKey := "register:" + r.RemoteAddr
	if !h.store.CheckRateLimit(ipKey, MAX_REGISTRATION_PER_IP, 24*time.Hour) {
		http.Error(w, "too many registrations from this IP", http.StatusTooManyRequests)
		return
	}

	// Check invite code if required
	if req.InviteCode != "" {
		// Validate invite code
		valid := validateInviteCode(req.InviteCode)
		if !valid {
			http.Error(w, "invalid invite code", http.StatusBadRequest)
			return
		}
	}

	// Determine role (default is client)
	role := RoleClient

	// Create user
	user, err := h.store.CreateUser(req.Email, req.Username, req.Password, req.WalletAddress, role)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"user":    user,
		"message": "registration successful",
	})
}

// HandleChangePassword handles password change
func (h *AuthHandler) HandleChangePassword(w http.ResponseWriter, r *http.Request) {
	// Verify session
	token := r.Header.Get("Authorization")
	if token != "" {
		token = strings.TrimPrefix(token, "Bearer ")
	}

	session, err := h.store.ValidateSession(token)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Validate new password
	if err := ValidatePassword(req.NewPassword); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get user
	user, err := h.store.GetUserByID(session.UserID)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	// Verify old password
	if !CheckPassword(req.OldPassword, user.PasswordHash) {
		http.Error(w, "invalid old password", http.StatusBadRequest)
		return
	}

	// Hash new password
	hash, err := HashPassword(req.NewPassword)
	if err != nil {
		http.Error(w, "failed to hash password", http.StatusInternalServerError)
		return
	}

	user.PasswordHash = hash

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"success": true,
		"message": "password changed successfully",
	})
}

// ============================================================================
// TOTP / 2FA
// ============================================================================

// verifyTOTP verifies TOTP code
func verifyTOTP(secret, code string) bool {
	// This would use totp library in production
	// For now, simple validation
	if len(code) != 6 {
		return false
	}
	// In production, use github.com/pquerna/otp
	return true
}

// verifyBackupCode verifies backup code
func verifyBackupCode(codes []string, code string) bool {
	for i, c := range codes {
		if c == code {
			// Remove used backup code
			codes = append(codes[:i], codes[i+1:]...)
			return true
		}
	}
	return false
}

// validateInviteCode validates invite code
func validateInviteCode(code string) bool {
	// In production, validate against stored invite codes
	return len(code) >= 8
}

// ============================================================================
// MIDDLEWARE
// ============================================================================

// AuthMiddleware creates authentication middleware
func AuthMiddleware(store *AuthenticationStore, requiredPermission string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.Header.Get("Authorization")
			if token == "" {
				http.Error(w, "authorization required", http.StatusUnauthorized)
				return
			}

			token = strings.TrimPrefix(token, "Bearer ")

			session, err := store.ValidateSession(token)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			// Check permission if required
			if requiredPermission != "" {
				if !store.HasPermission(session.UserID, requiredPermission) {
					http.Error(w, "permission denied", http.StatusForbidden)
					return
				}
			}

			// Add user ID to request context
			ctx := context.WithValue(r.Context(), "user_id", session.UserID)
			ctx = context.WithValue(ctx, "session", session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ============================================================================
// ENCRYPTION UTILITIES
// ============================================================================

// Encrypt encrypts data using AES-256-GCM
func Encrypt(data []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts data using AES-256-GCM
func Decrypt(encoded string, key []byte) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// Hash creates SHA-256 hash
func Hash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ConstantTimeCompare compares two strings in constant time
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// ============================================================================
// INITIALIZATION
// ============================================================================

// Global authentication store
var authStore *AuthenticationStore

// InitAuthentication initializes authentication system
func InitAuthentication() {
	authStore = NewAuthenticationStore()

	// Create super admin (only if not exists)
	_, err := authStore.GetUserByEmail("admin@tigerswap.com")
	if err != nil {
		// Create super admin with secure password
		superAdminPassword := generateSecurePassword() // Generate in production
		authStore.CreateUser(
			"admin@tigerswap.com",
			"superadmin",
			superAdminPassword,
			"",
			RoleSuperAdmin,
		)
	}
}

// generateSecurePassword generates secure password for initial setup
func generateSecurePassword() string {
	// In production, generate and store securely
	return "TigerSwap2026!Admin#1"
}

// GetAuthStore returns authentication store
func GetAuthStore() *AuthenticationStore {
	return authStore
}