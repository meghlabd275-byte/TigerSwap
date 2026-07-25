// Package admin_service provides the TigerSwap Admin Management System
// Supports Super Admin and White Label Admin with comprehensive features
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

/* ============================================================================
   ADMIN SYSTEM TYPES
   ============================================================================ */

// AdminRole represents the admin role hierarchy
type AdminRole string

const (
	RoleSuperAdmin      AdminRole = "super_admin"      // Full platform access
	RoleWhiteLabelAdmin AdminRole = "white_label_admin" // White label client admin
	RoleProductAdmin    AdminRole = "product_admin"    // Product-specific admin
	RoleSupportAdmin    AdminRole = "support_admin"    // Support staff
	RoleFinanceAdmin   AdminRole = "finance_admin"    // Finance operations
	RoleComplianceAdmin AdminRole = "compliance_admin" // Compliance operations
)

// AdminStatus represents admin account status
type AdminStatus string

const (
	AdminStatusActive   AdminStatus = "active"
	AdminStatusInactive AdminStatus = "inactive"
	AdminStatusSuspended AdminStatus = "suspended"
	AdminStatusPending  AdminStatus = "pending"
)

// Admin represents an admin user
type Admin struct {
	ID               uuid.UUID            `json:"id" gorm:"primaryKey;type:uuid"`
	Email            string               `json:"email" gorm:"uniqueIndex;not null"`
	PasswordHash     string               `json:"-" gorm:"not null"`
	Role             AdminRole            `json:"role" gorm:"not null"`
	Status           AdminStatus          `json:"status" gorm:"default:active"`
	Name             string               `json:"name"`
	Phone            string               `json:"phone"`
	Permissions      []AdminPermission    `json:"permissions" gorm:"many2many:admin_permissions;"`
	WhiteLabelID     *uuid.UUID          `json:"white_label_id,omitempty"`
	CreatedBy        *uuid.UUID          `json:"created_by"`
	CreatedAt        time.Time           `json:"created_at"`
	UpdatedAt        time.Time           `json:"updated_at"`
	LastLoginAt      *time.Time          `json:"last_login_at"`
	LoginAttempts    int                 `json:"login_attempts"`
	LockedUntil      *time.Time          `json:"locked_until"`
	TwoFactorEnabled bool                `json:"two_factor_enabled"`
	TwoFactorSecret string              `json:"-"`
	APIKey           string               `json:"api_key"`
	IPWhitelist      []string            `json:"ip_whitelist"`
}

// AdminPermission represents admin permissions
type AdminPermission struct {
	ID          uuid.UUID `json:"id" gorm:"primaryKey;type:uuid"`
	Name        string   `json:"name" gorm:"uniqueIndex"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	CreatedAt   time.Time `json:"created_at"`
}

// AdminActivity represents admin activity log
type AdminActivity struct {
	ID          uuid.UUID `json:"id" gorm:"primaryKey;type:uuid"`
	AdminID     uuid.UUID `json:"admin_id" gorm:"type:uuid;index"`
	Action      string    `json:"action"`
	Resource    string    `json:"resource"`
	ResourceID  string    `json:"resource_id"`
	Details     string    `json:"details"`
	IPAddress   string    `json:"ip_address"`
	UserAgent   string    `json:"user_agent"`
	CreatedAt   time.Time `json:"created_at"`
}

// WhiteLabel represents a white label client
type WhiteLabel struct {
	ID               uuid.UUID        `json:"id" gorm:"primaryKey;type:uuid"`
	Name             string           `json:"name" gorm:"not null"`
	Domain           string           `json:"domain" gorm:"uniqueIndex"`
	CustomBranding   bool             `json:"custom_branding"`
	LogoURL          string           `json:"logo_url"`
	PrimaryColor     string           `json:"primary_color"`
	SecondaryColor   string           `json:"secondary_color"`
	Status           string           `json:"status"`
	Features         []string         `json:"features"`
	BlockchainAccess []int64          `json:"blockchain_access"`
	FeeStructure    *FeeStructure    `json:"fee_structure"`
	CreatedAt        time.Time        `json:"created_at"`
	UpdatedAt        time.Time        `json:"updated_at"`
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
	db         *gorm.DB
	redis      *redis.Client
	jwtSecret  string
	permissions map[string]*AdminPermission
	mu         sync.RWMutex
}

// NewAdminService creates a new admin service
func NewAdminService(db *gorm.DB, redisClient *redis.Client) *AdminService {
	svc := &AdminService{
		db:          db,
		redis:       redisClient,
		jwtSecret:   getEnv("JWT_SECRET", "tigerswap-secret-key"),
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
	admin.APIKey = generateAPIKey()

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
	TotalUsers            int64             `json:"total_users"`
	ActiveUsers24h        int64             `json:"active_users_24h"`
	TotalVolume24h        float64           `json:"total_volume_24h"`
	TotalVolume7d         float64           `json:"total_volume_7d"`
	TotalLiquidity        float64           `json:"total_liquidity"`
	TotalTransactions     int64             `json:"total_transactions"`
	Transactions24h       int64             `json:"transactions_24h"`
	PendingWithdrawals    int64             `json:"pending_withdrawals"`
	PendingDeposits      int64             `json:"pending_deposits"`
	TotalFeesCollected   float64           `json:"total_fees_collected"`
	TopTradingPairs      []PairVolume      `json:"top_trading_pairs"`
	TopTokens            []TokenVolume     `json:"top_tokens"`
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
		TotalUsers:          0,
		ActiveUsers24h:      0,
		TotalVolume24h:      0,
		TotalVolume7d:       0,
		TotalLiquidity:      0,
		TotalTransactions:   0,
		Transactions24h:     0,
		PendingWithdrawals:  0,
		PendingDeposits:    0,
		TotalFeesCollected: 0,
	}

	// In production, these would query the database
	// For now, return mock data
	stats.TopTradingPairs = []PairVolume{
		{Pair: "ETH/USDC", Volume: 50000000},
		{Pair: "BTC/USDC", Volume: 75000000},
		{Pair: "ETH/USDT", Volume: 30000000},
	}

	stats.TopTokens = []TokenVolume{
		{Token: "ETH", Volume: 100000000},
		{Token: "BTC", Volume: 75000000},
		{Token: "USDC", Volume: 50000000},
	}

	stats.BlockchainDistribution = map[string]float64{
		"Ethereum":   45.5,
		"BNB Chain":  25.3,
		"Polygon":    15.2,
		"Arbitrum":   8.5,
		"Other":      5.5,
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

// Login handles admin login
func (h *Handler) Login(c *gin.Context) {
	var req struct {
		Email    string `json:"email" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	admin, err := h.adminService.Authenticate(c.Request.Context(), req.Email, req.Password, c.ClientIP())
	if err != nil {
		c.JSON(401, gin.H{"error": err.Error()})
		return
	}

	// Generate JWT token
	token, err := generateJWT(admin.ID, admin.Role, h.adminService.jwtSecret)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate token"})
		return
	}

	c.JSON(200, gin.H{
		"token": token,
		"admin": admin,
	})
}

// GetStats returns platform statistics
func (h *Handler) GetStats(c *gin.Context) {
	stats, err := h.adminService.GetPlatformStats()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, stats)
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

func isValidEmail(email string) bool {
	return strings.Contains(email, "@") && strings.Contains(email, ".")
}

func generateAPIKey() string {
	hash := sha256.Sum256([]byte(time.Now().String() + uuid.New().String()))
	return "tsk_" + hex.EncodeToString(hash[:])
}

func generateJWT(adminID uuid.UUID, role AdminRole, secret string) (string, error) {
	// In production, use proper JWT library
	return fmt.Sprintf("jwt_token_%s_%s", adminID, role), nil
}

/* ============================================================================
   MAIN
   ============================================================================ */

func main() {
	// Initialize database (mock for now)
	log.Println("Starting Admin Service...")

	// Set up Gin
	r := gin.Default()

	// Public routes
	r.POST("/api/v1/admin/login", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "message": "Admin login endpoint"})
	})

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "admin-service"})
	})

	// Start server
	go func() {
		log.Println("Starting Admin Service on :8090")
		if err := r.Run(":8090"); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Admin Service...")
}
