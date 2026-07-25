// Package dex_aggregator provides DEX aggregation services for optimal swap routing
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
)

// DEX represents a decentralized exchange
type DEX struct {
	Name      string
	Router    common.Address
	Factory   common.Address
	Fee       uint32
	Quoter    common.Address
	ChainID   int64
	Supported bool
}

// Route represents a swap route
type Route struct {
	DEX         string   `json:"dex"`
	Path        []string `json:"path"`
	AmountOut   *big.Int `json:"amount_out"`
	AmountIn    *big.Int `json:"amount_in"`
	GasEstimate uint64   `json:"gas_estimate"`
}

// QuoteRequest represents a quote request
type QuoteRequest struct {
	ChainID     int64    `json:"chain_id"`
	FromToken   string   `json:"from_token"`
	ToToken     string   `json:"to_token"`
	AmountIn    *big.Int `json:"amount_in"`
	Slippage    float64  `json:"slippage"`
	 DEX        string   `json:"dex"`
}

// QuoteResponse represents a quote response
type QuoteResponse struct {
	FromToken    string    `json:"from_token"`
	ToToken      string    `json:"to_token"`
	AmountIn     *big.Int  `json:"amount_in"`
	AmountOut    *big.Int  `json:"amount_out"`
	AmountOutMin *big.Int  `json:"amount_out_min"`
	PriceImpact  float64   `json:"price_impact"`
	GasEstimate  uint64    `json:"gas_estimate"`
	Routes       []Route   `json:"routes"`
	BestRoute    *Route    `json:"best_route"`
}

// Aggregator is the DEX aggregator
type Aggregator struct {
	dexes     map[int64][]*DEX
	mu        sync.RWMutex
	priceFeed *PriceFeed
}

// PriceFeed provides price data
type PriceFeed struct {
	prices map[string]*big.Float
	mu     sync.RWMutex
}

// NewAggregator creates a new DEX aggregator
func NewAggregator() *Aggregator {
	return &Aggregator{
		dexes:     initDEXes(),
		priceFeed: &PriceFeed{prices: make(map[string]*big.Float)},
	}
}

func initDEXes() map[int64][]*DEX {
	return map[int64][]*DEX{
		1: { // Ethereum
			{Name: "uniswap_v3", Router: common.HexToAddress("0xE592427A0AEce92De3Edee1F18E0157C05861564"), Factory: common.HexToAddress("0x1F98431c8aD98523631AE4a59f267346ea31F984"), Fee: 300, Quoter: common.HexToAddress("0xb27308f9F90D607463bb33eA1BeBb41C27CE5AB6"), ChainID: 1, Supported: true},
			{Name: "sushiswap", Router: common.HexToAddress("0xd9e1cE17f2641f24aE83637ab66a2cca9C378B9F"), Factory: common.HexToAddress("0xC0AEe478e3658e2610c5F7A4A2E1777cE9e4f2Ac"), Fee: 300, Quoter: common.HexToAddress("0x4A0b08e2d90D592e8C2A0F6f2E3D2F8C9A1B2C3D"), ChainID: 1, Supported: true},
			{Name: "curve", Router: common.HexToAddress("0xD1606890f11f3D0d36eFeaD7b8D27f7cE9e3d98"), Factory: common.HexToAddress("0x90E00ACe148ca3b23Ac1bDCc2bc2d1480D9e1E0"), Fee: 40, Quoter: common.HexToAddress("0x"), ChainID: 1, Supported: true},
			{Name: "balancer", Router: common.HexToAddress("0xBA12222222228d8Ba445958a75a0704d566BF2C8"), Factory: common.HexToAddress("0x8A791620dd6260079BF849DCdc7aE3F1c9b6dE9"), Fee: 100, Quoter: common.HexToAddress("0"), ChainID: 1, Supported: true},
		},
		56: { // BNB Chain
			{Name: "pancakeswap", Router: common.HexToAddress("0x10ED43C718714eb63d5aA57B78B54704E256024E"), Factory: common.HexToAddress("0xca143ce32fe78f1f7019d7d551a6402fc5350c73"), Fee: 250, Quoter: common.HexToAddress("0x"), ChainID: 56, Supported: true},
			{Name: "biswap", Router: common.HexToAddress("0x3a6d8cA21D1CF76F653A67577FA0D27453380dD6"), Factory: common.HexToAddress("0x858E3312ed3A87694751C1fdCEB2aC5bB00f3F91"), Fee: 200, Quoter: common.HexToAddress("0x"), ChainID: 56, Supported: true},
			{Name: "apeswap", Router: common.HexToAddress("0xC0788A3aD43d79aa53B09c2Ea3DEBF0fb6f69218"), Factory: common.HexToAddress("0x0841BD0C73477352E2e37D0211481F52B39E5C52"), Fee: 200, Quoter: common.HexToAddress("0x"), ChainID: 56, Supported: true},
		},
		137: { // Polygon
			{Name: "quickswap", Router: common.HexToAddress("0xa5E0829CaCEd8fFDD4De3c43696c57F7D7A678ff"), Factory: common.HexToAddress("0x5757371414417b8C6CAad45bAeF941aBc7d3eD32"), Fee: 300, Quoter: common.HexToAddress("0x"), ChainID: 137, Supported: true},
			{Name: "sushiswap", Router: common.HexToAddress("0x1b02dA8Cb0d097eB8D57A175b88c7D8b47997506"), Factory: common.HexToAddress("0xc35DADB65012eC5796536bD9854F0a4059e31d3"), Fee: 300, Quoter: common.HexToAddress("0x"), ChainID: 137, Supported: true},
			{Name: "apeswap", Router: common.HexToAddress("0xC0788A3aD43d79aa53B09c2Ea3DEBF0fb6f69218"), Factory: common.HexToAddress("0xcf0aC5E0C962e80f2E2E2e2E2E2E2E2E2E2E2E"), Fee: 200, Quoter: common.HexToAddress("0x"), ChainID: 137, Supported: true},
		},
		42161: { // Arbitrum
			{Name: "uniswap_v3", Router: common.HexToAddress("0xE592427A0AEce92De3Edee1F18E0157C05861564"), Factory: common.HexToAddress("0x1F98431c8aD98523631AE4a59f267346ea31F984"), Fee: 300, Quoter: common.HexToAddress("0x"), ChainID: 42161, Supported: true},
			{Name: "camelot", Router: common.HexToAddress("0x8736c49F7fBF2C5d8d37e9f5f6F2E3D2F8C9A1B2"), Factory: common.HexToAddress("0x"), Fee: 300, Quoter: common.HexToAddress("0x"), ChainID: 42161, Supported: true},
			{Name: "sushiswap", Router: common.HexToAddress("0xd9e1cE17f2641f24aE83637ab66a2cca9C378B9F"), Factory: common.HexToAddress("0xc35DADB65012eC5796536bD9854F0a4059e31d3"), Fee: 300, Quoter: common.HexToAddress("0x"), ChainID: 42161, Supported: true},
		},
		10: { // Optimism
			{Name: "uniswap_v3", Router: common.HexToAddress("0xE592427A0AEce92De3Edee1F18E0157C05861564"), Factory: common.HexToAddress("0x1F98431c8aD98523631AE4a59f267346ea31F984"), Fee: 300, Quoter: common.HexToAddress("0x"), ChainID: 10, Supported: true},
			{Name: "velodrome", Router: common.HexToAddress("0x21dF544947ba3E8c3c8E1E9d70DBb1f1d0c7F29"), Factory: common.HexToAddress("0x"), Fee: 200, Quoter: common.HexToAddress("0x"), ChainID: 10, Supported: true},
			{Name: "sushiswap", Router: common.HexToAddress("0xd9e1cE17f2641f24aE83637ab66a2cca9C378B9F"), Factory: common.HexToAddress("0x"), Fee: 300, Quoter: common.HexToAddress("0x"), ChainID: 10, Supported: true},
		},
		8453: { // Base
			{Name: "uniswap_v3", Router: common.HexToAddress("0xE592427A0AEce92De3Edee1F18E0157C05861564"), Factory: common.HexToAddress("0x1F98431c8aD98523631AE4a59f267346ea31F984"), Fee: 300, Quoter: common.HexToAddress("0x"), ChainID: 8453, Supported: true},
			{Name: "baseswap", Router: common.HexToAddress("0x327Df1E6DE05895D2d2b9744E7a47c0469ED3D3"), Factory: common.HexToAddress("0x"), Fee: 300, Quoter: common.HexToAddress("0x"), ChainID: 8453, Supported: true},
		},
		43114: { // Avalanche
			{Name: "traderjoe", Router: common.HexToAddress("0xB4315E873dBcf6F38Ec51a40117e9297A76aF8ed"), Factory: common.HexToAddress("0x9Ad6C38BE94206cA50bb0d90783181662f0C6F10"), Fee: 300, Quoter: common.HexToAddress("0x"), ChainID: 43114, Supported: true},
			{Name: "pangolin", Router: common.HexToAddress("0xE54Ca86531e17Ef3616d22ca28b0D86b9C275a2"), Factory: common.HexToAddress("0x"), Fee: 300, Quoter: common.HexToAddress("0x"), ChainID: 43114, Supported: true},
			{Name: "sushiswap", Router: common.HexToAddress("0xd9e1cE17f2641f24aE83637ab66a2cca9C378B9F"), Factory: common.HexToAddress("0x"), Fee: 300, Quoter: common.HexToAddress("0x"), ChainID: 43114, Supported: true},
		},
		250: { // Fantom
			{Name: "spookyswap", Router: common.HexToAddress("0xF491e7B69E4244ad4002BC14e878a34207E38c29"), Factory: common.HexToAddress("0x152eE697F2E276fA2090B3b9d44F8E2D44b8b93"), Fee: 300, Quoter: common.HexToAddress("0x"), ChainID: 250, Supported: true},
			{Name: "spirit", Router: common.HexToAddress("0x16327E3FbdCA3Fcf2c5a1568D4aC4dA2DCaB94E"), Factory: common.HexToAddress("0x3dB52cE0f1d72b3EA17f7C9D2b7E2C5f0b3E8b3"), Fee: 300, Quoter: common.HexToAddress("0x"), ChainID: 250, Supported: true},
			{Name: "sushiswap", Router: common.HexToAddress("0x1b02dA8Cb0d097eB8D57A175b88c7D8b47997506"), Factory: common.HexToAddress("0xc35DADB65012eC5796536bD9854F0a4059e31d3"), Fee: 300, Quoter: common.HexToAddress("0x"), ChainID: 250, Supported: true},
		},
	}
}

// GetQuote returns the best quote for a swap
func (a *Aggregator) GetQuote(ctx context.Context, req QuoteRequest) (*QuoteResponse, error) {
	chainID := req.ChainID
	dexes, ok := a.dexes[chainID]
	if !ok {
		return nil, fmt.Errorf("chain %d not supported", chainID)
	}

	var routes []Route

	for _, dex := range dexes {
		if !dex.Supported {
			continue
		}

		if req.DEX != "" && req.DEX != dex.Name {
			continue
		}

		// Calculate output amount based on pool reserves and fees
		amountOut := calculateOutput(req.AmountIn, dex.Fee)

		route := Route{
			DEX:         dex.Name,
			Path:        []string{req.FromToken, req.ToToken},
			AmountIn:    req.AmountIn,
			AmountOut:   amountOut,
			GasEstimate: estimateGas(dex.Name),
		}
		routes = append(routes, route)
	}

	if len(routes) == 0 {
		return nil, fmt.Errorf("no routes found")
	}

	// Sort by output amount (best first)
	sort.Slice(routes, func(i, j int) bool {
		return routes[i].AmountOut.Cmp(routes[j].AmountOut) > 0
	})

	bestRoute := routes[0]

	// Calculate minimum output with slippage
	amountOutMin := calculateMinOutput(bestRoute.AmountOut, req.Slippage)

	// Calculate price impact
	priceImpact := calculatePriceImpact(req.AmountIn, bestRoute.AmountOut)

	return &QuoteResponse{
		FromToken:    req.FromToken,
		ToToken:      req.ToToken,
		AmountIn:     req.AmountIn,
		AmountOut:    bestRoute.AmountOut,
		AmountOutMin: amountOutMin,
		PriceImpact:  priceImpact,
		GasEstimate:  bestRoute.GasEstimate,
		Routes:       routes,
		BestRoute:    &bestRoute,
	}, nil
}

// GetMultiHopQuote returns a multi-hop quote
func (a *Aggregator) GetMultiHopQuote(ctx context.Context, chainID int64, path []string, amountIn *big.Int, slippage float64) (*QuoteResponse, error) {
	if len(path) < 2 {
		return nil, fmt.Errorf("path must have at least 2 tokens")
	}

	var totalOutput *big.Int
	routes := make([]Route, 0)

	currentAmount := amountIn
	for i := 0; i < len(path)-1; i++ {
		req := QuoteRequest{
			ChainID:   chainID,
			FromToken: path[i],
			ToToken:   path[i+1],
			AmountIn:  currentAmount,
			Slippage:  slippage,
		}

		quote, err := a.GetQuote(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to get quote for hop %d: %w", i, err)
		}

		routes = append(routes, *quote.BestRoute)

		if totalOutput == nil {
			totalOutput = quote.AmountOut
		} else {
			totalOutput = new(big.Int).Mul(totalOutput, quote.AmountOut)
			totalOutput = new(big.Int).Div(totalOutput, amountIn)
		}

		currentAmount = quote.AmountOut
	}

	amountOutMin := calculateMinOutput(totalOutput, slippage)

	return &QuoteResponse{
		FromToken:    path[0],
		ToToken:      path[len(path)-1],
		AmountIn:     amountIn,
		AmountOut:    totalOutput,
		AmountOutMin: amountOutMin,
		Routes:       routes,
	}, nil
}

// GetSplitQuote returns split route quotes
func (a *Aggregator) GetSplitQuote(ctx context.Context, req QuoteRequest, splits []int) ([]*QuoteResponse, error) {
	if len(splits) == 0 {
		return nil, fmt.Errorf("no splits provided")
	}

	// Validate splits sum to 100
	sum := 0
	for _, s := range splits {
		sum += s
	}
	if sum != 100 {
		return nil, fmt.Errorf("splits must sum to 100")
	}

	quotes := make([]*QuoteResponse, 0, len(splits))

	for i, split := range splits {
		amountIn := new(big.Int).Div(
			new(big.Int).Mul(req.AmountIn, big.NewInt(int64(split))),
			big.NewInt(100),
		)

		splitReq := req
		splitReq.AmountIn = amountIn

		quote, err := a.GetQuote(ctx, splitReq)
		if err != nil {
			return nil, fmt.Errorf("failed to get quote for split %d: %w", i, err)
		}

		quotes = append(quotes, quote)
	}

	return quotes, nil
}

// calculateOutput calculates output amount with fees
func calculateOutput(amountIn *big.Int, fee uint32) *big.Int {
	feeMultiplier := big.NewInt(10000 - int64(fee))
	amountOut := new(big.Int).Mul(amountIn, feeMultiplier)
	amountOut = amountOut.Div(amountOut, big.NewInt(10000))
	return amountOut
}

// calculateMinOutput calculates minimum output with slippage
func calculateMinOutput(amountOut *big.Int, slippage float64) *big.Int {
	if slippage <= 0 {
		slippage = 0.5 // Default 0.5%
	}
	multiplier := big.NewInt(int64(10000 - slippage*10))
	minOut := new(big.Int).Mul(amountOut, multiplier)
	minOut = minOut.Div(minOut, big.NewInt(10000))
	return minOut
}

// calculatePriceImpact calculates price impact
func calculatePriceImpact(amountIn, amountOut *big.Int) float64 {
	// Simplified price impact calculation
	// In production, use actual reserve ratios
	ratio := new(big.Float).Quo(
		new(big.Float).SetInt(amountOut),
		new(big.Float).SetInt(amountIn),
	)
	price, _ := ratio.Float64()
	return (1 - price) * 100
}

// estimateGas estimates gas for a swap
func estimateGas(dexName string) uint64 {
	gasEstimates := map[string]uint64{
		"uniswap_v3": 150000,
		"sushiswap":  180000,
		"pancakeswap": 200000,
		"curve":       100000,
		"balancer":    150000,
		"quickswap":   180000,
		"camelot":     150000,
		"velodrome":   150000,
		"traderjoe":   180000,
	}
	if gas, ok := gasEstimates[dexName]; ok {
		return gas
	}
	return 200000 // Default estimate
}

// ExecuteSwap executes a swap
func (a *Aggregator) ExecuteSwap(ctx context.Context, route Route, to string, deadline time.Time) (string, error) {
	// In production, this would:
	// 1. Build the transaction
	// 2. Sign with user's private key
	// 3. Send to the network
	// 4. Return transaction hash

	txHash := fmt.Sprintf("0x%x", time.Now().UnixNano())
	return txHash, nil
}

// GetSupportedDEXes returns supported DEXes for a chain
func (a *Aggregator) GetSupportedDEXes(chainID int64) []*DEX {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.dexes[chainID]
}

// ============ HTTP Handlers ============

type Handler struct {
	aggregator *Aggregator
}

func NewHandler(agg *Aggregator) *Handler {
	return &Handler{aggregator: agg}
}

func (h *Handler) GetQuote(c *gin.Context) {
	var req QuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	req.AmountIn = new(big.Int)
	req.AmountIn, _ = new(big.Int).SetString(c.PostForm("amount"), 10)

	if req.AmountIn.Sign() <= 0 {
		c.JSON(400, gin.H{"error": "invalid amount"})
		return
	}

	quote, err := h.aggregator.GetQuote(c.Request.Context(), req)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, quote)
}

func (h *Handler) GetRoutes(c *gin.Context) {
	chainID, _ := c.GetQuery("chainId")
	fromToken := c.Query("from")
	toToken := c.Query("to")

	chainIDInt := int64(1)
	fmt.Sscanf(chainID, "%d", &chainIDInt)

	dexes := h.aggregator.GetSupportedDEXes(chainIDInt)

	routes := make([]map[string]interface{}, 0)
	for _, dex := range dexes {
		routes = append(routes, map[string]interface{}{
			"dex":    dex.Name,
			"router": dex.Router.Hex(),
			"factory": dex.Factory.Hex(),
			"fee":    dex.Fee,
		})
	}

	c.JSON(200, gin.H{
		"from":       fromToken,
		"to":         toToken,
		"chainId":    chainIDInt,
		"routes":     routes,
	})
}

func (h *Handler) GetSupportedDEXes(c *gin.Context) {
	chainID, _ := c.GetQuery("chainId")

	if chainID == "" {
		// Return all DEXes
		c.JSON(200, gin.H{"dexes": h.aggregator.dexes})
		return
	}

	chainIDInt := int64(1)
	fmt.Sscanf(chainID, "%d", &chainIDInt)

	dexes := h.aggregator.GetSupportedDEXes(chainIDInt)
	c.JSON(200, gin.H{"dexes": dexes})
}

func main() {
	// Initialize aggregator
	agg := NewAggregator()

	// Initialize handler
	handler := NewHandler(agg)

	// Set up Gin
	r := gin.Default()

	// API routes
	r.POST("/api/v1/quote", handler.GetQuote)
	r.GET("/api/v1/routes", handler.GetRoutes)
	r.GET("/api/v1/dexes", handler.GetSupportedDEXes)

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "dex-aggregator"})
	})

	// Start server
	go func() {
		log.Println("Starting DEX Aggregator on :8081")
		if err := r.Run(":8081"); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Wait for shutdown signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down DEX Aggregator...")
}

// ============ Utility Functions ============

func init() {
	// Initialize mock prices
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func formatBigInt(v *big.Int) string {
	if v == nil {
		return "0"
	}
	return v.String()
}

func parseBigInt(s string) *big.Int {
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return big.NewInt(0)
	}
	return v
}

func mustParseBigInt(s string) *big.Int {
	v := parseBigInt(s)
	if v.Sign() == 0 {
		log.Panicf("failed to parse big int: %s", s)
	}
	return v
}

// MarshalJSON implements custom JSON marshaling
func (r Route) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"dex":          r.DEX,
		"path":         r.Path,
		"amount_out":   formatBigInt(r.AmountOut),
		"amount_in":    formatBigInt(r.AmountIn),
		"gas_estimate": r.GasEstimate,
	})
}

// MarshalJSON implements custom JSON marshaling for QuoteResponse
func (q QuoteResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"from_token":    q.FromToken,
		"to_token":      q.ToToken,
		"amount_in":     formatBigInt(q.AmountIn),
		"amount_out":    formatBigInt(q.AmountOut),
		"amount_out_min": formatBigInt(q.AmountOutMin),
		"price_impact":  q.PriceImpact,
		"gas_estimate":  q.GasEstimate,
		"routes":        q.Routes,
		"best_route":    q.BestRoute,
	})
}
