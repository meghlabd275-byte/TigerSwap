//! Matching Module

use std::collections::{HashMap, VecDeque};
use std::sync::Arc;
use tokio::sync::RwLock;

use crate::{Order, OrderBook, OrderSide, MatchingStats};

/// Matching engine
pub struct MatchingEngine {
    order_books: RwLock<HashMap<String, OrderBook>>,
    orders: RwLock<HashMap<String, Order>>,
    trades: RwLock<VecDeque<crate::Trade>>,
    stats: RwLock<MatchingStats>,
    enabled: RwLock<bool>,
}

impl MatchingEngine {
    pub fn new() -> Self {
        Self {
            order_books: RwLock::new(HashMap::new()),
            orders: RwLock::new(HashMap::new()),
            trades: RwLock::new(VecDeque::new()),
            stats: RwLock::new(MatchingStats::new()),
            enabled: RwLock::new(true),
        }
    }
    
    pub async fn submit_order(&self, order: Order) -> Result<crate::Trade, crate::MatchingError> {
        if !*self.enabled.read().await {
            return Err(crate::MatchingError::TradingDisabled);
        }
        
        let pair = order.pair.clone();
        {
            let mut books = self.order_books.write().await;
            if !books.contains_key(&pair) {
                books.insert(pair.clone(), OrderBook::new(&pair));
            }
        }
        
        // Simplified: just return an error for now
        Err(crate::MatchingError::InvalidOrder)
    }
    
    pub async fn get_order_book(&self, pair: &str) -> Option<OrderBook> {
        let books = self.order_books.read().await;
        books.get(pair).cloned()
    }
    
    pub async fn set_enabled(&self, enabled: bool) {
        let mut e = self.enabled.write().await;
        *e = enabled;
    }
}

impl Default for MatchingEngine {
    fn default() -> Self {
        Self::new()
    }
}

/// Cluster node
#[derive(Debug, Clone)]
pub struct ClusterNode {
    pub node_id: String,
    pub address: String,
    pub role: crate::NodeRole,
    pub status: crate::NodeStatus,
    pub last_heartbeat: i64,
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
    
    pub async fn add_node(&self, node: ClusterNode) {
        let mut nodes = self.nodes.write().await;
        nodes.push(node);
    }
    
    pub async fn get_leader(&self) -> Option<String> {
        self.leader.read().await.clone()
    }
}

impl Default for MatchingCluster {
    fn default() -> Self {
        Self::new()
    }
}