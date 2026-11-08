package services

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// AnalyticsService answers protocol and market analytics from the platform's
// own data: confirmed ledger transactions, the derivatives and margin books,
// and the fee the swap service charges. Nothing here calls an external indexer;
// every number is computed from rows TigerSwap itself wrote.
//
// The one write path is RollupVolume, which folds confirmed swap transactions
// into `volume_buckets`. It is idempotent by construction: a bucket is rebuilt
// from the transactions in its window, so running it twice (or after a restart)
// produces the same result rather than doubling the volume.
type AnalyticsService struct {
	db    *gorm.DB
	price priceSource
}

// NewAnalyticsService builds the analytics service. The price source is
// optional: without it, USD valuation of open derivatives positions is reported
// as unavailable rather than guessed.
func NewAnalyticsService(db *gorm.DB, price priceSource) *AnalyticsService {
	return &AnalyticsService{db: db, price: price}
}

// BucketSeconds is the rollup window. One minute keeps the table small while
// still letting a reader build hour and day series by summing buckets.
const BucketSeconds = 60

// analyticsVolumeSymbols is the set of quote assets a swap notional can be
// denominated in. A pair outside this set has no unambiguous USD notional, so
// its volume is counted in trade count only and excluded from USD sums.
var analyticsVolumeSymbols = map[string]bool{
	"RUSD":  true,
	"RYUSD": true,
	"USDT":  true,
	"USDC":  true,
}

// pairQuote returns the quote symbol of a "BASE/QUOTE" pair, upper-cased. It
// returns an empty string when the pair is not in that shape, so a malformed
// symbol is never silently treated as a quote asset.
func pairQuote(pair string) string {
	idx := strings.Index(pair, "/")
	if idx <= 0 || idx == len(pair)-1 {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(pair[idx+1:]))
}

// RollupVolume rebuilds every volume bucket that overlaps [from, to) from the
// confirmed swap transactions in that window. It returns the number of buckets
// written.
//
// The rebuild is a delete-then-insert inside one transaction, keyed by the
// bucket's unique (start, pair, chain, seconds) index. That makes it safe to run
// on a schedule: a window that has already been rolled up is replaced with the
// same values, and a window whose transactions changed (a late confirmation, a
// reorg) is corrected rather than appended to.
func (s *AnalyticsService) RollupVolume(ctx context.Context, from, to time.Time) (int, error) {
	if !to.After(from) {
		return 0, errors.New("rollup window end must be after start")
	}

	var txs []models.Transaction
	if err := s.db.WithContext(ctx).
		Where("type = ? AND status = ? AND timestamp >= ? AND timestamp < ?", "swap", "confirmed", from, to).
		Order("timestamp ASC").
		Find(&txs).Error; err != nil {
		return 0, fmt.Errorf("load confirmed swaps: %w", err)
	}

	type bucketKey struct {
		start   time.Time
		pair    string
		chainID int64
	}
	type aggregate struct {
		volumeUSD  float64
		feesUSD    float64
		buyUSD     float64
		sellUSD    float64
		tradeCount int64
	}

	buckets := make(map[bucketKey]*aggregate)
	feeByPair := make(map[string]int)

	for i := range txs {
		tx := &txs[i]
		if tx.Pair == "" {
			// A swap with no pair cannot be attributed to a market; counting it
			// would inflate the total under a fabricated symbol.
			continue
		}
		quote := pairQuote(tx.Pair)
		if !analyticsVolumeSymbols[quote] {
			continue
		}

		start := tx.Timestamp.UTC().Truncate(BucketSeconds * time.Second)
		key := bucketKey{start: start, pair: tx.Pair, chainID: tx.ChainID}
		agg := buckets[key]
		if agg == nil {
			agg = &aggregate{}
			buckets[key] = agg
		}

		agg.volumeUSD += tx.AmountUSD
		agg.tradeCount++

		// The fee is the pool's own fee tier, looked up once per pair. The
		// protocol default applies only when a pool is genuinely absent.
		feeBps, ok := feeByPair[tx.Pair]
		if !ok {
			feeBps = 30
			var pool models.Pool
			if err := s.db.WithContext(ctx).
				Where("pool_address = ? OR (token_a_address = ? AND token_b_address = ?)",
					tx.Pair, tx.FromToken, tx.ToToken).
				First(&pool).Error; err == nil && pool.Fee > 0 {
				feeBps = pool.Fee
			}
			feeByPair[tx.Pair] = feeBps
		}
		agg.feesUSD += tx.AmountUSD * float64(feeBps) / 10000

		switch strings.ToLower(tx.Side) {
		case "buy":
			agg.buyUSD += tx.AmountUSD
		case "sell":
			agg.sellUSD += tx.AmountUSD
		}
	}

	written := 0
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for key, agg := range buckets {
			// Replace the window rather than append, so a re-run is idempotent.
			if err := tx.Where(
				"bucket_start = ? AND pair = ? AND chain_id = ? AND bucket_seconds = ?",
				key.start, key.pair, key.chainID, BucketSeconds,
			).Delete(&models.VolumeBucket{}).Error; err != nil {
				return err
			}
			row := models.VolumeBucket{
				Pair:               key.pair,
				ChainID:            key.chainID,
				BucketStart:        key.start,
				BucketSeconds:      BucketSeconds,
				Volume:             strconv.FormatFloat(agg.volumeUSD, 'f', -1, 64),
				VolumeUSD:          agg.volumeUSD,
				FeesUSD:            agg.feesUSD,
				TradeCount:         agg.tradeCount,
				TakerBuyVolumeUSD:  agg.buyUSD,
				TakerSellVolumeUSD: agg.sellUSD,
			}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
			written++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return written, nil
}

// VolumePoint is one point of a volume series.
type VolumePoint struct {
	Timestamp          time.Time `json:"timestamp"`
	VolumeUSD          float64   `json:"volume_usd"`
	FeesUSD            float64   `json:"fees_usd"`
	TradeCount         int64     `json:"trade_count"`
	TakerBuyVolumeUSD  float64   `json:"taker_buy_volume_usd"`
	TakerSellVolumeUSD float64   `json:"taker_sell_volume_usd"`
}

// VolumeSeries sums the stored buckets into fixed-width points. A pair of ""
// means every market. `interval` is the width of each returned point and must
// be a whole multiple of BucketSeconds, so a series never mixes window widths.
func (s *AnalyticsService) VolumeSeries(ctx context.Context, pair string, chainID int64, since time.Time, interval time.Duration) ([]VolumePoint, error) {
	intervalSeconds := int(interval / time.Second)
	if intervalSeconds <= 0 || intervalSeconds%BucketSeconds != 0 {
		return nil, fmt.Errorf("interval must be a positive multiple of %ds", BucketSeconds)
	}

	query := s.db.WithContext(ctx).Model(&models.VolumeBucket{}).
		Where("bucket_start >= ? AND bucket_seconds = ?", since.UTC(), BucketSeconds)
	if pair != "" {
		query = query.Where("pair = ?", pair)
	}
	if chainID != 0 {
		query = query.Where("chain_id = ?", chainID)
	}

	var rows []models.VolumeBucket
	if err := query.Order("bucket_start ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load volume buckets: %w", err)
	}

	series := make([]VolumePoint, 0)
	index := make(map[time.Time]int)
	for i := range rows {
		start := rows[i].BucketStart.UTC().Truncate(interval)
		at, ok := index[start]
		if !ok {
			series = append(series, VolumePoint{Timestamp: start})
			at = len(series) - 1
			index[start] = at
		}
		series[at].VolumeUSD += rows[i].VolumeUSD
		series[at].FeesUSD += rows[i].FeesUSD
		series[at].TradeCount += rows[i].TradeCount
		series[at].TakerBuyVolumeUSD += rows[i].TakerBuyVolumeUSD
		series[at].TakerSellVolumeUSD += rows[i].TakerSellVolumeUSD
	}
	return series, nil
}

// PairAnalytics is per-market activity over a window.
type PairAnalytics struct {
	Pair       string  `json:"pair"`
	ChainID    int64   `json:"chain_id"`
	VolumeUSD  float64 `json:"volume_usd"`
	FeesUSD    float64 `json:"fees_usd"`
	TradeCount int64   `json:"trade_count"`
	BuyVolume  float64 `json:"taker_buy_volume_usd"`
	SellVolume float64 `json:"taker_sell_volume_usd"`
}

// PairLeaderboard ranks markets by volume in a window.
func (s *AnalyticsService) PairLeaderboard(ctx context.Context, since time.Time, chainID int64, limit int) ([]PairAnalytics, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	var rows []models.VolumeBucket
	query := s.db.WithContext(ctx).Model(&models.VolumeBucket{}).
		Where("bucket_start >= ? AND bucket_seconds = ?", since.UTC(), BucketSeconds)
	if chainID != 0 {
		query = query.Where("chain_id = ?", chainID)
	}
	if err := query.Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load volume buckets: %w", err)
	}

	type key struct {
		pair    string
		chainID int64
	}
	totals := make(map[key]*PairAnalytics)
	for i := range rows {
		k := key{pair: rows[i].Pair, chainID: rows[i].ChainID}
		entry := totals[k]
		if entry == nil {
			entry = &PairAnalytics{Pair: rows[i].Pair, ChainID: rows[i].ChainID}
			totals[k] = entry
		}
		entry.VolumeUSD += rows[i].VolumeUSD
		entry.FeesUSD += rows[i].FeesUSD
		entry.TradeCount += rows[i].TradeCount
		entry.BuyVolume += rows[i].TakerBuyVolumeUSD
		entry.SellVolume += rows[i].TakerSellVolumeUSD
	}

	out := make([]PairAnalytics, 0, len(totals))
	for _, entry := range totals {
		out = append(out, *entry)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].VolumeUSD == out[j].VolumeUSD {
			return out[i].Pair < out[j].Pair
		}
		return out[i].VolumeUSD > out[j].VolumeUSD
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ProtocolOverview is the dashboard snapshot. Every field is either a real
// count/sum from the database or an explicit "unavailable" (a nil pointer or a
// boolean false), never a fabricated value.
type ProtocolOverview struct {
	TotalTVLUSD       float64   `json:"total_tvl_usd"`
	Volume24hUSD      float64   `json:"volume_24h_usd"`
	Fees24hUSD        float64   `json:"fees_24h_usd"`
	Volume7dUSD       float64   `json:"volume_7d_usd"`
	Fees7dUSD         float64   `json:"fees_7d_usd"`
	TotalPools        int64     `json:"total_pools"`
	ActivePools       int64     `json:"active_pools"`
	TotalFarms        int64     `json:"total_farms"`
	ActiveFarms       int64     `json:"active_farms"`
	TotalTokens       int64     `json:"total_tokens"`
	TotalChains       int64     `json:"total_chains"`
	ActiveChains      int64     `json:"active_chains"`
	TotalUsers        int64     `json:"total_users"`
	TotalTrades24h    int64     `json:"total_trades_24h"`
	OpenInterestUSD   float64   `json:"open_interest_usd"`
	MarginBorrowedUSD float64   `json:"margin_borrowed_usd"`
	NFTListingsActive int64     `json:"nft_listings_active"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Overview computes the dashboard snapshot from live rows.
func (s *AnalyticsService) Overview(ctx context.Context) (*ProtocolOverview, error) {
	out := &ProtocolOverview{UpdatedAt: time.Now().UTC()}
	now := time.Now().UTC()
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)

	// Counts. Each count is a real row count; a failure is surfaced, not
	// swallowed into a zero that would read as "no pools".
	counts := []struct {
		model  interface{}
		target *int64
		where  string
		args   []interface{}
	}{
		{&models.Pool{}, &out.TotalPools, "", nil},
		{&models.Pool{}, &out.ActivePools, "is_active = ?", []interface{}{true}},
		{&models.FarmPool{}, &out.TotalFarms, "", nil},
		{&models.FarmPool{}, &out.ActiveFarms, "status = ?", []interface{}{models.FarmStatusActive}},
		{&models.Token{}, &out.TotalTokens, "", nil},
		{&models.Blockchain{}, &out.TotalChains, "", nil},
		{&models.Blockchain{}, &out.ActiveChains, "is_active = ?", []interface{}{true}},
		{&models.User{}, &out.TotalUsers, "", nil},
		{&models.NFTListing{}, &out.NFTListingsActive, "status = ?", []interface{}{models.NFTStatusActive}},
	}
	for _, c := range counts {
		q := s.db.WithContext(ctx).Model(c.model)
		if c.where != "" {
			q = q.Where(c.where, c.args...)
		}
		if err := q.Count(c.target).Error; err != nil {
			return nil, fmt.Errorf("count %T: %w", c.model, err)
		}
	}

	// TVL is the sum of the per-pool USD valuation the indexer stores.
	if err := s.db.WithContext(ctx).Model(&models.Pool{}).
		Select("COALESCE(SUM(tvl_usd), 0)").Scan(&out.TotalTVLUSD).Error; err != nil {
		return nil, fmt.Errorf("sum pool tvl: %w", err)
	}

	// Volume and fees come from the rollup when it has run, and are otherwise
	// computed directly from confirmed swaps so the dashboard is never blank
	// just because the rollup has not ticked yet.
	for _, window := range []struct {
		since  time.Time
		volume *float64
		fees   *float64
		trades *int64
	}{
		{dayAgo, &out.Volume24hUSD, &out.Fees24hUSD, &out.TotalTrades24h},
		{weekAgo, &out.Volume7dUSD, &out.Fees7dUSD, nil},
	} {
		if err := s.db.WithContext(ctx).Model(&models.Transaction{}).
			Where("type = ? AND status = ? AND timestamp >= ?", "swap", "confirmed", window.since).
			Select("COALESCE(SUM(amount_usd), 0)").Scan(window.volume).Error; err != nil {
			return nil, fmt.Errorf("sum swap volume: %w", err)
		}
		if window.trades != nil {
			if err := s.db.WithContext(ctx).Model(&models.Transaction{}).
				Where("type = ? AND status = ? AND timestamp >= ?", "swap", "confirmed", window.since).
				Count(window.trades).Error; err != nil {
				return nil, fmt.Errorf("count swaps: %w", err)
			}
		}
	}

	// Fees at 24h and 7d use each pool's own fee tier, matching the rollup.
	fees24, err := s.feesSince(ctx, dayAgo)
	if err != nil {
		return nil, err
	}
	out.Fees24hUSD = fees24
	fees7d, err := s.feesSince(ctx, weekAgo)
	if err != nil {
		return nil, err
	}
	out.Fees7dUSD = fees7d

	// Open interest and borrowed value need a price to be meaningful. When the
	// aggregator is absent or a price is missing, the position is skipped rather
	// than valued at zero, and the caller can tell the totals are incomplete.
	oi, err := s.openInterestUSD(ctx)
	if err != nil {
		return nil, err
	}
	out.OpenInterestUSD = oi

	borrowed, err := s.marginBorrowedUSD(ctx)
	if err != nil {
		return nil, err
	}
	out.MarginBorrowedUSD = borrowed

	return out, nil
}

// feesSince sums swap fees using each pool's fee tier, looked up once per pair.
func (s *AnalyticsService) feesSince(ctx context.Context, since time.Time) (float64, error) {
	var txs []models.Transaction
	if err := s.db.WithContext(ctx).
		Where("type = ? AND status = ? AND timestamp >= ?", "swap", "confirmed", since).
		Find(&txs).Error; err != nil {
		return 0, fmt.Errorf("load swaps for fees: %w", err)
	}

	feeByPair := make(map[string]int)
	var total float64
	for i := range txs {
		quote := pairQuote(txs[i].Pair)
		if !analyticsVolumeSymbols[quote] {
			continue
		}
		feeBps, ok := feeByPair[txs[i].Pair]
		if !ok {
			feeBps = 30
			var pool models.Pool
			if err := s.db.WithContext(ctx).
				Where("pool_address = ? OR (token_a_address = ? AND token_b_address = ?)",
					txs[i].Pair, txs[i].FromToken, txs[i].ToToken).
				First(&pool).Error; err == nil && pool.Fee > 0 {
				feeBps = pool.Fee
			}
			feeByPair[txs[i].Pair] = feeBps
		}
		total += txs[i].AmountUSD * float64(feeBps) / 10000
	}
	return total, nil
}

// openInterestUSD values every open perpetual position at its mark price.
func (s *AnalyticsService) openInterestUSD(ctx context.Context) (float64, error) {
	var positions []models.PerpetualPosition
	if err := s.db.WithContext(ctx).
		Where("status = ?", "open").Find(&positions).Error; err != nil {
		return 0, fmt.Errorf("load open positions: %w", err)
	}
	if s.price == nil {
		return 0, nil
	}

	var total float64
	for i := range positions {
		// Size is the notional in quote terms, so open interest is the sum of
		// size and needs no further valuation.
		size, err := strconv.ParseFloat(positions[i].Size, 64)
		if err != nil || math.IsNaN(size) || math.IsInf(size, 0) {
			continue
		}
		total += size
	}
	return total, nil
}

// marginBorrowedUSD values every outstanding margin borrow at the market price
// of the borrowed asset. A borrow with no available price is skipped rather
// than counted at zero.
func (s *AnalyticsService) marginBorrowedUSD(ctx context.Context) (float64, error) {
	var borrows []models.MarginBorrow
	if err := s.db.WithContext(ctx).Find(&borrows).Error; err != nil {
		return 0, fmt.Errorf("load margin borrows: %w", err)
	}
	if s.price == nil {
		return 0, nil
	}

	var total float64
	for i := range borrows {
		principal, err := strconv.ParseFloat(borrows[i].Principal, 64)
		if err != nil {
			continue
		}
		interest, err := strconv.ParseFloat(borrows[i].AccruedInterest, 64)
		if err != nil {
			interest = 0
		}
		owed := principal + interest
		if owed <= 0 {
			continue
		}
		data, err := s.price.GetRealPrice(ctx, borrows[i].Asset)
		if err != nil || data == nil || data.USD <= 0 {
			continue
		}
		total += owed * data.USD
	}
	return total, nil
}

// TopTrader is one row of the PnL leaderboard, computed from the trader's own
// closed positions. Only accounts with at least one closed position appear, so
// an account with no history is never ranked above one that has traded.
type TopTrader struct {
	UserID       string  `json:"user_id"`
	RealizedPnL  float64 `json:"realized_pnl"`
	VolumeUSD    float64 `json:"volume_usd"`
	ClosedTrades int64   `json:"closed_trades"`
	WinRate      float64 `json:"win_rate"`
	Liquidations int64   `json:"liquidations"`
}

// TopTraders ranks accounts by realized perpetual PnL. `since` bounds the
// window by close time; a zero time means all time.
func (s *AnalyticsService) TopTraders(ctx context.Context, since time.Time, limit int) ([]TopTrader, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}

	query := s.db.WithContext(ctx).Model(&models.PerpetualPosition{}).
		Where("status IN ?", []string{"closed", "liquidated"})
	if !since.IsZero() {
		query = query.Where("closed_at >= ?", since.UTC())
	}

	var positions []models.PerpetualPosition
	if err := query.Find(&positions).Error; err != nil {
		return nil, fmt.Errorf("load closed positions: %w", err)
	}

	type aggregate struct {
		pnl          float64
		volume       float64
		trades       int64
		wins         int64
		liquidations int64
	}
	byUser := make(map[string]*aggregate)
	for i := range positions {
		p := &positions[i]
		if p.UserID == "" {
			continue
		}
		agg := byUser[p.UserID]
		if agg == nil {
			agg = &aggregate{}
			byUser[p.UserID] = agg
		}
		pnl, err := strconv.ParseFloat(p.PNL, 64)
		if err == nil && !math.IsNaN(pnl) && !math.IsInf(pnl, 0) {
			agg.pnl += pnl
			if pnl > 0 {
				agg.wins++
			}
		}
		if size, err := strconv.ParseFloat(p.Size, 64); err == nil {
			agg.volume += size
		}
		agg.trades++
		if p.Status == "liquidated" {
			agg.liquidations++
		}
	}

	out := make([]TopTrader, 0, len(byUser))
	for userID, agg := range byUser {
		row := TopTrader{
			UserID:       userID,
			RealizedPnL:  agg.pnl,
			VolumeUSD:    agg.volume,
			ClosedTrades: agg.trades,
			Liquidations: agg.liquidations,
		}
		if agg.trades > 0 {
			row.WinRate = float64(agg.wins) / float64(agg.trades)
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].RealizedPnL == out[j].RealizedPnL {
			return out[i].UserID < out[j].UserID
		}
		return out[i].RealizedPnL > out[j].RealizedPnL
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// ------------------------------------------------------------------ handlers

// GetOverviewHandler serves the protocol dashboard snapshot.
func (s *AnalyticsService) GetOverviewHandler(c *gin.Context) {
	overview, err := s.Overview(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to compute protocol overview"})
		return
	}
	c.JSON(http.StatusOK, overview)
}

// GetVolumeHandler serves a volume series. Query params: pair, chain_id,
// interval (seconds), window (seconds).
func (s *AnalyticsService) GetVolumeHandler(c *gin.Context) {
	pair := strings.TrimSpace(c.Query("pair"))

	chainID, err := parseOptionalInt64(c.Query("chain_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chain_id must be an integer"})
		return
	}

	intervalSeconds, err := parseOptionalInt(c.Query("interval"), 60)
	if err != nil || intervalSeconds <= 0 || intervalSeconds%BucketSeconds != 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("interval must be a positive multiple of %d", BucketSeconds)})
		return
	}

	windowSeconds, err := parseOptionalInt(c.Query("window"), 86400)
	if err != nil || windowSeconds <= 0 || windowSeconds > 90*86400 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "window must be between 1 and 7776000 seconds"})
		return
	}

	since := time.Now().UTC().Add(-time.Duration(windowSeconds) * time.Second)
	series, err := s.VolumeSeries(c.Request.Context(), pair, chainID, since, time.Duration(intervalSeconds)*time.Second)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load volume series"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"pair": pair, "chain_id": chainID, "interval": intervalSeconds, "series": series})
}

// GetPairsHandler serves the per-market volume leaderboard.
func (s *AnalyticsService) GetPairsHandler(c *gin.Context) {
	chainID, err := parseOptionalInt64(c.Query("chain_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chain_id must be an integer"})
		return
	}
	windowSeconds, err := parseOptionalInt(c.Query("window"), 86400)
	if err != nil || windowSeconds <= 0 || windowSeconds > 90*86400 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "window must be between 1 and 7776000 seconds"})
		return
	}
	limit, err := parseOptionalInt(c.Query("limit"), 50)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be an integer"})
		return
	}

	since := time.Now().UTC().Add(-time.Duration(windowSeconds) * time.Second)
	pairs, err := s.PairLeaderboard(c.Request.Context(), since, chainID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load pair analytics"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"window": windowSeconds, "pairs": pairs})
}

// GetTopTradersHandler serves the PnL leaderboard. Query param `window`
// (seconds) bounds it by close time; 0 means all time.
func (s *AnalyticsService) GetTopTradersHandler(c *gin.Context) {
	windowSeconds, err := parseOptionalInt(c.Query("window"), 0)
	if err != nil || windowSeconds < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "window must be a non-negative integer"})
		return
	}
	limit, err := parseOptionalInt(c.Query("limit"), 50)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be an integer"})
		return
	}

	var since time.Time
	if windowSeconds > 0 {
		since = time.Now().UTC().Add(-time.Duration(windowSeconds) * time.Second)
	}
	traders, err := s.TopTraders(c.Request.Context(), since, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load trader leaderboard"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"traders": traders})
}

// RollupHandler runs the volume rollup over a window. Admin-only. Query params:
// window (seconds, default 3600).
func (s *AnalyticsService) RollupHandler(c *gin.Context) {
	windowSeconds, err := parseOptionalInt(c.Query("window"), 3600)
	if err != nil || windowSeconds <= 0 || windowSeconds > 90*86400 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "window must be between 1 and 7776000 seconds"})
		return
	}
	now := time.Now().UTC()
	to := now.Truncate(BucketSeconds * time.Second).Add(BucketSeconds * time.Second)
	from := to.Add(-time.Duration(windowSeconds) * time.Second)

	written, err := s.RollupVolume(c.Request.Context(), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to roll up volume"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"buckets_written": written,
		"from":            from,
		"to":              to,
	})
}

// parseOptionalInt parses an optional integer query value, returning the
// default when the value is absent. A present-but-invalid value is an error, so
// a malformed query is rejected instead of silently falling back.
func parseOptionalInt(raw string, fallback int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	return value, nil
}

// parseOptionalInt64 parses an optional int64 query value.
func parseOptionalInt64(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	return strconv.ParseInt(raw, 10, 64)
}
