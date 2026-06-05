//! TigerSwap Order Book - Rust Implementation
//! High-performance order book for perpetual trading
//! Deterministic matching with price-time priority

use parking_lot::RwLock;
use std::collections::{BTreeMap, HashMap};
use std::sync::Arc;

// ============================================================================
// Constants
// ============================================================================

const MAX_ORDER_LEVELS: usize = 1000;
const PRICE_DECIMAL_PRECISION: u64 = 8;

// ============================================================================
// Data Structures
// ============================================================================

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum OrderSide {
    Bid,  // Buy / Long
    Ask,  // Sell / Short
}

impl OrderSide {
    pub fn opposite(&self) -> Self {
        match self {
            OrderSide::Bid => OrderSide::Ask,
            OrderSide::Ask => OrderSide::Bid,
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum OrderType {
    Market,
    Limit,
    StopLoss,
    StopLimit,
    TakeProfit,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum OrderStatus {
    Open,
    Partial,
    Filled,
    Cancelled,
    Expired,
}

#[derive(Debug, Clone)]
pub struct Order {
    pub id: String,
    pub user: String,
    pub market: String,
    pub side: OrderSide,
    pub order_type: OrderType,
    pub price: u64,      // Price in quote currency (scaled)
    pub size: u64,       // Order size in base currency
    pub filled: u64,     // Filled amount
    pub trigger_price: Option<u64>,
    pub post_only: bool,
    pub reduce_only: bool,
    pub time_in_force: TimeInForce,
    pub status: OrderStatus,
    pub created_at: u64,
    pub updated_at: u64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TimeInForce {
    GTC,  // Good Till Cancel
    IOC,  // Immediate Or Cancel
    FOK,  // Fill Or Kill
    GTD,  // Good Till Date
}

#[derive(Debug, Clone)]
pub struct OrderLevel {
    pub price: u64,
    pub orders: Vec<Order>,
    pub total_size: u64,
}

#[derive(Debug, Clone)]
pub struct Market {
    pub name: String,
    pub base_currency: String,
    pub quote_currency: String,
    pub mark_price: u64,
    pub index_price: u64,
    pub bid_price: u64,
    pub ask_price: u64,
    pub last_price: u64,
    pub high_24h: u64,
    pub low_24h: u64,
    pub volume_24h: u64,
    pub funding_rate: i64,
    pub next_funding_time: u64,
}

impl Market {
    pub fn new(name: &str, base: &str, quote: &str) -> Self {
        Self {
            name: name.to_string(),
            base_currency: base.to_string(),
            quote_currency: quote.to_string(),
            mark_price: 0,
            index_price: 0,
            bid_price: 0,
            ask_price: 0,
            last_price: 0,
            high_24h: 0,
            low_24h: 0,
            volume_24h: 0,
            funding_rate: 0,
            next_funding_time: 0,
        }
    }
}

#[derive(Debug, Clone)]
pub struct OrderBook {
    pub market: String,
    pub bids: BTreeMap<u64, Vec<Order>>,  // Price -> Orders (desc by price)
    pub asks: BTreeMap<u64, Vec<Order>>,   // Price -> Orders (asc by price)
    pub last_update_id: u64,
}

impl OrderBook {
    pub fn new(market: &str) -> Self {
        Self {
            market: market.to_string(),
            bids: BTreeMap::new(),
            asks: BTreeMap::new(),
            last_update_id: 0,
        }
    }
}

#[derive(Debug, Clone)]
pub struct Trade {
    pub id: String,
    pub market: String,
    pub price: u64,
    pub size: u64,
    pub side: OrderSide,
    pub maker_order_id: String,
    pub taker_order_id: String,
    pub maker: String,
    pub taker: String,
    pub timestamp: u64,
}

#[derive(Debug, Clone)]
pub struct Position {
    pub id: String,
    pub user: String,
    pub market: String,
    pub side: OrderSide,
    pub size: u64,
    pub entry_price: u64,
    pub unrealized_pnl: i64,
    pub realized_pnl: i64,
    pub margin: u64,
    pub leverage: u32,
    pub liquidation_price: u64,
    pub status: PositionStatus,
}

#[derive(Debug, Clone, Copy, PartialEq)]
pub enum PositionStatus {
    Open,
    Liquidating,
    Liquidated,
    Closed,
}

#[derive(Debug, Clone)]
pub struct Fill {
    pub order_id: String,
    pub trade_id: String,
    pub price: u64,
    pub size: u64,
    pub fee: u64,
    pub side: OrderSide,
    pub timestamp: u64,
}

// ============================================================================
// Order Book Engine
// ============================================================================

pub struct OrderBookEngine {
    markets: RwLock<HashMap<String, Market>>,
    order_books: RwLock<HashMap<String, OrderBook>>,
    orders: RwLock<HashMap<String, Order>>,
    positions: RwLock<HashMap<String, Position>>,
    trades: RwLock<Vec<Trade>>,

    // Position tracking: user -> market -> position_id
    user_positions: RwLock<HashMap<String, HashMap<String, String>>>,
}

impl OrderBookEngine {
    pub fn new() -> Self {
        Self {
            markets: RwLock::new(HashMap::new()),
            order_books: RwLock::new(HashMap::new()),
            orders: RwLock::new(HashMap::new()),
            positions: RwLock::new(HashMap::new()),
            trades: RwLock::new(Vec::new()),
            user_positions: RwLock::new(HashMap::new()),
        }
    }

    /// Create a new market
    pub fn create_market(&self, name: &str, base: &str, quote: &str, initial_price: u64) -> Result<Market, String> {
        let mut markets = self.markets.write();

        if markets.contains_key(name) {
            return Err(format!("Market {} already exists", name));
        }

        let market = Market::new(name, base, quote);
        let mut m = market.clone();
        m.mark_price = initial_price;
        m.index_price = initial_price;
        m.bid_price = initial_price - (initial_price / 100);
        m.ask_price = initial_price + (initial_price / 100);
        m.last_price = initial_price;

        markets.insert(name.to_string(), m);

        // Create order book
        let mut order_books = self.order_books.write();
        order_books.insert(name.to_string(), OrderBook::new(name));

        Ok(market)
    }

    /// Get market info
    pub fn get_market(&self, name: &str) -> Option<Market> {
        self.markets.read().get(name).cloned()
    }

    /// Place a new order
    pub fn place_order(&self, order: Order) -> Result<Vec<Fill>, String> {
        // Validate market exists
        if !self.markets.read().contains_key(&order.market) {
            return Err("Market not found".to_string());
        }

        // Store order
        let mut orders = self.orders.write();
        let mut o = order.clone();
        o.status = OrderStatus::Open;
        o.created_at = current_timestamp();
        o.updated_at = current_timestamp();

        let order_id = o.id.clone();
        orders.insert(order_id.clone(), o);

        // Match order
        drop(orders);
        let fills = self.match_order(&order_id)?;

        Ok(fills)
    }

    /// Cancel an order
    pub fn cancel_order(&self, order_id: &str, user: &str) -> Result<(), String> {
        let mut orders = self.orders.write();

        let order = orders.get_mut(order_id)
            .ok_or("Order not found")?;

        if order.user != user {
            return Err("Not authorized".to_string());
        }

        if order.status != OrderStatus::Open && order.status != OrderStatus::Partial {
            return Err("Cannot cancel order".to_string());
        }

        order.status = OrderStatus::Cancelled;
        order.updated_at = current_timestamp();

        Ok(())
    }

    /// Match an order against the book
    fn match_order(&self, order_id: &str) -> Result<Vec<Fill>, String> {
        let mut fills = Vec::new();
        let mut trades = self.trades.write();
        let mut orders = self.orders.write();
        let mut order_books = self.order_books.write();

        let order = orders.get_mut(order_id)
            .ok_or("Order not found")?
            .clone();

        if order.status != OrderStatus::Open {
            return Err("Order not open".to_string());
        }

        let book = order_books.get_mut(&order.market)
            .ok_or("Order book not found")?;

        let opposite_side = order.side.opposite();
        let book_side = match opposite_side {
            OrderSide::Bid => &mut book.asks,
            OrderSide::Ask => &mut book.bids,
        };

        let mut remaining = order.size - order.filled;

        // Walk through price levels
        let prices: Vec<u64> = match opposite_side {
            OrderSide::Bid => book.asks.keys().cloned().collect(),
            OrderSide::Ask => book.bids.keys().cloned().rev().collect(),
        };

        for price in prices {
            // Check price condition for limit orders
            if order.order_type == OrderType::Limit {
                match order.side {
                    OrderSide::Bid => {
                        if price > order.price { break; }
                    }
                    OrderSide::Ask => {
                        if price < order.price { break; }
                    }
                }
            }

            let price_level = book_side.get_mut(&price);
            let Some(level) = price_level else { continue; };

            let mut i = 0;
            while i < level.len() && remaining > 0 {
                let maker_order = &level[i];

                if maker_order.user == order.user {
                    i += 1;
                    continue;
                }

                // Check if maker is on opposite side
                if maker_order.side != opposite_side {
                    i += 1;
                    continue;
                }

                // Determine fill size
                let fill_size = remaining.min(maker_order.size - maker_order.filled);

                if fill_size == 0 {
                    i += 1;
                    continue;
                }

                // Create trade
                let trade_id = generate_id("trade");
                let trade = Trade {
                    id: trade_id.clone(),
                    market: order.market.clone(),
                    price,
                    size: fill_size,
                    side: order.side,
                    maker_order_id: maker_order.id.clone(),
                    taker_order_id: order.id.clone(),
                    maker: maker_order.user.clone(),
                    taker: order.user.clone(),
                    timestamp: current_timestamp(),
                };

                trades.push(trade.clone());

                // Update orders
                maker_order.filled += fill_size;
                if maker_order.filled >= maker_order.size {
                    maker_order.status = OrderStatus::Filled;
                    level.remove(i);
                } else {
                    maker_order.status = OrderStatus::Partial;
                    i += 1;
                }

                let order_ref = orders.get_mut(order_id).unwrap();
                order_ref.filled += fill_size;

                remaining -= fill_size;

                // Create fill record
                fills.push(Fill {
                    order_id: order.id.clone(),
                    trade_id,
                    price,
                    size: fill_size,
                    fee: calculate_fee(fill_size, price),
                    side: order.side,
                    timestamp: current_timestamp(),
                });
            }

            // Remove empty price levels
            if level.is_empty() {
                book_side.remove(&price);
            }
        }

        // Update order status
        {
            let order_ref = orders.get_mut(order_id).unwrap();
            order_ref.updated_at = current_timestamp();
            if remaining == 0 {
                order_ref.status = OrderStatus::Filled;
            } else if order_ref.filled > 0 {
                order_ref.status = OrderStatus::Partial;
            }
        }

        // Update last update ID
        book.last_update_id += 1;

        Ok(fills)
    }

    /// Get order book depth
    pub fn get_order_book(&self, market: &str, depth: usize) -> OrderBookSnapshot {
        let order_books = self.order_books.read();
        let book = match order_books.get(market) {
            Some(b) => b,
            None => return OrderBookSnapshot::default(),
        };

        let bids: Vec<(u64, u64)> = book.bids.iter()
            .rev()
            .take(depth)
            .map(|(p, o)| (*p, o.iter().map(|x| x.size - x.filled).sum()))
            .collect();

        let asks: Vec<(u64, u64)> = book.asks.iter()
            .take(depth)
            .map(|(p, o)| (*p, o.iter().map(|x| x.size - x.filled).sum()))
            .collect();

        let spread = if let (Some(&best_bid), Some(&best_ask)) = (bids.first(), asks.first()) {
            best_ask - best_bid
        } else {
            0
        };

        OrderBookSnapshot {
            market: market.to_string(),
            bids,
            asks,
            spread,
            spread_bps: if spread > 0 { (spread as f64 / bids.first().map(|p| *p as f64).unwrap_or(1.0) * 10000.0) as u64 } else { 0 },
            last_update_id: book.last_update_id,
        }
    }

    /// Get user positions
    pub fn get_positions(&self, user: &str) -> Vec<Position> {
        let positions = self.positions.read();
        let user_pos = self.user_positions.read();

        user_pos.get(user)
            .map(|mp| {
                mp.values()
                    .filter_map(|id| positions.get(id).cloned())
                    .collect()
            })
            .unwrap_or_default()
    }
}

#[derive(Debug, Clone, Default)]
pub struct OrderBookSnapshot {
    pub market: String,
    pub bids: Vec<(u64, u64)>,
    pub asks: Vec<(u64, u64)>,
    pub spread: u64,
    pub spread_bps: u64,
    pub last_update_id: u64,
}

// ============================================================================
// Helper Functions
// ============================================================================

fn current_timestamp() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

fn generate_id(prefix: &str) -> String {
    use std::time::SystemTime;
    let nanos = SystemTime::now()
        .duration_since(SystemTime::UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    format!("{}_{}", prefix, nanos)
}

fn calculate_fee(size: u64, price: u64) -> u64 {
    // 0.05% taker fee
    (size * price) / 2000
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    fn create_engine() -> OrderBookEngine {
        OrderBookEngine::new()
    }

    #[test]
    fn test_create_market() {
        let engine = create_engine();
        let result = engine.create_market("ETH-PERP", "ETH", "USD", 2450_000000); // $2450
        assert!(result.is_ok());
        let market = result.unwrap();
        assert_eq!(market.name, "ETH-PERP");
    }

    #[test]
    fn test_place_order() {
        let engine = create_engine();
        engine.create_market("ETH-PERP", "ETH", "USD", 2450_000000).unwrap();

        let order = Order {
            id: "order_1".to_string(),
            user: "user_1".to_string(),
            market: "ETH-PERP".to_string(),
            side: OrderSide::Bid,
            order_type: OrderType::Limit,
            price: 2400_000000,
            size: 1000,
            filled: 0,
            trigger_price: None,
            post_only: false,
            reduce_only: false,
            time_in_force: TimeInForce::GTC,
            status: OrderStatus::Open,
            created_at: 0,
            updated_at: 0,
        };

        let result = engine.place_order(order);
        assert!(result.is_ok());
    }

    #[test]
    fn test_order_book_snapshot() {
        let engine = create_engine();
        engine.create_market("ETH-PERP", "ETH", "USD", 2450_000000).unwrap();

        // Place some orders
        let bid = Order {
            id: "bid_1".to_string(),
            user: "user_1".to_string(),
            market: "ETH-PERP".to_string(),
            side: OrderSide::Bid,
            order_type: OrderType::Limit,
            price: 2400_000000,
            size: 1000,
            filled: 0,
            trigger_price: None,
            post_only: false,
            reduce_only: false,
            time_in_force: TimeInForce::GTC,
            status: OrderStatus::Open,
            created_at: 0,
            updated_at: 0,
        };

        engine.place_order(bid).unwrap();

        let snapshot = engine.get_order_book("ETH-PERP", 10);
        assert_eq!(snapshot.bids.len(), 1);
        assert_eq!(snapshot.bids[0].0, 2400_000000);
    }
}
