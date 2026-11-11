package main

import (
	"context"
	"math/big"
	"testing"

	"tigerswap/backend/go/router"
)

// TestAggregatorHubsParsesAndDedupes checks the router's bridge-token set comes
// from TIGERSWAP_HUBS, drops malformed entries and de-duplicates.
func TestAggregatorHubsParsesAndDedupes(t *testing.T) {
	t.Setenv("TIGERSWAP_HUBS", "0xAA00000000000000000000000000000000000001, garbage ,0xaa00000000000000000000000000000000000001,0xbb00000000000000000000000000000000000002")
	hubs := aggregatorHubs()
	if len(hubs) != 2 {
		t.Fatalf("expected 2 deduped hubs, got %d (%v)", len(hubs), hubs)
	}
	if hubs[0] != "0xaa00000000000000000000000000000000000001" {
		t.Fatalf("hub[0] = %q, want lower-cased address", hubs[0])
	}
	if hubs[1] != "0xbb00000000000000000000000000000000000002" {
		t.Fatalf("hub[1] = %q", hubs[1])
	}
}

// TestAggregatorHubsEmptyWhenUnset checks the default is no bridging, so the
// search never routes through a token the operator did not approve.
func TestAggregatorHubsEmptyWhenUnset(t *testing.T) {
	t.Setenv("TIGERSWAP_HUBS", "")
	if hubs := aggregatorHubs(); hubs != nil {
		t.Fatalf("expected no hubs, got %v", hubs)
	}
}

// TestGetBestQuoteRejectsBadInput checks the input guards run before any chain
// lookup.
func TestGetBestQuoteRejectsBadInput(t *testing.T) {
	a := &Aggregator{chains: map[int64]*chainDeployment{}}
	ctx := context.Background()

	if _, err := a.GetBestQuote(ctx, 1, "0x0000000000000000000000000000000000000001", "0x0000000000000000000000000000000000000002", big.NewInt(0), 0.5, 3); err == nil {
		t.Fatalf("expected error for zero amount")
	}
	if _, err := a.GetBestQuote(ctx, 1, "not-an-address", "0x0000000000000000000000000000000000000002", big.NewInt(100), 0.5, 3); err == nil {
		t.Fatalf("expected error for malformed from token")
	}
	if _, err := a.GetBestQuote(ctx, 1, "0x0000000000000000000000000000000000000001", "0xzz", big.NewInt(100), 0.5, 3); err == nil {
		t.Fatalf("expected error for malformed to token")
	}
}

// TestGetBestQuoteRequiresConfiguredChain checks an unknown chain is an explicit
// error, never an invented quote.
func TestGetBestQuoteRequiresConfiguredChain(t *testing.T) {
	a := &Aggregator{chains: map[int64]*chainDeployment{}}
	_, err := a.GetBestQuote(context.Background(), 999, "0x0000000000000000000000000000000000000001", "0x0000000000000000000000000000000000000002", big.NewInt(100), 0.5, 3)
	if err == nil {
		t.Fatalf("expected error for an unconfigured chain")
	}
}

// TestHopsOfConvertsRoute pins the wire hop DTO: each router hop becomes a
// token pair plus the family and pool to trade it on, in order.
func TestHopsOfConvertsRoute(t *testing.T) {
	route := &router.Route{Hops: []router.Hop{
		{From: "0xa", To: "0xb", Family: "tigerswap_v2", Pool: "0xp1"},
		{From: "0xb", To: "0xc", Family: "tigerswap_stable", Pool: "0xp2"},
	}}
	hops := hopsOf(route)
	if len(hops) != 2 {
		t.Fatalf("expected 2 hops, got %d", len(hops))
	}
	if hops[0].TokenIn != "0xa" || hops[0].TokenOut != "0xb" || hops[0].Family != "tigerswap_v2" || hops[0].Pool != "0xp1" {
		t.Fatalf("hop[0] = %+v", hops[0])
	}
	if hops[1].Family != "tigerswap_stable" || hops[1].Pool != "0xp2" {
		t.Fatalf("hop[1] = %+v", hops[1])
	}
	if hopsOf(nil) != nil {
		t.Fatalf("expected nil hops for a nil route")
	}
}
