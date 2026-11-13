package services

import (
	"math/big"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"tigerswap/backend/models"
)

func newWithdrawalFeeCollector(t *testing.T, percent, fixed float64) *FeeCollectorService {
	t.Helper()
	collector := NewFeeCollectorService()
	if err := collector.UpdateConfig(&FeeConfig{WithdrawFeePercent: percent, WithdrawFeeFixed: fixed}); err != nil {
		t.Fatalf("update config: %v", err)
	}
	return collector
}

func TestWithdrawalFeeExactAppliesPercentAndFloors(t *testing.T) {
	collector := newWithdrawalFeeCollector(t, 0.1, 0)

	// 0.1% of 1,000,000 base units is exactly 1,000.
	fee, err := collector.WithdrawalFeeExact(big.NewInt(1_000_000))
	if err != nil {
		t.Fatalf("fee: %v", err)
	}
	if fee.Cmp(big.NewInt(1_000)) != 0 {
		t.Fatalf("fee = %s, want 1000", fee)
	}

	// 0.1% of 150 is 0.15, floored to 0: the fee never exceeds the rate.
	small, err := collector.WithdrawalFeeExact(big.NewInt(150))
	if err != nil {
		t.Fatalf("fee: %v", err)
	}
	if small.Sign() != 0 {
		t.Fatalf("fee = %s, want 0", small)
	}
}

func TestWithdrawalFeeExactAppliesFixedFloorAndClamp(t *testing.T) {
	collector := newWithdrawalFeeCollector(t, 0.1, 5)

	// The fixed minimum wins when the percentage is smaller.
	fee, err := collector.WithdrawalFeeExact(big.NewInt(1_000))
	if err != nil {
		t.Fatalf("fee: %v", err)
	}
	if fee.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("fee = %s, want the fixed floor 5", fee)
	}

	// A fee can never exceed the withdrawn amount.
	clamped, err := collector.WithdrawalFeeExact(big.NewInt(3))
	if err != nil {
		t.Fatalf("fee: %v", err)
	}
	if clamped.Cmp(big.NewInt(3)) != 0 {
		t.Fatalf("fee = %s, want it clamped to 3", clamped)
	}
}

func TestWithdrawalFeeExactRejectsNonPositiveAmount(t *testing.T) {
	collector := newWithdrawalFeeCollector(t, 0.1, 0)
	if _, err := collector.WithdrawalFeeExact(big.NewInt(0)); err == nil {
		t.Fatal("a zero amount must not produce a fee")
	}
}

// TestChargeWithFeeMovesFeeToFeeAccount proves the fee is charged to the payer
// and credited to the platform fee account in one step.
func TestChargeWithFeeMovesFeeToFeeAccount(t *testing.T) {
	ledger, _ := newLedger(t)
	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}

	if err := ledger.ChargeWithFee("trader", 1, "native", big.NewInt(500), big.NewInt(5)); err != nil {
		t.Fatalf("charge: %v", err)
	}

	available, err := ledger.Available("trader", 1, "native")
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	if available.Cmp(big.NewInt(495)) != 0 {
		t.Fatalf("payer balance = %s, want 495", available)
	}
	feeBalance, err := ledger.Available(FeeAccountID, 1, "native")
	if err != nil {
		t.Fatalf("fee available: %v", err)
	}
	if feeBalance.Cmp(big.NewInt(5)) != 0 {
		t.Fatalf("fee account = %s, want 5", feeBalance)
	}
}

// TestChargeWithFeeRollsBackWhenFeeUnpayable proves the amount debit is undone
// when the payer cannot cover the fee as well, so the charge is all-or-nothing.
func TestChargeWithFeeRollsBackWhenFeeUnpayable(t *testing.T) {
	ledger, _ := newLedger(t)
	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(500)); err != nil {
		t.Fatalf("credit: %v", err)
	}

	if err := ledger.ChargeWithFee("trader", 1, "native", big.NewInt(500), big.NewInt(5)); err == nil {
		t.Fatal("a charge that cannot cover the fee must fail")
	}

	available, _ := ledger.Available("trader", 1, "native")
	if available.Cmp(big.NewInt(500)) != 0 {
		t.Fatalf("balance must be untouched after a failed charge, got %s", available)
	}
	feeBalance, _ := ledger.Available(FeeAccountID, 1, "native")
	if feeBalance.Sign() != 0 {
		t.Fatalf("fee account must be untouched, got %s", feeBalance)
	}
}

// TestWithdrawRefundsFeeWhenSendFails proves a withdrawal that fails after the
// charge returns both the amount and the fee, so the payer is not out of pocket.
func TestWithdrawRefundsFeeWhenSendFails(t *testing.T) {
	policy, ledger, db := newWithdrawalTestEnv(t, false)
	if err := db.AutoMigrate(&models.WithdrawalRequest{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}

	svc := &WalletService{db: db, blockchain: NewBlockchainClient(), ledger: ledger}
	svc.SetWithdrawalPolicy(policy)
	svc.SetFeeCollector(newWithdrawalFeeCollector(t, 0.1, 5))

	// No signer key is configured, so the send fails after the charge and the
	// refund path runs.
	rec := runAuthenticatedHandler(t, svc.Withdraw, "trader", map[string]interface{}{
		"to_address": "0x1111111111111111111111111111111111111111",
		"chain_id":   1,
		"token":      "native",
		"amount":     "500",
	}, nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected a failed send, got %d: %s", rec.Code, rec.Body.String())
	}

	available, _ := ledger.Available("trader", 1, "native")
	if available.Cmp(big.NewInt(1000)) != 0 {
		t.Fatalf("payer balance must be restored, got %s", available)
	}
	feeBalance, _ := ledger.Available(FeeAccountID, 1, "native")
	if feeBalance.Sign() != 0 {
		t.Fatalf("fee account must be empty after a refund, got %s", feeBalance)
	}
}

// TestRefusedQueuedWithdrawalReturnsFee proves refusing a queued withdrawal that
// charged a fee returns the fee as well as the amount.
func TestRefusedQueuedWithdrawalReturnsFee(t *testing.T) {
	policy, ledger, _ := newWithdrawalTestEnv(t, false)
	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}
	policy.SetFeeCollector(newWithdrawalFeeCollector(t, 0.1, 5))

	id, err := policy.CreatePendingRequest("trader", 1, "native", "0x1111111111111111111111111111111111111111", big.NewInt(500))
	if err != nil {
		t.Fatalf("queue: %v", err)
	}

	reserved, _ := ledger.Available("trader", 1, "native")
	if reserved.Cmp(big.NewInt(495)) != 0 {
		t.Fatalf("queuing must reserve the fee too, balance is %s", reserved)
	}

	rec := runHandlerWithMethod(t, policy.RefuseRequest, http.MethodPost, "/x", "admin", map[string]interface{}{"reason": "test"}, gin.Params{{Key: "id", Value: id.String()}})
	if rec.Code != http.StatusOK {
		t.Fatalf("refuse: %d %s", rec.Code, rec.Body.String())
	}

	available, _ := ledger.Available("trader", 1, "native")
	if available.Cmp(big.NewInt(1000)) != 0 {
		t.Fatalf("refusing must return the amount and the fee, balance is %s", available)
	}
	feeBalance, _ := ledger.Available(FeeAccountID, 1, "native")
	if feeBalance.Sign() != 0 {
		t.Fatalf("fee account must be empty after a refusal, got %s", feeBalance)
	}
}
