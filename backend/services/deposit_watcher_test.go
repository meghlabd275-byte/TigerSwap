package services

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

func newWatcherTestDB(t *testing.T) (*gorm.DB, *LedgerService, *DepositWatcher) {
	t.Helper()
	db := newWalletTestDB(t)
	if err := db.AutoMigrate(
		&models.AccountBalance{},
		&models.DepositAddress{},
		&models.ProcessedDeposit{},
		&models.DepositCursor{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ledger := NewLedgerService(db)
	watcher := NewDepositWatcher(db, NewBlockchainClient(), ledger)
	return db, ledger, watcher
}

// TestDepositWatcherCreditsOnce is the replay test: the same on-chain deposit
// observed twice must credit the user exactly once.
func TestDepositWatcherCreditsOnce(t *testing.T) {
	_, ledger, watcher := newWatcherTestDB(t)

	tx := types.NewTx(&types.LegacyTx{
		Nonce: 1,
		Value: big.NewInt(1_000_000_000_000_000_000),
	})

	amount := big.NewInt(1_000_000_000_000_000_000)
	for i := 0; i < 3; i++ {
		if err := watcher.creditDeposit(1, tx, 0, "user-1", nativeTokenKey, "ETH", 18, amount, 100); err != nil {
			t.Fatalf("creditDeposit attempt %d: %v", i, err)
		}
	}

	available, err := ledger.Available("user-1", 1, nativeTokenKey)
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	if available.Cmp(amount) != 0 {
		t.Fatalf("expected a single credit of %s, got %s", amount, available)
	}
}

// TestDepositWatcherCreditsDistinctLogs proves separate transfers in the same
// transaction are each credited.
func TestDepositWatcherCreditsDistinctLogs(t *testing.T) {
	_, ledger, watcher := newWatcherTestDB(t)

	tx := types.NewTx(&types.LegacyTx{Nonce: 2, Value: big.NewInt(0)})
	amount := big.NewInt(500)

	if err := watcher.creditDeposit(1, tx, 0, "user-1", "0xToken", "TGR", 18, amount, 10); err != nil {
		t.Fatalf("log 0: %v", err)
	}
	if err := watcher.creditDeposit(1, tx, 1, "user-1", "0xToken", "TGR", 18, amount, 10); err != nil {
		t.Fatalf("log 1: %v", err)
	}

	available, _ := ledger.Available("user-1", 1, "0xToken")
	if available.Cmp(big.NewInt(1000)) != 0 {
		t.Fatalf("expected 1000 across two logs, got %s", available)
	}
}

// TestDepositWatcherRejectsNonPositive proves a zero or negative transfer can
// never mint credit.
func TestDepositWatcherRejectsNonPositive(t *testing.T) {
	_, ledger, watcher := newWatcherTestDB(t)

	tx := types.NewTx(&types.LegacyTx{Nonce: 3, Value: big.NewInt(0)})
	if err := watcher.creditDeposit(1, tx, 0, "user-1", nativeTokenKey, "ETH", 18, big.NewInt(0), 5); err == nil {
		t.Fatalf("zero-value deposit must be rejected")
	}

	available, _ := ledger.Available("user-1", 1, nativeTokenKey)
	if available.Sign() != 0 {
		t.Fatalf("expected zero balance, got %s", available)
	}
}

// TestDepositAddressOwnersResolvesByChain proves the watcher maps only the
// addresses registered for the chain being scanned.
func TestDepositAddressOwnersResolvesByChain(t *testing.T) {
	db, _, watcher := newWatcherTestDB(t)

	rows := []models.DepositAddress{
		{UserID: "user-1", ChainID: 1, Address: "0xAaAaAaAaAaAaAaAaAaAaAaAaAaAaAaAaAaAaAaAa"},
		{UserID: "user-2", ChainID: 1, Address: "0xBbBbBbBbBbBbBbBbBbBbBbBbBbBbBbBbBbBbBbBb"},
		{UserID: "user-3", ChainID: 56, Address: "0xCcCcCcCcCcCcCcCcCcCcCcCcCcCcCcCcCcCcCcCc"},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed addresses: %v", err)
	}

	owners, err := watcher.depositAddressOwners(1)
	if err != nil {
		t.Fatalf("owners: %v", err)
	}
	if len(owners) != 2 {
		t.Fatalf("expected 2 owners on chain 1, got %d", len(owners))
	}
	if owner, ok := owners["0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"]; !ok || owner != "user-1" {
		t.Fatalf("expected user-1 for the lower-cased address, got %q (found=%v)", owner, ok)
	}
	if _, ok := owners["0xcccccccccccccccccccccccccccccccccccccccc"]; ok {
		t.Fatalf("chain 56 address leaked into chain 1 scan")
	}
}

// TestDepositWatcherCreditRequiresPositiveAmount covers the admin backfill path.
func TestDepositWatcherCreditRequiresPositiveAmount(t *testing.T) {
	_, _, watcher := newWatcherTestDB(t)

	if err := watcher.CreditDeposit("user-1", 1, nativeTokenKey, "ETH", 18, big.NewInt(0)); err == nil {
		t.Fatalf("zero deposit must be rejected")
	}
	if err := watcher.CreditDeposit("user-1", 1, nativeTokenKey, "ETH", 18, big.NewInt(10)); err != nil {
		t.Fatalf("valid deposit rejected: %v", err)
	}
}

// TestDepositWatcherDoesNotCreditUnregisteredAddress proves a transfer to an
// address the platform never issued is ignored.
func TestDepositWatcherDoesNotCreditUnregisteredAddress(t *testing.T) {
	_, ledger, watcher := newWatcherTestDB(t)

	owners, err := watcher.depositAddressOwners(1)
	if err != nil {
		t.Fatalf("owners: %v", err)
	}

	stranger := common.HexToAddress("0x9999999999999999999999999999999999999999")
	if _, ok := owners[stranger.Hex()]; ok {
		t.Fatalf("unregistered address unexpectedly mapped")
	}

	available, _ := ledger.Available("stranger", 1, nativeTokenKey)
	if available.Sign() != 0 {
		t.Fatalf("expected zero balance, got %s", available)
	}
}
