package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Treasury Service - Fee collection, revenue distribution, buyback engine, treasury analytics

type TreasuryConfig struct {
	FeeRecipient      string        `json:"fee_recipient"`
	BuybackAddress   string        `json:"buyback_address"`
	BuybackPercent  float64       `json:"buyback_percent"`
	DistributionMin float64       `json:"distribution_min"`
}

type Treasury struct {
	mu              sync.RWMutex
	balance         map[string]int64  // token -> amount
	collectedFees   map[string]int64
	distributions   []Distribution
	config         TreasuryConfig
}

type Distribution struct {
	ID          string    `json:"id"`
	Recipient   string    `json:"recipient"`
	Amount     int64    `json:"amount"`
	Token      string    `json:"token"`
	Timestamp  int64    `json:"timestamp"`
	Status     string    `json:"status"`
}

func NewTreasury(config TreasuryConfig) *Treasury {
	return &Treasury{
		balance:       make(map[string]int64),
		collectedFees: make(map[string]int64),
		distributions: make([]Distribution, 0),
		config:      config,
	}
}

func (t *Treasury) CollectFee(token string, amount int64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	t.collectedFees[token] += amount
	t.balance[token] += amount
}

func (t *Treasury) GetBalance(token string) int64 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.balance[token]
}

func (t *Treasury) Distribute(recipient string, amount int64, token string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	if t.balance[token] < amount {
		return fmt.Errorf("insufficient balance")
	}
	
	if float64(amount) < t.config.DistributionMin {
		return fmt.Errorf("amount below minimum")
	}
	
	t.balance[token] -= amount
	
	dist := Distribution{
		ID:         fmt.Sprintf("dist-%d", time.Now().Unix()),
		Recipient:  recipient,
		Amount:    amount,
		Token:     token,
		Timestamp: time.Now().Unix(),
		Status:    "completed",
	}
	t.distributions = append(t.distributions, dist)
	
	return nil
}

func (t *Treasury) RunBuyback(token string, amount int64) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	
	buybackAmount := int64(float64(t.balance[token]) * t.config.BuybackPercent)
	if buybackAmount < amount {
		buybackAmount = amount
	}
	
	if t.balance[token] < buybackAmount {
		return fmt.Errorf("insufficient balance for buyback")
	}
	
	t.balance[token] -= buybackAmount
	
	return nil
}

func (t *Treasury) GetAnalytics() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()
	
	return map[string]interface{}{
		"total_collected": t.collectedFees,
		"current_balance": t.balance,
		"distributions":  len(t.distributions),
	}
}

func main() {
	config := TreasuryConfig{
		FeeRecipient:     "0xTreasury",
		BuybackAddress:   "0xBuyback",
		BuybackPercent:   0.10,
		DistributionMin: 1000000,
	}
	
	treasury := NewTreasury(config)
	treasury.CollectFee("USDC", 1000000000)
	
	balance := treasury.GetBalance("USDC")
	fmt.Printf("Balance: %d\n", balance)
	
	analytics := treasury.GetAnalytics()
	json, _ := json.Marshal(analytics)
	fmt.Printf("Analytics: %s\n", json)
}