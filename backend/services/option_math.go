package services

import (
	"fmt"
	"math/big"
	"strings"
)

// option_math.go is the Go port of contracts/TigerSwapOptionMath.sol.
//
// The option engine prices every trade with the same formula the on-chain
// library uses, so an off-chain quote and an on-chain settlement agree. The
// arithmetic is exact: every value is a big.Int in 1e18 fixed point and
// division truncates toward zero, which is what the Solidity int256 operators
// do. Nothing here is a float, so a quote cannot drift with the host's
// floating point rounding mode.
//
// The constants and the order of operations mirror the Solidity line for line.
// The reference values pinned in contracts/test/TigerSwapOptionMath.t.sol are
// re-asserted in option_math_test.go against this port.

// Option fixed-point constants, identical to the Solidity library.
var (
	optWAD        = mustOptInt("1000000000000000000") // 1e18
	optWADSq      = mustOptInt("1000000000000000000000000000000000000")
	optLN2        = mustOptInt("693147180559945309") // ln(2) * 1e18
	optInvSqrt2Pi = mustOptInt("398942280401432678") // 1/sqrt(2*pi) * 1e18

	optASP     = mustOptInt("2316419") // 0.2316419 * 1e7
	optASB1    = mustOptInt("319381530")
	optASB2    = mustOptInt("-356563782")
	optASB3    = mustOptInt("1781477937")
	optASB4    = mustOptInt("-1821255978")
	optASB5    = mustOptInt("1330274429")
	optASScale = mustOptInt("1000000000") // 1e9
)

// optYearSeconds is the day count the Solidity library uses: 365 days.
const optYearSeconds = int64(365 * 24 * 60 * 60)

// mustOptInt parses a decimal literal into a big.Int, panicking on a malformed
// constant. It is only called for the package's own literals.
func mustOptInt(s string) *big.Int {
	value, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("option math: bad constant " + s)
	}
	return value
}

// optMul returns a * b / WAD, truncating toward zero. This is the Solidity
// `a * b / WAD` with int256 semantics.
func optMul(a, b *big.Int) *big.Int {
	return new(big.Int).Quo(new(big.Int).Mul(a, b), optWAD)
}

// optDiv returns a * WAD / b, truncating toward zero.
func optDiv(a, b *big.Int) *big.Int {
	return new(big.Int).Quo(new(big.Int).Mul(a, optWAD), b)
}

// optRawDiv returns a / b with no scaling, truncating toward zero.
func optRawDiv(a, b *big.Int) *big.Int {
	return new(big.Int).Quo(a, b)
}

// optLn returns ln(x / WAD) * WAD, i.e. ln of a 1e18 fixed-point value in 1e18
// fixed point. Mirrors TigerSwapOptionMath.ln.
func optLn(x *big.Int) (*big.Int, error) {
	if x.Sign() <= 0 {
		return nil, fmt.Errorf("ln of %s: value must be positive", x.String())
	}

	m := new(big.Int).Set(x)
	n := big.NewInt(0)
	if m.Cmp(optWAD) < 0 {
		for m.Cmp(optWAD) < 0 {
			m.Mul(m, big.NewInt(2))
			n.Sub(n, big.NewInt(1))
		}
	} else {
		twoWad := new(big.Int).Mul(big.NewInt(2), optWAD)
		for m.Cmp(twoWad) >= 0 {
			m.Quo(m, big.NewInt(2))
			n.Add(n, big.NewInt(1))
		}
	}

	// z = (m - WAD) * WAD / (m + WAD)
	numerator := new(big.Int).Mul(new(big.Int).Sub(m, optWAD), optWAD)
	denominator := new(big.Int).Add(m, optWAD)
	z := optRawDiv(numerator, denominator)
	zSquared := optMul(z, z)

	series := big.NewInt(0)
	term := new(big.Int).Set(z)
	for i := int64(1); i <= 21; i += 2 {
		// series += 2 * term / i
		contribution := optRawDiv(new(big.Int).Mul(big.NewInt(2), term), big.NewInt(i))
		series.Add(series, contribution)
		term = optMul(term, zSquared)
	}

	return new(big.Int).Add(series, new(big.Int).Mul(n, optLN2)), nil
}

// optExp returns e^x * WAD for x >= 0. Mirrors TigerSwapOptionMath.exp.
func optExp(x *big.Int) (*big.Int, error) {
	if x.Sign() < 0 {
		return nil, fmt.Errorf("exp of %s: value must be non-negative", x.String())
	}
	n := optRawDiv(x, optLN2)
	if n.Cmp(big.NewInt(200)) >= 0 {
		return nil, fmt.Errorf("exp of %s overflows", x.String())
	}
	remainder := new(big.Int).Sub(x, new(big.Int).Mul(n, optLN2))

	result := new(big.Int).Set(optWAD)
	term := new(big.Int).Set(optWAD)
	for i := int64(1); i <= 24; i++ {
		// term = term * remainder / (WAD * i)
		numerator := new(big.Int).Mul(term, remainder)
		denominator := new(big.Int).Mul(optWAD, big.NewInt(i))
		term = optRawDiv(numerator, denominator)
		result.Add(result, term)
	}
	if result.Sign() < 0 {
		return nil, fmt.Errorf("exp of %s underflows", x.String())
	}
	// result << n
	return new(big.Int).Lsh(result, uint(n.Int64())), nil
}

// optExpNeg returns e^{-x} * WAD for x >= 0.
func optExpNeg(x *big.Int) (*big.Int, error) {
	if x.Sign() < 0 {
		return nil, fmt.Errorf("expNeg of %s: value must be non-negative", x.String())
	}
	expValue, err := optExp(x)
	if err != nil {
		return nil, err
	}
	return optRawDiv(optWADSq, expValue), nil
}

// optInvSqrt returns 1/sqrt(x) * WAD via Newton's method, mirroring
// TigerSwapOptionMath.invSqrt.
func optInvSqrt(x *big.Int) (*big.Int, error) {
	if x.Sign() <= 0 {
		return nil, fmt.Errorf("invSqrt of %s: value must be positive", x.String())
	}
	y := new(big.Int).Set(optWAD)
	temp := new(big.Int).Set(x)
	fourWad := new(big.Int).Mul(big.NewInt(4), optWAD)
	for temp.Cmp(fourWad) >= 0 {
		temp.Quo(temp, big.NewInt(4))
		y.Quo(y, big.NewInt(2))
	}
	for temp.Cmp(optWAD) < 0 {
		temp.Mul(temp, big.NewInt(4))
		y.Mul(y, big.NewInt(2))
	}

	threeWad := new(big.Int).Mul(big.NewInt(3), optWAD)
	twoWad := new(big.Int).Mul(big.NewInt(2), optWAD)
	for i := 0; i < 40; i++ {
		ySquared := optMul(y, y)
		step := optMul(x, ySquared)
		// The update is y * (3 - step) / 2. When step >= 3*WAD the numerator
		// would be negative; halving y is a safe step toward the root and the
		// following iterations converge normally. This keeps the Go engine in
		// step with the Solidity library, which must guard the same case
		// because the subtraction is unsigned there.
		if step.Cmp(threeWad) >= 0 {
			y.Quo(y, big.NewInt(2))
			continue
		}
		numerator := new(big.Int).Mul(y, new(big.Int).Sub(threeWad, step))
		next := optRawDiv(numerator, twoWad)
		if next.Cmp(y) == 0 {
			break
		}
		y = next
	}
	return y, nil
}

// optNormalPdf returns the standard normal density at x, in 1e18 fixed point.
func optNormalPdf(x *big.Int) (*big.Int, error) {
	xSquared := optMul(x, x)
	halfXSquared := optRawDiv(xSquared, big.NewInt(2))
	expValue, err := optExpNeg(halfXSquared)
	if err != nil {
		return nil, err
	}
	return optMul(optInvSqrt2Pi, expValue), nil
}

// optNormalCdf returns the standard normal CDF at x, in 1e18 fixed point, using
// the Abramowitz & Stegun 26.2.17 rational approximation.
func optNormalCdf(x *big.Int) (*big.Int, error) {
	negative := x.Sign() < 0
	t := new(big.Int).Abs(x)

	// denominator = WAD + AS_P * t / 10000000
	scaled := optRawDiv(new(big.Int).Mul(optASP, t), mustOptInt("10000000"))
	denominator := new(big.Int).Add(optWAD, scaled)
	asT := optRawDiv(optWADSq, denominator)

	power := new(big.Int).Set(asT)
	poly := optMul(optASB1, power)
	for _, coefficient := range []*big.Int{optASB2, optASB3, optASB4, optASB5} {
		power = optMul(power, asT)
		poly.Add(poly, optMul(coefficient, power))
	}
	poly = optRawDiv(new(big.Int).Mul(poly, optWAD), optASScale)

	density, err := optNormalPdf(t)
	if err != nil {
		return nil, err
	}
	tail := optMul(density, poly)

	upper := new(big.Int).Sub(optWAD, tail)
	if upper.Sign() < 0 {
		upper = big.NewInt(0)
	}
	if upper.Cmp(optWAD) > 0 {
		upper = new(big.Int).Set(optWAD)
	}
	if negative {
		return new(big.Int).Sub(optWAD, upper), nil
	}
	return upper, nil
}

// optD1D2 computes d1, d2 and sigma*sqrt(T). T is the time to expiry in years,
// in 1e18 fixed point.
func optD1D2(spot, strike, t, riskFreeRate, volatility *big.Int) (d1, d2, sigmaSqrtT *big.Int, err error) {
	lnMoneyness, err := optLn(optDiv(spot, strike))
	if err != nil {
		return nil, nil, nil, err
	}
	// sigmaSquaredHalf = sigma * sigma / (2 * WAD)
	sigmaSquaredHalf := optRawDiv(
		new(big.Int).Mul(volatility, volatility),
		new(big.Int).Mul(big.NewInt(2), optWAD),
	)
	// numerator = lnMoneyness + (r + sigmaSquaredHalf) * T / WAD
	ratePlusHalf := new(big.Int).Add(riskFreeRate, sigmaSquaredHalf)
	numerator := new(big.Int).Add(lnMoneyness, optMul(ratePlusHalf, t))

	// invSqrtT = invSqrt(WAD * WAD / T)
	invSqrtT, err := optInvSqrt(optRawDiv(optWADSq, t))
	if err != nil {
		return nil, nil, nil, err
	}
	sigmaSqrtT = optMul(volatility, invSqrtT)
	if sigmaSqrtT.Sign() <= 0 {
		return nil, nil, nil, fmt.Errorf("zero denominator: sigma*sqrt(T) is zero")
	}
	d1 = optDiv(numerator, sigmaSqrtT)
	d2 = new(big.Int).Sub(d1, sigmaSqrtT)
	return d1, d2, sigmaSqrtT, nil
}

// OptionPrice is a premium in 1e18 fixed point.
type OptionPrice struct {
	Price *big.Int
}

// optBlackScholes prices a European option, cash settled, in 1e18 fixed point.
// spot, strike and the result share one unit; timeToExpiry is in seconds.
func optBlackScholes(spot, strike *big.Int, timeToExpiry int64, riskFreeRate, volatility *big.Int, isCall bool) (*big.Int, error) {
	if spot.Sign() <= 0 || strike.Sign() <= 0 {
		return nil, fmt.Errorf("invalid price inputs: spot and strike must be positive")
	}

	var intrinsic *big.Int
	if isCall {
		intrinsic = new(big.Int).Sub(spot, strike)
	} else {
		intrinsic = new(big.Int).Sub(strike, spot)
	}
	if intrinsic.Sign() < 0 {
		intrinsic = big.NewInt(0)
	}

	if timeToExpiry == 0 || volatility.Sign() == 0 {
		return intrinsic, nil
	}

	// T = timeToExpiry * WAD / 365 days
	t := optRawDiv(new(big.Int).Mul(big.NewInt(timeToExpiry), optWAD), big.NewInt(optYearSeconds))
	if t.Sign() <= 0 {
		return intrinsic, nil
	}

	d1, d2, _, err := optD1D2(spot, strike, t, riskFreeRate, volatility)
	if err != nil {
		return nil, err
	}
	nD1, err := optNormalCdf(d1)
	if err != nil {
		return nil, err
	}
	nD2, err := optNormalCdf(d2)
	if err != nil {
		return nil, err
	}
	discount, err := optExpNeg(optMul(riskFreeRate, t))
	if err != nil {
		return nil, err
	}

	var price *big.Int
	if isCall {
		// spot * nD1 / WAD - strike * discount / WAD * nD2 / WAD
		first := optMul(spot, nD1)
		second := optMul(optMul(strike, discount), nD2)
		price = new(big.Int).Sub(first, second)
	} else {
		// strike * discount / WAD * (WAD - nD2) / WAD - spot * (WAD - nD1) / WAD
		first := optMul(optMul(strike, discount), new(big.Int).Sub(optWAD, nD2))
		second := optMul(spot, new(big.Int).Sub(optWAD, nD1))
		price = new(big.Int).Sub(first, second)
	}

	if price.Cmp(intrinsic) < 0 {
		price = intrinsic
	}
	return price, nil
}

// OptionGreeks holds the first-order sensitivities, all in 1e18 fixed point.
type OptionGreeks struct {
	Delta *big.Int
	Gamma *big.Int
	Vega  *big.Int
	Theta *big.Int
}

// optGreeks returns delta, gamma, vega and theta for a European option.
func optGreeks(spot, strike *big.Int, timeToExpiry int64, riskFreeRate, volatility *big.Int, isCall bool) (*OptionGreeks, error) {
	if spot.Sign() <= 0 || strike.Sign() <= 0 {
		return nil, fmt.Errorf("invalid price inputs: spot and strike must be positive")
	}

	if timeToExpiry == 0 || volatility.Sign() == 0 {
		delta := big.NewInt(0)
		if isCall {
			if spot.Cmp(strike) > 0 {
				delta = new(big.Int).Set(optWAD)
			}
		} else if spot.Cmp(strike) < 0 {
			delta = new(big.Int).Neg(optWAD)
		}
		return &OptionGreeks{Delta: delta, Gamma: big.NewInt(0), Vega: big.NewInt(0), Theta: big.NewInt(0)}, nil
	}

	t := optRawDiv(new(big.Int).Mul(big.NewInt(timeToExpiry), optWAD), big.NewInt(optYearSeconds))
	if t.Sign() <= 0 {
		return &OptionGreeks{Delta: big.NewInt(0), Gamma: big.NewInt(0), Vega: big.NewInt(0), Theta: big.NewInt(0)}, nil
	}

	d1, _, sigmaSqrtT, err := optD1D2(spot, strike, t, riskFreeRate, volatility)
	if err != nil {
		return nil, err
	}
	nD1, err := optNormalCdf(d1)
	if err != nil {
		return nil, err
	}
	nD2, err := optNormalCdf(new(big.Int).Sub(d1, sigmaSqrtT))
	if err != nil {
		return nil, err
	}
	density, err := optNormalPdf(d1)
	if err != nil {
		return nil, err
	}

	delta := new(big.Int).Set(nD1)
	if !isCall {
		delta = new(big.Int).Sub(delta, optWAD)
	}

	// gamma = phi(d1) / (S * sigma * sqrt(T))
	gammaDenominator := optMul(spot, sigmaSqrtT)
	gamma := optRawDiv(new(big.Int).Mul(density, optWAD), gammaDenominator)

	// vega = S * phi(d1) * sqrt(T), sqrt(T) = sigma*sqrt(T) / sigma
	sqrtT := optDiv(sigmaSqrtT, volatility)
	vega := optMul(optMul(spot, density), sqrtT)

	discount, err := optExpNeg(optMul(riskFreeRate, t))
	if err != nil {
		return nil, err
	}

	// firstTerm = -S * phi(d1) * sigma / (2 * sigma*sqrt(T)). The two /WAD
	// truncations are kept where the Solidity has them, because an intermediate
	// truncation changes the last digit and this must agree with the chain.
	numerator := optMul(optMul(spot, density), volatility)
	denominator := new(big.Int).Mul(big.NewInt(2), sigmaSqrtT)
	firstTerm := new(big.Int).Neg(optRawDiv(numerator, denominator))

	rateStrikeDiscount := optMul(optMul(riskFreeRate, strike), discount)
	var secondTerm *big.Int
	if isCall {
		secondTerm = new(big.Int).Neg(optMul(rateStrikeDiscount, nD2))
	} else {
		secondTerm = optMul(rateStrikeDiscount, new(big.Int).Sub(optWAD, nD2))
	}

	return &OptionGreeks{
		Delta: delta,
		Gamma: gamma,
		Vega:  vega,
		Theta: new(big.Int).Add(firstTerm, secondTerm),
	}, nil
}

// parseWad parses a decimal string into 1e18 fixed point. It accepts at most 18
// fractional digits and rejects a malformed value rather than silently
// returning zero.
func parseWad(value string) (*big.Int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil, publicErr(fmt.Errorf("empty amount"))
	}
	negative := false
	if strings.HasPrefix(trimmed, "-") {
		negative = true
		trimmed = trimmed[1:]
	} else if strings.HasPrefix(trimmed, "+") {
		trimmed = trimmed[1:]
	}
	if trimmed == "" {
		return nil, publicErr(fmt.Errorf("amount %q has no digits", value))
	}

	parts := strings.SplitN(trimmed, ".", 2)
	whole := parts[0]
	fraction := ""
	if len(parts) == 2 {
		fraction = parts[1]
	}
	if whole == "" && fraction == "" {
		return nil, publicErr(fmt.Errorf("amount %q has no digits", value))
	}
	if len(fraction) > 18 {
		return nil, publicErr(fmt.Errorf("amount %q has more than 18 decimal places", value))
	}
	for _, part := range []string{whole, fraction} {
		for _, r := range part {
			if r < '0' || r > '9' {
				return nil, publicErr(fmt.Errorf("amount %q is not a number", value))
			}
		}
	}

	if whole == "" {
		whole = "0"
	}
	scaled := fraction + strings.Repeat("0", 18-len(fraction))
	combined := whole + scaled
	result, ok := new(big.Int).SetString(combined, 10)
	if !ok {
		return nil, publicErr(fmt.Errorf("amount %q is not a number", value))
	}
	if negative {
		result.Neg(result)
	}
	return result, nil
}

// formatWad renders a 1e18 fixed-point value as a decimal string with the
// trailing fractional zeros trimmed.
func formatWad(value *big.Int) string {
	if value == nil {
		return "0"
	}
	negative := value.Sign() < 0
	abs := new(big.Int).Abs(value)
	whole := new(big.Int).Quo(abs, optWAD)
	fraction := new(big.Int).Mod(abs, optWAD)

	sign := ""
	if negative {
		sign = "-"
	}
	if fraction.Sign() == 0 {
		return sign + whole.String()
	}
	// Pad to 18 digits, then trim trailing zeros.
	fractionText := fraction.String()
	fractionText = strings.Repeat("0", 18-len(fractionText)) + fractionText
	fractionText = strings.TrimRight(fractionText, "0")
	return sign + whole.String() + "." + fractionText
}
