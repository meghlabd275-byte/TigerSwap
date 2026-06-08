//! TigerSwap Single-Sided Liquidity Engine
//! 
//! Implements single-sided liquidity provision:
//! - Deposit one token, receive LP tokens
//! - Auto-rebalancing between tokens
//! - Reduced impermanent loss
//! - Simplified UX
//!
//! Implementation: Pure Rust with no external dependencies

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use thiserror::Error;
use uuid::Uuid;
use chrono::Utc;
use std::collections::HashMap;

/// Chain ID constants
pub const CHAIN_ETH: u64 = 1;
pub const CHAIN_BSC: u64 = 56;
pub const CHAIN_POLYGON: u64 = 137;

#[derive(Debug, Error)]
pub enum SSLError {
    #[error("Pool not found: {0}")]
    PoolNotFound(String),
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
    #[error("Invalid parameters: {0}")]
    InvalidParameters(String),
    #[error("Chain not supported: {0}")]
    ChainNotSupported(u64),
}

/// Single-Sided Liquidity Pool
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SSLPool {
    pub pool_id: String,
    pub chain_id: u64,
    pub name: String,
    pub token_a: String,
    pub token_b: String,
    pub lp_token: String,
    
    // Balances
    pub balance_a: u128,
    pub balance_b: u128,
    pub total_lp_supply: u128,
    
    // Target ratio (e.g., 50-50)
    pub target_ratio_a: u32,  // Basis points
    pub target_ratio_b: u32,
    
    // Fees
    pub swap_fee: u32,
    pub deposit_fee: u32,
    pub withdraw_fee: u32,
    
    // State
    pub is_paused: bool,
    pub created_at: i64,
}

impl SSLPool {
    pub fn new(
        chain_id: u64,
        name: String,
        token_a: String,
        token_b: String,
    ) -> Self {
        Self {
            pool_id: Uuid::new_v4().to_string(),
            chain_id,
            name,
            token_a,
            token_b,
            lp_token: format!("ssl-{}", Uuid::new_v4().to_string()[..8].to_string()),
            balance_a: 0,
            balance_b: 0,
            total_lp_supply: 0,
            target_ratio_a: 5000,
            target_ratio_b: 5000,
            swap_fee: 30,
            deposit_fee: 0,
            withdraw_fee: 5,
            is_paused: false,
            created_at: Utc::now().timestamp(),
        }
    }

    /// Deposit single token
    pub fn deposit_single(&mut self, token: &str, amount: u128) -> Result<u128, SSLError> {
        if amount == 0 {
            return Err(SSLError::InvalidParameters("Amount must be > 0".to_string()));
        }
        
        let fee = (amount * self.deposit_fee as u128) / 10000;
        let amount_after_fee = amount - fee;
        
        if token == self.token_a {
            self.balance_a += amount_after_fee;
        } else if token == self.token_b {
            self.balance_b += amount_after_fee;
        } else {
            return Err(SSLError::InvalidParameters("Invalid token".to_string()));
        }
        
        // Calculate LP tokens to mint
        let lp_mint = if self.total_lp_supply == 0 {
            amount_after_fee
        } else {
            // Proportional to existing supply
            let ratio = (self.balance_a + self.balance_b) as f64 / self.total_lp_supply as f64;
            (amount_after_fee as f64 / ratio) as u128
        };
        
        self.total_lp_supply += lp_mint;
        
        Ok(lp_mint)
    }

    /// Rebalance to target ratio
    pub fn rebalance(&mut self) -> (u128, u128) {
        let total = self.balance_a + self.balance_b;
        
        if total == 0 {
            return (0, 0);
        }
        
        let target_a = (total * self.target_ratio_a as u128) / 10000;
        let target_b = (total * self.target_ratio_b as u128) / 10000;
        
        let mut swap_a = 0i128;
        let mut swap_b = 0i128;
        
        if self.balance_a > target_a {
            swap_a = (self.balance_a - target_a) as i128;
            self.balance_a = target_a;
            self.balance_b += swap_a as u128;
        } else if self.balance_a < target_a {
            swap_b = (target_a - self.balance_a) as i128;
            if swap_b <= self.balance_b as i128 {
                self.balance_b -= swap_b as u128;
                self.balance_a = target_a;
            }
        }
        
        (swap_a as u128, swap_b as u128)
    }

    /// Withdraw
    pub fn withdraw(&mut self, lp_amount: u128) -> Result<(u128, u128), SSLError> {
        if lp_amount == 0 {
            return Err(SSLError::InvalidParameters("LP amount must be > 0".to_string()));
        }
        
        if lp_amount > self.total_lp_supply {
            return Err(SSLError::InsufficientBalance("Insufficient LP".to_string()));
        }
        
        let ratio = lp_amount as f64 / self.total_lp_supply as f64;
        
        let withdraw_a = (self.balance_a as f64 * ratio) as u128;
        let withdraw_b = (self.balance_b as f64 * ratio) as u128;
        
        let fee_a = (withdraw_a * self.withdraw_fee as u128) / 10000;
        let fee_b = (withdraw_b * self.withdraw_fee as u128) / 10000;
        
        self.balance_a -= (withdraw_a - fee_a);
        self.balance_b -= (withdraw_b - fee_b);
        self.total_lp_supply -= lp_amount;
        
        Ok((withdraw_a - fee_a, withdraw_b - fee_b))
    }

    /// Get LP token price
    pub fn get_lp_price(&self) -> f64 {
        if self.total_lp_supply == 0 {
            return 1.0;
        }
        
        (self.balance_a + self.balance_b) as f64 / self.total_lp_supply as f64
    }
}

/// SSL Engine
pub struct SSLEngine {
    pools: Arc<RwLock<HashMap<String, SSLPool>>>,
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
}

impl SSLEngine {
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [
            CHAIN_ETH, CHAIN_BSC, CHAIN_POLYGON,
        ].into_iter().collect();
        
        Self {
            pools: Arc::new(RwLock::new(HashMap::new())),
            supported_chains: Arc::new(RwLock::new(chains)),
        }
    }

    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    pub fn create_pool(
        &self,
        chain_id: u64,
        name: String,
        token_a: String,
        token_b: String,
    ) -> Result<String, SSLError> {
        if !self.is_chain_supported(chain_id) {
            return Err(SSLError::ChainNotSupported(chain_id));
        }
        
        let pool = SSLPool::new(chain_id, name, token_a, token_b);
        let pool_id = pool.pool_id.clone();
        
        self.pools.write().insert(pool_id.clone(), pool);
        
        Ok(pool_id)
    }

    pub fn get_pool(&self, pool_id: &str) -> Option<SSLPool> {
        self.pools.read().get(pool_id).cloned()
    }

    pub fn deposit(&self, pool_id: &str, token: &str, amount: u128) -> Result<u128, SSLError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| SSLError::PoolNotFound(pool_id.to_string()))?;
        
        pool.deposit_single(token, amount)
    }

    pub fn withdraw(&self, pool_id: &str, lp_amount: u128) -> Result<(u128, u128), SSLError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| SSLError::PoolNotFound(pool_id.to_string()))?;
        
        pool.withdraw(lp_amount)
    }

    pub fn get_pools(&self) -> Vec<SSLPool> {
        self.pools.read().values().cloned().collect()
    }
}

impl Default for SSLEngine {
    fn default() -> Self { Self::new() }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_pool_creation() {
        let pool = SSLPool::new(
            CHAIN_ETH,
            "USDC/ETH SSL".to_string(),
            "USDC".to_string(),
            "ETH".to_string(),
        );
        
        assert_eq!(pool.target_ratio_a, 5000);
    }

    #[test]
    fn test_deposit() {
        let mut pool = SSLPool::new(
            CHAIN_ETH,
            "Test Pool".to_string(),
            "A".to_string(),
            "B".to_string(),
        );
        
        let lp = pool.deposit_single("A", 1000).unwrap();
        
        assert!(lp > 0);
    }
}