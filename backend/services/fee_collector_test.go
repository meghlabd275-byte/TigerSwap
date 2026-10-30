package services

import (
	"testing"
)

func TestCalculateFeeExactUsesPercentage(t *testing.T) {
	// 0.3% of 1000 is exactly 3.
	fee, err := calculateFeeExact("1000", 0.3, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := feeAmountString(fee); got != "3.00000000" {
		t.Fatalf("expected 3.00000000, got %q", got)
	}
}

func TestCalculateFeeExactAppliesFixedFloor(t *testing.T) {
	// 0.3% of 1 is 0.003, below the 0.01 floor.
	fee, err := calculateFeeExact("1", 0.3, 0.01)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := feeAmountString(fee); got != "0.01000000" {
		t.Fatalf("expected the fixed floor 0.01000000, got %q", got)
	}
}

// A malformed amount used to be read as zero by fmt.Sscanf, which silently
// produced a zero fee instead of surfacing bad input.
func TestCalculateFeeExactRejectsMalformedAmount(t *testing.T) {
	for _, bad := range []string{"", "abc", "-5", "1e3", "0x10", "1/2"} {
		if _, err := calculateFeeExact(bad, 0.3, 0); err == nil {
			t.Fatalf("amount %q must be rejected", bad)
		}
	}
}

// Small amounts must not drift: 0.3% of 0.1 is exactly 0.0003.
func TestCalculateFeeExactIsPreciseForSmallAmounts(t *testing.T) {
	fee, err := calculateFeeExact("0.1", 0.3, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := feeAmountString(fee); got != "0.00030000" {
		t.Fatalf("expected 0.00030000, got %q", got)
	}
}
