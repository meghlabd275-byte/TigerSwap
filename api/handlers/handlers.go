package handlers

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strconv"

	"TigerSwap/api/middleware"
	"TigerSwap/api/models"
	"TigerSwap/services/trading_engine"
)

// OrderHandler handles order-related requests
type OrderHandler struct {
	engine *trading_engine.TradingEngine
}

// NewOrderHandler creates a new order handler
func NewOrderHandler(engine *trading_engine.TradingEngine) *OrderHandler {
	return &OrderHandler{
		engine: engine,
	}
}

// CreateOrder handles order creation
func (h *OrderHandler) CreateOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate
	if req.TokenIn == "" || req.TokenOut == "" {
		http.Error(w, "Missing token addresses", http.StatusBadRequest)
		return
	}

	if req.AmountIn.Cmp(big.NewInt(0)) <= 0 {
		http.Error(w, "Invalid amount", http.StatusBadRequest)
		return
	}

	// Create order
	order := &models.Order{
		TokenIn:      req.TokenIn,
		TokenOut:     req.TokenOut,
		AmountIn:     req.AmountIn,
		AmountOutMin: req.AmountOutMin,
		Price:       req.Price,
		OrderType:   req.OrderType,
		ExpiresAt:  req.ExpiresAt,
		IsNative:   req.IsNative,
	}

	if err := h.engine.CreateOrder(order); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create order: %v", err), http.StatusInternalServerError)
		return
	}

	// Respond
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.CreateOrderResponse{
		OrderID: order.ID,
		Status:  "pending",
	})
}

// GetOrder handles order retrieval
func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orderID, err := strconv.ParseUint(r.URL.Query().Get("orderId"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	order, ok := h.engine.GetOrder(orderID)
	if !ok {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

// CancelOrder handles order cancellation
func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.CancelOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if err := h.engine.CancelOrder(req.OrderID); err != nil {
		http.Error(w, fmt.Sprintf("Failed to cancel: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.CancelOrderResponse{
		OrderID: req.OrderID,
		Status:  "cancelled",
	})
}

// GetOrders handles getting all orders
func (h *OrderHandler) GetOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	orders := h.engine.GetOrders()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.GetOrdersResponse{
		Orders: orders,
		Count: len(orders),
	})
}

// GetOrderBook handles order book queries
func (h *OrderHandler) GetOrderBook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenIn := r.URL.Query().Get("tokenIn")
	tokenOut := r.URL.Query().Get("tokenOut")

	if tokenIn == "" || tokenOut == "" {
		http.Error(w, "Missing token addresses", http.StatusBadRequest)
		return
	}

	orders := h.engine.GetPendingOrders(tokenIn, tokenOut)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.GetOrderBookResponse{
		Orders: orders,
		Count: len(orders),
	})
}

// GetBestPrice handles best price queries
func (h *OrderHandler) GetBestPrice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenIn := r.URL.Query().Get("tokenIn")
	tokenOut := r.URL.Query().Get("tokenOut")
	side := r.URL.Query().Get("side") // "buy" or "sell"

	if tokenIn == "" || tokenOut == "" {
		http.Error(w, "Missing token addresses", http.StatusBadRequest)
		return
	}

	isBuy := side == "buy"
	price := h.engine.GetBestPrice(tokenIn, tokenOut, isBuy)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.GetBestPriceResponse{
		Price: price,
	})
}

// QuoteHandler handles quote requests
type QuoteHandler struct {
	engine *trading_engine.TradingEngine
}

// NewQuoteHandler creates a new quote handler
func NewQuoteHandler(engine *trading_engine.TradingEngine) *QuoteHandler {
	return &QuoteHandler{
		engine: engine,
	}
}

// GetQuote handles quote requests
func (h *QuoteHandler) GetQuote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenIn := r.URL.Query().Get("tokenIn")
	tokenOut := r.URL.Query().Get("tokenOut")
	amountStr := r.URL.Query().Get("amount")

	if tokenIn == "" || tokenOut == "" || amountStr == "" {
		http.Error(w, "Missing parameters", http.StatusBadRequest)
		return
	}

	amount, ok := new(big.Int).SetString(amountStr, 10)
	if !ok {
		http.Error(w, "Invalid amount", http.StatusBadRequest)
		return
	}

	price, err := h.engine.GetQuote(tokenIn, tokenOut, amount)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get quote: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.QuoteResponse{
		TokenIn:   tokenIn,
		TokenOut:  tokenOut,
		AmountIn: amount,
		AmountOut: price,
	})
}

// SwapHandler handles swap requests
type SwapHandler struct {
	engine *trading_engine.TradingEngine
}

// NewSwapHandler creates a new swap handler
func NewSwapHandler(engine *trading_engine.TradingEngine) *SwapHandler {
	return &SwapHandler{
		engine: engine,
	}
}

// ExecuteSwap handles swap execution
func (h *SwapHandler) ExecuteSwap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.SwapRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Get quote first
	amountOut, err := h.engine.GetQuote(req.TokenIn, req.TokenOut, req.AmountIn)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get quote: %v", err), http.StatusInternalServerError)
		return
	}

	// Apply slippage
	minOut := new(big.Int).Mul(amountOut, big.NewInt(9950))
	minOut = new(big.Int).Div(minOut, big.NewInt(10000))

	if minOut.Cmp(req.AmountOutMin) < 0 {
		http.Error(w, "Slippage exceeded", http.StatusBadRequest)
		return
	}

	// Execute swap
	txHash, err := h.engine.ExecuteSwap(req.TokenIn, req.TokenOut, req.AmountIn, minOut, req.Recipient)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to execute: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.SwapResponse{
		TxHash:   txHash,
		AmountIn: req.AmountIn,
		AmountOut: amountOut,
	})
}

// DCAServiceHandler handles DCA requests
type DCAServiceHandler struct {
	dcaService interface{}
}

// NewDCAServiceHandler creates a new DCA handler
func NewDCAServiceHandler(dca interface{}) *DCAServiceHandler {
	return &DCAServiceHandler{
		dcaService: dca,
	}
}

// CreateDCAPlan handles DCA plan creation
func (h *DCAServiceHandler) CreateDCAPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req models.CreateDCAPlanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Respond with plan ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.CreateDCAPlanResponse{
		PlanID: 1,
		Status: "active",
	})
}

// GetDCAPlan handles DCA plan retrieval
func (h *DCAServiceHandler) GetDCAPlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.DCAPlanResponse{})
}

// RouterHandler handles routing requests
type RouterHandler struct {
	router *trading_engine.TradingEngine
}

// NewRouterHandler creates a new router handler
func NewRouterHandler(router *trading_engine.TradingEngine) *RouterHandler {
	return &RouterHandler{
		router: router,
	}
}

// GetRoute handles route queries
func (h *RouterHandler) GetRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tokenIn := r.URL.Query().Get("tokenIn")
	tokenOut := r.URL.Query().Get("tokenOut")
	amountStr := r.URL.Query().Get("amount")

	if tokenIn == "" || tokenOut == "" || amountStr == "" {
		http.Error(w, "Missing parameters", http.StatusBadRequest)
		return
	}

	amount, ok := new(big.Int).SetString(amountStr, 10)
	if !ok {
		http.Error(w, "Invalid amount", http.StatusBadRequest)
		return
	}

	// Get route
	route, err := h.router.GetRoute(tokenIn, tokenOut, amount)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get route: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.RouteResponse{
		Path:       route.Path,
		AmountOut: route.AmountOut,
		GasUsed:   route.GasUsed,
	})
}

// GetSplitRoute handles split route queries
func (h *RouterHandler) GetSplitRoute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(models.SplitRouteResponse{})
}

// HealthHandler handles health check requests
type HealthHandler struct{}

// NewHealthHandler creates a new health handler
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// Health handles health checks
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

// RegisterRoutes registers all routes
func RegisterRoutes(mux *http.ServeMux, engine *trading_engine.TradingEngine) {
	orderHandler := NewOrderHandler(engine)
	quoteHandler := NewQuoteHandler(engine)
	swapHandler := NewSwapHandler(engine)
	routerHandler := NewRouterHandler(engine)
	healthHandler := NewHealthHandler()

	// Health
	mux.HandleFunc("/health", healthHandler.Health)

	// Orders
	mux.HandleFunc("/orders/create", orderHandler.CreateOrder)
	mux.HandleFunc("/orders/get", orderHandler.GetOrder)
	mux.HandleFunc("/orders/cancel", orderHandler.CancelOrder)
	mux.HandleFunc("/orders/list", orderHandler.GetOrders)
	mux.HandleFunc("/orders/book", orderHandler.GetOrderBook)
	mux.HandleFunc("/orders/best-price", orderHandler.GetBestPrice)

	// Quotes
	mux.HandleFunc("/quote", quoteHandler.GetQuote)

	// Swap
	mux.HandleFunc("/swap", swapHandler.ExecuteSwap)

	// Router
	mux.HandleFunc("/router/route", routerHandler.GetRoute)
	mux.HandleFunc("/router/split", routerHandler.GetSplitRoute)
}