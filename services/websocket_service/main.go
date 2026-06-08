package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ============ WebSocket Message Types ============

type MessageType string

const (
	// Subscribe messages
	MsgSubscribe   MessageType = "subscribe"
	MsgUnsubscribe MessageType = "unsubscribe"

	// Data messages
	MsgTicker      MessageType = "ticker"
	MsgTrade       MessageType = "trade"
	MsgOrderBook   MessageType = "orderbook"
	MsgPrice       MessageType = "price"
	MsgDepth       MessageType = "depth"
	MsgKline       MessageType = "kline"
	Msg liquidation MessageType = "liquidation"

	// Control messages
	MsgError     MessageType = "error"
	MsgPing       MessageType = "ping"
	MsgPong       MessageType = "pong"
	MsgSubscribed MessageType = "subscribed"
	MsgUnsubscribed MessageType = "unsubscribed"
)

// ============ Message Structures ============

type WSMessage struct {
	Type    MessageType     `json:"type"`
	Channel string          `json:"channel"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type SubscribePayload struct {
	Channels []string `json:"channels"`
	Pairs    []string `json:"pairs,omitempty"`
}

type TickerData struct {
	Pair       string  `json:"pair"`
	Bid        float64 `json:"bid"`
	Ask        float64 `json:"ask"`
	Last       float64 `json:"last"`
	Volume24h  float64 `json:"volume24h"`
	Change24h  float64 `json:"change24h"`
	High24h    float64 `json:"high24h"`
	Low24h     float64 `json:"low24h"`
	Timestamp  int64   `json:"timestamp"`
}

type TradeData struct {
	ID        string  `json:"id"`
	Pair      string  `json:"pair"`
	Price     float64 `json:"price"`
	Amount    float64 `json:"amount"`
	Side      string  `json:"side"`
	Timestamp int64   `json:"timestamp"`
}

type OrderBookEntry struct {
	Price  float64 `json:"price"`
	Amount float64 `json:"amount"`
}

type OrderBookData struct {
	Pair      string           `json:"pair"`
	Bids      []OrderBookEntry `json:"bids"`
	Asks      []OrderBookEntry `json:"asks"`
	Timestamp int64            `json:"timestamp"`
}

type KlineData struct {
	Pair      string  `json:"pair"`
	Open      float64 `json:"open"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Close     float64 `json:"close"`
	Volume    float64 `json:"volume"`
	Timestamp int64   `json:"timestamp"`
}

// ============ WebSocket Server ============

type WSServer struct {
	// Hub manages connections
	hub *ConnectionHub

	// Upgrader
	upgrader websocket.Upgrader

	// Config
	addr string
}

type ConnectionHub struct {
	// Registered connections
	connections map[*WSConnection]bool

	// Subscriptions
	subscriptions map[string]map[*WSConnection]bool

	// Broadcast
	broadcast chan []byte

	// Register
	register chan *WSConnection

	// Unregister
	unregister chan *WSConnection

	// Mutex
	mutex sync.RWMutex
}

type WSConnection struct {
	hub  *ConnectionHub
	conn *websocket.Conn

	// Buffered channel of outbound messages
	send chan []byte

	// Subscription
	subscriptions map[string]bool
}

type PricePublisher struct {
	hub       *ConnectionHub
	prices    map[string]float64
	publishers map[string]*time.Ticker
	mutex     sync.RWMutex
}

// NewWSServer creates a new WebSocket server
func NewWSServer(addr string) *WSServer {
	return &WSServer{
		addr: addr,
		hub:   NewConnectionHub(),
	}
}

// NewConnectionHub creates a new connection hub
func NewConnectionHub() *ConnectionHub {
	return &ConnectionHub{
		connections:  make(map[*WSConnection]bool),
		subscriptions: make(map[string]map[*WSConnection]bool),
		broadcast:    make(chan []byte, 256),
		register:     make(chan *WSConnection),
		unregister:   make(chan *WSConnection),
	}
}

// Run starts the hub
func (h *ConnectionHub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case conn := <-h.register:
			h.mutex.Lock()
			h.connections[conn] = true
			h.mutex.Unlock()

		case conn := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.connections[conn]; ok {
				delete(h.connections, conn)
				close(conn.send)
			}
			h.mutex.Unlock()

		case message := <-h.broadcast:
			h.mutex.RLock()
			for conn := range h.connections {
				select {
				case conn.send <- message:
				default:
					// Buffer full, close connection
					close(conn.send)
					delete(h.connections, conn)
				}
			}
			h.mutex.RUnlock()
		}
	}
}

// Subscribe subscribes a connection to a channel
func (h *ConnectionHub) Subscribe(conn *WSConnection, channel string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.subscriptions[channel] == nil {
		h.subscriptions[channel] = make(map[*WSConnection]bool)
	}
	h.subscriptions[channel][conn] = true
}

// Unsubscribe unsubscribes a connection from a channel
func (h *ConnectionHub) Unsubscribe(conn *WSConnection, channel string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if h.subscriptions[channel] != nil {
		delete(h.subscriptions[channel], conn)
	}
}

// Publish publishes a message to a channel
func (h *ConnectionHub) Publish(channel string, message []byte) {
	h.mutex.RLock()
	defer h.mutex.RUnlock()

	if subscribers, ok := h.subscriptions[channel]; ok {
		for conn := range subscribers {
			select {
			case conn.send <- message:
			default:
				// Buffer full
			}
		}
	}
}

// Broadcast broadcasts a message to all connections
func (h *ConnectionHub) Broadcast(message []byte) {
	select {
	case h.broadcast <- message:
	default:
		// Buffer full
	}
}

// HandleWebSocket handles WebSocket connections
func (s *WSServer) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Upgrade connection
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	// Create connection
	wsConn := &WSConnection{
		hub:           s.hub,
		conn:          conn,
		send:          make(chan []byte, 256),
		subscriptions: make(map[string]bool),
	}

	// Register
	s.hub.register <- wsConn

	// Start writer goroutine
	go s.writePump(wsConn)

	// Start reader goroutine
	go s.readPump(wsConn)
}

// readPump reads messages from the WebSocket connection
func (s *WSServer) readPump(conn *WSConnection) {
	defer func() {
		s.hub.unregister <- conn
		conn.conn.Close()
	}()

	conn.conn.SetReadLimit(4096)
	conn.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.conn.SetPongHandler(func(string) error {
		conn.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := conn.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// Handle message
		s.handleMessage(conn, message)
	}
}

// writePump writes messages to the WebSocket connection
func (s *WSServer) writePump(conn *WSConnection) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		conn.conn.Close()
	}()

	for {
		select {
		case message, ok := <-conn.send:
			conn.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Hub closed channel
				conn.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := conn.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages
			n := len(conn.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-conn.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			conn.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage handles incoming WebSocket messages
func (s *WSServer) handleMessage(conn *WSConnection, message []byte) {
	var msg WSMessage
	if err := json.Unmarshal(message, &msg); err != nil {
		s.sendError(conn, "Invalid message format")
		return
	}

	switch msg.Type {
	case MsgSubscribe:
		s.handleSubscribe(conn, msg.Payload)
	case MsgUnsubscribe:
		s.handleUnsubscribe(conn, msg.Payload)
	case MsgPing:
		s.sendPong(conn)
	default:
		s.sendError(conn, "Unknown message type")
	}
}

// handleSubscribe handles subscription requests
func (s *WSServer) handleSubscribe(conn *WSConnection, payload json.RawMessage) {
	var sub SubscribePayload
	if err := json.Unmarshal(payload, &sub); err != nil {
		s.sendError(conn, "Invalid subscribe payload")
		return
	}

	for _, channel := range sub.Channels {
		conn.subscriptions[channel] = true
		s.hub.Subscribe(conn, channel)
	}

	// Send confirmation
	response := WSMessage{
		Type:    MsgSubscribed,
		Channel: "subscriptions",
	}
	responseJSON, _ := json.Marshal(response)
	conn.send <- responseJSON
}

// handleUnsubscribe handles unsubscription requests
func (s *WSServer) handleUnsubscribe(conn *WSConnection, payload json.RawMessage) {
	var sub SubscribePayload
	if err := json.Unmarshal(payload, &sub); err != nil {
		s.sendError(conn, "Invalid unsubscribe payload")
		return
	}

	for _, channel := range sub.Channels {
		delete(conn.subscriptions, channel)
		s.hub.Unsubscribe(conn, channel)
	}

	// Send confirmation
	response := WSMessage{
		Type:    MsgUnsubscribed,
		Channel: "subscriptions",
	}
	responseJSON, _ := json.Marshal(response)
	conn.send <- responseJSON
}

func (s *WSServer) sendError(conn *WSConnection, errMsg string) {
	response := WSMessage{
		Type: MsgError,
	}
	responseJSON, _ := json.Marshal(map[string]string{"error": errMsg})
	conn.send <- responseJSON
}

func (s *WSServer) sendPong(conn *WSConnection) {
	response := WSMessage{
		Type: MsgPong,
	}
	responseJSON, _ := json.Marshal(response)
	conn.send <- responseJSON
}

// PublishTicker publishes ticker data
func (p *PricePublisher) PublishTicker(data TickerData) {
	channel := fmt.Sprintf("ticker:%s", data.Pair)
	
	msg := WSMessage{
		Type:    MsgTicker,
		Channel: channel,
	}
	msgJSON, _ := json.Marshal(data)
	msg.Payload = msgJSON

	response, _ := json.Marshal(msg)
	p.hub.Publish(channel, response)
}

// PublishTrade publishes trade data
func (p *PricePublisher) PublishTrade(data TradeData) {
	channel := fmt.Sprintf("trade:%s", data.Pair)
	
	msg := WSMessage{
		Type:    MsgTrade,
		Channel: channel,
	}
	msgJSON, _ := json.Marshal(data)
	msg.Payload = msgJSON

	response, _ := json.Marshal(msg)
	p.hub.Publish(channel, response)
}

// PublishOrderBook publishes order book data
func (p *PricePublisher) PublishOrderBook(data OrderBookData) {
	channel := fmt.Sprintf("orderbook:%s", data.Pair)
	
	msg := WSMessage{
		Type:    MsgOrderBook,
		Channel: channel,
	}
	msgJSON, _ := json.Marshal(data)
	msg.Payload = msgJSON

	response, _ := json.Marshal(msg)
	p.hub.Publish(channel, response)
}

// Run starts the WebSocket server
func (s *WSServer) Run(ctx context.Context) error {
	// Start hub
	go s.hub.Run(ctx)

	// HTTP handler
	http.HandleFunc("/ws", s.HandleWebSocket)

	log.Printf("WebSocket server starting on %s", s.addr)

	server := &http.Server{
		Addr: s.addr,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("WebSocket server error: %v", err)
		}
	}()

	<-ctx.Done()

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return server.Shutdown(ctx)
}

func main() {
	// Create server
	server := NewWSServer(":8080")

	// Create context
	ctx, cancel := context.WithCancel(context.Background())

	// Run server
	if err := server.Run(ctx); err != nil {
		log.Printf("Server error: %v", err)
	}

	cancel()
}