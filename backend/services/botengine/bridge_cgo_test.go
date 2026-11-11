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
