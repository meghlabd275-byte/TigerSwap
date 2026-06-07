package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ============================================================================
// TYPES
// ============================================================================

type ExternalPlatform struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Type           string    `json:"type"` // cex, dex, wallet, protocol
	APIKey         string    `json:"apiKey"`
	Tier           string    `json:"tier"` // free, basic, pro, enterprise
	IsActive       bool      `json:"isActive"`
	Permissions    Permissions `json:"permissions"`
	RateLimitPerMin int       `json:"rateLimitPerMin"`
	MonthlyFeeUsd   float64   `json:"monthlyFeeUsd"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Permissions struct {
	CanTrade       bool `json:"canTrade"`
	CanSwap        bool `json:"canSwap"`
	CanAddLiquidity bool `json:"canAddLiquidity"`
	CanBridge      bool `json:"canBridge"`
	CanCreateToken bool `json:"canCreateToken"`
}

type TierConfig struct {
	Name              string      `json:"name"`
	MonthlyFeeUsd    float64     `json:"monthlyFeeUsd"`
	MaxAPICallsPerMin int         `json:"maxApiCallsPerMin"`
	MaxDailyVolume    float64     `json:"maxDailyVolume"`
	MaxPositions     int         `json:"maxPositions"`
	Features         Permissions `json:"features"`
}

type TradingRequest struct {
	Platform string `json:"platform"`
	Symbol   string `json:"symbol"`
	Side     string `json:"side"` // buy, sell
	Type     string `json:"type"` // market, limit
	Amount   string `json:"amount"`
	Price    string `json:"price,omitempty"`
}

type SwapRequest struct {
	Platform string `json:"platform"`
	ChainID   int    `json:"chainId"`
	TokenIn  string `json:"tokenIn"`
	TokenOut string `json:"tokenOut"`
	AmountIn string `json:"amountIn"`
	Slippage float64 `json:"slippage"`
}

type LiquidityRequest struct {
	Platform string `json:"platform"`
	ChainID  int    `json:"chainId"`
	TokenA   string `json:"tokenA"`
	TokenB   string `json:"tokenB"`
	AmountA  string `json:"amountA"`
	AmountB  string `json:"amountB"`
}

type ApiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// ============================================================================
// TIER CONFIGURATIONS
// ============================================================================

var TierConfigs = map[string]TierConfig{
	"free": {
		Name:            "free",
		MonthlyFeeUsd:   0,
		MaxAPICallsPerMin: 60,
		MaxDailyVolume:   10000,
		MaxPositions:    3,
		Features: Permissions{
			CanTrade:        true,
			CanSwap:         false,
			CanAddLiquidity: false,
			CanBridge:      false,
			CanCreateToken: false,
		},
	},
	"basic": {
		Name:            "basic",
		MonthlyFeeUsd:   99,
		MaxAPICallsPerMin: 300,
		MaxDailyVolume:   100000,
		MaxPositions:    10,
		Features: Permissions{
			CanTrade:        true,
			CanSwap:        true,
			CanAddLiquidity: false,
			CanBridge:      false,
			CanCreateToken: false,
		},
	},
	"pro": {
		Name:            "pro",
		MonthlyFeeUsd:   299,
		MaxAPICallsPerMin: 1000,
		MaxDailyVolume:   1000000,
		MaxPositions:    50,
		Features: Permissions{
			CanTrade:        true,
			CanSwap:        true,
			CanAddLiquidity: true,
			CanBridge:     true,
			CanCreateToken: false,
		},
	},
	"enterprise": {
		Name:            "enterprise",
		MonthlyFeeUsd:   999,
		MaxAPICallsPerMin: 5000,
		MaxDailyVolume:   10000000,
		MaxPositions:    200,
		Features: Permissions{
			CanTrade:        true,
			CanSwap:        true,
			CanAddLiquidity: true,
			CanBridge:      true,
			CanCreateToken: true,
		},
	},
}

// ============================================================================
// STORAGE (in production, use database)
// ============================================================================

var platforms = make(map[string]ExternalPlatform)
var apiCalls = make(map[string]int)
var dailyVolume = make(map[string]float64)

// ============================================================================
// HANDLERS
// ============================================================================

// Get tier configurations
func GetTierConfigs(w http.ResponseWriter, r *http.Request) {
	tiers := []TierConfig{}
	for _, tier := range TierConfigs {
		tiers = append(tiers, tier)
	}
	respondJSON(w, ApiResponse{Success: true, Data: tiers})
}

// Register external platform
func RegisterPlatform(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		APIKey   string `json:"apiKey"`
		Tier     string `json:"tier"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, ApiResponse{Success: false, Error: "Invalid request"})
		return
	}

	if req.Name == "" || req.Type == "" || req.APIKey == "" {
		respondJSON(w, ApiResponse{Success: false, Error: "Missing required fields"})
		return
	}

	// Validate tier
	tierConfig, ok := TierConfigs[req.Tier]
	if !ok {
		req.Tier = "free"
		tierConfig = TierConfigs["free"]
	}

	platform := ExternalPlatform{
		ID:           fmt.Sprintf("platform_%d", time.Now().Unix()),
		Name:         req.Name,
		Type:         req.Type,
		APIKey:       req.APIKey,
		Tier:         req.Tier,
		IsActive:     true,
		Permissions: tierConfig.Features,
		RateLimitPerMin: tierConfig.MaxAPICallsPerMin,
		MonthlyFeeUsd: tierConfig.MonthlyFeeUsd,
		CreatedAt:   time.Now(),
	}

	platforms[platform.ID] = platform

	respondJSON(w, ApiResponse{
		Success: true,
		Data:    platform,
		Message: "Platform registered successfully",
	})
}

// Get platform info
func GetPlatform(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/external-platform/")
	
	platform, ok := platforms[id]
	if !ok {
		respondJSON(w, ApiResponse{Success: false, Error: "Platform not found"})
		return
	}

	respondJSON(w, ApiResponse{Success: true, Data: platform})
}

// Update platform
func UpdatePlatform(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name     string `json:"name,omitempty"`
		Tier    string `json:"tier,omitempty"`
		IsActive bool  `json:"isActive,omitempty"`
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/external-platform/update/")
	
	platform, ok := platforms[id]
	if !ok {
		respondJSON(w, ApiResponse{Success: false, Error: "Platform not found"})
		return
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, ApiResponse{Success: false, Error: "Invalid request"})
		return
	}

	if req.Name != "" {
		platform.Name = req.Name
	}
	if req.Tier != "" {
		if tierConfig, ok := TierConfigs[req.Tier]; ok {
			platform.Tier = req.Tier
			platform.Permissions = tierConfig.Features
			platform.RateLimitPerMin = tierConfig.MaxAPICallsPerMin
			platform.MonthlyFeeUsd = tierConfig.MonthlyFeeUsd
		}
	}
	if req.IsActive {
		platform.IsActive = req.IsActive
	}

	platforms[id] = platform

	respondJSON(w, ApiResponse{Success: true, Data: platform})
}

// Delete platform
func DeletePlatform(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/external-platform/delete/")
	
	if _, ok := platforms[id]; !ok {
		respondJSON(w, ApiResponse{Success: false, Error: "Platform not found"})
		return
	}

	delete(platforms, id)

	respondJSON(w, ApiResponse{Success: true, Message: "Platform deleted"})
}

// Trading handler
func ExecuteTrade(w http.ResponseWriter, r *http.Request) {
	var req TradingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, ApiResponse{Success: false, Error: "Invalid request"})
		return
	}

	if req.Platform == "" || req.Symbol == "" || req.Side == "" || req.Amount == "" {
		respondJSON(w, ApiResponse{Success: false, Error: "Missing required fields"})
		return
	}

	platform, ok := platforms[req.Platform]
	if !ok {
		respondJSON(w, ApiResponse{Success: false, Error: "Platform not found"})
		return
	}

	if !platform.IsActive {
		respondJSON(w, ApiResponse{Success: false, Error: "Platform is not active"})
		return
	}

	if !platform.Permissions.CanTrade {
		respondJSON(w, ApiResponse{Success: false, Error: "Trading not permitted for this tier"})
		return
	}

	// Execute trade (mock)
	order := map[string]interface{}{
		"id":        fmt.Sprintf("order_%d", time.Now().Unix()),
		"platform":  req.Platform,
		"symbol":    req.Symbol,
		"side":      req.Side,
		"type":      req.Type,
		"amount":    req.Amount,
		"price":     req.Price,
		"status":    "filled",
		"timestamp": time.Now().Unix(),
	}

	respondJSON(w, ApiResponse{
		Success: true,
		Data:    order,
		Message: "Trade executed successfully",
	})
}

// Swap handler
func ExecuteSwap(w http.ResponseWriter, r *http.Request) {
	var req SwapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, ApiResponse{Success: false, Error: "Invalid request"})
		return
	}

	if req.Platform == "" || req.TokenIn == "" || req.TokenOut == "" || req.AmountIn == "" {
		respondJSON(w, ApiResponse{Success: false, Error: "Missing required fields"})
		return
	}

	platform, ok := platforms[req.Platform]
	if !ok {
		respondJSON(w, ApiResponse{Success: false, Error: "Platform not found"})
		return
	}

	if !platform.Permissions.CanSwap {
		respondJSON(w, ApiResponse{Success: false, Error: "Swap not permitted for this tier"})
		return
	}

	// Execute swap (mock)
	swap := map[string]interface{}{
		"id":         fmt.Sprintf("swap_%d", time.Now().Unix()),
		"platform":   req.Platform,
		"chainId":    req.ChainID,
		"tokenIn":   req.TokenIn,
		"tokenOut":  req.TokenOut,
		"amountIn":  req.AmountIn,
		"slippage":  req.Slippage,
		"status":    "completed",
		"timestamp": time.Now().Unix(),
	}

	respondJSON(w, ApiResponse{
		Success: true,
		Data:    swap,
		Message: "Swap executed successfully",
	})
}

// Add liquidity handler
func AddLiquidity(w http.ResponseWriter, r *http.Request) {
	var req LiquidityRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondJSON(w, ApiResponse{Success: false, Error: "Invalid request"})
		return
	}

	if req.Platform == "" || req.TokenA == "" || req.TokenB == "" {
		respondJSON(w, ApiResponse{Success: false, Error: "Missing required fields"})
		return
	}

	platform, ok := platforms[req.Platform]
	if !ok {
		respondJSON(w, ApiResponse{Success: false, Error: "Platform not found"})
		return
	}

	if !platform.Permissions.CanAddLiquidity {
		respondJSON(w, ApiResponse{Success: false, Error: "Add liquidity not permitted for this tier"})
		return
	}

	// Add liquidity (mock)
	liquidity := map[string]interface{}{
		"id":         fmt.Sprintf("liq_%d", time.Now().Unix()),
		"platform":  req.Platform,
		"chainId":   req.ChainID,
		"tokenA":    req.TokenA,
		"tokenB":    req.TokenB,
		"amountA":   req.AmountA,
		"amountB":   req.AmountB,
		"status":    "added",
		"timestamp": time.Now().Unix(),
	}

	respondJSON(w, ApiResponse{
		Success: true,
		Data:    liquidity,
		Message: "Liquidity added successfully",
	})
}

// Get rate limit status
func GetRateLimit(w http.ResponseWriter, r *http.Request) {
	platformID := r.URL.Query().Get("platform")
	
	if platformID == "" {
		respondJSON(w, ApiResponse{Success: false, Error: "Missing platform ID"})
		return
	}

	platform, ok := platforms[platformID]
	if !ok {
		respondJSON(w, ApiResponse{Success: false, Error: "Platform not found"})
		return
	}

	used := apiCalls[platformID]
	remaining := platform.RateLimitPerMin - used

	respondJSON(w, ApiResponse{
		Success: true,
		Data: map[string]interface{}{
			"platform":   platformID,
			"used":       used,
			"limit":      platform.RateLimitPerMin,
			"remaining":  remaining,
			"resetAt":    time.Now().Add(time.Minute).Unix(),
		},
	})
}

// Get platform stats
func GetPlatformStats(w http.ResponseWriter, r *http.Request) {
	stats := map[string]interface{}{
		"totalPlatforms":  len(platforms),
		"activePlatforms": countActivePlatforms(),
		"byTier":        getPlatformsByTier(),
	}

	respondJSON(w, ApiResponse{Success: true, Data: stats})
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func respondJSON(w http.ResponseWriter, resp ApiResponse) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func countActivePlatforms() int {
	count := 0
	for _, p := range platforms {
		if p.IsActive {
			count++
		}
	}
	return count
}

func getPlatformsByTier() map[string]int {
	byTier := make(map[string]int)
	for _, p := range platforms {
		byTier[p.Tier]++
	}
	return byTier
}

// ============================================================================
// ROUTE REGISTRATION
// ============================================================================

func RegisterExternalPlatformRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/external-platform/tiers", GetTierConfigs)
	mux.HandleFunc("/api/external-platform/register", RegisterPlatform)
	mux.HandleFunc("/api/external-platform/", GetPlatform)
	mux.HandleFunc("/api/external-platform/update/", UpdatePlatform)
	mux.HandleFunc("/api/external-platform/delete/", DeletePlatform)
	mux.HandleFunc("/api/external-platform/trade", ExecuteTrade)
	mux.HandleFunc("/api/external-platform/swap", ExecuteSwap)
	mux.HandleFunc("/api/external-platform/liquidity", AddLiquidity)
	mux.HandleFunc("/api/external-platform/rate-limit", GetRateLimit)
	mux.HandleFunc("/api/external-platform/stats", GetPlatformStats)
}