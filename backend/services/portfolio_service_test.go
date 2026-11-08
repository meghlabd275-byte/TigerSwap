package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

var portfolioDBSeq uint64

// newPortfolioTestEnv builds a portfolio service over a real in-memory
// database. Holdings, valuation and PnL are exercised against actual rows
// rather than stubs, because the service's whole purpose is to reconcile
// against the platform's own ledger.
func newPortfolioTestEnv(t *testing.T, prices map[string]float64) (*PortfolioService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:portfolio-%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&portfolioDBSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })

	if err := db.AutoMigrate(
		&models.AccountBalance{}, &models.Transaction{}, &models.PerpetualPosition{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var price priceSource
	if prices != nil {
		price = &stubPriceAggregator{prices: prices}
	}
	return NewPortfolioService(db, price), db
}

func seedBalance(t *testing.T, db *gorm.DB, userID string, chainID int64, address, symbol string, decimals int, available, locked string) {
	t.Helper()
	row := models.AccountBalance{
		UserID:       userID,
		ChainID:      chainID,
		TokenAddress: address,
		Symbol:       symbol,
		Decimals:     decimals,
		Available:    available,
		Locked:       locked,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed balance: %v", err)
	}
}

func TestPortfolioHoldingsAreValuedAtTheOracle(t *testing.T) {
	svc, db := newPortfolioTestEnv(t, map[string]float64{"ETH": 2000, "USDT": 1})
	const user = "user-1"
	// 2 ETH (18 decimals) and 500 USDT (6 decimals).
	seedBalance(t, db, user, 1, "0x0000000000000000000000000000000000000001", "ETH", 18, "2000000000000000000", "0")
	seedBalance(t, db, user, 1, "0x0000000000000000000000000000000000000002", "USDT", 6, "500000000", "0")

	summary, err := svc.Summary(context.Background(), user)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if len(summary.Assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(summary.Assets))
	}
	// 2*2000 + 500 = 4500
	if summary.TotalValueUSD < 4499.99 || summary.TotalValueUSD > 4500.01 {
		t.Fatalf("total value = %v, want 4500", summary.TotalValueUSD)
	}
	for _, asset := range summary.Assets {
		if asset.ValueUSD == nil {
			t.Fatalf("asset %s has no USD value", asset.Symbol)
		}
	}
}

func TestPortfolioCountsLockedBalance(t *testing.T) {
	svc, db := newPortfolioTestEnv(t, map[string]float64{"TGR": 2})
	const user = "user-2"
	seedBalance(t, db, user, 1, "0x0000000000000000000000000000000000000003", "TGR", 18, "1000000000000000000", "500000000000000000")

	summary, err := svc.Summary(context.Background(), user)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if len(summary.Assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(summary.Assets))
	}
	// 1.5 TGR * 2 = 3.0
	if *summary.Assets[0].ValueUSD < 2.99 || *summary.Assets[0].ValueUSD > 3.01 {
		t.Fatalf("value = %v, want 3.0", *summary.Assets[0].ValueUSD)
	}
	if summary.Assets[0].Balance != "1500000000000000000" {
		t.Fatalf("balance = %q, want 1500000000000000000", summary.Assets[0].Balance)
	}
}

func TestPortfolioUnpricedHoldingIsNullAndExcludedFromTotal(t *testing.T) {
	svc, db := newPortfolioTestEnv(t, map[string]float64{"ETH": 1000})
	const user = "user-3"
	seedBalance(t, db, user, 1, "0x0000000000000000000000000000000000000001", "ETH", 18, "1000000000000000000", "0")
	seedBalance(t, db, user, 1, "0x0000000000000000000000000000000000000009", "MYSTERY", 18, "5000000000000000000", "0")

	summary, err := svc.Summary(context.Background(), user)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	var mystery *portfolioAssetDTO
	for i := range summary.Assets {
		if summary.Assets[i].Symbol == "MYSTERY" {
			mystery = &summary.Assets[i]
		}
	}
	if mystery == nil {
		t.Fatal("MYSTERY holding missing")
	}
	if mystery.ValueUSD != nil {
		t.Fatalf("unpriced holding should be null, got %v", *mystery.ValueUSD)
	}
	if summary.TotalValueUSD < 999.99 || summary.TotalValueUSD > 1000.01 {
		t.Fatalf("total = %v, want 1000 (unpriced excluded)", summary.TotalValueUSD)
	}
}

func TestPortfolioZeroBalanceIsOmitted(t *testing.T) {
	svc, db := newPortfolioTestEnv(t, map[string]float64{"ETH": 1000})
	const user = "user-4"
	seedBalance(t, db, user, 1, "0x0000000000000000000000000000000000000001", "ETH", 18, "0", "0")

	summary, err := svc.Summary(context.Background(), user)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if len(summary.Assets) != 0 {
		t.Fatalf("expected no assets for a zero balance, got %d", len(summary.Assets))
	}
	if summary.TotalValueUSD != 0 {
		t.Fatalf("total = %v, want 0", summary.TotalValueUSD)
	}
}

func TestPortfolioIsScopedToTheCaller(t *testing.T) {
	svc, db := newPortfolioTestEnv(t, map[string]float64{"ETH": 1000})
	seedBalance(t, db, "owner", 1, "0x0000000000000000000000000000000000000001", "ETH", 18, "1000000000000000000", "0")
	seedBalance(t, db, "intruder", 1, "0x0000000000000000000000000000000000000002", "ETH", 18, "9000000000000000000", "0")

	summary, err := svc.Summary(context.Background(), "owner")
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if len(summary.Assets) != 1 || summary.Assets[0].Balance != "1000000000000000000" {
		t.Fatalf("portfolio leaked another user's rows: %+v", summary.Assets)
	}
}

func TestPortfolioPnLSplitsRealizedAndUnrealized(t *testing.T) {
	svc, db := newPortfolioTestEnv(t, map[string]float64{"ETH": 1000})
	const user = "user-5"
	open := models.PerpetualPosition{UserID: user, Pair: "ETH/RUSD", Side: "long", Status: "open", PNL: "150.5"}
	closed := models.PerpetualPosition{UserID: user, Pair: "BTC/RUSD", Side: "short", Status: "closed", PNL: "-40.25"}
	other := models.PerpetualPosition{UserID: "someone-else", Pair: "ETH/RUSD", Side: "long", Status: "open", PNL: "9999"}
	for _, position := range []models.PerpetualPosition{open, closed, other} {
		row := position
		if err := db.Create(&row).Error; err != nil {
			t.Fatalf("seed position: %v", err)
		}
	}

	summary, err := svc.Summary(context.Background(), user)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.OpenPositions != 1 {
		t.Fatalf("open positions = %d, want 1", summary.OpenPositions)
	}
	if summary.UnrealizedPnLUSD != 150.5 {
		t.Fatalf("unrealized = %v, want 150.5", summary.UnrealizedPnLUSD)
	}
	if summary.RealizedPnLUSD != -40.25 {
		t.Fatalf("realized = %v, want -40.25", summary.RealizedPnLUSD)
	}
}

// stubPositionSource stands in for the live derivatives engine. The engine is
// the authority on an open position's current PnL; the persisted row only holds
// the value it was opened with.
type stubPositionSource struct {
	positions map[string][]*PerpetualPosition
}

func (s *stubPositionSource) GetUserPositions(userAddress string) []*PerpetualPosition {
	return s.positions[userAddress]
}

func TestPortfolioUnrealizedPnLUsesLiveEngineValue(t *testing.T) {
	svc, db := newPortfolioTestEnv(t, map[string]float64{"ETH": 1000})
	const user = "user-live"
	// The persisted row still carries the opening value of zero; the engine has
	// since re-marked the position to +250.
	stored := models.PerpetualPosition{
		UserID: user, Pair: "ETH/RUSD", Side: "long", Status: "open", PNL: "0",
	}
	if err := db.Create(&stored).Error; err != nil {
		t.Fatalf("seed position: %v", err)
	}
	svc.SetPositionSource(&stubPositionSource{positions: map[string][]*PerpetualPosition{
		user: {{ID: stored.ID.String(), UserAddress: user, Pair: "ETH/RUSD", Status: "open", UnrealizedPNL: "250"}},
	}})

	summary, err := svc.Summary(context.Background(), user)
	if err != nil {
		t.Fatalf("summary: %v", err)
	}
	if summary.UnrealizedPnLUSD != 250 {
		t.Fatalf("unrealized = %v, want the live engine value 250", summary.UnrealizedPnLUSD)
	}
	if summary.OpenPositions != 1 {
		t.Fatalf("open positions = %d, want 1 (not double-counted)", summary.OpenPositions)
	}
}

func TestPortfolioTransactionsAreCamelCaseAndArrayShaped(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, db := newPortfolioTestEnv(t, map[string]float64{"ETH": 1000})
	user := uuid.NewString()
	tx := models.Transaction{
		UserID:    uuid.MustParse(user),
		Hash:      "0xabc",
		ChainID:   1,
		Type:      "swap",
		ToToken:   "ETH",
		ToAmount:  "1000000000000000000",
		Pair:      "RUSD/ETH",
		Status:    "confirmed",
		Timestamp: time.Now().UTC(),
	}
	if err := db.Create(&tx).Error; err != nil {
		t.Fatalf("seed tx: %v", err)
	}

	router := gin.New()
	router.GET("/user/transactions", func(c *gin.Context) {
		c.Set("user_id", user)
		svc.GetTransactionsHandler(c)
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/user/transactions", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	// The frontend's list validator requires a bare JSON array.
	var rows []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &rows); err != nil {
		t.Fatalf("body is not a JSON array: %v (%s)", err, rec.Body.String())
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(rows))
	}
	for _, key := range []string{"hash", "chainId", "type", "status", "amount", "tokenSymbol", "createdAt"} {
		if _, ok := rows[0][key]; !ok {
			t.Fatalf("transaction is missing %q: %v", key, rows[0])
		}
	}
}

func TestPortfolioHandlerRequiresAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, _ := newPortfolioTestEnv(t, nil)
	router := gin.New()
	router.GET("/user/portfolio", svc.GetPortfolioHandler)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/user/portfolio", nil)
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

// TestPortfolioWireFormat pins the exact key set the frontend validators
// require. Run with -v to print the payloads, which is how the fixtures in
// frontend/src/lib/serverContract.test.ts are regenerated.
func TestPortfolioWireFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, db := newPortfolioTestEnv(t, map[string]float64{"ETH": 2000})
	user := uuid.NewString()
	seedBalance(t, db, user, 1, "0x0000000000000000000000000000000000000001", "ETH", 18, "2000000000000000000", "0")
	tx := models.Transaction{
		UserID:    uuid.MustParse(user),
		Hash:      "0xdeadbeef",
		ChainID:   1,
		Type:      "swap",
		ToToken:   "ETH",
		ToAmount:  "1000000000000000000",
		Pair:      "RUSD/ETH",
		Status:    "confirmed",
		Timestamp: time.Now().UTC(),
	}
	if err := db.Create(&tx).Error; err != nil {
		t.Fatalf("seed tx: %v", err)
	}

	rec := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(rec)
	context.Request = httptest.NewRequest(http.MethodGet, "/user/portfolio", nil)
	context.Set("user_id", user)
	svc.GetPortfolioHandler(context)
	if rec.Code != http.StatusOK {
		t.Fatalf("portfolio status = %d", rec.Code)
	}
	t.Logf("SERVER_PORTFOLIO = `%s`", rec.Body.String())
	assertKeys(t, rec.Body.String(),
		"totalValueUSD", "assets", "realizedPnlUsd", "unrealizedPnlUsd",
		"openPositions", "updatedAt",
	)

	rec2 := httptest.NewRecorder()
	context2, _ := gin.CreateTestContext(rec2)
	context2.Request = httptest.NewRequest(http.MethodGet, "/user/transactions", nil)
	context2.Set("user_id", user)
	svc.GetTransactionsHandler(context2)
	if rec2.Code != http.StatusOK {
		t.Fatalf("transactions status = %d", rec2.Code)
	}
	t.Logf("SERVER_TRANSACTIONS = `%s`", rec2.Body.String())
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(rec2.Body.Bytes(), &rows); err != nil {
		t.Fatalf("transactions body is not an array: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 transaction, got %d", len(rows))
	}
	assertKeys(t, string(rec2.Body.Bytes()[1:len(rec2.Body.Bytes())-1]),
		"hash", "chainId", "type", "status", "amount", "tokenSymbol", "createdAt",
	)
}
