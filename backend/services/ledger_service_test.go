package services

import (
	"errors"
	"math/big"
	"sync"
	"testing"

	"gorm.io/gorm"

	"tigerswap/backend/models"
)

func newLedger(t *testing.T) (*LedgerService, *gorm.DB) {
	t.Helper()
	db := newListingTestDB(t)
	if err := db.AutoMigrate(&models.AccountBalance{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewLedgerService(db), db
}

func TestLedgerCreditAndDebit(t *testing.T) {
	ledger, _ := newLedger(t)

	if err := ledger.Credit("user-1", 1, "0xABC", "RUSD", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}
	// Address case must not split the ledger into two rows.
	if err := ledger.Credit("user-1", 1, "0xabc", "RUSD", 18, big.NewInt(500)); err != nil {
		t.Fatalf("credit: %v", err)
	}

	available, err := ledger.Available("user-1", 1, "0xABC")
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	if available.Cmp(big.NewInt(1500)) != 0 {
		t.Fatalf("expected 1500, got %s", available)
	}

	if err := ledger.Debit("user-1", 1, "0xabc", big.NewInt(400)); err != nil {
		t.Fatalf("debit: %v", err)
	}
	available, _ = ledger.Available("user-1", 1, "0xABC")
	if available.Cmp(big.NewInt(1100)) != 0 {
		t.Fatalf("expected 1100, got %s", available)
	}
}

func TestLedgerDebitRejectsOverdraft(t *testing.T) {
	ledger, _ := newLedger(t)

	if err := ledger.Credit("user-1", 1, "native", "ETH", 18, big.NewInt(100)); err != nil {
		t.Fatalf("credit: %v", err)
	}

	err := ledger.Debit("user-1", 1, "native", big.NewInt(101))
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("expected ErrInsufficientBalance, got %v", err)
	}

	available, _ := ledger.Available("user-1", 1, "native")
	if available.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("failed debit must not change the balance, got %s", available)
	}
}

// TestLedgerDebitWithoutCredit proves a debit on an account with no history is
// rejected rather than creating negative funds.
func TestLedgerDebitWithoutCredit(t *testing.T) {
	ledger, _ := newLedger(t)

	err := ledger.Debit("stranger", 1, "native", big.NewInt(1))
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("expected ErrInsufficientBalance, got %v", err)
	}
	available, _ := ledger.Available("stranger", 1, "native")
	if available.Sign() != 0 {
		t.Fatalf("expected zero balance, got %s", available)
	}
}

func TestLedgerRejectsNonPositiveAmounts(t *testing.T) {
	ledger, _ := newLedger(t)

	if err := ledger.Credit("user-1", 1, "native", "ETH", 18, big.NewInt(0)); err == nil {
		t.Fatalf("zero credit must be rejected")
	}
	if err := ledger.Credit("user-1", 1, "native", "ETH", 18, big.NewInt(-5)); err == nil {
		t.Fatalf("negative credit must be rejected")
	}
	if err := ledger.Debit("user-1", 1, "native", big.NewInt(0)); err == nil {
		t.Fatalf("zero debit must be rejected")
	}
}

// TestLedgerConcurrentDebitsCannotOverdraw proves the ledger never overdraws
// under concurrency. Some attempts may be rejected by the storage engine's
// write lock on SQLite; what must hold is that no successful debit ever takes
// the balance below zero and that the successes exactly account for the drop.
func TestLedgerConcurrentDebitsCannotOverdraw(t *testing.T) {
	ledger, _ := newLedger(t)

	const starting = 100
	if err := ledger.Credit("racer", 1, "native", "ETH", 18, big.NewInt(starting)); err != nil {
		t.Fatalf("credit: %v", err)
	}

	const attempts = 20
	const spend = 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	succeeded := 0

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ledger.Debit("racer", 1, "native", big.NewInt(spend)); err == nil {
				mu.Lock()
				succeeded++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	available, err := ledger.Available("racer", 1, "native")
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	if available.Sign() < 0 {
		t.Fatalf("balance went negative: %s", available)
	}

	// Conservation: every success removed exactly `spend`, so the remaining
	// balance must equal starting - succeeded*spend.
	expected := big.NewInt(int64(starting - succeeded*spend))
	if available.Cmp(expected) != 0 {
		t.Fatalf("balance %s does not match %d successful debits (expected %s)", available, succeeded, expected)
	}
	if succeeded > starting/spend {
		t.Fatalf("more debits succeeded (%d) than the balance allows", succeeded)
	}
}

func TestLedgerBalancesListsAllRows(t *testing.T) {
	ledger, _ := newLedger(t)

	ledger.Credit("user-1", 1, "native", "ETH", 18, big.NewInt(1))
	ledger.Credit("user-1", 56, "0xUSDT", "USDT", 6, big.NewInt(2))
	ledger.Credit("user-2", 1, "native", "ETH", 18, big.NewInt(3))

	rows, err := ledger.Balances("user-1")
	if err != nil {
		t.Fatalf("balances: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows for user-1, got %d", len(rows))
	}
	for _, row := range rows {
		if row.UserID != "user-1" {
			t.Fatalf("leaked another user's row: %+v", row)
		}
	}
}
