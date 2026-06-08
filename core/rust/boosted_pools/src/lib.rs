//! TigerSwap Boosted Pools Engine
//! 
//! Implements Balancer V3-style boosted pools with yield generation:
//! - Aave integration for yield
//! - Linear pools
//! - Reinvestment pools
//! - Boosted stable pools
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
pub const CHAIN_POLYGON: u64 = 137;
pub const CHAIN_ARBITRUM: u64 = 42161;
pub const CHAIN_OPTIMISM: u64 = 10;
pub const CHAIN_BASE: u64 = 8453;

#[derive(Debug, Error)]
pub enum BoostedPoolError {
    #[error("Pool not found: {0}")]
    PoolNotFound(String),
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
    #[error("Yield insufficient: {0}")]
    YieldInsufficient(String),
    #[error("Invalid parameters: {0}")]
    InvalidParameters(String),
    #[error("Chain not supported: {0}")]
    ChainNotSupported(u64),
    #[error("Aave integration failed: {0}")]
    AaveError(String),
}

/// Pool type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum BoostedPoolType {
    Linear,           // Linear pool with yield
    AaveBoosted,      // Aave boosted
    Reinvest,        // Auto-compounding
    ComposableStable, // Stable with wrapped tokens
    Boosted2Token,   // Two token boosted
}

impl Default for BoostedPoolType {
    fn default() -> Self { BoostedPoolType::AaveBoosted }
}

/// Yield source
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct YieldSource {
    pub protocol: String,      // "aave", "compound", "yearn"
    pub underlying_token: String,
    pub yield_token: String,  // aUSDC, cUSDC, etc.
    pub apy: u256,            // Annual percentage yield (in basis points)
    pub last_updated: i64,
}

impl YieldSource {
    pub fn new(protocol: String, underlying: String, yield_token: String) -> Self {
        Self {
            protocol,
            underlying_token: underlying,
            yield_token,
            apy: 0,
            last_updated: Utc::now().timestamp(),
        }
    }

    pub fn update_apy(&mut self, apy: u256) {
        self.apy = apy;
        self.last_updated = Utc::now().timestamp();
    }
}

/// Boosted pool
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BoostedPool {
    pub pool_id: String,
    pub chain_id: u64,
    pub name: String,
    pub pool_type: BoostedPoolType,
    
    // Tokens
    pub main_token: String,     // USDC, USDT, etc.
    pub yield_token: String,    // aUSDC, aUSDT
    pub wrapped_token: String, // waUSDC
    
    // Yield source
    pub yield_source: YieldSource,
    
    // Balances
    pub total_main: u128,       // Main token balance
    pub total_yield: u128,     // Yield-bearing token balance
    pub available_liquidity: u128, // Liquid for swaps
    
    // Fees
    pub swap_fee: u256,
    pub yield_fee: u256,       // Fee on yield
    pub admin_fee: u256,
    
    // State
    pub is_paused: bool,
    pub total_supply: u128,
    
    pub created_at: i64,
    pub updated_at: i64,
}

impl BoostedPool {
    /// Create an Aave boosted pool
    pub fn new_aave_boosted(
        chain_id: u64,
        name: String,
        main_token: String,
        yield_token: String,
    ) -> Self {
        let yield_source = YieldSource::new("aave".to_string(), main_token.clone(), yield_token.clone());
        
        Self {
            pool_id: Uuid::new_v4().to_string(),
            chain_id,
            name,
            pool_type: BoostedPoolType::AaveBoosted,
            main_token,
            yield_token,
            wrapped_token: String::new(),
            yield_source,
            total_main: 0,
            total_yield: 0,
            available_liquidity: 0,
            swap_fee: 10,       // 0.1%
            yield_fee: 200,     // 20% of yield
            admin_fee: 200,    // 20% of yield fee
            is_paused: false,
            total_supply: 0,
            created_at: Utc::now().timestamp(),
            updated_at: Utc::now().timestamp(),
        }
    }

    /// Create a linear pool
    pub fn new_linear(chain_id: u64, name: String, main_token: String, wrapped_token: String) -> Self {
        let yield_source = YieldSource::new("linear".to_string(), main_token.clone(), wrapped_token.clone());
        
        Self {
            pool_id: Uuid::new_v4().to_string(),
            chain_id,
            name,
            pool_type: BoostedPoolType::Linear,
            main_token,
            yield_token: wrapped_token.clone(),
            wrapped_token,
            yield_source,
            total_main: 0,
            total_yield: 0,
            available_liquidity: 0,
            swap_fee: 10,
            yield_fee: 200,
            admin_fee: 200,
            is_paused: false,
            total_supply: 0,
            created_at: Utc::now().timestamp(),
            updated_at: Utc::now().timestamp(),
        }
    }

    /// Deposit main tokens
    pub fn deposit(&mut self, amount: u128) -> Result<u128, BoostedPoolError> {
        if amount == 0 {
            return Err(BoostedPoolError::InvalidParameters("Amount must be > 0".to_string()));
        }
        
        // Calculate yield tokens to mint
        let yield_to_mint = self.calculate_yield_mint(amount);
        
        self.total_main += amount;
        self.total_yield += yield_to_mint;
        self.available_liquidity += amount;
        self.updated_at = Utc::now().timestamp();
        
        Ok(yield_to_mint)
    }

    /// Withdraw main tokens
    pub fn withdraw(&mut self, yield_amount: u128) -> Result<u128, BoostedPoolError> {
        if yield_amount > self.total_yield {
            return Err(BoostedPoolError::InsufficientBalance("Insufficient yield tokens".to_string()));
        }
        
        // Calculate main tokens to receive
        let main_to_receive = self.calculate_main_receive(yield_amount);
        
        self.total_yield -= yield_amount;
        self.available_liquidity = self.available_liquidity.saturating_sub(main_to_receive);
        self.updated_at = Utc::now().timestamp();
        
        Ok(main_to_receive)
    }

    /// Swap (main <-> yield)
    pub fn swap(&mut self, from_main: bool, amount: u128, min_out: u128) -> Result<u128, BoostedPoolError> {
        let (in_token, out_token, out_amount) = if from_main {
            let out = self.calculate_yield_mint(amount);
            (self.main_token.clone(), self.yield_token.clone(), out)
        } else {
            let out = self.calculate_main_receive(amount);
            (self.yield_token.clone(), self.main_token.clone(), out)
        };
        
        if out_amount < min_out {
            return Err(BoostedPoolError::InvalidParameters("Slippage exceeded".to_string()));
        }
        
        if from_main {
            self.total_main += amount;
            self.total_yield += out_amount;
            self.available_liquidity += amount;
        } else {
            self.total_yield -= amount;
            self.available_liquidity = self.available_liquidity.saturating_sub(out_amount);
        }
        
        self.updated_at = Utc::now().timestamp();
        
        Ok(out_amount)
    }

    /// Calculate yield tokens to mint for main tokens
    fn calculate_yield_mint(&self, main_amount: u128) -> u128 {
        // 1:1 conversion rate initially
        main_amount
    }

    /// Calculate main tokens to receive for yield tokens
    fn calculate_main_receive(&self, yield_amount: u128) -> u128 {
        // 1:1 conversion rate
        yield_amount
    }

    /// Get swap output
    pub fn get_swap_output(&self, from_main: bool, amount: u128) -> u128 {
        if from_main {
            self.calculate_yield_mint(amount)
        } else {
            self.calculate_main_receive(amount)
        }
    }

    /// Update yield rate
    pub fn update_yield(&mut self, new_apy: u256) {
        self.yield_source.update_apy(new_apy);
    }

    /// Calculate pending yield
    pub fn calculate_pending_yield(&self, user_share: u128) -> u128 {
        if self.total_yield == 0 {
            return 0;
        }
        
        let time_elapsed = (Utc::now().timestamp() - self.updated_at) as u256;
        let yield_rate = self.yield_source.apy * time_elapsed / (365 days as u256 * 100);
        
        (user_share * yield_rate) / 10000
    }

    /// Get current APY
    pub fn get_apy(&self) -> u256 {
        self.yield_source.apy
    }

    /// Get liquidity available for swaps
    pub fn get_liquidity(&self) -> u128 {
        self.available_liquidity
    }
}

/// Linear pool (for token pairs with yield)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LinearPool {
    pub pool_id: String,
    pub chain_id: u64,
    pub name: String,
    pub token0: String,
    pub token1: String,
    pub main0: String,     // Main token 0
    pub main1: String,   // Main token 1
    pub wrapped0: String, // Wrapped token 0
    pub wrapped1: String, // Wrapped token 1
    
    // Balances
    pub balance0: u128,
    pub balance1: u128,
    
    // Fees
    pub fee: u256,
    
    pub created_at: i64,
}

impl LinearPool {
    pub fn new(chain_id: u64, name: String, token0: String, token1: String) -> Self {
        Self {
            pool_id: Uuid::new_v4().to_string(),
            chain_id,
            name,
            token0,
            token1,
            main0: token0.clone(),
            main1: token1.clone(),
            wrapped0: format!("w{}", token0),
            wrapped1: format!("w{}", token1),
            balance0: 0,
            balance1: 0,
            fee: 10,
            created_at: Utc::now().timestamp(),
        }
    }
}

/// Boosted Pools Engine
pub struct BoostedPoolsEngine {
    pools: Arc<RwLock<HashMap<String, BoostedPool>>>,
    linear_pools: Arc<RwLock<HashMap<String, LinearPool>>>,
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
}

impl BoostedPoolsEngine {
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [
            CHAIN_ETH, CHAIN_POLYGON, CHAIN_ARBITRUM, CHAIN_OPTIMISM, CHAIN_BASE,
        ].into_iter().collect();
        
        Self {
            pools: Arc::new(RwLock::new(HashMap::new())),
            linear_pools: Arc::new(RwLock::new(HashMap::new())),
            supported_chains: Arc::new(RwLock::new(chains)),
        }
    }

    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    /// Create Aave boosted pool
    pub fn create_aave_pool(
        &self,
        chain_id: u64,
        name: String,
        main_token: String,
        yield_token: String,
    ) -> Result<String, BoostedPoolError> {
        if !self.is_chain_supported(chain_id) {
            return Err(BoostedPoolError::ChainNotSupported(chain_id));
        }
        
        let pool = BoostedPool::new_aave_boosted(chain_id, name, main_token, yield_token);
        let pool_id = pool.pool_id.clone();
        
        self.pools.write().insert(pool_id.clone(), pool);
        
        Ok(pool_id)
    }

    /// Create linear pool
    pub fn create_linear_pool(
        &self,
        chain_id: u64,
        name: String,
        token0: String,
        token1: String,
    ) -> Result<String, BoostedPoolError> {
        if !self.is_chain_supported(chain_id) {
            return Err(BoostedPoolError::ChainNotSupported(chain_id));
        }
        
        let pool = LinearPool::new(chain_id, name, token0, token1);
        let pool_id = pool.pool_id.clone();
        
        self.linear_pools.write().insert(pool_id.clone(), pool);
        
        Ok(pool_id)
    }

    /// Get boosted pool
    pub fn get_pool(&self, pool_id: &str) -> Option<BoostedPool> {
        self.pools.read().get(pool_id).cloned()
    }

    /// Deposit
    pub fn deposit(&self, pool_id: &str, amount: u128) -> Result<u128, BoostedPoolError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| BoostedPoolError::PoolNotFound(pool_id.to_string()))?;
        
        pool.deposit(amount)
    }

    /// Withdraw
    pub fn withdraw(&self, pool_id: &str, yield_amount: u128) -> Result<u128, BoostedPoolError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| BoostedPoolError::PoolNotFound(pool_id.to_string()))?;
        
        pool.withdraw(yield_amount)
    }

    /// Swap
    pub fn swap(&self, pool_id: &str, from_main: bool, amount: u128, min_out: u128) -> Result<u128, BoostedPoolError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| BoostedPoolError::PoolNotFound(pool_id.to_string()))?;
        
        pool.swap(from_main, amount, min_out)
    }

    /// Get APY
    pub fn get_apy(&self, pool_id: &str) -> Result<u256, BoostedPoolError> {
        let pools = self.pools.read();
        let pool = pools.get(pool_id)
            .ok_or_else(|| BoostedPoolError::PoolNotFound(pool_id.to_string()))?;
        
        Ok(pool.get_apy())
    }

    /// Update yield rate
    pub fn update_yield(&self, pool_id: &str, apy: u256) -> Result<(), BoostedPoolError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| BoostedPoolError::PoolNotFound(pool_id.to_string()))?;
        
        pool.update_yield(apy);
        
        Ok(())
    }

    /// Get all pools
    pub fn get_pools(&self) -> Vec<BoostedPool> {
        self.pools.read().values().cloned().collect()
    }

    /// Add supported chain
    pub fn add_chain(&self, chain_id: u64) {
        self.supported_chains.write().insert(chain_id);
    }

    pub fn supported_chains(&self) -> Vec<u64> {
        self.supported_chains.read().iter().cloned().collect()
    }
}

impl Default for BoostedPoolsEngine {
    fn default() -> Self { Self::new() }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_aave_pool_creation() {
        let pool = BoostedPool::new_aave_boosted(
            CHAIN_ETH,
            "USDC-Aave".to_string(),
            "USDC".to_string(),
            "aUSDC".to_string(),
        );
        
        assert_eq!(pool.pool_type, BoostedPoolType::AaveBoosted);
    }

    #[test]
    fn test_linear_pool_creation() {
        let pool = LinearPool::new(
            CHAIN_ETH,
            "USDC/USDT-Linear".to_string(),
            "USDC".to_string(),
            "USDT".to_string(),
        );
        
        assert_eq!(pool.name, "USDC/USDT-Linear");
    }

    #[test]
    fn test_deposit() {
        let mut pool = BoostedPool::new_aave_boosted(
            CHAIN_ETH,
            "USDC-Aave".to_string(),
            "USDC".to_string(),
            "aUSDC".to_string(),
        );
        
        let yield_tokens = pool.deposit(1000).unwrap();
        
        assert_eq!(yield_tokens, 1000);
    }
}