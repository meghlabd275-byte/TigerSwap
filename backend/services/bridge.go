package services

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// BridgeService handles cross-chain bridging
type BridgeService struct {
	blockchain *BlockchainClient
	priceAgg  *PriceAggregator
	mu        sync.RWMutex
	
	// Active transfers
	transfers map[string]*BridgeTransfer // transferID -> transfer
	
	// Supported bridges
	supportedBridges map[int64][]BridgeConfig
}

// BridgeTransfer represents a bridge transfer
type BridgeTransfer struct {
	ID              string    `json:"id"`
	SrcChain        int64     `json:"src_chain"`
	DstChain        int64     `json:"dst_chain"`
	Sender          string    `json:"sender"`
	Recipient       string    `json:"recipient"`
	Token           string    `json:"token"`
	Amount          string    `json:"amount"`
	SrcTxHash       string    `json:"src_tx_hash"`
	DstTxHash       string    `json:"dst_tx_hash"`
	Status          string    `json:"status"` // pending, confirmed, delivered, failed
	Fee             string    `json:"fee"`
	FeeUSD          float64   `json:"fee_usd"`
	AmountReceived  string    `json:"amount_received"`
	CreatedAt       time.Time `json:"created_at"`
	ConfirmedAt     *time.Time `json:"confirmed_at"`
	DeliveredAt     *time.Time `json:"delivered_at"`
}

// BridgeConfig represents bridge configuration
type BridgeConfig struct {
	Name       string `json:"name"`
	Router     string `json:"router"`
	ChainAgnostic bool `json:"chain_agnostic"`
	AvgTime    int   `json:"avg_time_minutes"`
	FeePercent float64 `json:"fee_percent"`
}

// Quote represents bridge quote
type BridgeQuote struct {
	SrcChain       int64   `json:"src_chain"`
	DstChain       int64   `json:"dst_chain"`
	Token          string  `json:"token"`
	AmountIn       string  `json:"amount_in"`
	AmountOut      string  `json:"amount_out"`
	Fee            string  `json:"fee"`
	FeeUSD         float64 `json:"fee_usd"`
	EstimatedTime  string  `json:"estimated_time"`
	Router         string  `json:"router"`
	PriceImpact    float64 `json:"price_impact"`
}

func NewBridgeService(blockchain *BlockchainClient, priceAgg *PriceAggregator) *BridgeService {
	return &BridgeService{
		blockchain: blockchain,
		priceAgg:   priceAgg,
		transfers:  make(map[string]*BridgeTransfer),
		supportedBridges: map[int64][]BridgeConfig{
			1: { // Ethereum
				{Name: "LayerZero", Router: "0x0000000000000000000000000000000000000001", ChainAgnostic: true, AvgTime: 15, FeePercent: 0.1},
				{Name: "Wormhole", Router: "0x0000000000000000000000000000000000000002", ChainAgnostic: true, AvgTime: 20, FeePercent: 0.15},
				{Name: "Axelar", Router: "0x0000000000000000000000000000000000000003", ChainAgnostic: true, AvgTime: 25, FeePercent: 0.12},
			},
			137: { // Polygon
				{Name: "LayerZero", Router: "0x0000000000000000000000000000000000000001", ChainAgnostic: true, AvgTime: 10, FeePercent: 0.08},
				{Name: "Wormhole", Router: "0x0000000000000000000000000000000000000002", ChainAgnostic: true, AvgTime: 15, FeePercent: 0.1},
			},
			56: { // BSC
				{Name: "LayerZero", Router: "0x0000000000000000000000000000000000000001", ChainAgnostic: true, AvgTime: 12, FeePercent: 0.08},
				{Name: "Wormhole", Router: "0x0000000000000000000000000000000000000002", ChainAgnostic: true, AvgTime: 18, FeePercent: 0.12},
			},
			42161: { // Arbitrum
				{Name: "LayerZero", Router: "0x0000000000000000000000000000000000000001", ChainAgnostic: true, AvgTime: 15, FeePercent: 0.1},
			},
		},
	}
}

// GetSupportedChains returns chains supported for bridging
func (s *BridgeService) GetSupportedChains() []int64 {
	return []int64{1, 137, 56, 42161, 10, 8453, 43114, 250}
}

// GetQuote returns bridge quote
func (s *BridgeService) GetQuote(ctx context.Context, srcChain, dstChain int64, token, amount string) (*BridgeQuote, error) {
	// Get token price
	tokenSymbol := s.getSymbolFromAddress(token)
	priceData, _ := s.priceAgg.GetRealPrice(ctx, tokenSymbol)
	
	amountFloat := parseFloat(amount)
	amountUSD := amountFloat * priceData.USD
	
	// Get fee (varies by bridge)
	feePercent := 0.1 // 0.1% default
	feeUSD := amountUSD * feePercent / 100
	fee := feeUSD / priceData.USD
	
	amountOut := amountFloat - fee
	
	// Get bridge config
	bridges := s.supportedBridges[srcChain]
	avgTime := 15
	if len(bridges) > 0 {
		avgTime = bridges[0].AvgTime
	}
	
	return &BridgeQuote{
		SrcChain:      srcChain,
		DstChain:      dstChain,
		Token:         token,
		AmountIn:      amount,
		AmountOut:     fmt.Sprintf("%.8f", amountOut),
		Fee:           fmt.Sprintf("%.8f", fee),
		FeeUSD:        feeUSD,
		EstimatedTime: fmt.Sprintf("%d minutes", avgTime),
		Router:        bridges[0].Router,
		PriceImpact:   0.01, // 0.01% impact
	}, nil
}

// InitiateTransfer initiates a bridge transfer
func (s *BridgeService) InitiateTransfer(ctx context.Context, srcChain, dstChain int64, sender, recipient, token, amount string) (*BridgeTransfer, error) {
	// Get quote first
	quote, err := s.GetQuote(ctx, srcChain, dstChain, token, amount)
	if err != nil {
		return nil, err
	}
	
	transfer := &BridgeTransfer{
		ID:         fmt.Sprintf("bridge_%d", time.Now().UnixNano()),
		SrcChain:   srcChain,
		DstChain:   dstChain,
		Sender:     sender,
		Recipient:  recipient,
		Token:      token,
		Amount:     amount,
		Status:     "pending",
		Fee:        quote.Fee,
		FeeUSD:     quote.FeeUSD,
		AmountReceived: quote.AmountOut,
		CreatedAt: time.Now(),
	}
	
	s.mu.Lock()
	s.transfers[transfer.ID] = transfer
	s.mu.Unlock()
	
	// Simulate source chain transaction
	// In production, this would call the bridge router contract
	transfer.SrcTxHash = fmt.Sprintf("0x%x", big.NewInt(time.Now().UnixNano()))
	
	// Update status to confirmed
	now := time.Now()
	transfer.Status = "confirmed"
	transfer.ConfirmedAt = &now
	
	// In production, would wait for confirmation and then execute destination transaction
	// For simulation, mark as delivered after delay
	
	return transfer, nil
}

// GetTransfer returns transfer status
func (s *BridgeService) GetTransfer(transferID string) (*BridgeTransfer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	transfer, exists := s.transfers[transferID]
	if !exists {
		return nil, fmt.Errorf("transfer not found")
	}
	
	return transfer, nil
}

// GetUserTransfers returns all transfers for a user
func (s *BridgeService) GetUserTransfers(userAddress string) []*BridgeTransfer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []*BridgeTransfer
	for _, transfer := range s.transfers {
		if transfer.Sender == userAddress || transfer.Recipient == userAddress {
			result = append(result, transfer)
		}
	}
	
	return result
}

// SimulateConfirmation simulates bridge confirmation
func (s *BridgeService) SimulateConfirmation(transferID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	transfer, exists := s.transfers[transferID]
	if !exists {
		return fmt.Errorf("transfer not found")
	}
	
	if transfer.Status != "confirmed" {
		return fmt.Errorf("transfer not confirmed yet")
	}
	
	// Simulate destination transaction
	transfer.DstTxHash = fmt.Sprintf("0x%x", big.NewInt(time.Now().UnixNano()))
	
	now := time.Now()
	transfer.Status = "delivered"
	transfer.DeliveredAt = &now
	
	return nil
}

// GetSupportedBridges returns bridges for a chain
func (s *BridgeService) GetSupportedBridges(chainID int64) []BridgeConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	if bridges, ok := s.supportedBridges[chainID]; ok {
		return bridges
	}
	
	return []BridgeConfig{}
}

func (s *BridgeService) getSymbolFromAddress(address string) string {
	symbols := map[string]string{
		"0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48": "USDC",
		"0xdAC17F958D2ee523a2206206994597C13D831ec7": "USDT",
		"0x0000000000000000000000000000000000000000": "ETH",
		"0x2260FAC5E5542a773Aa44fBCfeDf7C193bc2C599": "WBTC",
	}
	
	for addr, symbol := range symbols {
		if strings.EqualFold(addr, address) {
			return symbol
		}
	}
	
	return "UNKNOWN"
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
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, quote)
}

func (s *BridgeService) InitiateTransferHandler(c *gin.Context) {
	var req struct {
		SrcChain  int64  `json:"src_chain" binding:"required"`
		DstChain  int64  `json:"dst_chain" binding:"required"`
		Sender    string `json:"sender" binding:"required"`
		Recipient string `json:"recipient" binding:"required"`
		Token     string `json:"token" binding:"required"`
		Amount    string `json:"amount" binding:"required"`
	}
	
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	
	transfer, err := s.InitiateTransfer(c.Request.Context(), req.SrcChain, req.DstChain, req.Sender, req.Recipient, req.Token, req.Amount)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, transfer)
}

func (s *BridgeService) GetTransferHandler(c *gin.Context) {
	transferID := c.Param("id")
	
	transfer, err := s.GetTransfer(transferID)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}
	
	c.JSON(200, transfer)
}

func (s *BridgeService) GetUserTransfersHandler(c *gin.Context) {
	userAddress := c.Query("user_address")
	
	transfers := s.GetUserTransfers(userAddress)
	c.JSON(200, gin.H{
		"transfers": transfers,
		"count":    len(transfers),
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
