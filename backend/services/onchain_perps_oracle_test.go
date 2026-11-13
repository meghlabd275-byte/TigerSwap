package services

import (
	"context"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"

	"tigerswap/backend/go/perps"
	"tigerswap/backend/go/perps/perpstest"
	"tigerswap/backend/models"
)

// TestUSDToWADIsExact proves the oracle writer scales a USD price to the
// contract's 1e18 integer without a float round-trip in the last digits.
func TestUSDToWADIsExact(t *testing.T) {
	cases := map[float64]string{
		1:      "1000000000000000000",
		60000:  "60000000000000000000000",
		0.5:    "500000000000000000",
		1234.5: "1234500000000000000000",
	}
	for usd, want := range cases {
		got, err := usdFloatToWAD(usd)
		if err != nil {
			t.Fatalf("%v: %v", usd, err)
		}
		if got.String() != want {
			t.Errorf("%v: want %s, got %s", usd, want, got.String())
		}
	}
	for _, bad := range []float64{0, -1} {
		if _, err := usdFloatToWAD(bad); err == nil {
			t.Errorf("%v must be rejected", bad)
		}
	}
}

// TestOracleWriterPushesMarkPrice proves the writer submits a real updatePrice
// transaction to the deployed contract, so a market the contract would otherwise
// reject with MARKET_PRICE_NOT_SET becomes tradable. It drives the writer
// against a real anvil chain and reads the price back from the chain.
func TestOracleWriterPushesMarkPrice(t *testing.T) {
	h := perpstest.Start(t)
	db := newOnChainPerpsTestDB(t)

	if err := db.Create(&models.FuturesMarket{Symbol: "BTC/RUSD", BaseAsset: "BTC", Status: models.ListingStatusActive}).Error; err != nil {
		t.Fatalf("create listing: %v", err)
	}

	svc := NewOnChainPerpsService(db, &stubPriceAggregator{prices: map[string]float64{"BTC": 61234.5}})
	if err := svc.RegisterDeployment(h.Deployment()); err != nil {
		t.Fatalf("register deployment: %v", err)
	}
	priv, err := crypto.HexToECDSA(perpstest.AnvilKey)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	svc.SetKeeperKey(priv)

	// The harness seeds the market at $60,000; the writer must overwrite it with
	// the price source's value.
	if err := svc.pushMarkPrices(context.Background(), h.ChainID); err != nil {
		t.Fatalf("pushMarkPrices: %v", err)
	}

	client := h.Client()
	price, err := client.MarkPrice(context.Background(), perps.MarketID("BTC/RUSD"))
	if err != nil {
		t.Fatalf("read mark price: %v", err)
	}
	want, _ := usdFloatToWAD(61234.5)
	if price.Cmp(want) != 0 {
		t.Fatalf("mark price not pushed: want %s, got %s", want.String(), price.String())
	}
}

// TestOracleWriterRefusesWrongSigner proves the writer fails closed when its
// signer is not the contract's oracle, rather than broadcasting transactions the
// chain would reject on every tick.
func TestOracleWriterRefusesWrongSigner(t *testing.T) {
	h := perpstest.Start(t)
	db := newOnChainPerpsTestDB(t)
	if err := db.Create(&models.FuturesMarket{Symbol: "BTC/RUSD", BaseAsset: "BTC", Status: models.ListingStatusActive}).Error; err != nil {
		t.Fatalf("create listing: %v", err)
	}

	svc := NewOnChainPerpsService(db, &stubPriceAggregator{prices: map[string]float64{"BTC": 61234.5}})
	if err := svc.RegisterDeployment(h.Deployment()); err != nil {
		t.Fatalf("register deployment: %v", err)
	}
	// A key that is not the contract's oracle.
	other, err := crypto.HexToECDSA("59c6995e998f97a5a0044966f0945389dc9e86dae88c7a8412f4603b6b78690d")
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	svc.SetKeeperKey(other)

	if err := svc.pushMarkPrices(context.Background(), h.ChainID); err == nil {
		t.Fatal("a signer that is not the contract oracle must be refused")
	}
}

// TestOracleWriterSkipsUnlistedMarkets proves a listing with no on-chain market
// is skipped rather than priced, so the writer never targets a market the
// contract does not know.
func TestOracleWriterSkipsUnlistedMarkets(t *testing.T) {
	h := perpstest.Start(t)
	db := newOnChainPerpsTestDB(t)
	// A listing with no corresponding on-chain market.
	if err := db.Create(&models.FuturesMarket{Symbol: "DOGE/RUSD", BaseAsset: "DOGE", Status: models.ListingStatusActive}).Error; err != nil {
		t.Fatalf("create listing: %v", err)
	}

	svc := NewOnChainPerpsService(db, &stubPriceAggregator{prices: map[string]float64{"DOGE": 0.2}})
	if err := svc.RegisterDeployment(h.Deployment()); err != nil {
		t.Fatalf("register deployment: %v", err)
	}
	priv, err := crypto.HexToECDSA(perpstest.AnvilKey)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	svc.SetKeeperKey(priv)

	if err := svc.pushMarkPrices(context.Background(), h.ChainID); err != nil {
		t.Fatalf("a listing with no on-chain market must be skipped, not fail: %v", err)
	}
}

// TestPriceableListingsDedupesAndSkipsOrderbook proves the listing projection
// keeps one entry per symbol and ignores an orderbook perp listing, which the
// CLOB matches rather than this engine.
func TestPriceableListingsDedupesAndSkipsOrderbook(t *testing.T) {
	db := newOnChainPerpsTestDB(t)
	if err := db.AutoMigrate(&models.PerpsMarket{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	rows := []interface{}{
		&models.FuturesMarket{Symbol: "BTC/RUSD", BaseAsset: "BTC", Status: models.ListingStatusActive},
		&models.PerpsMarket{Symbol: "BTC/RUSD", MarketType: "oracle", Status: models.ListingStatusActive},
		&models.PerpsMarket{Symbol: "ETH/RUSD", MarketType: "orderbook", Status: models.ListingStatusActive},
		&models.FuturesMarket{Symbol: "PAUSED/RUSD", BaseAsset: "PAUSED", Status: models.ListingStatusPaused},
	}
	for _, r := range rows {
		if err := db.Create(r).Error; err != nil {
			t.Fatalf("create: %v", err)
		}
	}
	svc := NewOnChainPerpsService(db, nil)
	got, err := svc.priceableListings()
	if err != nil {
		t.Fatalf("priceableListings: %v", err)
	}
	if len(got) != 1 || got[0].symbol != "BTC/RUSD" || got[0].oracleSymbol != "BTC" {
		t.Fatalf("want exactly one BTC/RUSD listing, got %+v", got)
	}
}
