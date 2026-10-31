package services

import (
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

// TestAmountToFloatScalesByDecimals verifies raw token amounts are scaled by the
// token's own decimals, so a 6-decimal stablecoin and an 18-decimal token are
// valued consistently.
func TestAmountToFloatScalesByDecimals(t *testing.T) {
	cases := []struct {
		name     string
		raw      string
		decimals int
		want     float64
	}{
		{"18 decimals whole unit", "1000000000000000000", 18, 1},
		{"6 decimals whole unit", "1000000", 6, 1},
		{"0 decimals", "42", 0, 42},
		{"18 decimals fractional", "1500000000000000000", 18, 1.5},
		{"6 decimals fractional", "2500000", 6, 2.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, ok := new(big.Int).SetString(tc.raw, 10)
			if !ok {
				t.Fatalf("bad test input %q", tc.raw)
			}
			got, err := amountToFloat(raw, tc.decimals)
			if err != nil {
				t.Fatalf("amountToFloat: %v", err)
			}
			if math.Abs(got-tc.want) > 1e-12 {
				t.Fatalf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

// TestAmountToFloatRejectsBadInput verifies a nil amount or an out-of-range
// decimals value is refused rather than silently valued as zero.
func TestAmountToFloatRejectsBadInput(t *testing.T) {
	if _, err := amountToFloat(nil, 18); err == nil {
		t.Fatal("a nil amount must be rejected")
	}
	if _, err := amountToFloat(big.NewInt(1), -1); err == nil {
		t.Fatal("negative decimals must be rejected")
	}
	if _, err := amountToFloat(big.NewInt(1), 79); err == nil {
		t.Fatal("decimals beyond the representable range must be rejected")
	}
}

// TestAPYFromFeesAnnualisesFees verifies APY is fee revenue annualised against
// locked value, and that a pool holding nothing reports zero rather than an
// infinite rate of return.
func TestAPYFromFeesAnnualisesFees(t *testing.T) {
	// 1 of fees per day on 365 of liquidity is 365/365 = 100% APY.
	if got := apyFromFees(1, 365); math.Abs(got-100) > 1e-9 {
		t.Fatalf("expected 100%% APY, got %v", got)
	}
	if got := apyFromFees(0, 1000); got != 0 {
		t.Fatalf("no fees must be 0 APY, got %v", got)
	}
	if got := apyFromFees(100, 0); got != 0 {
		t.Fatalf("an empty pool must be 0 APY, not infinite, got %v", got)
	}
	if got := apyFromFees(100, -5); got != 0 {
		t.Fatalf("negative liquidity must be 0 APY, got %v", got)
	}
}

// TestSwapEventABIParsesSwapLog pins the Swap event shape to the TigerSwap V2
// factory's event, so a factory change that breaks log decoding fails here.
func TestSwapEventABIParsesSwapLog(t *testing.T) {
	parsed, err := abi.JSON(strings.NewReader(swapEventABIJSON))
	if err != nil {
		t.Fatalf("parse Swap ABI: %v", err)
	}
	event, ok := parsed.Events["Swap"]
	if !ok {
		t.Fatal("Swap event missing from ABI")
	}
	if len(event.Inputs) != 6 {
		t.Fatalf("expected 6 Swap inputs (sender, amount0In, amount1In, amount0Out, amount1Out, to), got %d", len(event.Inputs))
	}
	// The indexed fields must match the factory: sender and to.
	if !event.Inputs[0].Indexed || !event.Inputs[5].Indexed {
		t.Fatal("sender and to must be indexed")
	}
	for _, i := range []int{1, 2, 3, 4} {
		if event.Inputs[i].Indexed {
			t.Fatalf("amount field %s must not be indexed", event.Inputs[i].Name)
		}
	}
}
