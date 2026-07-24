package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// ============== MESSAGE QUEUE ==============

type Message struct {
	ID        string          `json:"id"`
	Type     string          `json:"type"`
	Payload  json.RawMessage `json:"payload"`
	Priority int             `json:"priority"`
	Retry    int             `json:"retry"`
	Created  int64           `json:"created"`
}

type Queue struct {
	name      string
	mu        sync.RWMutex
	messages  []*Message
	redis     *redis.Client
	processed int64
	delivered int64
	failed    int64
}

func NewQueue(name string, redisAddr string) *Queue {
	q := &Queue{
		name:     name,
		messages: make([]*Message, 0, 1000),
		redis:    redis.NewClient(&redis.Options{Addr: redisAddr, PoolSize: 50}),
	}
	return q
}

func (q *Queue) Push(msg *Message) {
	q.mu.Lock()
	defer q.mu.Unlock()
	msg.ID = fmt.Sprintf("%d-%d", time.Now().UnixNano(), rand.Intn(10000))
	msg.Created = time.Now().UnixMilli()
	
	pos := 0
	for i, m := range q.messages {
		if msg.Priority > m.Priority {
			pos = i
			break
		}
		pos = i + 1
	}
	
	q.messages = append(q.messages, nil)
	copy(q.messages[pos+1:], q.messages[pos:])
	q.messages[pos] = msg
	
	ctx := context.Background()
	data, _ := json.Marshal(msg)
	q.redis.LPush(ctx, fmt.Sprintf("queue:%s", q.name), data)
}

func (q *Queue) Pop() *Message {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.messages) == 0 { return nil }
	msg := q.messages[0]
	q.messages = q.messages[1:]
	atomic.AddInt64(&q.processed, 1)
	return msg
}

func (q *Queue) Ack() {
	atomic.AddInt64(&q.delivered, 1)
	atomic.AddInt64(&q.processed, -1)
}

func (q *Queue) Nack() {
	atomic.AddInt64(&q.failed, 1)
	atomic.AddInt64(&q.processed, -1)
}

func (q *Queue) Size() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.messages)
}

func (q *Queue) Stats() (processed, delivered, failed int64) {
	return atomic.LoadInt64(&q.processed),
		atomic.LoadInt64(&q.delivered),
		atomic.LoadInt64(&q.failed)
}

// ============== ANALYTICS ==============

type Counter struct{ value int64 }
func (c *Counter) Incr(n int64) { atomic.AddInt64(&c.value, n) }
func (c *Counter) Value() int64 { return atomic.LoadInt64(&c.value) }

type Gauge struct{ value uint64 }
func (g *Gauge) Set(v float64) { atomic.StoreUint64(&g.value, math.Float64bits(v)) }
func (g *Gauge) Value() float64 { return math.Float64frombits(atomic.LoadUint64(&g.value)) }

type AnalyticsEngine struct {
	mu         sync.RWMutex
	counters   map[string]*Counter
	gauges     map[string]*Gauge
	redis      *redis.Client
}

func NewAnalytics(redisAddr string) *AnalyticsEngine {
	return &AnalyticsEngine{
		counters: make(map[string]*Counter),
		gauges:   make(map[string]*Gauge),
		redis:    redis.NewClient(&redis.Options{Addr: redisAddr, PoolSize: 10}),
	}
}

func (a *AnalyticsEngine) Counter(name string) *Counter {
	a.mu.Lock()
	defer a.mu.Unlock()
	if c, ok := a.counters[name]; ok { return c }
	c := &Counter{}
	a.counters[name] = c
	return c
}

func (a *AnalyticsEngine) Gauge(name string) *Gauge {
	a.mu.Lock()
	defer a.mu.Unlock()
	if g, ok := a.gauges[name]; ok { return g }
	g := &Gauge{}
	a.gauges[name] = g
	return g
}

// ============== STREAMS ==============

type StreamProcessor struct {
	mu       sync.RWMutex
	streams  map[string]Handler
	handlers map[string]Handler
	redis    *redis.Client
}

type Handler func(msg *Message) error

func NewStreamProcessor(redisAddr string) *StreamProcessor {
	return &StreamProcessor{
		streams:  make(map[string]Handler),
		handlers: make(map[string]Handler),
		redis:    redis.NewClient(&redis.Options{Addr: redisAddr, PoolSize: 50}),
	}
}

func (sp *StreamProcessor) RegisterHandler(stream string, handler Handler) {
	sp.mu.Lock()
	defer sp.mu.Unlock()
	sp.handlers[stream] = handler
}

func (sp *StreamProcessor) Publish(stream string, msg *Message) error {
	data, _ := json.Marshal(msg)
	ctx := context.Background()
	return sp.redis.XAdd(ctx, &redis.XAddArgs{
		Stream: stream,
		Values: map[string]interface{}{"data": string(data)},
	}).Err()
}

// ============== HANDLERS ==============

var queue *Queue
var analytics *AnalyticsEngine
var streamProc *StreamProcessor

func main() {
	redisAddr := getEnv("REDIS_HOST", "localhost:6379")
	
	queue = NewQueue("tigerswap", redisAddr)
	analytics = NewAnalytics(redisAddr)
	streamProc = NewStreamProcessor(redisAddr)
	
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	
	// Queue routes
	r.POST("/api/v1/queue/:name/push", func(c *gin.Context) {
		var msg Message
		if err := c.ShouldBindJSON(&msg); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		queue.Push(&msg)
		c.JSON(200, gin.H{"success": true})
	})
	
	r.GET("/api/v1/queue/:name/pop", func(c *gin.Context) {
		msg := queue.Pop()
		if msg == nil {
			c.JSON(404, gin.H{"error": "empty"})
			return
		}
		c.JSON(200, msg)
	})
	
	r.POST("/api/v1/queue/:name/ack", func(c *gin.Context) {
		queue.Ack()
		c.JSON(200, gin.H{"success": true})
	})
	
	r.GET("/api/v1/queue/:name/size", func(c *gin.Context) {
		c.JSON(200, gin.H{"size": queue.Size()})
	})
	
	// Analytics routes
	r.POST("/api/v1/analytics/counter/:name", func(c *gin.Context) {
		var req struct{ Value int64 }
		c.ShouldBindJSON(&req)
		analytics.Counter(c.Param("name")).Incr(req.Value)
		c.JSON(200, gin.H{"success": true})
	})
	
	r.POST("/api/v1/analytics/gauge/:name", func(c *gin.Context) {
		var req struct{ Value float64 }
		c.ShouldBindJSON(&req)
		analytics.Gauge(c.Param("name")).Set(req.Value)
		c.JSON(200, gin.H{"success": true})
	})
	
	r.GET("/api/v1/analytics/stats", func(c *gin.Context) {
		p, d, f := queue.Stats()
		c.JSON(200, gin.H{
			"queue":     gin.H{"processed": p, "delivered": d, "failed": f},
			"goroutines": runtime.NumGoroutine(),
		})
	})
	
	// Health
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})
	
	port := getEnv("PORT", "9092")
	fmt.Printf("TigerSwap MQ & Analytics: port=%s\n", port)
	
	srv := &http.Server{Addr: ":" + port, Handler: r}
	
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-quit
		srv.Shutdown(context.Background())
		os.Exit(0)
	}()
	
	srv.ListenAndServe()
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" { return v }
	return def
}
