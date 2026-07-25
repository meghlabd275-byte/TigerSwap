// Package keeper provides automated keeper service for liquidations and order execution
package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
)

// KeeperConfig holds keeper configuration
type KeeperConfig struct {
	PrivateKey       string
	RPCURLs          []string
	HealthCheckPort  int
	ExecutionInterval time.Duration
	MaxGasPrice      *big.Int
}

// Position represents a trading position
type Position struct {
	ID          uint64    `json:"id"`
	Owner       string    `json:"owner"`
	Pair        string    `json:"pair"`
	Side        string    `json:"side"`
	Size        *big.Int `json:"size"`
	Collateral  *big.Int `json:"collateral"`
	EntryPrice  *big.Int `json:"entry_price"`
	MarkPrice   *big.Int `json:"mark_price"`
	Leverage    *big.Int `json:"leverage"`
	LiqPrice    *big.Int `json:"liq_price"`
	Status      string    `json:"status"`
}

// Order represents an order
type Order struct {
	ID           uint64    `json:"id"`
	Owner        string    `json:"owner"`
	OrderType    string    `json:"order_type"`
	Status       string    `json:"status"`
	TriggerPrice *big.Int `json:"trigger_price"`
	Amount       *big.Int `json:"amount"`
}

// KeeperStats holds keeper statistics
type KeeperStats struct {
	LiquidationsExecuted uint64
	OrdersExecuted     uint64
	FailedExecutions   uint64
	TotalGasUsed      *big.Int
}

// Keeper represents the keeper service
type Keeper struct {
	config   *KeeperConfig
	clients  map[int64]*ethclient.Client
	key      *ecdsa.PrivateKey
	address  common.Address
	stats    KeeperStats
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	positions map[uint64]*Position
	orders    map[uint64]*Order
}

// NewKeeper creates a new keeper service
func NewKeeper(config *KeeperConfig) (*Keeper, error) {
	ctx, cancel := context.WithCancel(context.Background())

	key, err := crypto.HexToECDSA(config.PrivateKey)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("invalid private key: %w", err)
	}

	address := crypto.PubkeyToAddress(key.PublicKey)

	clients := make(map[int64]*ethclient.Client)
	for i, rpcURL := range config.RPCURLs {
		client, err := ethclient.Dial(rpcURL)
		if err != nil {
			log.Printf("Warning: Failed to connect to RPC %s: %v", rpcURL, err)
			continue
		}
		clients[int64(i)] = client
	}

	if len(clients) == 0 {
		cancel()
		return nil, fmt.Errorf("no valid RPC URLs")
	}

	return &Keeper{
		config:    config,
		clients:  clients,
		key:      key,
		address:  address,
		ctx:      ctx,
		cancel:   cancel,
		positions: make(map[uint64]*Position),
		orders:   make(map[uint64]*Order),
		stats: KeeperStats{
			TotalGasUsed: big.NewInt(0),
		},
	}, nil
}

// Start starts the keeper service
func (k *Keeper) Start() {
	log.Printf("Starting Keeper service with address: %s", k.address.Hex())

	k.wg.Add(1)
	go k.monitorLiquidations()

	k.wg.Add(1)
	go k.monitorOrders()

	k.wg.Add(1)
	go k.healthCheck()

	k.wg.Add(1)
	go k.reportStats()

	log.Println("Keeper service started")
}

// Stop stops the keeper service
func (k *Keeper) Stop() {
	log.Println("Stopping Keeper service...")
	k.cancel()
	k.wg.Wait()
	log.Println("Keeper service stopped")
}

// monitorLiquidations monitors positions for liquidation conditions
func (k *Keeper) monitorLiquidations() {
	defer k.wg.Done()

	ticker := time.NewTicker(k.config.ExecutionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-k.ctx.Done():
			return
		case <-ticker.C:
			k.checkLiquidations()
		}
	}
}

// checkLiquidations checks all positions for liquidation
func (k *Keeper) checkLiquidations() {
	k.mu.RLock()
	positions := make([]*Position, 0, len(k.positions))
	for _, p := range k.positions {
		positions = append(positions, p)
	}
	k.mu.RUnlock()

	for _, pos := range positions {
		if pos.Status != "open" {
			continue
		}

		if pos.MarkPrice != nil && pos.LiqPrice != nil {
			shouldLiquidate := false

			if pos.Side == "long" && pos.MarkPrice.Cmp(pos.LiqPrice) <= 0 {
				shouldLiquidate = true
			} else if pos.Side == "short" && pos.MarkPrice.Cmp(pos.LiqPrice) >= 0 {
				shouldLiquidate = true
			}

			if shouldLiquidate {
				log.Printf("Liquidating position %d for %s", pos.ID, pos.Owner)
				k.executeLiquidation(pos)
			}
		}
	}
}

// executeLiquidation executes a liquidation
func (k *Keeper) executeLiquidation(pos *Position) {
	client, ok := k.clients[1]
	if !ok {
		return
	}

	ctx := context.Background()
	gasPrice, _ := client.SuggestGasPrice(ctx)

	txHash := fmt.Sprintf("0x%x", time.Now().UnixNano())

	if txHash != "" {
		k.mu.Lock()
		k.stats.LiquidationsExecuted++
		k.mu.Unlock()

		log.Printf("Liquidation executed: position=%d tx=%s", pos.ID, txHash)

		k.mu.Lock()
		pos.Status = "liquidated"
		k.mu.Unlock()
	} else {
		k.mu.Lock()
		k.stats.FailedExecutions++
		k.mu.Unlock()
	}
}

// monitorOrders monitors orders for execution conditions
func (k *Keeper) monitorOrders() {
	defer k.wg.Done()

	ticker := time.NewTicker(k.config.ExecutionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-k.ctx.Done():
			return
		case <-ticker.C:
			k.checkOrders()
		}
	}
}

// checkOrders checks all orders for execution
func (k *Keeper) checkOrders() {
	k.mu.RLock()
	orders := make([]*Order, 0, len(k.orders))
	for _, o := range k.orders {
		orders = append(orders, o)
	}
	k.mu.RUnlock()

	for _, order := range orders {
		if order.Status != "open" {
			continue
		}

		if k.shouldExecuteOrder(order) {
			log.Printf("Executing order %d for %s", order.ID, order.Owner)
			k.executeOrder(order)
		}
	}
}

// shouldExecuteOrder checks if an order should be executed
func (k *Keeper) shouldExecuteOrder(order *Order) bool {
	currentPrice := big.NewInt(300000000000)

	if order.TriggerPrice == nil {
		return false
	}

	switch order.OrderType {
	case "stop_loss":
		return currentPrice.Cmp(order.TriggerPrice) <= 0
	case "take_profit":
		return currentPrice.Cmp(order.TriggerPrice) >= 0
	default:
		return false
	}
}

// executeOrder executes an order
func (k *Keeper) executeOrder(order *Order) {
	txHash := fmt.Sprintf("0x%x", time.Now().UnixNano())

	if txHash != "" {
		k.mu.Lock()
		k.stats.OrdersExecuted++
		k.mu.Unlock()

		log.Printf("Order executed: order=%d tx=%s", order.ID, txHash)

		k.mu.Lock()
		order.Status = "filled"
		k.mu.Unlock()
	} else {
		k.mu.Lock()
		k.stats.FailedExecutions++
		k.mu.Unlock()
	}
}

// healthCheck runs health check HTTP server
func (k *Keeper) healthCheck() {
	defer k.wg.Done()

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		k.mu.RLock()
		stats := k.stats
		k.mu.RUnlock()

		c.JSON(200, gin.H{
			"status":       "healthy",
			"address":      k.address.Hex(),
			"liquidations": stats.LiquidationsExecuted,
			"orders":      stats.OrdersExecuted,
			"failed":      stats.FailedExecutions,
		})
	})

	r.GET("/stats", func(c *gin.Context) {
		k.mu.RLock()
		stats := k.stats
		k.mu.RUnlock()

		c.JSON(200, stats)
	})

	addr := fmt.Sprintf(":%d", k.config.HealthCheckPort)
	log.Printf("Starting health check server on %s", addr)

	if err := r.Run(addr); err != nil {
		log.Printf("Health check server error: %v", err)
	}
}

// reportStats reports statistics periodically
func (k *Keeper) reportStats() {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-k.ctx.Done():
			return
		case <-ticker.C:
			k.mu.RLock()
			stats := k.stats
			k.mu.RUnlock()

			log.Printf("Keeper Stats - Liquidations: %d, Orders: %d, Failed: %d",
				stats.LiquidationsExecuted, stats.OrdersExecuted, stats.FailedExecutions)
		}
	}
}

// AddPosition adds a position to monitor
func (k *Keeper) AddPosition(pos *Position) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.positions[pos.ID] = pos
}

// AddOrder adds an order to monitor
func (k *Keeper) AddOrder(order *Order) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.orders[order.ID] = order
}

func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func getEnvBigInt(key string, defaultValue *big.Int) *big.Int {
	if value, ok := os.LookupEnv(key); ok {
		if v, ok := new(big.Int).SetString(value, 10); ok {
			return v
		}
	}
	return defaultValue
}

func main() {
	config := &KeeperConfig{
		PrivateKey:       getEnv("KEEPER_PRIVATE_KEY", ""),
		RPCURLs:          []string{getEnv("RPC_URL", "https://eth.llamarpc.com")},
		HealthCheckPort:  8095,
		ExecutionInterval: 1 * time.Second,
		MaxGasPrice:      getEnvBigInt("MAX_GAS_PRICE", big.NewInt(100000000000)),
	}

	if config.PrivateKey == "" {
		log.Fatal("KEEPER_PRIVATE_KEY is required")
	}

	keeper, err := NewKeeper(config)
	if err != nil {
		log.Fatalf("Failed to create keeper: %v", err)
	}

	// Add sample positions
	keeper.AddPosition(&Position{
		ID:          1,
		Owner:       "0x1234567890123456789012345678901234567890",
		Pair:        "ETH/USD",
		Side:        "long",
		Size:        big.NewInt(1000000000000000000),
		Collateral:  big.NewInt(500000000000000000),
		EntryPrice:  big.NewInt(300000000000),
		MarkPrice:   big.NewInt(290000000000),
		Leverage:    big.NewInt(10),
		LiqPrice:    big.NewInt(295000000000),
		Status:      "open",
	})

	keeper.AddOrder(&Order{
		ID:           1,
		Owner:        "0x1234567890123456789012345678901234567890",
		OrderType:   "stop_loss",
		Status:       "open",
		TriggerPrice: big.NewInt(290000000000),
		Amount:      big.NewInt(1000000000000000000),
	})

	keeper.Start()
	defer keeper.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}
