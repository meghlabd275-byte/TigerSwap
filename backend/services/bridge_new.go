package services

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"os"
	"strconv"
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
	"gorm.io/gorm"

	"tigerswap/backend/config"
	"tigerswap/backend/models"
)

// tigerBridgeABI is the TigerBridge lock/mint interface used by the relayer.
const tigerBridgeABI = `[
 {"inputs":[{"internalType":"address","name":"token","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"address","name":"recipient","type":"address"},{"internalType":"uint256","name":"dstChainId","type":"uint256"},{"internalType":"uint256","name":"nonce","type":"uint256"}],"name":"lock","outputs":[],"stateMutability":"nonpayable","type":"function"},
 {"inputs":[{"internalType":"address","name":"token","type":"address"},{"internalType":"uint256","name":"amount","type":"uint256"},{"internalType":"address","name":"recipient","type":"address"},{"internalType":"uint256","name":"nonce","type":"uint256"}],"name":"mint","outputs":[],"stateMutability":"nonpayable","type":"function"}
]`

// BridgeService handles TigerBridge cross-chain lock/mint transfers.
type BridgeService struct {
	blockchain *BlockchainClient
	priceAgg   *PriceAggregator
	db         *gorm.DB
	mu         sync.RWMutex

	// Supported bridges (self-hosted relayer routes)
	supportedBridges map[int64][]BridgeConfig
}

// BridgeTransfer represents a bridge transfer.
type BridgeTransfer struct {
	ID             string     `json:"id"`
	SrcChain       int64      `json:"src_chain"`
	DstChain       int64      `json:"dst_chain"`
	Sender         string     `json:"sender"`
	Recipient      string     `json:"recipient"`
	Token          string     `json:"token"`
	Amount         string     `json:"amount"`
	SrcTxHash      string     `json:"src_tx_hash"`
	DstTxHash      string     `json:"dst_tx_hash"`
	Nonce          uint64     `json:"nonce"`
	Status         string     `json:"status"` // pending, confirmed, delivered, failed
	Fee            string     `json:"fee"`
	FeeUSD         float64    `json:"fee_usd"`
	AmountReceived string     `json:"amount_received"`
	CreatedAt      time.Time  `json:"created_at"`
	ConfirmedAt    *time.Time `json:"confirmed_at"`
	DeliveredAt    *time.Time `json:"delivered_at"`
}

// BridgeConfig represents a self-hosted relayer route configuration.
type BridgeConfig struct {
	Name          string  `json:"name"`
	Router        string  `json:"router"`
	ChainAgnostic bool    `json:"chain_agnostic"`
	AvgTime       int     `json:"avg_time_minutes"`
	FeePercent    float64 `json:"fee_percent"`
}

// BridgeQuote represents a bridge quote.
type BridgeQuote struct {
	SrcChain      int64   `json:"src_chain"`
	DstChain      int64   `json:"dst_chain"`
	Token         string  `json:"token"`
	AmountIn      string  `json:"amount_in"`
	AmountOut     string  `json:"amount_out"`
	Fee           string  `json:"fee"`
	FeeUSD        float64 `json:"fee_usd"`
	EstimatedTime string  `json:"estimated_time"`
	Router        string  `json:"router"`
	PriceImpact   float64 `json:"price_impact"`
}

func NewBridgeService(db *gorm.DB, blockchain *BlockchainClient, priceAgg *PriceAggregator) *BridgeService {
	return &BridgeService{
		blockchain: blockchain,
		priceAgg:   priceAgg,
		db:         db,
		supportedBridges: map[int64][]BridgeConfig{
			1:     {{Name: "TigerBridge", Router: tigerBridgeRouter(1), ChainAgnostic: true, AvgTime: 15, FeePercent: 0.1}},
			137:   {{Name: "TigerBridge", Router: tigerBridgeRouter(137), ChainAgnostic: true, AvgTime: 10, FeePercent: 0.08}},
			56:    {{Name: "TigerBridge", Router: tigerBridgeRouter(56), ChainAgnostic: true, AvgTime: 12, FeePercent: 0.08}},
			42161: {{Name: "TigerBridge", Router: tigerBridgeRouter(42161), ChainAgnostic: true, AvgTime: 15, FeePercent: 0.1}},
			10:    {{Name: "TigerBridge", Router: tigerBridgeRouter(10), ChainAgnostic: true, AvgTime: 15, FeePercent: 0.1}},
			8453:  {{Name: "TigerBridge", Router: tigerBridgeRouter(8453), ChainAgnostic: true, AvgTime: 15, FeePercent: 0.1}},
			43114: {{Name: "TigerBridge", Router: tigerBridgeRouter(43114), ChainAgnostic: true, AvgTime: 20, FeePercent: 0.12}},
		},
	}
}

// tigerBridgeRouter returns the configured TigerBridge router for a chain.
func tigerBridgeRouter(chainID int64) string {
	if v := os.Getenv("TIGERBRIDGE_ROUTER_" + strconv.FormatInt(chainID, 10)); v != "" {
		return v
	}
	return os.Getenv("TIGERBRIDGE_ROUTER")
}

// relayerKey loads the TigerBridge relayer signing key.
func relayerKey() (*ecdsa.PrivateKey, error) {
	raw := os.Getenv("TIGERBRIDGE_RELAYER_KEY")
	if raw == "" {
		return nil, fmt.Errorf("TIGERBRIDGE_RELAYER_KEY is not configured")
	}
	return crypto.HexToECDSA(strings.TrimPrefix(raw, "0x"))
}

// GetSupportedChains returns chains supported for bridging.
func (s *BridgeService) GetSupportedChains() []int64 {
	chains := make([]int64, 0, len(s.supportedBridges))
	for id := range s.supportedBridges {
		chains = append(chains, id)
	}
	return chains
}

// route returns the relayer route for a source chain.
func (s *BridgeService) route(srcChain int64) (BridgeConfig, error) {
	bridges := s.supportedBridges[srcChain]
	if len(bridges) == 0 {
		return BridgeConfig{}, fmt.Errorf("no TigerBridge route configured for chain %d", srcChain)
	}
	if bridges[0].Router == "" {
		return BridgeConfig{}, fmt.Errorf("TigerBridge router not configured for chain %d", srcChain)
	}
	return bridges[0], nil
}

// GetQuote returns a bridge quote based on the configured relayer fee.
func (s *BridgeService) GetQuote(ctx context.Context, srcChain, dstChain int64, token, amount string) (*BridgeQuote, error) {
	route, err := s.route(srcChain)
	if err != nil {
		return nil, err
	}
	if srcChain == dstChain {
		return nil, fmt.Errorf("source and destination chains must differ")
	}
	if _, ok := s.supportedBridges[dstChain]; !ok {
		return nil, fmt.Errorf("destination chain %d is not supported", dstChain)
	}

	amountFloat, err := strconv.ParseFloat(amount, 64)
	if err != nil || amountFloat <= 0 {
		return nil, fmt.Errorf("invalid amount %q", amount)
	}

	tokenSymbol := s.getSymbolFromAddress(srcChain, token)
	priceData, err := s.priceAgg.GetRealPrice(ctx, tokenSymbol)
	if err != nil {
		return nil, fmt.Errorf("price unavailable for %s: %w", tokenSymbol, err)
	}
	if priceData.USD <= 0 {
		return nil, fmt.Errorf("invalid price for %s", tokenSymbol)
	}

	amountUSD := amountFloat * priceData.USD
	feeUSD := amountUSD * route.FeePercent / 100
	fee := feeUSD / priceData.USD
	amountOut := amountFloat - fee
	if amountOut <= 0 {
		return nil, fmt.Errorf("amount too small to cover bridge fee")
	}

	return &BridgeQuote{
		SrcChain:      srcChain,
		DstChain:      dstChain,
		Token:         token,
		AmountIn:      amount,
		AmountOut:     strconv.FormatFloat(amountOut, 'f', 8, 64),
		Fee:           strconv.FormatFloat(fee, 'f', 8, 64),
		FeeUSD:        feeUSD,
		EstimatedTime: fmt.Sprintf("%d minutes", route.AvgTime),
		Router:        route.Router,
		PriceImpact:   0.0,
	}, nil
}

// InitiateTransfer locks the token on the source chain via the TigerBridge router.
func (s *BridgeService) InitiateTransfer(ctx context.Context, srcChain, dstChain int64, sender, recipient, token, amount string) (*BridgeTransfer, error) {
	quote, err := s.GetQuote(ctx, srcChain, dstChain, token, amount)
	if err != nil {
		return nil, err
	}
	if !common.IsHexAddress(sender) || !common.IsHexAddress(recipient) {
		return nil, fmt.Errorf("invalid sender or recipient address")
	}
	if !common.IsHexAddress(token) {
		return nil, fmt.Errorf("invalid token address")
	}
	if s.blockchain == nil {
		return nil, fmt.Errorf("blockchain client not configured")
	}

	route, err := s.route(srcChain)
	if err != nil {
		return nil, err
	}

	amountInt, ok := new(big.Int).SetString(amount, 10)
	if !ok || amountInt.Sign() <= 0 {
		return nil, fmt.Errorf("invalid amount %q", amount)
	}

	nonce := uint64(time.Now().UnixNano())
	transfer := &BridgeTransfer{
		ID:             "bridge_" + uuid.New().String(),
		SrcChain:       srcChain,
		DstChain:       dstChain,
		Sender:         sender,
		Recipient:      recipient,
		Token:          token,
		Amount:         amount,
		Nonce:          nonce,
		Status:         "pending",
		Fee:            quote.Fee,
		FeeUSD:         quote.FeeUSD,
		AmountReceived: quote.AmountOut,
		CreatedAt:      time.Now(),
	}

	// Persist before broadcasting so a crash never loses the intent.
	if err := s.persist(transfer); err != nil {
		return nil, fmt.Errorf("persist transfer: %w", err)
	}

	bridgeABI, err := abi.JSON(strings.NewReader(tigerBridgeABI))
	if err != nil {
		return nil, err
	}
	data, err := bridgeABI.Pack("lock",
		common.HexToAddress(token), amountInt,
		common.HexToAddress(recipient), big.NewInt(dstChain), new(big.Int).SetUint64(nonce))
	if err != nil {
		return nil, err
	}

	key, err := relayerKey()
	if err != nil {
		return nil, err
	}
	client, err := s.blockchain.GetClient(srcChain)
	if err != nil {
		return nil, err
	}

	txHash, err := signAndSend(ctx, client, key, srcChain, common.HexToAddress(route.Router), big.NewInt(0), data)
	if err != nil {
		transfer.Status = "failed"
		_ = s.persist(transfer)
		return nil, fmt.Errorf("lock transaction failed: %w", err)
	}
	transfer.SrcTxHash = txHash

	// Wait for the source-chain confirmation before the relayer mints.
	receipt, err := waitForReceipt(ctx, client, common.HexToHash(txHash), 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("awaiting lock confirmation: %w", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		transfer.Status = "failed"
		_ = s.persist(transfer)
		return nil, fmt.Errorf("lock transaction reverted: %s", txHash)
	}

	now := time.Now()
	transfer.Status = "confirmed"
	transfer.ConfirmedAt = &now
	if err := s.persist(transfer); err != nil {
		return nil, fmt.Errorf("persist confirmed transfer: %w", err)
	}

	return transfer, nil
}

// ReleaseMint mints the bridged token on the destination chain. Called by the
// relayer once a source-chain lock is confirmed.
func (s *BridgeService) ReleaseMint(ctx context.Context, transferID string) (*BridgeTransfer, error) {
	transfer, err := s.GetTransfer(transferID)
	if err != nil {
		return nil, err
	}
	if transfer.Status != "confirmed" {
		return nil, fmt.Errorf("transfer %s is not confirmed (status %s)", transferID, transfer.Status)
	}
	if s.blockchain == nil {
		return nil, fmt.Errorf("blockchain client not configured")
	}

	route, err := s.route(transfer.DstChain)
	if err != nil {
		return nil, err
	}

	amountInt, ok := new(big.Int).SetString(transfer.Amount, 10)
	if !ok || amountInt.Sign() <= 0 {
		return nil, fmt.Errorf("invalid stored amount %q", transfer.Amount)
	}

	bridgeABI, err := abi.JSON(strings.NewReader(tigerBridgeABI))
	if err != nil {
		return nil, err
	}
	data, err := bridgeABI.Pack("mint",
		common.HexToAddress(transfer.Token), amountInt,
		common.HexToAddress(transfer.Recipient), new(big.Int).SetUint64(transfer.Nonce))
	if err != nil {
		return nil, err
	}

	key, err := relayerKey()
	if err != nil {
		return nil, err
	}
	client, err := s.blockchain.GetClient(transfer.DstChain)
	if err != nil {
		return nil, err
	}

	txHash, err := signAndSend(ctx, client, key, transfer.DstChain, common.HexToAddress(route.Router), big.NewInt(0), data)
	if err != nil {
		return nil, fmt.Errorf("mint transaction failed: %w", err)
	}
	transfer.DstTxHash = txHash

	receipt, err := waitForReceipt(ctx, client, common.HexToHash(txHash), 5*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("awaiting mint confirmation: %w", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		transfer.Status = "failed"
		_ = s.persist(transfer)
		return nil, fmt.Errorf("mint transaction reverted: %s", txHash)
	}

	now := time.Now()
	transfer.Status = "delivered"
	transfer.DeliveredAt = &now
	if err := s.persist(transfer); err != nil {
		return nil, fmt.Errorf("persist delivered transfer: %w", err)
	}

	return transfer, nil
}

// ProcessPending sweeps confirmed transfers and releases mints on the destination chain.
func (s *BridgeService) ProcessPending(ctx context.Context) error {
	if s.db == nil {
		return fmt.Errorf("database not configured")
	}
	var rows []models.BridgeTransfer
	if err := s.db.Where("status = ?", "confirmed").Order("created_at asc").Limit(50).Find(&rows).Error; err != nil {
		return err
	}
	for i := range rows {
		if _, err := s.ReleaseMint(ctx, rows[i].TransferID); err != nil {
			// Continue processing others; the row stays confirmed for the next sweep.
			continue
		}
	}
	return nil
}

// persist upserts a transfer into the database.
func (s *BridgeService) persist(transfer *BridgeTransfer) error {
	if s.db == nil {
		return fmt.Errorf("database not configured")
	}
	var row models.BridgeTransfer
	err := s.db.Where("transfer_id = ?", transfer.ID).First(&row).Error
	row.TransferID = transfer.ID
	row.SrcChain = transfer.SrcChain
	row.DstChain = transfer.DstChain
	row.Sender = transfer.Sender
	row.Recipient = transfer.Recipient
	row.Token = transfer.Token
	row.Amount = transfer.Amount
	row.SrcTxHash = transfer.SrcTxHash
	row.DstTxHash = transfer.DstTxHash
	row.Nonce = transfer.Nonce
	row.Status = transfer.Status
	row.Fee = transfer.Fee
	row.FeeUSD = transfer.FeeUSD
	row.AmountReceived = transfer.AmountReceived
	row.ConfirmedAt = transfer.ConfirmedAt
	row.DeliveredAt = transfer.DeliveredAt
	row.UpdatedAt = time.Now()
	if err == nil {
		return s.db.Save(&row).Error
	}
	row.CreatedAt = transfer.CreatedAt
	return s.db.Create(&row).Error
}

// GetTransfer returns transfer status.
func (s *BridgeService) GetTransfer(transferID string) (*BridgeTransfer, error) {
	if s.db != nil {
		var row models.BridgeTransfer
		if err := s.db.Where("transfer_id = ?", transferID).First(&row).Error; err == nil {
			return bridgeTransferFromModel(&row), nil
		}
	}
	return nil, fmt.Errorf("transfer not found")
}

// GetUserTransfers returns all transfers for a user.
func (s *BridgeService) GetUserTransfers(userAddress string) []*BridgeTransfer {
	if s.db == nil {
		return nil
	}
	var rows []models.BridgeTransfer
	if err := s.db.Where("sender = ? OR recipient = ?", userAddress, userAddress).
		Order("created_at desc").Find(&rows).Error; err != nil {
		return nil
	}
	result := make([]*BridgeTransfer, 0, len(rows))
	for i := range rows {
		result = append(result, bridgeTransferFromModel(&rows[i]))
	}
	return result
}

func bridgeTransferFromModel(m *models.BridgeTransfer) *BridgeTransfer {
	return &BridgeTransfer{
		ID:             m.TransferID,
		SrcChain:       m.SrcChain,
		DstChain:       m.DstChain,
		Sender:         m.Sender,
		Recipient:      m.Recipient,
		Token:          m.Token,
		Amount:         m.Amount,
		SrcTxHash:      m.SrcTxHash,
		DstTxHash:      m.DstTxHash,
		Nonce:          m.Nonce,
		Status:         m.Status,
		Fee:            m.Fee,
		FeeUSD:         m.FeeUSD,
		AmountReceived: m.AmountReceived,
		CreatedAt:      m.CreatedAt,
		ConfirmedAt:    m.ConfirmedAt,
		DeliveredAt:    m.DeliveredAt,
	}
}

// GetSupportedBridges returns bridges for a chain.
func (s *BridgeService) GetSupportedBridges(chainID int64) []BridgeConfig {
	if bridges, ok := s.supportedBridges[chainID]; ok {
		return bridges
	}
	return []BridgeConfig{}
}

// getSymbolFromAddress resolves a token address to its symbol from the shared
// chain registry, keyed by the source chain, so a bridge quote prices the
// token that is actually being bridged rather than a hardcoded list.
func (s *BridgeService) getSymbolFromAddress(chainID int64, address string) string {
	if symbol, ok := config.TokenSymbol(chainID, address); ok {
		return symbol
	}
	return ""
}

// API Handlers

func (s *BridgeService) GetQuoteHandler(c *gin.Context) {
	var req struct {
		SrcChain int64  `json:"src_chain" binding:"required"`
		DstChain int64  `json:"dst_chain" binding:"required"`
		Token    string `json:"token" binding:"required"`
		Amount   string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	quote, err := s.GetQuote(c.Request.Context(), req.SrcChain, req.DstChain, req.Token, req.Amount)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, quote)
}

// userWalletAddress resolves the custody wallet address bound to the
// authenticated caller. Bridge identity is the caller's own custody wallet, so
// a client can never claim another address as the sender of a transfer.
func (s *BridgeService) userWalletAddress(userID string) (string, error) {
	if s.db == nil {
		return "", fmt.Errorf("database not configured")
	}
	var wallet models.MasterWallet
	if err := s.db.Where("user_id = ?", userID).First(&wallet).Error; err != nil {
		return "", fmt.Errorf("no custody wallet for this account")
	}
	if !common.IsHexAddress(wallet.Address) {
		return "", fmt.Errorf("custody wallet address is not a valid address")
	}
	return wallet.Address, nil
}

// InitiateTransferHandler locks the caller's tokens on the source chain.
func (s *BridgeService) InitiateTransferHandler(c *gin.Context) {
	var req struct {
		SrcChain  int64  `json:"src_chain" binding:"required"`
		DstChain  int64  `json:"dst_chain" binding:"required"`
		Recipient string `json:"recipient" binding:"required"`
		Token     string `json:"token" binding:"required"`
		Amount    string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	sender, err := s.userWalletAddress(userID)
	if err != nil {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	}

	transfer, err := s.InitiateTransfer(c.Request.Context(), req.SrcChain, req.DstChain, sender, req.Recipient, req.Token, req.Amount)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, transfer)
}

// GetTransferHandler returns one transfer, but only to a party of it.
func (s *BridgeService) GetTransferHandler(c *gin.Context) {
	transferID := c.Param("id")

	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	caller, err := s.userWalletAddress(userID)
	if err != nil {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	}

	transfer, err := s.GetTransfer(transferID)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	if !strings.EqualFold(transfer.Sender, caller) && !strings.EqualFold(transfer.Recipient, caller) {
		c.JSON(404, gin.H{"error": "transfer not found"})
		return
	}

	c.JSON(200, transfer)
}

// GetUserTransfersHandler returns the caller's own transfers.
func (s *BridgeService) GetUserTransfersHandler(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	caller, err := s.userWalletAddress(userID)
	if err != nil {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	}

	transfers := s.GetUserTransfers(caller)
	c.JSON(200, gin.H{
		"transfers": transfers,
		"count":     len(transfers),
	})
}

func (s *BridgeService) GetSupportedBridgesHandler(c *gin.Context) {
	chainID := c.Param("chain_id")

	var chainIDInt int64
	fmt.Sscanf(chainID, "%d", &chainIDInt)

	bridges := s.GetSupportedBridges(chainIDInt)
	c.JSON(200, gin.H{
		"chain_id": chainIDInt,
		"bridges":  bridges,
	})
}

func (s *BridgeService) GetSupportedChainsHandler(c *gin.Context) {
	chains := s.GetSupportedChains()
	c.JSON(200, gin.H{
		"chains": chains,
	})
}

// unused import guards
var (
	_ = ethereum.NotFound
	_ = sync.RWMutex{}
)
