package services

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"sort"
	"strconv"
	"strings"

	"tigerswap/backend/config"
)

// maxPerpAmountMagnitude caps a single position size or collateral amount. The
// bound is far above any real position but keeps every derived value finite:
// float64 overflows to +Inf past roughly 1.8e308, and an infinite notional
// would silently satisfy every risk comparison.
const maxPerpAmountMagnitude = 1e24

// maxPerpAmountRat is the exact rational form of the cap. Converting the float
// with SetFloat64 would introduce a binary artefact that leaves the bound a
// hair above or below 10^24; the exact value keeps the limit the round number
// an operator reads.
var maxPerpAmountRat = new(big.Rat).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(24), nil))

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
		return nil, publicErr(fmt.Errorf("invalid amount %q: %w", amount, err))
	}
	if value.Sign() <= 0 {
		return nil, publicErr(fmt.Errorf("amount %q must be positive", amount))
	}
	trimmed := strings.TrimSpace(amount)
	if dot := strings.IndexByte(trimmed, '.'); dot >= 0 && len(trimmed)-dot-1 > 8 {
		return nil, publicErr(fmt.Errorf("amount %q has more than 8 decimal places", amount))
	}
	limit := maxPerpAmountRat
	if value.Cmp(limit) > 0 {
		return nil, publicErr(fmt.Errorf("amount %q exceeds the maximum of %g", amount, maxPerpAmountMagnitude))
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
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.openInterestRatLocked(symbol)
}

// openInterestRatLocked totals open interest assuming the caller already holds
// the engine lock (read or write). It exists so the open path can evaluate the
// open-interest cap under the write lock it takes for the check-then-insert,
// without re-acquiring the same RWMutex, which is not reentrant and would
// deadlock.
func (e *PerpetualEngine) openInterestRatLocked(symbol string) (long, short *big.Rat, err error) {
	long = new(big.Rat)
	short = new(big.Rat)

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

// perpSettlement describes the asset a position's margin and PnL are
// denominated in. Margin is escrowed from, and settlement paid to, the ledger
// row keyed by (chainID, tokenAddress) for this asset.
type perpSettlement struct {
	Symbol   string
	ChainID  int64
	Address  string
	Decimals int
}

// perpSettlementChain returns the chain whose token registry defines settlement
// assets. TIGERSWAP_DEFAULT_CHAIN wins; otherwise the lowest configured chain id
// is used so the choice is deterministic rather than map-order dependent.
func PerpSettlementChainID() int64 {
	if raw := strings.TrimSpace(os.Getenv("TIGERSWAP_DEFAULT_CHAIN")); raw != "" {
		if id, err := strconv.ParseInt(raw, 10, 64); err == nil {
			return id
		}
	}
	ids := make([]int64, 0)
	for id := range config.GetBlockchainConfigs() {
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return 0
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids[0]
}

// perpSettlementQuote returns the quote side of a listing symbol: the asset a
// position on that symbol settles in. "BTC/RUSD" settles in RUSD. A symbol with
// no quote side cannot be settled, so it is refused rather than defaulted.
func perpSettlementQuote(pair string) (string, error) {
	symbol := strings.ToUpper(strings.TrimSpace(pair))
	if i := strings.IndexByte(symbol, '/'); i > 0 && i < len(symbol)-1 {
		return strings.TrimSpace(symbol[i+1:]), nil
	}
	if i := strings.LastIndexByte(symbol, '-'); i > 0 && i < len(symbol)-1 {
		return strings.TrimSpace(symbol[i+1:]), nil
	}
	return "", fmt.Errorf("listing %q has no quote asset to settle in", pair)
}

// resolveSettlementFromConfig resolves a listing's settlement asset from the
// chain token registry. It is wired as the engine's default resolver so the
// asset is read from real configuration rather than assumed.
func resolveSettlementFromConfig(chainID int64) func(pair string) (perpSettlement, error) {
	return func(pair string) (perpSettlement, error) {
		quote, err := perpSettlementQuote(pair)
		if err != nil {
			return perpSettlement{}, err
		}
		address, ok := config.TokenAddress(chainID, quote)
		if !ok {
			return perpSettlement{}, fmt.Errorf("settlement asset %s for %s is not configured on chain %d", quote, pair, chainID)
		}
		decimals, ok := config.TokenDecimals(chainID, address)
		if !ok {
			return perpSettlement{}, fmt.Errorf("settlement asset %s for %s has no configured decimals on chain %d", quote, pair, chainID)
		}
		return perpSettlement{Symbol: quote, ChainID: chainID, Address: address, Decimals: decimals}, nil
	}
}

// resolveAssetFromConfig resolves a bare asset symbol (BTC, RUSD, ...) to its
// ledger entry from the chain token registry. The margin engine names its assets
// directly rather than as a pair, so it needs this resolver instead of the
// pair-based perp one.
func resolveAssetFromConfig(chainID int64) func(asset string) (perpSettlement, error) {
	return func(asset string) (perpSettlement, error) {
		symbol := strings.ToUpper(strings.TrimSpace(asset))
		if symbol == "" {
			return perpSettlement{}, errors.New("asset symbol is empty")
		}
		address, ok := config.TokenAddress(chainID, symbol)
		if !ok {
			return perpSettlement{}, fmt.Errorf("asset %s is not configured on chain %d", symbol, chainID)
		}
		decimals, ok := config.TokenDecimals(chainID, address)
		if !ok {
			return perpSettlement{}, fmt.Errorf("asset %s has no configured decimals on chain %d", symbol, chainID)
		}
		return perpSettlement{Symbol: symbol, ChainID: chainID, Address: address, Decimals: decimals}, nil
	}
}

// scaleToBaseUnits converts an exact decimal amount into the token's smallest
// unit, truncating any remainder. Truncation (never rounding up) means the
// ledger can never be credited more than the exact amount owed.
func scaleToBaseUnits(value *big.Rat, decimals int) *big.Int {
	if value == nil {
		return big.NewInt(0)
	}
	scaled := new(big.Rat).Mul(value, new(big.Rat).SetInt(pow10(decimals)))
	return new(big.Int).Quo(scaled.Num(), scaled.Denom())
}

// perpFeeAccount is the platform ledger account that receives perp liquidation
// fees. It is a sentinel, not a real user id, so no account can collide with it
// and no user can spend the collected fees.
const perpFeeAccount = "__perp_fee__"

// settlementFor resolves the settlement asset for a listing. It requires both a
// ledger to escrow against and a resolver to name the asset, so a deployment
// that forgets either refuses to open a position rather than opening one that
// moves no funds.
func (e *PerpetualEngine) settlementFor(pair string) (perpSettlement, error) {
	if e.ledger == nil {
		return perpSettlement{}, errors.New("perpetual engine has no ledger configured; positions cannot be escrowed")
	}
	if e.settlementResolver == nil {
		return perpSettlement{}, fmt.Errorf("no settlement asset resolver configured for %s", pair)
	}
	settlement, err := e.settlementResolver(pair)
	if err != nil {
		return perpSettlement{}, err
	}
	if settlement.Symbol == "" || settlement.Address == "" {
		return perpSettlement{}, fmt.Errorf("listing %s has no resolvable settlement asset", pair)
	}
	if settlement.Decimals < 0 || settlement.Decimals > 36 {
		return perpSettlement{}, fmt.Errorf("settlement asset %s has invalid decimals %d", settlement.Symbol, settlement.Decimals)
	}
	return settlement, nil
}

// settlementPayout is the exact amount a trader receives when a position closes:
// the escrowed margin plus PnL, floored at zero. A loss larger than the margin
// leaves nothing to return rather than a negative payout that would debit the
// trader again.
func settlementPayout(collateral, pnl *big.Rat) *big.Rat {
	payout := new(big.Rat).Add(collateral, pnl)
	if payout.Sign() < 0 {
		return new(big.Rat)
	}
	return payout
}
