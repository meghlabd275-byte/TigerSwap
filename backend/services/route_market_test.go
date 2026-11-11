package services

import (
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"tigerswap/backend/go/router"
)

func bigFromString(t *testing.T, s string) *big.Int {
	t.Helper()
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		t.Fatalf("bad int %q", s)
	}
	return v
}

// TestRouteHubsUsesRegisteredTokens checks the router's hub set is built from
// TigerSwap's own token registry and bounded, so the path search stays cheap.
func TestRouteHubsUsesRegisteredTokens(t *testing.T) {
	hubs := routeHubs(1) // Ethereum
	if len(hubs) == 0 {
		t.Fatalf("expected hubs for chain 1")
	}
	if len(hubs) > routeHubLimit {
		t.Fatalf("expected at most %d hubs, got %d", routeHubLimit, len(hubs))
	}
	for _, hub := range hubs {
		if !strings.HasPrefix(hub, "0x") {
			t.Fatalf("hub %q is not an address", hub)
		}
		if hub != strings.ToLower(hub) {
			t.Fatalf("hub %q is not lower-cased", hub)
		}
	}
	// The native asset must be present as a routing hub.
	if hubs[0] != "0x0000000000000000000000000000000000000000" {
		t.Fatalf("expected native asset first, got %q", hubs[0])
	}
}

// TestRouteHubsHonoursOverride checks an operator override replaces the
// registry-derived set and filters out malformed entries.
func TestRouteHubsHonoursOverride(t *testing.T) {
	t.Setenv("TIGERSWAP_ROUTE_HUBS", "0xAbC0000000000000000000000000000000000001,not-an-address,0xabc0000000000000000000000000000000000001")
	hubs := routeHubs(1)
	if len(hubs) != 1 {
		t.Fatalf("expected 1 deduped hub, got %d (%v)", len(hubs), hubs)
	}
	if hubs[0] != "0xabc0000000000000000000000000000000000001" {
		t.Fatalf("unexpected hub %q", hubs[0])
	}
}

// TestRouteSourceLabel checks the source label reports hop count.
func TestRouteSourceLabel(t *testing.T) {
	one := &router.Route{Hops: []router.Hop{{Pool: "p1", Family: "tigerswap_v2"}}}
	two := &router.Route{Hops: []router.Hop{
		{Pool: "p1", Family: "tigerswap_v2"},
		{Pool: "p2", Family: "tigerswap_stable"},
	}}
	if got := routeSourceLabel(one); got != "tiger_router_1hop" {
		t.Fatalf("expected tiger_router_1hop, got %q", got)
	}
	if got := routeSourceLabel(two); got != "tiger_router_2hop" {
		t.Fatalf("expected tiger_router_2hop, got %q", got)
	}
}

// TestRouteLegsFromSplitConverts checks split legs are converted to wire DTOs
// with the final hop's family and pool.
func TestRouteLegsFromSplitConverts(t *testing.T) {
	route := &router.Route{Hops: []router.Hop{
		{Pool: "p1", Family: "tigerswap_v2", From: "0xa", To: "0xb"},
		{Pool: "p2", Family: "tigerswap_stable", From: "0xb", To: "0xc"},
	}}
	legs := routeLegsFromSplit([]router.SplitLeg{{
		Route:     route,
		AmountIn:  bigFromString(t, "100"),
		AmountOut: bigFromString(t, "95"),
	}})
	if len(legs) != 1 {
		t.Fatalf("expected 1 leg, got %d", len(legs))
	}
	leg := legs[0]
	if leg.Family != "tigerswap_stable" || leg.Pool != "p2" {
		t.Fatalf("unexpected leg family/pool: %s/%s", leg.Family, leg.Pool)
	}
	if len(leg.Path) != 3 || leg.Path[0] != "0xa" || leg.Path[2] != "0xc" {
		t.Fatalf("unexpected leg path %v", leg.Path)
	}
	if leg.AmountIn != "100" || leg.AmountOut != "95" {
		t.Fatalf("unexpected leg amounts %s/%s", leg.AmountIn, leg.AmountOut)
	}
}

// TestSwapQuoteDTOLegsOmittedWhenAbsent pins the wire contract: a single-route
// quote carries no `legs` key, and a split quote does. The frontend validator
// treats `legs` as optional, so this is the boundary it relies on.
func TestSwapQuoteDTOLegsOmittedWhenAbsent(t *testing.T) {
	plain := swapQuoteDTO{
		AmountIn: "1", AmountOut: "2", AmountOutMin: "2",
		PriceImpact: 0, Route: []string{"0xa", "0xb"}, GasEstimate: 100000,
	}
	raw, err := json.Marshal(plain)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "legs") {
		t.Fatalf("expected no legs key for a single-route quote: %s", raw)
	}

	split := plain
	split.Legs = []routeLegDTO{{
		Path: []string{"0xa", "0xb"}, Family: "tigerswap_v2", Pool: "0xp",
		AmountIn: "1", AmountOut: "2",
	}}
	raw, err = json.Marshal(split)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"legs"`) {
		t.Fatalf("expected a legs key for a split quote: %s", raw)
	}
}
