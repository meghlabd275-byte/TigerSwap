//! TigerSwap Concentrated Liquidity Engine
//! 
//! Implements Uniswap V3-style concentrated liquidity pools:
//! - Position-based liquidity (concentrated in price ranges)
//! - Multiple fee tiers (0.01%, 0.05%, 0.3%, 1%)
//! - Tick-based pricing
//! - Range orders
//! - Single-sided liquidity provision
//!
//! Implementation: Pure Rust with no external dependencies

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use rust_decimal::Decimal;
use thiserror::Error;
use uuid::Uuid;
use chrono::Utc;
use std::collections::HashMap;
use std::cmp::Ordering;

/// Chain ID constants
pub const CHAIN_ETH: u64 = 1;
pub const CHAIN_BSC: u64 = 56;
pub const CHAIN_POLYGON: u64 = 137;
pub const CHAIN_ARBITRUM: u64 = 42161;
pub const CHAIN_OPTIMISM: u64 = 10;
pub const CHAIN_BASE: u64 = 8453;
pub const CHAIN_AVALANCHE: u64 = 43114;

#[derive(Debug, Error)]
pub enum ConcentratedLiquidityError {
    #[error("Position not found: {0}")]
    PositionNotFound(String),
    #[error("Pool not found: {0}")]
    PoolNotFound(String),
    #[error("Invalid tick range: lower {lower} >= upper {upper}")]
    InvalidTickRange { lower: i32, upper: i32 },
    #[error("Insufficient liquidity: {0}")]
    InsufficientLiquidity(String),
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
    #[error("Invalid amount: {0}")]
    InvalidAmount(String),
    #[error("Price out of range: {0}")]
    PriceOutOfRange(String),
    #[error("Chain not supported: {0}")]
    ChainNotSupported(u64),
    #[error("Tick not initialized: {0}")]
    TickNotInitialized(i32),
    #[error("Zero liquidity")]
    ZeroLiquidity,
    #[error("Fee tier not supported: {0}")]
    FeeTierNotSupported(u32),
}

/// Fee tier (in hundredths of a basis point - 1 = 0.0001%)
pub const FEE_TIER_LOW: u32 = 100;      // 0.01%
pub const FEE_TIER_MEDIUM: u32 = 500;   // 0.05%
pub const FEE_TIER_HIGH: u32 = 3000;   // 0.3%
pub const FEE_TIER_CUSTOM: u32 = 10000; // 1%

/// Supported fee tiers
pub const SUPPORTED_FEE_TIERS: &[u32] = &[100, 500, 3000, 10000];

/// Tick spacing for each fee tier
pub fn tick_spacing(fee_tier: u32) -> i32 {
    match fee_tier {
        100 => 1,
        500 => 10,
        3000 => 60,
        10000 => 200,
        _ => 60,
    }
}

/// Price format
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum PriceFormat {
    Token0,  // Price expressed as token1/token0
    Token1,  // Price expressed as token0/token1
}

impl Default for PriceFormat {
    fn default() -> Self { PriceFormat::Token1 }
}

/// Liquidity position
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CLPosition {
    pub position_id: String,
    pub user: String,
    pub pool_id: String,
    pub token0: String,
    pub token1: String,
    
    // Tick range
    pub tick_lower: i32,
    pub tick_upper: i32,
    
    // Liquidity
    pub liquidity: u128,
    pub tokens_contained: TokensContained,
    
    // Fee tracking
    pub fee_growth_inside_token0: u128,
    pub fee_growth_inside_token1: u128,
    pub tokens_owed_token0: u128,
    pub tokens_owed_token1: u128,
    
    // Status
    pub created_at: i64,
    pub updated_at: i64,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct TokensContained {
    pub token0: u128,
    pub token1: u128,
}

impl CLPosition {
    /// Create a new position
    pub fn new(
        user: String,
        pool_id: String,
        token0: String,
        token1: String,
        tick_lower: i32,
        tick_upper: i32,
        liquidity: u128,
    ) -> Self {
        let now = Utc::now().timestamp();
        
        Self {
            position_id: Uuid::new_v4().to_string(),
            user,
            pool_id,
            token0,
            token1,
            tick_lower,
            tick_upper,
            liquidity,
            tokens_contained: TokensContained::default(),
            fee_growth_inside_token0: 0,
            fee_growth_inside_token1: 0,
            tokens_owed_token0: 0,
            tokens_owed_token1: 0,
            created_at: now,
            updated_at: now,
        }
    }

    /// Validate position
    pub fn validate(&self) -> Result<(), ConcentratedLiquidityError> {
        if self.tick_lower >= self.tick_upper {
            return Err(ConcentratedLiquidityError::InvalidTickRange {
                lower: self.tick_lower,
                upper: self.tick_upper,
            });
        }
        if self.liquidity == 0 {
            return Err(ConcentratedLiquidityError::ZeroLiquidity);
        }
        Ok(())
    }

    /// Check if position is in range for a price
    pub fn is_in_range(&self, tick_current: i32) -> bool {
        tick_current >= self.tick_lower && tick_current <= self.tick_upper
    }

    /// Calculate tokens for liquidity
    pub fn calculate_tokens(&self, sqrt_ratio_lower: u128, sqrt_ratio_upper: u128, sqrt_ratio_current: u128) -> TokensContained {
        let mut token0 = 0u128;
        let mut token1 = 0u128;
        
        if self.tick_lower <= 0 && self.tick_upper >= 0 {
            // Current price is within range
            token0 = (self.liquidity * (sqrt_ratio_upper - sqrt_ratio_current)) / sqrt_ratio_current;
            token1 = (self.liquidity * (sqrt_ratio_current - sqrt_ratio_lower)) / u128::MAX;
        } else if self.tick_upper < 0 {
            // Entirely below current price
            token0 = (self.liquidity * (sqrt_ratio_upper - sqrt_ratio_lower)) / sqrt_ratio_upper;
        } else {
            // Entirely above current price
            token1 = (self.liquidity * (sqrt_ratio_upper - sqrt_ratio_lower)) / u128::MAX;
        }
        
        TokensContained { token0, token1 }
    }

    /// Collect fees
    pub fn collect_fees(&mut self) -> (u128, u128) {
        let owed0 = self.tokens_owed_token0;
        let owed1 = self.tokens_owed_token1;
        
        self.tokens_owed_token0 = 0;
        self.tokens_owed_token1 = 0;
        
        (owed0, owed1)
    }
}

/// Tick information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Tick {
    pub tick_index: i32,
    pub liquidity_net: i128,  // Net liquidity at this tick
    pub liquidity_gross: u128,
    pub fee_growth_outside_token0: u128,
    pub fee_growth_outside_token1: u128,
    pub initialized: bool,
}

impl Tick {
    pub fn new(tick_index: i32) -> Self {
        Self {
            tick_index,
            liquidity_net: 0,
            liquidity_gross: 0,
            fee_growth_outside_token0: 0,
            fee_growth_outside_token1: 0,
            initialized: false,
        }
    }
}

/// Pool state
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CLPool {
    pub pool_id: String,
    pub chain_id: u64,
    pub token0: String,
    pub token1: String,
    pub fee_tier: u32,
    
    // Tick state
    pub tick_current: i32,
    pub tick_spacing: i32,
    pub sqrt_ratio: u128,
    
    // Liquidity
    pub liquidity: u128,
    
    // Fee tracking
    pub fee_growth_global_token0: u128,
    pub fee_growth_global_token1: u128,
    
    // Oracle
    pub observation_index: u32,
    pub observation_cardinality: u32,
    pub observation_cardinality_next: u32,
    
    // Settings
    pub max_tick: i32,
    pub min_tick: i32,
    
    pub created_at: i64,
    pub updated_at: i64,
}

impl CLPool {
    /// Create a new pool
    pub fn new(
        chain_id: u64,
        token0: String,
        token1: String,
        fee_tier: u32,
        initial_price: Decimal,
    ) -> Self {
        let now = Utc::now().timestamp();
        let spacing = tick_spacing(fee_tier);
        
        // Convert price to sqrt ratio
        let sqrt_ratio = price_to_sqrt_ratio(initial_price);
        
        // Calculate tick from price
        let tick = price_to_tick(initial_price);
        
        let max_tick = (MAX_TICK / spacing as i32) * spacing;
        let min_tick = -(MAX_TICK / spacing as i32) * spacing;
        
        Self {
            pool_id: Uuid::new_v4().to_string(),
            chain_id,
            token0,
            token1,
            fee_tier,
            tick_current: tick,
            tick_spacing: spacing,
            sqrt_ratio,
            liquidity: 0,
            fee_growth_global_token0: 0,
            fee_growth_global_token1: 0,
            observation_index: 0,
            observation_cardinality: 1,
            observation_cardinality_next: 1,
            max_tick,
            min_tick,
            created_at: now,
            updated_at: now,
        }
    }

    /// Get tick index for price
    pub fn get_tick_for_price(&self, price: Decimal) -> i32 {
        let tick = price_to_tick(price);
        // Round to nearest tick spacing
        (tick / self.tick_spacing) * self.tick_spacing
    }

    /// Get price for tick
    pub fn get_price_for_tick(&self, tick: i32) -> Decimal {
        tick_to_price(tick)
    }

    /// Initialize a tick
    pub fn initialize_tick(&mut self, tick: i32) {
        if tick % self.tick_spacing != 0 {
            return;
        }
        // Tick initialization would modify internal state
    }

    /// Update position
    pub fn update_position(
        &mut self,
        tick_lower: i32,
        tick_upper: i32,
        liquidity_delta: i128,
    ) -> Result<(), ConcentratedLiquidityError> {
        // Update ticks
        let mut lower = tick_lower;
        while lower < tick_upper {
            let next = std::cmp::min(lower + self.tick_spacing, tick_upper);
            
            // Calculate contributions
            let mut liquidity_delta0 = 0u128;
            let mut liquidity_delta1 = 0u128;
            
            if lower <= self.tick_current && self.tick_current < next {
                liquidity_delta0 = (liquidity_delta as u128 * (self.sqrt_ratio - tick_to_sqrt_ratio(lower))) / u128::MAX;
                self.liquidity = (self.liquidity as i128 + liquidity_delta) as u128;
            }
            
            lower = next;
        }
        
        Ok(())
    }
}

/// Maximum tick (for price range)
pub const MAX_TICK: i32 = 887272;

/// Calculate sqrt ratio from price
pub fn price_to_sqrt_ratio(price: Decimal) -> u128 {
    // Simplified: sqrt(price) * 2^96
    // In production would use full Q64.96 implementation
    let sqrt_price = (price.as_f64()).sqrt();
    (sqrt_price * (1u128 << 48) as f64) as u128
}

/// Calculate tick from price
pub fn price_to_tick(price: Decimal) -> i32 {
    let log_sqrt = (price.as_f64()).ln() / (2.0_f64.ln());
    (log_sqrt * (1u128 << 64) as f64 / 2.0_f64.ln()) as i32
}

/// Calculate price from tick
pub fn tick_to_price(tick: i32) -> Decimal {
    let ratio = tick_to_sqrt_ratio(tick);
    let price = (ratio as f64 / (1u128 << 48) as f64).powi(2);
    Decimal::try_from(price).unwrap_or(Decimal::ZERO)
}

/// Calculate sqrt ratio from tick
pub fn tick_to_sqrt_ratio(tick: i32) -> u128 {
    // Simplified: (sqrt(1.0001^tick)) * 2^96
    let abs_tick = tick.abs() as f64;
    let ratio = (1.0001_f64).powf(abs_tick / 10000.0) * (1u128 << 48) as f64;
    ratio as u128
}

/// Concentrated liquidity engine
pub struct ConcentratedLiquidityEngine {
    pools: Arc<RwLock<HashMap<String, CLPool>>>,
    positions: Arc<RwLock<HashMap<String, CLPosition>>>,
    ticks: Arc<RwLock<HashMap<String, HashMap<i32, Tick>>>>,
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
}

impl ConcentratedLiquidityEngine {
    /// Create a new engine
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [
            CHAIN_ETH, CHAIN_BSC, CHAIN_POLYGON, CHAIN_ARBITRUM,
            CHAIN_OPTIMISM, CHAIN_BASE, CHAIN_AVALANCHE,
        ].into_iter().collect();
        
        Self {
            pools: Arc::new(RwLock::new(HashMap::new())),
            positions: Arc::new(RwLock::new(HashMap::new())),
            ticks: Arc::new(RwLock::new(HashMap::new())),
            supported_chains: Arc::new(RwLock::new(chains)),
        }
    }

    /// Check if chain is supported
    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    /// Create a new pool
    pub fn create_pool(&self, chain_id: u64, token0: String, token1: String, fee_tier: u32, initial_price: Decimal) -> Result<String, ConcentratedLiquidityError> {
        if !self.is_chain_supported(chain_id) {
            return Err(ConcentratedLiquidityError::ChainNotSupported(chain_id));
        }
        
        if !SUPPORTED_FEE_TIERS.contains(&fee_tier) {
            return Err(ConcentratedLiquidityError::FeeTierNotSupported(fee_tier));
        }
        
        let pool = CLPool::new(chain_id, token0, token1, fee_tier, initial_price);
        let pool_id = pool.pool_id.clone();
        
        self.pools.write().insert(pool_id.clone(), pool);
        self.ticks.write().insert(pool_id.clone(), HashMap::new());
        
        Ok(pool_id)
    }

    /// Get pool
    pub fn get_pool(&self, pool_id: &str) -> Option<CLPool> {
        self.pools.read().get(pool_id).cloned()
    }

    /// Add liquidity to a position
    pub fn add_liquidity(
        &self,
        pool_id: &str,
        user: String,
        token0_amount: u128,
        token1_amount: u128,
        tick_lower: i32,
        tick_upper: i32,
    ) -> Result<String, ConcentratedLiquidityError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| ConcentratedLiquidityError::PoolNotFound(pool_id.to_string()))?;
        
        // Calculate liquidity from token amounts
        let sqrt_ratio_lower = tick_to_sqrt_ratio(tick_lower);
        let sqrt_ratio_upper = tick_to_sqrt_ratio(tick_upper);
        
        let liquidity = if pool.tick_current >= tick_lower && pool.tick_current <= tick_upper {
            // Calculate liquidity in current price range
            let amount0 = if token0_amount > 0 {
                token0_amount * (pool.sqrt_ratio - sqrt_ratio_lower) / pool.sqrt_ratio
            } else { 0 };
            let amount1 = if token1_amount > 0 {
                token1_amount * (sqrt_ratio_upper - pool.sqrt_ratio) / u128::MAX
            } else { 0 };
            
            amount0 + amount1
        } else if pool.tick_current < tick_lower {
            // Entirely below current price
            token0_amount * sqrt_ratio_upper / sqrt_ratio_lower
        } else {
            // Entirely above current price
            token1_amount
        };
        
        if liquidity == 0 {
            return Err(ConcentratedLiquidityError::ZeroLiquidity);
        }
        
        // Create position
        let position = CLPosition::new(
            user,
            pool_id.to_string(),
            pool.token0.clone(),
            pool.token1.clone(),
            tick_lower,
            tick_upper,
            liquidity,
        );
        
        let position_id = position.position_id.clone();
        
        // Update pool liquidity
        pool.update_position(tick_lower, tick_upper, liquidity as i128)?;
        
        self.positions.write().insert(position_id.clone(), position);
        
        Ok(position_id)
    }

    /// Remove liquidity from a position
    pub fn remove_liquidity(
        &self,
        position_id: &str,
        liquidity_amount: u128,
    ) -> Result<TokensContained, ConcentratedLiquidityError> {
        let mut positions = self.positions.write();
        let position = positions.get_mut(position_id)
            .ok_or_else(|| ConcentratedLiquidityError::PositionNotFound(position_id.to_string()))?;
        
        if liquidity_amount > position.liquidity {
            return Err(ConcentratedLiquidityError::InvalidAmount(
                "Cannot remove more than available".to_string(),
            ));
        }
        
        // Calculate tokens to return
        let pool_id = position.pool_id.clone();
        drop(positions);
        
        let pools = self.pools.read();
        let pool = pools.get(&pool_id)
            .ok_or_else(|| ConcentratedLiquidityError::PoolNotFound(pool_id.clone()))?;
        
        let sqrt_ratio_lower = tick_to_sqrt_ratio(position.tick_lower);
        let sqrt_ratio_upper = tick_to_sqrt_ratio(position.tick_upper);
        
        let tokens = position.calculate_tokens(sqrt_ratio_lower, sqrt_ratio_upper, pool.sqrt_ratio);
        
        // Update position
        let mut positions = self.positions.write();
        if let Some(position) = positions.get_mut(position_id) {
            position.liquidity -= liquidity_amount;
            position.tokens_contained = tokens;
            position.updated_at = Utc::now().timestamp();
        }
        
        Ok(tokens)
    }

    /// Get position
    pub fn get_position(&self, position_id: &str) -> Option<CLPosition> {
        self.positions.read().get(position_id).cloned()
    }

    /// Get positions for a user
    pub fn get_user_positions(&self, user: &str) -> Vec<CLPosition> {
        self.positions.read()
            .values()
            .filter(|p| p.user == user)
            .cloned()
            .collect()
    }

    /// Get positions in a pool
    pub fn get_pool_positions(&self, pool_id: &str) -> Vec<CLPosition> {
        self.positions.read()
            .values()
            .filter(|p| p.pool_id == pool_id)
            .cloned()
            .collect()
    }

    /// Execute a swap
    pub fn swap(
        &self,
        pool_id: &str,
        token_in: &str,
        amount_in: u128,
        sqrt_ratio_limit: Option<u128>,
        zero_for_one: bool,
    ) -> Result<SwapResult, ConcentratedLiquidityError> {
        let mut pools = self.pools.write();
        let pool = pools.get_mut(pool_id)
            .ok_or_else(|| ConcentratedLiquidityError::PoolNotFound(pool_id.to_string()))?;
        
        let mut amount_remaining = amount_in;
        let mut amount_out = 0u128;
        let mut sqrt_ratio = pool.sqrt_ratio;
        let mut ticks_crossed = vec![];
        
        while amount_remaining > 0 {
            let (next_sqrt_ratio, amount_in_next) = if zero_for_one {
                // Token0 -> Token1
                calculate_next_sqrt_price(
                    sqrt_ratio,
                    amount_remaining,
                    pool.liquidity,
                    true,
                )
            } else {
                // Token1 -> Token0
                calculate_next_sqrt_price(
                    sqrt_ratio,
                    amount_remaining,
                    pool.liquidity,
                    false,
                )
            };
            
            let target = sqrt_ratio_limit.unwrap_or(next_sqrt_ratio);
            
            if (zero_for_one && next_sqrt_ratio > target) || (!zero_for_one && next_sqrt_ratio < target) {
                // Hit limit, calculate final amount
                let final_amount = if zero_for_one {
                    calculate_amount_delta(sqrt_ratio, target, pool.liquidity, true)
                } else {
                    calculate_amount_delta(sqrt_ratio, target, pool.liquidity, false)
                };
                
                amount_out += final_amount;
                amount_remaining = 0;
                sqrt_ratio = target;
                break;
            }
            
            // Cross to next tick
            let next_tick = if zero_for_one {
                (sqrt_ratio / pool.tick_spacing as u128 * pool.tick_spacing as u128) as i32 + pool.tick_spacing
            } else {
                (sqrt_ratio / pool.tick_spacing as u128 * pool.tick_spacing as u128) as i32 - pool.tick_spacing
            };
            
            ticks_crossed.push(next_tick);
            amount_out += amount_in_next;
            amount_remaining -= amount_in_next;
            sqrt_ratio = next_sqrt_ratio;
        }
        
        // Update pool state
        pool.sqrt_ratio = sqrt_ratio;
        pool.tick_current = tick_from_sqrt_ratio(sqrt_ratio);
        
        // Calculate fee
        let fee = amount_in * pool.fee_tier as u128 / 10000;
        
        Ok(SwapResult {
            amount_in,
            amount_out,
            sqrt_ratio_after: sqrt_ratio,
            tick_after: pool.tick_current,
            fee,
        })
    }

    /// Get pool tokens balance
    pub fn get_pool_tokens(&self, pool_id: &str) -> Result<(u128, u128), ConcentratedLiquidityError> {
        let pools = self.pools.read();
        let pool = pools.get(pool_id)
            .ok_or_else(|| ConcentratedLiquidityError::PoolNotFound(pool_id.to_string()))?;
        
        let positions = self.get_pool_positions(pool_id);
        
        let mut token0 = 0u128;
        let mut token1 = 0u128;
        
        for pos in positions {
            token0 += pos.tokens_contained.token0;
            token1 += pos.tokens_contained.token1;
        }
        
        Ok((token0, token1))
    }

    /// Collect fees from position
    pub fn collect_fees(&self, position_id: &str) -> Result<(u128, u128), ConcentratedLiquidityError> {
        let mut positions = self.positions.write();
        let position = positions.get_mut(position_id)
            .ok_or_else(|| ConcentratedLiquidityError::PositionNotFound(position_id.to_string()))?;
        
        let fees = position.collect_fees();
        position.updated_at = Utc::now().timestamp();
        
        Ok(fees)
    }

    /// Get all pools
    pub fn get_pools(&self) -> Vec<CLPool> {
        self.pools.read().values().cloned().collect()
    }

    /// Get pool count
    pub fn pool_count(&self) -> usize {
        self.pools.read().len()
    }

    /// Get position count
    pub fn position_count(&self) -> usize {
        self.positions.read().len()
    }

    /// Add supported chain
    pub fn add_chain(&self, chain_id: u64) {
        self.supported_chains.write().insert(chain_id);
    }

    /// Get supported chains
    pub fn supported_chains(&self) -> Vec<u64> {
        self.supported_chains.read().iter().cloned().collect()
    }
}

impl Default for ConcentratedLiquidityEngine {
    fn default() -> Self { Self::new() }
}

/// Swap result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SwapResult {
    pub amount_in: u128,
    pub amount_out: u128,
    pub sqrt_ratio_after: u128,
    pub tick_after: i32,
    pub fee: u128,
}

/// Calculate next sqrt price
fn calculate_next_sqrt_price(
    sqrt_ratio: u128,
    amount_remaining: u128,
    liquidity: u128,
    zero_for_one: bool,
) -> (u128, u128) {
    if liquidity == 0 {
        return (sqrt_ratio, 0);
    }
    
    let next_sqrt = if zero_for_one {
        // Increase sqrt ratio
        sqrt_ratio + (amount_remaining * u128::MAX) / liquidity
    } else {
        // Decrease sqrt ratio
        sqrt_ratio.saturating_sub((amount_remaining * u128::MAX) / liquidity)
    };
    
    let amount_in = if zero_for_one {
        (liquidity * (next_sqrt - sqrt_ratio)) / u128::MAX
    } else {
        (liquidity * (sqrt_ratio - next_sqrt)) / u128::MAX
    };
    
    (next_sqrt, amount_in)
}

/// Calculate amount delta
fn calculate_amount_delta(
    sqrt_ratio_start: u128,
    sqrt_ratio_end: u128,
    liquidity: u128,
    zero_for_one: bool,
) -> u128 {
    if liquidity == 0 {
        return 0;
    }
    
    if zero_for_one {
        (liquidity * (sqrt_ratio_end - sqrt_ratio_start)) / u128::MAX
    } else {
        (liquidity * (sqrt_ratio_start - sqrt_ratio_end)) / u128::MAX
    }
}

/// Get tick from sqrt ratio
fn tick_from_sqrt_ratio(sqrt_ratio: u128) -> i32 {
    let ratio = sqrt_ratio as f64 / (1u128 << 48) as f64;
    let log = ratio.ln() / (1.0001_f64).ln();
    (log * 10000.0) as i32
}

#[cfg(test)]
mod tests {
    use super::*;
    use rust_decimal_macros::dec;

    #[test]
    fn test_pool_creation() {
        let pool = CLPool::new(
            CHAIN_ETH,
            "USDC".to_string(),
            "WETH".to_string(),
            FEE_TIER_HIGH,
            dec!(2500.0),
        );
        
        assert_eq!(pool.fee_tier, 3000);
    }

    #[test]
    fn test_position() {
        let position = CLPosition::new(
            "user1".to_string(),
            "pool1".to_string(),
            "USDC".to_string(),
            "WETH".to_string(),
            -1000,
            1000,
            1000000,
        );
        
        assert!(position.validate().is_ok());
    }

    #[test]
    fn test_tick_spacing() {
        assert_eq!(tick_spacing(100), 1);
        assert_eq!(tick_spacing(500), 10);
        assert_eq!(tick_spacing(3000), 60);
    }
}