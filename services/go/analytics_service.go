// ============================================================================
// TIGERSWAP ANALYTICS SERVICE
// TVL, Volume, Revenue calculations, Leaderboard, Protocol statistics
// ============================================================================

package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// MODELS
// ============================================================================

// ProtocolAnalytics represents overall protocol analytics
type ProtocolAnalytics struct {
	Timestamp          int64   `json:"timestamp"`
	TotalValueLocked  float64 `json:"total_value_locked"`
	Volume24h         float64 `json:"volume_24h"`
	Volume7d          float64 `json:"volume_7d"`
	Volume30d         float64 `json:"volume_30d"`
	Revenue24h        float64 `json:"revenue_24h"`
	Revenue7d         float64 `json:"revenue_7d"`
	Revenue30d        float64 `json:"revenue_30d"`
	FeesByType        map[string]float64 `json:"fees_by_type"`
	UniqueTraders24h  int     `json:"unique_traders_24h"`
	UniqueTraders7d   int     `json:"unique_traders_7d"`
	ActivePools      int     `json:"active_pools"`
	ActiveUsers     int     `json:"active_users"`
}

// MarketAnalytics represents market-specific analytics
type MarketAnalytics struct {
	Market            string  `json:"market"`
	Volume24h        float64 `json:"volume_24h"`
	Volume7d         float64 `json:"volume_7d"`
	Fees24h           float64 `json:"fees_24h"`
	PriceChange24h    float64 `json:"price_change_24h"`
	PriceChange7d     float64 `json:"price_change_7d"`
	High24h           float64 `json:"high_24h"`
	Low24h            float64 `json:"low_24h"`
	TVL               float64 `json:"tvl"`
	Liquidity         float64 `json:"liquidity"`
	TradeCount24h    int     `json:"trade_count_24h"`
	UniqueTraders    int     `json:"unique_traders"`
}

// UserAnalytics represents user-specific analytics
type UserAnalytics struct {
	UserID            string  `json:"user_id"`
	TotalVolume      float64 `json:"total_volume"`
	TotalFees        float64 `json:"total_fees"`
	Volume24h       float64 `json:"volume_24h"`
	Fees24h          float64 `json:"fees_24h"`
	TradeCount       int     `json:"trade_count"`
	OpenPositions    int     `json:"open_positions"`
	RealizedPnL     float64 `json:"realized_pnl"`
	UnrealizedPnL    float64 `json:"unrealized_pnl"`
	LastActive      int64   `json:"last_active"`
}

// PoolAnalytics represents pool analytics
type PoolAnalytics struct {
	PoolAddress      string  `json:"pool_address"`
	Token0           string  `json:"token0"`
	Token1           string  `json:"token1"`
	TVL              float64 `json:"tvl"`
	Volume24h        float64 `json:"volume_24h"`
	Fees24h          float64 `json:"fees_24h"`
	APR              float64 `json:"apr"`
	Utilization     float64 `json:"utilization"`
}

// LeaderboardEntry represents leaderboard entry
type LeaderboardEntry struct {
	Rank          int     `json:"rank"`
	UserID        string  `json:"user_id"`
	Volume       float64 `json:"volume"`
	Fees          float64 `json:"fees"`
	TradeCount   int     `json:"trade_count"`
	PnL           float64 `json:"pnl"`
}

// ============================================================================
// ANALYTICS STORE
// ============================================================================

type AnalyticsStore struct {
	mu sync.RWMutex

	// Historical data
	dailyMetrics    map[string]*DailyMetrics // date -> metrics
	marketMetrics map[string]*MarketMetrics // market -> metrics
	userMetrics  map[string]*UserMetrics // user -> metrics

	// Aggregated
	totalVolume   float64
	totalFees    float64
	totalTVL    float64
}

// Daily metrics
type DailyMetrics struct {
	Date            string
	Volume          float64
	Fees            float64
	TradeCount      int
	UniqueTraders  int
}

// Market metrics
type MarketMetrics struct {
	Market        string
	Volume24h    float64
	Volume7d     float64
	Fees24h      float64
	PriceHigh24h float64
	PriceLow24h  float64
	TVL          float64
}

// User metrics
type UserMetrics struct {
	UserID       string
	Volume       float64
	Fees        float64
	TradeCount  int
	LastActive  int64
}

func NewAnalyticsStore() *AnalyticsStore {
	return &AnalyticsStore{
		dailyMetrics:    make(map[string]*DailyMetrics),
		marketMetrics: make(map[string]*MarketMetrics),
		userMetrics:  make(map[string]*UserMetrics),
	}
}

// ============================================================================
// ANALYTICS OPERATIONS
// ============================================================================

// RecordTrade records a trade for analytics
func (s *AnalyticsStore) RecordTrade(userID, market, token0, token1 string, volume, fee float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	date := now.Format("2006-01-02")

	// Daily metrics
	dm, ok := s.dailyMetrics[date]
	if !ok {
		dm = &DailyMetrics{Date: date}
		s.dailyMetrics[date] = dm
	}
	dm.Volume += volume
	dm.Fees += fee
	dm.TradeCount++

	// Market metrics
	mm, ok := s.marketMetrics[market]
	if !ok {
		mm = &MarketMetrics{Market: market}
		s.marketMetrics[market] = mm
	}
	mm.Volume24h += volume
	mm.Fees24h += fee

	// User metrics
	um, ok := s.userMetrics[userID]
	if !ok {
		um = &UserMetrics{UserID: userID}
		s.userMetrics[userID] = um
	}
	um.Volume += volume
	um.Fees += fee
	um.TradeCount++
	um.LastActive = now.Unix()

	// Totals
	s.totalVolume += volume
	s.totalFees += fee
}

// RecordTVL records TVL update
func (s *AnalyticsStore) RecordTVL(tvl float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.totalTVL = tvl
}

// GetProtocolAnalytics gets protocol-level analytics
func (s *AnalyticsStore) GetProtocolAnalytics() *ProtocolAnalytics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	now := time.Now()
	date := now.Format("2006-01-02")

	// Calculate period metrics
	volume7d := s.totalVolume // Simplified
	volume30d := s.totalVolume
	revenue7d := s.totalFees
	revenue30d := s.totalFees

	// Get daily data for last 7 days
	var volume7dCalc, revenue7dCalc float64
	for i := 0; i < 7; i++ {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		if dm, ok := s.dailyMetrics[d]; ok {
			volume7dCalc += dm.Volume
			revenue7dCalc += dm.Fees
		}
	}

	return &ProtocolAnalytics{
		Timestamp:         now.Unix(),
		TotalValueLocked:  s.totalTVL,
		Volume24h:         s.totalVolume / 30, // Simplified
		Volume7d:          volume7dCalc,
		Volume30d:         volume30d,
		Revenue24h:        s.totalFees / 30,
		Revenue7d:         revenue7dCalc,
		Revenue30d:        revenue30d,
		UniqueTraders24h:  len(s.userMetrics),
		ActivePools:       len(s.marketMetrics),
		ActiveUsers:      len(s.userMetrics),
	}
}

// GetMarketAnalytics gets market-specific analytics
func (s *AnalyticsStore) GetMarketAnalytics(market string) *MarketAnalytics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mm, ok := s.marketMetrics[market]
	if !ok {
		return &MarketAnalytics{Market: market}
	}

	return &MarketAnalytics{
		Market:       market,
		Volume24h:    mm.Volume24h,
		Volume7d:     mm.Volume7d,
		Fees24h:      mm.Fees24h,
		TVL:          mm.TVL,
		TradeCount24h: mm.Volume24h > 0, // Simplified
	}
}

// GetUserAnalytics gets user-specific analytics
func (s *AnalyticsStore) GetUserAnalytics(userID string) *UserAnalytics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	um, ok := s.userMetrics[userID]
	if !ok {
		return &UserAnalytics{UserID: userID}
	}

	return &UserAnalytics{
		UserID:       um.UserID,
		TotalVolume:  um.Volume,
		TotalFees:    um.Fees,
		TradeCount:  um.TradeCount,
		LastActive:  um.LastActive,
	}
}

// GetLeaderboard gets top traders
func (s *AnalyticsStore) GetLeaderboard(limit int) []*LeaderboardEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Convert to slice and sort
	entries := make([]*LeaderboardEntry, 0, len(s.userMetrics))
	for _, um := range s.userMetrics {
		entries = append(entries, &LeaderboardEntry{
			UserID:      um.UserID,
			Volume:     um.Volume,
			Fees:        um.Fees,
			TradeCount: um.TradeCount,
		})
	}

	// Sort by volume
	for i := 0; i < len(entries); i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].Volume > entries[i].Volume {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	// Limit results
	if limit > 0 && len(entries) > limit {
		entries = entries[:limit]
	}

	// Add ranks
	for i, e := range entries {
		e.Rank = i + 1
	}

	return entries
}

// ============================================================================
// HTTP HANDLERS
// ============================================================================

type AnalyticsHandler struct {
	store *AnalyticsStore
}

func NewAnalyticsHandler(store *AnalyticsStore) *AnalyticsHandler {
	return &AnalyticsHandler{store: store}
}

func (h *AnalyticsHandler) HandleGetProtocolAnalytics(w http.ResponseWriter, r *http.Request) {
	analytics := h.store.GetProtocolAnalytics()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analytics)
}

func (h *AnalyticsHandler) HandleGetMarketAnalytics(w http.ResponseWriter, r *http.Request) {
	market := r.URL.Query().Get("market")
	if market == "" {
		http.Error(w, "market required", http.StatusBadRequest)
		return
	}

	analytics := h.store.GetMarketAnalytics(market)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analytics)
}

func (h *AnalyticsHandler) HandleGetLeaderboard(w http.ResponseWriter, r *http.Request) {
	limit := 100
	fmt.Sscanf(r.URL.Query().Get("limit"), "%d", &limit)

	leaderboard := h.store.GetLeaderboard(limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(leaderboard)
}

// ============================================================================
// GLOBAL
// ============================================================================

var analyticsStore *AnalyticsStore

func InitAnalytics() {
	analyticsStore = NewAnalyticsStore()
}

func GetAnalyticsStore() *AnalyticsStore {
	return analyticsStore
}