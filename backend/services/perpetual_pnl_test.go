package services

import (
	"math"
	"strconv"
	"testing"
)

// parsePNL converts the string PnL returned by the engine into a float for
// comparison.
func parsePNL(t *testing.T, value string) float64 {
	t.Helper()
	parsed, err := strconvParse(value)
	if err != nil {
		t.Fatalf("pnl %q is not numeric: %v", value, err)
	}
	return parsed
}

func strconvParse(value string) (float64, error) {
	return strconv.ParseFloat(value, 64)
}

// TestPerpetualPNLIsNotionalNotLeveragedTwice pins the PnL definition against
// the margin rule used to open the position.
//
// OpenPosition requires `size / leverage` of collateral, so `size` is the
// notional value. A 10,000 notional long at 10x opened at 100 and marked at
// 110 has moved 10%, which is 1,000 of profit, not the 10,000 that applying
// the leverage a second time would produce.
func TestPerpetualPNLIsNotionalNotLeveragedTwice(t *testing.T) {
	engine := &PerpetualEngine{}

	long := &PerpetualPosition{
		Pair:       "BTC/RUSD",
		Side:       "long",
		Size:       "10000",
		EntryPrice: 100,
		Leverage:   10,
	}
	if got := parsePNL(t, engine.calculatePNL(long, 110)); math.Abs(got-1000) > 1e-6 {
		t.Fatalf("long pnl = %v, want 1000 (10%% of 10000 notional)", got)
	}

	short := &PerpetualPosition{
		Pair:       "BTC/RUSD",
		Side:       "short",
		Size:       "10000",
		EntryPrice: 100,
		Leverage:   10,
	}
	if got := parsePNL(t, engine.calculatePNL(short, 90)); math.Abs(got-1000) > 1e-6 {
		t.Fatalf("short pnl = %v, want 1000 when price falls 10%%", got)
	}
	if got := parsePNL(t, engine.calculatePNL(short, 110)); math.Abs(got+1000) > 1e-6 {
		t.Fatalf("short pnl = %v, want -1000 when price rises 10%%", got)
	}
}

// TestPerpetualPNLMatchesCollateralReturn asserts the PnL is consistent with
// the margin actually posted: a 10% favourable move on a 10x position doubles
// the 1,000 of collateral, i.e. a 100% return on margin.
func TestPerpetualPNLMatchesCollateralReturn(t *testing.T) {
	engine := &PerpetualEngine{}
	size, leverage := 10000.0, 10
	collateral := size / float64(leverage)

	position := &PerpetualPosition{
		Side:       "long",
		Size:       "10000",
		EntryPrice: 100,
		Leverage:   leverage,
	}
	pnl := parsePNL(t, engine.calculatePNL(position, 110))
	if math.Abs(pnl/collateral-1.0) > 1e-6 {
		t.Fatalf("pnl/collateral = %v, want 1.0", pnl/collateral)
	}
}

// TestCopyTradingPNLUsesTheSameDefinition ensures the copy engine cannot drift
// from the perpetual engine.
func TestCopyTradingPNLUsesTheSameDefinition(t *testing.T) {
	engine := &CopyTradingEngine{}
	position := &CopyPosition{
		Side:         "long",
		Size:         "10000",
		EntryPrice:   100,
		CurrentPrice: 110,
		Leverage:     10,
	}
	if got := parsePNL(t, engine.calculateCopyPNL(position)); math.Abs(got-1000) > 1e-6 {
		t.Fatalf("copy pnl = %v, want 1000", got)
	}
}

// TestPerpetualPNLHandlesZeroEntryPrice guards against a division by zero on a
// malformed position.
func TestPerpetualPNLHandlesZeroEntryPrice(t *testing.T) {
	engine := &PerpetualEngine{}
	position := &PerpetualPosition{Side: "long", Size: "100", EntryPrice: 0}
	if got := parsePNL(t, engine.calculatePNL(position, 110)); got != 0 {
		t.Fatalf("pnl with zero entry = %v, want 0", got)
	}
}
