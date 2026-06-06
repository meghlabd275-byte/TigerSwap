// TigerSwap Complete Admin API
// Full admin management for all platform operations
// All fees go to admin addresses - complete dynamic configuration

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

	"github.com/gorilla/mev"
)

// ============================================================================
// CONSTANTS
// ============================================================================

const (
	API_VERSION = "2.0.0"
)

// ============================================================================
// ENUMS
// ============================================================================

type UserRole string

const (
	RoleSuperAdmin       UserRole = "super_admin"
	RoleAdmin           UserRole = "admin"
	RoleFinanceAdmin     UserRole = "finance_admin"
	RoleBotOperator     UserRole = "bot_operator"
	RoleTradingAdmin    UserRole = "trading_admin"
	RoleClient          UserRole = "client"
)

type FeeType string

const (
	FeeSwap          FeeType = "swap"
	FeeLiquidity    FeeType = "liquidity"
	FeeWithdrawal  FeeType = "withdrawal"
	FeeDeposit     FeeType = "deposit"
	FeeBot         FeeType = "bot_subscription"
	FeeAPI         FeeType = "api_key"
	FeeListing     FeeType = "listing"
	FeeTrading     FeeType = "trading"
	FeeTransfer    FeeType = "transfer"
)

type BlockchainType string

const (
	BlockchainEVM     BlockchainType = "evm"
	BlockchainSolana  BlockchainType = "solana"
	BlockchainAptos   BlockchainType = "aptos"
	BlockchainSui     BlockchainType = "sui"
	BlockchainTon     BlockchainType = "ton"
	BlockchainCosmos  BlockchainType = "cosmos"
	BlockchainPi      BlockchainType = "pinetwork"
)

// ============================================================================
// MODELS - Complete Admin Management
// ============================================================================

// Admin User
type AdminUser struct {
	ID                string    `json:"id"`
	WalletAddress    string    `json:"wallet_address"`
	Email            string    `json:"email"`
	Username         string    `json:"username"`
	Role             UserRole  `json:"role"`
	IsActive         bool      `json:"is_active"`
	Permissions      []string  `json:"permissions"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
	LastLoginAt       time.Time `json:"last_login_at"`
}

// Blockchain Configuration
type Blockchain struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Symbol          string          `json:"symbol"`
	ChainId         int             `json:"chain_id"`
	ChainIdHex      string          `json:"chain_id_hex"`
	Type            BlockchainType  `json:"type"`
	RPCUrl          string          `json:"rpc_url"`
	ExplorerUrl     string          `json:"explorer_url"`
	ExplorerApiUrl string          `json:"explorer_api_url"`
	ExplorerApiKey string          `json:"explorer_api_key"`
	NativeToken    string          `json:"native_token"`
	Decimals       int             `json:"decimals"`
	Slip44         int             `json:"slip44"`
	IsActive       bool            `json:"is_active"`
	AvgGasPriceGwei float64        `json:"avg_gas_price_gwei"`
	IsTestnet      bool            `json:"is_testnet"`
	LogoUrl        string          `json:"logo_url"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// Token Configuration
type Token struct {
	ID                string    `json:"id"`
	ChainId          int       `json:"chain_id"`
	Symbol           string    `json:"symbol"`
	Name             string    `json:"name"`
	ContractAddress string    `json:"contract_address"`
	Decimals         int       `json:"decimals"`
	IsActive         bool      `json:"is_active"`
	IsVerified       bool      `json:"is_verified"`
	IsStablecoin     bool      `json:"is_stablecoin"`
	IsWrappedNative bool      `json:"is_wrapped_native"`
	LogoUrl          string    `json:"logo_url"`
	CoingeckoId     string    `json:"coingecko_id"`
	PriceUsd         float64   `json:"price_usd"`
	MarketCap       float64   `json:"market_cap"`
	Volume24h       float64   `json:"volume_24h"`
	CreatedAt      time.Time `json:"created_at"`
}

// Fee Configuration
type FeeConfig struct {
	ID                string    `json:"id"`
	FeeType          FeeType   `json:"fee_type"`
	ChainId          int      `json:"chain_id"`
	TokenSymbol      string   `json:"token_symbol"`
	FeeAmountUsd    float64  `json:"fee_amount_usd"`
	FeePercentage   float64  `json:"fee_percentage"`
	MinFeeUsd        float64  `json:"min_fee_usd"`
	MaxFeeUsd        float64  `json:"max_fee_usd"`
	IsActive         bool     `json:"is_active"`
	UpdatedBy       string    `json:"updated_by"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Admin Fee Address (where all fees go)
type AdminFeeAddress struct {
	ID              string    `json:"id"`
	FeeType         FeeType   `json:"fee_type"`
	ChainId         int      `json:"chain_id"`
	TokenSymbol     string   `json:"token_symbol"`
	WalletAddress  string   `json:"wallet_address"`
	IsActive       bool     `json:"is_active"`
	Priority       int      `json:"priority"`
	IsPrimary      bool     `json:"is_primary"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// Bot Subscription Tier
type BotTier struct {
	ID               string             `json:"id"`
	Name             string             `json:"name"`
	DisplayName      string             `json:"display_name"`
	MonthlyFeeUsd   float64           `json:"monthly_fee_usd"`
	PerDexFeeUsd    float64           `json:"per_dex_fee_usd"`
	PerCexFeeUsd    float64           `json:"per_cex_fee_usd"`
	MaxBots         int               `json:"max_bots"`
	MaxDexs         int               `json:"max_dexs"`
	MaxCexs        int               `json:"max_cexs"`
	MaxPositionUsd   float64           `json:"max_position_usd"`
	MaxDailyVolume  float64           `json:"max_daily_volume"`
	LatencyTargetMs int               `json:"latency_target_ms"`
	Features       map[string]bool    `json:"features"`
	IsActive       bool              `json:"is_active"`
}

// Bot Instance
type BotInstance struct {
	ID               string    `json:"id"`
	UserId           string    `json:"user_id"`
	BotType          string    `json:"bot_type"`
	Name            string    `json:"name"`
	Status          string    `json:"status"` // running, stopped, paused, error
	ConnectedDexs   []string `json:"connected_dexs"`
	ConnectedCexs  []string `json:"connected_cexs"`
	TradingPairs    []string `json:"trading_pairs"`
	TotalPnl        float64  `json:"total_pnl"`
	TotalVolume    float64  `json:"total_volume"`
	TotalOrders    int      `json:"total_orders"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	LastTradeAt    time.Time `json:"last_trade_at"`
}

// External API Connection (for external users)
type ExternalConnection struct {
	ID             string    `json:"id"`
	UserId         string    `json:"user_id"`
	PlatformName  string    `json:"platform_name"`
	PlatformType  string    `json:"platform_type"` // cex, dex, wallet
	ApiKey        string    `json:"api_key"`
	ApiSecret     string    `json:"api_secret"`
	IsActive      bool      `json:"is_active"`
	CanTrade     bool      `json:"can_trade"`
	CanSwap      bool      `json:"can_swap"`
	CanAddLiq   bool      `json:"can_add_liquidity"`
	CanBridge    bool      `json:"can_bridge"`
	Tier         string    `json:"tier"`
	RateLimitMin  int       `json:"rate_limit_per_minute"`
	MonthlyFeeUsd float64  `json:"monthly_fee_usd"`
	TotalFeesPaid float64  `json:"total_fees_paid"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Listing Request
type ListingRequest struct {
	ID              string    `json:"id"`
	TokenId        string    `json:"token_id"`
	ChainId        int       `json:"chain_id"`
	Symbol         string    `json:"symbol"`
	Name           string    `json:"name"`
	ContractAddress string   `json:"contract_address"`
	Tier           string    `json:"tier"` // basic, standard, premium, premium_plus
	Status        string    `json:"status"` // pending, approved, rejected
	OneTimeFeeUsd  float64  `json:"one_time_fee_usd"`
	MonthlyFeeUsd   float64  `json:"monthly_fee_usd"`
	RequestedBy   string    `json:"requested_by"`
	ApprovedBy    string    `json:"approved_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Fee Collection Record
type FeeCollection struct {
	ID              string    `json:"id"`
	UserId         string    `json:"user_id"`
	FeeType        FeeType   `json:"fee_type"`
	ChainId        int      `json:"chain_id"`
	AmountUsd     float64  `json:"amount_usd"`
	AmountToken   string   `json:"amount_token"`
	TokenSymbol   string   `json:"token_symbol"`
	TxHash        string   `json:"tx_hash"`
	Status        string   `json:"status"` // pending, collected, distributed
	CollectedAt   time.Time `json:"collected_at"`
}

// ============================================================================
// DATABASE (In-Memory)
// ============================================================================

var (
	// Admin management
	adminUsers    = make(map[string]*AdminUser)
	adminSessions = make(map[string]*Session)

	// Blockchain management
	blockchains = make(map[string]*Blockchain)
	tokens      = make(map[string]*Token)

	// Fee management
	feeConfigs       = make(map[string]*FeeConfig)
	adminFeeAddresses = make(map[string]*AdminFeeAddress)
	feeCollections   = make(map[string]*FeeCollection)

	// Bot management
	botTiers     = make(map[string]*BotTier)
	botInstances = make(map[string]*BotInstance)

	// External connections
	externalConnections = make(map[string]*ExternalConnection)

	// Listing management
	listingRequests = make(map[string]*ListingRequest)

	// Security
	encryptionKey []byte
	mu           sync.RWMutex
)

type Session struct {
	Token     string    `json:"token"`
	UserId    string    `json:"user_id"`
	Role      UserRole  `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

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

func encrypt(data []byte) ([]byte, error) {
	block, _ := aes.NewCipher(encryptionKey)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	rand.Read(nonce)
	return gcm.Seal(nonce, nonce, data, nil), nil
}

func decrypt(data []byte) ([]byte, error) {
	block, _ := aes.NewCipher(encryptionKey)
	gcm, _ := cipher.NewGCM(block)
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// ============================================================================
// ROLE CHECKERS
// ============================================================================

func canManageAll(role UserRole) bool {
	return role == RoleSuperAdmin || role == RoleAdmin
}

func canManageFees(role UserRole) bool {
	return role == RoleSuperAdmin || role == RoleFinanceAdmin
}

func canManageBots(role UserRole) bool {
	return role == RoleSuperAdmin || role == RoleBotOperator
}

func canManageTrading(role UserRole) bool {
	return role == RoleSuperAdmin || role == RoleTradingAdmin
}

// ============================================================================
// INITIALIZATION
// ============================================================================

func initDefaultData() {
	// Initialize encryption key
	encryptionKey = make([]byte, 32)
	copy(encryptionKey, []byte("tiger-admin-api-key-32-bytes!!"))

	// Initialize blockchains
	blockchains["1"] = &Blockchain{
		ID: "1", Name: "Ethereum", Symbol: "ETH", ChainId: 1, ChainIdHex: "0x1",
		Type: BlockchainEVM, RPCUrl: "https://eth.llamarpc.com", ExplorerUrl: "https://etherscan.io",
		NativeToken: "ETH", Decimals: 18, Slip44: 60, IsActive: true, AvgGasPriceGwei: 20.0,
	}
	blockchains["56"] = &Blockchain{
		ID: "56", Name: "BNB Chain", Symbol: "BNB", ChainId: 56, ChainIdHex: "0x38",
		Type: BlockchainEVM, RPCUrl: "https://bsc-dataseed.binance.org", ExplorerUrl: "https://bscscan.com",
		NativeToken: "BNB", Decimals: 18, Slip44: 60, IsActive: true, AvgGasPriceGwei: 3.0,
	}
	blockchains["137"] = &Blockchain{
		ID: "137", Name: "Polygon", Symbol: "MATIC", ChainId: 137, ChainIdHex: "0x89",
		Type: BlockchainEVM, RPCUrl: "https://polygon-rpc.com", ExplorerUrl: "https://polygonscan.com",
		NativeToken: "MATIC", Decimals: 18, Slip44: 60, IsActive: true, AvgGasPriceGwei: 50.0,
	}
	blockchains["42161"] = &Blockchain{
		ID: "42161", Name: "Arbitrum One", Symbol: "ETH", ChainId: 42161, ChainIdHex: "0xa4b1",
		Type: BlockchainEVM, RPCUrl: "https://arb1.arbitrum.io/rpc", ExplorerUrl: "https://arbiscan.io",
		NativeToken: "ETH", Decimals: 18, Slip44: 60, IsActive: true, AvgGasPriceGwei: 0.1,
	}
	blockchains["10"] = &Blockchain{
		ID: "10", Name: "Optimism", Symbol: "ETH", ChainId: 10, ChainIdHex: "0xa",
		Type: BlockchainEVM, RPCUrl: "https://mainnet.optimism.io", ExplorerUrl: "https://optimistic.etherscan.io",
		NativeToken: "ETH", Decimals: 18, Slip44: 60, IsActive: true, AvgGasPriceGwei: 0.001,
	}
	blockchains["8453"] = &Blockchain{
		ID: "8453", Name: "Base", Symbol: "ETH", ChainId: 8453, ChainIdHex: "0x2105",
		Type: BlockchainEVM, RPCUrl: "https://mainnet.base.org", ExplorerUrl: "https://basescan.org",
		NativeToken: "ETH", Decimals: 18, Slip44: 60, IsActive: true, AvgGasPriceGwei: 0.001,
	}
	blockchains["43114"] = &Blockchain{
		ID: "43114", Name: "Avalanche", Symbol: "AVAX", ChainId: 43114, ChainIdHex: "0xa86a",
		Type: BlockchainEVM, RPCUrl: "https://api.avax.network/ext/bc/C/rpc", ExplorerUrl: "https://snowtrace.io",
		NativeToken: "AVAX", Decimals: 18, Slip44: 60, IsActive: true, AvgGasPriceGwei: 25.0,
	}
	blockchains["101"] = &Blockchain{
		ID: "101", Name: "Solana", Symbol: "SOL", ChainId: 101, ChainIdHex: "101",
		Type: BlockchainSolana, RPCUrl: "https://api.mainnet-beta.solana.com", ExplorerUrl: "https://explorer.solana.com",
		NativeToken: "SOL", Decimals: 9, Slip44: 501, IsActive: true, AvgGasPriceGwei: 0.00025,
	}
	blockchains["1100"] = &Blockchain{
		ID: "1100", Name: "Aptos", Symbol: "APT", ChainId: 1100, ChainIdHex: "1100",
		Type: BlockchainAptos, RPCUrl: "https://fullnode.mainnet.aptoslabs.com", ExplorerUrl: "https://explorer.aptoslabs.com",
		NativeToken: "APT", Decimals: 8, Slip44: 637, IsActive: true, AvgGasPriceGwei: 100.0,
	}
	blockchains["7821"] = &Blockchain{
		ID: "7821", Name: "Sui", Symbol: "SUI", ChainId: 7821, ChainIdHex: "7821",
		Type: BlockchainSui, RPCUrl: "https://fullnode.mainnet.sui.io", ExplorerUrl: "https://explorer.sui.io",
		NativeToken: "SUI", Decimals: 9, Slip44: 784, IsActive: true, AvgGasPriceGwei: 1000.0,
	}
	blockchains["6060"] = &Blockchain{
		ID: "6060", Name: "Toncoin", Symbol: "TON", ChainId: 6060, ChainIdHex: "6060",
		Type: BlockchainTon, RPCUrl: "https://toncenter.com/api/v2", ExplorerUrl: "https://tonviewer.com",
		NativeToken: "TON", Decimals: 9, Slip44: 607, IsActive: true, AvgGasPriceGwei: 1.0,
	}
	blockchains["3141"] = &Blockchain{
		ID: "3141", Name: "Pi Network", Symbol: "PI", ChainId: 3141, ChainIdHex: "3141",
		Type: BlockchainPi, RPCUrl: "https://minepi.com/api/gateway", ExplorerUrl: "https://explorer.minepi.com",
		NativeToken: "PI", Decimals: 18, Slip44: 314159, IsActive: true, AvgGasPriceGwei: 0.0,
	}

	// Initialize fee configs
	feeConfigs["swap"] = &FeeConfig{
		ID: "swap", FeeType: FeeSwap, FeeAmountUsd: 0, FeePercentage: 0.3,
		MinFeeUsd: 0.01, IsActive: true,
	}
	feeConfigs["liquidity"] = &FeeConfig{
		ID: "liquidity", FeeType: FeeLiquidity, FeeAmountUsd: 0, FeePercentage: 0.25,
		IsActive: true,
	}
	feeConfigs["withdrawal"] = &FeeConfig{
		ID: "withdrawal", FeeType: FeeWithdrawal, FeeAmountUsd: 0, FeePercentage: 0.1,
		MinFeeUsd: 1.0, IsActive: true,
	}
	feeConfigs["bot_tier1"] = &FeeConfig{
		ID: "bot_tier1", FeeType: FeeBot, FeeAmountUsd: 2500, FeePercentage: 0,
		IsActive: true,
	}
	feeConfigs["bot_tier2"] = &FeeConfig{
		ID: "bot_tier2", FeeType: FeeBot, FeeAmountUsd: 5000, FeePercentage: 0,
		IsActive: true,
	}
	feeConfigs["bot_tier3"] = &FeeConfig{
		ID: "bot_tier3", FeeType: FeeBot, FeeAmountUsd: 10000, FeePercentage: 0,
		IsActive: true,
	}
	feeConfigs["api_free"] = &FeeConfig{
		ID: "api_free", FeeType: FeeAPI, FeeAmountUsd: 0, FeePercentage: 0,
		IsActive: true,
	}
	feeConfigs["api_basic"] = &FeeConfig{
		ID: "api_basic", FeeType: FeeAPI, FeeAmountUsd: 99, FeePercentage: 0,
		IsActive: true,
	}
	feeConfigs["api_pro"] = &FeeConfig{
		ID: "api_pro", FeeType: FeeAPI, FeeAmountUsd: 299, FeePercentage: 0,
		IsActive: true,
	}
	feeConfigs["api_enterprise"] = &FeeConfig{
		ID: "api_enterprise", FeeType: FeeAPI, FeeAmountUsd: 999, FeePercentage: 0,
		IsActive: true,
	}
	feeConfigs["listing_basic"] = &FeeConfig{
		ID: "listing_basic", FeeType: FeeListing, FeeAmountUsd: 5000, FeePercentage: 0,
		IsActive: true,
	}
	feeConfigs["listing_standard"] = &FeeConfig{
		ID: "listing_standard", FeeType: FeeListing, FeeAmountUsd: 10000, FeePercentage: 0,
		IsActive: true,
	}
	feeConfigs["listing_premium"] = &FeeConfig{
		ID: "listing_premium", FeeType: FeeListing, FeeAmountUsd: 15000, FeePercentage: 0,
		IsActive: true,
	}
	feeConfigs["listing_premium_plus"] = &FeeConfig{
		ID: "listing_premium_plus", FeeType: FeeListing, FeeAmountUsd: 25000, FeePercentage: 0,
		IsActive: true,
	}

	// Initialize bot tiers
	botTiers["tier_1"] = &BotTier{
		ID: "tier_1", Name: "tier_1", DisplayName: "Basic",
		MonthlyFeeUsd: 2500, PerDexFeeUsd: 500, PerCexFeeUsd: 50,
		MaxBots: 1, MaxDexs: 5, MaxCexs: 20, MaxPositionUsd: 100000, MaxDailyVolume: 1000000,
		LatencyTargetMs: 100, Features: map[string]bool{"arbitrage": true, "sniping": false},
		IsActive: true,
	}
	botTiers["tier_2"] = &BotTier{
		ID: "tier_2", Name: "tier_2", DisplayName: "Pro",
		MonthlyFeeUsd: 5000, PerDexFeeUsd: 750, PerCexFeeUsd: 75,
		MaxBots: 3, MaxDexs: 10, MaxCexs: 50, MaxPositionUsd: 500000, MaxDailyVolume: 5000000,
		LatencyTargetMs: 50, Features: map[string]bool{"arbitrage": true, "sniping": true, "mev": true},
		IsActive: true,
	}
	botTiers["tier_3"] = &BotTier{
		ID: "tier_3", Name: "tier_3", DisplayName: "Enterprise",
		MonthlyFeeUsd: 10000, PerDexFeeUsd: 1000, PerCexFeeUsd: 100,
		MaxBots: 10, MaxDexs: 20, MaxCexs: 200, MaxPositionUsd: 5000000, MaxDailyVolume: 50000000,
		LatencyTargetMs: 10, Features: map[string]bool{"arbitrage": true, "sniping": true, "mev": true, "flash_loan": true},
		IsActive: true,
	}

	// Initialize admin fee addresses (ALL FEES GO HERE)
	adminFeeAddresses["swap_eth"] = &AdminFeeAddress{
		ID: "swap_eth", FeeType: FeeSwap, ChainId: 1, TokenSymbol: "ETH",
		WalletAddress: "0x0000000000000000000000000000000000000000", IsActive: true, Priority: 1, IsPrimary: true,
	}
	adminFeeAddresses["swap_bsc"] = &AdminFeeAddress{
		ID: "swap_bsc", FeeType: FeeSwap, ChainId: 56, TokenSymbol: "BNB",
		WalletAddress: "0x0000000000000000000000000000000000000000", IsActive: true, Priority: 1, IsPrimary: true,
	}
	adminFeeAddresses["bot"] = &AdminFeeAddress{
		ID: "bot", FeeType: FeeBot, ChainId: 0, TokenSymbol: "USD",
		WalletAddress: "0x0000000000000000000000000000000000000000", IsActive: true, Priority: 1, IsPrimary: true,
	}
	adminFeeAddresses["api"] = &AdminFeeAddress{
		ID: "api", FeeType: FeeAPI, ChainId: 0, TokenSymbol: "USD",
		WalletAddress: "0x0000000000000000000000000000000000000000", IsActive: true, Priority: 1, IsPrimary: true,
	}
	adminFeeAddresses["listing"] = &AdminFeeAddress{
		ID: "listing", FeeType: FeeListing, ChainId: 0, TokenSymbol: "USD",
		WalletAddress: "0x0000000000000000000000000000000000000000", IsActive: true, Priority: 1, IsPrimary: true,
	}

	fmt.Println("[*] Admin API initialized with full management")
	fmt.Println("[*] Blockchains: 12 (7 EVM + 5 Non-EVM)")
	fmt.Println("[*] Fee types: swap, liquidity, withdrawal, bot, api, listing")
	fmt.Println("[*] Bot tiers: Basic ($2500), Pro ($5000), Enterprise ($10000)")
	fmt.Println("[*] All fees go to admin addresses")
}

// ============================================================================
// MIDDLEWARE
// ============================================================================

func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		session, exists := adminSessions[token]
		mu.RUnlock()

		if !exists || time.Now().After(session.ExpiresAt) {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), "user_id", session.UserId)
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

func getUserId(r *http.Request) string {
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
// ADMIN: BLOCKCHAIN MANAGEMENT
// ============================================================================

func getBlockchains(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageAll(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	list := make([]*Blockchain, 0, len(blockchains))
	for _, b := range blockchains {
		list = append(list, b)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"blockchains": list,
		"count":     len(list),
	})
}

func addBlockchain(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageAll(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	var req Blockchain
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	req.ID = fmt.Sprintf("%d", req.ChainId)
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	mu.Lock()
	blockchains[req.ID] = &req
	mu.Unlock()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":    "Blockchain added",
		"blockchain": &req,
	})
}

func updateBlockchain(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageAll(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	mu.Lock()
	blockchain, exists := blockchains[id]
	if !exists {
		mu.Unlock()
		respondError(w, http.StatusNotFound, "Blockchain not found")
		return
	}

	var req Blockchain
	json.NewDecoder(r.Body).Decode(&req)

	blockchain.RPCUrl = req.RPCUrl
	blockchain.ExplorerUrl = req.ExplorerUrl
	blockchain.ExplorerApiUrl = req.ExplorerApiUrl
	blockchain.ExplorerApiKey = req.ExplorerApiKey
	blockchain.IsActive = req.IsActive
	blockchain.AvgGasPriceGwei = req.AvgGasPriceGwei
	blockchain.UpdatedAt = time.Now()

	mu.Unlock()

	respondJSON(w, http.StatusOK, map[string]string{"message": "Blockchain updated"})
}

func deleteBlockchain(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageAll(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	mu.Lock()
	delete(blockchains, id)
	mu.Unlock()

	respondJSON(w, http.StatusOK, map[string]string{"message": "Blockchain deleted"})
}

// ============================================================================
// ADMIN: FEE MANAGEMENT
// ============================================================================

func getFeeConfigs(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageFees(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	list := make([]*FeeConfig, 0, len(feeConfigs))
	for _, f := range feeConfigs {
		list = append(list, f)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"fees":  list,
		"count": len(list),
	})
}

func updateFeeConfig(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageFees(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	var req FeeConfig
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	mu.Lock()
	feeConfigs[req.ID] = &req
	mu.Unlock()

	respondJSON(w, http.StatusOK, map[string]string{"message": "Fee config updated"})
}

func getAdminFeeAddresses(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageFees(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	list := make([]*AdminFeeAddress, 0, len(adminFeeAddresses))
	for _, a := range adminFeeAddresses {
		list = append(list, a)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"addresses": list,
		"count":    len(list),
	})
}

func updateAdminFeeAddress(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageFees(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	var req AdminFeeAddress
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	if !strings.HasPrefix(req.WalletAddress, "0x") || len(req.WalletAddress) != 42 {
		respondError(w, http.StatusBadRequest, "Invalid wallet address")
		return
	}

	req.UpdatedAt = time.Now()

	mu.Lock()
	adminFeeAddresses[req.ID] = &req
	mu.Unlock()

	respondJSON(w, http.StatusOK, map[string]string{"message": "Fee address updated"})
}

// ============================================================================
// ADMIN: BOT MANAGEMENT
// ============================================================================

func getBotTiers(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageBots(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	list := make([]*BotTier, 0, len(botTiers))
	for _, t := range botTiers {
		list = append(list, t)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"tiers": list,
	})
}

func createBotTier(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageBots(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	var req BotTier
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	mu.Lock()
	botTiers[req.ID] = &req
	mu.Unlock()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "Bot tier created",
		"bot_tier": &req,
	})
}

func getBotInstances(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageBots(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	list := make([]*BotInstance, 0, len(botInstances))
	for _, b := range botInstances {
		list = append(list, b)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"bots":  list,
		"count": len(list),
	})
}

func controlBot(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageBots(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	var req struct {
		Action string `json:"action"` // start, stop, pause
	}
	json.NewDecoder(r.Body).Decode(&req)

	mu.Lock()
	bot, exists := botInstances[id]
	if !exists {
		mu.Unlock()
		respondError(w, http.StatusNotFound, "Bot not found")
		return
	}

	switch req.Action {
	case "start":
		bot.Status = "running"
	case "stop":
		bot.Status = "stopped"
	case "pause":
		bot.Status = "paused"
	}
	bot.UpdatedAt = time.Now()

	mu.Unlock()

	respondJSON(w, http.StatusOK, map[string]string{"message": "Bot " + req.Action})
}

// ============================================================================
// ADMIN: EXTERNAL CONNECTIONS MANAGEMENT
// ============================================================================

func getExternalConnections(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageAll(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	list := make([]*ExternalConnection, 0, len(externalConnections))
	for _, c := range externalConnections {
		c.ApiSecret = "***"
		list = append(list, c)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"connections": list,
		"count":       len(list),
	})
}

func createExternalConnection(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageAll(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	var req ExternalConnection
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request")
		return
	}

	req.ID = generateID()
	req.ApiKey = generateSessionToken()

	// Encrypt secret
	encrypted, _ := encrypt([]byte(req.ApiSecret))
	req.ApiSecret = hex.EncodeToString(encrypted)

	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	mu.Lock()
	externalConnections[req.ID] = &req
	mu.Unlock()

	// Collect fee
	collectFee(req.UserId, FeeAPI, 0, req.MonthlyFeeUsd)

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "External connection created",
		"api_key":  req.ApiKey,
		"monthly_fee": req.MonthlyFeeUsd,
	})
}

func updateExternalConnection(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageAll(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	mu.Lock()
	conn, exists := externalConnections[id]
	if !exists {
		mu.Unlock()
		respondError(w, http.StatusNotFound, "Connection not found")
		return
	}

	var req ExternalConnection
	json.NewDecoder(r.Body).Decode(&req)

	conn.IsActive = req.IsActive
	conn.CanTrade = req.CanTrade
	conn.CanSwap = req.CanSwap
	conn.CanAddLiq = req.CanAddLiq
	conn.CanBridge = req.CanBridge
	conn.Tier = req.Tier
	conn.UpdatedAt = time.Now()

	mu.Unlock()

	respondJSON(w, http.StatusOK, map[string]string{"message": "Connection updated"})
}

// ============================================================================
// ADMIN: LISTING MANAGEMENT
// ============================================================================

func getListingRequests(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageAll(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	list := make([]*ListingRequest, 0, len(listingRequests))
	for _, l := range listingRequests {
		list = append(list, l)
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"requests": list,
		"count":   len(list),
	})
}

func approveListing(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageAll(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	vars := mux.Vars(r)
	id := vars["id"]

	mu.Lock()
	listing, exists := listingRequests[id]
	if !exists {
		mu.Unlock()
		respondError(w, http.StatusNotFound, "Listing request not found")
		return
	}

	listing.Status = "approved"
	listing.ApprovedBy = getUserId(r)
	listing.UpdatedAt = time.Now()

	mu.Unlock()

	respondJSON(w, http.StatusOK, map[string]string{"message": "Listing approved"})
}

// ============================================================================
// ADMIN: FEE COLLECTIONS
// ============================================================================

func collectFee(userId string, feeType FeeType, chainId int, amountUsd float64) {
	fee := &FeeCollection{
		ID:          generateID(),
		UserId:      userId,
		FeeType:    feeType,
		ChainId:    chainId,
		AmountUsd: amountUsd,
		Status:    "collected",
		CollectedAt: time.Now(),
	}

	mu.Lock()
	feeCollections[fee.ID] = fee
	mu.Unlock()
}

func getFeeCollections(w http.ResponseWriter, r *http.Request) {
	role := getRole(r)
	if !canManageFees(role) {
		respondError(w, http.StatusForbidden, "Insufficient permissions")
		return
	}

	mu.RLock()
	defer mu.RUnlock()

	list := make([]*FeeCollection, 0, len(feeCollections))
	for _, f := range feeCollections {
		list = append(list, f)
	}

	total := 0.0
	for _, f := range list {
		total += f.AmountUsd
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"fees":      list,
		"total_usd": total,
		"count":    len(list),
	})
}

// ============================================================================
// HEALTH & METRICS
// ============================================================================

func healthCheck(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":              "healthy",
		"version":             API_VERSION,
		"blockchains":         len(blockchains),
		"fee_configs":         len(feeConfigs),
		"admin_fee_addresses":   len(adminFeeAddresses),
		"bot_tiers":           len(botTiers),
		"bot_instances":      len(botInstances),
		"external_connections": len(externalConnections),
		"fee_collections":    len(feeCollections),
	})
}

// ============================================================================
// ROUTER SETUP
// ============================================================================

func setupRoutes(r *mux.Router) {
	// Public
	r.HandleFunc("/api/v1/health", healthCheck).Methods("GET")

	// Blockchain management
	blockchain := r.PathPrefix("/api/v1/admin/blockchains").Subrouter()
	blockchain.UseHandler(authMiddleware)
	blockchain.HandleFunc("", getBlockchains).Methods("GET")
	blockchain.HandleFunc("", addBlockchain).Methods("POST")
	blockchain.HandleFunc("/{id}", updateBlockchain).Methods("PUT")
	blockchain.HandleFunc("/{id}", deleteBlockchain).Methods("DELETE")

	// Fee management
	fees := r.PathPrefix("/api/v1/admin/fees").Subrouter()
	fees.UseHandler(authMiddleware)
	fees.HandleFunc("/configs", getFeeConfigs).Methods("GET")
	fees.HandleFunc("/configs", updateFeeConfig).Methods("PUT", "POST")
	fees.HandleFunc("/addresses", getAdminFeeAddresses).Methods("GET")
	fees.HandleFunc("/addresses", updateAdminFeeAddress).Methods("PUT", "POST")
	fees.HandleFunc("/collections", getFeeCollections).Methods("GET")

	// Bot management
	bots := r.PathPrefix("/api/v1/admin/bots").Subrouter()
	bots.UseHandler(authMiddleware)
	bots.HandleFunc("/tiers", getBotTiers).Methods("GET")
	bots.HandleFunc("/tiers", createBotTier).Methods("POST")
	bots.HandleFunc("/instances", getBotInstances).Methods("GET")
	bots.HandleFunc("/instances/{id}/control", controlBot).Methods("POST")

	// External connections
	ext := r.PathPrefix("/api/v1/admin/external").Subrouter()
	ext.UseHandler(authMiddleware)
	ext.HandleFunc("/connections", getExternalConnections).Methods("GET")
	ext.HandleFunc("/connections", createExternalConnection).Methods("POST")
	ext.HandleFunc("/connections/{id}", updateExternalConnection).Methods("PUT")

	// Listing management
	listings := r.PathPrefix("/api/v1/admin/listings").Subrouter()
	listings.UseHandler(authMiddleware)
	listings.HandleFunc("", getListingRequests).Methods("GET")
	listings.HandleFunc("/{id}/approve", approveListing).Methods("POST")
}

// ============================================================================
// MAIN
// ============================================================================

func main() {
	initDefaultData()

	r := mux.NewRouter()
	r.Use(loggingMiddleware)

	setupRoutes(r)

	fmt.Printf("[*] TigerSwap Admin API v%s\n", API_VERSION)
	fmt.Printf("[*] Full management: blockchains, fees, bots, external connections, listings\n")
	fmt.Printf("[*] Listening on :8081\n")
	http.ListenAndServe(":8081", r)
}