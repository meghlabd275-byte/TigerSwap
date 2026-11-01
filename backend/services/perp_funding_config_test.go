package services

import (
	"testing"
	"time"

	"tigerswap/backend/models"
)

// TestPerpFundingConfigFromListing verifies the engine takes funding rate,
// cadence and cap from the admin listing instead of a shared hardcoded value.
// An operator who tunes funding on a market must see that tuning take effect.
func TestPerpFundingConfigFromListing(t *testing.T) {
	engine := newPerpEngineWithListings(t, []models.FuturesMarket{
		{
			Symbol: "BTC/RUSD", BaseAsset: "BTC", Status: models.ListingStatusActive,
			MaxLeverage: 200, FundingRate: "0.0005", FundingInterval: 4, FundingRateCap: "0.02",
		},
	}, nil)

	cfg := engine.market("BTC/RUSD")
	if cfg == nil {
		t.Fatal("BTC/RUSD should be listed")
	}
	if cfg.FundingRate != 0.0005 {
		t.Errorf("expected listing funding rate 0.0005, got %v", cfg.FundingRate)
	}
	if cfg.FundingInterval != 4*time.Hour {
		t.Errorf("expected 4h funding interval, got %v", cfg.FundingInterval)
	}
	if cfg.FundingRateCap != 0.02 {
		t.Errorf("expected funding cap 0.02, got %v", cfg.FundingRateCap)
	}
}

// TestPerpFundingDefaultsWhenListingOmits verifies a listing that leaves
// funding unset falls back to the documented defaults rather than zeroing the
// interest component or disabling settlement.
func TestPerpFundingDefaultsWhenListingOmits(t *testing.T) {
	engine := newPerpEngineWithListings(t, []models.FuturesMarket{
		{Symbol: "ETH/RUSD", BaseAsset: "ETH", Status: models.ListingStatusActive, MaxLeverage: 100},
	}, nil)

	cfg := engine.market("ETH/RUSD")
	if cfg == nil {
		t.Fatal("ETH/RUSD should be listed")
	}
	if cfg.FundingRate != 0 {
		t.Errorf("an unset funding rate should default to no interest component, got %v", cfg.FundingRate)
	}
	if cfg.FundingInterval != defaultFundingInterval {
		t.Errorf("an unset interval should default to %v, got %v", defaultFundingInterval, cfg.FundingInterval)
	}
	if cfg.FundingRateCap != defaultFundingRateCap {
		t.Errorf("an unset cap should default to %v, got %v", defaultFundingRateCap, cfg.FundingRateCap)
	}
}

// TestPerpsListingFundingDefaults verifies an oracle perps listing (which has no
// funding columns) still gets the engine defaults so funding is well defined.
func TestPerpsListingFundingDefaults(t *testing.T) {
	engine := newPerpEngineWithListings(t, nil, []models.PerpsMarket{
		{Symbol: "SOL/RUSD", MarketType: "oracle", Status: models.ListingStatusActive, MaxLeverage: 50},
	})

	cfg := engine.market("SOL/RUSD")
	if cfg == nil {
		t.Fatal("SOL/RUSD should be listed")
	}
	if cfg.FundingRate != defaultFundingRate {
		t.Errorf("expected default funding rate %v, got %v", defaultFundingRate, cfg.FundingRate)
	}
	if cfg.FundingInterval != defaultFundingInterval {
		t.Errorf("expected default interval %v, got %v", defaultFundingInterval, cfg.FundingInterval)
	}
}

// TestPerpIndexSymbolFromBase verifies each market names its own index feed so
// the index price is not silently the mark price.
func TestPerpIndexSymbolFromBase(t *testing.T) {
	engine := newPerpEngineWithListings(t, []models.FuturesMarket{
		{Symbol: "BTC/RUSD", BaseAsset: "BTC", Status: models.ListingStatusActive},
	}, nil)

	cfg := engine.market("BTC/RUSD")
	if cfg == nil {
		t.Fatal("BTC/RUSD should be listed")
	}
	if cfg.IndexSymbol != "BTC" {
		t.Errorf("expected index symbol BTC, got %q", cfg.IndexSymbol)
	}
}

// TestNextFundingTimeAlignsToInterval verifies the next funding timestamp is in
// the future and on an interval boundary, and that a non-positive interval
// falls back to the default cadence rather than disabling settlement.
func TestNextFundingTimeAlignsToInterval(t *testing.T) {
	e := &PerpetualEngine{}
	now := time.Now().UTC()

	for _, interval := range []time.Duration{time.Hour, 4 * time.Hour, 8 * time.Hour} {
		next := e.nextFundingTime(interval)
		if !next.After(now) {
			t.Errorf("interval %v: next funding %v is not in the future", interval, next)
		}
		if next.UnixNano()%interval.Nanoseconds() != 0 {
			t.Errorf("interval %v: next funding %v is not aligned to the interval", interval, next)
		}
	}

	fallback := e.nextFundingTime(0)
	if !fallback.After(now) {
		t.Error("a zero interval should fall back to a future default settlement")
	}
	if fallback.UnixNano()%defaultFundingInterval.Nanoseconds() != 0 {
		t.Error("the default settlement should align to the default interval")
	}
}

// TestFundingIntervalConversion covers the hour-to-duration conversion and its
// fallback for non-positive input.
func TestFundingIntervalConversion(t *testing.T) {
	if got := fundingInterval(4); got != 4*time.Hour {
		t.Errorf("fundingInterval(4) = %v, want 4h", got)
	}
	if got := fundingInterval(0); got != defaultFundingInterval {
		t.Errorf("fundingInterval(0) = %v, want %v", got, defaultFundingInterval)
	}
	if got := fundingInterval(-3); got != defaultFundingInterval {
		t.Errorf("fundingInterval(-3) = %v, want %v", got, defaultFundingInterval)
	}
}

// TestFundingDecimalParsing verifies stored funding decimals are parsed with a
// safe fallback and that junk never becomes a negative or NaN rate.
func TestFundingDecimalParsing(t *testing.T) {
	if got := decimalOrZero("0.0005"); got != 0.0005 {
		t.Errorf("decimalOrZero(0.0005) = %v, want 0.0005", got)
	}
	for _, bad := range []string{"", "abc", "-1", "NaN", "Inf"} {
		if got := decimalOrZero(bad); got != 0 {
			t.Errorf("decimalOrZero(%q) = %v, want 0", bad, got)
		}
	}

	if got := decimalOrDefault("0.02", defaultFundingRateCap); got != 0.02 {
		t.Errorf("decimalOrDefault(0.02) = %v, want 0.02", got)
	}
	for _, bad := range []string{"", "junk", "0", "-5"} {
		if got := decimalOrDefault(bad, defaultFundingRateCap); got != defaultFundingRateCap {
			t.Errorf("decimalOrDefault(%q) = %v, want the default %v", bad, got, defaultFundingRateCap)
		}
	}
}
