package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tigerwallet/fiat-service/config"
	"github.com/tigerwallet/fiat-service/models"
	"gorm.io/gorm"
)

type FiatService struct {
	db        *gorm.DB
	redis     *redis.Client
	cfg       *config.Config
	providers map[string]Provider
}

type Provider interface {
	GetName() string
	GetQuote(ctx context.Context, req *QuoteRequest) (*FiatQuote, error)
	CreateOrder(ctx context.Context, req *CreateOrderRequest) (*OrderResponse, error)
	GetOrderStatus(ctx context.Context, orderID string) (*OrderStatus, error)
}

type QuoteRequest struct {
	FromCurrency string
	ToCurrency   string
	FromAmount   float64
	PaymentMethod string
	IPAddress    string
}

type CreateOrderRequest struct {
	QuoteID       string
	WalletAddress string
	PaymentMethodID string
	IPAddress     string
	UserAgent     string
}

type OrderResponse struct {
	OrderID         string
	PaymentURL     string
	PaymentDetails interface{}
	ExpiresAt      time.Time
}

type OrderStatus struct {
	Status        string
	PaymentStatus string
	CryptoStatus  string
	TxHash        string
	UpdatedAt     time.Time
}

func NewFiatService(db *gorm.DB, redis *redis.Client, cfg *config.Config) *FiatService {
	svc := &FiatService{
		db:        db,
		redis:     redis,
		cfg:       cfg,
		providers: make(map[string]Provider),
	}

	// Initialize providers
	svc.initProviders()

	return svc
}

func (s *FiatService) initProviders() {
	// Register providers based on configuration
	if s.cfg.StripeAPIKey != "" {
		s.providers["stripe"] = &StripeProvider{
			apiKey: s.cfg.StripeAPIKey,
			cfg:    s.cfg,
		}
	}

	if s.cfg.MoonPayAPIKey != "" {
		s.providers["moonpay"] = &MoonPayProvider{
			apiKey:    s.cfg.MoonPayAPIKey,
			secretKey: s.cfg.MoonPaySecretKey,
		}
	}

	if s.cfg.TransakAPIKey != "" {
		s.providers["transak"] = &TransakProvider{
			apiKey: s.cfg.TransakAPIKey,
		}
	}

	// Add default provider for demo
	s.providers["demo"] = &DemoProvider{cfg: s.cfg}
}

// GetProviders returns all available fiat providers
func (s *FiatService) GetProviders(ctx context.Context) ([]models.FiatProvider, error) {
	var providers []models.FiatProvider
	err := s.db.Where("is_active = ?", true).Find(&providers).Error
	if err != nil {
		return nil, err
	}

	// If no providers in DB, return defaults
	if len(providers) == 0 {
		providers = s.getDefaultProviders()
	}

	return providers, nil
}

// GetQuote returns a quote for fiat-crypto exchange
func (s *FiatService) GetQuote(ctx context.Context, providerID string, req *QuoteRequest) (*models.FiatQuote, error) {
	provider, ok := s.providers[providerID]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	quote, err := provider.GetQuote(ctx, req)
	if err != nil {
		return nil, err
	}

	// Cache the quote
	quoteJSON, _ := json.Marshal(quote)
	s.redis.Set(ctx, "quote:"+quote.QuoteID, quoteJSON, 15*time.Minute)

	return quote, nil
}

// CreateOrder creates a new fiat order
func (s *FiatService) CreateOrder(ctx context.Context, providerID string, req *CreateOrderRequest) (*models.FiatOrder, error) {
	// Get cached quote
	quoteJSON, err := s.redis.Get(ctx, "quote:"+req.QuoteID).Result()
	if err != nil {
		return nil, fmt.Errorf("quote not found or expired")
	}

	var quote models.FiatQuote
	if err := json.Unmarshal([]byte(quoteJSON), &quote); err != nil {
		return nil, err
	}

	// Create order in database
	order := &models.FiatOrder{
		ProviderID:     providerID,
		WalletAddress:  req.WalletAddress,
		FiatAmount:     quote.FiatAmount,
		FiatCurrency:   quote.FiatCurrency,
		CryptoAmount:   quote.CryptoAmount,
		CryptoCurrency: quote.CryptoCurrency,
		ExchangeRate:   quote.ExchangeRate,
		ProviderFee:    quote.ProviderFee,
		ServiceFee:     quote.ServiceFee,
		TotalFees:      quote.TotalFees,
		QuoteID:        req.QuoteID,
		QuoteExpiresAt: quote.ExpiresAt,
		IPAddress:      req.IPAddress,
		UserAgent:      req.UserAgent,
		Status:         models.OrderStatusPending,
		PaymentStatus:  models.PaymentStatusPending,
		CryptoStatus:   models.CryptoStatusPending,
	}

	if err := s.db.Create(order).Error; err != nil {
		return nil, err
	}

	// Call provider to create order
	provider, ok := s.providers[providerID]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", providerID)
	}

	providerReq := &CreateOrderRequest{
		QuoteID:        req.QuoteID,
		WalletAddress:  req.WalletAddress,
		PaymentMethodID: req.PaymentMethodID,
	}

	providerResp, err := provider.CreateOrder(ctx, providerReq)
	if err != nil {
		order.Status = models.OrderStatusFailed
		order.ErrorMessage = err.Error()
		s.db.Save(order)
		return order, err
	}

	order.ProviderOrderID = providerResp.OrderID
	s.db.Save(order)

	return order, nil
}

// GetOrder returns order by ID
func (s *FiatService) GetOrder(ctx context.Context, orderID string) (*models.FiatOrder, error) {
	var order models.FiatOrder
	err := s.db.Where("id = ?", orderID).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

// CompleteOrder marks an order as completed
func (s *FiatService) CompleteOrder(ctx context.Context, orderID string, txHash string) error {
	now := time.Now()
	return s.db.Model(&models.FiatOrder{}).Where("id = ?", orderID).Updates(map[string]interface{}{
		"status":           models.OrderStatusCompleted,
		"crypto_status":    models.CryptoStatusCompleted,
		"transaction_hash": txHash,
		"completed_at":     now,
		"updated_at":       now,
	}).Error
}

// CancelOrder cancels an order
func (s *FiatService) CancelOrder(ctx context.Context, orderID string) error {
	return s.db.Model(&models.FiatOrder{}).Where("id = ? AND status = ?", orderID, models.OrderStatusPending).Updates(map[string]interface{}{
		"status":      models.OrderStatusCancelled,
		"updated_at":  time.Now(),
	}).Error
}

func (s *FiatService) getDefaultProviders() []models.FiatProvider {
	return []models.FiatProvider{
		{
			ID:                   uuid.New().String(),
			Name:                 "Stripe",
			Logo:                 "https://logos.stripe.com//logo.png",
			Description:          "Buy crypto with credit/debit card via Stripe",
			SupportedCurrencies:  []string{"USD", "EUR", "GBP"},
			FeePercent:           2.5,
			MinAmount:            50,
			MaxAmount:            25000,
			IsActive:             true,
		},
		{
			ID:                   uuid.New().String(),
			Name:                 "MoonPay",
			Logo:                 "https://www.moonpay.com/logo.png",
			Description:          "Buy crypto with credit/debit card or bank transfer",
			SupportedCurrencies:  []string{"USD", "EUR", "GBP", "AUD", "CAD"},
			FeePercent:           3.0,
			MinAmount:            30,
			MaxAmount:            50000,
			IsActive:             true,
		},
		{
			ID:                   "demo",
			Name:                 "Demo Provider",
			Logo:                 "",
			Description:          "Demo provider for testing",
			SupportedCurrencies:  []string{"USD", "EUR"},
			FeePercent:           1.0,
			MinAmount:            10,
			MaxAmount:            10000,
			IsActive:             true,
		},
	}
}

// Demo Provider implementation
type DemoProvider struct {
	cfg *config.Config
}

func (p *DemoProvider) GetName() string { return "Demo Provider" }

func (p *DemoProvider) GetQuote(ctx context.Context, req *QuoteRequest) (*models.FiatQuote, error) {
	// Mock exchange rates
	rates := map[string]float64{
		"ETH": 2500.00,
		"WBTC": 45000.00,
		"USDC": 1.00,
	}

	rate, ok := rates[req.ToCurrency]
	if !ok {
		rate = 2500.00 // Default to ETH rate
	}

	// Calculate crypto amount
	cryptoAmount := req.FromAmount / rate

	// Calculate fees
	providerFee := req.FromAmount * 0.02 // 2%
	serviceFee := req.FromAmount * (p.cfg.ServiceFeePercent / 100)

	return &models.FiatQuote{
		ProviderID:      "demo",
		QuoteID:         uuid.New().String(),
		FiatAmount:      req.FromAmount,
		FiatCurrency:    req.FromCurrency,
		CryptoAmount:    cryptoAmount,
		CryptoCurrency:  req.ToCurrency,
		ExchangeRate:    rate,
		ProviderFee:     providerFee,
		ServiceFee:      serviceFee,
		TotalFees:       providerFee + serviceFee,
		PaymentMethods: []models.PaymentMethodOption{
			{
				ID:             "card",
				Name:           "Credit/Debit Card",
				Type:           "CARD",
				FeePercent:     2.5,
				ProcessingTime: "5-15 minutes",
			},
			{
				ID:             "apple_pay",
				Name:           "Apple Pay",
				Type:           "APPLE_PAY",
				FeePercent:     2.5,
				ProcessingTime: "5-15 minutes",
			},
			{
				ID:             "google_pay",
				Name:           "Google Pay",
				Type:           "GOOGLE_PAY",
				FeePercent:     2.5,
				ProcessingTime: "5-15 minutes",
			},
		},
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}, nil
}

func (p *DemoProvider) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*OrderResponse, error) {
	return &OrderResponse{
		OrderID:    uuid.New().String(),
		PaymentURL: "https://demo.tigerwallet.io/pay/" + uuid.New().String(),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
	}, nil
}

func (p *DemoProvider) GetOrderStatus(ctx context.Context, orderID string) (*OrderStatus, error) {
	return &OrderStatus{
		Status:        "pending",
		PaymentStatus: "pending",
		CryptoStatus:  "pending",
		UpdatedAt:     time.Now(),
	}, nil
}

// StripeProvider implementation
type StripeProvider struct {
	apiKey string
	cfg    *config.Config
}

func (p *StripeProvider) GetName() string { return "Stripe" }

func (p *StripeProvider) GetQuote(ctx context.Context, req *QuoteRequest) (*models.FiatQuote, error) {
	// Real implementation would call Stripe API
	return &models.FiatQuote{
		ProviderID:     "stripe",
		QuoteID:        uuid.New().String(),
		FiatAmount:     req.FromAmount,
		FiatCurrency:   req.FromCurrency,
		CryptoAmount:   req.FromAmount / 2500,
		CryptoCurrency: "ETH",
		ExchangeRate:   2500,
		ExpiresAt:      time.Now().Add(10 * time.Minute),
	}, nil
}

func (p *StripeProvider) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*OrderResponse, error) {
	return &OrderResponse{
		OrderID:    uuid.New().String(),
		PaymentURL: "https://checkout.stripe.com/" + uuid.New().String(),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
	}, nil
}

func (p *StripeProvider) GetOrderStatus(ctx context.Context, orderID string) (*OrderStatus, error) {
	return &OrderStatus{
		Status:     "pending",
		UpdatedAt:  time.Now(),
	}, nil
}

// MoonPayProvider implementation
type MoonPayProvider struct {
	apiKey    string
	secretKey string
}

func (p *MoonPayProvider) GetName() string { return "MoonPay" }

func (p *MoonPayProvider) GetQuote(ctx context.Context, req *QuoteRequest) (*models.FiatQuote, error) {
	return &models.FiatQuote{
		ProviderID:     "moonpay",
		QuoteID:        uuid.New().String(),
		FiatAmount:     req.FromAmount,
		FiatCurrency:   req.FromCurrency,
		CryptoAmount:   req.FromAmount / 2500,
		CryptoCurrency: "ETH",
		ExchangeRate:   2500,
		ExpiresAt:      time.Now().Add(10 * time.Minute),
	}, nil
}

func (p *MoonPayProvider) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*OrderResponse, error) {
	return &OrderResponse{
		OrderID:    uuid.New().String(),
		PaymentURL: "https://buy.moonpay.com/" + uuid.New().String(),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
	}, nil
}

func (p *MoonPayProvider) GetOrderStatus(ctx context.Context, orderID string) (*OrderStatus, error) {
	return &OrderStatus{
		Status:     "pending",
		UpdatedAt:  time.Now(),
	}, nil
}

// TransakProvider implementation
type TransakProvider struct {
	apiKey string
}

func (p *TransakProvider) GetName() string { return "Transak" }

func (p *TransakProvider) GetQuote(ctx context.Context, req *QuoteRequest) (*models.FiatQuote, error) {
	return &models.FiatQuote{
		ProviderID:     "transak",
		QuoteID:        uuid.New().String(),
		FiatAmount:     req.FromAmount,
		FiatCurrency:   req.FromCurrency,
		CryptoAmount:   req.FromAmount / 2500,
		CryptoCurrency: "ETH",
		ExchangeRate:   2500,
		ExpiresAt:      time.Now().Add(10 * time.Minute),
	}, nil
}

func (p *TransakProvider) CreateOrder(ctx context.Context, req *CreateOrderRequest) (*OrderResponse, error) {
	return &OrderResponse{
		OrderID:    uuid.New().String(),
		PaymentURL: "https://global.transak.com/" + uuid.New().String(),
		ExpiresAt:  time.Now().Add(30 * time.Minute),
	}, nil
}

func (p *TransakProvider) GetOrderStatus(ctx context.Context, orderID string) (*OrderStatus, error) {
	return &OrderStatus{
		Status:     "pending",
		UpdatedAt:  time.Now(),
	}, nil
}
