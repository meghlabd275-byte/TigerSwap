package orchestrator

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"TigerSwap/services/dca_service/config"
	"TigerSwap/services/trading_engine/router"
)

// DCAPlan represents a DCA plan
type DCAPlan struct {
	ID                uint64
	Owner             common.Address
	TokenIn           common.Address
	TokenOut          common.Address
	AmountInPerExec   *big.Int
	Interval         time.Duration
	NextExecTime     time.Time
	ExecutionsDone   uint64
	MaxExecutions    uint64
	Status           string
}

// DCAOrchestrator orchestrates DCA plan execution
type DCAOrchestrator struct {
	cfg     *config.Config
	client *ethclient.Client
	router *router.DEXRouter
	plans  map[uint64]*DCAPlan
}

// NewDCAOrchestrator creates a new DCA orchestrator
func NewDCAOrchestrator(cfg *config.Config, client *ethclient.Client) (*DCAOrchestrator, error) {
	router, err := router.NewDEXRouter(&config.Config{
		RPCURL: cfg.RPCURL,
		DEXs:   cfg.DEXs,
	})
	if err != nil {
		return nil, err
	}

	return &DCAOrchestrator{
		cfg:    cfg,
		client: client,
		router: router,
		plans: make(map[uint64]*DCAPlan),
	}, nil
}

// LoadPlans loads active DCA plans
func (o *DCAOrchestrator) LoadPlans(ctx context.Context) error {
	// In production, load from database or contract events
	return nil
}

// ExecuteDuePlans executes all due DCA plans
func (o *DCAOrchestrator) ExecuteDuePlans(ctx context.Context) {
	now := time.Now()

	for _, plan := range o.plans {
		if plan.Status != "Active" {
			continue
		}

		if now.Before(plan.NextExecTime) {
			continue
		}

		// Execute plan
		if err := o.executePlan(ctx, plan); err != nil {
			fmt.Printf("[DCA] Failed to execute plan %d: %v\n", plan.ID, err)
			continue
		}

		fmt.Printf("[DCA] Executed plan %d\n", plan.ID)
	}
}

// executePlan executes a single DCA plan
func (o *DCAOrchestrator) executePlan(ctx context.Context, plan *DCAPlan) error {
	// Get quote
	amountOut, err := o.router.GetQuote(
		plan.TokenIn.String(),
		plan.TokenOut.String(),
		plan.AmountInPerExec,
	)
	if err != nil {
		return fmt.Errorf("failed to get quote: %w", err)
	}

	// Apply slippage
	minOut := new(big.Int).Mul(amountOut, big.NewInt(int64(10000-o.cfg.MaxSlippageBPS)))
	minOut = new(big.Int).Div(minOut, big.NewInt(10000))

	// Execute swap
	txHash, err := o.router.ExecuteSwap(
		plan.TokenIn.String(),
		plan.TokenOut.String(),
		plan.AmountInPerExec,
		minOut,
		plan.Owner.String(),
	)
	if err != nil {
		return fmt.Errorf("failed to execute swap: %w", err)
	}

	// Wait for confirmation
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	receipt, err := o.client.TransactionReceipt(ctx, common.HexToHash(txHash))
	if err != nil {
		return fmt.Errorf("failed to get receipt: %w", err)
	}

	if receipt.Status != 1 {
		return fmt.Errorf("transaction failed")
	}

	// Update plan
	plan.ExecutionsDone++
	plan.NextExecTime = time.Now().Add(plan.Interval)

	// Check if completed
	if plan.MaxExecutions > 0 && plan.ExecutionsDone >= plan.MaxExecutions {
		plan.Status = "Completed"
	}

	return nil
}

// CheckPlans checks plan conditions
func (o *DCAOrchestrator) CheckPlans(ctx context.Context) {
	for _, plan := range o.plans {
		if plan.Status != "Active" {
			continue
		}

		// Check price conditions
		// In production, check price triggers
	}
}

// AddPlan adds a new DCA plan
func (o *DCAOrchestrator) AddPlan(plan *DCAPlan) {
	o.plans[plan.ID] = plan
}

// RemovePlan removes a DCA plan
func (o *DCAOrchestrator) RemovePlan(planID uint64) {
	delete(o.plans, planID)
}

// GetPlan returns a plan
func (o *DCAOrchestrator) GetPlan(planID uint64) (*DCAPlan, bool) {
	plan, ok := o.plans[planID]
	return plan, ok
}

// GetPlans returns all plans
func (o *DCAOrchestrator) GetPlans() []*DCAPlan {
	plans := make([]*DCAPlan, 0, len(o.plans))
	for _, plan := range o.plans {
		plans = append(plans, plan)
	}
	return plans
}

// GetActivePlans returns active plans
func (o *DCAOrchestrator) GetActivePlans() []*DCAPlan {
	var plans []*DCAPlan
	for _, plan := range o.plans {
		if plan.Status == "Active" {
			plans = append(plans, plan)
		}
	}
	return plans
}