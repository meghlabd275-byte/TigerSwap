package services

import (
	"math/big"
	"testing"
)

// TestPerpLiquidationMatchesConfiguredMaintenanceMargin pins the off-chain
// health check to the listing's own maintenance-margin rate. The engine used to
// derive a liquidation price from a hardcoded 0.5 buffer and ignore the
// configured rate entirely, so a listing with any other maintenance margin was
// liquidated at the wrong price. The on-chain contract
// (TigerSwapPerpetuals.liquidatePosition) and the keeper both decide on
// equity/notional against the real rate; the engine has to agree with them or a
// position is liquidatable on one side and healthy on the other.
func TestPerpLiquidationMatchesConfiguredMaintenanceMargin(t *testing.T) {
	// 10x long from 100 with a 5% maintenance margin: the maintenance level sits
	// at equity/notional = 0.05, i.e. price 95. A price of 94 is below it and must
	// be liquidatable; a price of 96 is above it and must not be.
	cfg := &perpMarketConfig{Symbol: "BTC/RUSD", MaxLeverage: 100, MaintenanceMarginRate: 0.05}
	position := &PerpetualPosition{
		Pair: "BTC/RUSD", Side: "long", Size: "10000", Collateral: "1000",
		EntryPrice: 100, Leverage: 10,
	}

	if !liquidationRequired(cfg, position, 94) {
		t.Errorf("price 94 with a 5%% maintenance margin must be liquidatable")
	}
	if liquidationRequired(cfg, position, 96) {
		t.Errorf("price 96 with a 5%% maintenance margin must not be liquidatable")
	}

	// A 0.5 hardcoded buffer would have put the boundary at 95 for every listing
	// regardless of rate. With a 20% maintenance margin and 3,000 of margin the
	// boundary moves to price 90, so 85 is liquidatable even though the old
	// formula called it safe.
	tight := &perpMarketConfig{Symbol: "BTC/RUSD", MaxLeverage: 100, MaintenanceMarginRate: 0.20}
	tightPosition := &PerpetualPosition{
		Pair: "BTC/RUSD", Side: "long", Size: "10000", Collateral: "3000",
		EntryPrice: 100, Leverage: 3,
	}
	if !liquidationRequired(tight, tightPosition, 85) {
		t.Errorf("price 85 with a 20%% maintenance margin must be liquidatable")
	}
	if liquidationRequired(tight, tightPosition, 91) {
		t.Errorf("price 91 with a 20%% maintenance margin must not be liquidatable")
	}
}

// TestPerpLiquidationShortSideIsSymmetric checks the short side of the same
// boundary so a sign error cannot hide behind the long-side test.
func TestPerpLiquidationShortSideIsSymmetric(t *testing.T) {
	cfg := &perpMarketConfig{Symbol: "BTC/RUSD", MaxLeverage: 100, MaintenanceMarginRate: 0.05}
	position := &PerpetualPosition{
		Pair: "BTC/RUSD", Side: "short", Size: "10000", Collateral: "1000",
		EntryPrice: 100, Leverage: 10,
	}
	if !liquidationRequired(cfg, position, 106) {
		t.Errorf("price 106 with a 5%% maintenance margin must be liquidatable for a short")
	}
	if liquidationRequired(cfg, position, 104) {
		t.Errorf("price 104 with a 5%% maintenance margin must not be liquidatable for a short")
	}
}

// TestPerpLiquidationIsExactAtTheBoundary is the reason the decision is computed
// in rational arithmetic: at exactly the maintenance level the position is not
// yet liquidatable (the contract requires strictly below maintenance), and a
// binary-rounding comparison would flip that verdict.
func TestPerpLiquidationIsExactAtTheBoundary(t *testing.T) {
	cfg := &perpMarketConfig{Symbol: "BTC/RUSD", MaxLeverage: 100, MaintenanceMarginRate: 0.05}
	position := &PerpetualPosition{
		Pair: "BTC/RUSD", Side: "long", Size: "10000", Collateral: "1000",
		EntryPrice: 100, Leverage: 10,
	}
	// equity at price 95 = 1000 + (95-100)/100*10000 = 500; notional = 10000;
	// ratio = 0.05 exactly = maintenance, so it is NOT below maintenance.
	if liquidationRequired(cfg, position, 95) {
		t.Errorf("a position exactly at maintenance must not be liquidated")
	}
	// One hundredth of a price unit lower is strictly below and must liquidate.
	if !liquidationRequired(cfg, position, 94.99) {
		t.Errorf("a position just below maintenance must be liquidated")
	}
}

// TestLiquidationPriceMatchesMaintenanceLevel checks the displayed liquidation
// price is derived from the same rate the health check uses, so the number a
// trader sees is the number that decides their liquidation.
func TestLiquidationPriceMatchesMaintenanceLevel(t *testing.T) {
	cfg := &perpMarketConfig{Symbol: "BTC/RUSD", MaxLeverage: 100, MaintenanceMarginRate: 0.05}
	size := big.NewRat(10000, 1)
	collateral := big.NewRat(1000, 1)
	price, err := liquidationPriceRat(cfg, 100, size, collateral, true)
	if err != nil {
		t.Fatalf("liquidationPriceRat: %v", err)
	}
	want := big.NewRat(95, 1)
	if price.Cmp(want) != 0 {
		t.Errorf("long liquidation price = %s, want 95", price.RatString())
	}

	shortPrice, err := liquidationPriceRat(cfg, 100, size, collateral, false)
	if err != nil {
		t.Fatalf("liquidationPriceRat short: %v", err)
	}
	wantShort := big.NewRat(105, 1)
	if shortPrice.Cmp(wantShort) != 0 {
		t.Errorf("short liquidation price = %s, want 105", shortPrice.RatString())
	}
}
