package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// API Response types
type SwapQuote struct {
	InputToken  string  `json:"inputToken"`
	OutputToken string  `json:"outputToken"`
	InputAmount string  `json:"inputAmount"`
	OutputAmount string `json:"outputAmount"`
	PriceImpact float64 `json:"priceImpact"`
	GasEstimate string  `json:"gasEstimate"`
	Route       []Route `json:"route"`
}

type Route struct {
	Protocol  string   `json:"protocol"`
	Path      []string `json:"path"`
	Pool      string   `json:"pool"`
	Percent   int      `json:"percent"`
}

type Token struct {
	Symbol    string `json:"symbol"`
	Address   string `json:"address"`
	Chain     string `json:"chain"`
	Decimals  int    `json:"decimals"`
	LogoURI   string `json:"logoURI"`
	Name      string `json:"name"`
	PriceUSD  string `json:"priceUSD"`
}

type Chain struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	RPC      string `json:"rpc"`
	Explorer string `json:"explorer"`
	Native   string `json:"nativeToken"`
	Wrapped  string `json:"wrappedToken"`
}

type SwapRequest struct {
	FromToken string   `json:"fromToken"`
	ToToken   string   `json:"toToken"`
	Amount    string   `json:"amount"`
	Slippage  float64  `json:"slippage"`
	GasPrice  string   `json:"gasPrice"`
	Routes    []Route  `json:"routes"`
	Referrer  string   `json:"referrer"`
}

type BridgeRequest struct {
	FromChain   string `json:"fromChain"`
	ToChain     string `json:"toChain"`
	Token       string `json:"token"`
	Amount      string `json:"amount"`
	DestAddress string `json:"destAddress"`
}

type GasEstimate struct {
	Slow   string `json:"slow"`
	Standard string `json:"standard"`
	Fast    string `json:"fast"`
}

type QuoteResponse struct {
	Success   bool       `json:"success"`
	Data      SwapQuote  `json:"data"`
	Error     string     `json:"error,omitempty"`
	Timestamp time.Time  `json:"timestamp"`
}

type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
	Version   string    `json:"version"`
	Uptime    int64     `json:"uptime"`
}

// Handlers
func healthHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "1.0.0",
		Uptime:    time.Now().Unix() - startTime,
	})
}

func getQuoteHandler(w http.ResponseWriter, r *http.Request) {
	var req SwapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Mock quote calculation
	quote := SwapQuote{
		InputToken:   req.FromToken,
		OutputToken:  req.ToToken,
		InputAmount:  req.Amount,
		OutputAmount: calculateOutputAmount(req.Amount),
		PriceImpact: 0.5,
		GasEstimate: "150000",
		Route: []Route{
			{Protocol: "uniswap", Path: []string{req.FromToken, req.ToToken}, Pool: "0x...", Percent: 100},
		},
	}

	json.NewEncoder(w).Encode(QuoteResponse{
		Success:   true,
		Data:      quote,
		Timestamp: time.Now(),
	})
}

func executeSwapHandler(w http.ResponseWriter, r *http.Request) {
	var req SwapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Mock swap execution
	txHash := fmt.Sprintf("0x%x", time.Now().UnixNano())
	
	response := map[string]interface{}{
		"success":  true,
		"txHash":   txHash,
		"message":  "Swap submitted successfully",
		"timestamp": time.Now(),
	}
	
	json.NewEncoder(w).Encode(response)
}

func getTokensHandler(w http.ResponseWriter, r *http.Request) {
	// Return mock tokens
	tokens := []Token{
		{Symbol: "ETH", Address: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", Chain: "ethereum", Decimals: 18, Name: "Ethereum"},
		{Symbol: "USDT", Address: "0xdAC17F958D2ee523a2206206994597C13D831ec7", Chain: "ethereum", Decimals: 6, Name: "Tether USD"},
		{Symbol: "USDC", Address: "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", Chain: "ethereum", Decimals: 6, Name: "USD Coin"},
		{Symbol: "BNB", Address: "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c", Chain: "bsc", Decimals: 18, Name: "BNB"},
		{Symbol: "MATIC", Address: "0x7D1AfA7B7fb4105dc500DB53d06Eb2F7E3eCa44c", Chain: "polygon", Decimals: 18, Name: "Polygon"},
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    tokens,
	})
}

func getChainsHandler(w http.ResponseWriter, r *http.Request) {
	chains := []Chain{
		{ID: 1, Name: "ethereum", RPC: "https://eth.llamarpc.com", Explorer: "https://etherscan.io", Native: "ETH", Wrapped: "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2"},
		{ID: 56, Name: "bsc", RPC: "https://bsc.llamarpc.com", Explorer: "https://bscscan.com", Native: "BNB", Wrapped: "0xbb4CdB9CBd36B01bD1cBaEBF2De08d9173bc095c"},
		{ID: 137, Name: "polygon", RPC: "https://polygon.llamarpc.com", Explorer: "https://polygonscan.com", Native: "MATIC", Wrapped: "0x0d500B1d8E8eF31E21C99d1Db9A6444d3ADf1270"},
		{ID: 42161, Name: "arbitrum", RPC: "https://arbitrum.llamarpc.com", Explorer: "https://arbiscan.io", Native: "ETH", Wrapped: "0x82aF49447D8a07e3bd95BD0d56f35241523fBab1"},
		{ID: 10, Name: "optimism", RPC: "https://optimism.llamarpc.com", Explorer: "https://optimistic.etherscan.io", Native: "ETH", Wrapped: "0x4200000000000000000000000000000000000042"},
		{ID: 43114, Name: "avalanche", RPC: "https://avax.llamarpc.com", Explorer: "https://snowtrace.io", Native: "AVAX", Wrapped: "0xB31f66AA3C1e78502F98da20086eDCD3Fd1D0b8C"},
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    chains,
	})
}

func bridgeQuoteHandler(w http.ResponseWriter, r *http.Request) {
	var req BridgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	response := map[string]interface{}{
		"success":      true,
		"inputAmount":  req.Amount,
		"outputAmount": req.Amount, // 1:1 for native bridging
		"bridgeFee":    "0.01",
		"estimatedTime": "10 minutes",
		"route": []map[string]string{
			{"protocol": "tigerbridge", "fromChain": req.FromChain, "toChain": req.ToChain},
		},
	}
	
	json.NewEncoder(w).Encode(response)
}

func executeBridgeHandler(w http.ResponseWriter, r *http.Request) {
	var req BridgeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	txHash := fmt.Sprintf("0x%x", time.Now().UnixNano())
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":     true,
		"txHash":      txHash,
		"bridgeId":    fmt.Sprintf("bridge_%d", time.Now().Unix()),
		"message":     "Bridge initiated",
		"timestamp":   time.Now(),
	})
}

func gasEstimateHandler(w http.ResponseWriter, r *http.Request) {
	gas := GasEstimate{
		Slow:     "20",
		Standard: "35",
		Fast:     "50",
	}
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"data":    gas,
	})
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade failed:", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Send initial connection confirmation
	conn.WriteJSON(map[string]interface{}{
		"type":    "connected",
		"message": "TigerSwap WebSocket connected",
	})

	// Simulate price updates
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			priceUpdate := map[string]interface{}{
				"type": "price_update",
				"data": map[string]string{
					"ETH":  "2450.50",
					"USDT": "1.00",
					"USDC": "1.00",
					"BNB":  "310.25",
				},
				"timestamp": time.Now(),
			}
			conn.WriteJSON(priceUpdate)
		}
	}
}

// Helper functions
func calculateOutputAmount(input string) string {
	return fmt.Sprintf("%.6f", 0.85)
}

var startTime int64

func main() {
	startTime = time.Now().Unix()
	
	router := mux.NewRouter()
	
	// Health
	router.HandleFunc("/health", healthHandler).Methods("GET")
	
	// Quote routes
	router.HandleFunc("/api/v1/quote", getQuoteHandler).Methods("POST")
	router.HandleFunc("/api/v1/swap", executeSwapHandler).Methods("POST")
	router.HandleFunc("/api/v1/bridge/quote", bridgeQuoteHandler).Methods("POST")
	router.HandleFunc("/api/v1/bridge/execute", executeBridgeHandler).Methods("POST")
	router.HandleFunc("/api/v1/gas", gasEstimateHandler).Methods("GET")
	
	// Token and chain info
	router.HandleFunc("/api/v1/tokens", getTokensHandler).Methods("GET")
	router.HandleFunc("/api/v1/chains", getChainsHandler).Methods("GET")
	
	// WebSocket
	router.HandleFunc("/ws", wsHandler)
	
	// CORS middleware
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	
	fmt.Println("TigerSwap API Gateway starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}