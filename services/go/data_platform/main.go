package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// ============================================================================
// Data Platform - Advanced Analytics Infrastructure
// ============================================================================

// ============================================================================
// Data Types
// ============================================================================

// TimeSeries data point
type TimeSeriesPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value    float64  `json:"value"`
}

// OHLCV candlestick
type Candlestick struct {
	Pair      string    `json:"pair"`
	Open     float64   `json:"open"`
	High     float64   `json:"high"`
	Low      float64   `json:"low"`
	Close    float64   `json:"close"`
	Volume   float64   `json:"volume"`
	Time     int64     `json:"time"`
}

// Token metrics
type TokenMetrics struct {
	Token           string    `json:"token"`
	Price          float64   `json:"price"`
	Volume24h      float64   `json:"volume_24h"`
	Change24h      float64   `json:"change_24h"`
	MarketCap      float64   `json:"market_cap"`
	FDV            float64   `json:"fdv"`
	Circulating    float64   `json:"circulating"`
	TotalSupply    float64   `json:"total_supply"`
	Holders        int64     `json:"holders"`
	LastUpdated    int64     `json:"last_updated"`
}

// Pool metrics
type PoolMetrics struct {
	PoolID       string    `json:"pool_id"`
	Pair         string    `json:"pair"`
	TVL          float64   `json:"tvl"`
	Volume24h   float64   `json:"volume_24h"`
	Fees24h      float64   `json:"fees_24h"`
	APY          float64   `json:"apy"`
	APR          float64   `json:"apr"`
	LastUpdated int64     `json:"last_updated"`
}

// User portfolio snapshot
type PortfolioSnapshot struct {
	UserID      string             `json:"user_id"`
	Timestamp   int64              `json:"timestamp"`
	TotalValue  float64            `json:"total_value"`
	Positions  []PositionSnapshot `json:"positions"`
}

// Position snapshot
type PositionSnapshot struct {
	Token      string  `json:"token"`
	Amount     float64 `json:"amount"`
	Value     float64 `json:"value"`
	CostBasis  float64 `json:"cost_basis"`
	PnL       float64 `json:"pnl"`
	PnLPercent float64 `json:"pnl_percent"`
}

// ============================================================================
// Data Warehouse (ClickHouse-like interface)
// ============================================================================

// DataWarehouse handles analytical queries
type DataWarehouse struct {
	// In production, this would connect to ClickHouse
	timeSeries map[string][]TimeSeriesPoint
	ohlcv     map[string][]Candlestick
	tokens    map[string]*TokenMetrics
	pools     map[string]*PoolMetrics
}

// NewDataWarehouse creates new data warehouse
func NewDataWarehouse() *DataWarehouse {
	return &DataWarehouse{
		timeSeries: make(map[string][]TimeSeriesPoint),
		ohlcv:     make(map[string][]Candlestick),
		tokens:    make(map[string]*TokenMetrics),
		pools:     make(map[string]*PoolMetrics),
	}
}

// ============================================================================
// Time Series Operations
// ============================================================================

// IngestTimeSeries ingests time series data
func (dw *DataWarehouse) IngestTimeSeries(metric string, points []TimeSeriesPoint) {
	dw.timeSeries[metric] = append(dw.timeSeries[metric], points...)
	
	// Keep last 1M points per metric
	if len(dw.timeSeries[metric]) > 1000000 {
		dw.timeSeries[metric] = dw.timeSeries[metric][len(dw.timeSeries[metric])-1000000:]
	}
}

// QueryTimeSeries queries time series data
func (dw *DataWarehouse) QueryTimeSeries(metric string, start, end time.Time) []TimeSeriesPoint {
	points, ok := dw.timeSeries[metric]
	if !ok {
		return nil
	}
	
	var result []TimeSeriesPoint
	for _, p := range points {
		if p.Timestamp.After(start) && p.Timestamp.Before(end) {
			result = append(result, p)
		}
	}
	
	return result
}

// AggregateTimeSeries aggregates time series data
func (dw *DataWarehouse) AggregateTimeSeries(metric string, interval time.Duration) map[time.Time]float64 {
	points, ok := dw.timeSeries[metric]
	if !ok {
		return nil
	}
	
	aggregated := make(map[time.Time]float64)
	
	for _, p := range points {
		rounded := p.Timestamp.Truncate(interval)
		aggregated[rounded] += p.Value
	}
	
	return aggregated
}

// ============================================================================
// OHLCV Operations
// ============================================================================

// IngestCandlestick ingests candlestick data
func (dw *DataWarehouse) IngestCandlestick(candle Candlestick) {
	key := fmt.Sprintf("%s_%d", candle.Pair, candle.Time/3600)
	dw.ohlcv[key] = append(dw.ohlcv[key], candle)
}

// QueryOHLCV queries candlestick data
func (dw *DataWarehouse) QueryOHLCV(pair string, start, end int64) []Candlestick {
	var result []Candlestick
	
	for key, candles := range dw.ohlcv {
		if len(key) > len(pair) && key[:len(pair)] == pair {
			for _, c := range candles {
				if c.Time >= start && c.Time <= end {
					result = append(result, c)
				}
			}
		}
	}
	
	return result
}

// ============================================================================
// Token Metrics Operations
// ============================================================================

// UpdateTokenMetrics updates token metrics
func (dw *DataWarehouse) UpdateTokenMetrics(token string, metrics *TokenMetrics) {
	dw.tokens[token] = metrics
}

// GetTokenMetrics gets token metrics
func (dw *DataWarehouse) GetTokenMetrics(token string) *TokenMetrics {
	return dw.tokens[token]
}

// GetTopTokens gets top tokens by market cap
func (dw *DataWarehouse) GetTopTokens(limit int) []*TokenMetrics {
	var tokens []*TokenMetrics
	for _, t := range dw.tokens {
		tokens = append(tokens, t)
	}
	
	// Sort by market cap
	for i := 0; i < len(tokens)-1; i++ {
		for j := i + 1; j < len(tokens); j++ {
			if tokens[j].MarketCap > tokens[i].MarketCap {
				tokens[i], tokens[j] = tokens[j], tokens[i]
			}
		}
	}
	
	if len(tokens) > limit {
		tokens = tokens[:limit]
	}
	
	return tokens
}

// ============================================================================
// Pool Metrics Operations
// ============================================================================

// UpdatePoolMetrics updates pool metrics
func (dw *DataWarehouse) UpdatePoolMetrics(poolID string, metrics *PoolMetrics) {
	dw.pools[poolID] = metrics
}

// GetPoolMetrics gets pool metrics
func (dw *DataWarehouse) GetPoolMetrics(poolID string) *PoolMetrics {
	return dw.pools[poolID]
}

// GetTopPools gets top pools by TVL
func (dw *DataWarehouse) GetTopPools(limit int) []*PoolMetrics {
	var pools []*PoolMetrics
	for _, p := range dw.pools {
		pools = append(pools, p)
	}
	
	// Sort by TVL
	for i := 0; i < len(pools)-1; i++ {
		for j := i + 1; j < len(pools); j++ {
			if pools[j].TVL > pools[i].TVL {
				pools[i], pools[j] = pools[j], pools[i]
			}
		}
	}
	
	if len(pools) > limit {
		pools = pools[:limit]
	}
	
	return pools
}

// ============================================================================
// ETL Pipeline
// ============================================================================

// ETL handles data extraction, transformation, loading
type ETL struct {
	warehouse *DataWarehouse
}

// NewETL creates new ETL pipeline
func NewETL(warehouse *DataWarehouse) *ETL {
	return &ETL{warehouse: warehouse}
}

// ExtractPoolData extracts pool data from source
func (e *ETL) ExtractPoolData(poolID string) map[string]interface{} {
	// Simulated extraction
	return map[string]interface{}{
		"pool_id":     poolID,
		"tvl":         1000000.0,
		"volume_24h":  500000.0,
		"fees_24h":    1000.0,
	}
}

// TransformPoolData transforms raw pool data
func (e *ETL) TransformPoolData(raw map[string]interface{}) *PoolMetrics {
	metrics := &PoolMetrics{
		PoolID:     raw["pool_id"].(string),
		TVL:        raw["tvl"].(float64),
		Volume24h: raw["volume_24h"].(float64),
		Fees24h:    raw["fees_24h"].(float64),
		LastUpdated: time.Now().Unix(),
	}
	
	// Calculate APY/APR
	if metrics.TVL > 0 {
		metrics.APY = (metrics.Fees24h * 365) / metrics.TVL * 100
		metrics.APR = metrics.APY
	}
	
	return metrics
}

// LoadPoolData loads transformed data to warehouse
func (e *ETL) LoadPoolData(metrics *PoolMetrics) {
	e.warehouse.UpdatePoolMetrics(metrics.PoolID, metrics)
}

// RunPipeline runs the ETL pipeline
func (e *ETL) RunPipeline(poolIDs []string) {
	for _, poolID := range poolIDs {
		raw := e.ExtractPoolData(poolID)
		transformed := e.TransformPoolData(raw)
		e.LoadPoolData(transformed)
	}
}

// ============================================================================
// Real-time Aggregator
// ============================================================================

// RealtimeAggregator handles real-time data aggregation
type RealtimeAggregator struct {
	warehouse *DataWarehouse
	buffers   map[string][]float64
	interval time.Duration
}

// NewRealtimeAggregator creates new real-time aggregator
func NewRealtimeAggregator(warehouse *DataWarehouse, interval time.Duration) *RealtimeAggregator {
	return &RealtimeAggregator{
		warehouse: warehouse,
		buffers:   make(map[string][]float64),
		interval:  interval,
	}
}

// Push pushes value to buffer
func (ra *RealtimeAggregator) Push(key string, value float64) {
	ra.buffers[key] = append(ra.buffers[key], value)
	
	// Flush if buffer too large
	if len(ra.buffers[key]) > 10000 {
		ra.Flush(key)
	}
}

// Flush flushes buffer to warehouse
func (ra *RealtimeAggregator) Flush(key string) {
	if len(ra.buffers[key]) == 0 {
		return
	}
	
	// Calculate aggregate
	var sum float64
	for _, v := range ra.buffers[key] {
		sum += v
	}
	avg := sum / float64(len(ra.buffers[key]))
	
	// Store in warehouse
	ra.warehouse.IngestTimeSeries(key, []TimeSeriesPoint{
		{Timestamp: time.Now(), Value: avg},
	})
	
	// Clear buffer
	ra.buffers[key] = nil
}

// ============================================================================
// Historical Archives
// ============================================================================

// Archive handles historical data archival
type Archive struct {
	warehouse *DataWarehouse
}

// NewArchive creates new archive
func (ra *Archive) Archive(warehouse *DataWarehouse) *Archive {
	return &Archive{warehouse: warehouse}
}

// Snapshot creates portfolio snapshot
func (ra *Archive) Snapshot(userID string, positions []PositionSnapshot) *PortfolioSnapshot {
	var totalValue float64
	for _, p := range positions {
		totalValue += p.Value
	}
	
	return &PortfolioSnapshot{
		UserID:     userID,
		Timestamp: time.Now().Unix(),
		TotalValue: totalValue,
		Positions: positions,
	}
}

// ============================================================================
// Main
// ============================================================================

func main() {
	fmt.Println("TigerSwap Data Platform")
	fmt.Println("====================");
	
	// Initialize data warehouse
	warehouse := NewDataWarehouse()
	fmt.Println("Data warehouse initialized")
	
	// Initialize ETL
	etl := NewETL(warehouse)
	
	// Run ETL pipeline
	fmt.Println("\nRunning ETL pipeline...")
	poolIDs := []string{"ETH-USDC", "BTC-USDC", "ETH-BTC"}
	etl.RunPipeline(poolIDs)
	
	// Get top pools
	fmt.Println("\nTop pools by TVL:")
	topPools := warehouse.GetTopPools(10)
	for i, p := range topPools {
		fmt.Printf("  %d. %s: $%.2f (24h: $%.2f, APY: %.2f%%)\n",
			i+1, p.Pair, p.TVL, p.Volume24h, p.APY)
	}
	
	// Update token metrics
	fmt.Println("\nUpdating token metrics...")
	tokens := []string{"ETH", "USDC", "BTC", "SOL"}
	for _, token := range tokens {
		metrics := &TokenMetrics{
			Token:        token,
			Price:       100.0,
			Volume24h:   1000000.0,
			Change24h:   5.0,
			MarketCap:   1000000000.0,
			LastUpdated: time.Now().Unix(),
		}
		warehouse.UpdateTokenMetrics(token, metrics)
	}
	
	// Get top tokens
	fmt.Println("\nTop tokens by market cap:")
	topTokens := warehouse.GetTopTokens(10)
	for i, t := range topTokens {
		fmt.Printf("  %d. %s: $%.2f (24h: $%.2f, Change: %.2f%%)\n",
			i+1, t.Token, t.Price, t.Volume24h, t.Change24h)
	}
	
	// Real-time aggregation
	fmt.Println("\nSetting up real-time aggregation...")
	aggregator := NewRealtimeAggregator(warehouse, time.Second)
	
	// Simulate real-time data
	for i := 0; i < 100; i++ {
		aggregator.Push("price.ETH", 100.0+float64(i))
		aggregator.Push("volume.ETH", float64(i*100))
	}
	
	// Query time series
	fmt.Println("\nQuerying time series data...")
	points := warehouse.QueryTimeSeries("price.ETH", 
		time.Now().Add(-time.Hour), time.Now())
	fmt.Printf("  Found %d data points\n", len(points))
	
	// Historical snapshot
	fmt.Println("\nCreating portfolio snapshot...")
	positions := []PositionSnapshot{
		{Token: "ETH", Amount: 10, Value: 1000, CostBasis: 800, PnL: 200, PnLPercent: 25},
		{Token: "USDC", Amount: 5000, Value: 5000, CostBasis: 5000, PnL: 0, PnLPercent: 0},
	}
	archive := NewArchive(warehouse)
	snapshot := archive.Snapshot("user1", positions)
	fmt.Printf("  Portfolio: $%.2f\n", snapshot.TotalValue)
	
	fmt.Println("\nData Platform ready!")
	
	// Keep running for real-time data
	fmt.Println("\nListening for real-time data...")
	ctx := context.Background()
	<-ctx.Done()
}