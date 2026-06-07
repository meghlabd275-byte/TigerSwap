//! Metrics Module

use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// TVL
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TVL {
    pub total: f64,
    pub by_token: HashMap<String, f64>,
    pub by_pool: HashMap<String, f64>,
    pub block_number: u64,
    pub timestamp: i64,
}

impl TVL {
    pub fn new() -> Self {
        Self {
            total: 0.0,
            by_token: HashMap::new(),
            by_pool: HashMap::new(),
            block_number: 0,
            timestamp: chrono::Utc::now().timestamp(),
        }
    }
}

/// PnL
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PnL {
    pub realized_pnl: f64,
    pub unrealized_pnl: f64,
    pub total_pnl: f64,
    pub fees_paid: f64,
    pub funding_received: f64,
    pub period: super::TimePeriod,
}

impl PnL {
    pub fn new(period: super::TimePeriod) -> Self {
        Self {
            realized_pnl: 0.0,
            unrealized_pnl: 0.0,
            total_pnl: 0.0,
            fees_paid: 0.0,
            funding_received: 0.0,
            period,
        }
    }
}

/// Risk Metrics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RiskMetrics {
    pub var_95: f64,
    pub var_99: f64,
    pub sharpe_ratio: f64,
    pub max_drawdown: f64,
    pub volatility: f64,
    pub beta: f64,
    pub correlation: f64,
    pub timestamp: i64,
}

impl RiskMetrics {
    pub fn new() -> Self {
        Self {
            var_95: 0.0,
            var_99: 0.0,
            sharpe_ratio: 0.0,
            max_drawdown: 0.0,
            volatility: 0.0,
            beta: 0.0,
            correlation: 0.0,
            timestamp: chrono::Utc::now().timestamp(),
        }
    }
}

/// Performance Metrics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PerformanceMetrics {
    pub total_return: f64,
    pub annualized_return: f64,
    pub daily_return: f64,
    pub weekly_return: f64,
    pub monthly_return: f64,
    pub yearly_return: f64,
    pub cumulative_return: f64,
    pub timestamp: i64,
}

impl PerformanceMetrics {
    pub fn new() -> Self {
        Self {
            total_return: 0.0,
            annualized_return: 0.0,
            daily_return: 0.0,
            weekly_return: 0.0,
            monthly_return: 0.0,
            yearly_return: 0.0,
            cumulative_return: 0.0,
            timestamp: chrono::Utc::now().timestamp(),
        }
    }
}

impl Default for TVL {
    fn default() -> Self { Self::new() }
}

impl Default for RiskMetrics {
    fn default() -> Self { Self::new() }
}

impl Default for PerformanceMetrics {
    fn default() -> Self { Self::new() }
}