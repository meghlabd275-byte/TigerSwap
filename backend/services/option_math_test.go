package services

import (
	"math/big"
	"testing"
)

// These tests pin the Go port of TigerSwapOptionMath.sol to the same reference
// values the Solidity test suite asserts. The Solidity comments name the Python
// reference implementation, and the values below are those numbers. If the Go
// arithmetic and the on-chain library ever disagree, an off-chain quote would
// not match an on-chain settlement, so both must produce the same price.

func mustWad(t *testing.T, s string) *big.Int {
	t.Helper()
	value, err := parseWad(s)
	if err != nil {
		t.Fatalf("parseWad(%q): %v", s, err)
	}
	return value
}

// assertWadClose fails unless got is within toleranceBps of want. A zero want
// allows an absolute difference of 1.
func assertWadClose(t *testing.T, got, want *big.Int, toleranceBps int64, label string) {
	t.Helper()
	diff := new(big.Int).Sub(got, want)
	diff.Abs(diff)
	allowed := new(big.Int).Quo(new(big.Int).Mul(want, big.NewInt(toleranceBps)), big.NewInt(10000))
	allowed.Abs(allowed)
	if allowed.Sign() == 0 {
		allowed = big.NewInt(1)
	}
	if diff.Cmp(allowed) > 0 {
		t.Fatalf("%s: got %s, want %s (diff %s, allowed %s)",
			label, formatWad(got), formatWad(want), diff.String(), allowed.String())
	}
}

func TestOptionLnOfOneIsZero(t *testing.T) {
	result, err := optLn(optWAD)
	if err != nil {
		t.Fatalf("optLn: %v", err)
	}
	if result.Sign() != 0 {
		t.Fatalf("ln(1) should be 0, got %s", result.String())
	}
}

func TestOptionLnOfTwo(t *testing.T) {
	result, err := optLn(new(big.Int).Mul(big.NewInt(2), optWAD))
	if err != nil {
		t.Fatalf("optLn: %v", err)
	}
	// ln(2) = 0.693147180559945309
	expected := mustOptInt("693147180559945309")
	diff := new(big.Int).Sub(result, expected)
	diff.Abs(diff)
	if diff.Cmp(big.NewInt(1e9)) >= 0 {
		t.Fatalf("ln(2) off by %s", diff.String())
	}
}

func TestOptionLnOfE(t *testing.T) {
	// e = 2.718281828459045235
	result, err := optLn(mustOptInt("2718281828459045235"))
	if err != nil {
		t.Fatalf("optLn: %v", err)
	}
	diff := new(big.Int).Sub(result, optWAD)
	diff.Abs(diff)
	if diff.Cmp(big.NewInt(1e10)) >= 0 {
		t.Fatalf("ln(e) off by %s", diff.String())
	}
}

func TestOptionLnHandlesSmallNumbers(t *testing.T) {
	// ln(0.5) = -0.693147...
	result, err := optLn(mustOptInt("500000000000000000"))
	if err != nil {
		t.Fatalf("optLn: %v", err)
	}
	expected := mustOptInt("-693147180559945309")
	diff := new(big.Int).Sub(result, expected)
	diff.Abs(diff)
	if diff.Cmp(big.NewInt(1e9)) >= 0 {
		t.Fatalf("ln(0.5) off by %s", diff.String())
	}
}

func TestOptionLnRejectsZero(t *testing.T) {
	if _, err := optLn(big.NewInt(0)); err == nil {
		t.Fatal("ln(0) must be rejected")
	}
}

func TestOptionExpOfZeroIsOne(t *testing.T) {
	result, err := optExp(big.NewInt(0))
	if err != nil {
		t.Fatalf("optExp: %v", err)
	}
	if result.Cmp(optWAD) != 0 {
		t.Fatalf("exp(0) should be 1e18, got %s", result.String())
	}
}

func TestOptionExpOfOne(t *testing.T) {
	result, err := optExp(optWAD)
	if err != nil {
		t.Fatalf("optExp: %v", err)
	}
	expected := mustOptInt("2718281828459045235")
	diff := new(big.Int).Sub(result, expected)
	diff.Abs(diff)
	if diff.Cmp(big.NewInt(1e10)) >= 0 {
		t.Fatalf("exp(1) off by %s", diff.String())
	}
}

func TestOptionExpNegRejectsNegative(t *testing.T) {
	if _, err := optExpNeg(big.NewInt(-1)); err == nil {
		t.Fatal("expNeg of a negative value must be rejected")
	}
}

func TestOptionInvSqrt(t *testing.T) {
	// 1/sqrt(4) = 0.5
	result, err := optInvSqrt(new(big.Int).Mul(big.NewInt(4), optWAD))
	if err != nil {
		t.Fatalf("optInvSqrt: %v", err)
	}
	assertWadClose(t, result, mustOptInt("500000000000000000"), 1, "invSqrt(4)")

	// 1/sqrt(1) = 1
	result, err = optInvSqrt(optWAD)
	if err != nil {
		t.Fatalf("optInvSqrt: %v", err)
	}
	assertWadClose(t, result, optWAD, 1, "invSqrt(1)")
}

func TestOptionInvSqrtRejectsZero(t *testing.T) {
	if _, err := optInvSqrt(big.NewInt(0)); err == nil {
		t.Fatal("invSqrt(0) must be rejected")
	}
}

// TestOptionInvSqrtShortDatedArgument guards the case whose normalised mantissa
// lands above three: the Newton update would otherwise drive y negative. A
// one-month option produces exactly this argument via WAD*WAD/T, and before the
// guard the series could not be priced at all.
func TestOptionInvSqrtShortDatedArgument(t *testing.T) {
	// T for 30 days, then WAD*WAD/T as the pricing path computes it.
	t30 := optRawDiv(new(big.Int).Mul(big.NewInt(int64(30*24*60*60)), optWAD), big.NewInt(optYearSeconds))
	argument := optRawDiv(optWADSq, t30)

	result, err := optInvSqrt(argument)
	if err != nil {
		t.Fatalf("optInvSqrt: %v", err)
	}
	if result.Sign() <= 0 {
		t.Fatalf("invSqrt must be positive, got %s", result.String())
	}

	// 1/sqrt(WAD*WAD/T) = sqrt(T)/WAD. For T = 30/365, sqrt(T) = 0.286691089540497920.
	want := mustOptInt("286691089540497920")
	assertWadClose(t, result, want, 20, "invSqrt short-dated argument")
}

// TestOptionShortDatedSeriesPrices verifies a series shorter than three months
// prices to a positive value below the spot, the range any sane option occupies.
func TestOptionShortDatedSeriesPrices(t *testing.T) {
	spot := mustOptInt("2000000000000000000000")
	strike := mustOptInt("2000000000000000000000")
	volatility := mustOptInt("800000000000000000")

	for _, days := range []int64{1, 7, 30, 60, 90} {
		seconds := days * 24 * 60 * 60
		price, err := optBlackScholes(spot, strike, seconds, mustOptInt("50000000000000000"), volatility, true)
		if err != nil {
			t.Fatalf("%d-day call: %v", days, err)
		}
		if price.Sign() <= 0 {
			t.Fatalf("%d-day call must be positive, got %s", days, price.String())
		}
		if price.Cmp(spot) >= 0 {
			t.Fatalf("%d-day call must be below spot, got %s", days, price.String())
		}
	}
}

func TestOptionNormalCdfAtZeroIsHalf(t *testing.T) {
	result, err := optNormalCdf(big.NewInt(0))
	if err != nil {
		t.Fatalf("optNormalCdf: %v", err)
	}
	assertWadClose(t, result, mustOptInt("500000000000000000"), 1, "Phi(0)")
}

func TestOptionNormalCdfSymmetry(t *testing.T) {
	positive, err := optNormalCdf(optWAD)
	if err != nil {
		t.Fatalf("optNormalCdf: %v", err)
	}
	negative, err := optNormalCdf(new(big.Int).Neg(optWAD))
	if err != nil {
		t.Fatalf("optNormalCdf: %v", err)
	}
	// Phi(1) + Phi(-1) = 1
	sum := new(big.Int).Add(positive, negative)
	assertWadClose(t, sum, optWAD, 1, "Phi(1)+Phi(-1)")
	// Phi(1) = 0.841344746
	assertWadClose(t, positive, mustOptInt("841344746068542949"), 5, "Phi(1)")
}

func TestOptionNormalCdfTails(t *testing.T) {
	// Phi(-6) is essentially zero.
	result, err := optNormalCdf(mustOptInt("-6000000000000000000"))
	if err != nil {
		t.Fatalf("optNormalCdf: %v", err)
	}
	if result.Cmp(big.NewInt(1e9)) >= 0 {
		t.Fatalf("Phi(-6) should be near 0, got %s", result.String())
	}
}

// TestOptionAtTheMoneyCallMatchesReference pins the ATM call to the value the
// Solidity suite asserts: bs(2000,2000,1y,5%,80%) = 656.4196493398014.
func TestOptionAtTheMoneyCallMatchesReference(t *testing.T) {
	price, err := optBlackScholes(
		mustOptInt("2000000000000000000000"),
		mustOptInt("2000000000000000000000"),
		optYearSeconds,
		mustOptInt("50000000000000000"),
		mustOptInt("800000000000000000"),
		true,
	)
	if err != nil {
		t.Fatalf("optBlackScholes: %v", err)
	}
	assertWadClose(t, price, mustOptInt("656419649339801400000"), 100, "ATM call")
}

// TestOptionOutOfTheMoneyCallMatchesReference pins bs(2000,2500,0.5y,5%,60%)
// = 192.84771107754466.
func TestOptionOutOfTheMoneyCallMatchesReference(t *testing.T) {
	price, err := optBlackScholes(
		mustOptInt("2000000000000000000000"),
		mustOptInt("2500000000000000000000"),
		optYearSeconds/2,
		mustOptInt("50000000000000000"),
		mustOptInt("600000000000000000"),
		true,
	)
	if err != nil {
		t.Fatalf("optBlackScholes: %v", err)
	}
	assertWadClose(t, price, mustOptInt("192847711077544660000"), 100, "OTM call")
}

// TestOptionInTheMoneyPutMatchesReference pins bs(2000,1500,0.25y,5%,90%) put
// = 115.33905944655538.
func TestOptionInTheMoneyPutMatchesReference(t *testing.T) {
	price, err := optBlackScholes(
		mustOptInt("2000000000000000000000"),
		mustOptInt("1500000000000000000000"),
		optYearSeconds/4,
		mustOptInt("50000000000000000"),
		mustOptInt("900000000000000000"),
		false,
	)
	if err != nil {
		t.Fatalf("optBlackScholes: %v", err)
	}
	assertWadClose(t, price, mustOptInt("115339059446555380000"), 100, "ITM put")
}

// TestOptionZeroRateAtTheMoneyCallMatchesReference pins
// bs(2000,2000,1y,0,100%) = 765.8498450960524.
func TestOptionZeroRateAtTheMoneyCallMatchesReference(t *testing.T) {
	price, err := optBlackScholes(
		mustOptInt("2000000000000000000000"),
		mustOptInt("2000000000000000000000"),
		optYearSeconds,
		big.NewInt(0),
		optWAD,
		true,
	)
	if err != nil {
		t.Fatalf("optBlackScholes: %v", err)
	}
	assertWadClose(t, price, mustOptInt("765849845096052400000"), 100, "zero rate ATM call")
}

// TestOptionExpiredIsIntrinsicOnly verifies an option at expiry is worth only
// its intrinsic value, in every combination.
func TestOptionExpiredIsIntrinsicOnly(t *testing.T) {
	cases := []struct {
		name   string
		spot   string
		strike string
		isCall bool
		want   string
	}{
		{"ITM call", "2000", "1500", true, "500"},
		{"OTM call", "2000", "2500", true, "0"},
		{"ITM put", "2000", "2500", false, "500"},
		{"OTM put", "2000", "1500", false, "0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			price, err := optBlackScholes(
				mustWad(t, tc.spot), mustWad(t, tc.strike), 0,
				mustOptInt("50000000000000000"), mustOptInt("800000000000000000"), tc.isCall,
			)
			if err != nil {
				t.Fatalf("optBlackScholes: %v", err)
			}
			if price.Cmp(mustWad(t, tc.want)) != 0 {
				t.Fatalf("expired %s = %s, want %s", tc.name, formatWad(price), tc.want)
			}
		})
	}
}

// TestOptionZeroVolatilityIsIntrinsicOnly verifies zero volatility prices at
// intrinsic rather than dividing by zero.
func TestOptionZeroVolatilityIsIntrinsicOnly(t *testing.T) {
	price, err := optBlackScholes(
		mustOptInt("2000000000000000000000"),
		mustOptInt("1500000000000000000000"),
		optYearSeconds,
		mustOptInt("50000000000000000"),
		big.NewInt(0),
		true,
	)
	if err != nil {
		t.Fatalf("optBlackScholes: %v", err)
	}
	assertWadClose(t, price, mustOptInt("500000000000000000000"), 1, "zero vol call")
}

// TestOptionPutCallParity verifies C - P = S - K e^{-rT}, the no-arbitrage
// relation every correct Black-Scholes implementation must satisfy.
func TestOptionPutCallParity(t *testing.T) {
	spot := mustOptInt("2000000000000000000000")
	strike := mustOptInt("2000000000000000000000")
	rate := mustOptInt("50000000000000000")
	volatility := mustOptInt("800000000000000000")

	call, err := optBlackScholes(spot, strike, optYearSeconds, rate, volatility, true)
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	put, err := optBlackScholes(spot, strike, optYearSeconds, rate, volatility, false)
	if err != nil {
		t.Fatalf("put: %v", err)
	}

	discount, err := optExpNeg(rate)
	if err != nil {
		t.Fatalf("expNeg: %v", err)
	}
	forward := optMul(strike, discount)
	expected := new(big.Int).Sub(spot, forward)

	callMinusPut := new(big.Int).Sub(call, put)
	callMinusPut.Abs(callMinusPut)
	assertWadClose(t, callMinusPut, expected, 20, "put-call parity")
}

func TestOptionCallPriceIncreasesWithVolatility(t *testing.T) {
	low, err := optBlackScholes(
		mustOptInt("2000000000000000000000"), mustOptInt("2000000000000000000000"),
		optYearSeconds, mustOptInt("50000000000000000"), mustOptInt("300000000000000000"), true)
	if err != nil {
		t.Fatalf("low vol: %v", err)
	}
	high, err := optBlackScholes(
		mustOptInt("2000000000000000000000"), mustOptInt("2000000000000000000000"),
		optYearSeconds, mustOptInt("50000000000000000"), optWAD, true)
	if err != nil {
		t.Fatalf("high vol: %v", err)
	}
	if high.Cmp(low) <= 0 {
		t.Fatalf("higher volatility must cost more: low=%s high=%s", formatWad(low), formatWad(high))
	}
}

func TestOptionLongerExpiryCostsMore(t *testing.T) {
	short, err := optBlackScholes(
		mustOptInt("2000000000000000000000"), mustOptInt("2000000000000000000000"),
		optYearSeconds/4, mustOptInt("50000000000000000"), mustOptInt("800000000000000000"), true)
	if err != nil {
		t.Fatalf("short: %v", err)
	}
	long, err := optBlackScholes(
		mustOptInt("2000000000000000000000"), mustOptInt("2000000000000000000000"),
		optYearSeconds, mustOptInt("50000000000000000"), mustOptInt("800000000000000000"), true)
	if err != nil {
		t.Fatalf("long: %v", err)
	}
	if long.Cmp(short) <= 0 {
		t.Fatalf("more time must cost more: short=%s long=%s", formatWad(short), formatWad(long))
	}
}

func TestOptionRejectsZeroSpot(t *testing.T) {
	if _, err := optBlackScholes(
		big.NewInt(0), mustOptInt("2000000000000000000000"),
		optYearSeconds, mustOptInt("50000000000000000"), mustOptInt("800000000000000000"), true,
	); err == nil {
		t.Fatal("a zero spot must be rejected")
	}
}

func TestOptionCallDeltaBetweenZeroAndOne(t *testing.T) {
	greeks, err := optGreeks(
		mustOptInt("2000000000000000000000"), mustOptInt("2000000000000000000000"),
		optYearSeconds, mustOptInt("50000000000000000"), mustOptInt("800000000000000000"), true)
	if err != nil {
		t.Fatalf("optGreeks: %v", err)
	}
	if greeks.Delta.Sign() <= 0 {
		t.Fatalf("call delta must be positive, got %s", greeks.Delta.String())
	}
	if greeks.Delta.Cmp(optWAD) >= 0 {
		t.Fatalf("call delta must be below one, got %s", greeks.Delta.String())
	}
}

// TestOptionAtTheMoneyCallDeltaMatchesReference pins the ATM call delta to the
// reference the Solidity suite asserts: 0.678138598836346.
func TestOptionAtTheMoneyCallDeltaMatchesReference(t *testing.T) {
	greeks, err := optGreeks(
		mustOptInt("2000000000000000000000"), mustOptInt("2000000000000000000000"),
		optYearSeconds, mustOptInt("50000000000000000"), mustOptInt("800000000000000000"), true)
	if err != nil {
		t.Fatalf("optGreeks: %v", err)
	}
	expected := mustOptInt("678138598836346000")
	diff := new(big.Int).Sub(greeks.Delta, expected)
	diff.Abs(diff)
	if diff.Cmp(big.NewInt(1e11)) >= 0 {
		t.Fatalf("ATM call delta off by %s", diff.String())
	}
}

func TestOptionPutDeltaIsNegative(t *testing.T) {
	greeks, err := optGreeks(
		mustOptInt("2000000000000000000000"), mustOptInt("2000000000000000000000"),
		optYearSeconds, mustOptInt("50000000000000000"), mustOptInt("800000000000000000"), false)
	if err != nil {
		t.Fatalf("optGreeks: %v", err)
	}
	if greeks.Delta.Sign() >= 0 {
		t.Fatalf("put delta must be negative, got %s", greeks.Delta.String())
	}
}

func TestParseWadRejectsMalformedInput(t *testing.T) {
	for _, bad := range []string{"", "  ", "abc", "1.2.3", "1e18", "--1", "1.0000000000000000000"} {
		if _, err := parseWad(bad); err == nil {
			t.Fatalf("parseWad(%q) must be rejected", bad)
		}
	}
}

func TestParseWadRoundTrips(t *testing.T) {
	cases := map[string]string{
		"0":                    "0",
		"1":                    "1",
		"1.5":                  "1.5",
		"0.000000000000000001": "0.000000000000000001",
		"2000":                 "2000",
		"123.456":              "123.456",
		"-2.25":                "-2.25",
	}
	for input, want := range cases {
		value, err := parseWad(input)
		if err != nil {
			t.Fatalf("parseWad(%q): %v", input, err)
		}
		if got := formatWad(value); got != want {
			t.Fatalf("parseWad(%q) -> formatWad = %q, want %q", input, got, want)
		}
	}
}
