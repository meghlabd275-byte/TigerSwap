package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// OrderService - Handles order management
type OrderService struct {
	orders    map[string]*Order
	accounts map[string]*Account
}

// Order types
type OrderType string
type OrderSide string
type OrderStatus string

const (
	OrderTypeMarket  OrderType = "market"
	OrderTypeLimit  OrderType = "limit"
	OrderTypeStop   OrderType = "stop"
	
	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"
	
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusOpen    OrderStatus = "open"
	OrderStatusPartial OrderStatus = "partial"
	OrderStatusFilled  OrderStatus = "filled"
	OrderStatusCancelled OrderStatus = "cancelled"
)

// Order represents a trading order
type Order struct {
	ID            string                 `json:"id"`
	UserID        string                 `json:"user_id"`
	Symbol        string                 `json:"symbol"`
	Type         OrderType              `json:"type"`
	Side         OrderSide              `json:"side"`
	Quantity     float64                `json:"quantity"`
	Price        float64                `json:"price"`
	FilledQty    float64                `json:"filled_qty"`
	AvgFillPrice float64                `json:"avg_fill_price"`
	Status       OrderStatus            `json:"status"`
	CreatedAt    int64                  `json:"created_at"`
	UpdatedAt    int64                  `json:"updated_at"`
	ExpiresAt    int64                  `json:"expires_at"`
	Metadata     map[string]interface{} `json:"metadata"`
}

// Account represents a trading account
type Account struct {
	UserID       string             `json:"user_id"`
	Balances    map[string]float64 `json:"balances"`
	OpenOrders  []string           `json:"open_orders"`
	Positions  map[string]float64 `json:"positions"`
	MarginUsed float64            `json:"margin_used"`
}

// NewOrderService creates a new order service
func NewOrderService() *OrderService {
	return &OrderService{
		orders:    make(map[string]*Order),
		accounts: make(map[string]*Account),
	}
}

// CreateOrder creates a new order
func (s *OrderService) CreateOrder(order *Order) error {
	order.ID = generateOrderID()
	order.Status = OrderStatusOpen
	order.CreatedAt = time.Now().Unix()
	order.UpdatedAt = time.Now().Unix()
	
	// Validate order
	if order.Quantity <= 0 {
		return fmt.Errorf("invalid quantity")
	}
	
	if order.Type == OrderTypeLimit && order.Price <= 0 {
		return fmt.Errorf("invalid price")
	}
	
	// Check account balance
	account, ok := s.accounts[order.UserID]
	if !ok {
		return fmt.Errorf("account not found")
	}
	
	// Reserve balance
	requiredBalance := order.Quantity * order.Price
	if order.Side == OrderSideBuy {
		if account.Balances[order.Symbol] < requiredBalance {
			return fmt.Errorf("insufficient balance")
		}
		account.Balances[order.Symbol] -= requiredBalance
		account.MarginUsed += requiredBalance
	}
	
	s.orders[order.ID] = order
	account.OpenOrders = append(account.OpenOrders, order.ID)
	
	return nil
}

// FillOrder partially or fully fills an order
func (s *OrderService) FillOrder(orderID string, fillQty float64, price float64) error {
	order, ok := s.orders[orderID]
	if !ok {
		return fmt.Errorf("order not found")
	}
	
	if order.Status == OrderStatusFilled || order.Status == OrderStatusCancelled {
		return fmt.Errorf("order not active")
	}
	
	order.FilledQty += fillQty
	order.AvgFillPrice = ((order.AvgFillPrice * (order.FilledQty - fillQty)) + (price * fillQty)) / order.FilledQty
	order.UpdatedAt = time.Now().Unix()
	
	if order.FilledQty >= order.Quantity {
		order.Status = OrderStatusFilled
	} else {
		order.Status = OrderStatusPartial
	}
	
	// Update account balances
	account, ok := s.accounts[order.UserID]
	if !ok {
		return fmt.Errorf("account not found")
	}
	
	// Remove from open orders if filled
	if order.Status == OrderStatusFilled {
		for i, id := range account.OpenOrders {
			if id == orderID {
				account.OpenOrders = append(account.OpenOrders[:i], account.OpenOrders[i+1:]...)
				break
			}
		}
		account.MarginUsed -= order.Quantity * order.Price
	}
	
	// Update position
	position := account.Positions[order.Symbol]
	if order.Side == OrderSideBuy {
		position += fillQty
	} else {
		position -= fillQty
	}
	account.Positions[order.Symbol] = position
	
	return nil
}

// CancelOrder cancels an order
func (s *OrderService) CancelOrder(orderID string, userID string) error {
	order, ok := s.orders[orderID]
	if !ok {
		return fmt.Errorf("order not found")
	}
	
	if order.UserID != userID {
		return fmt.Errorf("unauthorized")
	}
	
	if order.Status == OrderStatusFilled || order.Status == OrderStatusCancelled {
		return fmt.Errorf("order not active")
	}
	
	order.Status = OrderStatusCancelled
	order.UpdatedAt = time.Now().Unix()
	
	// Return reserved balance
	account, ok := s.accounts[userID]
	if ok {
		for i, id := range account.OpenOrders {
			if id == orderID {
				account.OpenOrders = append(account.OpenOrders[:i], account.OpenOrders[i+1:]...)
				break
			}
		}
		account.MarginUsed -= order.Quantity * order.Price
	}
	
	return nil
}

// GetOrder returns an order by ID
func (s *OrderService) GetOrder(orderID string) (*Order, error) {
	order, ok := s.orders[orderID]
	if !ok {
		return nil, fmt.Errorf("order not found")
	}
	return order, nil
}

// GetUserOrders returns all orders for a user
func (s *OrderService) GetUserOrders(userID string) []*Order {
	var userOrders []*Order
	for _, order := range s.orders {
		if order.UserID == userID {
			userOrders = append(userOrders, order)
		}
	}
	return userOrders
}

// GetOpenOrders returns all open orders for a user
func (s *OrderService) GetOpenOrders(userID string) []*Order {
	var openOrders []*Order
	account, ok := s.accounts[userID]
	if !ok {
		return openOrders
	}
	
	for _, orderID := range account.OpenOrders {
		order, ok := s.orders[orderID]
		if ok {
			openOrders = append(openOrders, order)
		}
	}
	return openOrders
}

// CreateAccount creates a new trading account
func (s *OrderService) CreateAccount(userID string, balances map[string]float64) error {
	account := &Account{
		UserID:      userID,
		Balances:   balances,
		OpenOrders: []string{},
		Positions: make(map[string]float64),
	}
	s.accounts[userID] = account
	return nil
}

// GetAccount returns an account
func (s *OrderService) GetAccount(userID string) (*Account, error) {
	account, ok := s.accounts[userID]
	if !ok {
		return nil, fmt.Errorf("account not found")
	}
	return account, nil
}

func generateOrderID() string {
	return fmt.Sprintf("ORD%d%d", time.Now().Unix(), time.Now().Nanosecond())
}

func main() {
	fmt.Println("Order Service initialized")
}