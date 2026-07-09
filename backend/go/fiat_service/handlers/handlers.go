package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/tigerwallet/fiat-service/models"
	"github.com/tigerwallet/fiat-service/services"
)

type FiatHandler struct {
	fiatService  *services.FiatService
	orderService *services.OrderService
}

func NewFiatHandler(fiatService *services.FiatService, orderService *services.OrderService) *FiatHandler {
	return &FiatHandler{
		fiatService:  fiatService,
		orderService: orderService,
	}
}

// GetProviders returns all available fiat providers
func (h *FiatHandler) GetProviders(c *gin.Context) {
	providers, err := h.fiatService.GetProviders(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"providers": providers,
	})
}

// GetQuote returns a quote for fiat-crypto exchange
func (h *FiatHandler) GetQuote(c *gin.Context) {
	providerID := c.Param("id")
	
	var req struct {
		FromCurrency  string  `json:"from_currency" binding:"required"`
		ToCurrency    string  `json:"to_currency" binding:"required"`
		FromAmount    float64 `json:"from_amount" binding:"required,gt=0"`
		PaymentMethod string  `json:"payment_method"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	quote, err := h.fiatService.GetQuote(c.Request.Context(), providerID, &services.QuoteRequest{
		FromCurrency:  req.FromCurrency,
		ToCurrency:    req.ToCurrency,
		FromAmount:    req.FromAmount,
		PaymentMethod: req.PaymentMethod,
		IPAddress:    c.ClientIP(),
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, quote)
}

// CreateOrder creates a new fiat order
func (h *FiatHandler) CreateOrder(c *gin.Context) {
	providerID := c.Param("id")

	var req struct {
		QuoteID         string `json:"quote_id" binding:"required"`
		WalletAddress   string `json:"wallet_address" binding:"required"`
		PaymentMethodID string `json:"payment_method_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order, err := h.fiatService.CreateOrder(c.Request.Context(), providerID, &services.CreateOrderRequest{
		QuoteID:        req.QuoteID,
		WalletAddress:  req.WalletAddress,
		PaymentMethodID: req.PaymentMethodID,
		IPAddress:     c.ClientIP(),
		UserAgent:     c.Request.UserAgent(),
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, order)
}

// GetOrder returns order by ID
func (h *FiatHandler) GetOrder(c *gin.Context) {
	orderID := c.Param("id")

	order, err := h.fiatService.GetOrder(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	c.JSON(http.StatusOK, order)
}

// CompleteOrder marks an order as completed
func (h *FiatHandler) CompleteOrder(c *gin.Context) {
	orderID := c.Param("id")

	var req struct {
		TransactionHash string `json:"transaction_hash" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := h.fiatService.CompleteOrder(c.Request.Context(), orderID, req.TransactionHash)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "completed"})
}

// CancelOrder cancels an order
func (h *FiatHandler) CancelOrder(c *gin.Context) {
	orderID := c.Param("id")

	err := h.fiatService.CancelOrder(c.Request.Context(), orderID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

// StripeWebhook handles Stripe webhook events
func (h *FiatHandler) StripeWebhook(c *gin.Context) {
	payload := c.GetRawData()

	// Verify webhook signature in production
	// signature := c.GetHeader("Stripe-Signature")

	var event struct {
		Type string `json:"type"`
		Data struct {
			Object struct {
				ID string `json:"id"`
			} `json:"object"`
		} `json:"data"`
	}

	// Parse event in production

	// Handle different event types
	// - payment_intent.succeeded
	// - payment_intent.payment_failed

	c.JSON(http.StatusOK, gin.H{"received": true})
}

// CoinbaseWebhook handles Coinbase Commerce webhook events
func (h *FiatHandler) CoinbaseWebhook(c *gin.Context) {
	payload := c.GetRawData()

	// Verify webhook signature in production

	c.JSON(http.StatusOK, gin.H{"received": true})
}
