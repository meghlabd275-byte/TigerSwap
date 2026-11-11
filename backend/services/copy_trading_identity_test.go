package services

import (
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"tigerswap/backend/models"
)

// newCopyIdentityEngine builds an engine backed by a real in-memory database so
// the follower resolver exercises the actual query path, not a stub.
func newCopyIdentityEngine(t *testing.T) (*CopyTradingEngine, *models.User) {
	t.Helper()
	db := newWalletTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.CopyTrader{}, &models.CopyFollow{}, &models.CopyPositionRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	user := models.User{
		Email:         "follower@example.com",
		Username:      "follower",
		WalletAddress: "0x1111111111111111111111111111111111111111",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	engine := NewCopyTradingEngine(nil, &stubPriceSource{price: 100}).SetDB(db)
	engine.RegisterTrader("0x2222222222222222222222222222222222222222", "Trader", "")
	return engine, &user
}

// TestStartFollowingKeysFollowerByWallet proves the follow handler stores the
// follower under the account's bound wallet address, not the account UUID. The
// wallet copy-trade path resolves the same address, so a followed trader must
// be copyable from either route; keying one by the UUID and the other by the
// address silently broke the relationship.
func TestStartFollowingKeysFollowerByWallet(t *testing.T) {
	engine, user := newCopyIdentityEngine(t)

	rec := runAuthenticatedHandler(t, engine.StartFollowingHandler, user.ID.String(), gin.H{
		"trader_address": "0x2222222222222222222222222222222222222222",
		"copy_ratio":     0.5,
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var follow models.CopyFollow
	if err := engine.db.Where("follower_address = ?", user.WalletAddress).First(&follow).Error; err != nil {
		t.Fatalf("config must be keyed by wallet %q: %v", user.WalletAddress, err)
	}
	var byUUID int64
	if err := engine.db.Model(&models.CopyFollow{}).Where("follower_address = ?", user.ID.String()).Count(&byUUID).Error; err != nil {
		t.Fatalf("count by uuid: %v", err)
	}
	if byUUID != 0 {
		t.Fatal("config must not be keyed by the account UUID")
	}

	// A copy placed under the wallet address (the domain the wallet route uses)
	// must be accepted, proving the two routes agree on one identity.
	if _, err := engine.CopyTrade(t.Context(), user.WalletAddress, "0x2222222222222222222222222222222222222222", "BTC/RUSD", "long", "1000", ""); err != nil {
		t.Fatalf("copy under the wallet address must succeed: %v", err)
	}
}

// TestStartFollowingRejectsAccountWithoutWallet proves an account with no bound
// wallet is refused rather than being keyed under a value the wallet route can
// never reproduce.
func TestStartFollowingRejectsAccountWithoutWallet(t *testing.T) {
	db := newWalletTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.CopyTrader{}, &models.CopyFollow{}, &models.CopyPositionRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	user := models.User{Email: "nowallet@example.com", Username: "nowallet"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	engine := NewCopyTradingEngine(nil, &stubPriceSource{price: 100}).SetDB(db)
	engine.RegisterTrader("0x2222222222222222222222222222222222222222", "Trader", "")

	rec := runAuthenticatedHandler(t, engine.StartFollowingHandler, user.ID.String(), gin.H{
		"trader_address": "0x2222222222222222222222222222222222222222",
		"copy_ratio":     1.0,
	}, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for an account with no wallet, got %d: %s", rec.Code, rec.Body.String())
	}
	var followCount int64
	if err := engine.db.Model(&models.CopyFollow{}).Count(&followCount).Error; err != nil {
		t.Fatalf("count follows: %v", err)
	}
	if followCount != 0 {
		t.Fatalf("no config must be stored, got %d follows", followCount)
	}
}

// TestGetPositionsResolvesCallerWallet proves the positions endpoint returns the
// caller's own positions by resolving the wallet address, and that the emitted
// follower_address is a hex address the frontend validator accepts.
func TestGetPositionsResolvesCallerWallet(t *testing.T) {
	engine, user := newCopyIdentityEngine(t)

	if err := engine.StartFollowing(user.WalletAddress, "0x2222222222222222222222222222222222222222", CopyConfig{CopyRatio: 1}); err != nil {
		t.Fatalf("StartFollowing: %v", err)
	}
	if _, err := engine.CopyTrade(t.Context(), user.WalletAddress, "0x2222222222222222222222222222222222222222", "BTC/RUSD", "long", "100", ""); err != nil {
		t.Fatalf("CopyTrade: %v", err)
	}

	rec := runAuthenticatedHandler(t, engine.GetPositionsHandler, user.ID.String(), nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); !strings.Contains(body, `"follower_address":"0x1111111111111111111111111111111111111111"`) {
		t.Fatalf("positions must carry the caller's wallet address, got %s", body)
	}
}
