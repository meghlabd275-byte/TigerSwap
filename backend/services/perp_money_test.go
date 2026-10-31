package services

import (
	"math"
	"math/big"
	"strings"
	"testing"

	"tigerswap/backend/models"
)

// TestPerpAmountValueRejectsNonFiniteInput is the regression test for a funds
// safety hole: fmt.Sscanf parses "NaN" and "Inf" successfully, and both satisfy
// a naive `value > 0` check, so a caller could previously open a position with a
// NaN or infinite size. Every comparison against NaN is false, which meant such
// a position could never be liquidated.
func TestPerpAmountValueRejectsNonFiniteInput(t *testing.T) {
	rejected := []string{
		"NaN", "nan", "Inf", "+Inf", "-Inf", "Infinity",
		"0", "-1", "-0.5", "", "   ", "abc", "1e5", "0x10",
		"1,000", "1.2.3", "+1", "1.000000001", // 9 fractional digits
	}
	for _, input := range rejected {
		if _, err := perpAmountValue(input); err == nil {
			t.Errorf("perpAmountValue(%q) was accepted; it must be rejected", input)
		}
	}
}

// TestPerpAmountValueAcceptsRealAmounts pins the amounts the engine does accept.
func TestPerpAmountValueAcceptsRealAmounts(t *testing.T) {
	accepted := map[string]string{
		"1":                "1",
		"0.00000001":       "1/100000000",
		"10000":            "10000",
		"1234.56789012":    "30864197253/25000000",
		"0.1":              "1/10",
		"9999999999999999": "9999999999999999",
	}
	for input, want := range accepted {
		got, err := perpAmountValue(input)
		if err != nil {
			t.Errorf("perpAmountValue(%q) rejected: %v", input, err)
			continue
		}
		if got.RatString() != want {
			t.Errorf("perpAmountValue(%q) = %s, want %s", input, got.RatString(), want)
		}
	}
}

// TestPerpAmountValueRejectsAbsurdMagnitude verifies a value large enough to
// overflow a float64 into infinity is refused, since an infinite notional would
// satisfy every risk comparison.
func TestPerpAmountValueRejectsAbsurdMagnitude(t *testing.T) {
	huge := strings.Repeat("9", 400)
	if _, err := perpAmountValue(huge); err == nil {
		t.Fatal("a 400-digit amount must be rejected")
	}
	if _, err := perpAmountValue("1e30"); err == nil {
		t.Fatal("an exponent-notation amount must be rejected")
	}
}

// TestRationalToFloatRejectsNonFinite verifies a value that cannot be a finite
// float64 is reported rather than silently becoming +Inf.
func TestRationalToFloatRejectsNonFinite(t *testing.T) {
	huge, _ := new(big.Rat).SetString(strings.Repeat("9", 400))
	if _, err := rationalToFloat(huge); err == nil {
		t.Fatal("an unrepresentable magnitude must be rejected")
	}
	got, err := rationalToFloat(big.NewRat(3, 2))
	if err != nil {
		t.Fatalf("rationalToFloat(3/2): %v", err)
	}
	if math.Abs(got-1.5) > 1e-12 {
		t.Fatalf("expected 1.5, got %v", got)
	}
}

// TestOpenInterestCapAppliesToPerpsListings verifies a perps listing's cap is
// read as an exact value, and that a futures listing (which has no cap column)
// is treated as unlimited rather than as a zero cap that blocks all trading.
func TestOpenInterestCapAppliesToPerpsListings(t *testing.T) {
	capped := &perpMarketConfig{Symbol: "BTC/RUSD", MaxOpenInterest: "1000000"}
	cap, limited, err := openInterestCapRat(capped)
	if err != nil {
		t.Fatalf("openInterestCapRat: %v", err)
	}
	if !limited {
		t.Fatal("a listing with a max_open_interest must be capped")
	}
	if cap.RatString() != "1000000" {
		t.Fatalf("expected cap 1000000, got %s", cap.RatString())
	}

	uncapped := &perpMarketConfig{Symbol: "ETH/RUSD", MaxOpenInterest: ""}
	if _, limited, err := openInterestCapRat(uncapped); err != nil || limited {
		t.Fatalf("a futures listing must be uncapped, got limited=%v err=%v", limited, err)
	}

	bad := &perpMarketConfig{Symbol: "SOL/RUSD", MaxOpenInterest: "not-a-number"}
	if _, _, err := openInterestCapRat(bad); err == nil {
		t.Fatal("an unreadable cap must be an error, not a silent zero")
	}

	negative := &perpMarketConfig{Symbol: "TGR/RUSD", MaxOpenInterest: "0"}
	if _, _, err := openInterestCapRat(negative); err == nil {
		t.Fatal("a non-positive cap must be an error")
	}
}

// TestPerpMarketConfigKeepsExactOpenInterest verifies the listing's cap survives
// conversion without going through float64, so a cap expressed with full
// precision is enforced exactly.
func TestPerpMarketConfigKeepsExactOpenInterest(t *testing.T) {
	cfg := perpsMarketConfig(&models.PerpsMarket{
		Symbol:          "BTC/RUSD",
		MarketType:      "oracle",
		Status:          models.ListingStatusActive,
		MaxLeverage:     100,
		MaxOpenInterest: "1000000.12345678",
	})
	if cfg == nil {
		t.Fatal("expected a tradable config")
	}
	if cfg.MaxOpenInterest != "1000000.12345678" {
		t.Fatalf("open interest cap lost precision: %q", cfg.MaxOpenInterest)
	}
}

// TestOpenInterestRatSumsExactNotional verifies open interest is totalled from
// the positions themselves and split by side.
func TestOpenInterestRatSumsExactNotional(t *testing.T) {
	engine := &PerpetualEngine{positions: map[string]*PerpetualPosition{
		"a": {ID: "a", Pair: "BTC/RUSD", Side: "long", Size: "1000.5", Status: "open"},
		"b": {ID: "b", Pair: "BTC/RUSD", Side: "long", Size: "0.5", Status: "open"},
		"c": {ID: "c", Pair: "BTC/RUSD", Side: "short", Size: "300", Status: "open"},
		"d": {ID: "d", Pair: "BTC/RUSD", Side: "long", Size: "9999", Status: "closed"},
		"e": {ID: "e", Pair: "ETH/RUSD", Side: "long", Size: "777", Status: "open"},
	}}

	long, short, err := engine.openInterestRat("BTC/RUSD")
	if err != nil {
		t.Fatalf("openInterestRat: %v", err)
	}
	if long.RatString() != "1001" {
		t.Fatalf("long open interest = %s, want 1001 (closed and other-symbol positions excluded)", long.RatString())
	}
	if short.RatString() != "300" {
		t.Fatalf("short open interest = %s, want 300", short.RatString())
	}
}

// TestOpenInterestRatReportsUnreadableSize verifies a corrupt position size is
// surfaced instead of being skipped, which would understate open interest and
// let the cap be exceeded.
func TestOpenInterestRatReportsUnreadableSize(t *testing.T) {
	engine := &PerpetualEngine{positions: map[string]*PerpetualPosition{
		"good": {ID: "good", Pair: "BTC/RUSD", Side: "long", Size: "100", Status: "open"},
		"bad":  {ID: "bad", Pair: "BTC/RUSD", Side: "long", Size: "NaN", Status: "open"},
	}}
	if _, _, err := engine.openInterestRat("BTC/RUSD"); err == nil {
		t.Fatal("an unreadable position size must be reported")
	}
}

// TestNotionalPNLIsExactAndDirectional verifies PnL is the price move applied to
// the notional, with the sign set by the side, computed exactly.
func TestNotionalPNLIsExactAndDirectional(t *testing.T) {
	// A 10% rise on 10,000 notional is exactly 1,000 either way.
	long, err := notionalPNL(100, 110, "10000", "long")
	if err != nil {
		t.Fatalf("notionalPNL: %v", err)
	}
	if long.RatString() != "1000" {
		t.Fatalf("long pnl = %s, want 1000", long.RatString())
	}

	short, err := notionalPNL(100, 110, "10000", "short")
	if err != nil {
		t.Fatalf("notionalPNL: %v", err)
	}
	if short.RatString() != "-1000" {
		t.Fatalf("short pnl = %s, want -1000", short.RatString())
	}

	// No move means no PnL.
	flat, err := notionalPNL(100, 100, "10000", "long")
	if err != nil {
		t.Fatalf("notionalPNL: %v", err)
	}
	if flat.Sign() != 0 {
		t.Fatalf("flat pnl = %s, want 0", flat.RatString())
	}
}

// TestNotionalPNLHandlesBadInput verifies a zero cost basis yields zero rather
// than a division by zero, and a corrupt size is reported.
func TestNotionalPNLHandlesBadInput(t *testing.T) {
	zero, err := notionalPNL(0, 110, "100", "long")
	if err != nil {
		t.Fatalf("zero entry price must yield zero pnl, got error %v", err)
	}
	if zero.Sign() != 0 {
		t.Fatalf("zero entry price pnl = %s, want 0", zero.RatString())
	}
	if _, err := notionalPNL(100, 110, "NaN", "long"); err == nil {
		t.Fatal("a non-finite size must be rejected")
	}
}

// TestLiquidationPayoutFloorsAtZero verifies a position whose loss exceeds its
// margin pays the liquidator nothing, rather than a negative fee that would
// charge the liquidator for doing the work.
func TestLiquidationPayoutFloorsAtZero(t *testing.T) {
	collateral := big.NewRat(1000, 1)

	// A winning position still pays the fee on its remaining margin.
	fee, err := liquidationPayout(collateral, big.NewRat(500, 1), 0.05, "BTC/RUSD")
	if err != nil {
		t.Fatalf("liquidationPayout: %v", err)
	}
	if fee.RatString() != "75" {
		t.Fatalf("fee on 1500 remaining at 5%% = %s, want 75", fee.RatString())
	}

	// A loss larger than the margin pays nothing.
	fee, err = liquidationPayout(collateral, big.NewRat(-5000, 1), 0.05, "BTC/RUSD")
	if err != nil {
		t.Fatalf("liquidationPayout: %v", err)
	}
	if fee.Sign() != 0 {
		t.Fatalf("fee must floor at 0 for an underwater position, got %s", fee.RatString())
	}
}

// TestLiquidationPayoutRejectsBadRate verifies an out-of-range listing rate is
// refused rather than applied.
func TestLiquidationPayoutRejectsBadRate(t *testing.T) {
	collateral := big.NewRat(1000, 1)
	for _, rate := range []float64{-0.1, 1.5, math.NaN(), math.Inf(1)} {
		if _, err := liquidationPayout(collateral, big.NewRat(0, 1), rate, "BTC/RUSD"); err == nil {
			t.Errorf("liquidation fee rate %v must be rejected", rate)
		}
	}
}

// TestLiquidatePositionRefusesHealthyPosition verifies the settlement entry
// point cannot be used to close a position that is not flagged liquidatable, so
// a caller cannot seize a healthy trader's margin.
func TestLiquidatePositionRefusesHealthyPosition(t *testing.T) {
	engine := &PerpetualEngine{positions: map[string]*PerpetualPosition{
		"p1": {ID: "p1", Pair: "BTC/RUSD", Side: "long", Size: "1000", Collateral: "100", Status: "open"},
	}}
	if _, err := engine.LiquidatePosition(t.Context(), "p1"); err == nil {
		t.Fatal("an open (healthy) position must not be liquidatable")
	}
	if engine.positions["p1"].Status != "open" {
		t.Fatalf("a refused liquidation must leave the position untouched, got status %q", engine.positions["p1"].Status)
	}
}

// TestOpenInterestIncludesFlaggedPositions verifies a position that has been
// flagged for liquidation still counts toward open interest until it is
// actually settled, since it is still an exposure on the book.
func TestOpenInterestIncludesFlaggedPositions(t *testing.T) {
	engine := &PerpetualEngine{positions: map[string]*PerpetualPosition{
		"a": {ID: "a", Pair: "BTC/RUSD", Side: "long", Size: "100", Status: "open"},
		"b": {ID: "b", Pair: "BTC/RUSD", Side: "long", Size: "50", Status: "liquidatable"},
		"c": {ID: "c", Pair: "BTC/RUSD", Side: "long", Size: "999", Status: "liquidated"},
	}}
	long, _, err := engine.openInterestRat("BTC/RUSD")
	if err != nil {
		t.Fatalf("openInterestRat: %v", err)
	}
	if long.RatString() != "150" {
		t.Fatalf("open interest = %s, want 150 (flagged positions count, settled ones do not)", long.RatString())
	}
}

// TestClosePositionRefusesFlaggedPosition verifies a trader cannot escape the
// liquidation fee by closing a position the risk engine has already flagged.
func TestClosePositionRefusesFlaggedPosition(t *testing.T) {
	engine := &PerpetualEngine{positions: map[string]*PerpetualPosition{
		"p1": {ID: "p1", UserAddress: "0xabc", Pair: "BTC/RUSD", Side: "long", Size: "1000", Collateral: "100", Status: "liquidatable"},
	}}
	if _, err := engine.ClosePosition(t.Context(), "p1", "0xabc"); err == nil {
		t.Fatal("a flagged position must not be closable")
	}
	if engine.positions["p1"].Status != "liquidatable" {
		t.Fatalf("a refused close must leave the position untouched, got %q", engine.positions["p1"].Status)
	}
}

// TestPositionIsLiveDistinguishesTerminalStates pins the status semantics the
// open-interest and pricing paths rely on.
func TestPositionIsLiveDistinguishesTerminalStates(t *testing.T) {
	for _, status := range []string{"open", "liquidatable"} {
		if !positionIsLive(status) {
			t.Errorf("status %q must count as live", status)
		}
	}
	for _, status := range []string{"closed", "liquidated", ""} {
		if positionIsLive(status) {
			t.Errorf("status %q must not count as live", status)
		}
	}
}

// TestLiquidatablePositionsReportsFlaggedOnly verifies the liquidation bot's
// input list contains exactly the positions the risk engine has flagged.
func TestLiquidatablePositionsReportsFlaggedOnly(t *testing.T) {
	engine := &PerpetualEngine{positions: map[string]*PerpetualPosition{
		"a": {ID: "a", Status: "open"},
		"b": {ID: "b", Status: "liquidatable"},
		"c": {ID: "c", Status: "liquidated"},
	}}
	flagged := engine.LiquidatablePositions()
	if len(flagged) != 1 || flagged[0].ID != "b" {
		t.Fatalf("expected only position b, got %d entries", len(flagged))
	}
}
