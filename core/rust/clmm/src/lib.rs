//! TigerSwap Concentrated Liquidity Market Maker (CLMM)
//! 
//! Implementation of Uniswap V3-style concentrated liquidity:
//! - Range orders
//! - Tick-based pricing
//! - Position management
//! - Fee calculation
//!
//! Uses Rust for high performance

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use serde::{Deserialize, Serialize};
use uint::construct_uint;

construct_uint! {
    pub struct U256(4);
}

// ==================== TICK ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Tick {
    pub index: i32,
    pub liquidityGross: U256,
    pub liquidityNet: U256,
    pub feeGrowthOutside0: U256,
    pub feeGrowthOutside1: U256,
    pub initialized: bool,
}

impl Tick {
    pub fn new(index: i32) -> Self {
        Self {
            index,
            liquidityGross: U256::zero(),
            liquidityNet: U256::zero(),
            feeGrowthOutside0: U256::zero(),
            feeGrowthOutside1: U256::zero(),
            initialized: false,
        }
    }
    
    pub fn add_liquidity(&mut self, amount: U256) {
        self.liquidityGross = self.liquidityGross + amount;
        self.liquidityNet = self.liquidityNet + amount;
        self.initialized = true;
    }
    
    pub fn remove_liquidity(&mut self, amount: U256) -> Result<(), CLMMError> {
        if amount > self.liquidityGross {
            return Err(CLMMError::InsufficientLiquidity);
        }
        self.liquidityGross = self.liquidityGross - amount;
        self.liquidityNet = self.liquidityNet - amount;
        Ok(())
    }
}

// ==================== POSITION ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Position {
    pub id: [u8; 32],
    pub owner: [u8; 20],
    pub token0: [u8; 20],
    pub token1: [u8; 20],
    pub tickLower: i32,
    pub tickUpper: i32,
    pub liquidity: U256,
    pub feeGrowthInside0: U256,
    pub feeGrowthInside1: U256,
    pub tokensOwed0: U256,
    pub tokensOwed1: U256,
}

impl Position {
    pub fn new(
        owner: [u8; 20],
        token0: [u8; 20],
        token1: [u8; 20],
        tickLower: i32,
        tickUpper: i32,
    ) -> Self {
        let mut id = [0u8; 32];
        id[..20].copy_from_slice(&owner);
        id[20..].copy_from_slice(&(tickLower as u32).to_le_bytes());
        
        Self {
            id,
            owner,
            token0,
            token1,
            tickLower,
            tickUpper,
            liquidity: U256::zero(),
            feeGrowthInside0: U256::zero(),
            feeGrowthInside1: U256::zero(),
            tokensOwed0: U256::zero(),
            tokensOwed1: U256::zero(),
        }
    }
    
    pub fn update(&mut self, deltaLiquidity: U256) -> Result<(), CLMMError> {
        if deltaLiquidity > U256::zero() {
            self.liquidity = self.liquidity + deltaLiquidity;
        } else {
            let delta = U256::zero() - deltaLiquidity;
            if delta > self.liquidity {
                return Err(CLMMError::InsufficientLiquidity);
            }
            self.liquidity = self.liquidity - delta;
        }
        Ok(())
    }
    
    pub fn collect_fees(&mut self, feeGrowthInside0: U256, feeGrowthInside1: U256) -> (U256, U256) {
        let fees0 = (feeGrowthInside0 - self.feeGrowthInside0) * self.liquidity / U256::from(10).pow(24);
        let fees1 = (feeGrowthInside1 - self.feeGrowthInside1) * self.liquidity / U256::from(10).pow(24);
        
        self.tokensOwed0 = self.tokensOwed0 + fees0;
        self.tokensOwed1 = self.tokensOwed1 + fees1;
        
        self.feeGrowthInside0 = feeGrowthInside0;
        self.feeGrowthInside1 = feeGrowthInside1;
        
        (fees0, fees1)
    }
}

// ==================== TICK MAP ====================

pub struct TickMap {
    ticks: HashMap<i32, Tick>,
}

impl TickMap {
    pub fn new() -> Self {
        Self { ticks: HashMap::new() }
    }
    
    pub fn get_or_create(&mut self, index: i32) -> &mut Tick {
        self.ticks.entry(index).or_insert_with(|| Tick::new(index))
    }
    
    pub fn get(&self, index: i32) -> Option<&Tick> {
        self.ticks.get(&index)
    }
    
    pub fn get_mut(&mut self, index: i32) -> Option<&mut Tick> {
        self.ticks.get_mut(&index)
    }
    
    pub fn get_neighbor_ticks(&self, currentTick: i32) -> (Option<&Tick>, Option<&Tick>) {
        let lower = self.ticks.get(&(currentTick - 1));
        let upper = self.ticks.get(&(currentTick + 1));
        (lower, upper)
    }
}

// ==================== POOL ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Pool {
    pub token0: [u8; 20],
    pub token1: [u8; 20],
    pub fee: u24,
    pub tickSpacing: i32,
    pub sqrtPriceX96: U256,
    pub tick: i32,
    pub liquidity: U256,
    pub feeGrowthGlobal0: U256,
    pub feeGrowthGlobal1: U256,
    pub tickMap: TickMap,
    pub positions: HashMap<[u8; 32], Position>,
}

impl Pool {
    pub fn new(
        token0: [u8; 20],
        token1: [u8; 20],
        fee: u24,
        tickSpacing: i32,
    ) -> Self {
        Self {
            token0,
            token1,
            fee: fee as u24,
            tickSpacing,
            sqrtPriceX96: U256::from(79228162514264337593543950336u64), // 2^96
            tick: 0,
            liquidity: U256::zero(),
            feeGrowthGlobal0: U256::zero(),
            feeGrowthGlobal1: U256::zero(),
            tickMap: TickMap::new(),
            positions: HashMap::new(),
        }
    }
    
    // Calculate sqrt ratio from price
    pub fn price_to_sqrt_ratio(price: U256) -> U256 {
        // sqrt(price) * 2^96
        let sqrt_price = price.sqrt();
        sqrt_price * U256::from(2).pow(96)
    }
    
    // Calculate price from sqrt ratio
    pub fn sqrt_ratio_to_price(sqrtRatio: U256) -> U256 {
        (sqrtRatio / U256::from(2).pow(96)).pow(2)
    }
    
    // Get tick from sqrt price
    pub fn get_tick(&self, sqrtPriceX96: &U256) -> i32 {
        // Simplified tick calculation
        let price = self.sqrt_ratio_to_price(*sqrtPriceX96);
        // Use log2 approximation for tick
        let mut tick = 0;
        let mut p = price;
        while p > U256::from(10001) {
            p = p / U256::from(10000);
            tick += 1;
        }
        tick - 887272 // Offset to align with Uniswap
    }
    
    // Add liquidity to position
    pub fn add_liquidity(
        &mut self,
        owner: [u8; 20],
        tickLower: i32,
        tickUpper: i32,
        amount: U256,
    ) -> Result<U256, CLMMError> {
        // Validate ticks
        if tickLower >= tickUpper {
            return Err(CLMMError::InvalidTickRange);
        }
        
        if tickLower % self.tickSpacing != 0 || tickUpper % self.tickSpacing != 0 {
            return Err(CLMMError::InvalidTickSpacing);
        }
        
        // Get or create position
        let position_id = self.get_position_id(&owner, tickLower, tickUpper);
        
        let position = self.positions
            .entry(position_id)
            .or_insert_with(|| Position::new(
                owner, 
                self.token0, 
                self.token1, 
                tickLower, 
                tickUpper,
            ));
        
        // Update ticks
        let lower_tick = self.tickMap.get_or_create(tickLower);
        lower_tick.add_liquidity(amount);
        
        let upper_tick = self.tickMap.get_or_create(tickUpper);
        upper_tick.add_liquidity(amount);
        
        // Update position
        position.update(amount)?;
        
        // Update pool liquidity
        let current_tick = self.tick;
        if current_tick >= tickLower && current_tick <= tickUpper {
            self.liquidity = self.liquidity + amount;
        }
        
        Ok(amount)
    }
    
    fn get_position_id(&self, owner: &[u8; 20], tickLower: i32, tickUpper: i32) -> [u8; 32] {
        let mut id = [0u8; 32];
        id[..20].copy_from_slice(owner);
        id[20..24].copy_from_slice(&(tickLower as u32).to_le_bytes());
        id[24..28].copy_from_slice(&(tickUpper as u32).to_le_bytes());
        id
    }
    
    // Swap tokens
    pub fn swap(
        &mut self,
        zeroForOne: bool,
        amountSpecified: U256,
    ) -> Result<(U256, U256), CLMMError> {
        let mut amountRemaining = amountSpecified;
        let mut amountCalculated = U256::zero();
        
        while amountRemaining > U256::zero() {
            let (sqrtPriceNextX96, tickNext) = self.get_next_tick(zeroForOne)?;
            
            let amountIn = self.calculate_amount_in(
                sqrtPriceNextX96,
                zeroForOne,
                amountRemaining,
            )?;
            
            // Update fees
            let feeAmount = amountIn * U256::from(self.fee) / U256::from(1000000);
            let amountInMinusFee = amountIn - feeAmount;
            
            // Update price
            self.sqrtPriceX96 = sqrtPriceX96;
            self.tick = tickNext;
            
            if zeroForOne {
                amountCalculated = amountCalculated + amountInMinusFee;
                self.feeGrowthGlobal0 = self.feeGrowthGlobal0 + feeAmount;
            } else {
                amountCalculated = amountCalculated + amountInMinusFee;
                self.feeGrowthGlobal1 = self.feeGrowthGlobal1 + feeAmount;
            }
            
            amountRemaining = amountRemaining - amountIn;
        }
        
        Ok((amountSpecified - amountRemaining, amountCalculated))
    }
    
    fn get_next_tick(&self, zeroForOne: bool) -> Result<(U256, i32), CLMMError> {
        // Simplified - in production, would search tick map
        let tick = if zeroForOne { self.tick - self.tickSpacing } else { self.tick + self.tickSpacing };
        let sqrtPrice = Self::price_to_sqrt_ratio(U256::from(1)); // Simplified
        
        Ok((sqrtPrice, tick))
    }
    
    fn calculate_amount_in(
        &self,
        sqrtPriceNextX96: U256,
        zeroForOne: bool,
        amountRemaining: U256,
    ) -> Result<U256, CLMMError> {
        // Simplified amount calculation
        let deltaSqrt = sqrtPriceNextX96 - self.sqrtPriceX96;
        
        if zeroForOne {
            // Amount in = (L * deltaSqrt) / sqrtPrice
            let amountIn = self.liquidity * deltaSqrt / self.sqrtPriceX96;
            Ok(amountIn)
        } else {
            // Amount in = L * deltaSqrt
            let amountIn = self.liquidity * deltaSqrt;
            Ok(amountIn)
        }
    }
    
    // Collect fees from position
    pub fn collect_fees(&mut self, owner: [u8; 20], tickLower: i32, tickUpper: i32) -> Result<(U256, U256), CLMMError> {
        let position_id = self.get_position_id(&owner, tickLower, tickUpper);
        
        if let Some(position) = self.positions.get_mut(&position_id) {
            let (fees0, fees1) = position.collect_fees(
                self.feeGrowthGlobal0,
                self.feeGrowthGlobal1,
            );
            Ok((fees0, fees1))
        } else {
            Err(CLMMError::PositionNotFound)
        }
    }
}

// ==================== CLMM ERROR ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum CLMMError {
    InsufficientLiquidity,
    InvalidTickRange,
    InvalidTickSpacing,
    PositionNotFound,
    SwapFailed,
}

impl std::fmt::Display for CLMMError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            CLMMError::InsufficientLiquidity => write!(f, "Insufficient liquidity"),
            CLMMError::InvalidTickRange => write!(f, "Invalid tick range"),
            CLMMError::InvalidTickSpacing => write!(f, "Invalid tick spacing"),
            CLMMError::PositionNotFound => write!(f, "Position not found"),
            CLMMError::SwapFailed => write!(f, "Swap failed"),
        }
    }
}

// ==================== FACTORY ====================

pub struct CLMMFactory {
    pools: Arc<RwLock<HashMap<[u8; 32], Pool>>>,
}

impl CLMMFactory {
    pub fn new() -> Self {
        Self {
            pools: Arc::new(RwLock::new(HashMap::new())),
        }
    }
    
    pub async fn create_pool(
        &self,
        token0: [u8; 20],
        token1: [u8; 20],
        fee: u24,
    ) -> Result<[u8; 32], CLMMError> {
        let pool = Pool::new(token0, token1, fee, 10);
        
        let mut pool_id = [0u8; 32];
        pool_id[..20].copy_from_slice(&token0);
        pool_id[20..].copy_from_slice(&token1);
        
        self.pools.write().await.insert(pool_id, pool);
        
        Ok(pool_id)
    }
    
    pub async fn get_pool(&self, token0: [u8; 20], token1: [u8; 20]) -> Option<Pool> {
        let mut pool_id = [0u8; 32];
        pool_id[..20].copy_from_slice(&token0);
        pool_id[20..].copy_from_slice(&token1);
        
        let pools = self.pools.read().await;
        pools.get(&pool_id).cloned()
    }
}

// ==================== HELPER TRAITS ====================

trait U256Math {
    fn sqrt(&self) -> U256;
    fn pow(&self, exp: u32) -> U256;
}

impl U256Math for U256 {
    fn sqrt(&self) -> U256 {
        // Simplified square root
        let mut x = *self;
        let mut y = (x + U256::one()) / U256::from(2);
        while y < x {
            x = y;
            y = (x + *self / x) / U256::from(2);
        }
        x
    }
    
    fn pow(&self, exp: u32) -> U256 {
        let mut result = U256::one();
        let mut base = *self;
        let mut e = exp;
        
        while e > 0 {
            if e % 2 == 1 {
                result = result * base;
            }
            base = base * base;
            e = e / 2;
        }
        
        result
    }
}

// ==================== PUBLIC API ====================

pub mod api {
    use super::*;
    
    pub type CLMMPoolHandle = Arc<Pool>;
    pub type CLMMFactoryHandle = Arc<CLMMFactory>;
    
    pub fn create_factory() -> CLMMFactoryHandle {
        Arc::new(CLMMFactory::new())
    }
    
    pub fn create_pool(
        token0: [u8; 20],
        token1: [u8; 20],
        fee: u24,
    ) -> Pool {
        Pool::new(token0, token1, fee, 10)
    }
}

// ==================== TESTS ====================

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_position_creation() {
        let owner = [0u8; 20];
        let token0 = [1u8; 20];
        let token1 = [2u8; 20];
        
        let position = Position::new(owner, token0, token1, -1000, 1000);
        
        assert_eq!(position.liquidity, U256::zero());
    }
    
    #[test]
    fn test_pool_creation() {
        let token0 = [1u8; 20];
        let token1 = [2u8; 20];
        
        let pool = Pool::new(token0, token1, 3000, 10);
        
        assert_eq!(pool.fee, 3000);
        assert_eq!(pool.tickSpacing, 10);
    }
}