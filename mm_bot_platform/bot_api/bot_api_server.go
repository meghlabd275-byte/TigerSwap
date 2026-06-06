// TigerSwap Bot Platform API Server
// Complete REST API for bot management with role-based access control
// All fees go to admin addresses, complete subscription management

package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// CONSTANTS
// ============================================================================

const (
	API_VERSION        = "1.0.0"
	MAX_REQUEST_SIZE  = 10 * 1024 * 1024
	DEFAULT_TIMEOUT  = 30 * time.Second
)

// ============================================================================
// ENUMS
// ============================================================================

type UserRole string

const (
	RoleSuperAdmin     UserRole = "super_admin"
	RoleBotOperator    UserRole = "bot_operator"
	RoleFinanceAdmin   UserRole = "finance_admin"
	RoleClient         UserRole = "client"
)

// ============================================================================
// MODELS
// ============================================================================

// User with role
type User struct {
	ID                string    `json:"id"`
	WalletAddress    string    `json:"wallet_address"`
	Email            string    `json:"email"`
	Username         string    `json:"username"`
	Role             UserRole  `json:"role"`
	IsActive         bool      `json:"is_active"`
	BotTier          string    `json:"bot_tier"`
	MaxBots          int       `json:"max_bots"`
	MaxDEXS          int       `json:"max_dexs"`
	MaxCEXs          int       `json:"max_cexs"`
	TotalBots        int       `json:"total_bots"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Bot Tier Configuration
type BotTier struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"`
	DisplayName      string    `json:"display_name"`
	MonthlyFeeUSD    float64   `json:"monthly_fee_usd"`
	PerDEXFeeUSD     float64   `json:"per_dex_fee_usd"`
	PerCEXFeeUSD     float64   `json:"per_cex_fee_usd"`
	MaxBots          int       `json:"max_bots"`
	MaxDEXs          int       `json:"max_dexs"`
	MaxCEXs          int       `json:"max_cexs"`
	MaxPositionUSD   float64   `json:"max_position_usd"`
	MaxDailyVolume   float64   `json:"max_daily_volume"`
	LatencyTargetMs  int       `json:"latency_target_ms"`
	Features        map[string]bool `json:"features"`
	IsActive         bool      `json:"is_active"`
}

// Bot Instance
type BotInstance struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	BotType        string    `json:"bot_type"`
	Name           string    `json:"name"`
	Status         string    `json:"status"` // running, stopped, error, paused
	ConnectedDEXS  []string `json:"connected_dexs"`
	ConnectedCEXs  []string `json:"connected_cexs"`
	TradingPairs   []string `json:"trading_pairs"`
	TotalPnL      float64  `json:"total_pnl"`
	TotalVolume   float64  `json:"total_volume"`
	TotalOrders   int      `json:"total_orders"`
	AvgLatencyUs  int      `json:"avg_latency_us"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	LastTradeAt   time.Time `json:"last_trade_at"`
}

// Bot Subscription
type BotSubscription struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	TierID          string    `json:"tier_id"`
	NumDEXs         int       `json:"num_dexs"`
	NumCEXs         int       `json:"num_cexs"`
	MonthlyFee     float64   `json:"monthly_fee"`
	PerDEXFee      float64   `json:"per_dex_fee"`
	PerCEXFee      float64   `json:"per_cex_fee"`
	TotalMonthly  float64   `json:"total_monthly"`
	Status         string    `json:"status"` // active, paused, cancelled, expired
	CycleStart     time.Time `json:"cycle_start"`
	CycleEnd      time.Time `json:"cycle_end"`
	NextBilling   time.Time `json:"next_billing"`
	CreatedAt     time.Time `json:"created_at"`
}

// Fee Configuration
type FeeConfig struct {
	ID              string    `json:"id"`
	FeeType        string    `json:"fee_type"` // swap, liquidity, withdrawal, bot_subscription, api_key, listing
	ChainID        int       `json:"chain_id"`
	TokenSymbol    string    `json:"token_symbol"`
	FeeAmountUSD  float64   `json:"fee_amount_usd"`
	FeePercentage float64   `json:"fee_percentage"`
	IsActive      bool      `json:"is_active"`
	MinFeeUSD     float64   `json:"min_fee_usd"`
	MaxFeeUSD     float64   `json:"max_fee_usd"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Admin Fee Address
type AdminFeeAddress struct {
	ID              string    `json:"id"`
	FeeType        string    `json:"fee_type"`
	ChainID        int       `json:"chain_id"`
	WalletAddress string    `json:"wallet_address"`
	TokenSymbol   string    `json:"token_symbol"`
	IsActive     bool      `json:"is_active"`
	Priority    int       `json:"priority"`
	CreatedAt   time.Time `json:"created_at"`
}

// External CEX Connection (for users connecting their own CEX accounts)
type UserCEXConnection struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	ExchangeName   string    `json:"exchange_name"` // binance, coinbase, kraken, etc.
	AccountID      string    `json:"account_id"`
	IsActive       bool      `json:"is_active"`
	CanTrade      bool      `json:"can_trade"`
	CanWithdraw  bool      `json:"can_withdraw"`
	CanDeposit   bool      `json:"can_deposit"`
	LastSyncAt   time.Time `json:"last_sync_at"`
	SyncStatus   string    `json:"sync_status"` // idle, syncing, error
	ErrorMsg    string    `json:"error_msg"`
	CreatedAt   time.Time `json:"created_at"`
}

// External DEX Connection (for users connecting their own DEX wallets)
type UserDEXConnection struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	DEXName        string    `json:"dex_name"` // uniswap, pancakeswap, etc.
	ChainID        int       `json:"chain_id"`
	WalletAddress string    `json:"wallet_address"`
	IsActive       bool      `json:"is_active"`
	MaxSlippageBps int      `json:"max_slippage_bps"`
	GasLimit      int       `json:"gas_limit"`
	LastTxHash   string    `json:"last_tx_hash"`
	LastTxAt    time.Time `json:"last_tx_at"`
	CreatedAt   time.Time `json:"created_at"`
}

// API Key for external users
type APIKey struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	KeyName         string    `json:"key_name"`
	APIKey         string    `json:"api_key"`
	Tier            string    `json:"tier"` // free, basic, pro, enterprise
	Permissions    map[string]bool `json:"permissions"`
	RateLimitMin   int       `json:"rate_limit_per_minute"`
	RateLimitDay  int       `json:"rate_limit_per_day"`
	IsActive      bool      `json:"is_active"`
	LastUsedAt    time.Time `json:"last_used_at"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
}

// ============================================================================
// DATABASE (In-Memory for Demo)
// ============================================================================

var (
	users            = make(map[string]*User)
	botTiers        = make(map[string]*BotTier)
	botInstances    = make(map[string]*BotInstance)
	botSubscriptions = make(map[string]*BotSubscription)
	feeConfigs      = make(map[string]*FeeConfig)
	adminFeeAddresses = make(map[string]*AdminFeeAddress)
	userCEXConnections = make(map[string]*UserCEXConnection)
	userDEXConnections = make(map[string]*UserDEXConnection)
	apiKeys         = make(map[string]*APIKey)
	sessions        = make(map[string]*Session)
	encryptionKey  []byte
)

// ============================================================================
// SESSION MANAGEMENT
// ============================================================================

type Session struct {
	Token       string    `json:"token"`
	UserID     string    `json:"user_id"`
	ExpiresAt  time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================================
// AUTHENTICATION & AUTHORIZATION
// ============================================================================

// Verify role-based access
func canManageAllBots(role UserRole) bool {
	return role == RoleSuperAdmin || role == RoleBotOperator
}

func canManageFees(role UserRole) bool {
	return role == RoleSuperAdmin
}

func canSuspendPlatform(role UserRole) bool {
	return role == RoleSuperAdmin
}

func canViewAllStats(role UserRole) bool {
	return role == RoleSuperAdmin || role == RoleBotOperator
}

// ============================================================================
// ENCRYPTION
// ============================================================================

func encrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	
	return gcm.Seal(nonce, nonce, data, nil), nil
}

func decrypt(data []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// ============================================================================
// PASSWORD HASHING
// ============================================================================

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

func checkPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// ============================================================================
// API KEY GENERATION
// ============================================================================

func generateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func generateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// ============================================================================
// INITIALIZE DEFAULT DATA
// ============================================================================

func initDefaultData() {
	// Initialize encryption key
	encryptionKey = make([]byte, 32)
	copy(encryptionKey, []byte("tigerswap-secret-key-32-bytes!!"))
	
	// Create default bot tiers
	botTiers["tier_1"] = &BotTier{
		ID:              "tier_1",
		Name:            "tier_1",
		DisplayName:    "Basic",
		MonthlyFeeUSD:  2500,
		PerDEXFeeUSD:   500,
		PerCEXFeeUSD:   50,
		MaxBots:        1,
		MaxDEXs:        5,
		MaxCEXs:        20,
		MaxPositionUSD: 100000,
		MaxDailyVolume: 1000000,
		LatencyTargetMs: 100,
		Features:       map[string]bool{"arbitrage": true, "sniping": false},
		IsActive:       true,
	}
	
	botTiers["tier_2"] = &BotTier{
		ID:              "tier_2",
		Name:            "tier_2",
		DisplayName:    "Pro",
		MonthlyFeeUSD:  5000,
		PerDEXFeeUSD:   750,
		PerCEXFeeUSD:   75,
		MaxBots:        3,
		MaxDEXs:        10,
		MaxCEXs:        50,
		MaxPositionUSD: 500000,
		MaxDailyVolume: 5000000,
		LatencyTargetMs: 50,
		Features:       map[string]bool{"arbitrage": true, "sniping": true, "mev": true},
		IsActive:       true,
	}
	
	botTiers["tier_3"] = &BotTier{
		ID:              "tier_3",
		Name:            "tier_3",
		DisplayName:    "Enterprise",
		MonthlyFeeUSD:  10000,
		PerDEXFeeUSD:   1000,
		PerCEXFeeUSD:   100,
		MaxBots:        10,
		MaxDEXs:        20,
		MaxCEXs:        200,
		MaxPositionUSD: 5000000,
		MaxDailyVolume: 50000000,
		LatencyTargetMs: 10,
		Features:       map[string]bool{"arbitrage": true, "sniping": true, "mev": true, "flash_loan": true},
		IsActive:       true,
	}
	
	// Create default fee configs
	feeConfigs["swap"] = &FeeConfig{
		ID:              "swap",
		FeeType:        "swap",
		FeeAmountUSD:   0.3,
		FeePercentage: 0.0,
		IsActive:       true,
	}
	
	feeConfigs["bot_subscription"] = &FeeConfig{
		ID:              "bot_subscription",
		FeeType:        "bot_subscription",
		FeeAmountUSD:   0,
		FeePercentage: 0,
		IsActive:       true,
	}
	
	feeConfigs["api_key"] = &FeeConfig{
		ID:              "api_key",
		FeeType:        "api_key",
		FeeAmountUSD:   99,
		FeePercentage: 0,
		IsActive:       true,
	}
	
	feeConfigs["listing_basic"] = &FeeConfig{
		ID:              "listing_basic",
		FeeType:        "listing",
		FeeAmountUSD:   5000,
		FeePercentage: 0,
		IsActive:       true,
	}
	
	feeConfigs["listing_premium"] = &FeeConfig{
		ID:              "listing_premium",
		FeeType:        "listing",
		FeeAmountUSD:   15000,
		FeePercentage: 0,
		IsActive:       true,
	}
	
	// Initialize admin fee addresses (these receive ALL platform fees)
	// Admin will configure these via the dashboard
	fmt.Println("[*] Default data initialized")
	fmt.Println("[*] Bot tiers: tier_1 ($2500/mo), tier_2 ($5000/mo), tier_3 ($10000/mo)")
	fmt.Println("[*] Fee configs: swap (0.3%), api_key ($99/mo), listing ($5k-$15k)")
}

// ============================================================================
// MIDDLEWARE
// ============================================================================

type AuthMiddleware struct {
	handler http.Handler
}

func (m *AuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Skip auth for public endpoints
	if strings.HasPrefix(r.URL.Path, "/api/v1/public") ||
	   strings.HasPrefix(r.URL.Path, "/api/v1/health") {
		m.handler.ServeHTTP(w, r)
		return
	}
	
	// Get token from header
	token := r.Header.Get("Authorization")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	
	if token == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	
	// Remove "Bearer " prefix
	if strings.HasPrefix(token, "Bearer ") {
		token = token[7:]
	}
	
	// Verify session
	session, exists := sessions[token]
	if !exists || time.Now().After(session.ExpiresAt) {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}
	
	// Add user ID to context
	ctx := context.WithValue(r.Context(), "user_id", session.UserID)
	r = r.WithContext(ctx)
	
	m.handler.ServeHTTP(w, r)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("[%s] %s %s - %v\n", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}

// ============================================================================
// RESPONSE HELPERS
// ============================================================================

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}

func getUserID(r *http.Request) string {
	if id := r.Context().Value("user_id"); id != nil {
		return id.(string)
	}
	return ""
}

func getUser(r *http.Request) *User {
	userID := getUserID(r)
	if userID == "" {
		return nil
	}
	return users[userID]
}

// ============================================================================
// ADMIN API ENDPOINTS
// ============================================================================

// Get all users - admin only
func handleAdminGetUsers(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil || !canManageAllBots(user.Role) {
		respondError(w, http.StatusForbidden, "Admin access required")
		return
	}
	
	result := make([]*User, 0, len(users))
	for _, u := range users {
		result = append(result, u)
	}
	respondJSON(w, http.StatusOK, result)
}

// Get platform statistics - admin only
func handleAdminGetStats(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil || !canViewAllStats(user.Role) {
		respondError(w, http.StatusForbidden, "Admin access required")
		return
	}
	
	totalBots := len(botInstances)
	totalUsers := len(users)
	totalVolume := 0.0
	totalPnL := 0.0
	
	for _, bot := range botInstances {
		totalVolume += bot.TotalVolume
		totalPnL += bot.TotalPnL
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"total_users":    totalUsers,
		"total_bots":    totalBots,
		"total_volume":  totalVolume,
		"total_pnl":    totalPnL,
		"active_bots":  countActiveBots(),
		"api_version":  API_VERSION,
	})
}

func countActiveBots() int {
	count := 0
	for _, bot := range botInstances {
		if bot.Status == "running" {
			count++
		}
	}
	return count
}

// ============================================================================
// BOT TIER ENDPOINTS
// ============================================================================

// Get all bot tiers
func handleGetBotTiers(w http.ResponseWriter, r *http.Request) {
	result := make([]*BotTier, 0, len(botTiers))
	for _, tier := range botTiers {
		if tier.IsActive {
			result = append(result, tier)
		}
	}
	respondJSON(w, http.StatusOK, result)
}

// ============================================================================
// BOT INSTANCE ENDPOINTS
// ============================================================================

// Get user's bots
func handleGetBots(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	result := make([]*BotInstance, 0)
	for _, bot := range botInstances {
		if bot.UserID == user.ID || canManageAllBots(user.Role) {
			result = append(result, bot)
		}
	}
	respondJSON(w, http.StatusOK, result)
}

// Create new bot
func handleCreateBot(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	// Check bot limit
	if user.TotalBots >= user.MaxBots {
		respondError(w, http.StatusForbidden, "Bot limit reached for your tier")
		return
	}
	
	var req struct {
		BotType     string   `json:"bot_type"`
		Name       string   `json:"name"`
		TradingPairs []string `json:"trading_pairs"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	// Generate bot ID
	botID := fmt.Sprintf("bot_%d", len(botInstances)+1)
	
	bot := &BotInstance{
		ID:             botID,
		UserID:         user.ID,
		BotType:        req.BotType,
		Name:           req.Name,
		Status:         "stopped",
		TradingPairs:  req.TradingPairs,
		ConnectedDEXS: []string{},
		ConnectedCEXs: []string{},
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	
	botInstances[botID] = bot
	user.TotalBots++
	
	respondJSON(w, http.StatusCreated, bot)
}

// Start bot
func handleStartBot(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	vars := mux.Vars(r)
	botID := vars["id"]
	
	bot, exists := botInstances[botID]
	if !exists {
		respondError(w, http.StatusNotFound, "Bot not found")
		return
	}
	
	// Check ownership (admin can manage any bot)
	if bot.UserID != user.ID && !canManageAllBots(user.Role) {
		respondError(w, http.StatusForbidden, "Access denied")
		return
	}
	
	bot.Status = "running"
	bot.UpdatedAt = time.Now()
	
	respondJSON(w, http.StatusOK, bot)
}

// Stop bot
func handleStopBot(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	vars := mux.Vars(r)
	botID := vars["id"]
	
	bot, exists := botInstances[botID]
	if !exists {
		respondError(w, http.StatusNotFound, "Bot not found")
		return
	}
	
	if bot.UserID != user.ID && !canManageAllBots(user.Role) {
		respondError(w, http.StatusForbidden, "Access denied")
		return
	}
	
	bot.Status = "stopped"
	bot.UpdatedAt = time.Now()
	
	respondJSON(w, http.StatusOK, bot)
}

// ============================================================================
// SUBSCRIPTION ENDPOINTS
// ============================================================================

// Get user's subscription
func handleGetSubscription(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	for _, sub := range botSubscriptions {
		if sub.UserID == user.ID && sub.Status == "active" {
			respondJSON(w, http.StatusOK, sub)
			return
		}
	}
	
	respondError(w, http.StatusNotFound, "No active subscription")
}

// Create subscription
func handleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	var req struct {
		TierID   string `json:"tier_id"`
		NumDEXs  int    `json:"num_dexs"`
		NumCEXs  int    `json:"num_cexs"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	tier, exists := botTiers[req.TierID]
	if !exists {
		respondError(w, http.StatusNotFound, "Tier not found")
		return
	}
	
	// Calculate total fee
	numDEXs := req.NumDEXs
	numCEXs := req.NumCEXs
	if numDEXs == 0 {
		numDEXs = tier.MaxDEXs
	}
	if numCEXs == 0 {
		numCEXs = tier.MaxCEXs
	}
	
	totalMonthly := tier.MonthlyFeeUSD +
		float64(numDEXs)*tier.PerDEXFeeUSD +
		float64(numCEXs)*tier.PerCEXFeeUSD
	
	sub := &BotSubscription{
		ID:            fmt.Sprintf("sub_%d", len(botSubscriptions)+1),
		UserID:        user.ID,
		TierID:        req.TierID,
		NumDEXs:       numDEXs,
		NumCEXs:       numCEXs,
		MonthlyFee:   tier.MonthlyFeeUSD,
		PerDEXFee:    tier.PerDEXFeeUSD,
		PerCEXFee:    tier.PerCEXFeeUSD,
		TotalMonthly: totalMonthly,
		Status:       "active",
		CycleStart:   time.Now(),
		CycleEnd:     time.Now().Add(30 * 24 * time.Hour),
		NextBilling:  time.Now().Add(30 * 24 * time.Hour),
		CreatedAt:    time.Now(),
	}
	
	botSubscriptions[sub.ID] = sub
	
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"subscription":  sub,
		"total_monthly": totalMonthly,
		"payment_info":  "Pay to admin fee address - details in dashboard",
	})
}

// ============================================================================
// FEE MANAGEMENT ENDPOINTS
// ============================================================================

// Get all fee configs - admin only
func handleGetFeeConfigs(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil || !canManageFees(user.Role) {
		respondError(w, http.StatusForbidden, "Admin access required")
		return
	}
	
	result := make([]*FeeConfig, 0, len(feeConfigs))
	for _, fc := range feeConfigs {
		result = append(result, fc)
	}
	respondJSON(w, http.StatusOK, result)
}

// Update fee config - admin only
func handleUpdateFeeConfig(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil || !canManageFees(user.Role) {
		respondError(w, http.StatusForbidden, "Admin access required")
		return
	}
	
	var req struct {
		FeeType        string  `json:"fee_type"`
		FeeAmountUSD  float64 `json:"fee_amount_usd"`
		FeePercentage float64 `json:"fee_percentage"`
		MinFeeUSD     float64 `json:"min_fee_usd"`
		MaxFeeUSD     float64 `json:"max_fee_usd"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	fc, exists := feeConfigs[req.FeeType]
	if !exists {
		fc = &FeeConfig{
			ID:       req.FeeType,
			FeeType:  req.FeeType,
			IsActive: true,
		}
		feeConfigs[req.FeeType] = fc
	}
	
	fc.FeeAmountUSD = req.FeeAmountUSD
	fc.FeePercentage = req.FeePercentage
	fc.MinFeeUSD = req.MinFeeUSD
	fc.MaxFeeUSD = req.MaxFeeUSD
	fc.UpdatedAt = time.Now()
	
	respondJSON(w, http.StatusOK, fc)
}

// Get admin fee addresses
func handleGetAdminFeeAddresses(w http.ResponseWriter, r *http.Request) {
	result := make([]*AdminFeeAddress, 0, len(adminFeeAddresses))
	for _, addr := range adminFeeAddresses {
		if addr.IsActive {
			result = append(result, addr)
		}
	}
	respondJSON(w, http.StatusOK, result)
}

// Set admin fee address - admin only
func handleSetAdminFeeAddress(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil || !canManageFees(user.Role) {
		respondError(w, http.StatusForbidden, "Admin access required")
		return
	}
	
	var req struct {
		FeeType       string `json:"fee_type"`
		ChainID      int    `json:"chain_id"`
		WalletAddr  string `json:"wallet_address"`
		TokenSymbol string `json:"token_symbol"`
		Priority    int    `json:"priority"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	addr := &AdminFeeAddress{
		ID:              fmt.Sprintf("afa_%d", len(adminFeeAddresses)+1),
		FeeType:        req.FeeType,
		ChainID:        req.ChainID,
		WalletAddress: req.WalletAddr,
		TokenSymbol:   req.TokenSymbol,
		IsActive:      true,
		Priority:     req.Priority,
		CreatedAt:    time.Now(),
	}
	
	adminFeeAddresses[addr.ID] = addr
	
	respondJSON(w, http.StatusCreated, addr)
}

// ============================================================================
// EXTERNAL CEX CONNECTION ENDPOINTS
// ============================================================================

// Get user's CEX connections
func handleGetCEXConnections(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	result := make([]*UserCEXConnection, 0)
	for _, conn := range userCEXConnections {
		if conn.UserID == user.ID {
			result = append(result, conn)
		}
	}
	respondJSON(w, http.StatusOK, result)
}

// Add CEX connection
func handleAddCEXConnection(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	var req struct {
		ExchangeName string `json:"exchange_name"`
		AccountID   string `json:"account_id"`
		APIKey      string `json:"api_key"`
		APISecret  string `json:"api_secret"`
		CanTrade   bool   `json:"can_trade"`
		CanWithdraw bool  `json:"can_withdraw"`
		CanDeposit bool   `json:"can_deposit"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	// Encrypt API credentials
	apiKeyEnc, _ := encrypt([]byte(req.APIKey), encryptionKey)
	apiSecretEnc, _ := encrypt([]byte(req.APISecret), encryptionKey)
	
	conn := &UserCEXConnection{
		ID:             fmt.Sprintf("cex_%d", len(userCEXConnections)+1),
		UserID:         user.ID,
		ExchangeName:  req.ExchangeName,
		AccountID:     req.AccountID,
		IsActive:       true,
		CanTrade:      req.CanTrade,
		CanWithdraw:  req.CanWithdraw,
		CanDeposit:   req.CanDeposit,
		SyncStatus:    "idle",
		CreatedAt:     time.Now(),
	}
	
	// Store encrypted credentials (in production, store in secure DB)
	_ = apiKeyEnc
	_ = apiSecretEnc
	
	userCEXConnections[conn.ID] = conn
	
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":           conn.ID,
		"exchange":     conn.ExchangeName,
		"status":      "connected",
		"can_trade":    conn.CanTrade,
		"can_withdraw": conn.CanWithdraw,
	})
}

// ============================================================================
// EXTERNAL DEX CONNECTION ENDPOINTS
// ============================================================================

// Get user's DEX connections
func handleGetDEXConnections(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	result := make([]*UserDEXConnection, 0)
	for _, conn := range userDEXConnections {
		if conn.UserID == user.ID {
			result = append(result, conn)
		}
	}
	respondJSON(w, http.StatusOK, result)
}

// Add DEX connection
func handleAddDEXConnection(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	var req struct {
		DEXName        string `json:"dex_name"`
		ChainID        int    `json:"chain_id"`
		WalletAddress  string `json:"wallet_address"`
		MaxSlippageBps int    `json:"max_slippage_bps"`
		GasLimit       int    `json:"gas_limit"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	conn := &UserDEXConnection{
		ID:              fmt.Sprintf("dex_%d", len(userDEXConnections)+1),
		UserID:          user.ID,
		DEXName:         req.DEXName,
		ChainID:         req.ChainID,
		WalletAddress:   req.WalletAddress,
		IsActive:        true,
		MaxSlippageBps:  req.MaxSlippageBps,
		GasLimit:        req.GasLimit,
		CreatedAt:       time.Now(),
	}
	
	userDEXConnections[conn.ID] = conn
	
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"id":             conn.ID,
		"dex":            conn.DEXName,
		"chain_id":       conn.ChainID,
		"wallet_address": conn.WalletAddress,
		"status":         "connected",
	})
}

// ============================================================================
// API KEY ENDPOINTS
// ============================================================================

// Get user's API keys
func handleGetAPIKeys(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	result := make([]*APIKey, 0)
	for _, key := range apiKeys {
		if key.UserID == user.ID {
			// Don't expose the actual key
			result = append(result, &APIKey{
				ID:             key.ID,
				UserID:         key.UserID,
				KeyName:        key.KeyName,
				APIKey:         "***" + key.APIKey[len(key.APIKey)-4:],
				Tier:           key.Tier,
				Permissions:   key.Permissions,
				RateLimitMin:  key.RateLimitMin,
				RateLimitDay:  key.RateLimitDay,
				IsActive:      key.IsActive,
				LastUsedAt:    key.LastUsedAt,
				ExpiresAt:     key.ExpiresAt,
				CreatedAt:     key.CreatedAt,
			})
		}
	}
	respondJSON(w, http.StatusOK, result)
}

// Create API key
func handleCreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user := getUser(r)
	if user == nil {
		respondError(w, http.StatusUnauthorized, "Authentication required")
		return
	}
	
	var req struct {
		KeyName  string `json:"key_name"`
		Tier     string `json:"tier"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	keyStr, _ := generateAPIKey()
	
	key := &APIKey{
		ID:             fmt.Sprintf("key_%d", len(apiKeys)+1),
		UserID:         user.ID,
		KeyName:        req.KeyName,
		APIKey:         keyStr,
		Tier:           req.Tier,
		Permissions:   map[string]bool{"trading": true, "reading": true},
		RateLimitMin:   60,
		RateLimitDay:  10000,
		IsActive:      true,
		ExpiresAt:     time.Now().Add(365 * 24 * time.Hour),
		CreatedAt:      time.Now(),
	}
	
	apiKeys[key.ID] = key
	
	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"api_key":        keyStr,
		"api_secret":     "Generate in dashboard",
		"tier":           key.Tier,
		"rate_limits":     fmt.Sprintf("%d/min, %d/day", key.RateLimitMin, key.RateLimitDay),
		"expires_at":    key.ExpiresAt,
	})
}

// ============================================================================
// AUTH ENDPOINTS
// ============================================================================

// Login
func handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	
	// Find user
	var user *User
	for _, u := range users {
		if u.Email == req.Email {
			user = u
			break
		}
	}
	
	if user == nil {
		// Create new user for demo
		user = &User{
			ID:            fmt.Sprintf("user_%d", len(users)+1),
			Email:         req.Email,
			WalletAddress: req.Email,
			Username:     req.Email,
			Role:          RoleClient,
			IsActive:      true,
			BotTier:       "tier_1",
			MaxBots:       1,
			MaxDEXS:       5,
			MaxCEXs:       20,
			TotalBots:     0,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		users[user.ID] = user
	}
	
	// Create session
	token, _ := generateSessionToken()
	sessions[token] = &Session{
		Token:      token,
		UserID:     user.ID,
		ExpiresAt:  time.Now().Add(24 * time.Hour),
		CreatedAt:  time.Now(),
	}
	
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"token":      token,
		"user_id":   user.ID,
		"email":     user.Email,
		"role":      user.Role,
		"bot_tier":  user.BotTier,
		"expires":  "24h",
	})
}

// Logout
func handleLogout(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if strings.HasPrefix(token, "Bearer ") {
		token = token[7:]
	}
	
	if token != "" {
		delete(sessions, token)
	}
	
	respondJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

// ============================================================================
// HEALTH & PUBLIC ENDPOINTS
// ============================================================================

func handleHealth(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":       "healthy",
		"api_version": API_VERSION,
		"timestamp":   time.Now(),
	})
}

// ============================================================================
// ROUTER SETUP
// ============================================================================

func setupRouter() *mux.Router {
	router := mux.NewRouter()
	
	// Public endpoints
	router.HandleFunc("/api/v1/health", handleHealth).Methods("GET")
	router.HandleFunc("/api/v1/public/tiers", handleGetBotTiers).Methods("GET")
	
	// Auth endpoints
	router.HandleFunc("/api/v1/auth/login", handleLogin).Methods("POST")
	router.HandleFunc("/api/v1/auth/logout", handleLogout).Methods("POST")
	
	// Bot endpoints
	router.HandleFunc("/api/v1/bots", handleGetBots).Methods("GET")
	router.HandleFunc("/api/v1/bots", handleCreateBot).Methods("POST")
	router.HandleFunc("/api/v1/bots/{id}/start", handleStartBot).Methods("POST")
	router.HandleFunc("/api/v1/bots/{id}/stop", handleStopBot).Methods("POST")
	
	// Subscription endpoints
	router.HandleFunc("/api/v1/subscription", handleGetSubscription).Methods("GET")
	router.HandleFunc("/api/v1/subscription", handleCreateSubscription).Methods("POST")
	
	// Fee endpoints (admin)
	router.HandleFunc("/api/v1/fees", handleGetFeeConfigs).Methods("GET")
	router.HandleFunc("/api/v1/fees", handleUpdateFeeConfig).Methods("PUT")
	router.HandleFunc("/api/v1/admin/fee-addresses", handleGetAdminFeeAddresses).Methods("GET")
	router.HandleFunc("/api/v1/admin/fee-addresses", handleSetAdminFeeAddress).Methods("POST")
	
	// External CEX endpoints
	router.HandleFunc("/api/v1/cex", handleGetCEXConnections).Methods("GET")
	router.HandleFunc("/api/v1/cex", handleAddCEXConnection).Methods("POST")
	
	// External DEX endpoints
	router.HandleFunc("/api/v1/dex", handleGetDEXConnections).Methods("GET")
	router.HandleFunc("/api/v1/dex", handleAddDEXConnection).Methods("POST")
	
	// API key endpoints
	router.HandleFunc("/api/v1/keys", handleGetAPIKeys).Methods("GET")
	router.HandleFunc("/api/v1/keys", handleCreateAPIKey).Methods("POST")
	
	// Admin endpoints
	router.HandleFunc("/api/v1/admin/users", handleAdminGetUsers).Methods("GET")
	router.HandleFunc("/api/v1/admin/stats", handleAdminGetStats).Methods("GET")
	
	return router
}

// ============================================================================
// MAIN
// ============================================================================

func main() {
	fmt.Println("===========================================")
	fmt.Println("  TigerSwap Bot Platform API Server")
	fmt.Println("  Version:", API_VERSION)
	fmt.Println("===========================================")
	
	// Initialize default data
	initDefaultData()
	
	// Setup router
	router := setupRouter()
	
	// Apply middleware
	handler := loggingMiddleware(router)
	
	// Start server
	fmt.Println("[*] Server starting on :8080")
	fmt.Println("[*] API documentation: /api/v1/public/tiers")
	
	err := http.ListenAndServe(":8080", &handler)
	if err != nil {
		fmt.Println("Error:", err)
	}
}