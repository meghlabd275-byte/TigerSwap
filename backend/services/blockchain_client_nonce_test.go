package services

import (
	"errors"
	"testing"
)

// isNonceCollision must classify the retryable nonce races and nothing else.
// A false negative would fail a payout that a re-read nonce would have landed;
// a false positive would retry a permanent rejection and hide the real error.
func TestIsNonceCollision(t *testing.T) {
	retryable := []string{
		"nonce too low",
		"Nonce Too Low: next nonce 5, tx nonce 3",
		"already known",
		"replacement transaction underpriced",
		"nonce too high",
	}
	for _, message := range retryable {
		if !isNonceCollision(errors.New(message)) {
			t.Errorf("expected %q to be a retryable nonce collision", message)
		}
	}

	permanent := []string{
		"insufficient funds for gas * price + value",
		"invalid sender",
		"execution reverted",
		"context deadline exceeded",
	}
	for _, message := range permanent {
		if isNonceCollision(errors.New(message)) {
			t.Errorf("expected %q to be a permanent error", message)
		}
	}
}

// senderLock must hand the same mutex back for one (chain, sender) pair so
// concurrent sends serialise, and a different mutex for a different signer so
// unrelated wallets do not block each other.
func TestSenderLockIsPerChainAndSigner(t *testing.T) {
	client := &BlockchainClient{}

	first := client.senderLock(1, "0xAbC0000000000000000000000000000000000001")
	same := client.senderLock(1, "0xabc0000000000000000000000000000000000001")
	if first != same {
		t.Fatal("same chain and sender (case-insensitive) must share one mutex")
	}

	otherChain := client.senderLock(56, "0xAbC0000000000000000000000000000000000001")
	if otherChain == first {
		t.Fatal("a different chain must not share the mutex")
	}

	otherSigner := client.senderLock(1, "0xAbC0000000000000000000000000000000000002")
	if otherSigner == first {
		t.Fatal("a different signer must not share the mutex")
	}
}
