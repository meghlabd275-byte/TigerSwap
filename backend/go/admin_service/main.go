// Package admin_service provides the TigerSwap Admin Management System
// Supports Super Admin and White Label Admin with comprehensive features
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"net/mail"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"tigerswap/backend/config"
	"tigerswap/backend/models"
)

/* ============================================================================
   ADMIN SYSTEM TYPES
   ============================================================================ */

// AdminRole represents the admin role hierarchy
type AdminRole string

const (
	RoleSuperAdmin      AdminRole = "super_admin"       // Full platform access
	RoleWhiteLabelAdmin AdminRole = "white_label_admin" // White label client admin
	RoleProductAdmin    AdminRole = "product_admin"     // Product-specific admin
	RoleSupportAdmin    AdminRole = "support_admin"     // Support staff
	RoleFinanceAdmin    AdminRole = "finance_admin"     // Finance operations
	RoleComplianceAdmin AdminRole = "compliance_admin"  // Compliance operations
)

// AdminStatus represents admin account status
type AdminStatus string

const (
	AdminStatusActive    AdminStatus = "active"
	AdminStatusInactive  AdminStatus = "inactive"
	AdminStatusSuspended AdminStatus = "suspended"
	AdminStatusPending   AdminStatus = "pending"
)

// Admin represents an admin user
type Admin struct {
	ID               uuid.UUID         `json:"id" gorm:"primaryKey;type:uuid"`
	Email            string            `json:"email" gorm:"uniqueIndex;not null"`
	PasswordHash     string            `json:"-" gorm:"not null"`
	Role             AdminRole         `json:"role" gorm:"not null"`
	Status           AdminStatus       `json:"status" gorm:"default:active"`
	Name             string            `json:"name"`
	Phone            string            `json:"phone"`
	Permissions      []AdminPermission `json:"permissions" gorm:"many2many:admin_permissions;"`
	WhiteLabelID     *uuid.UUID        `json:"white_label_id,omitempty"`
	CreatedBy        *uuid.UUID        `json:"created_by"`
	CreatedAt        time.Time         `json:"created_at"`
	UpdatedAt        time.Time         `json:"updated_at"`
	LastLoginAt      *time.Time        `json:"last_login_at"`
	LoginAttempts    int               `json:"login_attempts"`
	LockedUntil      *time.Time        `json:"locked_until"`
	TwoFactorEnabled bool              `json:"two_factor_enabled"`
	TwoFactorSecret  string            `json:"-"`
	APIKey           string            `json:"api_key"`
	IPWhitelist      []string          `json:"ip_whitelist"`
}

// AdminPermission represents admin permissions
type AdminPermission struct {
	ID          uuid.UUID `json:"id" gorm:"primaryKey;type:uuid"`
	Name        string    `json:"name" gorm:"uniqueIndex"`
	Description string    `json:"description"`
	Category    string    `json:"category"`
	CreatedAt   time.Time `json:"created_at"`
}

// AdminActivity represents admin activity log
type AdminActivity struct {
	ID         uuid.UUID `json:"id" gorm:"primaryKey;type:uuid"`
	AdminID    uuid.UUID `json:"admin_id" gorm:"type:uuid;index"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource"`
	ResourceID string    `json:"resource_id"`
	Details    string    `json:"details"`
	IPAddress  string    `json:"ip_address"`
	UserAgent  string    `json:"user_agent"`
	CreatedAt  time.Time `json:"created_at"`
}

// WhiteLabel represents a white label client
type WhiteLabel struct {
	ID               uuid.UUID     `json:"id" gorm:"primaryKey;type:uuid"`
	Name             string        `json:"name" gorm:"not null"`
	Domain           string        `json:"domain" gorm:"uniqueIndex"`
	CustomBranding   bool          `json:"custom_branding"`
	LogoURL          string        `json:"logo_url"`
	PrimaryColor     string        `json:"primary_color"`
	SecondaryColor   string        `json:"secondary_color"`
	Status           string        `json:"status"`
	Features         []string      `json:"features"`
	BlockchainAccess []int64       `json:"blockchain_access"`
	FeeStructure     *FeeStructure `json:"fee_structure"`
	CreatedAt        time.Time     `json:"created_at"`
	UpdatedAt        time.Time     `json:"updated_at"`
}

// FeeStructure represents custom fee structure for white label
type FeeStructure struct {
	SwapFee        float64 `json:"swap_fee"`
	WithdrawFee    float64 `json:"withdraw_fee"`
	DepositFee     float64 `json:"deposit_fee"`
	TransactionFee float64 `json:"transaction_fee"`
}

/* ============================================================================
   ADMIN SERVICE
   ============================================================================ */

// AdminService handles admin operations
type AdminService struct {
	db          *gorm.DB
	redis       *redis.Client
	jwtSecret   string
	permissions map[string]*AdminPermission
	mu          sync.RWMutex
}

// NewAdminService creates a new admin service
func NewAdminService(db *gorm.DB, redisClient *redis.Client) *AdminService {
	svc := &AdminService{
		db:          db,
		redis:       redisClient,
		jwtSecret:   os.Getenv("JWT_SECRET"),
		permissions: make(map[string]*AdminPermission),
	}
	svc.initDefaultPermissions()
	return svc
}

// initDefaultPermissions initializes default admin permissions
func (s *AdminService) initDefaultPermissions() {
	defaultPermissions := []AdminPermission{
		// User Management
		{Name: "users.view", Description: "View user list and details", Category: "users"},
		{Name: "users.create", Description: "Create new users", Category: "users"},
		{Name: "users.edit", Description: "Edit user details", Category: "users"},
		{Name: "users.delete", Description: "Delete users", Category: "users"},
		{Name: "users.kyc", Description: "Manage KYC", Category: "users"},

		// Admin Management
		{Name: "admins.view", Description: "View admin list", Category: "admins"},
		{Name: "admins.create", Description: "Create admins", Category: "admins"},
		{Name: "admins.edit", Description: "Edit admins", Category: "admins"},
		{Name: "admins.delete", Description: "Delete admins", Category: "admins"},
		{Name: "admins.permissions", Description: "Manage permissions", Category: "admins"},

		// White Label Management
		{Name: "whitelabel.view", Description: "View white labels", Category: "whitelabel"},
		{Name: "whitelabel.create", Description: "Create white labels", Category: "whitelabel"},
		{Name: "whitelabel.edit", Description: "Edit white labels", Category: "whitelabel"},
		{Name: "whitelabel.delete", Description: "Delete white labels", Category: "whitelabel"},

		// Blockchain Management
		{Name: "blockchain.view", Description: "View blockchains", Category: "blockchain"},
		{Name: "blockchain.add", Description: "Add blockchains", Category: "blockchain"},
		{Name: "blockchain.edit", Description: "Edit blockchains", Category: "blockchain"},
		{Name: "blockchain.delete", Description: "Delete blockchains", Category: "blockchain"},

		// Token Management
		{Name: "tokens.view", Description: "View tokens", Category: "tokens"},
		{Name: "tokens.add", Description: "Add tokens", Category: "tokens"},
		{Name: "tokens.edit", Description: "Edit tokens", Category: "tokens"},
		{Name: "tokens.delete", Description: "Delete tokens", Category: "tokens"},
		{Name: "tokens.verify", Description: "Verify tokens", Category: "tokens"},

		// Trading Pairs
		{Name: "pairs.view", Description: "View trading pairs", Category: "pairs"},
		{Name: "pairs.create", Description: "Create trading pairs", Category: "pairs"},
		{Name: "pairs.edit", Description: "Edit trading pairs", Category: "pairs"},
		{Name: "pairs.delete", Description: "Delete trading pairs", Category: "pairs"},
		{Name: "pairs.suspend", Description: "Suspend trading pairs", Category: "pairs"},

		// Liquidity
		{Name: "liquidity.view", Description: "View liquidity", Category: "liquidity"},
		{Name: "liquidity.add", Description: "Add liquidity", Category: "liquidity"},
		{Name: "liquidity.remove", Description: "Remove liquidity", Category: "liquidity"},
		{Name: "liquidity.import", Description: "Import liquidity", Category: "liquidity"},

		// Fees
		{Name: "fees.view", Description: "View fees", Category: "fees"},
		{Name: "fees.edit", Description: "Edit fees", Category: "fees"},

		// Orders & Transactions
		{Name: "orders.view", Description: "View orders", Category: "orders"},
		{Name: "orders.cancel", Description: "Cancel orders", Category: "orders"},
		{Name: "transactions.view", Description: "View transactions", Category: "transactions"},

		// Withdrawal Management
		{Name: "withdrawals.view", Description: "View withdrawals", Category: "withdrawals"},
		{Name: "withdrawals.approve", Description: "Approve withdrawals", Category: "withdrawals"},
		{Name: "withdrawals.reject", Description: "Reject withdrawals", Category: "withdrawals"},
		{Name: "withdrawals.process", Description: "Process withdrawals", Category: "withdrawals"},

		// Deposits
		{Name: "deposits.view", Description: "View deposits", Category: "deposits"},

		// Analytics
		{Name: "analytics.view", Description: "View analytics", Category: "analytics"},

		// Settings
		{Name: "settings.view", Description: "View settings", Category: "settings"},
		{Name: "settings.edit", Description: "Edit settings", Category: "settings"},

		// System
		{Name: "system.health", Description: "View system health", Category: "system"},
		{Name: "system.logs", Description: "View system logs", Category: "system"},
	}

	for _, p := range defaultPermissions {
		s.permissions[p.Name] = &p
	}
}

// CreateAdmin creates a new admin
func (s *AdminService) CreateAdmin(ctx context.Context, admin *Admin, creatorID uuid.UUID) error {
	// Validate email
	if !isValidEmail(admin.Email) {
		return fmt.Errorf("invalid email format")
	}

	// Check if email already exists
	var count int64
	if err := s.db.Model(&Admin{}).Where("email = ?", admin.Email).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("email already exists")
	}

	// Validate role permissions
	if admin.Role == RoleSuperAdmin && !s.isSuperAdmin(creatorID) {
		return fmt.Errorf("only super admin can create super admins")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(admin.PasswordHash), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	admin.PasswordHash = string(hashedPassword)

	// Generate API key
	apiKey, err := generateAPIKey()
	if err != nil {
		return err
	}
	admin.APIKey = apiKey

	// Set defaults
	if admin.Status == "" {
		admin.Status = AdminStatusActive
	}
	if admin.ID == uuid.Nil {
		admin.ID = uuid.New()
	}

	// Log activity
	s.logActivity(ctx, creatorID, "admin.create", "admin", admin.ID.String(),
		fmt.Sprintf("Created admin: %s with role: %s", admin.Email, admin.Role))

	return s.db.Create(admin).Error
}

// Authenticate authenticates an admin
func (s *AdminService) Authenticate(ctx context.Context, email, password, ipAddress string) (*Admin, error) {
	var admin Admin
	if err := s.db.Where("email = ?", email).First(&admin).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("invalid credentials")
		}
		return nil, err
	}

	// Check if locked
	if admin.LockedUntil != nil && admin.LockedUntil.After(time.Now()) {
		return nil, fmt.Errorf("account locked until %s", admin.LockedUntil.Format(time.RFC3339))
	}

	// Check status
	if admin.Status != AdminStatusActive {
		return nil, fmt.Errorf("account is %s", admin.Status)
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(admin.PasswordHash), []byte(password)); err != nil {
		// Increment failed attempts
		admin.LoginAttempts++
		if admin.LoginAttempts >= 5 {
			lockedUntil := time.Now().Add(15 * time.Minute)
			admin.LockedUntil = &lockedUntil
		}
		s.db.Save(&admin)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Reset failed attempts
	admin.LoginAttempts = 0
	admin.LastLoginAt = new(time.Time)
	*admin.LastLoginAt = time.Now()
	s.db.Save(&admin)

	// Log activity
	s.logActivity(ctx, admin.ID, "admin.login", "admin", admin.ID.String(), "Login successful")

	return &admin, nil
}

// HasPermission checks if admin has a specific permission
func (s *AdminService) HasPermission(adminID uuid.UUID, permission string) bool {
	var admin Admin
	if err := s.db.Preload("Permissions").First(&admin, adminID).Error; err != nil {
		return false
	}

	// Super admin has all permissions
	if admin.Role == RoleSuperAdmin {
		return true
	}

	for _, p := range admin.Permissions {
		if p.Name == permission {
			return true
		}
	}
	return false
}

// logActivity logs admin activity
func (s *AdminService) logActivity(ctx context.Context, adminID uuid.UUID, action, resource, resourceID, details string) {
	activity := AdminActivity{
		ID:         uuid.New(),
		AdminID:    adminID,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Details:    details,
		CreatedAt:  time.Now(),
	}

	// Get IP from context if available
	if ip, ok := ctx.Value("ip_address").(string); ok {
		activity.IPAddress = ip
	}

	s.db.Create(&activity)
}

// isSuperAdmin checks if user is a super admin
func (s *AdminService) isSuperAdmin(adminID uuid.UUID) bool {
	var admin Admin
	if err := s.db.First(&admin, adminID).Error; err != nil {
		return false
	}
	return admin.Role == RoleSuperAdmin
}

/* ============================================================================
   WHITE LABEL MANAGEMENT
   ============================================================================ */

// CreateWhiteLabel creates a new white label
func (s *AdminService) CreateWhiteLabel(ctx context.Context, wl *WhiteLabel, adminID uuid.UUID) error {
	if wl.ID == uuid.Nil {
		wl.ID = uuid.New()
	}
	if wl.Status == "" {
		wl.Status = "active"
	}

	s.logActivity(ctx, adminID, "whitelabel.create", "white_label", wl.ID.String(),
		fmt.Sprintf("Created white label: %s (%s)", wl.Name, wl.Domain))

	return s.db.Create(wl).Error
}

// UpdateWhiteLabel updates a white label
func (s *AdminService) UpdateWhiteLabel(ctx context.Context, id uuid.UUID, updates map[string]interface{}, adminID uuid.UUID) error {
	s.logActivity(ctx, adminID, "whitelabel.edit", "white_label", id.String(), "Updated white label")
	return s.db.Model(&WhiteLabel{}).Where("id = ?", id).Updates(updates).Error
}

// GetWhiteLabel gets a white label by ID
func (s *AdminService) GetWhiteLabel(id uuid.UUID) (*WhiteLabel, error) {
	var wl WhiteLabel
	if err := s.db.First(&wl, id).Error; err != nil {
		return nil, err
	}
	return &wl, nil
}

// ListWhiteLabels lists all white labels
func (s *AdminService) ListWhiteLabels(status string, page, pageSize int) ([]WhiteLabel, int64, error) {
	var whiteLabels []WhiteLabel
	var total int64

	query := s.db.Model(&WhiteLabel{})
	if status != "" {
		query = query.Where("status = ?", status)
	}

	query.Count(&total)
	query.Offset((page - 1) * pageSize).Limit(pageSize)

	return whiteLabels, total, query.Find(&whiteLabels).Error
}

/* ============================================================================
   PLATFORM STATISTICS
   ============================================================================ */

// PlatformStats represents platform statistics
type PlatformStats struct {
	TotalUsers             int64              `json:"total_users"`
	ActiveUsers24h         int64              `json:"active_users_24h"`
	TotalVolume24h         float64            `json:"total_volume_24h"`
	TotalVolume7d          float64            `json:"total_volume_7d"`
	TotalLiquidity         float64            `json:"total_liquidity"`
	TotalTransactions      int64              `json:"total_transactions"`
	Transactions24h        int64              `json:"transactions_24h"`
	PendingWithdrawals     int64              `json:"pending_withdrawals"`
	PendingDeposits        int64              `json:"pending_deposits"`
	TotalFeesCollected     float64            `json:"total_fees_collected"`
	TopTradingPairs        []PairVolume       `json:"top_trading_pairs"`
	TopTokens              []TokenVolume      `json:"top_tokens"`
	BlockchainDistribution map[string]float64 `json:"blockchain_distribution"`
}

// PairVolume represents trading volume for a pair
type PairVolume struct {
	Pair   string  `json:"pair"`
	Volume float64 `json:"volume"`
}

// TokenVolume represents trading volume for a token
type TokenVolume struct {
	Token  string  `json:"token"`
	Volume float64 `json:"volume"`
}

// GetPlatformStats returns platform statistics
func (s *AdminService) GetPlatformStats() (*PlatformStats, error) {
	stats := &PlatformStats{
		TotalUsers:         0,
		ActiveUsers24h:     0,
		TotalVolume24h:     0,
		TotalVolume7d:      0,
		TotalLiquidity:     0,
		TotalTransactions:  0,
		Transactions24h:    0,
		PendingWithdrawals: 0,
		PendingDeposits:    0,
		TotalFeesCollected: 0,
	}

	if s.db == nil {
		return nil, fmt.Errorf("database not configured")
	}

	since := time.Now().Add(-24 * time.Hour)
	weekAgo := time.Now().Add(-7 * 24 * time.Hour)

	if err := s.db.Model(&models.User{}).Count(&stats.TotalUsers).Error; err != nil {
		return nil, fmt.Errorf("count users: %w", err)
	}
	if err := s.db.Model(&Admin{}).Where("last_login_at >= ?", since).Count(&stats.ActiveUsers24h).Error; err != nil {
		return nil, fmt.Errorf("count active admins: %w", err)
	}

	var agg struct {
		Volume24h float64
		Volume7d  float64
		Txs24h    int64
		TxsTotal  int64
	}
	if err := s.db.Model(&models.Transaction{}).
		Select("COALESCE(SUM(CASE WHEN created_at >= ? THEN amount_usd ELSE 0 END), 0) AS volume24h, "+
			"COALESCE(SUM(CASE WHEN created_at >= ? THEN amount_usd ELSE 0 END), 0) AS volume7d, "+
			"COALESCE(SUM(CASE WHEN created_at >= ? THEN 1 ELSE 0 END), 0) AS txs24h, "+
			"COUNT(*) AS txs_total", since, weekAgo, since).
		Scan(&agg).Error; err != nil {
		return nil, fmt.Errorf("aggregate transactions: %w", err)
	}
	stats.TotalVolume24h = agg.Volume24h
	stats.TotalVolume7d = agg.Volume7d
	stats.Transactions24h = agg.Txs24h
	stats.TotalTransactions = agg.TxsTotal

	if err := s.db.Model(&models.Transaction{}).
		Where("type = ? AND status = ?", "withdrawal", "pending").
		Count(&stats.PendingWithdrawals).Error; err != nil {
		return nil, fmt.Errorf("count pending withdrawals: %w", err)
	}
	if err := s.db.Model(&models.Transaction{}).
		Where("type = ? AND status = ?", "deposit", "pending").
		Count(&stats.PendingDeposits).Error; err != nil {
		return nil, fmt.Errorf("count pending deposits: %w", err)
	}

	if err := s.db.Model(&models.Pool{}).
		Select("COALESCE(SUM(tvl_usd), 0)").
		Scan(&stats.TotalLiquidity).Error; err != nil {
		return nil, fmt.Errorf("sum pool liquidity: %w", err)
	}

	if err := s.db.Model(&models.Transaction{}).
		Select("pair AS pair, COALESCE(SUM(amount_usd), 0) AS volume").
		Where("created_at >= ?", since).
		Group("pair").
		Order("volume DESC").
		Limit(10).
		Scan(&stats.TopTradingPairs).Error; err != nil {
		return nil, fmt.Errorf("top pairs: %w", err)
	}

	if err := s.db.Model(&models.Transaction{}).
		Select("to_token AS token, COALESCE(SUM(amount_usd), 0) AS volume").
		Where("created_at >= ?", since).
		Group("to_token").
		Order("volume DESC").
		Limit(10).
		Scan(&stats.TopTokens).Error; err != nil {
		return nil, fmt.Errorf("top tokens: %w", err)
	}

	var dist []struct {
		ChainID int64
		Count   int64
	}
	if err := s.db.Model(&models.Transaction{}).
		Select("chain_id, COUNT(*) AS count").
		Where("created_at >= ?", since).
		Group("chain_id").
		Scan(&dist).Error; err != nil {
		return nil, fmt.Errorf("blockchain distribution: %w", err)
	}
	var totalCount int64
	for _, d := range dist {
		totalCount += d.Count
	}
	stats.BlockchainDistribution = make(map[string]float64, len(dist))
	if totalCount > 0 {
		for _, d := range dist {
			stats.BlockchainDistribution[fmt.Sprintf("chain_%d", d.ChainID)] =
				float64(d.Count) / float64(totalCount) * 100
		}
	}

	return stats, nil
}

/* ============================================================================
   HTTP HANDLERS
   ============================================================================ */

type Handler struct {
	adminService *AdminService
}

func NewHandler(as *AdminService) *Handler {
	return &Handler{adminService: as}
}

/* ============================================================================
   UTILITY FUNCTIONS
   ============================================================================ */

func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

// isValidEmail validates the address with net/mail and rejects display-name
// forms so only bare addresses are accepted.
func isValidEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	if addr.Address != email {
		return false
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return false
	}
	domain := email[at+1:]
	return strings.Contains(domain, ".") && !strings.HasSuffix(domain, ".")
}

// generateAPIKey returns a cryptographically random 32-byte API key.
func generateAPIKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api key: %w", err)
	}
	return "tsk_" + hex.EncodeToString(buf), nil
}

// generateJWT issues a signed HS256 JWT for an authenticated admin.
func generateJWT(adminID uuid.UUID, role AdminRole, secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("JWT secret is not configured")
	}
	now := time.Now()
	claims := jwt.MapClaims{
		"sub":  adminID.String(),
		"role": string(role),
		"iat":  now.Unix(),
		"exp":  now.Add(12 * time.Hour).Unix(),
		"iss":  "tigerswap-admin",
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

/* ============================================================================
   MAIN
   ============================================================================ */

// bootstrapAdmin creates the first super-admin from environment variables when
// the admin table is empty. It refuses to start with a weak or missing
// password so the service never runs with a default credential.
func bootstrapAdmin(ctx context.Context, svc *AdminService) error {
	var count int64
	if err := svc.db.Model(&Admin{}).Count(&count).Error; err != nil {
		return fmt.Errorf("count admins: %w", err)
	}
	if count > 0 {
		return nil
	}

	email := os.Getenv("BOOTSTRAP_ADMIN_EMAIL")
	password := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if email == "" || password == "" {
		return fmt.Errorf("no admins exist and BOOTSTRAP_ADMIN_EMAIL/BOOTSTRAP_ADMIN_PASSWORD are not set")
	}
	if len(password) < 12 {
		return fmt.Errorf("BOOTSTRAP_ADMIN_PASSWORD must be at least 12 characters")
	}
	if !isValidEmail(email) {
		return fmt.Errorf("BOOTSTRAP_ADMIN_EMAIL is not a valid email")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash bootstrap password: %w", err)
	}

	admin := &Admin{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: string(hash),
		Role:         RoleSuperAdmin,
		Status:       AdminStatusActive,
		Name:         getEnv("BOOTSTRAP_ADMIN_NAME", "Super Admin"),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := svc.db.Create(admin).Error; err != nil {
		return fmt.Errorf("create bootstrap admin: %w", err)
	}
	log.Printf("Bootstrap super admin created: %s", email)
	return nil
}

// requireAuth validates the bearer JWT and stores the admin claims on the
// request context. It aborts with 401 when the token is missing or invalid.
func (h *Handler) requireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		secret := h.adminService.jwtSecret
		if secret == "" {
			c.AbortWithStatusJSON(500, gin.H{"error": "JWT secret is not configured"})
			return
		}

		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(401, gin.H{"error": "missing bearer token"})
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
			}
			return []byte(secret), nil
		}, jwt.WithIssuer("tigerswap-admin"), jwt.WithExpirationRequired())
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid or expired token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid token claims"})
			return
		}
		sub, _ := claims["sub"].(string)
		adminID, err := uuid.Parse(sub)
		if err != nil {
			c.AbortWithStatusJSON(401, gin.H{"error": "invalid subject"})
			return
		}

		c.Set("admin_id", adminID)
		if role, ok := claims["role"].(string); ok {
			c.Set("admin_role", AdminRole(role))
		}
		c.Next()
	}
}

func (h *Handler) handleLogin(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "email and password are required"})
		return
	}

	admin, err := h.adminService.Authenticate(c.Request.Context(), req.Email, req.Password, c.ClientIP())
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error()})
		return
	}

	token, err := generateJWT(admin.ID, admin.Role, h.adminService.jwtSecret)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(200, gin.H{"token": token, "admin": admin})
}

func (h *Handler) handleStats(c *gin.Context) {
	stats, err := h.adminService.GetPlatformStats()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, stats)
}

func (h *Handler) handleListWhiteLabels(c *gin.Context) {
	status := c.Query("status")
	page := 1
	pageSize := 50
	if p := c.Query("page"); p != "" {
		fmt.Sscanf(p, "%d", &page)
	}
	if ps := c.Query("page_size"); ps != "" {
		fmt.Sscanf(ps, "%d", &pageSize)
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}

	items, total, err := h.adminService.ListWhiteLabels(status, page, pageSize)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"items": items, "total": total, "page": page, "page_size": pageSize})
}

func (h *Handler) handleCreateWhiteLabel(c *gin.Context) {
	var wl WhiteLabel
	if err := c.ShouldBindJSON(&wl); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	adminID, _ := c.Get("admin_id")
	if err := h.adminService.CreateWhiteLabel(c.Request.Context(), &wl, adminID.(uuid.UUID)); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, wl)
}

func (h *Handler) handleUpdateWhiteLabel(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid id"})
		return
	}
	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	// Only brandable, non-identifying columns may be updated through the API.
	allowed := map[string]bool{
		"name": true, "domain": true, "custom_branding": true, "logo_url": true,
		"primary_color": true, "secondary_color": true, "status": true,
		"features": true, "blockchain_access": true,
	}
	for key := range updates {
		if !allowed[key] {
			c.JSON(400, gin.H{"error": fmt.Sprintf("field %q is not updatable", key)})
			return
		}
	}
	adminID, _ := c.Get("admin_id")
	if err := h.adminService.UpdateWhiteLabel(c.Request.Context(), id, updates, adminID.(uuid.UUID)); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, gin.H{"status": "updated"})
}

func main() {
	log.Println("Starting Admin Service...")

	db, err := config.InitDB()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	if err := db.AutoMigrate(
		&Admin{},
		&AdminPermission{},
		&AdminActivity{},
		&WhiteLabel{},
		&models.User{},
		&models.Token{},
		&models.Transaction{},
		&models.Pool{},
	); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	redisClient := config.InitRedis()
	adminService := NewAdminService(db, redisClient)
	if adminService.jwtSecret == "" {
		log.Fatal("JWT_SECRET must be configured for the admin service")
	}

	if err := bootstrapAdmin(context.Background(), adminService); err != nil {
		log.Fatalf("Admin bootstrap failed: %v", err)
	}

	handler := NewHandler(adminService)

	r := gin.New()
	r.Use(gin.Recovery(), gin.Logger())

	r.POST("/api/v1/admin/login", handler.handleLogin)
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "admin-service"})
	})

	authed := r.Group("/api/v1/admin", handler.requireAuth())
	authed.GET("/stats", handler.handleStats)
	authed.GET("/white-labels", handler.handleListWhiteLabels)
	authed.POST("/white-labels", handler.handleCreateWhiteLabel)
	authed.PUT("/white-labels/:id", handler.handleUpdateWhiteLabel)

	addr := getEnv("ADMIN_SERVICE_ADDR", ":8090")
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		log.Printf("Admin Service listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down Admin Service...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Admin Service shutdown error: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
}
