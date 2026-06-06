package main

import (
	"fmt"
	"sync"
	"time"
)

// Indexer Service - Multi-chain indexer for Ethereum, BSC, Polygon, Arbitrum, Base, Solana

type ChainConfig struct {
	ChainID       uint32
	Name          string
	RPCURL        string
	StartBlock    uint64
}

type Block struct {
	ChainID   uint32   `json:"chain_id"`
	Number   uint64   `json:"number"`
	Hash     string   `json:"hash"`
	TxCount  int      `json:"tx_count"`
	Timestamp int64   `json:"timestamp"`
}

type TokenTransfer struct {
	ChainID    uint32 `json:"chain_id"`
	TxHash    string `json:"tx_hash"`
	From      string `json:"from"`
	To        string `json:"to"`
	Token     string `json:"token"`
	Amount    string `json:"amount"`
	Value     int64  `json:"value"`
	Timestamp int64  `json:"timestamp"`
}

type Indexer struct {
	mu          sync.RWMutex
	chains      map[uint32]ChainConfig
	blocks      map[uint64]map[uint32]Block  // chain -> block number -> block
	transfers   []TokenTransfer
	latestBlock map[uint32]uint64
}

func NewIndexer() *Indexer {
	return &Indexer{
		chains:      make(map[uint32]ChainConfig),
		blocks:     make(map[uint64]map[uint32]Block),
		transfers:  make([]TokenTransfer, 0),
		latestBlock: make(map[uint32]uint64),
	}
}

func (i *Indexer) AddChain(config ChainConfig) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.chains[config.ChainID] = config
	i.latestBlock[config.ChainID] = config.StartBlock
}

func (i *Indexer) IndexBlock(chainID uint32, block Block) {
	i.mu.Lock()
	defer i.mu.Unlock()
	
	if _, ok := i.blocks[block.Number]; !ok {
		i.blocks[block.Number] = make(map[uint32]Block)
	}
	i.blocks[block.Number][chainID] = block
	i.latestBlock[chainID] = block.Number
}

func (i *Indexer) IndexTransfer(transfer TokenTransfer) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.transfers = append(i.transfers, transfer)
}

func (i *Indexer) GetLatestBlock(chainID uint32) (uint64, bool) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	
	latest, ok := i.latestBlock[chainID]
	return latest, ok
}

func (i *Indexer) GetTransfers(address string, limit int) []TokenTransfer {
	i.mu.RLock()
	defer i.mu.RUnlock()
	
	var result []TokenTransfer
	for _, t := range i.transfers {
		if t.From == address || t.To == address {
			result = append(result, t)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}
	return result
}

func (i *Indexer) GetTokenBalance(token, address string) int64 {
	i.mu.RLock()
	defer i.mu.RUnlock()
	
	var balance int64
	for _, t := range i.transfers {
		if t.Token == token {
			if t.To == address {
				balance += t.Value
			} else if t.From == address {
				balance -= t.Value
			}
		}
	}
	return balance
}

func (i *Indexer) SyncChain(chainID uint32) error {
	i.mu.RLock()
	chain, ok := i.chains[chainID]
	i.mu.RUnlock()
	
	if !ok {
		return fmt.Errorf("chain not found: %d", chainID)
	}
	
	// In production: fetch blocks from RPC
	latest := i.latestBlock[chainID]
	for b := latest; b < latest+100; b++ {
		block := Block{
			ChainID:   chainID,
			Number:   b,
			Hash:     fmt.Sprintf("0x%x", b),
			TxCount:  0,
			Timestamp: time.Now().Unix(),
		}
		i.IndexBlock(chainID, block)
	}
	
	return nil
}

func main() {
	indexer := NewIndexer()
	
	// Add supported chains
	indexer.AddChain(ChainConfig{ChainID: 1, Name: "Ethereum", StartBlock: 19000000})
	indexer.AddChain(ChainConfig{ChainID: 56, Name: "BSC", StartBlock: 35000000})
	indexer.AddChain(ChainConfig{ChainID: 137, Name: "Polygon", StartBlock: 50000000})
	indexer.AddChain(ChainConfig{ChainID: 42161, Name: "Arbitrum", StartBlock: 180000000})
	indexer.AddChain(ChainConfig{ChainID: 8453, Name: "Base", StartBlock: 10000000})
	
	// Index transfers
	indexer.IndexTransfer(TokenTransfer{
		ChainID:  1,
		TxHash:   "0x123",
		From:    "0xA",
		To:      "0xB",
		Token:   "0xUSDC",
		Amount:  "1000",
		Value:   1000,
		Timestamp: time.Now().Unix(),
	})
	
	balance := indexer.GetTokenBalance("0xUSDC", "0xB")
	fmt.Printf("Balance: %d\n", balance)
	
	transfers := indexer.GetTransfers("0xA", 10)
	fmt.Printf("Transfers: %d\n", len(transfers))
}