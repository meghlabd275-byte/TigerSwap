package main

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"

	"tigerswap/backend/go/amm"
)

// rpcStub answers eth_call with a fixed, ABI-encoded uint256 and records the
// calldata it received, so the real quote path can be exercised end to end.
type rpcStub struct {
	t       *testing.T
	reply   *big.Int
	seen    []byte
	failRPC bool
}

func (s *rpcStub) serve(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID     json.RawMessage   `json:"id"`
		Method string            `json:"method"`
		Params []json.RawMessage `json:"params"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.t.Errorf("decode request: %v", err)
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if s.failRPC {
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      req.ID,
			"error":   map[string]interface{}{"code": -32000, "message": "execution reverted"},
		})
		return
	}

	if req.Method == "eth_call" && len(req.Params) > 0 {
		var call struct {
			Data  string `json:"data"`
			Input string `json:"input"`
		}
		if err := json.Unmarshal(req.Params[0], &call); err != nil {
			s.t.Errorf("decode call object: %v", err)
		}
		payload := call.Data
		if payload == "" {
			payload = call.Input
		}
		raw, err := hex.DecodeString(strings.TrimPrefix(payload, "0x"))
		if err != nil {
			s.t.Errorf("decode calldata: %v", err)
		}
		s.seen = raw
	}

	result := "0x" + hex.EncodeToString(common.LeftPadBytes(s.reply.Bytes(), 32))
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result":  result,
	})
}

func newStubClient(t *testing.T, stub *rpcStub) *ethclient.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(stub.serve))
	t.Cleanup(srv.Close)
	client, err := ethclient.Dial(srv.URL)
	if err != nil {
		t.Fatalf("dial stub rpc: %v", err)
	}
	t.Cleanup(client.Close)
	return client
}

func TestQuoteV3RequiresConfiguredQuoter(t *testing.T) {
	d := &chainDeployment{chainID: 1, quoter: common.Address{}}
	if _, err := d.quoteV3(t.Context(), common.HexToAddress("0x01"), common.HexToAddress("0x02"), big.NewInt(1000), 5); err == nil {
		t.Fatal("an unconfigured quoter must be an error, not a silent zero quote")
	}
}

func TestQuoteV3ReturnsQuoterOutput(t *testing.T) {
	stub := &rpcStub{t: t, reply: big.NewInt(987654321)}
	d := &chainDeployment{
		chainID: 1,
		client:  newStubClient(t, stub),
		quoter:  common.HexToAddress("0x00000000000000000000000000000000000000aa"),
	}

	tokenIn := common.HexToAddress("0x00000000000000000000000000000000000000bb")
	tokenOut := common.HexToAddress("0x00000000000000000000000000000000000000cc")

	got, err := d.quoteV3(t.Context(), tokenIn, tokenOut, big.NewInt(5000), 5)
	if err != nil {
		t.Fatalf("quoteV3: %v", err)
	}
	if got.String() != "987654321" {
		t.Fatalf("quote = %s, want 987654321", got)
	}

	// The calldata must be the quoter selector followed by the four arguments.
	if len(stub.seen) < 4+32*4 {
		t.Fatalf("calldata too short: %d bytes", len(stub.seen))
	}
	if got := common.Bytes2Hex(stub.seen[:4]); got != hex.EncodeToString(quoterABI.Methods["quoteExactInputSingle"].ID) {
		t.Fatalf("selector = %s, want the quoteExactInputSingle selector", got)
	}
	if arg := common.BytesToAddress(stub.seen[4:36]); arg != tokenIn {
		t.Errorf("arg0 = %s, want %s", arg, tokenIn)
	}
	if arg := common.BytesToAddress(stub.seen[36:68]); arg != tokenOut {
		t.Errorf("arg1 = %s, want %s", arg, tokenOut)
	}
	if fee := new(big.Int).SetBytes(stub.seen[68:100]); fee.Uint64() != 5 {
		t.Errorf("fee arg = %s, want 5 (hundredths of a bip)", fee)
	}
	if amt := new(big.Int).SetBytes(stub.seen[100:132]); amt.Uint64() != 5000 {
		t.Errorf("amountIn arg = %s, want 5000", amt)
	}
}

func TestQuoteV3RejectsZeroOutput(t *testing.T) {
	stub := &rpcStub{t: t, reply: big.NewInt(0)}
	d := &chainDeployment{
		chainID: 1,
		client:  newStubClient(t, stub),
		quoter:  common.HexToAddress("0x00000000000000000000000000000000000000aa"),
	}

	if _, err := d.quoteV3(t.Context(), common.HexToAddress("0x01"), common.HexToAddress("0x02"), big.NewInt(1000), 5); err == nil {
		t.Fatal("a zero quote must be rejected rather than returned as a real price")
	}
}

func TestQuoteV3SurfacesRPCFailure(t *testing.T) {
	stub := &rpcStub{t: t, reply: big.NewInt(1), failRPC: true}
	d := &chainDeployment{
		chainID: 1,
		client:  newStubClient(t, stub),
		quoter:  common.HexToAddress("0x00000000000000000000000000000000000000aa"),
	}

	if _, err := d.quoteV3(t.Context(), common.HexToAddress("0x01"), common.HexToAddress("0x02"), big.NewInt(1000), 5); err == nil {
		t.Fatal("an RPC failure must not be swallowed")
	}
}

func TestFamilyFeesCoverEveryAggregatedFamily(t *testing.T) {
	for _, family := range []amm.Family{amm.FamilyV2, amm.FamilyV3, amm.FamilyStable, amm.FamilyWeighted} {
		if _, ok := familyFees[family]; !ok {
			t.Errorf("family %s has no configured fee", family)
		}
	}
}
