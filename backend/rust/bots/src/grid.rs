//! The grid bot.
//!
//! Divides a price range into levels and buys below the mid while selling
//! above it, one grid step at a time. Each level fills at most once per sweep,
//! so the bot cannot spin on a single price.

use std::collections::BTreeSet;

use crate::fixed::{BotError, BotResult, Fixed};
use crate::market::{BotOrder, MarketSnapshot, OrderKind, Side};

/// Operator settings for the grid bot.
#[derive(Clone, Debug)]
pub struct GridConfig {
    /// Lowest price in the range.
    pub lower_price: Fixed,
    /// Highest price in the range.
    pub upper_price: Fixed,
    /// Number of levels; must be at least two.
    pub levels: u32,
    /// Base-unit size of every grid order.
    pub order_size: Fixed,
    /// Stop quoting if the price leaves the range by this many basis points.
    pub stop_out_bps: i64,
}

impl GridConfig {
    pub fn validate(&self) -> BotResult<()> {
        if self.levels < 2 {
            return Err(BotError::InvalidConfig("grid needs at least two levels"));
        }
        if self.lower_price <= Fixed::ZERO {
            return Err(BotError::InvalidConfig("lower_price must be positive"));
        }
        if self.upper_price <= self.lower_price {
            return Err(BotError::InvalidConfig("upper_price must exceed lower_price"));
        }
        if !self.order_size.is_positive() {
            return Err(BotError::InvalidConfig("order_size must be positive"));
        }
        if self.stop_out_bps <= 0 {
            return Err(BotError::InvalidConfig("stop_out_bps must be positive"));
        }
        Ok(())
    }

    /// The price of each grid level, ascending.
    pub fn level_prices(&self) -> BotResult<Vec<Fixed>> {
        self.validate()?;
        let span = self.upper_price.sub(self.lower_price)?;
        let step = span.div(Fixed::from_integer(self.levels as i128 - 1)?)?;
        if step <= Fixed::ZERO {
            return Err(BotError::InvalidConfig("grid step rounds to zero"));
        }
        let mut prices = Vec::with_capacity(self.levels as usize);
        let mut price = self.lower_price;
        for _ in 0..self.levels {
            prices.push(price);
            price = price.add(step)?;
        }
        Ok(prices)
    }
}

/// The grid bot's working state.
#[derive(Debug)]
pub struct GridBot {
    config: GridConfig,
    levels: Vec<Fixed>,
    /// Levels already filled during the current sweep.
    filled: BTreeSet<usize>,
}

impl GridBot {
    pub fn new(config: GridConfig) -> BotResult<Self> {
        let levels = config.level_prices()?;
        Ok(Self {
            config,
            levels,
            filled: BTreeSet::new(),
        })
    }

    pub fn config(&self) -> &GridConfig {
        &self.config
    }

    /// The number of levels filled since the last reset.
    pub fn filled_count(&self) -> usize {
        self.filled.len()
    }

    /// Clears the per-sweep fill record.
    pub fn reset_sweep(&mut self) {
        self.filled.clear();
    }

    /// Produces the grid order for the current price.
    ///
    /// Buys at the nearest unfilled level below the mid, sells at the nearest
    /// unfilled level above it. Returns no order when the price is outside the
    /// range or every level on that side is filled.
    pub fn step(&mut self, snapshot: &MarketSnapshot) -> BotResult<Vec<BotOrder>> {
        let mid = snapshot
            .mid()
            .ok_or(BotError::InvalidMarket("book has no prices"))?;

        if !self.in_range(mid)? {
            return Ok(Vec::new());
        }

        // A level is a buy target when it sits below the mid, a sell target when
        // it sits above it. The buy takes the *highest* level below the mid and
        // the sell the *lowest* above, so both legs sit next to the price
        // instead of jumping to the far edge of the range.
        let buy_target = (0..self.levels.len())
            .rev()
            .find(|index| !self.filled.contains(index) && self.levels[*index] < mid)
            .map(|index| (index, self.levels[index]));

        let sell_target = (0..self.levels.len())
            .find(|index| !self.filled.contains(index) && self.levels[*index] > mid)
            .map(|index| (index, self.levels[index]));

        let mut orders = Vec::new();

        if let Some((index, price)) = buy_target {
            self.filled.insert(index);
            orders.push(BotOrder::new(
                snapshot.symbol.clone(),
                Side::Buy,
                OrderKind::Limit,
                price,
                self.config.order_size,
                false,
                "grid",
            )?);
        }

        if let Some((index, price)) = sell_target {
            self.filled.insert(index);
            orders.push(BotOrder::new(
                snapshot.symbol.clone(),
                Side::Sell,
                OrderKind::Limit,
                price,
                self.config.order_size,
                false,
                "grid",
            )?);
        }

        Ok(orders)
    }

    /// True while the price stays inside the configured range, allowing for the
    /// stop-out tolerance.
    pub fn in_range(&self, price: Fixed) -> BotResult<bool> {
        let low = self
            .config
            .lower_price
            .apply_bps(-self.config.stop_out_bps)?;
        let high = self
            .config
            .upper_price
            .apply_bps(self.config.stop_out_bps)?;
        Ok(price >= low && price <= high)
    }

    /// The grid levels, ascending.
    pub fn levels(&self) -> &[Fixed] {
        &self.levels
    }
}
