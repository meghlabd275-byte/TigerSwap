package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestHealthCheck tests the health check endpoint
func TestHealthCheck(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", HealthCheck())

	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response["status"] != "healthy" {
		t.Errorf("Expected status healthy, got %v", response["status"])
	}
}

// TestGetQuote tests the quote endpoint
func TestGetQuote(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := NewSwapHandler(nil)

	r.POST("/api/v1/swap/quote", handler.GetQuote)

	body := `{
		"chain_id": 1,
		"from_token": "0x0000000000000000000000000000000000000000",
		"to_token": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		"amount": "1000000000000000000"
	}`

	req, _ := http.NewRequest("POST", "/api/v1/swap/quote", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestGetTokens tests the tokens endpoint
func TestGetTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := NewTokenHandler(nil)
	r.GET("/api/v1/tokens", handler.GetTokens)

	req, _ := http.NewRequest("GET", "/api/v1/tokens", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var tokens []Token
	if err := json.Unmarshal(w.Body.Bytes(), &tokens); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(tokens) == 0 {
		t.Error("Expected tokens, got empty list")
	}
}

// TestGetChains tests the chains endpoint
func TestGetChains(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := NewChainHandler(nil)
	r.GET("/api/v1/chains", handler.GetChains)

	req, _ := http.NewRequest("GET", "/api/v1/chains", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var chains []Chain
	if err := json.Unmarshal(w.Body.Bytes(), &chains); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(chains) == 0 {
		t.Error("Expected chains, got empty list")
	}
}

// TestCreateWallet tests wallet creation
func TestCreateWallet(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := NewWalletHandler(nil)
	r.POST("/api/v1/wallet/create", handler.CreateWallet)

	body := `{"chainId": 1}`
	req, _ := http.NewRequest("POST", "/api/v1/wallet/create", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestGetPools tests the pools endpoint
func TestGetPools(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := NewPoolHandler(nil)
	r.GET("/api/v1/pools", handler.GetPools)

	req, _ := http.NewRequest("GET", "/api/v1/pools", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestStakingPools tests staking pools endpoint
func TestStakingPools(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := NewStakingHandler(nil)
	r.GET("/api/v1/staking/pools", handler.GetPools)

	req, _ := http.NewRequest("GET", "/api/v1/staking/pools", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var pools []StakingPool
	if err := json.Unmarshal(w.Body.Bytes(), &pools); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(pools) == 0 {
		t.Error("Expected pools, got empty list")
	}
}

// TestBridgeChains tests bridge chains endpoint
func TestBridgeChains(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := NewBridgeHandler(nil)
	r.GET("/api/v1/bridge/chains", handler.GetSupportedChains)

	req, _ := http.NewRequest("GET", "/api/v1/bridge/chains", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// TestFarmingPools tests farming pools endpoint
func TestFarmingPools(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := NewFarmingHandler(nil)
	r.GET("/api/v1/farming/pools", handler.GetPools)

	req, _ := http.NewRequest("GET", "/api/v1/farming/pools", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

// BenchmarkHealthCheck benchmarks the health check endpoint
func BenchmarkHealthCheck(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", HealthCheck())

	req, _ := http.NewRequest("GET", "/health", nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}

// BenchmarkGetQuote benchmarks the quote endpoint
func BenchmarkGetQuote(b *testing.B) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	handler := NewSwapHandler(nil)
	r.POST("/api/v1/swap/quote", handler.GetQuote)

	body := `{
		"chain_id": 1,
		"from_token": "0x0000000000000000000000000000000000000000",
		"to_token": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48",
		"amount": "1000000000000000000"
	}`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("POST", "/api/v1/swap/quote", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
