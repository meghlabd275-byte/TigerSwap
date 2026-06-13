package main

// ============================================================================
// TigerSwap WebSocket Price Feed - Ultra Low Latency
// Real-time price streaming for DEX Aggregator
// ============================================================================

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ============================================================================
// Configuration
// ============================================================================

type Config struct {
	Port            string
	ReadTimeout     time.Duration
	WriteTimeout   time.Duration
	PingInterval   time.Duration
	MaxMessageSize int64
}

var defaultConfig = Config{
	Port:            ":8080",
	ReadTimeout:     10 * time.Second,
	WriteTimeout:    10 * time.Second,
	PingInterval:    30 * time.Second,
	MaxMessageSize: 1024 * 1024,
}

// ============================================================================
// Price Data Types
// ============================================================================

type PriceUpdate struct {
	Symbol    string  `json:"symbol"`
	Price    float64 `json:"price"`
	Volume24h float64 `json:"volume24h"`
	Change24h float64 `json:"change24h"`
	Timestamp int64  `json:"timestamp"`
}

type OrderBookLevel struct {
	Price float64 `json:"price"`
	Size  float64 `json:"size"`
}

type OrderBookUpdate struct {
	Symbol string          `json:"symbol"`
	Bids  []OrderBookLevel `json:"bids"`
	Asks  []OrderBookLevel `json:"asks"`
	Time  int64           `json:"timestamp"`
}

type Trade struct {
	Symbol    string  `json:"symbol"`
	Side      string  `json:"side"` // "buy" or "sell"
	Price     float64 `json:"price"`
	Size      float64 `json:"size"`
	Fee       float64 `json:"fee"`
	Timestamp int64  `json:"timestamp"`
	TxHash    string  `json:"txHash"`
}

// ============================================================================
// Price Feed Manager
// ============================================================================

type PriceFeed struct {
	mu           sync.RWMutex
	subscribers   map[*websocket.Conn]bool
	prices       map[string]*PriceUpdate
	orderBooks   map[string]*OrderBookUpdate
	lastUpdate   map[string]time.Time
	config      Config
	upgrader    websocket.Upgrader
	shutdown   chan struct{}
}

func NewPriceFeed(config Config) *PriceFeed {
	return &PriceFeed{
		subscribers: make(map[*websocket.Conn]bool),
		prices:     make(map[string]*PriceUpdate),
		orderBooks: make(map[string]*OrderBookUpdate),
		lastUpdate: make(map[string]time.Time),
		config:     config,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  int(config.MaxMessageSize),
			WriteBufferSize: int(config.MaxMessageSize),
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
		shutdown: make(chan struct{}),
	}
}

// ============================================================================
// Price Updates
// ============================================================================

func (pf *PriceFeed) UpdatePrice(symbol string, price, volume24h, change24h float64) {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	pf.prices[symbol] = &PriceUpdate{
		Symbol:    symbol,
		Price:    price,
		Volume24h: volume24h,
		Change24h: change24h,
		Timestamp: time.Now().UnixMilli(),
	}
	pf.lastUpdate[symbol] = time.Now()
}

func (pf *PriceFeed) UpdateOrderBook(symbol string, bids, asks []OrderBookLevel) {
	pf.mu.Lock()
	defer pf.mu.Unlock()

	pf.orderBooks[symbol] = &OrderBookUpdate{
		Symbol: symbol,
		Bids:  bids,
		Asks:  asks,
		Time:  time.Now().UnixMilli(),
	}
}

// ============================================================================
// Broadcast
// ============================================================================

func (pf *PriceFeed) BroadcastPriceUpdate() {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	// Broadcast to all subscribers
	for conn := range pf.subscribers {
		for _, price := range pf.prices {
			if err := conn.WriteJSON(price); err != nil {
				log.Printf("Broadcast error: %v", err)
				conn.Close()
			}
		}
	}
}

func (pf *PriceFeed) BroadcastOrderBook() {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	for conn := range pf.subscribers {
		for _, ob := range pf.orderBooks {
			if err := conn.WriteJSON(ob); err != nil {
				log.Printf("Broadcast error: %v", err)
				conn.Close()
			}
		}
	}
}

// ============================================================================
// WebSocket Handler
// ============================================================================

func (pf *PriceFeed) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := pf.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Upgrade error: %v", err)
		return
	}

	pf.mu.Lock()
	pf.subscribers[conn] = true
	pf.mu.Unlock()

	log.Printf("New subscriber connected. Total: %d", len(pf.subscribers))

	// Send initial prices
	pf.mu.RLock()
	for _, price := range pf.prices {
		conn.WriteJSON(price)
	}
	pf.mu.RUnlock()

	// Heartbeat goroutine
	go pf.handleHeartbeat(conn)

	// Message reader
	go pf.readMessages(conn)
}

func (pf *PriceFeed) handleHeartbeat(conn *websocket.Conn) {
	ticker := time.NewTicker(pf.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pf.mu.RLock()
			isActive := pf.subscribers[conn]
			pf.mu.RUnlock()

			if !isActive {
				return
			}

			if err := conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(5*time.Second)); err != nil {
				log.Printf("Ping error: %v", err)
				conn.Close()
				return
			}
		case <-pf.shutdown:
			conn.Close()
			return
		}
	}
}

func (pf *PriceFeed) readMessages(conn *websocket.Conn) {
	defer func() {
		pf.mu.Lock()
		delete(pf.subscribers, conn)
		pf.mu.Unlock()
		conn.Close()
		log.Printf("Subscriber disconnected. Total: %d", len(pf.subscribers))
	}()

	conn.SetReadLimit(pf.config.MaxMessageSize)
	conn.SetReadDeadline(time.Now().Add(pf.config.ReadTimeout))

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Read error: %v", err)
			}
			break
		}

		// Handle subscription messages
		var msg map[string]interface{}
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		// Process subscription request
		if msgType, ok := msg["type"].(string); ok {
			switch msgType {
			case "subscribe":
				// Handle subscription
			case "unsubscribe":
				// Handle unsubscription
			}
		}
	}
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func (pf *PriceFeed) HandlePrices(w http.ResponseWriter, r *http.Request) {
	pf.mu.RLock()
	defer pf.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(pf.prices)
}

func (pf *PriceFeed) HandleOrderBook(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		http.Error(w, "symbol required", http.StatusBadRequest)
		return
	}

	pf.mu.RLock()
	defer pf.mu.RUnlock()

	if ob, ok := pf.orderBooks[symbol]; ok {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(ob)
	} else {
		http.Error(w, "symbol not found", http.StatusNotFound)
	}
}

func (pf *PriceFeed) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "healthy",
		"subscribers": len(pf.subscribers),
		"timestamp":  time.Now().Unix(),
	})
}

// ============================================================================
// Main
// ============================================================================

func main() {
	config := defaultConfig
	pf := NewPriceFeed(config)

	// Start price update simulator (in production, connect to real DEX RPCs)
	go pf.simulatePriceUpdates()

	// HTTP routes
	http.HandleFunc("/ws", pf.HandleConnection)
	http.HandleFunc("/prices", pf.HandlePrices)
	http.HandleFunc("/orderbook", pf.HandleOrderBook)
	http.HandleFunc("/health", pf.HandleHealth)

	log.Printf("Starting WebSocket Price Feed on %s", config.Port)
	if err := http.ListenAndServe(config.Port, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// ============================================================================
// Price Simulator (Replace with real DEX connections)
// ============================================================================

func (pf *PriceFeed) simulatePriceUpdates() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	prices := map[string]float64{
		"ETH/USDC": 3500.0,
		"BTC/USDC": 65000.0,
		"SOL/USDC": 180.0,
		"ETH/USDT": 3500.0,
		"WBTC/ETH": 18.5,
	}

	for {
		select {
		case <-ticker.C:
			for symbol, basePrice := range prices {
				// Add small random variation
				variation := (basePrice * 0.001) * (float64(time.Now().UnixNano()%1000) / 1000 - 0.5)
				price := basePrice + variation
				volume := basePrice * 1000000 * (0.9 + float64(time.Now().UnixNano()%100000)/100000)
				change := (price - basePrice) / basePrice * 100

				pf.UpdatePrice(symbol, price, volume, change)
			}
		case <-pf.shutdown:
			return
		}
	}
}