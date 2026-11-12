package services

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// TestBridgeInboundDigestMatchesSolidity pins the off-chain digest to the exact
// value TigerSwapBridge.inboundDigest produces for the same message. The Solidity
// side asserts the same constant in testInboundDigestMatchesGoRelayer; if either
// encoding drifts, one of the two tests fails and the relayer would otherwise
// sign a digest the contract rejects.
func TestBridgeInboundDigestMatchesSolidity(t *testing.T) {
	msgIDBytes, _ := hex.DecodeString("1111111111111111111111111111111111111111111111111111111111111111")
	var msgID [32]byte
	copy(msgID[:], msgIDBytes)

	msg := bridgeMessage{
		MessageID: msgID,
		FromChain: big.NewInt(1),
		ToChain:   big.NewInt(8888),
		Token:     common.HexToAddress("0x0000000000000000000000000000000000000abc"),
		Amount:    big.NewInt(5000),
		Receiver:  common.HexToAddress("0x000000000000000000000000000000000000cafe"),
	}

	digest, err := bridgeInboundDigest(msg)
	if err != nil {
		t.Fatalf("bridgeInboundDigest: %v", err)
	}
	const want = "9f5a44e1a36e3e2d69d2da1946056f826ba29536cdbcb776fac74ea4b5cb0139"
	if got := hex.EncodeToString(digest); got != want {
		t.Fatalf("inbound digest = %s, want %s (Solidity/Go encoding drifted)", got, want)
	}
}

// TestBridgeABIMatchesContract proves the relayer's ABI declares exactly the
// entry points TigerSwapBridge.sol exposes. A mismatch is what let the old
// relayer build calldata the contract could not decode.
func TestBridgeABIMatchesContract(t *testing.T) {
	parsed, err := abi.JSON(strings.NewReader(tigerBridgeABI))
	if err != nil {
		t.Fatalf("parse tigerBridgeABI: %v", err)
	}
	for _, name := range []string{"lock", "completeBridge"} {
		if _, ok := parsed.Methods[name]; !ok {
			t.Fatalf("tigerBridgeABI is missing method %q", name)
		}
	}
	for _, name := range []string{"BridgeInitiated", "BridgeCompleted"} {
		if _, ok := parsed.Events[name]; !ok {
			t.Fatalf("tigerBridgeABI is missing event %q", name)
		}
	}
}

// TestParseBridgeInitiatedRejectsMismatch proves the relayer will not confirm a
// lock whose on-chain event disagrees with the intended transfer. Without this
// check a successful receipt for a different payload would be accepted.
func TestParseBridgeInitiatedRejectsMismatch(t *testing.T) {
	parsed, err := abi.JSON(strings.NewReader(tigerBridgeABI))
	if err != nil {
		t.Fatalf("parse ABI: %v", err)
	}

	router := common.HexToAddress("0x00000000000000000000000000000000000000b0")
	sender := common.HexToAddress("0x00000000000000000000000000000000000000a1")
	token := common.HexToAddress("0x0000000000000000000000000000000000000abc")
	recipient := common.HexToAddress("0x000000000000000000000000000000000000cafe")

	var msgID [32]byte
	copy(msgID[:], []byte("msg-id-32-bytes-................"))

	// Build a real BridgeInitiated log for the wrong recipient.
	data, err := parsed.Events["BridgeInitiated"].Inputs.NonIndexed().Pack(
		token, big.NewInt(5000), big.NewInt(2),
		common.HexToAddress("0x00000000000000000000000000000000000000ff"), msgID,
	)
	if err != nil {
		t.Fatalf("pack event: %v", err)
	}
	eventID := parsed.Events["BridgeInitiated"].ID
	receipt := &types.Receipt{Logs: []*types.Log{{
		Address: router,
		Topics:  []common.Hash{eventID, common.BytesToHash(sender.Bytes()), common.BytesToHash(big.NewInt(137).Bytes())},
		Data:    data,
	}}}

	if _, _, err := parseBridgeInitiated(parsed, receipt, router, sender, 137, token, recipient); err == nil {
		t.Fatal("parseBridgeInitiated accepted an event whose receiver differs from the transfer")
	}
}

// TestSignInboundDigestProducesContractVerifiableSignature proves the relayer's
// signatures are in the (r||s||v) shape and v-range the contract's _recover
// accepts. crypto.Sign emits v as 0/1; the contract rejects anything but 27/28.
func TestSignInboundDigestProducesContractVerifiableSignature(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	raw := crypto.FromECDSA(key)
	t.Setenv("TIGERBRIDGE_VALIDATOR_KEYS", hex.EncodeToString(raw))

	digest := crypto.Keccak256([]byte("tigerswap-bridge-digest"))
	sigs, err := signInboundDigest(digest)
	if err != nil {
		t.Fatalf("signInboundDigest: %v", err)
	}
	if len(sigs) != 65 {
		t.Fatalf("signature length = %d, want 65", len(sigs))
	}
	if v := sigs[64]; v != 27 && v != 28 {
		t.Fatalf("signature v = %d, want 27 or 28", v)
	}
	// The contract recovers the signer from the digest; confirm the recovered
	// address is the validator whose key we signed with. Go's ecrecover wants
	// v in 0/1, so adjust a copy while leaving the emitted signature at 27/28.
	evmSig := make([]byte, len(sigs))
	copy(evmSig, sigs)
	evmSig[64] -= 27
	recovered, err := crypto.Ecrecover(digest, evmSig)
	if err != nil {
		t.Fatalf("ecrecover: %v", err)
	}
	pub, err := crypto.UnmarshalPubkey(recovered)
	if err != nil {
		t.Fatalf("unmarshal pubkey: %v", err)
	}
	if crypto.PubkeyToAddress(*pub) != crypto.PubkeyToAddress(key.PublicKey) {
		t.Fatal("recovered address does not match the validator key")
	}
}

// TestBridgeTransferWireFormat pins the JSON shape the frontend validator and
// the SDK decoders expect. When a field is added or renamed the matching
// validator in frontend/src/lib/types.ts must change with it.
func TestBridgeTransferWireFormat(t *testing.T) {
	transfer := BridgeTransfer{
		ID:            "bridge_1",
		SrcChain:      1,
		DstChain:      8888,
		Sender:        "0x00000000000000000000000000000000000000a1",
		Recipient:     "0x000000000000000000000000000000000000cafe",
		Token:         "0x0000000000000000000000000000000000000abc",
		Amount:        "1000",
		MessageID:     "0x1111111111111111111111111111111111111111111111111111111111111111",
		MessageAmount: "997",
		Status:        "confirmed",
	}

	raw, err := json.Marshal(transfer)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{"id", "src_chain", "dst_chain", "sender", "recipient", "token", "amount", "message_id", "message_amount", "status"} {
		if _, ok := got[key]; !ok {
			t.Fatalf("bridge transfer JSON is missing %q; got keys %v", key, got)
		}
	}
}
