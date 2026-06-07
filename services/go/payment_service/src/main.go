package main

import (
	"encoding/json"
	"fmt"
	"time"
)

// PaymentService - Handles payments and settlements
type PaymentService struct {
	transactions map[string]*Transaction
	payments    map[string]*Payment
}

// Transaction represents a payment transaction
type Transaction struct {
	ID          string                 `json:"id"`
	UserID     string                 `json:"user_id"`
	Type       string                 `json:"type"`
	Amount    float64                `json:"amount"`
	Currency  string                 `json:"currency"`
	Status    string                 `json:"status"`
	Method    string                 `json:"method"`
	Reference string                 `json:"reference"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt int64                  `json:"created_at"`
	UpdatedAt int64                  `json:"updated_at"`
}

// Payment methods
const (
	MethodBankTransfer = "bank_transfer"
	MethodCreditCard = "credit_card"
	MethodCrypto     = "crypto"
	MethodStable    = "stable"
)

// Transaction statuses
const (
	StatusPending   = "pending"
	StatusProcessing = "processing"
	StatusCompleted = "completed"
	StatusFailed   = "failed"
	StatusRefunded = "refunded"
)

// NewPaymentService creates a new payment service
func NewPaymentService() *PaymentService {
	return &PaymentService{
		transactions: make(map[string]*Transaction),
		payments:    make(map[string]*Payment),
	}
}

// CreateTransaction creates a new transaction
func (s *PaymentService) CreateTransaction(tx *Transaction) error {
	tx.ID = generateTransactionID()
	tx.Status = StatusPending
	tx.CreatedAt = time.Now().Unix()
	tx.UpdatedAt = time.Now().Unix()
	
	// Validate transaction
	if tx.Amount <= 0 {
		return fmt.Errorf("invalid amount")
	}
	
	if tx.Currency == "" {
		return fmt.Errorf("currency required")
	}
	
	s.transactions[tx.ID] = tx
	return nil
}

// ProcessTransaction processes a transaction
func (s *PaymentService) ProcessTransaction(txID string) error {
	tx, ok := s.transactions[txID]
	if !ok {
		return fmt.Errorf("transaction not found")
	}
	
	if tx.Status != StatusPending {
		return fmt.Errorf("transaction not pending")
	}
	
	tx.Status = StatusProcessing
	tx.UpdatedAt = time.Now().Unix()
	
	// Simulate processing
	switch tx.Method {
	case MethodBankTransfer:
		// Would integrate with banking API
		tx.Status = StatusCompleted
	case MethodCreditCard:
		// Would integrate with payment processor
		tx.Status = StatusCompleted
	case MethodCrypto:
		// Would wait for confirmations
		tx.Status = StatusCompleted
	case MethodStable:
		// Instant settlement
		tx.Status = StatusCompleted
	}
	
	tx.UpdatedAt = time.Now().Unix()
	return nil
}

// RefundTransaction refunds a transaction
func (s *PaymentService) RefundTransaction(txID string) error {
	tx, ok := s.transactions[txID]
	if !ok {
		return fmt.Errorf("transaction not found")
	}
	
	if tx.Status != StatusCompleted {
		return fmt.Errorf("transaction not completed")
	}
	
	tx.Status = StatusRefunded
	tx.UpdatedAt = time.Now().Unix()
	return nil
}

// GetTransaction returns a transaction
func (s *PaymentService) GetTransaction(txID string) (*Transaction, error) {
	tx, ok := s.transactions[txID]
	if !ok {
		return nil, fmt.Errorf("transaction not found")
	}
	return tx, nil
}

// GetUserTransactions returns all transactions for a user
func (s *PaymentService) GetUserTransactions(userID string) []*Transaction {
	var userTxs []*Transaction
	for _, tx := range s.transactions {
		if tx.UserID == userID {
			userTxs = append(userTxs, tx)
		}
	}
	return userTxs
}

// GetTransactionStats returns transaction statistics
func (s *PaymentService) GetTransactionStats(userID string) map[string]interface{} {
	stats := map[string]interface{}{
		"total":      0,
		"completed":  0,
		"pending":    0,
		"failed":     0,
		"volume":     0.0,
	}
	
	for _, tx := range s.transactions {
		if tx.UserID == userID {
			stats["total"].(int)++
			
			switch tx.Status {
			case StatusCompleted:
				stats["completed"].(int)++
			case StatusPending, StatusProcessing:
				stats["pending"].(int)++
			case StatusFailed:
				stats["failed"].(int)++
			}
			
			if tx.Status == StatusCompleted {
				stats["volume"].(float64) += tx.Amount
			}
		}
	}
	
	return stats
}

// Payment represents a payment record
type Payment struct {
	ID        string  `json:"id"`
	TxID     string  `json:"tx_id"`
	UserID   string  `json:"user_id"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Status   string  `json:"status"`
}

func generateTransactionID() string {
	return fmt.Sprintf("TX%d%d", time.Now().Unix(), time.Now().Nanosecond())
}

func main() {
	fmt.Println("Payment Service initialized")
}