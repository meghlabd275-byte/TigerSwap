// Package router implements TigerSwap's own multi-hop Smart Router.
//
// It searches the graph of TigerSwap pools for the best path between two
// tokens, folds the real per-hop AMM math along each candidate path, and can
// split a single order across several paths to reduce price impact. Every hop
// is priced through the Market interface, which reads live pool state; the
// package itself holds no prices and never invents one.
//
// Nothing here calls a third-party DEX, router or aggregator. The pool graph
// is built from TigerSwap's own factory and pool contracts.
package router

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strings"
)

// MaxRoutes bounds how many candidate paths the search will consider. A larger
// bound costs more on-chain calls for a diminishing chance of a better price.
const MaxRoutes = 64

// Hop is one tradable edge: a single pool that swaps `From` for `To`.
//
// Price returns the output for a positive input amount and must read the pool's
// live state. A hop that cannot price (empty pool, reverted call) returns an
// error rather than a zero, so a broken pool never wins the route search.
type Hop struct {
	Pool   string
	Family string
	FeeBps uint32
	From   string
	To     string
	Price  func(ctx context.Context, amountIn *big.Int) (*big.Int, error)
}

// Market enumerates the tradable hops between two tokens. An implementation
// reads TigerSwap's own factory/pool contracts and returns one Hop per family
// that has a pool for the pair.
type Market interface {
	Hops(ctx context.Context, from, to string) ([]Hop, error)
}

// Route is an ordered list of hops from a source token to a destination token.
type Route struct {
	Hops []Hop
}

// TokenPath returns the token sequence the route trades through.
func (r *Route) TokenPath() []string {
	if len(r.Hops) == 0 {
		return nil
	}
	path := make([]string, 0, len(r.Hops)+1)
	path = append(path, r.Hops[0].From)
	for _, hop := range r.Hops {
		path = append(path, hop.To)
	}
	return path
}

// Key uniquely identifies the route by its pool/family sequence, so two routes
// are only merged when they trade through exactly the same pools.
func (r *Route) Key() string {
	parts := make([]string, 0, len(r.Hops))
	for _, hop := range r.Hops {
		parts = append(parts, strings.ToLower(hop.Pool)+":"+hop.Family)
	}
	return strings.Join(parts, "|")
}

// Quote folds the real AMM math along the route and returns the final output.
// A hop that yields no output aborts the whole quote, because a route that
// cannot complete is not tradable.
func Quote(ctx context.Context, route *Route, amountIn *big.Int) (*big.Int, error) {
	if amountIn == nil || amountIn.Sign() <= 0 {
		return nil, errors.New("amount in must be positive")
	}
	if len(route.Hops) == 0 {
		return nil, errors.New("route has no hops")
	}

	current := new(big.Int).Set(amountIn)
	for i, hop := range route.Hops {
		if hop.Price == nil {
			return nil, fmt.Errorf("hop %d (%s -> %s) has no price function", i, hop.From, hop.To)
		}
		out, err := hop.Price(ctx, current)
		if err != nil {
			return nil, fmt.Errorf("hop %d (%s -> %s): %w", i, hop.From, hop.To, err)
		}
		if out == nil || out.Sign() <= 0 {
			return nil, fmt.Errorf("hop %d (%s -> %s) produced no output", i, hop.From, hop.To)
		}
		current = out
	}
	return current, nil
}

// Router searches the pool graph for the best route between two tokens.
type Router struct {
	market  Market
	maxHops int
	// hubs are the intermediate tokens the search may route through, beyond the
	// two endpoints. Restricting the set keeps the number of on-chain pool
	// lookups bounded while still covering the usual high-liquidity bridges.
	hubs []string
	// maxRoutes bounds how many candidate paths are collected.
	maxRoutes int
}

// New builds a router. maxHops is clamped to at least 1 and at most 4; hubs are
// de-duplicated case-insensitively. A nil market is rejected on first use.
func New(market Market, hubs []string, maxHops int) *Router {
	if maxHops < 1 {
		maxHops = 1
	}
	if maxHops > 4 {
		maxHops = 4
	}

	seen := make(map[string]bool, len(hubs))
	clean := make([]string, 0, len(hubs))
	for _, hub := range hubs {
		key := strings.ToLower(strings.TrimSpace(hub))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		clean = append(clean, key)
	}

	return &Router{market: market, maxHops: maxHops, hubs: clean, maxRoutes: MaxRoutes}
}

// SetMaxRoutes lowers the candidate-route bound (useful for tests and for
// latency-sensitive callers). Values below 1 are ignored.
func (r *Router) SetMaxRoutes(n int) {
	if n >= 1 {
		r.maxRoutes = n
	}
}

// FindRoutes returns every candidate path from tokenIn to tokenOut within the
// hop limit, best-effort: a hop that cannot be read is skipped, not fatal.
func (r *Router) FindRoutes(ctx context.Context, tokenIn, tokenOut string) ([]*Route, error) {
	if r.market == nil {
		return nil, errors.New("router has no market")
	}
	from := strings.ToLower(strings.TrimSpace(tokenIn))
	to := strings.ToLower(strings.TrimSpace(tokenOut))
	if from == "" || to == "" {
		return nil, errors.New("token in and token out are required")
	}
	if from == to {
		return nil, errors.New("token in and token out are identical")
	}

	// Candidate destinations for each step: the hubs plus the final token.
	candidates := make([]string, 0, len(r.hubs)+1)
	seen := map[string]bool{from: true, to: true}
	for _, hub := range r.hubs {
		if !seen[hub] {
			seen[hub] = true
			candidates = append(candidates, hub)
		}
	}
	candidates = append(candidates, to)

	var routes []*Route
	visited := map[string]bool{from: true}
	r.dfs(ctx, from, to, candidates, visited, nil, &routes)

	sort.SliceStable(routes, func(i, j int) bool {
		if len(routes[i].Hops) != len(routes[j].Hops) {
			return len(routes[i].Hops) < len(routes[j].Hops)
		}
		return routes[i].Key() < routes[j].Key()
	})
	return routes, nil
}

// dfs walks the graph depth-first. `candidates` is the ordered candidate set for
// each step; a candidate already on the current path is never revisited, which
// keeps the search acyclic and finite. The target is only ever appended, never
// recursed into, so every route in `out` ends at the target.
func (r *Router) dfs(ctx context.Context, current, target string, candidates []string, visited map[string]bool, path []Hop, out *[]*Route) {
	if len(*out) >= r.maxRoutes || len(path) >= r.maxHops {
		return
	}

	for _, next := range candidates {
		if len(*out) >= r.maxRoutes {
			return
		}
		if visited[next] {
			continue
		}

		hops, err := r.market.Hops(ctx, current, next)
		if err != nil || len(hops) == 0 {
			continue
		}
		for _, hop := range hops {
			if len(*out) >= r.maxRoutes {
				return
			}
			visited[next] = true
			path = append(path, hop)
			if next == target {
				*out = append(*out, &Route{Hops: append([]Hop(nil), path...)})
			} else {
				r.dfs(ctx, next, target, candidates, visited, path, out)
			}
			path = path[:len(path)-1]
			delete(visited, next)
		}
	}
}

// Quoted is one candidate route together with its output for a fixed input.
type Quoted struct {
	Route     *Route
	AmountIn  *big.Int
	AmountOut *big.Int
}

// BestRoute quotes every candidate and returns the single best one. It is the
// direct-route answer: one path, all of the input.
func (r *Router) BestRoute(ctx context.Context, tokenIn, tokenOut string, amountIn *big.Int) (*Quoted, error) {
	routes, err := r.FindRoutes(ctx, tokenIn, tokenOut)
	if err != nil {
		return nil, err
	}
	quoted := r.quoteAll(ctx, routes, amountIn)
	if len(quoted) == 0 {
		return nil, fmt.Errorf("no TigerSwap route from %s to %s", tokenIn, tokenOut)
	}
	return quoted[0], nil
}

// quoteAll prices each route and returns the quotable ones, sorted by output
// descending. Routes that cannot price are dropped rather than scored as zero.
func (r *Router) quoteAll(ctx context.Context, routes []*Route, amountIn *big.Int) []*Quoted {
	quoted := make([]*Quoted, 0, len(routes))
	for _, route := range routes {
		out, err := Quote(ctx, route, amountIn)
		if err != nil {
			continue
		}
		quoted = append(quoted, &Quoted{Route: route, AmountIn: new(big.Int).Set(amountIn), AmountOut: out})
	}
	sort.SliceStable(quoted, func(i, j int) bool {
		return quoted[i].AmountOut.Cmp(quoted[j].AmountOut) > 0
	})
	return quoted
}
