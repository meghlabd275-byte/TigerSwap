// TigerSwap Notification Service - Go Implementation
// Real-time notifications for price alerts, order fills, and more

package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// NotificationType notification types
type NotificationType string

const (
	TypePriceAlert     NotificationType = "price_alert"
	TypeOrderFilled    NotificationType = "order_filled"
	TypeOrderPartial   NotificationType = "order_partial"
	TypeOrderCancelled NotificationType = "order_cancelled"
	TypeSwapSuccess    NotificationType = "swap_success"
	TypeSwapFailed     NotificationType = "swap_failed"
	TypeLiquidityAdded NotificationType = "liquidity_added"
	TypeLiquidityRemoved NotificationType = "liquidity_removed"
	TypeLargeTransaction NotificationType = "large_transaction"
	TypeSystemUpdate   NotificationType = "system_update"
	TypeSecurityAlert  NotificationType = "security_alert"
)

// NotificationChannel notification channels
type NotificationChannel string

const (
	ChannelInApp   NotificationChannel = "in_app"
	ChannelEmail   NotificationChannel = "email"
	ChannelPush    NotificationChannel = "push"
	ChannelSMS     NotificationChannel = "sms"
	ChannelDiscord NotificationChannel = "discord"
	ChannelTelegram NotificationChannel = "telegram"
)

// Notification notification structure
type Notification struct {
	ID         string               `json:"id"`
	Type       NotificationType      `json:"type"`
	Title      string               `json:"title"`
	Message    string               `json:"message"`
	Data       map[string]interface{} `json:"data,omitempty"`
	Read       bool                 `json:"read"`
	Dismissed  bool                 `json:"dismissed"`
	CreatedAt  int64                `json:"createdAt"`
	ExpiresAt  int64                `json:"expiresAt,omitempty"`
	Channels   []NotificationChannel `json:"channels"`
	UserID     string               `json:"userId,omitempty"`
	WalletAddress string            `json:"walletAddress,omitempty"`
}

// PriceAlert price alert structure
type PriceAlert struct {
	ID          string `json:"id"`
	TokenAddress string `json:"tokenAddress"`
	TokenSymbol  string `json:"tokenSymbol"`
	TargetPrice  float64 `json:"targetPrice"`
	Condition    string `json:"condition"`
	Triggered    bool   `json:"triggered"`
	TriggeredAt  int64  `json:"triggeredAt,omitempty"`
	CreatedAt    int64  `json:"createdAt"`
	UserID       string `json:"userId"`
}

// NotificationPreferences user preferences
type NotificationPreferences struct {
	Email              bool    `json:"email"`
	Push               bool    `json:"push"`
	SMS                bool    `json:"sms"`
	Discord            bool    `json:"discord"`
	Telegram           bool    `json:"telegram"`
	PriceAlerts        bool    `json:"priceAlerts"`
	OrderUpdates       bool    `json:"orderUpdates"`
	SwapUpdates        bool    `json:"swapUpdates"`
	LiquidityUpdates   bool    `json:"liquidityUpdates"`
	LargeTransactions  bool    `json:"largeTransactions"`
	SystemUpdates      bool    `json:"systemUpdates"`
	MinTransactionThreshold float64 `json:"minTransactionThreshold"`
}

// NotificationService notification service
type NotificationService struct {
	mu           sync.RWMutex
	notifications map[string]*Notification
	priceAlerts   map[string]*PriceAlert
	preferences   map[string]*NotificationPreferences
	subscribers   map[string][]func(*Notification)
	priceCheckInterval *time.Ticker
}

func NewNotificationService() *NotificationService {
	s := &NotificationService{
		notifications: make(map[string]*Notification),
		priceAlerts:   make(map[string]*PriceAlert),
		preferences:   make(map[string]*NotificationPreferences),
		subscribers:   make(map[string][]func(*Notification)),
	}
	s.startPriceMonitoring()
	return s
}

func defaultPreferences() *NotificationPreferences {
	return &NotificationPreferences{
		Email:              true,
		Push:               true,
		SMS:                false,
		Discord:            false,
		Telegram:           false,
		PriceAlerts:        true,
		OrderUpdates:       true,
		SwapUpdates:        true,
		LiquidityUpdates:  true,
		LargeTransactions: true,
		SystemUpdates:     true,
		MinTransactionThreshold: 10000,
	}
}

// SendNotification sends a notification
func (s *NotificationService) SendNotification(userID string, notification *Notification) *Notification {
	s.mu.Lock()
	defer s.mu.Unlock()

	notification.ID = fmt.Sprintf("notif_%d_%d", time.Now().Unix(), time.Now().UnixNano()%10000)
	notification.CreatedAt = time.Now().Unix()
	notification.Read = false
	notification.Dismissed = false

	s.notifications[notification.ID] = notification

	// Get user preferences
	prefs := s.getPreferences(userID)

	// Send to each enabled channel
	for _, channel := range notification.Channels {
		if s.isChannelEnabled(prefs, channel, notification.Type) {
			s.sendToChannel(userID, notification, channel)
		}
	}

	// Notify subscribers
	s.notifySubscribers(userID, notification)

	return notification
}

// GetNotifications retrieves notifications for a user
func (s *NotificationService) GetNotifications(userID string, unreadOnly bool) []*Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Notification, 0)
	for _, n := range s.notifications {
		if n.UserID == userID || n.WalletAddress == userID {
			if unreadOnly && n.Read {
				continue
			}
			result = append(result, n)
		}
	}

	// Sort by creation time descending
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].CreatedAt < result[j].CreatedAt {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// MarkAsRead marks a notification as read
func (s *NotificationService) MarkAsRead(notificationID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if n, ok := s.notifications[notificationID]; ok {
		n.Read = true
	}
}

// MarkAllAsRead marks all notifications as read for a user
func (s *NotificationService) MarkAllAsRead(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, n := range s.notifications {
		if n.UserID == userID && !n.Read {
			n.Read = true
		}
	}
}

// GetUnreadCount returns unread notification count
func (s *NotificationService) GetUnreadCount(userID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, n := range s.notifications {
		if (n.UserID == userID || n.WalletAddress == userID) && !n.Read && !n.Dismissed {
			count++
		}
	}
	return count
}

// CreatePriceAlert creates a price alert
func (s *NotificationService) CreatePriceAlert(userID, tokenAddress, tokenSymbol string, targetPrice float64, condition string) *PriceAlert {
	s.mu.Lock()
	defer s.mu.Unlock()

	alert := &PriceAlert{
		ID:          fmt.Sprintf("alert_%d", time.Now().UnixNano()),
		TokenAddress: tokenAddress,
		TokenSymbol:  tokenSymbol,
		TargetPrice:  targetPrice,
		Condition:    condition,
		Triggered:    false,
		CreatedAt:    time.Now().Unix(),
		UserID:       userID,
	}

	s.priceAlerts[alert.ID] = alert
	return alert
}

// GetPriceAlerts retrieves price alerts for a user
func (s *NotificationService) GetPriceAlerts(userID string) []*PriceAlert {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*PriceAlert, 0)
	for _, a := range s.priceAlerts {
		if a.UserID == userID && !a.Triggered {
			result = append(result, a)
		}
	}
	return result
}

// GetPreferences returns user preferences
func (s *NotificationService) GetPreferences(userID string) *NotificationPreferences {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getPreferences(userID)
}

func (s *NotificationService) getPreferences(userID string) *NotificationPreferences {
	if prefs, ok := s.preferences[userID]; ok {
		return prefs
	}
	return defaultPreferences()
}

// UpdatePreferences updates user preferences
func (s *NotificationService) UpdatePreferences(userID string, prefs *NotificationPreferences) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.preferences[userID] = prefs
}

// Subscribe subscribes to notifications
func (s *NotificationService) Subscribe(userID string, callback func(*Notification)) {
	s.mu.Lock()
	defer s.mu.Unlock()

	callbacks := s.subscribers[userID]
	callbacks = append(callbacks, callback)
	s.subscribers[userID] = callbacks
}

func (s *NotificationService) notifySubscribers(userID string, notification *Notification) {
	callbacks := s.subscribers[userID]
	for _, cb := range callbacks {
		go cb(notification)
	}
}

func (s *NotificationService) isChannelEnabled(prefs *NotificationPreferences, channel NotificationChannel, notifType NotificationType) bool {
	switch channel {
	case ChannelEmail:
		return prefs.Email
	case ChannelPush:
		return prefs.Push
	case ChannelSMS:
		return prefs.SMS
	case ChannelDiscord:
		return prefs.Discord
	case ChannelTelegram:
		return prefs.Telegram
	}

	switch notifType {
	case TypePriceAlert:
		return prefs.PriceAlerts
	case TypeOrderFilled, TypeOrderPartial, TypeOrderCancelled:
		return prefs.OrderUpdates
	case TypeSwapSuccess, TypeSwapFailed:
		return prefs.SwapUpdates
	case TypeLiquidityAdded, TypeLiquidityRemoved:
		return prefs.LiquidityUpdates
	case TypeLargeTransaction:
		return prefs.LargeTransactions
	case TypeSystemUpdate, TypeSecurityAlert:
		return prefs.SystemUpdates
	}
	return true
}

func (s *NotificationService) sendToChannel(userID string, notification *Notification, channel NotificationChannel) {
	switch channel {
	case ChannelEmail:
		fmt.Printf("[EMAIL] To: %s, Subject: %s, Body: %s\n", userID, notification.Title, notification.Message)
	case ChannelPush:
		fmt.Printf("[PUSH] User: %s, Title: %s, Body: %s\n", userID, notification.Title, notification.Message)
	case ChannelSMS:
		fmt.Printf("[SMS] User: %s, Message: %s: %s\n", userID, notification.Title, notification.Message)
	case ChannelDiscord:
		fmt.Printf("[DISCORD] User: %s, Title: %s, Body: %s\n", userID, notification.Title, notification.Message)
	case ChannelTelegram:
		fmt.Printf("[TELEGRAM] User: %s, Title: %s, Body: %s\n", userID, notification.Title, notification.Message)
	}
}

func (s *NotificationService) startPriceMonitoring() {
	s.priceCheckInterval = time.NewTicker(60 * time.Second)
	go func() {
		for range s.priceCheckInterval.C {
			s.checkPriceAlerts()
		}
	}()
}

func (s *NotificationService) checkPriceAlerts() {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Mock prices
	mockPrices := map[string]float64{
		"ETH": 2450.0,
		"BTC": 62500.0,
		"BNB": 310.0,
	}

	for _, alert := range s.priceAlerts {
		if alert.Triggered {
			continue
		}

		currentPrice, ok := mockPrices[alert.TokenSymbol]
		if !ok {
			continue
		}

		triggered := false
		switch alert.Condition {
		case "above":
			triggered = currentPrice >= alert.TargetPrice
		case "below":
			triggered = currentPrice <= alert.TargetPrice
		case "crosses":
			triggered = currentPrice == alert.TargetPrice
		}

		if triggered {
			alert.Triggered = true
			alert.TriggeredAt = time.Now().Unix()

			// Create notification
			notif := &Notification{
				Type:      TypePriceAlert,
				Title:     fmt.Sprintf("Price Alert: %s", alert.TokenSymbol),
				Message:   fmt.Sprintf("%s has reached $%.2f (target: $%.2f)", alert.TokenSymbol, currentPrice, alert.TargetPrice),
				Channels:  []NotificationChannel{ChannelInApp, ChannelPush},
				UserID:    alert.UserID,
			}
			s.SendNotification(alert.UserID, notif)
		}
	}
}

func main() {
	fmt.Println("TigerSwap Notification Service - Go")
	fmt.Println("==================================")
	fmt.Println()

	svc := NewNotificationService()

	// Send notification
	notif := &Notification{
		Type:    TypeSwapSuccess,
		Title:   "Swap Completed",
		Message: "Your swap of 1 ETH for 2450 USDT has been completed.",
		Channels: []NotificationChannel{ChannelInApp, ChannelPush, ChannelEmail},
		UserID:  "user_123",
	}

	result := svc.SendNotification("user_123", notif)
	fmt.Printf("Notification sent: %s\n", result.ID)
	fmt.Println()

	// Create price alert
	alert := svc.CreatePriceAlert("user_123", "0x...", "ETH", 2500.0, "above")
	fmt.Printf("Price alert created: %s\n", alert.ID)
	fmt.Println()

	// Get unread count
	count := svc.GetUnreadCount("user_123")
	fmt.Printf("Unread notifications: %d\n", count)
	fmt.Println()

	// Get preferences
	prefs := svc.GetPreferences("user_123")
	data, _ := json.MarshalIndent(prefs, "", "  ")
	fmt.Println("User Preferences:")
	fmt.Println(string(data))
}