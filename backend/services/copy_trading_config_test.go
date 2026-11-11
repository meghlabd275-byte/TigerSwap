package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// newCopyTestEngine builds a copy-trading engine backed by a real in-memory
// database, so the persistence path is exercised rather than a memory map.
func newCopyTestEngine(t *testing.T) (*CopyTradingEngine, *gorm.DB) {
	t.Helper()
	db := newListingTestDB(t)
	if err := db.AutoMigrate(&models.CopyTrader{}, &models.CopyFollow{}, &models.CopyPositionRecord{}); err != nil {
		t.Fatalf("migrate copy trading: %v", err)
	}
	return NewCopyTradingEngine(nil, &stubPriceSource{price: 100}).SetDB(db), db
}

// seedCopyPosition inserts one copy position owned by follower, returning its id.
func seedCopyPosition(t *testing.T, db *gorm.DB, follower, status string) string {
	t.Helper()
	row := models.CopyPositionRecord{
		FollowerAddress: follower,
		Pair:            "BTC/RUSD",
		Side:            "long",
		Size:            "100",
		EntryPrice:      "100.00000000",
		CurrentPrice:    "100.00000000",
		PNL:             "0",
		Status:          status,
		OpenedAt:        time.Now(),
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed copy position: %v", err)
	}
	return row.ID.String()
}

// loadCopyPositionStatus reads the status of the single seeded copy position.
func loadCopyPositionStatus(t *testing.T, db *gorm.DB) string {
	t.Helper()
	var row models.CopyPositionRecord
	if err := db.First(&row).Error; err != nil {
		t.Fatalf("load copy position: %v", err)
	}
	return row.Status
}

// reloadCopyPosition reads a copy position back from the store, since a
// maintenance pass writes its result to the database rather than to a shared
// in-memory object.
func reloadCopyPosition(t *testing.T, engine *CopyTradingEngine, id string) *CopyPosition {
	t.Helper()
	var row models.CopyPositionRecord
	if err := engine.db.Where("id = ?", id).First(&row).Error; err != nil {
		t.Fatalf("reload copy position: %v", err)
	}
	return copyPositionFromModel(&row)
}

// stubPriceSource returns a fixed USD price for any symbol. It is a test double
// for the oracle interface only: it exercises the real copy-trading logic, not
// a mock of it.
type stubPriceSource struct {
	price float64
	err   error
}

func (s *stubPriceSource) GetRealPrice(_ context.Context, _ string) (*PriceData, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &PriceData{Symbol: "BTC", USD: s.price}, nil
}

func (s *stubPriceSource) GetPrice(_ string) (*PriceData, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &PriceData{Symbol: "BTC", USD: s.price}, nil
}

// TestStartFollowingAppliesCopyRatio proves the copy_ratio supplied at follow
// time actually scales the copied notional. Before the fix the ratio was
// accepted and then discarded, so every copy was the full size.
func TestStartFollowingAppliesCopyRatio(t *testing.T) {
	engine, _ := newCopyTestEngine(t)
	engine.RegisterTrader("0xtrader", "Trader", "")

	if err := engine.StartFollowing("0xfollower", "0xtrader", CopyConfig{CopyRatio: 0.25, AutoClose: true}); err != nil {
		t.Fatalf("StartFollowing: %v", err)
	}

	position, err := engine.CopyTrade(context.Background(), "0xfollower", "0xtrader", "BTC/RUSD", "long", "1000", "")
	if err != nil {
		t.Fatalf("CopyTrade: %v", err)
	}
	if position.Size != "250" {
		t.Fatalf("copied size = %q, want 250 (0.25 of 1000)", position.Size)
	}
}

// Copy-trading state must survive a restart. Traders, follow relationships and
// copied positions used to live in process memory, so the leaderboard emptied
// and every open copy disappeared the moment the API restarted. A second engine
// over the same database must still see all three.
func TestCopyTradingStateSurvivesRestart(t *testing.T) {
	engine, db := newCopyTestEngine(t)
	engine.RegisterTrader("0xtrader", "Trader", "")
	if err := engine.StartFollowing("0xfollower", "0xtrader", CopyConfig{CopyRatio: 1}); err != nil {
		t.Fatalf("StartFollowing: %v", err)
	}
	position, err := engine.CopyTrade(context.Background(), "0xfollower", "0xtrader", "BTC/RUSD", "long", "1000", "")
	if err != nil {
		t.Fatalf("CopyTrade: %v", err)
	}

	// A fresh engine over the same database is what a restart looks like.
	restarted := NewCopyTradingEngine(nil, &stubPriceSource{price: 100}).SetDB(db)

	if _, err := restarted.GetTrader("0xtrader"); err != nil {
		t.Fatalf("trader lost on restart: %v", err)
	}
	if traders := restarted.GetTopTraders(10); len(traders) != 1 {
		t.Fatalf("leaderboard lost on restart: %d traders", len(traders))
	}
	positions := restarted.GetFollowerPositions("0xfollower")
	if len(positions) != 1 || positions[0].ID != position.ID {
		t.Fatalf("copy position lost on restart: %+v", positions)
	}
}

// TestCopyTradeRejectsUnknownFollower proves a copy cannot be placed for an
// account that is not following the trader.
func TestCopyTradeRejectsUnknownFollower(t *testing.T) {
	engine, _ := newCopyTestEngine(t)
	engine.RegisterTrader("0xtrader", "Trader", "")

	if _, err := engine.CopyTrade(context.Background(), "0xstranger", "0xtrader", "BTC/RUSD", "long", "100", ""); err == nil {
		t.Fatal("expected a copy from a non-follower to be rejected")
	}
}

// TestStopFollowingClearsConfig proves a stopped relationship drops its stored
// configuration, so a later follow starts from a clean state rather than
// silently reusing the old ratio.
func TestStopFollowingClearsConfig(t *testing.T) {
	engine, _ := newCopyTestEngine(t)
	engine.RegisterTrader("0xtrader", "Trader", "")

	if err := engine.StartFollowing("0xfollower", "0xtrader", CopyConfig{CopyRatio: 0.5}); err != nil {
		t.Fatalf("StartFollowing: %v", err)
	}
	var followCount int64
	if err := engine.db.Model(&models.CopyFollow{}).Where("follower_address = ?", "0xfollower").Count(&followCount).Error; err != nil {
		t.Fatalf("count follow: %v", err)
	}
	if followCount != 1 {
		t.Fatal("config was not stored on follow")
	}
	if err := engine.StopFollowing("0xfollower"); err != nil {
		t.Fatalf("StopFollowing: %v", err)
	}
	if err := engine.db.Model(&models.CopyFollow{}).Where("follower_address = ?", "0xfollower").Count(&followCount).Error; err != nil {
		t.Fatalf("count follow: %v", err)
	}
	if followCount != 0 {
		t.Fatal("config survived an unfollow")
	}
}

// TestUpdateCopyPositionsAppliesStopLoss proves a position whose move breaches
// the configured stop-loss is closed by the maintenance pass.
func TestUpdateCopyPositionsAppliesStopLoss(t *testing.T) {
	engine, _ := newCopyTestEngine(t)
	engine.RegisterTrader("0xtrader", "Trader", "")

	if err := engine.StartFollowing("0xfollower", "0xtrader", CopyConfig{CopyRatio: 1, StopLoss: 5}); err != nil {
		t.Fatalf("StartFollowing: %v", err)
	}
	position, err := engine.CopyTrade(context.Background(), "0xfollower", "0xtrader", "BTC/RUSD", "long", "1000", "")
	if err != nil {
		t.Fatalf("CopyTrade: %v", err)
	}

	// The price falls 10%, past the 5% stop.
	engine.priceAgg = &stubPriceSource{price: 90}
	if err := engine.UpdateCopyPositions(context.Background()); err != nil {
		t.Fatalf("UpdateCopyPositions: %v", err)
	}
	position = reloadCopyPosition(t, engine, position.ID)

	if position.Status != "closed" {
		t.Fatalf("position status = %q, want closed after a stop-loss breach", position.Status)
	}
	if position.ClosedAt == nil {
		t.Fatal("closed position has no close time")
	}
}

// TestUpdateCopyPositionsAppliesTakeProfit proves a position whose move reaches
// the configured take-profit is closed.
func TestUpdateCopyPositionsAppliesTakeProfit(t *testing.T) {
	engine, _ := newCopyTestEngine(t)
	engine.RegisterTrader("0xtrader", "Trader", "")

	if err := engine.StartFollowing("0xfollower", "0xtrader", CopyConfig{CopyRatio: 1, TakeProfit: 5}); err != nil {
		t.Fatalf("StartFollowing: %v", err)
	}
	position, err := engine.CopyTrade(context.Background(), "0xfollower", "0xtrader", "BTC/RUSD", "long", "1000", "")
	if err != nil {
		t.Fatalf("CopyTrade: %v", err)
	}

	engine.priceAgg = &stubPriceSource{price: 110}
	if err := engine.UpdateCopyPositions(context.Background()); err != nil {
		t.Fatalf("UpdateCopyPositions: %v", err)
	}
	position = reloadCopyPosition(t, engine, position.ID)

	if position.Status != "closed" {
		t.Fatalf("position status = %q, want closed after a take-profit hit", position.Status)
	}
}

// TestUpdateCopyPositionsLeavesHealthyPositionOpen proves a position inside its
// stop and take levels is not closed.
func TestUpdateCopyPositionsLeavesHealthyPositionOpen(t *testing.T) {
	engine, _ := newCopyTestEngine(t)
	engine.RegisterTrader("0xtrader", "Trader", "")

	if err := engine.StartFollowing("0xfollower", "0xtrader", CopyConfig{CopyRatio: 1, StopLoss: 5, TakeProfit: 5}); err != nil {
		t.Fatalf("StartFollowing: %v", err)
	}
	position, err := engine.CopyTrade(context.Background(), "0xfollower", "0xtrader", "BTC/RUSD", "long", "1000", "")
	if err != nil {
		t.Fatalf("CopyTrade: %v", err)
	}

	engine.priceAgg = &stubPriceSource{price: 101}
	if err := engine.UpdateCopyPositions(context.Background()); err != nil {
		t.Fatalf("UpdateCopyPositions: %v", err)
	}
	position = reloadCopyPosition(t, engine, position.ID)
	if position.Status != "open" {
		t.Fatalf("position status = %q, want open", position.Status)
	}
	if position.PNL == "" || position.PNL == "0" {
		t.Fatalf("position PnL = %q, want a non-zero mark", position.PNL)
	}
}

// TestUpdateCopyPositionsSkipsUnpriceablePair proves a pair the oracle cannot
// price is left untouched rather than closed at a fabricated level.
func TestUpdateCopyPositionsSkipsUnpriceablePair(t *testing.T) {
	engine, _ := newCopyTestEngine(t)
	engine.RegisterTrader("0xtrader", "Trader", "")
	if err := engine.StartFollowing("0xfollower", "0xtrader", CopyConfig{CopyRatio: 1, StopLoss: 5}); err != nil {
		t.Fatalf("StartFollowing: %v", err)
	}
	position, err := engine.CopyTrade(context.Background(), "0xfollower", "0xtrader", "BTC/RUSD", "long", "1000", "")
	if err != nil {
		t.Fatalf("CopyTrade: %v", err)
	}

	engine.priceAgg = &stubPriceSource{err: errors.New("oracle unavailable")}
	if err := engine.UpdateCopyPositions(context.Background()); err != nil {
		t.Fatalf("UpdateCopyPositions: %v", err)
	}
	position = reloadCopyPosition(t, engine, position.ID)
	if position.Status != "open" {
		t.Fatalf("position status = %q, want open when the oracle cannot price it", position.Status)
	}
}
