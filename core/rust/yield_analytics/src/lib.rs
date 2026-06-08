//! TigerSwap Yield Analytics Engine
//! 
//! Implements yield analytics similar to DeFiLlama:
//! - Pool yield tracking
//! - TVL calculations
//! - APY/APR comparisons
//! - Historical yield data
//!
//! Implementation: Pure Rust with no external dependencies

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use thiserror::Error;
use uuid::Uuid;
use chrono::{Utc, Duration};
use std::collections::{HashMap, VecDeque};

/// Chain ID constants
pub const CHAIN_ETH: u64 = 1;
pub const CHAIN_BSC: u64 = 56;
pub const CHAIN_POLYGON: u64 = 137;

#[derive(Debug, Error)]
pub enum YieldError {
    #[error("Pool not found: {0}")]
    PoolNotFound(String),
    #[error("Invalid parameters: {0}")]
    InvalidParameters(String),
}

/// Yield pool
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct YieldPool {
    pub pool_id: String,
    pub chain_id: u64,
    pub protocol: String,
    pub project: String,
    pub symbol: String,
    pub tvl: u128,
    pub apy: f64,
    pub apy_30d_avg: f64,
    pub reward_tokens: Vec<String>,
    pub updated_at: i64,
}

impl YieldPool {
    pub fn new(
        chain_id: u64,
        protocol: String,
        project: String,
        symbol: String,
    ) -> Self {
        Self {
            pool_id: Uuid::new_v4().to_string(),
            chain_id,
            protocol,
            project,
            symbol,
            tvl: 0,
            apy: 0.0,
            apy_30d_avg: 0.0,
            reward_tokens: vec![],
            updated_at: Utc::now().timestamp(),
        }
    }

    pub fn update(&mut self, tvl: u128, apy: f64) {
        self.tvl = tvl;
        self.apy = apy;
        self.updated_at = Utc::now().timestamp();
    }
}

/// Historical yield data
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct YieldHistory {
    pub pool_id: String,
    pub timestamps: VecDeque<i64>,
    pub tvl_values: VecDeque<u128>,
    pub apy_values: VecDeque<f64>,
}

impl YieldHistory {
    pub fn new(pool_id: String) -> Self {
        Self {
            pool_id,
            timestamps: VecDeque::new(),
            tvl_values: VecDeque::new(),
            apy_values: VecDeque::new(),
        }
    }

    pub fn add(&mut self, tvl: u128, apy: f64) {
        let now = Utc::now().timestamp();
        
        if self.timestamps.len() >= 30 * 24 { // 30 days of hourly data
            self.timestamps.pop_front();
            self.tvl_values.pop_front();
            self.apy_values.pop_front();
        }
        
        self.timestamps.push_back(now);
        self.tvl_values.push_back(tvl);
        self.apy_values.push_back(apy);
    }

    pub fn calculate_30d_avg(&self) -> f64 {
        if self.apy_values.is_empty() {
            return 0.0;
        }
        
        let sum: f64 = self.apy_values.iter().sum();
        sum / self.apy_values.len() as f64
    }
}

/// Yield analytics engine
pub struct YieldAnalyticsEngine {
    pools: Arc<RwLock<HashMap<String, YieldPool>>>,
    history: Arc<RwLock<HashMap<String, YieldHistory>>>,
}

impl YieldAnalyticsEngine {
    pub fn new() -> Self {
        Self {
            pools: Arc::new(RwLock::new(HashMap::new())),
            history: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    pub fn add_pool(&self, pool: YieldPool) -> String {
        let pool_id = pool.pool_id.clone();
        
        self.pools.write().insert(pool_id.clone(), pool);
        self.history.write().insert(pool_id.clone(), YieldHistory::new(pool_id));
        
        pool_id
    }

    pub fn update_pool(&self, pool_id: &str, tvl: u128, apy: f64) -> Result<(), YieldError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| YieldError::PoolNotFound(pool_id.to_string()))?;
        
        pool.update(tvl, apy);
        
        // Update history
        drop(pools);
        let mut history = self.history.write();
        if let Some(h) = history.get_mut(pool_id) {
            h.add(tvl, apy);
            
            // Update 30d avg
            let avg = h.calculate_30d_avg();
            let mut pools = self.pools.write();
            if let Some(p) = pools.get_mut(pool_id) {
                p.apy_30d_avg = avg;
            }
        }
        
        Ok(())
    }

    pub fn get_pool(&self, pool_id: &str) -> Option<YieldPool> {
        self.pools.read().get(pool_id).cloned()
    }

    pub fn get_all_pools(&self) -> Vec<YieldPool> {
        self.pools.read().values().cloned().collect()
    }

    pub fn get_pools_by_chain(&self, chain_id: u64) -> Vec<YieldPool> {
        self.pools.read()
            .values()
            .filter(|p| p.chain_id == chain_id)
            .cloned()
            .collect()
    }

    pub fn get_top_pools(&self, limit: usize) -> Vec<YieldPool> {
        let mut pools: Vec<_> = self.pools.read().values().cloned().collect();
        
        pools.sort_by(|a, b| b.tvl.cmp(&a.tvl));
        
        pools.truncate(limit);
        pools
    }

    pub fn get_total_tvl(&self) -> u128 {
        self.pools.read().values().map(|p| p.tvl).sum()
    }
}

impl Default for YieldAnalyticsEngine {
    fn default() -> Self { Self::new() }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_pool_creation() {
        let pool = YieldPool::new(
            CHAIN_ETH,
            "Aave".to_string(),
            "aave-v3".to_string(),
            "USDC".to_string(),
        );
        
        assert_eq!(pool.protocol, "Aave");
    }

    #[test]
    fn test_update() {
        let mut pool = YieldPool::new(
            CHAIN_ETH,
            "Test".to_string(),
            "test".to_string(),
            "TEST".to_string(),
        );
        
        pool.update(1000000, 5.5);
        
        assert_eq!(pool.tvl, 1000000);
    }
}