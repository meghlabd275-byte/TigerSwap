//! Pool representation and operations for the routing engine

use num_bigint::BigUint;
use serde::{Deserialize, Serialize};
use std::fmt;
use std::sync::Arc;

/// DEX configuration for known DEXes
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DexConfig {
    pub name: &'static str,
    pub logo: &'static str,
    pub color: &'static str,
    pub factory: &'static str,
}

lazy_static::lazy_static! {
    /// Known DEX configurations
    pub static ref DEX_CONFIGS: std::collections::HashMap<&'static str, DexConfig> = {
        let mut m = std::collections::HashMap::new();
        m.insert("uniswap_v2", DexConfig {
            name: "Uniswap V2",
            logo: "🦄",
            color: "#FF007A",
            factory: "0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f",
        });
        m.insert("uniswap_v3", DexConfig {
            name: "Uniswap V3",
            logo: "🦄",
            color: "#FF007A",
            factory: "0x1F98431c8aD98523631AE4a59f267346ea31F984",
        });
        m.insert("sushiswap", DexConfig {
            name: "SushiSwap",
            logo: "🍣",
            color: "#FA52A0",
            factory: "0xC0AEe478e3658e2610c5F7A4A2E1777cE9e4f2c",
        });
        m.insert("pancakeswap", DexConfig {
            name: "PancakeSwap",
            logo: "🥞",
            color: "#633001",
            factory: "0x10970514F9494A73d7F43B8dEXb2C2B2E22F288",
        });
        m.insert("quickswap", DexConfig {
            name: "QuickSwap",
            logo: "⚡",
            color: "#6c8fc5",
            factory: "0x5757371414417b8C6CAad45bAeF941aB7dab7B3B",
        });
        m
    };
}

/// Unique identifier for a pool (token pair + DEX)
#[derive(Debug, Clone, Hash, Eq, PartialEq, Serialize, Deserialize)]
pub struct PoolKey {
    pub token0: String,
    pub token1: String,
    pub dex: String,
}

impl PoolKey {
    pub fn new(token0: &str, token1: &str, dex: &str) -> Self {
        // Normalize token order for uniqueness
        let (t0, t1) = if token0.to_lowercase() < token1.to_lowercase() {
            (token0.to_string(), token1.to_string())
        } else {
            (token1.to_string(), token0.to_string())
        };
        Self {
            token0: t0,
            token1: t1,
            dex: dex.to_string(),
        }
    }
}

/// A liquidity pool on a DEX
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Pool {
    /// Pool address
    pub address: String,
    /// Token0 address
    pub token0: String,
    /// Token1 address  
    pub token1: String,
    /// Reserve of token0
    pub reserve0: BigUint,
    /// Reserve of token1
    pub reserve1: BigUint,
    /// Fee in basis points
    pub fee_bps: u32,
    /// DEX name
    pub dex: String,
    /// Liquidity depth score
    pub liquidity_score: f64,
    /// Last updated timestamp
    pub last_update: u64,
}

impl Pool {
    /// Create a new pool
    pub fn new(
        token0: String,
        token1: String,
        address: String,
        reserve0: BigUint,
        reserve1: BigUint,
        fee_bps: u32,
        dex: String,
    ) -> Self {
        let liquidity_score = Self::calculate_liquidity_score(&reserve0, &reserve1);
        Self {
            address,
            token0,
            token1,
            reserve0,
            reserve1,
            fee_bps,
            dex,
            liquidity_score,
            last_update: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_secs(),
        }
    }

    /// Calculate liquidity score based on reserves
    fn calculate_liquidity_score(reserve0: &BigUint, reserve1: &BigUint) -> f64 {
        use num_traits::ToPrimitive;
        let r0 = reserve0.to_f64().unwrap_or(0.0);
        let r1 = reserve1.to_f64().unwrap_or(0.0);
        2.0 * (r0 * r1).sqrt()
    }

    /// Get the spot price (token1/token0)
    pub fn spot_price(&self) -> f64 {
        use num_traits::ToPrimitive;
        if self.reserve0 == BigUint::from(0u64) {
            return 0.0;
        }
        self.reserve1.to_f64().unwrap_or(0.0) / self.reserve0.to_f64().unwrap_or(1.0)
    }

    /// Calculate amount out using constant product formula
    /// amount_out = (amount_in * reserve_out) / (reserve_in + amount_in)
    /// With fee: amount_in_with_fee = amount_in * (10000 - fee_bps) / 10000
    pub fn get_amount_out(&self, amount_in: &BigUint) -> BigUint {
        if amount_in == &BigUint::from(0u64) {
            return BigUint::from(0u64);
        }
        
        let fee_multiplier = 10000 - self.fee_bps;
        let amount_in_with_fee = amount_in * fee_multiplier / 10000;
        
        let numerator = amount_in_with_fee * &self.reserve1;
        let denominator = &self.reserve0 + amount_in_with_fee;
        
        if denominator == BigUint::from(0u64) {
            return BigUint::from(0u64);
        }
        
        &self.reserve0 * &numerator / denominator
    }

    /// Calculate amount in required for desired amount out
    pub fn get_amount_in(&self, amount_out: &BigUint) -> BigUint {
        if amount_out == &BigUint::from(0u64) {
            return BigUint::from(0u64);
        }
        
        let numerator = &self.reserve0 * amount_out * 10000;
        let denominator = (&self.reserve1 - amount_out) * (10000 - self.fee_bps);
        
        if denominator == BigUint::from(0u64) {
            return BigUint::from(0u64);
        }
        
        (numerator / denominator) + BigUint::from(1u64)
    }

    /// Update reserves
    pub fn update_reserves(&mut self, reserve0: BigUint, reserve1: BigUint) {
        self.reserve0 = reserve0;
        self.reserve1 = reserve1;
        self.liquidity_score = Self::calculate_liquidity_score(&self.reserve0, &self.reserve1);
        self.last_update = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap()
            .as_secs();
    }

    /// Check if pool has sufficient liquidity
    pub fn has_liquidity(&self, min_liquidity: f64) -> bool {
        self.liquidity_score >= min_liquidity
    }

    /// Calculate price impact for a trade
    pub fn price_impact(&self, amount_in: &BigUint) -> f64 {
        let spot = self.spot_price();
        if spot == 0.0 {
            return 0.0;
        }
        
        let exec_price = self.get_amount_out(amount_in).to_f64().unwrap_or(0.0) 
            / amount_in.to_f64().unwrap_or(1.0);
        
        ((spot - exec_price) / spot * 100.0).max(0.0)
    }
}

impl fmt::Display for Pool {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "Pool({}::{}/{} [fee: {}bps] [liq: {:.2}])",
            self.dex,
            &self.token0[..std::cmp::min(8, self.token0.len())],
            &self.token1[..std::cmp::min(8, self.token0.len())],
            self.fee_bps,
            self.liquidity_score
        )
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_constant_product() {
        let pool = Pool::new(
            "A".to_string(),
            "B".to_string(),
            "0xpool".to_string(),
            BigUint::from(1_000_000u64),
            BigUint::from(1_000_000u64),
            300,
            "uniswap_v2".to_string(),
        );
        
        let amount_in = BigUint::from(1000u64);
        let amount_out = pool.get_amount_out(&amount_in);
        
        // With 0.3% fee, amount_in_with_fee = 997
        // amount_out = 1000 * 1000 / (1000 + 997) ≈ 500.75
        assert!(amount_out > BigUint::from(400u64));
        assert!(amount_out < BigUint::from(600u64));
    }

    #[test]
    fn test_pool_key_normalization() {
        let key1 = PoolKey::new("0xAAA", "0xbbb", "uniswap");
        let key2 = PoolKey::new("0xbbb", "0xAAA", "uniswap");
        
        assert_eq!(key1.token0, key2.token0);
        assert_eq!(key1.token1, key2.token1);
    }
}