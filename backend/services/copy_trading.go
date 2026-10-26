package services

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
)

// CopyTradingEngine handles real copy trading
type CopyTradingEngine struct {
	blockchain *BlockchainClient
	priceAgg   *PriceAggregator
	mu         sync.RWMutex

	// Trader registry
	traders map[string]*Trader // address -> trader info

	// Follow relationships
	followers map[string][]string // trader -> []follower
	following map[string]string   // follower -> trader

	// Copy positions
	copyPositions map[string]*CopyPosition // positionID -> position
	orders        map[string]*CopyOrder
}

// Trader represents a trader that can be copied
type Trader struct {
	Address        string    `json:"address"`
	Name           string    `json:"name"`
	Avatar         string    `json:"avatar"`
	WinRate        float64   `json:"win_rate"`
	TotalProfit    string    `json:"total_profit"`
	WeeklyProfit   float64   `json:"weekly_profit"`
	MonthlyProfit  float64   `json:"monthly_profit"`
	TotalTrades    int       `json:"total_trades"`
	Followers      int       `json:"followers"`
	TotalVolume    string    `json:"total_volume"`
	AvgHoldingTime string    `json:"avg_holding_time"`
	IsVerified     bool      `json:"is_verified"`
	IsPro          bool      `json:"is_pro"`
	RiskScore      string    `json:"risk_score"` // low, medium, high
	Strategies     []string  `json:"strategies"`
	LastActive     time.Time `json:"last_active"`
}

// CopyPosition represents a copied position
type CopyPosition struct {
	ID               string     `json:"id"`
	FollowerAddress  string     `json:"follower_address"`
	TraderAddress    string     `json:"trader_address"`
	OriginalPosition string     `json:"original_position"`
	Pair             string     `json:"pair"`
	Side             string     `json:"side"`
	Size             string     `json:"size"`
	EntryPrice       float64    `json:"entry_price"`
	CurrentPrice     float64    `json:"current_price"`
	Leverage         int        `json:"leverage"`
	PNL              string     `json:"pnl"`
	Status           string     `json:"status"` // open, closed
	OpenedAt         time.Time  `json:"opened_at"`
	ClosedAt         *time.Time `json:"closed_at"`
	CopiedAt         time.Time  `json:"copied_at"`
}

// CopyOrder represents a copy trade order
type CopyOrder struct {
	ID              string     `json:"id"`
	FollowerAddress string     `json:"follower_address"`
	TraderAddress   string     `json:"trader_address"`
	TraderOrderID   string     `json:"trader_order_id"`
	Pair            string     `json:"pair"`
	Side            string     `json:"side"`
	Size            string     `json:"size"`
	Status          string     `json:"status"` // pending, executed, failed
	CreatedAt       time.Time  `json:"created_at"`
	ExecutedAt      *time.Time `json:"executed_at"`
}

// CopyConfig represents copy trading configuration
type CopyConfig struct {
	FollowerAddress string  `json:"follower_address"`
	TraderAddress   string  `json:"trader_address"`
	CopyRatio       float64 `json:"copy_ratio"` // 0.1 - 1.0 (10% - 100%)
	AutoClose       bool    `json:"auto_close"`
	StopLoss        float64 `json:"stop_loss"`   // percentage
	TakeProfit      float64 `json:"take_profit"` // percentage
}

func NewCopyTradingEngine(blockchain *BlockchainClient, priceAgg *PriceAggregator) *CopyTradingEngine {
	return &CopyTradingEngine{
		blockchain:    blockchain,
		priceAgg:      priceAgg,
		traders:       make(map[string]*Trader),
		followers:     make(map[string][]string),
		following:     make(map[string]string),
		copyPositions: make(map[string]*CopyPosition),
		orders:        make(map[string]*CopyOrder),
	}
}

// RegisterTrader registers a new trader
func (e *CopyTradingEngine) RegisterTrader(address, name, avatar string) *Trader {
	e.mu.Lock()
	defer e.mu.Unlock()

	trader := &Trader{
		Address:       address,
		Name:          name,
		Avatar:        avatar,
		WinRate:       0,
		TotalProfit:   "0",
		WeeklyProfit:  0,
		MonthlyProfit: 0,
		TotalTrades:   0,
		Followers:     0,
		TotalVolume:   "0",
		IsVerified:    false,
		IsPro:         false,
		RiskScore:     "medium",
		LastActive:    time.Now(),
	}

	e.traders[address] = trader
	return trader
}

// GetTrader returns trader info
func (e *CopyTradingEngine) GetTrader(address string) (*Trader, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	trader, exists := e.traders[address]
	if !exists {
		return nil, fmt.Errorf("trader not found")
	}

	return trader, nil
}

// GetTopTraders returns top traders
func (e *CopyTradingEngine) GetTopTraders(limit int) []*Trader {
	e.mu.RLock()
	defer e.mu.RUnlock()

	traders := make([]*Trader, 0, len(e.traders))
	for _, trader := range e.traders {
		traders = append(traders, trader)
	}

	sort.SliceStable(traders, func(i, j int) bool {
		if traders[i].WeeklyProfit != traders[j].WeeklyProfit {
			return traders[i].WeeklyProfit > traders[j].WeeklyProfit
		}
		return traders[i].TotalTrades > traders[j].TotalTrades
	})

	if limit > 0 && len(traders) > limit {
		traders = traders[:limit]
	}

	return traders
}

// StartFollowing starts following a trader
func (e *CopyTradingEngine) StartFollowing(followerAddress, traderAddress string, config CopyConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Verify trader exists
	_, exists := e.traders[traderAddress]
	if !exists {
		return fmt.Errorf("trader not found")
	}

	// Check if already following
	if _, alreadyFollowing := e.following[followerAddress]; alreadyFollowing {
		return fmt.Errorf("already following a trader")
	}

	// Add follow relationship
	e.following[followerAddress] = traderAddress

	if e.followers[traderAddress] == nil {
		e.followers[traderAddress] = []string{}
	}
	e.followers[traderAddress] = append(e.followers[traderAddress], followerAddress)

	// Update follower count
	e.traders[traderAddress].Followers++

	return nil
}

// StopFollowing stops following a trader
func (e *CopyTradingEngine) StopFollowing(followerAddress string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	traderAddress, exists := e.following[followerAddress]
	if !exists {
		return fmt.Errorf("not following any trader")
	}

	// Remove from followers list
	for i, follower := range e.followers[traderAddress] {
		if follower == followerAddress {
			e.followers[traderAddress] = append(e.followers[traderAddress][:i], e.followers[traderAddress][i+1:]...)
			break
		}
	}

	// Update follower count
	e.traders[traderAddress].Followers--

	// Remove following
	delete(e.following, followerAddress)

	return nil
}

// CopyTrade copies a trade from trader to follower
func (e *CopyTradingEngine) CopyTrade(ctx context.Context, followerAddress, traderAddress, pair, side, size, originalPositionID string) (*CopyPosition, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Verify following relationship
	followingTrader, exists := e.following[followerAddress]
	if !exists || followingTrader != traderAddress {
		return nil, fmt.Errorf("not following this trader")
	}

	// Get current price
	price, err := e.getPriceForPair(ctx, pair)
	if err != nil {
		return nil, err
	}

	position := &CopyPosition{
		ID:               fmt.Sprintf("cp_%d", time.Now().UnixNano()),
		FollowerAddress:  followerAddress,
		TraderAddress:    traderAddress,
		OriginalPosition: originalPositionID,
		Pair:             pair,
		Side:             side,
		Size:             size,
		EntryPrice:       price,
		CurrentPrice:     price,
		Leverage:         1,
		PNL:              "0",
		Status:           "open",
		OpenedAt:         time.Now(),
		CopiedAt:         time.Now(),
	}

	e.copyPositions[position.ID] = position

	return position, nil
}

// UpdateCopyPositions updates all copy positions with current prices
func (e *CopyTradingEngine) UpdateCopyPositions(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, position := range e.copyPositions {
		if position.Status != "open" {
			continue
		}

		price, err := e.getPriceForPair(ctx, position.Pair)
		if err != nil {
			continue
		}

		position.CurrentPrice = price
		position.PNL = e.calculateCopyPNL(position)
	}

	return nil
}

// CloseCopyPosition closes a copy position
func (e *CopyTradingEngine) CloseCopyPosition(positionID string) (*CopyPosition, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	position, exists := e.copyPositions[positionID]
	if !exists {
		return nil, fmt.Errorf("position not found")
	}

	if position.Status != "open" {
		return nil, fmt.Errorf("position already closed")
	}

	now := time.Now()
	position.Status = "closed"
	position.ClosedAt = &now

	return position, nil
}

// GetFollowerPositions returns all copy positions for a follower
func (e *CopyTradingEngine) GetFollowerPositions(followerAddress string) []*CopyPosition {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var positions []*CopyPosition
	for _, pos := range e.copyPositions {
		if pos.FollowerAddress == followerAddress && pos.Status == "open" {
			positions = append(positions, pos)
		}
	}

	return positions
}

func (e *CopyTradingEngine) getPriceForPair(ctx context.Context, pair string) (float64, error) {
	base := strings.Split(pair, "/")[0]
	priceData, err := e.priceAgg.GetRealPrice(ctx, base)
	if err != nil {
		return 0, err
	}
	return priceData.USD, nil
}

func (e *CopyTradingEngine) calculateCopyPNL(position *CopyPosition) string {
	size := parseFloat(position.Size)

	var pnl float64
	if position.Side == "long" {
		pnl = (position.CurrentPrice - position.EntryPrice) * size / position.EntryPrice * float64(position.Leverage)
	} else {
		pnl = (position.EntryPrice - position.CurrentPrice) * size / position.EntryPrice * float64(position.Leverage)
	}

	return fmt.Sprintf("%.8f", pnl)
}

// API Handlers

// RegisterTraderHandler registers a trader after validating the address.
func (e *CopyTradingEngine) RegisterTraderHandler(c *gin.Context) {
	var req struct {
		Address string `json:"address" binding:"required"`
		Name    string `json:"name" binding:"required"`
		Avatar  string `json:"avatar"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !common.IsHexAddress(req.Address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid trader address"})
		return
	}
	if len(req.Name) > 64 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name must be at most 64 characters"})
		return
	}

	address := strings.ToLower(req.Address)
	trader := e.RegisterTrader(address, req.Name, req.Avatar)
	c.JSON(http.StatusOK, trader)
}

func (e *CopyTradingEngine) GetTraderHandler(c *gin.Context) {
	address := strings.ToLower(c.Param("address"))

	trader, err := e.GetTrader(address)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, trader)
}

func (e *CopyTradingEngine) GetTopTradersHandler(c *gin.Context) {
	limit := 10
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	traders := e.GetTopTraders(limit)
	c.JSON(http.StatusOK, gin.H{
		"traders": traders,
		"count":   len(traders),
	})
}

// StartFollowingHandler begins copying a trader for the authenticated caller.
func (e *CopyTradingEngine) StartFollowingHandler(c *gin.Context) {
	var req struct {
		TraderAddress string  `json:"trader_address" binding:"required"`
		CopyRatio     float64 `json:"copy_ratio"`
		StopLoss      float64 `json:"stop_loss"`
		TakeProfit    float64 `json:"take_profit"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// The follower is the authenticated account, never a client-supplied value.
	follower := c.GetString("user_id")
	if follower == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "authentication required"})
		return
	}
	if !common.IsHexAddress(req.TraderAddress) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid trader address"})
		return
	}
	if req.CopyRatio <= 0 || req.CopyRatio > 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "copy_ratio must be between 0 and 1"})
		return
	}

	config := CopyConfig{
		FollowerAddress: follower,
		TraderAddress:   strings.ToLower(req.TraderAddress),
		CopyRatio:       req.CopyRatio,
		AutoClose:       true,
		StopLoss:        req.StopLoss,
		TakeProfit:      req.TakeProfit,
	}

	if err := e.StartFollowing(follower, config.TraderAddress, config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Now following trader"})
}

// StopFollowingHandler stops copying for the authenticated caller.
func (e *CopyTradingEngine) StopFollowingHandler(c *gin.Context) {
	follower := c.GetString("user_id")
	if follower == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "authentication required"})
		return
	}

	if err := e.StopFollowing(follower); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Stopped following"})
}

// GetPositionsHandler returns the authenticated caller's copy positions.
func (e *CopyTradingEngine) GetPositionsHandler(c *gin.Context) {
	follower := c.GetString("user_id")
	if follower == "" {
		c.JSON(http.StatusForbidden, gin.H{"error": "authentication required"})
		return
	}

	positions := e.GetFollowerPositions(follower)
	c.JSON(http.StatusOK, gin.H{
		"positions": positions,
		"count":     len(positions),
	})
}

// ClosePositionHandler closes a copy position owned by the caller.
func (e *CopyTradingEngine) ClosePositionHandler(c *gin.Context) {
	positionID := c.Param("id")
	follower := c.GetString("user_id")

	position, err := e.CloseCopyPosition(positionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if follower == "" || !strings.EqualFold(position.FollowerAddress, follower) {
		c.JSON(http.StatusForbidden, gin.H{"error": "position belongs to another account"})
		return
	}

	c.JSON(http.StatusOK, position)
}
