//! Advanced Oracle System
//! 
//! Provides decentralized oracle with consensus, committee, reputation, staking, and rewards

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::Arc;
use parking_lot::RwLock;
use chrono::Utc;

/// Oracle status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum OracleStatus {
    Active,
    Inactive,
    Slashed,
    Banned,
}

/// Oracle node
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OracleNode {
    pub address: String,
    pub stake: u128,
    pub reputation: u32,
    pub status: OracleStatus,
    pub last_update: i64,
    pub accuracy: f64,
    pub total_updates: u64,
}

impl OracleNode {
    pub fn new(address: String, stake: u128) -> Self {
        Self {
            address,
            stake,
            reputation: 100,
            status: OracleStatus::Active,
            last_update: Utc::now().timestamp(),
            accuracy: 1.0,
            total_updates: 0,
        }
    }

    pub fn update(&mut self, price: u128, correct: bool) {
        self.total_updates += 1;
        self.last_update = Utc::now().timestamp();
        
        if correct {
            self.accuracy = (self.accuracy * (self.total_updates - 1) as f64 + 1.0) / self.total_updates as f64;
        } else {
            self.accuracy = (self.accuracy * (self.total_updates - 1) as f64) / self.total_updates as f64;
        }
        
        // Update reputation based on accuracy
        self.reputation = (self.accuracy * 100.0) as u32;
    }
}

/// Price update
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PriceUpdate {
    pub token_pair: String,
    pub price: u128,
    pub timestamp: i64,
    pub block: u64,
    pub proposer: String,
    pub signature: String,
}

/// Oracle price
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OraclePrice {
    pub token_pair: String,
    pub price: u128,
    pub timestamp: i64,
    pub confidence: u32,
    pub sources: u32,
}

/// Advanced Oracle
pub struct AdvancedOracle {
    nodes: Arc<RwLock<HashMap<String, OracleNode>>>,
    prices: Arc<RwLock<HashMap<String, Vec<PriceUpdate>>>>,
    consensus_prices: Arc<RwLock<HashMap<String, OraclePrice>>>,
    min_stake: u128,
    min_nodes: usize,
}

impl AdvancedOracle {
    pub fn new() -> Self {
        Self {
            nodes: Arc::new(RwLock::new(HashMap::new())),
            prices: Arc::new(RwLock::new(HashMap::new())),
            consensus_prices: Arc::new(RwLock::new(HashMap::new())),
            min_stake: 100_000_000_000_000_000, // 100 tokens
            min_nodes: 3,
        }
    }

    pub fn register_node(&self, node: OracleNode) -> bool {
        if node.stake < self.min_stake {
            return false;
        }
        
        let mut nodes = self.nodes.write();
        nodes.insert(node.address.clone(), node);
        true
    }

    pub fn submit_price(&self, update: PriceUpdate) -> Result<(), String> {
        let nodes = self.nodes.read();
        if !nodes.contains_key(&update.proposer) {
            return Err("Node not registered".to_string());
        }
        
        let mut prices = self.prices.write();
        let token_prices = prices.entry(update.token_pair.clone()).or_insert_with(Vec::new);
        token_prices.push(update);
        
        Ok(())
    }

    pub fn compute_consensus(&self, token_pair: &str) -> Option<OraclePrice> {
        let prices = self.prices.read();
        let token_prices = prices.get(token_pair)?;
        
        if token_prices.len() < self.min_nodes {
            return None;
        }
        
        // Median price
        let mut sorted: Vec<u128> = token_prices.iter().map(|p| p.price).collect();
        sorted.sort();
        let mid = sorted.len() / 2;
        let median = sorted[mid];
        
        // Confidence based on node count
        let sources = token_prices.len() as u32;
        let confidence = ((sources as f64 / 21.0) * 100.0) as u32; // 21 needed for full
        
        Some(OraclePrice {
            token_pair: token_pair.to_string(),
            price: median,
            timestamp: Utc::now().timestamp(),
            confidence: confidence.min(100),
            sources,
        })
    }

    pub fn get_price(&self, token_pair: &str) -> Option<OraclePrice> {
        let consensus = self.consensus_prices.read();
        consensus.get(token_pair).cloned()
    }

    pub fn get_active_nodes(&self) -> Vec<OracleNode> {
        let nodes = self.nodes.read();
        nodes.values()
            .filter(|n| n.status == OracleStatus::Active)
            .cloned()
            .collect()
    }
}

impl Default for AdvancedOracle {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_oracle() {
        let oracle = AdvancedOracle::new();
        
        let node = OracleNode::new("0xnode1".to_string(), 500_000_000_000_000_000);
        assert!(oracle.register_node(node));
    }
}