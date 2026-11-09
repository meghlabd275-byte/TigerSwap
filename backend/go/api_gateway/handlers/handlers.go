// Package handlers provides HTTP handlers for the API Gateway
package handlers

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"tigerswap/backend/go/api_gateway/config"
	"tigerswap/backend/go/api_gateway/middleware"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

// Handler base struct
type Handler struct {
	config *config.Config
}

// NewHandler creates a new handler
func NewHandler(cfg *config.Config) *Handler {
	return &Handler{config: cfg}
}

// HealthCheck returns health check handler
func HealthCheck() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"service":   "tigerswap-api-gateway",
			"version":   "1.0.0",
		})
	}
}

// ReadyCheck returns ready check handler
func ReadyCheck(redisClient *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		if redisClient != nil {
			if err := redisClient.Ping(ctx).Err(); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"status": "not_ready",
					"error":  "Redis not available",
				})
				return
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "ready",
			"redis":  "connected",
		})
	}
}

// SwapHandler handles swap-related requests
type SwapHandler struct {
	*Handler
}

// NewSwapHandler creates a new swap handler
func NewSwapHandler(cfg *config.Config) *SwapHandler {
	return &SwapHandler{NewHandler(cfg)}
}

// QuoteRequest represents a quote request
type QuoteRequest struct {
	ChainID     int64  `json:"chain_id" binding:"required"`
	FromToken   string `json:"from_token" binding:"required"`
	ToToken     string `json:"to_token" binding:"required"`
	AmountIn    string `json:"amount_in" binding:"required"`
	SlippageBps int    `json:"slippage_bps"`
}

// QuoteResponse represents a quote response
// aggregatorQuote mirrors the TigerAggregator's quote response.
//
// The aggregator encodes its amounts as `*big.Int`, which JSON renders as a
// number, so they are decoded with json.Number. Decoding them into a string
// fails outright and would make every quote request return a 500.
type aggregatorQuote struct {
	FromToken    string      `json:"from_token"`
	ToToken      string      `json:"to_token"`
	AmountIn     json.Number `json:"amount_in"`
	AmountOut    json.Number `json:"amount_out"`
	AmountOutMin json.Number `json:"amount_out_min"`
	PriceImpact  float64     `json:"price_impact"`
	GasEstimate  uint64      `json:"gas_estimate"`
	BestRoute    *routeInfo  `json:"best_route"`
}

// routeInfo is one evaluated route from the aggregator.
type routeInfo struct {
	Family      string      `json:"family"`
	Path        []string    `json:"path"`
	AmountIn    json.Number `json:"amount_in"`
	AmountOut   json.Number `json:"amount_out"`
	FeeBps      uint32      `json:"fee_bps"`
	GasEstimate uint64      `json:"gas_estimate"`
}

// QuoteResponse is the quote shape returned to the browser client. Amounts stay
// as decimal strings so integer base units survive JSON without precision loss.
type QuoteResponse struct {
	AmountIn     string   `json:"amountIn"`
	AmountOut    string   `json:"amountOut"`
	AmountOutMin string   `json:"amountOutMin"`
	PriceImpact  float64  `json:"priceImpact"`
	Route        []string `json:"route"`
	GasEstimate  uint64   `json:"gasEstimate"`
}

// GetQuote proxies a quote from TigerAggregator, TigerSwap's own router.
//
// The client speaks camelCase with basis-point slippage; the aggregator speaks
// snake_case with a fractional slippage and integer amounts. The translation
// happens here so neither side has to know the other's spelling.
func (h *SwapHandler) GetQuote(c *gin.Context) {
	var req QuoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	amountIn, ok := new(big.Int).SetString(req.AmountIn, 10)
	if !ok || amountIn.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount_in must be a positive integer string"})
		return
	}

	slippageBps := req.SlippageBps
	if slippageBps <= 0 {
		slippageBps = 50
	}
	if slippageBps > 10000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slippage_bps must not exceed 10000"})
		return
	}

	payload := map[string]interface{}{
		"chain_id":   req.ChainID,
		"from_token": req.FromToken,
		"to_token":   req.ToToken,
		"amount_in":  amountIn,
		"slippage":   float64(slippageBps) / 10000.0,
	}

	url := fmt.Sprintf("%s/api/v1/quote", h.config.Upstream.DEXAggregator)
	resp, err := callUpstreamService(c, url, payload)
	if err != nil {
		respondUpstreamUnavailable(c, "TigerAggregator", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": "TigerAggregator could not price this swap"})
		return
	}

	var quote aggregatorQuote
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "TigerAggregator returned a malformed quote"})
		return
	}

	route := []string{quote.FromToken, quote.ToToken}
	if quote.BestRoute != nil && len(quote.BestRoute.Path) > 0 {
		route = quote.BestRoute.Path
	}

	c.JSON(http.StatusOK, QuoteResponse{
		AmountIn:     quote.AmountIn.String(),
		AmountOut:    quote.AmountOut.String(),
		AmountOutMin: quote.AmountOutMin.String(),
		PriceImpact:  quote.PriceImpact,
		Route:        route,
		GasEstimate:  quote.GasEstimate,
	})
}

// ExecuteSwap handles swap execution requests
func (h *SwapHandler) ExecuteSwap(c *gin.Context) {
	var req struct {
		ChainID     int64  `json:"chain_id" binding:"required"`
		FromToken   string `json:"from_token" binding:"required"`
		ToToken     string `json:"to_token" binding:"required"`
		Amount      string `json:"amount" binding:"required"`
		Slippage    string `json:"slippage"`
		FromAddress string `json:"from_address" binding:"required"`
		GasPrice    string `json:"gas_price"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	// Call DEX aggregator to execute swap
	url := fmt.Sprintf("%s/api/v1/execute", h.config.Upstream.DEXAggregator)
	resp, err := callUpstreamService(c, url, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to execute swap"})
		return
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondInternalError(c, err)
		return
	}

	c.JSON(http.StatusOK, result)
}

// GetRoutes returns the routes TigerAggregator actually found across
// TigerSwap's own pools.
//
// The list is whatever the aggregator evaluated; nothing is synthesised, and a
// pair with no route yields an empty list rather than an invented one. Only
// TigerSwap pool families appear, because the aggregator quotes nothing else.
func (h *SwapHandler) GetRoutes(c *gin.Context) {
	fromToken := c.Query("from")
	toToken := c.Query("to")

	if fromToken == "" || toToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from and to tokens are required"})
		return
	}

	chainID, err := strconv.ParseInt(c.Query("chain_id"), 10, 64)
	if err != nil || chainID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chain_id must be a positive integer"})
		return
	}

	amountIn, ok := new(big.Int).SetString(c.DefaultQuery("amount_in", "1"), 10)
	if !ok || amountIn.Sign() <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "amount_in must be a positive integer string"})
		return
	}

	payload := map[string]interface{}{
		"chain_id":  chainID,
		"path":      []string{fromToken, toToken},
		"amount_in": amountIn,
		"slippage":  0.005,
	}

	url := fmt.Sprintf("%s/api/v1/quote/multihop", h.config.Upstream.DEXAggregator)
	resp, err := callUpstreamService(c, url, payload)
	if err != nil {
		respondUpstreamUnavailable(c, "TigerAggregator", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": "TigerAggregator could not price this route"})
		return
	}

	var quote struct {
		Routes []routeInfo `json:"routes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&quote); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "TigerAggregator returned a malformed response"})
		return
	}

	if quote.Routes == nil {
		quote.Routes = []routeInfo{}
	}
	c.JSON(http.StatusOK, gin.H{"routes": quote.Routes})
}

// MultiHopSwap handles multi-hop swap requests
func (h *SwapHandler) MultiHopSwap(c *gin.Context) {
	var req struct {
		ChainID  int64    `json:"chain_id" binding:"required"`
		Path     []string `json:"path" binding:"required,min=2"`
		Amount   string   `json:"amount" binding:"required"`
		Slippage string   `json:"slippage"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/quote/multihop", h.config.Upstream.DEXAggregator)
	resp, err := callUpstreamService(c, url, req)
	if err != nil {
		respondUpstreamUnavailable(c, "TigerAggregator", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "TigerAggregator returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// PriceHandler handles price-related requests
type PriceHandler struct {
	*Handler
	redis *redis.Client
}

// NewPriceHandler creates a new price handler
func NewPriceHandler(cfg *config.Config, redisClient *redis.Client) *PriceHandler {
	return &PriceHandler{NewHandler(cfg), redisClient}
}

// GetPrices returns all prices.
//
// The upstream is TigerSwap's own oracle service. Its response is forwarded only
// after it is confirmed to be a JSON object, so a malformed or unexpected reply
// cannot be cached or passed through to a client.
func (h *PriceHandler) GetPrices(c *gin.Context) {
	symbols := c.Query("symbols")

	// Try cache first
	if h.redis != nil {
		cacheKey := fmt.Sprintf("prices:%s", symbols)
		ctx := context.Background()

		if cached, err := h.redis.Get(ctx, cacheKey).Result(); err == nil {
			var prices map[string]interface{}
			if json.Unmarshal([]byte(cached), &prices) == nil {
				c.JSON(http.StatusOK, prices)
				return
			}
		}
	}

	endpoint := fmt.Sprintf("%s/api/v1/oracle/prices", h.config.Upstream.PriceOracle)
	if symbols != "" {
		endpoint += "?symbols=" + url.QueryEscape(symbols)
	}
	resp, err := upstreamGet(c, endpoint)
	if err != nil {
		respondUpstreamUnavailable(c, "price oracle", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusBadGateway, gin.H{"error": "price oracle returned an error"})
		return
	}

	var prices map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&prices); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "price oracle returned a malformed response"})
		return
	}

	// Cache results
	if h.redis != nil {
		cacheKey := fmt.Sprintf("prices:%s", symbols)
		ctx := context.Background()
		if data, err := json.Marshal(prices); err == nil {
			h.redis.Set(ctx, cacheKey, string(data), 30*time.Second)
		}
	}

	c.JSON(http.StatusOK, prices)
}

// GetPrice returns price for a single symbol
func (h *PriceHandler) GetPrice(c *gin.Context) {
	symbol := c.Param("symbol")

	// Try cache
	if h.redis != nil {
		cacheKey := fmt.Sprintf("price:%s", strings.ToUpper(symbol))
		ctx := context.Background()

		if cached, err := h.redis.Get(ctx, cacheKey).Result(); err == nil {
			var price map[string]interface{}
			if json.Unmarshal([]byte(cached), &price) == nil {
				c.JSON(http.StatusOK, price)
				return
			}
		}
	}

	url := fmt.Sprintf("%s/api/v1/oracle/price/%s", h.config.Upstream.PriceOracle, url.PathEscape(symbol))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "price oracle", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "price oracle returned an error"})
		return
	}

	var price map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&price); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "price oracle returned a malformed response"})
		return
	}

	if h.redis != nil {
		ctx := context.Background()
		cacheKey := fmt.Sprintf("price:%s", strings.ToUpper(symbol))
		if data, err := json.Marshal(price); err == nil {
			h.redis.Set(ctx, cacheKey, string(data), 30*time.Second)
		}
	}

	c.JSON(http.StatusOK, price)
}

// GetPriceHistory returns price history
func (h *PriceHandler) GetPriceHistory(c *gin.Context) {
	symbol := c.Param("symbol")
	days := c.DefaultQuery("days", "7")

	url := fmt.Sprintf("%s/api/v1/oracle/price/%s/history?days=%s", h.config.Upstream.PriceOracle, url.PathEscape(symbol), url.QueryEscape(days))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "price oracle", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "price oracle returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// TokenHandler handles token-related requests
type TokenHandler struct {
	*Handler
}

// NewTokenHandler creates a new token handler
func NewTokenHandler(cfg *config.Config) *TokenHandler {
	return &TokenHandler{NewHandler(cfg)}
}

// Token represents a token
type Token struct {
	Address   string `json:"address"`
	Symbol    string `json:"symbol"`
	Name      string `json:"name"`
	Decimals  int    `json:"decimals"`
	ChainID   int64  `json:"chainId"`
	LogoURI   string `json:"logoURI"`
	PriceUSD  string `json:"priceUSD"`
	Volume24h string `json:"volume24h"`
}

// GetTokens returns all tokens
func (h *TokenHandler) GetTokens(c *gin.Context) {
	chainID := c.Query("chainId")

	url := fmt.Sprintf("%s/api/v1/tokens", h.config.Upstream.Blockchain)
	if chainID != "" {
		url = fmt.Sprintf("%s?chainId=%s", url, chainID)
	}
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "blockchain service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "blockchain service returned an error"})
		return
	}

	var tokens []Token
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, tokens)
}

// GetToken returns a single token
func (h *TokenHandler) GetToken(c *gin.Context) {
	address := c.Param("address")

	url := fmt.Sprintf("%s/api/v1/tokens/%s", h.config.Upstream.Blockchain, url.PathEscape(address))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "blockchain service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
		return
	}
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "blockchain service returned an error"})
		return
	}

	var token Token
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, token)
}

// SearchTokens searches for tokens
func (h *TokenHandler) SearchTokens(c *gin.Context) {
	query := c.Query("q")

	if len(query) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Query too short"})
		return
	}

	url := fmt.Sprintf("%s/api/v1/tokens/search?q=%s", h.config.Upstream.Blockchain, url.QueryEscape(query))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "blockchain service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "blockchain service returned an error"})
		return
	}

	var results []Token
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	if results == nil {
		results = []Token{}
	}

	c.JSON(http.StatusOK, results)
}

// PoolHandler handles pool-related requests
type PoolHandler struct {
	*Handler
}

// NewPoolHandler creates a new pool handler
func NewPoolHandler(cfg *config.Config) *PoolHandler {
	return &PoolHandler{NewHandler(cfg)}
}

// Pool represents a liquidity pool
type Pool struct {
	Token0       string `json:"token0"`
	Token1       string `json:"token1"`
	Reserve0     string `json:"reserve0"`
	Reserve1     string `json:"reserve1"`
	Fee          int    `json:"fee"`
	LiquidityUSD string `json:"liquidityUSD"`
	Volume24h    string `json:"volume24h"`
	APR          string `json:"apr"`
}

// GetPools returns all pools
func (h *PoolHandler) GetPools(c *gin.Context) {
	url := fmt.Sprintf("%s/api/v1/pools", h.config.Upstream.Blockchain)
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "blockchain service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "blockchain service returned an error"})
		return
	}

	var pools []Pool
	if err := json.NewDecoder(resp.Body).Decode(&pools); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, pools)
}

// GetPool returns a single pool
func (h *PoolHandler) GetPool(c *gin.Context) {
	tokenA := c.Param("tokenA")
	tokenB := c.Param("tokenB")

	url := fmt.Sprintf("%s/api/v1/pools/%s/%s", h.config.Upstream.Blockchain, url.PathEscape(tokenA), url.PathEscape(tokenB))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "blockchain service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "pool not found"})
		return
	}

	var pool Pool
	if err := json.NewDecoder(resp.Body).Decode(&pool); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, pool)
}

// GetPoolVolume returns pool volume
func (h *PoolHandler) GetPoolVolume(c *gin.Context) {
	tokenA := c.Param("tokenA")
	tokenB := c.Param("tokenB")

	url := fmt.Sprintf("%s/api/v1/pools/%s/%s/volume", h.config.Upstream.Blockchain, url.PathEscape(tokenA), url.PathEscape(tokenB))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "blockchain service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "pool volume unavailable"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// WalletHandler handles wallet-related requests
type WalletHandler struct {
	*Handler
}

// NewWalletHandler creates a new wallet handler
func NewWalletHandler(cfg *config.Config) *WalletHandler {
	return &WalletHandler{NewHandler(cfg)}
}

// CreateWallet creates a new wallet
// CreateWallet derives a fresh watch-only account for the caller.
//
// The gateway never returns a private key or a mnemonic. A seed phrase sent to
// a browser is a seed phrase the browser, every extension and every injected
// script can read, and it would be cached by any intermediary that does not
// strip the body. The address alone is safe to return and is all the frontend
// uses; custody stays with the platform wallet service, which is the only
// component that holds key material.
func (h *WalletHandler) CreateWallet(c *gin.Context) {
	if _, ok := requireIdentity(c); !ok {
		return
	}

	var req struct {
		ChainID int64 `json:"chainId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	address, err := generateWatchOnlyAddress()
	if err != nil {
		respondInternalError(c, err)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"address": address,
		"chainId": req.ChainID,
	})
}

// ImportWallet derives the address for a mnemonic without storing or echoing it.
//
// The mnemonic arrives over TLS, is used to derive the account address, and is
// never written back in the response or to a log. Custody is a separate,
// authenticated, server-side operation; this endpoint only answers "which
// address does this phrase control".
func (h *WalletHandler) ImportWallet(c *gin.Context) {
	if _, ok := requireIdentity(c); !ok {
		return
	}

	var req struct {
		Mnemonic string `json:"mnemonic" binding:"required"`
		ChainID  int64  `json:"chainId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	if !bip39.IsMnemonicValid(req.Mnemonic) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid mnemonic"})
		return
	}

	address, err := deriveAddressFromMnemonic(req.Mnemonic)
	if err != nil {
		respondBadRequest(c, err)
		return
	}

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, gin.H{
		"address":  address,
		"chainId":  req.ChainID,
		"imported": true,
	})
}

// GetBalance returns wallet balance
func (h *WalletHandler) GetBalance(c *gin.Context) {
	address := c.Param("address")
	chainID := c.DefaultQuery("chainId", "1")

	if !common.IsHexAddress(address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
		return
	}

	url := fmt.Sprintf("%s/api/v1/wallet/%s/balance?chainId=%s", h.config.Upstream.Wallet, url.PathEscape(address), url.QueryEscape(chainID))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "wallet service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "wallet service returned an error"})
		return
	}

	var balance map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&balance); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, balance)
}

// GetTransactions returns wallet transactions
func (h *WalletHandler) GetTransactions(c *gin.Context) {
	address := c.Param("address")
	if !common.IsHexAddress(address) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid address"})
		return
	}

	url := fmt.Sprintf("%s/api/v1/wallet/%s/transactions", h.config.Upstream.Wallet, url.PathEscape(address))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "wallet service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "wallet service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// Transfer handles transfer requests
func (h *WalletHandler) Transfer(c *gin.Context) {
	var req struct {
		FromAddress string `json:"from" binding:"required"`
		ToAddress   string `json:"to" binding:"required"`
		Amount      string `json:"amount" binding:"required"`
		Token       string `json:"token" binding:"required"`
		ChainID     int64  `json:"chainId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/wallet/transfer", h.config.Upstream.Wallet)
	resp, err := callUpstreamService(c, url, req)
	if err != nil {
		respondUpstreamUnavailable(c, "wallet service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "wallet service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// Approve handles token approval
func (h *WalletHandler) Approve(c *gin.Context) {
	var req struct {
		Owner   string `json:"owner" binding:"required"`
		Spender string `json:"spender" binding:"required"`
		Token   string `json:"token" binding:"required"`
		Amount  string `json:"amount" binding:"required"`
		ChainID int64  `json:"chainId" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/wallet/approve", h.config.Upstream.Wallet)
	resp, err := callUpstreamService(c, url, req)
	if err != nil {
		respondUpstreamUnavailable(c, "wallet service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "wallet service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// ChainHandler handles chain-related requests
type ChainHandler struct {
	*Handler
}

// NewChainHandler creates a new chain handler
func NewChainHandler(cfg *config.Config) *ChainHandler {
	return &ChainHandler{NewHandler(cfg)}
}

// Chain represents a blockchain
type Chain struct {
	ChainID     int64  `json:"chainId"`
	Name        string `json:"name"`
	Symbol      string `json:"symbol"`
	Explorer    string `json:"explorer"`
	RPC         string `json:"rpc"`
	NativeToken string `json:"nativeToken"`
}

// GetChains returns all supported chains
func (h *ChainHandler) GetChains(c *gin.Context) {
	url := fmt.Sprintf("%s/api/v1/chains", h.config.Upstream.Blockchain)
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "blockchain service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "blockchain service returned an error"})
		return
	}

	var chains []Chain
	if err := json.NewDecoder(resp.Body).Decode(&chains); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, chains)
}

// GetChain returns a single chain
func (h *ChainHandler) GetChain(c *gin.Context) {
	chainID, err := strconv.ParseInt(c.Param("chainId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chainId"})
		return
	}

	url := fmt.Sprintf("%s/api/v1/chains/%d", h.config.Upstream.Blockchain, chainID)
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "blockchain service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "Chain not found"})
		return
	}

	var chain Chain
	if err := json.NewDecoder(resp.Body).Decode(&chain); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, chain)
}

// GetGasFees returns gas fees for a chain
func (h *ChainHandler) GetGasFees(c *gin.Context) {
	chainID, err := strconv.ParseInt(c.Param("chainId"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chainId"})
		return
	}

	url := fmt.Sprintf("%s/api/v1/chains/%d/fees", h.config.Upstream.Blockchain, chainID)
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "blockchain service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "gas fees unavailable"})
		return
	}

	var fees map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&fees); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, fees)
}

// OrderHandler handles order-related requests
type OrderHandler struct {
	*Handler
}

// NewOrderHandler creates a new order handler
func NewOrderHandler(cfg *config.Config) *OrderHandler {
	return &OrderHandler{NewHandler(cfg)}
}

// Order represents a trading order
type Order struct {
	OrderID   string `json:"orderId"`
	UserID    string `json:"userId"`
	Type      string `json:"type"`
	Side      string `json:"side"`
	Price     string `json:"price"`
	Amount    string `json:"amount"`
	Filled    string `json:"filled"`
	Status    string `json:"status"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// CreateOrder creates a new order
func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var order Order
	if err := c.ShouldBindJSON(&order); err != nil {
		respondBadRequest(c, err)
		return
	}
	// The order belongs to the authenticated caller. A user id in the body is
	// overwritten rather than trusted, so an order cannot be placed in another
	// account's name.
	userID, ok := requireIdentity(c)
	if !ok {
		return
	}
	order.UserID = userID

	url := fmt.Sprintf("%s/api/v1/orders", h.config.Upstream.Blockchain)
	resp, err := callUpstreamService(c, url, order)
	if err != nil {
		respondUpstreamUnavailable(c, "order service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "order service returned an error"})
		return
	}

	var created Order
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

// GetOrder returns a single order
func (h *OrderHandler) GetOrder(c *gin.Context) {
	orderID := c.Param("orderId")

	url := fmt.Sprintf("%s/api/v1/orders/%s", h.config.Upstream.Blockchain, url.PathEscape(orderID))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "order service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "order not found"})
		return
	}

	var order Order
	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, order)
}

// UpdateOrder updates an order
func (h *OrderHandler) UpdateOrder(c *gin.Context) {
	if _, ok := requireIdentity(c); !ok {
		return
	}
	orderID := c.Param("orderId")

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		respondBadRequest(c, err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/orders/%s", h.config.Upstream.Blockchain, url.PathEscape(orderID))
	resp, err := callUpstreamJSON(c, http.MethodPut, url, updates)
	if err != nil {
		respondUpstreamUnavailable(c, "order service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "order service returned an error"})
		return
	}

	var order Order
	if err := json.NewDecoder(resp.Body).Decode(&order); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, order)
}

// CancelOrder cancels an order
func (h *OrderHandler) CancelOrder(c *gin.Context) {
	if _, ok := requireIdentity(c); !ok {
		return
	}
	orderID := c.Param("orderId")

	url := fmt.Sprintf("%s/api/v1/orders/%s", h.config.Upstream.Blockchain, url.PathEscape(orderID))
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodDelete, url, nil)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	req.Header.Set("X-Request-ID", middleware.GetRequestID(c))
	resp, err := upstreamClient.Do(req)
	if err != nil {
		respondUpstreamUnavailable(c, "order service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "order service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetUserOrders returns the caller's own orders.
//
// The path carries a user id for the upstream contract, but the value that is
// actually sent is the authenticated session's, so one account cannot read
// another's order history by editing the path.
func (h *OrderHandler) GetUserOrders(c *gin.Context) {
	userID, ok := requireIdentity(c)
	if !ok {
		return
	}

	url := fmt.Sprintf("%s/api/v1/orders/user/%s", h.config.Upstream.Blockchain, url.PathEscape(userID))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "order service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "order service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// PerpetualHandler handles perpetual trading
type PerpetualHandler struct {
	*Handler
}

// NewPerpetualHandler creates a new perpetual handler
func NewPerpetualHandler(cfg *config.Config) *PerpetualHandler {
	return &PerpetualHandler{NewHandler(cfg)}
}

// Position represents a perpetual position
type Position struct {
	PositionID       string `json:"positionId"`
	UserID           string `json:"userId"`
	Pair             string `json:"pair"`
	Side             string `json:"side"`
	Size             string `json:"size"`
	EntryPrice       string `json:"entryPrice"`
	MarkPrice        string `json:"markPrice"`
	LiquidationPrice string `json:"liquidationPrice"`
	Leverage         string `json:"leverage"`
	UnrealizedPNL    string `json:"unrealizedPnl"`
	Status           string `json:"status"`
}

// GetPositions returns the caller's own perpetual positions. The query string
// cannot select another account: the id sent upstream is the session's.
func (h *PerpetualHandler) GetPositions(c *gin.Context) {
	userID, ok := requireIdentity(c)
	if !ok {
		return
	}
	url := fmt.Sprintf("%s/api/v1/perpetual/positions?userId=%s", h.config.Upstream.Blockchain, url.QueryEscape(userID))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "perpetuals service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "perpetuals service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// OpenPosition opens a new position
func (h *PerpetualHandler) OpenPosition(c *gin.Context) {
	var pos Position
	if err := c.ShouldBindJSON(&pos); err != nil {
		respondBadRequest(c, err)
		return
	}
	// A position is opened for the authenticated caller; a body-supplied user id
	// is replaced, not honoured.
	userID, ok := requireIdentity(c)
	if !ok {
		return
	}
	pos.UserID = userID

	url := fmt.Sprintf("%s/api/v1/perpetual/positions", h.config.Upstream.Blockchain)
	resp, err := callUpstreamService(c, url, pos)
	if err != nil {
		respondUpstreamUnavailable(c, "perpetuals service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "perpetuals service returned an error"})
		return
	}

	var created Position
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

// UpdatePosition updates a position
func (h *PerpetualHandler) UpdatePosition(c *gin.Context) {
	if _, ok := requireIdentity(c); !ok {
		return
	}
	positionID := c.Param("positionId")

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		respondBadRequest(c, err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/perpetual/positions/%s", h.config.Upstream.Blockchain, url.PathEscape(positionID))
	resp, err := callUpstreamJSON(c, http.MethodPut, url, updates)
	if err != nil {
		respondUpstreamUnavailable(c, "perpetuals service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "perpetuals service returned an error"})
		return
	}

	var pos Position
	if err := json.NewDecoder(resp.Body).Decode(&pos); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, pos)
}

// ClosePosition closes a position
func (h *PerpetualHandler) ClosePosition(c *gin.Context) {
	if _, ok := requireIdentity(c); !ok {
		return
	}
	positionID := c.Param("positionId")

	url := fmt.Sprintf("%s/api/v1/perpetual/positions/%s", h.config.Upstream.Blockchain, url.PathEscape(positionID))
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodDelete, url, nil)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	req.Header.Set("X-Request-ID", middleware.GetRequestID(c))
	resp, err := upstreamClient.Do(req)
	if err != nil {
		respondUpstreamUnavailable(c, "perpetuals service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "perpetuals service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// StakingHandler handles staking operations
type StakingHandler struct {
	*Handler
}

// NewStakingHandler creates a new staking handler
func NewStakingHandler(cfg *config.Config) *StakingHandler {
	return &StakingHandler{NewHandler(cfg)}
}

// StakingPool represents a staking pool
type StakingPool struct {
	PoolID      string `json:"poolId"`
	Token       string `json:"token"`
	RewardToken string `json:"rewardToken"`
	APY         string `json:"apy"`
	TVL         string `json:"tvl"`
	Duration    int    `json:"duration"`
}

// GetPools returns staking pools
func (h *StakingHandler) GetPools(c *gin.Context) {
	url := fmt.Sprintf("%s/api/v1/staking/pools", h.config.Upstream.Blockchain)
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "staking service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "staking service returned an error"})
		return
	}

	var pools []StakingPool
	if err := json.NewDecoder(resp.Body).Decode(&pools); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	if pools == nil {
		pools = []StakingPool{}
	}
	c.JSON(http.StatusOK, pools)
}

// Stake stakes tokens for the authenticated caller. A userId in the body is
// ignored so a stake cannot be attributed to another account.
func (h *StakingHandler) Stake(c *gin.Context) {
	userID, ok := requireIdentity(c)
	if !ok {
		return
	}
	var req struct {
		UserID string `json:"userId"`
		PoolID string `json:"poolId" binding:"required"`
		Amount string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}
	req.UserID = userID

	url := fmt.Sprintf("%s/api/v1/staking/stake", h.config.Upstream.Blockchain)
	resp, err := callUpstreamService(c, url, req)
	if err != nil {
		respondUpstreamUnavailable(c, "staking service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "staking service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// Unstake unstakes tokens for the authenticated caller. The body cannot name
// another account.
func (h *StakingHandler) Unstake(c *gin.Context) {
	userID, ok := requireIdentity(c)
	if !ok {
		return
	}
	var req struct {
		UserID string `json:"userId"`
		PoolID string `json:"poolId" binding:"required"`
		Amount string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}
	req.UserID = userID

	url := fmt.Sprintf("%s/api/v1/staking/unstake", h.config.Upstream.Blockchain)
	resp, err := callUpstreamService(c, url, req)
	if err != nil {
		respondUpstreamUnavailable(c, "staking service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "staking service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetRewards returns the caller's own staking rewards. The query string cannot
// select another account.
func (h *StakingHandler) GetRewards(c *gin.Context) {
	userID, ok := requireIdentity(c)
	if !ok {
		return
	}

	url := fmt.Sprintf("%s/api/v1/staking/rewards?userId=%s", h.config.Upstream.Blockchain, url.QueryEscape(userID))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "staking service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "staking service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// BridgeHandler handles bridge operations
type BridgeHandler struct {
	*Handler
}

// NewBridgeHandler creates a new bridge handler
func NewBridgeHandler(cfg *config.Config) *BridgeHandler {
	return &BridgeHandler{NewHandler(cfg)}
}

// Transfer initiates a bridge transfer
func (h *BridgeHandler) Transfer(c *gin.Context) {
	if _, ok := requireIdentity(c); !ok {
		return
	}
	var req struct {
		FromChain   int64  `json:"fromChain" binding:"required"`
		ToChain     int64  `json:"toChain" binding:"required"`
		Token       string `json:"token" binding:"required"`
		Amount      string `json:"amount" binding:"required"`
		FromAddress string `json:"fromAddress" binding:"required"`
		ToAddress   string `json:"toAddress" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/bridge/transfer", h.config.Upstream.Blockchain)
	resp, err := callUpstreamService(c, url, req)
	if err != nil {
		respondUpstreamUnavailable(c, "bridge service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "bridge service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetTransferStatus returns bridge transfer status
func (h *BridgeHandler) GetTransferStatus(c *gin.Context) {
	txHash := c.Param("txHash")

	url := fmt.Sprintf("%s/api/v1/bridge/transfer/%s", h.config.Upstream.Blockchain, url.PathEscape(txHash))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "bridge service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "transfer not found"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetSupportedChains returns supported bridge chains
func (h *BridgeHandler) GetSupportedChains(c *gin.Context) {
	url := fmt.Sprintf("%s/api/v1/bridge/chains", h.config.Upstream.Blockchain)
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "bridge service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "bridge service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// FarmingHandler handles farming operations
type FarmingHandler struct {
	*Handler
}

// NewFarmingHandler creates a new farming handler
func NewFarmingHandler(cfg *config.Config) *FarmingHandler {
	return &FarmingHandler{NewHandler(cfg)}
}

// GetPools returns farming pools
func (h *FarmingHandler) GetPools(c *gin.Context) {
	url := fmt.Sprintf("%s/api/v1/farming/pools", h.config.Upstream.Blockchain)
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "farming service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "farming service returned an error"})
		return
	}

	var pools []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&pools); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	if pools == nil {
		pools = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, pools)
}

// Deposit deposits liquidity
// Deposit adds liquidity for the authenticated caller. A userId in the body is
// ignored so liquidity cannot be credited to another account.
func (h *FarmingHandler) Deposit(c *gin.Context) {
	userID, ok := requireIdentity(c)
	if !ok {
		return
	}
	var req struct {
		UserID  string `json:"userId"`
		PoolID  string `json:"poolId" binding:"required"`
		Amount0 string `json:"amount0" binding:"required"`
		Amount1 string `json:"amount1" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}
	req.UserID = userID

	url := fmt.Sprintf("%s/api/v1/farming/deposit", h.config.Upstream.Blockchain)
	resp, err := callUpstreamService(c, url, req)
	if err != nil {
		respondUpstreamUnavailable(c, "farming service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "farming service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// Withdraw withdraws liquidity
// Withdraw removes liquidity for the authenticated caller. The body cannot name
// another account.
func (h *FarmingHandler) Withdraw(c *gin.Context) {
	userID, ok := requireIdentity(c)
	if !ok {
		return
	}
	var req struct {
		UserID    string `json:"userId"`
		PoolID    string `json:"poolId" binding:"required"`
		Liquidity string `json:"liquidity" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}
	req.UserID = userID

	url := fmt.Sprintf("%s/api/v1/farming/withdraw", h.config.Upstream.Blockchain)
	resp, err := callUpstreamService(c, url, req)
	if err != nil {
		respondUpstreamUnavailable(c, "farming service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "farming service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetRewards returns the caller's own farming rewards.
func (h *FarmingHandler) GetRewards(c *gin.Context) {
	userID, ok := requireIdentity(c)
	if !ok {
		return
	}

	url := fmt.Sprintf("%s/api/v1/farming/rewards?userId=%s", h.config.Upstream.Blockchain, url.QueryEscape(userID))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "farming service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "farming service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// LaunchpadHandler handles launchpad operations
type LaunchpadHandler struct {
	*Handler
}

// NewLaunchpadHandler creates a new launchpad handler
func NewLaunchpadHandler(cfg *config.Config) *LaunchpadHandler {
	return &LaunchpadHandler{NewHandler(cfg)}
}

// GetProjects returns launchpad projects
func (h *LaunchpadHandler) GetProjects(c *gin.Context) {
	url := fmt.Sprintf("%s/api/v1/launchpad/projects", h.config.Upstream.Blockchain)
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "launchpad service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "launchpad service returned an error"})
		return
	}

	var projects []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	if projects == nil {
		projects = []map[string]interface{}{}
	}
	c.JSON(http.StatusOK, projects)
}

// GetProject returns a single project
func (h *LaunchpadHandler) GetProject(c *gin.Context) {
	id := c.Param("id")

	url := fmt.Sprintf("%s/api/v1/launchpad/projects/%s", h.config.Upstream.Blockchain, url.PathEscape(id))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "launchpad service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "project not found"})
		return
	}

	var project map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&project); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, project)
}

// CreateProject submits a new project for the authenticated caller. The
// submission is attributed to the session, not to any owner field the client
// supplies, so a project cannot be filed under another account.
func (h *LaunchpadHandler) CreateProject(c *gin.Context) {
	userID, ok := requireIdentity(c)
	if !ok {
		return
	}
	var project map[string]interface{}
	if err := c.ShouldBindJSON(&project); err != nil {
		respondBadRequest(c, err)
		return
	}
	project["ownerId"] = userID

	url := fmt.Sprintf("%s/api/v1/launchpad/projects", h.config.Upstream.Blockchain)
	resp, err := callUpstreamService(c, url, project)
	if err != nil {
		respondUpstreamUnavailable(c, "launchpad service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "launchpad service returned an error"})
		return
	}

	var created map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, created)
}

// Contribute contributes to a project
// Contribute records a launchpad contribution for the authenticated caller. A
// userId in the body is ignored so a contribution cannot be attributed to
// another account.
func (h *LaunchpadHandler) Contribute(c *gin.Context) {
	userID, ok := requireIdentity(c)
	if !ok {
		return
	}
	var req struct {
		UserID    string `json:"userId"`
		ProjectID string `json:"projectId" binding:"required"`
		Amount    string `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}
	req.UserID = userID

	url := fmt.Sprintf("%s/api/v1/launchpad/contribute", h.config.Upstream.Blockchain)
	resp, err := callUpstreamService(c, url, req)
	if err != nil {
		respondUpstreamUnavailable(c, "launchpad service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "launchpad service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// AdminHandler handles admin operations
type AdminHandler struct {
	*Handler
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(cfg *config.Config) *AdminHandler {
	return &AdminHandler{NewHandler(cfg)}
}

// GetStats returns platform statistics
func (h *AdminHandler) GetStats(c *gin.Context) {
	url := fmt.Sprintf("%s/api/v1/admin/stats", h.config.Upstream.Blockchain)
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "admin service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "admin service returned an error"})
		return
	}

	var stats map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetUsers returns all users
func (h *AdminHandler) GetUsers(c *gin.Context) {
	url := fmt.Sprintf("%s/api/v1/admin/users", h.config.Upstream.Blockchain)
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "admin service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "admin service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetUser returns a single user
func (h *AdminHandler) GetUser(c *gin.Context) {
	id := c.Param("id")

	url := fmt.Sprintf("%s/api/v1/admin/users/%s", h.config.Upstream.Blockchain, url.PathEscape(id))
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "admin service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "user not found"})
		return
	}

	var user map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

// UpdateUser updates a user
func (h *AdminHandler) UpdateUser(c *gin.Context) {
	id := c.Param("id")

	var updates map[string]interface{}
	if err := c.ShouldBindJSON(&updates); err != nil {
		respondBadRequest(c, err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/admin/users/%s", h.config.Upstream.Blockchain, url.PathEscape(id))
	resp, err := callUpstreamJSON(c, http.MethodPut, url, updates)
	if err != nil {
		respondUpstreamUnavailable(c, "admin service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "admin service returned an error"})
		return
	}

	var user map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, user)
}

// DeleteUser deletes a user
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	id := c.Param("id")

	url := fmt.Sprintf("%s/api/v1/admin/users/%s", h.config.Upstream.Blockchain, url.PathEscape(id))
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodDelete, url, nil)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	req.Header.Set("X-Request-ID", middleware.GetRequestID(c))
	resp, err := upstreamClient.Do(req)
	if err != nil {
		respondUpstreamUnavailable(c, "admin service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "admin service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetFeeSettings returns fee settings
func (h *AdminHandler) GetFeeSettings(c *gin.Context) {
	url := fmt.Sprintf("%s/api/v1/admin/fees", h.config.Upstream.Blockchain)
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "admin service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "admin service returned an error"})
		return
	}

	var fees map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&fees); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, fees)
}

// UpdateFeeSettings updates fee settings
func (h *AdminHandler) UpdateFeeSettings(c *gin.Context) {
	var fees map[string]interface{}
	if err := c.ShouldBindJSON(&fees); err != nil {
		respondBadRequest(c, err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/admin/fees", h.config.Upstream.Blockchain)
	resp, err := callUpstreamJSON(c, http.MethodPut, url, fees)
	if err != nil {
		respondUpstreamUnavailable(c, "admin service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "admin service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// GetWhitelist returns whitelist
func (h *AdminHandler) GetWhitelist(c *gin.Context) {
	url := fmt.Sprintf("%s/api/v1/admin/whitelist", h.config.Upstream.Blockchain)
	resp, err := upstreamGet(c, url)
	if err != nil {
		respondUpstreamUnavailable(c, "admin service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "admin service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// AddToWhitelist adds address to whitelist
func (h *AdminHandler) AddToWhitelist(c *gin.Context) {
	var req struct {
		Address string `json:"address" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequest(c, err)
		return
	}

	url := fmt.Sprintf("%s/api/v1/admin/whitelist", h.config.Upstream.Blockchain)
	resp, err := callUpstreamService(c, url, req)
	if err != nil {
		respondUpstreamUnavailable(c, "admin service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "admin service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusCreated, result)
}

// RemoveFromWhitelist removes address from whitelist
func (h *AdminHandler) RemoveFromWhitelist(c *gin.Context) {
	address := c.Param("address")

	url := fmt.Sprintf("%s/api/v1/admin/whitelist/%s", h.config.Upstream.Blockchain, url.PathEscape(address))
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodDelete, url, nil)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	req.Header.Set("X-Request-ID", middleware.GetRequestID(c))
	resp, err := upstreamClient.Do(req)
	if err != nil {
		respondUpstreamUnavailable(c, "admin service", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "admin service returned an error"})
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		respondUpstreamResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// ============ Helper Functions ============

// upstreamClient is shared by every outbound call so connections are pooled
// instead of re-dialled per request. The timeout bounds a slow upstream, and
// each request also carries the inbound request's context so a client that
// disconnects cancels the upstream call.
var upstreamClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        256,
		MaxIdleConnsPerHost: 64,
		IdleConnTimeout:     90 * time.Second,
	},
}

// upstreamGet performs a GET against an upstream service using the shared
// client and the caller's request context.
// respondInternalError logs an unexpected local failure and returns a fixed
// body. The raw error can name a file, a URL or a credential-bearing endpoint,
// so it is recorded for the operator and kept off the wire.
func respondInternalError(c *gin.Context, err error) {
	log.Printf("internal error on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}

// respondBadRequest answers a caller-correctable failure. The detail is logged
// and a fixed message returned so a decoder that quotes internal state cannot
// leak it; the status is what the client needs to react to.
func respondBadRequest(c *gin.Context, err error) {
	log.Printf("bad request on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusBadRequest, gin.H{"error": "request could not be processed"})
}

// respondUpstreamUnavailable answers a transport failure against an upstream
// service. The dialer's error quotes the endpoint, which can carry a key in its
// path, so only the service name and a fixed message are returned.
func respondUpstreamUnavailable(c *gin.Context, service string, err error) {
	log.Printf("%s upstream unreachable on %s %s: %v", service, c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusBadGateway, gin.H{"error": service + " upstream unavailable"})
}

// respondUpstreamResponseError answers a failure to decode an upstream body.
// The decode error can quote a fragment of the response, so it is logged and
// replaced with a fixed message.
func respondUpstreamResponseError(c *gin.Context, err error) {
	log.Printf("upstream response error on %s %s: %v", c.Request.Method, c.Request.URL.Path, err)
	c.JSON(http.StatusBadGateway, gin.H{"error": "upstream service returned an unreadable response"})
}

func upstreamGet(c *gin.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Request-ID", middleware.GetRequestID(c))
	return upstreamClient.Do(req)
}

// callUpstreamService POSTs a JSON body to an upstream service.
func callUpstreamService(c *gin.Context, url string, body interface{}) (*http.Response, error) {
	return callUpstreamJSON(c, http.MethodPost, url, body)
}

// callUpstreamJSON performs a JSON request against an upstream service.
func callUpstreamJSON(c *gin.Context, method, url string, body interface{}) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), method, url, reader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", middleware.GetRequestID(c))
	if auth := c.GetHeader("Authorization"); auth != "" {
		req.Header.Set("Authorization", auth)
	}

	return upstreamClient.Do(req)
}

// generateWatchOnlyAddress creates a BIP39 mnemonic, derives the BIP44
// Ethereum account and returns only the address. The key material is discarded
// in this process: the caller is handed an address, never a secret.
func generateWatchOnlyAddress() (string, error) {
	entropy, err := bip39.NewEntropy(256)
	if err != nil {
		return "", fmt.Errorf("entropy: %w", err)
	}
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", fmt.Errorf("mnemonic: %w", err)
	}
	_, address, err := deriveKeyFromMnemonic(mnemonic)
	if err != nil {
		return "", err
	}
	return address, nil
}

// deriveAddressFromMnemonic returns the BIP44 Ethereum address for a mnemonic.
func deriveAddressFromMnemonic(mnemonic string) (string, error) {
	_, addr, err := deriveKeyFromMnemonic(mnemonic)
	return addr, err
}

// deriveKeyFromMnemonic performs BIP39 seed + BIP32/BIP44 m/44'/60'/0'/0/0 derivation.
func deriveKeyFromMnemonic(mnemonic string) (*ecdsa.PrivateKey, string, error) {
	seed := bip39.NewSeed(mnemonic, "")
	master, err := bip32.NewMasterKey(seed)
	if err != nil {
		return nil, "", fmt.Errorf("bip32 master: %w", err)
	}

	purpose, err := master.NewChildKey(bip32.FirstHardenedChild + 44)
	if err != nil {
		return nil, "", err
	}
	coin, err := purpose.NewChildKey(bip32.FirstHardenedChild + 60)
	if err != nil {
		return nil, "", err
	}
	account, err := coin.NewChildKey(bip32.FirstHardenedChild + 0)
	if err != nil {
		return nil, "", err
	}
	change, err := account.NewChildKey(0)
	if err != nil {
		return nil, "", err
	}
	index, err := change.NewChildKey(0)
	if err != nil {
		return nil, "", err
	}

	key, err := crypto.ToECDSA(index.Key)
	if err != nil {
		return nil, "", fmt.Errorf("to ecdsa: %w", err)
	}
	return key, crypto.PubkeyToAddress(key.PublicKey).Hex(), nil
}
