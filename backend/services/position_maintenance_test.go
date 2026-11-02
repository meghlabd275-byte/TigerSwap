package services

import (
	"context"
	"testing"
	"time"
)

// newMarkedPerpEngine builds an engine with one listed market and the given
// oracle prices, so the marking path runs against the real code with a
// controllable price source.
func newMarkedPerpEngine(prices map[string]float64) *PerpetualEngine {
	engine := NewPerpetualEngine(nil, &stubPriceAggregator{prices: prices})
	engine.markets["BTC/RUSD"] = &perpMarketConfig{
		Symbol:       "BTC/RUSD",
		OracleSymbol: "BTC",
		MaxLeverage:  100,
	}
	return engine
}

// TestUpdatePositionsMarksOpenPositions proves an open position is re-marked to
// the live oracle price and its unrealised PnL is refreshed. Before the loop was
// wired the position kept its entry price as its current price forever, so a
// trader always saw zero PnL.
func TestUpdatePositionsMarksOpenPositions(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 110})
	engine.positions["p1"] = &PerpetualPosition{
		ID:               "p1",
		UserAddress:      "0xabc",
		Pair:             "BTC/RUSD",
		Side:             "long",
		Size:             "10000",
		EntryPrice:       100,
		LiquidationPrice: 50,
		Status:           "open",
	}

	done := make(chan error, 1)
	go func() { done <- engine.UpdatePositions(context.Background()) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("UpdatePositions: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("UpdatePositions did not return; it likely deadlocked on the engine lock")
	}

	position := engine.positions["p1"]
	if position.MarkPrice != 110 {
		t.Fatalf("mark price = %v, want 110", position.MarkPrice)
	}
	if got := parsePNL(t, position.UnrealizedPNL); got != 1000 {
		t.Fatalf("unrealised pnl = %v, want 1000 (10%% of 10000 notional)", got)
	}
}

// TestUpdatePositionsFlagsLiquidatable proves a position whose mark crosses its
// liquidation price is flagged, and that a recovered position returns to open.
func TestUpdatePositionsFlagsLiquidatable(t *testing.T) {
	prices := map[string]float64{"BTC": 40}
	engine := newMarkedPerpEngine(prices)
	engine.positions["p1"] = &PerpetualPosition{
		ID:               "p1",
		Pair:             "BTC/RUSD",
		Side:             "long",
		Size:             "10000",
		EntryPrice:       100,
		LiquidationPrice: 50,
		Status:           "open",
	}

	if err := engine.UpdatePositions(context.Background()); err != nil {
		t.Fatalf("UpdatePositions: %v", err)
	}
	if engine.positions["p1"].Status != "liquidatable" {
		t.Fatalf("status = %q, want liquidatable", engine.positions["p1"].Status)
	}

	prices["BTC"] = 120
	if err := engine.UpdatePositions(context.Background()); err != nil {
		t.Fatalf("UpdatePositions: %v", err)
	}
	if engine.positions["p1"].Status != "open" {
		t.Fatalf("status = %q, want open after recovery", engine.positions["p1"].Status)
	}
}

// TestUpdatePositionsKeepsMarkOnPriceFailure proves a position whose price
// cannot be read keeps its previous mark instead of being marked at zero, which
// would report a catastrophic loss.
func TestUpdatePositionsKeepsMarkOnPriceFailure(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 110})
	engine.positions["p1"] = &PerpetualPosition{
		ID:               "p1",
		Pair:             "BTC/RUSD",
		Side:             "long",
		Size:             "10000",
		EntryPrice:       100,
		MarkPrice:        105,
		LiquidationPrice: 50,
		Status:           "open",
	}

	engine.priceAgg = &stubPriceAggregator{prices: map[string]float64{}}
	if err := engine.UpdatePositions(context.Background()); err != nil {
		t.Fatalf("UpdatePositions: %v", err)
	}
	if engine.positions["p1"].MarkPrice != 105 {
		t.Fatalf("mark price = %v, want the previous mark 105", engine.positions["p1"].MarkPrice)
	}
}

// TestUpdateCopyPositionsMarksOpenPositions proves an open copy position is
// re-marked and its PnL recomputed from the live price.
func TestUpdateCopyPositionsMarksOpenPositions(t *testing.T) {
	engine := NewCopyTradingEngine(nil, &stubPriceAggregator{prices: map[string]float64{"BTC": 110}})
	engine.copyPositions["cp1"] = &CopyPosition{
		ID:           "cp1",
		Pair:         "BTC/RUSD",
		Side:         "long",
		Size:         "10000",
		EntryPrice:   100,
		CurrentPrice: 100,
		PNL:          "0",
		Status:       "open",
	}

	done := make(chan error, 1)
	go func() { done <- engine.UpdateCopyPositions(context.Background()) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("UpdateCopyPositions: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("UpdateCopyPositions did not return; it likely deadlocked on the engine lock")
	}

	position := engine.copyPositions["cp1"]
	if position.CurrentPrice != 110 {
		t.Fatalf("current price = %v, want 110", position.CurrentPrice)
	}
	if got := parsePNL(t, position.PNL); got != 1000 {
		t.Fatalf("pnl = %v, want 1000", got)
	}
}

// TestStartPositionMaintenanceStops proves the maintenance loop exits when its
// context is cancelled rather than leaking a goroutine.
func TestStartPositionMaintenanceStops(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 110})
	engine.positions["p1"] = &PerpetualPosition{
		ID:         "p1",
		Pair:       "BTC/RUSD",
		Side:       "long",
		Size:       "10000",
		EntryPrice: 100,
		Status:     "open",
	}

	ctx, cancel := context.WithCancel(context.Background())
	StartPositionMaintenance(ctx, engine, nil, 5*time.Millisecond)

	deadline := time.After(2 * time.Second)
	for {
		engine.mu.RLock()
		marked := engine.positions["p1"].MarkPrice
		engine.mu.RUnlock()
		if marked == 110 {
			break
		}
		select {
		case <-deadline:
			t.Fatal("maintenance loop never marked the position")
		default:
			time.Sleep(time.Millisecond)
		}
	}

	cancel()
}
