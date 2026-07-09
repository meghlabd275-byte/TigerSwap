package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/tigerwallet/fiat-service/config"
	"gorm.io/gorm"
)

type OrderService struct {
	db    *gorm.DB
	redis *redis.Client
	cfg   *config.Config
}

func NewOrderService(db *gorm.DB, redis *redis.Client, cfg *config.Config) *OrderService {
	return &OrderService{
		db:    db,
		redis: redis,
		cfg:   cfg,
	}
}

// ProcessOrderBackground processes orders in the background
func (s *OrderService) ProcessOrderBackground(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkPendingOrders(ctx)
		}
	}
}

func (s *OrderService) checkPendingOrders(ctx context.Context) {
	// Check orders that need processing
	// This would query for orders in "processing" state and check their status with providers
}

// NotifyUser sends a notification to the user when order status changes
func (s *OrderService) NotifyUser(userID string, orderID string, status string) error {
	// Publish to notification channel
	notification := map[string]interface{}{
		"type":      "fiat_order",
		"order_id":  orderID,
		"status":    status,
		"timestamp": time.Now().Unix(),
	}

	notificationJSON, _ := json.Marshal(notification)
	return s.redis.Publish(ctx, "notifications:"+userID, notificationJSON).Err()
}

// GetOrderHistory returns the order history for a user
func (s *OrderService) GetOrderHistory(ctx context.Context, userID string, limit, offset int) ([]map[string]interface{}, error) {
	// This would query the database for orders
	return []map[string]interface{}{}, nil
}

// CalculateOrderFees calculates the total fees for an order
func (s *OrderService) CalculateOrderFees(fiatAmount float64, providerFeePercent float64) (providerFee, serviceFee, totalFee float64) {
	providerFee = fiatAmount * (providerFeePercent / 100)
	serviceFee = fiatAmount * (s.cfg.ServiceFeePercent / 100)
	totalFee = providerFee + serviceFee
	return
}

// ValidateOrder validates an order before creation
func (s *OrderService) ValidateOrder(walletAddress string, fiatAmount float64, provider *ProviderConfig) error {
	// Validate wallet address format
	if walletAddress == "" || len(walletAddress) != 42 {
		return fmt.Errorf("invalid wallet address")
	}

	// Validate amount is within provider limits
	if fiatAmount < provider.MinAmount || fiatAmount > provider.MaxAmount {
		return fmt.Errorf("amount outside provider limits: min %v, max %v", provider.MinAmount, provider.MaxAmount)
	}

	return nil
}

type ProviderConfig struct {
	MinAmount   float64
	MaxAmount   float64
	FeePercent  float64
}
