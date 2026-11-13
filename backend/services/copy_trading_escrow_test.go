package services

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/gin-gonic/gin"

	"tigerswap/backend/models"
)

// TestCopyTradeEscrowsCollateral pins that opening a copy position debits the
// follower's ledger by the copied notional, so a position can never exist
// without the funds it promises.
func TestCopyTradeEscrowsCollateral(t *testing.T) {
	engine, _ := newCopyTestEngine(t)
	engine.RegisterTrader("0xtrader", "Trader", "")
	if err := engine.StartFollowing("0xfollower", "0xtrader", CopyConfig{CopyRatio: 1}); err != nil {
		t.Fatalf("StartFollowing: %v", err)
	}
	fundCopyFollower(t, engine, "0xfollower")

	before, err := engine.ledger.Available("0xfollower", copyEscrowChainID, copyEscrowToken)
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	if _, err := engine.CopyTrade(context.Background(), "0xfollower", "0xfollower", "0xtrader", "BTC/RUSD", "long", "1000", ""); err != nil {
		t.Fatalf("CopyTrade: %v", err)
	}
	after, err := engine.ledger.Available("0xfollower", copyEscrowChainID, copyEscrowToken)
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	spent := new(big.Int).Sub(before, after)
	want := new(big.Int).Mul(big.NewInt(1000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	if spent.Cmp(want) != 0 {
		t.Fatalf("escrowed %s, want %s (1000 RUSD at 18 decimals)", spent, want)
	}
}

// TestCopyTradeRefusedWithoutBalance pins that a copy with no funds is rejected
// and no position row is written, so the ledger and the book cannot diverge.
func TestCopyTradeRefusedWithoutBalance(t *testing.T) {
	engine, db := newCopyTestEngine(t)
	engine.RegisterTrader("0xtrader", "Trader", "")
	if err := engine.StartFollowing("0xbroker", "0xtrader", CopyConfig{CopyRatio: 1}); err != nil {
		t.Fatalf("StartFollowing: %v", err)
	}

	_, err := engine.CopyTrade(context.Background(), "0xbroker", "0xbroker", "0xtrader", "BTC/RUSD", "long", "1000", "")
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("err = %v, want ErrInsufficientBalance", err)
	}
	var count int64
	if err := db.Model(&models.CopyPositionRecord{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("a refused copy wrote %d position rows", count)
	}
}

// TestCloseCopyPositionSettlesEscrow pins that closing a winning position
// credits escrow plus realised PnL, and that a losing position is clamped so the
// follower is never credited a negative amount.
func TestCloseCopyPositionSettlesEscrow(t *testing.T) {
	engine, db := newCopyTestEngine(t)
	engine.RegisterTrader("0xtrader", "Trader", "")
	if err := engine.StartFollowing("0xfollower", "0xtrader", CopyConfig{CopyRatio: 1}); err != nil {
		t.Fatalf("StartFollowing: %v", err)
	}
	fundCopyFollower(t, engine, "0xfollower")

	pos, err := engine.CopyTrade(context.Background(), "0xfollower", "0xfollower", "0xtrader", "BTC/RUSD", "long", "1000", "")
	if err != nil {
		t.Fatalf("CopyTrade: %v", err)
	}
	// A 10% favourable move on the 1000 notional is +100.
	if err := db.Model(&models.CopyPositionRecord{}).Where("id = ?", pos.ID).
		Update("pnl", "100").Error; err != nil {
		t.Fatalf("set pnl: %v", err)
	}

	if _, err := engine.CloseCopyPosition(pos.ID, "0xfollower"); err != nil {
		t.Fatalf("CloseCopyPosition: %v", err)
	}
	bal, err := engine.ledger.Available("0xfollower", copyEscrowChainID, copyEscrowToken)
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	// Funded 1e24, escrowed 1000e18, credited 1100e18 → 1e24 - 1000e18 + 1100e18.
	want := new(big.Int).Sub(
		new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil),
		new(big.Int).Mul(big.NewInt(1000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)))
	want.Add(want, new(big.Int).Mul(big.NewInt(1100), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)))
	if bal.Cmp(want) != 0 {
		t.Fatalf("balance after close = %s, want %s", bal, want)
	}
}

// TestCloseCopyPositionClampsLoss pins that a loss larger than the escrow does
// not produce a negative credit and does not touch another account's balance.
func TestCloseCopyPositionClampsLoss(t *testing.T) {
	engine, db := newCopyTestEngine(t)
	engine.RegisterTrader("0xtrader", "Trader", "")
	if err := engine.StartFollowing("0xloser", "0xtrader", CopyConfig{CopyRatio: 1}); err != nil {
		t.Fatalf("StartFollowing: %v", err)
	}
	fundCopyFollower(t, engine, "0xloser")
	// A second, unrelated account that must not be affected.
	if err := engine.ledger.Credit("0xother", copyEscrowChainID, copyEscrowToken, "RUSD", 18, big.NewInt(7)); err != nil {
		t.Fatalf("fund other: %v", err)
	}

	pos, err := engine.CopyTrade(context.Background(), "0xloser", "0xloser", "0xtrader", "BTC/RUSD", "long", "1000", "")
	if err != nil {
		t.Fatalf("CopyTrade: %v", err)
	}
	// A loss far beyond the escrow.
	if err := db.Model(&models.CopyPositionRecord{}).Where("id = ?", pos.ID).
		Update("pnl", "-5000").Error; err != nil {
		t.Fatalf("set pnl: %v", err)
	}

	if _, err := engine.CloseCopyPosition(pos.ID, "0xloser"); err != nil {
		t.Fatalf("CloseCopyPosition: %v", err)
	}
	loser, err := engine.ledger.Available("0xloser", copyEscrowChainID, copyEscrowToken)
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	funded := new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil)
	escrow := new(big.Int).Mul(big.NewInt(1000), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	wantLoser := new(big.Int).Sub(funded, escrow)
	if loser.Cmp(wantLoser) != 0 {
		t.Fatalf("loser balance = %s, want %s (escrow fully consumed, no negative credit)", loser, wantLoser)
	}
	other, err := engine.ledger.Available("0xother", copyEscrowChainID, copyEscrowToken)
	if err != nil {
		t.Fatalf("available other: %v", err)
	}
	if other.Cmp(big.NewInt(7)) != 0 {
		t.Fatalf("another account was debited: %s", other)
	}
}

// TestCloseCopyPositionForbiddenForNonOwner pins that one account cannot close
// another account's position and no escrow is settled for the stranger.
func TestCloseCopyPositionForbiddenForNonOwner(t *testing.T) {
	engine, _ := newCopyTestEngine(t)
	engine.RegisterTrader("0xtrader", "Trader", "")
	if err := engine.StartFollowing("0xowner", "0xtrader", CopyConfig{CopyRatio: 1}); err != nil {
		t.Fatalf("StartFollowing: %v", err)
	}
	fundCopyFollower(t, engine, "0xowner")
	pos, err := engine.CopyTrade(context.Background(), "0xowner", "0xowner", "0xtrader", "BTC/RUSD", "long", "1000", "")
	if err != nil {
		t.Fatalf("CopyTrade: %v", err)
	}
	if _, err := engine.CloseCopyPosition(pos.ID, "0xstranger"); !errors.Is(err, errCopyPositionForbidden) {
		t.Fatalf("err = %v, want errCopyPositionForbidden", err)
	}
}

// TestWalletCopyTradeRequiresBalance pins the HTTP path: a follower with no
// platform balance is told to fund the account, not given a position.
func TestWalletCopyTradeRequiresBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine, user := newCopyIdentityEngine(t)
	if err := engine.StartFollowing(user.WalletAddress, "0x2222222222222222222222222222222222222222", CopyConfig{CopyRatio: 1}); err != nil {
		t.Fatalf("StartFollowing: %v", err)
	}

	rec := runAuthenticatedHandler(t, func(c *gin.Context) {
		// Mirror the wallet handler's call: ownerID is the session user id.
		_, err := engine.CopyTrade(c.Request.Context(), user.ID.String(), user.WalletAddress,
			"0x2222222222222222222222222222222222222222", "BTC/RUSD", "long", "1000", "")
		if errors.Is(err, ErrInsufficientBalance) {
			c.JSON(409, gin.H{"error": "Insufficient balance"})
			return
		}
		if err != nil {
			c.JSON(500, gin.H{"error": "copy execution failed"})
			return
		}
		c.JSON(200, gin.H{"success": true})
	}, user.ID.String(), nil, nil)

	if rec.Code != 409 {
		t.Fatalf("status = %d, want 409 (no funds)", rec.Code)
	}
}
