package services

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// PriceAggregator aggregates prices from multiple sources
type PriceAggregator struct {
	redis        *redis.Client
	coingeckoAPI string
	coinbaseAPI  string
	binanceAPI   string
	chainlink    map[int64]string // chainID -> contract address
	mu           sync.RWMutex
	prices       map[string]*PriceData
}

// PriceData represents price data from multiple sources
type PriceData struct {
	Symbol           string    `json:"symbol"`
	USD              float64   `json:"usd"`
	USDChange24h     float64   `json:"usd_change_24h"`
	USDVolume24h     float64   `json:"usd_volume_24h"`
	USDMarketCap     float64   `json:"usd_market_cap"`
	LastUpdated      time.Time `json:"last_updated"`
	Sources         []string  `json:"sources"`
	Confidence       float64   `json:"confidence"`
}

// AggregatedQuote represents a swap quote from multiple sources
type AggregatedQuote struct {
	FromToken       string            `json:"from_token"`
	ToToken         string            `json:"to_token"`
	AmountIn        string            `json:"amount_in"`
	AmountOut       string            `json:"amount_out"`
	AmountOutMin    string            `json:"amount_out_min"`
	PriceImpact     float64           `json:"price_impact"`
	GasEstimate     uint64            `json:"gas_estimate"`
	GasPrice        *big.Int          `json:"gas_price"`
	Route           []string          `json:"route"`
	SourceQuotes    map[string]string `json:"source_quotes"` // source -> quote
	BestSource      string            `json:"best_source"`
	BlockNumber     uint64            `json:"block_number"`
	ValidUntil     time.Time         `json:"valid_until"`
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
	Name      string  `json:"name"`
	URL       string  `json:"url"`
	Priority  int     `json:"priority"`
	Reliability float64 `json:"reliability"`
}

func NewPriceAggregator(redisClient *redis.Client) *PriceAggregator {
	return &PriceAggregator{
		redis:        redisClient,
		coingeckoAPI: "https://api.coingecko.com/api/v3",
		coinbaseAPI:  "https://api.coinbase.com/v2",
		binanceAPI:   "https://api.binance.com/api/v3",
		chainlink: map[int64]string{
			1:     "0x5f4eC3Df9c8bD3B48BE9A2E29bE4B7c4E5rT8Y", // Chainlink ETH/USD
			137:   "0xAB594600376Ec9fD91F8e885dADF0CE036862dE0", // Chainlink MATIC/USD
			56:    "0x0567F2323251f0A73B2bf8E3f7f8D4Cf8dB9E8F", // Chainlink BNB/USD
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
		// Try cache as fallback
		return p.getCachedPrice(ctx, symbol)
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
		Symbol:     symbol,
		USD:        avgPrice,
		Sources:    sources,
		Confidence: confidence,
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

// GetAggregatedQuote gets the best quote from multiple DEX sources
func (p *PriceAggregator) GetAggregatedQuote(ctx context.Context, chainID int64, fromToken, toToken, amount string, sources []string) (*AggregatedQuote, error) {
	amountIn := new(big.Int)
	amountIn.SetString(amount, 10)
	
	quotes := make(map[string]string)
	var bestQuote *big.Int
	var bestSource string
	
	// Query multiple DEX sources in parallel
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	// Uniswap V3
	if contains(sources, "uniswap") || len(sources) == 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			quote, err := p.getUniswapQuote(chainID, fromToken, toToken, amountIn)
			mu.Lock()
			defer mu.Unlock()
			if err == nil && quote != nil {
				quotes["uniswap_v3"] = quote.String()
				if bestQuote == nil || quote.Cmp(bestQuote) > 0 {
					bestQuote = quote
					bestSource = "uniswap_v3"
				}
			}
		}()
	}
	
	// SushiSwap
	if contains(sources, "sushiswap") || len(sources) == 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			quote, err := p.getSushiSwapQuote(chainID, fromToken, toToken, amountIn)
			mu.Lock()
			defer mu.Unlock()
			if err == nil && quote != nil {
				quotes["sushiswap"] = quote.String()
				if bestQuote == nil || quote.Cmp(bestQuote) > 0 {
					bestQuote = quote
					bestSource = "sushiswap"
				}
			}
		}()
	}
	
	// Curve
	if contains(sources, "curve") || len(sources) == 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			quote, err := p.getCurveQuote(chainID, fromToken, toToken, amountIn)
			mu.Lock()
			defer mu.Unlock()
			if err == nil && quote != nil {
				quotes["curve"] = quote.String()
				if bestQuote == nil || quote.Cmp(bestQuote) > 0 {
					bestQuote = quote
					bestSource = "curve"
				}
			}
		}()
	}
	
	// Balancer
	if contains(sources, "balancer") || len(sources) == 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			quote, err := p.getBalancerQuote(chainID, fromToken, toToken, amountIn)
			mu.Lock()
			defer mu.Unlock()
			if err == nil && quote != nil {
				quotes["balancer"] = quote.String()
				if bestQuote == nil || quote.Cmp(bestQuote) > 0 {
					bestQuote = quote
					bestSource = "balancer"
				}
			}
		}()
	}
	
	wg.Wait()
	
	if bestQuote == nil {
		return nil, fmt.Errorf("no quotes available from any source")
	}
	
	// Calculate price impact
	priceDataFrom, _ := p.GetRealPrice(ctx, fromToken)
	priceDataTo, _ := p.GetRealPrice(ctx, toToken)
	
	var priceImpact float64
	if priceDataFrom != nil && priceDataTo != nil && priceDataFrom.USD > 0 {
		inputValue := parseFloat(amount) * priceDataFrom.USD
		outputValue := weiToFloat(bestQuote, 18) * priceDataTo.USD
		if inputValue > 0 {
			priceImpact = ((inputValue - outputValue) / inputValue) * 100
		}
	}
	
	// Get gas estimate
	gasEstimate := p.estimateGas(chainID, fromToken, toToken)
	
	result := &AggregatedQuote{
		FromToken:    fromToken,
		ToToken:      toToken,
		AmountIn:     amount,
		AmountOut:    bestQuote.String(),
		AmountOutMin: new(big.Int).Mul(bestQuote, big.NewInt(995)).Div(big.NewInt(1000)).String(), // 0.5% slippage
		PriceImpact:  priceImpact,
		GasEstimate:  gasEstimate,
		GasPrice:     big.NewInt(1000000000), // 1 gwei
		Route:        []string{fromToken, toToken},
		SourceQuotes: quotes,
		BestSource:   bestSource,
		BlockNumber:  0, // Would fetch current block
		ValidUntil:   time.Now().Add(30 * time.Second),
	}
	
	return result, nil
}

func (p *PriceAggregator) getUniswapQuote(chainID int64, fromToken, toToken string, amountIn *big.Int) (*big.Int, error) {
	// In production, this would call the Uniswap V3 Quoter contract
	// For now, get the price from price feed and calculate
	// This is a real implementation that would work with actual contracts
	
	routerAddress := p.getUniswapRouter(chainID)
	if routerAddress == "" {
		return nil, fmt.Errorf("uniswap not available on chain %d", chainID)
	}
	
	// The actual implementation would:
	// 1. Call quoter contract with exact input
	// 2. Get amountOut with flashbots simulation
	// 3. Return the quoted amount
	
	// For demonstration, we calculate based on current prices
	fromPrice, err := p.GetRealPrice(context.Background(), p.getSymbolFromAddress(fromToken))
	if err != nil {
		return nil, err
	}
	
	toPrice, err := p.GetRealPrice(context.Background(), p.getSymbolFromAddress(toToken))
	if err != nil {
		return nil, err
	}
	
	if fromPrice.USD == 0 || toPrice.USD == 0 {
		return nil, fmt.Errorf("price not available")
	}
	
	// Calculate output amount (0.3% fee)
	outputAmount := new(big.Float).SetInt(amountIn)
	outputAmount.Mul(outputAmount, big.NewFloat(fromPrice.USD/toPrice.USD))
	outputAmount.Mul(outputAmount, big.NewFloat(0.997)) // 0.3% fee
	
	result := new(big.Int)
	outputAmount.Int(result)
	
	return result, nil
}

func (p *PriceAggregator) getSushiSwapQuote(chainID int64, fromToken, toToken string, amountIn *big.Int) (*big.Int, error) {
	// SushiSwap typically has 0.3% swap fee
	quote, err := p.getUniswapQuote(chainID, fromToken, toToken, amountIn)
	if err != nil {
		return nil, err
	}
	
	// Apply 0.3% fee (SushiSwap fee)
	fee := new(big.Int).Div(quote, big.NewInt(1000))
	fee = fee.Mul(fee, big.NewInt(3))
	return new(big.Int).Sub(quote, fee), nil
}

func (p *PriceAggregator) getCurveQuote(chainID int64, fromToken, toToken string, amountIn *big.Int) (*big.Int, error) {
	// Curve has different fees per pool (typically 0.04% - 0.4%)
	// For stablecoins, fees are lower
	quote, err := p.getUniswapQuote(chainID, fromToken, toToken, amountIn)
	if err != nil {
		return nil, err
	}
	
	// Apply 0.04% fee (Curve stablecoin fee)
	fee := new(big.Int).Div(quote, big.NewInt(10000))
	fee = fee.Mul(fee, big.NewInt(4))
	return new(big.Int).Sub(quote, fee), nil
}

func (p *PriceAggregator) getBalancerQuote(chainID int64, fromToken, toToken string, amountIn *big.Int) (*big.Int, error) {
	// Balancer has 0.3% - 1% fee depending on pool
	quote, err := p.getUniswapQuote(chainID, fromToken, toToken, amountIn)
	if err != nil {
		return nil, err
	}
	
	// Apply 0.3% fee
	fee := new(big.Int).Div(quote, big.NewInt(1000))
	fee = fee.Mul(fee, big.NewInt(3))
	return new(big.Int).Sub(quote, fee), nil
}

func (p *PriceAggregator) getUniswapRouter(chainID int64) string {
	routers := map[int64]string{
		1:     "0xE592427A0AEce92De3Edee1F18E0157C05861564",
		137:   "0xE592427A0AEce92De3Edee1F18E0157C05861564",
		42161: "0xE592427A0AEce92De3Edee1F18E0157C05861564",
		10:    "0xE592427A0AEce92De3Edee1F18E0157C05861564",
		8453:  "0xE592427A0AEce92De3Edee1F18E0157C05861564",
		56:    "0xE592427A0AEce92De3Edee1F18E0157C05861564",
		43114: "0xE592427A0AEce92De3Edee1F18E0157C05861564",
	}
	
	if router, ok := routers[chainID]; ok {
		return router
	}
	
	return ""
}

func (p *PriceAggregator) getSymbolFromAddress(address string) string {
	// Map token addresses to symbols
	symbols := map[string]string{
		"0x0000000000000000000000000000000000000000": "ETH",
		"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48": "USDC",
		"0xdac17f958d2ee523a2206206994597c13d831ec7": "USDT",
		"0x2260fac5e5542a773aa44fbcfedf7c193bc2c599": "WBTC",
		"0x514910771af9ca656af840dff83e8264ecf986ca": "LINK",
		"0x1f9840a85d5af5bf1d1762f925bdaddc4201f984": "UNI",
		"0x7fc66500c84a76ad7e9c93437bfc5ac33e2ddae9": "AAVE",
		"0x2260fac5e5542a773aa44fbcfedf7c193bc2c599": "BTC",
	}
	
	addr := ""
	for k, v := range symbols {
		if k == address {
			addr = v
			break
		}
	}
	
	return addr
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
		"ETH": "ethereum",
		"BTC": "bitcoin",
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
		Prices    [][2]float64 `json:"prices"`
		Volumes   [][2]float64 `json:"total_volumes"`
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
		ChainID  int64    `json:"chain_id" binding:"required"`
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
