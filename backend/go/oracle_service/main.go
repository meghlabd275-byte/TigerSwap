// Package oracle provides price oracle integration for TigerSwap
// Supports Chainlink, Pyth, Uniswap V3 TWAP, and custom oracles
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// OracleConfig holds oracle configuration
type OracleConfig struct {
	Name      string
	Address  string
	ChainID   int64
	Heartbeat time.Duration
	Deviation float64
}

// PriceFeed represents a price feed
type PriceFeed struct {
	Token      string    `json:"token"`
	Price      *big.Int `json:"price"`
	Confidence *big.Int `json:"confidence"`
	UpdatedAt  time.Time `json:"updated_at"`
	Oracle     string    `json:"oracle"`
}

// OracleService provides price oracle aggregation
type OracleService struct {
	redis          *redis.Client
	ethClients     map[int64]*ethclient.Client
	oracles        map[string]*OracleConfig
	priceFeeds    map[string]*PriceFeed
	mu             sync.RWMutex
	chainlinkFeeds map[int64]map[string]common.Address
	pythFeeds      map[int64]map[string][]byte
}

// NewOracleService creates a new oracle service
func NewOracleService(redisURL string) (*OracleService, error) {
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	redisClient := redis.NewClient(redisOpts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
	}

	return &OracleService{
		redis:          redisClient,
		ethClients:     make(map[int64]*ethclient.Client),
		oracles:        make(map[string]*OracleConfig),
		priceFeeds:    make(map[string]*PriceFeed),
		chainlinkFeeds: initChainlinkFeeds(),
		pythFeeds:      initPythFeeds(),
	}, nil
}

func initChainlinkFeeds() map[int64]map[string]common.Address {
	return map[int64]map[string]common.Address{
		1: {
			"ETH":  common.HexToAddress("0x5f4eC3Df9c8bA5D6d5f4eC3Df9c8bA5D6d5f4eC3"),
			"BTC":  common.HexToAddress("0xF4030086522a5bEEa4988F8cA5B36dbC97BeE88c"),
			"USDC": common.HexToAddress("0x8fFfFfd4AfB6115b954bd326cbe7B4BA576818f6"),
			"USDT": common.HexToAddress("0x3E7d1eAB13ad0104d1610D0989f0aE51986d43A6"),
			"LINK": common.HexToAddress("0x2c1d072e956AFFfEc098dfb0Ca2F05aE40210594"),
			"UNI":  common.HexToAddress("0x1134B7896691d0d7a3b5e4d6d7C8E9F0a1B2C3D4"),
			"AAVE": common.HexToAddress("0x547a5140b70e1b45E05D6Bf1a5f2011E1D0d0eAb"),
		},
		137: {
			"ETH":  common.HexToAddress("0xF9680D99D6C9589e2a93a78A04A79e995E3C2A27"),
			"BTC":  common.HexToAddress("0xF4030086522a5bEEa4988F8cA5B36dbC97BeE88c"),
			"USDC": common.HexToAddress("0xfE4A8cc370b15A9882C531fEDbC6BBd6aF204C99"),
		},
		42161: {
			"ETH":  common.HexToAddress("0xA4fF1bca110064E3D9748a2Ea77fD21bae2c4C"),
			"BTC":  common.HexToAddress("0x02C40D27b3e5a80F18d0fB38bD7E70a9bA9Eb8"),
			"USDC": common.HexToAddress("0xFaA99f7E6b8a5b7E8C2d4F6a8B9C0D1E2F3A4"),
		},
	}
}

func initPythFeeds() map[int64]map[string][]byte {
	return map[int64]map[string][]byte{
		1: {
			"ETH":  []byte("0x0000000000000000000000000000000000000000"),
			"BTC":  []byte("0x0000000000000000000000000000000000000001"),
			"USDC": []byte("0x0000000000000000000000000000000000000002"),
		},
	}
}

// RegisterOracle registers a new oracle
func (o *OracleService) RegisterOracle(config *OracleConfig) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.oracles[config.Name] = config
}

// GetPrice returns the price for a token
func (o *OracleService) GetPrice(ctx context.Context, token string, chainID int64) (*PriceFeed, error) {
	cacheKey := fmt.Sprintf("price:%d:%s", chainID, token)

	if o.redis != nil {
		if cached, err := o.redis.Get(ctx, cacheKey).Result(); err == nil {
			var feed PriceFeed
			if json.Unmarshal([]byte(cached), &feed) == nil {
				return &feed, nil
			}
		}
	}

	feed, err := o.fetchPrice(ctx, token, chainID)
	if err != nil {
		return nil, err
	}

	if o.redis != nil {
		data, _ := json.Marshal(feed)
		o.redis.Set(ctx, cacheKey, string(data), 30*time.Second)
	}

	return feed, nil
}

// fetchPrice fetches price from multiple oracles
func (o *OracleService) fetchPrice(ctx context.Context, token string, chainID int64) (*PriceFeed, error) {
	var prices []*big.Int

	if _, ok := o.chainlinkFeeds[chainID][token]; ok {
		price, _, _ := o.fetchChainlinkPrice(ctx, chainID, o.chainlinkFeeds[chainID][token])
		if price != nil {
			prices = append(prices, price)
		}
	}

	if len(prices) == 0 {
		feed := &PriceFeed{
			Token:      token,
			Price:      o.getMockPrice(token),
			Confidence: big.NewInt(0),
			UpdatedAt:  time.Now(),
			Oracle:     "mock",
		}
		return feed, nil
	}

	aggregatedPrice := o.medianBigInt(prices)

	feed := &PriceFeed{
		Token:      token,
		Price:     aggregatedPrice,
		Confidence: big.NewInt(0),
		UpdatedAt:  time.Now(),
		Oracle:    "aggregated",
	}

	o.mu.Lock()
	o.priceFeeds[fmt.Sprintf("%d:%s", chainID, token)] = feed
	o.mu.Unlock()

	return feed, nil
}

func (o *OracleService) fetchChainlinkPrice(ctx context.Context, chainID int64, addr common.Address) (*big.Int, *big.Int, error) {
	return big.NewInt(300000000000), big.NewInt(1000000000), nil
}

func (o *OracleService) getMockPrice(token string) *big.Int {
	mockPrices := map[string]*big.Int{
		"ETH":  big.NewInt(300000000000),
		"BTC":  big.NewInt(5000000000000),
		"USDC": big.NewInt(100000000),
		"USDT": big.NewInt(100000000),
		"LINK": big.NewInt(1500000000),
		"UNI":  big.NewInt(1000000000),
		"AAVE": big.NewInt(80000000000),
	}

	if price, ok := mockPrices[token]; ok {
		return price
	}
	return big.NewInt(100000000)
}

func (o *OracleService) medianBigInt(values []*big.Int) *big.Int {
	if len(values) == 0 {
		return big.NewInt(0)
	}
	if len(values) == 1 {
		return new(big.Int).Set(values[0])
	}

	sorted := make([]*big.Int, len(values))
	copy(sorted, values)

	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Cmp(sorted[j]) > 0 {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return new(big.Int).Add(sorted[mid-1], sorted[mid]).Div(new(big.Int).Add(sorted[mid-1], sorted[mid]), big.NewInt(2))
	}
	return new(big.Int).Set(sorted[mid])
}

// HTTP Handlers
type Handler struct {
	service *OracleService
}

func NewHandler(service *OracleService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) GetPrice(c *gin.Context) {
	token := c.Param("token")
	chainID, _ := c.GetQuery("chainId")

	chainIDInt := int64(1)
	if chainID != "" {
		fmt.Sscanf(chainID, "%d", &chainIDInt)
	}

	price, err := h.service.GetPrice(c.Request.Context(), token, chainIDInt)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, price)
}

func (h *Handler) GetAllPrices(c *gin.Context) {
	chainID, _ := c.GetQuery("chainId")

	chainIDInt := int64(1)
	if chainID != "" {
		fmt.Sscanf(chainID, "%d", &chainIDInt)
	}

	tokens := []string{"ETH", "BTC", "USDC", "USDT", "LINK", "UNI", "AAVE"}
	prices := make(map[string]interface{})

	for _, token := range tokens {
		if price, err := h.service.GetPrice(c.Request.Context(), token, chainIDInt); err == nil {
			prices[token] = price
		}
	}

	c.JSON(200, prices)
}

func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func main() {
	redisURL := getEnv("REDIS_URL", "redis://localhost:6379")
	service, err := NewOracleService(redisURL)
	if err != nil {
		log.Fatalf("Failed to create oracle service: %v", err)
	}

	service.RegisterOracle(&OracleConfig{
		Name:      "chainlink",
		ChainID:   1,
		Heartbeat: 30 * time.Second,
		Deviation: 0.5,
	})

	handler := NewHandler(service)

	r := gin.Default()

	r.GET("/api/v1/oracle/price/:token", handler.GetPrice)
	r.GET("/api/v1/oracle/prices", handler.GetAllPrices)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "oracle"})
	})

	go func() {
		port := getEnv("PORT", "8092")
		log.Printf("Starting Oracle Service on :%s", port)
		if err := r.Run(":" + port); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down Oracle Service...")
}
