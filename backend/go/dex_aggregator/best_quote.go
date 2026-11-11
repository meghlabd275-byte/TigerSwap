package main

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"tigerswap/backend/go/amm"
	"tigerswap/backend/go/router"
)

// aggMarket adapts a chainDeployment to router.Market so the TigerAggregator can
// search TigerSwap's own pool graph for the best multi-hop path. Every hop is
// priced through the same on-chain reads the direct quote uses, so a path the
// router finds is a real route and not an estimate.
type aggMarket struct {
	aggregator *Aggregator
	deployment *chainDeployment
	families   []amm.Family
}

// Hops returns one router.Hop per TigerSwap family that has a pool for the pair.
func (m aggMarket) Hops(ctx context.Context, from, to string) ([]router.Hop, error) {
	if !common.IsHexAddress(from) || !common.IsHexAddress(to) {
		return nil, fmt.Errorf("invalid token address")
	}
	fromAddr := common.HexToAddress(from)
	toAddr := common.HexToAddress(to)

	families := make([]amm.Family, len(m.families))
	copy(families, m.families)
	sort.Slice(families, func(i, j int) bool { return families[i] < families[j] })

	var hops []router.Hop
	for _, family := range families {
		family := family
		pool, err := m.deployment.resolvePool(ctx, fromAddr, toAddr, family)
		if err != nil {
			continue
		}
		fee := familyFees[family]
		hops = append(hops, router.Hop{
			Pool:   pool.Hex(),
			Family: string(family),
			FeeBps: fee,
			From:   from,
			To:     to,
			Price: func(ctx context.Context, amountIn *big.Int) (*big.Int, error) {
				return m.aggregator.quoteFamily(ctx, m.deployment, family, []common.Address{fromAddr, toAddr}, amountIn)
			},
		})
	}
	if len(hops) == 0 {
		return nil, fmt.Errorf("no TigerSwap pool for %s/%s", from, to)
	}
	return hops, nil
}

// aggregatorHubs returns the intermediate tokens the router may bridge through
// on a chain. It is read from TIGERSWAP_HUBS (a comma-separated list of token
// addresses); with none configured the router only considers direct pairs.
func aggregatorHubs() []string {
	raw := strings.TrimSpace(os.Getenv("TIGERSWAP_HUBS"))
	if raw == "" {
		return nil
	}
	var hubs []string
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		addr := strings.ToLower(strings.TrimSpace(part))
		if !common.IsHexAddress(addr) || seen[addr] {
			continue
		}
		seen[addr] = true
		hubs = append(hubs, addr)
	}
	return hubs
}

// BestQuoteResponse is the answer to a best-path quote request. It carries the
// winning path and, when the router split the order, the legs it divided the
// input across.
type BestQuoteResponse struct {
	FromToken    string           `json:"from_token"`
	ToToken      string           `json:"to_token"`
	AmountIn     *big.Int         `json:"amount_in"`
	AmountOut    *big.Int         `json:"amount_out"`
	AmountOutMin *big.Int         `json:"amount_out_min"`
	PriceImpact  float64          `json:"price_impact"`
	GasEstimate  uint64           `json:"gas_estimate"`
	Path         []string         `json:"path"`
	BestRoute    *Route           `json:"best_route"`
	Hops         []HopResult      `json:"hops,omitempty"`
	Split        bool             `json:"split"`
	Legs         []SplitLegResult `json:"legs,omitempty"`
}

// SplitLegResult is one leg of a split order on the wire.
type SplitLegResult struct {
	Family      string      `json:"family"`
	Path        []string    `json:"path"`
	Hops        []HopResult `json:"hops,omitempty"`
	AmountIn    *big.Int    `json:"amount_in"`
	AmountOut   *big.Int    `json:"amount_out"`
	GasEstimate uint64      `json:"gas_estimate"`
}

// HopResult is one executable hop: the token pair and the pool family to trade
// it on. A client builds router calldata directly from it.
type HopResult struct {
	TokenIn  string `json:"token_in"`
	TokenOut string `json:"token_out"`
	Family   string `json:"family"`
	Pool     string `json:"pool"`
}

// hopsOf converts a router route into wire hops.
func hopsOf(route *router.Route) []HopResult {
	if route == nil || len(route.Hops) == 0 {
		return nil
	}
	hops := make([]HopResult, 0, len(route.Hops))
	for _, hop := range route.Hops {
		hops = append(hops, HopResult{
			TokenIn:  hop.From,
			TokenOut: hop.To,
			Family:   hop.Family,
			Pool:     hop.Pool,
		})
	}
	return hops
}

// GetBestQuote searches TigerSwap's own pool graph for the best path from
// fromToken to toToken and returns it, splitting the order across independent
// routes when that yields more output. It fails closed: with no configured
// hubs the search degenerates to the direct pairs, which is exactly what
// GetQuote already covers, so callers that only want a direct quote should use
// GetQuote.
func (a *Aggregator) GetBestQuote(ctx context.Context, chainID int64, fromToken, toToken string, amountIn *big.Int, slippage float64, maxHops int) (*BestQuoteResponse, error) {
	if amountIn == nil || amountIn.Sign() <= 0 {
		return nil, fmt.Errorf("amount_in must be positive")
	}
	if !common.IsHexAddress(fromToken) || !common.IsHexAddress(toToken) {
		return nil, fmt.Errorf("invalid token address")
	}

	a.mu.RLock()
	d, ok := a.chains[chainID]
	a.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("chain %d has no TigerSwap deployment configured", chainID)
	}

	if maxHops < 1 {
		maxHops = 3
	}
	r := router.New(aggMarket{aggregator: a, deployment: d, families: a.families}, aggregatorHubs(), maxHops)

	best, err := r.BestQuote(ctx, fromToken, toToken, amountIn, 0)
	if err != nil {
		return nil, err
	}

	path := best.Route.TokenPath()
	family := ""
	if len(best.Route.Hops) > 0 {
		family = best.Route.Hops[len(best.Route.Hops)-1].Family
	}

	gas := estimateGas(amm.Family(family))
	if len(path) > 2 {
		gas += uint64(len(path)-2) * 50000
	}

	resp := &BestQuoteResponse{
		FromToken:    fromToken,
		ToToken:      toToken,
		AmountIn:     new(big.Int).Set(amountIn),
		AmountOut:    best.AmountOut,
		AmountOutMin: amm.ApplySlippage(best.AmountOut, slippage),
		PriceImpact:  amm.PriceImpactPercent(amountIn, best.AmountOut),
		GasEstimate:  gas,
		Path:         path,
		Split:        best.Split,
		BestRoute: &Route{
			Family:      family,
			Path:        path,
			AmountIn:    new(big.Int).Set(amountIn),
			AmountOut:   best.AmountOut,
			FeeBps:      familyFees[amm.Family(family)],
			GasEstimate: gas,
		},
	}

	if !best.Split {
		resp.Hops = hopsOf(best.Route)
	}

	if best.Split {
		for _, leg := range best.Legs {
			legPath := leg.Route.TokenPath()
			legFamily := ""
			if len(leg.Route.Hops) > 0 {
				legFamily = leg.Route.Hops[len(leg.Route.Hops)-1].Family
			}
			resp.Legs = append(resp.Legs, SplitLegResult{
				Family:      legFamily,
				Path:        legPath,
				Hops:        hopsOf(leg.Route),
				AmountIn:    leg.AmountIn,
				AmountOut:   leg.AmountOut,
				GasEstimate: estimateGas(amm.Family(legFamily)),
			})
		}
	}

	return resp, nil
}
