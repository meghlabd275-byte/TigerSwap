// Package botengine bridges the Go control plane to the Rust bot engine.
//
// The strategy arithmetic lives in the Rust `tigerswap-bots` crate, which is
// compiled to a C ABI and linked into the server. This package is the only
// place that crosses the language boundary: it turns Go values into the JSON
// documents the engine expects and turns its replies back into typed results.
//
// The bridge is built only with the `rust` build tag (see bridge_cgo.go). A
// default build has no engine, and the control plane reports that explicitly
// rather than pretending bots run.
package botengine

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrUnavailable is returned when the server was built without the Rust engine.
var ErrUnavailable = errors.New("bot engine is not built into this binary (build with -tags rust)")

// Order is an order the engine decided to place. Prices and quantities are
// exact decimal strings, never floats.
type Order struct {
	Symbol     string `json:"symbol"`
	Side       string `json:"side"`
	Kind       string `json:"kind"`
	Price      string `json:"price"`
	Quantity   string `json:"quantity"`
	ReduceOnly bool   `json:"reduce_only"`
	Source     string `json:"source"`
}

// RebalanceLeg is one trade the rebalance planner wants.
type RebalanceLeg struct {
	Symbol   string `json:"symbol"`
	Side     string `json:"side"`
	Amount   string `json:"amount"`
	DriftBps int64  `json:"drift_bps"`
}

// Liquidation is the debt and collateral of one liquidation the engine wants.
type Liquidation struct {
	PositionID       string `json:"position_id"`
	RepayAmount      string `json:"repay_amount"`
	CollateralSeized string `json:"collateral_seized"`
	BonusEarned      string `json:"bonus_earned"`
	HealthFactor     string `json:"health_factor"`
}

// Engine drives the Rust bot engine. Every method is safe for concurrent use.
type Engine interface {
	// Upsert configures or replaces the bot with the given id.
	Upsert(id uint64, strategy string, config json.RawMessage) error
	// Remove forgets a bot.
	Remove(id uint64) error
	// Step advances a snapshot-driven bot and returns the orders it produced.
	Step(id uint64, snapshot json.RawMessage) ([]Order, error)
	// Arbitrage runs an arbitrage bot across the supplied venues.
	Arbitrage(id uint64, venues json.RawMessage) ([]Order, error)
	// Sniper evaluates a sniper against a new listing.
	Sniper(id uint64, listing json.RawMessage) (*Order, error)
	// Rebalance plans the trades that restore a portfolio to its targets.
	Rebalance(id uint64, holdings json.RawMessage) ([]RebalanceLeg, error)
	// Liquidation evaluates a borrow position.
	Liquidation(id uint64, position json.RawMessage) (*Liquidation, error)
	// Copy mirrors a followed trader's signals, scaled by the copy ratio.
	Copy(id uint64, signals json.RawMessage) ([]Order, error)
	// Count is the number of configured bots.
	Count() uint64
	// Close releases the engine.
	Close()
}

// envelope is the reply every FFI call returns.
type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
}

// decodeEnvelope validates an FFI reply and returns its data.
func decodeEnvelope(raw string) (json.RawMessage, error) {
	var env envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return nil, fmt.Errorf("bot engine returned a malformed reply: %w", err)
	}
	if !env.OK {
		if env.Error == "" {
			env.Error = "bot engine reported an unspecified failure"
		}
		return nil, errors.New(env.Error)
	}
	return env.Data, nil
}
