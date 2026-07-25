// Package indexer provides blockchain indexing service for TigerSwap
// Real-time event processing and historical data indexing
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// IndexerConfig holds indexer configuration
type IndexerConfig struct {
	RPCURLs          []string
	StartBlock       uint64
	Contracts        []ContractConfig
	RedisURL         string
	HealthCheckPort  int
	ConfirmationDepth uint64
}

// ContractConfig holds contract configuration
type ContractConfig struct {
	Name    string
	Address common.Address
	ABI     string
	Events  []string
}

// IndexedEvent represents an indexed event
type IndexedEvent struct {
	ID          uint64          `json:"id"`
	Contract    string          `json:"contract"`
	Event      string          `json:"event"`
	TransactionHash string     `json:"transaction_hash"`
	BlockNumber uint64        `json:"block_number"`
	BlockHash   string        `json:"block_hash"`
	Timestamp   time.Time      `json:"timestamp"`
	LogIndex    uint           `json:"log_index"`
	Data        map[string]interface{} `json:"data"`
}

// Swap represents a swap event data
type Swap struct {
	ID              uint64    `json:"id"`
	TransactionHash string   `json:"transaction_hash"`
	BlockNumber    uint64    `json:"block_number"`
	Timestamp      time.Time `json:"timestamp"`
	Sender          string    `json:"sender"`
	FromToken      string    `json:"from_token"`
	ToToken        string    `json:"to_token"`
	FromAmount     string    `json:"from_amount"`
	ToAmount       string    `json:"to_amount"`
	FromAddress    string    `json:"from_address"`
	ToAddress      string    `json:"to_address"`
	GasUsed        uint64    `json:"gas_used"`
	GasPrice       string    `json:"gas_price"`
}

// LiquidityChange represents liquidity events
type LiquidityChange struct {
	ID              uint64    `json:"id"`
	TransactionHash string   `json:"transaction_hash"`
	BlockNumber    uint64    `json:"block_number"`
	Timestamp      time.Time `json:"timestamp"`
	Pair           string    `json:"pair"`
	LPToken        string    `json:"lp_token"`
	Provider       string    `json:"provider"`
	Amount0        string    `json:"amount_0"`
	Amount1        string    `json:"amount_1"`
	Liquidity      string    `json:"liquidity"`
	Action         string    `json:"action"` // add or remove
}

// IndexerStats holds indexer statistics
type IndexerStats struct {
	EventsProcessed   uint64
	SwapsIndexed     uint64
	LiquidityIndexed uint64
	LastBlockProcessed uint64
	LastUpdate       time.Time
}

// Indexer represents the blockchain indexer
type Indexer struct {
	config   *IndexerConfig
	client   *ethclient.Client
	redis    *redis.Client
	abis    map[string]abi.ABI
	stats    IndexerStats
	mu       sync.RWMutex
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	isRunning bool
	contracts map[string]common.Address
}

// NewIndexer creates a new indexer service
func NewIndexer(config *IndexerConfig) (*Indexer, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Connect to Ethereum client
	client, err := ethclient.Dial(config.RPCURLs[0])
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	// Connect to Redis
	redisOpts, _ := redis.ParseURL(config.RedisURL)
	redisClient := redis.NewClient(redisOpts)

	// Test Redis connection
	testCtx, testCancel := context.WithTimeout(ctx, 5*time.Second)
	defer testCancel()
	if err := redisClient.Ping(testCtx).Err(); err != nil {
		log.Printf("Warning: Redis connection failed: %v", err)
	}

	// Parse ABIs
	abis := make(map[string]abi.ABI)
	contracts := make(map[string]common.Address)

	for _, c := range config.Contracts {
		parsedABI, err := abi.JSON(strings.NewReader(c.ABI))
		if err != nil {
			log.Printf("Warning: Failed to parse ABI for %s: %v", c.Name, err)
			continue
		}
		abis[c.Name] = parsedABI
		contracts[c.Name] = c.Address
		log.Printf("Registered contract: %s at %s", c.Name, c.Address.Hex())
	}

	return &Indexer{
		config:    config,
		client:   client,
		redis:   redisClient,
		abis:    abis,
		contracts: contracts,
		ctx:     ctx,
		cancel:  cancel,
		stats:   IndexerStats{},
	}, nil
}

// Start starts the indexer service
func (i *Indexer) Start() {
	log.Println("Starting Indexer service...")

	// Get current block number
	header, err := i.client.HeaderByNumber(i.ctx, nil)
	if err != nil {
		log.Printf("Warning: Failed to get current block: %v", err)
	} else {
		log.Printf("Current block: %d", header.Number.Uint64())
	}

	i.mu.Lock()
	i.isRunning = true
	i.mu.Unlock()

	// Start block processing
	i.wg.Add(1)
	go i.processBlocks()

	// Start HTTP server
	i.wg.Add(1)
	go i.healthCheck()

	log.Println("Indexer service started")
}

// Stop stops the indexer service
func (i *Indexer) Stop() {
	log.Println("Stopping Indexer service...")

	i.mu.Lock()
	i.isRunning = false
	i.mu.Unlock()

	i.cancel()
	i.wg.Wait()

	if i.redis != nil {
		i.redis.Close()
	}

	log.Println("Indexer service stopped")
}

// processBlocks processes new blocks
func (i *Indexer) processBlocks() {
	defer i.wg.Done()

	// Get last processed block from Redis
	lastBlock := i.getLastProcessedBlock()

	// Subscribe to new heads
	headers := make(chan *types.Header)

	sub, err := i.client.SubscribeNewHead(i.ctx, headers)
	if err != nil {
		log.Printf("Warning: Failed to subscribe to new heads: %v", err)
		// Fall back to polling
		i.pollBlocks(lastBlock)
		return
	}
	defer sub.Unsubscribe()

	log.Printf("Subscribed to new heads from block %d", lastBlock)

	for {
		select {
		case <-i.ctx.Done():
			return
		case header := <-headers:
			if header == nil {
				continue
			}

			blockNum := header.Number.Uint64()

			// Process if block is confirmed
			if blockNum > i.config.ConfirmationDepth {
				processBlockNum := blockNum - i.config.ConfirmationDepth

				if processBlockNum > lastBlock {
					i.processBlock(processBlockNum)
					lastBlock = processBlockNum

					i.mu.Lock()
					i.stats.LastBlockProcessed = processBlockNum
					i.stats.LastUpdate = time.Now()
					i.mu.Unlock()

					i.setLastProcessedBlock(processBlockNum)
				}
			}
		}
	}
}

// pollBlocks polls for new blocks (fallback)
func (i *Indexer) pollBlocks(lastBlock uint64) {
	ticker := time.NewTicker(12 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-i.ctx.Done():
			return
		case <-ticker.C:
			header, err := i.client.HeaderByNumber(i.ctx, nil)
			if err != nil {
				continue
			}

			blockNum := header.Number.Uint64()

			if blockNum > i.config.ConfirmationDepth {
				processBlockNum := blockNum - i.config.ConfirmationDepth

				if processBlockNum > lastBlock {
					i.processBlock(processBlockNum)
					lastBlock = processBlockNum
				}
			}
		}
	}
}

// processBlock processes a single block
func (i *Indexer) processBlock(blockNum uint64) {
	log.Printf("Processing block %d", blockNum)

	// Get block with transactions
	block, err := i.client.BlockByNumber(i.ctx, big.NewInt(int64(blockNum)))
	if err != nil {
		log.Printf("Failed to get block %d: %v", blockNum, err)
		return
	}

	// Process each transaction receipt
	for _, tx := range block.Receipts() {
		for _, log := range tx.Logs {
			// Check if log is from one of our contracts
			for name, addr := range i.contracts {
				if log.Address == addr {
					i.processLog(log, block)
					break
				}
			}
		}
	}

	// Update stats
	i.mu.Lock()
	i.stats.EventsProcessed++
	i.mu.Unlock()
}

// processLog processes a single log
func (i *Indexer) processLog(logEntry *types.Log, block *types.Block) {
	// Find which contract
	var contractName string
	var contractAddr common.Address
	for name, addr := range i.contracts {
		if addr == logEntry.Address {
			contractName = name
			contractAddr = addr
			break
		}
	}

	if contractName == "" {
		return
	}

	// Get ABI
	parsedABI, ok := i.abis[contractName]
	if !ok {
		return
	}

	// Parse event
	event, err := parsedABI.EventByID(logEntry.Topics[0])
	if err != nil {
		return
	}

	// Parse event data
	data, err := event.Inputs.UnpackValues(logEntry.Data)
	if err != nil {
		log.Printf("Failed to unpack event data: %v", err)
		return
	}

	// Create indexed event
	indexedEvent := IndexedEvent{
		ID:              uint64(time.Now().UnixNano()),
		Contract:        contractName,
		Event:           event.Name,
		TransactionHash: logEntry.TxHash.Hex(),
		BlockNumber:     logEntry.BlockNumber,
		BlockHash:       logEntry.BlockHash.Hex(),
		Timestamp:       time.Unix(int64(block.Time()), 0),
		LogIndex:        logEntry.Index,
		Data:            make(map[string]interface{}),
	}

	// Store event data
	for idx, input := range event.Inputs {
		if idx < len(data) {
			indexedEvent.Data[input.Name] = data[idx]
		}
	}

	// Store in Redis
	i.storeEvent(&indexedEvent)

	// Process specific event types
	switch event.Name {
	case "Swap":
		i.processSwap(logEntry, &indexedEvent, block)
	case "Mint", "Burn":
		i.processLiquidity(logEntry, &indexedEvent, block)
	}

	i.mu.Lock()
	i.stats.EventsProcessed++
	i.mu.Unlock()
}

// processSwap processes swap events
func (i *Indexer) processSwap(logEntry *types.Log, event *IndexedEvent, block *types.Block) {
	swap := Swap{
		ID:              uint64(time.Now().UnixNano()),
		TransactionHash: event.TransactionHash,
		BlockNumber:    event.BlockNumber,
		Timestamp:      event.Timestamp,
	}

	// Extract data
	if sender, ok := event.Data["sender"].(common.Address); ok {
		swap.Sender = sender.Hex()
	}

	i.mu.Lock()
	i.stats.SwapsIndexed++
	i.mu.Unlock()

	log.Printf("Indexed swap: tx=%s block=%d", swap.TransactionHash, swap.BlockNumber)
}

// processLiquidity processes liquidity events
func (i *Indexer) processLiquidity(logEntry *types.Log, event *IndexedEvent, block *types.Block) {
	action := "add"
	if event.Event == "Burn" {
		action = "remove"
	}

	liquidity := LiquidityChange{
		ID:              uint64(time.Now().UnixNano()),
		TransactionHash: event.TransactionHash,
		BlockNumber:    event.BlockNumber,
		Timestamp:      event.Timestamp,
		Action:         action,
	}

	i.mu.Lock()
	i.stats.LiquidityIndexed++
	i.mu.Unlock()

	log.Printf("Indexed liquidity: tx=%s action=%s", liquidity.TransactionHash, action)
}

// storeEvent stores event in Redis
func (i *Indexer) storeEvent(event *IndexedEvent) {
	if i.redis == nil {
		return
	}

	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	key := fmt.Sprintf("events:%d:%s", event.BlockNumber, event.TransactionHash)
	i.redis.Set(i.ctx, key, string(data), 24*time.Hour)
}

// getLastProcessedBlock gets the last processed block from Redis
func (i *Indexer) getLastProcessedBlock() uint64 {
	if i.redis == nil {
		return i.config.StartBlock
	}

	val, err := i.redis.Get(i.ctx, "indexer:last_block").Result()
	if err != nil {
		return i.config.StartBlock
	}

	var block uint64
	fmt.Sscanf(val, "%d", &block)
	return block
}

// setLastProcessedBlock sets the last processed block in Redis
func (i *Indexer) setLastProcessedBlock(block uint64) {
	if i.redis == nil {
		return
	}

	i.redis.Set(i.ctx, "indexer:last_block", fmt.Sprintf("%d", block), 0)
}

// healthCheck runs health check HTTP server
func (i *Indexer) healthCheck() {
	defer i.wg.Done()

	r := gin.Default()

	r.GET("/health", func(c *gin.Context) {
		i.mu.RLock()
		stats := i.stats
		running := i.isRunning
		i.mu.RUnlock()

		c.JSON(200, gin.H{
			"status":            "healthy",
			"running":           running,
			"events_processed":  stats.EventsProcessed,
			"swaps_indexed":     stats.SwapsIndexed,
			"liquidity_indexed": stats.LiquidityIndexed,
			"last_block":        stats.LastBlockProcessed,
			"last_update":       stats.LastUpdate.Format(time.RFC3339),
		})
	})

	r.GET("/stats", func(c *gin.Context) {
		i.mu.RLock()
		stats := i.stats
		i.mu.RUnlock()

		c.JSON(200, stats)
	})

	addr := fmt.Sprintf(":%d", i.config.HealthCheckPort)
	log.Printf("Starting indexer health check on %s", addr)

	if err := r.Run(addr); err != nil {
		log.Printf("Health check server error: %v", err)
	}
}

// GetSwaps retrieves swaps for a given block range
func (i *Indexer) GetSwaps(ctx context.Context, startBlock, endBlock uint64) ([]Swap, error) {
	var swaps []Swap

	if i.redis == nil {
		return swaps, nil
	}

	for block := startBlock; block <= endBlock; block++ {
		pattern := fmt.Sprintf("events:%d:*", block)
		keys, err := i.redis.Keys(ctx, pattern).Result()
		if err != nil {
			continue
		}

		for _, key := range keys {
			data, err := i.redis.Get(ctx, key).Result()
			if err != nil {
				continue
			}

			var event IndexedEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue
			}

			if event.Event == "Swap" {
				swap := Swap{
					ID:              event.ID,
					TransactionHash: event.TransactionHash,
					BlockNumber:    event.BlockNumber,
					Timestamp:      event.Timestamp,
				}
				swaps = append(swaps, swap)
			}
		}
	}

	return swaps, nil
}

func getEnv(key, defaultValue string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return defaultValue
}

func getEnvUint64(key string, defaultValue uint64) uint64 {
	if value, ok := os.LookupEnv(key); ok != nil {
		var val uint64
		fmt.Sscanf(value, "%d", &val)
		if val > 0 {
			return val
		}
	}
	return defaultValue
}

func main() {
	config := &IndexerConfig{
		RPCURLs:          []string{getEnv("RPC_URL", "https://eth.llamarpc.com")},
		StartBlock:       getEnvUint64("START_BLOCK", 18000000),
		RedisURL:         getEnv("REDIS_URL", "redis://localhost:6379"),
		HealthCheckPort:  8096,
		ConfirmationDepth: 12,
		Contracts: []ContractConfig{
			{
				Name:    "TigerSwapRouter",
				Address: common.HexToAddress("0x000000000000000000000000000000000000000001"),
				ABI:     `[{"anonymous":false,"name":"Swap","inputs":[{"indexed":false,"name":"sender","type":"address"},{"indexed":false,"name":"amount0In","type":"uint256"},{"indexed":false,"name":"amount1In","type":"uint256"},{"indexed":false,"name":"amount0Out","type":"uint256"},{"indexed":false,"name":"amount1Out","type":"uint256"},{"indexed":true,"name":"to","type":"address"}]}]`,
				Events:  []string{"Swap"},
			},
		},
	}

	indexer, err := NewIndexer(config)
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
	}

	indexer.Start()
	defer indexer.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
}
