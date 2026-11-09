package services

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

var analyticsDBSeq uint64

// newAnalyticsTestEnv builds an analytics service over a real in-memory
// database. The rollup, the series and the leaderboard are exercised against
// actual rows rather than stubs, because the whole point of the service is that
// it reconciles against the platform's own data.
func newAnalyticsTestEnv(t *testing.T, prices map[string]float64) (*AnalyticsService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:analytics-%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&analyticsDBSeq, 1))
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
		&models.Pool{}, &models.Transaction{}, &models.PerpetualPosition{},
		&models.MarginBorrow{}, &models.FarmPool{}, &models.Token{},
		&models.Blockchain{}, &models.User{}, &models.NFTListing{},
		&models.VolumeBucket{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	var price priceSource
	if prices != nil {
		price = &stubPriceAggregator{prices: prices}
	}
	return NewAnalyticsService(db, price), db
}

// seedSwap inserts a confirmed swap at a timestamp.
func seedSwap(t *testing.T, db *gorm.DB, pair string, chainID int64, amountUSD float64, side string, at time.Time) {
	t.Helper()
	tx := models.Transaction{
		Hash:      fmt.Sprintf("0x%s-%d", pair, at.UnixNano()),
		ChainID:   chainID,
		Type:      "swap",
		Pair:      pair,
		Side:      side,
		AmountUSD: amountUSD,
		Status:    "confirmed",
		Timestamp: at,
	}
	if err := db.Create(&tx).Error; err != nil {
		t.Fatalf("seed swap: %v", err)
	}
}

func TestRollupVolumeAggregatesConfirmedSwaps(t *testing.T) {
	svc, db := newAnalyticsTestEnv(t, nil)
	base := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

	// Two swaps in the same minute and one in the next minute.
	seedSwap(t, db, "TGR/RUSD", 1, 100, "buy", base.Add(5*time.Second))
	seedSwap(t, db, "TGR/RUSD", 1, 50, "sell", base.Add(40*time.Second))
	seedSwap(t, db, "TGR/RUSD", 1, 25, "buy", base.Add(70*time.Second))

	// A pending swap must not be counted: only confirmed volume is real.
	pending := models.Transaction{
		Hash: "0xpending", ChainID: 1, Type: "swap", Pair: "TGR/RUSD",
		Side: "buy", AmountUSD: 9999, Status: "pending", Timestamp: base.Add(10 * time.Second),
	}
	if err := db.Create(&pending).Error; err != nil {
		t.Fatalf("seed pending: %v", err)
	}

	written, err := svc.RollupVolume(context.Background(), base, base.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("RollupVolume: %v", err)
	}
	if written != 2 {
		t.Fatalf("expected 2 buckets (one per minute), got %d", written)
	}

	var buckets []models.VolumeBucket
	if err := db.Order("bucket_start ASC").Find(&buckets).Error; err != nil {
		t.Fatalf("load buckets: %v", err)
	}
	if buckets[0].VolumeUSD != 150 {
		t.Errorf("first bucket volume = %v, want 150", buckets[0].VolumeUSD)
	}
	if buckets[0].TradeCount != 2 {
		t.Errorf("first bucket trades = %d, want 2", buckets[0].TradeCount)
	}
	if buckets[0].TakerBuyVolumeUSD != 100 || buckets[0].TakerSellVolumeUSD != 50 {
		t.Errorf("first bucket buy/sell = %v/%v, want 100/50", buckets[0].TakerBuyVolumeUSD, buckets[0].TakerSellVolumeUSD)
	}
	if buckets[1].VolumeUSD != 25 || buckets[1].TradeCount != 1 {
		t.Errorf("second bucket = %+v, want volume 25 and 1 trade", buckets[1])
	}
	// Fees at the default 30 bps: 150 * 0.003 = 0.45.
	if diff := buckets[0].FeesUSD - 0.45; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("first bucket fees = %v, want 0.45", buckets[0].FeesUSD)
	}
}

func TestRollupVolumeIsIdempotent(t *testing.T) {
	svc, db := newAnalyticsTestEnv(t, nil)
	base := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	seedSwap(t, db, "TGR/RUSD", 1, 100, "buy", base.Add(5*time.Second))

	for i := 0; i < 3; i++ {
		if _, err := svc.RollupVolume(context.Background(), base, base.Add(time.Minute)); err != nil {
			t.Fatalf("RollupVolume run %d: %v", i, err)
		}
	}

	var buckets []models.VolumeBucket
	if err := db.Find(&buckets).Error; err != nil {
		t.Fatalf("load buckets: %v", err)
	}
	// Re-running the same window must not append a second bucket.
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket after 3 identical runs, got %d", len(buckets))
	}
	if buckets[0].VolumeUSD != 100 {
		t.Errorf("volume = %v, want 100 (not multiplied by the run count)", buckets[0].VolumeUSD)
	}
}

func TestRollupVolumeCorrectsAChangedWindow(t *testing.T) {
	svc, db := newAnalyticsTestEnv(t, nil)
	base := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	seedSwap(t, db, "TGR/RUSD", 1, 100, "buy", base.Add(5*time.Second))

	if _, err := svc.RollupVolume(context.Background(), base, base.Add(time.Minute)); err != nil {
		t.Fatalf("first rollup: %v", err)
	}
	// A late confirmation lands in the same window.
	seedSwap(t, db, "TGR/RUSD", 1, 60, "buy", base.Add(20*time.Second))
	if _, err := svc.RollupVolume(context.Background(), base, base.Add(time.Minute)); err != nil {
		t.Fatalf("second rollup: %v", err)
	}

	var buckets []models.VolumeBucket
	if err := db.Find(&buckets).Error; err != nil {
		t.Fatalf("load buckets: %v", err)
	}
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(buckets))
	}
	if buckets[0].VolumeUSD != 160 {
		t.Errorf("volume = %v, want 160 (the window is rebuilt, not appended)", buckets[0].VolumeUSD)
	}
}

func TestRollupRejectsAnInvertedWindow(t *testing.T) {
	svc, _ := newAnalyticsTestEnv(t, nil)
	base := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	if _, err := svc.RollupVolume(context.Background(), base, base); err == nil {
		t.Fatal("an empty window should be rejected, not silently rolled up")
	}
}

func TestRollupExcludesNonQuotePairs(t *testing.T) {
	svc, db := newAnalyticsTestEnv(t, nil)
	base := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

	seedSwap(t, db, "TGR/RUSD", 1, 100, "buy", base.Add(5*time.Second))
	// A pair quoted in an asset with no USD notional is excluded rather than
	// valued at a fabricated rate.
	seedSwap(t, db, "TGR/FOO", 1, 500, "buy", base.Add(6*time.Second))
	// A malformed pair symbol is excluded too.
	seedSwap(t, db, "TGRRUSD", 1, 700, "buy", base.Add(7*time.Second))

	if _, err := svc.RollupVolume(context.Background(), base, base.Add(time.Minute)); err != nil {
		t.Fatalf("RollupVolume: %v", err)
	}

	var buckets []models.VolumeBucket
	if err := db.Find(&buckets).Error; err != nil {
		t.Fatalf("load buckets: %v", err)
	}
	if len(buckets) != 1 || buckets[0].Pair != "TGR/RUSD" {
		t.Fatalf("expected only the TGR/RUSD bucket, got %+v", buckets)
	}
	if buckets[0].VolumeUSD != 100 {
		t.Errorf("volume = %v, want 100", buckets[0].VolumeUSD)
	}
}

func TestRollupUsesThePoolFeeTier(t *testing.T) {
	svc, db := newAnalyticsTestEnv(t, nil)
	base := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)

	// A pool with a 1% tier, matched by its address, so the rollup must use 100
	// bps rather than the 30 bps default.
	pool := models.Pool{
		PoolAddress: "TGR/RUSD", ChainID: 1, Fee: 100, IsActive: true,
		TokenAAddress: "0xaaa", TokenBAddress: "0xbbb",
	}
	if err := db.Create(&pool).Error; err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	seedSwap(t, db, "TGR/RUSD", 1, 1000, "buy", base.Add(5*time.Second))

	if _, err := svc.RollupVolume(context.Background(), base, base.Add(time.Minute)); err != nil {
		t.Fatalf("RollupVolume: %v", err)
	}

	var bucket models.VolumeBucket
	if err := db.First(&bucket).Error; err != nil {
		t.Fatalf("load bucket: %v", err)
	}
	// 1000 at 100 bps = 10.
	if diff := bucket.FeesUSD - 10; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("fees = %v, want 10 (the pool's own 1%% tier)", bucket.FeesUSD)
	}
}

func TestVolumeSeriesBucketsIntoFixedWidthPoints(t *testing.T) {
	svc, db := newAnalyticsTestEnv(t, nil)
	base := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 4; i++ {
		seedSwap(t, db, "TGR/RUSD", 1, 10, "buy", base.Add(time.Duration(i)*time.Minute+5*time.Second))
	}
	if _, err := svc.RollupVolume(context.Background(), base, base.Add(10*time.Minute)); err != nil {
		t.Fatalf("RollupVolume: %v", err)
	}

	// Four one-minute buckets summed into two-minute points.
	series, err := svc.VolumeSeries(context.Background(), "TGR/RUSD", 0, base, 2*time.Minute)
	if err != nil {
		t.Fatalf("VolumeSeries: %v", err)
	}
	if len(series) != 2 {
		t.Fatalf("expected 2 points, got %d", len(series))
	}
	if series[0].VolumeUSD != 20 || series[1].VolumeUSD != 20 {
		t.Errorf("series volumes = %v/%v, want 20/20", series[0].VolumeUSD, series[1].VolumeUSD)
	}
	if series[0].TradeCount != 2 {
		t.Errorf("first point trades = %d, want 2", series[0].TradeCount)
	}
	if !series[0].Timestamp.Equal(base) {
		t.Errorf("first point timestamp = %v, want %v", series[0].Timestamp, base)
	}
}

func TestVolumeSeriesRejectsANonMultipleInterval(t *testing.T) {
	svc, _ := newAnalyticsTestEnv(t, nil)
	// 90s is not a whole multiple of the 60s bucket, so points would mix widths.
	if _, err := svc.VolumeSeries(context.Background(), "", 0, time.Now().Add(-time.Hour), 90*time.Second); err == nil {
		t.Fatal("a non-multiple interval should be rejected")
	}
}

func TestPairLeaderboardRanksByVolume(t *testing.T) {
	svc, db := newAnalyticsTestEnv(t, nil)
	base := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	seedSwap(t, db, "TGR/RUSD", 1, 100, "buy", base.Add(5*time.Second))
	seedSwap(t, db, "ETH/RUSD", 1, 900, "sell", base.Add(6*time.Second))
	seedSwap(t, db, "ETH/RUSD", 1, 100, "buy", base.Add(7*time.Second))

	if _, err := svc.RollupVolume(context.Background(), base, base.Add(time.Minute)); err != nil {
		t.Fatalf("RollupVolume: %v", err)
	}

	pairs, err := svc.PairLeaderboard(context.Background(), base, 0, 10)
	if err != nil {
		t.Fatalf("PairLeaderboard: %v", err)
	}
	if len(pairs) != 2 {
		t.Fatalf("expected 2 pairs, got %d", len(pairs))
	}
	if pairs[0].Pair != "ETH/RUSD" || pairs[0].VolumeUSD != 1000 {
		t.Errorf("top pair = %+v, want ETH/RUSD at 1000", pairs[0])
	}
	if pairs[0].BuyVolume != 100 || pairs[0].SellVolume != 900 {
		t.Errorf("top pair buy/sell = %v/%v, want 100/900", pairs[0].BuyVolume, pairs[0].SellVolume)
	}
}

func TestOverviewCountsRealRows(t *testing.T) {
	svc, db := newAnalyticsTestEnv(t, nil)
	now := time.Now().UTC()

	if err := db.Create(&models.Pool{PoolAddress: "0xpool1", ChainID: 1, IsActive: true, TvlUSD: 1000}).Error; err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	if err := db.Create(&models.Pool{PoolAddress: "0xpool2", ChainID: 1, IsActive: true, TvlUSD: 500}).Error; err != nil {
		t.Fatalf("seed pool: %v", err)
	}
	// Deactivate through an update, which is how an operator pauses a pool. A
	// Create with the false zero value would let GORM apply the column default.
	if err := db.Model(&models.Pool{}).Where("pool_address = ?", "0xpool2").
		Update("is_active", false).Error; err != nil {
		t.Fatalf("deactivate pool: %v", err)
	}
	if err := db.Create(&models.FarmPool{Key: "f1", Name: "F1", Status: models.FarmStatusActive}).Error; err != nil {
		t.Fatalf("seed farm: %v", err)
	}
	if err := db.Create(&models.Blockchain{Name: "Ethereum", IsActive: true}).Error; err != nil {
		t.Fatalf("seed chain: %v", err)
	}
	if err := db.Create(&models.Token{Symbol: "TGR", Name: "Tiger", Address: "0xtgr", ChainID: 1}).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}
	seedSwap(t, db, "TGR/RUSD", 1, 1000, "buy", now.Add(-time.Hour))
	seedSwap(t, db, "TGR/RUSD", 1, 2000, "buy", now.Add(-48*time.Hour))

	overview, err := svc.Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if overview.TotalPools != 2 || overview.ActivePools != 1 {
		t.Errorf("pools total/active = %d/%d, want 2/1", overview.TotalPools, overview.ActivePools)
	}
	if overview.TotalFarms != 1 || overview.ActiveFarms != 1 {
		t.Errorf("farms total/active = %d/%d, want 1/1", overview.TotalFarms, overview.ActiveFarms)
	}
	if overview.TotalChains != 1 || overview.ActiveChains != 1 {
		t.Errorf("chains total/active = %d/%d, want 1/1", overview.TotalChains, overview.ActiveChains)
	}
	if overview.TotalTVLUSD != 1500 {
		t.Errorf("tvl = %v, want 1500", overview.TotalTVLUSD)
	}
	// Only the last-hour swap is inside the 24h window.
	if overview.Volume24hUSD != 1000 {
		t.Errorf("volume24h = %v, want 1000", overview.Volume24hUSD)
	}
	if overview.TotalTrades24h != 1 {
		t.Errorf("trades24h = %d, want 1", overview.TotalTrades24h)
	}
	// Both swaps are inside the 7d window.
	if overview.Volume7dUSD != 3000 {
		t.Errorf("volume7d = %v, want 3000", overview.Volume7dUSD)
	}
	// Fees at 30 bps: 1000 * 0.003 = 3.
	if diff := overview.Fees24hUSD - 3; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("fees24h = %v, want 3", overview.Fees24hUSD)
	}
}

func TestOverviewValuesOpenInterestAndBorrows(t *testing.T) {
	svc, db := newAnalyticsTestEnv(t, map[string]float64{"RUSD": 1.0, "USDT": 1.0})

	if err := db.Create(&models.PerpetualPosition{
		UserID: "u1", Pair: "BTC/RUSD", Side: "long", Size: "5000", Status: "open",
	}).Error; err != nil {
		t.Fatalf("seed position: %v", err)
	}
	// A closed position is not open interest.
	if err := db.Create(&models.PerpetualPosition{
		UserID: "u1", Pair: "BTC/RUSD", Side: "long", Size: "9999", Status: "closed",
	}).Error; err != nil {
		t.Fatalf("seed closed position: %v", err)
	}
	if err := db.Create(&models.MarginBorrow{
		Asset: "RUSD", Principal: "1000", AccruedInterest: "50", LastAccrual: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("seed borrow: %v", err)
	}

	overview, err := svc.Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if overview.OpenInterestUSD != 5000 {
		t.Errorf("open interest = %v, want 5000", overview.OpenInterestUSD)
	}
	// Principal plus accrued interest, valued at the RUSD price of 1.
	if overview.MarginBorrowedUSD != 1050 {
		t.Errorf("margin borrowed = %v, want 1050", overview.MarginBorrowedUSD)
	}
}

func TestTopTradersRanksByRealizedPnL(t *testing.T) {
	svc, db := newAnalyticsTestEnv(t, nil)
	closed := time.Now().UTC().Add(-time.Hour)

	seed := func(userID, pnl, size, status string) {
		t.Helper()
		pos := models.PerpetualPosition{
			UserID: userID, Pair: "BTC/RUSD", Side: "long", Size: size,
			Status: status, PNL: pnl, ClosedAt: &closed,
		}
		if err := db.Create(&pos).Error; err != nil {
			t.Fatalf("seed position: %v", err)
		}
	}
	seed("alice", "500", "10000", "closed")
	seed("alice", "-100", "5000", "closed")
	seed("bob", "100", "1000", "closed")
	seed("carol", "-50", "2000", "liquidated")

	traders, err := svc.TopTraders(context.Background(), time.Time{}, 10)
	if err != nil {
		t.Fatalf("TopTraders: %v", err)
	}
	if len(traders) != 3 {
		t.Fatalf("expected 3 traders, got %d", len(traders))
	}
	if traders[0].UserID != "alice" || traders[0].RealizedPnL != 400 {
		t.Errorf("top trader = %+v, want alice at 400", traders[0])
	}
	// Alice won 1 of 2 closed trades.
	if diff := traders[0].WinRate - 0.5; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("alice win rate = %v, want 0.5", traders[0].WinRate)
	}
	if traders[0].VolumeUSD != 15000 {
		t.Errorf("alice volume = %v, want 15000", traders[0].VolumeUSD)
	}
	if traders[2].UserID != "carol" || traders[2].Liquidations != 1 {
		t.Errorf("last trader = %+v, want carol with 1 liquidation", traders[2])
	}
}

func TestTopTradersWindowExcludesOldCloses(t *testing.T) {
	svc, db := newAnalyticsTestEnv(t, nil)
	old := time.Now().UTC().Add(-30 * 24 * time.Hour)
	pos := models.PerpetualPosition{
		UserID: "alice", Pair: "BTC/RUSD", Side: "long", Size: "1000",
		Status: "closed", PNL: "500", ClosedAt: &old,
	}
	if err := db.Create(&pos).Error; err != nil {
		t.Fatalf("seed position: %v", err)
	}

	traders, err := svc.TopTraders(context.Background(), time.Now().UTC().Add(-24*time.Hour), 10)
	if err != nil {
		t.Fatalf("TopTraders: %v", err)
	}
	if len(traders) != 0 {
		t.Fatalf("expected no traders inside a 24h window, got %+v", traders)
	}
}

func TestOverviewToleratesAMissingPriceSource(t *testing.T) {
	// With no price source the borrow valuation is skipped, not fabricated.
	svc, db := newAnalyticsTestEnv(t, nil)
	if err := db.Create(&models.MarginBorrow{
		Asset: "RUSD", Principal: "1000", AccruedInterest: "0", LastAccrual: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("seed borrow: %v", err)
	}

	overview, err := svc.Overview(context.Background())
	if err != nil {
		t.Fatalf("Overview: %v", err)
	}
	if overview.MarginBorrowedUSD != 0 {
		t.Errorf("borrowed = %v, want 0 when no price is available", overview.MarginBorrowedUSD)
	}
}

// ------------------------------------------------------- wire format capture

// TestAnalyticsResponseWireFormat pins the exact JSON shape of every analytics
// response. The frontend validators in `frontend/src/lib/types.ts` are written
// against these shapes and checked against captured payloads in
// `frontend/src/lib/serverContract.test.ts`; the payloads there are copied from
// this test's log output. Run with `-v` to print each payload verbatim.
func TestAnalyticsResponseWireFormat(t *testing.T) {
	svc, db := newAnalyticsTestEnv(t, map[string]float64{"RUSD": 1.0})
	// Seed inside the handler's window (relative to now) so the captured
	// payloads carry real volume rather than an empty series.
	base := time.Now().UTC().Truncate(time.Minute).Add(-5 * time.Minute)

	seedSwap(t, db, "TGR/RUSD", 1, 1000, "buy", base.Add(5*time.Second))
	seedSwap(t, db, "ETH/RUSD", 1, 500, "sell", base.Add(6*time.Second))
	if err := db.Create(&models.PerpetualPosition{
		UserID: "u1", Pair: "BTC/RUSD", Side: "long", Size: "2500", Status: "open",
	}).Error; err != nil {
		t.Fatalf("seed position: %v", err)
	}
	if err := db.Create(&models.PerpetualPosition{
		UserID: "u1", Pair: "BTC/RUSD", Side: "long", Size: "2500",
		Status: "closed", PNL: "120", ClosedAt: &base,
	}).Error; err != nil {
		t.Fatalf("seed closed position: %v", err)
	}
	if _, err := svc.RollupVolume(context.Background(), base, base.Add(time.Minute)); err != nil {
		t.Fatalf("RollupVolume: %v", err)
	}

	// Overview.
	status, body := callHandler(t, svc.GetOverviewHandler, "1", http.MethodGet, "/analytics/overview", "", nil)
	if status != http.StatusOK {
		t.Fatalf("overview status = %d: %s", status, body)
	}
	assertKeys(t, body,
		"active_chains", "active_farms", "active_pools", "fees_24h_usd", "fees_7d_usd",
		"margin_borrowed_usd", "nft_listings_active", "open_interest_usd", "total_chains",
		"total_farms", "total_pools", "total_tokens", "total_trades_24h", "total_tvl_usd",
		"total_users", "updated_at", "volume_24h_usd", "volume_7d_usd",
	)
	t.Logf("OVERVIEW: %s", body)

	// Volume series.
	status, body = callHandler(t, svc.GetVolumeHandler, "1", http.MethodGet,
		"/analytics/volume?pair=TGR%2FRUSD&interval=60&window=3600", "", nil)
	if status != http.StatusOK {
		t.Fatalf("volume status = %d: %s", status, body)
	}
	assertKeys(t, body, "chain_id", "interval", "pair", "series")
	t.Logf("VOLUME: %s", body)

	// Pairs.
	status, body = callHandler(t, svc.GetPairsHandler, "1", http.MethodGet,
		"/analytics/pairs?window=3600&limit=10", "", nil)
	if status != http.StatusOK {
		t.Fatalf("pairs status = %d: %s", status, body)
	}
	assertKeys(t, body, "pairs", "window")
	t.Logf("PAIRS: %s", body)

	// Traders.
	status, body = callHandler(t, svc.GetTopTradersHandler, "1", http.MethodGet,
		"/analytics/traders?limit=10", "", nil)
	if status != http.StatusOK {
		t.Fatalf("traders status = %d: %s", status, body)
	}
	assertKeys(t, body, "traders")
	t.Logf("TRADERS: %s", body)

	// Rollup.
	status, body = callHandler(t, svc.RollupHandler, "1", http.MethodPost,
		"/admin/analytics/rollup?window=3600", "", nil)
	if status != http.StatusOK {
		t.Fatalf("rollup status = %d: %s", status, body)
	}
	assertKeys(t, body, "buckets_written", "from", "to")
	t.Logf("ROLLUP: %s", body)
}

// TestAnalyticsHandlersRejectMalformedQueries guards the query boundary: a
// malformed parameter is a 400, never a silent fallback to a default.
func TestAnalyticsHandlersRejectMalformedQueries(t *testing.T) {
	svc, _ := newAnalyticsTestEnv(t, nil)

	cases := []struct {
		name    string
		handler gin.HandlerFunc
		target  string
	}{
		{"volume non-multiple interval", svc.GetVolumeHandler, "/analytics/volume?interval=90"},
		{"volume negative window", svc.GetVolumeHandler, "/analytics/volume?window=-1"},
		{"volume huge window", svc.GetVolumeHandler, "/analytics/volume?window=999999999"},
		{"volume bad chain", svc.GetVolumeHandler, "/analytics/volume?chain_id=abc"},
		{"pairs bad window", svc.GetPairsHandler, "/analytics/pairs?window=0"},
		{"pairs bad chain", svc.GetPairsHandler, "/analytics/pairs?chain_id=x"},
		{"traders negative window", svc.GetTopTradersHandler, "/analytics/traders?window=-5"},
		{"rollup bad window", svc.RollupHandler, "/admin/analytics/rollup?window=0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := callHandler(t, tc.handler, "1", http.MethodGet, tc.target, "", nil)
			if status != http.StatusBadRequest {
				t.Fatalf("expected 400, got %d: %s", status, body)
			}
		})
	}
}
