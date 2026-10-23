// Package amm implements the swap math used by TigerSwap's own pool families.
//
// The formulas here mirror the on-chain TigerSwap contracts exactly so that
// off-chain quoting matches execution. Nothing in this package depends on a
// third-party DEX.
package amm

import (
	"errors"
	"fmt"
	"math"
	"math/big"
)

// BPS is the fee denominator used across the AMM math.
const BPS = 10000

// Family identifies a TigerSwap pool family.
type Family string

const (
	FamilyV2       Family = "tigerswap_v2"       // constant product x*y=k
	FamilyV3       Family = "tigerswap_v3"       // concentrated liquidity
	FamilyStable   Family = "tigerswap_stable"   // StableSwap invariant
	FamilyWeighted Family = "tigerswap_weighted" // weighted pools
)

// AmountOutXY returns the constant-product output for a swap with a fee in bps.
func AmountOutXY(amountIn, reserveIn, reserveOut *big.Int, feeBps uint32) (*big.Int, error) {
	if amountIn == nil || amountIn.Sign() <= 0 {
		return nil, errors.New("amountIn must be positive")
	}
	if reserveIn == nil || reserveOut == nil || reserveIn.Sign() <= 0 || reserveOut.Sign() <= 0 {
		return nil, errors.New("pool has no liquidity")
	}
	if feeBps >= BPS {
		return nil, fmt.Errorf("fee %d bps is invalid", feeBps)
	}

	amountInWithFee := new(big.Int).Mul(amountIn, big.NewInt(int64(BPS-feeBps)))
	numerator := new(big.Int).Mul(amountInWithFee, reserveOut)
	denominator := new(big.Int).Add(new(big.Int).Mul(reserveIn, big.NewInt(BPS)), amountInWithFee)
	return new(big.Int).Div(numerator, denominator), nil
}

// AmountOutStable returns the 2-coin StableSwap output using Newton's method.
// amp is the amplification coefficient read from the pool.
func AmountOutStable(amountIn, reserveIn, reserveOut, amp *big.Int, feeBps uint32) (*big.Int, error) {
	if amountIn == nil || amountIn.Sign() <= 0 {
		return nil, errors.New("amountIn must be positive")
	}
	if reserveIn == nil || reserveOut == nil || reserveIn.Sign() <= 0 || reserveOut.Sign() <= 0 {
		return nil, errors.New("pool has no liquidity")
	}
	if amp == nil || amp.Sign() <= 0 {
		return nil, errors.New("stable pool amplification not set")
	}
	if feeBps >= BPS {
		return nil, fmt.Errorf("fee %d bps is invalid", feeBps)
	}

	amountInAfterFee := new(big.Int).Div(
		new(big.Int).Mul(amountIn, big.NewInt(int64(BPS-feeBps))),
		big.NewInt(BPS),
	)

	newReserveIn := new(big.Int).Add(reserveIn, amountInAfterFee)
	d := StableInvariant(reserveIn, reserveOut, amp)
	y := GetY(newReserveIn, d, amp)
	if y.Sign() <= 0 || y.Cmp(reserveOut) >= 0 {
		return nil, errors.New("stable swap output exceeds reserves")
	}
	return new(big.Int).Sub(reserveOut, y), nil
}

// StableInvariant computes the StableSwap invariant D for a 2-coin pool using
// the canonical Curve Newton iteration with n=2 and Ann = A*n.
func StableInvariant(x, y, amp *big.Int) *big.Int {
	ann := new(big.Int).Mul(amp, big.NewInt(2)) // A * n, n = 2
	sum := new(big.Int).Add(x, y)
	if sum.Sign() == 0 {
		return big.NewInt(0)
	}
	d := new(big.Int).Set(sum)
	two := big.NewInt(2)

	for i := 0; i < 255; i++ {
		// D_P = D * D/(x*n) * D/(y*n)
		dProd := new(big.Int).Set(d)
		dProd.Div(new(big.Int).Mul(dProd, d), new(big.Int).Mul(two, x))
		dProd.Div(new(big.Int).Mul(dProd, d), new(big.Int).Mul(two, y))

		dPrev := new(big.Int).Set(d)
		num := new(big.Int).Add(new(big.Int).Mul(ann, sum), new(big.Int).Mul(dProd, two))
		num.Mul(num, d)
		den := new(big.Int).Add(
			new(big.Int).Mul(new(big.Int).Sub(ann, big.NewInt(1)), d),
			new(big.Int).Mul(big.NewInt(3), dProd),
		)
		d = new(big.Int).Div(num, den)

		if new(big.Int).Abs(new(big.Int).Sub(d, dPrev)).Cmp(big.NewInt(1)) <= 0 {
			break
		}
	}
	return d
}

// GetY solves for the new balance of the second coin given D, using the
// canonical Curve quadratic iteration for n=2. xIn is the updated balance of
// the other coin.
func GetY(xIn, d, amp *big.Int) *big.Int {
	ann := new(big.Int).Mul(amp, big.NewInt(2)) // A * n, n = 2
	two := big.NewInt(2)

	// c = D * D/(xIn*n) * D/(Ann*n)
	c := new(big.Int).Set(d)
	c.Div(new(big.Int).Mul(c, d), new(big.Int).Mul(two, xIn))
	c.Div(new(big.Int).Mul(c, d), new(big.Int).Mul(ann, two))

	b := new(big.Int).Add(xIn, new(big.Int).Div(d, ann))
	y := new(big.Int).Set(d)

	for i := 0; i < 255; i++ {
		yPrev := new(big.Int).Set(y)
		num := new(big.Int).Add(new(big.Int).Mul(y, y), c)
		den := new(big.Int).Add(new(big.Int).Mul(two, y), new(big.Int).Sub(b, d))
		if den.Sign() <= 0 {
			break
		}
		y = new(big.Int).Div(num, den)
		if new(big.Int).Abs(new(big.Int).Sub(y, yPrev)).Cmp(big.NewInt(1)) <= 0 {
			break
		}
	}
	return y
}

// AmountOutWeighted returns the weighted-pool output for a swap.
func AmountOutWeighted(amountIn, reserveIn, reserveOut, weightIn, weightOut *big.Int, feeBps uint32) (*big.Int, error) {
	if amountIn == nil || amountIn.Sign() <= 0 {
		return nil, errors.New("amountIn must be positive")
	}
	if reserveIn == nil || reserveOut == nil || reserveIn.Sign() <= 0 || reserveOut.Sign() <= 0 {
		return nil, errors.New("pool has no liquidity")
	}
	if weightIn == nil || weightOut == nil || weightIn.Sign() <= 0 || weightOut.Sign() <= 0 {
		return nil, errors.New("pool weights not set")
	}
	if feeBps >= BPS {
		return nil, fmt.Errorf("fee %d bps is invalid", feeBps)
	}

	amountInAfterFee := new(big.Int).Div(
		new(big.Int).Mul(amountIn, big.NewInt(int64(BPS-feeBps))),
		big.NewInt(BPS),
	)

	base := new(big.Float).Quo(
		new(big.Float).SetInt(reserveIn),
		new(big.Float).SetInt(new(big.Int).Add(reserveIn, amountInAfterFee)),
	)
	exponent := new(big.Float).Quo(
		new(big.Float).SetInt(weightIn),
		new(big.Float).SetInt(weightOut),
	)
	exp64, _ := exponent.Float64()
	base64, _ := base.Float64()
	if base64 <= 0 {
		return nil, errors.New("weighted swap ratio invalid")
	}

	ratio := math.Pow(base64, exp64)
	out := new(big.Float).Mul(
		new(big.Float).SetInt(reserveOut),
		new(big.Float).Sub(big.NewFloat(1), big.NewFloat(ratio)),
	)

	result, _ := out.Int(nil)
	if result.Sign() <= 0 {
		return nil, errors.New("weighted swap yields no output")
	}
	return result, nil
}

// ApplySlippage returns amountOut reduced by slippagePercent.
func ApplySlippage(amountOut *big.Int, slippagePercent float64) *big.Int {
	if slippagePercent < 0 {
		slippagePercent = 0
	}
	multiplier := big.NewInt(int64(BPS - slippagePercent*100))
	if multiplier.Sign() <= 0 {
		return big.NewInt(0)
	}
	return new(big.Int).Div(new(big.Int).Mul(amountOut, multiplier), big.NewInt(BPS))
}

// PriceImpactPercent returns the price impact of a swap in percent.
func PriceImpactPercent(amountIn, amountOut *big.Int) float64 {
	if amountIn == nil || amountIn.Sign() <= 0 || amountOut == nil {
		return 0
	}
	ratio := new(big.Float).Quo(
		new(big.Float).SetInt(amountOut),
		new(big.Float).SetInt(amountIn),
	)
	price, _ := ratio.Float64()
	return (1 - price) * 100
}
