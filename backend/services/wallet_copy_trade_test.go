package services

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"tigerswap/backend/models"
)

// TestCreateCopyTradeUsesAuthenticatedWallet proves the follower address is the
// account's own bound wallet, not a client-supplied value. An attacker cannot
// mirror a trade under another wallet's identity.
func TestCreateCopyTradeUsesAuthenticatedWallet(t *testing.T) {
	db := newWalletTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.CopyTrade{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := &WalletService{db: db}

	user := models.User{
		Email:         "trader@example.com",
		Username:      "trader",
		WalletAddress: "0x1111111111111111111111111111111111111111",
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rec := runAuthenticatedHandler(t, svc.CreateCopyTrade, user.ID.String(), gin.H{
		"trader_address":   "0x2222222222222222222222222222222222222222",
		"follower_address": "0x9999999999999999999999999999999999999999",
		"pair":             "BTC/RUSD",
		"amount":           "1",
		"side":             "long",
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var stored models.CopyTrade
	if err := db.Where("follower_id = ?", user.ID.String()).First(&stored).Error; err != nil {
		t.Fatalf("load copy trade: %v", err)
	}
	if stored.FollowerAddress != "0x1111111111111111111111111111111111111111" {
		t.Fatalf("follower must be the account wallet, got %q", stored.FollowerAddress)
	}
	if stored.TraderAddress != "0x2222222222222222222222222222222222222222" {
		t.Fatalf("trader address must be recorded as supplied, got %q", stored.TraderAddress)
	}
}

// TestCreateCopyTradeRejectsAccountWithoutWallet proves an account that has not
// bound a wallet cannot copy a trade, rather than mirroring under an empty or
// attacker-chosen address.
func TestCreateCopyTradeRejectsAccountWithoutWallet(t *testing.T) {
	db := newWalletTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.CopyTrade{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := &WalletService{db: db}

	user := models.User{Email: "nowallet@example.com", Username: "nowallet"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}

	rec := runAuthenticatedHandler(t, svc.CreateCopyTrade, user.ID.String(), gin.H{
		"trader_address":   "0x2222222222222222222222222222222222222222",
		"follower_address": "0x9999999999999999999999999999999999999999",
		"pair":             "BTC/RUSD",
		"amount":           "1",
		"side":             "long",
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an account with no wallet, got %d: %s", rec.Code, rec.Body.String())
	}

	var count int64
	db.Model(&models.CopyTrade{}).Count(&count)
	if count != 0 {
		t.Fatalf("no copy trade must be persisted, got %d", count)
	}
}

// TestCreateCopyTradeRequiresAuthentication proves the handler refuses a request
// with no session user.
func TestCreateCopyTradeRequiresAuthentication(t *testing.T) {
	db := newWalletTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.CopyTrade{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	svc := &WalletService{db: db}

	rec := runAuthenticatedHandler(t, svc.CreateCopyTrade, "", gin.H{
		"trader_address": "0x2222222222222222222222222222222222222222",
		"pair":           "BTC/RUSD",
		"amount":         "1",
		"side":           "long",
	}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}
