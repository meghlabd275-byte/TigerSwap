package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// Sentinels for the copy-trading engine's caller-caused failures.
var (
	errTraderNotFound        = errors.New("trader not found")
	errAlreadyFollowing      = errors.New("already following a trader")
	errNotFollowing          = errors.New("not following any trader")
	errCopyPositionNotFound  = errors.New("copy position not found")
	errCopyPositionClosed    = errors.New("copy position already closed")
	errCopyPositionForbidden = errors.New("copy position belongs to another account")
	errNoWalletBound         = errors.New("no wallet is bound to this account")
)

// CopyTradingEngine handles real copy trading
type CopyTradingEngine struct {
	blockchain *BlockchainClient
	priceAgg   priceSource
	mu         sync.RWMutex

	// Trader registry
	traders map[string]*Trader // address -> trader info

	// Follow relationships
	followers map[string][]string // trader -> []follower
	following map[string]string   // follower -> trader

	// Copy positions
	copyPositions map[string]*CopyPosition // positionID -> position
	orders        map[string]*CopyOrder

	// Per-follower copy configuration, keyed by follower address. The ratio
	// scales every copied size and the stop/take levels are enforced when a
	// position is re-marked, so the values the follower supplied actually take
	// effect instead of being silently discarded.
	configs map[string]CopyConfig

	// db resolves the authenticated account to its bound wallet address. The
	// engine keys a follower by that address, so the HTTP handlers and the
	// wallet copy-trade path agree on one identity domain; keying one by the
	// account UUID and the other by the address would make a followed trader
	// impossible to copy from the wallet route.
	db *gorm.DB
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

func NewCopyTradingEngine(blockchain *BlockchainClient, priceAgg priceSource) *CopyTradingEngine {
	return &CopyTradingEngine{
		blockchain:    blockchain,
		priceAgg:      priceAgg,
		traders:       make(map[string]*Trader),
		followers:     make(map[string][]string),
		following:     make(map[string]string),
		copyPositions: make(map[string]*CopyPosition),
		orders:        make(map[string]*CopyOrder),
		configs:       make(map[string]CopyConfig),
	}
}

// SetDB attaches the database used to resolve an account's bound wallet. The
// engine stores a follower by wallet address, so it needs the account row to map
// the authenticated session to that address.
func (e *CopyTradingEngine) SetDB(db *gorm.DB) *CopyTradingEngine {
	e.db = db
	return e
}

// resolveFollowerAddress maps the authenticated account to its bound wallet
// address and fails closed when the account has none. Identity is never taken
// from the request, and the engine keys a follower by wallet address so the
// follow handlers and the wallet copy-trade path use one identity domain.
func (e *CopyTradingEngine) resolveFollowerAddress(c *gin.Context) (string, error) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return "", errCopyPositionForbidden
	}
	if e.db == nil {
		return "", fmt.Errorf("copy trading store is not configured")
	}
	var user models.User
	if err := e.db.Select("wallet_address").Where("id = ?", userID).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errNoWalletBound
		}
		return "", fmt.Errorf("failed to load account: %w", err)
	}
	address := strings.TrimSpace(user.WalletAddress)
	if !common.IsHexAddress(address) {
		return "", errNoWalletBound
	}
	return strings.ToLower(address), nil
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
		return errTraderNotFound
	}

	// Check if already following
	if _, alreadyFollowing := e.following[followerAddress]; alreadyFollowing {
		return errAlreadyFollowing
	}

	// Add follow relationship
	e.following[followerAddress] = traderAddress

	// Persist the copy configuration so the ratio and stop/take levels apply to
	// every position copied under this relationship.
	config.FollowerAddress = followerAddress
	config.TraderAddress = traderAddress
	e.configs[followerAddress] = config

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
		return errNotFollowing
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
	delete(e.configs, followerAddress)

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

	// The follower's configured ratio scales the copied size: a 0.5 ratio copies
	// half the trader's notional. A missing or invalid config falls back to the
	// full size, which is the ratio the follow request defaults to.
	config, hasConfig := e.configs[followerAddress]
	if hasConfig && config.CopyRatio > 0 && config.CopyRatio <= 1 && config.CopyRatio != 1 {
		notional, parseErr := parseDecimal(size)
		if parseErr != nil {
			return nil, fmt.Errorf("copy size %q is not a decimal amount: %w", size, parseErr)
		}
		ratio, ratioErr := rationalFromFloat(config.CopyRatio)
		if ratioErr != nil {
			return nil, ratioErr
		}
		scaled := new(big.Rat).Mul(notional, ratio)
		if scaled.Sign() <= 0 {
			return nil, fmt.Errorf("copy_ratio scales the size to zero")
		}
		size = trimDecimal(formatDecimal(scaled, 18))
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

// UpdateCopyPositions refreshes the mark price and unrealised PnL of every open
// copy position. The oracle calls happen before the write lock is taken so a
// slow price read cannot stall every other copy-trading operation.
func (e *CopyTradingEngine) UpdateCopyPositions(ctx context.Context) error {
	if e.priceAgg == nil {
		return nil
	}
	e.mu.RLock()
	pairs := make(map[string]struct{})
	for _, position := range e.copyPositions {
		if position.Status == "open" {
			pairs[position.Pair] = struct{}{}
		}
	}
	e.mu.RUnlock()

	prices := make(map[string]float64, len(pairs))
	for pair := range pairs {
		price, err := e.getPriceForPair(ctx, pair)
		if err != nil || price <= 0 {
			continue
		}
		prices[pair] = price
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	for _, position := range e.copyPositions {
		if position.Status != "open" {
			continue
		}
		price, ok := prices[position.Pair]
		if !ok {
			continue
		}
		position.CurrentPrice = price
		position.PNL = e.calculateCopyPNL(position)

		// Enforce the follower's stop-loss and take-profit. Both are percentages
		// of the position's entry notional, so they are compared against the
		// signed price move rather than the absolute price.
		config, hasConfig := e.configs[position.FollowerAddress]
		if !hasConfig || (config.StopLoss <= 0 && config.TakeProfit <= 0) {
			continue
		}
		movePct, err := e.copyMovePercent(position)
		if err != nil {
			continue
		}
		if config.StopLoss > 0 && movePct <= -config.StopLoss {
			now := time.Now()
			position.Status = "closed"
			position.ClosedAt = &now
			continue
		}
		if config.TakeProfit > 0 && movePct >= config.TakeProfit {
			now := time.Now()
			position.Status = "closed"
			position.ClosedAt = &now
		}
	}

	return nil
}

// copyMovePercent returns the signed percentage move of a position from its
// entry price to its current price, positive when the position is in profit.
func (e *CopyTradingEngine) copyMovePercent(position *CopyPosition) (float64, error) {
	if position.EntryPrice <= 0 || math.IsNaN(position.EntryPrice) || math.IsInf(position.EntryPrice, 0) {
		return 0, errors.New("position has no usable entry price")
	}
	if math.IsNaN(position.CurrentPrice) || math.IsInf(position.CurrentPrice, 0) {
		return 0, errors.New("position has no usable mark price")
	}
	move := (position.CurrentPrice - position.EntryPrice) / position.EntryPrice
	if strings.EqualFold(position.Side, "short") {
		move = -move
	}
	return move * 100, nil
}

// CloseCopyPosition closes a copy position owned by caller. Ownership is
// checked under the engine lock before any mutation, so one account can never
// close another account's position and a check cannot race a close.
func (e *CopyTradingEngine) CloseCopyPosition(positionID, caller string) (*CopyPosition, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	position, exists := e.copyPositions[positionID]
	if !exists {
		return nil, errCopyPositionNotFound
	}

	if caller == "" || !strings.EqualFold(position.FollowerAddress, caller) {
		return nil, errCopyPositionForbidden
	}

	if position.Status != "open" {
		return nil, errCopyPositionClosed
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

// calculateCopyPNL mirrors the perpetual engine's PnL: `Size` is the notional
// value of the copied position, so the leverage is already reflected in the
// collateral and must not be applied again to the price move.
func (e *CopyTradingEngine) calculateCopyPNL(position *CopyPosition) string {
	pnl, err := notionalPNL(position.EntryPrice, position.CurrentPrice, position.Size, position.Side)
	if err != nil {
		// A position whose size cannot be read has no meaningful PnL; reporting zero
		// never credits the follower with a gain.
		return "0"
	}
	return trimDecimal(formatDecimal(pnl, 8))
}

// trimDecimal removes trailing zeros (and a trailing dot) from a fixed-point
// decimal string so a scaled size is stored in its canonical shortest form.
func trimDecimal(value string) string {
	if !strings.Contains(value, ".") {
		return value
	}
	value = strings.TrimRight(value, "0")
	return strings.TrimSuffix(value, ".")
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
		respondBadRequestError(c, err)
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
		respondServiceError(c, err)
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
		respondBadRequestError(c, err)
		return
	}

	// The follower is the authenticated account's own wallet, never a
	// client-supplied value. The engine keys a follower by wallet address so
	// the wallet copy-trade path resolves the same identity.
	follower, err := e.resolveFollowerAddress(c)
	if err != nil {
		respondServiceError(c, err)
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
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Now following trader"})
}

// StopFollowingHandler stops copying for the authenticated caller.
func (e *CopyTradingEngine) StopFollowingHandler(c *gin.Context) {
	follower, err := e.resolveFollowerAddress(c)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	if err := e.StopFollowing(follower); err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Stopped following"})
}

// GetPositionsHandler returns the authenticated caller's copy positions.
func (e *CopyTradingEngine) GetPositionsHandler(c *gin.Context) {
	follower, err := e.resolveFollowerAddress(c)
	if err != nil {
		respondServiceError(c, err)
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
	follower, err := e.resolveFollowerAddress(c)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	position, err := e.CloseCopyPosition(positionID, follower)
	if err != nil {
		respondServiceError(c, err)
		return
	}

	c.JSON(http.StatusOK, position)
}
