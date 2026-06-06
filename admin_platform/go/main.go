// TigerSwap Admin Platform - Go Microservices
// Production-ready admin API with DEX/CEX/Wallet management

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
// Data Structures
// ============================================================================

type Admin struct {
    ID        string `json:"id"`
    Username  string `json:"username"`
    Email     string `json:"email"`
    Role      string `json:"role"`
    IsActive  bool   `json:"is_active"`
    CreatedAt int64  `json:"created_at"`
}

type ConnectedDEX struct {
    ID        string  `json:"id"`
    Name      string  `json:"name"`
    ChainID   uint64  `json:"chain_id"`
    Status    string  `json:"status"`
    Volume24h float64 `json:"volume_24h"`
    TradingFee float64 `json:"trading_fee"`
}

type ConnectedCEX struct {
    ID      string `json:"id"`
    Name    string `json:"name"`
    Status  string `json:"status"`
    CanTrade bool  `json:"can_trade"`
}

type HDWallet struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    Type   string `json:"type"`
    Balance float64 `json:"balance"`
}

type AuditLog struct {
    ID        string `json:"id"`
    AdminID   string `json:"admin_id"`
    Action    string `json:"action"`
    Timestamp int64  `json:"timestamp"`
}

type FeeStructure struct {
    ID        string  `json:"id"`
    FeeType   string  `json:"fee_type"`
    MakerFee  float64 `json:"maker_fee"`
    TakerFee  float64 `json:"taker_fee"`
}

type PlatformStats struct {
    TotalVolume24h float64 `json:"total_volume_24h"`
    TotalUsers     int64   `json:"total_users"`
    TotalTrades24h int64   `json:"total_trades_24h"`
    DEXCount       int     `json:"dex_count"`
    CEXCount       int     `json:"cex_count"`
}

// ============================================================================
// Admin Service
// ============================================================================

type AdminService struct {
    mu      sync.RWMutex
    admins  map[string]*Admin
    dexes   map[string]*ConnectedDEX
    cexes   map[string]*ConnectedCEX
    wallets map[string]*HDWallet
    fees    map[string]*FeeStructure
    audit   []*AuditLog
}

func NewAdminService() *AdminService {
    svc := &AdminService{
        admins:  make(map[string]*Admin),
        dexes:  make(map[string]*ConnectedDEX),
        cexes:  make(map[string]*ConnectedCEX),
        wallets: make(map[string]*HDWallet),
        fees:   make(map[string]*FeeStructure),
        audit:  make([]*AuditLog, 0),
    }
    
    // Initialize default admin
    svc.admins["admin_001"] = &Admin{
        ID: "admin_001",
        Username: "tigerswap_admin",
        Email: "admin@tigerswap.io",
        Role: "super_admin",
        IsActive: true,
        CreatedAt: time.Now().Unix(),
    }
    
    // Initialize default DEXes
    svc.initializeDefaultDEXes()
    
    return svc
}

func (s *AdminService) initializeDefaultDEXes() {
    s.dexes["dex_uniswap_v2"] = &ConnectedDEX{
        ID: "dex_uniswap_v2",
        Name: "Uniswap V2",
        ChainID: 1,
        Status: "active",
        Volume24h: 150_000_000,
        TradingFee: 0.003,
    }
    s.dexes["dex_uniswap_v3"] = &ConnectedDEX{
        ID: "dex_uniswap_v3",
        Name: "Uniswap V3",
        ChainID: 1,
        Status: "active",
        Volume24h: 500_000_000,
        TradingFee: 0.003,
    }
    s.dexes["dex_pancakeswap"] = &ConnectedDEX{
        ID: "dex_pancakeswap",
        Name: "PancakeSwap",
        ChainID: 56,
        Status: "active",
        Volume24h: 200_000_000,
        TradingFee: 0.0025,
    }
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func (s *AdminService) ListDEXes(w http.ResponseWriter, r *http.Request) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    dexes := make([]*ConnectedDEX, 0, len(s.dexes))
    for _, dex := range s.dexes {
        dexes = append(dexes, dex)
    }
    
    respondJSON(w, map[string]interface{}{
        "success": true,
        "data":    dexes,
        "count":   len(dexes),
    })
}

func (s *AdminService) GetDEXStats(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    dexID := vars["dex_id"]
    
    s.mu.RLock()
    dex, ok := s.dexes[dexID]
    s.mu.RUnlock()
    
    if !ok {
        http.Error(w, "DEX not found", http.StatusNotFound)
        return
    }
    
    respondJSON(w, map[string]interface{}{
        "success": true,
        "data":    dex,
    })
}

func (s *AdminService) ModifyDEX(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    dexID := vars["dex_id"]
    
    var updates map[string]interface{}
    if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    s.mu.Lock()
    defer s.mu.Unlock()
    
    dex, ok := s.dexes[dexID]
    if !ok {
        http.Error(w, "DEX not found", http.StatusNotFound)
        return
    }
    
    if status, ok := updates["status"].(string); ok {
        dex.Status = status
    }
    if fee, ok := updates["trading_fee"].(float64); ok {
        dex.TradingFee = fee
    }
    
    s.logAction("admin_001", "dex:modify", "dex", dexID, updates)
    
    respondJSON(w, map[string]interface{}{
        "success": true,
        "data":    dex,
    })
}

func (s *AdminService) SuspendDEX(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    dexID := vars["dex_id"]
    
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if dex, ok := s.dexes[dexID]; ok {
        dex.Status = "suspended"
        s.logAction("admin_001", "dex:suspend", "dex", dexID, map[string]interface{}{
            "reason": "Admin action",
        })
    }
    
    respondJSON(w, map[string]interface{}{
        "success": true,
        "message": "DEX suspended successfully",
    })
}

func (s *AdminService) ListCEXes(w http.ResponseWriter, r *http.Request) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    cexes := make([]*ConnectedCEX, 0, len(s.cexes))
    for _, cex := range s.cexes {
        cexes = append(cexes, cex)
    }
    
    respondJSON(w, map[string]interface{}{
        "success": true,
        "data":    cexes,
    })
}

func (s *AdminService) ConnectCEX(w http.ResponseWriter, r *http.Request) {
    var req struct {
        Name  string `json:"name"`
        Type  string `json:"exchange_type"`
        APIKey string `json:"api_key"`
    }
    
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    cex := &ConnectedCEX{
        ID:      fmt.Sprintf("cex_%s_%d", req.Type, time.Now().Unix()),
        Name:    req.Name,
        Status:  "connected",
        CanTrade: true,
    }
    
    s.mu.Lock()
    s.cexes[cex.ID] = cex
    s.mu.Unlock()
    
    respondJSON(w, map[string]interface{}{
        "success": true,
        "data":    cex,
    })
}

func (s *AdminService) GetPlatformStats(w http.ResponseWriter, r *http.Request) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    var totalVolume float64
    for _, dex := range s.dexes {
        if dex.Status == "active" {
            totalVolume += dex.Volume24h
        }
    }
    
    stats := PlatformStats{
        TotalVolume24h: totalVolume,
        TotalUsers:     50000,
        TotalTrades24h: 100000,
        DEXCount:       len(s.dexes),
        CEXCount:       len(s.cexes),
    }
    
    respondJSON(w, map[string]interface{}{
        "success": true,
        "data":    stats,
    })
}

func (s *AdminService) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    start := len(s.audit) - 100
    if start < 0 {
        start = 0
    }
    
    respondJSON(w, map[string]interface{}{
        "success": true,
        "data":    s.audit[start:],
    })
}

func (s *AdminService) ManageWallets(w http.ResponseWriter, r *http.Request) {
    // List all wallets
    s.mu.RLock()
    wallets := make([]*HDWallet, 0, len(s.wallets))
    for _, wallet := range s.wallets {
        wallets = append(wallets, wallet)
    }
    s.mu.RUnlock()
    
    respondJSON(w, map[string]interface{}{
        "success": true,
        "data":    wallets,
    })
}

func (s *AdminService) ManageFees(w http.ResponseWriter, r *http.Request) {
    s.mu.RLock()
    fees := make([]*FeeStructure, 0, len(s.fees))
    for _, fee := range s.fees {
        fees = append(fees, fee)
    }
    s.mu.RUnlock()
    
    respondJSON(w, map[string]interface{}{
        "success": true,
        "data":    fees,
    })
}

func (s *AdminService) logAction(adminID, action, resourceType, resourceID string, details map[string]interface{}) {
    log := &AuditLog{
        ID:        fmt.Sprintf("log_%d", time.Now().UnixNano()),
        AdminID:   adminID,
        Action:    action,
        Timestamp: time.Now().Unix(),
    }
    s.audit = append(s.audit, log)
    
    // Keep only last 10000 logs
    if len(s.audit) > 10000 {
        s.audit = s.audit[len(s.audit)-10000:]
    }
}

func respondJSON(w http.ResponseWriter, data interface{}) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(data)
}

// ============================================================================
// Main
// ============================================================================

func main() {
    svc := NewAdminService()
    
    router := mux.NewRouter()
    router.Use(LoggingMiddleware)
    router.Use(RecoveryMiddleware)
    
    // DEX routes
    router.HandleFunc("/api/admin/dexes", svc.ListDEXes).Methods("GET")
    router.HandleFunc("/api/admin/dexes/{dex_id}", svc.GetDEXStats).Methods("GET")
    router.HandleFunc("/api/admin/dexes/{dex_id}", svc.ModifyDEX).Methods("PUT")
    router.HandleFunc("/api/admin/dexes/{dex_id}/suspend", svc.SuspendDEX).Methods("POST")
    
    // CEX routes
    router.HandleFunc("/api/admin/cexes", svc.ListCEXes).Methods("GET")
    router.HandleFunc("/api/admin/cexes", svc.ConnectCEX).Methods("POST")
    
    // Stats & Audit
    router.HandleFunc("/api/admin/stats", svc.GetPlatformStats).Methods("GET")
    router.HandleFunc("/api/admin/audit", svc.GetAuditLogs).Methods("GET")
    router.HandleFunc("/api/admin/wallets", svc.ManageWallets).Methods("GET")
    router.HandleFunc("/api/admin/fees", svc.ManageFees).Methods("GET")
    
    // Health
    router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"status":"healthy"}`))
    }).Methods("GET")
    
    addr := ":8081"
    log.Printf("TigerSwap Admin API starting on %s", addr)
    log.Fatal(http.ListenAndServe(addr, router))
}

func LoggingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        log.Printf("%s %s", r.Method, r.URL.Path)
        next.ServeHTTP(w, r)
        log.Printf("%s %s - %v", r.Method, r.URL.Path, time.Since(start))
    })
}

func RecoveryMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.Printf("Panic: %v", err)
                http.Error(w, "Internal Server Error", http.StatusInternalServerError)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
