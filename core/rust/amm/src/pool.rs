//! Pool implementation for AMM

use super::math::{Q96, PriceMath};
use num_bigint::BigUint;
use parking_lot::RwLock;
use serde::{Deserialize, Serialize};
use std::sync::Arc;

/// Fee tiers
pub const FEE_TIERS: &[u32] = &[1, 5, 30, 100];

/// Pool configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PoolConfig {
    pub token0: String,
    pub token1: String,
    pub fee: u32,
    pub tick_spacing: u32,
    pub sqrt_price_x96: Option<BigUint>,
}

/// Swap result
#[derive(Debug, Clone)]
pub struct SwapResult {
    pub amount_out: BigUint,
    pub new_price: BigUint,
    pub fee_amount: BigUint,
}

/// Core pool state
#[derive(Debug, Clone)]
struct PoolState {
    token0: String,
    token1: String,
    fee: u32,
    tick_spacing: u32,
    sqrt_price_x96: BigUint,
    current_tick: i32,
    gross_liquidity: BigUint,
    reserves0: BigUint,
    reserves1: BigUint,
    fee_growth_global0: BigUint,
    fee_growth_global1: BigUint,
}

/// Core AMM Pool
pub struct PoolCore {
    state: RwLock<PoolState>,
}

impl PoolCore {
    /// Create a new pool
    pub fn new(token0: String, token1: String, fee: u32, tick_spacing: u32, sqrt_price_x96: BigUint) -> Self {
        let current_tick = PriceMath::get_tick_at_sqrt_price(&sqrt_price_x96);
        
        let state = PoolState {
            token0,
            token1,
            fee,
            tick_spacing,
            sqrt_price_x96,
            current_tick,
            gross_liquidity: BigUint::from(0u64),
            reserves0: BigUint::from(0u64),
            reserves1: BigUint::from(0u64),
            fee_growth_global0: BigUint::from(0u64),
            fee_growth_global1: BigUint::from(0u64),
        };
        
        Self {
            state: RwLock::new(state),
        }
    }

    /// Get current pool state
    pub fn get_state(&self) -> PoolState {
        self.state.read().clone()
    }

    /// Add liquidity to the pool
    pub fn add_liquidity(&self, amount0: BigUint, amount1: BigUint) -> BigUint {
        let mut state = self.state.write();
        
        state.reserves0 += amount0.clone();
        state.reserves1 += amount1.clone();
        
        let liquidity = if amount0 > amount1 { amount0 } else { amount1 };
        state.gross_liquidity += liquidity.clone();
        
        liquidity
    }

    /// Execute a swap
    pub fn swap(&self, amount_in: &BigUint, fee: u32) -> SwapResult {
        let mut state = self.state.write();
        
        let fee_multiplier = BigUint::from(1000000u64) - BigUint::from(fee as u64);
        let amount_in_with_fee = (amount_in * fee_multiplier) / BigUint::from(1000000u64);
        
        let new_reserve0 = state.reserves0.clone() + amount_in_with_fee.clone();
        let new_reserve1 = (&state.reserves0 * &state.reserves1) / &new_reserve0;
        let amount_out = &state.reserves1 - &new_reserve1;
        
        let sqrt_price_change = amount_in_with_fee.clone() / BigUint::from(1_000_000_000_000u64);
        let new_sqrt_price = state.sqrt_price_x96.clone() + sqrt_price_change;
        let new_tick = PriceMath::get_tick_at_sqrt_price(&new_sqrt_price);
        
        state.reserves0 = new_reserve0;
        state.reserves1 = new_reserve1;
        state.sqrt_price_x96 = new_sqrt_price;
        state.current_tick = new_tick;
        
        let fee_amount = amount_in - &amount_in_with_fee;
        
        SwapResult {
            amount_out,
            new_price: new_sqrt_price,
            fee_amount,
        }
    }

    /// Get reserve0
    pub fn get_reserve0(&self) -> BigUint {
        self.state.read().reserves0.clone()
    }

    /// Get reserve1
    pub fn get_reserve1(&self) -> BigUint {
        self.state.read().reserves1.clone()
    }

    /// Get current price
    pub fn get_current_price(&self) -> f64 {
        let state = self.state.read();
        let sqrt_price = &state.sqrt_price_x96;
        let q96_f64 = Q96.to_f64().unwrap_or(1.0);
        sqrt_price.to_f64().unwrap_or(0.0) / q96_f64
    }

    /// Get token0
    pub fn token0(&self) -> String {
        self.state.read().token0.clone()
    }

    /// Get token1
    pub fn token1(&self) -> String {
        self.state.read().token1.clone()
    }

    /// Get fee
    pub fn fee(&self) -> u32 {
        self.state.read().fee
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_pool_creation() {
        let pool = PoolCore::new(
            "0xA".to_string(),
            "0xB".to_string(),
            30,
            60,
            Q96.clone(),
        );
        
        let state = pool.get_state();
        assert_eq!(state.token0, "0xA");
        assert_eq!(state.token1, "0xB");
        assert_eq!(state.fee, 30);
    }

    #[test]
    fn test_add_liquidity() {
        let pool = PoolCore::new(
            "0xA".to_string(),
            "0xB".to_string(),
            30,
            60,
            Q96.clone(),
        );
        
        let liquidity = pool.add_liquidity(
            BigUint::from(1000u64),
            BigUint::from(1000u64),
        );
        
        assert!(liquidity > BigUint::from(0u64));
    }
}