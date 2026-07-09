package indexer

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/tidwall/gjson"
)

// Indexer handles blockchain event indexing
type Indexer struct {
	// Ethereum client
	client *ethclient.Client
	rpcClient *rpc.Client

	// Database
	db *sqlx.DB

	// Redis
	redis *redis.Client

	// Configuration
	startBlock uint64
	batchSize int
	workers int

	// State
	currentBlock uint64
	isRunning bool
	mu sync.RWMutex

	// Contracts
	contracts map[string]ContractConfig

	// Channels
	stopChan chan struct{}
}

// ContractConfig holds contract information
type ContractConfig {
	Address common.Address
	ABI abi.ABI
	Events []EventConfig
}

// EventConfig holds event information
type EventConfig {
	Name string
	ID common.Hash
}

// SwapEvent represents a token swap
type SwapEvent struct {
	ID int64 `db:"id"`
	TransactionHash string `db:"transaction_hash"`
	BlockNumber int64 `db:"block_number"`
	Timestamp time.Time `db:"timestamp"`
	PairAddress string `db:"pair_address"`
	Sender string `db:"sender"`
	FromToken string `db:"from_token"`
	ToToken string `db:"to_token"`
	FromAmount string `db:"from_amount"`
	ToAmount string `db:"to_amount"`
	LogIndex uint64 `db:"log_index"`
}

// MintEvent represents liquidity mint
type MintEvent struct {
	ID int64 `db:"id"`
	TransactionHash string `db:"transaction_hash"`
	BlockNumber int64 `db:"block_number"`
	Timestamp time.Time `db:"timestamp"`
	PairAddress string `db:"pair_address"`
	Owner string `db:"owner"`
	Amount0 string `db:"amount0"`
	Amount1 string `db:"amount1"`
}

// BurnEvent represents liquidity burn
type BurnEvent struct {
	ID int64 `db:"id"`
	TransactionHash string `db:"transaction_hash"`
	BlockNumber int64 `db:"block_number"`
	Timestamp time.Time `db:"timestamp"`
	PairAddress string `db:"pair_address"`
	Owner string `db:"owner"`
	Amount0 string `db:"amount0"`
	Amount1 string `db:"amount1"`
}

// TransferEvent represents token transfer
type TransferEvent struct {
	ID int64 `db:"id"`
	TransactionHash string `db:"transaction_hash"`
	BlockNumber int64 `db:"block_number"`
	Timestamp time.Time `db:"timestamp"`
	TokenAddress string `db:"token_address"`
	From string `db:"from_address"`
	To string `db:"to_address"`
	Amount string `db:"amount"`
	LogIndex uint64 `db:"log_index"`
}

// NewIndexer creates a new indexer
func NewIndexer(rpcURL, dsn, redisAddr string) *Indexer {
	// Connect to Ethereum
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatalf("Failed to connect to Ethereum: %v", err)
	}

	rpcClient, err := rpc.Dial(rpcURL)
	if err != nil {
		log.Fatalf("Failed to connect to RPC: %v", err)
	}

	// Connect to database
	db, err := sqlx.Connect("postgres", dsn)
	if err != nil {
		log.Printf("Warning: Failed to connect to database: %v", err)
	}

	// Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	// Create indexer
	idx := &Indexer{
		client: client,
		rpcClient: rpcClient,
		db: db,
		redis: rdb,
		startBlock: 18000000,
		batchSize: 1000,
		workers: 4,
		contracts: make(map[string]ContractConfig),
		stopChan: make(chan struct{}),
	}

	// Initialize contracts
	idx.initContracts()

	// Initialize database schema
	idx.initSchema()

	return idx
}

// initContracts initializes contract configurations
func (idx *Indexer) initContracts() {
	// Uniswap V2 Factory ABI (simplified)
	uniswapV2FactoryABI := `[{"type":"event","name":"PairCreated","inputs":[{"name":"token0","type":"address","indexed":true},{"name":"token1","type":"address","indexed":true},{"name":"pair","type":"address","indexed":false},{"name":"","type":"uint256","indexed":false}]}]`

	// Uniswap V2 Pair ABI
	uniswapV2PairABI := `[{"type":"event","name":"Swap","inputs":[{"name":"sender","type":"address","indexed":false},{"name":"amount0In","type":"uint256","indexed":false},{"name":"amount1In","type":"uint256","indexed":false},{"name":"amount0Out","type":"uint256","indexed":false},{"name":"amount1Out","type":"uint256","indexed":false},{"name":"to","type":"address","indexed":false}]},{"type":"event","name":"Mint","inputs":[{"name":"sender","type":"address","indexed":false},{"name":"amount0","type":"uint256","indexed":false},{"name":"amount1","type":"uint256","indexed":false}]},{"type":"event","name":"Burn","inputs":[{"name":"sender","type":"address","indexed":false},{"name":"amount0","type":"uint256","indexed":false},{"name":"amount1","type":"uint256","indexed":false},{"name":"to","type":"address","indexed":false}]}]`

	// ERC20 Transfer ABI
	erc20ABI := `[{"type":"event","name":"Transfer","inputs":[{"name":"from","type":"address","indexed":true},{"name":"to","type":"address","indexed":true},{"name":"value","type":"uint256","indexed":false}]}]`

	// Parse ABIs
	uniswapFactory, _ := abi.JSON(strings.NewReader(uniswapV2FactoryABI))
	uniswapPair, _ := abi.JSON(strings.NewReader(uniswapV2PairABI))
	erc20, _ := abi.JSON(strings.NewReader(erc20ABI))

	// Uniswap V2 Factory
	idx.contracts["uniswap_v2_factory"] = ContractConfig{
		Address: common.HexToAddress("0x5C69bEe701ef814a2B6a3EDD4B1652aB1bA38aD9"),
		ABI: uniswapFactory,
		Events: []EventConfig{
			{Name: "PairCreated", ID: common.HexToHash("0x0d3648bd0f6ba80134a2b6433c71f4f500a6e5c8e4e2c8e4e4e4e4e4e4e4e4e")},
		},
	}

	// ERC20
	idx.contracts["erc20"] = ContractConfig{
		Address: common.Address{},
		ABI: erc20,
		Events: []EventConfig{
			{Name: "Transfer", ID: common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")},
		},
	}
}

// initSchema initializes the database schema
func (idx *Indexer) initSchema() {
	if idx.db == nil {
		return
	}

	schema := `
	CREATE TABLE IF NOT EXISTS swap_events (
		id SERIAL PRIMARY KEY,
		transaction_hash VARCHAR(66) NOT NULL,
		block_number BIGINT NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		pair_address VARCHAR(42) NOT NULL,
		sender VARCHAR(42),
		from_token VARCHAR(42),
		to_token VARCHAR(42),
		from_amount VARCHAR(78),
		to_amount VARCHAR(78),
		log_index BIGINT,
		UNIQUE(transaction_hash, log_index)
	);

	CREATE TABLE IF NOT EXISTS mint_events (
		id SERIAL PRIMARY KEY,
		transaction_hash VARCHAR(66) NOT NULL,
		block_number BIGINT NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		pair_address VARCHAR(42) NOT NULL,
		owner VARCHAR(42),
		amount0 VARCHAR(78),
		amount1 VARCHAR(78),
		UNIQUE(transaction_hash)
	);

	CREATE TABLE IF NOT EXISTS burn_events (
		id SERIAL PRIMARY KEY,
		transaction_hash VARCHAR(66) NOT NULL,
		block_number BIGINT NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		pair_address VARCHAR(42) NOT NULL,
		owner VARCHAR(42),
		amount0 VARCHAR(78),
		amount1 VARCHAR(78),
		UNIQUE(transaction_hash)
	);

	CREATE TABLE IF NOT EXISTS transfer_events (
		id SERIAL PRIMARY KEY,
		transaction_hash VARCHAR(66) NOT NULL,
		block_number BIGINT NOT NULL,
		timestamp TIMESTAMP NOT NULL,
		token_address VARCHAR(42) NOT NULL,
		from_address VARCHAR(42),
		to_address VARCHAR(42),
		amount VARCHAR(78),
		log_index BIGINT,
		UNIQUE(transaction_hash, log_index)
	);

	CREATE TABLE IF NOT EXISTS indexed_blocks (
		block_number BIGINT PRIMARY KEY,
		timestamp TIMESTAMP NOT NULL,
		indexed_at TIMESTAMP DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_swap_pair ON swap_events(pair_address);
	CREATE INDEX IF NOT EXISTS idx_swap_timestamp ON swap_events(timestamp);
	CREATE INDEX IF NOT EXISTS idx_transfer_token ON transfer_events(token_address);
	`

	_, err := idx.db.Exec(schema)
	if err != nil {
		log.Printf("Warning: Failed to initialize schema: %v", err)
	}
}

// Start starts the indexer
func (idx *Indexer) Start(ctx context.Context) error {
	idx.mu.Lock()
	if idx.isRunning {
		idx.mu.Unlock()
		return fmt.Errorf("indexer already running")
	}
	idx.isRunning = true
	idx.mu.Unlock()

	// Get current block
	header, err := idx.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to get current block: %v", err)
	}

	idx.currentBlock = header.Number.Uint64()
	log.Printf("Current block: %d", idx.currentBlock)

	// Start block processing
	go idx.processBlocks(ctx)

	// Start event subscription
	go idx.subscribeEvents(ctx)

	return nil
}

// Stop stops the indexer
func (idx *Indexer) Stop() {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if !idx.isRunning {
		return
	}

	idx.isRunning = false
	close(idx.stopChan)
}

// processBlocks processes blocks in batches
func (idx *Indexer) processBlocks(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-idx.stopChan:
			return
		case <-ticker.C:
			if err := idx.processBatch(ctx); err != nil {
				log.Printf("Error processing batch: %v", err)
			}
		}
	}
}

// processBatch processes a batch of blocks
func (idx *Indexer) processBatch(ctx context.Context) error {
	header, err := idx.client.HeaderByNumber(ctx, nil)
	if err != nil {
		return err
	}

	currentBlock := header.Number.Uint64()

	// Process up to batchSize blocks
	endBlock := idx.currentBlock + uint64(idx.batchSize)
	if endBlock > currentBlock {
		endBlock = currentBlock
	}

	if endBlock <= idx.currentBlock {
		return nil
	}

	log.Printf("Processing blocks %d to %d", idx.currentBlock, endBlock)

	// Query logs for each contract
	for name, config := range idx.contracts {
		if config.Address == (common.Address{}) {
			continue // Skip contracts without address
		}

		logs, err := idx.client.FilterLogs(ctx, ethereum.FilterQuery{
			FromBlock: big.NewInt(int64(idx.currentBlock)),
			ToBlock: big.NewInt(int64(endBlock)),
			Addresses: []common.Address{config.Address},
		})

		if err != nil {
			log.Printf("Error filtering logs for %s: %v", name, err)
			continue
		}

		// Process logs
		for _, log := range logs {
			idx.processLog(ctx, log)
		}
	}

	// Update current block
	idx.currentBlock = endBlock

	// Cache block in Redis
	if idx.redis != nil {
		idx.redis.Set(ctx, "indexer:block", endBlock, 0)
	}

	return nil
}

// processLog processes a single log
func (idx *Indexer) processLog(ctx context.Context, log types.Log) {
	// Get block timestamp
	block, err := idx.client.BlockByNumber(ctx, big.NewInt(int64(log.BlockNumber)))
	if err != nil {
		log.Printf("Error getting block: %v", err)
		return
	}

	timestamp := time.Unix(int64(block.Time()), 0)

	// Try to parse as Swap event
	if len(log.Data) >= 96 {
		// This is a simplified check - in production, match against event signatures
		swap := SwapEvent{
			TransactionHash: log.TxHash.Hex(),
			BlockNumber: log.BlockNumber,
			Timestamp: timestamp,
			PairAddress: log.Address.Hex(),
			LogIndex: log.Index,
		}

		// Parse swap data (simplified)
		if len(log.Data) >= 32 {
			swap.FromAmount = new(big.Int).SetBytes(log.Data[0:32]).String()
		}
		if len(log.Data) >= 64 {
			swap.ToAmount = new(big.Int).SetBytes(log.Data[32:64]).String()
		}

		// Store in database
		if idx.db != nil {
			_, err := idx.db.Exec(`
				INSERT INTO swap_events (transaction_hash, block_number, timestamp, pair_address, from_amount, to_amount, log_index)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (transaction_hash, log_index) DO NOTHING`,
				swap.TransactionHash, swap.BlockNumber, swap.Timestamp,
				swap.PairAddress, swap.FromAmount, swap.ToAmount, swap.LogIndex)
			if err != nil {
				log.Printf("Error storing swap: %v", err)
			}
		}

		// Publish to Redis
		if idx.redis != nil {
			idx.redis.Publish(ctx, "events:swap", swap)
		}
	}
}

// subscribeEvents subscribes to new blocks
func (idx *Indexer) subscribeEvents(ctx context.Context) {
	headers := make(chan *types.Header)

	sub, err := idx.client.SubscribeNewHead(ctx, headers)
	if err != nil {
		log.Printf("Error subscribing to new heads: %v", err)
		return
	}
	defer sub.Unsubscribe()

	for {
		select {
		case <-ctx.Done():
			return
		case <-idx.stopChan:
			return
		case header := <-headers:
			if header == nil {
				continue
			}

			// Process new block
			go idx.processBlock(ctx, header.Number.Uint64())
		}
	}
}

// processBlock processes a single block
func (idx *Indexer) processBlock(ctx context.Context, blockNumber uint64) {
	// Get block
	block, err := idx.client.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		log.Printf("Error getting block %d: %v", blockNumber, err)
		return
	}

	// Process transactions
	for _, tx := range block.Transactions() {
		idx.processTransaction(ctx, tx, block)
	}
}

// processTransaction processes a transaction
func (idx *Indexer) processTransaction(ctx context.Context, tx *types.Transaction, block *types.Block) {
	// Skip contract creations
	if tx.To() == nil {
		return
	}

	// Get receipt
	receipt, err := idx.client.TransactionReceipt(ctx, tx.Hash())
	if err != nil {
		return
	}

	// Process logs
	for _, log := range receipt.Logs {
		idx.processLog(ctx, *log)
	}
}

// GetSwapEvents returns swap events for a pair
func (idx *Indexer) GetSwapEvents(ctx context.Context, pair string, startTime, endTime time.Time) ([]SwapEvent, error) {
	if idx.db == nil {
		return nil, fmt.Errorf("database not connected")
	}

	var events []SwapEvent
	err := db.Select(&events, `
		SELECT * FROM swap_events
		WHERE pair_address = $1 AND timestamp BETWEEN $2 AND $3
		ORDER BY timestamp DESC
		LIMIT 1000`,
		pair, startTime, endTime)

	return events, err
}

// GetVolume returns 24h volume for a pair
func (idx *Indexer) GetVolume(ctx context.Context, pair string) (string, error) {
	if idx.redis != nil {
		volume, err := idx.redis.Get(ctx, fmt.Sprintf("volume:%s", pair)).Result()
		if err == nil {
			return volume, nil
		}
	}

	return "0", nil
}
