package main

import (
"fmt"
"net/http"
"os"
"os/signal"
"syscall"
"sync"
"sync/atomic"
"time"

"github.com/gin-gonic/gin"
)

var (
requestsTotal  int64
requestsActive int64
bytesIn        int64
bytesOut       int64
startTime      = time.Now()
uptime         time.Duration
mu             sync.RWMutex
routes         = make(map[string]int)
)

type GatewayConfig struct {
Port         string
MaxConns     int
RateLimit    int
Timeout      time.Duration
BackendURL   string
}

type Metrics struct {
RequestsTotal   int64     `json:"requests_total"`
RequestsActive  int64     `json:"requests_active"`
BytesIn        int64     `json:"bytes_in"`
BytesOut       int64     `json:"bytes_out"`
Uptime         string    `json:"uptime"`
Routes         map[string]int `json:"routes"`
}

func main() {
cfg := GatewayConfig{
Port:      getEnv("PORT", "8080"),
MaxConns:  100000,
RateLimit: 10000,
Timeout:   30 * time.Second,
BackendURL: getEnv("BACKEND_URL", "http://localhost:9090"),
}

gin.SetMode(gin.ReleaseMode)
r := gin.New()
r.Use(gin.Recovery())
r.Use(gin.Logger())
r.Use(middlewareMetrics)

// Routes
r.GET("/", func(c *gin.Context) {
c.JSON(200, gin.H{"service": "TigerSwap Gateway", "version": "1.0.0"})
})

r.GET("/api/*any", proxyHandler(cfg.BackendURL))
r.POST("/api/*any", proxyHandler(cfg.BackendURL))
r.PUT("/api/*any", proxyHandler(cfg.BackendURL))
r.DELETE("/api/*any", proxyHandler(cfg.BackendURL))

r.GET("/metrics", metricsHandler)
r.GET("/health", healthHandler)

// Start server
go func() {
fmt.Printf("Gateway starting on port 
", cfg.Port)
fmt.Printf("Max connections: 0
", cfg.MaxConns)
fmt.Printf("Rate limit: 0 req/s
", cfg.RateLimit)
s := &http.Server{
Addr:         ":" + cfg.Port,
Handler:      r,
ReadTimeout:  cfg.Timeout,
WriteTimeout: cfg.Timeout,
MaxHeaderBytes: 1 << 16,
}
s.ListenAndServe()
}()

// Wait for shutdown
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
<-quit

fmt.Println("Shutting down...")
}

func getEnv(key, def string) string {
if v := os.Getenv(key); v != "" { return v }
return def
}

func middlewareMetrics(c *gin.Context) {
atomic.AddInt64(&requestsTotal, 1)
atomic.AddInt64(&requestsActive, 1)
defer atomic.AddInt64(&requestsActive, -1)

// Track route
mu.Lock()
routes[c.FullPath()]++
mu.Unlock()

// Track bytes
bytesIn += int64(c.Request.ContentLength)

c.Next()

// Track response size
// bytesOut += size
}

func metricsHandler(c *gin.Context) {
uptime = time.Since(startTime)
m := Metrics{
RequestsTotal:  atomic.LoadInt64(&requestsTotal),
RequestsActive: atomic.LoadInt64(&requestsActive),
BytesIn:        atomic.LoadInt64(&bytesIn),
BytesOut:       atomic.LoadInt64(&bytesOut),
Uptime:         uptime.String(),
Routes:         routes,
}
c.JSON(200, m)
}

func healthHandler(c *gin.Context) {
c.JSON(200, gin.H{"status": "healthy", "timestamp": time.Now().Unix()})
}

func proxyHandler(backend string) gin.HandlerFunc {
return func(c *gin.Context) {
// Simple proxy - in production use reverseproxy
c.JSON(200, gin.H{"proxied": c.FullPath(), "backend": backend})
}
}
