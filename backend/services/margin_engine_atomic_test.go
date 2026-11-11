package services

import (
	"context"
	"errors"
	"testing"

	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// TestMarginLiquidationIsAtomic verifies that a failure after the collateral has
// been seized but before the debt is repaid rolls the whole settlement back.
//
// Before the liquidation ran in one transaction, a crash (or a database error)
// between seizing collateral and reducing debt would leave the trader's
// collateral gone while the debt stayed on the books. Comparing each row against
// its pre-settlement value is the proof: a leaked seizure would change the
// collateral amount even though the debt reduction failed.
func TestMarginLiquidationIsAtomic(t *testing.T) {
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

	var beforeCollateral models.MarginCollateral
	if err := env.db.Where("account_id = ? AND asset = ?", account.ID, "BTC").First(&beforeCollateral).Error; err != nil {
		t.Fatalf("load collateral before: %v", err)
	}
	var beforeBorrow models.MarginBorrow
	if err := env.db.Where("account_id = ? AND asset = ?", account.ID, "RUSD").First(&beforeBorrow).Error; err != nil {
		t.Fatalf("load borrow before: %v", err)
	}

	// Collapse the collateral price so the account becomes liquidatable.
	prices["BTC"] = 30000

	// Force the borrow-table UPDATE that reduceDebt performs to fail. Collateral
	// is seized before debt is reduced, so without a transaction the seizure
	// would already be committed by the time this error fires.
	injected := errors.New("injected failure while reducing debt")
	if err := env.db.Callback().Update().Before("gorm:update").
		Register("test:fail_margin_borrow_update", func(tx *gorm.DB) {
			if tx.Statement != nil && tx.Statement.Table == "margin_borrows" {
				tx.AddError(injected)
			}
		}); err != nil {
		t.Fatalf("register failure injection: %v", err)
	}

	if _, err := env.engine.Liquidate(ctx, account.ID); !errors.Is(err, injected) {
		t.Fatalf("a settlement whose debt reduction fails must return the injected error, got %v", err)
	}

	var collateral models.MarginCollateral
	if err := env.db.Where("account_id = ? AND asset = ?", account.ID, "BTC").First(&collateral).Error; err != nil {
		t.Fatalf("load collateral: %v", err)
	}
	if collateral.Amount != beforeCollateral.Amount {
		t.Fatalf("collateral seize must roll back with the debt repayment: got %q, want %q", collateral.Amount, beforeCollateral.Amount)
	}

	var borrow models.MarginBorrow
	if err := env.db.Where("account_id = ? AND asset = ?", account.ID, "RUSD").First(&borrow).Error; err != nil {
		t.Fatalf("load borrow: %v", err)
	}
	if borrow.Principal != beforeBorrow.Principal {
		t.Fatalf("borrow must be untouched after a failed settlement: got %q, want %q", borrow.Principal, beforeBorrow.Principal)
	}
}
