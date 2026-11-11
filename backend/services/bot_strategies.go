package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"tigerswap/backend/config"
	"tigerswap/backend/models"
)

// This file holds the real data sources the non-snapshot bot strategies read,
// and the helpers that shape their input. Every source reads platform state
// TigerSwap itself wrote — its own AMM pools, its own listing tables, its own
// perp book. A source that cannot read its state returns an error and the bot
// is skipped; nothing is fabricated.

// botVenueSource supplies the per-venue pricing the arbitrage strategy reads.
type botVenueSource interface {
	Venues(ctx context.Context, bot models.BotInstance) (json.RawMessage, error)
}

// botListingSource supplies the newly listed market the sniper strategy reads.
type botListingSource interface {
	Listing(ctx context.Context, bot models.BotInstance) (json.RawMessage, error)
}

// poolVenueSource builds one arbitrage venue per TigerSwap AMM family for the
// bot's pair, pricing each through the aggregator's real on-chain family
// quotes. The venue's ask carries the family fee and its bid deducts it, which
// is how a pool prices a round trip, so the two legs the engine derives are
// executable as written.
type poolVenueSource struct {
	prices *PriceAggregator
}

// NewPoolVenueSource builds the arbitrage venue source.
func NewPoolVenueSource(prices *PriceAggregator) botVenueSource {
	return &poolVenueSource{prices: prices}
}

func (s *poolVenueSource) Venues(ctx context.Context, bot models.BotInstance) (json.RawMessage, error) {
	if s.prices == nil {
		return nil, errors.New("price aggregator is not configured")
	}
	base, quote, ok := splitPair(bot.Symbol)
	if !ok {
		return nil, fmt.Errorf("arbitrage bot symbol %q is not a BASE/QUOTE pair", bot.Symbol)
	}
	chainID, err := botVenueChainID()
	if err != nil {
		return nil, err
	}
	baseAddr, ok := config.TokenAddress(chainID, base)
	if !ok {
		return nil, fmt.Errorf("base asset %s is not listed on chain %d", base, chainID)
	}
	quoteAddr, ok := config.TokenAddress(chainID, quote)
	if !ok {
		return nil, fmt.Errorf("quote asset %s is not listed on chain %d", quote, chainID)
	}
	baseDecimals, ok := config.TokenDecimals(chainID, baseAddr)
	if !ok {
		return nil, fmt.Errorf("base asset %s has no registered decimals", base)
	}

	// Probe one whole base unit, so the per-family execution price reflects the
	// pool's real depth rather than a notional that rounding would swallow.
	probe := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(baseDecimals)), nil)
	depth := new(big.Rat).SetInt(probe)

	aggregate, err := s.prices.GetAggregatedQuote(ctx, chainID, baseAddr, quoteAddr, probe.String(), venueFamilies())
	if err != nil {
		return nil, err
	}

	amountIn, ok := new(big.Int).SetString(aggregate.AmountIn, 10)
	if !ok || amountIn.Sign() <= 0 {
		return nil, errors.New("aggregator returned an unusable input amount")
	}

	venues := make([]map[string]any, 0, len(aggregate.SourceQuotes))
	for family, outText := range aggregate.SourceQuotes {
		out, ok := new(big.Int).SetString(outText, 10)
		if !ok || out.Sign() <= 0 {
			continue
		}
		// Mid execution price in quote per base, as an exact rational.
		mid := new(big.Rat).SetFrac(out, amountIn)
		feeBps := int64(familyFeeBps(family))
		ask := applyBpsRat(mid, feeBps)
		bid := applyBpsRat(mid, -feeBps)
		if bid.Sign() <= 0 || bid.Cmp(ask) >= 0 {
			continue
		}
		venues = append(venues, map[string]any{
			"name":    family,
			"fee_bps": 0,
			"snapshot": map[string]any{
				"symbol":       bot.Symbol,
				"bids":         []map[string]string{{"price": ratString(bid), "quantity": ratString(depth)}},
				"asks":         []map[string]string{{"price": ratString(ask), "quantity": ratString(depth)}},
				"vwap":         ratString(mid),
				"timestamp_ms": time.Now().UTC().UnixMilli(),
			},
		})
	}
	if len(venues) < 2 {
		return nil, fmt.Errorf("fewer than two TigerSwap venues price %s", bot.Symbol)
	}
	return json.Marshal(venues)
}

// perpListingSource surfaces the most recently listed active perp market as the
// sniper's candidate. Its depth is the market's real resting liquidity, so a
// thin listing is refused by the engine's own impact guard rather than bought.
type perpListingSource struct {
	db    *gorm.DB
	perps *PerpetualEngine
}

// NewPerpListingSource builds the sniper listing source.
func NewPerpListingSource(db *gorm.DB, perps *PerpetualEngine) botListingSource {
	return &perpListingSource{db: db, perps: perps}
}

// botListingWindow is how recent a listing must be to count as new.
const botListingWindow = 24 * time.Hour

func (s *perpListingSource) Listing(ctx context.Context, bot models.BotInstance) (json.RawMessage, error) {
	if s.db == nil || s.perps == nil {
		return nil, errors.New("listing source is not configured")
	}
	var market models.PerpsMarket
	err := s.db.Where("status = ? AND created_at >= ?", models.ListingStatusActive, time.Now().UTC().Add(-botListingWindow)).
		Order("created_at DESC").First(&market).Error
	if err != nil {
		return nil, fmt.Errorf("no newly listed market: %w", err)
	}
	if market.Symbol != bot.Symbol {
		return nil, fmt.Errorf("newest listing %s is not the bot's symbol %s", market.Symbol, bot.Symbol)
	}
	snapshot, err := s.perps.BookSnapshot(ctx, market.Symbol, botBookLevels)
	if err != nil {
		return nil, err
	}
	raw, err := snapshot.MarshalSnapshot()
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]any{
		"symbol":    market.Symbol,
		"snapshot":  json.RawMessage(raw),
		"liquidity": bookLiquidity(snapshot),
	})
	if err != nil {
		return nil, err
	}
	return payload, nil
}

// bookLiquidity is the quote-currency value of the resting depth on both sides,
// so the sniper's minimum-liquidity guard reads a real number.
func bookLiquidity(snapshot *MarketSnapshotData) string {
	total := new(big.Rat)
	add := func(levels []BookLevel) {
		for _, level := range levels {
			price, err := parseDecimal(level.Price)
			if err != nil {
				continue
			}
			qty, err := parseDecimal(level.Quantity)
			if err != nil {
				continue
			}
			total.Add(total, new(big.Rat).Mul(price, qty))
		}
	}
	add(snapshot.Bids)
	add(snapshot.Asks)
	return ratString(total)
}

// splitPair splits a BASE/QUOTE symbol. It rejects anything else so a bot with a
// single-token symbol is never fed to a cross-venue strategy.
func splitPair(symbol string) (string, string, bool) {
	base, quote, ok := strings.Cut(strings.TrimSpace(symbol), "/")
	if !ok || base == "" || quote == "" {
		return "", "", false
	}
	return strings.ToUpper(base), strings.ToUpper(quote), true
}

// venueFamilies lists the AMM families the arbitrage source quotes, so the
// aggregator returns one price per family rather than a routed best.
func venueFamilies() []string {
	return []string{"tigerswap_v2", "tigerswap_v3", "tigerswap_stable", "tigerswap_weighted"}
}

// familyFeeBps maps an aggregator family label to its fee in basis points.
func familyFeeBps(family string) uint32 {
	switch family {
	case "tigerswap_v3":
		return 5
	case "tigerswap_stable":
		return 1
	default:
		return 30
	}
}

// applyBpsRat scales a rational by (10_000 + bps)/10_000.
func applyBpsRat(value *big.Rat, bps int64) *big.Rat {
	scaled := new(big.Rat).Mul(value, big.NewRat(10_000+bps, 10_000))
	if scaled.Sign() < 0 {
		return new(big.Rat)
	}
	return scaled
}

// ratString renders a rational as a plain decimal string, the only form the
// engine accepts. A value that does not terminate exactly is written to the
// engine's own scale of 18 places rather than rounded to a shorter one.
func ratString(value *big.Rat) string {
	if value == nil {
		return "0"
	}
	return trimDecimal(value.FloatString(18))
}

// botVenueChainID resolves the chain the arbitrage venue source quotes. It is
// operator-configured so a deployment never guesses which chain to read.
func botVenueChainID() (int64, error) {
	for _, key := range []string{"BOT_VENUE_CHAIN_ID", "DEPOSIT_WATCH_CHAINS"} {
		raw := strings.TrimSpace(os.Getenv(key))
		if raw == "" {
			continue
		}
		first := strings.TrimSpace(strings.Split(raw, ",")[0])
		if id, err := strconv.ParseInt(first, 10, 64); err == nil && id > 0 {
			return id, nil
		}
	}
	return 0, errors.New("BOT_VENUE_CHAIN_ID is not configured")
}

// botCopySource supplies the followed trader's real orders the copy strategy
// mirrors.
type botCopySource interface {
	Signals(ctx context.Context, bot models.BotInstance) (json.RawMessage, error)
}

// perpCopySource reads the followed trader's own resting orders from the
// perpetual book and renders them as copy signals. A trader with no working
// orders yields no signals, so the follower never copies an order that does not
// exist.
type perpCopySource struct {
	perps *PerpetualEngine
}

// NewPerpCopySource builds the copy signal source.
func NewPerpCopySource(perps *PerpetualEngine) botCopySource {
	return &perpCopySource{perps: perps}
}

// botCopySignalLimit bounds how many of a trader's orders one sweep mirrors, so
// a trader with a huge working book cannot make a follower place an unbounded
// batch.
const botCopySignalLimit = 20

func (s *perpCopySource) Signals(_ context.Context, bot models.BotInstance) (json.RawMessage, error) {
	if s.perps == nil {
		return nil, errors.New("perpetual engine is not configured")
	}
	traderAddress, err := copyTraderAddress(bot.Config)
	if err != nil {
		return nil, err
	}
	orders := s.perps.GetUserOrders(traderAddress)
	signals := make([]map[string]any, 0, len(orders))
	for _, order := range orders {
		if order.Status != "pending" {
			continue
		}
		// The operator chooses which market to copy by the bot's own symbol, so a
		// follower never mirrors a market it was not configured for.
		if bot.Symbol != "" && order.Pair != bot.Symbol {
			continue
		}
		quantity, err := perpOrderBaseQuantity(order)
		if err != nil {
			continue
		}
		side := "buy"
		if order.Side == "short" {
			side = "sell"
		}
		kind := "limit"
		if order.OrderType == "market" {
			kind = "market"
		}
		signals = append(signals, map[string]any{
			"symbol":    order.Pair,
			"market":    "perp",
			"side":      side,
			"kind":      kind,
			"price":     strconv.FormatFloat(order.Price, 'f', 18, 64),
			"quantity":  quantity,
			"source_id": order.ID,
		})
		if len(signals) >= botCopySignalLimit {
			break
		}
	}
	return json.Marshal(signals)
}

// copyTraderAddress extracts the followed trader's address from a copy bot's
// validated configuration.
func copyTraderAddress(config string) (string, error) {
	var parsed struct {
		TraderAddress string `json:"trader_address"`
	}
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return "", fmt.Errorf("copy configuration is not valid JSON: %w", err)
	}
	address := strings.ToLower(strings.TrimSpace(parsed.TraderAddress))
	if address == "" {
		return "", errors.New("copy configuration has no trader_address")
	}
	return address, nil
}

// perpOrderBaseQuantity converts a perpetual order's notional size into base
// units at its own price, as an exact decimal string. A market order that has
// not been priced yet carries no usable base quantity and is skipped.
func perpOrderBaseQuantity(order *PerpetualOrder) (string, error) {
	size, err := parseDecimal(order.Size)
	if err != nil {
		return "", err
	}
	if order.Price <= 0 {
		return "", errors.New("order has no price")
	}
	price, err := rationalFromFloat(order.Price)
	if err != nil {
		return "", err
	}
	base := new(big.Rat).Quo(size, price)
	if base.Sign() <= 0 {
		return "", errors.New("order has no base quantity")
	}
	return ratString(base), nil
}

// rebalanceHoldings renders the bot owner's real portfolio as the target set
// the rebalance planner reads. Each target's `value` is the holding's live USD
// value; a holding the oracle cannot price is reported with a zero value rather
// than a guessed one, so the planner never trades against an invented number.
func (r *BotRunner) rebalanceHoldings(ctx context.Context, bot models.BotInstance) (json.RawMessage, error) {
	targets, err := rebalanceTargets(bot.Config)
	if err != nil {
		return nil, err
	}
	holdings, err := r.portfolio.Holdings(ctx, bot.UserID)
	if err != nil {
		return nil, err
	}
	bySymbol := make(map[string]float64, len(holdings))
	for _, holding := range holdings {
		if holding.ValueUSD == nil {
			continue
		}
		bySymbol[strings.ToUpper(holding.Symbol)] += *holding.ValueUSD
	}

	out := make([]map[string]any, 0, len(targets))
	for _, target := range targets {
		value := bySymbol[strings.ToUpper(target.Symbol)]
		out = append(out, map[string]any{
			"symbol":     target.Symbol,
			"weight_bps": target.WeightBps,
			"value":      strconv.FormatFloat(value, 'f', 18, 64),
		})
	}
	return json.Marshal(out)
}

// rebalanceTarget is one entry of a rebalance bot's target portfolio.
type rebalanceTarget struct {
	Symbol    string `json:"symbol"`
	WeightBps int64  `json:"weight_bps"`
}

// rebalanceTargets extracts the target set from a rebalance bot's validated
// configuration.
func rebalanceTargets(config string) ([]rebalanceTarget, error) {
	var parsed struct {
		Targets []rebalanceTarget `json:"targets"`
	}
	if err := json.Unmarshal([]byte(config), &parsed); err != nil {
		return nil, fmt.Errorf("rebalance configuration is not valid JSON: %w", err)
	}
	if len(parsed.Targets) == 0 {
		return nil, errors.New("rebalance configuration has no targets")
	}
	return parsed.Targets, nil
}

// liquidationPosition renders one margin account as the borrow position the
// liquidation engine evaluates, using the account's own listing for the
// liquidation threshold and bonus so the bot and the engine agree.
func (r *BotRunner) liquidationPosition(ctx context.Context, bot models.BotInstance, candidate LiquidationCandidate) (json.RawMessage, error) {
	accountID, err := uuid.Parse(candidate.AccountID)
	if err != nil {
		return nil, fmt.Errorf("account id %q is not a uuid: %w", candidate.AccountID, err)
	}
	var account models.MarginAccount
	if err := r.db.First(&account, "id = ?", accountID).Error; err != nil {
		return nil, err
	}
	cfg, err := r.margin.marginConfig(account.Pair)
	if err != nil {
		return nil, err
	}
	health, err := r.margin.computeHealth(ctx, r.db, accountID, cfg)
	if err != nil {
		return nil, err
	}
	payload, err := json.Marshal(map[string]any{
		"id":                    candidate.AccountID,
		"collateral_value":      strconv.FormatFloat(health.CollateralUSD, 'f', 18, 64),
		"debt_value":            strconv.FormatFloat(health.DebtUSD, 'f', 18, 64),
		"liquidation_threshold": strconv.FormatFloat(cfg.LiquidationThreshold, 'f', 18, 64),
		"liquidation_bonus_bps": int64(cfg.LiquidationBonus * 10_000),
	})
	if err != nil {
		return nil, err
	}
	return payload, nil
}
