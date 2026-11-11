package router

import (
	"context"
	"errors"
	"fmt"
	"math/big"
)

// SplitLeg is one part of a split order: a route and the input it carries.
type SplitLeg struct {
	Route     *Route
	AmountIn  *big.Int
	AmountOut *big.Int
}

// Split is a single order divided across several routes.
type Split struct {
	TotalIn  *big.Int
	TotalOut *big.Int
	Legs     []SplitLeg
}

// SplitOrder divides amountIn across independent routes to reduce price impact.
//
// Each AMM output curve is concave in its input, so the maximum total output is
// reached by repeatedly giving the next slice of input to the route with the
// best marginal output. Only routes whose pools are pairwise disjoint are
// considered, so no two legs share a pool and the per-route prices stay exact.
//
// slices is the number of equal input increments; larger is closer to optimal
// and costs proportionally more price evaluations. A non-positive slices value
// uses defaultSlices.
const defaultSlices = 20

// maxSplitRoutes bounds how many independent routes a split will use. Beyond a
// handful the marginal gain is negligible and the extra legs only add gas.
const maxSplitRoutes = 4

func (r *Router) SplitOrder(ctx context.Context, tokenIn, tokenOut string, amountIn *big.Int, slices int) (*Split, error) {
	if amountIn == nil || amountIn.Sign() <= 0 {
		return nil, errors.New("amount in must be positive")
	}
	if slices <= 0 {
		slices = defaultSlices
	}

	routes, err := r.FindRoutes(ctx, tokenIn, tokenOut)
	if err != nil {
		return nil, err
	}

	quoted := r.quoteAll(ctx, routes, amountIn)
	if len(quoted) == 0 {
		return nil, fmt.Errorf("no TigerSwap route from %s to %s", tokenIn, tokenOut)
	}
	return splitFromQuoted(ctx, quoted, amountIn, slices), nil
}

// BestQuote is the router's final answer for an order: the winning route and
// its output, plus the split legs when dividing the input across independent
// routes beat any single path. It performs one graph search and one round of
// quoting, so a caller pays for the search exactly once.
type BestQuote struct {
	Route     *Route
	AmountOut *big.Int
	Legs      []SplitLeg
	Split     bool
}

// BestQuote searches once and returns the better of the best single route and
// the best split, so a caller never trades a split that loses to one clean hop.
func (r *Router) BestQuote(ctx context.Context, tokenIn, tokenOut string, amountIn *big.Int, slices int) (*BestQuote, error) {
	if amountIn == nil || amountIn.Sign() <= 0 {
		return nil, errors.New("amount in must be positive")
	}
	if slices <= 0 {
		slices = defaultSlices
	}

	routes, err := r.FindRoutes(ctx, tokenIn, tokenOut)
	if err != nil {
		return nil, err
	}
	quoted := r.quoteAll(ctx, routes, amountIn)
	if len(quoted) == 0 {
		return nil, fmt.Errorf("no TigerSwap route from %s to %s", tokenIn, tokenOut)
	}

	best := quoted[0]
	out := &BestQuote{Route: best.Route, AmountOut: best.AmountOut}

	split := splitFromQuoted(ctx, quoted, amountIn, slices)
	if split != nil && len(split.Legs) > 1 && split.TotalOut.Cmp(out.AmountOut) > 0 {
		out.AmountOut = split.TotalOut
		out.Legs = split.Legs
		out.Split = true
		// A split has no single path; report the highest-output leg as the route
		// so a caller that only understands one path still gets the best one.
		if len(split.Legs) > 0 {
			out.Route = split.Legs[0].Route
		}
	}
	return out, nil
}

// splitFromQuoted runs the greedy marginal allocation over an already-quoted
// route set. It never re-searches the graph, so a caller that has already paid
// for FindRoutes and quoteAll does not pay again.
func splitFromQuoted(ctx context.Context, quoted []*Quoted, amountIn *big.Int, slices int) *Split {
	if len(quoted) == 0 {
		return nil
	}

	independent := selectIndependent(quoted)
	if len(independent) == 1 {
		return &Split{
			TotalIn:  new(big.Int).Set(amountIn),
			TotalOut: new(big.Int).Set(independent[0].AmountOut),
			Legs: []SplitLeg{{
				Route:     independent[0].Route,
				AmountIn:  new(big.Int).Set(amountIn),
				AmountOut: new(big.Int).Set(independent[0].AmountOut),
			}},
		}
	}

	// Greedy marginal allocation. current[i] is the input allocated so far to
	// route i and output[i] the output that input produces.
	n := len(independent)
	current := make([]*big.Int, n)
	output := make([]*big.Int, n)
	for i := range current {
		current[i] = big.NewInt(0)
		output[i] = big.NewInt(0)
	}

	step := new(big.Int).Div(new(big.Int).Set(amountIn), big.NewInt(int64(slices)))
	if step.Sign() <= 0 {
		step = big.NewInt(1)
	}

	remaining := new(big.Int).Set(amountIn)
	for remaining.Sign() > 0 {
		slice := new(big.Int).Set(step)
		if slice.Cmp(remaining) > 0 {
			slice = new(big.Int).Set(remaining)
		}

		bestIdx := -1
		var bestGain *big.Int
		for i := range independent {
			next := new(big.Int).Add(current[i], slice)
			out, err := Quote(ctx, independent[i].Route, next)
			if err != nil {
				continue
			}
			gain := new(big.Int).Sub(out, output[i])
			if bestIdx == -1 || gain.Cmp(bestGain) > 0 {
				bestIdx = i
				bestGain = gain
			}
		}
		if bestIdx == -1 {
			// No route can absorb more input; return what we have.
			break
		}

		current[bestIdx].Add(current[bestIdx], slice)
		output[bestIdx].Add(output[bestIdx], bestGain)
		remaining.Sub(remaining, slice)
	}

	split := &Split{TotalIn: new(big.Int).Set(amountIn), TotalOut: big.NewInt(0)}
	for i := range independent {
		if current[i].Sign() == 0 {
			continue
		}
		split.TotalOut.Add(split.TotalOut, output[i])
		split.Legs = append(split.Legs, SplitLeg{
			Route:     independent[i].Route,
			AmountIn:  new(big.Int).Set(current[i]),
			AmountOut: new(big.Int).Set(output[i]),
		})
	}
	if len(split.Legs) == 0 {
		// Allocation produced nothing usable; fall back to the single best route.
		return &Split{
			TotalIn:  new(big.Int).Set(amountIn),
			TotalOut: new(big.Int).Set(quoted[0].AmountOut),
			Legs: []SplitLeg{{
				Route:     quoted[0].Route,
				AmountIn:  new(big.Int).Set(amountIn),
				AmountOut: new(big.Int).Set(quoted[0].AmountOut),
			}},
		}
	}
	return split
}

// selectIndependent greedily builds a maximal set of routes that share no pool,
// taking higher-output routes first. The input is assumed sorted by output
// descending.
func selectIndependent(quoted []*Quoted) []*Quoted {
	used := make(map[string]bool)
	chosen := make([]*Quoted, 0, maxSplitRoutes)
	for _, q := range quoted {
		if len(chosen) >= maxSplitRoutes {
			break
		}
		if routeSharesPool(q.Route, used) {
			continue
		}
		for _, hop := range q.Route.Hops {
			used[poolKey(hop)] = true
		}
		chosen = append(chosen, q)
	}
	return chosen
}

func routeSharesPool(route *Route, used map[string]bool) bool {
	for _, hop := range route.Hops {
		if used[poolKey(hop)] {
			return true
		}
	}
	return false
}

func poolKey(hop Hop) string {
	return hop.Pool + ":" + hop.Family
}
