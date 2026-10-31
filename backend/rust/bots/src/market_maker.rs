//! The market maker bot.
//!
//! Quotes both sides around the mid, keeps the spread inside the configured
//! band, and re-centres when the reference price drifts past the rebalance
//! threshold. This replaces the Arrakis/Gamma/Charm style vault with a strategy
//! that runs in-process and depends on nothing external.

use crate::fixed::{BotError, BotResult, Fixed};
use crate::market::{BotOrder, MarketSnapshot, OrderKind, Side};

/// Operator settings for the market maker.
#[derive(Clone, Debug)]
pub struct MarketMakerConfig {
    /// Half-spread in basis points applied around the mid.
    pub spread_bps: i64,
    /// Depth the bot tries to keep on each side, in base units.
    pub target_depth: Fixed,
    /// Re-centre when the mid moves this many basis points from the last quote.
    pub rebalance_threshold_bps: i64,
    /// Never quote a spread tighter than this.
    pub min_spread_bps: i64,
    /// Never quote a spread wider than this.
    pub max_spread_bps: i64,
    /// Skip quoting when either side of the book is thinner than this.
    pub min_book_depth: Fixed,
}

impl MarketMakerConfig {
    /// Rejects a configuration that would make the bot quote nonsense.
    pub fn validate(&self) -> BotResult<()> {
        if self.spread_bps <= 0 {
            return Err(BotError::InvalidConfig("spread_bps must be positive"));
        }
        if self.spread_bps < self.min_spread_bps {
            return Err(BotError::InvalidConfig("spread_bps is below min_spread_bps"));
        }
        if self.spread_bps > self.max_spread_bps {
            return Err(BotError::InvalidConfig("spread_bps is above max_spread_bps"));
        }
        if self.min_spread_bps <= 0 {
            return Err(BotError::InvalidConfig("min_spread_bps must be positive"));
        }
        if self.max_spread_bps < self.min_spread_bps {
            return Err(BotError::InvalidConfig("max_spread_bps is below min_spread_bps"));
        }
        if !self.target_depth.is_positive() {
            return Err(BotError::InvalidConfig("target_depth must be positive"));
        }
        if self.rebalance_threshold_bps <= 0 {
            return Err(BotError::InvalidConfig(
                "rebalance_threshold_bps must be positive",
            ));
        }
        if self.min_book_depth.is_negative() {
            return Err(BotError::InvalidConfig("min_book_depth cannot be negative"));
        }
        Ok(())
    }
}

/// The market maker's working state.
#[derive(Debug)]
pub struct MarketMaker {
    config: MarketMakerConfig,
    last_quote_mid: Option<Fixed>,
}

impl MarketMaker {
    pub fn new(config: MarketMakerConfig) -> BotResult<Self> {
        config.validate()?;
        Ok(Self {
            config,
            last_quote_mid: None,
        })
    }

    pub fn config(&self) -> &MarketMakerConfig {
        &self.config
    }

    /// The mid the last quote was centred on.
    pub fn last_quote_mid(&self) -> Option<Fixed> {
        self.last_quote_mid
    }

    /// True when the mid has moved far enough to justify re-quoting.
    ///
    /// The first quote always goes out; after that the move must exceed the
    /// threshold, so the bot does not churn on every tick.
    pub fn needs_requote(&self, mid: Fixed) -> BotResult<bool> {
        let Some(last) = self.last_quote_mid else {
            return Ok(true);
        };
        if last.is_zero() {
            return Ok(true);
        }
        let delta = mid.sub(last)?.abs()?;
        let move_bps = delta.div(last)?;
        Ok(move_bps >= Fixed::bps(self.config.rebalance_threshold_bps)?)
    }

    /// Produces the two-sided quote for this snapshot.
    ///
    /// Returns an empty vector when the book is too thin to quote safely, which
    /// is preferable to quoting into a market the bot cannot value.
    pub fn quote(&mut self, snapshot: &MarketSnapshot) -> BotResult<Vec<BotOrder>> {
        let mid = snapshot
            .mid()
            .ok_or(BotError::InvalidMarket("book has no prices"))?;

        if !self.config.min_book_depth.is_zero() {
            let bid_depth = snapshot.depth_within(Side::Buy, self.config.spread_bps)?;
            let ask_depth = snapshot.depth_within(Side::Sell, self.config.spread_bps)?;
            if bid_depth < self.config.min_book_depth || ask_depth < self.config.min_book_depth {
                return Ok(Vec::new());
            }
        }

        if !self.needs_requote(mid)? {
            return Ok(Vec::new());
        }

        let bid_price = mid.apply_bps(-self.config.spread_bps)?;
        let ask_price = mid.apply_bps(self.config.spread_bps)?;

        if bid_price <= Fixed::ZERO {
            return Err(BotError::InvalidMarket("spread drives the bid below zero"));
        }

        // The bot must not cross the book it is quoting into.
        if let Some(best_ask) = snapshot.best_ask() {
            if bid_price >= best_ask {
                return Err(BotError::InvalidMarket("bid would cross the best ask"));
            }
        }
        if let Some(best_bid) = snapshot.best_bid() {
            if ask_price <= best_bid {
                return Err(BotError::InvalidMarket("ask would cross the best bid"));
            }
        }

        let quantity = self.size_for(snapshot)?;

        let orders = vec![
            BotOrder::new(
                snapshot.symbol.clone(),
                Side::Buy,
                OrderKind::PostOnly,
                bid_price,
                quantity,
                false,
                "market_maker",
            )?,
            BotOrder::new(
                snapshot.symbol.clone(),
                Side::Sell,
                OrderKind::PostOnly,
                ask_price,
                quantity,
                false,
                "market_maker",
            )?,
        ];

        self.last_quote_mid = Some(mid);
        Ok(orders)
    }

    /// Scales the target depth down when the book cannot absorb it.
    fn size_for(&self, snapshot: &MarketSnapshot) -> BotResult<Fixed> {
        let available_bid = snapshot.depth_within(Side::Buy, self.config.spread_bps)?;
        let available_ask = snapshot.depth_within(Side::Sell, self.config.spread_bps)?;
        let thinner = if available_bid < available_ask {
            available_bid
        } else {
            available_ask
        };

        if thinner.is_zero() {
            return Ok(self.config.target_depth);
        }

        // Quote at most a fifth of the visible depth so the bot's own order is
        // not the only liquidity in the book.
        let cap = thinner.div(Fixed::from_integer(5)?)?;
        if cap < self.config.target_depth {
            return Ok(cap);
        }
        Ok(self.config.target_depth)
    }
}
