//! The copy-trading bot.
//!
//! A copy bot does not price a market of its own; it mirrors the orders a
//! followed trader actually placed, scaled by the follower's copy ratio. The
//! signals are the trader's own orders, gathered by the control plane from the
//! same books the trader trades on, so a copied order is always derived from a
//! real order rather than an inferred one.
//!
//! Scaling is exact fixed-point arithmetic: a 50% ratio halves a quantity with
//! no float rounding, and a signal that scales to zero is dropped rather than
//! sent as a dust order.

use crate::fixed::{BotError, BotResult, Fixed};
use crate::market::{BotOrder, OrderKind, Side};

/// The market a signal belongs to, so a follower can copy spot only, perps
/// only, or both.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Market {
    Spot,
    Perp,
}

/// One order a followed trader placed, as the control plane observed it.
#[derive(Clone, Debug)]
pub struct CopySignal {
    pub symbol: String,
    pub market: Market,
    pub side: Side,
    pub kind: OrderKind,
    pub price: Fixed,
    pub quantity: Fixed,
    /// The trader's order id, carried through so the copy is auditable back to
    /// its source.
    pub source_id: String,
}

/// Operator settings for the copy bot.
#[derive(Clone, Debug)]
pub struct CopyConfig {
    /// Fraction of the trader's size to copy, in basis points. 10_000 is a
    /// one-for-one copy; 1 is a hundredth of a percent.
    pub scale_bps: i64,
    /// Whether to copy perpetual orders as well as spot orders.
    pub copy_perps: bool,
}

impl CopyConfig {
    pub fn validate(&self) -> BotResult<()> {
        if self.scale_bps <= 0 || self.scale_bps > 10_000 {
            return Err(BotError::InvalidConfig("scale_bps must be within 1..=10000"));
        }
        Ok(())
    }
}

/// The copy bot.
#[derive(Debug)]
pub struct CopyBot {
    config: CopyConfig,
}

impl CopyBot {
    pub fn new(config: CopyConfig) -> BotResult<Self> {
        config.validate()?;
        Ok(Self { config })
    }

    pub fn config(&self) -> &CopyConfig {
        &self.config
    }

    /// Mirrors the trader's signals, scaled by the copy ratio.
    ///
    /// A signal on a market the follower does not copy is skipped, and a signal
    /// whose scaled size rounds to zero is dropped: a copy that would send a
    /// zero-quantity order is no copy at all.
    pub fn transform(&self, signals: &[CopySignal]) -> BotResult<Vec<BotOrder>> {
        let scale = Fixed::ratio(self.config.scale_bps as i128, 10_000)?;
        let mut orders = Vec::with_capacity(signals.len());
        for signal in signals {
            if signal.market == Market::Perp && !self.config.copy_perps {
                continue;
            }
            let quantity = signal.quantity.mul(scale)?;
            if quantity <= Fixed::ZERO {
                continue;
            }
            orders.push(BotOrder::new(
                signal.symbol.clone(),
                signal.side,
                signal.kind,
                signal.price,
                quantity,
                false,
                "copy",
            )?);
        }
        Ok(orders)
    }
}
