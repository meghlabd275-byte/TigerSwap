package orderbook

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
)

// TestSelectorsMatchDeployedContract pins the embedded ABI to the compiled
// contract's selectors. A change to the Solidity signature changes the
// selector, so this fails loudly instead of sending calldata the chain would
// route to the wrong function (or nowhere).
func TestSelectorsMatchDeployedContract(t *testing.T) {
	want := map[string]string{
		"placeOrder(bytes32,uint256,uint256,bool,uint8,bool)": "400790c1",
		"cancelOrder(uint256)":                                "514fcac7",
		"deposit(bytes32,uint256)":                            "1de26e16",
		"withdraw(bytes32,uint256)":                           "040cf020",
		"liquidate(bytes32,address)":                          "771b51d2",
		"addLiquidity(bytes32,uint256)":                       "3b57e2bc",
		"removeLiquidity(bytes32,uint256)":                    "01bd4fa4",
		"depositInsurance(bytes32,uint256)":                   "7cb6028e",
		"createMarket(bytes32,address,address,uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256,uint256)": "c96f76dc",
		"setMarketActive(bytes32,bool)":          "87b87fb9",
		"updatePrice(bytes32,uint256,uint256)":   "c724a933",
		"accrueFunding(bytes32,uint256,uint256)": "9e2181e7",
		"getMarket(bytes32)":                     "c3c95c7b",
		"getPosition(bytes32,address)":           "5c388821",
		"getOrder(uint256)":                      "d09ef241",
		"getTraderOrders(address)":               "ce3ca00b",
		"getFundingInfo(bytes32)":                "84acda6a",
		"bestBidAsk(bytes32)":                    "954a2726",
		"depth(bytes32,uint256)":                 "eebe50af",
		"balances(bytes32,address)":              "d93d7361",
		"poolLiquidity(bytes32)":                 "c55ed851",
		"orderMarket(uint256)":                   "cf549c8a",
		"claimUnpaid(bytes32)":                   "479a3d80",
		"unpaidCredit(bytes32,address)":          "2e31af06",
	}
	for sig, hexSel := range want {
		got := orderBookABI.Methods[methodName(sig)].ID
		if hex.EncodeToString(got) != hexSel {
			t.Errorf("%s selector = %s, want %s", sig, hex.EncodeToString(got), hexSel)
		}
	}
}

func methodName(sig string) string {
	for i := 0; i < len(sig); i++ {
		if sig[i] == '(' {
			return sig[:i]
		}
	}
	return sig
}

// TestMarketIDMatchesKeccak pins the market id derivation to keccak256(symbol),
// the convention the contract and the perps client both use.
func TestMarketIDMatchesKeccak(t *testing.T) {
	got := MarketID("BTC/RUSD")
	want := crypto.Keccak256Hash([]byte("BTC/RUSD"))
	if got != want {
		t.Fatalf("MarketID = %s, want %s", got.Hex(), want.Hex())
	}
}

// TestPlaceOrderCalldataRoundTrips encodes a placeOrder call and decodes it back,
// so a signature change is caught by the decoder rather than only by a selector.
func TestPlaceOrderCalldataRoundTrips(t *testing.T) {
	marketID := MarketID("BTC/RUSD")
	call, err := (&Client{deployment: Deployment{OrderBook: [20]byte{1, 2, 3}}}).BuildPlaceOrder(
		marketID, big.NewInt(1e18), new(big.Int).Mul(big.NewInt(60000), big.NewInt(1e18)), true, TIFPostOnly, false,
	)
	if err != nil {
		t.Fatalf("BuildPlaceOrder: %v", err)
	}
	if len(call.Data) < 4 {
		t.Fatalf("calldata too short: %d", len(call.Data))
	}
	gotSelector := hex.EncodeToString(call.Data[:4])
	if gotSelector != "400790c1" {
		t.Fatalf("selector = %s, want 400790c1", gotSelector)
	}
	args, err := orderBookABI.Methods["placeOrder"].Inputs.Unpack(call.Data[4:])
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	if args[0].([32]byte) != marketID {
		t.Errorf("market id round-trip mismatch")
	}
	if args[1].(*big.Int).Cmp(big.NewInt(1e18)) != 0 {
		t.Errorf("quantity round-trip mismatch: %v", args[1])
	}
	if args[4].(uint8) != TIFPostOnly {
		t.Errorf("tif round-trip mismatch: %v", args[4])
	}
}

// TestOrderFilledEventUnpacks pins the OrderFilled event shape to the indexed
// orderId the indexer reads from topics[1]. The indexer decodes the non-indexed
// tail into (price, quantity, isMaker, fee); a shape change would make every
// fill silently drop, so this builds a real log and unpacks it.
func TestOrderFilledEventUnpacks(t *testing.T) {
	event := orderBookABI.Events["OrderFilled"]
	if event.ID == ([32]byte{}) {
		t.Fatal("OrderFilled event missing from ABI")
	}
	if len(event.Inputs) != 7 {
		t.Fatalf("OrderFilled has %d inputs, want 7", len(event.Inputs))
	}
	if !event.Inputs[0].Indexed || event.Inputs[0].Name != "orderId" {
		t.Fatalf("first input = %s indexed=%v, want indexed orderId", event.Inputs[0].Name, event.Inputs[0].Indexed)
	}
	if event.Inputs[3].Indexed || event.Inputs[3].Name != "price" {
		t.Fatalf("fourth input = %s indexed=%v, want non-indexed price", event.Inputs[3].Name, event.Inputs[3].Indexed)
	}

	data, err := event.Inputs.NonIndexed().Pack(
		mustBig(t, "60000000000000000000000"), // price
		big.NewInt(1e18),                      // quantity
		true,                                  // isMaker
		mustBig(t, "12000000000000000000"),    // fee
	)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	var decoded struct {
		Price    *big.Int
		Quantity *big.Int
		IsMaker  bool
		Fee      *big.Int
	}
	if err := orderBookABI.UnpackIntoInterface(&decoded, "OrderFilled", data); err != nil {
		t.Fatalf("unpack: %v", err)
	}
	if decoded.Price.Cmp(mustBig(t, "60000000000000000000000")) != 0 {
		t.Errorf("price = %v", decoded.Price)
	}
	if !decoded.IsMaker {
		t.Errorf("isMaker = false, want true")
	}
	if decoded.Fee.Cmp(mustBig(t, "12000000000000000000")) != 0 {
		t.Errorf("fee = %v", decoded.Fee)
	}
}

func mustBig(t *testing.T, s string) *big.Int {
	t.Helper()
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		t.Fatalf("bad integer %q", s)
	}
	return v
}
