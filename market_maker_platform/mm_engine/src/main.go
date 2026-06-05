package main

import (
	"fmt"
	"time"
)

// Market Maker Platform - MM Engine
// Handles market making operations across DEXs

type MarketMakerConfig struct {
	Enabled       bool
	MinSpreadBps  int
	MaxSpreadBps  int
	OrderSizeMin  float64
	OrderSizeMax  float64
	MaxPosition   float64
	MaxDailyVolume float64
}

type MarketMakerEngine struct {
	ID       string
	config   MarketMakerConfig
	positions map[string]*Position
	orders   map[string]*Order
	stats    *MMStats
}

type Position struct {
	Pair        string
	BaseAmount  float64
	QuoteAmount float64
	AvgBasePrice float64
	AvgQuotePrice float64
	PnL         float64
}

type Order struct {
	ID        string
	Pair      string
	Side      string  // BID or ASK
	Price     float64
	Size      float64
	Filled    float64
	Status    string  // OPEN, FILLED, CANCELLED
	Timestamp int64
}

type MMStats struct {
	TotalVolume    float64
	DailyVolume    float64
	TotalPnL       float64
	DailyPnL       float64
	OrderCount     int
	FilledCount    int
	OpenCount      int
}

func NewMarketMakerEngine(id string) *MarketMakerEngine {
	return &MarketMakerEngine{
		ID:       id,
		config:   MarketMakerConfig{true, 50, 200, 100, 10000, 50000, 1000000},
		positions: make(map[string]*Position),
		orders:   make(map[string]*Order),
		stats:    &MMStats{},
	}
}

func (mm *MarketMakerEngine) calculateSpread(midPrice float64, volatility float64) float64 {
	baseSpread := float64(mm.config.MinSpreadBps) / 10000
	spread := baseSpread + (volatility * 0.5)
	if spread > float64(mm.config.MaxSpreadBps)/10000 {
		spread = float64(mm.config.MaxSpreadBps) / 10000
	}
	return spread
}

func (mm *MarketMakerEngine) calculateBidPrice(midPrice float64, spread float64) float64 {
	return midPrice * (1 - spread)
}

func (mm *MarketMakerEngine) calculateAskPrice(midPrice float64, spread float64) float64 {
	return midPrice * (1 + spread)
}

func (mm *MarketMakerEngine) calculateOrderSize(balance float64) float64 {
	maxSize := balance * 0.3 // Use max 30% of balance
	if maxSize > mm.config.OrderSizeMax {
		maxSize = mm.config.OrderSizeMax
	}
	if maxSize < mm.config.OrderSizeMin {
		maxSize = mm.config.OrderSizeMin
	}
	return maxSize
}

func (mm *MarketMakerEngine) createOrders(pair string, midPrice float64, volatility float64) []*Order {
	spread := mm.calculateSpread(midPrice, volatility)
	bidPrice := mm.calculateBidPrice(midPrice, spread)
	askPrice := mm.calculateAskPrice(midPrice, spread)
	
	bidSize := mm.calculateOrderSize(10000)
	askSize := mm.calculateOrderSize(10000)
	
	orders := []*Order{
		{
			ID:        fmt.Sprintf("order_%d", time.Now().UnixNano()),
			Pair:      pair,
			Side:      "BID",
			Price:     bidPrice,
			Size:      bidSize,
			Filled:    0,
			Status:    "OPEN",
			Timestamp: time.Now().Unix(),
		},
		{
			ID:        fmt.Sprintf("order_%d", time.Now().UnixNano()+1),
			Pair:      pair,
			Side:      "ASK",
			Price:     askPrice,
			Size:      askSize,
			Filled:    0,
			Status:    "OPEN",
			Timestamp: time.Now().Unix(),
		},
	}
	
	for _, o := range orders {
		mm.orders[o.ID] = o
	}
	
	return orders
}

func (mm *MarketMakerEngine) fillOrder(orderID string, size float64) error {
	order, ok := mm.orders[orderID]
	if !ok {
		return fmt.Errorf("order not found")
	}
	
	order.Filled += size
	if order.Filled >= order.Size {
		order.Status = "FILLED"
		mm.stats.FilledCount++
	}
	
	// Update position
	pair := order.Pair
	pos := mm.positions[pair]
	if pos == nil {
		pos = &Position{Pair: pair}
		mm.positions[pair] = pos
	}
	
	if order.Side == "BID" {
		pos.BaseAmount += size
		pos.QuoteAmount -= size * order.Price
		pos.AvgBasePrice = (pos.QuoteAmount / pos.BaseAmount) * -1
	} else {
		pos.BaseAmount -= size
		pos.QuoteAmount += size * order.Price
		pos.AvgQuotePrice = pos.QuoteAmount / (pos.BaseAmount * -1)
	}
	
	mm.stats.OrderCount++
	mm.stats.OpenCount = len(mm.orders) - mm.stats.FilledCount
	
	return nil
}

func (mm *MarketMakerEngine) getPosition(pair string) *Position {
	return mm.positions[pair]
}

func (mm *MarketMakerEngine) getStats() *MMStats {
	return mm.stats
}

func (mm *MarketMakerEngine) getOpenOrders() []*Order {
	var open []*Order
	for _, o := range mm.orders {
		if o.Status == "OPEN" {
			open = append(open, o)
		}
	}
	return open
}

func (mm *MarketMakerEngine) cancelOrder(orderID string) error {
	order, ok := mm.orders[orderID]
	if !ok {
		return fmt.Errorf("order not found")
	}
	order.Status = "CANCELLED"
	return nil
}

func main() {
	fmt.Println("TigerSwap Market Maker Platform v1.0")
	fmt.Println("==================================")
	
	mm := NewMarketMakerEngine("mm_engine_001")
	
	// Simulate market making for ETH/USDT
	pair := "ETH/USDT"
	midPrice := 2450.50
	volatility := 0.02
	
	// Create orders
	orders := mm.createOrders(pair, midPrice, volatility)
	fmt.Printf("\nCreated %d orders for %s\n", len(orders), pair)
	for _, o := range orders {
		fmt.Printf("  %s: %.2f @ $%.2f (filled: %.2f, status: %s)\n", 
			o.ID, o.Size, o.Price, o.Filled, o.Status)
	}
	
	// Simulate fills
	for _, o := range orders {
		fillSize := o.Size * 0.7
		mm.fillOrder(o.ID, fillSize)
		fmt.Printf("Filled %.2f of order %s\n", fillSize, o.ID)
	}
	
	// Get position
	pos := mm.getPosition(pair)
	fmt.Printf("\nPosition for %s:\n", pair)
	fmt.Printf("  Base: %.4f, Quote: $%.2f\n", pos.BaseAmount, pos.QuoteAmount)
	
	// Get stats
	stats := mm.getStats()
	fmt.Printf("\nMM Stats:\n")
	fmt.Printf("  Total Orders: %d, Filled: %d, Open: %d\n", stats.OrderCount, stats.FilledCount, stats.OpenCount)
	fmt.Printf("  Open Orders: %d\n", len(mm.getOpenOrders()))
}