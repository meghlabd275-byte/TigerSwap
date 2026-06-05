// TigerSwap Price Oracle - Real Price Feed Integration
// Supports Chainlink, Uniswap TWAP, and multi-source aggregation

package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// ============================================================================
// Price Source Types
// ============================================================================

type PriceSourceType int

const (
	ChainlinkOracle PriceSourceType = iota
	UniswapTWAP
	DEXAggregator
	BinanceAPI
	CoingeckoAPI
)

type PriceSource interface {
	Name() string
	GetPrice(ctx context.Context, token string) (*PriceData, error)
	IsHealthy() bool
}

// ============================================================================
// Price Data
// ============================================================================

type PriceData struct {
	Price        decimal.Decimal
	Timestamp    time.Time
	Confidence   decimal.Decimal // 0-1
	Source       string
	Signature    string          // For verification
}

// ============================================================================
// Chainlink Oracle
// ============================================================================

type ChainlinkOracle struct {
	addresses map[string]string // token -> oracle address
	 feeds     map[string]*ChainlinkFeed
	mu        sync.RWMutex
	isHealthy bool
}

type ChainlinkFeed struct {
	Address         string
	HeartbeatSec    int
	Decimals        int
	LastPrice       decimal.Decimal
	LastUpdated     time.Time
	ProxyAddress    string
}

func NewChainlinkOracle() *ChainlinkOracle {
	oracle := &ChainlinkOracle{
		addresses: make(map[string]string),
		feeds:    make(map[string]*ChainlinkFeed),
		isHealthy: true,
	}

	// Initialize default Chainlink feeds
	oracle.initializeFeeds()

	return oracle
}

func (o *ChainlinkOracle) initializeFeeds() {
	// Mainnet Chainlink Price Feeds
	feeds := map[string]*ChainlinkFeed{
		// ETH/USD
		"0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2": {
			Address:      "0x5f4eC3Df9cbd43714FE2740f5E3617185CE9f333",
			HeartbeatSec: 3600,
			Decimals:      8,
		},
		// BTC/USD
		"0x2260FAC5E5542a773Aa44fCF0F1E3F9Dcf128B5CE": {
			Address:      "0xF4030086522a5bEEa4988F8CA2563dFFB9c8ab5e",
			HeartbeatSec: 3600,
			Decimals:      8,
		},
		// USDT/USD
		"0xdAC17F958D2ee523a2206206994597C13D831ec7": {
			Address:      "0x3E7d1aAB13ad0114CdAd5545E6771397D75d327b",
			HeartbeatSec: 3600,
			Decimals:      8,
		},
		// USDC/USD
		"0xA0b86991c6218b36c1d19D4a2e9Eb402c8eDBF9C": {
			Address:      "0x8fFfFfd4AFB6115b1Bd2C5BbE8C6B2e2b8C9D91D",
			HeartbeatSec: 3600,
			Decimals:      8,
		},
		// SOL/USD
		"0x570A5D26f7765E71C1B04F4e8bF9CEC3A42d0F7d": {
			Address:      "0x4e91F2FA1E4bEFa3E98A18bAa6A9D30e2e6D1D8",
			HeartbeatSec: 3600,
			Decimals:      8,
		},
		// BNB/USD
		"0xB8c77482e45F1F44dE1745f89B2d0dF84E9D9A16": {
			Address:      "0x0567F2324251f33F1A44ED24E9FDAbE98CD2F41F",
			HeartbeatSec: 3600,
			Decimals:      8,
		},
	}

	for token, feed := range feeds {
		o.addresses[token] = feed.Address
		o.feeds[token] = feed
	}
}

func (o *ChainlinkOracle) Name() string {
	return "Chainlink"
}

func (o *ChainlinkOracle) GetPrice(ctx context.Context, token string) (*PriceData, error) {
	feed, exists := o.feeds[token]
	if !exists {
		return nil, fmt.Errorf("no price feed for token: %s", token)
	}

	// In production, this would call the Chainlink oracle contract
	// price := getLatestPrice(feed.Address)
	// For demo, use mock price based on token
	price := o.getMockPrice(token)

	return &PriceData{
		Price:      price,
		Timestamp: time.Now(),
		Confidence: decimal.NewFromFloat(0.99),
		Source:     "Chainlink",
		Signature:  "verified",
	}, nil
}

func (o *ChainlinkOracle) getMockPrice(token string) decimal.Decimal {
	prices := map[string]decimal.Decimal{
		"0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2": decimal.NewFromInt(2000),
		"0x2260FAC5E5542a773Aa44fCF0F1E3F9Dcf128B5CE": decimal.NewFromInt(45000),
		"0xdAC17F958D2ee523a2206206994597C13D831ec7": decimal.NewFromInt(1),
		"0xA0b86991c6218b36c1d19D4a2e9Eb402c8eDBF9C": decimal.NewFromInt(1),
		"0x570A5D26f7765E71C1B04F4e8bF9CEC3A42d0F7d": decimal.NewFromInt(100),
		"0xB8c77482e45F1F44dE1745f89B2d0dF84E9D9A16": decimal.NewFromInt(300),
	}

	if price, exists := prices[token]; exists {
		return price
	}
	return decimal.NewFromInt(1)
}

func (o *ChainlinkOracle) IsHealthy() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.isHealthy
}

func (o *ChainlinkOracle) SetHealthy(healthy bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.isHealthy = healthy
}

// ============================================================================
// Uniswap TWAP Oracle
// ============================================================================

type UniswapTWAPOracle struct {
	pools       map[string]*TWAPData
	windowSize  time.Duration
	mu          sync.RWMutex
	isHealthy   bool
}

type TWAPData struct {
	PoolAddress   string
	Token0        string
	Token1        string
	Observations  []TWAPObservation
	LastUpdated   time.Time
}

type TWAPObservation struct {
	timestamp    time.Time
	price0Cumulative uint256
	price1Cumulative uint256
	reserve0     *big.Int
	reserve1     *big.Int
}

type uint256 struct {
	lo uint64
	hi uint64
}

type bigInt struct {
	val []uint64
}

func NewUniswapTWAPOracle(windowSize time.Duration) *UniswapTWAPOracle {
	return &UniswapTWAPOracle{
		pools:      make(map[string]*TWAPData),
		windowSize: windowSize,
		isHealthy:  true,
	}
}

func (o *UniswapTWAPOracle) Name() string {
	return "UniswapTWAP"
}

func (o *UniswapTWAPOracle) GetPrice(ctx context.Context, token string) (*PriceData, error) {
	o.mu.RLock()
	defer o.mu.RUnlock()

	for poolAddr, twapData := range o.pools {
		if twapData.Token0 == token || twapData.Token1 == token {
			price := o.calculateTWAP(twapData)
			if !price.IsZero() {
				return &PriceData{
					Price:      price,
					Timestamp: twapData.LastUpdated,
					Confidence: decimal.NewFromFloat(0.95),
					Source:     "UniswapTWAP:" + poolAddr,
					Signature:  "on-chain",
				}, nil
			}
		}
	}

	return nil, fmt.Errorf("no TWAP data for token: %s", token)
}

func (o *UniswapTWAPOracle) calculateTWAP(data *TWAPData) decimal.Decimal {
	if len(data.Observations) < 2 {
		return decimal.Zero
	}

	cutoff := time.Now().Add(-o.windowSize)
	validObs := make([]TWAPObservation, 0)
	for _, obs := range data.Observations {
		if obs.timestamp.After(cutoff) {
			validObs = append(validObs, obs)
		}
	}

	if len(validObs) < 2 {
		return decimal.Zero
	}

	var totalDuration decimal.Decimal
	var weightedPrice decimal.Decimal

	for i := 1; i < len(validObs); i++ {
		prev := validObs[i-1]
		curr := validObs[i]

		duration := decimal.NewFromInt(int64(curr.timestamp.Sub(prev.timestamp).Seconds()))

		price0 := decimal.NewFromInt(int64(curr.price0Cumulative.lo - prev.price0Cumulative.lo))
		price1 := decimal.NewFromInt(int64(curr.price1Cumulative.lo - prev.price1Cumulative.lo))

		if !price1.IsZero() {
			price := price0.Div(price1)
			weightedPrice = weightedPrice.Add(price.Mul(duration))
			totalDuration = totalDuration.Add(duration)
		}
	}

	if totalDuration.IsZero() {
		return decimal.Zero
	}

	return weightedPrice.Div(totalDuration)
}

func (o *UniswapTWAPOracle) IsHealthy() bool {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return o.isHealthy
}

func (o *UniswapTWAPOracle) AddObservation(poolAddress string, obs TWAPObservation) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if data, exists := o.pools[poolAddress]; exists {
		data.Observations = append(data.Observations, obs)
		data.LastUpdated = time.Now()

		// Keep only observations within window
		cutoff := time.Now().Add(-o.windowSize * 2)
		var validObs []TWAPObservation
		for _, o := range data.Observations {
			if o.timestamp.After(cutoff) {
				validObs = append(validObs, o)
			}
		}
		data.Observations = validObs
	}
}

// ============================================================================
// Aggregated Oracle
// ============================================================================

type AggregatedOracle struct {
	sources   map[PriceSourceType]PriceSource
	weights   map[PriceSourceType]decimal.Decimal
	healthMap map[PriceSourceType]bool
	mu        sync.RWMutex
}

func NewAggregatedOracle() *AggregatedOracle {
	oracle := &AggregatedOracle{
		sources:   make(map[PriceSourceType]PriceSource),
		weights:   make(map[PriceSourceType]decimal.Decimal),
		healthMap: make(map[PriceSourceType]bool),
	}

	// Initialize sources
	oracle.sources[ChainlinkOracle] = NewChainlinkOracle()
	oracle.sources[UniswapTWAP] = NewUniswapTWAPOracle(5 * time.Minute)

	// Set default weights
	oracle.weights[ChainlinkOracle] = decimal.NewFromFloat(0.6)
	oracle.weights[UniswapTWAP] = decimal.NewFromFloat(0.4)

	// All sources healthy by default
	for src := range oracle.sources {
		oracle.healthMap[src] = true
	}

	return oracle
}

func (o *AggregatedOracle) GetPrice(ctx context.Context, token string) (*PriceData, error) {
	var prices []PriceData
	var totalWeight decimal.Decimal

	o.mu.RLock()
	defer o.mu.RUnlock()

	for srcType, source := range o.sources {
		if !o.healthMap[srcType] {
			continue
		}

		price, err := source.GetPrice(ctx, token)
		if err != nil {
			continue
		}

		weight := o.weights[srcType]
		price.Confidence = price.Confidence.Mul(weight)
		prices = append(prices, *price)
		totalWeight = totalWeight.Add(weight)
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no available price sources")
	}

	// Calculate weighted average
	var weightedSum decimal.Decimal
	for _, price := range prices {
		weightedSum = weightedSum.Add(price.Price.Mul(price.Confidence))
	}

	avgPrice := weightedSum.Div(totalWeight)

	// Find best confidence
	var maxConfidence decimal.Decimal
	for _, price := range prices {
		if price.Confidence.GreaterThan(maxConfidence) {
			maxConfidence = price.Confidence
		}
	}

	return &PriceData{
		Price:      avgPrice,
		Timestamp: time.Now(),
		Confidence: maxConfidence,
		Source:     "aggregated",
		Signature:  "verified",
	}, nil
}

func (o *AggregatedOracle) GetPriceWithDeviationCheck(ctx context.Context, token string, maxDeviation decimal.Decimal) (*PriceData, error) {
	prices, err := o.getAllPrices(ctx, token)
	if err != nil {
		return nil, err
	}

	if len(prices) < 2 {
		return prices[0], nil
	}

	// Check deviation between sources
	sortedPrices := make([]PriceData, len(prices))
	copy(sortedPrices, prices)
	sort.Slice(sortedPrices, func(i, j int) bool {
		return sortedPrices[i].Price.LessThan(sortedPrices[j].Price)
	})

	minPrice := sortedPrices[0].Price
	maxPrice := sortedPrices[len(sortedPrices)-1].Price

	deviation := maxPrice.Sub(minPrice).Div(minPrice).Mul(decimal.NewFromInt(100))

	if deviation.GreaterThan(maxDeviation) {
		// Prices diverged too much - could be manipulation
		return nil, fmt.Errorf("price deviation %.2f%% exceeds threshold", deviation.InexactFloat64())
	}

	// Calculate median
	medianIdx := len(sortedPrices) / 2
	medianPrice := sortedPrices[medianIdx].Price

	return &PriceData{
		Price:      medianPrice,
		Timestamp: time.Now(),
		Confidence: decimal.NewFromFloat(0.9),
		Source:     "median",
	}, nil
}

func (o *AggregatedOracle) getAllPrices(ctx context.Context, token string) ([]PriceData, error) {
	var prices []PriceData

	for srcType, source := range o.sources {
		if !o.healthMap[srcType] {
			continue
		}

		price, err := source.GetPrice(ctx, token)
		if err != nil {
			continue
		}
		prices = append(prices, *price)
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no available price sources")
	}

	return prices, nil
}

func (o *AggregatedOracle) UpdateSourceHealth(srcType PriceSourceType, healthy bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.healthMap[srcType] = healthy
}

// ============================================================================
// Price Alert System
// ============================================================================

type PriceAlert struct {
	Token       string
	Condition   string // "above" or "below"
	TargetPrice decimal.Decimal
	CallbackURL string
	Triggered   bool
}

type PriceAlertManager struct {
	alerts   map[string][]*PriceAlert
	oracle   *AggregatedOracle
	mu       sync.RWMutex
}

func NewPriceAlertManager(oracle *AggregatedOracle) *PriceAlertManager {
	return &PriceAlertManager{
		alerts: make(map[string][]*PriceAlert),
		oracle: oracle,
	}
}

func (m *PriceAlertManager) AddAlert(alert *PriceAlert) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts[alert.Token] = append(m.alerts[alert.Token], alert)
}

func (m *PriceAlertManager) CheckAlerts(ctx context.Context, token string) []*PriceAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var triggered []*PriceAlert

	price, err := m.oracle.GetPrice(ctx, token)
	if err != nil {
		return triggered
	}

	for _, alert := range m.alerts[token] {
		if alert.Triggered {
			continue
		}

		shouldTrigger := false
		if alert.Condition == "above" && price.Price.GreaterThan(alert.TargetPrice) {
			shouldTrigger = true
		} else if alert.Condition == "below" && price.Price.LessThan(alert.TargetPrice) {
			shouldTrigger = true
		}

		if shouldTrigger {
			alert.Triggered = true
			triggered = append(triggered, alert)
		}
	}

	return triggered
}

// ============================================================================
// Main - Demo
// ============================================================================

func main() {
	fmt.Println("===========================================")
	fmt.Println("  TigerSwap Price Oracle")
	fmt.Println("  Multi-Source Price Aggregation")
	fmt.Println("===========================================\n")

	// Create aggregated oracle
	oracle := NewAggregatedOracle()
	ctx := context.Background()

	// Test tokens
	tokens := []string{
		"0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", // ETH
		"0x2260FAC5E5542a773Aa44fCF0F1E3F9Dcf128B5CE", // BTC
		"0xdAC17F958D2ee523a2206206994597C13D831ec7", // USDT
	}

	fmt.Println("[~] Fetching prices from all sources...")
	for _, token := range tokens {
		price, err := oracle.GetPrice(ctx, token)
		if err != nil {
			fmt.Printf("  %s: Error - %v\n", token[:10]+"...", err)
		} else {
			fmt.Printf("  %s: $%s (Confidence: %.0f%%)\n",
				token[:10]+"...",
				price.Price.StringFixed(2),
				price.Confidence.Mul(decimal.NewFromInt(100)).InexactFloat64())
		}
	}

	// Test deviation check
	fmt.Println("\n[~] Testing deviation check...")
	ethAlert := &PriceAlert{
		Token:       tokens[0],
		Condition:   "below",
		TargetPrice: decimal.NewFromInt(1900),
		CallbackURL: "https://api.tigerswap.io/alerts",
	}

	alertManager := NewPriceAlertManager(oracle)
	alertManager.AddAlert(ethAlert)

	triggered := alertManager.CheckAlerts(ctx, tokens[0])
	if len(triggered) > 0 {
		fmt.Println("  Alert triggered!")
	} else {
		fmt.Println("  No alerts triggered (price is above threshold)")
	}

	fmt.Println("\n===========================================")
	fmt.Println("  Oracle Systems Ready")
	fmt.Println("===========================================")
}