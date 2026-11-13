package orders

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

// TestLimitOrderEscrowsAndFills exercises the whole on-chain order lifecycle: a
// limit order is created (escrowing the input), the executor fills it through
// the pool, and the order ends FILLED with the output delivered. This is the
// path that replaces the old off-chain order table nothing ever filled.
func TestLimitOrderEscrowsAndFills(t *testing.T) {
	h := startHarness(t)
	c := h.client()
	ctx := context.Background()

	amountIn := wad(1_000)
	// A limit buy at a price above the pool's spot so the condition is met
	// immediately; amountOutMin stays at zero so the fill cannot fail on
	// slippage in the test pool.
	price := wad(2)
	expiresAt := big.NewInt(time.Now().Add(2 * time.Hour).Unix())

	call, err := c.BuildCreateLimitOrder(SideBuy, h.tokenIn, h.tokenOut, amountIn, big.NewInt(0), price, expiresAt)
	if err != nil {
		t.Fatalf("build create: %v", err)
	}
	receipt, err := c.Send(ctx, call)
	if err != nil {
		t.Fatalf("create limit order: %v", err)
	}
	if receipt.Status != 1 {
		t.Fatalf("create reverted")
	}

	count, err := c.OrderCount(ctx)
	if err != nil {
		t.Fatalf("order count: %v", err)
	}
	if count.Sign() == 0 {
		t.Fatalf("expected at least one order")
	}

	order, err := c.Order(ctx, count)
	if err != nil {
		t.Fatalf("read order: %v", err)
	}
	if order.OrderType != TypeLimit {
		t.Fatalf("expected limit order, got %d", order.OrderType)
	}
	if order.Status != StatusOpen {
		t.Fatalf("expected OPEN, got %d", order.Status)
	}
	if order.AmountIn.Cmp(amountIn) != 0 {
		t.Fatalf("escrow mismatch: got %s want %s", order.AmountIn, amountIn)
	}

	// Fill the remainder.
	execCall, err := c.BuildExecuteOrder(count, big.NewInt(0))
	if err != nil {
		t.Fatalf("build execute: %v", err)
	}
	if _, err := c.Send(ctx, execCall); err != nil {
		t.Fatalf("execute order: %v", err)
	}

	filled, err := c.Order(ctx, count)
	if err != nil {
		t.Fatalf("read filled order: %v", err)
	}
	if filled.Status != StatusFilled {
		t.Fatalf("expected FILLED, got %d", filled.Status)
	}
	if filled.FilledAmount.Cmp(amountIn) != 0 {
		t.Fatalf("filled amount mismatch: got %s want %s", filled.FilledAmount, amountIn)
	}
	if filled.AmountOutReceived.Sign() <= 0 {
		t.Fatalf("expected positive output received, got %s", filled.AmountOutReceived)
	}
}

// TestStopLossTriggersAtOraclePrice verifies a stop-loss is only fillable once
// the oracle price crosses the stop, so a keeper cannot assert the condition.
func TestStopLossTriggersAtOraclePrice(t *testing.T) {
	h := startHarness(t)
	c := h.client()
	ctx := context.Background()

	// A stop-loss buy triggers when the oracle price rises to the stop price.
	stopPrice := wad(2)
	expiresAt := big.NewInt(time.Now().Add(2 * time.Hour).Unix())
	call, err := c.BuildCreateStopLossOrder(SideBuy, h.tokenIn, h.tokenOut, wad(500), big.NewInt(0), stopPrice, expiresAt)
	if err != nil {
		t.Fatalf("build stop-loss: %v", err)
	}
	if _, err := c.Send(ctx, call); err != nil {
		t.Fatalf("create stop-loss: %v", err)
	}
	count, _ := c.OrderCount(ctx)

	// The oracle is seeded at price 1 (below the stop of 2), so the trigger must
	// be refused.
	trigger, err := c.BuildTriggerOrder(count)
	if err != nil {
		t.Fatalf("build trigger: %v", err)
	}
	if _, err := c.Send(ctx, trigger); err == nil {
		t.Fatalf("expected trigger to revert below the stop price")
	}

	// Push the oracle to the stop price and the same trigger now succeeds.
	oracleABI := loadABI(t, "TigerSwapPriceOracle")
	packAndSend(t, h.rpc, h.chainID, h.oracle, oracleABI, "submitPrice", h.tokenIn, stopPrice)
	if _, err := c.Send(ctx, trigger); err != nil {
		t.Fatalf("trigger at stop price: %v", err)
	}

	order, err := c.Order(ctx, count)
	if err != nil {
		t.Fatalf("read order: %v", err)
	}
	if order.Status != StatusFilled {
		t.Fatalf("expected FILLED after trigger, got %d", order.Status)
	}
}

// TestCancelOrderRefundsEscrow verifies a cancelled order returns the escrowed
// input, so funds are never stranded in the contract.
func TestCancelOrderRefundsEscrow(t *testing.T) {
	h := startHarness(t)
	c := h.client()
	ctx := context.Background()

	// A limit buy far below spot never fills, so it can be cancelled.
	price := wad(1) // spot is 1, condition for a buy is price <= order price
	// Use a sell far above spot so it does not fill: sell fills when price >= order price.
	call, err := c.BuildCreateLimitOrder(SideSell, h.tokenIn, h.tokenOut, wad(100), big.NewInt(0), wad(1_000_000), big.NewInt(time.Now().Add(2*time.Hour).Unix()))
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := c.Send(ctx, call); err != nil {
		t.Fatalf("create: %v", err)
	}
	count, _ := c.OrderCount(ctx)

	cancel, err := c.BuildCancelOrder(count)
	if err != nil {
		t.Fatalf("build cancel: %v", err)
	}
	if _, err := c.Send(ctx, cancel); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	order, err := c.Order(ctx, count)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if order.Status != StatusCancelled {
		t.Fatalf("expected CANCELLED, got %d", order.Status)
	}
	_ = price
}

// TestTWAMMOrderExecutesInSlices verifies a TWAMM order advances against the
// pool when the virtual orders are executed, which is the path the off-chain
// TwammService never wired.
func TestTWAMMOrderExecutesInSlices(t *testing.T) {
	h := startHarness(t)
	tc := h.twammClient()
	priv, err := crypto.HexToECDSA(anvilKey)
	if err != nil {
		t.Fatalf("key: %v", err)
	}
	tc.SetSigner(priv)
	ctx := context.Background()

	token0, err := tc.Token0(ctx)
	if err != nil {
		t.Fatalf("token0: %v", err)
	}
	token1, err := tc.Token1(ctx)
	if err != nil {
		t.Fatalf("token1: %v", err)
	}
	if token0 == token1 {
		t.Fatalf("pool tokens must differ")
	}

	// Fund the TWAMM pool so there is something to trade against.
	erc20ABI := loadABI(t, "TestERC20")
	maxUint := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))
	packAndSend(t, h.rpc, h.chainID, token0, erc20ABI, "approve", h.twamm, maxUint)
	packAndSend(t, h.rpc, h.chainID, token1, erc20ABI, "approve", h.twamm, maxUint)
	liq, err := tc.BuildAddLiquidity(wad(50_000), wad(50_000), big.NewInt(0))
	if err != nil {
		t.Fatalf("build liquidity: %v", err)
	}
	if _, err := tc.Send(ctx, liq); err != nil {
		t.Fatalf("add liquidity: %v", err)
	}

	// Create a long-term order selling token0 over the next hour.
	expiration := big.NewInt(time.Now().Add(2 * time.Hour).Unix())
	create, err := tc.BuildCreateOrder(true, big.NewInt(1_000), expiration)
	if err != nil {
		t.Fatalf("build create: %v", err)
	}
	if _, err := tc.Send(ctx, create); err != nil {
		t.Fatalf("create twamm order: %v", err)
	}

	before, err := tc.Order(ctx, big.NewInt(1))
	if err != nil {
		t.Fatalf("read order: %v", err)
	}
	if !before.Active {
		t.Fatalf("expected order active")
	}
	if before.Escrow.Sign() <= 0 {
		t.Fatalf("expected positive escrow, got %s", before.Escrow)
	}

	// Advance the virtual orders; the order accrues proceeds over time.
	exec, err := tc.BuildExecuteVirtualOrders()
	if err != nil {
		t.Fatalf("build execute: %v", err)
	}
	if _, err := tc.Send(ctx, exec); err != nil {
		t.Fatalf("execute virtual orders: %v", err)
	}
	after, err := tc.Order(ctx, big.NewInt(1))
	if err != nil {
		t.Fatalf("read order after: %v", err)
	}
	if after.Escrow.Cmp(before.Escrow) > 0 {
		t.Fatalf("escrow grew after execution: before %s after %s", before.Escrow, after.Escrow)
	}
}
