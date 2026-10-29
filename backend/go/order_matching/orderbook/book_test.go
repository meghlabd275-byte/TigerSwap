package orderbook

import (
	"errors"
	"math"
	"sync"
	"testing"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	e := NewEngine()
	if err := e.RegisterPair(PairConfig{
		Pair:        "BTC/RUSD",
		MakerFeeBps: 0,
		TakerFeeBps: 2,
	}); err != nil {
		t.Fatalf("register pair: %v", err)
	}
	return e
}

func mustSubmit(t *testing.T, e *Engine, o Order) (Order, []Trade) {
	t.Helper()
	placed, trades, err := e.Submit(o)
	if err != nil {
		t.Fatalf("submit %+v: %v", o, err)
	}
	return placed, trades
}

// TestRestingOrderDoesNotTrade verifies a limit order with no counterparty
// rests on the book and reports its full quantity as leaves.
func TestRestingOrderDoesNotTrade(t *testing.T) {
	e := testEngine(t)

	placed, trades := mustSubmit(t, e, Order{
		Trader: "alice", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit,
		Price: 60000, Quantity: 1,
	})

	if len(trades) != 0 {
		t.Fatalf("expected no trades, got %d", len(trades))
	}
	if placed.Status != StatusOpen {
		t.Fatalf("expected status open, got %q", placed.Status)
	}
	if placed.Leaves != 1 {
		t.Fatalf("expected 1 leaf, got %v", placed.Leaves)
	}
	if placed.Filled != 0 {
		t.Fatalf("expected 0 filled, got %v", placed.Filled)
	}
}

// TestCrossingOrderFillsAtMakerPrice verifies the resting order sets the trade
// price: a taker buying above the ask pays the ask, not its own limit.
func TestCrossingOrderFillsAtMakerPrice(t *testing.T) {
	e := testEngine(t)

	mustSubmit(t, e, Order{
		Trader: "maker", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit,
		Price: 60000, Quantity: 1,
	})

	taker, trades := mustSubmit(t, e, Order{
		Trader: "taker", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit,
		Price: 61000, Quantity: 1,
	})

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Price != 60000 {
		t.Fatalf("expected trade at maker price 60000, got %v", trades[0].Price)
	}
	if trades[0].Quantity != 1 {
		t.Fatalf("expected quantity 1, got %v", trades[0].Quantity)
	}
	if taker.Status != StatusFilled {
		t.Fatalf("expected taker filled, got %q", taker.Status)
	}
	if taker.Filled != 1 || taker.Leaves != 0 {
		t.Fatalf("expected fully filled, got filled=%v leaves=%v", taker.Filled, taker.Leaves)
	}
}

// TestPriceTimePriority verifies orders at a better price fill first, and that
// orders at the same price fill in arrival order.
func TestPriceTimePriority(t *testing.T) {
	e := testEngine(t)

	// Two asks at the same price; the first must fill first.
	mustSubmit(t, e, Order{Trader: "ask1", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 60000, Quantity: 1})
	mustSubmit(t, e, Order{Trader: "ask2", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 60000, Quantity: 1})
	// A better ask that must be consumed before either of them.
	mustSubmit(t, e, Order{Trader: "ask0", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 59900, Quantity: 1})

	_, trades := mustSubmit(t, e, Order{
		Trader: "taker", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit,
		Price: 60100, Quantity: 3,
	})

	if len(trades) != 3 {
		t.Fatalf("expected 3 trades, got %d", len(trades))
	}
	wantPrices := []float64{59900, 60000, 60000}
	for i, want := range wantPrices {
		if trades[i].Price != want {
			t.Fatalf("trade %d: expected price %v, got %v", i, want, trades[i].Price)
		}
	}
	if trades[1].Maker != "ask1" || trades[2].Maker != "ask2" {
		t.Fatalf("same-price orders must fill in arrival order, got %s then %s", trades[1].Maker, trades[2].Maker)
	}
}

// TestPartialFillThenRemainderRests verifies a limit order that only partly
// crosses keeps the remainder on the book at its own price.
func TestPartialFillThenRemainderRests(t *testing.T) {
	e := testEngine(t)

	mustSubmit(t, e, Order{Trader: "maker", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 60000, Quantity: 1})

	taker, trades := mustSubmit(t, e, Order{
		Trader: "taker", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit,
		Price: 60000, Quantity: 2,
	})

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if taker.Status != StatusPartial {
		t.Fatalf("expected partial, got %q", taker.Status)
	}
	if taker.Filled != 1 || taker.Leaves != 1 {
		t.Fatalf("expected filled 1 leaves 1, got filled=%v leaves=%v", taker.Filled, taker.Leaves)
	}

	book, err := e.Book("BTC/RUSD")
	if err != nil {
		t.Fatalf("book: %v", err)
	}
	if got := book.BestBid(); got != 60000 {
		t.Fatalf("expected the remainder to rest as the best bid at 60000, got %v", got)
	}
	if got := book.OpenOrders(); got != 1 {
		t.Fatalf("expected 1 resting order, got %d", got)
	}
}

// TestMarketOrderNeverRests verifies a market order consumes what it can and
// does not leave a resting order behind.
func TestMarketOrderNeverRests(t *testing.T) {
	e := testEngine(t)

	mustSubmit(t, e, Order{Trader: "maker", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 60000, Quantity: 1})

	// Market buy for more than the book holds.
	taker, trades := mustSubmit(t, e, Order{
		Trader: "taker", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeMarket, Quantity: 3,
	})

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if taker.Status != StatusCancelled {
		t.Fatalf("expected an unfilled market remainder to be cancelled, got %q", taker.Status)
	}
	if taker.Leaves != 0 {
		t.Fatalf("a market order must not rest, got leaves %v", taker.Leaves)
	}

	book, _ := e.Book("BTC/RUSD")
	if book.OpenOrders() != 0 {
		t.Fatalf("expected an empty book, got %d resting orders", book.OpenOrders())
	}
}

// TestIOCDropsRemainder verifies an immediate-or-cancel order fills what it can
// and cancels the rest.
func TestIOCDropsRemainder(t *testing.T) {
	e := testEngine(t)

	mustSubmit(t, e, Order{Trader: "maker", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 60000, Quantity: 1})

	taker, trades := mustSubmit(t, e, Order{
		Trader: "taker", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit,
		Price: 60000, Quantity: 2, TIF: TIFIOC,
	})

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if taker.Status != StatusCancelled {
		t.Fatalf("expected IOC remainder cancelled, got %q", taker.Status)
	}
	if taker.Leaves != 0 {
		t.Fatalf("expected no leaves for an IOC remainder, got %v", taker.Leaves)
	}
}

// TestFOKRejectsWhenUnfillable verifies a fill-or-kill order is rejected whole
// when the book cannot fill it completely, leaving the book untouched.
func TestFOKRejectsWhenUnfillable(t *testing.T) {
	e := testEngine(t)

	mustSubmit(t, e, Order{Trader: "maker", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 60000, Quantity: 1})

	_, _, err := e.Submit(Order{
		Trader: "taker", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit,
		Price: 60000, Quantity: 5, TIF: TIFFOK,
	})
	if !errors.Is(err, ErrNoLiquidity) {
		t.Fatalf("expected ErrNoLiquidity, got %v", err)
	}

	book, _ := e.Book("BTC/RUSD")
	if book.OpenOrders() != 1 {
		t.Fatalf("a rejected FOK order must not touch the book, got %d resting orders", book.OpenOrders())
	}
}

// TestFOKFillsWhenFillable verifies a fill-or-kill order executes when the
// whole quantity is available.
func TestFOKFillsWhenFillable(t *testing.T) {
	e := testEngine(t)

	mustSubmit(t, e, Order{Trader: "maker", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 60000, Quantity: 2})

	taker, trades := mustSubmit(t, e, Order{
		Trader: "taker", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit,
		Price: 60000, Quantity: 2, TIF: TIFFOK,
	})

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if taker.Status != StatusFilled {
		t.Fatalf("expected filled, got %q", taker.Status)
	}
}

// TestLimitOrderDoesNotCrossThroughItsLimit verifies an order that cannot reach
// the resting price rests instead of trading through.
func TestLimitOrderDoesNotCrossThroughItsLimit(t *testing.T) {
	e := testEngine(t)

	mustSubmit(t, e, Order{Trader: "maker", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 60000, Quantity: 1})

	// A buy capped at 59000 must not trade against a 60000 ask.
	taker, trades := mustSubmit(t, e, Order{
		Trader: "taker", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit,
		Price: 59000, Quantity: 1,
	})

	if len(trades) != 0 {
		t.Fatalf("expected no trades, got %d", len(trades))
	}
	if taker.Status != StatusOpen {
		t.Fatalf("expected the order to rest, got %q", taker.Status)
	}

	book, _ := e.Book("BTC/RUSD")
	if book.BestBid() != 59000 {
		t.Fatalf("expected best bid 59000, got %v", book.BestBid())
	}
	if book.BestAsk() != 60000 {
		t.Fatalf("expected best ask 60000, got %v", book.BestAsk())
	}
}

// TestCancelEnforcesOwnership verifies one trader cannot cancel another's order.
func TestCancelEnforcesOwnership(t *testing.T) {
	e := testEngine(t)

	placed, _ := mustSubmit(t, e, Order{
		Trader: "alice", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit,
		Price: 60000, Quantity: 1,
	})

	if _, err := e.Cancel(placed.OrderID, "mallory"); !errors.Is(err, ErrNotOwner) {
		t.Fatalf("expected ErrNotOwner, got %v", err)
	}

	book, _ := e.Book("BTC/RUSD")
	if book.OpenOrders() != 1 {
		t.Fatalf("the order must still be resting, got %d", book.OpenOrders())
	}

	if _, err := e.Cancel(placed.OrderID, "alice"); err != nil {
		t.Fatalf("owner cancel failed: %v", err)
	}
	if book.OpenOrders() != 0 {
		t.Fatalf("expected an empty book after cancel, got %d", book.OpenOrders())
	}
}

// TestSelfTradePrevention verifies a trader never fills against their own
// resting order.
func TestSelfTradePrevention(t *testing.T) {
	e := testEngine(t)

	mustSubmit(t, e, Order{Trader: "alice", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 60000, Quantity: 1})
	mustSubmit(t, e, Order{Trader: "bob", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 60000, Quantity: 1})

	// Alice buys: her own ask must be skipped, so she trades with Bob.
	alice, trades := mustSubmit(t, e, Order{
		Trader: "alice", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit,
		Price: 60000, Quantity: 1,
	})

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if trades[0].Maker != "bob" {
		t.Fatalf("expected to trade with bob, got %q", trades[0].Maker)
	}
	if alice.Status != StatusFilled {
		t.Fatalf("expected alice filled, got %q", alice.Status)
	}

	// Alice's own ask was cancelled by self-trade prevention.
	book, _ := e.Book("BTC/RUSD")
	if book.OpenOrders() != 0 {
		t.Fatalf("expected an empty book, got %d resting orders", book.OpenOrders())
	}
}

// TestUnlistedPairIsRejected verifies the engine refuses a pair that was never
// listed.
func TestUnlistedPairIsRejected(t *testing.T) {
	e := testEngine(t)

	_, _, err := e.Submit(Order{
		Trader: "alice", Pair: "DOGE/RUSD", Side: SideBuy, Type: TypeLimit,
		Price: 1, Quantity: 1,
	})
	if !errors.Is(err, ErrUnknownPair) {
		t.Fatalf("expected ErrUnknownPair, got %v", err)
	}
}

// TestValidationRejectsBadInput verifies the engine refuses malformed orders
// rather than trading them.
func TestValidationRejectsBadInput(t *testing.T) {
	e := testEngine(t)

	cases := []struct {
		name  string
		order Order
		want  error
	}{
		{"bad side", Order{Trader: "a", Pair: "BTC/RUSD", Side: "up", Type: TypeLimit, Price: 1, Quantity: 1}, ErrBadSide},
		{"bad type", Order{Trader: "a", Pair: "BTC/RUSD", Side: SideBuy, Type: "iceberg", Price: 1, Quantity: 1}, ErrBadType},
		{"bad tif", Order{Trader: "a", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit, Price: 1, Quantity: 1, TIF: "XYZ"}, ErrBadTIF},
		{"zero price", Order{Trader: "a", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit, Price: 0, Quantity: 1}, ErrBadPrice},
		{"negative price", Order{Trader: "a", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit, Price: -1, Quantity: 1}, ErrBadPrice},
		{"zero quantity", Order{Trader: "a", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit, Price: 1, Quantity: 0}, ErrBadQuantity},
		{"negative quantity", Order{Trader: "a", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit, Price: 1, Quantity: -5}, ErrBadQuantity},
		{"nan quantity", Order{Trader: "a", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit, Price: 1, Quantity: math.NaN()}, ErrBadQuantity},
		{"inf price", Order{Trader: "a", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit, Price: math.Inf(1), Quantity: 1}, ErrBadPrice},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := e.Submit(tc.order); !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

// TestQuantityRangeIsEnforced verifies a pair's configured quantity bounds are
// applied.
func TestQuantityRangeIsEnforced(t *testing.T) {
	e := NewEngine()
	if err := e.RegisterPair(PairConfig{
		Pair:        "TGR/RUSD",
		MinQuantity: 10,
		MaxQuantity: 1000,
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	if _, _, err := e.Submit(Order{Trader: "a", Pair: "TGR/RUSD", Side: SideBuy, Type: TypeLimit, Price: 1, Quantity: 5}); !errors.Is(err, ErrQtyRange) {
		t.Fatalf("expected ErrQtyRange for a small order, got %v", err)
	}
	if _, _, err := e.Submit(Order{Trader: "a", Pair: "TGR/RUSD", Side: SideBuy, Type: TypeLimit, Price: 1, Quantity: 5000}); !errors.Is(err, ErrQtyRange) {
		t.Fatalf("expected ErrQtyRange for a large order, got %v", err)
	}
	if _, _, err := e.Submit(Order{Trader: "a", Pair: "TGR/RUSD", Side: SideBuy, Type: TypeLimit, Price: 1, Quantity: 100}); err != nil {
		t.Fatalf("an in-range order must be accepted: %v", err)
	}
}

// TestFeesAreComputed verifies maker and taker fees come from the pair config.
func TestFeesAreComputed(t *testing.T) {
	e := NewEngine()
	if err := e.RegisterPair(PairConfig{Pair: "ETH/RUSD", MakerFeeBps: 0, TakerFeeBps: 2}); err != nil {
		t.Fatalf("register: %v", err)
	}

	mustSubmit(t, e, Order{Trader: "maker", Pair: "ETH/RUSD", Side: SideSell, Type: TypeLimit, Price: 2000, Quantity: 1})
	_, trades := mustSubmit(t, e, Order{Trader: "taker", Pair: "ETH/RUSD", Side: SideBuy, Type: TypeLimit, Price: 2000, Quantity: 1})

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	// 2 bps of a 2000 notional is 0.4.
	if math.Abs(trades[0].TakerFee-0.4) > 1e-9 {
		t.Fatalf("expected taker fee 0.4, got %v", trades[0].TakerFee)
	}
	if trades[0].MakerFee != 0 {
		t.Fatalf("expected maker fee 0, got %v", trades[0].MakerFee)
	}
	if trades[0].Notional != 2000 {
		t.Fatalf("expected notional 2000, got %v", trades[0].Notional)
	}
}

// TestDepthAggregatesByPrice verifies depth groups orders by price and reports
// the aggregated quantity and order count.
func TestDepthAggregatesByPrice(t *testing.T) {
	e := testEngine(t)

	mustSubmit(t, e, Order{Trader: "a", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit, Price: 60000, Quantity: 1})
	mustSubmit(t, e, Order{Trader: "b", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit, Price: 60000, Quantity: 2})
	mustSubmit(t, e, Order{Trader: "c", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit, Price: 59900, Quantity: 1})
	mustSubmit(t, e, Order{Trader: "d", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 60100, Quantity: 5})

	book, _ := e.Book("BTC/RUSD")
	bids, asks := book.Depth(10)

	if len(bids) != 2 {
		t.Fatalf("expected 2 bid levels, got %d", len(bids))
	}
	if bids[0].Price != 60000 || bids[0].Quantity != 3 || bids[0].Orders != 2 {
		t.Fatalf("unexpected best bid level: %+v", bids[0])
	}
	if bids[1].Price != 59900 {
		t.Fatalf("expected second level 59900, got %v", bids[1].Price)
	}
	if len(asks) != 1 || asks[0].Price != 60100 || asks[0].Quantity != 5 {
		t.Fatalf("unexpected ask levels: %+v", asks)
	}
}

// TestTradeTapeIsBounded verifies the tape keeps the most recent trades and
// reports them oldest-first.
func TestTradeTapeIsBounded(t *testing.T) {
	e := testEngine(t)

	mustSubmit(t, e, Order{Trader: "maker", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 60000, Quantity: 10})
	for i := 0; i < 5; i++ {
		mustSubmit(t, e, Order{Trader: "taker", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit, Price: 60000, Quantity: 1})
	}

	book, _ := e.Book("BTC/RUSD")
	trades := book.Trades(0)
	if len(trades) != 5 {
		t.Fatalf("expected 5 trades, got %d", len(trades))
	}
	for i := 1; i < len(trades); i++ {
		if trades[i].Timestamp < trades[i-1].Timestamp {
			t.Fatalf("trades must be oldest-first, %d is out of order", i)
		}
	}
	if got := book.Trades(2); len(got) != 2 {
		t.Fatalf("expected the tape to honour a limit, got %d", len(got))
	}
}

// TestStatsCountTrades verifies the engine counters reflect real activity.
func TestStatsCountTrades(t *testing.T) {
	e := testEngine(t)

	mustSubmit(t, e, Order{Trader: "maker", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 60000, Quantity: 3})
	mustSubmit(t, e, Order{Trader: "taker", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit, Price: 60000, Quantity: 3})
	_, _, _ = e.Submit(Order{Trader: "x", Pair: "NOPE/RUSD", Side: SideBuy, Type: TypeLimit, Price: 1, Quantity: 1})

	stats := e.Stats()
	if stats.Submitted != 2 {
		t.Fatalf("expected 2 accepted orders, got %d", stats.Submitted)
	}
	if stats.Trades != 1 {
		t.Fatalf("expected 1 trade, got %d", stats.Trades)
	}
	if stats.Rejected != 1 {
		t.Fatalf("expected 1 rejected order, got %d", stats.Rejected)
	}
	if stats.ListedPairs != 1 {
		t.Fatalf("expected 1 listed pair, got %d", stats.ListedPairs)
	}
}

// TestOnTradeCallbackFires verifies the settlement hook receives every trade.
func TestOnTradeCallbackFires(t *testing.T) {
	e := testEngine(t)

	var mu sync.Mutex
	var seen []Trade
	e.OnTrade(func(tr Trade) {
		mu.Lock()
		seen = append(seen, tr)
		mu.Unlock()
	})

	mustSubmit(t, e, Order{Trader: "maker", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 60000, Quantity: 2})
	mustSubmit(t, e, Order{Trader: "taker", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit, Price: 60000, Quantity: 2})

	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 1 {
		t.Fatalf("expected 1 callback, got %d", len(seen))
	}
	if seen[0].TradeID == 0 {
		t.Fatal("expected the callback to receive an assigned trade id")
	}
}

// TestUnregisterPairStopsTrading verifies delisting a pair stops submissions.
func TestUnregisterPairStopsTrading(t *testing.T) {
	e := testEngine(t)

	if !e.UnregisterPair("BTC/RUSD") {
		t.Fatal("expected the pair to be listed")
	}
	if _, _, err := e.Submit(Order{Trader: "a", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit, Price: 1, Quantity: 1}); !errors.Is(err, ErrUnknownPair) {
		t.Fatalf("expected ErrUnknownPair after delisting, got %v", err)
	}
	if e.UnregisterPair("BTC/RUSD") {
		t.Fatal("expected the second unregister to report the pair was absent")
	}
}

// TestConcurrentSubmitIsRaceFree exercises the engine from many goroutines so
// the race detector can check the locking.
func TestConcurrentSubmitIsRaceFree(t *testing.T) {
	e := NewEngine()
	if err := e.RegisterPair(PairConfig{Pair: "BTC/RUSD", TakerFeeBps: 2}); err != nil {
		t.Fatalf("register: %v", err)
	}

	const workers = 8
	const perWorker = 200

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				side := SideBuy
				if (w+i)%2 == 0 {
					side = SideSell
				}
				_, _, _ = e.Submit(Order{
					Trader: "trader",
					Pair:   "BTC/RUSD",
					Side:   side,
					Type:   TypeLimit,
					Price:  float64(59000 + (i % 20)),
					Quantity: 1,
				})
			}
		}(w)
	}
	wg.Wait()

	// Every accepted order either rests or fully fills; the book must remain
	// internally consistent (no negative leaves, no empty levels).
	book, _ := e.Book("BTC/RUSD")
	bids, asks := book.Depth(1000)
	for _, l := range append(append([]Level{}, bids...), asks...) {
		if l.Quantity <= 0 {
			t.Fatalf("book contains a non-positive level: %+v", l)
		}
		if l.Orders <= 0 {
			t.Fatalf("book contains an empty level: %+v", l)
		}
	}
}

// TestIntegerPriceExactness verifies prices survive the integer round trip
// without drift, which a float comparison would not guarantee.
func TestIntegerPriceExactness(t *testing.T) {
	e := testEngine(t)

	mustSubmit(t, e, Order{Trader: "maker", Pair: "BTC/RUSD", Side: SideSell, Type: TypeLimit, Price: 0.1, Quantity: 0.3})
	taker, trades := mustSubmit(t, e, Order{Trader: "taker", Pair: "BTC/RUSD", Side: SideBuy, Type: TypeLimit, Price: 0.1, Quantity: 0.3})

	if len(trades) != 1 {
		t.Fatalf("expected 1 trade, got %d", len(trades))
	}
	if taker.Status != StatusFilled {
		t.Fatalf("expected an exact fill, got %q with leaves %v", taker.Status, taker.Leaves)
	}
	if trades[0].Quantity != 0.3 {
		t.Fatalf("expected quantity 0.3, got %v", trades[0].Quantity)
	}
}
