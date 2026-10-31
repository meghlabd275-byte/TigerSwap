//! The market view a bot reasons about, and the orders it emits.
//!
//! A bot never talks to a venue directly. It receives a [`MarketSnapshot`] and
//! returns [`BotOrder`]s, which the caller executes. That keeps every bot
//! deterministic and testable without a network.

use crate::fixed::{BotError, BotResult, Fixed};

/// One price level of the order book.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub struct Level {
    pub price: Fixed,
    pub quantity: Fixed,
}

impl Level {
    pub fn new(price: Fixed, quantity: Fixed) -> Self {
        Self { price, quantity }
    }
}

/// The side an order is placed on.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum Side {
    Buy,
    Sell,
}

impl Side {
    pub fn opposite(self) -> Side {
        match self {
            Side::Buy => Side::Sell,
            Side::Sell => Side::Buy,
        }
    }
}

/// A point-in-time view of one market.
///
/// `bids` are ordered best-first (highest price) and `asks` best-first
/// (lowest price), matching what the matching engine emits.
#[derive(Clone, Debug)]
pub struct MarketSnapshot {
    pub symbol: String,
    pub bids: Vec<Level>,
    pub asks: Vec<Level>,
    /// Volume-weighted price over a recent window, in quote currency.
    pub vwap: Fixed,
    /// Unix milliseconds the snapshot was taken.
    pub timestamp_ms: i64,
}

impl MarketSnapshot {
    /// Builds a snapshot, checking that the book is ordered and uncrossed.
    pub fn new(
        symbol: impl Into<String>,
        bids: Vec<Level>,
        asks: Vec<Level>,
        vwap: Fixed,
        timestamp_ms: i64,
    ) -> BotResult<Self> {
        for pair in bids.windows(2) {
            if pair[0].price < pair[1].price {
                return Err(BotError::InvalidMarket("bids are not ordered best-first"));
            }
        }
        for pair in asks.windows(2) {
            if pair[0].price > pair[1].price {
                return Err(BotError::InvalidMarket("asks are not ordered best-first"));
            }
        }
        if let (Some(best_bid), Some(best_ask)) = (bids.first(), asks.first()) {
            if best_bid.price >= best_ask.price {
                return Err(BotError::InvalidMarket("book is crossed"));
            }
        }
        Ok(Self {
            symbol: symbol.into(),
            bids,
            asks,
            vwap,
            timestamp_ms,
        })
    }

    pub fn best_bid(&self) -> Option<Fixed> {
        self.bids.first().map(|level| level.price)
    }

    pub fn best_ask(&self) -> Option<Fixed> {
        self.asks.first().map(|level| level.price)
    }

    /// The mid price, or the last trade price when one side is empty.
    ///
    /// Returns `None` only when the book is completely empty, so a bot never
    /// invents a price out of nothing.
    pub fn mid(&self) -> Option<Fixed> {
        match (self.best_bid(), self.best_ask()) {
            (Some(bid), Some(ask)) => bid.add(ask).ok().map(|sum| Fixed::from_raw(sum.raw() / 2)),
            (Some(bid), None) => Some(bid),
            (None, Some(ask)) => Some(ask),
            (None, None) => None,
        }
    }

    /// A reference price for sizing: the mid when the book has both sides,
    /// otherwise the volume-weighted price.
    pub fn reference_price(&self) -> BotResult<Fixed> {
        if let Some(mid) = self.mid() {
            return Ok(mid);
        }
        if self.vwap.is_positive() {
            return Ok(self.vwap);
        }
        Err(BotError::InvalidMarket("no price available"))
    }

    /// Depth available within `bps` of the mid, on one side.
    pub fn depth_within(&self, side: Side, bps: i64) -> BotResult<Fixed> {
        let reference = self.reference_price()?;
        let bound = reference.apply_bps(if side == Side::Buy { -bps } else { bps })?;
        let levels = match side {
            Side::Buy => &self.bids,
            Side::Sell => &self.asks,
        };
        let mut total = Fixed::ZERO;
        for level in levels {
            let in_range = match side {
                Side::Buy => level.price >= bound,
                Side::Sell => level.price <= bound,
            };
            if !in_range {
                break;
            }
            total = total.add(level.quantity)?;
        }
        Ok(total)
    }
}

/// The kind of order a bot wants placed.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum OrderKind {
    /// Rest at `price` until filled or cancelled.
    Limit,
    /// Take whatever is available up to the limit price.
    Market,
    /// Rest only if it would not take liquidity.
    PostOnly,
}

/// An order a bot wants the caller to place.
#[derive(Clone, Debug, PartialEq, Eq)]
pub struct BotOrder {
    pub symbol: String,
    pub side: Side,
    pub kind: OrderKind,
    pub price: Fixed,
    pub quantity: Fixed,
    /// Reduce a position rather than open one.
    pub reduce_only: bool,
    /// A short tag identifying which bot produced the order.
    pub source: &'static str,
}

impl BotOrder {
    /// Validates the order before it is returned to the caller.
    pub fn new(
        symbol: impl Into<String>,
        side: Side,
        kind: OrderKind,
        price: Fixed,
        quantity: Fixed,
        reduce_only: bool,
        source: &'static str,
    ) -> BotResult<Self> {
        if quantity <= Fixed::ZERO {
            return Err(BotError::InvalidConfig("order quantity must be positive"));
        }
        if kind != OrderKind::Market && price <= Fixed::ZERO {
            return Err(BotError::InvalidConfig("limit order price must be positive"));
        }
        Ok(Self {
            symbol: symbol.into(),
            side,
            kind,
            price,
            quantity,
            reduce_only,
            source,
        })
    }
}
