package services

import (
	"context"
	"math/big"
	"testing"

	"tigerswap/backend/models"
)

// marginBalance returns a user's available balance for a test asset.
func marginBalance(t *testing.T, env *marginTestEnv, user, asset string) *big.Int {
	t.Helper()
	settled, err := env.engine.assetLedger(asset)
	if err != nil {
		t.Fatalf("resolve %s: %v", asset, err)
	}
	available, err := env.ledger.Available(user, settled.ChainID, settled.Address)
	if err != nil {
		t.Fatalf("available %s: %v", asset, err)
	}
	return available
}

// TestMarginDepositEscrowsRealBalance verifies a collateral deposit debits the
// trader's ledger balance by the deposited amount.
func TestMarginDepositEscrowsRealBalance(t *testing.T) {
	ctx := context.Background()
	env := newMarginTestEnv(t, map[string]float64{"BTC": 60000, "RUSD": 1})
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	account, err := env.engine.OpenAccount(ctx, "user-1", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}

	before := marginBalance(t, env, "user-1", "BTC")
	if err := env.engine.DepositCollateral(ctx, "user-1", account.ID, "BTC", "2"); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	after := marginBalance(t, env, "user-1", "BTC")

	escrowed := new(big.Int).Sub(before, after)
	want := new(big.Int).Mul(big.NewInt(2), pow10(8))
	if escrowed.Cmp(want) != 0 {
		t.Fatalf("escrowed %s units, want %s", escrowed, want)
	}
}

// TestMarginDepositRefusesUnfundedTrader verifies a trader with no balance
// cannot deposit collateral, so escrow is always backed by real funds.
func TestMarginDepositRefusesUnfundedTrader(t *testing.T) {
	ctx := context.Background()
	env := newMarginTestEnv(t, map[string]float64{"BTC": 60000, "RUSD": 1})
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	account, err := env.engine.OpenAccount(ctx, "broke-user", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}
	if err := env.engine.DepositCollateral(ctx, "broke-user", account.ID, "BTC", "1"); err == nil {
		t.Fatal("an unfunded trader must not be able to deposit collateral")
	}
	var count int64
	if err := env.db.Model(&models.MarginCollateral{}).Where("account_id = ?", account.ID).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("a refused deposit must leave no collateral row, got %d", count)
	}
}

// TestMarginBorrowCreditsAndRepayDebits verifies a borrow credits the trader the
// borrowed asset and a repayment debits it back.
func TestMarginBorrowCreditsAndRepayDebits(t *testing.T) {
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

	before := marginBalance(t, env, "user-1", "RUSD")
	if err := env.engine.Borrow(ctx, "user-1", account.ID, "RUSD", "1000"); err != nil {
		t.Fatalf("borrow: %v", err)
	}
	afterBorrow := marginBalance(t, env, "user-1", "RUSD")
	gained := new(big.Int).Sub(afterBorrow, before)
	if gained.Cmp(new(big.Int).Mul(big.NewInt(1000), pow10(8))) != 0 {
		t.Fatalf("borrow credited %s units, want 1000e8", gained)
	}

	if err := env.engine.Repay(ctx, "user-1", account.ID, "RUSD", "1000"); err != nil {
		t.Fatalf("repay: %v", err)
	}
	afterRepay := marginBalance(t, env, "user-1", "RUSD")
	if afterRepay.Cmp(before) != 0 {
		t.Fatalf("repay must return the balance to its pre-borrow value: got %s, want %s", afterRepay, before)
	}
}

// TestMarginLiquidationCreditsLiquidator verifies a liquidation moves the seized
// collateral to the liquidator's real ledger balance.
func TestMarginLiquidationCreditsLiquidator(t *testing.T) {
	ctx := context.Background()
	prices := map[string]float64{"BTC": 60000, "RUSD": 1}
	env := newMarginTestEnv(t, prices)
	env.seedMarginMarket(t, "BTC/RUSD", "BTC", "RUSD", 0.7, 0.75)

	account, err := env.engine.OpenAccount(ctx, "user-1", OpenAccountRequest{Pair: "BTC/RUSD"})
	if err != nil {
		t.Fatalf("open account: %v", err)
	}
	if err := env.engine.DepositCollateral(ctx, "user-1", account.ID, "BTC", "1"); err != nil {
		t.Fatalf("deposit: %v", err)
	}
	if err := env.engine.Borrow(ctx, "user-1", account.ID, "RUSD", "40000"); err != nil {
		t.Fatalf("borrow: %v", err)
	}

	// The collateral price collapses, making the account liquidatable.
	prices["BTC"] = 30000

	liquidatorBefore := marginBalance(t, env, "liquidator", "BTC")
	if _, err := env.engine.Liquidate(ctx, account.ID, "liquidator"); err != nil {
		t.Fatalf("liquidate: %v", err)
	}
	liquidatorAfter := marginBalance(t, env, "liquidator", "BTC")

	if liquidatorAfter.Cmp(liquidatorBefore) <= 0 {
		t.Fatalf("liquidation must credit the liquidator real collateral: before %s after %s", liquidatorBefore, liquidatorAfter)
	}
}
