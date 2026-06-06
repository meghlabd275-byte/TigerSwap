//! TigerSwap AMM Engine - Production-Ready Concentrated Liquidity
//! 
//! COMPLETELY SELF-CONTAINED implementation with:
//! - Constant product AMM (x*y=k)
//! - Concentrated liquidity (Uniswap V3 style)
//! - Stable swaps
//! - Fee tiers with proper growth calculations
//! - Tick management with crossing logic
//! - Position management for liquidity providers
//! - Full swap execution with price impact
//! 
//! NO external dependencies for core logic.

mod math;
mod pool;
mod factory;
mod swap;
mod tick;
mod position;

pub use math::{FullMath, BitMath, PriceMath, Q96, Q128, MAX_UINT256, TickMath};
pub use pool::{PoolCore, PoolConfig, SwapResult, FEE_TIERS, PoolState, SwapDirection};
pub use factory::{AMMFactory, SwapRouter};
pub use swap::{SwapExecutor, SwapParams};
pub use tick::{Tick, TickMap};
pub use position::{Position, PositionId};

/// Fee tier constants in basis points (with 6 decimal precision)
pub mod fee {
    /// 0.01% - stable pairs (USDC/USDT, etc.)
    pub const STABLE: u32 = 100;
    /// 0.05% - low volatility pairs
    pub const LOW: u32 = 500;
    /// 0.30% - standard pairs (ETH/USDC)
    pub const MEDIUM: u32 = 3000;
    /// 1.00% - exotic pairs
    pub const HIGH: u32 = 10000;
    /// Custom fee tier
    pub const CUSTOM: u32 = 0;
}

/// Tick spacing for fee tiers
pub mod tick_spacing {
    pub const STABLE: i32 = 1;
    pub const LOW: i32 = 10;
    pub const MEDIUM: i32 = 60;
    pub const HIGH: i32 = 200;
}

/// Maximum tick value
pub const MAX_TICK: i32 = 221818;
pub const MIN_TICK: i32 = -221818;

/// Convert fee basis points to fee tier
pub fn fee_tier_from_bps(bps: u32) -> u32 {
    match bps {
        100 => fee::STABLE,
        500 => fee::LOW,
        3000 => fee::MEDIUM,
        10000 => fee::HIGH,
        _ => fee::CUSTOM,
    }
}

/// Convert fee tier to human readable percentage
pub fn fee_tier_to_percentage(tier: u32) -> f64 {
    tier as f64 / 1000000.0 * 100.0
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
    fn test_tick_from_sqrt_price() {
        let sqrt_price = PriceMath::get_sqrt_price_at_tick(100);
        let tick = PriceMath::get_tick_at_sqrt_price(&sqrt_price);
        assert!(tick >= 99 && tick <= 101);
    }

    #[test]
    fn test_pool_creation() {
        let config = PoolConfig {
            token0: "0xA".to_string(),
            token1: "0xB".to_string(),
            fee: fee::MEDIUM,
            tick_spacing: tick_spacing::MEDIUM,
            sqrt_price_x96: Some(Q96),
        };
        
        let factory = AMMFactory::new();
        let pool = factory.create_pool(config);
        
        assert!(pool.is_ok());
    }

    #[test]
    fn test_fee_tier_conversion() {
        assert_eq!(fee_tier_to_percentage(fee::STABLE), 0.01);
        assert_eq!(fee_tier_to_percentage(fee::LOW), 0.05);
        assert_eq!(fee_tier_to_percentage(fee::MEDIUM), 0.3);
        assert_eq!(fee_tier_to_percentage(fee::HIGH), 1.0);
    }

    #[test]
    fn test_tick_range() {
        assert!(MAX_TICK > 0);
        assert!(MIN_TICK < 0);
        assert_eq!(MIN_TICK, -MAX_TICK);
    }
}
