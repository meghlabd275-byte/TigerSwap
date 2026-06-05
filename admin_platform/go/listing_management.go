// TigerSwap Listing Management - Go Implementation
// Enterprise token listing and trading pair management

package main

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ListingTier token listing tier
type ListingTier string

const (
	Tier1 ListingTier = "tier1" // Major pairs (BTC, ETH)
	Tier2 ListingTier = "tier2" // Established tokens
	Tier3 ListingTier = "tier3" // New tokens
	Tier4 ListingTier = "tier4" // Community tokens
)

// PairStatus trading pair status
type PairStatus string

const (
	PairPending   PairStatus = "pending"
	PairApproved  PairStatus = "approved"
	PairActive    PairStatus = "active"
	PairDelisted  PairStatus = "delisted"
	PairSuspended PairStatus = "suspended"
)

// ApplicationStatus listing application status
type ApplicationStatus string

const (
	AppSubmitted      ApplicationStatus = "submitted"
	AppDocPending     ApplicationStatus = "document_pending"
	AppAuditPending   ApplicationStatus = "audit_pending"
	AppApproved       ApplicationStatus = "approved"
	AppRejected       ApplicationStatus = "rejected"
	AppFeePending     ApplicationStatus = "fee_pending"
)

// TokenInfo token information
type TokenInfo struct {
	Address    string  `json:"address"`
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name"`
	Decimals   int     `json:"decimals"`
	Logo       string  `json:"logo"`
	ChainID    int64   `json:"chainId"`
	IsNative   bool    `json:"isNative"`
	IsStable   bool    `json:"isStable"`
	TotalSupply string `json:"totalSupply"`
	Holders    int     `json:"holders"`
	MarketCap  string  `json:"marketCap"`
	Website    string  `json:"website,omitempty"`
	Twitter    string  `json:"twitter,omitempty"`
	Telegram   string  `json:"telegram,omitempty"`
}

// TradingPair a trading pair
type TradingPair struct {
	ID            string       `json:"id"`
	PairName      string       `json:"pairName"`
	BaseToken     *TokenInfo   `json:"baseToken"`
	QuoteToken    *TokenInfo   `json:"quoteToken"`
	ChainID       int64        `json:"chainId"`
	DEX           string       `json:"dex"`
	PoolAddress   string       `json:"poolAddress"`
	LPTokenAddress string      `json:"lpTokenAddress"`
	CreatedAt     int64        `json:"createdAt"`
	CreatedBy     string       `json:"createdBy"`
	ListingFee    string       `json:"listingFee"`
	TradingFee    string       `json:"tradingFee"`
	Status        PairStatus   `json:"status"`
	Price         string       `json:"price"`
	PriceChange24h float64     `json:"priceChange24h"`
	Volume24h     string       `json:"volume24h"`
	Liquidity     string       `json:"liquidity"`
	IsStablePair  bool         `json:"isStablePair"`
	IsFeatured    bool         `json:"isFeatured"`
	Tier          ListingTier  `json:"tier"`
	Metadata      PairMetadata `json:"metadata"`
}

// PairMetadata pair metadata
type PairMetadata struct {
	Category     string `json:"category"`
	Tags         []string `json:"tags"`
	Warning      string `json:"warning,omitempty"`
	AuditStatus  string `json:"auditStatus,omitempty"`
	KYCStatus    string `json:"kycStatus,omitempty"`
}

// ListingApplication a listing application
type ListingApplication struct {
	ID            string           `json:"id"`
	ApplicantAddress string         `json:"applicantAddress"`
	Token         *TokenInfo       `json:"token"`
	BaseToken     string           `json:"baseToken"`
	ListingTier   ListingTier      `json:"listingTier"`
	RequestedDEX  []string         `json:"requestedDex"`
	ListingFee    string           `json:"listingFee"`
	TradingFee    string           `json:"tradingFee"`
	Status        ApplicationStatus `json:"status"`
	SubmittedAt   int64            `json:"submittedAt"`
	ReviewedAt    int64            `json:"reviewedAt,omitempty"`
	ReviewedBy    string           `json:"reviewedBy,omitempty"`
	Notes         string          `json:"notes,omitempty"`
}

// FeeConfig listing fee configuration
type FeeConfig struct {
	ListingFee       string  `json:"listingFee"`
	ListingFeeUSD    string  `json:"listingFeeUsd"`
	TradingFee       string  `json:"tradingFee"`
	TradingFeeMaker  string  `json:"tradingFeeMaker"`
	TradingFeeTaker  string  `json:"tradingFeeTaker"`
	lpRewardFee      string  `json:"lpRewardFee"`
	WithdrawalFee    string  `json:"withdrawalFee"`
	DepositFee       string  `json:"depositFee"`
	StablePairDiscount float64 `json:"stablePairDiscount"`
}

// TokenListingManager manages token listings
type TokenListingManager struct {
	mu         sync.RWMutex
	pairs      map[string]*TradingPair
	applications map[string]*ListingApplication
	feeConfig  FeeConfig
}

func NewTokenListingManager() *TokenListingManager {
	m := &TokenListingManager{
		pairs:        make(map[string]*TradingPair),
		applications: make(map[string]*ListingApplication),
	}
	m.initializeDefaultPairs()
	m.initializeDefaultFees()
	return m
}

func (m *TokenListingManager) initializeDefaultFees() {
	m.feeConfig = FeeConfig{
		ListingFee:       "1000",
		ListingFeeUSD:    "500",
		TradingFee:       "0.25",
		TradingFeeMaker:  "0.20",
		TradingFeeTaker:  "0.30",
		lpRewardFee:      "0.02",
		WithdrawalFee:    "0.001",
		DepositFee:       "0",
		StablePairDiscount: 50,
	}
}

func (m *TokenListingManager) initializeDefaultPairs() {
	defaultPairs := []*TradingPair{
		{
			ID:          "pair_eth_usdt",
			PairName:    "ETH/USDT",
			ChainID:     1,
			DEX:         "uniswap",
			Status:      PairActive,
			Price:       "2450.50",
			PriceChange24h: 2.5,
			Volume24h:  "125000000",
			Liquidity:   "50000000",
			IsFeatured: true,
			Tier:       Tier1,
			Metadata:   PairMetadata{Category: "Major", Tags: []string{"blue-chip", "high-liquidity"}, AuditStatus: "passed", KYCStatus: "verified"},
		},
		{
			ID:          "pair_btc_usdt",
			PairName:    "BTC/USDT",
			ChainID:     1,
			DEX:         "uniswap",
			Status:      PairActive,
			Price:       "62500",
			PriceChange24h: 1.2,
			Volume24h:  "250000000",
			Liquidity:   "100000000",
			IsFeatured: true,
			Tier:       Tier1,
			Metadata:   PairMetadata{Category: "Major", Tags: []string{"blue-chip", "high-liquidity"}, AuditStatus: "passed", KYCStatus: "verified"},
		},
		{
			ID:          "pair_bnb_usdt",
			PairName:    "BNB/USDT",
			ChainID:     56,
			DEX:         "pancakeswap",
			Status:      PairActive,
			Price:       "310.25",
			PriceChange24h: 3.1,
			Volume24h:  "80000000",
			Liquidity:   "35000000",
			IsFeatured: true,
			Tier:       Tier1,
			Metadata:   PairMetadata{Category: "Major", Tags: []string{"blue-chip", "high-liquidity"}, AuditStatus: "passed", KYCStatus: "verified"},
		},
	}

	for _, pair := range defaultPairs {
		pair.CreatedAt = time.Now().Unix()
		pair.CreatedBy = "system"
		pair.ListingFee = m.feeConfig.ListingFee
		pair.TradingFee = m.feeConfig.TradingFee
		m.pairs[pair.ID] = pair
	}
}

// CreateTradingPair creates a new trading pair
func (m *TokenListingManager) CreateTradingPair(baseToken, quoteToken *TokenInfo, dex, createdBy string) *TradingPair {
	m.mu.Lock()
	defer m.mu.Unlock()

	pair := &TradingPair{
		ID:            fmt.Sprintf("pair_%d", time.Now().UnixNano()),
		PairName:      fmt.Sprintf("%s/%s", baseToken.Symbol, quoteToken.Symbol),
		BaseToken:     baseToken,
		QuoteToken:    quoteToken,
		ChainID:       baseToken.ChainID,
		DEX:           dex,
		PoolAddress:   generateAddress(),
		LPTokenAddress: generateAddress(),
		CreatedAt:     time.Now().Unix(),
		CreatedBy:     createdBy,
		ListingFee:    m.feeConfig.ListingFee,
		TradingFee:    m.feeConfig.TradingFee,
		Status:        PairPending,
		IsStablePair:  quoteToken.IsStable,
		Tier:          determineTier(baseToken),
		Metadata:      PairMetadata{Category: "General", Tags: []string{}},
	}

	m.pairs[pair.ID] = pair
	return pair
}

// GetAllPairs returns all pairs
func (m *TokenListingManager) GetAllPairs() []*TradingPair {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*TradingPair, 0, len(m.pairs))
	for _, pair := range m.pairs {
		result = append(result, pair)
	}
	return result
}

// GetActivePairs returns active pairs
func (m *TokenListingManager) GetActivePairs() []*TradingPair {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*TradingPair, 0)
	for _, pair := range m.pairs {
		if pair.Status == PairActive {
			result = append(result, pair)
		}
	}
	return result
}

// GetFeaturedPairs returns featured pairs
func (m *TokenListingManager) GetFeaturedPairs() []*TradingPair {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*TradingPair, 0)
	for _, pair := range m.pairs {
		if pair.IsFeatured && pair.Status == PairActive {
			result = append(result, pair)
		}
	}
	return result
}

// ApprovePair approves a trading pair
func (m *TokenListingManager) ApprovePair(pairID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	pair, ok := m.pairs[pairID]
	if !ok {
		return false
	}

	pair.Status = PairActive
	return true
}

// SuspendPair suspends a trading pair
func (m *TokenListingManager) SuspendPair(pairID, reason string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	pair, ok := m.pairs[pairID]
	if !ok {
		return false
	}

	pair.Status = PairSuspended
	return true
}

// SetFeatured sets featured status
func (m *TokenListingManager) SetFeatured(pairID string, featured bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	pair, ok := m.pairs[pairID]
	if !ok {
		return false
	}

	pair.IsFeatured = featured
	return true
}

// SubmitListingApplication submits a listing application
func (m *TokenListingManager) SubmitListingApplication(applicant string, token *TokenInfo) *ListingApplication {
	m.mu.Lock()
	defer m.mu.Unlock()

	app := &ListingApplication{
		ID:               fmt.Sprintf("app_%d", time.Now().UnixNano()),
		ApplicantAddress: applicant,
		Token:           token,
		BaseToken:       "USDT",
		ListingTier:     Tier3,
		RequestedDEX:    []string{"tigerswap"},
		ListingFee:      m.feeConfig.ListingFee,
		TradingFee:      m.feeConfig.TradingFee,
		Status:          AppSubmitted,
		SubmittedAt:     time.Now().Unix(),
	}

	m.applications[app.ID] = app
	return app
}

// GetFeeConfig returns fee configuration
func (m *TokenListingManager) GetFeeConfig() FeeConfig {
	return m.feeConfig
}

// UpdateListingFee updates listing fee
func (m *TokenListingManager) UpdateListingFee(fee, feeUSD string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.feeConfig.ListingFee = fee
	m.feeConfig.ListingFeeUSD = feeUSD
}

// GetPair returns a pair by ID
func (m *TokenListingManager) GetPair(pairID string) *TradingPair {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pairs[pairID]
}

// SearchPairs searches pairs by name or symbol
func (m *TokenListingManager) SearchPairs(query string) []*TradingPair {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queryLower := toLower(query)
	result := make([]*TradingPair, 0)

	for _, pair := range m.pairs {
		if contains(toLower(pair.PairName), queryLower) ||
		   contains(toLower(pair.BaseToken.Symbol), queryLower) {
			result = append(result, pair)
		}
	}

	return result
}

func determineTier(token *TokenInfo) ListingTier {
	if token.Holders > 100000 {
		return Tier1
	}
	if token.Holders > 10000 {
		return Tier2
	}
	if token.Holders > 1000 {
		return Tier3
	}
	return Tier4
}

func generateAddress() string {
	return fmt.Sprintf("0x%x", time.Now().UnixNano())
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func main() {
	fmt.Println("TigerSwap Listing Management Service - Go")
	fmt.Println("=========================================")
	fmt.Println()

	mgr := NewTokenListingManager()

	fmt.Printf("Initialized with %d pairs\n", len(mgr.GetAllPairs()))
	fmt.Println()

	// List featured pairs
	pairs := mgr.GetFeaturedPairs()
	fmt.Println("Featured Pairs:")
	for _, pair := range pairs {
		fmt.Printf("  - %s: $%s (24h: %.2f%%)\n", pair.PairName, pair.Price, pair.PriceChange24h)
	}
	fmt.Println()

	// Test search
	results := mgr.SearchPairs("ETH")
	fmt.Printf("Search 'ETH' results: %d pairs\n", len(results))

	// ToJSON
	data, _ := json.MarshalIndent(mgr.GetFeeConfig(), "", "  ")
	fmt.Println("\nFee Config:")
	fmt.Println(string(data))
}