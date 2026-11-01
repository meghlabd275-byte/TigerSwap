package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// PerpetualEngine handles real perpetual trading
type PerpetualEngine struct {
	blockchain *BlockchainClient
	priceAgg   *PriceAggregator
	positions  map[string]*PerpetualPosition // positionID -> position
	orders     map[string]*PerpetualOrder
	mu         sync.RWMutex

	// db, when set, is the source of truth for tradable markets. A market is
	// tradable only while its admin-managed listing is active, so the engine can
	// never trade a symbol that was never listed.
	db *gorm.DB

	// markets caches the active listings by symbol. It is rebuilt from the
	// database by RefreshMarkets and read on the trading path.
	markets map[string]*perpMarketConfig
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
	FundingPaid      string     `json:"funding_paid"`
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

// perpMarketConfig is the tradable subset of an admin listing. It is derived
// from a FuturesMarket or PerpsMarket row and carries everything the trading
// path needs: the oracle symbol, leverage cap, margin rates and fees.
type perpMarketConfig struct {
	Symbol                string
	OracleSymbol          string
	IndexSymbol           string
	MaxLeverage           int
	MakerFeeRate          float64
	TakerFeeRate          float64
	MaintenanceMarginRate float64
	LiquidationFeeRate    float64
	MaxOpenInterest       string
	PriceImpactRate       float64

	// Funding configuration comes from the listing so an operator controls the
	// rate and cadence per market rather than every market sharing one constant.
	FundingRate     float64
	FundingInterval time.Duration
	FundingRateCap  float64
}

// NewPerpetualEngine builds an engine with no markets. Markets are loaded from
// the admin listing tables with SetDB, so the engine can only ever trade
// symbols an operator has explicitly listed.
func NewPerpetualEngine(blockchain *BlockchainClient, priceAgg *PriceAggregator) *PerpetualEngine {
	return &PerpetualEngine{
		blockchain: blockchain,
		priceAgg:   priceAgg,
		positions:  make(map[string]*PerpetualPosition),
		orders:     make(map[string]*PerpetualOrder),
		markets:    make(map[string]*perpMarketConfig),
	}
}

// SetDB attaches the database that holds the market listings and loads the
// currently active markets. It is separate from the constructor so tests can
// build an engine without a database.
func (e *PerpetualEngine) SetDB(db *gorm.DB) error {
	e.db = db
	return e.RefreshMarkets()
}

// RefreshMarkets reloads the active market registry from the listing tables.
// Futures and perpetual listings are both honoured; a symbol listed in both
// keeps the futures entry, which carries the richer risk model. Listings that
// are paused, delisting or in maintenance are excluded, so pausing a market in
// the admin immediately stops it being tradable.
func (e *PerpetualEngine) RefreshMarkets() error {
	if e.db == nil {
		return errors.New("perpetual engine has no database configured")
	}

	registry := make(map[string]*perpMarketConfig)

	var futures []models.FuturesMarket
	if err := e.db.Where("status = ?", models.ListingStatusActive).Find(&futures).Error; err != nil {
		return fmt.Errorf("load futures listings: %w", err)
	}
	for i := range futures {
		cfg := futuresMarketConfig(&futures[i])
		if cfg == nil {
			continue
		}
		registry[cfg.Symbol] = cfg
	}

	var perps []models.PerpsMarket
	if err := e.db.Where("status = ?", models.ListingStatusActive).Find(&perps).Error; err != nil {
		return fmt.Errorf("load perps listings: %w", err)
	}
	for i := range perps {
		cfg := perpsMarketConfig(&perps[i])
		if cfg == nil {
			continue
		}
		if _, exists := registry[cfg.Symbol]; !exists {
			registry[cfg.Symbol] = cfg
		}
	}

	e.mu.Lock()
	e.markets = registry
	e.mu.Unlock()
	return nil
}

// futuresMarketConfig converts a futures listing into a tradable market.
// Leverage is bounded by the hard engine ceiling so a mis-typed listing cannot
// open leverage the risk engine does not support.
func futuresMarketConfig(m *models.FuturesMarket) *perpMarketConfig {
	symbol := strings.ToUpper(strings.TrimSpace(m.Symbol))
	base := strings.ToUpper(strings.TrimSpace(m.BaseAsset))
	if symbol == "" || base == "" {
		return nil
	}

	return &perpMarketConfig{
		Symbol:                symbol,
		OracleSymbol:          base,
		IndexSymbol:           base,
		MaxLeverage:           clampLeverage(m.MaxLeverage),
		MakerFeeRate:          m.MakerFeeRate,
		TakerFeeRate:          m.TakerFeeRate,
		MaintenanceMarginRate: m.MaintenanceMarginRate,
		LiquidationFeeRate:    m.LiquidationFeeRate,
		FundingRate:           decimalOrZero(m.FundingRate),
		FundingInterval:       fundingInterval(m.FundingInterval),
		FundingRateCap:        decimalOrDefault(m.FundingRateCap, defaultFundingRateCap),
	}
}

// perpsMarketConfig converts an oracle perpetual listing into a tradable
// market. Orderbook listings are matched by the CLOB, not this engine, so only
// oracle-priced markets are registered here.
func perpsMarketConfig(m *models.PerpsMarket) *perpMarketConfig {
	symbol := strings.ToUpper(strings.TrimSpace(m.Symbol))
	if symbol == "" {
		return nil
	}
	if mt := strings.ToLower(strings.TrimSpace(m.MarketType)); mt != "" && mt != "oracle" {
		return nil
	}
	base := oracleBaseFromSymbol(symbol)
	if base == "" {
		return nil
	}

	return &perpMarketConfig{
		Symbol:                symbol,
		OracleSymbol:          base,
		IndexSymbol:           base,
		MaxLeverage:           clampLeverage(m.MaxLeverage),
		MakerFeeRate:          m.MakerFeeRate,
		TakerFeeRate:          m.TakerFeeRate,
		MaintenanceMarginRate: m.MaintenanceMarginRate,
		LiquidationFeeRate:    m.LiquidationFeeRate,
		MaxOpenInterest:       strings.TrimSpace(m.MaxOpenInterest),
		PriceImpactRate:       m.PriceImpactRate,
		FundingRate:           defaultFundingRate,
		FundingInterval:       defaultFundingInterval,
		FundingRateCap:        defaultFundingRateCap,
	}
}

// clampLeverage bounds a listing's leverage to the engine ceiling.
func clampLeverage(leverage int) int {
	if leverage <= 0 || leverage > maxPerpLeverage {
		return maxPerpLeverage
	}
	return leverage
}

// oracleBaseFromSymbol extracts the base asset from a listing symbol. Listings
// use the slash form ("BTC/RUSD"); the hyphen form ("BTC-USD") is also accepted
// so existing callers keep working. Returns "" when no base asset is present.
func oracleBaseFromSymbol(symbol string) string {
	s := strings.ToUpper(strings.TrimSpace(symbol))
	if s == "" {
		return ""
	}
	if i := strings.IndexAny(s, "/-"); i > 0 {
		return s[:i]
	}
	return s
}

// market returns the tradable configuration for a symbol, or nil when the
// symbol is not an active listing.
func (e *PerpetualEngine) market(pair string) *perpMarketConfig {
	symbol := strings.ToUpper(strings.TrimSpace(pair))
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.markets[symbol]
}

// Markets returns the active market symbols.
func (e *PerpetualEngine) Markets() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	symbols := make([]string, 0, len(e.markets))
	for symbol := range e.markets {
		symbols = append(symbols, symbol)
	}
	return symbols
}

// OpenPosition opens a new perpetual position. The position is opened against
// the engine's own mark and index prices; the caller's on-chain settlement is
// broadcast separately with a client-signed transaction.
func (e *PerpetualEngine) OpenPosition(ctx context.Context, ownerID string, chainID int64, pair, side, size, collateral string, leverage int) (*PerpetualPosition, error) {
	// Resolve the listing first: it is the source of truth for leverage and
	// fees, and an unlisted symbol must never be tradable.
	cfg := e.market(pair)
	if cfg == nil {
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
	sizeRat, err := perpAmountValue(size)
	if err != nil {
		return nil, err
	}
	collateralRat, err := perpAmountValue(collateral)
	if err != nil {
		return nil, err
	}
	if markPrice <= 0 || math.IsNaN(markPrice) || math.IsInf(markPrice, 0) {
		return nil, fmt.Errorf("no mark price available for %s", pair)
	}
	if leverage < 1 || leverage > cfg.MaxLeverage {
		return nil, fmt.Errorf("leverage must be between 1 and %d for %s", cfg.MaxLeverage, cfg.Symbol)
	}

	// Margin is size/leverage, computed exactly so a position sized to the limit
	// is accepted rather than refused by binary rounding.
	requiredMargin := new(big.Rat).Quo(sizeRat, new(big.Rat).SetInt64(int64(leverage)))
	if collateralRat.Cmp(requiredMargin) < 0 {
		need, err := rationalToFloat(requiredMargin)
		if err != nil {
			return nil, err
		}
		sizeValue, err := rationalToFloat(sizeRat)
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("insufficient collateral: need %.8f for %dx on size %.8f", need, leverage, sizeValue)
	}

	// Enforce the listing's open-interest cap. Without this a listed cap was
	// accepted by the admin API and then never applied to trading.
	if cap, limited, err := openInterestCapRat(cfg); err != nil {
		return nil, err
	} else if limited {
		long, short, err := e.openInterestRat(pair)
		if err != nil {
			return nil, err
		}
		open := new(big.Rat).Add(long, short)
		projected := new(big.Rat).Add(open, sizeRat)
		if projected.Cmp(cap) > 0 {
			return nil, fmt.Errorf("open interest cap reached for %s: %s open plus %s would exceed the %s cap",
				cfg.Symbol, open.RatString(), sizeRat.RatString(), cap.RatString())
		}
	}

	position := &PerpetualPosition{
		ID:               uuid.NewString(),
		UserAddress:      ownerID,
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
func (e *PerpetualEngine) ClosePosition(ctx context.Context, ownerID, positionID string) (*PerpetualPosition, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	position, exists := e.positions[positionID]
	if !exists {
		return nil, fmt.Errorf("position not found: %s", positionID)
	}
	if ownerID != "" && position.UserAddress != ownerID {
		// Report a missing position rather than a permission error so the
		// endpoint cannot be used to probe which position ids exist.
		return nil, fmt.Errorf("position not found: %s", positionID)
	}
	if position.Status == "liquidatable" {
		// A position the risk engine has flagged must settle through
		// LiquidatePosition, which applies the liquidation fee. Allowing a plain
		// close here would let the trader escape that penalty.
		return nil, fmt.Errorf("position %s is being liquidated and cannot be closed", positionID)
	}
	if position.Status != "open" {
		return nil, fmt.Errorf("position %s is not open (status %s)", positionID, position.Status)
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
		if pos.UserAddress == userAddress && positionIsLive(pos.Status) {
			userPositions = append(userPositions, pos)
		}
	}

	return userPositions
}

// UpdatePositions refreshes mark prices and unrealised PnL for every open
// position. A position that has crossed its liquidation price is marked so it
// can no longer be traded against, but this method deliberately does not settle
// it: liquidation moves funds and must be executed by an operator or liquidation
// bot through LiquidatePosition. Detecting a liquidation here and silently
// leaving the position open would let a trader close at a price the risk engine
// has already declared invalid.
func (e *PerpetualEngine) UpdatePositions(ctx context.Context) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, position := range e.positions {
		if !positionIsLive(position.Status) {
			continue
		}

		// Get current mark price
		markPrice, err := e.getMarkPrice(ctx, position.Pair)
		if err != nil {
			continue
		}

		position.MarkPrice = markPrice

		// A position that has crossed its liquidation price is flagged for
		// settlement; one that has recovered is returned to open, so a temporary
		// wick does not leave a healthy position permanently untradable.
		if e.isLiquidatable(position) {
			position.Status = "liquidatable"
		} else if position.Status == "liquidatable" {
			position.Status = "open"
		}
		position.UnrealizedPNL = e.calculatePNL(position, markPrice)
	}

	return nil
}

// LiquidatablePositions returns every position whose mark price has crossed its
// liquidation price and that has not yet been settled. A liquidation bot or an
// operator acts on this list; detection is separate from execution.
func (e *PerpetualEngine) LiquidatablePositions() []*PerpetualPosition {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var liquidatable []*PerpetualPosition
	for _, position := range e.positions {
		if position.Status == "liquidatable" {
			liquidatable = append(liquidatable, position)
		}
	}
	return liquidatable
}

// LiquidatePosition settles an under-collateralised position. The trader's
// remaining margin, less the listing's liquidation fee, is returned to the
// caller as the liquidator's reward and the position is closed. It refuses any
// position that is not currently flagged liquidatable, so it cannot be used to
// close a healthy position and seize its margin.
//
// Returns the liquidation fee owed to the caller.
func (e *PerpetualEngine) LiquidatePosition(ctx context.Context, positionID string) (*big.Rat, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	position, exists := e.positions[positionID]
	if !exists {
		return nil, fmt.Errorf("position not found: %s", positionID)
	}
	if position.Status != "liquidatable" {
		return nil, fmt.Errorf("position %s is not liquidatable (status %s)", positionID, position.Status)
	}

	cfg := e.market(position.Pair)
	if cfg == nil {
		return nil, fmt.Errorf("market %s is no longer listed", position.Pair)
	}

	// Re-price at settlement: the flag was set by an earlier update and the price
	// may have recovered since, in which case the position is no longer
	// liquidatable and must be left alone.
	markPrice, err := e.getMarkPrice(ctx, position.Pair)
	if err != nil {
		return nil, err
	}
	position.MarkPrice = markPrice
	if !e.isLiquidatable(position) {
		position.Status = "open"
		position.UnrealizedPNL = e.calculatePNL(position, markPrice)
		return nil, fmt.Errorf("position %s is no longer liquidatable at mark %g", positionID, markPrice)
	}

	// The liquidator receives the remaining margin less the liquidation fee. A
	// position whose loss exceeds its margin has nothing left to pay out.
	collateral, err := parseDecimal(position.Collateral)
	if err != nil {
		return nil, fmt.Errorf("position %s has unreadable collateral %q: %w", positionID, position.Collateral, err)
	}
	pnl, err := notionalPNL(position.EntryPrice, markPrice, position.Size, position.Side)
	if err != nil {
		return nil, fmt.Errorf("position %s: %w", positionID, err)
	}

	fee, err := liquidationPayout(collateral, pnl, cfg.LiquidationFeeRate, cfg.Symbol)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	position.Status = "liquidated"
	position.ClosedAt = &now
	position.UnrealizedPNL = formatDecimal(pnl, 8)

	// The fee is the liquidator's reward; the remainder is the trader's returned
	// margin. Both are reported in settlement terms.
	return fee, nil
}

// liquidationPayout computes the fee a liquidator earns on a settled position.
// The trader's remaining margin is the posted collateral plus the position's
// PnL; a loss larger than the collateral leaves nothing to pay out, so the fee
// floors at zero rather than going negative and charging the liquidator.
//
// It is a pure function so the funds arithmetic can be tested without a price
// feed or a database.
func liquidationPayout(collateral, pnl *big.Rat, feeRate float64, symbol string) (*big.Rat, error) {
	if collateral == nil || pnl == nil {
		return nil, errors.New("liquidation payout requires a collateral and PnL value")
	}

	rate, err := rationalFromFloat(feeRate)
	if err != nil {
		return nil, fmt.Errorf("listing %s has an unreadable liquidation fee rate %g: %w", symbol, feeRate, err)
	}
	if rate.Sign() < 0 || rate.Cmp(big.NewRat(1, 1)) > 0 {
		return nil, fmt.Errorf("listing %s has an out-of-range liquidation fee rate %g", symbol, feeRate)
	}

	remaining := new(big.Rat).Add(collateral, pnl)
	if remaining.Sign() < 0 {
		remaining = new(big.Rat)
	}
	return new(big.Rat).Mul(remaining, rate), nil
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
	cfg := e.market(pair)
	if cfg == nil {
		return 0, fmt.Errorf("market %s is not listed", pair)
	}

	priceData, err := e.priceAgg.GetRealPrice(ctx, cfg.OracleSymbol)
	if err != nil {
		return 0, err
	}
	if priceData.USD <= 0 {
		return 0, fmt.Errorf("oracle returned a non-positive price for %s", cfg.OracleSymbol)
	}

	return priceData.USD, nil
}

// getIndexPrice returns the underlying index price for a pair from the market's
// configured index symbol. It falls back to the mark price only when the
// listing does not name a separate index feed, so a listed index source is
// always honoured and a missing one cannot silently reuse the mark.
func (e *PerpetualEngine) getIndexPrice(ctx context.Context, pair string) (float64, error) {
	cfg := e.market(pair)
	if cfg == nil {
		return 0, fmt.Errorf("market %s is not listed", pair)
	}
	symbol := cfg.IndexSymbol
	if symbol == "" {
		symbol = cfg.OracleSymbol
	}

	priceData, err := e.priceAgg.GetRealPrice(ctx, symbol)
	if err != nil {
		return 0, err
	}
	if priceData.USD <= 0 {
		return 0, fmt.Errorf("oracle returned a non-positive price for %s", symbol)
	}
	return priceData.USD, nil
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

// calculatePNL returns the position's profit and loss in settlement terms.
//
// `position.Size` is the notional value of the position in quote terms: the
// open path requires `size / leverage` of collateral, so a size of 10,000 at
// 10x is a 1,000 margin position controlling 10,000 of notional. PnL is
// therefore the price move applied to that notional. Multiplying by the
// leverage a second time would report ten times the true profit, which is why
// it is deliberately absent here.
func (e *PerpetualEngine) calculatePNL(position *PerpetualPosition, currentPrice float64) string {
	pnl, err := notionalPNL(position.EntryPrice, currentPrice, position.Size, position.Side)
	if err != nil {
		// A position whose size cannot be read has no meaningful PnL. Reporting zero
		// is the conservative choice: it never credits the trader with a gain.
		return "0"
	}
	return formatDecimal(pnl, 8)
}

// notionalPNL computes the PnL of a position whose size is a notional amount,
// using exact rational arithmetic so the reported figure is not distorted by
// binary rounding. A zero or negative entry price yields zero: the position has
// no cost basis to measure a move against.
func notionalPNL(entryPrice, currentPrice float64, size, side string) (*big.Rat, error) {
	if entryPrice <= 0 || math.IsNaN(entryPrice) || math.IsInf(entryPrice, 0) {
		return new(big.Rat), nil
	}
	notional, err := parseDecimal(size)
	if err != nil {
		return nil, err
	}
	entry, err := rationalFromFloat(entryPrice)
	if err != nil {
		return nil, err
	}
	current, err := rationalFromFloat(currentPrice)
	if err != nil {
		return nil, err
	}

	move := new(big.Rat).Quo(new(big.Rat).Sub(current, entry), entry)
	if strings.EqualFold(side, "short") {
		move.Neg(move)
	}
	return move.Mul(move, notional), nil
}

func (e *PerpetualEngine) isValidPair(pair string) bool {
	return e.market(pair) != nil
}

// GetFundingRate computes the current funding rate for a pair.
//
// The rate is the listing's configured interest component plus the mark/index
// premium, bounded by the listing's funding cap. Both the interval and the cap
// come from the market's admin listing, so an operator can tune funding per
// market rather than every market sharing one constant. An unlisted pair is
// rejected rather than priced with defaults.
func (e *PerpetualEngine) GetFundingRate(ctx context.Context, pair string) (*FundingInfo, error) {
	cfg := e.market(pair)
	if cfg == nil {
		return nil, fmt.Errorf("market %s is not listed", pair)
	}

	markPrice, err := e.getMarkPrice(ctx, pair)
	if err != nil {
		return nil, err
	}

	indexPrice, err := e.getIndexPrice(ctx, pair)
	if err != nil {
		return nil, err
	}
	if indexPrice <= 0 {
		return nil, fmt.Errorf("index price for %s is not positive", pair)
	}

	// Premium is the mark/index deviation as a fraction of the index. It is
	// added to the listing's interest component to form the funding rate.
	premium := (markPrice - indexPrice) / indexPrice

	interestRate := cfg.FundingRate
	cap := cfg.FundingRateCap
	if cap <= 0 {
		cap = defaultFundingRateCap
	}

	fundingRate := premium + interestRate
	if fundingRate > cap {
		fundingRate = cap
	} else if fundingRate < -cap {
		fundingRate = -cap
	}

	return &FundingInfo{
		Pair:            pair,
		LastFundingRate: fundingRate,
		NextFundingTime: e.nextFundingTime(cfg.FundingInterval),
		InterestRate:    interestRate,
	}, nil
}

// nextFundingTime returns the next funding settlement for the given interval,
// aligned to the interval boundary. A non-positive interval falls back to the
// default cadence.
func (e *PerpetualEngine) nextFundingTime(interval time.Duration) time.Time {
	if interval <= 0 {
		interval = defaultFundingInterval
	}
	now := time.Now().UTC()
	next := now.Truncate(interval).Add(interval)
	if !next.After(now) {
		next = next.Add(interval)
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
	owner, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	positionID := c.Param("id")
	position, err := e.ClosePosition(c.Request.Context(), owner, positionID)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, position)
}

func (e *PerpetualEngine) GetPositionsHandler(c *gin.Context) {
	owner, ok := authenticatedUserID(c)
	if !ok {
		return
	}

	positions := e.GetUserPositions(owner)
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
		"pairs": e.Markets(),
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

// Default funding parameters applied to a listing that does not name its own.
// A futures listing may override the rate, cadence and cap; a perps listing
// currently exposes only the engine defaults.
const (
	defaultFundingRate     = 0.0001        // 0.01% per interval
	defaultFundingInterval = 8 * time.Hour // eight-hour settlement
	defaultFundingRateCap  = 0.01          // 1% absolute cap
)

// fundingInterval converts a listing's funding interval in hours to a duration.
// A zero or negative value falls back to the default cadence so a listing can
// never disable settlement by omission.
func fundingInterval(hours int) time.Duration {
	if hours <= 0 {
		return defaultFundingInterval
	}
	return time.Duration(hours) * time.Hour
}

// decimalOrZero parses a stored decimal into a float, yielding zero when the
// value is absent or unparseable. It is only used for non-negative funding
// parameters whose safe default is "no interest component".
func decimalOrZero(value string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v < 0 {
		return 0
	}
	return v
}

// decimalOrDefault parses a stored decimal into a float, yielding the supplied
// default when the value is absent or unparseable.
func decimalOrDefault(value string, def float64) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return def
	}
	return v
}

// maxPerpLeverage is the absolute leverage ceiling for engine-priced perpetual
// markets. It is the upper bound for a listing's own max_leverage; a listing may
// set anything from 1 up to this value. It is also the fallback when a listing
// leaves max_leverage unset or out of range, so a bad listing can never open
// more leverage than the risk engine supports.
const maxPerpLeverage = 1001

// GetMarketsHandler lists the engine's markets with live mark/index prices and
// funding. A market whose price feed is unavailable is still returned but is
// marked inactive, so a client never trades against a stale price.
func (e *PerpetualEngine) GetMarketsHandler(c *gin.Context) {
	ctx := c.Request.Context()
	symbols := e.Markets()

	markets := make([]PerpMarketView, 0, len(symbols))
	for _, symbol := range symbols {
		cfg := e.market(symbol)
		if cfg == nil {
			continue
		}

		view := PerpMarketView{
			MarketID:    symbol,
			Symbol:      symbol,
			MaxLeverage: cfg.MaxLeverage,
		}

		markPrice, err := e.getMarkPrice(ctx, symbol)
		if err != nil {
			// A market with no reachable price is returned inactive, so a
			// client never treats a stale price as tradable.
			markets = append(markets, view)
			continue
		}

		view.MarkPrice = markPrice
		// Report the market's own index price. Fall back to the mark only when
		// the index feed is momentarily unavailable, so the two never diverge
		// silently in the other direction.
		if indexPrice, err := e.getIndexPrice(ctx, symbol); err == nil {
			view.IndexPrice = indexPrice
		} else {
			view.IndexPrice = markPrice
		}
		view.IsActive = true

		if funding, err := e.GetFundingRate(ctx, symbol); err == nil {
			view.FundingRate = funding.LastFundingRate
		}

		long, short, err := e.openInterest(symbol)
		if err != nil {
			// A market whose positions cannot be valued is reported with zero open
			// interest rather than a partial total that would understate it.
			long, short = 0, 0
		}
		view.OpenInterestLong = long
		view.OpenInterestShort = short

		markets = append(markets, view)
	}

	c.JSON(200, gin.H{"markets": markets})
}

// openInterest totals the notional of open positions on one symbol.
func (e *PerpetualEngine) openInterest(symbol string) (float64, float64, error) {
	long, short, err := e.openInterestRat(symbol)
	if err != nil {
		return 0, 0, err
	}
	longValue, err := rationalToFloat(long)
	if err != nil {
		return 0, 0, err
	}
	shortValue, err := rationalToFloat(short)
	if err != nil {
		return 0, 0, err
	}
	return longValue, shortValue, nil
}
