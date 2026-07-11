/**
 * TigerSwap Go Indexer Service
 * High-performance blockchain indexer for real-time data
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

const (
	MaxRetries          = 3
	BlockBatchSize      = 100
	EventBatchSize      = 1000
	ReorgDepth          = 10
	MaxConcurrentBlocks = 10
)

type Config struct {
	ChainID          uint64
	RPCURL           string
	StartBlock       uint64
	DatabaseURL      string
	RedisURL         string
	LogLevel         string
}

type Indexer struct {
	config        *Config
	client        *ethclient.Client
	db            *gorm.DB
	redis         *redis.Client
	contracts     map[string]*ContractABI
	blockCache    *BlockCache
	eventHandlers map[string]EventHandler
	mu            sync.RWMutex
	running       bool
	stopChan      chan struct{}
}

type ContractABI struct {
	Address common.Address
	Name    string
	ABI     abi.ABI
	Events  map[string]abi.Event
}

type BlockCache struct {
	blocks  map[uint64]*types.Block
	mu      sync.RWMutex
	maxSize int
}

type EventHandler func(ctx context.Context, log types.Log) error

type Block struct {
	ID          uint64    `gorm:"primaryKey"`
	Number      uint64    `gorm:"uniqueIndex"`
	Hash        string    `gorm:"index"`
	ParentHash  string
	Timestamp   uint64
	GasUsed     uint64
	CreatedAt   time.Time
}

type Event struct {
	ID              uint64    `gorm:"primaryKey"`
	LogIndex        uint64    `gorm:"index"`
	TransactionHash string    `gorm:"index"`
	BlockNumber     uint64    `gorm:"index"`
	Address         string    `gorm:"index"`
	Topic0          string
	Data            string
	Removed         bool
	Timestamp       uint64
	CreatedAt       time.Time
}

type Pool struct {
	ID          uint64    `gorm:"primaryKey"`
	Address     string    `gorm:"uniqueIndex"`
	Token0      string
	Token1      string
	Reserve0    string
	Reserve1    string
	LastUpdated uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Swap struct {
	ID          uint64    `gorm:"primaryKey"`
	Hash        string    `gorm:"uniqueIndex"`
	PoolAddress string    `gorm:"index"`
	TokenIn     string
	TokenOut    string
	AmountIn    string
	AmountOut   string
	BlockNumber uint64    `gorm:"index"`
	Timestamp   uint64
	CreatedAt   time.Time
}

func NewIndexer(config *Config) (*Indexer, error) {
	client, err := ethclient.Dial(config.RPCURL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RPC: %w", err)
	}

	db, err := gorm.Open(postgres.Open(config.DatabaseURL), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: config.RedisURL,
	})

	indexer := &Indexer{
		config:        config,
		client:        client,
		db:            db,
		redis:         rdb,
		contracts:     make(map[string]*ContractABI),
		blockCache:    NewBlockCache(1000),
		eventHandlers: make(map[string]EventHandler),
		stopChan:      make(chan struct{}),
	}

	if err := indexer.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize indexer: %w", err)
	}

	return indexer, nil
}

func (i *Indexer) initialize() error {
	log.Println("Initializing indexer...")
	err := i.db.AutoMigrate(&Block{}, &Event{}, &Pool{}, &Swap{})
	if err != nil {
		return fmt.Errorf("failed to migrate database: %w", err)
	}
	i.registerDefaultContracts()
	i.registerEventHandlers()
	log.Println("Indexer initialized successfully")
	return nil
}

func (i *Indexer) registerDefaultContracts() {
	factoryABI := `[{"anonymous":false,"inputs":[{"indexed":true,"internalType":"address","name":"token0","type":"address"},{"indexed":true,"internalType":"address","name":"token1","type":"address"},{"indexed":false,"internalType":"address","name":"pair","type":"address"}],"name":"PairCreated","type":"event"},{"inputs":[{"internalType":"address","name":"tokenA","type":"address"},{"internalType":"address","name":"tokenB","type":"address"}],"name":"createPair","outputs":[{"internalType":"address","name":"pair","type":"address"}],"stateMutability":"nonpayable","type":"function"}]`
	i.RegisterContract(common.HexToAddress("0x5C69bEe701ef814a2B6ae3C96E8bD4aC5b0bE7a6"), "UniswapV2Factory", factoryABI)
}

func (i *Indexer) RegisterContract(address common.Address, name string, abiJSON string) error {
	parsedABI, err := abi.JSON(nil)
	if err != nil {
		return fmt.Errorf("failed to parse ABI for %s: %w", name, err)
	}

	events := make(map[string]abi.Event)
	for _, e := range parsedABI.Events {
		events[e.ID.Hex()] = e
	}

	i.contracts[address.Hex()] = &ContractABI{
		Address: address,
		Name:    name,
		ABI:     parsedABI,
		Events:  events,
	}

	log.Printf("Registered contract: %s at %s", name, address.Hex())
	return nil
}

func (i *Indexer) registerEventHandlers() {
	i.eventHandlers["0xd78ad95fa46c994b6551d0da85f2750561f89436244c9d87b3c3c9d2a800f47"] = func(ctx context.Context, log types.Log) error {
		return i.handleSwap(ctx, log)
	}
	i.eventHandlers["0x1c411e9a96e071241c2f21f772bf17c7f4d52ab9fa3c5a5d6e3a1c4a1a1a1a1a"] = func(ctx context.Context, log types.Log) error {
		return i.handleSync(ctx, log)
	}
}

func (i *Indexer) handleSwap(ctx context.Context, log types.Log) error {
	swap := Swap{
		Hash:        log.TxHash.Hex(),
		PoolAddress: log.Address.Hex(),
		BlockNumber: log.BlockNumber,
		Timestamp:   uint64(time.Now().Unix()),
		CreatedAt:    time.Now(),
	}

	if err := i.db.Create(&swap).Error; err != nil {
		return fmt.Errorf("failed to store swap: %w", err)
	}

	log.Printf("Indexed swap: %s", swap.Hash)
	return nil
}

func (i *Indexer) handleSync(ctx context.Context, log types.Log) error {
	if len(log.Data) < 32 {
		return nil
	}

	reserve0 := new(big.Int).SetBytes(log.Data[:32])
	reserve1 := new(big.Int).SetBytes(log.Data[32:64])

	pool := Pool{}
	result := i.db.Where("address = ?", log.Address.Hex()).First(&pool)

	if result.Error == gorm.ErrRecordNotFound {
		pool = Pool{
			Address:     log.Address.Hex(),
			Reserve0:    reserve0.String(),
			Reserve1:    reserve1.String(),
			LastUpdated: uint64(time.Now().Unix()),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		i.db.Create(&pool)
	} else if result.Error == nil {
		pool.Reserve0 = reserve0.String()
		pool.Reserve1 = reserve1.String()
		pool.LastUpdated = uint64(time.Now().Unix())
		pool.UpdatedAt = time.Now()
		i.db.Save(&pool)
	}

	return nil
}

func (i *Indexer) Start(ctx context.Context) error {
	i.mu.Lock()
	if i.running {
		i.mu.Unlock()
		return fmt.Errorf("indexer already running")
	}
	i.running = true
	i.mu.Unlock()

	log.Printf("Starting indexer from block %d...", i.config.StartBlock)

	currentBlock, err := i.client.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current block: %w", err)
	}

	log.Printf("Current block: %d", currentBlock)
	go i.processBlocks(ctx, i.config.StartBlock, currentBlock)
	go i.watchNewBlocks(ctx)

	log.Println("Indexer started successfully")
	return nil
}

func (i *Indexer) Stop() {
	i.mu.Lock()
	defer i.mu.Unlock()

	if !i.running {
		return
	}

	i.running = false
	close(i.stopChan)
	log.Println("Indexer stopped")
}

func (i *Indexer) processBlocks(ctx context.Context, startBlock, endBlock uint64) {
	var wg sync.WaitGroup
	concurrency := MaxConcurrentBlocks
	blockChan := make(chan uint64, concurrency)

	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for blockNum := range blockChan {
				if err := i.processBlock(ctx, blockNum); err != nil {
					log.Printf("Error processing block %d: %v", blockNum, err)
				}
			}
		}()
	}

	for blockNum := startBlock; blockNum <= endBlock; blockNum++ {
		select {
		case <-i.stopChan:
			break
		case blockChan <- blockNum:
		}
	}
	close(blockChan)
	wg.Wait()
}

func (i *Indexer) processBlock(ctx context.Context, blockNum uint64) error {
	block, err := i.client.BlockByNumber(ctx, new(big.Int).SetUint64(blockNum))
	if err != nil {
		return fmt.Errorf("failed to get block %d: %w", blockNum, err)
	}

	blockModel := Block{
		Number:    blockNum,
		Hash:      block.Hash().Hex(),
		ParentHash: block.ParentHash().Hex(),
		Timestamp: block.Time(),
		GasUsed:   block.GasUsed(),
		CreatedAt: time.Now(),
	}

	if err := i.db.Create(&blockModel).Error; err != nil {
		log.Printf("Failed to store block %d: %v", blockNum, err)
	}

	logs, err := i.client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(blockNum),
		ToBlock:   new(big.Int).SetUint64(blockNum),
		Addresses: i.getContractAddresses(),
	})
	if err != nil {
		return fmt.Errorf("failed to get logs for block %d: %w", blockNum, err)
	}

	var wg sync.WaitGroup
	for _, logEntry := range logs {
		wg.Add(1)
		go func(logEntry types.Log) {
			defer wg.Done()
			if err := i.processLog(ctx, logEntry); err != nil {
				log.Printf("Error processing log: %v", err)
			}
		}(logEntry)
	}
	wg.Wait()

	i.blockCache.Add(block)
	log.Printf("Processed block %d", blockNum)
	return nil
}

func (i *Indexer) processLog(ctx context.Context, logEntry types.Log) error {
	event := Event{
		LogIndex:        uint64(logEntry.Index),
		TransactionHash: logEntry.TxHash.Hex(),
		BlockNumber:     logEntry.BlockNumber,
		Address:         logEntry.Address.Hex(),
		Data:            hex.EncodeToString(logEntry.Data),
		Removed:         logEntry.Removed,
		Timestamp:       uint64(time.Now().Unix()),
		CreatedAt:       time.Now(),
	}

	if len(logEntry.Topics) > 0 {
		event.Topic0 = logEntry.Topics[0].Hex()
	}

	handler, ok := i.eventHandlers[event.Topic0]
	if ok {
		if err := handler(ctx, logEntry); err != nil {
			log.Printf("Error in event handler: %v", err)
		}
	}

	return i.db.Create(&event).Error
}

func (i *Indexer) watchNewBlocks(ctx context.Context) {
	headerChan := make(chan *types.Header)

	sub, err := i.client.SubscribeNewHead(ctx, headerChan)
	if err != nil {
		log.Printf("Failed to subscribe to new heads: %v", err)
		return
	}
	defer sub.Unsubscribe()

	for {
		select {
		case <-i.stopChan:
			return
		case header := <-headerChan:
			if header == nil {
				continue
			}
			go func(blockNum uint64) {
				if err := i.processBlock(ctx, blockNum); err != nil {
					log.Printf("Error processing new block %d: %v", blockNum, err)
				}
			}(header.Number.Uint64())
		}
	}
}

func (i *Indexer) getContractAddresses() []common.Address {
	addresses := make([]common.Address, 0, len(i.contracts))
	for _, contract := range i.contracts {
		addresses = append(addresses, contract.Address)
	}
	return addresses
}

func NewBlockCache(maxSize int) *BlockCache {
	return &BlockCache{
		blocks:  make(map[uint64]*types.Block),
		maxSize: maxSize,
	}
}

func (bc *BlockCache) Add(block *types.Block) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	bc.blocks[block.Number()] = block

	if len(bc.blocks) > bc.maxSize {
		minBlock := block.Number()
		for num := range bc.blocks {
			if num < minBlock {
				minBlock = num
			}
		}
		delete(bc.blocks, minBlock)
	}
}

func main() {
	config := &Config{
		ChainID:     1,
		RPCURL:      os.Getenv("RPC_URL"),
		StartBlock:  18000000,
		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),
	}

	indexer, err := NewIndexer(config)
	if err != nil {
		log.Fatalf("Failed to create indexer: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	if err := indexer.Start(ctx); err != nil {
		log.Fatalf("Failed to start indexer: %v", err)
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down indexer...")
	cancel()
	indexer.Stop()
	log.Println("Indexer shutdown complete")
}
