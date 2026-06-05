// TigerSwap Admin API - Go Implementation
// High-performance REST API for platform administration
// Manages DEXs, CEXs, HD Wallets, and all platform operations

package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// ============================================================================
// Constants
// ============================================================================

const (
	MaxRequestBodySize = 10 * 1024 * 1024 // 10MB
	ReadTimeout        = 30 * time.Second
	WriteTimeout       = 30 * time.Second
	IdleTimeout        = 120 * time.Second
)

// ============================================================================
// Enums
// ============================================================================

type AdminRole string

const (
	RoleSuperAdmin AdminRole = "super_admin"
	RoleDEXAdmin  AdminRole = "dex_admin"
	RoleCEXAdmin  AdminRole = "cex_admin"
	RoleFinance   AdminRole = "finance_admin"
)

type AdminAction string

const (
	ActionDEXCreate     AdminAction = "dex:create"
	ActionDEXModify     AdminAction = "dex:modify"
	ActionDEXSuspend    AdminAction = "dex:suspend"
	ActionCEXConnect    AdminAction = "cex:connect"
	ActionCEXTrade      AdminAction = "cex:trade"
	ActionWalletTransfer AdminAction = "wallet:transfer"
	ActionFinanceView   AdminAction = "finance:view"
)

// ============================================================================
// Models
// ============================================================================

type Admin struct {
	ID           string      `json:"id"`
	Username     string      `json:"username"`
	Email        string      `json:"email"`
	PasswordHash string      `json:"-"`
	Role         AdminRole   `json:"role"`
	Permissions  []string    `json:"permissions"`
	IsActive    bool        `json:"is_active"`
	LastLogin   int64       `json:"last_login"`
	CreatedAt   int64       `json:"created_at"`
}

type ConnectedDEX struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Slug        string   `json:"slug"`
	ChainID     uint64   `json:"chain_id"`
	Status      string   `json:"status"` // active, suspended, maintenance
	
	// Connection
	APIEndpoint    string `json:"api_endpoint"`
	SubgraphURL    string `json:"subgraph_url"`
	RouterAddress  string `json:"router_address"`
	FactoryAddress string `json:"factory_address"`
	
	// Stats
	Volume24h   float64 `json:"volume_24h"`
	Volume7d    float64 `json:"volume_7d"`
	TotalTrades int64   `json:"total_trades"`
	AvgLatency  float64 `json:"avg_latency_ms"`
	
	// Fees
	TradingFee       float64 `json:"trading_fee"`
	PlatformFeeShare float64 `json:"platform_fee_share"`
	
	// Status
	LastHealthCheck int64    `json:"last_health_check"`
	HealthStatus    string   `json:"health_status"`
	Errors          []string `json:"errors"`
	
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

type ConnectedCEX struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	ExchangeType string   `json:"exchange_type"` // binance, coinbase, kraken
	
	Status string `json:"status"` // connected, disconnected, error
	
	// Account
	AccountID   string   `json:"account_id"`
	AccountType string   `json:"account_type"` // spot, margin, futures
	SubAccounts []string `json:"sub_accounts"`
	
	// Permissions
	CanTrade    bool    `json:"can_trade"`
	CanWithdraw bool    `json:"can_withdraw"`
	CanDeposit  bool    `json:"can_deposit"`
	
	// Balances
	TotalBalanceUSD    float64 `json:"total_balance_usd"`
	AvailableBalanceUSD float64 `json:"available_balance_usd"`
	LockedBalanceUSD   float64 `json:"locked_balance_usd"`
	
	// Rate Limits
	RequestsPerSecond int `json:"requests_per_second"`
	RequestsPerMinute int `json:"requests_per_minute"`
	
	// Status
	LastSync   int64    `json:"last_sync"`
	LastTrade  int64    `json:"last_trade"`
	HealthStatus string `json:"health_status"`
	
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

type HDWallet struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	WalletType   string   `json:"wallet_type"` // master, derived, imported
	
	DerivationPath string `json:"derivation_path"`
	
	// Security
	SecurityLevel     string   `json:"security_level"` // standard, hardware, multisig
	RequiresSignatures int     `json:"requires_signatures"`
	AllowedOperations []string `json:"allowed_operations"`
	OperationLimits   map[string]float64 `json:"operation_limits"`
	
	// Status
	TotalBalanceUSD float64 `json:"total_balance_usd"`
	IsActive        bool     `json:"is_active"`
	
	CreatedAt int64 `json:"created_at"`
	UpdatedAt int64 `json:"updated_at"`
}

type AuditLog struct {
	ID           string            `json:"id"`
	AdminID      string            `json:"admin_id"`
	Action       string            `json:"action"`
	ResourceType string            `json:"resource_type"`
	ResourceID   string            `json:"resource_id"`
	Details      map[string]interface{} `json:"details"`
	IPAddress    string            `json:"ip_address"`
	Timestamp    int64             `json:"timestamp"`
}

type FeeStructure struct {
	ID        string  `json:"id"`
	FeeType   string  `json:"fee_type"` // trading, withdrawal, deposit, bridge
	Asset     string  `json:"asset"`
	ChainID   uint64  `json:"chain_id,omitempty"`
	
	MakerFee float64 `json:"maker_fee"`
	TakerFee float64 `json:"taker_fee"`
	FlatFee  float64 `json:"flat_fee"`
	
	IsActive   bool   `json:"is_active"`
	UpdatedBy  string `json:"updated_by"`
	UpdatedAt  int64  `json:"updated_at"`
}

type PlatformStats struct {
	TotalVolume24h float64 `json:"total_volume_24h"`
	TotalUsers     int64   `json:"total_users"`
	TotalTrades24h int64   `json:"total_trades_24h"`
	TotalFees24h   float64 `json:"total_fees_collected_24h"`
	DEXCount       int     `json:"dex_count"`
	CEXCount       int     `json:"cex_count"`
	WalletCount    int     `json:"wallet_count"`
}

// ============================================================================
// Admin Service
// ============================================================================

type AdminService struct {
	mu sync.RWMutex
	
	admins      map[string]*Admin
	dexes       map[string]*ConnectedDEX
	cexes       map[string]*ConnectedCEX
	wallets     map[string]*HDWallet
	feeStructs  map[string]*FeeStructure
	auditLogs   []*AuditLog
	
	stats       PlatformStats
}

func NewAdminService() *AdminService {
	svc := &AdminService{
		admins:     make(map[string]*Admin),
		dexes:      make(map[string]*ConnectedDEX),
		cexes:      make(map[string]*ConnectedCEX),
		wallets:    make(map[string]*HDWallet),
		feeStructs: make(map[string]*FeeStructure),
		auditLogs:  make([]*AuditLog, 0),
	}
	
	// Initialize default admin
	svc.admins["admin_001"] = &Admin{
		ID:        "admin_001",
		Username:  "tigerswap_admin",
		Email:     "admin@tigerswap.io",
		Role:      RoleSuperAdmin,
		IsActive:  true,
		CreatedAt:  time.Now().Unix(),
	}
	
	// Initialize default DEXes
	svc.initializeDefaultDEXes()
	
	return svc
}

func (s *AdminService) initializeDefaultDEXes() {
	dexes := []*ConnectedDEX{
		{
			ID:          "dex_uniswap_v2",
			Name:        "Uniswap V2",
			Slug:        "uniswap-v2",
			ChainID:     1,
			Status:      "active",
			APIEndpoint: "https://api.thegraph.com/subgraphs/name/uniswap/uniswap-v2",
			Volume24h:   150_000_000,
			Volume7d:    1_000_000_000,
			TotalTrades: 500_000,
			AvgLatency:  45,
			TradingFee:  0.003,
			CreatedAt:   time.Now().Unix(),
			UpdatedAt:   time.Now().Unix(),
		},
		{
			ID:          "dex_uniswap_v3",
			Name:        "Uniswap V3",
			Slug:        "uniswap-v3",
			ChainID:     1,
			Status:      "active",
			APIEndpoint: "https://api.thegraph.com/subgraphs/name/uniswap/uniswap-v3",
			Volume24h:   250_000_000,
			Volume7d:    1_800_000_000,
			TotalTrades: 800_000,
			AvgLatency:  55,
			TradingFee:  0.003,
			CreatedAt:   time.Now().Unix(),
			UpdatedAt:   time.Now().Unix(),
		},
		{
			ID:          "dex_sushiswap",
			Name:        "SushiSwap",
			Slug:        "sushiswap",
			ChainID:     1,
			Status:      "active",
			APIEndpoint: "https://api.thegraph.com/subgraphs/name/sushiswap/exchange",
			Volume24h:   50_000_000,
			Volume7d:    350_000_000,
			TotalTrades: 200_000,
			AvgLatency:  60,
			TradingFee:  0.003,
			CreatedAt:   time.Now().Unix(),
			UpdatedAt:   time.Now().Unix(),
		},
		{
			ID:          "dex_pancakeswap",
			Name:        "PancakeSwap",
			Slug:        "pancakeswap",
			ChainID:     56,
			Status:      "active",
			APIEndpoint: "https://bsc.streamingfast.io/subgraphs/name/pancakeswap/exchange-v2",
			Volume24h:   80_000_000,
			Volume7d:    550_000_000,
			TotalTrades: 300_000,
			AvgLatency:  40,
			TradingFee:  0.0025,
			CreatedAt:   time.Now().Unix(),
			UpdatedAt:   time.Now().Unix(),
		},
	}
	
	for _, dex := range dexes {
		s.dexes[dex.ID] = dex
	}
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func (s *AdminService) AuthenticateAdmin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		MFAcode  string `json:"mfa_code,omitempty"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Find admin by email
	var admin *Admin
	for _, a := range s.admins {
		if a.Email == req.Email && a.IsActive {
			admin = a
			break
		}
	}
	
	if admin == nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}
	
	// Generate session token
	token := generateToken(admin.ID)
	
	// Update last login
	admin.LastLogin = time.Now().Unix()
	
	s.logAction(admin.ID, "admin:login", "admin", admin.ID, map[string]interface{}{"event": "login"})
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"token":   token,
		"admin": map[string]interface{}{
			"id":       admin.ID,
			"username": admin.Username,
			"email":    admin.Email,
			"role":     admin.Role,
		},
	})
}

func (s *AdminService) ListDEXes(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	dexes := make([]*ConnectedDEX, 0, len(s.dexes))
	for _, dex := range s.dexes {
		dexes = append(dexes, dex)
	}
	
	// Sort by volume
	sort.Slice(dexes, func(i, j int) bool {
		return dexes[i].Volume24h > dexes[j].Volume24h
	})
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
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
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
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
	
	// Apply updates
	if status, ok := updates["status"].(string); ok {
		dex.Status = status
	}
	if tradingFee, ok := updates["trading_fee"].(float64); ok {
		dex.TradingFee = tradingFee
	}
	if feeShare, ok := updates["platform_fee_share"].(float64); ok {
		dex.PlatformFeeShare = feeShare
	}
	
	dex.UpdatedAt = time.Now().Unix()
	
	s.logAction("admin_001", "dex:modify", "dex", dexID, updates)
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    dex,
	})
}

func (s *AdminService) SuspendDEX(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dexID := vars["dex_id"]
	
	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
	
	dex.Status = "suspended"
	dex.UpdatedAt = time.Now().Unix()
	
	s.logAction("admin_001", "dex:suspend", "dex", dexID, map[string]interface{}{
		"reason": req.Reason,
	})
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
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
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    cexes,
		"count":   len(cexes),
	})
}

func (s *AdminService) ConnectCEX(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name         string `json:"name"`
		ExchangeType string `json:"exchange_type"`
		APIKey       string `json:"api_key"`
		APISecret    string `json:"api_secret"`
		AccountID    string `json:"account_id"`
		Permissions  struct {
			CanTrade    bool `json:"can_trade"`
			CanWithdraw bool `json:"can_withdraw"`
			CanDeposit  bool `json:"can_deposit"`
		} `json:"permissions"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	cex := &ConnectedCEX{
		ID:           fmt.Sprintf("cex_%s_%d", req.ExchangeType, time.Now().Unix()),
		Name:         req.Name,
		ExchangeType: req.ExchangeType,
		Status:       "connected",
		AccountID:    req.AccountID,
		AccountType:  "spot",
		CanTrade:     req.Permissions.CanTrade,
		CanWithdraw:  req.Permissions.CanWithdraw,
		CanDeposit:   req.Permissions.CanDeposit,
		CreatedAt:    time.Now().Unix(),
		UpdatedAt:    time.Now().Unix(),
	}
	
	s.mu.Lock()
	s.cexes[cex.ID] = cex
	s.mu.Unlock()
	
	s.logAction("admin_001", "cex:connect", "cex", cex.ID, map[string]interface{}{
		"name":          req.Name,
		"exchange_type": req.ExchangeType,
	})
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
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
		TotalUsers:     10000, // Would come from user service
		TotalTrades24h: 50000,
		TotalFees24h:   totalVolume * 0.003 * 0.1, // 10% of trading fees
		DEXCount:       len(s.dexes),
		CEXCount:       len(s.cexes),
		WalletCount:    len(s.wallets),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    stats,
	})
}

func (s *AdminService) GetAuditLogs(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	// Return last 100 logs
	start := len(s.auditLogs) - 100
	if start < 0 {
		start = 0
	}
	logs := s.auditLogs[start:]
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    logs,
		"count":   len(logs),
	})
}

// ============================================================================
// Helper Methods
// ============================================================================

func (s *AdminService) logAction(adminID, action, resourceType, resourceID string, details map[string]interface{}) {
	log := &AuditLog{
		ID:           fmt.Sprintf("log_%d", time.Now().UnixNano()),
		AdminID:      adminID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Details:      details,
		Timestamp:    time.Now().Unix(),
	}
	
	s.auditLogs = append(s.auditLogs, log)
	
	// Keep only last 10000 logs
	if len(s.auditLogs) > 10000 {
		s.auditLogs = s.auditLogs[len(s.auditLogs)-10000:]
	}
}

func generateToken(adminID string) string {
	data := fmt.Sprintf("%s:%d:%s", adminID, time.Now().Unix(), "secret")
	h := hmac.New(sha256.New, []byte("tigerswap-secret-key"))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))[:32]
}

// ============================================================================
// Main
// ============================================================================

func main() {
	svc := NewAdminService()
	
	router := mux.NewRouter()
	router.Use(LoggingMiddleware)
	router.Use(RecoveryMiddleware)
	
	// Auth routes
	router.HandleFunc("/api/admin/login", svc.AuthenticateAdmin).Methods("POST")
	
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
	
	// Health
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"healthy"}`))
	}).Methods("GET")
	
	addr := ":8081"
	log.Printf("TigerSwap Admin API starting on %s", addr)
	
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  ReadTimeout,
		WriteTimeout: WriteTimeout,
		IdleTimeout:  IdleTimeout,
	}
	
	log.Fatal(server.ListenAndServe())
}

// Middleware

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
