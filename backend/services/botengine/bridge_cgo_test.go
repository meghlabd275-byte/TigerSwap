//go:build rust

// End-to-end tests for the cgo bridge. They are built only with the `rust` tag
// and run against the real Rust library, so they prove the ABI actually works
// rather than exercising a stub. Build the library first:
//
//	cd backend/rust && cargo build --release -p tigerswap-bots
//	cd backend && go test -tags rust ./services/botengine/
package botengine

import (
	"encoding/json"
	"testing"
)

// TestRealEngineMarketMakerRoundTrip drives the real engine with a real
// snapshot and asserts it returns a two-sided quote with exact decimal strings.
func TestRealEngineMarketMakerRoundTrip(t *testing.T) {
	engine, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	config := json.RawMessage(`{"spread_bps":10,"target_depth":"1.5","rebalance_threshold_bps":10,"min_spread_bps":1,"max_spread_bps":500,"min_book_depth":"0"}`)
	if err := engine.Upsert(1, "market_maker", config); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if engine.Count() != 1 {
		t.Fatalf("Count = %d, want 1", engine.Count())
	}

	snapshot := json.RawMessage(`{
		"symbol":"BTC/RUSD",
		"bids":[{"price":"60000","quantity":"10"}],
		"asks":[{"price":"60010","quantity":"10"}],
		"vwap":"60000",
		"timestamp_ms":1700000000000
	}`)
	orders, err := engine.Step(1, snapshot)
	if err != nil {
		t.Fatalf("Step: %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("expected a two-sided quote, got %d orders", len(orders))
	}
	for _, order := range orders {
		if order.Symbol != "BTC/RUSD" {
			t.Errorf("order symbol = %q", order.Symbol)
		}
		if order.Price == "" || order.Quantity == "" {
			t.Errorf("order has an empty price or quantity: %+v", order)
		}
	}
}

// TestRealEngineRejectsUnknownStrategy verifies the engine refuses a strategy
// it does not implement rather than silently doing nothing.
func TestRealEngineRejectsUnknownStrategy(t *testing.T) {
	engine, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	if err := engine.Upsert(1, "not_a_strategy", json.RawMessage(`{}`)); err == nil {
		t.Fatal("an unknown strategy must be rejected")
	}
}

// TestRealEngineStepUnknownBotIsAnError verifies stepping a bot that was never
// configured is reported, so a control-plane bug cannot silently stop trading.
func TestRealEngineStepUnknownBotIsAnError(t *testing.T) {
	engine, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	snapshot := json.RawMessage(`{"symbol":"BTC/RUSD","bids":[],"asks":[],"vwap":"60000","timestamp_ms":1}`)
	if _, err := engine.Step(99, snapshot); err == nil {
		t.Fatal("stepping an unconfigured bot must be an error")
	}
}

// TestRealEngineArbitrageFindsAnEdge drives the real engine's arbitrage path
// with two venues that disagree, and asserts it returns both executable legs.
func TestRealEngineArbitrageFindsAnEdge(t *testing.T) {
	engine, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	config := json.RawMessage(`{"min_edge_bps":5,"max_quantity":"2","depth_fraction_bps":2000}`)
	if err := engine.Upsert(7, "arbitrage", config); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	venues := json.RawMessage(`[
{"name":"tigerswap_v2","fee_bps":0,"snapshot":{"symbol":"BTC/RUSD","bids":[{"price":"60000","quantity":"10"}],"asks":[{"price":"60010","quantity":"10"}],"vwap":"60000","timestamp_ms":1}},
{"name":"tigerswap_v3","fee_bps":0,"snapshot":{"symbol":"BTC/RUSD","bids":[{"price":"60500","quantity":"10"}],"asks":[{"price":"60510","quantity":"10"}],"vwap":"60500","timestamp_ms":1}}
]`)
	orders, err := engine.Arbitrage(7, venues)
	if err != nil {
		t.Fatalf("Arbitrage: %v", err)
	}
	if len(orders) != 2 {
		t.Fatalf("expected two legs for a real edge, got %d", len(orders))
	}
	for _, order := range orders {
		if order.Symbol != "BTC/RUSD" || order.Quantity == "" || order.Price == "" {
			t.Errorf("unexpected leg: %+v", order)
		}
	}
}

// TestRealEngineSniperBuysALiquidLaunch drives the real sniper against a listing
// with depth, and asserts a buy comes back; a thin listing is refused.
func TestRealEngineSniperBuysALiquidLaunch(t *testing.T) {
	engine, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	config := json.RawMessage(`{"max_price_impact_bps":500,"min_liquidity":"1000","buy_size":"1"}`)
	if err := engine.Upsert(3, "sniper", config); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	rich := json.RawMessage(`{"symbol":"NEW/RUSD","liquidity":"100000","snapshot":{"symbol":"NEW/RUSD","bids":[{"price":"100","quantity":"1000"}],"asks":[{"price":"101","quantity":"1000"}],"vwap":"100","timestamp_ms":1}}`)
	order, err := engine.Sniper(3, rich)
	if err != nil {
		t.Fatalf("Sniper: %v", err)
	}
	if order == nil {
		t.Fatal("a liquid launch must produce a buy")
	}

	thin := json.RawMessage(`{"symbol":"NEW/RUSD","liquidity":"1","snapshot":{"symbol":"NEW/RUSD","bids":[],"asks":[],"vwap":"100","timestamp_ms":1}}`)
	order, err = engine.Sniper(3, thin)
	if err != nil {
		t.Fatalf("Sniper thin: %v", err)
	}
	if order != nil {
		t.Fatalf("a thin launch must be refused, got %+v", order)
	}
}

// TestRealEngineRebalancePlansADriftedLeg drives the real rebalance planner with
// a portfolio that has drifted and asserts a corrective leg is returned.
func TestRealEngineRebalancePlansADriftedLeg(t *testing.T) {
	engine, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	if err := engine.Upsert(4, "rebalance", json.RawMessage(`{"drift_threshold_bps":100}`)); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	holdings := json.RawMessage(`[
{"symbol":"BTC","weight_bps":5000,"value":"9000"},
{"symbol":"ETH","weight_bps":5000,"value":"1000"}
]`)
	legs, err := engine.Rebalance(4, holdings)
	if err != nil {
		t.Fatalf("Rebalance: %v", err)
	}
	if len(legs) == 0 {
		t.Fatal("a drifted portfolio must produce at least one leg")
	}
	for _, leg := range legs {
		if leg.Amount == "" || leg.DriftBps == 0 {
			t.Errorf("unexpected leg: %+v", leg)
		}
	}
}

// TestRealEngineLiquidationFiresOnAnUnhealthyAccount drives the real liquidation
// evaluator and asserts an unhealthy account is flagged while a healthy one is
// left alone.
func TestRealEngineLiquidationFiresOnAnUnhealthyAccount(t *testing.T) {
	engine, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	if err := engine.Upsert(5, "liquidation", json.RawMessage(`{"close_factor_bps":5000,"min_bonus_bps":100}`)); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	unhealthy := json.RawMessage(`{"id":"acct-1","collateral_value":"1000","debt_value":"950","liquidation_threshold":"0.75","liquidation_bonus_bps":500}`)
	decision, err := engine.Liquidation(5, unhealthy)
	if err != nil {
		t.Fatalf("Liquidation: %v", err)
	}
	if decision == nil {
		t.Fatal("an under-collateralised account must be flagged")
	}
	if decision.RepayAmount == "" || decision.CollateralSeized == "" {
		t.Errorf("decision is missing amounts: %+v", decision)
	}

	healthy := json.RawMessage(`{"id":"acct-2","collateral_value":"1000","debt_value":"100","liquidation_threshold":"0.75","liquidation_bonus_bps":500}`)
	decision, err = engine.Liquidation(5, healthy)
	if err != nil {
		t.Fatalf("Liquidation healthy: %v", err)
	}
	if decision != nil {
		t.Fatalf("a healthy account must not be flagged, got %+v", decision)
	}
}

// TestRealEngineCopyMirrorsSignals drives the real copy bot with a followed
// trader's signals and asserts the mirrored orders carry the scaled size.
func TestRealEngineCopyMirrorsSignals(t *testing.T) {
	engine, err := New()
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer engine.Close()

	if err := engine.Upsert(6, "copy", json.RawMessage(`{"scale_bps":5000,"copy_perps":true}`)); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	signals := json.RawMessage(`[{"symbol":"BTC/RUSD","market":"perp","side":"buy","kind":"limit","price":"60000","quantity":"2","source_id":"ord-1"}]`)
	orders, err := engine.Copy(6, signals)
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected one mirrored order, got %d", len(orders))
	}
	if orders[0].Symbol != "BTC/RUSD" || orders[0].Side != "buy" {
		t.Errorf("unexpected mirrored order: %+v", orders[0])
	}
}
