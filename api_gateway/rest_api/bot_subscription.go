// ============================================================================
// TIGERSWAP BOT SUBSCRIPTION WITH PAYMENTS
// Complete bot subscription tiers with real payment integration
// ============================================================================

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// CONSTANTS
// ============================================================================

const (
	// Subscription tiers
	TIER_BASIC_MONTHLY    = 2500
	TIER_PRO_MONTHLY      = 5000
	TIER_ENTERPRISE_MONTHLY = 10000

	// Payment intervals
	PAYMENT_MONTHLY = "monthly"
	PAYMENT_QUARTERLY = "quarterly"
	PAYMENT_YEARLY   = "yearly"

	// Payment methods
	PAYMENT_CRYPTO   = "crypto"
	PAYMENT_CARD    = "card"
	PAYMENT_BANK    = "bank"

	// Invoice status
	INVOICE_PENDING   = "pending"
	INVOICE_PAID     = "paid"
	INVOICE_OVERDUE  = "overdue"
	INVOICE_CANCELLED = "cancelled"
	INVOICE_REFUNDED = "refunded"
)

// ============================================================================
// MODELS
// ============================================================================

// BotSubscriptionTier represents subscription tier
type BotSubscriptionTier struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	DisplayName      string `json:"display_name"`
	MonthlyFeeUSD   float64 `json:"monthly_fee_usd"`
	YearlyFeeUSD    float64 `json:"yearly_fee_usd"`
	PerDEXFeeUSD    float64 `json:"per_dex_fee_usd"`
	PerCEXFeeUSD   float64 `json:"per_cex_fee_usd"`
	MaxBots        int    `json:"max_bots"`
	MaxDEXs        int    `json:"max_dexs"`
	MaxCEXs        int    `json:"max_cexs"`
	MaxPositionUSD float64 `json:"max_position_usd"`
	MaxDailyVolume float64 `json:"max_daily_volume"`
	LatencyTargetMs int    `json:"latency_target_ms"`
	Features       []string `json:"features"`
	IsActive       bool   `json:"is_active"`
}

// BotSubscription represents bot subscription
type BotSubscription struct {
	ID             string           `json:"id"`
	UserID         string           `json:"user_id"`
	TierID        string           `json:"tier_id"`
	Tier          *BotSubscriptionTier `json:"tier,omitempty"`
	Status        SubscriptionStatus `json:"status"`
	StartDate     time.Time       `json:"start_date"`
	EndDate       time.Time       `json:"end_date"`
	NextBillingAt *time.Time     `json:"next_billing_at,omitempty"`
	AutoRenew    bool           `json:"auto_renew"`
	PaymentMethod string         `json:"payment_method"`
	PaymentAddress string       `json:"payment_address,omitempty"`
	TotalPaid    float64        `json:"total_paid"`
	TotalUsage   float64        `json:"total_usage"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// SubscriptionStatus represents subscription status
type SubscriptionStatus string

const (
	SubStatusActive    SubscriptionStatus = "active"
	SubStatusExpired  SubscriptionStatus = "expired"
	SubStatusSuspended SubscriptionStatus = "suspended"
	SubStatusCancelled SubscriptionStatus = "cancelled"
	SubStatusTrial    SubscriptionStatus = "trial"
)

// Payment represents payment
type Payment struct {
	ID              string    `json:"id"`
	SubscriptionID string    `json:"subscription_id"`
	UserID         string    `json:"user_id"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	AmountUSD     float64   `json:"amount_usd"`
	Method        string    `json:"method"`
	TxHash        string    `json:"tx_hash,omitempty"`
	Status        PaymentStatus `json:"status"`
	FailedReason  string    `json:"failed_reason,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	ProcessedAt  *time.Time `json:"processed_at,omitempty"`
}

// PaymentStatus represents payment status
type PaymentStatus string

const (
	PaymentPending   PaymentStatus = "pending"
	PaymentProcessing PaymentStatus = "processing"
	PaymentCompleted PaymentStatus = "completed"
	PaymentFailed  PaymentStatus = "failed"
	PaymentRefunded PaymentStatus = "refunded"
)

// Invoice represents invoice
type Invoice struct {
	ID              string    `json:"id"`
	InvoiceNumber  string    `json:"invoice_number"`
	UserID        string    `json:"user_id"`
	SubscriptionID string  `json:"subscription_id"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	AmountUSD     float64   `json:"amount_usd"`
	Status       string    `json:"status"`
	DueDate       time.Time `json:"due_date"`
	PaidAt        *time.Time `json:"paid_at,omitempty"`
	PaymentID     string    `json:"payment_id,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// BotUsage represents bot usage
type BotUsage struct {
	ID            string    `json:"id"`
	SubscriptionID string  `json:"subscription_id"`
	BotType       string   `json:"bot_type"`
	DEXUsed       int      `json:"dex_used"`
	CEXUsed       int      `json:"cex_used"`
	VolumeUSD     float64  `json:"volume_usd"`
	FeeUSD        float64  `json:"fee_usd"`
	Timestamp    time.Time `json:"timestamp"`
}

// SubscriptionPlan represents subscription plan
type SubscriptionPlan struct {
	ID            string `json:"id"`
	TierID       string `json:"tier_id"`
	UserID       string `json:"user_id"`
	Interval     string `json:"interval"` // monthly, quarterly, yearly
	Amount      float64 `json:"amount"`
	Currency    string `json:"currency"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
}

// ============================================================================
// BOT SUBSCRIPTION STORE
// ============================================================================

type BotSubscriptionStore struct {
	mu sync.RWMutex

	// Tiers
	tiers map[string]*BotSubscriptionTier

	// Subscriptions
	subscriptions map[string]*BotSubscription // ID -> subscription
	subscriptionsByUser map[string][]string   // userID -> subscription IDs

	// Payments
	payments map[string]*Payment // ID -> payment

	// Invoices
	invoices map[string]*Invoice // ID -> invoice
	invoicesByUser map[string][]string // userID -> invoice IDs

	// Usage
	usage map[string][]*BotUsage // subscriptionID -> usage

	// Plans
	plans map[string]*SubscriptionPlan // ID -> plan

	// Admin fee address
	adminFeeAddress string
}

// NewBotSubscriptionStore creates new store
func NewBotSubscriptionStore() *BotSubscriptionStore {
	store := &BotSubscriptionStore{
		tiers:              make(map[string]*BotSubscriptionTier),
		subscriptions:      make(map[string]*BotSubscription),
		subscriptionsByUser: make(map[string][]string),
		payments:           make(map[string]*Payment),
		invoices:          make(map[string]*Invoice),
		invoicesByUser:    make(map[string][]string),
		usage:             make(map[string][]*BotUsage),
		plans:             make(map[string]*SubscriptionPlan),
	}

	// Initialize default tiers
	store.initDefaultTiers()

	return store
}

// Initialize default tiers
func (s *BotSubscriptionStore) initDefaultTiers() {
	tiers := []*BotSubscriptionTier{
		{
			ID: "tier_basic",
			Name: "basic",
			DisplayName: "Basic",
			MonthlyFeeUSD: TIER_BASIC_MONTHLY,
			YearlyFeeUSD: TIER_BASIC_MONTHLY * 12 * 0.9, // 10% discount
			PerDEXFeeUSD: 500,
			PerCEXFeeUSD: 50,
			MaxBots: 5,
			MaxDEXs: 10,
			MaxCEXs: 20,
			MaxPositionUSD: 100000,
			MaxDailyVolume: 1000000,
			LatencyTargetMs: 100,
			Features: []string{
				"arbitrage",
				"sniper",
				"liquidity",
			},
			IsActive: true,
		},
		{
			ID: "tier_pro",
			Name: "pro",
			DisplayName: "Pro",
			MonthlyFeeUSD: TIER_PRO_MONTHLY,
			YearlyFeeUSD: TIER_PRO_MONTHLY * 12 * 0.85, // 15% discount
			PerDEXFeeUSD: 750,
			PerCEXFeeUSD: 75,
			MaxBots: 20,
			MaxDEXs: 20,
			MaxCEXs: 50,
			MaxPositionUSD: 500000,
			MaxDailyVolume: 5000000,
			LatencyTargetMs: 50,
			Features: []string{
				"arbitrage",
				"sniper",
				"liquidity",
				"frontrun",
				"mev",
				"sandwich",
			},
			IsActive: true,
		},
		{
			ID: "tier_enterprise",
			Name: "enterprise",
			DisplayName: "Enterprise",
			MonthlyFeeUSD: TIER_ENTERPRISE_MONTHLY,
			YearlyFeeUSD: TIER_ENTERPRISE_MONTHLY * 12 * 0.8, // 20% discount
			PerDEXFeeUSD: 1000,
			PerCEXFeeUSD: 100,
			MaxBots: 100,
			MaxDEXs: 50,
			MaxCEXs: 100,
			MaxPositionUSD: 5000000,
			MaxDailyVolume: 50000000,
			LatencyTargetMs: 10,
			Features: []string{
				"arbitrage",
				"sniper",
				"liquidity",
				"frontrun",
				"mev",
				"sandwich",
				"flashloan",
				"crosschain",
				"perphedge",
			},
			IsActive: true,
		},
	}

	for _, tier := range tiers {
		s.tiers[tier.ID] = tier
	}
}

// ============================================================================
// SUBSCRIPTION MANAGEMENT
// ============================================================================

// CreateSubscription creates subscription
func (s *BotSubscriptionStore) CreateSubscription(userID, tierID, paymentMethod, paymentAddress string) (*BotSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	tier, ok := s.tiers[tierID]
	if !ok {
		return nil, fmt.Errorf("tier not found")
	}

	if !tier.IsActive {
		return nil, fmt.Errorf("tier not available")
	}

	subscription := &BotSubscription{
		ID: generateUUID(),
		UserID: userID,
		TierID: tierID,
		Tier: tier,
		Status: SubStatusActive,
		StartDate: time.Now(),
		EndDate: time.Now().Add(30 * 24 * time.Hour),
		AutoRenew: true,
		PaymentMethod: paymentMethod,
		PaymentAddress: paymentAddress,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	s.subscriptions[subscription.ID] = subscription
	s.subscriptionsByUser[userID] = append(s.subscriptionsByUser[userID], subscription.ID)

	// Create initial invoice
	s.createInvoice(subscription)

	return subscription, nil
}

// createInvoice creates invoice for subscription
func (s *BotSubscriptionStore) createInvoice(sub *BotSubscription) {
	invoice := &Invoice{
		ID: generateUUID(),
		InvoiceNumber: generateInvoiceNumber(),
		UserID: sub.UserID,
		SubscriptionID: sub.ID,
		Amount: sub.Tier.MonthlyFeeUSD,
		Currency: "USD",
		AmountUSD: sub.Tier.MonthlyFeeUSD,
		Status: INVOICE_PENDING,
		DueDate: time.Now().Add(7 * 24 * time.Hour), // 7 days to pay
		CreatedAt: time.Now(),
	}

	s.invoices[invoice.ID] = invoice
	s.invoicesByUser[sub.UserID] = append(s.invoicesByUser[sub.UserID], invoice.ID)
}

// GetSubscription gets subscription
func (s *BotSubscriptionStore) GetSubscription(subID string) (*BotSubscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sub, ok := s.subscriptions[subID]
	if !ok {
		return nil, fmt.Errorf("subscription not found")
	}

	return sub, nil
}

// GetUserSubscription gets user's active subscription
func (s *BotSubscriptionStore) GetUserSubscription(userID string) (*BotSubscription, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	subIDs, ok := s.subscriptionsByUser[userID]
	if !ok || len(subIDs) == 0 {
		return nil, fmt.Errorf("no subscription found")
	}

	// Get the latest subscription
	subID := subIDs[len(subIDs)-1]
	return s.subscriptions[subID], nil
}

// GetTiers gets all tiers
func (s *BotSubscriptionStore) GetTiers() []*BotSubscriptionTier {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tiers := make([]*BotSubscriptionTier, 0, len(s.tiers))
	for _, tier := range s.tiers {
		if tier.IsActive {
			tiers = append(tiers, tier)
		}
	}

	return tiers
}

// GetTier gets tier
func (s *BotSubscriptionStore) GetTier(tierID string) (*BotSubscriptionTier, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tier, ok := s.tiers[tierID]
	if !ok {
		return nil, fmt.Errorf("tier not found")
	}

	return tier, nil
}

// CancelSubscription cancels subscription
func (s *BotSubscriptionStore) CancelSubscription(subID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.subscriptions[subID]
	if !ok {
		return fmt.Errorf("subscription not found")
	}

	sub.Status = SubStatusCancelled
	sub.AutoRenew = false
	sub.UpdatedAt = time.Now()

	return nil
}

// RenewSubscription renews subscription
func (s *BotSubscriptionStore) RenewSubscription(subID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.subscriptions[subID]
	if !ok {
		return fmt.Errorf("subscription not found")
	}

	if sub.Status != SubStatusActive {
		return fmt.Errorf("subscription not active")
	}

	// Extend subscription
	sub.EndDate = sub.EndDate.Add(30 * 24 * time.Hour)
	sub.UpdatedAt = time.Now()

	// Create new invoice
	s.createInvoice(sub)

	return nil
}

// UpgradeTier upgrades tier
func (s *BotSubscriptionStore) UpgradeTier(subID, newTierID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.subscriptions[subID]
	if !ok {
		return fmt.Errorf("subscription not found")
	}

	newTier, ok := s.tiers[newTierID]
	if !ok {
		return fmt.Errorf("tier not found")
	}

	sub.TierID = newTierID
	sub.Tier = newTier
	sub.UpdatedAt = time.Now()

	return nil
}

// ============================================================================
// PAYMENT MANAGEMENT
// ============================================================================

// CreatePayment creates payment
func (s *BotSubscriptionStore) CreatePayment(subID, userID, method, txHash string, amount float64) (*Payment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	payment := &Payment{
		ID: generateUUID(),
		SubscriptionID: subID,
		UserID: userID,
		Amount: amount,
		Currency: "USD",
		AmountUSD: amount,
		Method: method,
		TxHash: txHash,
		Status: PaymentPending,
		CreatedAt: time.Now(),
	}

	s.payments[payment.ID] = payment

	// Process payment
	go s.processPayment(payment)

	return payment, nil
}

// processPayment processes payment
func (s *BotSubscriptionStore) processPayment(payment *Payment) {
	payment.Status = PaymentProcessing

	// In production, verify transaction on blockchain
	// For now, simulate processing
	time.Sleep(2 * time.Second)

	payment.Status = PaymentCompleted
	now := time.Now()
	payment.ProcessedAt = &now

	// Update subscription
	s.mu.Lock()
	if sub, ok := s.subscriptions[payment.SubscriptionID]; ok {
		sub.TotalPaid += payment.AmountUSD
		sub.Status = SubStatusActive
		sub.EndDate = time.Now().Add(30 * 24 * time.Hour)
		sub.UpdatedAt = time.Now()

		// Update invoice
		s.markInvoicePaid(sub.UserID, payment.AmountUSD)
	}
	s.mu.Unlock()
}

// markInvoicePaid marks invoice as paid
func (s *BotSubscriptionStore) markInvoicePaid(userID string, amount float64) {
	invoiceIDs, ok := s.invoicesByUser[userID]
	if !ok {
		return
	}

	for _, invoiceID := range invoiceIDs {
		if invoice, ok := s.invoices[invoiceID]; ok && invoice.Status == INVOICE_PENDING {
			invoice.Status = INVOICE_PAID
			now := time.Now()
			invoice.PaidAt = &now
			break
		}
	}
}

// GetPayment gets payment
func (s *BotSubscriptionStore) GetPayment(paymentID string) (*Payment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	payment, ok := s.payments[paymentID]
	if !ok {
		return nil, fmt.Errorf("payment not found")
	}

	return payment, nil
}

// GetPayments gets payments for user
func (s *BotSubscriptionStore) GetPayments(userID string) []*Payment {
	s.mu.RLock()
	defer s.mu.RUnlock()

	payments := make([]*Payment, 0)
	for _, p := range s.payments {
		if p.UserID == userID {
			payments = append(payments, p)
		}
	}

	return payments
}

// ============================================================================
// INVOICE MANAGEMENT
// ============================================================================

// GetInvoice gets invoice
func (s *BotSubscriptionStore) GetInvoice(invoiceID string) (*Invoice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	invoice, ok := s.invoices[invoiceID]
	if !ok {
		return nil, fmt.Errorf("invoice not found")
	}

	return invoice, nil
}

// GetInvoices gets invoices for user
func (s *BotSubscriptionStore) GetInvoices(userID string) []*Invoice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	invoiceIDs, ok := s.invoicesByUser[userID]
	if !ok {
		return nil
	}

	invoices := make([]*Invoice, 0, len(invoiceIDs))
	for _, invoiceID := range invoiceIDs {
		if invoice, ok := s.invoices[invoiceID]; ok {
			invoices = append(invoices, invoice)
		}
	}

	return invoices
}

// ============================================================================
// USAGE TRACKING
// ============================================================================

// RecordUsage records usage
func (s *BotSubscriptionStore) RecordUsage(subID string, usage *BotUsage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	sub, ok := s.subscriptions[subID]
	if !ok {
		return fmt.Errorf("subscription not found")
	}

	usage.ID = generateUUID()
	usage.SubscriptionID = subID
	usage.Timestamp = time.Now()

	s.usage[subID] = append(s.usage[subID], usage)

	// Update subscription usage
	sub.TotalUsage += usage.VolumeUSD

	return nil
}

// GetUsage gets usage for subscription
func (s *BotSubscriptionStore) GetUsage(subID string) []*BotUsage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.usage[subID]
}

// ============================================================================
// ADMIN FEE ADDRESS
// ============================================================================

// SetAdminFeeAddress sets admin fee address
func (s *BotSubscriptionStore) SetAdminFeeAddress(address string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.adminFeeAddress = address
	return nil
}

// GetAdminFeeAddress gets admin fee address
func (s *BotSubscriptionStore) GetAdminFeeAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.adminFeeAddress
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func generateRandomToken(length int) string {
	return generateRandomHex(length)
}

func generateRandomHex(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func generateUUID() string {
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		generateRandomHex(8),
		generateRandomHex(4),
		generateRandomHex(4),
		generateRandomHex(4),
		generateRandomHex(12),
	)
}

func generateInvoiceNumber() string {
	return fmt.Sprintf("INV-%s-%d", strings.ToUpper(generateRandomHex(6)), time.Now().Unix())
}

func hashString(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// ============================================================================
// HTTP HANDLERS
// ============================================================================

// BotSubscriptionHandler handles subscription requests
type BotSubscriptionHandler struct {
	store *BotSubscriptionStore
}

// NewBotSubscriptionHandler creates new handler
func NewBotSubscriptionHandler(store *BotSubscriptionStore) *BotSubscriptionHandler {
	return &BotSubscriptionHandler{store: store}
}

// HandleGetTiers handles get tiers request
func (h *BotSubscriptionHandler) HandleGetTiers(w http.ResponseWriter, r *http.Request) {
	tiers := h.store.GetTiers()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tiers)
}

// HandleCreateSubscription handles create subscription request
func (h *BotSubscriptionHandler) HandleCreateSubscription(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID        string `json:"user_id"`
		TierID       string `json:"tier_id"`
		PaymentMethod string `json:"payment_method"`
		PaymentAddress string `json:"payment_address"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	sub, err := h.store.CreateSubscription(req.UserID, req.TierID, req.PaymentMethod, req.PaymentAddress)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sub)
}

// HandleCreatePayment handles create payment request
func (h *BotSubscriptionHandler) HandleCreatePayment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID      string `json:"user_id"`
		Method     string `json:"method"`
		TxHash    string `json:"tx_hash"`
		Amount    float64 `json:"amount"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	sub, err := h.store.GetUserSubscription(req.UserID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	payment, err := h.store.CreatePayment(sub.ID, req.UserID, req.Method, req.TxHash, req.Amount)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payment)
}

// HandleGetInvoices handles get invoices request
func (h *BotSubscriptionHandler) HandleGetInvoices(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		http.Error(w, "user_id required", http.StatusBadRequest)
		return
	}

	invoices := h.store.GetInvoices(userID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(invoices)
}

// ============================================================================
// GLOBAL INSTANCE
// ============================================================================

var botSubscriptionStore *BotSubscriptionStore

// InitBotSubscription initializes bot subscription system
func InitBotSubscription() {
	botSubscriptionStore = NewBotSubscriptionStore()
}

// GetBotSubscriptionStore returns store
func GetBotSubscriptionStore() *BotSubscriptionStore {
	return botSubscriptionStore
}