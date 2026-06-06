//! AMM Factory - creates and manages pools

use super::pool::{PoolCore, PoolConfig, SwapResult, FEE_TIERS};
use super::math::Q96;
use ahash::AHashMap;
use parking_lot::RwLock;
use std::sync::Arc;

/// AMM Factory for creating and managing pools
pub struct AMMFactory {
    pools: RwLock<AHashMap<String, Arc<PoolCore>>>,
    fee_tiers: Vec<u32>,
}

impl AMMFactory {
    /// Create a new factory with default fee tiers
    pub fn new() -> Self {
        Self {
            pools: RwLock::new(AHashMap::default()),
            fee_tiers: FEE_TIERS.to_vec(),
        }
    }

    /// Create a new factory with custom fee tiers
    pub fn with_fee_tiers(fee_tiers: Vec<u32>) -> Self {
        Self {
            pools: RwLock::new(AHashMap::default()),
            fee_tiers,
        }
    }

    /// Create a new pool
    pub fn create_pool(&self, config: PoolConfig) -> Result<Arc<PoolCore>, String> {
        let key = self.pool_key(&config.token0, &config.token1, config.fee);
        
        {
            let pools = self.pools.read();
            if pools.contains_key(&key) {
                return Err("Pool already exists".to_string());
            }
        }
        
        let sqrt_price = config.sqrt_price_x96.unwrap_or_else(|| Q96.clone());
        
        let pool = Arc::new(PoolCore::new(
            config.token0,
            config.token1,
            config.fee,
            config.tick_spacing as i32,
            sqrt_price,
        ));
        
        let mut pools = self.pools.write();
        pools.insert(key, pool.clone());
        
        Ok(pool)
    }

    /// Get a pool by token pair and fee
    pub fn get_pool(&self, token0: &str, token1: &str, fee: u32) -> Option<Arc<PoolCore>> {
        let key = self.pool_key(token0, token1, fee);
        let pools = self.pools.read();
        pools.get(&key).cloned()
    }

    /// Get all pools for a token pair (all fee tiers)
    pub fn get_pools_by_pair(&self, token0: &str, token1: &str) -> Vec<Arc<PoolCore>> {
        let pools = self.pools.read();
        pools.values()
            .filter(|pool| {
                let state = pool.get_state();
                let t0 = token0.to_lowercase();
                let t1 = token1.to_lowercase();
                let s0 = state.token0.to_lowercase();
                let s1 = state.token1.to_lowercase();
                (s0 == t0 && s1 == t1) || (s0 == t1 && s1 == t0)
            })
            .cloned()
            .collect()
    }

    /// Get all pools
    pub fn get_all_pools(&self) -> Vec<Arc<PoolCore>> {
        let pools = self.pools.read();
        pools.values().cloned().collect()
    }

    /// Get pools count
    pub fn pools_count(&self) -> usize {
        self.pools.read().len()
    }

    /// Get supported fee tiers
    pub fn get_fee_tiers(&self) -> &[u32] {
        &self.fee_tiers
    }

    fn pool_key(&self, token0: &str, token1: &str, fee: u32) -> String {
        let (t0, t1) = if token0.to_lowercase() < token1.to_lowercase() {
            (token0.to_lowercase(), token1.to_lowercase())
        } else {
            (token1.to_lowercase(), token0.to_lowercase())
        };
        format!("{}-{}-{}", t0, t1, fee)
    }

    /// Find best pool for a swap
    pub fn find_best_pool(&self, token_in: &str, token_out: &str) -> Option<Arc<PoolCore>> {
        let pools = self.get_pools_by_pair(token_in, token_out);
        
        if pools.is_empty() {
            return None;
        }

        let mut sorted: Vec<_> = pools;
        sorted.sort_by(|a, b| {
            let liquidity_a = a.liquidity();
            let liquidity_b = b.liquidity();
            liquidity_b.cmp(&liquidity_a)
        });
        
        sorted.into_iter().next()
    }

    /// Remove a pool
    pub fn remove_pool(&self, token0: &str, token1: &str, fee: u32) -> Option<Arc<PoolCore>> {
        let key = self.pool_key(token0, token1, fee);
        let mut pools = self.pools.write();
        pools.remove(&key)
    }
}

impl Default for AMMFactory {
    fn default() -> Self {
        Self::new()
    }
}

/// Swap Router for finding best routes
pub struct SwapRouter {
    factory: Arc<AMMFactory>,
}

impl SwapRouter {
    pub fn new(factory: Arc<AMMFactory>) -> Self {
        Self { factory }
    }

    /// Find the best route for a swap
    pub fn find_best_route(&self, token_in: &str, token_out: &str, amount_in: &num_bigint::BigUint) -> Vec<Arc<PoolCore>> {
        let pools = self.factory.get_pools_by_pair(token_in, token_out);
        
        let mut sorted: Vec<_> = pools.into_iter().collect();
        sorted.sort_by(|a, b| {
            let price_a = a.get_current_price();
            let price_b = b.get_current_price();
            if token_in.to_lowercase() == a.token0().to_lowercase() {
                price_a.partial_cmp(&price_b).unwrap()
            } else {
                price_b.partial_cmp(&price_a).unwrap()
            }
        });
        
        sorted
    }

    /// Execute a swap on a pool
    pub fn execute_swap(
        &self,
        pool: &PoolCore,
        amount_in: &num_bigint::BigUint,
        min_amount_out: &num_bigint::BigUint,
        zero_for_one: bool,
    ) -> Result<SwapResult, String> {
        let result = pool.swap(amount_in, zero_for_one, None)?;
        
        if result.amount_out < *min_amount_out {
            return Err("Slippage tolerance exceeded".to_string());
        }
        
        Ok(result)
    }

    /// Get all pools
    pub fn get_all_pools(&self) -> Vec<Arc<PoolCore>> {
        self.factory.get_all_pools()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_factory_creation() {
        let factory = AMMFactory::new();
        assert_eq!(factory.pools_count(), 0);
    }

    #[test]
    fn test_create_pool() {
        let factory = AMMFactory::new();
        
        let config = PoolConfig {
            token0: "0xA".to_string(),
            token1: "0xB".to_string(),
            fee: 3000,
            tick_spacing: 60,
            sqrt_price_x96: None,
        };
        
        let pool = factory.create_pool(config);
        assert!(pool.is_ok());
        assert_eq!(factory.pools_count(), 1);
    }

    #[test]
    fn test_get_pool() {
        let factory = AMMFactory::new();
        
        let config = PoolConfig {
            token0: "0xA".to_string(),
            token1: "0xB".to_string(),
            fee: 3000,
            tick_spacing: 60,
            sqrt_price_x96: None,
        };
        
        factory.create_pool(config).unwrap();
        
        let pool = factory.get_pool("0xA", "0xB", 3000);
        assert!(pool.is_some());
    }

    #[test]
    fn test_find_best_pool() {
        let factory = AMMFactory::new();
        
        factory.create_pool(PoolConfig {
            token0: "0xA".to_string(),
            token1: "0xB".to_string(),
            fee: 100,
            tick_spacing: 10,
            sqrt_price_x96: None,
        }).unwrap();
        
        factory.create_pool(PoolConfig {
            token0: "0xA".to_string(),
            token1: "0xB".to_string(),
            fee: 3000,
            tick_spacing: 60,
            sqrt_price_x96: None,
        }).unwrap();
        
        let best = factory.find_best_pool("0xA", "0xB");
        assert!(best.is_some());
    }
}
