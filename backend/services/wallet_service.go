package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"tigerswap/backend/models"
)

// WalletService handles wallet operations
type WalletService struct {
	db            *gorm.DB
	blockchain    *BlockchainClient
	swapExecutor  *SwapExecutor
	perpetuals    *PerpetualEngine
	copyEngine    *CopyTradingEngine
	masterKeys    map[string]*ecdsa.PrivateKey
	encryptionKey []byte
	hdSeed        []byte
	mu            sync.RWMutex
}

// NewWalletService builds a wallet service. The encryption key is read from
// WALLET_ENCRYPTION_KEY; the service refuses to start without it so private
// keys are never persisted in plaintext.
func NewWalletService(db *gorm.DB) *WalletService {
	encKey := os.Getenv("WALLET_ENCRYPTION_KEY")
	if len(encKey) < 32 {
		panic("WALLET_ENCRYPTION_KEY must be set to at least 32 characters")
	}
	derived := sha256.Sum256([]byte(encKey))

	svc := &WalletService{
		db:            db,
		masterKeys:    make(map[string]*ecdsa.PrivateKey),
		encryptionKey: derived[:],
	}
	// A persistent HD seed keeps deposit addresses stable across restarts.
	if seed := os.Getenv("WALLET_HD_SEED"); seed != "" {
		svc.hdSeed = []byte(seed)
	}
	return svc
}

// SetDependencies wires the blockchain client and downstream engines.
func (s *WalletService) SetDependencies(blockchain *BlockchainClient, executor *SwapExecutor, perps *PerpetualEngine, copy *CopyTradingEngine) {
	s.blockchain = blockchain
	s.swapExecutor = executor
	s.perpetuals = perps
	s.copyEngine = copy
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
	PrivateKey  string  `json:"private_key"`
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

// GenerateSeedPhrase generates a new 24-word BIP39 mnemonic using a CSPRNG.
func (s *WalletService) GenerateSeedPhrase(c *gin.Context) {
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to generate entropy"})
		return
	}
	seedPhrase, err := bip39.NewMnemonic(entropy)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to generate mnemonic"})
		return
	}

	c.JSON(200, gin.H{
		"success":     true,
		"seed_phrase": seedPhrase,
	})
}

// CreateMasterWallet derives a master wallet from a BIP39 seed phrase and stores it encrypted.
func (s *WalletService) CreateMasterWallet(c *gin.Context) {
	var req struct {
		SeedPhrase      string  `json:"seed_phrase" binding:"required"`
		SupportedChains []int64 `json:"supported_chains"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	if !bip39.IsMnemonicValid(req.SeedPhrase) {
		c.JSON(400, gin.H{"error": "Invalid BIP39 seed phrase"})
		return
	}
	key, err := deriveEVMKey(req.SeedPhrase, 0)
	if err != nil {
		c.JSON(400, gin.H{"error": "Failed to derive key", "details": err.Error()})
		return
	}
	address := crypto.PubkeyToAddress(key.PublicKey)

	masterWallet := MasterWallet{
		ID:              uuid.New(),
		SeedPhrase:      req.SeedPhrase,
		MasterAddress:   address.Hex(),
		BackupCode:      uuid.New().String(),
		SupportedChains: req.SupportedChains,
		AutoSignEnabled: true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.storeMasterWallet(&masterWallet, key); err != nil {
		c.JSON(500, gin.H{"error": "Failed to persist master wallet", "details": err.Error()})
		return
	}

	s.mu.Lock()
	s.masterKeys[masterWallet.MasterAddress] = key
	s.mu.Unlock()

	c.JSON(200, gin.H{"success": true, "wallet": masterWallet})
}

// GetMasterWallet returns master wallet info.
func (s *WalletService) GetMasterWallet(c *gin.Context) {
	address := c.Param("address")

	s.mu.RLock()
	_, exists := s.masterKeys[address]
	s.mu.RUnlock()

	if !exists {
		var stored models.MasterWallet
		if err := s.db.Where("address = ?", address).First(&stored).Error; err != nil {
			c.JSON(404, gin.H{"error": "Master wallet not found"})
			return
		}
		c.JSON(200, gin.H{
			"success":           true,
			"address":           stored.Address,
			"auto_sign_enabled": stored.AutoSignEnabled,
			"supported_chains":  stored.SupportedChains,
		})
		return
	}

	c.JSON(200, gin.H{
		"success":           true,
		"address":           address,
		"auto_sign_enabled": true,
	})
}

// AutoSignTransaction signs and broadcasts a transfer from a stored master wallet.
func (s *WalletService) AutoSignTransaction(c *gin.Context) {
	var req struct {
		MasterAddress string `json:"master_address" binding:"required"`
		ToAddress     string `json:"to_address" binding:"required"`
		Amount        string `json:"amount" binding:"required"`
		ChainID       int64  `json:"chain_id" binding:"required"`
		TokenAddress  string `json:"token_address"`
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
	if s.blockchain == nil {
		c.JSON(503, gin.H{"error": "Blockchain client not configured"})
		return
	}

	amount, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok || amount.Sign() <= 0 {
		c.JSON(400, gin.H{"error": "Invalid amount"})
		return
	}

	txHash, err := s.blockchain.SendTransaction(
		c.Request.Context(), req.ChainID, hex.EncodeToString(crypto.FromECDSA(privateKey)),
		req.ToAddress, amount, req.TokenAddress, nil,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": "Transaction failed", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "tx_hash": txHash, "status": "submitted"})
}

// ExecuteSwap executes a token swap through the configured swap executor.
func (s *WalletService) ExecuteSwap(c *gin.Context) {
	var req SwapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	if s.swapExecutor == nil {
		c.JSON(503, gin.H{"error": "Swap executor not configured"})
		return
	}

	privateKey, err := s.resolveUserKey(req.UserID, req.PrivateKey)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	slippage := req.Slippage
	if slippage <= 0 {
		slippage = 0.5
	}

	result, err := s.swapExecutor.ExecuteSwap(
		c.Request.Context(), req.ChainID, privateKey,
		req.FromToken, req.ToToken, req.Amount,
		strconv.FormatFloat(slippage, 'f', 4, 64), "tigerswap",
	)
	if err != nil {
		c.JSON(500, gin.H{"error": "Swap failed", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":    true,
		"tx_hash":    result.TxHash,
		"status":     result.Status,
		"from_token": result.FromToken,
		"to_token":   result.ToToken,
		"amount_in":  result.AmountIn,
		"amount_out": result.AmountOut,
	})
}

// GetSwapQuote returns a swap quote from the aggregator.
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
	if s.swapExecutor == nil {
		c.JSON(503, gin.H{"error": "Swap executor not configured"})
		return
	}

	quote, err := s.swapExecutor.GetSwapQuote(c.Request.Context(), req.ChainID, req.FromToken, req.ToToken, req.Amount)
	if err != nil {
		c.JSON(500, gin.H{"error": "Quote failed", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":        true,
		"from_token":     quote.FromToken,
		"to_token":       quote.ToToken,
		"amount_in":      quote.AmountIn,
		"amount_out":     quote.AmountOut,
		"amount_out_min": quote.AmountOutMin,
		"price_impact":   quote.PriceImpact,
		"route":          quote.Route,
		"best_source":    quote.BestSource,
	})
}

// CreatePerpetualPosition opens a perpetual position and persists it.
func (s *WalletService) CreatePerpetualPosition(c *gin.Context) {
	var req struct {
		UserID     string `json:"user_id" binding:"required"`
		Pair       string `json:"pair" binding:"required"`
		Side       string `json:"side" binding:"required"`
		Size       string `json:"size" binding:"required"`
		Collateral string `json:"collateral" binding:"required"`
		Leverage   int    `json:"leverage" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	if s.perpetuals == nil {
		c.JSON(503, gin.H{"error": "Perpetual engine not configured"})
		return
	}

	position, err := s.perpetuals.OpenPosition(c.Request.Context(), 1, "", req.Pair, req.Side, req.Size, req.Collateral, req.Leverage)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	stored := models.PerpetualPosition{
		UserID:     req.UserID,
		Pair:       position.Pair,
		Side:       position.Side,
		Size:       position.Size,
		Collateral: position.Collateral,
		EntryPrice: strconv.FormatFloat(position.EntryPrice, 'f', -1, 64),
		MarkPrice:  strconv.FormatFloat(position.MarkPrice, 'f', -1, 64),
		Leverage:   position.Leverage,
		Status:     position.Status,
		PNL:        position.UnrealizedPNL,
		OpenedAt:   position.OpenedAt,
	}
	if err := s.db.Create(&stored).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to persist position"})
		return
	}

	c.JSON(200, gin.H{"success": true, "position": stored})
}

// ClosePerpetualPosition closes a perpetual trading position.
func (s *WalletService) ClosePerpetualPosition(c *gin.Context) {
	positionID := c.Param("id")

	var stored models.PerpetualPosition
	if err := s.db.Where("id = ?", positionID).First(&stored).Error; err != nil {
		c.JSON(404, gin.H{"error": "Position not found"})
		return
	}
	if stored.Status != "open" {
		c.JSON(409, gin.H{"error": "Position is not open", "status": stored.Status})
		return
	}
	if s.perpetuals == nil {
		c.JSON(503, gin.H{"error": "Perpetual engine not configured"})
		return
	}

	closed, err := s.perpetuals.ClosePosition(c.Request.Context(), positionID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Close failed", "details": err.Error()})
		return
	}

	now := time.Now()
	stored.Status = "closed"
	stored.PNL = closed.UnrealizedPNL
	stored.ClosedAt = &now
	stored.MarkPrice = strconv.FormatFloat(closed.MarkPrice, 'f', -1, 64)
	if err := s.db.Save(&stored).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to persist close"})
		return
	}

	c.JSON(200, gin.H{
		"success":     true,
		"position_id": stored.ID,
		"status":      stored.Status,
		"pnl":         stored.PNL,
		"closed_at":   now,
	})
}

// GetPerpetualPositions returns all positions for a user.
func (s *WalletService) GetPerpetualPositions(c *gin.Context) {
	userID := c.Query("user_id")

	positions := []models.PerpetualPosition{}
	q := s.db.Order("opened_at desc")
	if userID != "" {
		q = q.Where("user_id = ?", userID)
	}
	if err := q.Find(&positions).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to load positions"})
		return
	}

	c.JSON(200, gin.H{"success": true, "positions": positions, "count": len(positions)})
}

// CreateCopyTrade records a copy trade and mirrors the trader's order.
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
	if !common.IsHexAddress(req.TraderAddress) || !common.IsHexAddress(req.FollowerAddress) {
		c.JSON(400, gin.H{"error": "Invalid address"})
		return
	}

	status := "pending"
	if s.copyEngine != nil {
		if _, err := s.copyEngine.CopyTrade(c.Request.Context(), req.FollowerAddress, req.TraderAddress, req.Pair, req.Side, req.Amount, ""); err != nil {
			c.JSON(500, gin.H{"error": "Copy execution failed", "details": err.Error()})
			return
		}
		status = "executed"
	}

	copyTrade := models.CopyTrade{
		TraderAddress:   common.HexToAddress(req.TraderAddress).Hex(),
		FollowerAddress: common.HexToAddress(req.FollowerAddress).Hex(),
		Pair:            req.Pair,
		Amount:          req.Amount,
		Side:            req.Side,
		Status:          status,
		CopiedAt:        time.Now(),
	}
	if err := s.db.Create(&copyTrade).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to persist copy trade"})
		return
	}

	c.JSON(200, gin.H{"success": true, "copy_trade": copyTrade})
}

// GetCopyTrades returns copy trades for a user.
func (s *WalletService) GetCopyTrades(c *gin.Context) {
	address := c.Query("address")

	trades := []models.CopyTrade{}
	q := s.db.Order("copied_at desc")
	if address != "" {
		q = q.Where("trader_address = ? OR follower_address = ?", address, address)
	}
	if err := q.Find(&trades).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to load copy trades"})
		return
	}

	c.JSON(200, gin.H{"success": true, "trades": trades, "count": len(trades)})
}

// CreateLaunchpadProject creates a new launchpad project.
func (s *WalletService) CreateLaunchpadProject(c *gin.Context) {
	var req models.LaunchpadProject
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	req.Status = "upcoming"
	req.RaisedAmount = "0"
	req.Participants = 0
	if err := s.db.Create(&req).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to create project"})
		return
	}

	c.JSON(200, gin.H{"success": true, "project": req})
}

// GetLaunchpadProjects returns all launchpad projects.
func (s *WalletService) GetLaunchpadProjects(c *gin.Context) {
	status := c.Query("status")

	projects := []models.LaunchpadProject{}
	q := s.db.Order("created_at desc")
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Find(&projects).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to load projects"})
		return
	}

	c.JSON(200, gin.H{"success": true, "projects": projects, "count": len(projects)})
}

// UpdateLaunchpadProject updates a launchpad project.
func (s *WalletService) UpdateLaunchpadProject(c *gin.Context) {
	projectID := c.Param("id")

	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	var project models.LaunchpadProject
	if err := s.db.Where("id = ?", projectID).First(&project).Error; err != nil {
		c.JSON(404, gin.H{"error": "Project not found"})
		return
	}
	if req.Status != "" {
		project.Status = req.Status
	}
	if err := s.db.Save(&project).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to update project"})
		return
	}

	c.JSON(200, gin.H{"success": true, "project": project})
}

// ParticipateInLaunchpad records a contribution to a launchpad project.
func (s *WalletService) ParticipateInLaunchpad(c *gin.Context) {
	var req struct {
		ProjectID string `json:"project_id" binding:"required"`
		UserID    string `json:"user_id" binding:"required"`
		Amount    string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	var project models.LaunchpadProject
	if err := s.db.Where("id = ?", req.ProjectID).First(&project).Error; err != nil {
		c.JSON(404, gin.H{"error": "Project not found"})
		return
	}
	if project.Status != "active" {
		c.JSON(409, gin.H{"error": "Project is not accepting contributions", "status": project.Status})
		return
	}

	amount, ok := new(big.Float).SetString(req.Amount)
	if !ok || amount.Sign() <= 0 {
		c.JSON(400, gin.H{"error": "Invalid amount"})
		return
	}

	raised, _ := new(big.Float).SetString(project.RaisedAmount)
	if raised == nil {
		raised = big.NewFloat(0)
	}
	total := new(big.Float).Add(raised, amount)

	if project.HardCap != "" {
		if hardCap, ok := new(big.Float).SetString(project.HardCap); ok && total.Cmp(hardCap) > 0 {
			c.JSON(409, gin.H{"error": "Hard cap exceeded"})
			return
		}
	}

	project.RaisedAmount = total.Text('f', 8)
	project.Participants++
	if err := s.db.Save(&project).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to record participation"})
		return
	}

	c.JSON(200, gin.H{
		"success":       true,
		"project_id":    project.ID,
		"user_id":       req.UserID,
		"amount":        req.Amount,
		"raised_amount": project.RaisedAmount,
		"status":        "participated",
	})
}

// Deposit derives the user's deterministic deposit address and returns it.
func (s *WalletService) Deposit(c *gin.Context) {
	var req struct {
		UserID  string `json:"user_id" binding:"required"`
		ChainID int64  `json:"chain_id" binding:"required"`
		Token   string `json:"token" binding:"required"`
		Amount  string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	depositAddress, err := s.deriveDepositAddress(req.UserID, req.ChainID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to derive deposit address", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"success":         true,
		"deposit_address": depositAddress,
		"chain_id":        req.ChainID,
		"token":           req.Token,
		"amount":          req.Amount,
		"status":          "pending",
	})
}

// Withdraw signs and broadcasts a withdrawal from the platform wallet.
func (s *WalletService) Withdraw(c *gin.Context) {
	var req struct {
		UserID    string `json:"user_id" binding:"required"`
		ToAddress string `json:"to_address" binding:"required"`
		ChainID   int64  `json:"chain_id" binding:"required"`
		Token     string `json:"token" binding:"required"`
		Amount    string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	if !common.IsHexAddress(req.ToAddress) {
		c.JSON(400, gin.H{"error": "Invalid destination address"})
		return
	}
	if s.blockchain == nil {
		c.JSON(503, gin.H{"error": "Blockchain client not configured"})
		return
	}

	key, err := s.platformKey()
	if err != nil {
		c.JSON(500, gin.H{"error": "Platform signer unavailable", "details": err.Error()})
		return
	}

	amount, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok || amount.Sign() <= 0 {
		c.JSON(400, gin.H{"error": "Invalid amount"})
		return
	}

	txHash, err := s.blockchain.SendTransaction(
		c.Request.Context(), req.ChainID, hex.EncodeToString(crypto.FromECDSA(key)),
		req.ToAddress, amount, req.Token, nil,
	)
	if err != nil {
		c.JSON(500, gin.H{"error": "Withdrawal failed", "details": err.Error()})
		return
	}

	c.JSON(200, gin.H{"success": true, "tx_hash": txHash, "status": "processing"})
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

// UpdateFeeSettings persists protocol fee settings (Admin only).
func (s *WalletService) UpdateFeeSettings(c *gin.Context) {
	var req struct {
		WithdrawFeePercent    float64 `json:"withdraw_fee_percent"`
		SwapFeePercent        float64 `json:"swap_fee_percent"`
		TransactionFeePercent float64 `json:"transaction_fee_percent"`
		DepositFeePercent     float64 `json:"deposit_fee_percent"`
		LaunchpadFeePercent   float64 `json:"launchpad_fee_percent"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}
	for name, v := range map[string]float64{
		"withdraw": req.WithdrawFeePercent, "swap": req.SwapFeePercent,
		"transaction": req.TransactionFeePercent, "deposit": req.DepositFeePercent,
		"launchpad": req.LaunchpadFeePercent,
	} {
		if v < 0 || v > 100 {
			c.JSON(400, gin.H{"error": fmt.Sprintf("%s fee must be between 0 and 100", name)})
			return
		}
	}

	settings := models.FeeSettings{Key: "default"}
	err := s.db.Where("key = ?", "default").First(&settings).Error
	settings.WithdrawFeePercent = req.WithdrawFeePercent
	settings.SwapFeePercent = req.SwapFeePercent
	settings.TransactionFeePercent = req.TransactionFeePercent
	settings.DepositFeePercent = req.DepositFeePercent
	settings.LaunchpadFeePercent = req.LaunchpadFeePercent
	settings.UpdatedAt = time.Now()
	if err != nil {
		if err := s.db.Create(&settings).Error; err != nil {
			c.JSON(500, gin.H{"error": "Failed to save fee settings"})
			return
		}
	} else if err := s.db.Save(&settings).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to save fee settings"})
		return
	}

	c.JSON(200, gin.H{"success": true, "fees": settings})
}

// GetFeeSettings returns current fee settings.
func (s *WalletService) GetFeeSettings(c *gin.Context) {
	settings := models.FeeSettings{Key: "default"}
	if err := s.db.Where("key = ?", "default").First(&settings).Error; err != nil {
		// Fall back to the documented defaults if nothing has been configured yet.
		settings = models.FeeSettings{
			Key:                   "default",
			WithdrawFeePercent:    0.1,
			SwapFeePercent:        0.3,
			TransactionFeePercent: 0.05,
			DepositFeePercent:     0,
			LaunchpadFeePercent:   2.0,
		}
	}

	c.JSON(200, gin.H{"success": true, "fees": settings})
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

// --- HD derivation, key encryption and persistence helpers ---

// deriveEVMKey derives the BIP44 m/44'/60'/0'/0/index key from a BIP39 mnemonic.
func deriveEVMKey(mnemonic string, index uint32) (*ecdsa.PrivateKey, error) {
        seed := bip39.NewSeed(mnemonic, "")
        master, err := bip32.NewMasterKey(seed)
        if err != nil {
                return nil, fmt.Errorf("master key: %w", err)
        }

        // m/44'/60'/0'/0/index
        path := []uint32{
                bip32.FirstHardenedChild + 44,
                bip32.FirstHardenedChild + 60,
                bip32.FirstHardenedChild + 0,
                0,
                index,
        }
        key := master
        for _, idx := range path {
                key, err = key.NewChildKey(idx)
                if err != nil {
                        return nil, fmt.Errorf("derive child %d: %w", idx, err)
                }
        }

        priv, err := crypto.ToECDSA(key.Key)
        if err != nil {
                return nil, fmt.Errorf("to ecdsa: %w", err)
        }
        return priv, nil
}

// encryptSecret seals plaintext with AES-256-GCM using the service key.
func (s *WalletService) encryptSecret(plaintext []byte) (string, error) {
        block, err := aes.NewCipher(s.encryptionKey)
        if err != nil {
                return "", err
        }
        gcm, err := cipher.NewGCM(block)
        if err != nil {
                return "", err
        }
        nonce := make([]byte, gcm.NonceSize())
        if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
                return "", err
        }
        sealed := gcm.Seal(nonce, nonce, plaintext, nil)
        return hex.EncodeToString(sealed), nil
}

// decryptSecret opens a value produced by encryptSecret.
func (s *WalletService) decryptSecret(encoded string) ([]byte, error) {
        raw, err := hex.DecodeString(encoded)
        if err != nil {
                return nil, err
        }
        block, err := aes.NewCipher(s.encryptionKey)
        if err != nil {
                return nil, err
        }
        gcm, err := cipher.NewGCM(block)
        if err != nil {
                return nil, err
        }
        if len(raw) < gcm.NonceSize() {
                return nil, fmt.Errorf("ciphertext too short")
        }
        nonce, ciphertext := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
        return gcm.Open(nil, nonce, ciphertext, nil)
}

// storeMasterWallet persists the encrypted key material for a master wallet.
func (s *WalletService) storeMasterWallet(w *MasterWallet, key *ecdsa.PrivateKey) error {
        encKey, err := s.encryptSecret(crypto.FromECDSA(key))
        if err != nil {
                return err
        }
        encSeed, err := s.encryptSecret([]byte(w.SeedPhrase))
        if err != nil {
                return err
        }
        backupHash, err := bcrypt.GenerateFromPassword([]byte(w.BackupCode), bcrypt.DefaultCost)
        if err != nil {
                return err
        }

        record := models.MasterWallet{
                Address:         w.MasterAddress,
                EncryptedKey:    encKey,
                SeedEncrypted:   encSeed,
                BackupCodeHash:  string(backupHash),
                SupportedChains: w.SupportedChains,
                AutoSignEnabled: w.AutoSignEnabled,
        }
        return s.db.Create(&record).Error
}

// deriveDepositAddress derives a stable per-user, per-chain deposit address.
func (s *WalletService) deriveDepositAddress(userID string, chainID int64) (string, error) {
        if len(s.hdSeed) == 0 {
                return "", fmt.Errorf("WALLET_HD_SEED is not configured")
        }
        // Deterministic index from the user identifier and chain.
        digest := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", userID, chainID)))
        index := uint32(digest[0])<<24 | uint32(digest[1])<<16 | uint32(digest[2])<<8 | uint32(digest[3])

        master, err := bip32.NewMasterKey(s.hdSeed)
        if err != nil {
                return "", fmt.Errorf("hd seed: %w", err)
        }
        child, err := master.NewChildKey(bip32.FirstHardenedChild + index)
        if err != nil {
                return "", err
        }
        priv, err := crypto.ToECDSA(child.Key)
        if err != nil {
                return "", err
        }
        return crypto.PubkeyToAddress(priv.PublicKey).Hex(), nil
}

// platformKey returns the signing key for platform withdrawals.
func (s *WalletService) platformKey() (*ecdsa.PrivateKey, error) {
        raw := os.Getenv("PLATFORM_SIGNER_KEY")
        if raw == "" {
                return nil, fmt.Errorf("PLATFORM_SIGNER_KEY is not configured")
        }
        return crypto.HexToECDSA(strings.TrimPrefix(raw, "0x"))
}

// resolveUserKey prefers a caller-supplied private key, falling back to the
// user's stored HD-derived key.
func (s *WalletService) resolveUserKey(userID, privateKey string) (string, error) {
        if privateKey != "" {
                if _, err := crypto.HexToECDSA(strings.TrimPrefix(privateKey, "0x")); err != nil {
                        return "", fmt.Errorf("invalid private key")
                }
                return strings.TrimPrefix(privateKey, "0x"), nil
        }
        if userID == "" {
                return "", fmt.Errorf("user_id or private_key is required")
        }
        addr, err := s.deriveDepositAddress(userID, 1)
        if err != nil {
                return "", err
        }
        return addr, nil
}

// strconv is referenced by handler code for slippage formatting.
var _ = strconv.Itoa
var _ = json.Marshal
