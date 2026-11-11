package services

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"

	"tigerswap/backend/models"
	"tigerswap/backend/services/botengine"
)

// TestBookSnapshotAggregatesRestingOrders proves the snapshot a bot reasons
// about is built from the engine's own resting orders and its own oracle mark.
// A long limit becomes a bid and a short limit an ask, and a size expressed as
// notional is converted back to base units through the order's price.
func TestBookSnapshotAggregatesRestingOrders(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 60000})
	ctx := context.Background()

	// Two long limits at the same price aggregate into one bid level.
	if _, err := engine.CreateOrder(ctx, "maker", "BTC/RUSD", "long", "limit", "5990", 59900, 0, 3600); err != nil {
		t.Fatalf("create long order: %v", err)
	}
	if _, err := engine.CreateOrder(ctx, "maker", "BTC/RUSD", "long", "limit", "5990", 59900, 0, 3600); err != nil {
		t.Fatalf("create second long order: %v", err)
	}
	if _, err := engine.CreateOrder(ctx, "maker", "BTC/RUSD", "short", "limit", "6010", 60100, 0, 3600); err != nil {
		t.Fatalf("create short order: %v", err)
	}

	snapshot, err := engine.BookSnapshot(ctx, "BTC/RUSD", 20)
	if err != nil {
		t.Fatalf("BookSnapshot: %v", err)
	}
	if snapshot.VWAP != "60000" {
		t.Errorf("vwap = %s, want the oracle mark 60000", snapshot.VWAP)
	}
	if len(snapshot.Bids) != 1 || len(snapshot.Asks) != 1 {
		t.Fatalf("expected one bid and one ask level, got %d and %d", len(snapshot.Bids), len(snapshot.Asks))
	}
	if snapshot.Bids[0].Price != "59900" {
		t.Errorf("best bid = %s, want 59900", snapshot.Bids[0].Price)
	}
	if snapshot.Asks[0].Price != "60100" {
		t.Errorf("best ask = %s, want 60100", snapshot.Asks[0].Price)
	}
	// 5990 notional at 59900 is 0.1 base units per order, so 0.2 aggregated.
	if snapshot.Bids[0].Quantity != "0.2" {
		t.Errorf("bid quantity = %s, want 0.2 (two 0.1 orders)", snapshot.Bids[0].Quantity)
	}
}

// TestBookSnapshotRefusesAnUnlistedMarket verifies a market with no listing is
// refused rather than handed a snapshot with an invented price.
func TestBookSnapshotRefusesAnUnlistedMarket(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 60000})
	if _, err := engine.BookSnapshot(context.Background(), "DOGE/RUSD", 20); err == nil {
		t.Fatal("BookSnapshot must refuse an unlisted market")
	}
}

// TestPlaceOrdersEnforcesRiskLimits drives the real placement path with a real
// perpetual engine and a real database: an order over max_order_size is dropped
// and an order within it becomes a real resting order.
func TestPlaceOrdersEnforcesRiskLimits(t *testing.T) {
	db := newBotsTestDB(t)
	bots := NewBotsService(db)
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 60000})

	bot := models.BotInstance{
		UserID:        "trader-1",
		Name:          "mm",
		Strategy:      models.StrategyMarketMaker,
		Symbol:        "BTC/RUSD",
		Status:        models.BotStatusActive,
		Config:        `{"spread_bps":10,"target_depth":"1.5"}`,
		MaxOrderSize:  "10000",
		MaxDailyLoss:  "0",
		MaxOpenOrders: 5,
	}
	if err := db.Create(&bot).Error; err != nil {
		t.Fatalf("create bot: %v", err)
	}

	runner := NewBotRunner(db, bots, engine, nil)
	orders := []botengine.Order{
		// 1 * 60000 = 60000, over the 10000 limit: must be dropped.
		{Symbol: "BTC/RUSD", Side: "buy", Kind: "limit", Price: "60000", Quantity: "1", Source: "test"},
		// 0.1 * 60000 = 6000, within the limit: must rest as a real order.
		{Symbol: "BTC/RUSD", Side: "buy", Kind: "limit", Price: "59000", Quantity: "0.1", Source: "test"},
	}

	placed, err := runner.placeOrders(context.Background(), bot, orders, "60000")
	if err != nil {
		t.Fatalf("placeOrders: %v", err)
	}
	if len(placed) != 1 {
		t.Fatalf("expected one accepted order, got %d", len(placed))
	}
	if placed[0].Price != "59000" {
		t.Errorf("accepted order price = %s, want 59000", placed[0].Price)
	}

	// The accepted order must be a real resting order on the engine.
	pending, err := bots.pendingOrderCount(bot.ID)
	if err != nil {
		t.Fatalf("pendingOrderCount: %v", err)
	}
	if len(engine.orders) != 1 {
		t.Fatalf("expected one resting order on the engine, got %d", len(engine.orders))
	}
	_ = pending
}

// TestPlaceOrdersHonoursMaxOpenOrders verifies the runner stops placing once the
// bot has reached its working-order ceiling.
func TestPlaceOrdersHonoursMaxOpenOrders(t *testing.T) {
	db := newBotsTestDB(t)
	bots := NewBotsService(db)
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 60000})

	bot := models.BotInstance{
		UserID: "trader-1", Name: "mm", Strategy: models.StrategyMarketMaker,
		Symbol: "BTC/RUSD", Status: models.BotStatusActive, Config: `{}`,
		MaxOrderSize: "10000", MaxDailyLoss: "0", MaxOpenOrders: 1,
	}
	if err := db.Create(&bot).Error; err != nil {
		t.Fatalf("create bot: %v", err)
	}
	// One order is already working.
	if err := db.Create(&models.BotOrder{
		BotID: bot.ID, UserID: bot.UserID, Symbol: bot.Symbol, Side: "buy",
		OrderKind: "limit", Price: "58000", Quantity: "0.1", Source: "test", Status: "pending",
	}).Error; err != nil {
		t.Fatalf("seed working order: %v", err)
	}

	runner := NewBotRunner(db, bots, engine, nil)
	orders := []botengine.Order{
		{Symbol: "BTC/RUSD", Side: "buy", Kind: "limit", Price: "59000", Quantity: "0.1", Source: "test"},
	}
	placed, err := runner.placeOrders(context.Background(), bot, orders, "60000")
	if err != nil {
		t.Fatalf("placeOrders: %v", err)
	}
	if len(placed) != 0 {
		t.Fatalf("expected no orders once the ceiling is reached, got %d", len(placed))
	}
	if len(engine.orders) != 0 {
		t.Fatalf("expected no resting orders on the engine, got %d", len(engine.orders))
	}
}

// TestOrderNotionalIsExact verifies the sizing arithmetic is exact and refuses a
// non-positive price.
func TestOrderNotionalIsExact(t *testing.T) {
	notional, err := orderNotional("0.1", "60000")
	if err != nil {
		t.Fatalf("orderNotional: %v", err)
	}
	if notional.Cmp(new(big.Rat).SetInt64(6000)) != 0 {
		t.Errorf("notional = %s, want 6000", notional.RatString())
	}
	if _, err := orderNotional("0.1", "0"); err == nil {
		t.Error("a zero price must be refused")
	}
	if _, err := orderNotional("-1", "60000"); err == nil {
		t.Error("a negative quantity must be refused")
	}
}

// TestEnsureLoadedOnlyReconfiguresOnChange verifies the engine is configured
// once and only reconfigured when the stored configuration changes, so a
// running bot's state is not reset on every sweep.
func TestEnsureLoadedOnlyReconfiguresOnChange(t *testing.T) {
	db := newBotsTestDB(t)
	bots := NewBotsService(db)
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 60000})

	fake := &recordingEngine{}
	runner := NewBotRunner(db, bots, engine, fake)

	bot := models.BotInstance{
		ID: 7, Strategy: models.StrategyGrid, Symbol: "BTC/RUSD",
		Config: `{"lower_price":"1","upper_price":"2","levels":2,"order_size":"1"}`,
	}
	if err := runner.ensureLoaded(bot); err != nil {
		t.Fatalf("ensureLoaded: %v", err)
	}
	if fake.upserts != 1 {
		t.Fatalf("expected one upsert, got %d", fake.upserts)
	}
	// Same config again: no second upsert.
	if err := runner.ensureLoaded(bot); err != nil {
		t.Fatalf("ensureLoaded: %v", err)
	}
	if fake.upserts != 1 {
		t.Fatalf("expected the config to be loaded once, got %d upserts", fake.upserts)
	}
	// Changed config: a reconfiguration.
	bot.Config = `{"lower_price":"1","upper_price":"3","levels":3,"order_size":"1"}`
	if err := runner.ensureLoaded(bot); err != nil {
		t.Fatalf("ensureLoaded: %v", err)
	}
	if fake.upserts != 2 {
		t.Fatalf("expected a second upsert after a config change, got %d", fake.upserts)
	}
}

// recordingEngine is a minimal test double for the C ABI boundary. It records
// the calls the control plane makes so the lifecycle wiring can be asserted
// without linking the Rust library.
type recordingEngine struct {
	upserts  int
	removes  int
	lastConf json.RawMessage
}

func (e *recordingEngine) Upsert(_ uint64, _ string, config json.RawMessage) error {
	e.upserts++
	e.lastConf = config
	return nil
}
func (e *recordingEngine) Remove(_ uint64) error { e.removes++; return nil }
func (e *recordingEngine) Step(_ uint64, _ json.RawMessage) ([]botengine.Order, error) {
	return nil, nil
}
func (e *recordingEngine) Arbitrage(_ uint64, _ json.RawMessage) ([]botengine.Order, error) {
	return nil, nil
}
func (e *recordingEngine) Sniper(_ uint64, _ json.RawMessage) (*botengine.Order, error) {
	return nil, nil
}
func (e *recordingEngine) Rebalance(_ uint64, _ json.RawMessage) ([]botengine.RebalanceLeg, error) {
	return nil, nil
}
func (e *recordingEngine) Liquidation(_ uint64, _ json.RawMessage) (*botengine.Liquidation, error) {
	return nil, nil
}
func (e *recordingEngine) Copy(_ uint64, _ json.RawMessage) ([]botengine.Order, error) {
	return nil, nil
}
func (e *recordingEngine) Count() uint64 { return 0 }
func (e *recordingEngine) Close()        {}

// TestSweepUnloadsInactiveBots verifies a bot that is no longer active is
// removed from the engine so it stops trading.
func TestSweepUnloadsInactiveBots(t *testing.T) {
	db := newBotsTestDB(t)
	bots := NewBotsService(db)
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 60000})
	fake := &recordingEngine{}
	runner := NewBotRunner(db, bots, engine, fake)

	active := models.BotInstance{
		UserID: "u", Name: "grid", Strategy: models.StrategyGrid, Symbol: "BTC/RUSD",
		Status: models.BotStatusActive, Config: `{}`, MaxOrderSize: "1", MaxOpenOrders: 1,
	}
	if err := db.Create(&active).Error; err != nil {
		t.Fatalf("create bot: %v", err)
	}
	if err := runner.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if fake.upserts != 1 {
		t.Fatalf("expected the active bot to be loaded, got %d upserts", fake.upserts)
	}

	// Pause the bot: the next sweep must unload it.
	if err := db.Model(&models.BotInstance{}).Where("id = ?", active.ID).
		Update("status", models.BotStatusPaused).Error; err != nil {
		t.Fatalf("pause bot: %v", err)
	}
	if err := runner.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if fake.removes != 1 {
		t.Fatalf("expected the paused bot to be unloaded, got %d removes", fake.removes)
	}
}
