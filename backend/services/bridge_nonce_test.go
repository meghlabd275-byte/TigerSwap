package services

import (
	"testing"
)

// TestBridgeNonceIsUnique guards the switch away from a wall-clock nonce. The
// nonce is folded into the on-chain messageId, so two transfers created in the
// same tick must not share one; the old time.Now().UnixNano() scheme did.
func TestBridgeNonceIsUnique(t *testing.T) {
	const count = 20000
	seen := make(map[uint64]struct{}, count)
	for i := 0; i < count; i++ {
		nonce, err := newBridgeNonce()
		if err != nil {
			t.Fatalf("newBridgeNonce: %v", err)
		}
		if _, dup := seen[nonce]; dup {
			t.Fatalf("duplicate nonce %d after %d draws", nonce, i)
		}
		seen[nonce] = struct{}{}
	}
}
