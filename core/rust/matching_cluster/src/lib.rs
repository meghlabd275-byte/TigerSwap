//! Distributed Matching Cluster
//! 
//! Provides distributed order matching with Raft consensus for high availability
//! and multi-region replication.

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::Arc;
use parking_lot::RwLock;
use thiserror::Error;
use uuid::Uuid;
use chrono::{DateTime, Utc};

/// Matching cluster errors
#[derive(Debug, Error)]
pub enum MatchingError {
    #[error("Not leader")]
    NotLeader,
    #[error("Leader unknown")]
    LeaderUnknown,
    #[error("Consensus error: {0}")]
    ConsensusError(String),
    #[error("Invalid order: {0}")]
    InvalidOrder(String),
    #[error("Order not found: {0}")]
    OrderNotFound(String),
    #[error("Insufficient liquidity")]
    InsufficientLiquidity,
    #[error("Node not available")]
    NodeNotAvailable,
}

/// Order side
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum OrderSide {
    Buy,
    Sell,
}

/// Order type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum OrderType {
    Limit,
    Market,
    StopLoss,
    StopLimit,
}

/// Order status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum OrderStatus {
    Pending,
    Open,
    PartiallyFilled,
    Filled,
    Cancelled,
    Rejected,
    Expired,
}

/// Order
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Order {
    /// Order ID
    pub order_id: String,
    /// Owner address
    pub owner: String,
    /// Market (e.g., "ETH-USDC")
    pub market: String,
    /// Order side
    pub side: OrderSide,
    /// Order type
    pub order_type: OrderType,
    /// Price (for limit orders)
    pub price: u128,
    /// Amount
    pub amount: u128,
    /// Filled amount
    pub filled: u128,
    /// Remaining amount
    pub remaining: u128,
    /// Status
    pub status: OrderStatus,
    /// Created timestamp
    pub created_at: i64,
    /// Updated timestamp
    pub updated_at: i64,
    /// Expires at
    pub expires_at: i64,
}

impl Order {
    pub fn new(
        owner: String,
        market: String,
        side: OrderSide,
        order_type: OrderType,
        price: u128,
        amount: u128,
    ) -> Self {
        let now = Utc::now().timestamp();
        Self {
            order_id: Uuid::new_v4().to_string(),
            owner,
            market,
            side,
            order_type,
            price,
            amount,
            filled: 0,
            remaining: amount,
            status: OrderStatus::Pending,
            created_at: now,
            updated_at: now,
            expires_at: now + 3600, // 1 hour
        }
    }

    /// Validate order
    pub fn validate(&self) -> Result<(), MatchingError> {
        if self.amount == 0 {
            return Err(MatchingError::InvalidOrder("Amount is zero".to_string()));
        }
        if self.price == 0 && self.order_type == OrderType::Limit {
            return Err(MatchingError::InvalidOrder("Limit order without price".to_string()));
        }
        if self.market.is_empty() {
            return Err(MatchingError::InvalidOrder("Empty market".to_string()));
        }
        Ok(())
    }

    /// Fill order partially
    pub fn fill(&mut self, amount: u128) -> bool {
        if self.remaining < amount {
            return false;
        }
        self.filled += amount;
        self.remaining -= amount;
        self.updated_at = Utc::now().timestamp();
        
        if self.remaining == 0 {
            self.status = OrderStatus::Filled;
        } else {
            self.status = OrderStatus::PartiallyFilled;
        }
        true
    }

    /// Cancel order
    pub fn cancel(&mut self) {
        self.status = OrderStatus::Cancelled;
        self.updated_at = Utc::now().timestamp();
    }
}

/// Trade (match result)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Trade {
    /// Trade ID
    pub trade_id: String,
    /// Order ID (maker)
    pub maker_order_id: String,
    /// Order ID (taker)
    pub taker_order_id: String,
    /// Market
    pub market: String,
    /// Price
    pub price: u128,
    /// Amount
    pub amount: u128,
    /// Maker address
    pub maker: String,
    /// Taker address
    pub taker: String,
    /// Timestamp
    pub timestamp: i64,
    /// Block number
    pub block_number: u64,
}

impl Trade {
    pub fn new(
        maker_order_id: String,
        taker_order_id: String,
        market: String,
        price: u128,
        amount: u128,
        maker: String,
        taker: String,
    ) -> Self {
        Self {
            trade_id: Uuid::new_v4().to_string(),
            maker_order_id,
            taker_order_id,
            market,
            price,
            amount,
            maker,
            taker,
            timestamp: Utc::now().timestamp(),
            block_number: 0,
        }
    }
}

/// Order book (price level)
#[derive(Debug, Clone)]
pub struct PriceLevel {
    pub price: u128,
    pub orders: Vec<Order>,
    pub total_amount: u128,
}

impl PriceLevel {
    pub fn new(price: u128) -> Self {
        Self {
            price,
            orders: Vec::new(),
            total_amount: 0,
        }
    }

    pub fn add_order(&mut self, order: Order) {
        self.total_amount += order.remaining;
        self.orders.push(order);
    }

    pub fn remove_order(&mut self, order_id: &str) -> Option<Order> {
        if let Some(pos) = self.orders.iter().position(|o| o.order_id == order_id) {
            let order = self.orders.remove(pos);
            self.total_amount = self.total_amount.saturating_sub(order.remaining);
            Some(order)
        } else {
            None
        }
    }
}

/// Order book
#[derive(Debug, Default)]
pub struct OrderBook {
    /// Market
    pub market: String,
    /// Bids (buy orders by price)
    pub bids: HashMap<u128, PriceLevel>,
    /// Asks (sell orders by price)
    pub asks: HashMap<u128, PriceLevel>,
}

impl OrderBook {
    pub fn new(market: String) -> Self {
        Self {
            market,
            bids: HashMap::new(),
            asks: HashMap::new(),
        }
    }

    /// Add order to book
    pub fn add_order(&mut self, order: Order) -> Result<(), MatchingError> {
        order.validate()?;
        
        let levels = match order.side {
            OrderSide::Buy => &mut self.bids,
            OrderSide::Sell => &mut self.asks,
        };
        
        let level = levels.entry(order.price).or_insert_with(|| PriceLevel::new(order.price));
        level.add_order(order);
        Ok(())
    }

    /// Cancel order
    pub fn cancel_order(&mut self, order_id: &str, side: OrderSide) -> Option<Order> {
        let levels = match side {
            OrderSide::Buy => &mut self.bids,
            OrderSide::Sell => &mut self.asks,
        };
        
        for level in levels.values_mut() {
            if let Some(order) = level.remove_order(order_id) {
                return Some(order);
            }
        }
        None
    }

    /// Get best bid
    pub fn best_bid(&self) -> Option<(u128, u128)> {
        self.bids.keys().max().map(|&price| {
            let level = &self.bids[&price];
            (price, level.total_amount)
        })
    }

    /// Get best ask
    pub fn best_ask(&self) -> Option<(u128, u128)> {
        self.asks.keys().min().map(|&price| {
            let level = &self.asks[&price];
            (price, level.total_amount)
        })
    }

    /// Get spread
    pub fn spread(&self) -> Option<u128> {
        let best_bid = self.best_bid()?.0;
        let best_ask = self.best_ask()?.0;
        if best_ask > best_bid {
            Some(best_ask - best_bid)
        } else {
            None
        }
    }
}

/// Cluster node role
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum NodeRole {
    Follower,
    Candidate,
    Leader,
}

/// Cluster node
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Node {
    /// Node ID
    pub node_id: String,
    /// Address
    pub address: String,
    /// Port
    pub port: u16,
    /// Role
    pub role: NodeRole,
    /// Is active
    pub is_active: bool,
    /// Last heartbeat
    pub last_heartbeat: i64,
    /// Region
    pub region: String,
}

impl Node {
    pub fn new(node_id: String, address: String, port: u16, region: String) -> Self {
        Self {
            node_id,
            address,
            port,
            role: NodeRole::Follower,
            is_active: true,
            last_heartbeat: Utc::now().timestamp(),
            region,
        }
    }
}

/// Matching cluster state
pub struct MatchingCluster {
    /// Cluster ID
    cluster_id: String,
    /// Current node ID
    node_id: String,
    /// Role
    role: NodeRole,
    /// Term (for Raft)
    term: u64,
    /// Voted for
    voted_for: Option<String>,
    /// Leader ID
    leader_id: Option<String>,
    /// Nodes
    nodes: Arc<RwLock<HashMap<String, Node>>>,
    /// Order books by market
    order_books: Arc<RwLock<HashMap<String, OrderBook>>>,
    /// Pending orders
    pending_orders: Arc<RwLock<Vec<Order>>>,
    /// Recent trades
    recent_trades: Arc<RwLock<Vec<Trade>>>,
    /// Current block
    current_block: u64,
}

impl MatchingCluster {
    pub fn new(cluster_id: String, node_id: String) -> Self {
        Self {
            cluster_id,
            node_id,
            role: NodeRole::Follower,
            term: 0,
            voted_for: None,
            leader_id: None,
            nodes: Arc::new(RwLock::new(HashMap::new())),
            order_books: Arc::new(RwLock::new(HashMap::new())),
            pending_orders: Arc::new(RwLock::new(Vec::new())),
            recent_trades: Arc::new(RwLock::new(Vec::new())),
            current_block: 0,
        }
    }

    /// Add node to cluster
    pub fn add_node(&self, node: Node) {
        let mut nodes = self.nodes.write();
        nodes.insert(node.node_id.clone(), node);
    }

    /// Get node
    pub fn get_node(&self, node_id: &str) -> Option<Node> {
        let nodes = self.nodes.read();
        nodes.get(node_id).cloned()
    }

    /// Get leader
    pub fn get_leader(&self) -> Option<Node> {
        let leader_id = self.leader_id.as_ref()?;
        let nodes = self.nodes.read();
        nodes.get(leader_id).cloned()
    }

    /// Check if current node is leader
    pub fn is_leader(&self) -> bool {
        self.role == NodeRole::Leader
    }

    /// Submit order (from API)
    pub fn submit_order(&self, order: Order) -> Result<String, MatchingError> {
        // In production: replicate via Raft
        // For now: validate and add to pending
        order.validate()?;
        
        let order_id = order.order_id.clone();
        let mut pending = self.pending_orders.write();
        pending.push(order);
        
        Ok(order_id)
    }

    /// Cancel order
    pub fn cancel_order(&self, order_id: &str, owner: &str) -> Result<(), MatchingError> {
        // Find and cancel order
        let mut pending = self.pending_orders.write();
        
        for order in pending.iter_mut() {
            if order.order_id == order_id && order.owner == owner {
                order.cancel();
                return Ok(());
            }
        }
        
        // Check order books
        let mut books = self.order_books.write();
        for book in books.values_mut() {
            if book.cancel_order(order_id, OrderSide::Buy).is_some() {
                return Ok(());
            }
            if book.cancel_order(order_id, OrderSide::Sell).is_some() {
                return Ok(());
            }
        }
        
        Err(MatchingError::OrderNotFound(order_id.to_string()))
    }

    /// Get order book for market
    pub fn get_order_book(&self, market: &str) -> Option<OrderBook> {
        let books = self.order_books.read();
        books.get(market).cloned()
    }

    /// Get recent trades
    pub fn get_recent_trades(&self, market: &str, limit: usize) -> Vec<Trade> {
        let trades = self.recent_trades.read();
        trades.iter()
            .filter(|t| t.market == market)
            .take(limit)
            .cloned()
            .collect()
    }

    /// Become leader (after election)
    pub fn become_leader(&mut self, term: u64) {
        self.role = NodeRole::Leader;
        self.term = term;
        self.leader_id = Some(self.node_id.clone());
        
        // Update node role
        let mut nodes = self.nodes.write();
        if let Some(node) = nodes.get_mut(&self.node_id) {
            node.role = NodeRole::Leader;
        }
    }

    /// Step down as leader
    pub fn step_down(&mut self) {
        self.role = NodeRole::Follower;
        self.leader_id = None;
    }

    /// Get cluster status
    pub fn status(&self) -> ClusterStatus {
        let nodes = self.nodes.read();
        let active_count = nodes.values().filter(|n| n.is_active).count();
        
        ClusterStatus {
            cluster_id: self.cluster_id.clone(),
            node_id: self.node_id.clone(),
            role: self.role,
            term: self.term,
            leader: self.leader_id.clone(),
            node_count: nodes.len(),
            active_nodes: active_count,
            order_book_count: self.order_books.read().len(),
            current_block: self.current_block,
        }
    }
}

/// Cluster status
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ClusterStatus {
    pub cluster_id: String,
    pub node_id: String,
    pub role: NodeRole,
    pub term: u64,
    pub leader: Option<String>,
    pub node_count: usize,
    pub active_nodes: usize,
    pub order_book_count: usize,
    pub current_block: u64,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_order_creation() {
        let order = Order::new(
            "0x1234".to_string(),
            "ETH-USDC".to_string(),
            OrderSide::Buy,
            OrderType::Limit,
            2000_000_000, // $2000
            1_000_000_000_000_000_000, // 1 ETH
        );
        
        assert!(order.validate().is_ok());
        assert_eq!(order.status, OrderStatus::Pending);
    }

    #[test]
    fn test_order_book() {
        let mut book = OrderBook::new("ETH-USDC".to_string());
        
        let order = Order::new(
            "0x1234".to_string(),
            "ETH-USDC".to_string(),
            OrderSide::Buy,
            OrderType::Limit,
            2000_000_000,
            1_000_000_000_000_000_000,
        );
        
        book.add_order(order).unwrap();
        assert!(book.best_bid().is_some());
    }

    #[test]
    fn test_cluster() {
        let cluster = MatchingCluster::new("test".to_string(), "node1".to_string());
        
        let node = Node::new(
            "node1".to_string(),
            "192.168.1.1".to_string(),
            8080,
            "us-east".to_string(),
        );
        
        cluster.add_node(node);
        assert!(cluster.get_leader().is_none());
    }
}