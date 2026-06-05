//! TigerSwap AMM Engine - Concentrated Liquidity Implementation
//! 
//! High-performance AMM with support for:
//! - Constant product AMM (x*y=k)
//! - Concentrated liquidity (like Uniswap V3)
//! - Stable swaps
//! - Fee tiers

mod math;
mod pool;
mod factory;
mod swap;

pub use math::{FullMath, BitMath, PriceMath, Q96, Q128, MAX_UINT256};
pub use pool::{PoolCore, PoolConfig, SwapResult, FEE_TIERS};
pub use factory::{AMMFactory, SwapRouter};
pub use swap::{SwapExecutor, SwapParams};

/// Fee tier constants in basis points
pub mod fee {
    pub const STABLE: u32 = 1;    // 0.01% - stable pairs
    pub const LOW: u32 = 5;       // 0.05% - low volatility
    pub const MEDIUM: u32 = 30;   // 0.30% - standard
    pub const HIGH: u32 = 100;    // 1.00% - exotic pairs
    pub const CUSTOM: u32 = 0;    // Custom fee
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_sqrt_price_from_tick() {
        let sqrt_price = PriceMath::get_sqrt_price_at_tick(0);
        assert_eq!(sqrt_price, Q96);
    }

    #[test]
    fn test_pool_creation() {
        let config = PoolConfig {
            token0: "0xA".to_string(),
            token1: "0xB".to_string(),
            fee: fee::MEDIUM,
            tick_spacing: 60,
            sqrt_price_x96: Some(Q96),
        };
        
        let factory = AMMFactory::new();
        let pool = factory.create_pool(config);
        
        assert!(pool.is_ok());
    }
}