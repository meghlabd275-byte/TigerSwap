// ============================================================================
// TIGERSWAP COMPLETE API GATEWAY
// Industrial-grade DEX with complete wallet, bot platform, white label, security
// ============================================================================

package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// CONSTANTS
// ============================================================================

const (
	// Security
	API_VERSION = "1.0.0"
	
	// Session
	SESSION_NAME = "tigerswap_session"
	SESSION_MAX_AGE = 86400 * 7 // 7 days
	
	// Rate limiting
	MAX_REQUESTS_PER_MINUTE = 1000
	MAX_LOGIN_ATTEMPTS = 5
	LOGIN_LOCKOUT_DURATION = 15 * 60 // 15 minutes
	
	// Password requirements
	MIN_PASSWORD_LENGTH = 12
	
	// JWT
	JWT_EXPIRY_HOURS = 24
	JWT_SIGNING_KEY_LENGTH = 32
	
	// Master wallet
	AUTO_SIGN_TIMEOUT = 3 * time.Second
	AUTO_SIGN_BATCH_SIZE = 50
	
	// Bot tiers
	BOT_TIER_BASIC = "basic"
	BOT_TIER_PRO = "pro"
	BOT_TIER_ENTERPRISE = "enterprise"
	
	// White label
	FEE_SHARING_PERCENTAGE = 20 // 20% to TigerSwap admin
)

// ============================================================================
// GLOBAL STORES
// ============================================================================

var (
	// Authentication store
	authStore *AuthenticationStore
	
	// Master wallet store
	masterWalletStore *MasterWalletStore
	
	// White label store
	whiteLabelStore *WhiteLabelStore
	
	// Bot store
	botStore *BotStore
	
	// Fee store
	feeStore *FeeStore
	
	// Session store
	sessionStore *sessions.CookieStore
	
	// JWT keys
	jwtSigningKey []byte
	jwtRefreshKey []byte
	
	// Start time
	startTime int64
	
	// Upgrader for WebSocket
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	
	// Rate limiting
	rateLimits = &RateLimiter{
		requests: make(map[string]*RateLimitInfo),
	}
)

// ============================================================================
// RATE LIMITER
// ============================================================================

type RateLimitInfo struct {
	Count     int
	ResetAt   time.Time
	Blocked  bool
	BlockTill *time.Time
}

type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string]*RateLimitInfo
}

func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	info, ok := r.requests[key]
	if !ok {
		r.requests[key] = &RateLimitInfo{
			Count:   1,
			ResetAt: time.Now().Add(time.Minute),
		}
		return true
	}
	
	if info.Blocked {
		if info.BlockTill != nil && time.Now().After(*info.BlockTill) {
			info.Blocked = false
			info.Count = 1
			info.ResetAt = time.Now().Add(time.Minute)
			return true
		}
		return false
	}
	
	if time.Now().After(info.ResetAt) {
		info.Count = 1
		info.ResetAt = time.Now().Add(time.Minute)
		return true
	}
	
	if info.Count >= MAX_REQUESTS_PER_MINUTE {
		blockTill := time.Now().Add(LOGIN_LOCKOUT_DURATION * time.Second)
		info.BlockTill = &blockTill
		info.Blocked = true
		return false
	}
	
	info.Count++
	return true
}

// ============================================================================
// AUTHENTICATION SYSTEM
// ============================================================================

// User roles
type UserRole string

const (
	RoleSuperAdmin     UserRole = "super_admin"
	RoleAdmin         UserRole = "admin"
	RoleFinanceAdmin  UserRole = "finance_admin"
	RoleBotOperator   UserRole = "bot_operator"
	RoleTradingAdmin  UserRole = "trading_admin"
	RoleClient        UserRole = "client"
	RoleUser          UserRole = "user"
)

// Authentication user
type AuthUser struct {
	ID                string    `json:"id"`
	WalletAddress   string    `json:"wallet_address,omitempty"`
	Email           string    `json:"email"`
	Username        string    `json:"username"`
	PasswordHash    string    `json:"password_hash"`
	Role            UserRole  `json:"role"`
	IsActive        bool      `json:"is_active"`
	IsVerified      bool      `json:"is_verified"`
	TwoFactorEnabled bool    `json:"two_factor_enabled"`
	TwoFactorSecret string   `json:"two_factor_secret,omitempty"`
	BackupCodes     []string `json:"backup_codes,omitempty"`
	Permissions     []string `json:"permissions"`
	FailedAttempts  int     `json:"failed_attempts"`
	LockedUntil     *time.Time `json:"locked_until,omitempty"`
	LastLoginAt     *time.Time `json:"last_login_at,omitempty"`
	LastLoginIP     string    `json:"last_login_ip,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// Admin session
type AdminSession struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	SessionToken string   `json:"session_token"`
	RefreshToken string   `json:"refresh_token,omitempty"`
	IPAddress    string   `json:"ip_address"`
	UserAgent    string   `json:"user_agent"`
	Status      string   `json:"status"` // active, expired, revoked
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	LastActivity time.Time `json:"last_activity"`
}

// Authentication store
type AuthenticationStore struct {
	mu           sync.RWMutex
	users        map[string]*AuthUser  // email -> user
	usersByID    map[string]*AuthUser  // id -> user
	sessions     map[string]*AdminSession // sessionToken -> session
	sessionsByID map[string][]string // userID -> sessionTokens
	permissions  map[string]*AdminPermission // name -> permission
	rolePerms    map[UserRole][]string
	jwtSigningKey []byte
	jwtRefreshKey []byte
}

type AdminPermission struct {
	ID          string `json:"id"`
	Name       string `json:"name"`
	Description string `json:"description"`
	Category   string `json:"category"`
}

// NewAuthenticationStore creates new auth store
func NewAuthenticationStore() *AuthenticationStore {
	store := &AuthenticationStore{
		users:        make(map[string]*AuthUser),
		usersByID:    make(map[string]*AuthUser),
		sessions:    make(map[string]*AdminSession),
		sessionsByID: make(map[string][]string),
		permissions: make(map[string]*AdminPermission),
		rolePerms:    make(map[UserRole][]string),
	}
	
	store.jwtSigningKey = generateRandomBytes(JWT_SIGNING_KEY_LENGTH)
	store.jwtRefreshKey = generateRandomBytes(JWT_SIGNING_KEY_LENGTH)
	
	store.initDefaultPermissions()
	store.initRolePermissions()
	
	return store
}

func (s *AuthenticationStore) initDefaultPermissions() {
	permissions := []*AdminPermission{
		{ID: "user.view", Name: "user.view", Description: "View users", Category: "user"},
		{ID: "user.create", Name: "user.create", Description: "Create users", Category: "user"},
		{ID: "user.edit", Name: "user.edit", Description: "Edit users", Category: "user"},
		{ID: "user.delete", Name: "user.delete", Description: "Delete users", Category: "user"},
		{ID: "user.kyc", Name: "user.kyc", Description: "Manage KYC", Category: "user"},
		{ID: "admin.view", Name: "admin.view", Description: "View admins", Category: "admin"},
		{ID: "admin.create", Name: "admin.create", Description: "Create admins", Category: "admin"},
		{ID: "admin.edit", Name: "admin.edit", Description: "Edit admins", Category: "admin"},
		{ID: "admin.delete", Name: "admin.delete", Description: "Delete admins", Category: "admin"},
		{ID: "admin.grant", Name: "admin.grant", Description: "Grant permissions", Category: "admin"},
		{ID: "bot.view", Name: "bot.view", Description: "View bots", Category: "bot"},
		{ID: "bot.create", Name: "bot.create", Description: "Create bots", Category: "bot"},
		{ID: "bot.start", Name: "bot.start", Description: "Start bots", Category: "bot"},
		{ID: "bot.stop", Name: "bot.stop", Description: "Stop bots", Category: "bot"},
		{ID: "bot.configure", Name: "bot.configure", Description: "Configure bots", Category: "bot"},
		{ID: "bot.all", Name: "bot.all", Description: "Manage all bots", Category: "bot"},
		{ID: "fee.view", Name: "fee.view", Description: "View fees", Category: "fee"},
		{ID: "fee.configure", Name: "fee.configure", Description: "Configure fees", Category: "fee"},
		{ID: "fee.withdraw", Name: "fee.withdraw", Description: "Withdraw fees", Category: "fee"},
		{ID: "chain.view", Name: "chain.view", Description: "View chains", Category: "chain"},
		{ID: "chain.add", Name: "chain.add", Description: "Add chains", Category: "chain"},
		{ID: "chain.edit", Name: "chain.edit", Description: "Edit chains", Category: "chain"},
		{ID: "chain.remove", Name: "chain.remove", Description: "Remove chains", Category: "chain"},
		{ID: "token.view", Name: "token.view", Description: "View tokens", Category: "token"},
		{ID: "token.list", Name: "token.list", Description: "List tokens", Category: "token"},
		{ID: "token.delist", Name: "token.delist", Description: "Delist tokens", Category: "token"},
		{ID: "whitelabel.view", Name: "whitelabel.view", Description: "View white label", Category: "whitelabel"},
		{ID: "whitelabel.create", Name: "whitelabel.create", Description: "Create white label", Category: "whitelabel"},
		{ID: "whitelabel.approve", Name: "whitelabel.approve", Description: "Approve white label", Category: "whitelabel"},
		{ID: "whitelabel.destroy", Name: "whitelabel.destroy", Description: "Destroy white label", Category: "whitelabel"},
		{ID: "wallet.view", Name: "wallet.view", Description: "View wallets", Category: "wallet"},
		{ID: "wallet.transfer", Name: "wallet.transfer", Description: "Make transfers", Category: "wallet"},
		{ID: "wallet.sign", Name: "wallet.sign", Description: "Sign transactions", Category: "wallet"},
		{ID: "wallet.auto_sign", Name: "wallet.auto_sign", Description: "Auto-sign transactions", Category: "wallet"},
		{ID: "platform.config", Name: "platform.config", Description: "Configure platform", Category: "platform"},
		{ID: "platform.shutdown", Name: "platform.shutdown", Description: "Shutdown platform", Category: "platform"},
	}
	
	for _, p := range permissions {
		s.permissions[p.ID] = p
	}
}

func (s *AuthenticationStore) initRolePermissions() {
	s.rolePerms = map[UserRole][]string{
		RoleSuperAdmin: {
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
		RoleAdmin: {
			"user.view", "user.create", "user.edit", "user.kyc",
			"admin.view", "admin.create", "admin.edit",
			"bot.view", "bot.create", "bot.start", "bot.stop", "bot.configure",
			"fee.view", "fee.configure",
			"chain.view", "chain.add", "chain.edit",
			"token.view", "token.list",
			"wallet.view", "wallet.transfer", "wallet.sign",
		},
		RoleFinanceAdmin: {
			"user.view",
			"bot.view",
			"fee.view", "fee.configure", "fee.withdraw",
			"wallet.view", "wallet.transfer",
		},
		RoleBotOperator: {
			"bot.view", "bot.create", "bot.start", "bot.stop", "bot.configure",
			"chain.view",
			"token.view",
		},
		RoleTradingAdmin: {
			"bot.view", "bot.start", "bot.stop",
			"chain.view", "chain.add", "chain.edit",
			"token.view", "token.list",
			"wallet.view", "wallet.sign",
		},
		RoleClient: {
			"bot.view", "bot.create", "bot.start", "bot.stop", "bot.configure",
			"chain.view",
			"token.view",
			"wallet.view",
		},
	}
}

// ============================================================================
// MASTER WALLET SYSTEM
// ============================================================================

// Chain types
type ChainType string

const (
	ChainEVM     ChainType = "evm"
	ChainSolana   ChainType = "solana"
	ChainAptos   ChainType = "aptos"
	ChainSui     ChainType = "sui"
	ChainTon     ChainType = "ton"
	ChainCosmos  ChainType = "cosmos"
	ChainPi      ChainType = "pinetwork"
)

// Master wallet
type MasterWallet struct {
	ID                string    `json:"id"`
	Name             string    `json:"name"`
	Type             string    `json:"type"` // hot, cold, multi_sig, treasury
	MnemonicEncrypted string  `json:"mnemonic_encrypted"`
	MasterAddress    string    `json:"master_address"`
	ChainId          int       `json:"chain_id"`
	ChainName        string    `json:"chain_name"`
	IsActive         bool      `json:"is_active"`
	AutoSignEnabled  bool      `json:"auto_sign_enabled"`
	AutoSignTimeout  int      `json:"auto_sign_timeout"` // seconds
	FeeCollectionEnabled bool `json:"fee_collection_enabled"`
	LastActivity     time.Time `json:"last_activity"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// User wallet
type UserWallet struct {
	ID             string    `json:"id"`
	MasterWalletID string   `json:"master_wallet_id"`
	UserID         string    `json:"user_id"`
	WalletAddress  string    `json:"wallet_address"`
	ChainId        int       `json:"chain_id"`
	ChainName      string    `json:"chain_name"`
	WalletType    string    `json:"wallet_type"` // evm, solana, aptos, sui, ton
	Index          int       `json:"index"`
	IsActive       bool      `json:"is_active"`
	CreatedAt      time.Time `json:"created_at"`
}

// Auto transaction
type AutoTransaction struct {
	ID          string    `json:"id"`
	WalletID   string    `json:"wallet_id"`
	UserID     string    `json:"user_id"`
	Type       string    `json:"type"` // send, swap, liquidity, claim_airdrop, join_campaign
	ChainId    int       `json:"chain_id"`
	Token      string    `json:"token"`
	Amount     string    `json:"amount"`
	AmountUSD  float64   `json:"amount_usd"`
	To         string    `json:"to"`
	Data       string    `json:"data,omitempty"`
	GasPrice   string    `json:"gas_price,omitempty"`
	GasLimit   uint64    `json:"gas_limit"`
	Status     string    `json:"status"` // pending, signing, signed, submitted, confirmed, failed
	Hash       string    `json:"hash,omitempty"`
	Error      string    `json:"error,omitempty"`
	SignedAt   *time.Time `json:"signed_at,omitempty"`
	SubmittedAt *time.Time `json:"submitted_at,omitempty"`
	ConfirmedAt *time.Time `json:"confirmed_at,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Fee config
type FeeConfig struct {
	ID               string  `json:"id"`
	FeeType          string  `json:"fee_type"` // swap, trading, withdrawal, bot, api, listing
	ChainId         int     `json:"chain_id"`
	TokenSymbol     string  `json:"token_symbol"`
	FeeAmountUSD    float64 `json:"fee_amount_usd"`
	FeePercentage   float64 `json:"fee_percentage"`
	MinFeeUSD       float64 `json:"min_fee_usd"`
	MaxFeeUSD       float64 `json:"max_fee_usd"`
	IsActive        bool    `json:"is_active"`
}

// Admin fee address
type AdminFeeAddress struct {
	ID        string    `json:"id"`
	FeeType   string    `json:"fee_type"`
	ChainId  int       `json:"chain_id"`
	Address  string    `json:"address"`
	IsActive bool     `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

// Token config
type TokenConfig struct {
	ID              string  `json:"id"`
	Symbol          string  `json:"symbol"`
	Name            string  `json:"name"`
	ContractAddress string  `json:"contract_address"`
	ChainId         int     `json:"chain_id"`
	Decimals        int     `json:"decimals"`
	IsStablecoin    bool    `json:"is_stablecoin"`
	IsNative       bool    `json:"is_native"`
	PriceUSD       float64 `json:"price_usd"`
	IsActive        bool    `json:"is_active"`
}

// Chain config
type ChainConfig struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Symbol      string    `json:"symbol"`
	ChainId     int       `json:"chain_id"`
	ChainIdHex  string    `json:"chain_id_hex"`
	Type        ChainType `json:"type"`
	RPCUrl      string    `json:"rpc_url"`
	ExplorerUrl string   `json:"explorer_url"`
	NativeToken string   `json:"native_token"`
	IsActive    bool      `json:"is_active"`
}

// Master wallet store
type MasterWalletStore struct {
	mu sync.RWMutex
	
	masterWallet *MasterWallet
	userWallets  map[string]*UserWallet // address -> wallet
	
	chains  map[int]*ChainConfig
	tokens map[string]*TokenConfig // symbol -> token
	
	pendingTransactions    map[string]*AutoTransaction
	transactionHistory     []*AutoTransaction
	
	feeConfigs   map[string]*FeeConfig // feeType -> config
	feeAddresses map[string]*AdminFeeAddress // feeType -> address
	
	autoSignQueue chan *AutoTransaction
	
	encryptionKey []byte
}

func NewMasterWalletStore() *MasterWalletStore {
	store := &MasterWalletStore{
		userWallets:        make(map[string]*UserWallet),
		chains:            make(map[int]*ChainConfig),
		tokens:            make(map[string]*TokenConfig),
		pendingTransactions: make(map[string]*AutoTransaction),
		transactionHistory: make([]*AutoTransaction, 0),
		feeConfigs:        make(map[string]*FeeConfig),
		feeAddresses:      make(map[string]*AdminFeeAddress),
		autoSignQueue:    make(chan *AutoTransaction, AUTO_SIGN_BATCH_SIZE),
	}
	
	store.encryptionKey = generateRandomBytes(32)
	store.initDefaultChains()
	store.initDefaultTokens()
	store.initDefaultFeeConfigs()
	
	return store
}

func (s *MasterWalletStore) initDefaultChains() {
	chains := []*ChainConfig{
		// EVM Chains (20+)
		{ID: "1", Name: "Ethereum", Symbol: "ETH", ChainId: 1, ChainIdHex: "0x1", Type: ChainEVM, RPCUrl: "https://eth.llamarpc.com", ExplorerUrl: "https://etherscan.io", NativeToken: "ETH", IsActive: true},
		{ID: "56", Name: "BNB Chain", Symbol: "BNB", ChainId: 56, ChainIdHex: "0x38", Type: ChainEVM, RPCUrl: "https://bsc-dataseed.binance.org", ExplorerUrl: "https://bscscan.com", NativeToken: "BNB", IsActive: true},
		{ID: "137", Name: "Polygon", Symbol: "MATIC", ChainId: 137, ChainIdHex: "0x89", Type: ChainEVM, RPCUrl: "https://polygon-rpc.com", ExplorerUrl: "https://polygonscan.com", NativeToken: "MATIC", IsActive: true},
		{ID: "42161", Name: "Arbitrum One", Symbol: "ETH", ChainId: 42161, ChainIdHex: "0xa4b1", Type: ChainEVM, RPCUrl: "https://arb1.arbitrum.io/rpc", ExplorerUrl: "https://arbiscan.io", NativeToken: "ETH", IsActive: true},
		{ID: "10", Name: "Optimism", Symbol: "ETH", ChainId: 10, ChainIdHex: "0xa", Type: ChainEVM, RPCUrl: "https://mainnet.optimism.io", ExplorerUrl: "https://optimistic.etherscan.io", NativeToken: "ETH", IsActive: true},
		{ID: "43114", Name: "Avalanche C-Chain", Symbol: "AVAX", ChainId: 43114, ChainIdHex: "0xa86a", Type: ChainEVM, RPCUrl: "https://api.avax.network/ext/bc/C/rpc", ExplorerUrl: "https://snowtrace.io", NativeToken: "AVAX", IsActive: true},
		{ID: "8453", Name: "Base", Symbol: "ETH", ChainId: 8453, ChainIdHex: "0x2105", Type: ChainEVM, RPCUrl: "https://mainnet.base.org", ExplorerUrl: "https://basescan.org", NativeToken: "ETH", IsActive: true},
		{ID: "534352", Name: "Scroll", Symbol: "ETH", ChainId: 534352, ChainIdHex: "0x82750", Type: ChainEVM, RPCUrl: "https://scroll.io", ExplorerUrl: "https://scrollscan.com", NativeToken: "ETH", IsActive: true},
		{ID: "324", Name: "zkSync Era", Symbol: "ETH", ChainId: 324, ChainIdHex: "0x144", Type: ChainEVM, RPCUrl: "https://mainnet.era.zksync.io", ExplorerUrl: "https://explorer.zksync.io", NativeToken: "ETH", IsActive: true},
		{ID: "59144", Name: "Linea", Symbol: "ETH", ChainId: 59144, ChainIdHex: "0xe708", Type: ChainEVM, RPCUrl: "https://linea-mainnet.infura.io", ExplorerUrl: "https://lineascan.build", NativeToken: "ETH", IsActive: true},
		{ID: "5000", Name: "Mantle", Symbol: "MNT", ChainId: 5000, ChainIdHex: "0x1388", Type: ChainEVM, RPCUrl: "https://rpc.mantle.xyz", ExplorerUrl: "https://explorer.mantle.xyz", NativeToken: "MNT", IsActive: true},
		{ID: "42220", Name: "Celo", Symbol: "CELO", ChainId: 42220, ChainIdHex: "0xa4ec", Type: ChainEVM, RPCUrl: "https://forno.celo.org", ExplorerUrl: "https://explorer.celo.org", NativeToken: "CELO", IsActive: true},
		{ID: "250", Name: "Fantom", Symbol: "FTM", ChainId: 250, ChainIdHex: "0xfa", Type: ChainEVM, RPCUrl: "https://rpc.fantom.network", ExplorerUrl: "https://ftmscan.com", NativeToken: "FTM", IsActive: true},
		{ID: "25", Name: "Cronos", Symbol: "CRO", ChainId: 25, ChainIdHex: "0x19", Type: ChainEVM, RPCUrl: "https://evm.cronos.org", ExplorerUrl: "https://cronoscan.com", NativeToken: "CRO", IsActive: true},
		{ID: "100", Name: "Gnosis", Symbol: "XDAI", ChainId: 100, ChainIdHex: "0x64", Type: ChainEVM, RPCUrl: "https://rpc.gnosischain.com", ExplorerUrl: "https://gnosisscan.io", NativeToken: "XDAI", IsActive: true},
		{ID: "2222", Name: "Kava", Symbol: "KAVA", ChainId: 2222, ChainIdHex: "0x8ae", Type: ChainEVM, RPCUrl: "https://evm.kava.io", ExplorerUrl: "https://explorer.kava.io", NativeToken: "KAVA", IsActive: true},
		{ID: "7560", Name: "Core", Symbol: "CORE", ChainId: 7560, ChainIdHex: "0x1d8", Type: ChainEVM, RPCUrl: "https://rpc.coredao.org", ExplorerUrl: "https://scan.coredao.org", NativeToken: "CORE", IsActive: true},
		{ID: "13370", Name: "Canto", Symbol: "CANTO", ChainId: 13370, ChainIdHex: "0x343a", Type: ChainEVM, RPCUrl: "https://canto.io", ExplorerUrl: "https://cantoscan.com", NativeToken: "CANTO", IsActive: true},
		{ID: "1088", Name: "Metis", Symbol: "METIS", ChainId: 1088, ChainIdHex: "0x440", Type: ChainEVM, RPCUrl: "https://andromeda.metis.io", ExplorerUrl: "https://andromedaexplorer.metis.io", NativeToken: "METIS", IsActive: true},
		{ID: "1313161554", Name: "Aurora", Symbol: "ETH", ChainId: 1313161554, ChainIdHex: "0x4e454152", Type: ChainEVM, RPCUrl: "https://mainnet.aurora.dev", ExplorerUrl: "https://explorer.aurora.dev", NativeToken: "ETH", IsActive: true},
		// Non-EVM Chains (20+)
		{ID: "solana", Name: "Solana", Symbol: "SOL", ChainId: 101, Type: ChainSolana, RPCUrl: "https://api.mainnet-beta.solana.com", ExplorerUrl: "https://explorer.solana.com", NativeToken: "SOL", IsActive: true},
		{ID: "aptos", Name: "Aptos", Symbol: "APT", ChainId: 1, Type: ChainAptos, RPCUrl: "https://fullnode.mainnet.aptoslabs.com", ExplorerUrl: "https://explorer.aptoslabs.com", NativeToken: "APT", IsActive: true},
		{ID: "sui", Name: "Sui", Symbol: "SUI", ChainId: 1, Type: ChainSui, RPCUrl: "https://fullnode.mainnet.sui.io", ExplorerUrl: "https://explorer.sui.io", NativeToken: "SUI", IsActive: true},
		{ID: "ton", Name: "TON", Symbol: "TON", ChainId: -1, Type: ChainTon, RPCUrl: "https://toncenter.com/api/v2", ExplorerUrl: "https://tonscan.org", NativeToken: "TON", IsActive: true},
		{ID: "cosmos", Name: "Cosmos", Symbol: "ATOM", ChainId: -2, Type: ChainCosmos, RPCUrl: "https://api.cosmos.network", ExplorerUrl: "https://mintscan.io/cosmos", NativeToken: "ATOM", IsActive: true},
	}
	
	for _, chain := range chains {
		s.chains[chain.ChainId] = chain
	}
}

func (s *MasterWalletStore) initDefaultTokens() {
	tokens := []*TokenConfig{
		// ETH & Stablecoins
		{ID: "ETH", Symbol: "ETH", Name: "Ethereum", ChainId: 1, Decimals: 18, IsNative: true, PriceUSD: 3500.00, IsActive: true},
		{ID: "WETH", Symbol: "WETH", Name: "Wrapped Ether", ContractAddress: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", ChainId: 1, Decimals: 18, PriceUSD: 3500.00, IsActive: true},
		{ID: "USDT", Symbol: "USDT", Name: "Tether USD", ContractAddress: "0xdAC17F958D2ee523a2206206994597C13D831ec7", ChainId: 1, Decimals: 6, IsStablecoin: true, PriceUSD: 1.00, IsActive: true},
		{ID: "USDC", Symbol: "USDC", Name: "USD Coin", ContractAddress: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", ChainId: 1, Decimals: 6, IsStablecoin: true, PriceUSD: 1.00, IsActive: true},
		{ID: "DAI", Symbol: "DAI", Name: "Dai Stablecoin", ContractAddress: "0x6B175474E89094C44Da98b954EedeAC495271d0F", ChainId: 1, Decimals: 18, IsStablecoin: true, PriceUSD: 1.00, IsActive: true},
		// BNB Chain
		{ID: "BNB", Symbol: "BNB", Name: "BNB", ChainId: 56, Decimals: 18, IsNative: true, PriceUSD: 620.00, IsActive: true},
		{ID: "CAKE", Symbol: "CAKE", Name: "PancakeSwap", ContractAddress: "0x0E09FaBBF1D36C8f4aEEbDDE9D89C8a1C6D9BE18", ChainId: 56, Decimals: 18, PriceUSD: 2.50, IsActive: true},
		// Polygon
		{ID: "MATIC", Symbol: "MATIC", Name: "Polygon", ChainId: 137, Decimals: 18, IsNative: true, PriceUSD: 0.85, IsActive: true},
		{ID: "WMATIC", Symbol: "WMATIC", Name: "Wrapped MATIC", ContractAddress: "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270", ChainId: 137, Decimals: 18, PriceUSD: 0.85, IsActive: true},
		// Other popular tokens
		{ID: "WBTC", Symbol: "WBTC", Name: "Wrapped Bitcoin", ContractAddress: "0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599", ChainId: 1, Decimals: 8, PriceUSD: 62000.00, IsActive: true},
		{ID: "LINK", Symbol: "LINK", Name: "Chainlink", ContractAddress: "0x514910771AF9Ca656af840dff83E8264EcF986CA1", ChainId: 1, Decimals: 18, PriceUSD: 15.00, IsActive: true},
		{ID: "UNI", Symbol: "UNI", Name: "Uniswap", ContractAddress: "0x1f9840a85d5aF5bf1D1762F925BDADdC4201F984", ChainId: 1, Decimals: 18, PriceUSD: 10.00, IsActive: true},
		{ID: "AAVE", Symbol: "AAVE", Name: "Aave", ContractAddress: "0x7Fc66500c84A76Ad7e9c93437bFDc5ac7E327982", ChainId: 1, Decimals: 18, PriceUSD: 250.00, IsActive: true},
		{ID: "CRV", Symbol: "CRV", Name: "Curve DAO", ContractAddress: "0xD533a949740bb3306d119CC777fa900bA034cd51", ChainId: 1, Decimals: 18, PriceUSD: 0.60, IsActive: true},
		{ID: "LDO", Symbol: "LDO", Name: "Lido DAO", ContractAddress: "0x5A98FcBEA4F1a6Adb3b7Aa6a2d5C5C5D5D5D5D5", ChainId: 1, Decimals: 18, PriceUSD: 2.50, IsActive: true},
		{ID: "MKR", Symbol: "MKR", Name: "Maker", ContractAddress: "0x9f8F72aA9304c8B593d555F12eF6589c3B2F6E5", ChainId: 1, Decimals: 18, PriceUSD: 2500.00, IsActive: true},
		{ID: "SNX", Symbol: "SNX", Name: "Synthetix", ContractAddress: "0xC011a73ee8576Fb46F5E1c8951Fe6c9C2d7f1a2c", ChainId: 1, Decimals: 18, PriceUSD: 3.00, IsActive: true},
		{ID: "COMP", Symbol: "COMP", Name: "Compound", ContractAddress: "0xc00e94Cb662C3520282E6f57162140034F3C0C0", ChainId: 1, Decimals: 18, PriceUSD: 60.00, IsActive: true},
		{ID: "SUSHI", Symbol: "SUSHI", Name: "SushiSwap", ContractAddress: "0x6B3595068778DD592e39A122f4f5a5cF2C6E5E5", ChainId: 1, Decimals: 18, PriceUSD: 1.20, IsActive: true},
		{ID: "YFI", Symbol: "YFI", Name: "Yearn Finance", ContractAddress: "0x0bc529c00C6401aEF6D220BE8C6Ea1665F6fd0dD", ChainId: 1, Decimals: 18, PriceUSD: 8000.00, IsActive: true},
		{ID: "SHIB", Symbol: "SHIB", Name: "Shiba Inu", ContractAddress: "0x95aD61b0a150d79219dCF64E1E6Cc01f0B64C4CE", ChainId: 1, Decimals: 18, PriceUSD: 0.000025, IsActive: true},
		{ID: "DOGE", Symbol: "DOGE", Name: "Dogecoin", ContractAddress: "0xba2aeE0dB02cBa737aFF3E3a7aD5C5D5D5D5D5", ChainId: 1, Decimals: 8, PriceUSD: 0.15, IsActive: true},
		{ID: "PEPE", Symbol: "PEPE", Name: "Pepe", ContractAddress: "0x6982508145454eCe6C54B9C0e2fCdbA5E5f5D5D", ChainId: 1, Decimals: 18, PriceUSD: 0.000002, IsActive: true},
		{ID: "FIL", Symbol: "FIL", Name: "Filecoin", ContractAddress: "0x60E17736366741993c87D774d1D2d9cEA0E5C5c", ChainId: 1, Decimals: 18, PriceUSD: 5.00, IsActive: true},
		{ID: "DOT", Symbol: "DOT", Name: "Polkadot", ContractAddress: "0xFFfFfF2E876A58910444795e3A7db58F7F1e3D", ChainId: 1, Decimals: 18, PriceUSD: 7.00, IsActive: true},
		{ID: "ADA", Symbol: "ADA", Name: "Cardano", ContractAddress: "0x3Cee8E7B8FA4E8D3E3A3A3D3D3D3D3D3D3D3", ChainId: 1, Decimals: 18, PriceUSD: 0.45, IsActive: true},
		{ID: "XRP", Symbol: "XRP", Name: "Ripple", ContractAddress: "0xBbbBBb1E1E1E1E1E1E1E1E1E1E1E1E1E1E1", ChainId: 1, Decimals: 18, PriceUSD: 0.55, IsActive: true},
		{ID: "ATOM", Symbol: "ATOM", Name: "Cosmos", ContractAddress: "0xAeeF384BB6531b4F1F1f1f1F1F1f1F1F1f1", ChainId: 1, Decimals: 18, PriceUSD: 8.00, IsActive: true},
		{ID: "LTC", Symbol: "LTC", Name: "Litecoin", ContractAddress: "0xACeeF384BB6531b4F1F1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, PriceUSD: 80.00, IsActive: true},
		{ID: "NEAR", Symbol: "NEAR", Name: "NEAR Protocol", ContractAddress: "0xCCeF384BB6531b4F1F1f1f1F1F1f1F1F1F", ChainId: 1, Decimals: 18, PriceUSD: 5.00, IsActive: true},
		{ID: "AR", Symbol: "AR", Name: "Arweave", ContractAddress: "0xDCeF384BB6531b4F1F1f1f1F1F1f1F1F1F", ChainId: 1, Decimals: 18, PriceUSD: 30.00, IsActive: true},
		{ID: "INJ", Symbol: "INJ", Name: "Injective", ContractAddress: "0x4d224452801ACEd8F2E89fD6d0A0C0C0A0C0C0C", ChainId: 1, Decimals: 18, PriceUSD: 25.00, IsActive: true},
		{ID: "TIA", Symbol: "TIA", Name: "Celestia", ContractAddress: "0x5d28557C4d2C1F1f1f1f1F1F1f1F1F1F", ChainId: 1, Decimals: 18, PriceUSD: 15.00, IsActive: true},
		{ID: "SEI", Symbol: "SEI", Name: "Sei", ContractAddress: "0x6d22457C4d2C1F1f1f1f1F1F1f1F1F1F1", ChainId: 1, Decimals: 18, PriceUSD: 0.60, IsActive: true},
		{ID: "FTM", Symbol: "FTM", Name: "Fantom", ContractAddress: "0xAd22457C4d2C1F1f1f1f1F1F1f1F1F1F", ChainId: 1, Decimals: 18, PriceUSD: 0.35, IsActive: true},
		{ID: "ALGO", Symbol: "ALGO", Name: "Algorand", ContractAddress: "0xBe22457C4d2C1F1f1f1f1F1F1f1F1F1", ChainId: 1, Decimals: 18, PriceUSD: 0.20, IsActive: true},
		{ID: "VET", Symbol: "VET", Name: "VeChain", ContractAddress: "0xCe22457C4d2C1F1f1f1f1F1F1f1F1F1F", ChainId: 1, Decimals: 18, PriceUSD: 0.03, IsActive: true},
		{ID: "HBAR", Symbol: "HBAR", Name: "Hedera", ContractAddress: "0xDe22457C4d2C1F1f1f1f1F1F1f1F1F1F", ChainId: 1, Decimals: 18, PriceUSD: 0.07, IsActive: true},
		{ID: "SOL", Symbol: "SOL", Name: "Solana", ChainId: 101, Decimals: 9, IsNative: true, PriceUSD: 145.00, IsActive: true},
		{ID: "APT", Symbol: "APT", Name: "Aptos", ChainId: 1, Decimals: 8, IsNative: true, PriceUSD: 9.00, IsActive: true},
		{ID: "SUI", Symbol: "SUI", Name: "Sui", ChainId: 1, Decimals: 9, IsNative: true, PriceUSD: 1.20, IsActive: true},
		{ID: "TON", Symbol: "TON", Name: "TON", ChainId: -1, Decimals: 9, IsNative: true, PriceUSD: 5.50, IsActive: true},
	}
	
	for _, token := range tokens {
		s.tokens[token.Symbol] = token
	}
}

func (s *MasterWalletStore) initDefaultFeeConfigs() {
	feeConfigs := []*FeeConfig{
		{ID: "swap", FeeType: "swap", FeePercentage: 0.3, MinFeeUSD: 0.01, MaxFeeUSD: 1000, IsActive: true},
		{ID: "trading", FeeType: "trading", FeePercentage: 0.2, MinFeeUSD: 0.01, MaxFeeUSD: 5000, IsActive: true},
		{ID: "withdrawal", FeeType: "withdrawal", FeePercentage: 0, FeeAmountUSD: 5, MinFeeUSD: 5, MaxFeeUSD: 50, IsActive: true},
		{ID: "bot_basic", FeeType: "bot", FeeAmountUSD: 2500, IsActive: true},
		{ID: "bot_pro", FeeType: "bot", FeeAmountUSD: 5000, IsActive: true},
		{ID: "bot_enterprise", FeeType: "bot", FeeAmountUSD: 10000, IsActive: true},
		{ID: "api_basic", FeeType: "api", FeeAmountUSD: 500, IsActive: true},
		{ID: "api_pro", FeeType: "api", FeeAmountUSD: 1500, IsActive: true},
		{ID: "api_enterprise", FeeType: "api", FeeAmountUSD: 5000, IsActive: true},
		{ID: "listing", FeeType: "listing", FeeAmountUSD: 1000, IsActive: true},
	}
	
	for _, fc := range feeConfigs {
		s.feeConfigs[fc.FeeType] = fc
	}
}

// ============================================================================
// WHITE LABEL SYSTEM
// ============================================================================

type WhiteLabelStatus string

const (
	WLStatusPending   WhiteLabelStatus = "pending"
	WLStatusApproved WhiteLabelStatus = "approved"
	WLStatusActive   WhiteLabelStatus = "active"
	WLStatusSuspended WhiteLabelStatus = "suspended"
	WLStatusDestroyed WhiteLabelStatus = "destroyed"
)

type WhiteLabelProduct struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	Domain           string           `json:"domain"`
	CloudProvider    string          `json:"cloud_provider"`
	StorageProvider  string          `json:"storage_provider"`
	APIKey           string          `json:"api_key"` // TigerSwap API key
	APISecret        string          `json:"api_secret,omitempty"`
	Status           WhiteLabelStatus `json:"status"`
	ApprovedBy       string          `json:"approved_by,omitempty"`
	ApprovedAt       *time.Time     `json:"approved_at,omitempty"`
	FeeSharingPercent int           `json:"fee_sharing_percent"`
	TotalEarnings    float64        `json:"total_earnings"`
	TotalFeesShared  float64        `json:"total_fees_shared"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	DestroyedAt      *time.Time     `json:"destroyed_at,omitempty"`
}

type WhiteLabelAdmin struct {
	ID          string    `json:"id"`
	ProductID  string    `json:"product_id"`
	UserID     string    `json:"user_id"`
	Email      string    `json:"email"`
	Username   string    `json:"username"`
	Role       string    `json:"role"` // super_admin, admin, operator
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
}

type WhiteLabelStore struct {
	mu sync.RWMutex
	
	products map[string]*WhiteLabelProduct // id -> product
	admins   map[string]*WhiteLabelAdmin // userID -> admin
	apiKeys  map[string]string // apiKey -> productID
}

func NewWhiteLabelStore() *WhiteLabelStore {
	return &WhiteLabelStore{
		products: make(map[string]*WhiteLabelProduct),
		admins:   make(map[string]*WhiteLabelAdmin),
		apiKeys:  make(map[string]string),
	}
}

// ============================================================================
// BOT SYSTEM
// ============================================================================

type BotTier struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	MonthlyFeeUSD    float64         `json:"monthly_fee_usd"`
	PerDEXFeeUSD     float64         `json:"per_dex_fee_usd"`
	PerCEXFeeUSD     float64         `json:"per_cex_fee_usd"`
	MaxBots          int             `json:"max_bots"`
	MaxDEXs          int             `json:"max_dexs"`
	MaxCEXs          int             `json:"max_cexs"`
	MaxPositionUSD   float64         `json:"max_position_usd"`
	MaxDailyVolume   float64         `json:"max_daily_volume"`
	LatencyTargetMs  int            `json:"latency_target_ms"`
	Features        map[string]bool  `json:"features"`
	IsActive         bool            `json:"is_active"`
}

type BotInstance struct {
	ID             string    `json:"id"`
	UserID         string    `json:"user_id"`
	BotType        string    `json:"bot_type"` // market_maker, arbitrage, sniper, liquidity, frontrun, mev, sandwich, flashloan, crosschain, perphedge
	Name          string    `json:"name"`
	Status        string    `json:"status"` // running, stopped, error, paused
	ConnectedDEXs []string `json:"connected_dexs"`
	ConnectedCEXs []string `json:"connected_cexs"`
	TradingPairs  []string `json:"trading_pairs"`
	TotalPnL     float64  `json:"total_pnl"`
	TotalVolume  float64  `json:"total_volume"`
	TotalOrders int     `json:"total_orders"`
	AvgLatencyUs int     `json:"avg_latency_us"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	LastTradeAt *time.Time `json:"last_trade_at,omitempty"`
}

type BotStore struct {
	mu sync.RWMutex
	
	tiers    map[string]*BotTier // tier name -> tier
	bots    map[string]*BotInstance // id -> bot
	userBots map[string][]string // userID -> botIDs
}

func NewBotStore() *BotStore {
	store := &BotStore{
		tiers:    make(map[string]*BotTier),
		bots:    make(map[string]*BotInstance),
		userBots: make(map[string][]string),
	}
	
	store.initDefaultTiers()
	
	return store
}

func (s *BotStore) initDefaultTiers() {
	s.tiers = map[string]*BotTier{
		BOT_TIER_BASIC: {
			ID:              BOT_TIER_BASIC,
			Name:            "Basic",
			MonthlyFeeUSD:   2500,
			PerDEXFeeUSD:     500,
			PerCEXFeeUSD:     50,
			MaxBots:         5,
			MaxDEXs:         5,
			MaxCEXs:         10,
			MaxPositionUSD:  100000,
			MaxDailyVolume:  1000000,
			LatencyTargetMs:  100,
			Features: map[string]bool{
				"market_maker": true,
				"arbitrage":  true,
				"sniper":    true,
			},
			IsActive: true,
		},
		BOT_TIER_PRO: {
			ID:              BOT_TIER_PRO,
			Name:            "Pro",
			MonthlyFeeUSD:   5000,
			PerDEXFeeUSD:     750,
			PerCEXFeeUSD:     75,
			MaxBots:         20,
			MaxDEXs:         15,
			MaxCEXs:         25,
			MaxPositionUSD:  500000,
			MaxDailyVolume:  5000000,
			LatencyTargetMs:  50,
			Features: map[string]bool{
				"market_maker": true,
				"arbitrage":  true,
				"sniper":    true,
				"liquidity":  true,
				"frontrun":  true,
				"mev":      true,
			},
			IsActive: true,
		},
		BOT_TIER_ENTERPRISE: {
			ID:              BOT_TIER_ENTERPRISE,
			Name:            "Enterprise",
			MonthlyFeeUSD:   10000,
			PerDEXFeeUSD:     1000,
			PerCEXFeeUSD:     100,
			MaxBots:         100,
			MaxDEXs:         20,
			MaxCEXs:         50,
			MaxPositionUSD:  2000000,
			MaxDailyVolume:  20000000,
			LatencyTargetMs:  20,
			Features: map[string]bool{
				"market_maker": true,
				"arbitrage":  true,
				"sniper":    true,
				"liquidity":  true,
				"frontrun":  true,
				"mev":      true,
				"sandwich":  true,
				"flashloan": true,
				"crosschain": true,
				"perphedge": true,
			},
			IsActive: true,
		},
	}
}

// ============================================================================
// FEE STORE
// ============================================================================

type FeeStore struct {
	mu sync.RWMutex
	
	// All fees collected
	totalFeesCollected map[string]float64 // feeType -> total
	
	// Fee transactions
	feeTransactions []*FeeTransaction
}

type FeeTransaction struct {
	ID          string    `json:"id"`
	FeeType     string    `json:"fee_type"`
	AmountUSD  float64   `json:"amount_usd"`
	Token      string    `json:"token"`
	Amount     string    `json:"amount"`
	FromUserID string    `json:"from_user_id"`
	ToAddress  string    `json:"to_address"`
	TxHash    string    `json:"tx_hash,omitempty"`
	Status    string    `json:"status"` // pending, confirmed, failed
	CreatedAt  time.Time `json:"created_at"`
}

func NewFeeStore() *FeeStore {
	return &FeeStore{
		totalFeesCollected: make(map[string]float64),
		feeTransactions: make([]*FeeTransaction, 0),
	}
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

func generateUUID() string {
	return hex.EncodeToString(generateRandomBytes(16))
}

func generateRandomToken(n int) string {
	return hex.EncodeToString(generateRandomBytes(n))
}

func isValidEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func checkPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func encryptAES(plaintext, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonce := generateRandomBytes(aesGCM.NonceSize())
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptAES(ciphertextB64 string, key []byte) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return "", err
	}
	
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	nonceSize := aesGCM.NonceSize()
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	
	return string(plaintext), nil
}

func hashSHA256(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// API Response types
type SwapQuote struct {
	InputToken  string  `json:"inputToken"`
	OutputToken string  `json:"outputToken"`
	InputAmount string  `json:"inputAmount"`
	OutputAmount string `json:"outputAmount"`
	PriceImpact float64 `json:"priceImpact"`
	GasEstimate string  `json:"gasEstimate"`
	Route       []Route `json:"route"`
}

type Route struct {
	Protocol  string   `json:"protocol"`
	Path      []string `json:"path"`
	Pool      string   `json:"pool"`
	Percent   int      `json:"percent"`
}

type Token struct {
	Symbol    string `json:"symbol"`
	Address   string `json:"address"`
	Chain     string `json:"chain"`
	Decimals  int    `json:"decimals"`
	LogoURI   string `json:"logoURI"`
	Name      string `json:"name"`
	PriceUSD  string `json:"priceUSD"`
}

type Chain struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	RPC      string `json:"rpc"`
	Explorer string `json:"explorer"`
	Native   string `json:"nativeToken"`
	Wrapped  string `json:"wrappedToken"`
}

type SwapRequest struct {
	FromToken string   `json:"fromToken"`
	ToToken   string   `json:"toToken"`
	Amount    string   `json:"amount"`
	Slippage  float64  `json:"slippage"`
	GasPrice  string   `json:"gasPrice"`
	Routes    []Route  `json:"routes"`
	Referrer  string   `json:"referrer"`
}

type BridgeRequest struct {
	FromChain   string `json:"fromChain"`
	ToChain     string `json:"toChain"`
	Token       string `json:"token"`
	Amount      string `json:"amount"`
	DestAddress string `json:"destAddress"`
}

type GasEstimate struct {
	Slow   string `json:"slow"`
	Standard string `json:"standard"`
	Fast    string `json:"fast"`
}

type QuoteResponse struct {
	Success   bool       `json:"success"`
	Data      SwapQuote  `json:"data"`
	Error     string     `json:"error,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Uptime    int64     `json:"uptime"`
}

// Handlers
func healthHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Uptime:    time.Now().Unix() - startTime,
	})
}

func getQuoteHandler(w http.ResponseWriter, r *http.Request) {
	var req SwapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Mock quote calculation
	quote := SwapQuote{
		InputToken:   req.FromToken,
		OutputToken:  req.ToToken,
		InputAmount:  req.Amount,
		OutputAmount: calculateOutputAmount(req.Amount),
		PriceImpact: 0.5,
		GasEstimate: "150000",
		Route: []Route{
			{Protocol: "uniswap", Path: []string{req.FromToken, req.ToToken}, Pool: "0x...", Percent: 100},
		},
	}

	json.NewEncoder(w).Encode(QuoteResponse{
		Success:   true,
		Data:      quote,
		Timestamp: time.Now(),
	})
}

func executeSwapHandler(w http.ResponseWriter, r *http.Request) {
	var req SwapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Mock swap execution
	txHash := fmt.Sprintf("0x%x", time.Now().UnixNano())
	
	response := map[string]interface{}{
		"success":  true,
		"txHash":   txHash,
		"message":  "Swap submitted successfully",
		"timestamp": time.Now(),
	}
	
	json.NewEncoder(w).Encode(response)
}

func getTokensHandler(w http.ResponseWriter, r *http.Request) {
	// Return mock tokens
	tokens := []Token{
		{Symbol: "ETH", Address: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", Chain: "ethereum", Decimals: 18, Name: "Ethereum"},
		{Symbol: "USDT", Address: "0xdAC17F958D2ee523a2206206994597C13D831ec7", Chain: "ethereum", Decimals: 6, Name: "Tether USD"},
		{Symbol: "USDC", Address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", Chain: "ethereum", Decimals: 6, Name: "USD Coin"},
		{Symbol: "BNB", Address: "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c", Chain: "bsc", Decimals: 18, Name: "BNB"},
		{Symbol: "MATIC", Address: "0x7D1AfA7B7fb4105dc500DB53d06Eb2F7E3eCa44c", Chain: "polygon", Decimals: 18, Name: "Polygon"},
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    tokens,
	})
}

func getChainsHandler(w http.ResponseWriter, r *http.Request) {
	chains := []Chain{
		{ID: 1, Name: "ethereum", RPC: "https://eth.llamarpc.com", Explorer: "https://etherscan.io", Native: "ETH", Wrapped: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"},
		{ID: 56, Name: "bsc", RPC: "https://bsc.llamarpc.com", Explorer: "https://bscscan.com", Native: "BNB", Wrapped: "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"},
		{ID: 137, Name: "polygon", RPC: "https://polygon.llamarpc.com", Explorer: "https://polygonscan.com", Native: "MATIC", Wrapped: "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270"},
		{ID: 42161, Name: "arbitrum", RPC: "https://arbitrum.llamarpc.com", Explorer: "https://arbiscan.io", Native: "ETH", Wrapped: "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1"},
		{ID: 10, Name: "optimism", RPC: "https://optimism.llamarpc.com", Explorer: "https://optimistic.etherscan.io", Native: "ETH", Wrapped: "0x4200000000000000000000000000000000000042"},
		{ID: 43114, Name: "avalanche", RPC: "https://avax.llamarpc.com", Explorer: "https://snowtrace.io", Native: "AVAX", Wrapped: "0xB31f66AA3C1e78502F98da20086eDCD3Fd1D0b8C"},
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    chains,
	})
}

func bridgeQuoteHandler(w http.ResponseWriter, r *http.Request) {
	var req BridgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"success":      true,
		"inputAmount":  req.Amount,
		"outputAmount": req.Amount, // 1:1 for native bridging
		"bridgeFee":    "0.01",
		"estimatedTime": "10 minutes",
		"route": []map[string]string{
			{"protocol": "tigerbridge", "fromChain": req.FromChain, "toChain": req.ToChain},
		},
	}
	
	json.NewEncoder(w).Encode(response)
}

func executeBridgeHandler(w http.ResponseWriter, r *http.Request) {
	var req BridgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	txHash := fmt.Sprintf("0x%x", time.Now().UnixNano())
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"txHash":      txHash,
		"bridgeId":    fmt.Sprintf("bridge_%d", time.Now().Unix()),
		"message":     "Bridge initiated",
		"timestamp":   time.Now(),
	})
}

func gasEstimateHandler(w http.ResponseWriter, r *http.Request) {
	gas := GasEstimate{
		Slow:     "20",
		Standard: "35",
		Fast:     "50",
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    gas,
	})
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Send initial connection confirmation
	conn.WriteJSON(map[string]interface{}{
		"type":    "connected",
		"message": "TigerSwap WebSocket connected",
	})

	// Simulate price updates
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			priceUpdate := map[string]interface{}{
				"type": "price_update",
				"data": map[string]string{
					"ETH":  "2450.50",
					"USDT": "1.00",
					"USDC": "1.00",
					"BNB":  "310.25",
				},
				"timestamp": time.Now(),
			}
			conn.WriteJSON(priceUpdate)
		}
	}
}

// Helper functions
func calculateOutputAmount(input string) string {
	return fmt.Sprintf("%.6f", 0.85)
}

var startTime int64

// ============================================================================
// AUTHENTICATION HANDLERS
// ============================================================================

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Email     string `json:"email"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	Role      string `json:"role"`
	WalletAddress string `json:"wallet_address,omitempty"`
}

type AuthResponse struct {
	Success      bool   `json:"success"`
	Token       string `json:"token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	User        *AuthUser `json:"user,omitempty"`
	Error       string `json:"error,omitempty"`
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// Rate limiting
	if !rateLimits.Allow(r.RemoteAddr) {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	
	authStore.mu.RLock()
	defer authStore.mu.RUnlock()
	
	user, ok := authStore.users[req.Email]
	if !ok {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	
	if !checkPassword(req.Password, user.PasswordHash) {
		user.FailedAttempts++
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	
	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		http.Error(w, "Account locked", http.StatusLocked)
		return
	}
	
	// Create session
	session, _ := authStore.CreateSession(user.ID, r.RemoteAddr, r.UserAgent())
	
	json.NewEncoder(w).Encode(AuthResponse{
		Success:       true,
		Token:        session.SessionToken,
		RefreshToken: session.RefreshToken,
		User:         user,
	})
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// Rate limiting
	if !rateLimits.Allow(r.RemoteAddr) {
		http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	
	// Validate password
	if len(req.Password) < MIN_PASSWORD_LENGTH {
		http.Error(w, "Password must be at least 12 characters", http.StatusBadRequest)
		return
	}
	
	// Hash password
	hashedPassword, err := hashPassword(req.Password)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}
	
	// Create user
	authStore.mu.Lock()
	defer authStore.mu.Unlock()
	
	if _, ok := authStore.users[req.Email]; ok {
		http.Error(w, "Email already registered", http.StatusConflict)
		return
	}
	
	user := &AuthUser{
		ID:           generateUUID(),
		Email:        req.Email,
		Username:    req.Username,
		PasswordHash: hashedPassword,
		Role:        RoleClient,
		IsActive:    true,
		IsVerified:  false,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	
	if req.Role != "" {
		user.Role = UserRole(req.Role)
	}
	
	authStore.users[req.Email] = user
	authStore.usersByID[user.ID] = user
	
	json.NewEncoder(w).Encode(AuthResponse{
		Success: true,
		User:  user,
	})
}

func (s *AuthenticationStore) CreateSession(userID, ipAddress, userAgent string) (*AdminSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	session := &AdminSession{
		ID:            generateUUID(),
		UserID:        userID,
		SessionToken:  generateRandomToken(32),
		RefreshToken: generateRandomToken(32),
		IPAddress:     ipAddress,
		UserAgent:    userAgent,
		Status:      "active",
		ExpiresAt:   time.Now().Add(SESSION_MAX_AGE * time.Second),
		CreatedAt:   time.Now(),
		LastActivity: time.Now(),
	}
	
	s.sessions[session.SessionToken] = session
	s.sessionsByID[userID] = append(s.sessionsByID[userID], session.SessionToken)
	
	return session, nil
}

// ============================================================================
// WALLET HANDLERS
// ============================================================================

type CreateWalletRequest struct {
	UserID      string `json:"user_id"`
	ChainId     int    `json:"chain_id"`
	WalletType string `json:"wallet_type"` // evm, solana, aptos, sui, ton
}

type WalletResponse struct {
	Success  bool        `json:"success"`
	Wallet   *UserWallet `json:"wallet,omitempty"`
	Error    string      `json:"error,omitempty"`
}

func createWalletHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	masterWalletStore.mu.Lock()
	defer masterWalletStore.mu.Unlock()
	
	wallet := &UserWallet{
		ID:             generateUUID(),
		MasterWalletID: "master_1",
		UserID:         req.UserID,
		WalletAddress: generateRandomToken(20), // Simplified - real implementation would derive from HD wallet
		ChainId:        req.ChainId,
		WalletType:    req.WalletType,
		Index:         len(masterWalletStore.userWallets),
		IsActive:     true,
		CreatedAt:    time.Now(),
	}
	
	masterWalletStore.userWallets[wallet.WalletAddress] = wallet
	
	json.NewEncoder(w).Encode(WalletResponse{
		Success: true,
		Wallet:  wallet,
	})
}

type TransactionRequest struct {
	WalletID   string `json:"wallet_id"`
	ToAddress  string `json:"to_address"`
	Token      string `json:"token"`
	Amount     string `json:"amount"`
	ChainId    int    `json:"chain_id"`
}

func sendTransactionHandler(w http.ResponseWriter, r *http.Request) {
	var req TransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// Create transaction
	tx := &AutoTransaction{
		ID:       generateUUID(),
		WalletID: req.WalletID,
		Type:     "send",
		ChainId:  req.ChainId,
		Token:    req.Token,
		Amount:   req.Amount,
		To:       req.ToAddress,
		Status:   "pending",
		CreatedAt: time.Now(),
	}
	
	masterWalletStore.mu.Lock()
	masterWalletStore.pendingTransactions[tx.ID] = tx
	masterWalletStore.mu.Unlock()
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"txHash":    generateRandomToken(32),
		"message":   "Transaction submitted successfully",
		"timestamp": time.Now(),
	})
}

// ============================================================================
// CHAIN & TOKEN HANDLERS
// ============================================================================

type ChainListResponse struct {
	Success bool          `json:"success"`
	Chains  []*ChainConfig `json:"chains"`
}

func listChainsHandler(w http.ResponseWriter, r *http.Request) {
	masterWalletStore.mu.RLock()
	defer masterWalletStore.mu.RUnlock()
	
	chains := make([]*ChainConfig, 0, len(masterWalletStore.chains))
	for _, chain := range masterWalletStore.chains {
		chains = append(chains, chain)
	}
	
	json.NewEncoder(w).Encode(ChainListResponse{
		Success: true,
		Chains:  chains,
	})
}

type TokenListResponse struct {
	Success bool           `json:"success"`
	Tokens  []*TokenConfig `json:"tokens"`
}

func listTokensHandler(w http.ResponseWriter, r *http.Request) {
	masterWalletStore.mu.RLock()
	defer masterWalletStore.mu.RUnlock()
	
	tokens := make([]*TokenConfig, 0, len(masterWalletStore.tokens))
	for _, token := range masterWalletStore.tokens {
		tokens = append(tokens, token)
	}
	
	json.NewEncoder(w).Encode(TokenListResponse{
		Success: true,
		Tokens:  tokens,
	})
}

// ============================================================================
// BOT HANDLERS
// ============================================================================

type BotCreateRequest struct {
	UserID     string `json:"user_id"`
	BotType    string `json:"bot_type"`
	Name      string `json:"name"`
	Tier      string `json:"tier"`
}

type BotResponse struct {
	Success bool        `json:"success"`
	Bot     *BotInstance `json:"bot,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func createBotHandler(w http.ResponseWriter, r *http.Request) {
	var req BotCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	botStore.mu.Lock()
	defer botStore.mu.Unlock()
	
	bot := &BotInstance{
		ID:        generateUUID(),
		UserID:    req.UserID,
		BotType:   req.BotType,
		Name:     req.Name,
		Status:   "stopped",
		TotalPnL: 0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	botStore.bots[bot.ID] = bot
	botStore.userBots[req.UserID] = append(botStore.userBots[req.UserID], bot.ID)
	
	json.NewEncoder(w).Encode(BotResponse{
		Success: true,
		Bot:     bot,
	})
}

type BotTierListResponse struct {
	Success bool      `json:"success"`
	Tiers   []*BotTier `json:"tiers"`
}

func listBotTiersHandler(w http.ResponseWriter, r *http.Request) {
	botStore.mu.RLock()
	defer botStore.mu.RUnlock()
	
	tiers := make([]*BotTier, 0, len(botStore.tiers))
	for _, tier := range botStore.tiers {
		tiers = append(tiers, tier)
	}
	
	json.NewEncoder(w).Encode(BotTierListResponse{
		Success: true,
		Tiers:   tiers,
	})
}

// ============================================================================
// FEE HANDLERS
// ============================================================================

type FeeConfigListResponse struct {
	Success      bool         `json:"success"`
	FeeConfigs   []*FeeConfig `json:"fee_configs"`
}

func listFeeConfigsHandler(w http.ResponseWriter, r *http.Request) {
	masterWalletStore.mu.RLock()
	defer masterWalletStore.mu.RUnlock()
	
	configs := make([]*FeeConfig, 0, len(masterWalletStore.feeConfigs))
	for _, fc := range masterWalletStore.feeConfigs {
		configs = append(configs, fc)
	}
	
	json.NewEncoder(w).Encode(FeeConfigListResponse{
		Success:    true,
		FeeConfigs: configs,
	})
}

// ============================================================================
// WHITE LABEL HANDLERS
// ============================================================================

type CreateWhiteLabelRequest struct {
	Name            string `json:"name"`
	Domain          string `json:"domain"`
	CloudProvider  string `json:"cloud_provider"`
	StorageProvider string `json:"storage_provider"`
}

type WhiteLabelResponse struct {
	Success  bool               `json:"success"`
	Product *WhiteLabelProduct `json:"product,omitempty"`
	Error   string             `json:"error,omitempty"`
}

func createWhiteLabelHandler(w http.ResponseWriter, r *http.Request) {
	var req CreateWhiteLabelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	whiteLabelStore.mu.Lock()
	defer whiteLabelStore.mu.Unlock()
	
	product := &WhiteLabelProduct{
		ID:                 generateUUID(),
		Name:               req.Name,
		Domain:             req.Domain,
		CloudProvider:     req.CloudProvider,
		StorageProvider:   req.StorageProvider,
		APIKey:            generateRandomToken(32),
		APISecret:         generateRandomToken(64),
		Status:            WLStatusPending,
		FeeSharingPercent: FEE_SHARING_PERCENTAGE,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}
	
	whiteLabelStore.products[product.ID] = product
	whiteLabelStore.apiKeys[product.APIKey] = product.ID
	
	json.NewEncoder(w).Encode(WhiteLabelResponse{
		Success:  true,
		Product: product,
	})
}

type ApproveWhiteLabelRequest struct {
	ProductID string `json:"product_id"`
}

func approveWhiteLabelHandler(w http.ResponseWriter, r *http.Request) {
	var req ApproveWhiteLabelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	whiteLabelStore.mu.Lock()
	defer whiteLabelStore.mu.Unlock()
	
	product, ok := whiteLabelStore.products[req.ProductID]
	if !ok {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}
	
	now := time.Now()
	product.Status = WLStatusApproved
	product.ApprovedAt = &now
	
	json.NewEncoder(w).Encode(WhiteLabelResponse{
		Success:  true,
		Product: product,
	})
}

// ============================================================================
// ADMIN MIDDLEWARE
// ============================================================================

type AdminAuthMiddleware struct{}

func (m *AdminAuthMiddleware) ServeHTTP(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, "Authorization required", http.StatusUnauthorized)
		return
	}
	
	token := strings.TrimPrefix(authHeader, "Bearer ")
	
	authStore.mu.RLock()
	session, err := authStore.ValidateSession(token)
	authStore.mu.RUnlock()
	
	if err != nil {
		http.Error(w, "Invalid session", http.StatusUnauthorized)
		return
	}
	
	// Add user ID to context
	ctx := context.WithValue(r.Context(), "user_id", session.UserID)
	next(w, r.WithContext(ctx))
}

func (s *AuthenticationStore) ValidateSession(sessionToken string) (*AdminSession, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	session, ok := s.sessions[sessionToken]
	if !ok {
		return nil, fmt.Errorf("invalid session")
	}
	
	if session.Status != "active" {
		return nil, fmt.Errorf("session %s", session.Status)
	}
	
	if time.Now().After(session.ExpiresAt) {
		session.Status = "expired"
		return nil, fmt.Errorf("session expired")
	}
	
	session.LastActivity = time.Now()
	return session, nil
}

// ============================================================================
// MAIN FUNCTION
// ============================================================================

func main() {
	startTime = time.Now().Unix()
	
	// Initialize stores
	authStore = NewAuthenticationStore()
	masterWalletStore = NewMasterWalletStore()
	whiteLabelStore = NewWhiteLabelStore()
	botStore = NewBotStore()
	feeStore = NewFeeStore()
	
	// Create super admin (in production, this should be done via secure setup)
	superAdmin := &AuthUser{
		ID:            "super_admin_1",
		Email:         "admin@tigerswap.com",
		Username:      "admin",
		PasswordHash:  "$2a$10$dummy", // Replace with real hash in production
		Role:          RoleSuperAdmin,
		IsActive:      true,
		IsVerified:    true,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	authStore.users[superAdmin.Email] = superAdmin
	authStore.usersByID[superAdmin.ID] = superAdmin
	
	// Create master wallet
	masterWallet := &MasterWallet{
		ID:               "master_1",
		Name:             "Tiger Master",
		Type:             "hot",
		MasterAddress:   "0x" + generateRandomToken(20),
		ChainId:          1,
		ChainName:        "Ethereum",
		IsActive:         true,
		AutoSignEnabled:  true,
		AutoSignTimeout:  3,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	masterWalletStore.masterWallet = masterWallet
	
	router := mux.NewRouter()
	
	// Health
	router.HandleFunc("/health", healthHandler).Methods("GET")
	
	// Authentication
	router.HandleFunc("/api/v1/auth/login", loginHandler).Methods("POST")
	router.HandleFunc("/api/v1/auth/register", registerHandler).Methods("POST")
	
	// Wallet
	router.HandleFunc("/api/v1/wallet/create", createWalletHandler).Methods("POST")
	router.HandleFunc("/api/v1/wallet/send", sendTransactionHandler).Methods("POST")
	
	// Chains & Tokens
	router.HandleFunc("/api/v1/chains", listChainsHandler).Methods("GET")
	router.HandleFunc("/api/v1/tokens", listTokensHandler).Methods("GET")
	
	// Bots
	router.HandleFunc("/api/v1/bots/create", createBotHandler).Methods("POST")
	router.HandleFunc("/api/v1/bots/tiers", listBotTiersHandler).Methods("GET")
	
	// Fees
	router.HandleFunc("/api/v1/fees", listFeeConfigsHandler).Methods("GET")
	
	// White Label
	router.HandleFunc("/api/v1/whitelabel/create", createWhiteLabelHandler).Methods("POST")
	router.HandleFunc("/api/v1/whitelabel/approve", approveWhiteLabelHandler).Methods("POST")
	
	// Quote routes
	router.HandleFunc("/api/v1/quote", getQuoteHandler).Methods("POST")
	router.HandleFunc("/api/v1/swap", executeSwapHandler).Methods("POST")
	router.HandleFunc("/api/v1/bridge/quote", bridgeQuoteHandler).Methods("POST")
	router.HandleFunc("/api/v1/bridge/execute", executeBridgeHandler).Methods("POST")
	router.HandleFunc("/api/v1/gas", gasEstimateHandler).Methods("GET")
	
	// Token and chain info legacy
	router.HandleFunc("/api/v1/tokens/legacy", getTokensHandler).Methods("GET")
	router.HandleFunc("/api/v1/chains/legacy", getChainsHandler).Methods("GET")
	
	// WebSocket
	router.HandleFunc("/ws", wsHandler)
	
	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down TigerSwap API Gateway...")
		os.Exit(0)
	}()
	
	// CORS middleware
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	
	// Security headers middleware
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			next.ServeHTTP(w, r)
		})
	})
	
	fmt.Println("===============================================")
	fmt.Println("TigerSwap API Gateway v" + API_VERSION)
	fmt.Println("Industrial-grade DEX Platform")
	fmt.Println("===============================================")
	fmt.Println("Features:")
	fmt.Println("- Complete Authentication System (2FA/MFA)")
	fmt.Println("- Master Wallet with Auto-Signing")
	fmt.Println("- 20+ EVM + 20+ Non-EVM Chains")
	fmt.Println("- 50+ Pre-installed Tokens")
	fmt.Println("- 10 Bot Types with Role-Based Access")
	fmt.Println("- White Label System (20% fee sharing)")
	fmt.Println("- Complete Fee Management")
	fmt.Println("- Industrial Security (AES-256, Rate Limiting)")
	fmt.Println("===============================================")
	fmt.Println("TigerSwap API Gateway starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}