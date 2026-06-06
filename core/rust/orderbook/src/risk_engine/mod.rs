//! Risk Engine for Order Book
//! 
//! Manages position risk, wallet risk, and market making risk calculations.

use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum RiskError {
    #[error("Risk limit exceeded: {0}")]
    RiskLimitExceeded(String),
    #[error("Position not found: {0}")]
    PositionNotFound(String),
}

// ============================================================================
// Types
// ============================================================================

#[derive(Debug, Clone)]
pub struct RiskLimits {
    pub max_position_size: u64,
    pub max_leverage: f64,
    pub max_daily_loss: u64,
    pub max_open_orders: usize,
    pub max_notional: u64,
}

impl Default for RiskLimits {
    fn default() -> Self {
        Self {
            max_position_size: 1_000_000_000_000, // 1M
            max_leverage: 10.0,
            max_daily_loss: 100_000_000, // 100k
            max_open_orders: 100,
            max_notional: 10_000_000_000, // 10M
        }
    }
}

#[derive(Debug, Clone)]
pub struct RiskCheck {
    pub allowed: bool,
    pub reason: String,
    pub current: f64,
    pub limit: f64,
}

// ============================================================================
// Risk Engine
// ============================================================================

pub struct RiskEngine {
    limits: RwLock<RiskLimits>,
    daily_pnl: RwLock<HashMap<String, i64>>,
    order_counts: RwLock<HashMap<String, usize>>,
}

impl RiskEngine {
    pub fn new() -> Self {
        Self {
            limits: RwLock::new(RiskLimits::default()),
            daily_pnl: RwLock::new(HashMap::new()),
            order_counts: RwLock::new(HashMap::new()),
        }
    }
    
    pub fn set_limits(&self, limits: RiskLimits) {
        *self.limits.write().unwrap() = limits;
    }
    
    pub fn check_position_size(&self, size: u64) -> RiskCheck {
        let limits = self.limits.read().unwrap();
        RiskCheck {
            allowed: size <= limits.max_position_size,
            reason: if size > limits.max_position_size { "max_position_size" } else { "" }.to_string(),
            current: size as f64,
            limit: limits.max_position_size as f64,
        }
    }
    
    pub fn check_leverage(&self, leverage: f64) -> RiskCheck {
        let limits = self.limits.read().unwrap();
        RiskCheck {
            allowed: leverage <= limits.max_leverage,
            reason: if leverage > limits.max_leverage { "max_leverage" } else { "" }.to_string(),
            current: leverage,
            limit: limits.max_leverage,
        }
    }
    
    pub fn check_daily_loss(&self, user: &str, pnl: i64) -> RiskCheck {
        let limits = self.limits.read().unwrap();
        let daily = self.daily_pnl.read().unwrap().get(user).copied().unwrap_or(0);
        let total = daily + pnl;
        
        RiskCheck {
            allowed: total.abs() as u64 <= limits.max_daily_loss,
            reason: if total.abs() as u64 > limits.max_daily_loss { "max_daily_loss" } else { "" }.to_string(),
            current: total.abs() as f64,
            limit: limits.max_daily_loss as f64,
        }
    }
    
    pub fn check_open_orders(&self, user: &str) -> RiskCheck {
        let limits = self.limits.read().unwrap();
        let count = self.order_counts.read().unwrap().get(user).copied().unwrap_or(0);
        
        RiskCheck {
            allowed: count < limits.max_open_orders,
            reason: if count >= limits.max_open_orders { "max_open_orders" } else { "" }.to_string(),
            current: count as f64,
            limit: limits.max_open_orders as f64,
        }
    }
    
    pub fn check_notional(&self, notional: u64) -> RiskCheck {
        let limits = self.limits.read().unwrap();
        RiskCheck {
            allowed: notional <= limits.max_notional,
            reason: if notional > limits.max_notional { "max_notional" } else { "" }.to_string(),
            current: notional as f64,
            limit: limits.max_notional as f64,
        }
    }
    
    pub fn record_pnl(&self, user: &str, pnl: i64) {
        let mut daily = self.daily_pnl.write().unwrap();
        *daily.entry(user.to_string()).or_insert(0) += pnl;
    }
    
    pub fn record_order(&self, user: &str) {
        let mut counts = self.order_counts.write().unwrap();
        *counts.entry(user.to_string()).or_insert(0) += 1;
    }
    
    pub fn reset_daily(&self, user: &str) {
        self.daily_pnl.write().unwrap().remove(user);
        self.order_counts.write().unwrap().remove(user);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_position_check() {
        let engine = RiskEngine::new();
        let check = engine.check_position_size(1000);
        assert!(check.allowed);
    }
    
    #[test]
    fn test_leverage_check() {
        let engine = RiskEngine::new();
        let check = engine.check_leverage(5.0);
        assert!(check.allowed);
    }
}