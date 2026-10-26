package services

import (
	"context"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// PerpetualEngine handles real perpetual trading
type PerpetualEngine struct {
	blockchain     *BlockchainClient
	priceAgg       *PriceAggregator
	positions      map[string]*PerpetualPosition // positionID -> position
	orders         map[string]*PerpetualOrder
	mu             sync.RWMutex
	supportedPairs []string
}

// PerpetualPosition represents a real perpetual position
type PerpetualPosition struct {
	ID               string     `json:"id"`
	UserAddress      string     `json:"user_address"`
	Pair             string     `json:"pair"`       // BTC-USD, ETH-USD
	Side             string     `json:"side"`       // long, short
	Size             string     `json:"size"`       // Position size in USD
	Collateral       string     `json:"collateral"` // Margin deposited
	EntryPrice       float64    `json:"entry_price"`
	MarkPrice        float64    `json:"mark_price"`
	IndexPrice       float64    `json:"index_price"`
	LiquidationPrice float64    `json:"liquidation_price"`
	Leverage         int        `json:"leverage"`
	UnrealizedPNL    string     `json:"unrealized_pnl"`
	FundingPaid      string     `json:" funding_paid"`
	Status           string     `json:"status"` // open, closed, liquidated
	OpenedAt         time.Time  `json:"opened_at"`
	ClosedAt         *time.Time `json:"closed_at"`
	BlockNumber      uint64     `json:"block_number"`
}

// PerpetualOrder represents a perpetual order
type PerpetualOrder struct {
	ID           string    `json:"id"`
	UserAddress  string    `json:"user_address"`
	Pair         string    `json:"pair"`
	Side         string    `json:"side"`
	OrderType    string    `json:"order_type"` // market, limit
	Size         string    `json:"size"`
	Price        float64   `json:"price"`
	TriggerPrice float64   `json:"trigger_price"`
	Status       string    `json:"status"` // pending, filled, cancelled
	FilledSize   string    `json:"filled_size"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// FundingInfo tracks funding rates
type FundingInfo struct {
	Pair            string    `json:"pair"`
	LastFundingRate float64   `json:"last_funding_rate"`
	NextFundingTime time.Time `json:"next_funding_time"`
	InterestRate    float64   `json:"interest_rate"`
}

func NewPerpetualEngine(blockchain *BlockchainClient, priceAgg *PriceAggregator) *PerpetualEngine {
	return &PerpetualEngine{
		blockchain:     blockchain,
		priceAgg:       priceAgg,
		positions:      make(map[string]*PerpetualPosition),
		orders:         make(map[string]*PerpetualOrder),
		supportedPairs: []string{"BTC-USD", "ETH-USD", "SOL-USD", "AVAX-USD", "LINK-USD"},
	}
}

// OpenPosition opens a new perpetual position. The position is opened against
// the engine's own mark and index prices; the caller's on-chain settlement is
// broadcast separately with a client-signed transaction.
func (e *PerpetualEngine) OpenPosition(ctx context.Context, chainID int64, pair, side, size, collateral string, leverage int) (*PerpetualPosition, error) {
	// Validate inputs
	if !e.isValidPair(pair) {
		return nil, fmt.Errorf("unsupported pair: %s", pair)
	}

	// Get current mark price
	markPrice, err := e.getMarkPrice(ctx, pair)
	if err != nil {
		return nil, fmt.Errorf("failed to get mark price: %v", err)
	}

	// Get index price
	indexPrice, err := e.getIndexPrice(ctx, pair)
	if err != nil {
		return nil, fmt.Errorf("failed to get index price: %v", err)
	}

	// Calculate liquidation price
	liquidationPrice := e.calculateLiquidationPrice(markPrice, leverage, side == "long")

	// Validate numeric inputs and enforce leverage limits.
	sizeFloat := parseFloat(size)
	collateralFloat := parseFloat(collateral)
	if sizeFloat <= 0 || collateralFloat <= 0 {
		return nil, fmt.Errorf("size and collateral must be positive")
	}
	if markPrice <= 0 {
		return nil, fmt.Errorf("no mark price available for %s", pair)
	}
	if leverage < 1 || leverage > 1001 {
		return nil, fmt.Errorf("leverage must be between 1 and 1001")
	}
	if requiredMargin := sizeFloat / float64(leverage); collateralFloat+1e-9 < requiredMargin {
		return nil, fmt.Errorf("insufficient collateral: need %.8f for %dx on size %.8f", requiredMargin, leverage, sizeFloat)
	}

	position := &PerpetualPosition{
		ID:               fmt.Sprintf("pos_%d", time.Now().UnixNano()),
		UserAddress:      "", // Would derive from private key
		Pair:             pair,
		Side:             side,
		Size:             size,
		Collateral:       collateral,
		EntryPrice:       markPrice,
		MarkPrice:        markPrice,
		IndexPrice:       indexPrice,
		LiquidationPrice: liquidationPrice,
		Leverage:         leverage,
		UnrealizedPNL:    "0",
		FundingPaid:      "0",
		Status:           "open",
		OpenedAt:         time.Now(),
		BlockNumber:      0,
	}

	// Store position
	e.mu.Lock()
	e.positions[position.ID] = position
	e.mu.Unlock()

	return position, nil
}

// ClosePosition closes an existing position
func (e *PerpetualEngine) ClosePosition(ctx context.Context, positionID string) (*PerpetualPosition, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	position, exists := e.positions[positionID]
	if !exists {
		return nil, fmt.Errorf("position not found: %s", positionID)
	}

	// Get current price
	markPrice, err := e.getMarkPrice(ctx, position.Pair)
	if err != nil {
		return nil, err
	}

	// Calculate PNL
	pnl := e.calculatePNL(position, markPrice)

	now := time.Now()
	position.Status = "closed"
	position.ClosedAt = &now
	position.MarkPrice = markPrice
	position.UnrealizedPNL = pnl

	return position, nil
}

// GetPosition returns a specific position
func (e *PerpetualEngine) GetPosition(positionID string) (*PerpetualPosition, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	position, exists := e.positions[positionID]
	if !exists {
		return nil, fmt.Errorf("position not found")
	}

	return position, nil
}

// GetUserPositions returns all positions for a user
func (e *PerpetualEngine) GetUserPositions(userAddress string) []*PerpetualPosition {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var userPositions []*PerpetualPosition
	for _, pos := range e.positions {
		if pos.UserAddress == userAddress && pos.Status == "open" {
			userPositions = append(userPositions, pos)
		}
	}

	return userPositions
}

// UpdatePositions updates all positions with current prices and checks liquidation
func (e *PerpetualEngine) UpdatePositions(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, position := range e.positions {
		if position.Status != "open" {
			continue
		}

		// Get current mark price
		markPrice, err := e.getMarkPrice(ctx, position.Pair)
		if err != nil {
			continue
		}

		position.MarkPrice = markPrice

		// Check liquidation
		if e.isLiquidatable(position) {
			position.Status = "liquidated"
			// In real implementation, would trigger liquidation transaction
		} else {
			// Update unrealized PNL
			position.UnrealizedPNL = e.calculatePNL(position, markPrice)
		}
	}

	return nil
}

// CreateOrder creates a new perpetual order
func (e *PerpetualEngine) CreateOrder(ctx context.Context, userAddress, pair, side, orderType, size string, price, triggerPrice float64, expiresIn int) (*PerpetualOrder, error) {
	if !e.isValidPair(pair) {
		return nil, fmt.Errorf("unsupported pair: %s", pair)
	}

	order := &PerpetualOrder{
		ID:           fmt.Sprintf("ord_%d", time.Now().UnixNano()),
		UserAddress:  userAddress,
		Pair:         pair,
		Side:         side,
		OrderType:    orderType,
		Size:         size,
		Price:        price,
		TriggerPrice: triggerPrice,
		Status:       "pending",
		FilledSize:   "0",
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(time.Duration(expiresIn) * time.Second),
	}

	e.mu.Lock()
	e.orders[order.ID] = order
	e.mu.Unlock()

	// If market order, execute immediately
	if orderType == "market" {
		return e.executeOrder(ctx, order)
	}

	return order, nil
}

// CancelOrder cancels an order
func (e *PerpetualEngine) CancelOrder(orderID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	order, exists := e.orders[orderID]
	if !exists {
		return fmt.Errorf("order not found")
	}

	if order.Status != "pending" {
		return fmt.Errorf("order cannot be cancelled")
	}

	order.Status = "cancelled"
	return nil
}

func (e *PerpetualEngine) executeOrder(ctx context.Context, order *PerpetualOrder) (*PerpetualOrder, error) {
	// Get current price
	markPrice, err := e.getMarkPrice(ctx, order.Pair)
	if err != nil {
		return nil, err
	}

	// Execute at mark price
	order.Status = "filled"
	order.FilledSize = order.Size
	order.Price = markPrice

	return order, nil
}

func (e *PerpetualEngine) getMarkPrice(ctx context.Context, pair string) (float64, error) {
	// Extract base asset from pair (e.g., BTC from BTC-USD)
	base := strings.Split(pair, "-")[0]

	// Get price from aggregator
	priceData, err := e.priceAgg.GetRealPrice(ctx, base)
	if err != nil {
		return 0, err
	}

	return priceData.USD, nil
}

func (e *PerpetualEngine) getIndexPrice(ctx context.Context, pair string) (float64, error) {
	// Index price is same as mark price in this implementation
	// In production, would use different oracle
	return e.getMarkPrice(ctx, pair)
}

func (e *PerpetualEngine) calculateLiquidationPrice(entryPrice float64, leverage int, isLong bool) float64 {
	// Liquidation price = entryPrice * (1 - 1/leverage * 0.5)
	// 50% buffer of the position value
	liquidationThreshold := 1.0 / float64(leverage) * 0.5

	if isLong {
		return entryPrice * (1 - liquidationThreshold)
	}
	return entryPrice * (1 + liquidationThreshold)
}

func (e *PerpetualEngine) isLiquidatable(position *PerpetualPosition) bool {
	if position.Side == "long" {
		return position.MarkPrice <= position.LiquidationPrice
	}
	return position.MarkPrice >= position.LiquidationPrice
}

func (e *PerpetualEngine) calculatePNL(position *PerpetualPosition, currentPrice float64) string {
	size := parseFloat(position.Size)
	entryPrice := position.EntryPrice

	var pnl float64
	if position.Side == "long" {
		pnl = (currentPrice - entryPrice) * size / entryPrice * float64(position.Leverage)
	} else {
		pnl = (entryPrice - currentPrice) * size / entryPrice * float64(position.Leverage)
	}

	return fmt.Sprintf("%.8f", pnl)
}

func (e *PerpetualEngine) isValidPair(pair string) bool {
	for _, p := range e.supportedPairs {
		if p == pair {
			return true
		}
	}
	return false
}

// GetFundingRate calculates the funding rate for a pair
func (e *PerpetualEngine) GetFundingRate(ctx context.Context, pair string) (*FundingInfo, error) {
	markPrice, err := e.getMarkPrice(ctx, pair)
	if err != nil {
		return nil, err
	}

	indexPrice, err := e.getIndexPrice(ctx, pair)
	if err != nil {
		return nil, err
	}

	// Calculate premium
	premium := (markPrice - indexPrice) / indexPrice * 100

	// Interest rate (typically 0.01% per 8 hours)
	interestRate := 0.01

	// Funding rate = premium + interest (clamped)
	fundingRate := premium + interestRate
	if fundingRate > 1 {
		fundingRate = 1
	} else if fundingRate < -1 {
		fundingRate = -1
	}

	return &FundingInfo{
		Pair:            pair,
		LastFundingRate: fundingRate,
		NextFundingTime: e.nextFundingTime(),
		InterestRate:    interestRate,
	}, nil
}

func (e *PerpetualEngine) nextFundingTime() time.Time {
	// Funding happens every 8 hours
	now := time.Now()
	next := now.Truncate(8 * time.Hour).Add(8 * time.Hour)
	if now.After(next) {
		next = next.Add(8 * time.Hour)
	}
	return next
}

// API Handlers

// OpenPositionHandler broadcasts a client-signed position-opening transaction.
// The server never receives a private key.
func (e *PerpetualEngine) OpenPositionHandler(c *gin.Context) {
	var req struct {
		ChainID    int64  `json:"chain_id" binding:"required"`
		SignedTx   string `json:"signed_tx" binding:"required"`
		Pair       string `json:"pair" binding:"required"`
		Side       string `json:"side" binding:"required"`
		Size       string `json:"size" binding:"required"`
		Collateral string `json:"collateral" binding:"required"`
		Leverage   int    `json:"leverage" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	txHash, err := e.BroadcastTransaction(c.Request.Context(), req.ChainID, req.SignedTx)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "tx_hash": txHash, "status": "submitted"})
}

// BroadcastTransaction submits a client-signed transaction for this engine.
func (e *PerpetualEngine) BroadcastTransaction(ctx context.Context, chainID int64, signedTxHex string) (string, error) {
	client, err := e.blockchain.GetClient(chainID)
	if err != nil {
		return "", err
	}
	return broadcastSignedTransaction(ctx, client, signedTxHex)
}

func (e *PerpetualEngine) ClosePositionHandler(c *gin.Context) {
	positionID := c.Param("id")

	position, err := e.ClosePosition(c.Request.Context(), positionID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, position)
}

func (e *PerpetualEngine) GetPositionsHandler(c *gin.Context) {
	userAddress := c.Query("user_address")

	positions := e.GetUserPositions(userAddress)

	c.JSON(200, gin.H{
		"positions": positions,
		"count":     len(positions),
	})
}

func (e *PerpetualEngine) CreateOrderHandler(c *gin.Context) {
	var req struct {
		UserAddress  string  `json:"user_address" binding:"required"`
		Pair         string  `json:"pair" binding:"required"`
		Side         string  `json:"side" binding:"required"`
		OrderType    string  `json:"order_type" binding:"required"`
		Size         string  `json:"size" binding:"required"`
		Price        float64 `json:"price"`
		TriggerPrice float64 `json:"trigger_price"`
		ExpiresIn    int     `json:"expires_in"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	expiresIn := req.ExpiresIn
	if expiresIn == 0 {
		expiresIn = 3600 // 1 hour default
	}

	order, err := e.CreateOrder(c.Request.Context(), req.UserAddress, req.Pair, req.Side, req.OrderType, req.Size, req.Price, req.TriggerPrice, expiresIn)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, order)
}

func (e *PerpetualEngine) GetFundingRateHandler(c *gin.Context) {
	pair := c.Param("pair")

	funding, err := e.GetFundingRate(c.Request.Context(), pair)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, funding)
}

func (e *PerpetualEngine) GetSupportedPairsHandler(c *gin.Context) {
	c.JSON(200, gin.H{
		"pairs": e.supportedPairs,
	})
}

// PerpMarketView is the public description of a tradable perpetual market.
type PerpMarketView struct {
	MarketID          string  `json:"marketId"`
	Symbol            string  `json:"symbol"`
	MarkPrice         float64 `json:"markPrice"`
	IndexPrice        float64 `json:"indexPrice"`
	FundingRate       float64 `json:"fundingRate"`
	MaxLeverage       int     `json:"maxLeverage"`
	OpenInterestLong  float64 `json:"openInterestLong"`
	OpenInterestShort float64 `json:"openInterestShort"`
	IsActive          bool    `json:"isActive"`
}

// maxPerpLeverage caps leverage for engine-priced perpetual markets.
const maxPerpLeverage = 100

// GetMarketsHandler lists the engine's markets with live mark/index prices and
// funding. A market whose price feed is unavailable is still returned but is
// marked inactive, so a client never trades against a stale price.
func (e *PerpetualEngine) GetMarketsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	markets := make([]PerpMarketView, 0, len(e.supportedPairs))

	for _, pair := range e.supportedPairs {
		view := PerpMarketView{
			MarketID:    pair,
			Symbol:      pair,
			MaxLeverage: maxPerpLeverage,
		}

		markPrice, err := e.getMarkPrice(ctx, pair)
		if err != nil {
			markets = append(markets, view)
			continue
		}

		view.MarkPrice = markPrice
		view.IndexPrice = markPrice
		view.IsActive = true

		if funding, err := e.GetFundingRate(ctx, pair); err == nil {
			view.FundingRate = funding.LastFundingRate
		}

		e.mu.RLock()
		for _, position := range e.positions {
			if position.Pair != pair || position.Status != "open" {
				continue
			}
			size, ok := new(big.Float).SetString(position.Size)
			if !ok {
				continue
			}
			notional, _ := size.Float64()
			if position.Side == "short" {
				view.OpenInterestShort += notional
			} else {
				view.OpenInterestLong += notional
			}
		}
		e.mu.RUnlock()

		markets = append(markets, view)
	}

	c.JSON(200, gin.H{"markets": markets})
}
