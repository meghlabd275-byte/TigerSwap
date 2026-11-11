package services

import (
	"math/big"
	"testing"

	"tigerswap/backend/go/amm"
)

const (
	addrA = "0x1111111111111111111111111111111111111111"
	addrB = "0x2222222222222222222222222222222222222222"
	addrC = "0x3333333333333333333333333333333333333333"
)

// TestFamilyFromNameMapsKnownNames checks every family a quote can name maps to
// its on-chain code and an unknown name is rejected rather than defaulted.
func TestFamilyFromNameMapsKnownNames(t *testing.T) {
	cases := map[string]amm.Family{
		"":                   amm.FamilyV2,
		"tigerswap_v2":       amm.FamilyV2,
		"xyk":                amm.FamilyV2,
		"tigerswap_v3":       amm.FamilyV3,
		"concentrated":       amm.FamilyV3,
		"tigerswap_stable":   amm.FamilyStable,
		"stableswap":         amm.FamilyStable,
		"tigerswap_weighted": amm.FamilyWeighted,
		"balancer":           amm.FamilyWeighted,
	}
	for name, want := range cases {
		got, err := familyFromName(name)
		if err != nil {
			t.Fatalf("familyFromName(%q): %v", name, err)
		}
		if got != want {
			t.Fatalf("familyFromName(%q) = %q, want %q", name, got, want)
		}
	}
	if _, err := familyFromName("uniswap_v3"); err == nil {
		t.Fatal("expected an unknown family to be rejected, not defaulted")
	}
}

// TestHopsFromWireRejectsMalformedHops checks the wire -> router-hop boundary
// validates every address and family before any calldata is built.
func TestHopsFromWireRejectsMalformedHops(t *testing.T) {
	exec := &SwapExecutor{}

	good, err := exec.hopsFromWire([]routeHopDTO{
		{TokenIn: addrA, TokenOut: addrB, Family: "tigerswap_v2"},
		{TokenIn: addrB, TokenOut: addrC, Family: "tigerswap_stable"},
	})
	if err != nil {
		t.Fatalf("hopsFromWire: %v", err)
	}
	if len(good) != 2 || good[1].Family != amm.FamilyStable {
		t.Fatalf("unexpected hops %+v", good)
	}

	if _, err := exec.hopsFromWire([]routeHopDTO{{TokenIn: "nope", TokenOut: addrB, Family: "tigerswap_v2"}}); err == nil {
		t.Fatal("expected an invalid token address to be rejected")
	}
	if _, err := exec.hopsFromWire([]routeHopDTO{{TokenIn: addrA, TokenOut: addrB, Family: "sushi_v2"}}); err == nil {
		t.Fatal("expected an unknown family to be rejected")
	}
}

// TestSplitLegsFromWireRejectsInvalidLegs checks a split leg with a bad amount
// or an unknown family is rejected instead of producing a call that cannot fill.
func TestSplitLegsFromWireRejectsInvalidLegs(t *testing.T) {
	exec := &SwapExecutor{}

	legs, err := exec.splitLegsFromWire([]routeLegDTO{
		{Path: []string{addrA, addrB}, Family: "tigerswap_v2", AmountIn: "100"},
		{
			AmountIn: "50",
			Hops: []routeHopDTO{
				{TokenIn: addrA, TokenOut: addrC, Family: "tigerswap_v2"},
				{TokenIn: addrC, TokenOut: addrB, Family: "tigerswap_stable"},
			},
		},
	})
	if err != nil {
		t.Fatalf("splitLegsFromWire: %v", err)
	}
	if len(legs) != 2 {
		t.Fatalf("expected 2 legs, got %d", len(legs))
	}
	if legs[0].AmountIn.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("leg 0 amount = %s, want 100", legs[0].AmountIn)
	}
	// The hop-based leg keeps its full path, not just the endpoints.
	if len(legs[1].Hops) != 2 || legs[1].Hops[1].Family != amm.FamilyStable {
		t.Fatalf("leg 1 hops = %+v", legs[1].Hops)
	}

	if _, err := exec.splitLegsFromWire([]routeLegDTO{{Path: []string{addrA, addrB}, Family: "tigerswap_v2", AmountIn: "0"}}); err == nil {
		t.Fatal("expected a non-positive leg amount to be rejected")
	}
	if _, err := exec.splitLegsFromWire([]routeLegDTO{{Path: []string{addrA, addrB}, Family: "nope", AmountIn: "5"}}); err == nil {
		t.Fatal("expected an unknown leg family to be rejected")
	}
	if _, err := exec.splitLegsFromWire([]routeLegDTO{{Path: []string{addrA}, Family: "tigerswap_v2", AmountIn: "5"}}); err == nil {
		t.Fatal("expected a leg with no path to be rejected")
	}
}
