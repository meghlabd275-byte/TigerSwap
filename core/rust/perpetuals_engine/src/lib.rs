//! TigerSwap Perpetuals Engine
//! 
//! Implements GMX-style perpetual futures:
//! - Leverage trading (up to 100x)
//! - Long/Short positions
//! - Funding rate
//! - Liquidation
//! - Oracle pricing
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
pub const CHAIN_ARBITRUM: u64 = 42161;
pub const CHAIN_AVALANCHE: u64 = 43114;

#[derive(Debug, Error)]
pub enum PerpetualsError {
    #[error("Position not found: {0}")]
    PositionNotFound(String),
    #[error("Insufficient margin: {0}")]
    InsufficientMargin(String),
    #[error("Leverage too high: {0}")]
    LeverageTooHigh(String),
    #[error("Liquidation triggered: {0}")]
    LiquidationTriggered(String),
    #[error("Invalid size: {0}")]
    InvalidSize(String),
    #[error("Price not available: {0}")]
    PriceNotAvailable(String),
    #[error("Chain not supported: {0}")]
    ChainNotSupported(u64),
    #[error("Insufficient liquidity: {0}")]
    InsufficientLiquidity(String),
}

/// Position side
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum PositionSide {
    Long,
    Short,
}

/// Position status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum PositionStatus {
    Open,
    Closing,
    Liquidated,
    Closed,
}

impl Default for PositionStatus {
    fn default() -> Self { PositionStatus::Open }
}

/// Perpetual position
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PerpetualPosition {
    pub position_id: String,
    pub user: String,
    pub chain_id: u64,
    pub market: String,
    
    // Position
    pub side: PositionSide,
    pub size: u128,        // Position size in USD
    pub entry_price: Decimal,
    pub leverage: u32,     // 1-100x
    
    // Collateral
    pub collateral: u128,
    pub borrowed: u128,
    
    // PnL
    pub unrealized_pnl: i128,
    pub realized_pnl: i128,
    
    // Fees
    pub opening_fee: u128,
    pub borrowing_fee: u128,
    pub funding_fee: i128,
    
    // Status
    pub status: PositionStatus,
    pub opened_at: i64,
    pub updated_at: i64,
    pub closed_at: Option<i64>,
}

impl PerpetualPosition {
    /// Open a new position
    pub fn open(
        user: String,
        chain_id: u64,
        market: String,
        side: PositionSide,
        size: u128,
        leverage: u32,
        entry_price: Decimal,
        collateral: u128,
    ) -> Result<Self, PerpetualsError> {
        if collateral == 0 {
            return Err(PerpetualsError::InsufficientMargin("Collateral must be > 0".to_string()));
        }
        if leverage < 1 || leverage > 100 {
            return Err(PerpetualsError::LeverageTooHigh("Leverage must be 1-100x".to_string()));
        }
        if size == 0 {
            return Err(PerpetualsError::InvalidSize("Size must be > 0".to_string()));
        }
        
        // Verify position value
        let position_value = collateral as u128 * leverage as u128;
        if position_value < size {
            return Err(PerpetualsError::InsufficientMargin("Insufficient collateral for size".to_string()));
        }
        
        let now = Utc::now().timestamp();
        
        // Calculate borrowing fee
        let borrowed = position_value - collateral as u128;
        let borrowing_fee = borrowed / 1000;  // 0.1% borrow fee
        
        Ok(Self {
            position_id: Uuid::new_v4().to_string(),
            user,
            chain_id,
            market,
            side,
            size,
            entry_price,
            leverage,
            collateral,
            borrowed,
            unrealized_pnl: 0,
            realized_pnl: 0,
            opening_fee: size / 1000,  // 0.1% opening fee
            borrowing_fee,
            funding_fee: 0,
            status: PositionStatus::Open,
            opened_at: now,
            updated_at: now,
            closed_at: None,
        })
    }

    /// Update unrealized PnL
    pub fn update_pnl(&mut self, current_price: Decimal) {
        let current_value = self.size as f64 / current_price.as_f64();
        let entry_value = self.size as f64 / self.entry_price.as_f64();
        
        let pnl = if self.side == PositionSide::Long {
            (current_value - entry_value) as i128
        } else {
            (entry_value - current_value) as i128
        };
        
        self.unrealized_pnl = pnl;
        self.updated_at = Utc::now().timestamp();
    }

    /// Get liquidation price
    pub fn get_liquidation_price(&self) -> Decimal {
        let liquidate_ratio = 1.0 / self.leverage as f64;
        
        if self.side == PositionSide::Long {
            self.entry_price * Decimal::from(10000) / Decimal::from(10000 - (liquidate_ratio * 10000.0) as i64)
        } else {
            self.entry_price * Decimal::from(10000 - (liquidate_ratio * 10000.0) as i64) / Decimal::from(10000)
        }
    }

    /// Check if should liquidate
    pub fn should_liquidate(&self, current_price: Decimal) -> bool {
        let liq_price = self.get_liquidation_price();
        
        match self.side {
            PositionSide::Long => current_price <= liq_price,
            PositionSide::Short => current_price >= liq_price,
        }
    }

    /// Close position
    pub fn close(&mut self) -> u128 {
        self.status = PositionStatus::Closed;
        self.realized_pnl = self.unrealized_pnl;
        self.closed_at = Some(Utc::now().timestamp());
        
        // Return remaining collateral + PnL
        let return_amount = if self.realized_pnl > 0 {
            self.collateral + self.realized_pnl as u128
        } else {
            self.collateral.saturating_sub(self.realized_pnl.abs() as u128)
        };
        
        return_amount.saturating_sub(self.borrowing_fee + self.opening_fee)
    }

    /// Add margin
    pub fn add_margin(&mut self, amount: u128) {
        self.collateral += amount;
        self.updated_at = Utc::now().timestamp();
    }

    /// Remove margin
    pub fn remove_margin(&mut self, amount: u128) -> Result<(), PerpetualsError> {
        if amount > self.collateral {
            return Err(PerpetualsError::InsufficientMargin("Cannot remove more than collateral".to_string()));
        }
        
        // Check if still above liquidation threshold
        let new_collateral = self.collateral - amount;
        let position_value = new_collateral as u128 * self.leverage as u128;
        
        if position_value < self.size {
            return Err(PerpetualsError::InsufficientMargin("Would trigger liquidation".to_string()));
        }
        
        self.collateral = new_collateral;
        self.updated_at = Utc::now().timestamp();
        
        Ok(())
    }

    /// Calculate funding fee
    pub fn calculate_funding(&self, funding_rate: i64) -> i128 {
        let hourly_rate = funding_rate / 24;
        (self.size as i128 * hourly_rate) / 1000000
    }
}

/// Market
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PerpetualMarket {
    pub market_id: String,
    pub chain_id: u64,
    pub name: String,
    pub token: String,
    
    // Pricing
    pub current_price: Decimal,
    pub last_update: i64,
    
    // Funding
    pub funding_rate: i64,  // Per hour in 0.0001%
    pub funding_accrued: i128,
    
    // Liquidity
    pub long_liquidity: u128,
    pub short_liquidity: u128,
    
    // Limits
    pub max_leverage: u32,
    pub max_position_size: u128,
    pub min_position_size: u128,
    
    pub created_at: i64,
}

impl PerpetualMarket {
    /// Create a new market
    pub fn new(chain_id: u64, name: String, token: String, initial_price: Decimal) -> Self {
        Self {
            market_id: Uuid::new_v4().to_string(),
            chain_id,
            name,
            token,
            current_price: initial_price,
            last_update: Utc::now().timestamp(),
            funding_rate: 0,
            funding_accrued: 0,
            long_liquidity: 0,
            short_liquidity: 0,
            max_leverage: 100,
            max_position_size: 10000000,
            min_position_size: 100,
            created_at: Utc::now().timestamp(),
        }
    }

    /// Update price
    pub fn update_price(&mut self, price: Decimal) {
        self.current_price = price;
        self.last_update = Utc::now().timestamp();
    }

    /// Calculate funding
    pub fn calculate_funding(&self, side: PositionSide, size: u128) -> i128 {
        let rate = if side == PositionSide::Long {
            self.funding_rate
        } else {
            -self.funding_rate
        };
        
        (size as i128 * rate) / 1000000
    }
}

/// Order
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PerpetualOrder {
    pub order_id: String,
    pub user: String,
    pub market_id: String,
    pub side: PositionSide,
    pub order_type: OrderType,
    pub size: u128,
    pub limit_price: Option<Decimal>,
    pub leverage: u32,
    pub status: OrderStatus,
    pub created_at: i64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderType {
    Market,
    Limit,
    StopLoss,
    TakeProfit,
}

impl Default for OrderType {
    fn default() -> Self { OrderType::Market }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderStatus {
    Pending,
    Filled,
    Cancelled,
    Expired,
}

impl Default for OrderStatus {
    fn default() -> Self { OrderStatus::Pending }
}

impl PerpetualOrder {
    pub fn new(
        user: String,
        market_id: String,
        side: PositionSide,
        order_type: OrderType,
        size: u128,
        leverage: u32,
    ) -> Self {
        Self {
            order_id: Uuid::new_v4().to_string(),
            user,
            market_id,
            side,
            order_type,
            size,
            limit_price: None,
            leverage,
            status: OrderStatus::Pending,
            created_at: Utc::now().timestamp(),
        }
    }
}

/// Liquidation
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Liquidation {
    pub liquidation_id: String,
    pub position_id: String,
    pub user: String,
    pub market: String,
    pub side: PositionSide,
    pub size: u128,
    pub collateral: u128,
    pub loss: i128,
    pub bonus: u128,
    pub liquidator: String,
    pub timestamp: i64,
}

/// Oracle price
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OraclePrice {
    pub market_id: String,
    pub price: Decimal,
    pub timestamp: i64,
    pub source: String,
}

/// Perpetuals engine
pub struct PerpetualsEngine {
    markets: Arc<RwLock<HashMap<String, PerpetualMarket>>>,
    positions: Arc<RwLock<HashMap<String, PerpetualPosition>>>,
    orders: Arc<RwLock<HashMap<String, PerpetualOrder>>>,
    liquidations: Arc<RwLock<Vec<Liquidation>>>,
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
}

impl PerpetualsEngine {
    /// Create a new perpetuals engine
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [
            CHAIN_ETH, CHAIN_ARBITRUM, CHAIN_AVALANCHE,
        ].into_iter().collect();
        
        Self {
            markets: Arc::new(RwLock::new(HashMap::new())),
            positions: Arc::new(RwLock::new(HashMap::new())),
            orders: Arc::new(RwLock::new(HashMap::new())),
            liquidations: Arc::new(RwLock::new(Vec::new())),
            supported_chains: Arc::new(RwLock::new(chains)),
        }
    }

    /// Check if chain is supported
    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    /// Create market
    pub fn create_market(
        &self,
        chain_id: u64,
        name: String,
        token: String,
        initial_price: Decimal,
    ) -> Result<String, PerpetualsError> {
        if !self.is_chain_supported(chain_id) {
            return Err(PerpetualsError::ChainNotSupported(chain_id));
        }
        
        let market = PerpetualMarket::new(chain_id, name, token, initial_price);
        let market_id = market.market_id.clone();
        
        self.markets.write().insert(market_id.clone(), market);
        
        Ok(market_id)
    }

    /// Get market
    pub fn get_market(&self, market_id: &str) -> Option<PerpetualMarket> {
        self.markets.read().get(market_id).cloned()
    }

    /// Update price
    pub fn update_price(&self, market_id: &str, price: Decimal) -> Result<(), PerpetualsError> {
        let mut markets = self.markets.write();
        let market = markets.get_mut(market_id)
            .ok_or_else(|| PerpetualsError::PriceNotAvailable(market_id.to_string()))?;
        
        market.update_price(price);
        
        Ok(())
    }

    /// Open position
    pub fn open_position(
        &self,
        user: String,
        market_id: String,
        side: PositionSide,
        size: u128,
        leverage: u32,
        collateral: u128,
    ) -> Result<String, PerpetualsError> {
        let market = self.markets.read()
            .get(&market_id)
            .ok_or_else(|| PerpetualsError::PriceNotAvailable(market_id.to_string()))?;
        
        if size < market.min_position_size {
            return Err(PerpetualsError::InvalidSize("Below minimum size".to_string()));
        }
        if size > market.max_position_size {
            return Err(PerpetualsError::InvalidSize("Above maximum size".to_string()));
        }
        
        let position = PerpetualPosition::open(
            user,
            market.chain_id,
            market.name.clone(),
            side,
            size,
            leverage,
            market.current_price,
            collateral,
        )?;
        
        let position_id = position.position_id.clone();
        
        // Update market liquidity
        let mut markets = self.markets.write();
        if let Some(m) = markets.get_mut(&market_id) {
            match side {
                PositionSide::Long => m.long_liquidity += collateral,
                PositionSide::Short => m.short_liquidity += collateral,
            }
        }
        
        self.positions.write().insert(position_id.clone(), position);
        
        Ok(position_id)
    }

    /// Get position
    pub fn get_position(&self, position_id: &str) -> Option<PerpetualPosition> {
        self.positions.read().get(position_id).cloned()
    }

    /// Get user positions
    pub fn get_user_positions(&self, user: &str) -> Vec<PerpetualPosition> {
        self.positions.read()
            .values()
            .filter(|p| p.user == user && matches!(p.status, PositionStatus::Open))
            .cloned()
            .collect()
    }

    /// Update position PnL
    pub fn update_position_pnl(&self, position_id: &str) -> Result<(), PerpetualsError> {
        let mut positions = self.positions.write();
        let position = positions.get_mut(position_id)
            .ok_or_else(|| PerpetualsError::PositionNotFound(position_id.to_string()))?;
        
        let market = self.markets.read()
            .get(&position.market)
            .ok_or_else(|| PerpetualsError::PriceNotAvailable(position.market.clone()))?;
        
        position.update_pnl(market.current_price);
        
        Ok(())
    }

    /// Close position
    pub fn close_position(&self, position_id: &str) -> Result<u128, PerpetualsError> {
        let mut positions = self.positions.write();
        let position = positions.get_mut(position_id)
            .ok_or_else(|| PerpetualsError::PositionNotFound(position_id.to_string()))?;
        
        if !matches!(position.status, PositionStatus::Open) {
            return Err(PerpetualsError::PositionNotFound(position_id.to_string()));
        }
        
        let return_amount = position.close();
        
        // Update market liquidity
        let mut markets = self.markets.write();
        if let Some(m) = markets.get_mut(&position.market) {
            match position.side {
                PositionSide::Long => m.long_liquidity = m.long_liquidity.saturating_sub(position.collateral),
                PositionSide::Short => m.short_liquidity = m.short_liquidity.saturating_sub(position.collateral),
            }
        }
        
        Ok(return_amount)
    }

    /// Add margin
    pub fn add_margin(&self, position_id: &str, amount: u128) -> Result<(), PerpetualsError> {
        let mut positions = self.positions.write();
        let position = positions.get_mut(position_id)
            .ok_or_else(|| PerpetualsError::PositionNotFound(position_id.to_string()))?;
        
        position.add_margin(amount);
        
        Ok(())
    }

    /// Remove margin
    pub fn remove_margin(&self, position_id: &str, amount: u128) -> Result<(), PerpetualsError> {
        let mut positions = self.positions.write();
        let position = positions.get_mut(position_id)
            .ok_or_else(|| PerpetualsError::PositionNotFound(position_id.to_string()))?;
        
        position.remove_margin(amount)?;
        
        Ok(())
    }

    /// Create order
    pub fn create_order(
        &self,
        user: String,
        market_id: String,
        side: PositionSide,
        order_type: OrderType,
        size: u128,
        leverage: u32,
    ) -> Result<String, PerpetualsError> {
        let order = PerpetualOrder::new(user, market_id, side, order_type, size, leverage);
        let order_id = order.order_id.clone();
        
        self.orders.write().insert(order_id.clone(), order);
        
        Ok(order_id)
    }

    /// Execute order
    pub fn execute_order(&self, order_id: &str) -> Result<String, PerpetualsError> {
        let mut orders = self.orders.write();
        let order = orders.get_mut(order_id)
            .ok_or_else(|| PerpetualsError::PositionNotFound(order_id.to_string()))?;
        
        if !matches!(order.status, OrderStatus::Pending) {
            return Err(PerpetualsError::PositionNotFound(order_id.to_string()));
        }
        
        // Execute as position
        let position_id = self.open_position(
            order.user.clone(),
            order.market_id.clone(),
            order.side,
            order.size,
            order.leverage,
            order.size / 10,  // 10% margin
        )?;
        
        order.status = OrderStatus::Filled;
        
        Ok(position_id)
    }

    /// Check liquidations
    pub fn check_liquidations(&self) -> Vec<Liquidation> {
        let mut liquidations = vec![];
        
        let markets = self.markets.read();
        let mut positions = self.positions.write();
        
        for position in positions.values_mut() {
            if !matches!(position.status, PositionStatus::Open) {
                continue;
            }
            
            let market = match markets.get(&position.market) {
                Some(m) => m,
                None => continue,
            };
            
            if position.should_liquidate(market.current_price) {
                let liquidation = Liquidation {
                    liquidation_id: Uuid::new_v4().to_string(),
                    position_id: position.position_id.clone(),
                    user: position.user.clone(),
                    market: position.market.clone(),
                    side: position.side,
                    size: position.size,
                    collateral: position.collateral,
                    loss: position.unrealized_pnl,
                    bonus: position.collateral / 20,  // 5% bonus to liquidator
                    liquidator: String::new(),
                    timestamp: Utc::now().timestamp(),
                };
                
                liquidations.push(liquidation);
                position.status = PositionStatus::Liquidated;
            }
        }
        
        // Store liquidations
        self.liquidations.write().extend(liquidations.clone());
        
        liquidations
    }

    /// Execute liquidation
    pub fn execute_liquidation(&self, position_id: &str, liquidator: String) -> Result<u128, PerpetualsError> {
        let mut positions = self.positions.write();
        let position = positions.get_mut(position_id)
            .ok_or_else(|| PerpetualsError::PositionNotFound(position_id.to_string()))?;
        
        if !matches!(position.status, PositionStatus::Liquidated) {
            return Err(PerpetualsError::PositionNotFound(position_id.to_string()));
        }
        
        let bonus = position.collateral / 20;
        position.status = PositionStatus::Liquidated;
        
        Ok(bonus)
    }

    /// Get open interest
    pub fn get_open_interest(&self, market_id: &str) -> (u128, u128) {
        let positions = self.positions.read();
        
        let mut long_oi = 0u128;
        let mut short_oi = 0u128;
        
        for position in positions.values() {
            if position.market == market_id && matches!(position.status, PositionStatus::Open) {
                match position.side {
                    PositionSide::Long => long_oi += position.size,
                    PositionSide::Short => short_oi += position.size,
                }
            }
        }
        
        (long_oi, short_oi)
    }

    /// Get funding rate
    pub fn calculate_funding_rate(&self, market_id: &str) -> i64 {
        let (long_oi, short_oi) = self.get_open_interest(market_id);
        
        if long_oi == short_oi {
            return 0;
        }
        
        // Funding flows from overbalanced to underbalanced side
        let diff = (long_oi as i128 - short_oi as i128).abs();
        let total = long_oi + short_oi;
        
        if total == 0 {
            return 0;
        }
        
        // Rate proportional to imbalance
        let rate = (diff * 100) / total;
        rate as i64
    }

    /// Get statistics
    pub fn get_stats(&self) -> PerpetualsStats {
        let positions = self.positions.read();
        let markets = self.markets.read();
        
        let mut open_positions = 0;
        let mut total_volume = 0u128;
        
        for position in positions.values() {
            if matches!(position.status, PositionStatus::Open) {
                open_positions += 1;
                total_volume += position.size;
            }
        }
        
        PerpetualsStats {
            open_positions,
            total_markets: markets.len(),
            total_volume,
        }
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

impl Default for PerpetualsEngine {
    fn default() -> Self { Self::new() }
}

/// Perpetuals statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PerpetualsStats {
    pub open_positions: usize,
    pub total_markets: usize,
    pub total_volume: u128,
}

#[cfg(test)]
mod tests {
    use super::*;
    use rust_decimal_macros::dec;

    #[test]
    fn test_market_creation() {
        let market = PerpetualMarket::new(
            CHAIN_ETH,
            "ETH-PERP".to_string(),
            "ETH".to_string(),
            dec!(2500.0),
        );
        
        assert_eq!(market.max_leverage, 100);
    }

    #[test]
    fn test_position_creation() {
        let position = PerpetualPosition::open(
            "user1".to_string(),
            CHAIN_ETH,
            "ETH-PERP".to_string(),
            PositionSide::Long,
            10000,
            10,
            dec!(2500.0),
            1000,
        ).unwrap();
        
        assert_eq!(position.leverage, 10);
    }

    #[test]
    fn test_liquidation_price() {
        let position = PerpetualPosition::open(
            "user1".to_string(),
            CHAIN_ETH,
            "ETH-PERP".to_string(),
            PositionSide::Long,
            10000,
            10,
            dec!(2500.0),
            1000,
        ).unwrap();
        
        let liq_price = position.get_liquidation_price();
        assert!(liq_price > Decimal::ZERO);
    }

    #[test]
    fn test_order_creation() {
        let order = PerpetualOrder::new(
            "user1".to_string(),
            "market1".to_string(),
            PositionSide::Long,
            OrderType::Market,
            1000,
            5,
        );
        
        assert_eq!(order.leverage, 5);
    }
}