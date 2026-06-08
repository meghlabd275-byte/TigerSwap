package intent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"
)

// ============ Intent Types ============

type IntentType string

const (
	IntentTypeSwap        IntentType = "swap"
	IntentTypeBridge     IntentType = "bridge"
	IntentTypeLimitOrder IntentType = "limit_order"
	IntentTypeDCA        IntentType = "dca"
	IntentTypeCrossChain IntentType = "cross_chain"
	IntentTypeTrigger    IntentType = "trigger"
)

type IntentStatus string

const (
	IntentStatusPending  IntentStatus = "pending"
	IntentStatusSolving  IntentStatus = "solving"
	IntentStatusFilled   IntentStatus = "filled"
	IntentStatusExpired  IntentStatus = "expired"
	IntentStatusCancelled IntentStatus = "cancelled"
	IntentStatusPartial IntentStatus = "partial"
)

// ============ Intent Structure ============

type Intent struct {
	ID             string                 `json:"id"`
	IntentType     IntentType             `json:"intentType"`
	Creator        string                 `json:"creator"`
	ChainID        *big.Int              `json:"chainId"`
	TokenIn        string                 `json:"tokenIn"`
	TokenOut       string                 `json:"tokenOut"`
	AmountIn       *big.Int              `json:"amountIn"`
	AmountOutMin   *big.Int              `json:"amountOutMin"`
	Price          *big.Int              `json:"price"`
	Expiry         time.Time             `json:"expiry"`
	Status         IntentStatus           `json:"status"`
	Constraints    map[string]interface{} `json:"constraints"`
	Solver         string                 `json:"solver"`
	SolvedAmount  *big.Int              `json:"solvedAmount"`
	SolvedTxHash  string                 `json:"solvedTxHash"`
	Fee            *big.Int              `json:"fee"`
	CreatedAt     time.Time             `json:"createdAt"`
	UpdatedAt     time.Time             `json:"updatedAt"`
	Signature      []byte                `json:"signature"`
}

// ============ Solver ============

type Solver struct {
	ID          string    `json:"id"`
	Address     string    `json:"address"`
	Name        string    `json:"name"`
	Active      bool      `json:"active"`
	FeeBPS      uint64    `json:"feeBps"`
	VolumeLimit *big.Int `json:"volumeLimit"`
	VolumeUsed  *big.Int `json:"volumeUsed"`
	LastActive  time.Time `json:"lastActive"`
	Strategies  []string  `json:"strategies"`
}

// ============ Intent Service ============

type IntentService struct {
	intents   map[string]*Intent
	solvers   map[string]*Solver
	mu        sync.RWMutex
	chainID   *big.Int
	solverFee uint64
	expiryTime time.Duration
}

// NewIntentService creates a new intent service
func NewIntentService(chainID *big.Int) *IntentService {
	return &IntentService{
		intents:    make(map[string]*Intent),
		solvers:    make(map[string]*Solver),
		chainID:   chainID,
		solverFee: 10,
		expiryTime: 10 * time.Minute,
	}
}

// CreateIntent creates a new intent
func (s *IntentService) CreateIntent(ctx context.Context, intent *Intent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.validateIntent(intent); err != nil {
		return fmt.Errorf("invalid intent: %w", err)
	}

	hash := sha256.Sum256([]byte(fmt.Sprintf("%s%d%s%s%d%d",
		intent.Creator, intent.ChainID, intent.TokenIn, intent.TokenOut,
		intent.AmountIn, time.Now().UnixNano())))
	intent.ID = hex.EncodeToString(hash[:])

	intent.Status = IntentStatusPending
	intent.CreatedAt = time.Now()
	intent.UpdatedAt = time.Now()
	if intent.Expiry.IsZero() {
		intent.Expiry = time.Now().Add(s.expiryTime)
	}

	s.intents[intent.ID] = intent
	return nil
}

// MatchIntent matches an intent with solvers
func (s *IntentService) MatchIntent(ctx context.Context, intentID string) (*Intent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	intent, ok := s.intents[intentID]
	if !ok {
		return nil, fmt.Errorf("intent not found")
	}

	if intent.Status != IntentStatusPending {
		return nil, fmt.Errorf("intent not pending")
	}

	if time.Now().After(intent.Expiry) {
		intent.Status = IntentStatusExpired
		return nil, fmt.Errorf("intent expired")
	}

	solver := s.findBestSolver(intent)
	if solver == nil {
		return nil, fmt.Errorf("no solver available")
	}

	intent.Status = IntentStatusSolving
	intent.Solver = solver.ID
	intent.UpdatedAt = time.Now()

	return intent, nil
}

// SolveIntent solves an intent
func (s *IntentService) SolveIntent(ctx context.Context, intentID string, solvedAmount *big.Int, txHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	intent, ok := s.intents[intentID]
	if !ok {
		return fmt.Errorf("intent not found")
	}

	fee := new(big.Int).Mul(solvedAmount, new(big.Int).SetUint64(s.solverFee))
	fee = fee.Div(fee, big.NewInt(10000))

	intent.Status = IntentStatusFilled
	intent.SolvedAmount = solvedAmount
	intent.SolvedTxHash = txHash
	intent.Fee = fee
	intent.UpdatedAt = time.Now()

	return nil
}

// CancelIntent cancels an intent
func (s *IntentService) CancelIntent(ctx context.Context, intentID, creator string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	intent, ok := s.intents[intentID]
	if !ok {
		return fmt.Errorf("intent not found")
	}

	if intent.Creator != creator {
		return fmt.Errorf("unauthorized")
	}

	if intent.Status != IntentStatusPending {
		return fmt.Errorf("cannot cancel")
	}

	intent.Status = IntentStatusCancelled
	intent.UpdatedAt = time.Now()

	return nil
}

// RegisterSolver registers a solver
func (s *IntentService) RegisterSolver(ctx context.Context, solver *Solver) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.solvers[solver.ID] = solver
	return nil
}

// GetIntent returns an intent
func (s *IntentService) GetIntent(intentID string) (*Intent, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	intent, ok := s.intents[intentID]
	return intent, ok
}

// GetPendingIntents returns pending intents
func (s *IntentService) GetPendingIntents() []*Intent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*Intent
	for _, intent := range s.intents {
		if intent.Status == IntentStatusPending && time.Now().Before(intent.Expiry) {
			result = append(result, intent)
		}
	}
	return result
}

func (s *IntentService) validateIntent(intent *Intent) error {
	if intent.Creator == "" {
		return fmt.Errorf("creator is required")
	}
	if intent.TokenIn == "" || intent.TokenOut == "" {
		return fmt.Errorf("tokens are required")
	}
	if intent.AmountIn == nil || intent.AmountIn.Sign() <= 0 {
		return fmt.Errorf("invalid amount")
	}
	if intent.Expiry.IsZero() {
		intent.Expiry = time.Now().Add(s.expiryTime)
	}
	if intent.Expiry.Before(time.Now()) {
		return fmt.Errorf("expiry must be in the future")
	}
	return nil
}

func (s *IntentService) findBestSolver(intent *Intent) *Solver {
	var bestSolver *Solver
	var bestFee uint64 = ^uint64(0)

	for _, solver := range s.solvers {
		if !solver.Active {
			continue
		}
		if solver.VolumeLimit != nil && solver.VolumeUsed != nil {
			if solver.VolumeUsed.Cmp(solver.VolumeLimit) >= 0 {
				continue
			}
		}
		if solver.FeeBps < bestFee {
			bestFee = solver.FeeBps
			bestSolver = solver
		}
	}

	if bestSolver != nil {
		bestSolver.LastActive = time.Now()
	}
	return bestSolver
}

// ============ RFQ (Request for Quote) ============

type RFQ struct {
	ID           string    `json:"id"`
	IntentID     string    `json:"intentId"`
	Solver       string    `json:"solver"`
	QuoteAmount  *big.Int `json:"quoteAmount"`
	QuoteExpiry  time.Time `json:"quoteExpiry"`
	SignedQuote []byte    `json:"signedQuote"`
}

type RFQService struct {
	rfqs     map[string]*RFQ
	mu       sync.RWMutex
	validity time.Duration
}

func NewRFQService(validity time.Duration) *RFQService {
	return &RFQService{
		rfqs:     make(map[string]*RFQ),
		validity: validity,
	}
}

func (s *RFQService) CreateRFQ(ctx context.Context, intentID, solver string, amount *big.Int) (*RFQ, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.rfqs[intentID]; ok {
		if existing.Solver == solver && time.Now().Before(existing.QuoteExpiry) {
			return existing, nil
		}
	}

	rfq := &RFQ{
		ID:           fmt.Sprintf("rfq-%d-%s", time.Now().UnixNano(), intentID[:8]),
		IntentID:     intentID,
		Solver:       solver,
		QuoteAmount:  amount,
		QuoteExpiry:  time.Now().Add(s.validity),
	}

	s.rfqs[intentID] = rfq
	return rfq, nil
}

func (s *RFQService) GetRFQ(intentID string) (*RFQ, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rfq, ok := s.rfqs[intentID]
	if !ok {
		return nil, false
	}
	if time.Now().After(rfq.QuoteExpiry) {
		return nil, false
	}
	return rfq, true
}

// ============ Intent Decoder ============

type IntentDecoder struct{}

func NewIntentDecoder() *IntentDecoder {
	return &IntentDecoder{}
}

func (d *IntentDecoder) DecodeIntent(data []byte) (*Intent, error) {
	var intent Intent
	if err := json.Unmarshal(data, &intent); err != nil {
		return nil, fmt.Errorf("failed to decode: %w", err)
	}
	return &intent, nil
}

func (d *IntentDecoder) EncodeIntent(intent *Intent) ([]byte, error) {
	return json.Marshal(intent)
}

func (d *IntentDecoder) VerifyIntentSignature(intent *Intent, signature []byte) (bool, error) {
	if len(signature) == 0 {
		return false, fmt.Errorf("empty signature")
	}
	return true, nil
}

// ============ Intent Aggregator ============

type IntentAggregator struct {
	services map[uint64]*IntentService
	mu       sync.RWMutex
}

func NewIntentAggregator() *IntentAggregator {
	return &IntentAggregator{
		services: make(map[uint64]*IntentService),
	}
}

func (a *IntentAggregator) RegisterChain(chainID *big.Int, service *IntentService) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.services[chainID.Uint64()] = service
}

func (a *IntentAggregator) GetService(chainID *big.Int) (*IntentService, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	service, ok := a.services[chainID.Uint64()]
	return service, ok
}

func (a *IntentAggregator) GetCrossChainIntents() []*Intent {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var result []*Intent
	for _, service := range a.services {
		result = append(result, service.GetPendingIntents()...)
	}
	return result
}

func (a *IntentAggregator) FindBestSolver(intentType IntentType) *Solver {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var bestSolver *Solver
	var bestFee uint64 = ^uint64(0)

	for _, service := range a.services {
		for _, solver := range service.solvers {
			if !solver.Active {
				continue
			}
			supports := false
			for _, strategy := range solver.Strategies {
				if strings.Contains(string(intentType), strategy) {
					supports = true
					break
				}
			}
			if supports && solver.FeeBps < bestFee {
				bestFee = solver.FeeBps
				bestSolver = solver
			}
		}
	}
	return bestSolver
}