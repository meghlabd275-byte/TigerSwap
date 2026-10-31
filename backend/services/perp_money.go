package services

import (
	"fmt"
	"math"
	"math/big"
	"strings"
)

// maxPerpAmountMagnitude caps a single position size or collateral amount. The
// bound is far above any real position but keeps every derived value finite:
// float64 overflows to +Inf past roughly 1.8e308, and an infinite notional
// would silently satisfy every risk comparison.
const maxPerpAmountMagnitude = 1e24

// perpAmountValue parses a caller-supplied size or collateral amount into an
// exact rational.
//
// The previous float path was unsafe in a way that mattered: fmt.Sscanf parses
// "NaN" and "Inf" successfully, and both satisfy a `value > 0` guard, so a
// caller could open a position whose size or collateral was NaN or infinity.
// Every later comparison against NaN is false, so such a position could never
// be liquidated. This function rejects non-numeric input, non-positive values,
// more than eight fractional digits and absurd magnitudes, so a malformed
// amount can never enter the risk path.
func perpAmountValue(amount string) (*big.Rat, error) {
	value, err := parseDecimal(amount)
	if err != nil {
		return nil, fmt.Errorf("invalid amount %q: %w", amount, err)
	}
	if value.Sign() <= 0 {
		return nil, fmt.Errorf("amount %q must be positive", amount)
	}
	trimmed := strings.TrimSpace(amount)
	if dot := strings.IndexByte(trimmed, '.'); dot >= 0 && len(trimmed)-dot-1 > 8 {
		return nil, fmt.Errorf("amount %q has more than 8 decimal places", amount)
	}
	limit := new(big.Rat).SetFloat64(maxPerpAmountMagnitude)
	if value.Cmp(limit) > 0 {
		return nil, fmt.Errorf("amount %q exceeds the maximum of %g", amount, maxPerpAmountMagnitude)
	}
	return value, nil
}

// rationalToFloat converts an exact value to float64 for valuation, refusing a
// value that is not finite rather than returning an infinity that would
// propagate into a price or PnL calculation.
func rationalToFloat(value *big.Rat) (float64, error) {
	converted, _ := value.Float64()
	if math.IsNaN(converted) || math.IsInf(converted, 0) {
		return 0, fmt.Errorf("value %s is not representable as a number", value.RatString())
	}
	return converted, nil
}

// positionNotionalRat returns a position's notional value, which for this engine
// is its size in quote terms.
func positionNotionalRat(position *PerpetualPosition) (*big.Rat, error) {
	return perpAmountValue(position.Size)
}

// positionIsLive reports whether a position still exists on the book. A
// position flagged "liquidatable" has been detected but not yet settled, so it
// still counts toward open interest and must keep being re-priced. Only "closed"
// and "liquidated" are terminal.
func positionIsLive(status string) bool {
	return status == "open" || status == "liquidatable"
}

// openInterestRat totals the notional of open positions on a symbol as exact
// rationals, so the figure can be compared against a listing's cap without
// rounding. A position whose size cannot be read is reported rather than
// skipped: silently omitting it would understate open interest and let the cap
// be exceeded.
func (e *PerpetualEngine) openInterestRat(symbol string) (long, short *big.Rat, err error) {
	long = new(big.Rat)
	short = new(big.Rat)

	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, position := range e.positions {
		if position.Pair != symbol || !positionIsLive(position.Status) {
			continue
		}
		notional, err := positionNotionalRat(position)
		if err != nil {
			return nil, nil, fmt.Errorf("position %s on %s: %w", position.ID, symbol, err)
		}
		if position.Side == "short" {
			short.Add(short, notional)
		} else {
			long.Add(long, notional)
		}
	}
	return long, short, nil
}

// openInterestCapRat returns a market's open-interest cap and whether the
// listing sets one at all. Futures listings carry no max_open_interest column,
// so for those the cap is unlimited and the boolean is false.
func openInterestCapRat(cfg *perpMarketConfig) (*big.Rat, bool, error) {
	if strings.TrimSpace(cfg.MaxOpenInterest) == "" {
		return nil, false, nil
	}
	cap, err := parseDecimal(cfg.MaxOpenInterest)
	if err != nil {
		return nil, false, fmt.Errorf("listing %s has an unreadable max_open_interest %q: %w", cfg.Symbol, cfg.MaxOpenInterest, err)
	}
	if cap.Sign() <= 0 {
		return nil, false, fmt.Errorf("listing %s has a non-positive max_open_interest %q", cfg.Symbol, cfg.MaxOpenInterest)
	}
	return cap, true, nil
}
