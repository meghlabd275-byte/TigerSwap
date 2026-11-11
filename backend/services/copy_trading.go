package services

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

// CopyTradingEngine handles real copy trading. Traders, follow relationships and
// copied positions are held in the database: keeping them in process memory
// would erase the leaderboard and every open copy the moment the API restarts.
type CopyTradingEngine struct {
	blockchain *BlockchainClient
	priceAgg   priceSource

	// db resolves the authenticated account to its bound wallet address and is
	// the store for traders, follows and copied positions. The engine keys a
	// follower by wallet address, so the HTTP handlers and the wallet copy-trade
	// path agree on one identity domain; keying one by the account UUID and the
	// other by the address would make a followed trader impossible to copy from
	// the wallet route.
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
		blockchain: blockchain,
		priceAgg:   priceAgg,
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

// RegisterTrader registers a trader, persisting the profile so the leaderboard
// survives a restart. Re-registering an address refreshes the profile fields
// while preserving the statistics accumulated so far.
func (e *CopyTradingEngine) RegisterTrader(address, name, avatar string) *Trader {
	address = strings.ToLower(strings.TrimSpace(address))
	now := time.Now()
	trader := &Trader{
		Address:     address,
		Name:        name,
		Avatar:      avatar,
		TotalProfit: "0",
		TotalVolume: "0",
		RiskScore:   "medium",
		LastActive:  now,
	}
	if e.db == nil {
		return trader
	}

	var existing models.CopyTrader
	err := e.db.Where("address = ?", address).First(&existing).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		row := copyTraderToModel(trader)
		if err := e.db.Create(&row).Error; err != nil {
			log.Printf("copy trading: persist trader %s: %v", address, err)
		}
	case err != nil:
		log.Printf("copy trading: load trader %s: %v", address, err)
	default:
		if err := e.db.Model(&models.CopyTrader{}).Where("address = ?", address).
			Updates(map[string]interface{}{"name": name, "avatar": avatar, "last_active": now}).Error; err != nil {
			log.Printf("copy trading: update trader %s: %v", address, err)
		}
		trader.Followers = existing.Followers
	}
	return trader
}

// GetTrader returns a trader's profile.
func (e *CopyTradingEngine) GetTrader(address string) (*Trader, error) {
	if e.db == nil {
		return nil, errTraderNotFound
	}
	var row models.CopyTrader
	if err := e.db.Where("address = ?", strings.ToLower(strings.TrimSpace(address))).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errTraderNotFound
		}
		return nil, err
	}
	return copyTraderFromModel(&row), nil
}

// GetTopTraders returns the leaderboard ordered by weekly profit then trade
// count, read from the store so a restart cannot empty it.
func (e *CopyTradingEngine) GetTopTraders(limit int) []*Trader {
	if e.db == nil {
		return nil
	}
	query := e.db.Order("weekly_profit DESC, total_trades DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var rows []models.CopyTrader
	if err := query.Find(&rows).Error; err != nil {
		return nil
	}
	traders := make([]*Trader, 0, len(rows))
	for i := range rows {
		traders = append(traders, copyTraderFromModel(&rows[i]))
	}
	return traders
}

// StartFollowing starts following a trader. The follow row and the trader's
// follower count move together in one transaction so the count can never drift
// from the relationships.
func (e *CopyTradingEngine) StartFollowing(followerAddress, traderAddress string, config CopyConfig) error {
	if e.db == nil {
		return fmt.Errorf("copy trading store is not configured")
	}
	followerAddress = strings.ToLower(strings.TrimSpace(followerAddress))
	traderAddress = strings.ToLower(strings.TrimSpace(traderAddress))

	return e.db.Transaction(func(tx *gorm.DB) error {
		var trader models.CopyTrader
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("address = ?", traderAddress).First(&trader).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errTraderNotFound
			}
			return err
		}

		var existing models.CopyFollow
		switch err := tx.Where("follower_address = ?", followerAddress).First(&existing).Error; {
		case err == nil:
			return errAlreadyFollowing
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return err
		}

		follow := models.CopyFollow{
			FollowerAddress: followerAddress,
			TraderAddress:   traderAddress,
			CopyRatio:       config.CopyRatio,
			AutoClose:       config.AutoClose,
			StopLoss:        config.StopLoss,
			TakeProfit:      config.TakeProfit,
		}
		if err := tx.Create(&follow).Error; err != nil {
			return err
		}
		return tx.Model(&models.CopyTrader{}).Where("address = ?", traderAddress).
			Update("followers", gorm.Expr("followers + 1")).Error
	})
}

// StopFollowing stops following a trader and decrements the trader's follower
// count in the same transaction.
func (e *CopyTradingEngine) StopFollowing(followerAddress string) error {
	if e.db == nil {
		return fmt.Errorf("copy trading store is not configured")
	}
	followerAddress = strings.ToLower(strings.TrimSpace(followerAddress))

	return e.db.Transaction(func(tx *gorm.DB) error {
		var follow models.CopyFollow
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("follower_address = ?", followerAddress).First(&follow).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errNotFollowing
			}
			return err
		}
		if err := tx.Delete(&models.CopyFollow{}, "id = ?", follow.ID).Error; err != nil {
			return err
		}
		return tx.Model(&models.CopyTrader{}).
			Where("address = ? AND followers > 0", follow.TraderAddress).
			Update("followers", gorm.Expr("followers - 1")).Error
	})
}

// CopyTrade copies a trade from a trader to a follower and persists the copy
// position.
func (e *CopyTradingEngine) CopyTrade(ctx context.Context, followerAddress, traderAddress, pair, side, size, originalPositionID string) (*CopyPosition, error) {
	if e.db == nil {
		return nil, fmt.Errorf("copy trading store is not configured")
	}
	followerAddress = strings.ToLower(strings.TrimSpace(followerAddress))
	traderAddress = strings.ToLower(strings.TrimSpace(traderAddress))

	var follow models.CopyFollow
	if err := e.db.Where("follower_address = ?", followerAddress).First(&follow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("not following this trader")
		}
		return nil, err
	}
	if follow.TraderAddress != traderAddress {
		return nil, fmt.Errorf("not following this trader")
	}

	price, err := e.getPriceForPair(ctx, pair)
	if err != nil {
		return nil, err
	}

	// The follower's configured ratio scales the copied size: a 0.5 ratio copies
	// half the trader's notional.
	if follow.CopyRatio > 0 && follow.CopyRatio <= 1 && follow.CopyRatio != 1 {
		notional, parseErr := parseDecimal(size)
		if parseErr != nil {
			return nil, fmt.Errorf("copy size %q is not a decimal amount: %w", size, parseErr)
		}
		ratio, ratioErr := rationalFromFloat(follow.CopyRatio)
		if ratioErr != nil {
			return nil, ratioErr
		}
		scaled := new(big.Rat).Mul(notional, ratio)
		if scaled.Sign() <= 0 {
			return nil, fmt.Errorf("copy_ratio scales the size to zero")
		}
		size = trimDecimal(formatDecimal(scaled, 18))
	}

	now := time.Now()
	position := &CopyPosition{
		ID:               uuid.NewString(),
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
		OpenedAt:         now,
		CopiedAt:         now,
	}
	row := copyPositionToModel(position)
	if err := e.db.Create(&row).Error; err != nil {
		return nil, fmt.Errorf("persist copy position: %w", err)
	}
	// The row's generated id is authoritative: the id returned to the caller must
	// be the one the close and listing paths will find.
	position.ID = row.ID.String()
	return position, nil
}

// UpdateCopyPositions re-marks every open copy position to the live oracle
// price, refreshes its unrealised PnL and enforces the follower's stop-loss and
// take-profit. Every price is read before any write, so a slow oracle call
// cannot hold the write path open.
func (e *CopyTradingEngine) UpdateCopyPositions(ctx context.Context) error {
	if e.priceAgg == nil || e.db == nil {
		return nil
	}

	var rows []models.CopyPositionRecord
	if err := e.db.Where("status = ?", "open").Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	positions := make([]*CopyPosition, 0, len(rows))
	for i := range rows {
		positions = append(positions, copyPositionFromModel(&rows[i]))
	}

	// Price each distinct pair once for the whole pass.
	prices := make(map[string]float64)
	for _, position := range positions {
		if _, ok := prices[position.Pair]; ok {
			continue
		}
		price, err := e.getPriceForPair(ctx, position.Pair)
		if err != nil || price <= 0 {
			prices[position.Pair] = 0
			continue
		}
		prices[position.Pair] = price
	}

	// Load every follow configuration once, so the stop/take levels apply from
	// the store rather than from memory that a restart would have emptied.
	follows := make(map[string]models.CopyFollow)
	var followRows []models.CopyFollow
	if err := e.db.Find(&followRows).Error; err == nil {
		for i := range followRows {
			follows[followRows[i].FollowerAddress] = followRows[i]
		}
	}

	for _, position := range positions {
		price := prices[position.Pair]
		if price <= 0 {
			continue
		}
		position.CurrentPrice = price
		position.PNL = e.calculateCopyPNL(position)

		closed := false
		if config, ok := follows[position.FollowerAddress]; ok && (config.StopLoss > 0 || config.TakeProfit > 0) {
			if movePct, err := e.copyMovePercent(position); err == nil {
				if config.StopLoss > 0 && movePct <= -config.StopLoss {
					closed = true
				}
				if config.TakeProfit > 0 && movePct >= config.TakeProfit {
					closed = true
				}
			}
		}

		updates := map[string]interface{}{
			"current_price": formatPrice(position.CurrentPrice),
			"pnl":           position.PNL,
		}
		if closed {
			now := time.Now()
			updates["status"] = "closed"
			updates["closed_at"] = &now
		}
		if err := e.db.Model(&models.CopyPositionRecord{}).Where("id = ?", position.ID).Updates(updates).Error; err != nil {
			return err
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

// CloseCopyPosition closes a copy position owned by caller. Ownership is checked
// under a row lock before any mutation, so one account can never close another
// account's position and a check cannot race a close.
func (e *CopyTradingEngine) CloseCopyPosition(positionID, caller string) (*CopyPosition, error) {
	if e.db == nil {
		return nil, fmt.Errorf("copy trading store is not configured")
	}
	id, err := uuid.Parse(strings.TrimSpace(positionID))
	if err != nil {
		return nil, errCopyPositionNotFound
	}

	var closed *CopyPosition
	err = e.db.Transaction(func(tx *gorm.DB) error {
		var row models.CopyPositionRecord
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			First(&row, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errCopyPositionNotFound
			}
			return err
		}
		if caller == "" || !strings.EqualFold(row.FollowerAddress, caller) {
			return errCopyPositionForbidden
		}
		if row.Status != "open" {
			return errCopyPositionClosed
		}
		now := time.Now()
		if err := tx.Model(&models.CopyPositionRecord{}).Where("id = ?", row.ID).
			Updates(map[string]interface{}{"status": "closed", "closed_at": &now}).Error; err != nil {
			return err
		}
		row.Status = "closed"
		row.ClosedAt = &now
		closed = copyPositionFromModel(&row)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return closed, nil
}

// GetFollowerPositions returns a follower's open copy positions.
func (e *CopyTradingEngine) GetFollowerPositions(followerAddress string) []*CopyPosition {
	if e.db == nil {
		return nil
	}
	var rows []models.CopyPositionRecord
	if err := e.db.
		Where("follower_address = ? AND status = ?", strings.ToLower(strings.TrimSpace(followerAddress)), "open").
		Order("opened_at DESC").Find(&rows).Error; err != nil {
		return nil
	}
	positions := make([]*CopyPosition, 0, len(rows))
	for i := range rows {
		positions = append(positions, copyPositionFromModel(&rows[i]))
	}
	return positions
}

// copyTraderToModel and copyTraderFromModel convert between the wire profile
// and the persisted row; strategies are stored comma-separated.
func copyTraderToModel(t *Trader) models.CopyTrader {
	return models.CopyTrader{
		Address:        t.Address,
		Name:           t.Name,
		Avatar:         t.Avatar,
		WinRate:        t.WinRate,
		TotalProfit:    t.TotalProfit,
		WeeklyProfit:   t.WeeklyProfit,
		MonthlyProfit:  t.MonthlyProfit,
		TotalTrades:    t.TotalTrades,
		Followers:      t.Followers,
		TotalVolume:    t.TotalVolume,
		AvgHoldingTime: t.AvgHoldingTime,
		IsVerified:     t.IsVerified,
		IsPro:          t.IsPro,
		RiskScore:      t.RiskScore,
		Strategies:     strings.Join(t.Strategies, ","),
		LastActive:     t.LastActive,
	}
}

func copyTraderFromModel(m *models.CopyTrader) *Trader {
	var strategies []string
	if m.Strategies != "" {
		strategies = strings.Split(m.Strategies, ",")
	}
	return &Trader{
		Address:        m.Address,
		Name:           m.Name,
		Avatar:         m.Avatar,
		WinRate:        m.WinRate,
		TotalProfit:    m.TotalProfit,
		WeeklyProfit:   m.WeeklyProfit,
		MonthlyProfit:  m.MonthlyProfit,
		TotalTrades:    m.TotalTrades,
		Followers:      m.Followers,
		TotalVolume:    m.TotalVolume,
		AvgHoldingTime: m.AvgHoldingTime,
		IsVerified:     m.IsVerified,
		IsPro:          m.IsPro,
		RiskScore:      m.RiskScore,
		Strategies:     strategies,
		LastActive:     m.LastActive,
	}
}

func copyPositionToModel(p *CopyPosition) models.CopyPositionRecord {
	return models.CopyPositionRecord{
		FollowerAddress:  p.FollowerAddress,
		TraderAddress:    p.TraderAddress,
		OriginalPosition: p.OriginalPosition,
		Pair:             p.Pair,
		Side:             p.Side,
		Size:             p.Size,
		EntryPrice:       formatPrice(p.EntryPrice),
		CurrentPrice:     formatPrice(p.CurrentPrice),
		Leverage:         p.Leverage,
		PNL:              p.PNL,
		Status:           p.Status,
		OpenedAt:         p.OpenedAt,
		ClosedAt:         p.ClosedAt,
		CopiedAt:         p.CopiedAt,
	}
}

func copyPositionFromModel(m *models.CopyPositionRecord) *CopyPosition {
	return &CopyPosition{
		ID:               m.ID.String(),
		FollowerAddress:  m.FollowerAddress,
		TraderAddress:    m.TraderAddress,
		OriginalPosition: m.OriginalPosition,
		Pair:             m.Pair,
		Side:             m.Side,
		Size:             m.Size,
		EntryPrice:       parsePrice(m.EntryPrice),
		CurrentPrice:     parsePrice(m.CurrentPrice),
		Leverage:         m.Leverage,
		PNL:              m.PNL,
		Status:           m.Status,
		OpenedAt:         m.OpenedAt,
		ClosedAt:         m.ClosedAt,
		CopiedAt:         m.CopiedAt,
	}
}

// formatPrice and parsePrice round-trip a price through a decimal string so a
// stored mark price is exact to eight places and never loses precision to a
// float formatter.
func formatPrice(value float64) string {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return "0"
	}
	return strconv.FormatFloat(value, 'f', 8, 64)
}

func parsePrice(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
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
