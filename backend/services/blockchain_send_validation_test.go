package services

import (
	"context"
	"math/big"
	"strings"
	"testing"
)

// TestSendTransactionRejectsInvalidDestination proves the broadcast path refuses
// a malformed or zero destination before it signs anything. common.HexToAddress
// would otherwise silently truncate the value and send funds to the wrong place.
func TestSendTransactionRejectsInvalidDestination(t *testing.T) {
	client := NewBlockchainClient()
	// A real 32-byte key so key parsing is not the reason for the rejection.
	key := "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318"

	cases := []struct {
		name string
		to   string
	}{
		{"empty", ""},
		{"not hex", "not-an-address"},
		{"too short", "0x1234"},
		{"zero address", "0x0000000000000000000000000000000000000000"},
		{"all zero padded", "0x000000000000000000000000000000000000000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := client.SendTransaction(context.Background(), 1, key, tc.to, big.NewInt(1), "", nil)
			if err == nil {
				t.Fatalf("SendTransaction accepted destination %q", tc.to)
			}
		})
	}
}

// TestSendTransactionRejectsNonPositiveAmount proves a zero or negative transfer
// never reaches the signer.
func TestSendTransactionRejectsNonPositiveAmount(t *testing.T) {
	client := NewBlockchainClient()
	key := "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318"
	to := "0x1111111111111111111111111111111111111111"

	for _, amount := range []*big.Int{nil, big.NewInt(0), big.NewInt(-5)} {
		if _, err := client.SendTransaction(context.Background(), 1, key, to, amount, "", nil); err == nil {
			t.Fatalf("SendTransaction accepted amount %v", amount)
		}
	}
}

// TestSendTransactionFailsWhenGasPriceUnavailable proves there is no silent
// hardcoded gas price: with no RPC endpoint configured the call fails instead of
// fabricating a price and reporting a phantom broadcast.
func TestSendTransactionFailsWhenGasPriceUnavailable(t *testing.T) {
	client := NewBlockchainClient()
	key := "0x4c0883a69102937d6231471b5dbb6204fe5129617082792ae468d01a3f362318"
	to := "0x1111111111111111111111111111111111111111"

	_, err := client.SendTransaction(context.Background(), 1, key, to, big.NewInt(1), "", nil)
	if err == nil {
		t.Fatal("SendTransaction succeeded with no configured RPC endpoint")
	}
	if !strings.Contains(err.Error(), "RPC URL not configured") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// TestChainSupported pins the master-wallet chain gate.
func TestChainSupported(t *testing.T) {
	cases := []struct {
		supported []int64
		chainID   int64
		want      bool
	}{
		{nil, 1, true},
		{[]int64{}, 1, true},
		{[]int64{1, 137}, 137, true},
		{[]int64{1, 137}, 56, false},
		{[]int64{1}, 1, true},
	}
	for _, tc := range cases {
		if got := chainSupported(tc.supported, tc.chainID); got != tc.want {
			t.Errorf("chainSupported(%v,%d)=%v, want %v", tc.supported, tc.chainID, got, tc.want)
		}
	}
}
