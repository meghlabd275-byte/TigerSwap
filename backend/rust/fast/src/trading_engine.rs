//! Real order book and matching engine for TigerSwap.
//!
//! The book keeps price-time priority using `BTreeMap` price levels. Matching is
//! a proper price/time walk: a marketable order consumes resting liquidity from
//! the best opposite level until it is filled, cancelled, or the book runs out.
//! Partial fills, self-trade prevention, time-in-force semantics and fee
//! accounting are all real.

use std::collections::{BTreeMap, HashMap};
use std::sync::atomic::{AtomicU64, Ordering};

use parking_lot::RwLock;

/// Order side.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u8)]
pub enum Side {
    Buy = 0,
    Sell = 1,
}

impl Side {
    pub fn from_u8(value: u8) -> Option<Self> {
        match value {
            0 => Some(Side::Buy),
            1 => Some(Side::Sell),
            _ => None,
        }
    }

    fn opposite(self) -> Side {
        match self {
            Side::Buy => Side::Sell,
            Side::Sell => Side::Buy,
        }
    }
}

/// Order type.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u8)]
pub enum OrderType {
    Market = 0,
    Limit = 1,
    Stop = 2,
    StopLimit = 3,
}

impl OrderType {
    pub fn from_u8(value: u8) -> Option<Self> {
        match value {
            0 => Some(OrderType::Market),
            1 => Some(OrderType::Limit),
            2 => Some(OrderType::Stop),
            3 => Some(OrderType::StopLimit),
            _ => None,
        }
    }
}

/// Order status.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(u8)]
pub enum OrderStatus {
    Pending = 0,
    Open = 1,
    PartiallyFilled = 2,
    Filled = 3,
    Cancelled = 4,
    Rejected = 5,
}

/// Time in force.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TimeInForce {
    /// Rest on the book until filled or cancelled.
    Gtc,
    /// Fill as much as possible, cancel the remainder.
    Ioc,
    /// Fill completely or reject.
    Fok,
    /// Only add liquidity, never cross the spread.
    PostOnly,
}

/// An order resting in the book or arriving for execution.
#[derive(Debug, Clone, Copy)]
#[repr(C)]
pub struct FastOrder {
    pub order_id: u64,
    pub trader_id: u64,
    pub pair_id: u64,
    pub price: u64,
    pub quantity: u64,
    pub filled: u64,
    pub side: u8,
    pub order_type: u8,
    pub status: u8,
    pub created_at: u64,
}

impl FastOrder {
    pub fn new(
        order_id: u64,
        trader_id: u64,
        pair_id: u64,
        price: u64,
        quantity: u64,
        side: u8,
        order_type: u8,
    ) -> Self {
        Self {
            order_id,
            trader_id,
            pair_id,
            price,
            quantity,
            filled: 0,
            side,
            order_type,
            status: OrderStatus::Open as u8,
            created_at: current_timestamp_ns(),
        }
    }

    pub fn side(&self) -> Option<Side> {
        Side::from_u8(self.side)
    }

    pub fn remaining(&self) -> u64 {
        self.quantity.saturating_sub(self.filled)
    }

    pub fn is_open(&self) -> bool {
        self.status == OrderStatus::Open as u8 || self.status == OrderStatus::PartiallyFilled as u8
    }
}

/// A completed match.
#[derive(Debug, Clone, Copy)]
#[repr(C)]
pub struct FastTrade {
    pub trade_id: u64,
    pub maker_order_id: u64,
    pub taker_order_id: u64,
    pub pair_id: u64,
    pub price: u64,
    pub quantity: u64,
    pub fee: u64,
    pub timestamp: u64,
}

/// Aggregated quantity at a single price.
#[derive(Debug, Clone, Copy)]
pub struct PriceLevel {
    pub price: u64,
    pub quantity: u64,
    pub orders: u32,
}

/// Reason an order could not be placed.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RejectReason {
    PostOnlyWouldCross,
    FillOrKillUnfillable,
    NoLiquidity,
    InvalidOrder,
    StopRequiresTrigger,
}

/// Result of submitting an order.
#[derive(Debug, Clone)]
pub enum SubmitOutcome {
    Accepted {
        order_id: u64,
        trades: Vec<FastTrade>,
        remaining: u64,
    },
    Rejected(RejectReason),
}

/// Price-time priority order book for one pair.
pub struct FastOrderBook {
    pair_id: u64,
    tick_size: u64,
    maker_fee_bps: u64,
    taker_fee_bps: u64,

    next_order_id: AtomicU64,
    next_trade_id: AtomicU64,

    bids: RwLock<BTreeMap<u64, Vec<FastOrder>>>,
    asks: RwLock<BTreeMap<u64, Vec<FastOrder>>>,
    orders: RwLock<HashMap<u64, FastOrder>>,
    trades: RwLock<Vec<FastTrade>>,
    last_price: AtomicU64,
    total_volume: AtomicU64,
}

impl FastOrderBook {
    /// Builds a book for `pair_id` with a minimum price increment of `tick_size`.
    pub fn new(pair_id: u64, tick_size: u64, maker_fee_bps: u64, taker_fee_bps: u64) -> Self {
        Self {
            pair_id,
            tick_size: tick_size.max(1),
            maker_fee_bps,
            taker_fee_bps,
            next_order_id: AtomicU64::new(1),
            next_trade_id: AtomicU64::new(1),
            bids: RwLock::new(BTreeMap::new()),
            asks: RwLock::new(BTreeMap::new()),
            orders: RwLock::new(HashMap::new()),
            trades: RwLock::new(Vec::new()),
            last_price: AtomicU64::new(0),
            total_volume: AtomicU64::new(0),
        }
    }

    pub fn pair_id(&self) -> u64 {
        self.pair_id
    }

    /// Submits an order using good-till-cancelled semantics.
    pub fn submit_order(&self, order: FastOrder) -> SubmitOutcome {
        self.submit(order, TimeInForce::Gtc)
    }

    /// Submits an order with an explicit time in force.
    pub fn submit(&self, mut order: FastOrder, tif: TimeInForce) -> SubmitOutcome {
        let side = match order.side() {
            Some(side) => side,
            None => return SubmitOutcome::Rejected(RejectReason::InvalidOrder),
        };

        if order.quantity == 0 {
            return SubmitOutcome::Rejected(RejectReason::InvalidOrder);
        }

        let order_type = match OrderType::from_u8(order.order_type) {
            Some(order_type) => order_type,
            None => return SubmitOutcome::Rejected(RejectReason::InvalidOrder),
        };

        if matches!(order_type, OrderType::Stop | OrderType::StopLimit) {
            // Stop orders must be armed by a trigger; they are never marketable on arrival.
            return SubmitOutcome::Rejected(RejectReason::StopRequiresTrigger);
        }

        // `tick_size` is clamped to at least 1 in `new`, so the modulo is safe.
        if matches!(order_type, OrderType::Limit)
            && (order.price == 0 || order.price % self.tick_size != 0)
        {
            return SubmitOutcome::Rejected(RejectReason::InvalidOrder);
        }

        let order_id = self.next_order_id.fetch_add(1, Ordering::AcqRel);
        order.order_id = order_id;
        order.pair_id = self.pair_id;

        if matches!(tif, TimeInForce::PostOnly) && self.would_cross(&order, side) {
            return SubmitOutcome::Rejected(RejectReason::PostOnlyWouldCross);
        }

        if matches!(tif, TimeInForce::Fok) && self.fillable_quantity(&order, side) < order.quantity {
            return SubmitOutcome::Rejected(RejectReason::FillOrKillUnfillable);
        }

        let trades = self.match_order(&mut order, side);
        let mut remaining = order.remaining();

        if matches!(order_type, OrderType::Market) {
            // A market order never rests: it either filled or it is cancelled.
            if order.filled == 0 {
                return SubmitOutcome::Rejected(RejectReason::NoLiquidity);
            }
            // `filled` must stay the executed quantity. Overwriting it with the
            // requested size would report a partially filled market order as
            // complete and book volume that never traded.
            order.status = if remaining > 0 {
                OrderStatus::Cancelled as u8
            } else {
                OrderStatus::Filled as u8
            };
            remaining = 0;
            self.orders.write().insert(order_id, order);
            return SubmitOutcome::Accepted {
                order_id,
                trades,
                remaining,
            };
        }

        if remaining > 0 && matches!(tif, TimeInForce::Ioc | TimeInForce::Fok) {
            // The remainder is cancelled, so it must not be recorded as filled
            // or as partially filled and resting.
            order.status = OrderStatus::Cancelled as u8;
            self.orders.write().insert(order_id, order);
            return SubmitOutcome::Accepted {
                order_id,
                trades,
                remaining: 0,
            };
        }

        if remaining > 0 {
            if self.would_cross(&order, side) {
                // Only the trader's own resting liquidity is left in the way.
                // Resting here would leave a crossed book, so cancel instead.
                order.status = OrderStatus::Cancelled as u8;
                self.orders.write().insert(order_id, order);
                return SubmitOutcome::Accepted {
                    order_id,
                    trades,
                    remaining: 0,
                };
            }

            order.status = if order.filled > 0 {
                OrderStatus::PartiallyFilled as u8
            } else {
                OrderStatus::Open as u8
            };
            self.rest_order(order, side);
        } else {
            order.status = OrderStatus::Filled as u8;
        }

        self.orders.write().insert(order_id, order);

        SubmitOutcome::Accepted {
            order_id,
            trades,
            remaining,
        }
    }

    /// Cancels a resting order owned by `trader_id`.
    pub fn cancel_order(&self, order_id: u64, trader_id: u64) -> bool {
        let snapshot = {
            let mut orders = self.orders.write();
            let order = match orders.get_mut(&order_id) {
                Some(order) => order,
                None => return false,
            };

            if order.trader_id != trader_id || !order.is_open() {
                return false;
            }

            // Only the status changes: overwriting `filled` with the full
            // quantity would report a partially filled order as completely
            // executed.
            order.status = OrderStatus::Cancelled as u8;
            *order
        };

        let side = match snapshot.side() {
            Some(side) => side,
            None => return false,
        };

        let mut book = match side {
            Side::Buy => self.bids.write(),
            Side::Sell => self.asks.write(),
        };

        if let Some(resting) = book.get_mut(&snapshot.price) {
            resting.retain(|order| order.order_id != order_id);
            if resting.is_empty() {
                book.remove(&snapshot.price);
            }
        }

        true
    }

    pub fn get_order(&self, order_id: u64) -> Option<FastOrder> {
        self.orders.read().get(&order_id).copied()
    }

    /// Best bid and best ask, if both sides are populated.
    pub fn get_spread(&self) -> Option<(u64, u64)> {
        let best_bid = self.bids.read().keys().next_back().copied();
        let best_ask = self.asks.read().keys().next().copied();

        match (best_bid, best_ask) {
            (Some(bid), Some(ask)) => Some((bid, ask)),
            _ => None,
        }
    }

    /// Last traded price, or zero before the first trade.
    pub fn last_price(&self) -> u64 {
        self.last_price.load(Ordering::Acquire)
    }

    pub fn total_volume(&self) -> u64 {
        self.total_volume.load(Ordering::Acquire)
    }

    /// Aggregated depth: asks ascending, then bids descending.
    pub fn get_depth(&self, levels: usize) -> Vec<PriceLevel> {
        let mut result = Vec::new();

        {
            let asks = self.asks.read();
            for (price, resting) in asks.iter().take(levels) {
                result.push(PriceLevel {
                    price: *price,
                    quantity: resting.iter().map(|order| order.remaining()).sum(),
                    orders: resting.len() as u32,
                });
            }
        }

        {
            let bids = self.bids.read();
            for (price, resting) in bids.iter().rev().take(levels) {
                result.push(PriceLevel {
                    price: *price,
                    quantity: resting.iter().map(|order| order.remaining()).sum(),
                    orders: resting.len() as u32,
                });
            }
        }

        result
    }

    /// All trades executed so far.
    pub fn trades(&self) -> Vec<FastTrade> {
        self.trades.read().clone()
    }

    /// Summary statistics for the book.
    pub fn get_stats(&self) -> OrderBookStats {
        let bid_levels = self.bids.read().len() as u64;
        let ask_levels = self.asks.read().len() as u64;
        let open_orders = self
            .orders
            .read()
            .values()
            .filter(|order| order.is_open())
            .count() as u64;

        OrderBookStats {
            pair_id: self.pair_id,
            bid_levels,
            ask_levels,
            open_orders,
            trades: self.trades.read().len() as u64,
            last_price: self.last_price(),
            total_volume: self.total_volume(),
            maker_fee_bps: self.maker_fee_bps,
            taker_fee_bps: self.taker_fee_bps,
        }
    }

    // ---- internals ----

    fn best_price(&self, side: Side) -> Option<u64> {
        let book = match side {
            Side::Buy => self.bids.read(),
            Side::Sell => self.asks.read(),
        };
        match side {
            Side::Buy => book.keys().next_back().copied(),
            Side::Sell => book.keys().next().copied(),
        }
    }

    fn would_cross(&self, order: &FastOrder, side: Side) -> bool {
        match self.best_price(side.opposite()) {
            Some(best) => match side {
                Side::Buy => order.price >= best,
                Side::Sell => order.price <= best,
            },
            None => false,
        }
    }

    fn fillable_quantity(&self, order: &FastOrder, side: Side) -> u64 {
        let mut available = 0u64;
        // A market order has no price bound, so it can take every level. Applying
        // the limit price here would make the check depend on the placeholder
        // price and reject, for example, every fill-or-kill market buy.
        let is_market = order.order_type == OrderType::Market as u8;

        match side {
            Side::Buy => {
                for (price, resting) in self.asks.read().iter() {
                    if !is_market && price > &order.price {
                        break;
                    }
                    available += resting.iter().map(|o| o.remaining()).sum::<u64>();
                    if available >= order.quantity {
                        return available;
                    }
                }
            }
            Side::Sell => {
                for (price, resting) in self.bids.read().iter().rev() {
                    if !is_market && price < &order.price {
                        break;
                    }
                    available += resting.iter().map(|o| o.remaining()).sum::<u64>();
                    if available >= order.quantity {
                        return available;
                    }
                }
            }
        }

        available
    }

    fn match_order(&self, order: &mut FastOrder, side: Side) -> Vec<FastTrade> {
        let mut trades = Vec::new();
        let opposite = side.opposite();
        // A market order takes any available liquidity regardless of price.
        let is_market = order.order_type == OrderType::Market as u8;

        while order.remaining() > 0 {
            let price = match self.best_price(opposite) {
                Some(price) => price,
                None => break,
            };

            if !is_market {
                let crosses = match side {
                    Side::Buy => price <= order.price,
                    Side::Sell => price >= order.price,
                };
                if !crosses {
                    break;
                }
            }

            let filled_at_level = self.match_level(order, opposite, price, &mut trades);
            if filled_at_level == 0 {
                // The level held only our own orders; stop rather than loop.
                break;
            }
        }

        if !trades.is_empty() {
            if let Some(last) = trades.last() {
                self.last_price.store(last.price, Ordering::Release);
            }
            let volume: u64 = trades.iter().map(|trade| trade.quantity).sum();
            self.total_volume.fetch_add(volume, Ordering::AcqRel);
            self.trades.write().extend_from_slice(&trades);
        }

        trades
    }

    /// Consumes liquidity from a single price level, returning the filled quantity.
    fn match_level(
        &self,
        order: &mut FastOrder,
        opposite: Side,
        price: u64,
        trades: &mut Vec<FastTrade>,
    ) -> u64 {
        let mut filled = 0u64;

        let mut book = match opposite {
            Side::Buy => self.bids.write(),
            Side::Sell => self.asks.write(),
        };

        let resting = match book.get_mut(&price) {
            Some(resting) => resting,
            None => return 0,
        };

        let mut index = 0;
        while index < resting.len() && order.remaining() > 0 {
            if resting[index].trader_id == order.trader_id {
                // Self-trade prevention: skip our own resting liquidity.
                index += 1;
                continue;
            }

            let fill = resting[index].remaining().min(order.remaining());
            resting[index].filled += fill;
            order.filled += fill;
            filled += fill;

            let trade_id = self.next_trade_id.fetch_add(1, Ordering::AcqRel);
            let notional = price.saturating_mul(fill);
            let fee = notional.saturating_mul(self.taker_fee_bps) / 10_000;

            trades.push(FastTrade {
                trade_id,
                maker_order_id: resting[index].order_id,
                taker_order_id: order.order_id,
                pair_id: self.pair_id,
                price,
                quantity: fill,
                fee,
                timestamp: current_timestamp_ns(),
            });

            if resting[index].remaining() == 0 {
                resting[index].status = OrderStatus::Filled as u8;
                let maker_snapshot = resting.remove(index);
                self.orders.write().insert(maker_snapshot.order_id, maker_snapshot);
            } else {
                resting[index].status = OrderStatus::PartiallyFilled as u8;
                let maker_snapshot = resting[index];
                self.orders.write().insert(maker_snapshot.order_id, maker_snapshot);
                index += 1;
            }
        }

        if resting.is_empty() {
            book.remove(&price);
        }

        filled
    }

    fn rest_order(&self, order: FastOrder, side: Side) {
        let mut book = match side {
            Side::Buy => self.bids.write(),
            Side::Sell => self.asks.write(),
        };
        book.entry(order.price).or_default().push(order);
    }
}

/// Snapshot of order book statistics.
#[derive(Debug, Clone, Copy)]
pub struct OrderBookStats {
    pub pair_id: u64,
    pub bid_levels: u64,
    pub ask_levels: u64,
    pub open_orders: u64,
    pub trades: u64,
    pub last_price: u64,
    pub total_volume: u64,
    pub maker_fee_bps: u64,
    pub taker_fee_bps: u64,
}

/// Nanoseconds since the Unix epoch.
pub fn current_timestamp_ns() -> u64 {
    use std::time::{SystemTime, UNIX_EPOCH};
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map(|duration| duration.as_nanos() as u64)
        .unwrap_or(0)
}

#[cfg(test)]
mod tests {
    use super::*;

    fn book() -> FastOrderBook {
        FastOrderBook::new(1, 1, 0, 2)
    }

    fn limit(trader: u64, price: u64, qty: u64, side: Side) -> FastOrder {
        FastOrder::new(0, trader, 0, price, qty, side as u8, OrderType::Limit as u8)
    }

    fn market(trader: u64, qty: u64, side: Side) -> FastOrder {
        FastOrder::new(0, trader, 0, 0, qty, side as u8, OrderType::Market as u8)
    }

    #[test]
    fn resting_orders_populate_the_book() {
        let book = book();
        assert!(matches!(
            book.submit_order(limit(1, 100, 10, Side::Buy)),
            SubmitOutcome::Accepted { .. }
        ));
        assert!(matches!(
            book.submit_order(limit(2, 101, 5, Side::Sell)),
            SubmitOutcome::Accepted { .. }
        ));

        assert_eq!(book.get_spread(), Some((100, 101)));
        assert_eq!(book.get_depth(10).len(), 2);
    }

    #[test]
    fn crossing_order_trades_at_maker_price() {
        let book = book();
        book.submit_order(limit(1, 100, 10, Side::Sell));

        match book.submit_order(limit(2, 105, 4, Side::Buy)) {
            SubmitOutcome::Accepted { trades, .. } => {
                assert_eq!(trades.len(), 1);
                assert_eq!(trades[0].price, 100);
                assert_eq!(trades[0].quantity, 4);
                assert_eq!(trades[0].fee, 100 * 4 * 2 / 10_000);
            }
            other => panic!("unexpected outcome {other:?}"),
        }

        assert_eq!(book.last_price(), 100);
        assert_eq!(book.total_volume(), 4);
    }

    #[test]
    fn partial_fill_leaves_remainder_resting() {
        let book = book();
        book.submit_order(limit(1, 100, 10, Side::Sell));

        match book.submit(limit(2, 100, 25, Side::Buy), TimeInForce::Gtc) {
            SubmitOutcome::Accepted { remaining, trades, .. } => {
                assert_eq!(trades.len(), 1);
                assert_eq!(remaining, 15);
            }
            other => panic!("unexpected outcome {other:?}"),
        }

        let depth = book.get_depth(5);
        assert_eq!(depth.len(), 1);
        assert_eq!(depth[0].price, 100);
        assert_eq!(depth[0].quantity, 15);
    }

    #[test]
    fn ioc_cancels_the_unfilled_remainder() {
        let book = book();
        book.submit_order(limit(1, 100, 5, Side::Sell));

        match book.submit(limit(2, 100, 20, Side::Buy), TimeInForce::Ioc) {
            SubmitOutcome::Accepted { trades, .. } => assert_eq!(trades.len(), 1),
            other => panic!("unexpected outcome {other:?}"),
        }

        assert!(book.get_depth(5).is_empty());
    }

    #[test]
    fn fok_rejects_when_book_is_too_thin() {
        let book = book();
        book.submit_order(limit(1, 100, 5, Side::Sell));

        assert!(matches!(
            book.submit(limit(2, 100, 20, Side::Buy), TimeInForce::Fok),
            SubmitOutcome::Rejected(RejectReason::FillOrKillUnfillable)
        ));
        assert!(book.trades().is_empty());
    }

    #[test]
    fn fok_fills_completely_when_liquidity_suffices() {
        let book = book();
        book.submit_order(limit(1, 100, 30, Side::Sell));

        match book.submit(limit(2, 100, 20, Side::Buy), TimeInForce::Fok) {
            SubmitOutcome::Accepted { trades, remaining, .. } => {
                assert_eq!(trades.len(), 1);
                assert_eq!(remaining, 0);
            }
            other => panic!("unexpected outcome {other:?}"),
        }
    }

    #[test]
    fn post_only_never_crosses() {
        let book = book();
        book.submit_order(limit(1, 100, 5, Side::Sell));

        assert!(matches!(
            book.submit(limit(2, 100, 5, Side::Buy), TimeInForce::PostOnly),
            SubmitOutcome::Rejected(RejectReason::PostOnlyWouldCross)
        ));
        assert!(book.trades().is_empty());
    }

    #[test]
    fn post_only_rests_when_not_crossing() {
        let book = book();
        book.submit_order(limit(1, 105, 5, Side::Sell));

        assert!(matches!(
            book.submit(limit(2, 100, 5, Side::Buy), TimeInForce::PostOnly),
            SubmitOutcome::Accepted { .. }
        ));
        assert_eq!(book.get_spread(), Some((100, 105)));
    }

    #[test]
    fn market_order_sweeps_multiple_levels() {
        let book = book();
        book.submit_order(limit(1, 100, 5, Side::Sell));
        book.submit_order(limit(2, 101, 5, Side::Sell));
        book.submit_order(limit(3, 102, 5, Side::Sell));

        match book.submit_order(market(9, 12, Side::Buy)) {
            SubmitOutcome::Accepted { trades, .. } => {
                assert_eq!(trades.len(), 3);
                assert_eq!(trades[0].price, 100);
                assert_eq!(trades[1].price, 101);
                assert_eq!(trades[2].price, 102);
                assert_eq!(trades.iter().map(|t| t.quantity).sum::<u64>(), 12);
            }
            other => panic!("unexpected outcome {other:?}"),
        }

        // The last level was only partially consumed, so 3 units remain resting.
        let depth = book.get_depth(10);
        assert_eq!(depth.len(), 1);
        assert_eq!(depth[0].price, 102);
        assert_eq!(depth[0].quantity, 3);
    }

    #[test]
    fn market_order_without_liquidity_is_rejected() {
        let book = book();
        assert!(matches!(
            book.submit_order(market(9, 12, Side::Buy)),
            SubmitOutcome::Rejected(RejectReason::NoLiquidity)
        ));
    }

    #[test]
    fn self_trade_is_prevented() {
        let book = book();
        book.submit_order(limit(1, 100, 10, Side::Sell));

        match book.submit_order(limit(1, 100, 10, Side::Buy)) {
            SubmitOutcome::Accepted { trades, .. } => assert!(trades.is_empty()),
            other => panic!("unexpected outcome {other:?}"),
        }
        assert!(book.trades().is_empty());
    }

    #[test]
    fn cancel_removes_liquidity() {
        let book = book();
        let order_id = match book.submit_order(limit(1, 100, 10, Side::Buy)) {
            SubmitOutcome::Accepted { order_id, .. } => order_id,
            other => panic!("unexpected outcome {other:?}"),
        };

        assert!(!book.cancel_order(order_id, 999));
        assert!(book.cancel_order(order_id, 1));
        assert!(!book.cancel_order(order_id, 1));
        assert!(book.get_depth(5).is_empty());
    }

    #[test]
    fn price_must_respect_tick_size() {
        let book = FastOrderBook::new(1, 10, 0, 2);
        assert!(matches!(
            book.submit_order(limit(1, 105, 10, Side::Buy)),
            SubmitOutcome::Rejected(RejectReason::InvalidOrder)
        ));
    }

    #[test]
    fn zero_quantity_is_rejected() {
        let book = book();
        assert!(matches!(
            book.submit_order(limit(1, 100, 0, Side::Buy)),
            SubmitOutcome::Rejected(RejectReason::InvalidOrder)
        ));
    }

    #[test]
    fn stop_orders_are_not_immediately_marketable() {
        let book = book();
        let stop = FastOrder::new(0, 1, 0, 100, 10, Side::Buy as u8, OrderType::Stop as u8);
        assert!(matches!(
            book.submit_order(stop),
            SubmitOutcome::Rejected(RejectReason::StopRequiresTrigger)
        ));
    }

    /// A fill-or-kill market order must be evaluated against all available
    /// liquidity, exactly like a fill-or-kill limit order that crosses.
    #[test]
    fn fok_market_order_is_symmetric_across_sides() {
        let bids = FastOrderBook::new(1, 1, 0, 2);
        bids.submit_order(limit(1, 100, 30, Side::Buy));
        assert!(matches!(
            bids.submit(market(2, 20, Side::Sell), TimeInForce::Fok),
            SubmitOutcome::Accepted { remaining: 0, .. }
        ));

        let asks = FastOrderBook::new(1, 1, 0, 2);
        asks.submit_order(limit(1, 100, 30, Side::Sell));
        assert!(
            matches!(
                asks.submit(market(2, 20, Side::Buy), TimeInForce::Fok),
                SubmitOutcome::Accepted { remaining: 0, .. }
            ),
            "a market buy has no price bound and must fill against the asks"
        );
    }

    /// A fill-or-kill market order still fails when the book is too thin.
    #[test]
    fn fok_market_order_rejects_when_thin() {
        let book = FastOrderBook::new(1, 1, 0, 2);
        book.submit_order(limit(1, 100, 5, Side::Sell));
        assert!(matches!(
            book.submit(market(2, 20, Side::Buy), TimeInForce::Fok),
            SubmitOutcome::Rejected(RejectReason::FillOrKillUnfillable)
        ));
        assert!(book.trades().is_empty());
    }

    /// A partially filled market order must report the quantity that actually
    /// traded, not the quantity that was requested, and must leave no phantom
    /// resting order behind.
    #[test]
    fn market_order_reports_only_the_filled_quantity() {
        let book = book();
        book.submit_order(limit(1, 100, 12, Side::Sell));

        let order_id = match book.submit_order(market(2, 20, Side::Buy)) {
            SubmitOutcome::Accepted { order_id, trades, .. } => {
                assert_eq!(trades.iter().map(|t| t.quantity).sum::<u64>(), 12);
                order_id
            }
            other => panic!("unexpected outcome {other:?}"),
        };

        let order = book.get_order(order_id).expect("order recorded");
        assert_eq!(order.filled, 12, "filled must be the executed quantity");
        assert_eq!(order.remaining(), 8);
        assert!(!order.is_open(), "a market order never rests");
        assert_eq!(book.total_volume(), 12);
        assert_eq!(book.get_stats().open_orders, 0);
        assert!(book.get_depth(5).is_empty());
    }

    /// The unfilled remainder of an IOC order is cancelled, so it must not be
    /// counted as an open or partially filled resting order.
    #[test]
    fn ioc_cancelled_remainder_is_not_open() {
        let book = book();
        book.submit_order(limit(1, 100, 5, Side::Sell));

        let order_id = match book.submit(limit(2, 100, 20, Side::Buy), TimeInForce::Ioc) {
            SubmitOutcome::Accepted { order_id, trades, .. } => {
                assert_eq!(trades.len(), 1);
                order_id
            }
            other => panic!("unexpected outcome {other:?}"),
        };

        let order = book.get_order(order_id).expect("order recorded");
        assert_eq!(order.filled, 5);
        assert_eq!(order.status, OrderStatus::Cancelled as u8);
        assert!(!order.is_open());
        assert_eq!(book.get_stats().open_orders, 0);
    }

    /// A cancelled partially filled order must keep the quantity it executed.
    #[test]
    fn cancel_preserves_the_executed_quantity() {
        let book = book();
        book.submit_order(limit(1, 100, 10, Side::Sell));

        let order_id = match book.submit(limit(2, 100, 25, Side::Buy), TimeInForce::Gtc) {
            SubmitOutcome::Accepted { order_id, .. } => order_id,
            other => panic!("unexpected outcome {other:?}"),
        };

        assert!(book.cancel_order(order_id, 2));
        let order = book.get_order(order_id).expect("order recorded");
        assert_eq!(order.filled, 10, "only 10 units traded");
        assert_eq!(order.remaining(), 15);
        assert_eq!(order.status, OrderStatus::Cancelled as u8);
    }

    /// Self-trade prevention must not leave a crossed book behind.
    #[test]
    fn self_trade_does_not_leave_a_crossed_book() {
        let book = book();
        book.submit_order(limit(1, 100, 10, Side::Sell));

        match book.submit_order(limit(1, 100, 10, Side::Buy)) {
            SubmitOutcome::Accepted { trades, .. } => assert!(trades.is_empty()),
            other => panic!("unexpected outcome {other:?}"),
        }

        assert!(book.trades().is_empty());
        assert_eq!(book.get_spread(), None, "the incoming order must not rest");
        // The original resting sell is untouched.
        let depth = book.get_depth(5);
        assert_eq!(depth.len(), 1);
        assert_eq!(depth[0].quantity, 10);
    }

    #[test]
    fn stats_track_book_activity() {
        let book = book();
        book.submit_order(limit(1, 100, 10, Side::Sell));
        book.submit_order(limit(2, 99, 5, Side::Buy));
        book.submit_order(limit(3, 100, 4, Side::Buy));

        let stats = book.get_stats();
        assert_eq!(stats.pair_id, 1);
        assert_eq!(stats.trades, 1);
        assert_eq!(stats.last_price, 100);
        assert_eq!(stats.total_volume, 4);
        assert_eq!(stats.taker_fee_bps, 2);
        // The 99 bid and the partially filled sell both remain open.
        assert_eq!(stats.open_orders, 2);
    }

    #[test]
    fn aggressive_sell_hits_bids_in_descending_price_order() {
        let book = book();
        book.submit_order(limit(1, 100, 5, Side::Buy));
        book.submit_order(limit(2, 99, 5, Side::Buy));

        match book.submit(limit(3, 99, 10, Side::Sell), TimeInForce::Gtc) {
            SubmitOutcome::Accepted { trades, .. } => {
                assert_eq!(trades.len(), 2);
                assert_eq!(trades[0].price, 100);
                assert_eq!(trades[1].price, 99);
            }
            other => panic!("unexpected outcome {other:?}"),
        }
    }

    #[test]
    fn depth_aggregates_orders_at_the_same_price() {
        let book = book();
        book.submit_order(limit(1, 100, 10, Side::Buy));
        book.submit_order(limit(2, 100, 15, Side::Buy));

        let depth = book.get_depth(5);
        assert_eq!(depth.len(), 1);
        assert_eq!(depth[0].quantity, 25);
        assert_eq!(depth[0].orders, 2);
    }

    #[test]
    fn cancel_then_rebook_is_consistent() {
        let book = book();
        let first = match book.submit_order(limit(1, 100, 10, Side::Buy)) {
            SubmitOutcome::Accepted { order_id, .. } => order_id,
            other => panic!("unexpected outcome {other:?}"),
        };
        assert!(book.cancel_order(first, 1));

        book.submit_order(limit(2, 100, 7, Side::Buy));
        let depth = book.get_depth(5);
        assert_eq!(depth[0].quantity, 7);

        let stats = book.get_stats();
        assert_eq!(stats.open_orders, 1);
    }
}
