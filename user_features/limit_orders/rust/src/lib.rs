//! TigerSwap Limit Orders Service - Rust Implementation
//! 
//! High-performance limit orders with order book matching

use serde::{Deserialize, Serialize};
use std::collections::{HashMap, VecDeque};
use std::sync::Arc;
use parking_lot::RwLock;

/// Order side
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderSide {
    Buy,
    Sell,
}

/// Order type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderType {
    Limit,
    Stop,
    StopLimit,
}

/// Order status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderStatus {
    Pending,
    Partial,
    Filled,
    Cancelled,
    Expired,
}

/// Order structure
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Order {
    pub id: String,
    pub user_address: String,
    pub pair: String,
    pub side: OrderSide,
    pub order_type: OrderType,
    pub price: f64,
    pub trigger_price: Option<f64>,
    pub quantity: f64,
    pub filled_quantity: f64,
    pub avg_fill_price: f64,
    pub slippage_bps: u32,
    pub status: OrderStatus,
    pub chain_id: u64,
    pub tx_hash: Option<String>,
    pub created_at: u64,
    pub updated_at: u64,
    pub expires_at: u64,
    pub is_active: bool,
}

impl Order {
    pub fn new(
        user_address: String,
        pair: String,
        side: OrderSide,
        order_type: OrderType,
        price: f64,
        quantity: f64,
    ) -> Self {
        let now = current_timestamp();
        Self {
            id: generate_id("order"),
            user_address,
            pair,
            side,
            order_type,
            price,
            trigger_price: None,
            quantity,
            filled_quantity: 0.0,
            avg_fill_price: 0.0,
            slippage_bps: 50,
            status: OrderStatus::Pending,
            chain_id: 1,
            tx_hash: None,
            created_at: now,
            updated_at: now,
            expires_at: now + 30 * 24 * 60 * 60 * 1000, // 30 days
            is_active: true,
        }
    }
}

/// Order book entry
#[derive(Debug, Clone)]
pub struct OrderBookEntry {
    pub price: f64,
    pub quantity: f64,
    pub orders: usize,
}

/// Order book
#[derive(Debug, Clone)]
pub struct OrderBook {
    pub pair: String,
    pub bids: Vec<OrderBookEntry>,
    pub asks: Vec<OrderBookEntry>,
    pub spread: f64,
    pub spread_percent: f64,
    pub last_update_time: u64,
}

/// Trade result
#[derive(Debug, Clone)]
pub struct OrderBookTrade {
    pub id: String,
    pub price: f64,
    pub quantity: f64,
    pub side: OrderSide,
    pub timestamp: u64,
    pub tx_hash: String,
}

/// Limit Orders Service
pub struct LimitOrdersService {
    orders: RwLock<HashMap<String, Order>>,
    order_books: RwLock<HashMap<String, OrderBook>>,
    config: ServiceConfig,
}

#[derive(Debug, Clone)]
pub struct ServiceConfig {
    pub max_orders_per_block: usize,
    pub order_expiration_ms: u64,
    pub min_order_size: f64,
    pub max_order_size: f64,
    pub price_precision_decimals: usize,
    pub enable_stop_orders: bool,
    pub enable_stop_limit_orders: bool,
}

impl Default for ServiceConfig {
    fn default() -> Self {
        Self {
            max_orders_per_block: 100,
            order_expiration_ms: 30 * 24 * 60 * 60 * 1000,
            min_order_size: 0.001,
            max_order_size: 1_000_000.0,
            price_precision_decimals: 8,
            enable_stop_orders: true,
            enable_stop_limit_orders: true,
        }
    }
}

impl LimitOrdersService {
    pub fn new() -> Self {
        Self {
            orders: RwLock::new(HashMap::new()),
            order_books: RwLock::new(HashMap::new()),
            config: ServiceConfig::default(),
        }
    }

    /// Create a new order
    pub fn create_order(&self, params: CreateOrderParams) -> Result<Order, String> {
        if params.quantity < self.config.min_order_size {
            return Err(format!("Order too small: minimum {}", self.config.min_order_size));
        }
        if params.quantity > self.config.max_order_size {
            return Err(format!("Order too large: maximum {}", self.config.max_order_size));
        }

        let mut order = Order::new(
            params.user_address,
            params.pair,
            params.side,
            params.order_type,
            params.price,
            params.quantity,
        );

        if let Some(trigger) = params.trigger_price {
            order.trigger_price = Some(trigger);
        }
        if let Some(slippage) = params.slippage_bps {
            order.slippage_bps = slippage;
        }

        // Store order
        let id = order.id.clone();
        self.orders.write().insert(id.clone(), order.clone());

        // Update order book
        self.update_order_book(&order.pair);

        Ok(order)
    }

    /// Cancel an order
    pub fn cancel_order(&self, order_id: &str, user_address: &str) -> Result<bool, String> {
        let mut orders = self.orders.write();
        
        let order = orders.get_mut(order_id)
            .ok_or("Order not found")?;

        if order.user_address.to_lowercase() != user_address.to_lowercase() {
            return Err("Not authorized to cancel this order".to_string());
        }

        if order.status != OrderStatus::Pending && order.status != OrderStatus::Partial {
            return Err("Order cannot be cancelled".to_string());
        }

        order.status = OrderStatus::Cancelled;
        order.is_active = false;
        order.updated_at = current_timestamp();

        // Update order book
        drop(orders);
        self.update_order_book(&orders.get(order_id).map(|o| o.pair.clone()).unwrap_or_default());

        Ok(true)
    }

    /// Get order book for a pair
    pub fn get_order_book(&self, pair: &str) -> OrderBook {
        self.order_books.read()
            .get(pair)
            .cloned()
            .unwrap_or_else(|| OrderBook {
                pair: pair.to_string(),
                bids: Vec::new(),
                asks: Vec::new(),
                spread: 0.0,
                spread_percent: 0.0,
                last_update_time: current_timestamp(),
            })
    }

    /// Update order book
    fn update_order_book(&self, pair: &str) {
        let orders = self.orders.read();
        
        let mut bid_prices: HashMap<i64, (f64, usize)> = HashMap::new();
        let mut ask_prices: HashMap<i64, (f64, usize)> = HashMap::new();
        let precision = 10_i64.pow(self.config.price_precision_decimals as u32);

        for order in orders.values() {
            if order.pair != pair || !order.is_active || order.status != OrderStatus::Pending {
                continue;
            }

            let price_key = (order.price * precision as f64) as i64;
            let remaining = order.quantity - order.filled_quantity;

            match order.side {
                OrderSide::Buy => {
                    let entry = bid_prices.entry(price_key).or_insert((0.0, 0));
                    entry.0 += remaining;
                    entry.1 += 1;
                }
                OrderSide::Sell => {
                    let entry = ask_prices.entry(price_key).or_insert((0.0, 0));
                    entry.0 += remaining;
                    entry.1 += 1;
                }
            }
        }

        let mut bids: Vec<_> = bid_prices.into_iter()
            .map(|(k, (q, c))| OrderBookEntry {
                price: k as f64 / precision as f64,
                quantity: q,
                orders: c,
            })
            .collect();
        
        let mut asks: Vec<_> = ask_prices.into_iter()
            .map(|(k, (q, c))| OrderBookEntry {
                price: k as f64 / precision as f64,
                quantity: q,
                orders: c,
            })
            .collect();

        // Sort bids descending, asks ascending
        bids.sort_by(|a, b| b.price.partial_cmp(&a.price).unwrap_or(std::cmp::Ordering::Equal));
        asks.sort_by(|a, b| a.price.partial_cmp(&b.price).unwrap_or(std::cmp::Ordering::Equal));

        let spread = if let (Some(&best_bid), Some(&best_ask)) = (bids.first(), asks.first()) {
            best_ask.price - best_bid.price
        } else {
            0.0
        };

        let mid_price = (bids.first().map(|b| b.price).unwrap_or(0.0) 
            + asks.first().map(|a| a.price).unwrap_or(0.0)) / 2.0;
        
        let spread_percent = if mid_price > 0.0 {
            (spread / mid_price) * 100.0
        } else {
            0.0
        };

        let book = OrderBook {
            pair: pair.to_string(),
            bids: bids.into_iter().take(50).collect(),
            asks: asks.into_iter().take(50).collect(),
            spread,
            spread_percent,
            last_update_time: current_timestamp(),
        };

        self.order_books.write().insert(pair.to_string(), book);
    }

    /// Match orders (simple price-time priority)
    pub fn match_orders(&self, pair: &str) -> Vec<OrderBookTrade> {
        let mut trades = Vec::new();
        let book = self.get_order_book(pair);

        if book.bids.is_empty() || book.asks.is_empty() {
            return trades;
        }

        let best_bid = book.bids[0].price;
        let best_ask = book.asks[0].price;

        // Check if there's a match
        if best_bid >= best_ask {
            let match_price = (best_bid + best_ask) / 2.0;
            let match_quantity = book.bids[0].quantity.min(book.asks[0].quantity);

            if match_quantity >= self.config.min_order_size {
                trades.push(OrderBookTrade {
                    id: generate_id("trade"),
                    price: match_price,
                    quantity: match_quantity,
                    side: OrderSide::Buy,
                    timestamp: current_timestamp(),
                    tx_hash: generate_tx_hash(),
                });
            }
        }

        trades
    }

    /// Get user orders
    pub fn get_user_orders(&self, user_address: &str) -> Vec<Order> {
        let orders = self.orders.read();
        let mut result: Vec<_> = orders.values()
            .filter(|o| o.user_address.to_lowercase() == user_address.to_lowercase())
            .cloned()
            .collect();

        result.sort_by(|a, b| b.created_at.cmp(&a.created_at));
        result
    }

    /// Cleanup expired orders
    pub fn cleanup_expired_orders(&self) -> usize {
        let mut orders = self.orders.write();
        let now = current_timestamp();
        let mut cleaned = 0;

        for order in orders.values_mut() {
            if order.is_active && order.status == OrderStatus::Pending && order.expires_at < now {
                order.status = OrderStatus::Expired;
                order.is_active = false;
                order.updated_at = now;
                cleaned += 1;
            }
        }

        cleaned
    }
}

impl Default for LimitOrdersService {
    fn default() -> Self {
        Self::new()
    }
}

/// Parameters for creating an order
#[derive(Debug, Clone)]
pub struct CreateOrderParams {
    pub user_address: String,
    pub pair: String,
    pub side: OrderSide,
    pub order_type: OrderType,
    pub price: f64,
    pub quantity: f64,
    pub trigger_price: Option<f64>,
    pub slippage_bps: Option<u32>,
}

fn current_timestamp() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_millis() as u64
}

fn generate_id(prefix: &str) -> String {
    format!("{}_{}", prefix, current_timestamp())
}

fn generate_tx_hash() -> String {
    use std::time::SystemTime;
    let nanos = SystemTime::now()
        .duration_since(SystemTime::UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    format!("0x{:064x}", nanos)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_create_order() {
        let service = LimitOrdersService::new();
        
        let params = CreateOrderParams {
            user_address: "0x123".to_string(),
            pair: "ETH/USDT".to_string(),
            side: OrderSide::Buy,
            order_type: OrderType::Limit,
            price: 2500.0,
            quantity: 1.0,
            trigger_price: None,
            slippage_bps: Some(50),
        };
        
        let order = service.create_order(params);
        assert!(order.is_ok());
    }

    #[test]
    fn test_order_book() {
        let service = LimitOrdersService::new();
        
        // Create some orders
        let buy = CreateOrderParams {
            user_address: "0x123".to_string(),
            pair: "ETH/USDT".to_string(),
            side: OrderSide::Buy,
            order_type: OrderType::Limit,
            price: 2400.0,
            quantity: 10.0,
            trigger_price: None,
            slippage_bps: None,
        };
        service.create_order(buy).unwrap();
        
        let sell = CreateOrderParams {
            user_address: "0x456".to_string(),
            pair: "ETH/USDT".to_string(),
            side: OrderSide::Sell,
            order_type: OrderType::Limit,
            price: 2500.0,
            quantity: 10.0,
            trigger_price: None,
            slippage_bps: None,
        };
        service.create_order(sell).unwrap();
        
        let book = service.get_order_book("ETH/USDT");
        assert_eq!(book.bids.len(), 1);
        assert_eq!(book.asks.len(), 1);
    }
}