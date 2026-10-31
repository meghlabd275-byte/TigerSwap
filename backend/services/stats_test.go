package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

var statsDBSeq uint64

func newStatsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&statsDBSeq, 1))
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
		&models.User{}, &models.Transaction{}, &models.Token{},
		&models.Pool{}, &models.Blockchain{}, &models.Order{}, &models.PerpetualPosition{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return db
}

func getJSON(t *testing.T, handler gin.HandlerFunc, path string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	handler(c)
	return rec
}

func decodeStats(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return out
}

// TestMarketStatsReportsZeroWhenThereIsNoActivity verifies the stats endpoint
// does not invent volume when nothing has traded.
func TestMarketStatsReportsZeroWhenThereIsNoActivity(t *testing.T) {
	db := newStatsTestDB(t)
	svc := NewSwapService(db, nil)

	rec := getJSON(t, svc.GetMarketStats, "/api/v1/market/stats")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	stats := decodeStats(t, rec)
	for _, key := range []string{"totalTVL", "volume24h"} {
		if stats[key].(float64) != 0 {
			t.Fatalf("expected %s to be 0 with no activity, got %v", key, stats[key])
		}
	}
	if stats["totalPairs"].(float64) != 0 {
		t.Fatalf("expected totalPairs 0 with no pools, got %v", stats["totalPairs"])
	}
}

// TestMarketStatsAggregatesConfirmedSwaps verifies volume and fees are computed
// from real confirmed swap rows and exclude pending or failed ones.
func TestMarketStatsAggregatesConfirmedSwaps(t *testing.T) {
	db := newStatsTestDB(t)
	svc := NewSwapService(db, nil)

	// A pool with a 0.3% fee tier.
	if err := db.Create(&models.Pool{
		PoolAddress: "0xpool1", TokenAAddress: "0xa", TokenBAddress: "0xb",
		Fee: 30, IsActive: true, TvlUSD: 500000,
	}).Error; err != nil {
		t.Fatalf("seed pool: %v", err)
	}

	now := time.Now()
	rows := []models.Transaction{
		{Hash: "0x1", Type: "swap", Status: "confirmed", Pair: "0xpool1", AmountUSD: 10000, Timestamp: now.Add(-time.Hour)},
		{Hash: "0x2", Type: "swap", Status: "confirmed", Pair: "0xpool1", AmountUSD: 5000, Timestamp: now.Add(-2 * time.Hour)},
		// Older than 24h but within 7d.
		{Hash: "0x3", Type: "swap", Status: "confirmed", Pair: "0xpool1", AmountUSD: 20000, Timestamp: now.Add(-48 * time.Hour)},
		// Not confirmed: must be excluded.
		{Hash: "0x4", Type: "swap", Status: "pending", Pair: "0xpool1", AmountUSD: 99999, Timestamp: now},
		{Hash: "0x5", Type: "swap", Status: "failed", Pair: "0xpool1", AmountUSD: 99999, Timestamp: now},
		// Not a swap: must be excluded.
		{Hash: "0x6", Type: "bridge", Status: "confirmed", Pair: "0xpool1", AmountUSD: 99999, Timestamp: now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed transactions: %v", err)
	}

	stats := decodeStats(t, getJSON(t, svc.GetMarketStats, "/api/v1/market/stats"))

	if got := stats["volume24h"].(float64); got != 15000 {
		t.Fatalf("expected 24h volume 15000, got %v", got)
	}
	if got := stats["totalTVL"].(float64); got != 500000 {
		t.Fatalf("expected tvl 500000, got %v", got)
	}
	if got := stats["totalPairs"].(float64); got != 1 {
		t.Fatalf("expected 1 active pair, got %v", got)
	}

	// Fees and swap counts are not part of the public stats contract, so they are
	// asserted against the aggregation itself rather than through the endpoint.
	volume24h, fees24h, swaps24h := svc.swapTotals(time.Now().Add(-24 * time.Hour))
	if volume24h != 15000 {
		t.Fatalf("expected 24h volume 15000, got %v", volume24h)
	}
	// 15000 at 0.3% is 45.
	if fees24h != 45 {
		t.Fatalf("expected 24h fees 45, got %v", fees24h)
	}
	if swaps24h != 2 {
		t.Fatalf("expected 2 swaps in 24h, got %v", swaps24h)
	}
	volume7d, _, _ := svc.swapTotals(time.Now().Add(-7 * 24 * time.Hour))
	if volume7d != 35000 {
		t.Fatalf("expected 7d volume 35000, got %v", volume7d)
	}
}

// TestAdminDashboardAggregatesVolume verifies the admin dashboard derives its
// volume figure from confirmed swaps rather than reporting a constant.
func TestAdminDashboardAggregatesVolume(t *testing.T) {
	db := newStatsTestDB(t)
	svc := NewAdminService(db)

	now := time.Now()
	if err := db.Create(&[]models.Transaction{
		{Hash: "0xd1", Type: "swap", Status: "confirmed", AmountUSD: 4000, Timestamp: now.Add(-time.Minute)},
		{Hash: "0xd2", Type: "swap", Status: "confirmed", AmountUSD: 1000, Timestamp: now.Add(-time.Minute)},
		{Hash: "0xd3", Type: "swap", Status: "pending", AmountUSD: 5000, Timestamp: now},
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	stats := decodeStats(t, getJSON(t, svc.GetDashboard, "/api/v1/admin/dashboard"))
	if got := stats["totalVolume"].(float64); got != 5000 {
		t.Fatalf("expected 5000 volume, got %v", got)
	}
	if got := stats["totalTransactions"].(float64); got != 3 {
		t.Fatalf("expected 3 transactions, got %v", got)
	}
	if stats["totalUsers"] == nil || stats["activePools"] == nil {
		t.Fatal("expected the dashboard to report every aggregate")
	}
}
