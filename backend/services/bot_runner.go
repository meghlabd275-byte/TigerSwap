package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"tigerswap/backend/models"
	"tigerswap/backend/services/botengine"
)

// DefaultBotSweepInterval is how often the bot runner advances every active bot.
// It is short enough that a grid or market maker reacts to a move on the next
// tick and long enough that the oracle is not hammered.
const DefaultBotSweepInterval = 5 * time.Second

// botOrderTTL is how long a bot's resting order stays live before the perp
// engine expires it. A bot re-quotes on the next sweep, so a short life keeps
// the book from filling with abandoned quotes.
const botOrderTTL = 3600

// botBookLevels is how many levels per side the runner feeds to a strategy.
const botBookLevels = 20

// BotRunner advances every active bot against live market data.
//
// It is the control plane the Rust engine was built for: on each sweep it reads
// the caller's own resting orders and the oracle mark into a snapshot, hands it
// to the strategy, and turns the orders the strategy returns into real resting
// orders on the perpetual engine. Nothing here is simulated — a bot that cannot
// price its market is skipped rather than fed a fabricated book.
//
// The snapshot-driven strategies (market maker, grid, DCA, TWAP) reason about a
// single book. The remaining strategies (arbitrage, sniper, rebalance,
// liquidation, copy) are driven by the sources below, each of which reads live
// platform state: the AMM family pools, the listing tables, the caller's own
// portfolio, the margin book and the followed trader's own orders.
type BotRunner struct {
	db       *gorm.DB
	bots     *BotsService
	perps    *PerpetualEngine
	engine   botengine.Engine
	interval time.Duration

	// venues and listings are the real cross-venue and new-listing sources the
	// arbitrage and sniper strategies read. margin and portfolio back the
	// liquidation and rebalance strategies, and copy backs the copy strategy.
	// Each is optional; a strategy whose source is not wired is skipped rather
	// than run against invented data.
	venues    botVenueSource
	listings  botListingSource
	margin    *MarginEngine
	portfolio *PortfolioService
	copy      botCopySource

	// loaded records the configuration the engine currently holds per bot, so a
	// bot is upserted once and a configuration change is picked up without
	// discarding a running bot's state on every sweep.
	loaded map[uint]string
}

// NewBotRunner builds a runner. The engine may be nil, in which case Run logs
// that bots cannot trade and returns; that is a misconfiguration, not a reason
// to panic.
func NewBotRunner(db *gorm.DB, bots *BotsService, perps *PerpetualEngine, engine botengine.Engine) *BotRunner {
	return &BotRunner{
		db:       db,
		bots:     bots,
		perps:    perps,
		engine:   engine,
		interval: DefaultBotSweepInterval,
		loaded:   map[uint]string{},
	}
}

// SetVenueSource attaches the cross-venue source the arbitrage strategy reads.
func (r *BotRunner) SetVenueSource(source botVenueSource) { r.venues = source }

// SetListingSource attaches the new-listing source the sniper strategy reads.
func (r *BotRunner) SetListingSource(source botListingSource) { r.listings = source }

// SetMarginEngine attaches the margin book the liquidation strategy acts on.
func (r *BotRunner) SetMarginEngine(engine *MarginEngine) { r.margin = engine }

// SetPortfolioService attaches the portfolio the rebalance strategy reads.
func (r *BotRunner) SetPortfolioService(service *PortfolioService) { r.portfolio = service }

// SetCopySource attaches the followed-trader order source the copy strategy reads.
func (r *BotRunner) SetCopySource(source botCopySource) { r.copy = source }

// SetInterval overrides the sweep cadence.
func (r *BotRunner) SetInterval(interval time.Duration) {
	if interval > 0 {
		r.interval = interval
	}
}

// Run sweeps until ctx is cancelled.
func (r *BotRunner) Run(ctx context.Context) {
	if r.engine == nil {
		log.Println("bot runner not started: the Rust engine is not built into this binary")
		return
	}
	if r.bots == nil || r.perps == nil {
		log.Println("bot runner not started: bots service or perpetual engine is nil")
		return
	}

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.Sweep(ctx); err != nil {
				log.Printf("bot runner: sweep failed: %v", err)
			}
		}
	}
}

// Sweep advances every active bot once. It is exported so a test can drive one
// pass deterministically instead of waiting on the ticker.
func (r *BotRunner) Sweep(ctx context.Context) error {
	if r.engine == nil {
		return errors.New("bot runner has no engine")
	}

	active, err := r.bots.activeBots()
	if err != nil {
		return fmt.Errorf("load active bots: %w", err)
	}

	live := make(map[uint]bool, len(active))
	for i := range active {
		bot := active[i]
		live[bot.ID] = true
		r.runBot(ctx, bot)
	}

	// Forget bots that are no longer active, so a paused bot stops trading and
	// its strategy state does not leak into a later run.
	for id := range r.loaded {
		if !live[id] {
			if err := r.engine.Remove(uint64(id)); err != nil {
				log.Printf("bot runner: could not unload bot %d: %v", id, err)
			}
			delete(r.loaded, id)
		}
	}
	return nil
}

// runBot advances one bot. A failure is logged and the sweep continues: one
// bot's misconfiguration must not stop every other bot from trading.
//
// The dispatch is by strategy. The snapshot-driven strategies reason about one
// book; the others read their own real source (cross-venue prices, new
// listings, the margin book, the caller's portfolio, the followed trader's
// orders). A strategy whose source is not wired is skipped with a log rather
// than run against invented data.
func (r *BotRunner) runBot(ctx context.Context, bot models.BotInstance) {
	if err := r.ensureLoaded(bot); err != nil {
		log.Printf("bot runner: bot %d could not be configured: %v", bot.ID, err)
		return
	}

	switch bot.Strategy {
	case models.StrategyMarketMaker, models.StrategyGrid, models.StrategyDCA, models.StrategyTWAP:
		r.runSnapshotBot(ctx, bot)
	case models.StrategyArbitrage:
		r.runArbitrageBot(ctx, bot)
	case models.StrategySniper:
		r.runSniperBot(ctx, bot)
	case models.StrategyRebalance:
		r.runRebalanceBot(ctx, bot)
	case models.StrategyLiquidation:
		r.runLiquidationBot(ctx, bot)
	case models.StrategyCopy:
		r.runCopyBot(ctx, bot)
	default:
		log.Printf("bot runner: bot %d has unknown strategy %q", bot.ID, bot.Strategy)
	}
}

// runSnapshotBot feeds the bot a live book snapshot and places what it returns.
func (r *BotRunner) runSnapshotBot(ctx context.Context, bot models.BotInstance) {
	snapshot, err := r.perps.BookSnapshot(ctx, bot.Symbol, botBookLevels)
	if err != nil {
		// An unlisted symbol or an unavailable oracle is expected while a market
		// is paused; the bot simply does not trade this sweep.
		return
	}
	rawSnapshot, err := snapshot.MarshalSnapshot()
	if err != nil {
		log.Printf("bot runner: bot %d snapshot could not be encoded: %v", bot.ID, err)
		return
	}

	orders, err := r.engine.Step(uint64(bot.ID), rawSnapshot)
	if err != nil {
		log.Printf("bot runner: bot %d step failed: %v", bot.ID, err)
		return
	}
	r.placeAndRecord(ctx, bot, orders, snapshot.VWAP)
}

// runArbitrageBot reads the real cross-venue prices and places the two legs.
func (r *BotRunner) runArbitrageBot(ctx context.Context, bot models.BotInstance) {
	if r.venues == nil {
		return
	}
	venues, err := r.venues.Venues(ctx, bot)
	if err != nil {
		log.Printf("bot runner: bot %d could not read venues: %v", bot.ID, err)
		return
	}
	orders, err := r.engine.Arbitrage(uint64(bot.ID), venues)
	if err != nil {
		log.Printf("bot runner: bot %d arbitrage failed: %v", bot.ID, err)
		return
	}
	r.placeAndRecord(ctx, bot, orders, "")
}

// runSniperBot evaluates the newest real listing and places the buy it returns.
func (r *BotRunner) runSniperBot(ctx context.Context, bot models.BotInstance) {
	if r.listings == nil {
		return
	}
	listing, err := r.listings.Listing(ctx, bot)
	if err != nil {
		log.Printf("bot runner: bot %d could not read listing: %v", bot.ID, err)
		return
	}
	order, err := r.engine.Sniper(uint64(bot.ID), listing)
	if err != nil {
		log.Printf("bot runner: bot %d sniper failed: %v", bot.ID, err)
		return
	}
	if order == nil {
		return
	}
	r.placeAndRecord(ctx, bot, []botengine.Order{*order}, "")
}

// runRebalanceBot reads the bot owner's real portfolio and places the legs that
// restore it to the configured targets.
func (r *BotRunner) runRebalanceBot(ctx context.Context, bot models.BotInstance) {
	if r.portfolio == nil {
		return
	}
	holdings, err := r.rebalanceHoldings(ctx, bot)
	if err != nil {
		log.Printf("bot runner: bot %d could not read portfolio: %v", bot.ID, err)
		return
	}
	legs, err := r.engine.Rebalance(uint64(bot.ID), holdings)
	if err != nil {
		log.Printf("bot runner: bot %d rebalance failed: %v", bot.ID, err)
		return
	}
	if len(legs) == 0 {
		return
	}
	orders := make([]botengine.Order, 0, len(legs))
	for _, leg := range legs {
		orders = append(orders, botengine.Order{
			Symbol:   leg.Symbol,
			Side:     leg.Side,
			Kind:     "market",
			Quantity: leg.Amount,
			Source:   "rebalance",
		})
	}
	r.placeAndRecord(ctx, bot, orders, "")
}

// runLiquidationBot scans the real margin book and liquidates every account the
// engine flags, paying the bonus the account's own listing defines.
func (r *BotRunner) runLiquidationBot(ctx context.Context, bot models.BotInstance) {
	if r.margin == nil {
		return
	}
	candidates, err := r.margin.LiquidatableAccounts(ctx)
	if err != nil {
		log.Printf("bot runner: bot %d could not scan the margin book: %v", bot.ID, err)
		return
	}
	placed := 0
	for _, candidate := range candidates {
		if placed >= bot.MaxOpenOrders {
			break
		}
		position, err := r.liquidationPosition(ctx, bot, candidate)
		if err != nil {
			log.Printf("bot runner: bot %d could not assess account %s: %v", bot.ID, candidate.AccountID, err)
			continue
		}
		decision, err := r.engine.Liquidation(uint64(bot.ID), position)
		if err != nil {
			log.Printf("bot runner: bot %d liquidation assessment failed: %v", bot.ID, err)
			continue
		}
		if decision == nil {
			continue
		}
		accountID, err := uuid.Parse(candidate.AccountID)
		if err != nil {
			log.Printf("bot runner: bot %d account id %q is not a uuid: %v", bot.ID, candidate.AccountID, err)
			continue
		}
		result, err := r.margin.Liquidate(ctx, accountID)
		if err != nil {
			log.Printf("bot runner: bot %d could not liquidate account %s: %v", bot.ID, candidate.AccountID, err)
			continue
		}
		if result == nil {
			log.Printf("bot runner: bot %d liquidation of account %s returned no settlement", bot.ID, candidate.AccountID)
			continue
		}
		record := models.BotOrder{
			BotID:      bot.ID,
			UserID:     bot.UserID,
			Symbol:     bot.Symbol,
			Side:       "sell",
			OrderKind:  "market",
			Price:      strconv.FormatFloat(result.HealthFactor, 'f', -1, 64),
			Quantity:   strconv.FormatFloat(result.DebtUSD, 'f', -1, 64),
			ReduceOnly: true,
			Source:     "liquidation",
			Status:     "filled",
		}
		if err := r.bots.recordBotOrders(bot, []models.BotOrder{record}); err != nil {
			log.Printf("bot runner: bot %d liquidation record failed: %v", bot.ID, err)
		}
		placed++
	}
}

// runCopyBot mirrors the followed trader's real orders, scaled by the copy
// ratio, into the follower's own book.
func (r *BotRunner) runCopyBot(ctx context.Context, bot models.BotInstance) {
	if r.copy == nil {
		return
	}
	signals, err := r.copy.Signals(ctx, bot)
	if err != nil {
		log.Printf("bot runner: bot %d could not read trader signals: %v", bot.ID, err)
		return
	}
	if len(signals) == 0 {
		return
	}
	orders, err := r.engine.Copy(uint64(bot.ID), signals)
	if err != nil {
		log.Printf("bot runner: bot %d copy failed: %v", bot.ID, err)
		return
	}
	r.placeAndRecord(ctx, bot, orders, "")
}

// placeAndRecord places the strategy's orders and records the accepted ones. It
// is the single exit for every strategy that emits orders, so the risk limits
// and the audit trail are applied identically to all of them.
func (r *BotRunner) placeAndRecord(ctx context.Context, bot models.BotInstance, orders []botengine.Order, mark string) {
	if len(orders) == 0 {
		return
	}
	placed, err := r.placeOrders(ctx, bot, orders, mark)
	if err != nil {
		log.Printf("bot runner: bot %d could not place orders: %v", bot.ID, err)
	}
	if len(placed) == 0 {
		return
	}
	if err := r.bots.recordBotOrders(bot, placed); err != nil {
		log.Printf("bot runner: bot %d orders could not be recorded: %v", bot.ID, err)
	}
}

// ensureLoaded configures the engine with the bot's stored configuration the
// first time it is seen, and reconfigures it only when the stored configuration
// changes. Reconfiguring on every sweep would reset a grid's filled levels and a
// DCA bot's schedule.
func (r *BotRunner) ensureLoaded(bot models.BotInstance) error {
	if current, ok := r.loaded[bot.ID]; ok && current == bot.Config {
		return nil
	}
	if err := r.engine.Upsert(uint64(bot.ID), string(bot.Strategy), json.RawMessage(bot.Config)); err != nil {
		return err
	}
	r.loaded[bot.ID] = bot.Config
	return nil
}

// placeOrders turns strategy orders into real resting orders on the perpetual
// engine and returns the audit rows for the ones it accepted.
//
// Every limit the operator set is enforced here: the order's notional must be
// within max_order_size, the bot must not exceed max_open_orders working orders,
// and an order that cannot be priced is refused rather than sent at a guess.
func (r *BotRunner) placeOrders(ctx context.Context, bot models.BotInstance, orders []botengine.Order, mark string) ([]models.BotOrder, error) {
	working, err := r.bots.pendingOrderCount(bot.ID)
	if err != nil {
		return nil, err
	}
	if working >= int64(bot.MaxOpenOrders) {
		return nil, nil
	}

	maxSize, err := parseDecimal(bot.MaxOrderSize)
	if err != nil {
		return nil, fmt.Errorf("max_order_size is not a decimal: %w", err)
	}

	placed := make([]models.BotOrder, 0, len(orders))
	for _, order := range orders {
		if working+int64(len(placed)) >= int64(bot.MaxOpenOrders) {
			break
		}
		// A single-symbol bot only ever trades its own symbol; a multi-symbol
		// strategy (rebalance) trades the symbols its plan named. Either way the
		// market must be listed, which is checked when the order is created.
		if bot.Strategy != models.StrategyRebalance && order.Symbol != bot.Symbol {
			return placed, fmt.Errorf("strategy returned an order for %s but the bot trades %s", order.Symbol, bot.Symbol)
		}
		if order.Symbol == "" {
			return placed, errors.New("strategy returned an order with no symbol")
		}

		orderType := "limit"
		if order.Kind == "market" {
			orderType = "market"
		}

		// A limit order prices at its own price; a market order prices at the
		// market's own oracle mark, read fresh so a multi-symbol plan is not
		// stamped with the primary symbol's price.
		price := order.Price
		if orderType == "market" {
			if mark != "" && order.Symbol == bot.Symbol {
				price = mark
			} else {
				resolved, err := r.perps.getMarkPrice(ctx, order.Symbol)
				if err != nil {
					log.Printf("bot runner: bot %d market order for %s has no mark: %v", bot.ID, order.Symbol, err)
					continue
				}
				price = strconv.FormatFloat(resolved, 'f', -1, 64)
			}
		}

		notional, err := orderNotional(order.Quantity, price)
		if err != nil {
			log.Printf("bot runner: bot %d order could not be sized: %v", bot.ID, err)
			continue
		}
		if notional.Cmp(maxSize) > 0 {
			log.Printf("bot runner: bot %d order notional %s exceeds max_order_size %s", bot.ID, notional.RatString(), bot.MaxOrderSize)
			continue
		}

		side := "long"
		if order.Side == "sell" {
			side = "short"
		}
		sizeText := formatDecimal(notional, 8)
		limitPrice := 0.0
		if orderType == "limit" {
			parsed, err := parsePositiveFloat(price)
			if err != nil {
				log.Printf("bot runner: bot %d limit price is invalid: %v", bot.ID, err)
				continue
			}
			limitPrice = parsed
		}

		perpetual, err := r.perps.CreateOrder(ctx, bot.UserID, order.Symbol, side, orderType, sizeText, limitPrice, 0, botOrderTTL)
		if err != nil {
			log.Printf("bot runner: bot %d order was rejected: %v", bot.ID, err)
			continue
		}

		status := "pending"
		if perpetual.Status == "filled" {
			status = "filled"
		}
		placed = append(placed, models.BotOrder{
			BotID:      bot.ID,
			UserID:     bot.UserID,
			Symbol:     order.Symbol,
			Side:       order.Side,
			OrderKind:  order.Kind,
			Price:      price,
			Quantity:   order.Quantity,
			ReduceOnly: order.ReduceOnly,
			Source:     order.Source,
			Status:     status,
		})
	}
	return placed, nil
}

// orderNotional returns the quote-currency value of an order as an exact
// rational. A zero price cannot size an order.
func orderNotional(quantity, price string) (*big.Rat, error) {
	qty, err := parseDecimal(quantity)
	if err != nil {
		return nil, fmt.Errorf("quantity %q: %w", quantity, err)
	}
	px, err := parseDecimal(price)
	if err != nil {
		return nil, fmt.Errorf("price %q: %w", price, err)
	}
	if px.Sign() <= 0 {
		return nil, errors.New("price is not positive")
	}
	notional := new(big.Rat).Mul(qty, px)
	if notional.Sign() <= 0 {
		return nil, errors.New("notional is not positive")
	}
	return notional, nil
}

// parsePositiveFloat parses a decimal string into a positive, finite float64.
func parsePositiveFloat(value string) (float64, error) {
	rational, err := parseDecimal(value)
	if err != nil {
		return 0, err
	}
	converted, err := rationalToFloat(rational)
	if err != nil {
		return 0, err
	}
	if converted <= 0 {
		return 0, errors.New("value is not positive")
	}
	return converted, nil
}
