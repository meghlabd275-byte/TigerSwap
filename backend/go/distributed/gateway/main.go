package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

var (
	requestsTotal  int64
	requestsActive int64
	bytesIn        int64
	bytesOut       int64
	startTime      = time.Now()
	mu             sync.RWMutex
	routes         = make(map[string]int)
)

type GatewayConfig struct {
	Port       string
	MaxConns   int
	RateLimit  int
	Timeout    time.Duration
	BackendURL string
}

type Metrics struct {
	RequestsTotal  int64          `json:"requests_total"`
	RequestsActive int64          `json:"requests_active"`
	BytesIn        int64          `json:"bytes_in"`
	BytesOut       int64          `json:"bytes_out"`
	Uptime         string         `json:"uptime"`
	Routes         map[string]int `json:"routes"`
}

func main() {
	cfg := GatewayConfig{
		Port:       getEnv("PORT", "8080"),
		MaxConns:   100000,
		RateLimit:  10000,
		Timeout:    30 * time.Second,
		BackendURL: getEnv("BACKEND_URL", "http://localhost:9090"),
	}

	proxy, err := newProxy(cfg.BackendURL)
	if err != nil {
		log.Fatalf("invalid BACKEND_URL %q: %v", cfg.BackendURL, err)
	}

	limiter := rate.NewLimiter(rate.Limit(cfg.RateLimit), cfg.RateLimit)

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	r.Use(rateLimitMiddleware(limiter))
	r.Use(middlewareMetrics)

	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{"service": "TigerSwap Gateway", "version": "1.0.0"})
	})

	// Every /api request is forwarded verbatim to the upstream backend. The
	// proxy streams the real response and never fabricates a body.
	r.Any("/api/*any", gin.WrapH(proxy))

	r.GET("/metrics", metricsHandler)
	r.GET("/health", healthHandler)

	srv := &http.Server{
		Addr:           ":" + cfg.Port,
		Handler:        r,
		ReadTimeout:    cfg.Timeout,
		WriteTimeout:   cfg.Timeout,
		MaxHeaderBytes: 1 << 16,
	}

	go func() {
		fmt.Printf("Gateway starting on port %s\n", cfg.Port)
		fmt.Printf("Upstream backend: %s\n", cfg.BackendURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("gateway server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("gateway shutdown error: %v", err)
	}
}

// newProxy builds a reverse proxy for the upstream backend. The target must be
// an absolute http/https URL; anything else is a configuration error rather
// than something to guess at.
func newProxy(backendURL string) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(backendURL)
	if err != nil {
		return nil, err
	}
	if target.Scheme != "http" && target.Scheme != "https" {
		return nil, fmt.Errorf("scheme must be http or https, got %q", target.Scheme)
	}
	if target.Host == "" {
		return nil, fmt.Errorf("host is empty")
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		log.Printf("upstream %s failed for %s %s: %v", target, req.Method, req.URL.Path, err)
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error":"upstream backend unavailable"}`))
	}
	return proxy, nil
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// rateLimitMiddleware rejects requests above the configured rate with 429
// rather than letting them through unchecked.
func rateLimitMiddleware(limiter *rate.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			return
		}
		c.Next()
	}
}

func middlewareMetrics(c *gin.Context) {
	atomic.AddInt64(&requestsTotal, 1)
	atomic.AddInt64(&requestsActive, 1)
	defer atomic.AddInt64(&requestsActive, -1)

	mu.Lock()
	routes[c.FullPath()]++
	mu.Unlock()

	atomic.AddInt64(&bytesIn, c.Request.ContentLength)

	c.Next()

	atomic.AddInt64(&bytesOut, int64(c.Writer.Size()))
}

func metricsHandler(c *gin.Context) {
	mu.RLock()
	routesCopy := make(map[string]int, len(routes))
	for k, v := range routes {
		routesCopy[k] = v
	}
	mu.RUnlock()

	m := Metrics{
		RequestsTotal:  atomic.LoadInt64(&requestsTotal),
		RequestsActive: atomic.LoadInt64(&requestsActive),
		BytesIn:        atomic.LoadInt64(&bytesIn),
		BytesOut:       atomic.LoadInt64(&bytesOut),
		Uptime:         time.Since(startTime).String(),
		Routes:         routesCopy,
	}
	c.JSON(200, m)
}

func healthHandler(c *gin.Context) {
	c.JSON(200, gin.H{"status": "healthy", "timestamp": time.Now().Unix()})
}
