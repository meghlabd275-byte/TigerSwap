//! Liquidation Engine
//! 
//! Handles liquidation of undercollateralized positions in perpetual trading.
//! Supports cross-margin, portfolio-margin, and isolated margin modes.

use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum LiquidationError {
    #[error("Position not found: {0}")]
    PositionNotFound(String),
    #[error("Insufficient collateral: {0}")]
    InsufficientCollateral(String),
    #[error("Liquidation failed: {0}")]
    LiquidationFailed(String),
    #[error("Oracle error: {0}")]
    OracleError(String),
}

// ============================================================================
// Types
// ============================================================================

/// Position information
#[derive(Debug, Clone)]
pub struct Position {
    pub position_id: String,
    pub user: String,
    pub market: String,
    pub size: i64,      // Positive = long, negative = short
    pub entry_price: u64,
    pub margin: u64,
    pub unrealized_pnl: i64,
}

/// Liquidation event
#[derive(Debug, Clone)]
pub struct LiquidationEvent {
    pub position_id: String,
    pub user: String,
    pub market: String,
    pub liquidation_price: u64,
    pub current_price: u64,
    pub margin_ratio: f64,
    pub liquidator: Option<String>,
    pub bonus: u64,
    pub penalty: u64,
}

/// Margin mode
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MarginMode {
    Isolated,
    Cross,
    Portfolio,
}

/// Liquidation result
#[derive(Debug, Clone)]
pub struct LiquidationResult {
    pub position_id: String,
    pub liquidated: bool,
    pub size: u64,
    pub fill_price: u64,
    pub bonus: u64,
    pub penalty: u64,
    pub fees: u64,
}

// ============================================================================
// Liquidation Engine
// ============================================================================

pub struct LiquidationEngine {
    positions: RwLock<HashMap<String, Position>>,
    oracle_prices: RwLock<HashMap<String, u64>>,
    margin_mode: MarginMode,
    liquidation_threshold: f64,
    penalty_rate: f64,
    bonus_rate: f64,
}

impl LiquidationEngine {
    pub fn new() -> Self {
        Self {
            positions: RwLock::new(HashMap::new()),
            oracle_prices: RwLock::new(HashMap::new()),
            margin_mode: MarginMode::Cross,
            liquidation_threshold: 0.01, // 1% margin ratio
            penalty_rate: 0.001,         // 0.1% penalty
            bonus_rate: 0.002,            // 0.2% bonus for liquidator
        }
    }
    
    /// Set margin mode
    pub fn set_margin_mode(&self, mode: MarginMode) {
        let mut mode = self.margin_mode;
    }
    
    /// Update oracle price for market
    pub fn update_price(&self, market: &str, price: u64) {
        self.oracle_prices.write().unwrap().insert(market.to_string(), price);
    }
    
    /// Register position
    pub fn register_position(&self, position: Position) {
        self.positions.write().unwrap().insert(position.position_id.clone(), position);
    }
    
    /// Calculate margin ratio for position
    pub fn calculate_margin_ratio(&self, position: &Position) -> f64 {
        let prices = self.oracle_prices.read().unwrap();
        let current_price = prices.get(&position.market).copied().unwrap_or(position.entry_price);
        
        if current_price == 0 {
            return 0.0;
        }
        
        let position_value = (position.size.unsigned_abs() * current_price) as f64;
        if position_value == 0.0 {
            return 0.0;
        }
        
        let total_margin = position.margin as f64 + position.unrealized_pnl as f64;
        total_margin / position_value
    }
    
    /// Calculate liquidation price
    pub fn calculate_liquidation_price(&self, position: &Position) -> u64 {
        let margin_ratio = self.liquidation_threshold;
        
        let entry_price = position.entry_price;
        let size = position.size.unsigned_abs() as f64;
        
        if size == 0.0 || entry_price == 0 {
            return 0;
        }
        
        // Simplified: liquidation price based on margin ratio
        let required_margin = size * entry_price as f64 * margin_ratio;
        let available_margin = position.margin as f64;
        
        if available_margin >= required_margin {
            return entry_price;
        }
        
        // Calculate price at which position becomes liquidatable
        let diff = required_margin - available_margin;
        let price_adjustment = diff / size;
        
        if position.size > 0 {
            entry_price.saturating_sub(price_adjustment as u64)
        } else {
            entry_price.saturating_add(price_adjustment as u64)
        }
    }
    
    /// Check if position should be liquidated
    pub fn should_liquidate(&self, position: &Position) -> bool {
        let margin_ratio = self.calculate_margin_ratio(position);
        margin_ratio < self.liquidation_threshold
    }
    
    /// Execute liquidation
    pub fn liquidate(
        &self,
        position_id: &str,
        liquidator: &str,
    ) -> Result<LiquidationResult, LiquidationError> {
        let position = self.positions.read().unwrap()
            .get(position_id)
            .cloned()
            .ok_or_else(|| LiquidationError::PositionNotFound(position_id.to_string()))?;
        
        if !self.should_liquidate(&position) {
            return Err(LiquidationError::InsufficientCollateral(
                "position not liquidatable".to_string()
            ));
        }
        
        let prices = self.oracle_prices.read().unwrap();
        let current_price = prices.get(&position.market).copied().unwrap_or(position.entry_price);
        let liquidation_price = self.calculate_liquidation_price(&position);
        
        // Calculate penalty and bonus
        let penalty = (position.margin as f64 * self.penalty_rate) as u64;
        let bonus = (position.margin as f64 * self.bonus_rate) as u64;
        let fees = penalty + bonus;
        
        // Create liquidation result
        let result = LiquidationResult {
            position_id: position_id.to_string(),
            liquidated: true,
            size: position.size.unsigned_abs(),
            fill_price: current_price,
            bonus,
            penalty,
            fees,
        };
        
        // Remove position
        drop(prices);
        self.positions.write().unwrap().remove(position_id);
        
        Ok(result)
    }
    
    /// Get positions needing liquidation
    pub fn get_liquidatable_positions(&self) -> Vec<Position> {
        self.positions.read().unwrap()
            .values()
            .filter(|p| self.should_liquidate(p))
            .cloned()
            .collect()
    }
    
    /// Batch liquidate multiple positions
    pub fn batch_liquidate(&self, liquidator: &str) -> Vec<LiquidationResult> {
        let liquidatable = self.get_liquidatable_positions();
        let mut results = Vec::new();
        
        for position in liquidatable {
            match self.liquidate(&position.position_id, liquidator) {
                Ok(result) => results.push(result),
                Err(_) => continue,
            }
        }
        
        results
    }
}

// ============================================================================
// Isolated Margin Liquidation
// ============================================================================

impl LiquidationEngine {
    /// Calculate isolated margin requirements
    pub fn calculate_isolated_requirement(&self, position: &Position) -> u64 {
        let prices = self.oracle_prices.read().unwrap();
        let current_price = prices.get(&position.market).copied().unwrap_or(position.entry_price);
        
        let position_value = position.size.unsigned_abs() * current_price;
        
        // Isolated margin: position value * maintenance margin ratio
        (position_value as f64 * self.liquidation_threshold) as u64
    }
    
    /// Check isolated margin liquidation
    pub fn check_isolated_liquidation(&self, position: &Position) -> bool {
        let requirement = self.calculate_isolated_requirement(position);
        position.margin < requirement
    }
}

// ============================================================================
// Cross Margin Liquidation
// ============================================================================

impl LiquidationEngine {
    /// Calculate cross margin requirements
    pub fn calculate_cross_requirement(&self, positions: &[Position]) -> u64 {
        // Simplified: sum of all isolated requirements
        positions.iter()
            .map(|p| self.calculate_isolated_requirement(p))
            .sum()
    }
    
    /// Check cross margin liquidation
    pub fn check_cross_liquidation(&self, user: &str) -> bool {
        let positions: Vec<Position> = self.positions.read().unwrap()
            .values()
            .filter(|p| p.user == user)
            .cloned()
            .collect();
        
        if positions.is_empty() {
            return false;
        }
        
        // Total margin across all positions
        let total_margin: u64 = positions.iter().map(|p| p.margin).sum();
        
        // Total required
        let required = self.calculate_cross_requirement(&positions);
        
        total_margin < required
    }
}

// ============================================================================
// Portfolio Margin Liquidation
// ============================================================================

impl LiquidationEngine {
    /// Calculate portfolio margin using risk-based approach
    pub fn calculate_portfolio_requirement(&self, positions: &[Position]) -> u64 {
        // Simplified portfolio margin: net position + worst-case scenario
        // In production: use sophisticated VaR/Spot margin calculation
        
        if positions.is_empty() {
            return 0;
        }
        
        // Calculate net exposure
        let mut net_exposure = 0i64;
        for position in positions {
            net_exposure += position.size;
        }
        
        if net_exposure == 0 {
            // Fully hedged, use maintenance margin
            return self.calculate_cross_requirement(positions);
        }
        
        // Use larger of net exposure or maintenance margin
        let maintenance = self.calculate_cross_requirement(positions);
        let exposure_value = net_exposure.unsigned_abs() * positions[0].entry_price;
        
        std::cmp::max(maintenance, exposure_value) as u64
    }
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_liquidation_engine_creation() {
        let engine = LiquidationEngine::new();
        assert_eq!(engine.liquidation_threshold, 0.01);
    }

    #[test]
    fn test_register_position() {
        let engine = LiquidationEngine::new();
        
        let position = Position {
            position_id: "pos-1".to_string(),
            user: "user-1".to_string(),
            market: "ETH-USD".to_string(),
            size: 1000,
            entry_price: 3000_000000, // $3000
            margin: 100_000000, // $100
            unrealized_pnl: 0,
        };
        
        engine.register_position(position.clone());
        assert!(engine.positions.read().unwrap().contains_key("pos-1"));
    }

    #[test]
    fn test_margin_ratio_calculation() {
        let engine = LiquidationEngine::new();
        engine.update_price("ETH-USD", 2500_000000);
        
        let position = Position {
            position_id: "pos-1".to_string(),
            user: "user-1".to_string(),
            market: "ETH-USD".to_string(),
            size: 1000,
            entry_price: 3000_000000,
            margin: 100_000000,
            unrealized_pnl: 0,
        };
        
        let ratio = engine.calculate_margin_ratio(&position);
        assert!(ratio > 0.0);
    }

    #[test]
    fn test_liquidation_price() {
        let engine = LiquidationEngine::new();
        
        let position = Position {
            position_id: "pos-1".to_string(),
            user: "user-1".to_string(),
            market: "ETH-USD".to_string(),
            size: 1000,
            entry_price: 3000_000000,
            margin: 100_000000,
            unrealized_pnl: 0,
        };
        
        let liq_price = engine.calculate_liquidation_price(&position);
        assert!(liq_price > 0);
    }

    #[test]
    fn test_should_liquidate() {
        let engine = LiquidationEngine::new();
        engine.update_price("ETH-USD", 2500_000000);
        
        let position = Position {
            position_id: "pos-1".to_string(),
            user: "user-1".to_string(),
            market: "ETH-USD".to_string(),
            size: 1000,
            entry_price: 3000_000000,
            margin: 10_000000, // Very low margin
            unrealized_pnl: 0,
        };
        
        assert!(engine.should_liquidate(&position));
    }

    #[test]
    fn test_liquidation() {
        let engine = LiquidationEngine::new();
        engine.update_price("ETH-USD", 2500_000000);
        
        let position = Position {
            position_id: "pos-1".to_string(),
            user: "user-1".to_string(),
            market: "ETH-USD".to_string(),
            size: 1000,
            entry_price: 3000_000000,
            margin: 10_000000,
            unrealized_pnl: 0,
        };
        
        engine.register_position(position);
        
        let result = engine.liquidate("pos-1", "liquidator-1").unwrap();
        assert!(result.liquidated);
    }

    #[test]
    fn test_batch_liquidation() {
        let engine = LiquidationEngine::new();
        engine.update_price("ETH-USD", 2500_000000);
        
        for i in 0..5 {
            let position = Position {
                position_id: format!("pos-{}", i),
                user: format!("user-{}", i),
                market: "ETH-USD".to_string(),
                size: 1000,
                entry_price: 3000_000000,
                margin: 10_000000,
                unrealized_pnl: 0,
            };
            engine.register_position(position);
        }
        
        let results = engine.batch_liquidate("liquidator-1");
        assert_eq!(results.len(), 5);
    }
}