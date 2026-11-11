package services

import (
	"context"
	"sync"
	"testing"
)

// TestGetUserPositionsDoesNotAliasLiveState proves the engine hands out copies
// of its positions rather than pointers into the live map. The position
// maintenance loop re-marks a position on a background tick while a handler
// serialises the same position; returning the live pointer let the reader
// observe a half-written mark price and PnL, and the race detector flags it.
func TestGetUserPositionsDoesNotAliasLiveState(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 100})
	engine.positions["p1"] = &PerpetualPosition{
		ID: "p1", UserAddress: "0xabc", Pair: "BTC/RUSD", Side: "long",
		Size: "10000", EntryPrice: 100, LiquidationPrice: 50, Status: "open",
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			if err := engine.UpdatePositions(context.Background()); err != nil {
				t.Errorf("update positions: %v", err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			for _, position := range engine.GetUserPositions("0xabc") {
				_ = position.MarkPrice
				_ = position.UnrealizedPNL
			}
		}
	}()
	wg.Wait()
}

// TestGetUserOrdersDoesNotAliasLiveState proves the same for orders: the
// matching loop mutates an order while a handler serialises it.
func TestGetUserOrdersDoesNotAliasLiveState(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 85})
	order, err := engine.CreateOrder(context.Background(), "u", "BTC/RUSD", "long", "limit", "1", 90, 0, 3600)
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			if err := engine.MatchPendingOrders(context.Background()); err != nil {
				t.Errorf("match: %v", err)
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			for _, o := range engine.GetUserOrders("u") {
				_ = o.Status
				_ = o.FilledSize
			}
		}
	}()
	wg.Wait()

	_ = order
}

// TestCreateOrderAllocatesUniqueIDs proves concurrent order creation cannot
// collide on an id. The id used to be the wall-clock nanosecond, which two
// orders placed in the same tick share: the second insert overwrote the first
// in the in-memory book and failed on the primary key in the database.
func TestCreateOrderAllocatesUniqueIDs(t *testing.T) {
	engine := newMarkedPerpEngine(map[string]float64{"BTC": 100})
	ctx := context.Background()
	const count = 2000

	ids := make([]string, count)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			order, err := engine.CreateOrder(ctx, "u", "BTC/RUSD", "long", "limit", "1", 90, 0, 3600)
			if err != nil {
				t.Errorf("create order: %v", err)
				return
			}
			ids[i] = order.ID
		}(i)
	}
	close(start)
	wg.Wait()

	seen := make(map[string]struct{}, count)
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate order id %q", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != count {
		t.Fatalf("allocated %d distinct ids for %d orders", len(seen), count)
	}
}

// TestFeeRecordsSnapshotIsImmutable proves the fee collector hands out copies of
// its records. DistributeFees flips a record's status in place under the write
// lock; a handler that serialised the live pointer read that field concurrently.
func TestFeeRecordsSnapshotIsImmutable(t *testing.T) {
svc := NewFeeCollectorService()
if _, err := svc.CollectSwapFee("RUSD", "TGR", "1000", "0xabc", 1, "0xuser", 1, 1); err != nil {
t.Fatalf("collect swap fee: %v", err)
}

var wg sync.WaitGroup
wg.Add(2)
go func() {
defer wg.Done()
for i := 0; i < 2000; i++ {
for _, record := range svc.GetFeeRecords("swap") {
_ = record.Status
}
}
}()
go func() {
defer wg.Done()
for i := 0; i < 2000; i++ {
svc.mu.Lock()
for _, record := range svc.allRecordsLocked() {
record.Status = "distributed"
}
svc.mu.Unlock()
}
}()
wg.Wait()
}
