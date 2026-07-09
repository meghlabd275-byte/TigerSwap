package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// OrderRequest represents order creation request
type OrderRequest struct {
	MarketID   string  `json:"market_id" binding:"required"`
	Side       string  `json:"side" binding:"required,oneof=buy sell"`
	OrderType  string  `json:"order_type" binding:"required,oneof=limit market stop_loss stop_limit take_profit take_profit_limit"`
	Price      string  `json:"price"`
	Quantity   string  `json:"quantity" binding:"required"`
	StopPrice  string  `json:"stop_price"`
	TimeInForce string `json:"time_in_force" binding:"oneof=gtc ioc fok gtd"`
	PostOnly   bool    `json:"post_only"`
}

// Order represents an order
type Order struct {
	ID            string    `json:"id"`
	MarketID      string    `json:"market_id"`
	UserID        string    `json:"user_id"`
	Side          string    `json:"side"`
	OrderType     string    `json:"order_type"`
	Price         string    `json:"price"`
	Quantity      string    `json:"quantity"`
	FilledQty     string    `json:"filled_qty"`
	AvgFillPrice  string    `json:"avg_fill_price"`
	Status        string    `json:"status"`
	TimeInForce   string    `json:"time_in_force"`
	StopPrice     string    `json:"stop_price,omitempty"`
	CreatedAt     int64     `json:"created_at"`
	UpdatedAt     int64     `json:"updated_at"`
}

// CreateOrder creates a new order
// @Summary Create Order
// @Description Create a new trading order
// @Tags orders
// @Accept json
// @Produce json
// @Param request body OrderRequest true "Order request"
// @Success 201 {object} Order
// @Router /api/v1/orders [post]
func CreateOrder(c *gin.Context) {
	var req OrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get user ID from context (set by auth middleware)
	userID, _ := c.Get("user_id")

	order := Order{
		ID:            "order_" + generateID(),
		MarketID:      req.MarketID,
		UserID:        userID.(string),
		Side:          req.Side,
		OrderType:     req.OrderType,
		Price:         req.Price,
		Quantity:      req.Quantity,
		FilledQty:     "0",
		AvgFillPrice:  "0",
		Status:        "open",
		TimeInForce:   req.TimeInForce,
		StopPrice:     req.StopPrice,
		CreatedAt:     time.Now().Unix(),
		UpdatedAt:     time.Now().Unix(),
	}

	// In production, submit to matching engine
	c.JSON(http.StatusCreated, order)
}

// ListOrders returns user's orders
// @Summary List Orders
// @Description Get user's orders
// @Tags orders
// @Accept json
// @Produce json
// @Param market query string false "Market ID"
// @Param status query string false "Order status"
// @Param limit query int false "Limit"
// @Success 200 {array} Order
// @Router /api/v1/orders [get]
func ListOrders(c *gin.Context) {
	userID, _ := c.Get("user_id")
	market := c.Query("market")
	status := c.Query("status")

	orders := []Order{
		{
			ID:           "order_1",
			MarketID:     "ETH-USDT",
			UserID:       userID.(string),
			Side:         "buy",
			OrderType:    "limit",
			Price:        "3200.00",
			Quantity:     "1.0",
			FilledQty:    "0",
			AvgFillPrice: "0",
			Status:       "open",
			TimeInForce:  "gtc",
			CreatedAt:    time.Now().Unix() - 3600,
			UpdatedAt:    time.Now().Unix() - 3600,
		},
	}

	// Filter by market and status
	if market != "" {
		var filtered []Order
		for _, o := range orders {
			if o.MarketID == market {
				filtered = append(filtered, o)
			}
		}
		orders = filtered
	}

	if status != "" {
		var filtered []Order
		for _, o := range orders {
			if o.Status == status {
				filtered = append(filtered, o)
			}
		}
		orders = filtered
	}

	c.JSON(http.StatusOK, orders)
}

// GetOrder returns order details
// @Summary Get Order
// @Description Get order details by ID
// @Tags orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} Order
// @Router /api/v1/orders/{id} [get]
func GetOrder(c *gin.Context) {
	orderID := c.Param("id")

	order := Order{
		ID:            orderID,
		MarketID:      "ETH-USDT",
		UserID:        "user_1",
		Side:          "buy",
		OrderType:     "limit",
		Price:         "3200.00",
		Quantity:      "1.0",
		FilledQty:     "0.5",
		AvgFillPrice:  "3200.00",
		Status:        "partially_filled",
		TimeInForce:   "gtc",
		CreatedAt:     time.Now().Unix() - 3600,
		UpdatedAt:     time.Now().Unix(),
	}

	c.JSON(http.StatusOK, order)
}

// CancelOrder cancels an order
// @Summary Cancel Order
// @Description Cancel an existing order
// @Tags orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Success 200 {object} map[string]string
// @Router /api/v1/orders/{id} [delete]
func CancelOrder(c *gin.Context) {
	orderID := c.Param("id")

	// In production, submit cancellation to matching engine
	c.JSON(http.StatusOK, gin.H{
		"id":      orderID,
		"status":  "cancelled",
		"message": "Order cancelled successfully",
	})
}

// CancelAllOrders cancels all user orders
// @Summary Cancel All Orders
// @Description Cancel all open orders
// @Tags orders
// @Accept json
// @Produce json
// @Param market query string false "Market ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/orders [delete]
func CancelAllOrders(c *gin.Context) {
	market := c.Query("market")

	// In production, cancel all orders
	c.JSON(http.StatusOK, gin.H{
		"cancelled":  5,
		"market":    market,
		"timestamp": time.Now().Unix(),
	})
}

// ModifyOrder modifies an existing order
// @Summary Modify Order
// @Description Modify order price or quantity
// @Tags orders
// @Accept json
// @Produce json
// @Param id path string true "Order ID"
// @Param request body map[string]string true "Modify request"
// @Success 200 {object} Order
// @Router /api/v1/orders/{id}/modify [post]
func ModifyOrder(c *gin.Context) {
	orderID := c.Param("id")

	var req map[string]string
	c.ShouldBindJSON(&req)

	order := Order{
		ID:            orderID,
		MarketID:      "ETH-USDT",
		UserID:        "user_1",
		Side:          "buy",
		OrderType:     "limit",
		Price:         req["price"],
		Quantity:      req["quantity"],
		FilledQty:     "0",
		AvgFillPrice:  "0",
		Status:        "open",
		TimeInForce:  "gtc",
		CreatedAt:     time.Now().Unix() - 3600,
		UpdatedAt:     time.Now().Unix(),
	}

	c.JSON(http.StatusOK, order)
}
