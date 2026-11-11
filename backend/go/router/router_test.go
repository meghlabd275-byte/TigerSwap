package router

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"
)

// stubPool is an in-memory constant-product pool used to exercise the router's
// path search and split math without an RPC endpoint. It performs the same
// x*y=k math the on-chain pool does, so a route's output is a real result and
// not a canned number.
type stubPool struct {
	pool       string
	family     string
	feeBps     uint32
	reserveIn  *big.Int
	reserveOut *big.Int
	broken     bool
	priceCalls int
	failAt     *big.Int
}

func (p *stubPool) price(_ context.Context, amountIn *big.Int) (*big.Int, error) {
	p.priceCalls++
	if p.broken {
		return nil, errors.New("pool is broken")
	}
	if p.failAt != nil && amountIn.Cmp(p.failAt) >= 0 {
		return nil, errors.New("insufficient liquidity")
	}
	if amountIn.Sign() <= 0 {
		return nil, errors.New("amount must be positive")
	}
	// x*y=k with fee: out = inWithFee * rOut / (rIn*10000 + inWithFee)
	inWithFee := new(big.Int).Mul(amountIn, big.NewInt(int64(10000-p.feeBps)))
	numerator := new(big.Int).Mul(inWithFee, p.reserveOut)
	denominator := new(big.Int).Add(new(big.Int).Mul(p.reserveIn, big.NewInt(10000)), inWithFee)
	out := new(big.Int).Div(numerator, denominator)
	if out.Sign() <= 0 {
		return nil, errors.New("no output")
	}
	return out, nil
}

// stubMarket builds a graph from named pools. Each pool connects two tokens and
// carries a family so two families can serve the same pair.
type stubMarket struct {
	// pools[from][to] = hops available for that direction.
	pools map[string]map[string][]Hop
}

func newStubMarket() *stubMarket {
	return &stubMarket{pools: map[string]map[string][]Hop{}}
}

func (m *stubMarket) add(a, b, pool, family string, feeBps uint32, reserveA, reserveB *big.Int) *stubPool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	pa := &stubPool{pool: pool, family: family, feeBps: feeBps, reserveIn: reserveA, reserveOut: reserveB}
	pb := &stubPool{pool: pool, family: family, feeBps: feeBps, reserveIn: reserveB, reserveOut: reserveA}

	hopAB := Hop{Pool: pool, Family: family, FeeBps: feeBps, From: a, To: b, Price: pa.price}
	hopBA := Hop{Pool: pool, Family: family, FeeBps: feeBps, From: b, To: a, Price: pb.price}

	if m.pools[a] == nil {
		m.pools[a] = map[string][]Hop{}
	}
	if m.pools[b] == nil {
		m.pools[b] = map[string][]Hop{}
	}
	m.pools[a][b] = append(m.pools[a][b], hopAB)
	m.pools[b][a] = append(m.pools[b][a], hopBA)
	return pa
}

func (m *stubMarket) Hops(_ context.Context, from, to string) ([]Hop, error) {
	byTo, ok := m.pools[strings.ToLower(from)]
	if !ok {
		return nil, errors.New("no pools for token")
	}
	hops, ok := byTo[strings.ToLower(to)]
	if !ok || len(hops) == 0 {
		return nil, errors.New("no pool for pair")
	}
	return hops, nil
}

func bi(s string) *big.Int {
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("bad int " + s)
	}
	return v
}

// TestFindRoutesFindsMultiHop checks the search discovers a two-hop path that
// only exists through an intermediate token.
func TestFindRoutesFindsMultiHop(t *testing.T) {
	m := newStubMarket()
	// A -> B -> C, with no direct A/C pool.
	m.add("A", "B", "poolAB", "tigerswap_v2", 30, bi("1000000"), bi("1000000"))
	m.add("B", "C", "poolBC", "tigerswap_v2", 30, bi("1000000"), bi("1000000"))

	r := New(m, []string{"B"}, 3)
	routes, err := r.FindRoutes(context.Background(), "A", "C")
	if err != nil {
		t.Fatalf("FindRoutes: %v", err)
	}
	if len(routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes))
	}
	path := routes[0].TokenPath()
	if len(path) != 3 || path[0] != "a" || path[1] != "b" || path[2] != "c" {
		t.Fatalf("unexpected path %v", path)
	}
}

// TestBestRoutePicksHigherOutput checks the search prefers the path with the
// larger real output when two paths exist.
func TestBestRoutePicksHigherOutput(t *testing.T) {
	m := newStubMarket()
	// Direct A->C through a thin pool, and A->B->C through deep pools.
	m.add("A", "C", "poolAC", "tigerswap_v2", 30, bi("1000"), bi("1000"))
	m.add("A", "B", "poolAB", "tigerswap_v2", 30, bi("1000000"), bi("1000000"))
	m.add("B", "C", "poolBC", "tigerswap_v2", 30, bi("1000000"), bi("1000000"))

	r := New(m, []string{"B"}, 3)
	best, err := r.BestRoute(context.Background(), "A", "C", bi("100"))
	if err != nil {
		t.Fatalf("BestRoute: %v", err)
	}
	if best.Route.Key() != "poolab:tigerswap_v2|poolbc:tigerswap_v2" {
		t.Fatalf("expected multi-hop route to win, got %s", best.Route.Key())
	}
	if best.AmountOut.Sign() <= 0 {
		t.Fatalf("expected positive output")
	}
}

// TestBestRouteKeepsDirectWhenBetter checks a deep direct pool beats a
// roundabout path.
func TestBestRouteKeepsDirectWhenBetter(t *testing.T) {
	m := newStubMarket()
	m.add("A", "C", "poolAC", "tigerswap_v2", 30, bi("1000000"), bi("1000000"))
	m.add("A", "B", "poolAB", "tigerswap_v2", 30, bi("100"), bi("100"))
	m.add("B", "C", "poolBC", "tigerswap_v2", 30, bi("100"), bi("100"))

	r := New(m, []string{"B"}, 3)
	best, err := r.BestRoute(context.Background(), "A", "C", bi("100"))
	if err != nil {
		t.Fatalf("BestRoute: %v", err)
	}
	if best.Route.Key() != "poolac:tigerswap_v2" {
		t.Fatalf("expected direct route, got %s", best.Route.Key())
	}
}

// TestFindRoutesRejectsIdenticalTokens guards against a self-route.
func TestFindRoutesRejectsIdenticalTokens(t *testing.T) {
	m := newStubMarket()
	r := New(m, nil, 3)
	if _, err := r.FindRoutes(context.Background(), "A", "A"); err == nil {
		t.Fatalf("expected error for identical tokens")
	}
}

// TestQuoteAbortsOnBrokenHop checks a route that cannot complete is an error,
// not a partial result.
func TestQuoteAbortsOnBrokenHop(t *testing.T) {
	m := newStubMarket()
	m.add("A", "B", "poolAB", "tigerswap_v2", 30, bi("1000"), bi("1000"))
	broken := m.add("B", "C", "poolBC", "tigerswap_v2", 30, bi("1000"), bi("1000"))
	broken.broken = true

	r := New(m, []string{"B"}, 3)
	if _, err := r.BestRoute(context.Background(), "A", "C", bi("100")); err == nil {
		t.Fatalf("expected error when a hop cannot price")
	}
}

// TestQuoteFoldsHops checks the output is the composition of the per-hop math.
func TestQuoteFoldsHops(t *testing.T) {
	m := newStubMarket()
	m.add("A", "B", "poolAB", "tigerswap_v2", 30, bi("1000"), bi("1000"))
	m.add("B", "C", "poolBC", "tigerswap_v2", 30, bi("1000"), bi("1000"))

	r := New(m, []string{"B"}, 3)
	best, err := r.BestRoute(context.Background(), "A", "C", bi("100"))
	if err != nil {
		t.Fatalf("BestRoute: %v", err)
	}

	// Hand-compute: hop1 out, then hop2 out.
	hop1 := new(big.Int).Div(
		new(big.Int).Mul(new(big.Int).Mul(bi("100"), big.NewInt(9970)), bi("1000")),
		new(big.Int).Add(new(big.Int).Mul(bi("1000"), big.NewInt(10000)), new(big.Int).Mul(bi("100"), big.NewInt(9970))),
	)
	hop2 := new(big.Int).Div(
		new(big.Int).Mul(new(big.Int).Mul(hop1, big.NewInt(9970)), bi("1000")),
		new(big.Int).Add(new(big.Int).Mul(bi("1000"), big.NewInt(10000)), new(big.Int).Mul(hop1, big.NewInt(9970))),
	)
	if best.AmountOut.Cmp(hop2) != 0 {
		t.Fatalf("output %s does not match folded math %s", best.AmountOut, hop2)
	}
}

// TestSplitOrderBeatsSingleRoute checks that splitting across two independent
// paths yields at least as much as either path alone, and conserves input.
func TestSplitOrderBeatsSingleRoute(t *testing.T) {
	m := newStubMarket()
	// Two independent A->C pools with identical, shallow liquidity. Splitting
	// the order must not lose to either single pool.
	m.add("A", "C", "poolAC1", "tigerswap_v2", 30, bi("10000"), bi("10000"))
	m.add("A", "C", "poolAC2", "tigerswap_weighted", 30, bi("10000"), bi("10000"))

	r := New(m, nil, 2)
	amount := bi("4000")
	best, err := r.BestRoute(context.Background(), "A", "C", amount)
	if err != nil {
		t.Fatalf("BestRoute: %v", err)
	}

	split, err := r.SplitOrder(context.Background(), "A", "C", amount, 20)
	if err != nil {
		t.Fatalf("SplitOrder: %v", err)
	}
	if split.TotalOut.Cmp(best.AmountOut) <= 0 {
		t.Fatalf("split output %s did not beat single route %s", split.TotalOut, best.AmountOut)
	}
	if len(split.Legs) != 2 {
		t.Fatalf("expected the order to split across 2 routes, got %d legs", len(split.Legs))
	}

	// Input must be conserved exactly across the legs.
	sum := big.NewInt(0)
	for _, leg := range split.Legs {
		sum.Add(sum, leg.AmountIn)
	}
	if sum.Cmp(amount) != 0 {
		t.Fatalf("split input %s does not equal requested %s", sum, amount)
	}
}

// TestSplitOrderNeverSharesPool checks the split only uses pool-disjoint routes
// so no leg's price depends on another leg.
func TestSplitOrderNeverSharesPool(t *testing.T) {
	m := newStubMarket()
	// A->B->C and A->B->D both use poolAB; a split must not use both.
	m.add("A", "B", "poolAB", "tigerswap_v2", 30, bi("10000"), bi("10000"))
	m.add("B", "C", "poolBC", "tigerswap_v2", 30, bi("10000"), bi("10000"))
	m.add("A", "C", "poolAC", "tigerswap_v2", 30, bi("10000"), bi("10000"))

	r := New(m, []string{"B"}, 3)
	split, err := r.SplitOrder(context.Background(), "A", "C", bi("5000"), 20)
	if err != nil {
		t.Fatalf("SplitOrder: %v", err)
	}

	used := map[string]bool{}
	for _, leg := range split.Legs {
		for _, hop := range leg.Route.Hops {
			key := poolKey(hop)
			if used[key] {
				t.Fatalf("pool %s used by two legs", key)
			}
			used[key] = true
		}
	}
}

// TestSplitOrderSingleRouteFallsBack checks that with only one usable path the
// split collapses to that single leg carrying the whole input.
func TestSplitOrderSingleRouteFallsBack(t *testing.T) {
	m := newStubMarket()
	m.add("A", "C", "poolAC", "tigerswap_v2", 30, bi("10000"), bi("10000"))

	r := New(m, nil, 2)
	amount := bi("1000")
	split, err := r.SplitOrder(context.Background(), "A", "C", amount, 20)
	if err != nil {
		t.Fatalf("SplitOrder: %v", err)
	}
	if len(split.Legs) != 1 {
		t.Fatalf("expected 1 leg, got %d", len(split.Legs))
	}
	if split.Legs[0].AmountIn.Cmp(amount) != 0 {
		t.Fatalf("expected full input on the single leg")
	}
}

// TestMaxHopsLimitsSearch checks the hop cap is honoured.
func TestMaxHopsLimitsSearch(t *testing.T) {
	m := newStubMarket()
	m.add("A", "B", "poolAB", "tigerswap_v2", 30, bi("1000"), bi("1000"))
	m.add("B", "C", "poolBC", "tigerswap_v2", 30, bi("1000"), bi("1000"))
	m.add("C", "D", "poolCD", "tigerswap_v2", 30, bi("1000"), bi("1000"))

	r := New(m, []string{"B", "C"}, 2)
	if _, err := r.BestRoute(context.Background(), "A", "D", bi("10")); err == nil {
		t.Fatalf("expected no route within 2 hops")
	}
}

// TestRouterRejectsNilMarket guards the fail-closed contract.
func TestRouterRejectsNilMarket(t *testing.T) {
	r := New(nil, nil, 2)
	if _, err := r.FindRoutes(context.Background(), "A", "B"); err == nil {
		t.Fatalf("expected error for nil market")
	}
}

// TestBestQuoteSplitsWhenBetter checks the combined answer prefers a split when
// it beats every single route, and reports the legs.
func TestBestQuoteSplitsWhenBetter(t *testing.T) {
	m := newStubMarket()
	m.add("A", "C", "poolAC1", "tigerswap_v2", 30, bi("10000"), bi("10000"))
	m.add("A", "C", "poolAC2", "tigerswap_weighted", 30, bi("10000"), bi("10000"))

	r := New(m, nil, 2)
	best, err := r.BestQuote(context.Background(), "A", "C", bi("4000"), 20)
	if err != nil {
		t.Fatalf("BestQuote: %v", err)
	}
	if !best.Split {
		t.Fatalf("expected a split to win")
	}
	if len(best.Legs) != 2 {
		t.Fatalf("expected 2 legs, got %d", len(best.Legs))
	}
	single, err := r.BestRoute(context.Background(), "A", "C", bi("4000"))
	if err != nil {
		t.Fatalf("BestRoute: %v", err)
	}
	if best.AmountOut.Cmp(single.AmountOut) <= 0 {
		t.Fatalf("BestQuote output %s did not beat best single route %s", best.AmountOut, single.AmountOut)
	}
}

// TestBestQuoteKeepsSingleWhenSplitWorse checks a lone route is returned whole
// rather than as a degenerate one-leg split.
func TestBestQuoteKeepsSingleWhenSplitWorse(t *testing.T) {
	m := newStubMarket()
	m.add("A", "C", "poolAC", "tigerswap_v2", 30, bi("1000000"), bi("1000000"))

	r := New(m, nil, 2)
	best, err := r.BestQuote(context.Background(), "A", "C", bi("1000"), 20)
	if err != nil {
		t.Fatalf("BestQuote: %v", err)
	}
	if best.Split {
		t.Fatalf("expected no split with a single usable route")
	}
	if len(best.Legs) != 0 {
		t.Fatalf("expected no legs, got %d", len(best.Legs))
	}
}

// TestBestQuoteRejectsNonPositive checks the input guard.
func TestBestQuoteRejectsNonPositive(t *testing.T) {
	m := newStubMarket()
	m.add("A", "C", "poolAC", "tigerswap_v2", 30, bi("1000"), bi("1000"))
	r := New(m, nil, 2)
	if _, err := r.BestQuote(context.Background(), "A", "C", big.NewInt(0), 20); err == nil {
		t.Fatalf("expected error for zero input")
	}
}
