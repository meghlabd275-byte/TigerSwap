package services

import (
	"math/big"
	"net/http"
	"strings"
	"testing"

	"tigerswap/backend/models"
)

// newWithdrawTestService builds a wallet service with a real ledger and a
// blockchain client. No private key is configured, so any withdrawal that
// passes the balance gate then fails at signing, which lets the test observe
// exactly where the request was rejected.
func newWithdrawTestService(t *testing.T) (*WalletService, *LedgerService) {
	t.Helper()
	db := newWalletTestDB(t)
	if err := db.AutoMigrate(&models.AccountBalance{}, &models.Transaction{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ledger := NewLedgerService(db)
	svc := &WalletService{db: db, blockchain: NewBlockchainClient(), ledger: ledger}
	return svc, ledger
}

// TestWithdrawRejectsUserWithoutBalance is the regression test for the
// fund-drain hole: a caller with no platform balance must not be able to make
// the platform wallet send funds anywhere.
func TestWithdrawRejectsUserWithoutBalance(t *testing.T) {
	svc, ledger := newWithdrawTestService(t)

	rec := runAuthenticatedHandler(t, svc.Withdraw, "attacker", map[string]interface{}{
		"to_address": "0x1111111111111111111111111111111111111111",
		"chain_id":   1,
		"token":      "native",
		"amount":     "1000000000000000000",
	}, nil)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for a caller with no balance, got %d: %s", rec.Code, rec.Body.String())
	}

	// The rejected attempt must not have created or changed any balance.
	available, err := ledger.Available("attacker", 1, "native")
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	if available.Sign() != 0 {
		t.Fatalf("rejected withdrawal changed the balance to %s", available)
	}
}

// TestWithdrawRejectsAmountAboveBalance proves a caller cannot withdraw more
// than they hold, even when they hold something.
func TestWithdrawRejectsAmountAboveBalance(t *testing.T) {
	svc, ledger := newWithdrawTestService(t)

	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}

	rec := runAuthenticatedHandler(t, svc.Withdraw, "trader", map[string]interface{}{
		"to_address": "0x1111111111111111111111111111111111111111",
		"chain_id":   1,
		"token":      "native",
		"amount":     "1001",
	}, nil)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for an over-balance withdrawal, got %d: %s", rec.Code, rec.Body.String())
	}

	available, _ := ledger.Available("trader", 1, "native")
	if available.Cmp(big.NewInt(1000)) != 0 {
		t.Fatalf("balance must be untouched, got %s", available)
	}
}

// TestWithdrawIgnoresClientSuppliedUserID proves a caller cannot spend another
// account's balance by putting that account's id in the request body.
func TestWithdrawIgnoresClientSuppliedUserID(t *testing.T) {
	svc, ledger := newWithdrawTestService(t)

	if err := ledger.Credit("victim", 1, "native", "ETH", 18, big.NewInt(5000)); err != nil {
		t.Fatalf("credit: %v", err)
	}

	// The body names the victim; the session says attacker.
	rec := runAuthenticatedHandler(t, svc.Withdraw, "attacker", map[string]interface{}{
		"user_id":    "victim",
		"to_address": "0x2222222222222222222222222222222222222222",
		"chain_id":   1,
		"token":      "native",
		"amount":     "1000",
	}, nil)

	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 because the session user has no balance, got %d: %s", rec.Code, rec.Body.String())
	}

	victimBalance, _ := ledger.Available("victim", 1, "native")
	if victimBalance.Cmp(big.NewInt(5000)) != 0 {
		t.Fatalf("victim balance changed to %s", victimBalance)
	}
}

// TestWithdrawRequiresSession proves an unauthenticated request is rejected
// before any balance or signing logic runs.
func TestWithdrawRequiresSession(t *testing.T) {
	svc, _ := newWithdrawTestService(t)

	rec := runAuthenticatedHandler(t, svc.Withdraw, "", map[string]interface{}{
		"to_address": "0x1111111111111111111111111111111111111111",
		"chain_id":   1,
		"token":      "native",
		"amount":     "1",
	}, nil)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without a session, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestWithdrawValidatesDestinationAddress proves a malformed destination is
// rejected before any balance is touched.
func TestWithdrawValidatesDestinationAddress(t *testing.T) {
	svc, ledger := newWithdrawTestService(t)
	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}

	rec := runAuthenticatedHandler(t, svc.Withdraw, "trader", map[string]interface{}{
		"to_address": "not-an-address",
		"chain_id":   1,
		"token":      "native",
		"amount":     "100",
	}, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a bad address, got %d: %s", rec.Code, rec.Body.String())
	}
	available, _ := ledger.Available("trader", 1, "native")
	if available.Cmp(big.NewInt(1000)) != 0 {
		t.Fatalf("balance must be untouched, got %s", available)
	}
}

// TestWithdrawRejectsNonPositiveAmount proves zero and negative amounts never
// reach the signer.
func TestWithdrawRejectsNonPositiveAmount(t *testing.T) {
	svc, ledger := newWithdrawTestService(t)
	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}

	for _, amount := range []string{"0", "-5"} {
		rec := runAuthenticatedHandler(t, svc.Withdraw, "trader", map[string]interface{}{
			"to_address": "0x1111111111111111111111111111111111111111",
			"chain_id":   1,
			"token":      "native",
			"amount":     amount,
		}, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for amount %q, got %d: %s", amount, rec.Code, rec.Body.String())
		}
	}
}

// TestGetBalancesReturnsOnlyCallersRows proves the balance endpoint is scoped
// to the session user.
func TestGetBalancesReturnsOnlyCallersRows(t *testing.T) {
	svc, ledger := newWithdrawTestService(t)
	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}
	if err := ledger.Credit("other", 1, "native", "ETH", 18, big.NewInt(9999)); err != nil {
		t.Fatalf("credit: %v", err)
	}

	rec := runAuthenticatedHandler(t, svc.GetBalances, "trader", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"user_id":"trader"`) || strings.Contains(body, "9999") {
		t.Fatalf("response leaked another account: %s", body)
	}
}
