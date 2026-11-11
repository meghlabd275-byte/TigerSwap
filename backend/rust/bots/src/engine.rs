//! The stateful bot engine the Go control plane drives.
//!
//! The strategies in the other modules are pure: they take a snapshot and
//! return orders. A running bot is not pure — a grid remembers which levels it
//! has filled, a DCA bot remembers when it last bought — so the engine keeps
//! that state per bot, keyed by the identifier the control plane assigns.
//!
//! Every call crosses a JSON boundary: the control plane sends a validated
//! configuration and a market snapshot, and the engine returns the orders a bot
//! decided to place. Values are carried as exact decimal strings, never floats,
//! so a price the engine quotes is the price the control plane sees.

use std::collections::HashMap;

use parking_lot::Mutex;
use serde_json::{json, Value};

use crate::arbitrage::{ArbitrageBot, ArbitrageConfig, Venue};
use crate::copy::{CopyBot, CopyConfig, CopySignal, Market};
use crate::fixed::{BotError, BotResult, Fixed};
use crate::grid::{GridBot, GridConfig};
use crate::guard::{
    BorrowPosition, Liquidation, LiquidationBot, LiquidationConfig, NewListing, RebalanceBot,
    RebalanceConfig, RebalanceTrade, SniperBot, SniperConfig, Target,
};
use crate::market::{BotOrder, Level, MarketSnapshot, OrderKind, Side};
use crate::market_maker::{MarketMaker, MarketMakerConfig};
use crate::schedule::{DcaBot, DcaConfig, TwapBot, TwapConfig};

/// A configured, stateful bot.
#[derive(Debug)]
enum BotEntry {
    MarketMaker(MarketMaker),
    Grid(GridBot),
    Dca(DcaBot),
    Twap(TwapBot),
    Arbitrage(ArbitrageBot),
    Sniper(SniperBot),
    Rebalance(RebalanceBot),
    Liquidation(LiquidationBot),
    Copy(CopyBot),
}

/// The engine holds one entry per running bot and serialises access to it.
#[derive(Debug, Default)]
pub struct Engine {
    bots: Mutex<HashMap<u64, BotEntry>>,
}

impl Engine {
    pub fn new() -> Self {
        Self::default()
    }

    /// Builds (or replaces) a bot from its validated configuration.
    ///
    /// Construction runs the strategy's own validation, so a configuration the
    /// engine cannot run is refused here rather than at the first tick.
    pub fn upsert(&self, id: u64, strategy: &str, config: &Value) -> BotResult<()> {
        let entry = build_entry(strategy, config)?;
        self.bots.lock().insert(id, entry);
        Ok(())
    }

    /// Removes a bot. Returns whether it was present.
    pub fn remove(&self, id: u64) -> bool {
        self.bots.lock().remove(&id).is_some()
    }

    pub fn len(&self) -> usize {
        self.bots.lock().len()
    }

    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }

    /// Advances a snapshot-driven bot and returns the orders it produced.
    ///
    /// An unconfigured id is an error, not an empty result, so a control-plane
    /// bug cannot silently stop a bot from trading.
    pub fn step(&self, id: u64, snapshot: &Value) -> BotResult<Vec<BotOrder>> {
        let snapshot = parse_snapshot(snapshot)?;
        let mut guard = self.bots.lock();
        let entry = guard
            .get_mut(&id)
            .ok_or(BotError::OutOfRange("bot is not configured"))?;
        match entry {
            BotEntry::MarketMaker(bot) => bot.quote(&snapshot),
            BotEntry::Grid(bot) => bot.step(&snapshot),
            BotEntry::Dca(bot) => bot.step(&snapshot),
            BotEntry::Twap(bot) => bot.step(&snapshot),
            _ => Err(BotError::NoAction),
        }
    }

    /// Finds the best two-venue arbitrage and returns both legs as orders.
    pub fn arbitrage(&self, id: u64, venues: &Value) -> BotResult<Vec<BotOrder>> {
        let parsed = parse_venues(venues)?;
        let guard = self.bots.lock();
        let entry = guard
            .get(&id)
            .ok_or(BotError::OutOfRange("bot is not configured"))?;
        let BotEntry::Arbitrage(bot) = entry else {
            return Err(BotError::InvalidConfig("bot is not an arbitrage bot"));
        };
        let Some(opportunity) = bot.find(&parsed)? else {
            return Ok(Vec::new());
        };
        let symbol = parsed
            .iter()
            .find(|venue| venue.name == opportunity.buy_venue)
            .map(|venue| venue.snapshot.symbol.clone())
            .ok_or(BotError::InvalidMarket("opportunity has no venue"))?;
        bot.orders(&opportunity, &symbol)
    }

    /// Evaluates a sniper against a newly listed pool.
    pub fn sniper(&self, id: u64, listing: &Value) -> BotResult<Option<BotOrder>> {
        let guard = self.bots.lock();
        let entry = guard
            .get(&id)
            .ok_or(BotError::OutOfRange("bot is not configured"))?;
        let BotEntry::Sniper(bot) = entry else {
            return Err(BotError::InvalidConfig("bot is not a sniper bot"));
        };
        let symbol = listing
            .get("symbol")
            .and_then(Value::as_str)
            .ok_or(BotError::InvalidMarket("listing has no symbol"))?;
        let snapshot = parse_snapshot(
            listing
                .get("snapshot")
                .ok_or(BotError::InvalidMarket("listing has no snapshot"))?,
        )?;
        let liquidity = listing
            .get("liquidity")
            .and_then(Value::as_str)
            .ok_or(BotError::InvalidMarket("listing has no liquidity"))
            .and_then(Fixed::parse)?;
        bot.evaluate(&NewListing {
            symbol: symbol.to_string(),
            snapshot,
            liquidity,
        })
    }

    /// Plans the trades that bring a portfolio back to its targets.
    pub fn rebalance(&self, id: u64, holdings: &Value) -> BotResult<Vec<RebalanceTrade>> {
        let guard = self.bots.lock();
        let entry = guard
            .get(&id)
            .ok_or(BotError::OutOfRange("bot is not configured"))?;
        let BotEntry::Rebalance(bot) = entry else {
            return Err(BotError::InvalidConfig("bot is not a rebalance bot"));
        };
        let array = holdings
            .as_array()
            .ok_or(BotError::InvalidConfig("holdings must be an array"))?;
        let mut targets = Vec::with_capacity(array.len());
        for holding in array {
            let symbol = holding
                .get("symbol")
                .and_then(Value::as_str)
                .ok_or(BotError::InvalidConfig("holding has no symbol"))?;
            let weight_bps = holding
                .get("weight_bps")
                .and_then(Value::as_i64)
                .ok_or(BotError::InvalidConfig("holding has no weight_bps"))?;
            let value = holding
                .get("value")
                .and_then(Value::as_str)
                .ok_or(BotError::InvalidConfig("holding has no value"))
                .and_then(Fixed::parse)?;
            targets.push(Target {
                symbol: symbol.to_string(),
                weight_bps,
                value,
            });
        }
        bot.plan(&targets)
    }

    /// Evaluates a borrow position, returning the liquidation when it is due.
    pub fn liquidation(&self, id: u64, position: &Value) -> BotResult<Option<Liquidation>> {
        let guard = self.bots.lock();
        let entry = guard
            .get(&id)
            .ok_or(BotError::OutOfRange("bot is not configured"))?;
        let BotEntry::Liquidation(bot) = entry else {
            return Err(BotError::InvalidConfig("bot is not a liquidation bot"));
        };
        let borrow = BorrowPosition {
            id: position
                .get("id")
                .and_then(Value::as_str)
                .ok_or(BotError::InvalidMarket("position has no id"))?
                .to_string(),
            collateral_value: parse_fixed_field(position, "collateral_value")?,
            debt_value: parse_fixed_field(position, "debt_value")?,
            liquidation_threshold: parse_fixed_field(position, "liquidation_threshold")?,
            liquidation_bonus_bps: position
                .get("liquidation_bonus_bps")
                .and_then(Value::as_i64)
                .ok_or(BotError::InvalidMarket("position has no liquidation_bonus_bps"))?,
        };
        bot.evaluate(&borrow)
    }

    /// Mirrors a followed trader's signals, scaled by the copy ratio.
    pub fn copy(&self, id: u64, signals: &Value) -> BotResult<Vec<BotOrder>> {
        let parsed = parse_copy_signals(signals)?;
        let guard = self.bots.lock();
        let entry = guard
            .get(&id)
            .ok_or(BotError::OutOfRange("bot is not configured"))?;
        let BotEntry::Copy(bot) = entry else {
            return Err(BotError::InvalidConfig("bot is not a copy bot"));
        };
        bot.transform(&parsed)
    }
}

/// Builds a strategy instance from its configuration JSON.
fn build_entry(strategy: &str, config: &Value) -> BotResult<BotEntry> {
    if !config.is_object() {
        return Err(BotError::InvalidConfig("config must be a JSON object"));
    }
    match strategy {
        "market_maker" => Ok(BotEntry::MarketMaker(MarketMaker::new(MarketMakerConfig {
            spread_bps: req_i64(config, "spread_bps")?,
            target_depth: req_fixed(config, "target_depth")?,
            rebalance_threshold_bps: opt_i64(config, "rebalance_threshold_bps", 10),
            min_spread_bps: opt_i64(config, "min_spread_bps", 1),
            max_spread_bps: opt_i64(config, "max_spread_bps", 500),
            min_book_depth: opt_fixed(config, "min_book_depth", Fixed::ZERO)?,
        })?)),
        "grid" => Ok(BotEntry::Grid(GridBot::new(GridConfig {
            lower_price: req_fixed(config, "lower_price")?,
            upper_price: req_fixed(config, "upper_price")?,
            levels: req_u32(config, "levels")?,
            order_size: req_fixed(config, "order_size")?,
            stop_out_bps: opt_i64(config, "stop_out_bps", 500),
        })?)),
        "dca" => Ok(BotEntry::Dca(DcaBot::new(DcaConfig {
            amount_per_interval: req_fixed(config, "amount_per_interval")?,
            interval_ms: req_i64(config, "interval_ms")?,
            max_price: opt_fixed(config, "max_price", Fixed::ZERO)?,
            total_intervals: opt_u32(config, "total_intervals", 0),
        })?)),
        "twap" => Ok(BotEntry::Twap(TwapBot::new(TwapConfig {
            total_quantity: req_fixed(config, "total_quantity")?,
            slices: req_u32(config, "slices")?,
            interval_ms: req_i64(config, "interval_ms")?,
            side: req_side(config, "side")?,
        })?)),
        "arbitrage" => Ok(BotEntry::Arbitrage(ArbitrageBot::new(ArbitrageConfig {
            min_edge_bps: req_i64(config, "min_edge_bps")?,
            max_quantity: req_fixed(config, "max_quantity")?,
            depth_fraction_bps: opt_i64(config, "depth_fraction_bps", 2_000),
        })?)),
        "sniper" => Ok(BotEntry::Sniper(SniperBot::new(SniperConfig {
            max_price_impact_bps: req_i64(config, "max_price_impact_bps")?,
            min_liquidity: opt_fixed(config, "min_liquidity", Fixed::ZERO)?,
            buy_size: req_fixed(config, "buy_size")?,
        })?)),
        "rebalance" => Ok(BotEntry::Rebalance(RebalanceBot::new(RebalanceConfig {
            drift_threshold_bps: opt_i64(config, "drift_threshold_bps", 100),
        })?)),
        "liquidation" => Ok(BotEntry::Liquidation(LiquidationBot::new(LiquidationConfig {
            close_factor_bps: opt_i64(config, "close_factor_bps", 5_000),
            min_bonus_bps: opt_i64(config, "min_bonus_bps", 100),
        })?)),
        "copy" => Ok(BotEntry::Copy(CopyBot::new(CopyConfig {
            scale_bps: req_i64(config, "scale_bps")?,
            copy_perps: config
                .get("copy_perps")
                .and_then(Value::as_bool)
                .unwrap_or(false),
        })?)),
        _ => Err(BotError::InvalidConfig("unknown strategy")),
    }
}

/// Parses the copy signals the control plane observed on a followed trader.
fn parse_copy_signals(value: &Value) -> BotResult<Vec<CopySignal>> {
    let array = value
        .as_array()
        .ok_or(BotError::InvalidConfig("signals must be an array"))?;
    let mut parsed = Vec::with_capacity(array.len());
    for signal in array {
        let symbol = signal
            .get("symbol")
            .and_then(Value::as_str)
            .ok_or(BotError::InvalidConfig("signal has no symbol"))?;
        let market = match signal.get("market").and_then(Value::as_str) {
            Some("spot") => Market::Spot,
            Some("perp") => Market::Perp,
            _ => return Err(BotError::InvalidConfig("signal market must be spot or perp")),
        };
        let side = match signal.get("side").and_then(Value::as_str) {
            Some("buy") => Side::Buy,
            Some("sell") => Side::Sell,
            _ => return Err(BotError::InvalidConfig("signal side must be buy or sell")),
        };
        let kind = match signal.get("kind").and_then(Value::as_str) {
            Some("limit") => OrderKind::Limit,
            Some("market") => OrderKind::Market,
            Some("post_only") => OrderKind::PostOnly,
            _ => return Err(BotError::InvalidConfig("signal kind is not recognised")),
        };
        let price = signal
            .get("price")
            .and_then(Value::as_str)
            .ok_or(BotError::InvalidConfig("signal has no price"))
            .and_then(Fixed::parse)?;
        let quantity = signal
            .get("quantity")
            .and_then(Value::as_str)
            .ok_or(BotError::InvalidConfig("signal has no quantity"))
            .and_then(Fixed::parse)?;
        let source_id = signal
            .get("source_id")
            .and_then(Value::as_str)
            .ok_or(BotError::InvalidConfig("signal has no source_id"))?
            .to_string();
        parsed.push(CopySignal {
            symbol: symbol.to_string(),
            market,
            side,
            kind,
            price,
            quantity,
            source_id,
        });
    }
    Ok(parsed)
}

fn parse_venues(value: &Value) -> BotResult<Vec<Venue>> {
    let array = value
        .as_array()
        .ok_or(BotError::InvalidConfig("venues must be an array"))?;
    let mut parsed = Vec::with_capacity(array.len());
    for venue in array {
        let name = venue
            .get("name")
            .and_then(Value::as_str)
            .ok_or(BotError::InvalidConfig("venue has no name"))?;
        let fee_bps = venue
            .get("fee_bps")
            .and_then(Value::as_i64)
            .ok_or(BotError::InvalidConfig("venue has no fee_bps"))?;
        let snapshot = parse_snapshot(
            venue
                .get("snapshot")
                .ok_or(BotError::InvalidConfig("venue has no snapshot"))?,
        )?;
        parsed.push(Venue::new(name.to_string(), snapshot, fee_bps)?);
    }
    Ok(parsed)
}

fn parse_fixed_field(value: &Value, key: &str) -> BotResult<Fixed> {
    value
        .get(key)
        .and_then(Value::as_str)
        .ok_or(BotError::InvalidMarket("position field is missing"))
        .and_then(Fixed::parse)
}

/// Parses the snapshot the control plane sends.
fn parse_snapshot(value: &Value) -> BotResult<MarketSnapshot> {
    let symbol = value
        .get("symbol")
        .and_then(Value::as_str)
        .ok_or(BotError::InvalidMarket("snapshot has no symbol"))?;
    let vwap = value
        .get("vwap")
        .and_then(Value::as_str)
        .ok_or(BotError::InvalidMarket("snapshot has no vwap"))
        .and_then(Fixed::parse)?;
    let timestamp_ms = value
        .get("timestamp_ms")
        .and_then(Value::as_i64)
        .ok_or(BotError::InvalidMarket("snapshot has no timestamp_ms"))?;

    let bids = parse_levels(value.get("bids"))?;
    let asks = parse_levels(value.get("asks"))?;

    MarketSnapshot::new(symbol.to_string(), bids, asks, vwap, timestamp_ms)
}

fn parse_levels(value: Option<&Value>) -> BotResult<Vec<Level>> {
    let Some(array) = value.and_then(Value::as_array) else {
        return Ok(Vec::new());
    };
    let mut out = Vec::with_capacity(array.len());
    for entry in array {
        let price = entry
            .get("price")
            .and_then(Value::as_str)
            .ok_or(BotError::InvalidMarket("level has no price"))
            .and_then(Fixed::parse)?;
        let quantity = entry
            .get("quantity")
            .and_then(Value::as_str)
            .ok_or(BotError::InvalidMarket("level has no quantity"))
            .and_then(Fixed::parse)?;
        out.push(Level::new(price, quantity));
    }
    Ok(out)
}

// ---- configuration accessors ----

fn req_fixed(config: &Value, key: &str) -> BotResult<Fixed> {
    config
        .get(key)
        .and_then(Value::as_str)
        .ok_or(BotError::InvalidConfig("missing decimal field"))
        .and_then(Fixed::parse)
}

fn opt_fixed(config: &Value, key: &str, default: Fixed) -> BotResult<Fixed> {
    match config.get(key).and_then(Value::as_str) {
        Some(text) => Fixed::parse(text),
        None => Ok(default),
    }
}

fn req_i64(config: &Value, key: &str) -> BotResult<i64> {
    config
        .get(key)
        .and_then(Value::as_i64)
        .ok_or(BotError::InvalidConfig("missing integer field"))
}

fn opt_i64(config: &Value, key: &str, default: i64) -> i64 {
    config.get(key).and_then(Value::as_i64).unwrap_or(default)
}

fn req_u32(config: &Value, key: &str) -> BotResult<u32> {
    let value = req_i64(config, key)?;
    u32::try_from(value).map_err(|_| BotError::InvalidConfig("integer field is out of range"))
}

fn opt_u32(config: &Value, key: &str, default: u32) -> u32 {
    config
        .get(key)
        .and_then(Value::as_i64)
        .and_then(|value| u32::try_from(value).ok())
        .unwrap_or(default)
}

fn req_side(config: &Value, key: &str) -> BotResult<Side> {
    match config.get(key).and_then(Value::as_str) {
        Some("buy") => Ok(Side::Buy),
        Some("sell") => Ok(Side::Sell),
        _ => Err(BotError::InvalidConfig("side must be buy or sell")),
    }
}

// ---- JSON renderers ----

/// Renders an order as the JSON the control plane consumes.
pub fn order_to_json(order: &BotOrder) -> Value {
    json!({
        "symbol": order.symbol,
        "side": match order.side {
            Side::Buy => "buy",
            Side::Sell => "sell",
        },
        "kind": match order.kind {
            OrderKind::Limit => "limit",
            OrderKind::Market => "market",
            OrderKind::PostOnly => "post_only",
        },
        "price": order.price.to_string(),
        "quantity": order.quantity.to_string(),
        "reduce_only": order.reduce_only,
        "source": order.source,
    })
}

/// Renders a rebalance leg.
pub fn rebalance_to_json(trade: &RebalanceTrade) -> Value {
    json!({
        "symbol": trade.symbol,
        "side": match trade.side {
            Side::Buy => "buy",
            Side::Sell => "sell",
        },
        "amount": trade.amount.to_string(),
        "drift_bps": trade.drift_bps,
    })
}

/// Renders a liquidation decision.
pub fn liquidation_to_json(liquidation: &Liquidation) -> Value {
    json!({
        "position_id": liquidation.position_id,
        "repay_amount": liquidation.repay_amount.to_string(),
        "collateral_seized": liquidation.collateral_seized.to_string(),
        "bonus_earned": liquidation.bonus_earned.to_string(),
        "health_factor": liquidation.health_factor.to_string(),
    })
}
