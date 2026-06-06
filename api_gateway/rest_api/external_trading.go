// TigerSwap External Trading API
// Complete REST API for external platforms (CEX/DEX) to connect and trade
// All fees go to TigerSwap admin addresses
// Tier-based access control for admins and external users

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
	"sync"
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
	RoleSuperAdmin         UserRole = "super_admin"
	RoleExchangeOperator UserRole = "exchange_operator"
	RoleFinanceAdmin      UserRole = "finance_admin"
	RoleClient           UserRole = "client"
)

type ConnectionStatus string

const (
	StatusActive   ConnectionStatus = "active"
	StatusPaused  ConnectionStatus = "paused"
	StatusError  ConnectionStatus = "error"
	StatusClosed ConnectionStatus = "closed"
)

// ============================================================================
// MODELS - External Platform Connections
// ============================================================================

// Connected CEX (Centralized Exchange)
type ConnectedCEX struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"` // Admin user ID
	ExchangeName    string    `json:"exchange_name"` // binance, coinbase, kraken, okx, etc.
	AccountID       string    `json:"account_id"` // Account ID on the exchange
	APIKey         string    `json:"api_key"` // Encrypted API key
	APISecret      string    `json:"api_secret"` // Encrypted API secret
	Passphrase     string    `json:"passphrase,omitempty"` // For exchanges that require it
	IsActive       bool      `json:"is_active"`
	CanTrade       bool      `json:"can_trade"`
	CanWithdraw    bool      `json:"can_withdraw"`
	CanDeposit     bool      `json:"can_deposit"`
	RateLimitRPM   int       `json:"rate_limit_per_minute"`
	MonthlyFeeUSD  float64   `json:"monthly_fee_usd"`
	TotalFeesPaid float64   `json:"total_fees_paid"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	LastSyncAt    time.Time `json:"last_sync_at"`
}

// Connected DEX (Decentralized Exchange)
type ConnectedDEX struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	DEXName         string    `json:"dex_name"` // uniswap, pancakeswap, sushiswap, etc.
	ChainID        int       `json:"chain_id"`
	WalletAddress  string    `json:"wallet_address"` // Wallet connected to DEX
	RouterAddress string    `json:"router_address"` // DEX router address
	FactoryAddress string   `json:"factory_address"` // DEX factory address
	IsActive       bool      `json:"is_active"`
	MaxSlippageBps int      `json:"max_slippage_bps"` // Maximum slippage in basis points
	GasLimit       int       `json:"gas_limit"` // Default gas limit
	NativeGasToken bool      `json:"native_gas_token"` // Pay gas with native token
	MonthlyFeeUSD  float64   `json:"monthly_fee_usd"`
	TotalFeesPaid float64   `json:"total_fees_paid"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	LastTxAt     time.Time `json:"last_tx_at"`
}

// External Platform (for external users connecting their TigerSwap to other platforms)
type ExternalPlatformConnection struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"` // External user ID
	PlatformName   string    `json:"platform_name"` // Name of external platform
	PlatformType  string    `json:"platform_type"` // cex, dex, wallet, protocol
	APIKey        string    `json:"api_key"` // TigerSwap API key for this connection
	WebhookURL    string    `json:"webhook_url"`
	IsActive      bool      `json:"is_active"`
	CanTrade     bool      `json:"can_trade"`
	CanSwap      bool      `json:"can_swap"`
	CanAddLiq    bool      `json:"can_add_liquidity"`
	CanBridge    bool      `json:"can_bridge"`
	RateLimitRPM int       `json:"rate_limit_per_minute"`
	Tier         string    `json:"tier"` // free, basic, pro, enterprise
	MonthlyFeeUSD float64  `json:"monthly_fee_usd"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Tier Configuration for External Access
type TierConfig struct {
	ID                string    `json:"id"`
	Name              string    `json:"name"` // free, basic, pro, enterprise
	DisplayName      string    `json:"display_name"`
	MonthlyFeeUSD     float64   `json:"monthly_fee_usd"`
	MaxAPICallsPerMin int       `json:"max_api_calls_per_minute"`
	MaxDailyVolume   float64   `json:"max_daily_volume"`
	MaxPositions    int       `json:"max_positions"`
	Features         map[string]bool `json:"features"` // trading, swap, liquidity, bridge, api_access
	IsActive         bool      `json:"is_active"`
}

// Trading Order
type Order struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	Platform     string    `json:"platform"` // cex name or dex name
	PlatformType string    `json:"platform_type"` // cex, dex
	Symbol       string    `json:"symbol"` // e.g., BTC/USDT
	Side         string    `json:"side"` // buy, sell
	Type         string    `json:"type"` // market, limit, stop_loss
	Price        string    `json:"price,omitempty"`
	Amount       string    `json:"amount"`
	FilledAmount string    `json:"filled_amount"`
	Status       string    `json:"status"` // pending, filled, partially_filled, cancelled
	Fee          float64   `json:"fee"`
	FeeToken     string    `json:"fee_token"`
	TxHash       string    `json:"tx_hash,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	ExecutedAt  time.Time `json:"executed_at,omitempty"`
}

// Swap Order (for DEX swaps)
type SwapOrder struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	Platform       string    `json:"platform"` // DEX name
	ChainID        int       `json:"chain_id"`
	TokenIn        string    `json:"token_in"`
	TokenOut       string    `json:"token_out"`
	AmountIn       string    `json:"amount_in"`
	AmountOutMin   string    `json:"amount_out_min"`
	Recipient     string    `json:"recipient"`
	Deadline       int       `json:"deadline"`
	Route          []string  `json:"route"` // Token path
	Status        string    `json:"status"` // pending, completed, failed
	TxHash         string    `json:"tx_hash"`
	GasUsed        string    `json:"gas_used"`
	FeeUSD         float64   `json:"fee_usd"`
	PriceImpact   float64   `json:"price_impact"`
	CreatedAt     time.Time `json:"created_at"`
	ExecutedAt    time.Time `json:"executed_at"`
}

// Fee Collection
type FeeCollection struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	FeeType        string    `json:"fee_type"` // swap, trading, api_key, subscription, listing
	Platform      string    `json:"platform,omitempty"` // For platform-specific fees
	AmountUSD    float64   `json:"amount_usd"`
	AmountToken  string    `json:"amount_token"`
	TokenSymbol  string    `json:"token_symbol"`
	ChainID     int       `json:"chain_id"`
	TxHash      string    `json:"tx_hash"`
	Status      string    `json:"status"` // pending, collected, failed
	CreatedAt   time.Time `json:"created_at"`
}

// Admin Fee Address Configuration
type AdminFeeAddress struct {
	ID              string    `json:"id"`
	FeeType        string    `json:"fee_type"` // swap, trading, bot, api, listing
	ChainID        int       `json:"chain_id"`
	TokenSymbol    string    `json:"token_symbol"`
	WalletAddress string    `json:"wallet_address"` // Where fees are sent
	IsActive      bool      `json:"is_active"`
	Priority     int       `json:"priority"` // Higher = preferred
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ============================================================================
// DATABASE (In-Memory for Demo)
// ============================================================================

var (
	connectedCEXs    = make(map[string]*ConnectedCEX)
	connectedDEXs   = make(map[string]*ConnectedDEX)
	externalConns  = make(map[string]*ExternalPlatformConnection)
	tierConfigs    = make(map[string]*TierConfig)
	orders        = make(map[string]*Order)
	swapOrders    = make(map[string]*SwapOrder)
	feeCollections = make(map[string]*FeeCollection)
	adminFeeAddrs = make(map[string]*AdminFeeAddress)
	sessions     = make(map[string]*Session)
	encryptionKey []byte
	mu           sync.RWMutex
)

type Session struct {
	Token     string    `json:"token"`
	UserID   string    `json:"user_id"`
	Role     UserRole  `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================================
// ENCRYPTION HELPERS
// ============================================================================

func encryptData(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(encryptionKey)
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

func decryptData(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(encryptionKey)
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

func encryptString(s string) (string, error) {
	encrypted, err := encryptData([]byte(s))
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(encrypted), nil
}

func decryptString(s string) (string, error) {
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return "", err
	}
	decrypted, err := decryptData(decoded)
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}

func generateID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func generateSessionToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// ============================================================================
// INITIALIZATION
// ============================================================================

func initDefaultData() {
	// Initialize encryption key
	encryptionKey = make([]byte, 32)
	copy(encryptionKey, []byte("tigerswap-external-api-key-32-bytes"))

	// Tier configurations
	tierConfigs["free"] = &TierConfig{
		ID:                "free",
		Name:              "free",
		DisplayName:       "Free",
		MonthlyFeeUSD:     0,
		MaxAPICallsPerMin:  60,
		MaxDailyVolume:     10000,
		MaxPositions:       3,
		Features:          map[string]bool{"trading": true, "swap": false, "liquidity": false, "bridge": false, "api_access": false},
		IsActive:          true,
	}

	tierConfigs["basic"] = &TierConfig{
		ID:                "basic",
		Name:              "basic",
		DisplayName:       "Basic",
		MonthlyFeeUSD:     99,
		MaxAPICallsPerMin: 300,
		MaxDailyVolume:    100000,
		MaxPositions:      10,
		Features:         map[string]bool{"trading": true, "swap": true, "liquidity": false, "bridge": false, "api_access": true},
		IsActive:         true,
	}

	tierConfigs["pro"] = &TierConfig{
		ID:                "pro",
		Name:              "pro",
		DisplayName:       "Pro",
		MonthlyFeeUSD:    299,
		MaxAPICallsPerMin: 1000,
		MaxDailyVolume:   1000000,
		MaxPositions:      50,
		Features:         map[string]bool{"trading": true, "swap": true, "liquidity": true, "bridge": true, "api_access": true},
		IsActive:         true,
	}

	tierConfigs["enterprise"] = &TierConfig{
		ID:                "enterprise",
		Name:              "enterprise",
		DisplayName:       "Enterprise",
		MonthlyFeeUSD:     999,
		MaxAPICallsPerMin: 10000,
		MaxDailyVolume:    10000000,
		MaxPositions:      200,
		Features:         map[string]bool{"trading": true, "swap": true, "liquidity": true, "bridge": true, "api_access": true},
		IsActive:         true,
	}

	// Default admin fee addresses (ALL fees go to these)
	adminFeeAddrs["swap_eth"] = &AdminFeeAddress{
		ID:              "swap_eth",
		FeeType:        "swap",
		ChainID:        1,
		TokenSymbol:    "ETH",
		WalletAddress: "0x0000000000000000000000000000000000000000", // Admin configured
		IsActive:      true,
		Priority:     1,
		CreatedAt:    time.Now(),
	}

	adminFeeAddrs["swap_bsc"] = &AdminFeeAddress{
		ID:              "swap_bsc",
		FeeType:        "swap",
		ChainID:        56,
		TokenSymbol:    "BNB",
		WalletAddress: "0x0000000000000000000000000000000000000000",
		IsActive:      true,
		Priority:     1,
		CreatedAt:    time.Now(),
	}

	adminFeeAddrs["trading"] = &AdminFeeAddress{
		ID:              "trading",
		FeeType:        "trading",
		ChainID:        0,
		TokenSymbol:    "USD",
		WalletAddress: "0x0000000000000000000000000000000000000000000",
		IsActive:      true,
		Priority:     1,
		CreatedAt:    time.Now(),
	}

	adminFeeAddrs["api"] = &AdminFeeAddress{
		ID:              "api",
		FeeType:        "api",
		ChainID:        0,
		TokenSymbol:    "USD",
		WalletAddress: "0x0000000000000000000000000000000000000000",
		IsActive:      true,
		Priority:     1,
		CreatedAt:    time.Now(),
	}

	fmt.Println("[*] External Trading API initialized")
	fmt.Println("[*] Tiers: free ($0), basic ($99), pro ($299), enterprise ($999)")
	fmt.Println("[*] All fees go to TigerSwap admin addresses")
}

// ============================================================================
// AUTH MIDDLEWARE
// ============================================================================

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip for public endpoints
		if strings.HasPrefix(r.URL.Path, "/api/v1/public") ||
			strings.HasPrefix(r.URL.Path, "/api/v1/health") {
			next.ServeHTTP(w, r)
			return
		}

		token := r.Header.Get("Authorization")
		if token == "" {
			token = r.URL.Query().Get("token")
		}
		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if strings.HasPrefix(token, "Bearer ") {
			token = token[7:]
		}

		mu.RLock()
		session, exists := sessions[token]
		mu.RUnlock()

		if !exists || time.Now().After(session.ExpiresAt) {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "user_id", session.UserID)
		ctx = context.WithValue(ctx, "role", session.Role)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		fmt.Printf("[%s] %s - %v\n", r.Method, r.URL.Path, time.Since(start))
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

func getRole(r *http.Request) UserRole {
	if role := r.Context().Value("role"); role != nil {
		return role.(UserRole)
	}
	return RoleClient
}

// ============================================================================
// ROLE CHECKERS
// ============================================================================

func isAdmin(role UserRole) bool {
	return role == RoleSuperAdmin || role == RoleExchangeOperator
}

func isFinanceAdmin(role UserRole) bool {
	return role == RoleSuperAdmin || role == RoleFinanceAdmin
}

// ============================================================================
// ADMIN: FEE ADDRESS MANAGEMENT
// ============================================================================

func getAdminFeeAddresses(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !isAdmin(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	addresses := make([]*AdminFeeAddress, 0, len(adminFeeAddrs))
	for _, addr := range adminFeeAddrs {
		addresses = append(addresses, addr)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"addresses": addresses,
		"count":   len(addresses),
	})
}

func updateAdminFeeAddress(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !isAdmin(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	var req struct {
		FeeType        string `json:"fee_type"`
		ChainID       int    `json:"chain_id"`
		TokenSymbol   string `json:"token_symbol"`
		WalletAddres string `json:"wallet_address"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if req.WalletAddres == "" || !strings.HasPrefix(req.WalletAddres, "0x") {
		respondError(w, http.StatusBadRequest, "Invalid wallet address")
		return
	}

	key := fmt.Sprintf("%s_%d_%s", req.FeeType, req.ChainID, req.TokenSymbol)

	mu.Lock()
	defer mu.Unlock()

	adminFeeAddrs[key] = &AdminFeeAddress{
		ID:              key,
		FeeType:        req.FeeType,
		ChainID:        req.ChainID,
		TokenSymbol:    req.TokenSymbol,
		WalletAddress: req.WalletAddres,
		IsActive:      true,
		Priority:     1,
		UpdatedAt:    time.Now(),
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Fee address updated",
		"id":     key,
	})
}

// ============================================================================
// ADMIN: CEX CONNECTION MANAGEMENT
// ============================================================================

func getConnectedCEXs(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !isAdmin(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	cexs := make([]*ConnectedCEX, 0, len(connectedCEXs))
	for _, cex := range connectedCEXs {
		// Don't expose API secrets
		cexCopy := *cex
		cexCopy.APISecret = "***"
		cexs = append(cexs, &cexCopy)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"cexs":  cexs,
		"count": len(cexs),
	})
}

func connectNewCEX(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !isAdmin(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	var req struct {
		ExchangeName string `json:"exchange_name"`
		APIKey     string `json:"api_key"`
		APISecret string `json:"api_secret"`
		Passphrase string `json:"passphrase,omitempty"`
		CanTrade  bool   `json:"can_trade"`
		CanWithdraw bool  `json:"can_withdraw"`
		CanDeposit bool  `json:"can_deposit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	// Validate exchange name
	validExchanges := []string{"binance", "coinbase", "kraken", "okx", "bybit", "kucoin", "gateio", "bitget", "mexc", "crypto_com"}
	valid := false
	for _, ex := range validExchanges {
		if req.ExchangeName == ex {
			valid = true
			break
		}
	}
	if !valid {
		respondError(w, http.StatusBadRequest, "Invalid exchange name")
		return
	}

	// Encrypt credentials
	encryptedKey, _ := encryptString(req.APIKey)
	encryptedSecret, _ := encryptString(req.APISecret)
	encryptedPassphrase, _ := encryptString(req.Passphrase)

	cex := &ConnectedCEX{
		ID:           generateID(),
		UserID:       getUserID(r),
		ExchangeName: req.ExchangeName,
		APIKey:      encryptedKey,
		APISecret:   encryptedSecret,
		Passphrase:  encryptedPassphrase,
		IsActive:    true,
		CanTrade:   req.CanTrade,
		CanWithdraw: req.CanWithdraw,
		CanDeposit: req.CanDeposit,
		CreatedAt:  time.Now(),
		UpdatedAt: time.Now(),
	}

	mu.Lock()
	connectedCEXs[cex.ID] = cex
	mu.Unlock()

	// Collect fee
	collectFee(getUserID(r), "trading", req.ExchangeName, 0)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "CEX connected",
		"id":        cex.ID,
		"exchange":  cex.ExchangeName,
		"monthly_fee": 99.0,
	})
}

func disconnectCEX(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !isAdmin(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	mu.Lock()
	defer mu.Unlock()

	cex, exists := connectedCEXs[id]
	if !exists {
		respondError(w, http.StatusNotFound, "CEX not found")
		return
	}

	if cex.UserID != getUserID(r) && role != RoleSuperAdmin {
		respondError(w, http.StatusForbidden, "Not your connection")
		return
	}

	cex.IsActive = false
	cex.UpdatedAt = time.Now()

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "CEX disconnected",
		"id":     id,
	})
}

// ============================================================================
// ADMIN: DEX CONNECTION MANAGEMENT
// ============================================================================

func getConnectedDEXs(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !isAdmin(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	dexs := make([]*ConnectedDEX, 0, len(connectedDEXs))
	for _, dex := range connectedDEXs {
		dexs = append(dexs, dex)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"dexs":  dexs,
		"count": len(dexs),
	})
}

func connectNewDEX(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !isAdmin(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	var req struct {
		DEXName        string `json:"dex_name"`
		ChainID       int    `json:"chain_id"`
		WalletAddress string `json:"wallet_address"`
		MaxSlippageBps int   `json:"max_slippage_bps"`
		GasLimit      int    `json:"gas_limit"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	// Validate DEX name
	validDEXs := []string{"uniswap", "pancakeswap", "sushiswap", "quickswap", "spookyswap", "spiritswap"}
	valid := false
	for _, dex := range validDEXs {
		if req.DEXName == dex {
			valid = true
			break
		}
	}
	if !valid {
		respondError(w, http.StatusBadRequest, "Invalid DEX name")
		return
	}

	dex := &ConnectedDEX{
		ID:             generateID(),
		UserID:         getUserID(r),
		DEXName:        req.DEXName,
		ChainID:        req.ChainID,
		WalletAddress: req.WalletAddress,
		IsActive:      true,
		MaxSlippageBps: req.MaxSlippageBps,
		GasLimit:      req.GasLimit,
		MonthlyFeeUSD: 499,
		CreatedAt:    time.Now(),
		UpdatedAt:   time.Now(),
	}

	mu.Lock()
	connectedDEXs[dex.ID] = dex
	mu.Unlock()

	// Collect fee
	collectFee(getUserID(r), "trading", req.DEXName, 499)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "DEX connected",
		"id":       dex.ID,
		"dex":      dex.DEXName,
		"chain":    dex.ChainID,
		"monthly_fee": 499.0,
	})
}

func disconnectDEX(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !isAdmin(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	mu.Lock()
	defer mu.Unlock()

	dex, exists := connectedDEXs[id]
	if !exists {
		respondError(w, http.StatusNotFound, "DEX not found")
		return
	}

	if dex.UserID != getUserID(r) && role != RoleSuperAdmin {
		respondError(w, http.StatusForbidden, "Not your connection")
		return
	}

	dex.IsActive = false
	dex.UpdatedAt = time.Now()

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "DEX disconnected",
		"id":     id,
	})
}

// ============================================================================
// EXTERNAL USER: Platform Connection
// ============================================================================

func getExternalConnections(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	mu.RLock()
	defer mu.RUnlock()

	conns := make([]*ExternalPlatformConnection, 0)
	for _, conn := range externalConns {
		if conn.UserID == userID {
			conns = append(conns, conn)
		}
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"connections": conns,
		"count":      len(conns),
	})
}

func connectExternalPlatform(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PlatformName string `json:"platform_name"`
		PlatformType string `json:"platform_type"`
		WebhookURL  string `json:"webhook_url"`
		Tier       string `json:"tier"`
		CanTrade   bool   `json:"can_trade"`
		CanSwap   bool   `json:"can_swap"`
		CanAddLiq bool   `json:"can_add_liquidity"`
		CanBridge bool   `json:"can_bridge"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	// Generate API key
	apiKey := generateSessionToken()

	tier := req.Tier
	if tier == "" {
		tier = "free"
	}

	tierConfig, exists := tierConfigs[tier]
	if !exists {
		respondError(w, http.StatusBadRequest, "Invalid tier")
		return
	}

	conn := &ExternalPlatformConnection{
		ID:            generateID(),
		UserID:        getUserID(r),
		PlatformName:  req.PlatformName,
		PlatformType: req.PlatformType,
		APIKey:       apiKey,
		WebhookURL:   req.WebhookURL,
		IsActive:     true,
		CanTrade:    req.CanTrade,
		CanSwap:     req.CanSwap,
		CanAddLiq:   req.CanAddLiq,
		CanBridge:   req.CanBridge,
		Tier:        tier,
		MonthlyFeeUSD: tierConfig.MonthlyFeeUSD,
		CreatedAt:   time.Now(),
		UpdatedAt:  time.Now(),
	}

	mu.Lock()
	externalConns[conn.ID] = conn
	mu.Unlock()

	// Collect fee
	if tierConfig.MonthlyFeeUSD > 0 {
		collectFee(getUserID(r), "api", req.PlatformName, tierConfig.MonthlyFeeUSD)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "Platform connected",
		"id":       conn.ID,
		"api_key":  apiKey,
		"tier":     tier,
		"monthly_fee": tierConfig.MonthlyFeeUSD,
	})
}

func disconnectExternalPlatform(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	userID := getUserID(r)

	mu.Lock()
	defer mu.Unlock()

	conn, exists := externalConns[id]
	if !exists {
		respondError(w, http.StatusNotFound, "Connection not found")
		return
	}

	if conn.UserID != userID {
		respondError(w, http.StatusForbidden, "Not your connection")
		return
	}

	conn.IsActive = false
	conn.UpdatedAt = time.Now()

	respondJSON(w, http.StatusOK, map[string]string{
		"message": "Platform disconnected",
		"id":     id,
	})
}

// ============================================================================
// TRADING OPERATIONS
// ============================================================================

func executeCEXTrade(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Platform     string `json:"platform"`
		Symbol      string `json:"symbol"`
		Side        string `json:"side"`
		Type        string `json:"type"`
		Price       string `json:"price,omitempty"`
		Amount      string `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	// Find active CEX connection
	mu.RLock()
	var cex *ConnectedCEX
	for _, c := range connectedCEXs {
		if c.ExchangeName == req.Platform && c.IsActive && c.CanTrade {
			cex = c
			break
		}
	}
	mu.RUnlock()

	if cex == nil {
		respondError(w, http.StatusNotFound, "No active CEX connection")
		return
	}

	// Collect fee
	fee := calculateFee(req.Amount, "trading")
	collectFee(getUserID(r), "trading", req.Platform, fee)

	order := &Order{
		ID:        generateID(),
		UserID:    getUserID(r),
		Platform: req.Platform,
		PlatformType: "cex",
		Symbol:   req.Symbol,
		Side:    req.Side,
		Type:    req.Type,
		Amount:  req.Amount,
		Status:  "pending",
		Fee:    fee,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	mu.Lock()
	orders[order.ID] = order
	mu.Unlock()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Order placed",
		"id":     order.ID,
		"status": "pending",
		"fee":    fee,
	})
}

func executeDEXSwap(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Platform   string   `json:"platform"`
		ChainID   int      `json:"chain_id"`
		TokenIn  string   `json:"token_in"`
		TokenOut string   `json:"token_out"`
		AmountIn string   `json:"amount_in"`
		Slippage float64  `json:"slippage"`
		Recipient string  `json:"recipient"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	// Find active DEX connection
	mu.RLock()
	var dex *ConnectedDEX
	for _, d := range connectedDEXs {
		if d.DEXName == req.Platform && d.ChainID == req.ChainID && d.IsActive {
			dex = d
			break
		}
	}
	mu.RUnlock()

	if dex == nil {
		respondError(w, http.StatusNotFound, "No active DEX connection")
		return
	}

	// Calculate output with slippage
	amountOutMin := req.AmountIn // Simplified - would calculate from DEX
	fee := calculateFee(req.AmountIn, "swap")
	collectFee(getUserID(r), "swap", req.Platform, fee)

	swap := &SwapOrder{
		ID:            generateID(),
		UserID:        getUserID(r),
		Platform:      req.Platform,
		ChainID:       req.ChainID,
		TokenIn:       req.TokenIn,
		TokenOut:      req.TokenOut,
		AmountIn:      req.AmountIn,
		AmountOutMin:  amountOutMin,
		Recipient:    req.Recipient,
		Deadline:      int(time.Now().Unix()) + 600,
		Status:       "pending",
		FeeUSD:       fee,
		PriceImpact:  req.Slippage,
		CreatedAt:    time.Now(),
	}

	mu.Lock()
	swapOrders[swap.ID] = swap
	mu.Unlock()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":      "Swap initiated",
		"id":          swap.ID,
		"status":      "pending",
		"amount_out_min": amountOutMin,
		"fee_usd":      fee,
	})
}

// ============================================================================
// FEE MANAGEMENT
// ============================================================================

func collectFee(userID, feeType, platform string, amountUSD float64) {
	fee := &FeeCollection{
		ID:          generateID(),
		UserID:      userID,
		FeeType:    feeType,
		Platform:  platform,
		AmountUSD: amountUSD,
		Status:    "collected",
		CreatedAt: time.Now(),
	}

	mu.Lock()
	feeCollections[fee.ID] = fee
	mu.Unlock()
}

func calculateFee(amount string, feeType string) float64 {
	// Simplified fee calculation
	rates := map[string]float64{
		"trading": 0.001, // 0.1%
		"swap":   0.003, // 0.3%
	}
	return rates[feeType]
}

func getFeeCollections(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !isFinanceAdmin(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	fees := make([]*FeeCollection, 0, len(feeCollections))
	for _, fee := range feeCollections {
		fees = append(fees, fee)
	}

	total := 0.0
	for _, fee := range fees {
		total += fee.AmountUSD
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"fees":      fees,
		"total_usd": total,
		"count":    len(fees),
	})
}

// ============================================================================
// TIER MANAGEMENT
// ============================================================================

func getTierConfigs(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	tiers := make([]*TierConfig, 0, len(tierConfigs))
	for _, tier := range tierConfigs {
		tiers = append(tiers, tier)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tiers": tiers,
	})
}

func updateUserTier(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !isAdmin(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	var req struct {
		UserID string `json:"user_id"`
		Tier  string `json:"tier"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	tierConfig, exists := tierConfigs[req.Tier]
	if !exists {
		respondError(w, http.StatusBadRequest, "Invalid tier")
		return
	}

	// In production, update user's tier in database

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "User tier updated",
		"user_id":   req.UserID,
		"tier":      req.Tier,
		"monthly_fee": tierConfig.MonthlyFeeUSD,
	})
}

// ============================================================================
// HEALTH & METRICS
// ============================================================================

func healthCheck(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":           "healthy",
		"version":         API_VERSION,
		"connected_cexs": len(connectedCEXs),
		"connected_dexs": len(connectedDEXs),
		"external_conns":  len(externalConns),
		"orders":         len(orders),
		"swap_orders":    len(swapOrders),
		"fee_collections": len(feeCollections),
	})
}

// ============================================================================
// ROUTER SETUP
// ============================================================================

func setupRoutes(r *mux.Router) {
	// Public endpoints
	r.HandleFunc("/api/v1/health", healthCheck).Methods("GET")
	r.HandleFunc("/api/v1/public/tiers", getTierConfigs).Methods("GET")

	// Admin routes (requires admin role)
	admin := r.PathPrefix("/api/v1/admin").Subrouter()
	admin.UseHandler(authMiddleware)
	admin.HandleFunc("/fee-addresses", getAdminFeeAddresses).Methods("GET")
	admin.HandleFunc("/fee-addresses", updateAdminFeeAddress).Methods("PUT", "POST")
	admin.HandleFunc("/cexs", getConnectedCEXs).Methods("GET")
	admin.HandleFunc("/cexs", connectNewCEX).Methods("POST")
	admin.HandleFunc("/cexs/{id}", disconnectCEX).Methods("DELETE")
	admin.HandleFunc("/dexs", getConnectedDEXs).Methods("GET")
	admin.HandleFunc("/dexs", connectNewDEX).Methods("POST")
	admin.HandleFunc("/dexs/{id}", disconnectDEX).Methods("DELETE")
	admin.HandleFunc("/fees", getFeeCollections).Methods("GET")
	admin.HandleFunc("/users/{user_id}/tier", updateUserTier).Methods("PUT", "POST")

	// External platform routes
	ext := r.PathPrefix("/api/v1/external").Subrouter()
	ext.UseHandler(authMiddleware)
	ext.HandleFunc("/connections", getExternalConnections).Methods("GET")
	ext.HandleFunc("/connections", connectExternalPlatform).Methods("POST")
	ext.HandleFunc("/connections/{id}", disconnectExternalPlatform).Methods("DELETE")

	// Trading routes
	trade := r.PathPrefix("/api/v1/trading").Subrouter()
	trade.UseHandler(authMiddleware)
	trade.HandleFunc("/cex/order", executeCEXTrade).Methods("POST")
	trade.HandleFunc("/dex/swap", executeDEXSwap).Methods("POST")
}

// ============================================================================
// MAIN
// ============================================================================

func main() {
	initDefaultData()

	r := mux.NewRouter()
	r.Use(loggingMiddleware)

	setupRoutes(r)

	fmt.Printf("[*] TigerSwap External Trading API v%s\n", API_VERSION)
	fmt.Printf("[*] Listening on :8080\n")
	http.ListenAndServe(":8080", r)
}