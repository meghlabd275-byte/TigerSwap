package services

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	"tigerswap/backend/config"
	"tigerswap/backend/go/amm"
	"tigerswap/backend/go/router"
	"tigerswap/backend/go/tigerswap"
)

// tigerMarket adapts a TigerSwap on-chain client to router.Market. It
// enumerates the families that have a pool for a pair and hands the router a
// price function that reads the pool's live state through QuoteFamily. A family
// with no pool is simply absent from the result, so the search never routes
// through a pool that does not exist.
type tigerMarket struct {
	client *tigerswap.Client
}

// Hops returns one router.Hop per TigerSwap family that has a pool for the
// pair. Every hop prices through the pool's real reserves.
func (m tigerMarket) Hops(ctx context.Context, from, to string) ([]router.Hop, error) {
	if !common.IsHexAddress(from) || !common.IsHexAddress(to) {
		return nil, fmt.Errorf("invalid token address")
	}
	fromAddr := common.HexToAddress(from)
	toAddr := common.HexToAddress(to)

	families := make([]amm.Family, 0, len(tigerFamilyFees))
	for family := range tigerFamilyFees {
		families = append(families, family)
	}
	sort.Slice(families, func(i, j int) bool { return families[i] < families[j] })

	var hops []router.Hop
	for _, family := range families {
		fee := tigerFamilyFees[family]
		pool, err := m.client.ResolvePool(ctx, fromAddr, toAddr, family)
		if err != nil {
			continue
		}

		family, fee, pool := family, fee, pool
		hops = append(hops, router.Hop{
			Pool:   pool.Hex(),
			Family: string(family),
			FeeBps: fee,
			From:   from,
			To:     to,
			Price: func(ctx context.Context, amountIn *big.Int) (*big.Int, error) {
				return m.client.QuoteFamily(ctx, family, []common.Address{fromAddr, toAddr}, amountIn, fee)
			},
		})
	}

	if len(hops) == 0 {
		return nil, fmt.Errorf("no TigerSwap pool for %s/%s", from, to)
	}
	return hops, nil
}

// routeHubLimit bounds how many intermediate tokens the router may bridge
// through. Each extra hub multiplies the candidate-path count, so the set is
// kept small: the high-liquidity quote assets are the useful bridges.
const routeHubLimit = 6

// routeHubs returns the intermediate tokens the router may route through on a
// chain. Operators can override the set with TIGERSWAP_ROUTE_HUBS (a
// comma-separated list of token addresses); otherwise the chain's registered
// tokens are used, which are the assets TigerSwap itself lists.
func routeHubs(chainID int64) []string {
	if override := strings.TrimSpace(os.Getenv("TIGERSWAP_ROUTE_HUBS")); override != "" {
		parts := strings.Split(override, ",")
		hubs := make([]string, 0, len(parts))
		seen := map[string]bool{}
		for _, part := range parts {
			addr := strings.ToLower(strings.TrimSpace(part))
			if !common.IsHexAddress(addr) || seen[addr] {
				continue
			}
			seen[addr] = true
			hubs = append(hubs, addr)
		}
		if len(hubs) > 0 {
			return hubs
		}
	}

	var hubs []string
	seen := map[string]bool{}
	add := func(addr string) {
		key := strings.ToLower(strings.TrimSpace(addr))
		if key == "" || seen[key] {
			return
		}
		seen[key] = true
		hubs = append(hubs, key)
	}

	if chain, ok := config.GetBlockchainConfigs()[chainID]; ok && chain.NativeToken != nil {
		addr := chain.NativeToken.Address
		if addr == "" {
			addr = "0x0000000000000000000000000000000000000000"
		}
		add(addr)
	}
	for _, token := range config.GetTokenConfigs()[chainID] {
		add(token.Address)
	}

	if len(hubs) > routeHubLimit {
		hubs = hubs[:routeHubLimit]
	}
	return hubs
}

// newTigerRouter builds a multi-hop router for a chain, or returns nil when no
// client can be created. A nil router makes the aggregator fall back to direct
// quotes rather than failing the whole request.
func (p *PriceAggregator) newTigerRouter(chainID int64) *router.Router {
	client, err := p.tigerClient(chainID)
	if err != nil {
		return nil
	}
	return router.New(tigerMarket{client: client}, routeHubs(chainID), 3)
}

// routeQuote is the router's answer for a request: the best path, its output,
// the executable hop plan, and the split legs when the input was divided across
// several routes.
type routeQuote struct {
	amountOut *big.Int
	path      []string
	hops      []routeHopDTO
	source    string
	legs      []router.SplitLeg
	split     bool
}

// bestMultiHopQuote searches TigerSwap's own pool graph for the best path from
// fromToken to toToken and returns the winning route. It reports ok=false when
// no router can be built or when no multi-hop path beats the caller's direct
// quote, so the aggregator can keep the direct answer.
//
// The split is only surfaced when it beats a single path: splitting a small
// order across pools almost always loses to one clean hop, and a caller should
// never pay extra gas for a worse fill.
func (p *PriceAggregator) bestMultiHopQuote(ctx context.Context, chainID int64, fromToken, toToken string, amountIn, directOut *big.Int) (*routeQuote, bool) {
	tigerRouter := p.newTigerRouter(chainID)
	if tigerRouter == nil {
		return nil, false
	}

	best, err := tigerRouter.BestQuote(ctx, fromToken, toToken, amountIn, 0)
	if err != nil || best == nil || best.Route == nil {
		return nil, false
	}

	quote := &routeQuote{
		amountOut: best.AmountOut,
		path:      best.Route.TokenPath(),
		hops:      hopsFromRoute(best.Route),
		source:    routeSourceLabel(best.Route),
	}
	if best.Split {
		quote.legs = best.Legs
		quote.split = true
		quote.source = "tiger_router_split"
	}

	if directOut != nil && quote.amountOut.Cmp(directOut) <= 0 {
		return nil, false
	}
	return quote, true
}

// routeSourceLabel names a route by its hop count so a caller can tell a direct
// quote from a multi-hop one.
func routeSourceLabel(route *router.Route) string {
	if route == nil {
		return "tiger_router"
	}
	return fmt.Sprintf("tiger_router_%dhop", len(route.Hops))
}
