package services

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"tigerswap/backend/models"
)

// TestMarginCollateralRowsAreUnique pins the database-level guarantee that a
// margin account can hold at most one collateral row and one borrow row per
// asset. Before the composite unique index existed, two concurrent deposits (or
// borrows) on the same asset could each insert a row; the balance of one row then
// hid behind the other, so the account's real collateral was understated and a
// repay could clear a debt row while a duplicate kept the account borrowable.
//
// The insert is expected to be refused by the database, which is exactly what
// the index is for.
func TestMarginCollateralRowsAreUnique(t *testing.T) {
	ctx := context.Background()
	env := newMarginTestEnv(t, map[string]float64{"BTC": 60000, "RUSD": 1})
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	account, err := env.engine.OpenAccount(ctx, "user-1", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}

	first := models.MarginCollateral{AccountID: account.ID, Asset: "BTC", Amount: "1.00000000"}
	if err := env.db.Create(&first).Error; err != nil {
		t.Fatalf("first collateral insert: %v", err)
	}
	second := models.MarginCollateral{AccountID: account.ID, Asset: "BTC", Amount: "2.00000000"}
	if err := env.db.Create(&second).Error; err == nil {
		t.Fatalf("a second collateral row for the same (account, asset) must be refused")
	}

	var count int64
	if err := env.db.Model(&models.MarginCollateral{}).
		Where("account_id = ? AND asset = ?", account.ID, "BTC").Count(&count).Error; err != nil {
		t.Fatalf("count collateral: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one collateral row for BTC, got %d", count)
	}

	borrow := models.MarginBorrow{AccountID: account.ID, Asset: "RUSD", Principal: "1000.00000000", AccruedInterest: "0"}
	if err := env.db.Create(&borrow).Error; err != nil {
		t.Fatalf("first borrow insert: %v", err)
	}
	duplicate := models.MarginBorrow{AccountID: account.ID, Asset: "RUSD", Principal: "500.00000000", AccruedInterest: "0"}
	if err := env.db.Create(&duplicate).Error; err == nil {
		t.Fatalf("a second borrow row for the same (account, asset) must be refused")
	}
}

// TestMarginDepositPreservesAnExistingHolding pins the lost-update fix in
// DepositCollateral: a second deposit of the same asset must add to the stored
// amount rather than overwrite it. The pre-fix code read the amount without a
// row lock, so a deposit that raced another could write a stale total.
func TestMarginDepositPreservesAnExistingHolding(t *testing.T) {
	ctx := context.Background()
	env := newMarginTestEnv(t, map[string]float64{"BTC": 60000, "RUSD": 1})
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	account, err := env.engine.OpenAccount(ctx, "user-1", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}
	if err := env.engine.DepositCollateral(ctx, "user-1", account.ID, "BTC", "1"); err != nil {
		t.Fatalf("first deposit: %v", err)
	}
	if err := env.engine.DepositCollateral(ctx, "user-1", account.ID, "BTC", "0.5"); err != nil {
		t.Fatalf("second deposit: %v", err)
	}

	var collateral models.MarginCollateral
	if err := env.db.Where("account_id = ? AND asset = ?", account.ID, "BTC").First(&collateral).Error; err != nil {
		t.Fatalf("load collateral: %v", err)
	}
	if collateral.Amount != "1.50000000" {
		t.Fatalf("a second deposit must add to the holding: got %q, want %q", collateral.Amount, "1.50000000")
	}

	var count int64
	if err := env.db.Model(&models.MarginCollateral{}).
		Where("account_id = ? AND asset = ?", account.ID, "BTC").Count(&count).Error; err != nil {
		t.Fatalf("count collateral: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected a single collateral row after two deposits, got %d", count)
	}
}

// TestMarginDepositRollsBackWhenTheLedgerIsShort proves the debit and the
// collateral write are one unit of work: a deposit the trader cannot afford
// leaves neither a debit nor a collateral row behind.
func TestMarginDepositRollsBackWhenTheLedgerIsShort(t *testing.T) {
	ctx := context.Background()
	env := newMarginTestEnv(t, map[string]float64{"BTC": 60000, "RUSD": 1})
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	account, err := env.engine.OpenAccount(ctx, "user-1", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}
	// Drain the trader so the next deposit's debit cannot be satisfied.
	settled, err := env.engine.assetLedger("BTC")
	if err != nil {
		t.Fatalf("resolve BTC: %v", err)
	}
	available, err := env.ledger.Available("user-1", settled.ChainID, settled.Address)
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	if err := env.ledger.Debit("user-1", settled.ChainID, settled.Address, available); err != nil {
		t.Fatalf("drain: %v", err)
	}

	if err := env.engine.DepositCollateral(ctx, "user-1", account.ID, "BTC", "1"); !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("an unfunded deposit must fail with ErrInsufficientBalance, got %v", err)
	}
	var count int64
	if err := env.db.Model(&models.MarginCollateral{}).
		Where("account_id = ? AND asset = ?", account.ID, "BTC").Count(&count).Error; err != nil {
		t.Fatalf("count collateral: %v", err)
	}
	if count != 0 {
		t.Fatalf("a failed deposit must leave no collateral row, found %d", count)
	}
}

// TestMarginBorrowAddsToAnExistingDebt pins the lost-update fix in Borrow: a
// second borrow of the same asset must add to the stored principal, and the
// ledger credit must match the total debt rather than one of the borrows.
func TestMarginBorrowAddsToAnExistingDebt(t *testing.T) {
	ctx := context.Background()
	env := newMarginTestEnv(t, map[string]float64{"BTC": 60000, "RUSD": 1})
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	account, err := env.engine.OpenAccount(ctx, "user-1", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}
	if err := env.engine.DepositCollateral(ctx, "user-1", account.ID, "BTC", "1"); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if err := env.engine.Borrow(ctx, "user-1", account.ID, "RUSD", "10000"); err != nil {
		t.Fatalf("first borrow: %v", err)
	}
	if err := env.engine.Borrow(ctx, "user-1", account.ID, "RUSD", "5000"); err != nil {
		t.Fatalf("second borrow: %v", err)
	}

	var borrow models.MarginBorrow
	if err := env.db.Where("account_id = ? AND asset = ?", account.ID, "RUSD").First(&borrow).Error; err != nil {
		t.Fatalf("load borrow: %v", err)
	}
	if borrow.Principal != "15000.00000000" {
		t.Fatalf("a second borrow must add to the debt: got %q, want %q", borrow.Principal, "15000.00000000")
	}

	var count int64
	if err := env.db.Model(&models.MarginBorrow{}).
		Where("account_id = ? AND asset = ?", account.ID, "RUSD").Count(&count).Error; err != nil {
		t.Fatalf("count borrows: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected a single borrow row after two borrows, got %d", count)
	}

	// The trader was credited both borrows, so the debt must equal the credit.
	settled, err := env.engine.assetLedger("RUSD")
	if err != nil {
		t.Fatalf("resolve RUSD: %v", err)
	}
	credited, err := env.ledger.Available("user-1", settled.ChainID, settled.Address)
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	funded := new(big.Int).Mul(big.NewInt(1000000), pow10(8))
	gained := new(big.Int).Sub(credited, funded)
	want := scaleToBaseUnits(big.NewRat(15000, 1), settled.Decimals)
	if gained.Cmp(want) != 0 {
		t.Fatalf("credit must match the recorded debt: credited %s, debt %s", gained.String(), want.String())
	}
}
