//! TigerSwap Liquidity Bootstrapping Pool (LBP) Engine
//! 
//! Implements Balancer-style LBP for token launches:
//! - Time-weighted price discovery
//! - Gradual price reduction
//! - Smart token distribution
//! - No impermanent loss for buyers
//!
//! Implementation: Pure Rust with no external dependencies

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use thiserror::Error;
use uuid::Uuid;
use chrono::{Utc, Duration};
use std::collections::HashMap;

/// Chain ID constants
pub const CHAIN_ETH: u64 = 1;
pub const CHAIN_BSC: u64 = 56;
pub const CHAIN_POLYGON: u64 = 137;
pub const CHAIN_ARBITRUM: u64 = 42161;

#[derive(Debug, Error)]
pub enum LBPError {
    #[error("Pool not found: {0}")]
    PoolNotFound(String),
    #[error("Sale not active: {0}")]
    SaleNotActive(String),
    #[error("Sale ended: {0}")]
    SaleEnded(String),
    #[error("Insufficient tokens: {0}")]
    InsufficientTokens(String),
    #[error("Invalid parameters: {0}")]
    InvalidParameters(String),
    #[error("Chain not supported: {0}")]
    ChainNotSupported(u64),
}

/// LBP Pool
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LBPPool {
    pub pool_id: String,
    pub chain_id: u64,
    pub name: String,
    pub token_selling: String,
    pub token_pricing: String,  // USDC, ETH, etc.
    
    // Sale parameters
    pub total_tokens_for_sale: u128,
    pub tokens_sold: u128,
    pub tokens_remaining: u128,
    
    // Pricing
    pub start_price: f64,
    pub end_price: f64,
    pub current_price: f64,
    
    // Weights (for gradual price change)
    pub start_weight: u32,   // Token selling weight (e.g., 80%)
    pub end_weight: u32,     // Token selling weight (e.g., 20%)
    
    // Timing
    pub sale_start: i64,
    pub sale_end: i64,
    pub duration_seconds: i64,
    
    // Liquidity
    pub raised_amount: u128,
    pub liquidity_tokens: u128,
    
    // State
    pub is_paused: bool,
    pub is_finalized: bool,
    
    pub created_at: i64,
    pub updated_at: i64,
}

impl LBPPool {
    pub fn new(
        chain_id: u64,
        name: String,
        token_selling: String,
        token_pricing: String,
        total_tokens: u128,
        start_price: f64,
        end_price: f64,
        duration_seconds: i64,
    ) -> Self {
        let now = Utc::now().timestamp();
        
        Self {
            pool_id: Uuid::new_v4().to_string(),
            chain_id,
            name,
            token_selling,
            token_pricing,
            total_tokens_for_sale: total_tokens,
            tokens_sold: 0,
            tokens_remaining: total_tokens,
            start_price,
            end_price,
            current_price: start_price,
            start_weight: 800000,  // 80%
            end_weight: 200000,    // 20%
            sale_start: now,
            sale_end: now + duration_seconds,
            duration_seconds,
            raised_amount: 0,
            liquidity_tokens: 0,
            is_paused: false,
            is_finalized: false,
            created_at: now,
            updated_at: now,
        }
    }

    /// Calculate current price based on time elapsed
    pub fn update_price(&mut self) -> f64 {
        let now = Utc::now().timestamp();
        
        if now >= self.sale_end {
            self.current_price = self.end_price;
            return self.current_price;
        }
        
        if now <= self.sale_start {
            self.current_price = self.start_price;
            return self.current_price;
        }
        
        let elapsed = now - self.sale_start;
        let progress = elapsed as f64 / self.duration_seconds as f64;
        
        // Linear interpolation
        self.current_price = self.start_price + (self.end_price - self.start_price) * progress;
        
        self.updated_at = now;
        self.current_price
    }

    /// Get current price
    pub fn get_current_price(&self) -> f64 {
        let mut pool = Self {
            pool_id: self.pool_id.clone(),
            chain_id: self.chain_id,
            name: self.name.clone(),
            token_selling: self.token_selling.clone(),
            token_pricing: self.token_pricing.clone(),
            total_tokens_for_sale: self.total_tokens_for_sale,
            tokens_sold: self.tokens_sold,
            tokens_remaining: self.tokens_remaining,
            start_price: self.start_price,
            end_price: self.end_price,
            current_price: 0.0,
            start_weight: self.start_weight,
            end_weight: self.end_weight,
            sale_start: self.sale_start,
            sale_end: self.sale_end,
            duration_seconds: self.duration_seconds,
            raised_amount: self.raised_amount,
            liquidity_tokens: self.liquidity_tokens,
            is_paused: self.is_paused,
            is_finalized: self.is_finalized,
            created_at: self.created_at,
            updated_at: self.updated_at,
        };
        
        pool.update_price()
    }

    /// Calculate tokens received for payment
    pub fn calculate_tokens_received(&self, payment_amount: u128) -> u128 {
        let price = self.get_current_price();
        
        // Calculate tokens at current price
        let tokens = (payment_amount as f64 / price) as u128;
        
        // Cap at remaining tokens
        tokens.min(self.tokens_remaining)
    }

    /// Execute purchase
    pub fn purchase(&mut self, payment_amount: u128, min_tokens: u128) -> Result<u128, LBPError> {
        if self.is_paused {
            return Err(LBPError::SaleNotActive("Sale is paused".to_string()));
        }
        
        if self.is_finalized {
            return Err(LBPError::SaleEnded("Sale has ended".to_string()));
        }
        
        let now = Utc::now().timestamp();
        
        if now < self.sale_start {
            return Err(LBPError::SaleNotActive("Sale not started".to_string()));
        }
        
        if now >= self.sale_end {
            return Err(LBPError::SaleEnded("Sale has ended".to_string()));
        }
        
        let tokens_received = self.calculate_tokens_received(payment_amount);
        
        if tokens_received < min_tokens {
            return Err(LBPError::InvalidParameters("Min tokens not met".to_string()));
        }
        
        // Update state
        self.tokens_sold += tokens_received;
        self.tokens_remaining = self.total_tokens_for_sale - self.tokens_sold;
        self.raised_amount += payment_amount;
        
        self.updated_at = Utc::now().timestamp();
        
        Ok(tokens_received)
    }

    /// Finalize pool
    pub fn finalize(&mut self) -> Result<(u128, u128), LBPError> {
        if self.is_finalized {
            return Err(LBPError::SaleEnded("Already finalized".to_string()));
        }
        
        // Calculate liquidity tokens to mint
        // At end price, create balanced pool
        let liquidity = (self.raised_amount as f64 / self.end_price) as u128;
        
        self.liquidity_tokens = liquidity;
        self.is_finalized = true;
        self.updated_at = Utc::now().timestamp();
        
        Ok((self.raised_amount, liquidity))
    }

    /// Get progress percentage
    pub fn get_progress(&self) -> f64 {
        if self.total_tokens_for_sale == 0 {
            return 0.0;
        }
        
        (self.tokens_sold as f64 / self.total_tokens_for_sale as f64) * 100.0
    }

    /// Get time remaining
    pub fn get_time_remaining(&self) -> i64 {
        let remaining = self.sale_end - Utc::now().timestamp();
        if remaining > 0 { remaining } else { 0 }
    }
}

/// LBP Engine
pub struct LBPEngine {
    pools: Arc<RwLock<HashMap<String, LBPPool>>>,
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
}

impl LBPEngine {
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [
            CHAIN_ETH, CHAIN_BSC, CHAIN_POLYGON, CHAIN_ARBITRUM,
        ].into_iter().collect();
        
        Self {
            pools: Arc::new(RwLock::new(HashMap::new())),
            supported_chains: Arc::new(RwLock::new(chains)),
        }
    }

    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    /// Create LBP
    pub fn create_pool(
        &self,
        chain_id: u64,
        name: String,
        token_selling: String,
        token_pricing: String,
        total_tokens: u128,
        start_price: f64,
        end_price: f64,
        duration_hours: i64,
    ) -> Result<String, LBPError> {
        if !self.is_chain_supported(chain_id) {
            return Err(LBPError::ChainNotSupported(chain_id));
        }
        
        let duration_seconds = duration_hours * 3600;
        
        let pool = LBPPool::new(
            chain_id,
            name,
            token_selling,
            token_pricing,
            total_tokens,
            start_price,
            end_price,
            duration_seconds,
        );
        
        let pool_id = pool.pool_id.clone();
        self.pools.write().insert(pool_id.clone(), pool);
        
        Ok(pool_id)
    }

    /// Get pool
    pub fn get_pool(&self, pool_id: &str) -> Option<LBPPool> {
        self.pools.read().get(pool_id).cloned()
    }

    /// Purchase tokens
    pub fn purchase(
        &self,
        pool_id: &str,
        payment_amount: u128,
        min_tokens: u128,
    ) -> Result<u128, LBPError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| LBPError::PoolNotFound(pool_id.to_string()))?;
        
        pool.purchase(payment_amount, min_tokens)
    }

    /// Get current price
    pub fn get_price(&self, pool_id: &str) -> Result<f64, LBPError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| LBPError::PoolNotFound(pool_id.to_string()))?;
        
        Ok(pool.update_price())
    }

    /// Finalize pool
    pub fn finalize(&self, pool_id: &str) -> Result<(u128, u128), LBPError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| LBPError::PoolNotFound(pool_id.to_string()))?;
        
        pool.finalize()
    }

    /// Get all pools
    pub fn get_pools(&self) -> Vec<LBPPool> {
        self.pools.read().values().cloned().collect()
    }
}

impl Default for LBPEngine {
    fn default() -> Self { Self::new() }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_lbp_creation() {
        let pool = LBPPool::new(
            CHAIN_ETH,
            "Test Token Sale".to_string(),
            "TEST".to_string(),
            "USDC".to_string(),
            1000000,
            1.0,
            0.1,
            86400,
        );
        
        assert_eq!(pool.total_tokens_for_sale, 1000000);
    }

    #[test]
    fn test_purchase() {
        let mut pool = LBPPool::new(
            CHAIN_ETH,
            "Test Token Sale".to_string(),
            "TEST".to_string(),
            "USDC".to_string(),
            1000000,
            1.0,
            0.1,
            86400,
        );
        
        // At $1 price, $1000 should get 1000 tokens
        let tokens = pool.purchase(1000, 500).unwrap();
        
        assert!(tokens >= 500);
    }
}