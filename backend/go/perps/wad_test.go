package perps

import (
	"math/big"
	"testing"
)

func TestLeverageWADIsExact(t *testing.T) {
	got := LeverageWAD(10)
	want := new(big.Int).Mul(big.NewInt(10), OneUnit)
	if got.Cmp(want) != 0 {
		t.Fatalf("LeverageWAD(10): want %s, got %s", want, got)
	}
}

func TestInitialMarginWADIsReciprocal(t *testing.T) {
	// A 10x market requires 10% initial margin.
	got := InitialMarginWAD(10)
	want := new(big.Int).Div(OneUnit, big.NewInt(10))
	if got.Cmp(want) != 0 {
		t.Fatalf("InitialMarginWAD(10): want %s, got %s", want, got)
	}
	// A non-positive leverage is rejected rather than dividing by zero.
	if InitialMarginWAD(0).Sign() != 0 {
		t.Fatal("InitialMarginWAD(0) must be zero")
	}
}

func TestRatioWADIsExactForCommonRatios(t *testing.T) {
	cases := map[float64]string{
		0.05:  "50000000000000000",
		0.1:   "100000000000000000",
		0.001: "1000000000000000",
		0.5:   "500000000000000000",
	}
	for in, want := range cases {
		if got := RatioWAD(in).String(); got != want {
			t.Errorf("RatioWAD(%v): want %s, got %s", in, want, got)
		}
	}
}
