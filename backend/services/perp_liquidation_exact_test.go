package services

import "testing"

// TestLiquidationRequiredIsExactAtTheBoundary pins the perp health check to
// exact arithmetic. The decision is `equity/notional < maintenance`, evaluated
// with big.Rat, so a position sitting exactly on its maintenance level must not
// be liquidated by a binary-rounding artefact -- the same property
// TestMarginHealthIsExact pins for the margin engine.
//
// A long opened at 100 with size 10,000 and collateral 1,000 has equity
// `collateral + size*(mark-entry)/entry`. At a 5% maintenance rate the position
// is liquidatable only once equity < 5% * notional = 500, which happens when
// the mark falls below 95. The exact boundary is 95: at 95 equity is exactly
// 500 and the position must be left alone.
func TestLiquidationRequiredIsExactAtTheBoundary(t *testing.T) {
	cfg := &perpMarketConfig{
		Symbol:                "BTC/RUSD",
		OracleSymbol:          "BTC",
		MaxLeverage:           100,
		MaintenanceMarginRate: 0.05,
	}
	position := &PerpetualPosition{
		ID:          "p1",
		UserAddress: "0xabc",
		Pair:        "BTC/RUSD",
		Side:        "long",
		Size:        "10000",
		Collateral:  "1000",
		EntryPrice:  100,
		Status:      "open",
	}

	// Exactly on the maintenance level: equity == maintenance * notional == 500.
	if liquidationRequired(cfg, position, 95) {
		t.Fatal("a position sitting exactly on its maintenance level must not be liquidated")
	}
	// One tick of the smallest meaningful move below: equity < 500.
	if !liquidationRequired(cfg, position, 94.99) {
		t.Fatal("a position below its maintenance level must be liquidated")
	}
	// Comfortably above the boundary.
	if liquidationRequired(cfg, position, 100) {
		t.Fatal("a position above its maintenance level must not be liquidated")
	}
}

// TestLiquidationRequiredShortMirrorsLong proves the short side uses the same
// exact threshold with the PnL sign flipped: a short is liquidated when the mark
// rises through the level, not when it falls.
func TestLiquidationRequiredShortMirrorsLong(t *testing.T) {
	cfg := &perpMarketConfig{
		Symbol:                "BTC/RUSD",
		OracleSymbol:          "BTC",
		MaxLeverage:           100,
		MaintenanceMarginRate: 0.05,
	}
	position := &PerpetualPosition{
		ID:          "p1",
		UserAddress: "0xabc",
		Pair:        "BTC/RUSD",
		Side:        "short",
		Size:        "10000",
		Collateral:  "1000",
		EntryPrice:  100,
		Status:      "open",
	}

	// A short gains as the mark falls and loses as it rises. At mark 105 the
	// loss is exactly 500, leaving equity exactly at the maintenance level.
	if liquidationRequired(cfg, position, 105) {
		t.Fatal("a short sitting exactly on its maintenance level must not be liquidated")
	}
	if !liquidationRequired(cfg, position, 105.01) {
		t.Fatal("a short above its maintenance level must be liquidated")
	}
	if liquidationRequired(cfg, position, 100) {
		t.Fatal("a short below its maintenance level must not be liquidated")
	}
}

// TestLiquidationRequiredRejectsUnreadablePosition proves a position whose size
// or collateral cannot be parsed is never treated as liquidatable: a malformed
// row must not be settled against the trader.
func TestLiquidationRequiredRejectsUnreadablePosition(t *testing.T) {
	cfg := &perpMarketConfig{
		Symbol:                "BTC/RUSD",
		MaintenanceMarginRate: 0.05,
	}
	position := &PerpetualPosition{
		Pair:       "BTC/RUSD",
		Side:       "long",
		Size:       "not-a-number",
		Collateral: "1000",
		EntryPrice: 100,
		Status:     "open",
	}
	if liquidationRequired(cfg, position, 50) {
		t.Fatal("a position with an unreadable size must not be liquidated")
	}

	position.Size = "10000"
	position.Collateral = "not-a-number"
	if liquidationRequired(cfg, position, 50) {
		t.Fatal("a position with unreadable collateral must not be liquidated")
	}
}
