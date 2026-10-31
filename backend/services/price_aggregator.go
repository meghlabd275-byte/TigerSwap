package services

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"tigerswap/backend/config"
	"tigerswap/backend/go/amm"
	"tigerswap/backend/go/tigerswap"
)

// PriceAggregator reads prices from TigerSwap's own on-chain oracle and quotes
// swaps against TigerSwap's own pool families. It never contacts a third-party
// exchange or price API, and it fails closed rather than inventing a price.
type PriceAggregator struct {
	redis        *redis.Client
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
		prices:       make(map[string]*PriceData),
	}
}

// GetRealPrice reads a symbol's price from TigerSwap's own on-chain oracle.
//
// There is deliberately no external price API here: the price either comes from
// the oracle TigerSwap itself runs, or the call fails. A caller that cannot get
// a price must not trade, so returning an error is the safe outcome.
func (p *PriceAggregator) GetRealPrice(ctx context.Context, symbol string) (*PriceData, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil, fmt.Errorf("symbol is required")
	}

	chainID, ok := p.defaultChainID()
	if !ok {
		return nil, fmt.Errorf("no TigerSwap chain configured; set TIGERSWAP_CHAINS")
	}

	price, err := p.PriceOnChain(ctx, chainID, sym)
	if err != nil {
		// Serve a recent cached price only when the oracle itself is
		// unreachable, and never for longer than the cache TTL.
		if cached, cerr := p.getCachedPrice(ctx, sym); cerr == nil {
			return cached, nil
		}
		return nil, err
	}

	p.cachePrice(ctx, sym, price)
	return price, nil
}

// PriceOnChain reads a symbol's price from the TigerSwap oracle on one chain.
func (p *PriceAggregator) PriceOnChain(ctx context.Context, chainID int64, symbol string) (*PriceData, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	addr, ok := config.TokenAddress(chainID, sym)
	if !ok {
		return nil, fmt.Errorf("unknown token %s on chain %d", sym, chainID)
	}

	client, err := p.tigerClient(chainID)
	if err != nil {
		return nil, err
	}

	usd, err := client.TokenPriceUSD(ctx, common.HexToAddress(addr))
	if err != nil {
		return nil, err
	}
	if usd <= 0 {
		return nil, fmt.Errorf("oracle returned a non-positive price for %s", sym)
	}

	reports, err := client.LiveOracleReports(ctx, common.HexToAddress(addr))
	if err != nil {
		return nil, err
	}

	return &PriceData{
		Symbol:      sym,
		USD:         usd,
		Sources:     []string{"tigerswap_oracle"},
		Confidence:  oracleConfidence(reports),
		LastUpdated: time.Now(),
	}, nil
}

// defaultChainID picks the chain used for symbol lookups when the caller does
// not specify one. TIGERSWAP_DEFAULT_CHAIN wins; otherwise the first configured
// chain is used, so behaviour is deterministic rather than map-order dependent.
func (p *PriceAggregator) defaultChainID() (int64, bool) {
	if raw := os.Getenv("TIGERSWAP_DEFAULT_CHAIN"); raw != "" {
		if id, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil {
			return id, true
		}
	}
	for _, d := range tigerswap.LoadDeployments() {
		return d.ChainID, true
	}
	return 0, false
}

// oracleConfidence maps the number of live oracle reports to a confidence in
// [0,1]. One report is a single source; three or more is a fully independent
// median.
func oracleConfidence(reports uint64) float64 {
	switch {
	case reports >= 3:
		return 1.0
	case reports == 2:
		return 0.8
	case reports == 1:
		return 0.5
	default:
		return 0
	}
}

func (p *PriceAggregator) cachePrice(ctx context.Context, symbol string, data *PriceData) {
	if p.redis == nil {
		return
	}
	key := fmt.Sprintf("price:%s", symbol)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return
	}
	p.redis.Set(ctx, key, jsonData, 5*time.Minute)
}

func (p *PriceAggregator) getCachedPrice(ctx context.Context, symbol string) (*PriceData, error) {
	if p.redis == nil {
		return nil, fmt.Errorf("price cache not configured")
	}
	key := fmt.Sprintf("price:%s", symbol)
	data, err := p.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("no cached price for %s", symbol)
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

	// Price impact from the oracle's own USD prices, when both tokens are known
	// on this chain. Each side is scaled by its own token's decimals: using a
	// fixed 18 would misvalue a 6-decimal token like USDC by 1e12 and report a
	// nonsense impact.
	var priceImpact float64
	fromSym, fromOK := config.TokenSymbol(chainID, fromToken)
	toSym, toOK := config.TokenSymbol(chainID, toToken)
	fromDecimals, fromDecOK := config.TokenDecimals(chainID, fromToken)
	toDecimals, toDecOK := config.TokenDecimals(chainID, toToken)
	if fromOK && toOK && fromDecOK && toDecOK {
		priceFrom, fromErr := p.PriceOnChain(ctx, chainID, fromSym)
		priceTo, toErr := p.PriceOnChain(ctx, chainID, toSym)
		if fromErr == nil && toErr == nil && priceFrom.USD > 0 && priceTo.USD > 0 {
			inputValue := weiToFloat(amountIn, fromDecimals) * priceFrom.USD
			outputValue := weiToFloat(best.out, toDecimals) * priceTo.USD
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

// getSymbolFromAddress resolves a token address to its symbol using the
// shared chain registry, so there is one source of truth for token metadata.
func (p *PriceAggregator) getSymbolFromAddress(chainID int64, address string) string {
	if symbol, ok := config.TokenSymbol(chainID, address); ok {
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

// PricePoint is one sampled price observation.
type PricePoint struct {
	Timestamp time.Time `json:"timestamp"`
	Price     float64   `json:"price"`
	Volume    float64   `json:"volume"`
}

// priceSample is the stored form of a price observation.
type priceSample struct {
	Price     float64 `json:"price"`
	Volume    float64 `json:"volume"`
	Timestamp int64   `json:"timestamp"`
}

func historyKey(symbol string) string { return "price:history:" + symbol }

// GetHistoricalPrices returns the price series TigerSwap has sampled for a
// symbol. There is no third-party history API: the series is built from the
// samples the oracle poller stored, so a caller sees only prices TigerSwap
// itself observed. With no samples it returns an empty series rather than
// fabricating one.
func (p *PriceAggregator) GetHistoricalPrices(ctx context.Context, symbol string, days int) ([]PricePoint, error) {
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	if days <= 0 {
		return nil, fmt.Errorf("days must be positive")
	}
	if p.redis == nil {
		return nil, fmt.Errorf("price history is unavailable: cache not configured")
	}

	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
	raw, err := p.redis.ZRangeByScore(ctx, historyKey(sym), &redis.ZRangeBy{
		Min: strconv.FormatInt(cutoff, 10),
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("read price history for %s: %w", sym, err)
	}

	points := make([]PricePoint, 0, len(raw))
	for _, member := range raw {
		var sample priceSample
		if err := json.Unmarshal([]byte(member), &sample); err != nil {
			continue
		}
		points = append(points, PricePoint{
			Timestamp: time.Unix(sample.Timestamp, 0),
			Price:     sample.Price,
			Volume:    sample.Volume,
		})
	}
	return points, nil
}

// RecordPriceSample appends an observed price to the symbol's history. The
// oracle poller is the only caller; it is the sole writer of price history.
func (p *PriceAggregator) RecordPriceSample(ctx context.Context, symbol string, price, volume float64) error {
	if p.redis == nil {
		return fmt.Errorf("price history is unavailable: cache not configured")
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return fmt.Errorf("symbol is required")
	}
	if price <= 0 {
		return fmt.Errorf("price must be positive")
	}

	now := time.Now()
	payload, err := json.Marshal(priceSample{Price: price, Volume: volume, Timestamp: now.Unix()})
	if err != nil {
		return err
	}
	key := historyKey(sym)
	if err := p.redis.ZAdd(ctx, key, redis.Z{Score: float64(now.Unix()), Member: string(payload)}).Err(); err != nil {
		return err
	}
	// Bound the window so history cannot grow without limit.
	cutoff := now.Add(-90 * 24 * time.Hour).Unix()
	return p.redis.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(cutoff, 10)).Err()
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
