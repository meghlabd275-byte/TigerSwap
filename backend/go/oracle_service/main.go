// Package main runs TigerSwap's price oracle service.
//
// The service reads prices from TigerSwap's own on-chain median oracle
// (TigerSwapPriceOracle) and exposes them over HTTP. It does not call any
// third-party price API, and it does not carry a table of hardcoded feed
// addresses: every price it serves is read from a contract that TigerSwap
// deploys and controls, on an RPC endpoint TigerSwap runs.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"tigerswap/backend/config"
	"tigerswap/backend/go/tigerswap"
)

// priceCacheTTL bounds how long an oracle read is reused.
const priceCacheTTL = 15 * time.Second

// historyRetention is how long observed prices are kept for 24h-change maths.
const historyRetention = 48 * time.Hour

// PriceFeed is a price read from TigerSwap's on-chain oracle.
type PriceFeed struct {
	Token     string    `json:"token"`
	ChainID   int64     `json:"chain_id"`
	Price     string    `json:"price"`     // WAD-scaled (1e18) integer string
	PriceUSD  float64   `json:"price_usd"` // decimal form, for display only
	Change24h float64   `json:"change_24h"`
	Reports   uint64    `json:"reports"` // live reporter count behind the median
	UpdatedAt time.Time `json:"updated_at"`
}

// OracleService reads prices from TigerSwap's own oracle contracts.
type OracleService struct {
	redis *redis.Client
	mu    sync.Mutex
	// clients caches one TigerSwap client per chain so RPC connections are reused.
	clients map[int64]*tigerswap.Client
}

// NewOracleService builds the service. Redis is optional: without it the service
// still answers, it just does not cache.
func NewOracleService(redisURL string) (*OracleService, error) {
	svc := &OracleService{clients: make(map[int64]*tigerswap.Client)}

	if strings.TrimSpace(redisURL) == "" {
		return svc, nil
	}
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(redisOpts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		// A missing cache degrades throughput, not correctness, so keep serving.
		log.Printf("price cache unavailable, continuing without it: %v", err)
		return svc, nil
	}
	svc.redis = client
	return svc, nil
}

// clientFor returns the TigerSwap client for a chain, creating it on first use.
func (o *OracleService) clientFor(chainID int64) (*tigerswap.Client, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	if c, ok := o.clients[chainID]; ok {
		return c, nil
	}

	for _, d := range tigerswap.LoadDeployments() {
		if d.ChainID != chainID {
			continue
		}
		if d.Oracle == (common.Address{}) {
			return nil, fmt.Errorf("chain %d has no TigerSwap oracle configured", chainID)
		}
		c, err := tigerswap.NewClient(d, nil)
		if err != nil {
			return nil, err
		}
		o.clients[chainID] = c
		return c, nil
	}
	return nil, fmt.Errorf("no TigerSwap deployment configured for chain %d", chainID)
}

// GetPrice returns the price for a token from TigerSwap's on-chain oracle.
func (o *OracleService) GetPrice(ctx context.Context, token string, chainID int64) (*PriceFeed, error) {
	sym := strings.ToUpper(strings.TrimSpace(token))
	if sym == "" {
		return nil, errors.New("token is required")
	}

	cacheKey := fmt.Sprintf("oracle:%d:%s", chainID, sym)
	if o.redis != nil {
		if cached, err := o.redis.Get(ctx, cacheKey).Result(); err == nil {
			var feed PriceFeed
			if json.Unmarshal([]byte(cached), &feed) == nil {
				return &feed, nil
			}
		}
	}

	asset, ok := config.TokenAddress(chainID, sym)
	if !ok {
		return nil, fmt.Errorf("unknown token %s on chain %d", sym, chainID)
	}

	client, err := o.clientFor(chainID)
	if err != nil {
		return nil, err
	}

	wad, err := client.TokenPriceWAD(ctx, common.HexToAddress(asset))
	if err != nil {
		return nil, err
	}
	reports, err := client.LiveOracleReports(ctx, common.HexToAddress(asset))
	if err != nil {
		return nil, err
	}

	asFloat, _ := new(big.Float).SetInt(wad).Float64()
	priceUSD := asFloat / 1e18

	change := o.recordAndComputeChange(ctx, chainID, sym, priceUSD)

	feed := &PriceFeed{
		Token:     sym,
		ChainID:   chainID,
		Price:     wad.String(),
		PriceUSD:  priceUSD,
		Change24h: change,
		Reports:   reports,
		UpdatedAt: time.Now(),
	}

	if o.redis != nil {
		if data, err := json.Marshal(feed); err == nil {
			o.redis.Set(ctx, cacheKey, string(data), priceCacheTTL)
		}
	}
	return feed, nil
}

// recordAndComputeChange appends the observation to the symbol's series and
// returns the change against the newest sample that is at least 24h old. With
// no such sample it returns 0, which means "not yet known" rather than a
// fabricated move.
func (o *OracleService) recordAndComputeChange(ctx context.Context, chainID int64, symbol string, price float64) float64 {
	if o.redis == nil || price <= 0 {
		return 0
	}

	key := fmt.Sprintf("oracle:history:%d:%s", chainID, symbol)
	now := time.Now()
	payload, err := json.Marshal(struct {
		Price     float64 `json:"price"`
		Timestamp int64   `json:"timestamp"`
	}{Price: price, Timestamp: now.Unix()})
	if err != nil {
		return 0
	}

	if err := o.redis.ZAdd(ctx, key, redis.Z{Score: float64(now.Unix()), Member: string(payload)}).Err(); err != nil {
		return 0
	}

	// The reference is the newest sample at or before the 24h-ago mark.
	cutoff := now.Add(-24 * time.Hour).Unix()
	refs, err := o.redis.ZRevRangeByScore(ctx, key, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    strconv.FormatInt(cutoff, 10),
		Offset: 0,
		Count:  1,
	}).Result()
	if err != nil || len(refs) == 0 {
		return 0
	}

	var ref struct {
		Price float64 `json:"price"`
	}
	if err := json.Unmarshal([]byte(refs[0]), &ref); err != nil || ref.Price <= 0 {
		return 0
	}

	// Bound the retained window so the series cannot grow without limit.
	o.redis.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(now.Add(-historyRetention).Unix(), 10))

	return ((price - ref.Price) / ref.Price) * 100
}

// PricePoint is one sampled price observation.
type PricePoint struct {
	Timestamp time.Time `json:"timestamp"`
	Price     float64   `json:"price"`
}

// GetHistory returns the price series TigerSwap itself sampled for a token. The
// series is the one recordAndComputeChange writes, so a caller only ever sees
// prices read from TigerSwap's own oracle. With no samples it returns an empty
// series rather than inventing points.
func (o *OracleService) GetHistory(ctx context.Context, token string, chainID int64, days int) ([]PricePoint, error) {
	sym := strings.ToUpper(strings.TrimSpace(token))
	if sym == "" {
		return nil, errors.New("token is required")
	}
	if days <= 0 {
		return nil, errors.New("days must be positive")
	}
	if o.redis == nil {
		return nil, errors.New("price history is unavailable: cache not configured")
	}

	key := fmt.Sprintf("oracle:history:%d:%s", chainID, sym)
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()

	members, err := o.redis.ZRangeByScore(ctx, key, &redis.ZRangeBy{
		Min: strconv.FormatInt(cutoff, 10),
		Max: "+inf",
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("read price history for %s: %w", sym, err)
	}

	points := make([]PricePoint, 0, len(members))
	for _, member := range members {
		var sample struct {
			Price     float64 `json:"price"`
			Timestamp int64   `json:"timestamp"`
		}
		if err := json.Unmarshal([]byte(member), &sample); err != nil {
			continue
		}
		points = append(points, PricePoint{
			Timestamp: time.Unix(sample.Timestamp, 0),
			Price:     sample.Price,
		})
	}
	return points, nil
}

// HTTP handlers

type Handler struct {
	service *OracleService
}

func NewHandler(service *OracleService) *Handler {
	return &Handler{service: service}
}

// chainIDFromQuery reads ?chainId=, defaulting to the first configured chain so
// the answer is deterministic when the caller does not specify one.
func chainIDFromQuery(c *gin.Context) (int64, error) {
	raw := strings.TrimSpace(c.Query("chainId"))
	if raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid chainId %q", raw)
		}
		return id, nil
	}
	for _, d := range tigerswap.LoadDeployments() {
		return d.ChainID, nil
	}
	return 0, errors.New("no TigerSwap chain configured; set TIGERSWAP_CHAINS")
}

func (h *Handler) GetPrice(c *gin.Context) {
	chainID, err := chainIDFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	feed, err := h.service.GetPrice(c.Request.Context(), c.Param("token"), chainID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "price unavailable", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, feed)
}

func (h *Handler) GetAllPrices(c *gin.Context) {
	chainID, err := chainIDFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Price every token the registry knows for this chain, so the set of answers
	// is driven by configuration rather than a hardcoded shortlist.
	chain, ok := config.GetBlockchainConfigs()[chainID]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("unsupported chain %d", chainID)})
		return
	}

	symbols := make([]string, 0, 8)
	if chain.NativeToken != nil {
		symbols = append(symbols, strings.ToUpper(chain.NativeToken.Symbol))
	}
	for _, t := range config.GetTokenConfigs()[chainID] {
		symbols = append(symbols, strings.ToUpper(t.Symbol))
	}

	prices := make(map[string]*PriceFeed, len(symbols))
	for _, sym := range symbols {
		feed, err := h.service.GetPrice(c.Request.Context(), sym, chainID)
		if err != nil {
			// A token without a fresh oracle answer is simply omitted; the caller
			// must not receive a placeholder price.
			continue
		}
		prices[sym] = feed
	}
	c.JSON(http.StatusOK, prices)
}

func (h *Handler) GetPriceHistory(c *gin.Context) {
	chainID, err := chainIDFromQuery(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	days := 7
	if raw := c.Query("days"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "days must be a positive integer"})
			return
		}
		days = parsed
	}

	token := c.Param("token")
	history, err := h.service.GetHistory(c.Request.Context(), token, chainID, days)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "price history unavailable", "details": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": strings.ToUpper(token), "chain_id": chainID, "history": history})
}

func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func main() {
	service, err := NewOracleService(getEnv("REDIS_URL", "redis://localhost:6379"))
	if err != nil {
		log.Fatalf("Failed to create oracle service: %v", err)
	}

	handler := NewHandler(service)

	r := gin.Default()
	r.GET("/api/v1/oracle/price/:token", handler.GetPrice)
	r.GET("/api/v1/oracle/prices", handler.GetAllPrices)
	r.GET("/api/v1/oracle/price/:token/history", handler.GetPriceHistory)
	r.GET("/api/v1/prices/:token", handler.GetPrice)
	r.GET("/api/v1/prices", handler.GetAllPrices)
	r.GET("/api/v1/prices/:token/history", handler.GetPriceHistory)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "oracle"})
	})

	server := &http.Server{
		Addr:              ":" + getEnv("PORT", "8092"),
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("Starting Oracle Service on %s", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Oracle Service...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
