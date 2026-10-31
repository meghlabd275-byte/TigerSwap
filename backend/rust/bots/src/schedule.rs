//! The DCA and TWAP schedulers.
//!
//! Both split an order over time. DCA spends a fixed amount of quote currency
//! on a fixed cadence; TWAP splits a fixed base quantity into equal slices. The
//! schedule is driven by the caller's clock, so a bot never depends on a timer
//! thread it does not own.

use crate::fixed::{BotError, BotResult, Fixed};
use crate::market::{BotOrder, MarketSnapshot, OrderKind, Side};

/// Operator settings for the DCA bot.
#[derive(Clone, Debug)]
pub struct DcaConfig {
    /// Quote currency spent per interval.
    pub amount_per_interval: Fixed,
    /// Minimum gap between buys, in milliseconds.
    pub interval_ms: i64,
    /// Largest price the bot will pay; zero means no ceiling.
    pub max_price: Fixed,
    /// Total number of buys; zero means run until stopped.
    pub total_intervals: u32,
}

impl DcaConfig {
    pub fn validate(&self) -> BotResult<()> {
        if !self.amount_per_interval.is_positive() {
            return Err(BotError::InvalidConfig("amount_per_interval must be positive"));
        }
        if self.interval_ms <= 0 {
            return Err(BotError::InvalidConfig("interval_ms must be positive"));
        }
        if self.max_price.is_negative() {
            return Err(BotError::InvalidConfig("max_price cannot be negative"));
        }
        Ok(())
    }
}

/// The DCA bot's working state.
#[derive(Debug)]
pub struct DcaBot {
    config: DcaConfig,
    last_buy_ms: Option<i64>,
    intervals_done: u32,
    spent: Fixed,
}

impl DcaBot {
    pub fn new(config: DcaConfig) -> BotResult<Self> {
        config.validate()?;
        Ok(Self {
            config,
            last_buy_ms: None,
            intervals_done: 0,
            spent: Fixed::ZERO,
        })
    }

    pub fn intervals_done(&self) -> u32 {
        self.intervals_done
    }

    pub fn spent(&self) -> Fixed {
        self.spent
    }

    pub fn is_complete(&self) -> bool {
        self.config.total_intervals != 0 && self.intervals_done >= self.config.total_intervals
    }

    /// Emits a buy when the interval has elapsed and the price is acceptable.
    pub fn step(&mut self, snapshot: &MarketSnapshot) -> BotResult<Vec<BotOrder>> {
        if self.is_complete() {
            return Ok(Vec::new());
        }

        if let Some(last) = self.last_buy_ms {
            let elapsed = snapshot.timestamp_ms.saturating_sub(last);
            if elapsed < self.config.interval_ms {
                return Ok(Vec::new());
            }
        }

        let price = snapshot.reference_price()?;

        if !self.config.max_price.is_zero() && price > self.config.max_price {
            return Ok(Vec::new());
        }
        if !price.is_positive() {
            return Err(BotError::InvalidMarket("reference price is not positive"));
        }

        // Buy whole base units: quantity = amount / price.
        let quantity = self.config.amount_per_interval.div(price)?;
        if quantity <= Fixed::ZERO {
            return Err(BotError::InvalidConfig("dca slice rounds to zero"));
        }

        self.last_buy_ms = Some(snapshot.timestamp_ms);
        self.intervals_done += 1;
        self.spent = self.spent.add(self.config.amount_per_interval)?;

        Ok(vec![BotOrder::new(
            snapshot.symbol.clone(),
            Side::Buy,
            OrderKind::Market,
            Fixed::ZERO,
            quantity,
            false,
            "dca",
        )?])
    }
}

/// Operator settings for the TWAP bot.
#[derive(Clone, Debug)]
pub struct TwapConfig {
    /// Total base quantity to sell or buy across the schedule.
    pub total_quantity: Fixed,
    /// Number of equal slices; must be at least one.
    pub slices: u32,
    /// Gap between slices, in milliseconds.
    pub interval_ms: i64,
    /// Which side the schedule trades.
    pub side: Side,
}

impl TwapConfig {
    pub fn validate(&self) -> BotResult<()> {
        if !self.total_quantity.is_positive() {
            return Err(BotError::InvalidConfig("total_quantity must be positive"));
        }
        if self.slices == 0 {
            return Err(BotError::InvalidConfig("slices must be at least one"));
        }
        if self.interval_ms <= 0 {
            return Err(BotError::InvalidConfig("interval_ms must be positive"));
        }
        Ok(())
    }
}

/// The TWAP bot's working state.
#[derive(Debug)]
pub struct TwapBot {
    config: TwapConfig,
    slice_size: Fixed,
    slices_done: u32,
    last_slice_ms: Option<i64>,
}

impl TwapBot {
    pub fn new(config: TwapConfig) -> BotResult<Self> {
        config.validate()?;
        let slice_size = config
            .total_quantity
            .div(Fixed::from_integer(config.slices as i128)?)?;
        if slice_size <= Fixed::ZERO {
            return Err(BotError::InvalidConfig("slice size rounds to zero"));
        }
        Ok(Self {
            config,
            slice_size,
            slices_done: 0,
            last_slice_ms: None,
        })
    }

    pub fn slices_done(&self) -> u32 {
        self.slices_done
    }

    pub fn slice_size(&self) -> Fixed {
        self.slice_size
    }

    pub fn is_complete(&self) -> bool {
        self.slices_done >= self.config.slices
    }

    /// Emits the next slice when its interval has elapsed.
    ///
    /// The final slice takes the remainder, so rounding cannot leave a dust
    /// quantity unsold.
    pub fn step(&mut self, snapshot: &MarketSnapshot) -> BotResult<Vec<BotOrder>> {
        if self.is_complete() {
            return Ok(Vec::new());
        }

        if let Some(last) = self.last_slice_ms {
            let elapsed = snapshot.timestamp_ms.saturating_sub(last);
            if elapsed < self.config.interval_ms {
                return Ok(Vec::new());
            }
        }

        let is_last = self.slices_done + 1 == self.config.slices;
        let quantity = if is_last {
            let placed = self.slice_size.mul(Fixed::from_integer(self.slices_done as i128)?)?;
            self.config.total_quantity.sub(placed)?
        } else {
            self.slice_size
        };

        if quantity <= Fixed::ZERO {
            return Err(BotError::InvalidConfig("twap remainder is not positive"));
        }

        self.last_slice_ms = Some(snapshot.timestamp_ms);
        self.slices_done += 1;

        Ok(vec![BotOrder::new(
            snapshot.symbol.clone(),
            self.config.side,
            OrderKind::Market,
            Fixed::ZERO,
            quantity,
            false,
            "twap",
        )?])
    }
}
