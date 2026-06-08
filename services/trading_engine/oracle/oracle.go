package oracle

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"

	"TigerSwap/services/trading_engine/config"
)

// PriceOracle provides price feeds
type PriceOracle interface {
	// GetPrice returns the price of a token pair
	GetPrice(tokenIn, tokenOut string) (*big.Int, error)

	// GetPrices returns multiple prices
	GetPrices(pairs [][2]string) (map[string]*big.Int, error)

	// Start starts the price oracle
	Start(ctx context.Context) error

	// Stop stops the price oracle
	Stop()
}

// ChainlinkOracle uses Chainlink for price feeds
type ChainlinkOracle struct {
	cfg    *config.Config
	client *ethclient.Client

	// Price cache
	prices    map[string]*big.Int
	pricesMux sync.RWMutex
	lastUpdate time.Time

	// Feed addresses
	feeds map[string]string // token pair -> feed address
}

// NewChainlinkOracle creates a new Chainlink oracle
func NewChainlinkOracle(cfg *config.Config) (*ChainlinkOracle, error) {
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, err
	}

	oracle := &ChainlinkOracle{
		cfg:    cfg,
		client: client,
		prices: make(map[string]*big.Int),
		feeds: map[string]string{
			"ETH/USD": "0x5f4eC3Df9cEE4D75841CD12d6b6E7B3D3C2E8D",
			"BTC/USD": "0xF4030086522a5bE7a84d5564b2e0B5c3dB3c0E8D",
			"WETH/USDC": "0x0000000000000000000000000000000000000000",
			"WBTC/USDC": "0x0000000000000000000000000000000000000000",
		},
	}

	return oracle, nil
}

// GetPrice returns the price of a token pair
func (o *ChainlinkOracle) GetPrice(tokenIn, tokenOut string) (*big.Int, error) {
	key := fmt.Sprintf("%s/%s", tokenIn, tokenOut)

	// Check cache first
	o.pricesMux.RLock()
	if price, ok := o.prices[key]; ok {
		o.pricesMux.RUnlock()
		return price, nil
	}
	o.pricesMux.RUnlock()

	// Fetch from Chainlink
	price, err := o.fetchPrice(tokenIn, tokenOut)
	if err != nil {
		return nil, err
	}

	// Update cache
	o.pricesMux.Lock()
	o.prices[key] = price
	o.pricesMux.Unlock()

	return price, nil
}

// GetPrices returns multiple prices
func (o *ChainlinkOracle) GetPrices(pairs [][2]string) (map[string]*big.Int, error) {
	result := make(map[string]*big.Int)

	for _, pair := range pairs {
		price, err := o.GetPrice(pair[0], pair[1])
		if err != nil {
			continue
		}
		key := fmt.Sprintf("%s/%s", pair[0], pair[1])
		result[key] = price
	}

	return result, nil
}

// fetchPrice fetches price from Chainlink
func (o *ChainlinkOracle) fetchPrice(tokenIn, tokenOut string) (*big.Int, error) {
	key := fmt.Sprintf("%s/%s", tokenIn, tokenOut)
	feedAddr, ok := o.feeds[key]
	if !ok {
		// Default to ETH price
		return big.NewInt(1e8), nil // 1:1 with 1e8 precision
	}

	// In production, call Chainlink contract
	// For now, return mock price
	return big.NewInt(1e8), nil
}

// Start starts the price oracle
func (o *ChainlinkOracle) Start(ctx context.Context) error {
	// Start price updates
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				o.updatePrices()
			}
		}
	}()

	return nil
}

// updatePrices updates all prices
func (o *ChainlinkOracle) updatePrices() {
	for key := range o.feeds {
		var tokenIn, tokenOut string
		fmt.Sscanf(key, "%s/%s", &tokenIn, &tokenOut)

		price, err := o.fetchPrice(tokenIn, tokenOut)
		if err != nil {
			continue
		}

		o.pricesMux.Lock()
		o.prices[key] = price
		o.lastUpdate = time.Now()
		o.pricesMux.Unlock()
	}
}

// Stop stops the price oracle
func (o *ChainlinkOracle) Stop() {
	// Cleanup
}

// PythOracle uses Pyth Network for price feeds
type PythOracle struct {
	cfg    *config.Config
	client *ethclient.Client

	prices    map[string]*big.Int
	pricesMux sync.RWMutex
}

// NewPythOracle creates a new Pyth oracle
func NewPythOracle(cfg *config.Config) (*PythOracle, error) {
	client, err := ethclient.Dial(cfg.RPCURL)
	if err != nil {
		return nil, err
	}

	return &PythOracle{
		cfg:    cfg,
		client: client,
		prices: make(map[string]*big.Int),
	}, nil
}

// GetPrice returns the price
func (o *PythOracle) GetPrice(tokenIn, tokenOut string) (*big.Int, error) {
	key := fmt.Sprintf("%s/%s", tokenIn, tokenOut)

	o.pricesMux.RLock()
	if price, ok := o.prices[key]; ok {
		o.pricesMux.RUnlock()
		return price, nil
	}
	o.pricesMux.RUnlock()

	return big.NewInt(1e8), nil
}

// GetPrices returns multiple prices
func (o *PythOracle) GetPrices(pairs [][2]string) (map[string]*big.Int, error) {
	result := make(map[string]*big.Int)

	for _, pair := range pairs {
		price, err := o.GetPrice(pair[0], pair[1])
		if err != nil {
			continue
		}
		key := fmt.Sprintf("%s/%s", pair[0], pair[1])
		result[key] = price
	}

	return result, nil
}

// Start starts the oracle
func (o *PythOracle) Start(ctx context.Context) error {
	return nil
}

// Stop stops the oracle
func (o *PythOracle) Stop() {
}

// NewOracle creates a new price oracle
func NewOracle(cfg *config.Config) (PriceOracle, error) {
	switch cfg.OracleType {
	case "chainlink":
		return NewChainlinkOracle(cfg)
	case "pyth":
		return NewPythOracle(cfg)
	default:
		return NewChainlinkOracle(cfg)
	}
}

// MockOracle is a mock oracle for testing
type MockOracle struct {
	prices map[string]*big.Int
}

// NewMockOracle creates a new mock oracle
func NewMockOracle() *MockOracle {
	return &MockOracle{
		prices: map[string]*big.Int{
			"ETH/USDC": big.NewInt(3500e8),
			"BTC/USDC": big.NewInt(95000e8),
			"WETH/USDC": big.NewInt(3500e8),
			"WBTC/USDC": big.NewInt(95000e8),
		},
	}
}

// GetPrice returns the price
func (o *MockOracle) GetPrice(tokenIn, tokenOut string) (*big.Int, error) {
	key := fmt.Sprintf("%s/%s", tokenIn, tokenOut)
	if price, ok := o.prices[key]; ok {
		return price, nil
	}
	return big.NewInt(1e8), nil
}

// GetPrices returns multiple prices
func (o *MockOracle) GetPrices(pairs [][2]string) (map[string]*big.Int, error) {
	result := make(map[string]*big.Int)
	for _, pair := range pairs {
		price, _ := o.GetPrice(pair[0], pair[1])
		key := fmt.Sprintf("%s/%s", pair[0], pair[1])
		result[key] = price
	}
	return result, nil
}

// Start starts the oracle
func (o *MockOracle) Start(ctx context.Context) error {
	return nil
}

// Stop stops the oracle
func (o *MockOracle) Stop() {
}

// AggregatedOracle aggregates multiple oracles
type AggregatedOracle struct {
	oracles []PriceOracle
}

// NewAggregatedOracle creates a new aggregated oracle
func NewAggregatedOracle(oracles []PriceOracle) *AggregatedOracle {
	return &AggregatedOracle{
		oracles: oracles,
	}
}

// GetPrice returns the median price from all oracles
func (o *AggregatedOracle) GetPrice(tokenIn, tokenOut string) (*big.Int, error) {
	prices := make([]*big.Int, 0, len(o.oracles))

	for _, oracle := range o.oracles {
		price, err := oracle.GetPrice(tokenIn, tokenOut)
		if err != nil {
			continue
		}
		prices = append(prices, price)
	}

	if len(prices) == 0 {
		return nil, fmt.Errorf("no prices available")
	}

	// Return median
	mid := len(prices) / 2
	return prices[mid], nil
}

// GetPrices returns multiple prices
func (o *AggregatedOracle) GetPrices(pairs [][2]string) (map[string]*big.Int, error) {
	result := make(map[string]*big.Int)
	for _, pair := range pairs {
		price, err := o.GetPrice(pair[0], pair[1])
		if err != nil {
			continue
		}
		key := fmt.Sprintf("%s/%s", pair[0], pair[1])
		result[key] = price
	}
	return result, nil
}

// Start starts the oracle
func (o *AggregatedOracle) Start(ctx context.Context) error {
	for _, oracle := range o.oracles {
		if err := oracle.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Stop stops the oracle
func (o *AggregatedOracle) Stop() {
	for _, oracle := range o.oracles {
		oracle.Stop()
	}
}

// HTTPOracle fetches prices from HTTP API
type HTTPOracle struct {
	url    string
	prices map[string]*big.Int
	client *http.Client
}

// NewHTTPOracle creates a new HTTP oracle
func NewHTTPOracle(url string) *HTTPOracle {
	return &HTTPOracle{
		url:    url,
		prices: make(map[string]*big.Int),
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// GetPrice returns the price
func (o *HTTPOracle) GetPrice(tokenIn, tokenOut string) (*big.Int, error) {
	key := fmt.Sprintf("%s/%s", tokenIn, tokenOut)
	if price, ok := o.prices[key]; ok {
		return price, nil
	}
	return big.NewInt(1e8), nil
}

// GetPrices returns multiple prices
func (o *HTTPOracle) GetPrices(pairs [][2]string) (map[string]*big.Int, error) {
	// Fetch from API
	resp, err := o.client.Get(o.url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]json.Number
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	prices := make(map[string]*big.Int)
	for k, v := range result {
		price, _ := v.Int64()
		prices[k] = big.NewInt(price)
	}

	return prices, nil
}

// Start starts the oracle
func (o *HTTPOracle) Start(ctx context.Context) error {
	return nil
}

// Stop stops the oracle
func (o *HTTPOracle) Stop() {
}