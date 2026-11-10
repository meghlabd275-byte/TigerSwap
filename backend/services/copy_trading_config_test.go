package services

import (
	"context"
	"errors"
	"testing"
)

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
	engine := NewCopyTradingEngine(nil, &stubPriceSource{price: 100})
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

// TestCopyTradeRejectsUnknownFollower proves a copy cannot be placed for an
// account that is not following the trader.
func TestCopyTradeRejectsUnknownFollower(t *testing.T) {
	engine := NewCopyTradingEngine(nil, &stubPriceSource{price: 100})
	engine.RegisterTrader("0xtrader", "Trader", "")

	if _, err := engine.CopyTrade(context.Background(), "0xstranger", "0xtrader", "BTC/RUSD", "long", "100", ""); err == nil {
		t.Fatal("expected a copy from a non-follower to be rejected")
	}
}

// TestStopFollowingClearsConfig proves a stopped relationship drops its stored
// configuration, so a later follow starts from a clean state rather than
// silently reusing the old ratio.
func TestStopFollowingClearsConfig(t *testing.T) {
	engine := NewCopyTradingEngine(nil, &stubPriceSource{price: 100})
	engine.RegisterTrader("0xtrader", "Trader", "")

	if err := engine.StartFollowing("0xfollower", "0xtrader", CopyConfig{CopyRatio: 0.5}); err != nil {
		t.Fatalf("StartFollowing: %v", err)
	}
	if _, ok := engine.configs["0xfollower"]; !ok {
		t.Fatal("config was not stored on follow")
	}
	if err := engine.StopFollowing("0xfollower"); err != nil {
		t.Fatalf("StopFollowing: %v", err)
	}
	if _, ok := engine.configs["0xfollower"]; ok {
		t.Fatal("config survived an unfollow")
	}
}

// TestUpdateCopyPositionsAppliesStopLoss proves a position whose move breaches
// the configured stop-loss is closed by the maintenance pass.
func TestUpdateCopyPositionsAppliesStopLoss(t *testing.T) {
	engine := NewCopyTradingEngine(nil, &stubPriceSource{price: 100})
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
	engine := NewCopyTradingEngine(nil, &stubPriceSource{price: 100})
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

	if position.Status != "closed" {
		t.Fatalf("position status = %q, want closed after a take-profit hit", position.Status)
	}
}

// TestUpdateCopyPositionsLeavesHealthyPositionOpen proves a position inside its
// stop and take levels is not closed.
func TestUpdateCopyPositionsLeavesHealthyPositionOpen(t *testing.T) {
	engine := NewCopyTradingEngine(nil, &stubPriceSource{price: 100})
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
	engine := NewCopyTradingEngine(nil, &stubPriceSource{price: 100})
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
	if position.Status != "open" {
		t.Fatalf("position status = %q, want open when the oracle cannot price it", position.Status)
	}
}
