package models

import (
	"time"

	"github.com/google/uuid"
)

type FiatProvider struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Name        string    `json:"name" gorm:"type:varchar(100);not null"`
	Logo        string    `json:"logo" gorm:"type:text"`
	Description string    `json:"description" gorm:"type:text"`
	SupportedCurrencies []string `json:"supported_currencies" gorm:"type:jsonb"`
	FeePercent  float64   `json:"fee_percent" gorm:"type:decimal(5,2)"`
	MinAmount   float64   `json:"min_amount" gorm:"type:decimal(18,2)"`
	MaxAmount   float64   `json:"max_amount" gorm:"type:decimal(18,2)"`
	IsActive    bool      `json:"is_active" gorm:"default:true"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (p *FiatProvider) BeforeCreate() error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

type PaymentMethod struct {
	ID          string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	ProviderID  string    `json:"provider_id" gorm:"type:varchar(36);not null"`
	Name        string    `json:"name" gorm:"type:varchar(100);not null"`
	Type        string    `json:"type" gorm:"type:varchar(50);not null"`
	Icon        string    `json:"icon" gorm:"type:text"`
	FeePercent  float64   `json:"fee_percent" gorm:"type:decimal(5,2)"`
	FixedFee    float64   `json:"fixed_fee" gorm:"type:decimal(18,8)"`
	ProcessingTime string `json:"processing_time" gorm:"type:varchar(50)"`
	IsActive    bool      `json:"is_active" gorm:"default:true"`
	CreatedAt   time.Time `json:"created_at"`
}

func (m *PaymentMethod) BeforeCreate() error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}

type FiatOrder struct {
	ID              string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	ProviderID      string    `json:"provider_id" gorm:"type:varchar(36);not null"`
	ProviderOrderID string    `json:"provider_order_id" gorm:"type:varchar(255)"`
	UserID          string    `json:"user_id" gorm:"type:varchar(36);not null"`
	WalletAddress   string    `json:"wallet_address" gorm:"type:varchar(42);not null"`
	
	// Amounts
	FiatAmount      float64   `json:"fiat_amount" gorm:"type:decimal(18,2);not null"`
	FiatCurrency    string    `json:"fiat_currency" gorm:"type:varchar(10);not null"`
	CryptoAmount    float64   `json:"crypto_amount" gorm:"type:decimal(38,0);not null"`
	CryptoCurrency  string    `json:"crypto_currency" gorm:"type:varchar(10);not null"`
	ExchangeRate    float64   `json:"exchange_rate" gorm:"type:decimal(30,8)"`
	
	// Fees
	ProviderFee     float64   `json:"provider_fee" gorm:"type:decimal(18,2)"`
	ServiceFee      float64   `json:"service_fee" gorm:"type:decimal(18,2)"`
	TotalFees       float64   `json:"total_fees" gorm:"type:decimal(18,2)"`
	
	// Payment
	PaymentMethodID string    `json:"payment_method_id" gorm:"type:varchar(36)"`
	PaymentStatus   string    `json:"payment_status" gorm:"type:varchar(50);default:'pending'"`
	
	// Crypto
	CryptoStatus    string    `json:"crypto_status" gorm:"type:varchar(50);default:'pending'"`
	TransactionHash string    `json:"transaction_hash" gorm:"type:varchar(66)"`
	
	// Metadata
	QuoteID         string    `json:"quote_id" gorm:"type:varchar(255)"`
	QuoteExpiresAt  time.Time `json:"quote_expires_at"`
	IPAddress       string    `json:"ip_address" gorm:"type:varchar(45)"`
	UserAgent       string    `json:"user_agent" gorm:"type:text"`
	
	Status          string    `json:"status" gorm:"type:varchar(50);default:'pending'"`
	ErrorMessage    string    `json:"error_message" gorm:"type:text"`
	
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	CompletedAt     *time.Time `json:"completed_at"`
}

func (o *FiatOrder) BeforeCreate() error {
	if o.ID == "" {
		o.ID = uuid.New().String()
	}
	return nil
}

// FiatQuote represents a quote for fiat-crypto exchange
type FiatQuote struct {
	ProviderID      string    `json:"provider_id"`
	QuoteID        string    `json:"quote_id"`
	FiatAmount     float64   `json:"fiat_amount"`
	FiatCurrency   string    `json:"fiat_currency"`
	CryptoAmount   float64   `json:"crypto_amount"`
	CryptoCurrency string    `json:"crypto_currency"`
	ExchangeRate   float64   `json:"exchange_rate"`
	ProviderFee    float64   `json:"provider_fee"`
	ServiceFee     float64   `json:"service_fee"`
	TotalFees      float64   `json:"totalFees"`
	PaymentMethods []PaymentMethodOption `json:"payment_methods"`
	ExpiresAt      time.Time `json:"expires_at"`
}

type PaymentMethodOption struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Type           string  `json:"type"`
	Icon           string  `json:"icon"`
	FeePercent     float64 `json:"fee_percent"`
	FixedFee       float64 `json:"fixed_fee"`
	ProcessingTime string  `json:"processing_time"`
}

// OrderStatus constants
const (
	OrderStatusPending    = "pending"
	OrderStatusProcessing = "processing"
	OrderStatusCompleted  = "completed"
	OrderStatusFailed     = "failed"
	OrderStatusCancelled  = "cancelled"
)

// PaymentStatus constants
const (
	PaymentStatusPending    = "pending"
	PaymentStatusProcessing = "processing"
	PaymentStatusCompleted = "completed"
	PaymentStatusFailed    = "failed"
	PaymentStatusCancelled = "cancelled"
)

// CryptoStatus constants
const (
	CryptoStatusPending    = "pending"
	CryptoStatusProcessing = "processing"
	CryptoStatusCompleted = "completed"
	CryptoStatusFailed    = "failed"
)
