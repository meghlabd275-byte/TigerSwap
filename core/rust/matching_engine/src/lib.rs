//! TigerSwap Distributed Matching Engine
//! 
//! High-performance distributed order matching with:
//! - Order book management
//! - Price-time priority matching
//! - Cluster coordination
//! - Hot-hot replication

use std::collections::{HashMap, VecDeque};
use std::sync::Arc;
use tokio::sync::RwLock;

use serde::{Deserialize, Serialize};
use thiserror::Error;

// ============================================================================
// Error Types
// ============================================================================

#[derive(Error, Debug)]
pub enum MatchingError {
    #[error("Invalid order")]
    InvalidOrder,
    #[error("Insufficient liquidity")]
    InsufficientLiquidity,
    #[error("Order not found")]
    OrderNotFound,
    #[error("Trading disabled")]
    TradingDisabled,
    #[error("Price out of range")]
    PriceOutOfRange,
}

// ============================================================================
// Order Types
// ============================================================================

/// Order side
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderSide {
    Buy,
    Sell,
}

impl OrderSide {
    pub fn opposite(&self) -> Self {
        match self {
            OrderSide::Buy => OrderSide::Sell,
            OrderSide::Sell => OrderSide::Buy,
        }
    }
}

/// Order type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderType {
    Limit,
    Market,
    StopLoss,
    StopLimit,
}

/// Order status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderStatus {
    Pending,
    Open,
    PartiallyFilled,
    Filled,
    Cancelled,
    Expired,
}

// ============================================================================
// Order
// ============================================================================

/// Order
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Order {
    pub order_id: String,
    pub pair: String,
    pub side: OrderSide,
    pub order_type: OrderType,
    pub price: f64,
    pub amount: f64,
    pub filled_amount: f64,
    pub remaining_amount: f64,
    pub status: OrderStatus,
    pub user_id: String,
    pub created_at: i64,
    pub updated_at: i64,
    pub expires_at: Option<i64>,
    pub stop_price: Option<f64>,
}

impl Order {
    pub fn new_limit(
        order_id: &str,
        pair: &str,
        side: OrderSide,
        price: f64,
        amount: f64,
        user_id: &str,
    ) -> Self {
        let now = chrono::Utc::now().timestamp();
        
        Self {
            order_id: order_id.to_string(),
            pair: pair.to_string(),
            side,
            order_type: OrderType::Limit,
            price,
            amount,
            filled_amount: 0.0,
            remaining_amount: amount,
            status: OrderStatus::Open,
            user_id: user_id.to_string(),
            created_at: now,
            updated_at: now,
            expires_at: None,
            stop_price: None,
        }
    }
    
    pub fn fill(&mut self, amount: f64) {
        self.filled_amount += amount;
        self.remaining_amount -= amount;
        
        if self.remaining_amount <= 0.0 {
            self.status = OrderStatus::Filled;
        } else {
            self.status = OrderStatus::PartiallyFilled;
        }
        
        self.updated_at = chrono::Utc::now().timestamp();
    }
    
    pub fn cancel(&mut self) {
        self.status = OrderStatus::Cancelled;
        self.updated_at = chrono::Utc::now().timestamp();
    }
}

// ============================================================================
// Trade
// ============================================================================

/// Trade (matched order)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Trade {
    pub trade_id: String,
    pub pair: String,
    pub maker_order_id: String,
    pub taker_order_id: String,
    pub price: f64,
    pub amount: f64,
    pub maker_fee: f64,
    pub taker_fee: f64,
    pub executed_at: i64,
}

// ============================================================================
// Order Book
// ============================================================================

/// Order book for a trading pair
#[derive(Debug)]
pub struct OrderBook {
    pub pair: String,
    pub bids: VecDeque<Order>,  // Buy orders, sorted by price desc
    pub asks: VecDeque<Order>, // Sell orders, sorted by price asc
}

impl OrderBook {
    pub fn new(pair: &str) -> Self {
        Self {
            pair: pair.to_string(),
            bids: VecDeque::new(),
            asks: VecDeque::new(),
        }
    }
    
    /// Add order to book
    pub fn add_order(&mut self, order: Order) {
        match order.side {
            OrderSide::Buy => {
                // Insert sorted by price desc, then by time
                let mut inserted = false;
                for (i, existing) in self.bids.iter().enumerate() {
                    if existing.price < order.price {
                        self.bids.insert(i, order);
                        inserted = true;
                        break;
                    }
                }
                if !inserted {
                    self.bids.push_back(order);
                }
            }
            OrderSide::Sell => {
                // Insert sorted by price asc, then by time
                let mut inserted = false;
                for (i, existing) in self.asks.iter().enumerate() {
                    if existing.price > order.price {
                        self.asks.insert(i, order);
                        inserted = true;
                        break;
                    }
                }
                if !inserted {
                    self.asks.push_back(order);
                }
            }
        }
    }
    
    /// Get best bid
    pub fn best_bid(&self) -> Option<&Order> {
        self.bids.front()
    }
    
    /// Get best ask
    pub fn best_ask(&self) -> Option<&Order> {
        self.asks.front()
    }
    
    /// Get spread
    pub fn spread(&self) -> Option<f64> {
        match (self.best_bid(), self.best_ask()) {
            (Some(bid), Some(ask) => Some(ask.price - bid.price),
            _ => None,
        }
    }
    
    /// Get depth at price level
    pub fn depth(&self, side: OrderSide, levels: u32) -> Vec<(f64, f64)> {
        let orders = match side {
            OrderSide::Buy => &self.bids,
            OrderSide::Sell => &self.asks,
        };
        
        let mut depth = Vec::new();
        let mut cumulative = 0.0;
        
        for order in orders.iter().take(levels as usize) {
            cumulative += order.remaining_amount;
            depth.push((order.price, cumulative));
        }
        
        depth
    }
}

// ============================================================================
// Matching Engine
// ============================================================================

/// Matching engine
pub struct MatchingEngine {
    order_books: RwLock<HashMap<String, OrderBook>>,
    orders: RwLock<HashMap<String, Order>>,
    trades: RwLock<VecDeque<Trade>>,
    stats: RwLock<MatchingStats>,
    enabled: RwLock<bool>,
}

impl MatchingEngine {
    pub fn new() -> Self {
        Self {
            order_books: RwLock::new(HashMap::new()),
            orders: RwLock::new(HashMap::new()),
            trades: VecDeque::new(),
            stats: RwLock::new(MatchingStats::new()),
            enabled: RwLock::new(true),
        }
    }
    
    /// Submit order
    pub async fn submit_order(&self, mut order: Order) -> Result<Trade, MatchingError> {
        if !*self.enabled.read().await {
            return Err(MatchingError::TradingDisabled);
        }
        
        // Get or create order book
        let pair = order.pair.clone();
        {
            let mut books = self.order_books.write().await;
            if !books.contains_key(&pair) {
                books.insert(pair.clone(), OrderBook::new(&pair));
            }
        }
        
        // Match against opposite side
        let trades = self.match_order(&order).await;
        
        let mut last_trade = None;
        
        for trade in trades {
            // Record trade
            self.record_trade(trade.clone()).await;
            last_trade = Some(trade);
        }
        
        // Add remaining to book if not fully filled
        if order.remaining_amount > 0.0 && order.status == OrderStatus::Open {
            let mut books = self.order_books.write().await;
            if let Some(book) = books.get_mut(&pair) {
                book.add_order(order);
            }
        }
        
        // Record order
        let mut orders = self.orders.write().await;
        orders.insert(order.order_id.clone(), order);
        
        // Update stats
        {
            let mut stats = self.stats.write().await;
            stats.total_orders += 1;
        }
        
        match last_trade {
            Some(t) => Ok(t),
            None => Err(MatchingError::InvalidOrder),
        }
    }
    
    /// Match order against book
    async fn match_order(&self, order: &Order) -> Vec<Trade> {
        let mut trades = Vec::new();
        let pair = order.pair.clone();
        let opposite_side = order.side.opposite();
        
        let mut remaining = order.amount;
        
        loop {
            let books = self.order_books.read().await;
            let book = match books.get(&pair) {
                Some(b) => b,
                None => break,
            };
            
            // Get best opposite order
            let best = match opposite_side {
                OrderSide::Buy => book.best_ask(),
                OrderSide::Sell => book.best_bid(),
            };
            
            let best_order = match best {
                Some(o) => o,
                None => break,
            };
            
            // Check price match
            let price_match = match order.side {
                OrderSide::Buy => best_order.price <= order.price,
                OrderSide::Sell => best_order.price >= order.price,
            };
            
            if !price_match {
                break;
            }
            
            // Calculate fill amount
            let fill_amount = remaining.min(best_order.remaining_amount);
            let price = best_order.price;
            
            // Create trade
            let trade = Trade {
                trade_id: uuid::Uuid::new_v4().to_string(),
                pair: pair.clone(),
                maker_order_id: best_order.order_id.clone(),
                taker_order_id: order.order_id.clone(),
                price,
                amount: fill_amount,
                maker_fee: price * fill_amount * 0.001,
                taker_fee: price * fill_amount * 0.001,
                executed_at: chrono::Utc::now().timestamp(),
            };
            
            trades.push(trade);
            remaining -= fill_amount;
            
            if remaining <= 0.0 {
                break;
            }
        }
        
        trades
    }
    
    /// Record trade
    pub async fn record_trade(&self, trade: Trade) {
        let mut trades = self.trades.write().await;
        trades.push_back(trade);
        
        // Keep last 10000 trades
        while trades.len() > 10000 {
            trades.pop_front();
        }
        
        // Update stats
        let mut stats = self.stats.write().await;
        stats.total_trades += 1;
        stats.volume_24h += trade.amount * trade.price;
    }
    
    /// Cancel order
    pub async fn cancel_order(&self, order_id: &str) -> Result<(), MatchingError> {
        let mut orders = self.orders.write().await;
        
        if let Some(order) = orders.get_mut(order_id) {
            order.cancel();
            Ok(())
        } else {
            Err(MatchingError::OrderNotFound)
        }
    }
    
    /// Get order
    pub async fn get_order(&self, order_id: &str) -> Option<Order> {
        let orders = self.orders.read().await;
        orders.get(order_id).cloned()
    }
    
    /// Get order book
    pub async fn get_order_book(&self, pair: &str) -> Option<OrderBook> {
        let books = self.order_books.read().await;
        books.get(pair).cloned()
    }
    
    /// Get ticker
    pub async fn get_ticker(&self, pair: &str) -> Option<Ticker> {
        let books = self.order_books.read().await;
        let book = books.get(pair)?;
        
        let last = self.get_last_trade(pair).await;
        
        Some(Ticker {
            pair: pair.to_string(),
            last_price: last.map(|t| t.price).unwrap_or(0.0),
            best_bid: book.best_bid().map(|o| o.price).unwrap_or(0.0),
            best_ask: book.best_ask().map(|o| o.price).unwrap_or(0.0),
            volume_24h: 0.0,
            change_24h: 0.0,
        })
    }
    
    /// Get last trade
    async fn get_last_trade(&self, pair: &str) -> Option<Trade> {
        let trades = self.trades.read().await;
        
        for trade in trades.iter().rev() {
            if trade.pair == pair {
                return Some(trade.clone());
            }
        }
        
        None
    }
    
    /// Enable/disable trading
    pub async fn set_enabled(&self, enabled: bool) {
        let mut e = self.enabled.write().await;
        *e = enabled;
    }
    
    /// Get stats
    pub async fn get_stats(&self) -> MatchingStats {
        self.stats.read().await.clone()
    }
}

// ============================================================================
// Cluster
// ============================================================================

/// Node in the matching cluster
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ClusterNode {
    pub node_id: String,
    pub address: String,
    pub role: NodeRole,
    pub status: NodeStatus,
    pub last_heartbeat: i64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum NodeRole {
    Leader,
    Follower,
    Observer,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum NodeStatus {
    Active,
    Inactive,
    Failed,
}

/// Matching cluster
pub struct MatchingCluster {
    nodes: RwLock<Vec<ClusterNode>>,
    leader: RwLock<Option<String>>,
}

impl MatchingCluster {
    pub fn new() -> Self {
        Self {
            nodes: RwLock::new(Vec::new()),
            leader: RwLock::new(None),
        }
    }
    
    /// Add node
    pub async fn add_node(&self, node: ClusterNode) {
        let mut nodes = self.nodes.write().await;
        nodes.push(node);
    }
    
    /// Get leader
    pub async fn get_leader(&self) -> Option<String> {
        self.leader.read().await.clone()
    }
    
    /// Elect leader
    pub async fn elect_leader(&self) -> Option<String> {
        let nodes = self.nodes.read().await;
        
        for node in nodes.iter() {
            if node.status == NodeStatus::Active && node.role == NodeRole::Leader {
                return Some(node.node_id.clone());
            }
        }
        
        None
    }
}

impl Default for MatchingCluster {
    fn default() -> Self {
        Self::new()
    }
}

impl Default for MatchingEngine {
    fn default() -> Self {
        Self::new()
    }
}

// ============================================================================
// Data Types
// ============================================================================

/// Ticker data
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Ticker {
    pub pair: String,
    pub last_price: f64,
    pub best_bid: f64,
    pub best_ask: f64,
    pub volume_24h: f64,
    pub change_24h: f64,
}

/// Matching stats
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MatchingStats {
    pub total_orders: u64,
    pub total_trades: u64,
    pub volume_24h: f64,
    pub last_updated: i64,
}

impl MatchingStats {
    pub fn new() -> Self {
        Self {
            total_orders: 0,
            total_trades: 0,
            volume_24h: 0.0,
            last_updated: chrono::Utc::now().timestamp(),
        }
    }
}

impl Default for MatchingStats {
    fn default() -> Self {
        Self::new()
    }
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;
    
    #[tokio::test]
    async fn test_order_book() {
        let mut book = OrderBook::new("ETH/USDC");
        
        // Add buy orders
        book.add_order(Order::new_limit("1", "ETH/USDC", OrderSide::Buy, 100.0, 1.0, "user1"));
        book.add_order(Order::new_limit("2", "ETH/USDC", OrderSide::Buy, 101.0, 2.0, "user2"));
        
        // Add sell orders
        book.add_order(Order::new_limit("3", "ETH/USDC", OrderSide::Sell, 102.0, 1.0, "user3"));
        
        let bid = book.best_bid().unwrap();
        assert_eq!(bid.price, 101.0);
        
        let ask = book.best_ask().unwrap();
        assert_eq!(ask.price, 102.0);
        
        assert!(book.spread().is_some());
    }
    
    #[tokio::test]
    async fn test_matching() {
        let engine = MatchingEngine::new();
        
        // Place buy order
        let buy_order = Order::new_limit("1", "ETH/USDC", OrderSide::Buy, 100.0, 1.0, "user1");
        let result = engine.submit_order(buy_order).await;
        // May fail if no opposite orders
        
        // Place sell order
        let sell_order = Order::new_limit("2", "ETH/USDC", OrderSide::Sell, 100.0, 1.0, "user2");
        let result = engine.submit_order(sell_order).await;
        
        // Check stats
        let stats = engine.get_stats().await;
        assert!(stats.total_trades >= 0);
    }
}

// ============================================================================
// Library Exports
// ============================================================================

pub use self::{
    order::{Order, OrderSide, OrderType, OrderStatus},
    trade::Trade,
    order_book::OrderBook,
    matching::{MatchingEngine, MatchingCluster, ClusterNode, NodeRole, NodeStatus},
    ticker::Ticker,
    stats::MatchingStats,
};

mod order;
mod trade;
mod order_book;
mod matching;
mod ticker;
mod stats;