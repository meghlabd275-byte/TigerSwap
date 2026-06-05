// TigerSwap Real-Time Trading Service - Go Implementation
// WebSocket-based price feeds, order book, and trade updates
// High-performance streaming for TigerSwap trading platform

package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// ============================================================================
// Constants
// ============================================================================

const (
	// Timeouts
	WriteWait      = 10 * time.Second
	PongWait       = 60 * time.Second
	PingPeriod     = (PongWait * 9) / 10
	MaxMessageSize = 4096

	// Subscription limits
	MaxSubscriptions = 50
)

// ============================================================================
// Data Structures
// ============================================================================

type SubscriptionType string

const (
	SubTrades    SubscriptionType = "trades"
	SubTicker    SubscriptionType = "ticker"
	SubOrderBook SubscriptionType = "orderbook"
	SubPrice     SubscriptionType = "price"
)

type WSClient struct {
	ID            string
	Subscriptions map[string]bool // "pair:channel"
	Protocol     *websocket.Conn
	mu           sync.Mutex
}

type OrderBookLevel struct {
	Price    float64 `json:"price"`
	Size     float64 `json:"size"`
	Total    float64 `json:"total"`
}

type OrderBook struct {
	Pair      string           `json:"pair"`
	Bids      []OrderBookLevel `json:"bids"`
	Asks      []OrderBookLevel `json:"asks"`
	Spread    float64          `json:"spread"`
	SpreadBPS float64          `json:"spread_bps"`
	Timestamp int64           `json:"timestamp"`
}

type Ticker struct {
	Pair       string  `json:"pair"`
	Price      float64 `json:"price"`
	Volume24h  float64 `json:"volume_24h"`
	Change24h  float64 `json:"change_24h"`
	High24h    float64 `json:"high_24h"`
	Low24h     float64 `json:"low_24h"`
	Bid        float64 `json:"bid"`
	Ask        float64 `json:"ask"`
	LastUpdate int64   `json:"last_update"`
}

type Trade struct {
	ID        string  `json:"id"`
	Pair      string  `json:"pair"`
	Side      string  `json:"side"` // buy, sell
	Price     float64 `json:"price"`
	Amount    float64 `json:"amount"`
	Timestamp int64   `json:"timestamp"`
	TxHash    string  `json:"tx_hash"`
	DEX       string  `json:"dex"`
}

type PriceTick struct {
	Pair       string  `json:"pair"`
	Price      float64 `json:"price"`
	Volume24h  float64 `json:"volume_24h"`
	Change24h  float64 `json:"change_24h"`
	High24h    float64 `json:"high_24h"`
	Low24h     float64 `json:"low_24h"`
	Timestamp  int64   `json:"timestamp"`
	Source     string  `json:"source"`
}

// ============================================================================
// WebSocket Factory
// ============================================================================

type WSF struct {
	// Connected clients
	clients map[string]*WSClient
	mu     sync.RWMutex

	// Subscription index: "pair:channel" -> set of client IDs
	subscriptions map[string]map[string]bool

	// Market data
	orderBooks map[string]*OrderBook
	tickers    map[string]*Ticker
	trades     map[string][]*Trade
	prices     map[string]*PriceTick

	// Statistics
	stats struct {
		TotalConnections int64
		TotalMessages   int64
		TotalSubscriptions int64
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for development
	},
}

func NewWSF() *WSF {
	f := &WSF{
		clients:      make(map[string]*WSClient),
		subscriptions: make(map[string]map[string]bool),
		orderBooks:    make(map[string]*OrderBook),
		tickers:      make(map[string]*Ticker),
		trades:       make(map[string][]*Trade),
		prices:       make(map[string]*PriceTick),
	}

	// Initialize with default pairs
	f.initializePairs()

	return f
}

func (f *WSF) initializePairs() {
	pairs := []string{"ETH_USDC", "ETH_USDT", "ETH_WBTC", "WBTC_USDC", "WBTC_USDT"}

	basePrices := map[string]float64{
		"ETH_USDC": 2450.0,
		"ETH_USDT": 2450.0,
		"ETH_WBTC": 0.04,
		"WBTC_USDC": 62500.0,
		"WBTC_USDT": 62500.0,
	}

	for _, pair := range pairs {
		f.orderBooks[pair] = f.generateOrderBook(pair, basePrices[pair])
		f.tickers[pair] = f.generateTicker(pair, basePrices[pair])
		f.trades[pair] = make([]*Trade, 0)
	}
}

func (f *WSF) generateOrderBook(pair string, basePrice float64) *OrderBook {
	ob := &OrderBook{
		Pair:      pair,
		Bids:      make([]OrderBookLevel, 0, 20),
		Asks:      make([]OrderBookLevel, 0, 20),
		Timestamp: time.Now().Unix(),
	}

	var bidTotal, askTotal float64

	// Generate 20 levels of bids
	for i := 0; i < 20; i++ {
		bidPrice := basePrice * (1 - 0.0001*float64(i+1))
		bidSize := 1.0 + float64(20-i)*0.5
		bidTotal += bidSize
		ob.Bids = append(ob.Bids, OrderBookLevel{
			Price: bidPrice,
			Size:  bidSize,
			Total: bidTotal,
		})
	}

	// Generate 20 levels of asks
	for i := 0; i < 20; i++ {
		askPrice := basePrice * (1 + 0.0001*float64(i+1))
		askSize := 1.0 + float64(20-i)*0.5
		askTotal += askSize
		ob.Asks = append(ob.Asks, OrderBookLevel{
			Price: askPrice,
			Size:  askSize,
			Total: askTotal,
		})
	}

	if len(ob.Bids) > 0 && len(ob.Asks) > 0 {
		ob.Spread = ob.Asks[0].Price - ob.Bids[0].Price
		ob.SpreadBPS = (ob.Spread / basePrice) * 10000
	}

	return ob
}

func (f *WSF) generateTicker(pair string, basePrice float64) *Ticker {
	return &Ticker{
		Pair:       pair,
		Price:      basePrice,
		Volume24h:  basePrice * 1000000,
		Change24h:  2.35,
		High24h:    basePrice * 1.03,
		Low24h:     basePrice * 0.97,
		Bid:        basePrice * 0.9999,
		Ask:        basePrice * 1.0001,
		LastUpdate: time.Now().Unix(),
	}
}

// ============================================================================
// Client Management
// ============================================================================

func (f *WSF) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	clientID := fmt.Sprintf("client_%d", time.Now().UnixNano())
	
	client := &WSClient{
		ID:            clientID,
		Subscriptions: make(map[string]bool),
		Protocol:     conn,
	}

	f.mu.Lock()
	f.clients[clientID] = client
	f.stats.TotalConnections++
	f.mu.Unlock()

	log.Printf("Client %s connected from %s", clientID, r.RemoteAddr)

	f.handleClient(client)

	f.mu.Lock()
	delete(f.clients, clientID)
	// Clean up subscriptions
	for key := range f.subscriptions {
		delete(f.subscriptions[key], clientID)
	}
	f.mu.Unlock()

	log.Printf("Client %s disconnected", clientID)
}

func (f *WSF) handleClient(client *WSClient) {
	// Send welcome message
	welcome := map[string]interface{}{
		"type":       "welcome",
		"client_id":  client.ID,
		"server_time": time.Now().Unix(),
	}
	client.SendJSON(welcome)

	// Start ping/pong handler
	go client.PingHandler()

	// Read messages
	for {
		_, message, err := client.Protocol.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error for client %s: %v", client.ID, err)
			}
			break
		}

		f.handleMessage(client, message)
	}
}

func (f *WSF) handleMessage(client *WSClient, message []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		client.SendJSON(map[string]string{"type": "error", "message": "Invalid JSON"})
		return
	}

	action, _ := msg["action"].(string)

	switch action {
	case "subscribe":
		f.handleSubscribe(client, msg)
	case "unsubscribe":
		f.handleUnsubscribe(client, msg)
	case "ping":
		client.SendJSON(map[string]interface{}{"type": "pong", "timestamp": time.Now().Unix()})
	case "get_orderbook":
		f.handleGetOrderBook(client, msg)
	case "get_ticker":
		f.handleGetTicker(client, msg)
	case "get_recent_trades":
		f.handleGetRecentTrades(client, msg)
	default:
		client.SendJSON(map[string]string{"type": "error", "message": fmt.Sprintf("Unknown action: %s", action)})
	}
}

func (f *WSF) handleSubscribe(client *WSClient, msg map[string]interface{}) {
	channels, ok := msg["channels"].([]interface{})
	if !ok {
		client.SendJSON(map[string]string{"type": "error", "message": "Invalid channels"})
		return
	}

	pair, _ := msg["pair"].(string)
	if pair == "" {
		client.SendJSON(map[string]string{"type": "error", "message": "Pair required"})
		return
	}

	// Check subscription limit
	client.mu.Lock()
	if len(client.Subscriptions)+len(channels) > MaxSubscriptions {
		client.mu.Unlock()
		client.SendJSON(map[string]interface{}{
			"type":    "error",
			"message": fmt.Sprintf("Too many subscriptions. Max: %d", MaxSubscriptions),
		})
		return
	}
	client.mu.Unlock()

	subscribed := make([]string, 0)

	for _, ch := range channels {
		channel, ok := ch.(string)
		if !ok {
			continue
		}

		// Validate channel
		if channel != string(SubTrades) && channel != string(SubTicker) &&
			channel != string(SubOrderBook) && channel != string(SubPrice) {
			continue
		}

		subKey := fmt.Sprintf("%s:%s", pair, channel)

		client.mu.Lock()
		client.Subscriptions[subKey] = true
		client.mu.Unlock()

		f.mu.Lock()
		if f.subscriptions[subKey] == nil {
			f.subscriptions[subKey] = make(map[string]bool)
		}
		f.subscriptions[subKey][client.ID] = true
		f.stats.TotalSubscriptions++
		f.mu.Unlock()

		subscribed = append(subscribed, subKey)

		// Send initial data
		switch channel {
		case string(SubOrderBook):
			f.mu.RLock()
			if ob, ok := f.orderBooks[pair]; ok {
				client.SendJSON(map[string]interface{}{
					"type": "orderbook_snapshot",
					"pair": pair,
					"data": ob,
				})
			}
			f.mu.RUnlock()

		case string(SubTicker):
			f.mu.RLock()
			if ticker, ok := f.tickers[pair]; ok {
				client.SendJSON(map[string]interface{}{
					"type": "ticker",
					"pair": pair,
					"data": ticker,
				})
			}
			f.mu.RUnlock()

		case string(SubTrades):
			f.mu.RLock()
			if trades, ok := f.trades[pair]; ok {
				tradesJSON := make([]map[string]interface{}, 0)
				for _, t := range trades {
					if len(tradesJSON) >= 20 {
						break
					}
					tradesJSON = append(tradesJSON, map[string]interface{}{
						"id":        t.ID,
						"side":      t.Side,
						"price":     t.Price,
						"amount":    t.Amount,
						"timestamp": t.Timestamp,
						"tx_hash":   t.TxHash,
						"dex":       t.DEX,
					})
				}
				client.SendJSON(map[string]interface{}{
					"type":  "recent_trades",
					"pair":  pair,
					"trades": tradesJSON,
				})
			}
			f.mu.RUnlock()
		}
	}

	client.SendJSON(map[string]interface{}{
		"type":      "subscribed",
		"channels": subscribed,
		"pair":     pair,
	})
}

func (f *WSF) handleUnsubscribe(client *WSClient, msg map[string]interface{}) {
	channels, ok := msg["channels"].([]interface{})
	if !ok {
		return
	}

	pair, _ := msg["pair"].(string)

	unsubscribed := make([]string, 0)

	for _, ch := range channels {
		channel, _ := ch.(string)
		subKey := fmt.Sprintf("%s:%s", pair, channel)

		client.mu.Lock()
		delete(client.Subscriptions, subKey)
		client.mu.Unlock()

		f.mu.Lock()
		if f.subscriptions[subKey] != nil {
			delete(f.subscriptions[subKey], client.ID)
		}
		f.mu.Unlock()

		unsubscribed = append(unsubscribed, subKey)
	}

	client.SendJSON(map[string]interface{}{
		"type":      "unsubscribed",
		"channels": unsubscribed,
		"pair":     pair,
	})
}

func (f *WSF) handleGetOrderBook(client *WSClient, msg map[string]interface{}) {
	pair, _ := msg["pair"].(string)
	depth, _ := msg["depth"].(float64)
	if depth == 0 {
		depth = 20
	}

	f.mu.RLock()
	ob, ok := f.orderBooks[pair]
	f.mu.RUnlock()

	if !ok {
		client.SendJSON(map[string]string{"type": "error", "message": "Pair not found"})
		return
	}

	client.SendJSON(map[string]interface{}{
		"type": "orderbook_snapshot",
		"pair": pair,
		"data": ob,
	})
}

func (f *WSF) handleGetTicker(client *WSClient, msg map[string]interface{}) {
	pair, _ := msg["pair"].(string)

	f.mu.RLock()
	ticker, ok := f.tickers[pair]
	f.mu.RUnlock()

	if !ok {
		client.SendJSON(map[string]string{"type": "error", "message": "Pair not found"})
		return
	}

	client.SendJSON(map[string]interface{}{
		"type": "ticker",
		"pair": pair,
		"data": ticker,
	})
}

func (f *WSF) handleGetRecentTrades(client *WSClient, msg map[string]interface{}) {
	pair, _ := msg["pair"].(string)
	limit, _ := msg["limit"].(float64)
	if limit == 0 {
		limit = 50
	}

	f.mu.RLock()
	trades, ok := f.trades[pair]
	f.mu.RUnlock()

	if !ok {
		client.SendJSON(map[string]string{"type": "error", "message": "Pair not found"})
		return
	}

	tradesJSON := make([]map[string]interface{}, 0)
	for i := len(trades) - 1; i >= 0 && len(tradesJSON) < int(limit); i-- {
		t := trades[i]
		tradesJSON = append(tradesJSON, map[string]interface{}{
			"id":        t.ID,
			"side":      t.Side,
			"price":     t.Price,
			"amount":    t.Amount,
			"timestamp": t.Timestamp,
			"tx_hash":   t.TxHash,
			"dex":       t.DEX,
		})
	}

	client.SendJSON(map[string]interface{}{
		"type":  "recent_trades",
		"pair":  pair,
		"trades": tradesJSON,
	})
}

// ============================================================================
// Broadcast Methods
// ============================================================================

func (f *WSF) broadcast(subKey string, message interface{}) {
	f.mu.RLock()
	clientIDs, ok := f.subscriptions[subKey]
	f.mu.RUnlock()

	if !ok || len(clientIDs) == 0 {
		return
	}

	msgBytes, err := json.Marshal(message)
	if err != nil {
		return
	}

	f.mu.RLock()
	for clientID := range clientIDs {
		if client, exists := f.clients[clientID]; exists {
			client.mu.Lock()
			err := client.Protocol.WriteMessage(websocket.TextMessage, msgBytes)
			client.mu.Unlock()

			if err != nil {
				log.Printf("Error broadcasting to client %s: %v", clientID, err)
			}
		}
	}
	f.mu.RUnlock()
}

func (f *WSF) BroadcastPriceUpdate(pair string, price float64) {
	msg := map[string]interface{}{
		"type":      "price_update",
		"pair":      pair,
		"price":     price,
		"timestamp": time.Now().Unix(),
	}
	f.broadcast(fmt.Sprintf("%s:price", pair), msg)
}

func (f *WSF) BroadcastOrderBookUpdate(pair string) {
	f.mu.RLock()
	ob, ok := f.orderBooks[pair]
	f.mu.RUnlock()

	if !ok {
		return
	}

	msg := map[string]interface{}{
		"type":            "orderbook_update",
		"pair":            pair,
		"bids":            ob.Bids[:10],
		"asks":            ob.Asks[:10],
		"last_update_id":  time.Now().UnixNano(),
		"timestamp":       ob.Timestamp,
	}
	f.broadcast(fmt.Sprintf("%s:orderbook", pair), msg)
}

func (f *WSF) BroadcastTrade(pair string, trade *Trade) {
	msg := map[string]interface{}{
		"type": "trade",
		"data": map[string]interface{}{
			"id":        trade.ID,
			"side":      trade.Side,
			"price":     trade.Price,
			"amount":    trade.Amount,
			"timestamp": trade.Timestamp,
			"tx_hash":   trade.TxHash,
			"dex":       trade.DEX,
		},
	}
	f.broadcast(fmt.Sprintf("%s:trades", pair), msg)
}

func (f *WSF) BroadcastTickerUpdate(pair string) {
	f.mu.RLock()
	ticker, ok := f.tickers[pair]
	f.mu.RUnlock()

	if !ok {
		return
	}

	msg := map[string]interface{}{
		"type": "ticker",
		"pair": pair,
		"data": ticker,
	}
	f.broadcast(fmt.Sprintf("%s:ticker", pair), msg)
}

// ============================================================================
// Client Methods
// ============================================================================

func (c *WSClient) SendJSON(v interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.Protocol.WriteJSON(v); err != nil {
		log.Printf("Error sending JSON to client %s: %v", c.ID, err)
	}
}

func (c *WSClient) PingHandler() {
	ticker := time.NewTicker(PingPeriod)
	defer ticker.Stop()

	for {
		<-ticker.C
		c.mu.Lock()
		if c.Protocol != nil {
			c.Protocol.SetWriteDeadline(time.Now().Add(WriteWait))
			if err := c.Protocol.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.mu.Unlock()
				return
			}
		}
		c.mu.Unlock()
	}
}

// ============================================================================
// Background Update Loop
// ============================================================================

func (f *WSF) StartUpdateLoop(ctx <-chan struct{}) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx:
			return
		case <-ticker.C:
			f.updatePrices()
		}
	}
}

func (f *WSF) updatePrices() {
	f.mu.Lock()
	defer f.mu.Unlock()

	for pair, ticker := range f.tickers {
		// Simulate price movement
		change := (float64(time.Now().UnixNano()%1000) - 500) / 100000.0
		ticker.Price *= (1 + change)
		ticker.Bid = ticker.Price * 0.9999
		ticker.Ask = ticker.Price * 1.0001
		ticker.LastUpdate = time.Now().Unix()

		// Update order book with new price
		if ob, ok := f.orderBooks[pair]; ok {
			for i := range ob.Bids {
				ob.Bids[i].Price = ticker.Price * (1 - 0.0001*float64(i+1))
			}
			for i := range ob.Asks {
				ob.Asks[i].Price = ticker.Price * (1 + 0.0001*float64(i+1))
			}
			ob.Spread = ob.Asks[0].Price - ob.Bids[0].Price
			ob.SpreadBPS = (ob.Spread / ticker.Price) * 10000
			ob.Timestamp = time.Now().Unix()
		}
	}
}

// ============================================================================
// REST Endpoints
// ============================================================================

func (f *WSF) HandleHealth(w http.ResponseWriter, r *http.Request) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "healthy",
		"clients":  len(f.clients),
		"timestamp": time.Now().Unix(),
	})
}

func (f *WSF) HandleStats(w http.ResponseWriter, r *http.Request) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"connections":   len(f.clients),
		"subscriptions": f.stats.TotalSubscriptions,
		"total_messages": f.stats.TotalMessages,
	})
}

// ============================================================================
// Main
// ============================================================================

func main() {
	log.Println("Starting TigerSwap Real-Time Trading Service...")

	f := NewWSF()

	router := mux.NewRouter()

	// WebSocket endpoint
	router.HandleFunc("/ws", f.HandleWebSocket)

	// REST endpoints
	router.HandleFunc("/health", f.HandleHealth)
	router.HandleFunc("/stats", f.HandleStats)

	// Market data REST endpoints
	router.HandleFunc("/api/v1/ticker/{pair}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		pair := vars["pair"]

		f.mu.RLock()
		ticker, ok := f.tickers[pair]
		f.mu.RUnlock()

		if !ok {
			http.NotFound(w, r)
			return
		}

		json.NewEncoder(w).Encode(ticker)
	})

	router.HandleFunc("/api/v1/orderbook/{pair}", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		pair := vars["pair"]

		f.mu.RLock()
		ob, ok := f.orderBooks[pair]
		f.mu.RUnlock()

		if !ok {
			http.NotFound(w, r)
			return
		}

		json.NewEncoder(w).Encode(ob)
	})

	// Start update loop
	ctx := make(chan struct{})
	go f.StartUpdateLoop(ctx)

	addr := ":8080"
	log.Printf("Real-Time Service listening on %s", addr)

	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatal(err)
	}
}
