package services

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"tigerswap/backend/models"
)

// TestPerpetualEngineRestoresOpenPositionsOnStartup pins the behaviour that a
// restart does not drop the open book. The engine prices, liquidates and closes
// positions from memory, so a position that is not reloaded on startup can never
// be marked, closed or liquidated again — it would sit in the database as
// "open" forever while the engine has no idea it exists.
func TestPerpetualEngineRestoresOpenPositionsOnStartup(t *testing.T) {
	db := newListingTestDB(t)

	if err := db.Create(&models.FuturesMarket{
		Symbol: "BTC/RUSD", BaseAsset: "BTC", QuoteAsset: "RUSD", Settlement: "RUSD",
		Status: models.ListingStatusActive, MaxLeverage: 200,
	}).Error; err != nil {
		t.Fatalf("seed listing: %v", err)
	}

	openID := uuid.New()
	closedID := uuid.New()
	liqID := uuid.New()
	rows := []models.PerpetualPosition{
		{
			ID: openID, UserID: "user-1", Pair: "BTC/RUSD", Side: "long",
			Size: "10000", Collateral: "1000", EntryPrice: "60000", MarkPrice: "61000",
			Leverage: 10, Status: "open", OpenedAt: time.Now(),
		},
		{
			ID: closedID, UserID: "user-1", Pair: "BTC/RUSD", Side: "long",
			Size: "5000", Collateral: "500", EntryPrice: "60000", MarkPrice: "60000",
			Leverage: 10, Status: "closed", OpenedAt: time.Now(),
		},
		{
			ID: liqID, UserID: "user-2", Pair: "BTC/RUSD", Side: "short",
			Size: "20000", Collateral: "2000", EntryPrice: "60000", MarkPrice: "66000",
			Leverage: 10, Status: "liquidatable", OpenedAt: time.Now(),
		},
	}
	for i := range rows {
		if err := db.Create(&rows[i]).Error; err != nil {
			t.Fatalf("seed position %s: %v", rows[i].ID, err)
		}
	}

	engine := NewPerpetualEngine(nil, nil)
	if err := engine.SetDB(db); err != nil {
		t.Fatalf("set db: %v", err)
	}

	// The open position must be back in the book with its risk parameters.
	open, err := engine.GetPosition(openID.String())
	if err != nil {
		t.Fatalf("open position was not restored: %v", err)
	}
	if open.Status != "open" {
		t.Errorf("restored status = %q, want open", open.Status)
	}
	if open.EntryPrice != 60000 {
		t.Errorf("restored entry price = %v, want 60000", open.EntryPrice)
	}
	if open.MarkPrice != 61000 {
		t.Errorf("restored mark price = %v, want 61000", open.MarkPrice)
	}
	if open.UserAddress != "user-1" {
		t.Errorf("restored owner = %q, want user-1", open.UserAddress)
	}
	// 10x long from 60000: liquidation = 60000 * (1 - 1/10 * 0.5) = 57000.
	if open.LiquidationPrice != 57000 {
		t.Errorf("restored liquidation price = %v, want 57000", open.LiquidationPrice)
	}

	// A position flagged liquidatable before the restart keeps that flag, so the
	// liquidation bot still sees it.
	flagged, err := engine.GetPosition(liqID.String())
	if err != nil {
		t.Fatalf("liquidatable position was not restored: %v", err)
	}
	if flagged.Status != "liquidatable" {
		t.Errorf("restored liquidatable status = %q, want liquidatable", flagged.Status)
	}
	if flagged.LiquidationPrice != 63000 {
		t.Errorf("short liquidation price = %v, want 63000", flagged.LiquidationPrice)
	}

	// A closed position must not be resurrected.
	if _, err := engine.GetPosition(closedID.String()); err == nil {
		t.Error("a closed position must not be restored into the open book")
	}

	// The restored open position counts toward open interest.
	long, short, err := engine.openInterestRat("BTC/RUSD")
	if err != nil {
		t.Fatalf("open interest: %v", err)
	}
	if long.RatString() != "10000" {
		t.Errorf("long open interest = %s, want 10000", long.RatString())
	}
	if short.RatString() != "20000" {
		t.Errorf("short open interest = %s, want 20000", short.RatString())
	}
}

// TestPerpetualOrdersSurviveRestart pins the behaviour that a restart does not
// drop the resting order book. A pending order used to live only in the
// engine's process memory, so after a restart it could never be filled or
// cancelled even though the database still listed it as pending.
func TestPerpetualOrdersSurviveRestart(t *testing.T) {
	db := newListingTestDB(t)
	if err := db.AutoMigrate(&models.PerpetualOrder{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := db.Create(&models.PerpetualOrder{
		ID: "ord_pending", UserID: "user-1", Pair: "BTC/RUSD", Side: "long",
		OrderType: "limit", Size: "1000", Price: "59000", Status: "pending",
		FilledSize: "0", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed pending order: %v", err)
	}
	if err := db.Create(&models.PerpetualOrder{
		ID: "ord_cancelled", UserID: "user-1", Pair: "BTC/RUSD", Side: "long",
		OrderType: "limit", Size: "1000", Price: "59000", Status: "cancelled",
		FilledSize: "0", CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed cancelled order: %v", err)
	}

	engine := NewPerpetualEngine(nil, nil)
	if err := engine.SetDB(db); err != nil {
		t.Fatalf("set db: %v", err)
	}

	// The pending order is restored and can still be cancelled by its owner.
	if err := engine.CancelOrder("ord_pending", "user-1"); err != nil {
		t.Fatalf("restored pending order should be cancellable: %v", err)
	}
	var reloaded models.PerpetualOrder
	if err := db.First(&reloaded, "id = ?", "ord_pending").Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if reloaded.Status != "cancelled" {
		t.Fatalf("cancellation must persist, status = %q", reloaded.Status)
	}

	// A cancelled order is not resurrected into the book.
	if err := engine.CancelOrder("ord_cancelled", "user-1"); !errors.Is(err, ErrPerpOrderNotFound) {
		t.Fatalf("a cancelled order must not be restored, got %v", err)
	}
}

// TestPerpetualEngineSkipsUnrestorableRows verifies a corrupt row is skipped
// rather than loaded with an invented value, and does not prevent the healthy
// rows from being restored.
func TestPerpetualEngineSkipsUnrestorableRows(t *testing.T) {
	db := newListingTestDB(t)

	bad := models.PerpetualPosition{
		ID: uuid.New(), UserID: "user-9", Pair: "BTC/RUSD", Side: "long",
		Size: "10000", Collateral: "1000", EntryPrice: "not-a-price", MarkPrice: "0",
		Leverage: 10, Status: "open", OpenedAt: time.Now(),
	}
	good := models.PerpetualPosition{
		ID: uuid.New(), UserID: "user-1", Pair: "BTC/RUSD", Side: "long",
		Size: "10000", Collateral: "1000", EntryPrice: "60000", MarkPrice: "60000",
		Leverage: 10, Status: "open", OpenedAt: time.Now(),
	}
	if err := db.Create(&bad).Error; err != nil {
		t.Fatalf("seed bad: %v", err)
	}
	if err := db.Create(&good).Error; err != nil {
		t.Fatalf("seed good: %v", err)
	}

	engine := NewPerpetualEngine(nil, nil)
	if err := engine.SetDB(db); err != nil {
		t.Fatalf("set db: %v", err)
	}

	if _, err := engine.GetPosition(good.ID.String()); err != nil {
		t.Fatalf("healthy position should be restored: %v", err)
	}
	if _, err := engine.GetPosition(bad.ID.String()); err == nil {
		t.Error("a position with an unparsable entry price must not be restored")
	}
}
