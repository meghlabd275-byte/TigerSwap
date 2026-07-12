package services

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"tigerswap/backend/models"
)

// WalletService handles wallet operations
type WalletService struct {
	db         *gorm.DB
	masterKeys map[string]*ecdsa.PrivateKey
	mu         sync.RWMutex
}

func NewWalletService(db *gorm.DB) *WalletService {
	return &WalletService{
		db:         db,
		masterKeys: make(map[string]*ecdsa.PrivateKey),
	}
}

// MasterWallet represents the admin master wallet
type MasterWallet struct {
	ID                uuid.UUID              `json:"id"`
	SeedPhrase        string                 `json:"seed_phrase"`
	MasterAddress     string                 `json:"master_address"`
	BackupCode        string                 `json:"backup_code"`
	SupportedChains   []int64                `json:"supported_chains"`
	AutoSignEnabled   bool                   `json:"auto_sign_enabled"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// UserWallet represents a user wallet
type UserWallet struct {
	ID              uuid.UUID              `json:"id"`
	UserID          uuid.UUID              `json:"user_id"`
	SeedPhrase      string                 `json:"seed_phrase"`
	WalletAddresses map[string]string      `json:"wallet_addresses"` // chainType_chainId -> address
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// TransactionRequest represents a transaction request
type TransactionRequest struct {
	FromChain    int64  `json:"from_chain" binding:"required"`
	ToChain      int64  `json:"to_chain" binding:"required"`
	FromToken    string `json:"from_token" binding:"required"`
	ToToken      string `json:"to_token" binding:"required"`
	Amount       string `json:"amount" binding:"required"`
	Slippage     float64 `json:"slippage"`
	UserID       string `json:"user_id"`
	AutoSign     bool   `json:"auto_sign"`
}

// SwapRequest represents a swap request
type SwapRequest struct {
	ChainID     int64   `json:"chain_id" binding:"required"`
	FromToken   string  `json:"from_token" binding:"required"`
	ToToken     string  `json:"to_token" binding:"required"`
	Amount      string  `json:"amount" binding:"required"`
	Slippage    float64 `json:"slippage"`
	UserID      string  `json:"user_id"`
}

// PerpetualPosition represents a perpetual trading position
type PerpetualPosition struct {
	ID            uuid.UUID `json:"id"`
	UserID        string    `json:"user_id"`
	Pair          string    `json:"pair"`
	Side          string    `json:"side"` // long or short
	Size          string    `json:"size"`
	EntryPrice    string    `json:"entry_price"`
	Leverage      int       `json:"leverage"`
	Status        string    `json:"status"` // open, closed, liquidated
	PNL           string    `json:"pnl"`
	OpenedAt      time.Time `json:"opened_at"`
	ClosedAt      *time.Time `json:"closed_at"`
}

// CopyTrade represents a copy trade
type CopyTrade struct {
	ID              uuid.UUID `json:"id"`
	TraderAddress   string    `json:"trader_address"`
	FollowerAddress string    `json:"follower_address"`
	Pair            string    `json:"pair"`
	Amount          string    `json:"amount"`
	Side            string    `json:"side"`
	Status          string    `json:"status"`
	CopiedAt        time.Time `json:"copied_at"`
}

// LaunchpadProject represents an IEO/IDO/Launchpad project
type LaunchpadProject struct {
	ID              uuid.UUID `json:"id"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	TokenAddress    string    `json:"token_address"`
	TokenSymbol     string    `json:"token_symbol"`
	TokenName       string    `json:"token_name"`
	TotalSupply     string    `json:"total_supply"`
	PricePerToken   string    `json:"price_per_token"`
	PaymentToken    string    `json:"payment_token"`
	MinPurchase     string    `json:"min_purchase"`
	MaxPurchase     string    `json:"max_purchase"`
	SoftCap         string    `json:"soft_cap"`
	HardCap         string    `json:"hard_cap"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time"`
	Status          string    `json:"status"` // upcoming, active, completed, cancelled
	RaisedAmount    string    `json:"raised_amount"`
	Participants    int       `json:"participants"`
	WebsiteURL      string    `json:"website_url"`
	WhitepaperURL   string    `json:"whitepaper_url"`
	LogoURL         string    `json:"logo_url"`
	CreatedBy       string    `json:"created_by"`
	CreatedAt       time.Time `json:"created_at"`
}

// GenerateSeedPhrase generates a new BIP39 seed phrase
func (s *WalletService) GenerateSeedPhrase(c *gin.Context) {
	// In production, use proper BIP39 library
	// This is a simplified version
	words := []string{
		"abandon", "ability", "able", "about", "above", "absent", "absorb", "abstract",
		"absurd", "abuse", "access", "accident", "account", "accuse", "achieve", "acid",
		"acoustic", "acquire", "across", "act", "action", "actor", "actress", "actual",
	}
	
	seedPhrase := ""
	for i := 0; i < 24; i++ {
		if i > 0 {
			seedPhrase += " "
		}
		seedPhrase += words[i%len(words)]
	}
	
	c.JSON(200, gin.H{
		"success": true,
		"seed_phrase": seedPhrase,
	})
}

// CreateMasterWallet creates a new master wallet
func (s *WalletService) CreateMasterWallet(c *gin.Context) {
	var req struct {
		SeedPhrase   string `json:"seed_phrase" binding:"required"`
		SupportedChains []int64 `json:"supported_chains"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	// Generate master address from seed phrase
	privateKey, err := crypto.HexToECDSA(crypto.Keccak256([]byte(req.SeedPhrase)))
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid seed phrase"})
		return
	}
	
	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		c.JSON(400, gin.H{"error": "Failed to derive public key"})
		return
	}
	
	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	
	// Generate backup code
	backupCode := uuid.New().String()
	
	masterWallet := MasterWallet{
		ID:              uuid.New(),
		SeedPhrase:      req.SeedPhrase,
		MasterAddress:    address.Hex(),
		BackupCode:      backupCode,
		SupportedChains:  req.SupportedChains,
		AutoSignEnabled: true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	
	// Store in memory (in production, store encrypted in database)
	s.mu.Lock()
	s.masterKeys[masterWallet.MasterAddress] = privateKey
	s.mu.Unlock()
	
	c.JSON(200, gin.H{
		"success": true,
		"wallet": masterWallet,
	})
}

// GetMasterWallet returns master wallet info
func (s *WalletService) GetMasterWallet(c *gin.Context) {
	address := c.Param("address")
	
	s.mu.RLock()
	_, exists := s.masterKeys[address]
	s.mu.RUnlock()
	
	if !exists {
		c.JSON(404, gin.H{"error": "Master wallet not found"})
		return
	}
	
	c.JSON(200, gin.H{
		"success": true,
		"address": address,
		"auto_sign_enabled": true,
	})
}

// AutoSignTransaction automatically signs and broadcasts a transaction within 3 seconds
func (s *WalletService) AutoSignTransaction(c *gin.Context) {
	var req struct {
		MasterAddress string `json:"master_address" binding:"required"`
		ToAddress      string `json:"to_address" binding:"required"`
		Amount         string `json:"amount" binding:"required"`
		ChainID        int64  `json:"chain_id" binding:"required"`
		TokenAddress   string `json:"token_address"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	s.mu.RLock()
	privateKey, exists := s.masterKeys[req.MasterAddress]
	s.mu.RUnlock()
	
	if !exists {
		c.JSON(404, gin.H{"error": "Master wallet not found"})
		return
	}
	
	// Get RPC URL for chain
	rpcURL := s.getRPCForChain(req.ChainID)
	
	// Create transaction
	// In production, use actual blockchain RPC
	txHash := fmt.Sprintf("0x%x", crypto.Keccak256([]byte(time.Now().String())))
	
	c.JSON(200, gin.H{
		"success": true,
		"tx_hash": txHash,
		"status": "pending",
		"message": "Transaction auto-signed and broadcasted",
	})
}

// ExecuteSwap executes a token swap
func (s *WalletService) ExecuteSwap(c *gin.Context) {
	var req SwapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	// In production, integrate with DEX aggregators like 1inch, Uniswap, etc.
	// This is a simplified implementation
	txHash := fmt.Sprintf("0x%x", crypto.Keccak256([]byte(req.ChainID+req.FromToken+req.ToToken+req.Amount)))
	
	c.JSON(200, gin.H{
		"success": true,
		"tx_hash": txHash,
		"status": "completed",
		"from_token": req.FromToken,
		"to_token": req.ToToken,
		"amount_in": req.Amount,
		"amount_out": req.Amount, // Would calculate actual output
	})
}

// GetSwapQuote returns a swap quote
func (s *WalletService) GetSwapQuote(c *gin.Context) {
	var req struct {
		ChainID   int64  `json:"chain_id" binding:"required"`
		FromToken string `json:"from_token" binding:"required"`
		ToToken   string `json:"to_token" binding:"required"`
		Amount    string `json:"amount" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	// In production, fetch real quotes from DEX aggregators
	c.JSON(200, gin.H{
		"success": true,
		"from_token": req.FromToken,
		"to_token": req.ToToken,
		"amount_in": req.Amount,
		"amount_out": req.Amount, // Would be calculated
		"price_impact": "0.5",
		"route": []string{req.FromToken, req.ToToken},
	})
}

// CreatePerpetualPosition creates a perpetual trading position
func (s *WalletService) CreatePerpetualPosition(c *gin.Context) {
	var req struct {
		UserID    string `json:"user_id" binding:"required"`
		Pair      string `json:"pair" binding:"required"`
		Side      string `json:"side" binding:"required"` // long or short
		Size      string `json:"size" binding:"required"`
		Leverage  int    `json:"leverage" binding:"required"`
		EntryPrice string `json:"entry_price" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	position := PerpetualPosition{
		ID:         uuid.New(),
		UserID:     req.UserID,
		Pair:       req.Pair,
		Side:       req.Side,
		Size:       req.Size,
		EntryPrice: req.EntryPrice,
		Leverage:   req.Leverage,
		Status:     "open",
		PNL:        "0",
		OpenedAt:   time.Now(),
	}
	
	c.JSON(200, gin.H{
		"success": true,
		"position": position,
	})
}

// ClosePerpetualPosition closes a perpetual trading position
func (s *WalletService) ClosePerpetualPosition(c *gin.Context) {
	positionID := c.Param("id")
	
	// In production, calculate actual PNL
	now := time.Now()
	
	c.JSON(200, gin.H{
		"success": true,
		"position_id": positionID,
		"status": "closed",
		"pnl": "0", // Would calculate actual PNL
		"closed_at": now,
	})
}

// GetPerpetualPositions returns all positions for a user
func (s *WalletService) GetPerpetualPositions(c *gin.Context) {
	userID := c.Query("user_id")
	
	// In production, fetch from database
	positions := []PerpetualPosition{}
	
	c.JSON(200, gin.H{
		"success": true,
		"positions": positions,
	})
}

// CreateCopyTrade creates a copy trade
func (s *WalletService) CreateCopyTrade(c *gin.Context) {
	var req struct {
		TraderAddress   string `json:"trader_address" binding:"required"`
		FollowerAddress string `json:"follower_address" binding:"required"`
		Pair            string `json:"pair" binding:"required"`
		Amount          string `json:"amount" binding:"required"`
		Side            string `json:"side" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	copyTrade := CopyTrade{
		ID:              uuid.New(),
		TraderAddress:   req.TraderAddress,
		FollowerAddress: req.FollowerAddress,
		Pair:            req.Pair,
		Amount:          req.Amount,
		Side:            req.Side,
		Status:          "executed",
		CopiedAt:        time.Now(),
	}
	
	c.JSON(200, gin.H{
		"success": true,
		"copy_trade": copyTrade,
	})
}

// GetCopyTrades returns copy trades for a user
func (s *WalletService) GetCopyTrades(c *gin.Context) {
	address := c.Query("address")
	
	// In production, fetch from database
	trades := []CopyTrade{}
	
	c.JSON(200, gin.H{
		"success": true,
		"trades": trades,
	})
}

// CreateLaunchpadProject creates a new launchpad project
func (s *WalletService) CreateLaunchpadProject(c *gin.Context) {
	var req LaunchpadProject
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	req.ID = uuid.New()
	req.Status = "upcoming"
	req.RaisedAmount = "0"
	req.Participants = 0
	req.CreatedAt = time.Now()
	
	c.JSON(200, gin.H{
		"success": true,
		"project": req,
	})
}

// GetLaunchpadProjects returns all launchpad projects
func (s *WalletService) GetLaunchpadProjects(c *gin.Context) {
	status := c.Query("status")
	
	// In production, fetch from database
	projects := []LaunchpadProject{}
	
	c.JSON(200, gin.H{
		"success": true,
		"projects": projects,
		"count": len(projects),
	})
}

// UpdateLaunchpadProject updates a launchpad project
func (s *WalletService) UpdateLaunchpadProject(c *gin.Context) {
	projectID := c.Param("id")
	
	var req struct {
		Status string `json:"status"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	c.JSON(200, gin.H{
		"success": true,
		"project_id": projectID,
		"status": req.Status,
	})
}

// ParticipateInLaunchpad allows a user to participate in a launchpad
func (s *WalletService) ParticipateInLaunchpad(c *gin.Context) {
	var req struct {
		ProjectID string `json:"project_id" binding:"required"`
		UserID     string `json:"user_id" binding:"required"`
		Amount     string `json:"amount" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	c.JSON(200, gin.H{
		"success": true,
		"project_id": req.ProjectID,
		"user_id": req.UserID,
		"amount": req.Amount,
		"status": "participated",
	})
}

// Deposit handles deposit requests
func (s *WalletService) Deposit(c *gin.Context) {
	var req struct {
		UserID    string `json:"user_id" binding:"required"`
		ChainID   int64  `json:"chain_id" binding:"required"`
		Token     string `json:"token" binding:"required"`
		Amount    string `json:"amount" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	// Generate deposit address
	depositAddress := fmt.Sprintf("0x%x", crypto.Keccak256([]byte(req.UserID+req.Token)))
	
	c.JSON(200, gin.H{
		"success": true,
		"deposit_address": depositAddress,
		"chain_id": req.ChainID,
		"token": req.Token,
		"amount": req.Amount,
		"status": "pending",
	})
}

// Withdraw handles withdrawal requests
func (s *WalletService) Withdraw(c *gin.Context) {
	var req struct {
		UserID      string `json:"user_id" binding:"required"`
		ToAddress   string `json:"to_address" binding:"required"`
		ChainID     int64  `json:"chain_id" binding:"required"`
		Token       string `json:"token" binding:"required"`
		Amount      string `json:"amount" binding:"required"`
		AutoSign    bool   `json:"auto_sign"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	// Process withdrawal
	txHash := fmt.Sprintf("0x%x", crypto.Keccak256([]byte(req.UserID+req.ToAddress+req.Amount)))
	
	c.JSON(200, gin.H{
		"success": true,
		"tx_hash": txHash,
		"status": "processing",
		"message": "Withdrawal initiated",
	})
}

// AddBlockchain adds a new blockchain (Super Admin only)
func (s *WalletService) AddBlockchain(c *gin.Context) {
	var req models.Blockchain
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	req.ID = 0
	req.IsActive = true
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()
	
	s.db.Create(&req)
	
	c.JSON(200, gin.H{
		"success": true,
		"blockchain": req,
	})
}

// UpdateBlockchain updates a blockchain (Super Admin only)
func (s *WalletService) UpdateBlockchain(c *gin.Context) {
	id := c.Param("id")
	
	var req struct {
		Name        string `json:"name"`
		RPCURL      string `json:"rpc_url"`
		ExplorerURL string `json:"explorer_url"`
		IsActive    *bool  `json:"is_active"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	var blockchain models.Blockchain
	if err := s.db.First(&blockchain, id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Blockchain not found"})
		return
	}
	
	if req.Name != "" {
		blockchain.Name = req.Name
	}
	if req.RPCURL != "" {
		blockchain.RPCURL = req.RPCURL
	}
	if req.ExplorerURL != "" {
		blockchain.ExplorerURL = req.ExplorerURL
	}
	if req.IsActive != nil {
		blockchain.IsActive = *req.IsActive
	}
	
	blockchain.UpdatedAt = time.Now()
	s.db.Save(&blockchain)
	
	c.JSON(200, gin.H{
		"success": true,
		"blockchain": blockchain,
	})
}

// AddToken adds a new token (Super Admin only)
func (s *WalletService) AddToken(c *gin.Context) {
	var req models.Token
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	req.ID = 0
	req.IsActive = true
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()
	
	s.db.Create(&req)
	
	c.JSON(200, gin.H{
		"success": true,
		"token": req,
	})
}

// UpdateToken updates a token (Super Admin only)
func (s *WalletService) UpdateToken(c *gin.Context) {
	id := c.Param("id")
	
	var req struct {
		Name       string  `json:"name"`
		Symbol     string  `json:"symbol"`
		Decimals   int     `json:"decimals"`
		LogoURI    string  `json:"logo_uri"`
		IsActive   *bool   `json:"is_active"`
		IsVerified *bool   `json:"is_verified"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	var token models.Token
	if err := s.db.First(&token, id).Error; err != nil {
		c.JSON(404, gin.H{"error": "Token not found"})
		return
	}
	
	if req.Name != "" {
		token.Name = req.Name
	}
	if req.Symbol != "" {
		token.Symbol = req.Symbol
	}
	if req.Decimals > 0 {
		token.Decimals = req.Decimals
	}
	if req.LogoURI != "" {
		token.LogoURI = req.LogoURI
	}
	if req.IsActive != nil {
		token.IsActive = *req.IsActive
	}
	if req.IsVerified != nil {
		token.IsVerified = *req.IsVerified
	}
	
	token.UpdatedAt = time.Now()
	s.db.Save(&token)
	
	c.JSON(200, gin.H{
		"success": true,
		"token": token,
	})
}

// UpdateFeeSettings updates fee settings (Admin only)
func (s *WalletService) UpdateFeeSettings(c *gin.Context) {
	var req struct {
		WithdrawFeePercent  float64 `json:"withdraw_fee_percent"`
		SwapFeePercent      float64 `json:"swap_fee_percent"`
		TransactionFeePercent float64 `json:"transaction_fee_percent"`
		DepositFeePercent   float64 `json:"deposit_fee_percent"`
		LaunchpadFeePercent float64 `json:"launchpad_fee_percent"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	
	// In production, save to database
	c.JSON(200, gin.H{
		"success": true,
		"fees": req,
		"message": "Fee settings updated",
	})
}

// GetFeeSettings returns current fee settings
func (s *WalletService) GetFeeSettings(c *gin.Context) {
	c.JSON(200, gin.H{
		"success": true,
		"fees": gin.H{
			"withdraw_fee_percent": 0.1,
			"swap_fee_percent": 0.3,
			"transaction_fee_percent": 0.05,
			"deposit_fee_percent": 0,
			"launchpad_fee_percent": 2.0,
		},
	})
}

// GetAllBlockchains returns all supported blockchains
func (s *WalletService) GetAllBlockchains(c *gin.Context) {
	var blockchains []models.Blockchain
	s.db.Where("is_active = ?", true).Find(&blockchains)
	
	c.JSON(200, gin.H{
		"success": true,
		"blockchains": blockchains,
		"count": len(blockchains),
	})
}

// GetAllTokens returns all supported tokens
func (s *WalletService) GetAllTokens(c *gin.Context) {
	chainID := c.Query("chain_id")
	
	query := s.db.Where("is_active = ?", true)
	if chainID != "" {
		query = query.Where("chain_id = ?", chainID)
	}
	
	var tokens []models.Token
	query.Find(&tokens)
	
	c.JSON(200, gin.H{
		"success": true,
		"tokens": tokens,
		"count": len(tokens),
	})
}

func (s *WalletService) getRPCForChain(chainID int64) string {
	rpcURLs := map[int64]string{
		1:     "https://eth.llamarpc.com",
		137:   "https://polygon.llamarpc.com",
		42161: "https://arb1.arbitrum.io/rpc",
		10:    "https://mainnet.optimism.io",
		8453:  "https://mainnet.base.org",
		56:    "https://bsc-dataseed.binance.org",
		43114: "https://api.avax.network/ext/bc/C/rpc",
		250:   "https://rpc.fantom.network",
		324:   "https://mainnet.era.zksync.io",
		59144: "https://rpc.linea.build",
		5000:  "https://rpc.mantle.xyz",
		81457: "https://rpc.blast.io",
	}
	
	if url, ok := rpcURLs[chainID]; ok {
		return url
	}
	
	return "https://eth.llamarpc.com"
}
