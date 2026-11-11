package tigerswap

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	"tigerswap/backend/go/amm"
)

// TestBuildMultihopCalldataRoundTrip pins the multi-hop calldata to the on-chain
// router ABI: the selector must match the declared function and the arguments
// must decode back to exactly the hops the caller passed, with each family mapped
// to its on-chain code.
func TestBuildMultihopCalldataRoundTrip(t *testing.T) {
	router := common.HexToAddress("0x00000000000000000000000000000000000000aa")
	c := &Client{deployment: Deployment{ChainID: 1, Router: router}}

	a := common.HexToAddress("0x0000000000000000000000000000000000000001")
	b := common.HexToAddress("0x0000000000000000000000000000000000000002")
	d := common.HexToAddress("0x0000000000000000000000000000000000000003")
	recipient := common.HexToAddress("0x00000000000000000000000000000000000000ff")

	hops := []MultihopHop{
		{TokenIn: a, TokenOut: b, Family: amm.FamilyV2},
		{TokenIn: b, TokenOut: d, Family: amm.FamilyStable},
	}
	amountIn := big.NewInt(1_000)
	amountOutMin := big.NewInt(900)
	mins := []*big.Int{big.NewInt(990), big.NewInt(905)}
	deadline := big.NewInt(2_000_000_000)

	target, data, err := c.BuildMultihopCalldata(hops, amountIn, amountOutMin, mins, recipient, deadline)
	if err != nil {
		t.Fatalf("BuildMultihopCalldata: %v", err)
	}
	if target != router {
		t.Fatalf("target = %s, want %s", target, router)
	}

	method, ok := routerABI.Methods["swapExactTokensForTokensMultihop"]
	if !ok {
		t.Fatal("router ABI is missing swapExactTokensForTokensMultihop")
	}
	if len(data) < 4 || string(data[:4]) != string(method.ID) {
		t.Fatalf("selector mismatch: got %x want %x", data[:4], method.ID)
	}

	args, err := method.Inputs.Unpack(data[4:])
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	// amountIn, amountOutMin, tokens, families, minOutPerHop, to, deadline
	gotIn, ok := args[0].(*big.Int)
	if !ok || gotIn.Cmp(amountIn) != 0 {
		t.Fatalf("amountIn = %v, want %v", args[0], amountIn)
	}
	gotMin, ok := args[1].(*big.Int)
	if !ok || gotMin.Cmp(amountOutMin) != 0 {
		t.Fatalf("amountOutMin = %v, want %v", args[1], amountOutMin)
	}
	tokens, ok := args[2].([]common.Address)
	if !ok || len(tokens) != 3 {
		t.Fatalf("tokens = %v, want 3 addresses", args[2])
	}
	if tokens[0] != a || tokens[1] != b || tokens[2] != d {
		t.Fatalf("token path = %v, want [%s %s %s]", tokens, a, b, d)
	}
	families, ok := args[3].([]uint8)
	if !ok || len(families) != 2 {
		t.Fatalf("families = %v", args[3])
	}
	if families[0] != 0 || families[1] != 2 {
		t.Fatalf("family codes = %v, want [0 2]", families)
	}
	gotMins, ok := args[4].([]*big.Int)
	if !ok || len(gotMins) != 2 {
		t.Fatalf("minOutPerHop = %v", args[4])
	}
	if gotMins[0].Cmp(mins[0]) != 0 || gotMins[1].Cmp(mins[1]) != 0 {
		t.Fatalf("minOutPerHop = %v, want %v", gotMins, mins)
	}
	if to, ok := args[5].(common.Address); !ok || to != recipient {
		t.Fatalf("to = %v, want %s", args[5], recipient)
	}
}

// TestBuildMultihopCalldataRejectsEmpty proves a route with no hops is refused
// rather than encoded as a call that would revert on-chain.
func TestBuildMultihopCalldataRejectsEmpty(t *testing.T) {
	c := &Client{deployment: Deployment{ChainID: 1, Router: common.HexToAddress("0xaa")}}
	if _, _, err := c.BuildMultihopCalldata(nil, big.NewInt(1), big.NewInt(0), nil, common.Address{}, big.NewInt(0)); err == nil {
		t.Fatal("expected an error for an empty route")
	}
}

// TestBuildSplitCalldataRoundTrip pins the split calldata to the ABI and proves
// the per-leg amounts survive the nested array encoding.
func TestBuildSplitCalldataRoundTrip(t *testing.T) {
	router := common.HexToAddress("0x00000000000000000000000000000000000000bb")
	c := &Client{deployment: Deployment{ChainID: 1, Router: router}}

	a := common.HexToAddress("0x0000000000000000000000000000000000000011")
	b := common.HexToAddress("0x0000000000000000000000000000000000000012")
	d := common.HexToAddress("0x0000000000000000000000000000000000000013")
	recipient := common.HexToAddress("0x00000000000000000000000000000000000000fe")

	legs := []SplitLeg{
		{Hops: []MultihopHop{{TokenIn: a, TokenOut: d, Family: amm.FamilyV2}}, AmountIn: big.NewInt(600)},
		{Hops: []MultihopHop{{TokenIn: a, TokenOut: b, Family: amm.FamilyV2}, {TokenIn: b, TokenOut: d, Family: amm.FamilyV3}}, AmountIn: big.NewInt(400)},
	}
	amountIn := big.NewInt(1000)
	amountOutMin := big.NewInt(950)

	_, data, err := c.BuildSplitCalldata(amountIn, amountOutMin, legs, recipient, big.NewInt(2_000_000_000))
	if err != nil {
		t.Fatalf("BuildSplitCalldata: %v", err)
	}

	method, ok := routerABI.Methods["swapExactTokensForTokensSplit"]
	if !ok {
		t.Fatal("router ABI is missing swapExactTokensForTokensSplit")
	}
	if string(data[:4]) != string(method.ID) {
		t.Fatalf("selector mismatch: got %x want %x", data[:4], method.ID)
	}
	args, err := method.Inputs.Unpack(data[4:])
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	amounts, ok := args[4].([]*big.Int)
	if !ok || len(amounts) != 2 {
		t.Fatalf("amountInPerLeg = %v", args[4])
	}
	if amounts[0].Cmp(big.NewInt(600)) != 0 || amounts[1].Cmp(big.NewInt(400)) != 0 {
		t.Fatalf("amountInPerLeg = %v, want [600 400]", amounts)
	}
	families, ok := args[3].([][]uint8)
	if !ok || len(families) != 2 || len(families[1]) != 2 {
		t.Fatalf("families = %v", args[3])
	}
	if families[1][0] != 0 || families[1][1] != 1 {
		t.Fatalf("leg 1 families = %v, want [0 1]", families[1])
	}
}
