package services

import (
	"context"
	"testing"
	"time"
)

// TestMatchPendingOrdersFillsLimitWhenPriceReached proves a resting limit order
// that never used to be evaluated is now filled once the market trades through
// its price.
func TestMatchPendingOrdersFillsLimitWhenPriceReached(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 100})
	ctx := context.Background()

	order, err := engine.CreateOrder(ctx, "user-1", "BTC/RUSD", "long", "limit", "1", 90, 0, 3600)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	// At 100 the buy limit at 90 is not met: it must stay resting.
	if err := engine.MatchPendingOrders(ctx); err != nil {
		t.Fatalf("match: %v", err)
	}
	if engine.orders[order.ID].Status != "pending" {
		t.Fatalf("a limit above the market must stay pending, got %q", engine.orders[order.ID].Status)
	}

	// The market falls to 85, through the limit, so the order fills.
	engine.priceAgg = &stubPriceAggregator{prices: map[string]float64{"BTC": 85}}
	if err := engine.MatchPendingOrders(ctx); err != nil {
		t.Fatalf("match: %v", err)
	}
	filled := engine.orders[order.ID]
	if filled.Status != "filled" {
		t.Fatalf("limit must fill once the market trades through it, got %q", filled.Status)
	}
	if filled.FilledSize != "1" {
		t.Fatalf("filled size = %q, want 1", filled.FilledSize)
	}
	if filled.Price != 85 {
		t.Fatalf("fill price = %v, want the live mark 85", filled.Price)
	}
}

// TestMatchPendingOrdersFillsStopWhenTriggered proves a stop order fills only
// once the market reaches its trigger.
func TestMatchPendingOrdersFillsStopWhenTriggered(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 100})
	ctx := context.Background()

	order, err := engine.CreateOrder(ctx, "user-1", "BTC/RUSD", "long", "stop", "1", 0, 110, 3600)
	if err != nil {
		t.Fatalf("create stop: %v", err)
	}

	if err := engine.MatchPendingOrders(ctx); err != nil {
		t.Fatalf("match: %v", err)
	}
	if engine.orders[order.ID].Status != "pending" {
		t.Fatalf("a stop below its trigger must stay pending, got %q", engine.orders[order.ID].Status)
	}

	engine.priceAgg = &stubPriceAggregator{prices: map[string]float64{"BTC": 115}}
	if err := engine.MatchPendingOrders(ctx); err != nil {
		t.Fatalf("match: %v", err)
	}
	if engine.orders[order.ID].Status != "filled" {
		t.Fatalf("stop must fill once the trigger is reached, got %q", engine.orders[order.ID].Status)
	}
}

// TestMatchPendingOrdersExpiresStaleOrders proves an order past its expiry is
// cancelled rather than resting forever.
func TestMatchPendingOrdersExpiresStaleOrders(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 100})

	engine.mu.Lock()
	engine.orders["ord_stale"] = &PerpetualOrder{
		ID:          "ord_stale",
		UserAddress: "user-1",
		Pair:        "BTC/RUSD",
		Side:        "long",
		OrderType:   "limit",
		Size:        "1",
		Price:       50,
		Status:      "pending",
		CreatedAt:   time.Now().Add(-2 * time.Hour),
		ExpiresAt:   time.Now().Add(-time.Hour),
	}
	engine.mu.Unlock()

	if err := engine.MatchPendingOrders(context.Background()); err != nil {
		t.Fatalf("match: %v", err)
	}
	if engine.orders["ord_stale"].Status != "expired" {
		t.Fatalf("a stale order must expire, got %q", engine.orders["ord_stale"].Status)
	}
}

// TestOrderConditionMetMirrorsOnChainSemantics pins the price-condition
// direction for each order type so a buy limit cannot fill above its price and
// a sell limit cannot fill below it.
func TestOrderConditionMetMirrorsOnChainSemantics(t *testing.T) {
	cases := []struct {
		name      string
		order     PerpetualOrder
		price     float64
		expectMet bool
	}{
		{"buy limit met at or below", PerpetualOrder{OrderType: "limit", Side: "long", Price: 90}, 89, true},
		{"buy limit not met above", PerpetualOrder{OrderType: "limit", Side: "long", Price: 90}, 91, false},
		{"sell limit met at or above", PerpetualOrder{OrderType: "limit", Side: "short", Price: 110}, 111, true},
		{"sell limit not met below", PerpetualOrder{OrderType: "limit", Side: "short", Price: 110}, 109, false},
		{"buy stop met at or above trigger", PerpetualOrder{OrderType: "stop", Side: "long", TriggerPrice: 110}, 111, true},
		{"buy stop not met below trigger", PerpetualOrder{OrderType: "stop", Side: "long", TriggerPrice: 110}, 109, false},
		{"sell stop met at or below trigger", PerpetualOrder{OrderType: "stop", Side: "short", TriggerPrice: 90}, 89, true},
		{"market never rests", PerpetualOrder{OrderType: "market", Side: "long"}, 100, false},
	}
	for _, tc := range cases {
		if got := orderConditionMet(&tc.order, tc.price); got != tc.expectMet {
			t.Errorf("%s: orderConditionMet = %v, want %v", tc.name, got, tc.expectMet)
		}
	}
}

// TestGetUserOrdersScopesToOwner proves one trader cannot see another's orders.
func TestGetUserOrdersScopesToOwner(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 100})
	ctx := context.Background()

	if _, err := engine.CreateOrder(ctx, "alice", "BTC/RUSD", "long", "limit", "1", 90, 0, 3600); err != nil {
		t.Fatalf("alice order: %v", err)
	}
	if _, err := engine.CreateOrder(ctx, "bob", "BTC/RUSD", "short", "limit", "1", 110, 0, 3600); err != nil {
		t.Fatalf("bob order: %v", err)
	}

	aliceOrders := engine.GetUserOrders("alice")
	if len(aliceOrders) != 1 {
		t.Fatalf("alice sees %d orders, want 1", len(aliceOrders))
	}
	if aliceOrders[0].UserAddress != "alice" {
		t.Fatalf("alice must only see her own orders, got %q", aliceOrders[0].UserAddress)
	}
}

// TestMatchPendingOrdersPersistsFill proves a fill is written through to the
// database, so a restart does not resurrect an order that already executed.
func TestMatchPendingOrdersPersistsFill(t *testing.T) {
	db := newListingTestDB(t)
	engine := NewPerpetualEngine(nil, &stubPriceAggregator{prices: map[string]float64{"BTC": 85}})
	if err := engine.SetDB(db); err != nil {
		t.Fatalf("set db: %v", err)
	}
	engine.markets["BTC/RUSD"] = &perpMarketConfig{
		Symbol: "BTC/RUSD", OracleSymbol: "BTC", MaxLeverage: 100,
	}

	order, err := engine.CreateOrder(context.Background(), "user-1", "BTC/RUSD", "long", "limit", "1", 90, 0, 3600)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}
	if err := engine.MatchPendingOrders(context.Background()); err != nil {
		t.Fatalf("match: %v", err)
	}

	var row struct{ Status string }
	if err := db.Table("perpetual_orders").Select("status").Where("id = ?", order.ID).Scan(&row).Error; err != nil {
		t.Fatalf("reload order: %v", err)
	}
	if row.Status != "filled" {
		t.Fatalf("persisted status = %q, want filled", row.Status)
	}
}
