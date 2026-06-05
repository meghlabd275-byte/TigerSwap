//! AMM Factory - creates and manages pools

use super::pool::{PoolCore, PoolConfig};
use ahash::AHashMap;
use parking_lot::RwLock;
use std::sync::Arc;

/// AMM Factory for creating and managing pools
pub struct AMMFactory {
    pools: RwLock<AHashMap<String, Arc<PoolCore>>>,
}

impl AMMFactory {
    /// Create a new factory
    pub fn new() -> Self {
        Self {
            pools: RwLock::new(AHashMap::default()),
        }
    }

    /// Create a new pool
    pub fn create_pool(&self, config: PoolConfig) -> Result<Arc<PoolCore>, String> {
        let key = self.pool_key(&config.token0, &config.token1, config.fee);
        
        // Check if pool already exists
        {
            let pools = self.pools.read();
            if pools.contains_key(&key) {
                return Err("Pool already exists".to_string());
            }
        }
        
        let sqrt_price = config.sqrt_price_x96.unwrap_or_else(|| {
            // Default to price of 1 (tick 0)
            use super::math::Q96;
            Q96.clone()
        });
        
        let pool = Arc::new(PoolCore::new(
            config.token0,
            config.token1,
            config.fee,
            config.tick_spacing,
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

    /// Get all pools for a token pair
    pub fn get_pools_by_pair(&self, token0: &str, token1: &str) -> Vec<Arc<PoolCore>> {
        let pools = self.pools.read();
        pools.values()
            .filter(|pool| {
                let state = pool.get_state();
                (state.token0.to_lowercase() == token0.to_lowercase() && 
                 state.token1.to_lowercase() == token1.to_lowercase()) ||
                (state.token0.to_lowercase() == token1.to_lowercase() && 
                 state.token1.to_lowercase() == token0.to_lowercase())
            })
            .cloned()
            .collect()
    }

    /// Get all pools
    pub fn get_all_pools(&self) -> Vec<Arc<PoolCore>> {
        let pools = self.pools.read();
        pools.values().cloned().collect()
    }

    /// Generate pool key
    fn pool_key(&self, token0: &str, token1: &str, fee: u32) -> String {
        let (t0, t1) = if token0.to_lowercase() < token1.to_lowercase() {
            (token0.to_lowercase(), token1.to_lowercase())
        } else {
            (token1.to_lowercase(), token0.to_lowercase())
        };
        format!("{}-{}-{}", t0, t1, fee)
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
    /// Create a new swap router
    pub fn new(factory: Arc<AMMFactory>) -> Self {
        Self { factory }
    }

    /// Find the best route for a swap
    pub fn find_best_route(&self, token_in: &str, token_out: &str, amount_in: &num_bigint::BigUint) -> Vec<Arc<PoolCore>> {
        let pools = self.factory.get_pools_by_pair(token_in, token_out);
        
        // Sort by price (best first)
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
    ) -> Result<super::pool::SwapResult, String> {
        let fee = pool.fee();
        let result = pool.swap(amount_in, fee);
        
        if result.amount_out < *min_amount_out {
            return Err("Slippage tolerance exceeded".to_string());
        }
        
        Ok(result)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_factory_creation() {
        let factory = AMMFactory::new();
        assert_eq!(factory.get_all_pools().len(), 0);
    }

    #[test]
    fn test_create_pool() {
        let factory = AMMFactory::new();
        
        let config = PoolConfig {
            token0: "0xA".to_string(),
            token1: "0xB".to_string(),
            fee: 30,
            tick_spacing: 60,
            sqrt_price_x96: None,
        };
        
        let pool = factory.create_pool(config);
        assert!(pool.is_ok());
    }
}