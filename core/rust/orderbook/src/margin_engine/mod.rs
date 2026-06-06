//! Margin Engine
//! 
//! Manages margin calculations for perpetual trading.
//! Supports isolated, cross, and portfolio margin modes.

use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::time::{SystemTime, UNIX_EPOCH};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum MarginError {
    #[error("Insufficient margin: {0}")]
    InsufficientMargin(String),
    #[error("Position not found: {0}")]
    PositionNotFound(String),
    #[error("Invalid operation: {0}")]
    InvalidOperation(String),
}

// ============================================================================
// Types
// ============================================================================

/// Margin mode
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum MarginMode {
    Isolated,   // Per-position margin
    Cross,      // Cross-margin across all positions
    Portfolio,  // Risk-based portfolio margin
}

/// Position with margin info
#[derive(Debug, Clone)]
pub struct MarginPosition {
    pub position_id: String,
    pub user: String,
    pub market: String,
    pub size: i64,
    pub entry_price: u64,
    pub margin: u64,
    pub open_order_margin: u64,
}

/// Account margin summary
#[derive(Debug, Clone)]
pub struct MarginSummary {
    pub user: String,
    pub total_margin: u64,
    pub available_margin: u64,
    pub margin_ratio: f64,
    pub initial_margin: u64,
    pub maintenance_margin: u64,
    pub unrealized_pnl: i64,
}

/// Margin requirement
#[derive(Debug, Clone)]
pub struct MarginRequirement {
    pub initial: u64,
    pub maintenance: u64,
    pub position_value: u64,
    pub leverage: f64,
}

// ============================================================================
// Margin Engine
// ============================================================================

pub struct MarginEngine {
    positions: RwLock<HashMap<String, MarginPosition>>,
    balances: RwLock<HashMap<String, u64>>,
    margin_mode: MarginMode,
    initial_ratio: f64,      // 50% initial margin
    maintenance_ratio: f64,   // 10% maintenance margin
    max_leverage: f64,       // 10x max leverage
}

impl MarginEngine {
    pub fn new() -> Self {
        Self {
            positions: RwLock::new(HashMap::new()),
            balances: RwLock::new(HashMap::new()),
            margin_mode: MarginMode::Cross,
            initial_ratio: 0.50,   // 50%
            maintenance_ratio: 0.10, // 10%
            max_leverage: 10.0,
        }
    }
    
    /// Set margin mode
    pub fn set_margin_mode(&self, mode: MarginMode) {
        let mut m = self.margin_mode;
    }
    
    /// Deposit collateral
    pub fn deposit(&self, user: &str, amount: u64) -> Result<(), MarginError> {
        let mut balances = self.balances.write().unwrap();
        let balance = balances.entry(user.to_string()).or_insert(0);
        *balance += amount;
        Ok(())
    }
    
    /// Withdraw collateral
    pub fn withdraw(&self, user: &str, amount: u64) -> Result<(), MarginError> {
        let mut balances = self.balances.write().unwrap();
        let balance = balances.get_mut(user)
            .ok_or_else(|| MarginError::InsufficientMargin("no balance".to_string()))?;
        
        if *balance < amount {
            return Err(MarginError::InsufficientMargin("insufficient balance".to_string()));
        }
        
        *balance -= amount;
        Ok(())
    }
    
    /// Get balance
    pub fn get_balance(&self, user: &str) -> u64 {
        self.balances.read().unwrap().get(user).copied().unwrap_or(0)
    }
    
    /// Open position with margin
    pub fn open_position(
        &self,
        position_id: String,
        user: &str,
        market: &str,
        size: i64,
        price: u64,
    ) -> Result<u64, MarginError> {
        let required = self.calculate_initial_margin(size, price)?;
        
        let mut balances = self.balances.write().unwrap();
        let balance = balances.entry(user.to_string()).or_insert(0);
        
        if *balance < required {
            return Err(MarginError::InsufficientMargin(
                format!("need {}, have {}", required, balance)
            ));
        }
        
        *balance -= required;
        
        let position = MarginPosition {
            position_id: position_id.clone(),
            user: user.to_string(),
            market: market.to_string(),
            size,
            entry_price: price,
            margin: required,
            open_order_margin: 0,
        };
        
        self.positions.write().unwrap().insert(position_id, position);
        
        Ok(required)
    }
    
    /// Close position and return margin
    pub fn close_position(&self, position_id: &str, user: &str, pnl: i64) -> Result<u64, MarginError> {
        let position = self.positions.write().unwrap()
            .remove(position_id)
            .ok_or_else(|| MarginError::PositionNotFound(position_id.to_string()))?;
        
        if position.user != user {
            return Err(MarginError::InvalidOperation("not your position".to_string()));
        }
        
        // Return margin + PnL
        let return_amount = position.margin as i64 + pnl;
        
        let mut balances = self.balances.write().unwrap();
        let balance = balances.entry(user.to_string()).or_insert(0);
        *balance += return_amount.max(0) as u64;
        
        Ok(return_amount.max(0) as u64)
    }
    
    /// Add to position
    pub fn add_to_position(
        &self,
        position_id: &str,
        user: &str,
        additional_size: i64,
        price: u64,
    ) -> Result<u64, MarginError> {
        let mut positions = self.positions.write().unwrap();
        let position = positions.get_mut(position_id)
            .ok_or_else(|| MarginError::PositionNotFound(position_id.to_string()))?;
        
        if position.user != user {
            return Err(MarginError::InvalidOperation("not your position".to_string()));
        }
        
        // Calculate additional margin needed
        let total_size = position.size + additional_size;
        let additional_margin = self.calculate_initial_margin(total_size.unsigned_abs(), price)?;
        
        let current_margin = position.margin;
        let new_margin = if additional_margin > current_margin {
            additional_margin - current_margin
        } else {
            0
        };
        
        // Check balance
        let mut balances = self.balances.write().unwrap();
        let balance = balances.entry(user).or_insert(0);
        
        if *balance < new_margin {
            return Err(MarginError::InsufficientMargin("insufficient balance".to_string()));
        }
        
        *balance -= new_margin;
        position.margin = additional_margin;
        position.size = total_size;
        
        Ok(new_margin)
    }
    
    /// Calculate margin summary for user
    pub fn get_margin_summary(&self, user: &str) -> MarginSummary {
        let positions: Vec<MarginPosition> = self.positions.read().unwrap()
            .values()
            .filter(|p| p.user == user)
            .cloned()
            .collect();
        
        let balance = self.get_balance(user);
        let total_margin: u64 = positions.iter().map(|p| p.margin).sum();
        let initial_margin: u64 = positions.iter()
            .map(|p| self.calculate_initial_margin(p.size.unsigned_abs(), p.entry_price).unwrap_or(0))
            .sum();
        let maintenance_margin: u64 = positions.iter()
            .map(|p| self.calculate_maintenance_margin(p.size.unsigned_abs(), p.entry_price).unwrap_or(0))
            .sum();
        
        let total_value: u64 = positions.iter()
            .map(|p| p.size.unsigned_abs() * p.entry_price)
            .sum();
        
        let margin_ratio = if total_value > 0 {
            total_margin as f64 / total_value as f64
        } else {
            0.0
        };
        
        MarginSummary {
            user: user.to_string(),
            total_margin: balance,
            available_margin: balance.saturating_sub(total_margin),
            margin_ratio,
            initial_margin,
            maintenance_margin,
            unrealized_pnl: 0, // Would be calculated from prices
        }
    }
    
    /// Calculate initial margin requirement
    pub fn calculate_initial_margin(&self, size: u64, price: u64) -> Result<u64, MarginError> {
        if size == 0 || price == 0 {
            return Ok(0);
        }
        
        let position_value = size * price;
        let margin = (position_value as f64 * self.initial_ratio) as u64;
        
        Ok(margin.max(1))
    }
    
    /// Calculate maintenance margin requirement
    pub fn calculate_maintenance_margin(&self, size: u64, price: u64) -> Result<u64, MarginError> {
        if size == 0 || price == 0 {
            return Ok(0);
        }
        
        let position_value = size * price;
        let margin = (position_value as f64 * self.maintenance_ratio) as u64;
        
        Ok(margin.max(1))
    }
    
    /// Check if user can open position
    pub fn can_open(&self, user: &str, size: u64, price: u64) -> bool {
        let summary = self.get_margin_summary(user);
        let required = self.calculate_initial_margin(size, price).unwrap_or(u64::MAX);
        
        summary.available_margin >= required
    }
    
    /// Get all positions for user
    pub fn get_positions(&self, user: &str) -> Vec<MarginPosition> {
        self.positions.read().unwrap()
            .values()
            .filter(|p| p.user == user)
            .cloned()
            .collect()
    }
}

// ============================================================================
// Cross Margin Calculations
// ============================================================================

impl MarginEngine {
    /// Calculate cross-margin requirements
    pub fn calculate_cross_margin(&self, user: &str) -> MarginRequirement {
        let positions = self.get_positions(user);
        
        let total_value: u64 = positions.iter()
            .map(|p| p.size.unsigned_abs() * p.entry_price)
            .sum();
        
        let initial = positions.iter()
            .map(|p| self.calculate_initial_margin(p.size.unsigned_abs(), p.entry_price).unwrap_or(0))
            .sum();
        
        let maintenance = positions.iter()
            .map(|p| self.calculate_maintenance_margin(p.size.unsigned_abs(), p.entry_price).unwrap_or(0))
            .sum();
        
        let leverage = if initial > 0 {
            total_value as f64 / initial as f64
        } else {
            0.0
        };
        
        MarginRequirement {
            initial,
            maintenance,
            position_value: total_value,
            leverage,
        }
    }
    
    /// Calculate maximum position size
    pub fn max_position_size(&self, user: &str, price: u64) -> u64 {
        let summary = self.get_margin_summary(user);
        
        if price == 0 || self.initial_ratio == 0.0 {
            return 0;
        }
        
        let max_value = summary.available_margin as f64 / self.initial_ratio;
        (max_value / price as f64) as u64
    }
}

// ============================================================================
// Portfolio Margin (Simplified)
// ============================================================================

impl MarginEngine {
    /// Calculate portfolio margin with risk offsets
    pub fn calculate_portfolio_margin(&self, user: &str) -> MarginRequirement {
        let positions = self.get_positions(user);
        
        // Calculate net exposure
        let mut net_exposure = 0i64;
        for position in &positions {
            net_exposure += position.size;
        }
        
        let total_value: u64 = positions.iter()
            .map(|p| p.size.unsigned_abs() * p.entry_price)
            .sum();
        
        // If fully hedged (net = 0), use reduced margin
        let initial = if net_exposure == 0 && !positions.is_empty() {
            // Use maintenance margin only for hedged
            positions.iter()
                .map(|p| self.calculate_maintenance_margin(p.size.unsigned_abs(), p.entry_price).unwrap_or(0))
                .sum()
        } else {
            self.calculate_cross_margin(user).initial
        };
        
        let maintenance = positions.iter()
            .map(|p| self.calculate_maintenance_margin(p.size.unsigned_abs(), p.entry_price).unwrap_or(0))
            .sum();
        
        let leverage = if initial > 0 {
            total_value as f64 / initial as f64
        } else {
            0.0
        };
        
        MarginRequirement {
            initial,
            maintenance,
            position_value: total_value,
            leverage,
        }
    }
}

// ============================================================================
// Helper Functions
// ============================================================================

fn current_timestamp() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_margin_engine_creation() {
        let engine = MarginEngine::new();
        assert_eq!(engine.margin_mode, MarginMode::Cross);
    }

    #[test]
    fn test_deposit_and_withdraw() {
        let engine = MarginEngine::new();
        engine.deposit("user-1", 10000).unwrap();
        
        assert_eq!(engine.get_balance("user-1"), 10000);
        
        engine.withdraw("user-1", 5000).unwrap();
        assert_eq!(engine.get_balance("user-1"), 5000);
    }

    #[test]
    fn test_open_position() {
        let engine = MarginEngine::new();
        engine.deposit("user-1", 100000).unwrap();
        
        let margin = engine.open_position(
            "pos-1".to_string(),
            "user-1",
            "ETH-USD",
            1000,
            3000_000000,
        ).unwrap();
        
        assert!(margin > 0);
    }

    #[test]
    fn test_insufficient_margin() {
        let engine = MarginEngine::new();
        engine.deposit("user-1", 100).unwrap();
        
        let result = engine.open_position(
            "pos-1".to_string(),
            "user-1",
            "ETH-USD",
            10000, // Large position
            3000_000000,
        );
        
        assert!(result.is_err());
    }

    #[test]
    fn test_close_position() {
        let engine = MarginEngine::new();
        engine.deposit("user-1", 100000).unwrap();
        
        engine.open_position(
            "pos-1".to_string(),
            "user-1",
            "ETH-USD",
            1000,
            3000_000000,
        ).unwrap();
        
        let returned = engine.close_position("pos-1", "user-1", 0).unwrap();
        assert!(returned > 0);
    }

    #[test]
    fn test_margin_summary() {
        let engine = MarginEngine::new();
        engine.deposit("user-1", 100000).unwrap();
        
        engine.open_position(
            "pos-1".to_string(),
            "user-1",
            "ETH-USD",
            1000,
            3000_000000,
        ).unwrap();
        
        let summary = engine.get_margin_summary("user-1");
        assert!(summary.total_margin > 0);
    }

    #[test]
    fn test_can_open() {
        let engine = MarginEngine::new();
        engine.deposit("user-1", 100000).unwrap();
        
        let can_open = engine.can_open("user-1", 1000, 3000_000000);
        assert!(can_open);
    }

    #[test]
    fn test_max_position_size() {
        let engine = MarginEngine::new();
        engine.deposit("user-1", 100000).unwrap();
        
        let max_size = engine.max_position_size("user-1", 3000_000000);
        assert!(max_size > 0);
    }

    #[test]
    fn test_cross_margin() {
        let engine = MarginEngine::new();
        engine.deposit("user-1", 100000).unwrap();
        
        engine.open_position(
            "pos-1".to_string(),
            "user-1",
            "ETH-USD",
            1000,
            3000_000000,
        ).unwrap();
        
        let req = engine.calculate_cross_margin("user-1");
        assert!(req.initial > 0);
    }
}