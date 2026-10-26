package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

// newListingTestDB builds an isolated in-memory database with the listing
// tables migrated. SQLite is used only as the test driver; production runs on
// PostgreSQL via config.InitDB.
func newListingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	// A unique name per invocation keeps repeated runs (go test -count=N) from
	// sharing one in-memory database through the shared cache.
	dsn := fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&testDBSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	// Keep one connection alive so the in-memory database is not dropped
	// between statements.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })

	if err := db.AutoMigrate(&models.FuturesMarket{}, &models.PerpsMarket{}, &models.MarginMarket{}, &models.AdminLog{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return db
}

var testDBSeq uint64

// postJSON drives a handler with a JSON body and returns the recorder.
func postJSON(t *testing.T, handler gin.HandlerFunc, path string, body interface{}, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	handler(c)
	return rec
}

func decodeMarket[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return out
}

func TestCreateFuturesMarketPersistsValidListing(t *testing.T) {
	db := newListingTestDB(t)
	svc := NewListingService(db)

	rec := postJSON(t, svc.CreateFuturesMarket, "/api/v1/admin/futures/markets", gin.H{
		"symbol":                  "BTC/RUSD",
		"contract_type":           "RUSD_M",
		"settlement":              "RUSD",
		"contract_size":           "1",
		"tick_size":               "0.1",
		"max_leverage":            200,
		"funding_rate":            "0.0001",
		"funding_interval_hours":  8,
		"funding_rate_cap":        "0.01",
		"oracle_sources":          "CHAINLINK,PYTH,TIGER",
		"maintenance_margin_rate": 0.005,
		"initial_margin_rate":     0.008,
		"liquidation_fee_rate":    0.005,
		"insurance_fund_share":    0.1,
		"maker_fee_rate":          0.0,
		"taker_fee_rate":          0.0002,
	}, nil)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	market := decodeMarket[models.FuturesMarket](t, rec)
	if market.Symbol != "BTC/RUSD" || market.BaseAsset != "BTC" || market.QuoteAsset != "RUSD" {
		t.Fatalf("unexpected market: %+v", market)
	}
	if market.Status != models.ListingStatusPaused {
		t.Fatalf("new listing must start paused, got %q", market.Status)
	}

	var stored models.FuturesMarket
	if err := db.Where("symbol = ?", "BTC/RUSD").First(&stored).Error; err != nil {
		t.Fatalf("market was not persisted: %v", err)
	}
	if stored.ID != market.ID {
		t.Fatalf("persisted id mismatch")
	}

	var audits int64
	db.Model(&models.AdminLog{}).Where("action = ?", "create_futures_market").Count(&audits)
	if audits != 1 {
		t.Fatalf("expected one audit entry, got %d", audits)
	}
}

func TestCreateFuturesMarketRejectsBadParameters(t *testing.T) {
	db := newListingTestDB(t)
	svc := NewListingService(db)

	base := gin.H{
		"symbol":                  "ETH/RUSD",
		"contract_type":           "RUSD_M",
		"settlement":              "RUSD",
		"max_leverage":            100,
		"funding_interval_hours":  8,
		"maintenance_margin_rate": 0.005,
		"initial_margin_rate":     0.01,
		"liquidation_fee_rate":    0.005,
		"insurance_fund_share":    0.1,
	}

	cases := []struct {
		name    string
		mutate  func(gin.H)
		wantMsg string
	}{
		{"bad symbol", func(h gin.H) { h["symbol"] = "ETHRUSD" }, "symbol must have the form BASE/QUOTE"},
		{"bad settlement", func(h gin.H) { h["settlement"] = "DOGE" }, "unsupported settlement asset"},
		{"bad contract type", func(h gin.H) { h["contract_type"] = "XYZ_M" }, "unsupported contract type"},
		{"leverage too high", func(h gin.H) { h["max_leverage"] = 5000 }, "max_leverage must be between 1 and 1001"},
		{"initial below maintenance", func(h gin.H) { h["initial_margin_rate"] = 0.001 }, "initial_margin_rate must be at least the maintenance margin rate"},
		{"bad funding interval", func(h gin.H) { h["funding_interval_hours"] = 0 }, "funding_interval_hours must be between 1 and 24"},
		{"bad listing type", func(h gin.H) { h["listing_type"] = "weekly" }, "unsupported listing type"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := gin.H{}
			for k, v := range base {
				body[k] = v
			}
			tc.mutate(body)

			rec := postJSON(t, svc.CreateFuturesMarket, "/api/v1/admin/futures/markets", body, nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
			}
			var payload map[string]string
			json.Unmarshal(rec.Body.Bytes(), &payload)
			if payload["error"] != tc.wantMsg {
				t.Fatalf("expected error %q, got %q", tc.wantMsg, payload["error"])
			}
		})
	}
}

func TestCreateFuturesMarketRejectsDuplicateSymbol(t *testing.T) {
	db := newListingTestDB(t)
	svc := NewListingService(db)

	body := gin.H{
		"symbol":                  "SOL/RUSD",
		"contract_type":           "RUSD_M",
		"settlement":              "RUSD",
		"max_leverage":            100,
		"funding_interval_hours":  8,
		"maintenance_margin_rate": 0.005,
		"initial_margin_rate":     0.01,
		"liquidation_fee_rate":    0.005,
		"insurance_fund_share":    0.1,
	}
	if rec := postJSON(t, svc.CreateFuturesMarket, "/x", body, nil); rec.Code != http.StatusCreated {
		t.Fatalf("first create failed: %d %s", rec.Code, rec.Body.String())
	}
	rec := postJSON(t, svc.CreateFuturesMarket, "/x", body, nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 on duplicate, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreatePerpsMarketSupportsUsdtAndUsdc(t *testing.T) {
	db := newListingTestDB(t)
	svc := NewListingService(db)

	for _, symbol := range []string{"BTC/USDT", "ETH/USDC"} {
		rec := postJSON(t, svc.CreatePerpsMarket, "/x", gin.H{
			"symbol":                  symbol,
			"market_type":             "orderbook",
			"collateral_assets":       "RUSD,USDT,USDC",
			"max_open_interest":       "1000000",
			"max_leverage":            100,
			"price_impact_rate":       0.0005,
			"borrow_fee_per_hour":     0.0001,
			"maker_fee_rate":          0.0,
			"taker_fee_rate":          0.0002,
			"maintenance_margin_rate": 0.005,
			"liquidation_fee_rate":    0.005,
		}, nil)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %s failed: %d %s", symbol, rec.Code, rec.Body.String())
		}
	}

	var count int64
	db.Model(&models.PerpsMarket{}).Count(&count)
	if count != 2 {
		t.Fatalf("expected 2 perps markets, got %d", count)
	}
}

func TestCreatePerpsMarketRejectsBadCollateral(t *testing.T) {
	db := newListingTestDB(t)
	svc := NewListingService(db)

	rec := postJSON(t, svc.CreatePerpsMarket, "/x", gin.H{
		"symbol":              "BTC/RUSD",
		"collateral_assets":   "RUSD,SHIB",
		"max_open_interest":   "1000000",
		"max_leverage":        100,
		"borrow_fee_per_hour": 0.0001,
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateMarginMarketEnforcesRiskInvariants(t *testing.T) {
	db := newListingTestDB(t)
	svc := NewListingService(db)

	valid := gin.H{
		"pair":                  "BTC/RUSD",
		"margin_mode":           "cross",
		"max_leverage":          10,
		"collateral_assets":     "BTC,ETH,TGR,RUSD",
		"borrowable_assets":     "RUSD,USDT,USDC",
		"base_rate_apy":         0.05,
		"kink_utilization":      0.8,
		"jump_multiplier":       1.0,
		"ltv":                   0.7,
		"liquidation_threshold": 0.75,
		"liquidation_bonus":     0.05,
		"close_factor":          0.5,
		"supply_cap":            "10000000",
		"borrow_cap":            "10000000",
		"risk_tier":             "low",
	}

	rec := postJSON(t, svc.CreateMarginMarket, "/x", valid, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	market := decodeMarket[models.MarginMarket](t, rec)
	if market.LTV != 0.7 || market.LiquidationThreshold != 0.75 {
		t.Fatalf("risk parameters not stored: %+v", market)
	}
	if market.Status != models.ListingStatusPaused {
		t.Fatalf("new margin listing must start paused")
	}

	badThreshold := gin.H{}
	for k, v := range valid {
		badThreshold[k] = v
	}
	badThreshold["pair"] = "ETH/RUSD"
	badThreshold["liquidation_threshold"] = 0.5
	if rec := postJSON(t, svc.CreateMarginMarket, "/x", badThreshold, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for threshold below LTV, got %d", rec.Code)
	}

	badLeverage := gin.H{}
	for k, v := range valid {
		badLeverage[k] = v
	}
	badLeverage["pair"] = "TGR/RUSD"
	badLeverage["max_leverage"] = 25
	if rec := postJSON(t, svc.CreateMarginMarket, "/x", badLeverage, nil); rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for spot margin leverage above 10, got %d", rec.Code)
	}
}

func TestUpdateFuturesMarketAppliesAndValidates(t *testing.T) {
	db := newListingTestDB(t)
	svc := NewListingService(db)

	create := postJSON(t, svc.CreateFuturesMarket, "/x", gin.H{
		"symbol":                  "TGR/RUSD",
		"contract_type":           "TGR_M",
		"settlement":              "RUSD",
		"max_leverage":            50,
		"funding_interval_hours":  8,
		"maintenance_margin_rate": 0.005,
		"initial_margin_rate":     0.02,
		"liquidation_fee_rate":    0.005,
		"insurance_fund_share":    0.1,
	}, nil)
	if create.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", create.Code, create.Body.String())
	}
	market := decodeMarket[models.FuturesMarket](t, create)

	rec := postJSON(t, svc.UpdateFuturesMarket, "/x",
		gin.H{"status": models.ListingStatusActive, "max_leverage": 125},
		gin.Params{{Key: "id", Value: market.ID.String()}})
	if rec.Code != http.StatusOK {
		t.Fatalf("update failed: %d %s", rec.Code, rec.Body.String())
	}
	updated := decodeMarket[models.FuturesMarket](t, rec)
	if updated.Status != models.ListingStatusActive || updated.MaxLeverage != 125 {
		t.Fatalf("update not applied: %+v", updated)
	}

	rec = postJSON(t, svc.UpdateFuturesMarket, "/x",
		gin.H{"status": "bogus"},
		gin.Params{{Key: "id", Value: market.ID.String()}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bogus status, got %d", rec.Code)
	}

	rec = postJSON(t, svc.UpdateFuturesMarket, "/x",
		gin.H{"max_leverage": 10},
		gin.Params{{Key: "id", Value: "not-a-uuid"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid id, got %d", rec.Code)
	}
}

func TestDeleteMovesListingToDelisting(t *testing.T) {
	db := newListingTestDB(t)
	svc := NewListingService(db)

	create := postJSON(t, svc.CreateMarginMarket, "/x", gin.H{
		"pair":                  "ETH/RUSD",
		"collateral_assets":     "ETH,RUSD",
		"borrowable_assets":     "RUSD",
		"base_rate_apy":         0.05,
		"kink_utilization":      0.8,
		"jump_multiplier":       1.0,
		"ltv":                   0.65,
		"liquidation_threshold": 0.7,
		"liquidation_bonus":     0.05,
		"close_factor":          0.5,
		"supply_cap":            "1000000",
		"borrow_cap":            "1000000",
		"max_leverage":          10,
	}, nil)
	if create.Code != http.StatusCreated {
		t.Fatalf("create failed: %d %s", create.Code, create.Body.String())
	}
	market := decodeMarket[models.MarginMarket](t, create)

	rec := postJSON(t, svc.DeleteMarginMarket, "/x", gin.H{},
		gin.Params{{Key: "id", Value: market.ID.String()}})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete failed: %d %s", rec.Code, rec.Body.String())
	}

	var stored models.MarginMarket
	if err := db.Where("id = ?", market.ID).First(&stored).Error; err != nil {
		t.Fatalf("market disappeared: %v", err)
	}
	if stored.Status != models.ListingStatusDelisting {
		t.Fatalf("expected delisting status, got %q", stored.Status)
	}
}
