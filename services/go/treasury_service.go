// ============================================================================
// TIGERSWAP TREASURY SERVICE
// Fee collection, revenue distribution, buyback engine, treasury analytics
// ============================================================================

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ============================================================================
// CONSTANTS
// ============================================================================

const (
	FeeCollectionInterval = 60 // seconds
	RevenueDistributionInterval = 3600 // seconds (hourly)
)

// ============================================================================
// MODELS
// ============================================================================

// Treasury represents the protocol treasury
type Treasury struct {
	ID                string            `json:"id"`
	TotalRevenue       float64         `json:"total_revenue"`
	TotalDistributed float64        `json:"total_distributed"`
	TotalBuyback     float64         `json:"total_buyback"`
	CurrentBalance   float64         `json:"current_balance"`
	AdminAddress     string          `json:"admin_address"`
	RevenueToken     string          `json:"revenue_token"`
	BuybackToken     string          `json:"buyback_token"`
	BuybackEnabled   bool           `json:"buyback_enabled"`
	DistributionEnabled bool       `json:"distribution_enabled"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// FeeDistribution represents fee distribution
type FeeDistribution struct {
	ID          string    `json:"id"`
	Type       string    `json:"type"` // swap, trading, bot, listing
	Amount     float64   `json:"amount"`
	Token      string   `json:"token"`
	ChainId    int      `json:"chain_id"`
	Recipient  string   `json:"recipient"`
	TxHash    string   `json:"tx_hash,omitempty"`
	Status    string   `json:"status"` // pending, completed, failed
	CreatedAt  time.Time `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// BuybackRecord represents buyback operation
type BuybackRecord struct {
	ID            string    `json:"id"`
	AmountUsed    float64  `json:"amount_used"`
	AmountBought float64  `json:"amount_bought"`
	TokenIn      string   `json:"token_in"`
	TokenOut     string   `json:"token_out"`
	Price       float64  `json:"price"`
	TxHash      string   `json:"tx_hash"`
	Status      string   `json:"status"`
	ExecutedAt   time.Time `json:"executed_at"`
}

// RevenueShare represents revenue sharing
type RevenueShare struct {
	ID          string    `json:"id"`
	Recipient   string   `json:"recipient"`
	ShareBasisPoints int    `json:"share_basis_points"` // 100 = 1%
	TotalReceived float64 `json:"total_received"`
	IsActive    bool     `json:"is_active"`
}

// TreasuryAnalytics represents treasury analytics
type TreasuryAnalytics struct {
	Period           string  `json:"period"` // daily, weekly, monthly
	TotalRevenue    float64 `json:"total_revenue"`
	TotalDistributed float64 `json:"total_distributed"`
	TotalBuyback   float64 `json:"total_buyback"`
	FeeBreakdown   map[string]float64 `json:"fee_breakdown"`
	TopRecipients []RecipientStats `json:"top_recipients"`
}

// RecipientStats represents recipient statistics
type RecipientStats struct {
	Recipient string  `json:"recipient"`
	Share     float64 `json:"share"`
	Received  float64 `json:"received"`
}

// ============================================================================
// TREASURY STORE
// ============================================================================

type TreasuryStore struct {
	mu sync.RWMutex

	treasury      *Treasury
	distributions []*FeeDistribution
	buybacks     []*BuybackRecord
	shares       map[string]*RevenueShare // recipient -> share

	// Fee tracking
	feeByType    map[string]float64
	feeByChain   map[int]float64
	feeByToken  map[string]float64

	// Admin fee addresses
	feeAddresses map[string]string // feeType -> address
}

// NewTreasuryStore creates new treasury store
func NewTreasuryStore() *TreasuryStore {
	return &TreasuryStore{
		distributions: make([]*FeeDistribution, 0),
		buybacks:     make([]*BuybackRecord, 0),
		shares:      make(map[string]*RevenueShare),
		feeByType:   make(map[string]float64),
		feeByChain:  make(map[int]float64),
		feeByToken: make(map[string]float64),
		feeAddresses: make(map[string]string),
	}
}

// ============================================================================
// TREASURY OPERATIONS
// ============================================================================

// InitializeTreasury initializes treasury
func (s *TreasuryStore) InitializeTreasury(adminAddress, revenueToken, buybackToken string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.treasury = &Treasury{
		ID:              generateUUID(),
		AdminAddress:     adminAddress,
		RevenueToken:    revenueToken,
		BuybackToken:    buybackToken,
		BuybackEnabled:  true,
		DistributionEnabled: true,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	return nil
}

// RecordFee records fee collection
func (s *TreasuryStore) RecordFee(feeType, token string, chainId int, amount float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.treasury == nil {
		return fmt.Errorf("treasury not initialized")
	}

	// Record fee
	s.feeByType[feeType] += amount
	s.feeByChain[chainId] += amount
	s.feeByToken[token] += amount

	// Update treasury
	s.treasury.TotalRevenue += amount
	s.treasury.CurrentBalance += amount
	s.treasury.UpdatedAt = time.Now()

	return nil
}

// CollectFees collects fees to admin address
func (s *TreasuryStore) CollectFees(feeType string) ([]*FeeDistribution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.treasury == nil {
		return nil, fmt.Errorf("treasury not initialized")
	}

	address, ok := s.feeAddresses[feeType]
	if !ok {
		address = s.treasury.AdminAddress
	}

	amount := s.feeByType[feeType]
	if amount <= 0 {
		return nil, nil
	}

	dist := &FeeDistribution{
		ID:         generateUUID(),
		Type:       feeType,
		Amount:     amount,
		Token:      "USD", // Default to stable
		ChainId:    1,    // Ethereum
		Recipient: address,
		Status:    "pending",
		CreatedAt: time.Now(),
	}

	s.distributions = append(s.distributions, dist)
	s.feeByType[feeType] = 0

	return []*FeeDistribution{dist}, nil
}

// SetRevenueShare sets revenue share for recipient
func (s *TreasuryStore) SetRevenueShare(recipient string, basisPoints int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.shares[recipient] = &RevenueShare{
		ID:               generateUUID(),
		Recipient:        recipient,
		ShareBasisPoints:  basisPoints,
		IsActive:         true,
	}

	return nil
}

// DistributeRevenue distributes revenue to recipients
func (s *TreasuryStore) DistributeRevenue() ([]*FeeDistribution, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.treasury == nil || !s.treasury.DistributionEnabled {
		return nil, fmt.Errorf("distribution disabled")
	}

	balance := s.treasury.CurrentBalance
	if balance <= 0 {
		return nil, nil
	}

	var distributions []*FeeDistribution

	for _, share := range s.shares {
		if !share.IsActive {
			continue
		}

		amount := balance * float64(share.ShareBasisPoints) / 10000

		dist := &FeeDistribution{
			ID:         generateUUID(),
			Type:       "revenue_share",
			Amount:     amount,
			Token:      s.treasury.RevenueToken,
			ChainId:    1,
			Recipient: share.Recipient,
			Status:    "pending",
			CreatedAt: time.Now(),
		}

		distributions = append(distributions, dist)
		share.TotalReceived += amount
		s.treasury.TotalDistributed += amount
		s.treasury.CurrentBalance -= amount
	}

	s.treasury.UpdatedAt = time.Now()

	return distributions, nil
}

// ExecuteBuyback executes buyback
func (s *TreasuryStore) ExecuteBuyback(amount float64) (*BuybackRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.treasury == nil || !s.treasury.BuybackEnabled {
		return nil, fmt.Errorf("buyback disabled")
	}

	if amount > s.treasury.CurrentBalance {
		return nil, fmt.Errorf("insufficient balance")
	}

	// Simplified - in production, integrate with DEXs
	record := &BuybackRecord{
		ID:            generateUUID(),
		AmountUsed:    amount,
		AmountBought: amount, // Simplified
		TokenIn:      s.treasury.BuybackToken,
		TokenOut:     s.treasury.RevenueToken,
		Price:       1.0, // Simplified
		Status:      "completed",
		ExecutedAt:  time.Now(),
	}

	s.buybacks = append(s.buybacks, record)
	s.treasury.TotalBuyback += amount
	s.treasury.CurrentBalance -= amount
	s.treasury.UpdatedAt = time.Now()

	return record, nil
}

// GetAnalytics gets treasury analytics
func (s *TreasuryStore) GetAnalytics(period string) *TreasuryAnalytics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	analytics := &TreasuryAnalytics{
		Period:           period,
		TotalRevenue:    s.treasury.TotalRevenue,
		TotalDistributed: s.treasury.TotalDistributed,
		TotalBuyback:    s.treasury.TotalBuyback,
		FeeBreakdown:    make(map[string]float64),
	}

	// Copy fee breakdown
	for k, v := range s.feeByType {
		analytics.FeeBreakdown[k] = v
	}

	// Top recipients
	for _, share := range s.shares {
		analytics.TopRecipients = append(analytics.TopRecipients, RecipientStats{
			Recipient: share.Recipient,
			Share:     float64(share.ShareBasisPoints) / 100,
			Received:  share.TotalReceived,
		})
	}

	return analytics
}

// GetTreasury gets treasury info
func (s *TreasuryStore) GetTreasury() *Treasury {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.treasury
}

// SetFeeAddress sets fee address
func (s *TreasuryStore) SetFeeAddress(feeType, address string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.feeAddresses[feeType] = address
	return nil
}

// ============================================================================
// HELPER FUNCTIONS
// ============================================================================

func generateUUID() string {
	return fmt.Sprintf("treasury_%d", time.Now().UnixNano())
}

// ============================================================================
// HTTP HANDLERS
// ============================================================================

type TreasuryHandler struct {
	store *TreasuryStore
}

func NewTreasuryHandler(store *TreasuryStore) *TreasuryHandler {
	return &TreasuryHandler{store: store}
}

// HandleGetTreasury handles get treasury request
func (h *TreasuryHandler) HandleGetTreasury(w http.ResponseWriter, r *http.Request) {
	treasury := h.store.GetTreasury()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(treasury)
}

// HandleCollectFee handles collect fee request
func (h *TreasuryHandler) HandleCollectFee(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FeeType string `json:"fee_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	distributions, err := h.store.CollectFees(req.FeeType)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(distributions)
}

// HandleGetAnalytics handles get analytics request
func (h *TreasuryHandler) HandleGetAnalytics(w http.ResponseWriter, r *http.Request) {
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "daily"
	}

	analytics := h.store.GetAnalytics(period)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(analytics)
}

// ============================================================================
// GLOBAL INSTANCE
// ============================================================================

var treasuryStore *TreasuryStore

func InitTreasury() {
	treasuryStore = NewTreasuryStore()
}

func GetTreasuryStore() *TreasuryStore {
	return treasuryStore
}