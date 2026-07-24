package main

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"net"
	"net/http"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

// ============== CONSISTENT HASHING ==============

type ConsistentHash struct {
	ring           map[uint32][]string
	nodes          map[string]*Node
	virtualReplicas int
	sortedKeys     []uint32
	mu             sync.RWMutex
}

type Node struct {
	Host string
	Port int
	ID   string
}

func NewConsistentHash(replicas int) *ConsistentHash {
	return &ConsistentHash{
		ring:           make(map[uint32][]string),
		nodes:          make(map[string]*Node),
		virtualReplicas: replicas,
		sortedKeys:     make([]uint32, 0),
	}
}

func (h *ConsistentHash) Add(node *Node) {
	h.mu.Lock()
	defer h.mu.Unlock()
	
	h.nodes[node.ID] = node
	
	for i := 0; i < h.virtualReplicas; i++ {
		key := fmt.Sprintf("%s-%d-%s", node.ID, i, node.Host)
		hash := h.hash(key)
		h.ring[hash] = append(h.ring[hash], node.ID)
		
		pos := sort.Search(len(h.sortedKeys), func(i int) bool { return h.sortedKeys[i] >= hash })
		h.sortedKeys = append(h.sortedKeys, 0)
		copy(h.sortedKeys[pos+1:], h.sortedKeys[pos:])
		h.sortedKeys[pos] = hash
	}
}

func (h *ConsistentHash) Get(key string) *Node {
	h.mu.RLock()
	defer h.mu.RUnlock()
	
	if len(h.nodes) == 0 { return nil }
	
	hash := h.hash(key)
	pos := sort.Search(len(h.sortedKeys), func(i int) bool { return h.sortedKeys[i] >= hash })
	if pos >= len(h.sortedKeys) { pos = 0 }
	
	hash = h.sortedKeys[pos]
	if ids, ok := h.ring[hash]; ok && len(ids) > 0 {
		return h.nodes[ids[0]]
	}
	return nil
}

func (h *ConsistentHash) hash(key string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(key))
	return h.Sum32()
}

// ============== DISTRIBUTED CACHE ==============

type Shard struct {
	id    int
	data  map[string]interface{}
	mu    sync.RWMutex
	redis *redis.Client
}

type ShardedCache struct {
	shards    []*Shard
	numShards int
}

func NewShardedCache(numShards int, redisAddrs []string) *ShardedCache {
	shards := make([]*Shard, numShards)
	for i := 0; i < numShards; i++ {
		shards[i] = &Shard{
			id:    i,
			data:  make(map[string]interface{}),
			redis: redis.NewClient(&redis.Options{Addr: redisAddrs[i%len(redisAddrs)], PoolSize: 50}),
		}
	}
	return &ShardedCache{shards: shards, numShards: numShards}
}

func (s *ShardedCache) getShard(key string) *Shard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return s.shards[h.Sum32()%uint32(s.numShards)]
}

func (s *ShardedCache) Get(key string) (interface{}, bool) {
	return s.getShard(key).Get(key)
}

func (s *ShardedCache) Set(key string, value interface{}) {
	s.getShard(key).Set(key, value)
}

func (s *ShardedCache) Delete(key string) {
	s.getShard(key).Delete(key)
}

func (shard *Shard) Get(key string) (interface{}, bool) {
	shard.mu.RLock()
	if val, ok := shard.data[key]; ok {
		shard.mu.RUnlock()
		return val, true
	}
	shard.mu.RUnlock()
	
	ctx := context.Background()
	data, err := shard.redis.Get(ctx, key).Result()
	if err != nil { return nil, false }
	
	var result interface{}
	if err := json.Unmarshal([]byte(data), &result); err != nil { return nil, false }
	
	shard.mu.Lock()
	shard.data[key] = result
	shard.mu.Unlock()
	
	return result, true
}

func (shard *Shard) Set(key string, value interface{}) {
	shard.mu.Lock()
	shard.data[key] = value
	shard.mu.Unlock()
	
	ctx := context.Background()
	data, _ := json.Marshal(value)
	shard.redis.Set(ctx, key, data, 24*time.Hour)
}

func (shard *Shard) Delete(key string) {
	shard.mu.Lock()
	delete(shard.data, key)
	shard.mu.Unlock()
	shard.redis.Del(context.Background(), key)
}

// ============== RATE LIMITER ==============

type SlidingWindowRateLimiter struct {
	requests map[string][]time.Time
	mu       sync.RWMutex
	rate     int
	window   time.Duration
}

func NewSlidingWindowRateLimiter(rate int, window time.Duration) *SlidingWindowRateLimiter {
	rl := &SlidingWindowRateLimiter{
		requests: make(map[string][]time.Time),
		rate:     rate,
		window:   window,
	}
	go rl.cleanup()
	return rl
}

func (rl *SlidingWindowRateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	
	now := time.Now()
	cutoff := now.Add(-rl.window)
	
	var valid []time.Time
	for _, t := range rl.requests[key] {
		if t.After(cutoff) { valid = append(valid, t) }
	}
	
	if len(valid) >= rl.rate {
		rl.requests[key] = valid
		return false
	}
	
	rl.requests[key] = append(valid, now)
	return true
}

func (rl *SlidingWindowRateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-rl.window * 2)
		for key, requests := range rl.requests {
			var valid []time.Time
			for _, t := range requests {
				if t.After(cutoff) { valid = append(valid, t) }
			}
			if len(valid) == 0 {
				delete(rl.requests, key)
			} else {
				rl.requests[key] = valid
			}
		}
		rl.mu.Unlock()
	}
}

// ============== WEBSOCKET POOL ==============

type WSPool struct {
	clients    map[*websocket.Conn]bool
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	broadcast  chan []byte
	mu         sync.RWMutex
}

func NewWSPool() *WSPool {
	return &WSPool{
		clients:    make(map[*websocket.Conn]bool),
		register:   make(chan *websocket.Conn, 256),
		unregister: make(chan *websocket.Conn, 256),
		broadcast:  make(chan []byte, 1024),
	}
}

func (p *WSPool) Run() {
	for {
		select {
		case client := <-p.register:
			p.mu.Lock()
			p.clients[client] = true
			p.mu.Unlock()
		case client := <-p.unregister:
			p.mu.Lock()
			delete(p.clients, client)
			p.mu.Unlock()
		case message := <-p.broadcast:
			p.mu.RLock()
			for client := range p.clients {
				client.WriteMessage(websocket.TextMessage, message)
			}
			p.mu.RUnlock()
		}
	}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  256,
	WriteBufferSize: 256,
	CheckOrigin:    func(r *http.Request) bool { return true },
}

// ============== HANDLERS ==============

var cache *ShardedCache
var rateLimiter *SlidingWindowRateLimiter

func main() {
	redisAddrs := []string{getEnv("REDIS_HOST", "localhost:6379")}
	cache = NewShardedCache(16, redisAddrs)
	rateLimiter = NewSlidingWindowRateLimiter(10000, time.Second)
	
	pool := NewWSPool()
	go pool.Run()
	
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	
	r.GET("/api/v1/cache/:key", func(c *gin.Context) {
		if !rateLimiter.Allow(c.ClientIP()) {
			c.JSON(429, gin.H{"error": "Rate limit exceeded"})
			return
		}
		key := c.Param("key")
		val, ok := cache.Get(key)
		if !ok {
			c.JSON(404, gin.H{"error": "Not found"})
			return
		}
		c.JSON(200, gin.H{"key": key, "value": val})
	})
	
	r.POST("/api/v1/cache/:key", func(c *gin.Context) {
		key := c.Param("key")
		var req struct{ Value interface{} `json:"value"` }
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		cache.Set(key, req.Value)
		c.JSON(200, gin.H{"success": true})
	})
	
	r.DELETE("/api/v1/cache/:key", func(c *gin.Context) {
		cache.Delete(c.Param("key"))
		c.JSON(200, gin.H{"success": true})
	})
	
	r.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request)
		if err != nil { return }
		defer conn.Close()
		pool.register <- conn
		defer pool.unregister <- conn
		for {
			_, _, err := conn.ReadMessage()
			if err != nil { break }
		}
	})
	
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy", "shards": 16})
	})
	
	port := getEnv("PORT", "9091")
	fmt.Printf("Distributed Cache Server: port=%s shards=16\n", port)
	r.Run(":" + port)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
	return def
}
