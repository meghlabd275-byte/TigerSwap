package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
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
	ledger        *LedgerService
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

// SetLedger wires the internal accounting service used to authorize withdrawals.
func (s *WalletService) SetLedger(ledger *LedgerService) {
	s.ledger = ledger
}

// GetBalances returns the authenticated caller's platform ledger balances.
func (s *WalletService) GetBalances(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	if s.ledger == nil {
		c.JSON(503, gin.H{"error": "Ledger not configured"})
		return
	}

	balances, err := s.ledger.Balances(userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to load balances"})
		return
	}
	c.JSON(200, gin.H{"success": true, "balances": balances})
}

// MasterWallet represents the admin master wallet. The seed phrase and backup
// code are secret material and are never serialized: the struct is tagged so a
// handler cannot leak them by returning it directly.
type MasterWallet struct {
	ID              uuid.UUID `json:"id"`
	UserID          string    `json:"user_id"`
	SeedPhrase      string    `json:"-"`
	MasterAddress   string    `json:"master_address"`
	BackupCode      string    `json:"-"`
	SupportedChains []int64   `json:"supported_chains"`
	AutoSignEnabled bool      `json:"auto_sign_enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// UserWallet represents a user wallet. The seed phrase is secret material and
// is never serialized.
type UserWallet struct {
	ID              uuid.UUID         `json:"id"`
	UserID          uuid.UUID         `json:"user_id"`
	SeedPhrase      string            `json:"-"`
	WalletAddresses map[string]string `json:"wallet_addresses"` // chainType_chainId -> address
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// SwapRequest represents a swap request. It carries a client-signed payload;
// a raw private key is never accepted over HTTP.
type SwapRequest struct {
	ChainID   int64   `json:"chain_id" binding:"required"`
	FromToken string  `json:"from_token" binding:"required"`
	ToToken   string  `json:"to_token" binding:"required"`
	Amount    string  `json:"amount" binding:"required"`
	Slippage  float64 `json:"slippage"`
	SignedTx  string  `json:"signed_tx" binding:"required"`
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

// CreateMasterWallet derives a master wallet from a BIP39 seed phrase and stores it encrypted.
func (s *WalletService) CreateMasterWallet(c *gin.Context) {
	var req struct {
		SeedPhrase      string  `json:"seed_phrase" binding:"required"`
		SupportedChains []int64 `json:"supported_chains"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	if !bip39.IsMnemonicValid(req.SeedPhrase) {
		c.JSON(400, gin.H{"error": "Invalid BIP39 seed phrase"})
		return
	}
	key, err := deriveEVMKey(req.SeedPhrase, 0)
	if err != nil {
		// The phrase failed to derive a key; the underlying error can name an
		// internal derivation detail, so it is logged rather than returned.
		log.Printf("wallet: derive key failed: %v", err)
		c.JSON(400, gin.H{"error": "Failed to derive key"})
		return
	}
	address := crypto.PubkeyToAddress(key.PublicKey)

	masterWallet := MasterWallet{
		ID:              uuid.New(),
		UserID:          userID,
		SeedPhrase:      req.SeedPhrase,
		MasterAddress:   address.Hex(),
		BackupCode:      uuid.New().String(),
		SupportedChains: req.SupportedChains,
		AutoSignEnabled: true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if err := s.storeMasterWallet(&masterWallet, key); err != nil {
		// `err` here is an internal failure (a signer, a database write or an
		// upstream RPC) whose text can carry a path or a connection string; it is
		// logged for the operator and not returned to the caller.
		log.Printf("wallet: failed to persist master wallet: %v", err)
		c.JSON(500, gin.H{"error": "failed to persist master wallet"})
		return
	}

	s.mu.Lock()
	s.masterKeys[masterWallet.MasterAddress] = key
	s.mu.Unlock()

	// The seed phrase is deliberately not echoed: the caller supplied it and it
	// is stored encrypted. The backup code is shown exactly once here, because it
	// is persisted only as a bcrypt hash and can never be retrieved again.
	c.JSON(200, gin.H{
		"success": true,
		"wallet": gin.H{
			"id":                masterWallet.ID,
			"address":           masterWallet.MasterAddress,
			"supported_chains":  masterWallet.SupportedChains,
			"auto_sign_enabled": masterWallet.AutoSignEnabled,
			"created_at":        masterWallet.CreatedAt,
		},
		"backup_code":        masterWallet.BackupCode,
		"backup_code_notice": "Store this backup code now; it is shown only once.",
	})
}

// GetMasterWallet returns the authenticated caller's master wallet info.
func (s *WalletService) GetMasterWallet(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var stored models.MasterWallet
	if err := s.db.Where("user_id = ?", userID).First(&stored).Error; err != nil {
		c.JSON(404, gin.H{"error": "Master wallet not found"})
		return
	}

	c.JSON(200, gin.H{
		"success":           true,
		"address":           stored.Address,
		"auto_sign_enabled": stored.AutoSignEnabled,
		"supported_chains":  stored.SupportedChains,
	})
}

// AutoSignTransaction signs and broadcasts a transfer from the caller's own
// stored master wallet. The wallet is looked up by owner, so one account can
// never spend from another account's custody wallet.
func (s *WalletService) AutoSignTransaction(c *gin.Context) {
	var req struct {
		ToAddress    string `json:"to_address" binding:"required"`
		Amount       string `json:"amount" binding:"required"`
		ChainID      int64  `json:"chain_id" binding:"required"`
		TokenAddress string `json:"token_address"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var stored models.MasterWallet
	if err := s.db.Where("user_id = ?", userID).First(&stored).Error; err != nil {
		c.JSON(404, gin.H{"error": "Master wallet not found"})
		return
	}
	if !stored.AutoSignEnabled {
		c.JSON(403, gin.H{"error": "Auto-sign is disabled for this wallet"})
		return
	}
	if !common.IsHexAddress(req.ToAddress) {
		c.JSON(400, gin.H{"error": "Invalid destination address"})
		return
	}
	if !chainSupported(stored.SupportedChains, req.ChainID) {
		c.JSON(400, gin.H{"error": "Chain is not enabled for this wallet"})
		return
	}
	if s.blockchain == nil {
		c.JSON(503, gin.H{"error": "Blockchain client not configured"})
		return
	}

	privateKey, err := s.loadMasterKey(&stored)
	if err != nil {
		// `err` here is an internal failure (a signer, a database write or an
		// upstream RPC) whose text can carry a path or a connection string; it is
		// logged for the operator and not returned to the caller.
		log.Printf("wallet: master wallet signer unavailable: %v", err)
		c.JSON(500, gin.H{"error": "master wallet signer unavailable"})
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
		// `err` here is an internal failure (a signer, a database write or an
		// upstream RPC) whose text can carry a path or a connection string; it is
		// logged for the operator and not returned to the caller.
		log.Printf("wallet: transaction failed: %v", err)
		c.JSON(500, gin.H{"error": "transaction failed"})
		return
	}

	c.JSON(200, gin.H{"success": true, "tx_hash": txHash, "status": "submitted"})
}

// chainSupported reports whether chainID appears in a master wallet's enabled
// chain list. An empty list means the wallet was created without an explicit
// restriction, so every configured chain is allowed.
func chainSupported(supported []int64, chainID int64) bool {
	if len(supported) == 0 {
		return true
	}
	for _, id := range supported {
		if id == chainID {
			return true
		}
	}
	return false
}

// loadMasterKey returns the decrypted private key for a stored master wallet,
// preferring the in-memory cache and falling back to the encrypted record.
func (s *WalletService) loadMasterKey(stored *models.MasterWallet) (*ecdsa.PrivateKey, error) {
	s.mu.RLock()
	cached, exists := s.masterKeys[stored.Address]
	s.mu.RUnlock()
	if exists {
		return cached, nil
	}

	raw, err := s.decryptSecret(stored.EncryptedKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt master key: %w", err)
	}
	key, err := crypto.ToECDSA(raw)
	if err != nil {
		return nil, fmt.Errorf("parse master key: %w", err)
	}

	s.mu.Lock()
	s.masterKeys[stored.Address] = key
	s.mu.Unlock()
	return key, nil
}

// ExecuteSwap broadcasts a client-signed swap. The server holds no private key:
// the client signs locally and posts the signed payload.
func (s *WalletService) ExecuteSwap(c *gin.Context) {
	var req SwapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	if s.swapExecutor == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Swap executor not configured"})
		return
	}

	txHash, err := s.swapExecutor.BroadcastTransaction(c.Request.Context(), req.ChainID, req.SignedTx)
	if err != nil {
		log.Printf("wallet: swap broadcast failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Swap failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"tx_hash": txHash,
		"status":  "submitted",
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
		respondBadRequestError(c, err)
		return
	}
	if s.swapExecutor == nil {
		c.JSON(503, gin.H{"error": "Swap executor not configured"})
		return
	}

	quote, err := s.swapExecutor.GetSwapQuote(c.Request.Context(), req.ChainID, req.FromToken, req.ToToken, req.Amount)
	if err != nil {
		// `err` here is an internal failure (a signer, a database write or an
		// upstream RPC) whose text can carry a path or a connection string; it is
		// logged for the operator and not returned to the caller.
		log.Printf("wallet: quote failed: %v", err)
		c.JSON(500, gin.H{"error": "quote failed"})
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

// authenticatedUserID returns the caller's id from the session middleware.
// Wallet operations act only on the authenticated account; a client-supplied
// user id is never trusted.
func authenticatedUserID(c *gin.Context) (string, bool) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authenticated session required"})
		return "", false
	}
	return userID, true
}

// CreatePerpetualPosition opens a perpetual position and persists it.
func (s *WalletService) CreatePerpetualPosition(c *gin.Context) {
	var req struct {
		Pair       string `json:"pair" binding:"required"`
		Side       string `json:"side" binding:"required"`
		Size       string `json:"size" binding:"required"`
		Collateral string `json:"collateral" binding:"required"`
		Leverage   int    `json:"leverage" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	if s.perpetuals == nil {
		c.JSON(503, gin.H{"error": "Perpetual engine not configured"})
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	position, err := s.perpetuals.OpenPosition(c.Request.Context(), userID, 1, req.Pair, req.Side, req.Size, req.Collateral, req.Leverage)
	if err != nil {
		respondBadRequestError(c, err)
		return
	}

	positionID, err := uuid.Parse(position.ID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to allocate position id"})
		return
	}

	stored := models.PerpetualPosition{
		ID:         positionID,
		UserID:     userID,
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

// ClosePerpetualPosition closes a perpetual trading position owned by the
// authenticated caller.
func (s *WalletService) ClosePerpetualPosition(c *gin.Context) {
	// The position id may arrive as a path parameter or in the body; accept
	// either so the same handler serves both route shapes.
	positionID := c.Param("id")
	if positionID == "" {
		var req struct {
			PositionID string `json:"position_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.PositionID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "position_id is required"})
			return
		}
		positionID = req.PositionID
	}

	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	var stored models.PerpetualPosition
	if err := s.db.Where("id = ? AND user_id = ?", positionID, userID).First(&stored).Error; err != nil {
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

	closed, err := s.perpetuals.ClosePosition(c.Request.Context(), userID, positionID)
	if err != nil {
		// `err` here is an internal failure (a signer, a database write or an
		// upstream RPC) whose text can carry a path or a connection string; it is
		// logged for the operator and not returned to the caller.
		log.Printf("wallet: close failed: %v", err)
		c.JSON(500, gin.H{"error": "close failed"})
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

// GetPerpetualPositions returns the authenticated caller's positions.
func (s *WalletService) GetPerpetualPositions(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	positions := []models.PerpetualPosition{}
	if err := s.db.Where("user_id = ?", userID).Order("opened_at desc").Find(&positions).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to load positions"})
		return
	}

	c.JSON(200, gin.H{"success": true, "positions": positions, "count": len(positions)})
}

// CreateCopyTrade records a copy trade and mirrors the trader's order. The
// follower is the authenticated caller's own wallet, resolved from the account
// rather than a client-supplied address, so one account cannot mirror a trade
// under another wallet's identity.
func (s *WalletService) CreateCopyTrade(c *gin.Context) {
	var req struct {
		TraderAddress string `json:"trader_address" binding:"required"`
		Pair          string `json:"pair" binding:"required"`
		Amount        string `json:"amount" binding:"required"`
		Side          string `json:"side" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	if !common.IsHexAddress(req.TraderAddress) {
		c.JSON(400, gin.H{"error": "Invalid trader address"})
		return
	}

	followerAddress, err := s.walletAddressForUser(userID)
	if err != nil {
		c.JSON(400, gin.H{"error": "connect a wallet before copying trades"})
		return
	}

	status := "pending"
	if s.copyEngine != nil {
		if _, err := s.copyEngine.CopyTrade(c.Request.Context(), followerAddress, strings.ToLower(req.TraderAddress), req.Pair, req.Side, req.Amount, ""); err != nil {
			// `err` here is an internal failure (a signer, a database write or an
			// upstream RPC) whose text can carry a path or a connection string; it is
			// logged for the operator and not returned to the caller.
			log.Printf("wallet: copy execution failed: %v", err)
			c.JSON(500, gin.H{"error": "copy execution failed"})
			return
		}
		status = "executed"
	}

	copyTrade := models.CopyTrade{
		FollowerID:      userID,
		TraderAddress:   common.HexToAddress(req.TraderAddress).Hex(),
		FollowerAddress: common.HexToAddress(followerAddress).Hex(),
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

// GetCopyTrades returns the authenticated caller's copy trades.
func (s *WalletService) GetCopyTrades(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	trades := []models.CopyTrade{}
	if err := s.db.Where("follower_id = ?", userID).Order("copied_at desc").Find(&trades).Error; err != nil {
		c.JSON(500, gin.H{"error": "Failed to load copy trades"})
		return
	}

	c.JSON(200, gin.H{"success": true, "trades": trades, "count": len(trades)})
}

// walletAddressForUser returns the wallet address bound to an account. A copy
// trade is executed under the account's own wallet, so an account with no bound
// wallet cannot copy a trade.
func (s *WalletService) walletAddressForUser(userID string) (string, error) {
	id, err := uuid.Parse(userID)
	if err != nil {
		return "", fmt.Errorf("invalid user id: %w", err)
	}
	var user models.User
	if err := s.db.Select("wallet_address").First(&user, "id = ?", id).Error; err != nil {
		return "", fmt.Errorf("failed to load account: %w", err)
	}
	address := strings.TrimSpace(user.WalletAddress)
	if !common.IsHexAddress(address) {
		return "", errors.New("account has no wallet address")
	}
	return strings.ToLower(address), nil
}

// Deposit derives the user's deterministic deposit address and returns it.
func (s *WalletService) Deposit(c *gin.Context) {
	var req struct {
		ChainID int64  `json:"chain_id" binding:"required"`
		Token   string `json:"token" binding:"required"`
		Amount  string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	depositAddress, err := s.deriveDepositAddress(userID, req.ChainID)
	if err != nil {
		// `err` here is an internal failure (a signer, a database write or an
		// upstream RPC) whose text can carry a path or a connection string; it is
		// logged for the operator and not returned to the caller.
		log.Printf("wallet: failed to derive deposit address: %v", err)
		c.JSON(500, gin.H{"error": "failed to derive deposit address"})
		return
	}

	// Persist the address-to-owner mapping so the deposit watcher can credit
	// the right account when funds arrive. The unique key keeps this idempotent
	// across repeat requests.
	mapping := models.DepositAddress{
		UserID:  userID,
		ChainID: req.ChainID,
		Address: depositAddress,
	}
	if err := s.db.Where("chain_id = ? AND address = ?", req.ChainID, depositAddress).
		FirstOrCreate(&mapping).Error; err != nil {
		// `err` here is an internal failure (a signer, a database write or an
		// upstream RPC) whose text can carry a path or a connection string; it is
		// logged for the operator and not returned to the caller.
		log.Printf("wallet: failed to register deposit address: %v", err)
		c.JSON(500, gin.H{"error": "failed to register deposit address"})
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

// Withdraw signs and broadcasts a withdrawal from the platform wallet. The
// amount is debited from the caller's ledger balance under a row lock, so a
// withdrawal can never exceed what the user holds on the platform. If the
// on-chain transfer fails the debit is reversed.
func (s *WalletService) Withdraw(c *gin.Context) {
	var req struct {
		ToAddress string `json:"to_address" binding:"required"`
		ChainID   int64  `json:"chain_id" binding:"required"`
		Token     string `json:"token" binding:"required"`
		Amount    string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
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
	if s.ledger == nil {
		c.JSON(503, gin.H{"error": "Ledger not configured"})
		return
	}

	amount, ok := new(big.Int).SetString(req.Amount, 10)
	if !ok || amount.Sign() <= 0 {
		c.JSON(400, gin.H{"error": "Invalid amount"})
		return
	}

	// Debit first: this is the authorization gate. It fails closed if the user
	// does not hold enough on the platform.
	if err := s.ledger.Debit(userID, req.ChainID, req.Token, amount); err != nil {
		if errors.Is(err, ErrInsufficientBalance) {
			c.JSON(409, gin.H{"error": "Insufficient balance"})
			return
		}
		// `err` here is an internal failure (a signer, a database write or an
		// upstream RPC) whose text can carry a path or a connection string; it is
		// logged for the operator and not returned to the caller.
		log.Printf("wallet: failed to reserve funds: %v", err)
		c.JSON(500, gin.H{"error": "failed to reserve funds"})
		return
	}

	key, err := s.platformKey()
	if err != nil {
		// Reverse the debit so the user is not charged for a failed withdrawal.
		s.ledger.Credit(userID, req.ChainID, req.Token, "", 0, amount)
		// `err` here is an internal failure (a signer, a database write or an
		// upstream RPC) whose text can carry a path or a connection string; it is
		// logged for the operator and not returned to the caller.
		log.Printf("wallet: platform signer unavailable: %v", err)
		c.JSON(500, gin.H{"error": "platform signer unavailable"})
		return
	}

	txHash, err := s.blockchain.SendTransaction(
		c.Request.Context(), req.ChainID, hex.EncodeToString(crypto.FromECDSA(key)),
		req.ToAddress, amount, req.Token, nil,
	)
	if err != nil {
		if creditErr := s.ledger.Credit(userID, req.ChainID, req.Token, "", 0, amount); creditErr != nil {
			log.Printf("failed to reverse withdrawal debit for user %s: %v", userID, creditErr)
		}
		// `err` here is an internal failure (a signer, a database write or an
		// upstream RPC) whose text can carry a path or a connection string; it is
		// logged for the operator and not returned to the caller.
		log.Printf("wallet: withdrawal failed: %v", err)
		c.JSON(500, gin.H{"error": "withdrawal failed"})
		return
	}

	s.db.Create(&models.Transaction{
		UserID:     parseUserUUID(userID),
		Hash:       txHash,
		ChainID:    req.ChainID,
		Type:       "withdraw",
		FromToken:  req.Token,
		FromAmount: req.Amount,
		ToAddress:  common.HexToAddress(req.ToAddress).Hex(),
		Status:     "pending",
		Timestamp:  time.Now(),
	})

	c.JSON(200, gin.H{"success": true, "tx_hash": txHash, "status": "processing"})
}

// parseUserUUID converts a session user id into a UUID, returning the nil UUID
// when the identifier is not a UUID (for example an external wallet subject).
func parseUserUUID(userID string) uuid.UUID {
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return uuid.Nil
	}
	return parsed
}

// AddToken adds a new token (Super Admin only)
func (s *WalletService) AddToken(c *gin.Context) {
	var req models.Token
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	req.ID = 0
	req.IsActive = true
	req.CreatedAt = time.Now()
	req.UpdatedAt = time.Now()

	s.db.Create(&req)

	c.JSON(200, gin.H{
		"success": true,
		"token":   req,
	})
}

// UpdateToken updates a token (Super Admin only)
func (s *WalletService) UpdateToken(c *gin.Context) {
	id := c.Param("id")

	var req struct {
		Name       string `json:"name"`
		Symbol     string `json:"symbol"`
		Decimals   int    `json:"decimals"`
		LogoURI    string `json:"logo_uri"`
		IsActive   *bool  `json:"is_active"`
		IsVerified *bool  `json:"is_verified"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
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
		"token":   token,
	})
}

// getRPCForChain resolves the chain's RPC endpoint from operator configuration.
// TigerSwap dials its own nodes; there is no third-party fallback, so an
// unconfigured chain yields an empty string and the caller must fail closed.
func (s *WalletService) getRPCForChain(chainID int64) string {
	return rpcFromEnv(chainID)
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
		UserID:          w.UserID,
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
