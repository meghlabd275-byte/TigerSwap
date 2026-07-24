package main

import (
"fmt"
"net/http"
"os"
"os/signal"
"sync"
"sync/atomic"
"syscall"

"github.com/gin-gonic/gin"
)

type LRUCache struct {
mu       sync.RWMutex
items    map[string]*list.Element
list     *list.List
capacity int
hits     int64
misses   int64
}

type lruItem struct {
key   string
value interface{}
}

func NewLRUCache(capacity int) *LRUCache {
return &LRUCache{
items:    make(map[string]*list.Element),
list:     list.New(),
capacity: capacity,
}
}

func (c *LRUCache) Get(key string) (interface{}, bool) {
c.mu.Lock()
defer c.mu.Unlock()
elem, ok := c.items[key]
if !ok {
atomic.AddInt64(&c.misses, 1)
return nil, false
}
c.list.MoveToFront(elem)
atomic.AddInt64(&c.hits, 1)
return elem.Value.(*lruItem).value, true
}

func (c *LRUCache) Set(key string, value interface{}) {
c.mu.Lock()
defer c.mu.Unlock()
if elem, ok := c.items[key]; ok {
elem.Value.(*lruItem).value = value
c.list.MoveToFront(elem)
return
}
elem := c.list.PushFront(&lruItem{key, value})
c.items[key] = elem
if c.list.Len() > c.capacity {
if oldest := c.list.Back(); oldest != nil {
c.list.Remove(oldest)
delete(c.items, oldest.Value.(*lruItem).key)
}
}
}

var lruCache *LRUCache

func main() {
lruCache = NewLRUCache(10000)
gin.SetMode(gin.ReleaseMode)
r := gin.New()
r.Use(gin.Recovery())
r.GET("/api/v1/cache/:key", func(c *gin.Context) {
key := c.Param("key")
if val, ok := lruCache.Get(key); ok {
c.JSON(200, gin.H{"key": key, "value": val})
return
}
c.JSON(404, gin.H{"error": "not found"})
})
r.POST("/api/v1/cache/:key", func(c *gin.Context) {
key := c.Param("key")
var req struct{ Value interface{} }
c.ShouldBindJSON(&req)
lruCache.Set(key, req.Value)
c.JSON(200, gin.H{"success": true})
})
r.GET("/health", func(c *gin.Context) {
c.JSON(200, gin.H{"status": "healthy"})
})
port := os.Getenv("PORT")
if port == "" { port = "9093" }
fmt.Printf("Cache Server: port=%s\n", port)
srv := &http.Server{Addr: ":" + port, Handler: r}
quit := make(chan os.Signal, 1)
signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
go func() { <-quit; srv.Shutdown(nil) }()
srv.ListenAndServe()
}
