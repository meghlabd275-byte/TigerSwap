package services

import (
	"testing"

	"tigerswap/backend/models"
)

// newPerpEngineWithListings builds an engine backed by an in-memory database
// seeded with the given listings.
func newPerpEngineWithListings(t *testing.T, futures []models.FuturesMarket, perps []models.PerpsMarket) *PerpetualEngine {
	t.Helper()
	db := newListingTestDB(t)
	for i := range futures {
		if err := db.Create(&futures[i]).Error; err != nil {
			t.Fatalf("seed futures %s: %v", futures[i].Symbol, err)
		}
	}
	for i := range perps {
		if err := db.Create(&perps[i]).Error; err != nil {
			t.Fatalf("seed perps %s: %v", perps[i].Symbol, err)
		}
	}

	engine := NewPerpetualEngine(nil, nil)
	if err := engine.SetDB(db); err != nil {
		t.Fatalf("set db: %v", err)
	}
	return engine
}

// TestPerpetualEngineLoadsActiveListings verifies the engine's market registry
// is driven by the admin listing tables rather than a hardcoded pair list.
func TestPerpetualEngineLoadsActiveListings(t *testing.T) {
	engine := newPerpEngineWithListings(t, []models.FuturesMarket{
		{Symbol: "BTC/RUSD", BaseAsset: "BTC", QuoteAsset: "RUSD", Settlement: "RUSD", Status: models.ListingStatusActive, MaxLeverage: 200},
		{Symbol: "ETH/RUSD", BaseAsset: "ETH", QuoteAsset: "RUSD", Settlement: "RUSD", Status: models.ListingStatusActive, MaxLeverage: 125},
	}, nil)

	if got := len(engine.Markets()); got != 2 {
		t.Fatalf("expected 2 active markets, got %d (%v)", got, engine.Markets())
	}

	btc := engine.market("BTC/RUSD")
	if btc == nil {
		t.Fatal("BTC/RUSD should be listed")
	}
	if btc.OracleSymbol != "BTC" {
		t.Errorf("expected oracle symbol BTC, got %q", btc.OracleSymbol)
	}
	if btc.MaxLeverage != 200 {
		t.Errorf("expected max leverage 200, got %d", btc.MaxLeverage)
	}

	// Symbol lookup is case-insensitive.
	if engine.market("btc/rusd") == nil {
		t.Error("market lookup should be case-insensitive")
	}
	if engine.market("DOGE/RUSD") != nil {
		t.Error("an unlisted market must not resolve")
	}
	if engine.isValidPair("DOGE/RUSD") {
		t.Error("an unlisted market must not be valid for trading")
	}
}

// TestPerpetualEngineExcludesInactiveListings verifies paused, delisting and
// maintenance markets are not tradable, so pausing a market in the admin takes
// effect on the next refresh.
func TestPerpetualEngineExcludesInactiveListings(t *testing.T) {
	engine := newPerpEngineWithListings(t, []models.FuturesMarket{
		{Symbol: "BTC/RUSD", BaseAsset: "BTC", Status: models.ListingStatusActive},
		{Symbol: "ETH/RUSD", BaseAsset: "ETH", Status: models.ListingStatusPaused},
		{Symbol: "SOL/RUSD", BaseAsset: "SOL", Status: models.ListingStatusDelisting},
		{Symbol: "AVAX/RUSD", BaseAsset: "AVAX", Status: models.ListingStatusMaintenance},
	}, nil)

	if engine.market("BTC/RUSD") == nil {
		t.Error("active market should be tradable")
	}
	for _, symbol := range []string{"ETH/RUSD", "SOL/RUSD", "AVAX/RUSD"} {
		if engine.market(symbol) != nil {
			t.Errorf("%s must not be tradable", symbol)
		}
	}
}

// TestPerpetualEngineClampsListingLeverage verifies a listing cannot open more
// leverage than the engine's hard ceiling, and a zero value falls back to the
// ceiling rather than being treated as unlimited.
func TestPerpetualEngineClampsListingLeverage(t *testing.T) {
	engine := newPerpEngineWithListings(t, []models.FuturesMarket{
		{Symbol: "BTC/RUSD", BaseAsset: "BTC", Status: models.ListingStatusActive, MaxLeverage: 5000},
		{Symbol: "ETH/RUSD", BaseAsset: "ETH", Status: models.ListingStatusActive, MaxLeverage: 0},
		{Symbol: "SOL/RUSD", BaseAsset: "SOL", Status: models.ListingStatusActive, MaxLeverage: 25},
	}, nil)

	if got := engine.market("BTC/RUSD").MaxLeverage; got != maxPerpLeverage {
		t.Errorf("over-ceiling leverage should clamp to %d, got %d", maxPerpLeverage, got)
	}
	if got := engine.market("ETH/RUSD").MaxLeverage; got != maxPerpLeverage {
		t.Errorf("unset leverage should default to %d, got %d", maxPerpLeverage, got)
	}
	if got := engine.market("SOL/RUSD").MaxLeverage; got != 25 {
		t.Errorf("in-range leverage should be preserved, got %d", got)
	}
}

// TestPerpetualEngineLoadsPerpsListings verifies oracle perpetual listings are
// registered and that orderbook listings are left to the CLOB.
func TestPerpetualEngineLoadsPerpsListings(t *testing.T) {
	engine := newPerpEngineWithListings(t, nil, []models.PerpsMarket{
		{Symbol: "BTC/RUSD", MarketType: "oracle", Status: models.ListingStatusActive, MaxLeverage: 50},
		{Symbol: "ETH/RUSD", MarketType: "orderbook", Status: models.ListingStatusActive},
	})

	if engine.market("BTC/RUSD") == nil {
		t.Error("oracle perps listing should be tradable on this engine")
	}
	if engine.market("ETH/RUSD") != nil {
		t.Error("orderbook listing should not be registered on the oracle engine")
	}
}

// TestPerpetualEngineFuturesWinsOnDuplicateSymbol verifies a symbol listed in
// both tables keeps the futures entry, which carries the richer risk model.
func TestPerpetualEngineFuturesWinsOnDuplicateSymbol(t *testing.T) {
	engine := newPerpEngineWithListings(t,
		[]models.FuturesMarket{{Symbol: "BTC/RUSD", BaseAsset: "BTC", Status: models.ListingStatusActive, MaxLeverage: 200, TakerFeeRate: 0.0004}},
		[]models.PerpsMarket{{Symbol: "BTC/RUSD", MarketType: "oracle", Status: models.ListingStatusActive, MaxLeverage: 10, TakerFeeRate: 0.009}},
	)

	cfg := engine.market("BTC/RUSD")
	if cfg == nil {
		t.Fatal("BTC/RUSD should be listed")
	}
	if cfg.MaxLeverage != 200 {
		t.Errorf("expected futures leverage 200 to win, got %d", cfg.MaxLeverage)
	}
	if cfg.TakerFeeRate != 0.0004 {
		t.Errorf("expected futures taker fee to win, got %v", cfg.TakerFeeRate)
	}
}

// TestPerpetualEngineRefreshPicksUpNewListing verifies a listing added after
// startup becomes tradable once the registry is refreshed, which is what the
// admin hook triggers.
func TestPerpetualEngineRefreshPicksUpNewListing(t *testing.T) {
	db := newListingTestDB(t)
	engine := NewPerpetualEngine(nil, nil)
	if err := engine.SetDB(db); err != nil {
		t.Fatalf("set db: %v", err)
	}
	if len(engine.Markets()) != 0 {
		t.Fatalf("expected no markets initially, got %v", engine.Markets())
	}

	if err := db.Create(&models.FuturesMarket{
		Symbol: "TGR/RUSD", BaseAsset: "TGR", Status: models.ListingStatusActive, MaxLeverage: 100,
	}).Error; err != nil {
		t.Fatalf("create listing: %v", err)
	}
	if err := engine.RefreshMarkets(); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	if engine.market("TGR/RUSD") == nil {
		t.Error("newly listed market should be tradable after refresh")
	}
}

// TestPerpetualEngineWithoutDatabase verifies the engine reports a clear error
// instead of silently trading with no listings.
func TestPerpetualEngineWithoutDatabase(t *testing.T) {
	engine := NewPerpetualEngine(nil, nil)
	if err := engine.RefreshMarkets(); err == nil {
		t.Error("refreshing without a database must fail")
	}
	if len(engine.Markets()) != 0 {
		t.Error("an engine with no database must expose no markets")
	}
}

// TestOracleBaseFromSymbol covers both listing symbol forms.
func TestOracleBaseFromSymbol(t *testing.T) {
	cases := map[string]string{
		"BTC/RUSD": "BTC",
		"ETH/USDT": "ETH",
		"BTC-USD":  "BTC",
		"SOL/RUSD": "SOL",
		"TGR":      "TGR",
		"":         "",
	}
	for in, want := range cases {
		if got := oracleBaseFromSymbol(in); got != want {
			t.Errorf("oracleBaseFromSymbol(%q) = %q, want %q", in, got, want)
		}
	}
}
