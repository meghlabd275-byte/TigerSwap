package services

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"tigerswap/backend/go/amm"
	"tigerswap/backend/go/tigerswap"
)

// PriceAggregator aggregates prices from multiple sources
type PriceAggregator struct {
	redis        *redis.Client
	coingeckoAPI string
	coinbaseAPI  string
	binanceAPI   string
	chainlink    map[int64]string // chainID -> contract address
	signer       *ecdsa.PrivateKey
	tigerClients map[int64]*tigerswap.Client
	mu           sync.RWMutex
	prices       map[string]*PriceData
}

// PriceData represents price data from multiple sources
type PriceData struct {
	Symbol       string    `json:"symbol"`
	USD          float64   `json:"usd"`
	USDChange24h float64   `json:"usd_change_24h"`
	USDVolume24h float64   `json:"usd_volume_24h"`
	USDMarketCap float64   `json:"usd_market_cap"`
	LastUpdated  time.Time `json:"last_updated"`
	Sources      []string  `json:"sources"`
	Confidence   float64   `json:"confidence"`
}

// AggregatedQuote represents a swap quote from multiple sources
type AggregatedQuote struct {
	FromToken    string            `json:"from_token"`
	ToToken      string            `json:"to_token"`
	AmountIn     string            `json:"amount_in"`
	AmountOut    string            `json:"amount_out"`
	AmountOutMin string            `json:"amount_out_min"`
	PriceImpact  float64           `json:"price_impact"`
	GasEstimate  uint64            `json:"gas_estimate"`
	GasPrice     *big.Int          `json:"gas_price"`
	Route        []string          `json:"route"`
	SourceQuotes map[string]string `json:"source_quotes"` // source -> quote
	BestSource   string            `json:"best_source"`
	BlockNumber  uint64            `json:"block_number"`
	ValidUntil   time.Time         `json:"valid_until"`
}

// RouteInfo represents a swap route
type RouteInfo struct {
	FromToken    string   `json:"from_token"`
	ToToken      string   `json:"to_token"`
	Path         []string `json:"path"`
	Adapters     []string `json:"adapters"`
	EstimatedOut string   `json:"estimated_out"`
	GasUsed      uint64   `json:"gas_used"`
}

// PriceSource represents a price source
type PriceSource struct {
	Name        string  `json:"name"`
	URL         string  `json:"url"`
	Priority    int     `json:"priority"`
	Reliability float64 `json:"reliability"`
}

func NewPriceAggregator(redisClient *redis.Client) *PriceAggregator {
	signer, _ := tigerswap.LoadSigner()
	return &PriceAggregator{
		redis:        redisClient,
		signer:       signer,
		tigerClients: make(map[int64]*tigerswap.Client),
		coingeckoAPI: "https://api.coingecko.com/api/v3",
		coinbaseAPI:  "https://api.coinbase.com/v2",
		binanceAPI:   "https://api.binance.com/api/v3",
		chainlink: map[int64]string{
			1:     "0x5f4eC3Df9c8bD3B48BE9A2E29bE4B7c4E5rT8Y",   // Chainlink ETH/USD
			137:   "0xAB594600376Ec9fD91F8e885dADF0CE036862dE0", // Chainlink MATIC/USD
			56:    "0x0567F2323251f0A73B2bf8E3f7f8D4Cf8dB9E8F",  // Chainlink BNB/USD
			43114: "0x31eF83a530Fde1B38ee9A181065E6e6A4eC3d3E5", // Chainlink AVAX/USD
		},
		prices: make(map[string]*PriceData),
	}
}

// GetRealPrice fetches real price from multiple sources
func (p *PriceAggregator) GetRealPrice(ctx context.Context, symbol string) (*PriceData, error) {
	// Try multiple sources in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex

	var coinGeckoPrice, coinbasePrice, binancePrice float64
	var cgErr, cbErr, binErr error

	// CoinGecko
	wg.Add(1)
	go func() {
		defer wg.Done()
		price, err := p.getCoinGeckoPrice(symbol)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			cgErr = err
		} else {
			coinGeckoPrice = price
		}
	}()

	// Coinbase
	wg.Add(1)
	go func() {
		defer wg.Done()
		price, err := p.getCoinbasePrice(symbol)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			cbErr = err
		} else {
			coinbasePrice = price
		}
	}()

	// Binance
	wg.Add(1)
	go func() {
		defer wg.Done()
		price, err := p.getBinancePrice(symbol)
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			binErr = err
		} else {
			binancePrice = price
		}
	}()

	wg.Wait()

	// Calculate weighted average based on reliability
	sources := []string{}
	prices := []float64{}
	weights := []float64{}

	if coinGeckoPrice > 0 {
		sources = append(sources, "coingecko")
		prices = append(prices, coinGeckoPrice)
		weights = append(weights, 0.4) // 40% weight
	}

	if coinbasePrice > 0 {
		sources = append(sources, "coinbase")
		prices = append(prices, coinbasePrice)
		weights = append(weights, 0.35) // 35% weight
	}

	if binancePrice > 0 {
		sources = append(sources, "binance")
		prices = append(prices, binancePrice)
		weights = append(weights, 0.25) // 25% weight
	}

	if len(prices) == 0 {
		// No live source responded: fall back to cache, otherwise fail closed
		// with the underlying source errors so callers never act on a stale guess.
		if cached, err := p.getCachedPrice(ctx, symbol); err == nil {
			return cached, nil
		}
		return nil, fmt.Errorf("no live price for %s (coingecko: %v, coinbase: %v, binance: %v)", symbol, cgErr, cbErr, binErr)
	}

	// Calculate weighted average
	var totalWeight float64
	var weightedSum float64
	for i := range prices {
		weightedSum += prices[i] * weights[i]
		totalWeight += weights[i]
	}

	avgPrice := weightedSum / totalWeight

	// Calculate confidence based on price spread
	var spread float64
	if len(prices) > 1 {
		minPrice := prices[0]
		maxPrice := prices[0]
		for _, price := range prices {
			if price < minPrice {
				minPrice = price
			}
			if price > maxPrice {
				maxPrice = price
			}
		}
		spread = (maxPrice - minPrice) / avgPrice
	}

	confidence := 1.0 - spread // Higher confidence if prices are close

	priceData := &PriceData{
		Symbol:      symbol,
		USD:         avgPrice,
		Sources:     sources,
		Confidence:  confidence,
		LastUpdated: time.Now(),
	}

	// Cache the price
	p.cachePrice(ctx, symbol, priceData)

	return priceData, nil
}

func (p *PriceAggregator) getCoinGeckoPrice(symbol string) (float64, error) {
	// Map common symbols to CoinGecko IDs
	coingeckoIDs := map[string]string{
		"ETH":   "ethereum",
		"BTC":   "bitcoin",
		"USDC":  "usd-coin",
		"USDT":  "tether",
		"BNB":   "binancecoin",
		"MATIC": "matic-network",
		"SOL":   "solana",
		"AVAX":  "avalanche-2",
		"LINK":  "chainlink",
		"UNI":   "uniswap",
		"AAVE":  "aave",
		"DOT":   "polkadot",
		"ATOM":  "cosmos",
		"LTC":   "litecoin",
		"XRP":   "ripple",
		"DOGE":  "dogecoin",
		"TRX":   "tron",
		"APT":   "aptos",
		"ARB":   "arbitrum",
		"OP":    "optimism",
		"FTM":   "fantom",
		"NEAR":  "near",
		"INJ":   "injective-protocol",
		"SUI":   "sui",
		"SEI":   "sei-protocol",
		"TIA":   "celestia",
		"SHIB":  "shiba-inu",
		"PEPE":  "pepe",
	}

	id, ok := coingeckoIDs[symbol]
	if !ok {
		id = symbol
	}

	url := fmt.Sprintf("%s/simple/price?ids=%s&vs_currencies=usd&include_24hr_change=true&include_24hr_vol=true&include_market_cap=true",
		p.coingeckoAPI, id)

	req, err := http.NewRequestWithContext(context.Background(), "GET", url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("coingecko API error: %d", resp.StatusCode)
	}

	var result map[string]struct {
		USD          float64 `json:"usd"`
		USDChange    float64 `json:"usd_24h_change"`
		USDVolume    float64 `json:"usd_24h_vol"`
		USDMarketCap float64 `json:"usd_market_cap"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	if data, ok := result[id]; ok {
		return data.USD, nil
	}

	return 0, fmt.Errorf("symbol not found: %s", symbol)
}

func (p *PriceAggregator) getCoinbasePrice(symbol string) (float64, error) {
	url := fmt.Sprintf("%s/prices/%s/spot", p.coinbaseAPI, symbol+"-USD")

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("coinbase API error: %d", resp.StatusCode)
	}

	var result struct {
		Data struct {
			Base     string `json:"base"`
			Currency string `json:"currency"`
			Amount   string `json:"amount"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	var amount float64
	fmt.Sscanf(result.Data.Amount, "%f", &amount)

	return amount, nil
}

func (p *PriceAggregator) getBinancePrice(symbol string) (float64, error) {
	url := fmt.Sprintf("%s/ticker/price?symbol=%sUSDT", p.binanceAPI, symbol)

	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("binance API error: %d", resp.StatusCode)
	}

	var result struct {
		Symbol string `json:"symbol"`
		Price  string `json:"price"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	var price float64
	fmt.Sscanf(result.Price, "%f", &price)

	return price, nil
}

func (p *PriceAggregator) cachePrice(ctx context.Context, symbol string, data *PriceData) {
	key := fmt.Sprintf("price:%s", symbol)
	jsonData, _ := json.Marshal(data)
	p.redis.Set(ctx, key, jsonData, 5*time.Minute)
}

func (p *PriceAggregator) getCachedPrice(ctx context.Context, symbol string) (*PriceData, error) {
	key := fmt.Sprintf("price:%s", symbol)
	data, err := p.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("price not found and no sources available")
	}

	var priceData PriceData
	if err := json.Unmarshal([]byte(data), &priceData); err != nil {
		return nil, err
	}

	return &priceData, nil
}

// GetAggregatedQuote quotes the best route across TigerSwap's own pool
// families (V2, V3, Stable, Weighted) by reading reserves from the TigerSwap
// factory/pool contracts on the requested chain.
func (p *PriceAggregator) GetAggregatedQuote(ctx context.Context, chainID int64, fromToken, toToken, amount string, sources []string) (*AggregatedQuote, error) {
	amountIn, ok := new(big.Int).SetString(amount, 10)
	if !ok || amountIn.Sign() <= 0 {
		return nil, fmt.Errorf("invalid amount %q", amount)
	}
	if !common.IsHexAddress(fromToken) || !common.IsHexAddress(toToken) {
		return nil, fmt.Errorf("invalid token address")
	}

	client, err := p.tigerClient(chainID)
	if err != nil {
		return nil, err
	}

	path := []common.Address{common.HexToAddress(fromToken), common.HexToAddress(toToken)}

	type familyQuote struct {
		family amm.Family
		out    *big.Int
	}
	var quotes []familyQuote
	var firstErr error

	for family, fee := range tigerFamilyFees {
		if len(sources) > 0 && !contains(sources, string(family)) {
			continue
		}
		out, err := client.QuoteFamily(ctx, family, path, amountIn, fee)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		quotes = append(quotes, familyQuote{family: family, out: out})
	}

	if len(quotes) == 0 {
		if firstErr != nil {
			return nil, firstErr
		}
		return nil, fmt.Errorf("no TigerSwap route available for %s -> %s on chain %d", fromToken, toToken, chainID)
	}

	sort.Slice(quotes, func(i, j int) bool { return quotes[i].out.Cmp(quotes[j].out) > 0 })
	best := quotes[0]

	sourceQuotes := make(map[string]string, len(quotes))
	for _, q := range quotes {
		sourceQuotes[string(q.family)] = q.out.String()
	}

	// Price impact from the token USD prices when available.
	var priceImpact float64
	if priceFrom, err := p.GetRealPrice(ctx, p.getSymbolFromAddress(fromToken)); err == nil && priceFrom.USD > 0 {
		if priceTo, err := p.GetRealPrice(ctx, p.getSymbolFromAddress(toToken)); err == nil && priceTo.USD > 0 {
			inputValue := weiToFloat(amountIn, 18) * priceFrom.USD
			outputValue := weiToFloat(best.out, 18) * priceTo.USD
			if inputValue > 0 {
				priceImpact = ((inputValue - outputValue) / inputValue) * 100
			}
		}
	}

	blockNumber, err := client.RPC().BlockNumber(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch current block: %w", err)
	}
	gasPrice, err := client.RPC().SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch gas price: %w", err)
	}

	return &AggregatedQuote{
		FromToken:    fromToken,
		ToToken:      toToken,
		AmountIn:     amountIn.String(),
		AmountOut:    best.out.String(),
		AmountOutMin: amm.ApplySlippage(best.out, 0.5).String(),
		PriceImpact:  priceImpact,
		GasEstimate:  p.estimateGas(chainID, fromToken, toToken),
		GasPrice:     gasPrice,
		Route:        []string{fromToken, toToken},
		SourceQuotes: sourceQuotes,
		BestSource:   string(best.family),
		BlockNumber:  blockNumber,
		ValidUntil:   time.Now().Add(30 * time.Second),
	}, nil
}

// tigerFamilyFees maps each TigerSwap pool family to its fee in basis points.
var tigerFamilyFees = map[amm.Family]uint32{
	amm.FamilyV2:       30,
	amm.FamilyV3:       5,
	amm.FamilyStable:   1,
	amm.FamilyWeighted: 30,
}

// tigerClient returns a TigerSwap client for a chain, creating it on first use.
func (p *PriceAggregator) tigerClient(chainID int64) (*tigerswap.Client, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.tigerClients == nil {
		p.tigerClients = make(map[int64]*tigerswap.Client)
	}
	if c, ok := p.tigerClients[chainID]; ok {
		return c, nil
	}

	for _, d := range tigerswap.LoadDeployments() {
		if d.ChainID != chainID {
			continue
		}
		c, err := tigerswap.NewClient(d, p.signer)
		if err != nil {
			return nil, err
		}
		p.tigerClients[chainID] = c
		return c, nil
	}
	return nil, fmt.Errorf("no TigerSwap deployment configured for chain %d", chainID)
}

// CloseTigerClients releases all cached TigerSwap RPC connections.
func (p *PriceAggregator) CloseTigerClients() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, c := range p.tigerClients {
		c.Close()
	}
	p.tigerClients = nil
}

// getSymbolFromAddress maps a well-known token address to its symbol.
func (p *PriceAggregator) getSymbolFromAddress(address string) string {
	symbols := map[string]string{
		"0x0000000000000000000000000000000000000000": "ETH",
		"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48": "USDC",
		"0xdac17f958d2ee523a2206206994597c13d831ec7": "USDT",
		"0x2260fac5e5542a773aa44fbcfedf7c193bc2c599": "WBTC",
		"0x514910771af9ca656af840dff83e8264ecf986ca": "LINK",
		"0x1f9840a85d5af5bf1d1762f925bdaddc4201f984": "UNI",
		"0x7fc66500c84a76ad7e9c93437bfc5ac33e2ddae9": "AAVE",
	}
	if symbol, ok := symbols[strings.ToLower(address)]; ok {
		return symbol
	}
	return ""
}

func (p *PriceAggregator) estimateGas(chainID int64, fromToken, toToken string) uint64 {
	// Estimate based on token types
	baseGas := uint64(21000) // Base transaction

	// Add gas for token transfers
	if fromToken != "0x0000000000000000000000000000000000000000" {
		baseGas += 65000 // Token transfer additional gas
	}

	// DEX-specific additions
	baseGas += 50000 // Swap operation

	return baseGas
}

// GetMultiChainPrices gets prices for multiple chains
func (p *PriceAggregator) GetMultiChainPrices(ctx context.Context, symbols []string) (map[string]*PriceData, error) {
	results := make(map[string]*PriceData)

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, symbol := range symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			price, err := p.GetRealPrice(ctx, sym)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				results[sym] = price
			}
		}(symbol)
	}

	wg.Wait()

	return results, nil
}

// GetHistoricalPrices gets historical price data
func (p *PriceAggregator) GetHistoricalPrices(ctx context.Context, symbol string, days int) ([]struct {
	Timestamp time.Time
	Price     float64
	Volume    float64
}, error) {
	// In production, fetch from CoinGecko
	// For now, return empty

	coingeckoIDs := map[string]string{
		"ETH":  "ethereum",
		"BTC":  "bitcoin",
		"USDC": "usd-coin",
		"USDT": "tether",
	}

	id, ok := coingeckoIDs[symbol]
	if !ok {
		id = symbol
	}

	url := fmt.Sprintf("%s/coins/%s/market_chart?vs_currency=usd&days=%d", p.coingeckoAPI, id, days)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API error: %d", resp.StatusCode)
	}

	var result struct {
		Prices  [][2]float64 `json:"prices"`
		Volumes [][2]float64 `json:"total_volumes"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	history := make([]struct {
		Timestamp time.Time
		Price     float64
		Volume    float64
	}, len(result.Prices))

	for i, p := range result.Prices {
		history[i] = struct {
			Timestamp time.Time
			Price     float64
			Volume    float64
		}{
			Timestamp: time.Unix(int64(p[0]/1000), 0),
			Price:     p[1],
		}
	}

	if len(result.Volumes) > 0 {
		for i, v := range result.Volumes {
			if i < len(history) {
				history[i].Volume = v[1]
			}
		}
	}

	return history, nil
}

// API Handlers

func (p *PriceAggregator) GetPriceHandler(c *gin.Context) {
	symbol := c.Param("symbol")

	price, err := p.GetRealPrice(c.Request.Context(), symbol)
	if err != nil {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, price)
}

func (p *PriceAggregator) GetPricesHandler(c *gin.Context) {
	symbols := c.QueryArray("symbols")
	if len(symbols) == 0 {
		c.JSON(400, gin.H{"error": "symbols required"})
		return
	}

	prices, err := p.GetMultiChainPrices(c.Request.Context(), symbols)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, prices)
}

func (p *PriceAggregator) GetQuoteHandler(c *gin.Context) {
	var req struct {
		ChainID   int64    `json:"chain_id" binding:"required"`
		FromToken string   `json:"from_token" binding:"required"`
		ToToken   string   `json:"to_token" binding:"required"`
		Amount    string   `json:"amount" binding:"required"`
		Sources   []string `json:"sources"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	sources := req.Sources
	if sources == nil {
		sources = []string{}
	}

	quote, err := p.GetAggregatedQuote(c.Request.Context(), req.ChainID, req.FromToken, req.ToToken, req.Amount, sources)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, quote)
}

func (p *PriceAggregator) GetHistoricalPricesHandler(c *gin.Context) {
	symbol := c.Param("symbol")
	days := 30

	if d := c.Query("days"); d != "" {
		fmt.Sscanf(d, "%d", &days)
	}

	history, err := p.GetHistoricalPrices(c.Request.Context(), symbol, days)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"symbol":  symbol,
		"history": history,
	})
}

// Helper functions
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func parseFloat(s string) float64 {
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func weiToFloat(wei *big.Int, decimals int) float64 {
	divisor := new(big.Int)
	divisor.Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)

	whole := new(big.Int).Div(wei, divisor)
	frac := new(big.Int).Mod(wei, divisor)

	fracFloat := new(big.Float).SetInt(frac)
	divisorFloat := new(big.Float).SetInt(divisor)
	fracFloat.Quo(fracFloat, divisorFloat)

	result := new(big.Float).SetInt(whole)
	result.Add(result, fracFloat)

	f, _ := result.Float64()
	return f
}
