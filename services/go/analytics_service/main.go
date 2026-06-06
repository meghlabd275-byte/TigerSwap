package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Analytics Service - TVL, Volume, Revenue, Leaderboard, Protocol Statistics

type Pool struct {
	Address      string `json:"address"`
	Token0       string `json:"token0"`
	Token1       string `json:"token1"`
	TVL          int64  `json:"tvl"`
	Volume24h    int64  `json:"volume_24h"`
	Fees24h      int64  `json:"fees_24h"`
}

type UserStats struct {
	User         string  `json:"user"`
	Volume       int64   `json:"volume"`
	FeesPaid     int64   `json:"fees_paid"`
	TradeCount   int     `json:"trade_count"`
	Rank         int     `json:"rank"`
}

type Analytics struct {
	mu          sync.RWMutex
	Pools       map[string]Pool
	Users      map[string]UserStats
	Volume24h  int64
	Fees24h    int64
	StartTime  time.Time
}

func NewAnalytics() *Analytics {
	return &Analytics{
		Pools:      make(map[string]Pool),
		Users:     make(map[string]UserStats),
		Volume24h: 0,
		Fees24h:    0,
		StartTime:  time.Now(),
	}
}

func (a *Analytics) UpdatePoolTVL(address string, tvl int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	if pool, ok := a.Pools[address]; ok {
		pool.TVL = tvl
		a.Pools[address] = pool
	} else {
		a.Pools[address] = Pool{Address: address, TVL: tvl}
	}
}

func (a *Analytics) RecordTrade(user string, pool string, amount int64, fee int64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	
	a.Volume24h += amount
	a.Fees24h += fee
	
	// Update user stats
	if stats, ok := a.Users[user]; ok {
		stats.Volume += amount
		stats.FeesPaid += fee
		stats.TradeCount++
		a.Users[user] = stats
	} else {
		a.Users[user] = UserStats{
			User:        user,
			Volume:     amount,
			FeesPaid:    fee,
			TradeCount: 1,
		}
	}
	
	// Update pool stats
	if p, ok := a.Pools[pool]; ok {
		p.Volume24h += amount
		p.Fees24h += fee
		a.Pools[pool] = p
	}
}

func (a *Analytics) GetTVL() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	var total int64
	for _, pool := range a.Pools {
		total += pool.TVL
	}
	return total
}

func (a *Analytics) GetVolume24h() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Volume24h
}

func (a *Analytics) GetFees24h() int64 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.Fees24h
}

func (a *Analytics) GetLeaderboard(limit int) []UserStats {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	users := make([]UserStats, 0, len(a.Users))
	for _, u := range a.Users {
		users = append(users, u)
	}
	
	// Sort by volume
	for i := 0; i < len(users)-1; i++ {
		for j := i + 1; j < len(users); j++ {
			if users[j].Volume > users[i].Volume {
				users[i], users[j] = users[j], users[i]
			}
		}
	}
	
	if limit > 0 && limit < len(users) {
		users = users[:limit]
	}
	
	// Assign ranks
	for i := range users {
		users[i].Rank = i + 1
	}
	
	return users
}

func (a *Analytics) GetProtocolStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()
	
	return map[string]interface{}{
		"tvl":          a.GetTVL(),
		"volume_24h":   a.Volume24h,
		"fees_24h":     a.Fees24h,
		"active_pools":  len(a.Pools),
		"active_users": len(a.Users),
		"uptime":       time.Since(a.StartTime).Seconds(),
	}
}

func main() {
	analytics := NewAnalytics()
	
	analytics.UpdatePoolTVL("0xPool1", 1000000000)
	analytics.UpdatePoolTVL("0xPool2", 500000000)
	
	analytics.RecordTrade("user1", "0xPool1", 100000, 100)
	analytics.RecordTrade("user2", "0xPool1", 200000, 200)
	analytics.RecordTrade("user1", "0xPool2", 50000, 50)
	
	fmt.Printf("TVL: %d\n", analytics.GetTVL())
	fmt.Printf("Volume 24h: %d\n", analytics.GetVolume24h())
	fmt.Printf("Fees 24h: %d\n", analytics.GetFees24h())
	
	leaderboard := analytics.GetLeaderboard(10)
	json, _ := json.Marshal(leaderboard)
	fmt.Printf("Leaderboard: %s\n", json)
}