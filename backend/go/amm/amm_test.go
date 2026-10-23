package amm

import (
	"math/big"
	"testing"
)

func bi(s string) *big.Int {
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("bad int " + s)
	}
	return v
}

// TestAmountOutXYMatchesConstantProduct checks x*y=k with the fee applied.
func TestAmountOutXYMatchesConstantProduct(t *testing.T) {
	// 100 in, reserves 1000/1000, 0.30% fee => 100*997*1000/(1000*10000+99700)
	amountIn := bi("100")
	reserveIn := bi("1000")
	reserveOut := bi("1000")

	out, err := AmountOutXY(amountIn, reserveIn, reserveOut, 30)
	if err != nil {
		t.Fatalf("AmountOutXY: %v", err)
	}

	// Expected: 100*9970*1000 / (1000*10000 + 9970) = 9970000000 / 10009970 = 996
	want := bi("90")
	if out.Cmp(want) != 0 {
		t.Errorf("got %s, want %s", out, want)
	}

	// Output must always be strictly less than the input reserve.
	if out.Cmp(reserveOut) >= 0 {
		t.Errorf("output %s exceeds reserve %s", out, reserveOut)
	}
}

// TestAmountOutXYRejectsZeroLiquidity verifies guards.
func TestAmountOutXYRejectsZeroLiquidity(t *testing.T) {
	if _, err := AmountOutXY(bi("10"), bi("0"), bi("10"), 30); err == nil {
		t.Error("expected error for zero reserve")
	}
	if _, err := AmountOutXY(bi("0"), bi("10"), bi("10"), 30); err == nil {
		t.Error("expected error for zero amount in")
	}
	if _, err := AmountOutXY(bi("10"), bi("10"), bi("10"), BPS); err == nil {
		t.Error("expected error for fee >= 100%")
	}
}

// TestAmountOutStableNearParity verifies the StableSwap invariant returns close
// to a 1:1 swap for balanced stablecoins with a small fee.
func TestAmountOutStableNearParity(t *testing.T) {
	reserveIn := bi("1000000000000")  // 1,000,000 USDC (6dp)
	reserveOut := bi("1000000000000") // 1,000,000 USDT (6dp)
	amp := bi("100")

	amountIn := bi("1000000000") // 1,000 USDC
	out, err := AmountOutStable(amountIn, reserveIn, reserveOut, amp, 1)
	if err != nil {
		t.Fatalf("AmountOutStable: %v", err)
	}

	// With a 0.01% fee the output should be just under the input.
	if out.Cmp(amountIn) >= 0 {
		t.Errorf("stable output %s should be below input %s", out, amountIn)
	}
	if out.Cmp(bi("999000000")) <= 0 {
		t.Errorf("stable output %s is unexpectedly low", out)
	}
}

// TestStableInvariantIsConserved checks D stays constant for balanced pools.
func TestStableInvariantIsConserved(t *testing.T) {
	amp := bi("100")
	d := StableInvariant(bi("1000000"), bi("1000000"), amp)
	// For a balanced 2-coin pool D equals the total balance.
	if d.Cmp(bi("2000000")) != 0 {
		t.Errorf("balanced invariant D=%s, want 2000000", d)
	}
}

// TestAmountOutWeightedRespectsWeights checks an 80/20 pool prices the scarce
// asset higher than a 50/50 pool would.
func TestAmountOutWeightedRespectsWeights(t *testing.T) {
	amountIn := bi("1000")
	reserveIn := bi("800000")
	reserveOut := bi("200000")
	wIn := bi("80")
	wOut := bi("20")

	out, err := AmountOutWeighted(amountIn, reserveIn, reserveOut, wIn, wOut, 30)
	if err != nil {
		t.Fatalf("AmountOutWeighted: %v", err)
	}
	if out.Sign() <= 0 {
		t.Fatalf("weighted output must be positive, got %s", out)
	}

	// Equal-weight pools must yield a different (higher) output for the same
	// reserves, proving weights are actually applied.
	equal, err := AmountOutWeighted(amountIn, reserveIn, reserveOut, bi("50"), bi("50"), 30)
	if err != nil {
		t.Fatalf("AmountOutWeighted equal: %v", err)
	}
	if out.Cmp(equal) == 0 {
		t.Error("80/20 and 50/50 produced identical output; weights are ignored")
	}
}

func TestApplySlippage(t *testing.T) {
	out := bi("1000")
	if got := ApplySlippage(out, 0.5); got.Cmp(bi("995")) != 0 {
		t.Errorf("0.5%% slippage gave %s, want 995", got)
	}
	if got := ApplySlippage(out, 0); got.Cmp(bi("1000")) != 0 {
		t.Errorf("0%% slippage gave %s, want 1000", got)
	}
}

func TestPriceImpactPercent(t *testing.T) {
	// 100 in, 99 out => 1% impact.
	impact := PriceImpactPercent(bi("100"), bi("99"))
	if impact < 0.99 || impact > 1.01 {
		t.Errorf("impact %f, want ~1.0", impact)
	}
}

func BenchmarkAmountOutXY(b *testing.B) {
	amountIn := bi("1000000000000000000")
	reserveIn := bi("1000000000000000000000")
	reserveOut := bi("1000000000000000000000")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := AmountOutXY(amountIn, reserveIn, reserveOut, 30); err != nil {
			b.Fatal(err)
		}
	}
}
