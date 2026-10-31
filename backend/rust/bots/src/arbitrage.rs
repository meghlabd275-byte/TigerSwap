//! The arbitrage bot.
//!
//! Finds a price difference between two of TigerSwap's own venues for the same
//! pair and sizes the trade so it stays profitable after both fees. The two
//! legs are emitted together: a buy on the cheap venue and a sell on the
//! expensive one.

use crate::fixed::{BotError, BotResult, Fixed};
use crate::market::{BotOrder, MarketSnapshot, OrderKind, Side};

/// One venue's view of a pair.
#[derive(Clone, Debug)]
pub struct Venue {
    /// A label such as `v2`, `v3`, `stable` or `weighted`.
    pub name: String,
    pub snapshot: MarketSnapshot,
    /// Taker fee on this venue, in basis points.
    pub fee_bps: i64,
}

impl Venue {
    pub fn new(name: impl Into<String>, snapshot: MarketSnapshot, fee_bps: i64) -> BotResult<Self> {
        if fee_bps < 0 {
            return Err(BotError::InvalidConfig("venue fee cannot be negative"));
        }
        Ok(Self {
            name: name.into(),
            snapshot,
            fee_bps,
        })
    }

    /// The price to buy at, including fee.
    pub fn effective_ask(&self) -> BotResult<Fixed> {
        let ask = self
            .snapshot
            .best_ask()
            .ok_or(BotError::InvalidMarket("venue has no asks"))?;
        ask.apply_bps(self.fee_bps)
    }

    /// The price to sell at, net of fee.
    pub fn effective_bid(&self) -> BotResult<Fixed> {
        let bid = self
            .snapshot
            .best_bid()
            .ok_or(BotError::InvalidMarket("venue has no bids"))?;
        bid.apply_bps(-self.fee_bps)
    }
}

/// A two-legged opportunity.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct Opportunity {
    pub buy_venue: String,
    pub sell_venue: String,
    /// Gross edge in basis points before sizing.
    pub edge_bps: i64,
    pub quantity: Fixed,
    /// Expected profit in quote currency.
    pub expected_profit: Fixed,
}

/// Operator settings for the arbitrage bot.
#[derive(Clone, Debug)]
pub struct ArbitrageConfig {
    /// Smallest edge worth trading, in basis points.
    pub min_edge_bps: i64,
    /// Largest base quantity the bot will trade in one opportunity.
    pub max_quantity: Fixed,
    /// Fraction of the visible depth the bot is willing to take, in basis
    /// points. 2_000 means 20%.
    pub depth_fraction_bps: i64,
}

impl ArbitrageConfig {
    pub fn validate(&self) -> BotResult<()> {
        if self.min_edge_bps <= 0 {
            return Err(BotError::InvalidConfig("min_edge_bps must be positive"));
        }
        if !self.max_quantity.is_positive() {
            return Err(BotError::InvalidConfig("max_quantity must be positive"));
        }
        if self.depth_fraction_bps <= 0 || self.depth_fraction_bps > 10_000 {
            return Err(BotError::InvalidConfig(
                "depth_fraction_bps must be within 1..=10000",
            ));
        }
        Ok(())
    }
}

/// The arbitrage bot.
#[derive(Debug)]
pub struct ArbitrageBot {
    config: ArbitrageConfig,
}

impl ArbitrageBot {
    pub fn new(config: ArbitrageConfig) -> BotResult<Self> {
        config.validate()?;
        Ok(Self { config })
    }

    pub fn config(&self) -> &ArbitrageConfig {
        &self.config
    }

    /// Finds the best two-venue opportunity among `venues`.
    ///
    /// Returns `None` when no pair clears the minimum edge, which is the common
    /// case and must not be reported as an error.
    pub fn find(&self, venues: &[Venue]) -> BotResult<Option<Opportunity>> {
        if venues.len() < 2 {
            return Ok(None);
        }

        let mut best: Option<Opportunity> = None;

        for buy in venues {
            for sell in venues {
                if buy.name == sell.name {
                    continue;
                }

                let ask = match buy.effective_ask() {
                    Ok(price) => price,
                    Err(_) => continue,
                };
                let bid = match sell.effective_bid() {
                    Ok(price) => price,
                    Err(_) => continue,
                };
                if bid <= ask {
                    continue;
                }

                let edge = bid.sub(ask)?;
                let edge_bps = edge.div(ask)?;
                if edge_bps < Fixed::bps(self.config.min_edge_bps)? {
                    continue;
                }

                let quantity = self.size_for(buy, sell)?;
                if quantity <= Fixed::ZERO {
                    continue;
                }

                let profit = edge.mul(quantity)?;

                let candidate = Opportunity {
                    buy_venue: buy.name.clone(),
                    sell_venue: sell.name.clone(),
                    edge_bps: edge_bps.to_bps()?,
                    quantity,
                    expected_profit: profit,
                };

                let better = match &best {
                    None => true,
                    Some(current) => candidate.expected_profit > current.expected_profit,
                };
                if better {
                    best = Some(candidate);
                }
            }
        }

        Ok(best)
    }

    /// The base quantity both legs can support, capped by config and depth.
    pub fn size_for(&self, buy: &Venue, sell: &Venue) -> BotResult<Fixed> {
        let buy_depth = buy
            .snapshot
            .asks
            .first()
            .map(|level| level.quantity)
            .unwrap_or(Fixed::ZERO);
        let sell_depth = sell
            .snapshot
            .bids
            .first()
            .map(|level| level.quantity)
            .unwrap_or(Fixed::ZERO);

        let thinner = if buy_depth < sell_depth {
            buy_depth
        } else {
            sell_depth
        };

        let usable = thinner
            .mul(Fixed::from_integer(10_000)?)?
            .div(Fixed::from_integer(10_000)?)?
            .mul(Fixed::ratio(self.config.depth_fraction_bps as i128, 10_000)?)?;

        if usable < self.config.max_quantity {
            return Ok(usable);
        }
        Ok(self.config.max_quantity)
    }

    /// Builds the two legs for an opportunity.
    pub fn orders(&self, opportunity: &Opportunity, symbol: &str) -> BotResult<Vec<BotOrder>> {
        if opportunity.quantity <= Fixed::ZERO {
            return Err(BotError::InvalidConfig("opportunity quantity is not positive"));
        }
        Ok(vec![
            BotOrder::new(
                symbol,
                Side::Buy,
                OrderKind::Market,
                Fixed::ZERO,
                opportunity.quantity,
                false,
                "arbitrage_buy",
            )?,
            BotOrder::new(
                symbol,
                Side::Sell,
                OrderKind::Market,
                Fixed::ZERO,
                opportunity.quantity,
                false,
                "arbitrage_sell",
            )?,
        ])
    }
}
