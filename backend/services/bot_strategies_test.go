package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sync/atomic"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
	"tigerswap/backend/services/botengine"
)

var botStrategiesDBSeq uint64

// newPortfolioTestDB opens an in-memory database with the balance table the
// portfolio reads, so the rebalance source is exercised against real rows.
func newPortfolioTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:botstrategies-%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&botStrategiesDBSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })
	if err := db.AutoMigrate(&models.AccountBalance{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// TestRebalanceHoldingsUsesRealPortfolioValues proves the rebalance source feeds
// the planner the owner's real holding values, valued from the live oracle, and
// reports a holding the oracle cannot price as zero rather than a guess.
func TestRebalanceHoldingsUsesRealPortfolioValues(t *testing.T) {
	db := newPortfolioTestDB(t)
	prices := &stubPriceAggregator{prices: map[string]float64{"BTC": 60000, "ETH": 3000}}

	// 1 BTC (8 decimals) and 10 ETH (18 decimals); TGR has no price.
	for _, row := range []models.AccountBalance{
		{UserID: "trader-1", ChainID: 1, TokenAddress: "0xbtc", Symbol: "BTC", Decimals: 8, Available: "100000000", Locked: "0"},
		{UserID: "trader-1", ChainID: 1, TokenAddress: "0xeth", Symbol: "ETH", Decimals: 18, Available: "10000000000000000000", Locked: "0"},
		{UserID: "trader-1", ChainID: 1, TokenAddress: "0xtgr", Symbol: "TGR", Decimals: 18, Available: "1000000000000000000", Locked: "0"},
	} {
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed balance: %v", err)
		}
	}

	runner := NewBotRunner(db, nil, nil, nil)
	runner.SetPortfolioService(NewPortfolioService(db, prices))

	bot := models.BotInstance{
		UserID:   "trader-1",
		Strategy: models.StrategyRebalance,
		Symbol:   "BTC/RUSD",
		Config:   `{"targets":[{"symbol":"BTC","weight_bps":5000,"value":"0"},{"symbol":"ETH","weight_bps":3000,"value":"0"},{"symbol":"TGR","weight_bps":2000,"value":"0"}]}`,
	}

	raw, err := runner.rebalanceHoldings(context.Background(), bot)
	if err != nil {
		t.Fatalf("rebalanceHoldings: %v", err)
	}
	var holdings []struct {
		Symbol    string `json:"symbol"`
		WeightBps int64  `json:"weight_bps"`
		Value     string `json:"value"`
	}
	if err := json.Unmarshal(raw, &holdings); err != nil {
		t.Fatalf("decode holdings: %v", err)
	}
	if len(holdings) != 3 {
		t.Fatalf("expected three holdings, got %d", len(holdings))
	}
	bySymbol := map[string]string{}
	for _, h := range holdings {
		bySymbol[h.Symbol] = h.Value
	}
	// 1 BTC * 60000 = 60000; 10 ETH * 3000 = 30000; TGR is unpriced -> 0.
	checkValue := func(symbol string, want float64) {
		got, ok := new(big.Rat).SetString(bySymbol[symbol])
		if !ok {
			t.Errorf("%s value %q is not a decimal", symbol, bySymbol[symbol])
			return
		}
		actual, _ := got.Float64()
		if math.Abs(actual-want) > 1e-9 {
			t.Errorf("%s value = %s, want %v", symbol, bySymbol[symbol], want)
		}
	}
	checkValue("BTC", 60000)
	checkValue("ETH", 30000)
	checkValue("TGR", 0)
}

// TestRebalanceTargetsRequireWeights proves a mis-specified target set is
// rejected by the source, so the planner is never asked to normalise it.
func TestRebalanceTargetsRequireWeights(t *testing.T) {
	if _, err := rebalanceTargets(`{"targets":[]}`); err == nil {
		t.Fatal("an empty target set must be rejected")
	}
	if _, err := rebalanceTargets(`{}`); err == nil {
		t.Fatal("a configuration with no targets must be rejected")
	}
	targets, err := rebalanceTargets(`{"targets":[{"symbol":"BTC","weight_bps":10000,"value":"0"}]}`)
	if err != nil {
		t.Fatalf("valid targets rejected: %v", err)
	}
	if len(targets) != 1 || targets[0].Symbol != "BTC" || targets[0].WeightBps != 10000 {
		t.Fatalf("unexpected targets: %+v", targets)
	}
}

// TestValidateRebalanceConfigEnforcesTargetTotal proves the bot-creation
// validator accepts a well-formed target set and rejects weights that do not
// total 100%.
func TestValidateRebalanceConfigEnforcesTargetTotal(t *testing.T) {
	valid := json.RawMessage(`{"drift_threshold_bps":100,"targets":[{"symbol":"BTC","weight_bps":6000,"value":"1"},{"symbol":"ETH","weight_bps":4000,"value":"1"}]}`)
	if _, err := validateStrategyConfig(models.StrategyRebalance, valid); err != nil {
		t.Fatalf("valid rebalance config rejected: %v", err)
	}
	badTotal := json.RawMessage(`{"targets":[{"symbol":"BTC","weight_bps":5000,"value":"1"},{"symbol":"ETH","weight_bps":4000,"value":"1"}]}`)
	if _, err := validateStrategyConfig(models.StrategyRebalance, badTotal); err == nil {
		t.Fatal("weights that do not total 10000 must be rejected")
	}
	badValue := json.RawMessage(`{"targets":[{"symbol":"BTC","weight_bps":10000,"value":"-1"}]}`)
	if _, err := validateStrategyConfig(models.StrategyRebalance, badValue); err == nil {
		t.Fatal("a negative target value must be rejected")
	}
}

// TestPlaceOrdersTradesAnySymbolForRebalance proves the rebalance strategy may
// place orders on the symbols its plan named, while a single-symbol strategy is
// still held to its own symbol.
func TestPlaceOrdersTradesAnySymbolForRebalance(t *testing.T) {
	db := newBotsTestDB(t)
	bots := NewBotsService(db)
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 60000, "ETH": 3000})
	engine.markets["ETH/RUSD"] = &perpMarketConfig{
		Symbol:       "ETH/RUSD",
		OracleSymbol: "ETH",
		MaxLeverage:  50,
	}

	rebalance := models.BotInstance{
		UserID:        "trader-1",
		Name:          "rb",
		Strategy:      models.StrategyRebalance,
		Symbol:        "BTC/RUSD",
		Status:        models.BotStatusActive,
		Config:        `{"targets":[{"symbol":"BTC","weight_bps":5000,"value":"1"},{"symbol":"ETH","weight_bps":5000,"value":"1"}]}`,
		MaxOrderSize:  "100000",
		MaxDailyLoss:  "0",
		MaxOpenOrders: 5,
	}
	if err := db.Create(&rebalance).Error; err != nil {
		t.Fatalf("create rebalance bot: %v", err)
	}

	runner := NewBotRunner(db, bots, engine, nil)
	orders := []botengine.Order{
		{Symbol: "ETH/RUSD", Side: "buy", Kind: "market", Quantity: "1", Source: "rebalance"},
	}
	placed, err := runner.placeOrders(context.Background(), rebalance, orders, "")
	if err != nil {
		t.Fatalf("placeOrders for rebalance: %v", err)
	}
	if len(placed) != 1 || placed[0].Symbol != "ETH/RUSD" {
		t.Fatalf("rebalance order for another listed symbol must be accepted, got %+v", placed)
	}

	// A market maker is still held to its own symbol.
	maker := models.BotInstance{
		UserID:        "trader-2",
		Name:          "mm",
		Strategy:      models.StrategyMarketMaker,
		Symbol:        "BTC/RUSD",
		Status:        models.BotStatusActive,
		Config:        `{"spread_bps":10,"target_depth":"1"}`,
		MaxOrderSize:  "100000",
		MaxDailyLoss:  "0",
		MaxOpenOrders: 5,
	}
	if err := db.Create(&maker).Error; err != nil {
		t.Fatalf("create maker bot: %v", err)
	}
	_, err = runner.placeOrders(context.Background(), maker, []botengine.Order{
		{Symbol: "ETH/RUSD", Side: "buy", Kind: "limit", Price: "3000", Quantity: "1", Source: "mm"},
	}, "")
	if err == nil {
		t.Fatal("a single-symbol bot must not place an order for another symbol")
	}
}

// TestLiquidationPositionReadsRealMarginState proves the liquidation source
// renders an account's real collateral, debt and the listing's own risk
// parameters, so the engine's decision is grounded in live state.
func TestLiquidationPositionReadsRealMarginState(t *testing.T) {
	ctx := context.Background()
	env := newMarginTestEnv(t, map[string]float64{"BTC": 60000, "RUSD": 1})
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	account, err := env.engine.OpenAccount(ctx, "user-1", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}
	if err := env.engine.DepositCollateral(ctx, "user-1", account.ID, "BTC", "1"); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if err := env.engine.Borrow(ctx, "user-1", account.ID, "RUSD", "42000"); err != nil {
		t.Fatalf("borrow: %v", err)
	}

	runner := NewBotRunner(env.db, nil, nil, nil)
	runner.SetMarginEngine(env.engine)

	bot := models.BotInstance{Strategy: models.StrategyLiquidation, Symbol: "BTC/RUSD"}
	raw, err := runner.liquidationPosition(ctx, bot, LiquidationCandidate{AccountID: account.ID.String()})
	if err != nil {
		t.Fatalf("liquidationPosition: %v", err)
	}
	var position struct {
		ID                   string  `json:"id"`
		CollateralValue      string  `json:"collateral_value"`
		DebtValue            string  `json:"debt_value"`
		LiquidationThreshold string  `json:"liquidation_threshold"`
		LiquidationBonusBps  float64 `json:"liquidation_bonus_bps"`
	}
	if err := json.Unmarshal(raw, &position); err != nil {
		t.Fatalf("decode position: %v", err)
	}
	if position.ID != account.ID.String() {
		t.Errorf("position id = %s, want %s", position.ID, account.ID)
	}
	if collateral, _ := parsePositiveFloat(position.CollateralValue); math.Abs(collateral-60000) > 1 {
		t.Errorf("collateral = %s, want ~60000", position.CollateralValue)
	}
	if debt, _ := parsePositiveFloat(position.DebtValue); math.Abs(debt-42000) > 1 {
		t.Errorf("debt = %s, want ~42000", position.DebtValue)
	}
	if threshold, _ := parsePositiveFloat(position.LiquidationThreshold); math.Abs(threshold-0.75) > 1e-9 {
		t.Errorf("liquidation threshold = %s, want 0.75", position.LiquidationThreshold)
	}
	if position.LiquidationBonusBps != 500 {
		t.Errorf("liquidation bonus bps = %v, want 500", position.LiquidationBonusBps)
	}
}

// TestPerpOrderBaseQuantityConvertsNotional proves a notional order is rendered
// as base units at its own price, and an order with no price is refused.
func TestPerpOrderBaseQuantityConvertsNotional(t *testing.T) {
	quantity, err := perpOrderBaseQuantity(&PerpetualOrder{Size: "60000", Price: 60000})
	if err != nil {
		t.Fatalf("perpOrderBaseQuantity: %v", err)
	}
	if quantity != "1" {
		t.Errorf("base quantity = %s, want 1", quantity)
	}
	if _, err := perpOrderBaseQuantity(&PerpetualOrder{Size: "100", Price: 0}); err == nil {
		t.Fatal("an order with no price must be refused")
	}
}

// TestCopyTraderAddressReadsConfig proves the copy source resolves the followed
// trader from the validated configuration and rejects a missing one.
func TestCopyTraderAddressReadsConfig(t *testing.T) {
	address, err := copyTraderAddress(`{"trader_address":"0xAbC0000000000000000000000000000000000001","scale_bps":5000}`)
	if err != nil {
		t.Fatalf("copyTraderAddress: %v", err)
	}
	if address != "0xabc0000000000000000000000000000000000001" {
		t.Errorf("address = %s, want lower-cased", address)
	}
	if _, err := copyTraderAddress(`{"scale_bps":5000}`); err == nil {
		t.Fatal("a copy config with no trader_address must be rejected")
	}
}

// TestPerpCopySourceRendersRestingOrders proves the copy source mirrors the
// followed trader's own working orders and nothing else.
func TestPerpCopySourceRendersRestingOrders(t *testing.T) {
	ctx := context.Background()
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 60000})
	if _, err := engine.CreateOrder(ctx, "0xleader", "BTC/RUSD", "long", "limit", "60000", 59000, 0, 3600); err != nil {
		t.Fatalf("leader order: %v", err)
	}
	// A second order on another listed market must not be mirrored by a bot
	// configured for BTC/RUSD.
	engine.markets["ETH/RUSD"] = &perpMarketConfig{Symbol: "ETH/RUSD", OracleSymbol: "ETH", MaxLeverage: 50}
	if _, err := engine.CreateOrder(ctx, "0xleader", "ETH/RUSD", "long", "limit", "3000", 3000, 0, 3600); err != nil {
		t.Fatalf("leader second order: %v", err)
	}

	source := NewPerpCopySource(engine)
	raw, err := source.Signals(ctx, models.BotInstance{
		Symbol: "BTC/RUSD",
		Config: `{"trader_address":"0xleader","scale_bps":5000}`,
	})
	if err != nil {
		t.Fatalf("Signals: %v", err)
	}
	var signals []struct {
		Symbol   string `json:"symbol"`
		Market   string `json:"market"`
		Side     string `json:"side"`
		Kind     string `json:"kind"`
		Quantity string `json:"quantity"`
	}
	if err := json.Unmarshal(raw, &signals); err != nil {
		t.Fatalf("decode signals: %v", err)
	}
	if len(signals) != 1 {
		t.Fatalf("expected one signal, got %d", len(signals))
	}
	if signals[0].Symbol != "BTC/RUSD" || signals[0].Side != "buy" || signals[0].Market != "perp" {
		t.Fatalf("unexpected signal: %+v", signals[0])
	}
	// 60000 notional at 59000 is ~1.0169 BTC.
	if quantity, _ := parsePositiveFloat(signals[0].Quantity); math.Abs(quantity-60000.0/59000.0) > 1e-6 {
		t.Errorf("signal quantity = %s, want ~1.0169", signals[0].Quantity)
	}
}

// TestSplitPairRejectsNonPairs proves a symbol that is not a BASE/QUOTE pair is
// rejected so a single-token bot is never fed to a cross-venue strategy.
func TestSplitPairRejectsNonPairs(t *testing.T) {
	if _, _, ok := splitPair("BTC"); ok {
		t.Fatal("a single-token symbol must not split")
	}
	if _, _, ok := splitPair("BTC/"); ok {
		t.Fatal("a missing quote must not split")
	}
	base, quote, ok := splitPair(" btc/rusd ")
	if !ok || base != "BTC" || quote != "RUSD" {
		t.Fatalf("splitPair = %q/%q/%v, want BTC/RUSD/true", base, quote, ok)
	}
}

// TestApplyBpsRatScalesProportionally proves the basis-point helper scales a
// price up for an ask and down for a bid, and never returns a negative price.
func TestApplyBpsRatScalesProportionally(t *testing.T) {
	mid := parseTestRat(t, "100")
	if got := applyBpsRat(mid, 30); got.RatString() != "1003/10" {
		t.Errorf("ask = %s, want 1003/10", got.RatString())
	}
	if got := applyBpsRat(mid, -30); got.RatString() != "997/10" {
		t.Errorf("bid = %s, want 997/10", got.RatString())
	}
	if got := applyBpsRat(mid, -20000); got.Sign() != 0 {
		t.Errorf("a bid below zero must clamp to zero, got %s", got.RatString())
	}
}

func parseTestRat(t *testing.T, value string) *big.Rat {
	t.Helper()
	rat, ok := new(big.Rat).SetString(value)
	if !ok {
		t.Fatalf("could not parse %s", value)
	}
	return rat
}
