// TigerSwap API Gateway - Go Implementation
// REST API, WebSocket, Rate Limiting, Authentication

package main

import (
    "encoding/json"
    "log"
    "net/http"
    "sync"
    "time"

    "github.com/gorilla/mux"
    "github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
    ReadBufferSize:  1024,
    WriteBufferSize: 1024,
}

type APIGateway struct {
    mu          sync.RWMutex
    connections map[string]*websocket.Conn
    rateLimiter *RateLimiter
}

type RateLimiter struct {
    requests map[string][]time.Time
    mu       sync.RWMutex
}

func NewRateLimiter() *RateLimiter {
    return &RateLimiter{
        requests: make(map[string][]time.Time),
    }
}

func (rl *RateLimiter) Allow(ip string, limit int, window time.Duration) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()

    now := time.Now()
    windowStart := now.Add(-window)

    // Filter old requests
    var valid []time.Time
    for _, t := range rl.requests[ip] {
        if t.After(windowStart) {
            valid = append(valid, t)
        }
    }

    if len(valid) >= limit {
        rl.requests[ip] = valid
        return false
    }

    rl.requests[ip] = append(valid, now)
    return true
}

func (ag *APIGateway) HandleQuote(w http.ResponseWriter, r *http.Request) {
    if !ag.rateLimiter.Allow(r.RemoteAddr, 100, time.Minute) {
        http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
        return
    }

    respondJSON(w, map[string]interface{}{
        "success": true,
        "price":   2000.0,
        "output":  "1000000",
    })
}

func (ag *APIGateway) HandleSwap(w http.ResponseWriter, r *http.Request) {
    respondJSON(w, map[string]interface{}{
        "success":   true,
        "tx_hash":    "0x123",
        "status":     "pending",
    })
}

func (ag *APIGateway) HandleWS(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        log.Printf("WebSocket upgrade failed: %v", err)
        return
    }

    ag.mu.Lock()
    id := r.RemoteAddr
    ag.connections[id] = conn
    ag.mu.Unlock()

    defer func() {
        ag.mu.Lock()
        delete(ag.connections, id)
        ag.mu.Unlock()
        conn.Close()
    }()

    for {
        _, msg, err := conn.ReadMessage()
        if err != nil {
            break
        }

        // Broadcast price updates
        var data map[string]interface{}
        json.Unmarshal(msg, &data)

        ag.mu.RLock()
        for _, c := range ag.connections {
            c.WriteJSON(map[string]interface{}{
                "type": "price_update",
                "data": data,
            })
        }
        ag.mu.RUnlock()
    }
}

func respondJSON(w http.ResponseWriter, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(data)
}

func main() {
    gateway := &APIGateway{
        connections: make(map[string]*websocket.Conn),
        rateLimiter: NewRateLimiter(),
    }

    router := mux.NewRouter()

    // REST endpoints
    router.HandleFunc("/api/v1/quote", gateway.HandleQuote).Methods("GET")
    router.HandleFunc("/api/v1/swap", gateway.HandleSwap).Methods("POST")
    router.HandleFunc("/api/v1/swap/{tx_hash}", gateway.HandleSwap).Methods("GET")

    // WebSocket
    router.HandleFunc("/ws", gateway.HandleWS)

    // Health
    router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        respondJSON(w, map[string]interface{}{"status": "healthy"})
    })

    log.Println("TigerSwap API Gateway starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", router))
}
