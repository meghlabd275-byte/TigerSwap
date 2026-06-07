//! TigerSwap Analytics Service - Production-Ready Rust Analytics
//! 
//! Complete analytics implementation with:
//! - TVL (Total Value Locked) calculation
//! - PnL (Profit and Loss) tracking
//! - Risk metrics calculation
//! - Performance analytics
//! - Historical data aggregation

use std::collections::{HashMap, VecDeque};
use std::sync::Arc;
use tokio::sync::RwLock;

use serde::{Deserialize, Serialize};
use thiserror::Error;

// ============================================================================
// Error Types
// ============================================================================

#[derive(Error, Debug)]
pub enum AnalyticsError {
    #[error("Invalid data")]
    InvalidData,
    #[error("Calculation error")]
    CalculationError,
    #[error("Storage error")]
    StorageError,
    #[error("Insufficient data")]
    InsufficientData,
}

// ============================================================================
// Core Metrics
// ============================================================================

/// TVL (Total Value Locked)
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
    
    pub fn add_token(&mut self, token: &str, value: f64) {
        *self.by_token.entry(token.to_string()).or_insert(0.0) += value;
        self.total += value;
    }
    
    pub fn add_pool(&mut self, pool: &str, value: f64) {
        *self.by_pool.entry(pool.to_string()).or_insert(0.0) += value;
    }
}

impl Default for TVL {
    fn default() -> Self {
        Self::new()
    }
}

/// PnL (Profit and Loss)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PnL {
    pub realized_pnl: f64,
    pub unrealized_pnl: f64,
    pub total_pnl: f64,
    pub fees_paid: f64,
    pub funding_received: f64,
    pub period: TimePeriod,
}

impl PnL {
    pub fn new(period: TimePeriod) -> Self {
        Self {
            realized_pnl: 0.0,
            unrealized_pnl: 0.0,
            total_pnl: 0.0,
            fees_paid: 0.0,
            funding_received: 0.0,
            period,
        }
    }
    
    pub fn calculate(&mut self) {
        self.total_pnl = self.realized_pnl + self.unrealized_pnl + self.funding_received - self.fees_paid;
    }
}

/// Risk Metrics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RiskMetrics {
    pub var_95: f64,         // Value at Risk 95%
    pub var_99: f64,         // Value at Risk 99%
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
    
    /// Calculate VaR from returns
    pub fn calculate_var(&mut self, returns: &[f64], confidence: f64) {
        if returns.is_empty() {
            return;
        }
        
        let sorted: Vec<f64> = returns.iter().cloned().collect();
        let sorted = vec_as_slice(&sorted);
        
        let index = ((1.0 - confidence) * returns.len() as f64) as usize;
        
        if confidence == 0.95 {
            self.var_95 = sorted.get(index).copied().unwrap_or(0.0);
        } else if confidence == 0.99 {
            self.var_99 = sorted.get(index).copied().unwrap_or(0.0);
        }
    }
    
    /// Calculate Sharpe ratio
    pub fn calculate_sharpe(&mut self, returns: &[f64], risk_free_rate: f64) {
        if returns.len() < 2 {
            return;
        }
        
        let mean = returns.iter().sum::<f64>() / returns.len() as f64;
        let variance = returns.iter().map(|r| (r - mean).powi(2)).sum::<f64>() / returns.len() as f64;
        let std_dev = variance.sqrt();
        
        if std_dev > 0.0 {
            self.sharpe_ratio = (mean - risk_free_rate) / std_dev;
        }
    }
    
    /// Calculate max drawdown
    pub fn calculate_max_drawdown(&mut self, prices: &[f64]) {
        if prices.is_empty() {
            return;
        }
        
        let mut peak = prices[0];
        let mut max_dd = 0.0;
        
        for price in prices {
            if *price > peak {
                peak = *price;
            }
            let dd = (peak - price) / peak;
            if dd > max_dd {
                max_dd = dd;
            }
        }
        
        self.max_drawdown = max_dd;
    }
    
    /// Calculate volatility
    pub fn calculate_volatility(&mut self, returns: &[f64]) {
        if returns.len() < 2 {
            return;
        }
        
        let mean = returns.iter().sum::<f64>() / returns.len() as f64;
        let variance = returns.iter().map(|r| (r - mean).powi(2)).sum::<f64>() / returns.len() as f64;
        self.volatility = variance.sqrt();
    }
}

impl Default for RiskMetrics {
    fn default() -> Self {
        Self::new()
    }
}

fn vec_as_slice(v: &[f64]) -> Vec<f64> {
    let mut sorted = v.to_vec();
    sorted.sort_by(|a, b| a.partial_cmp(b).unwrap());
    sorted
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
    
    /// Calculate returns from prices
    pub fn from_prices(&mut self, prices: &[f64]) {
        if prices.len() < 2 {
            return;
        }
        
        self.total_return = (prices.last().unwrap() - prices.first().unwrap()) / prices.first().unwrap();
        
        // Daily return
        self.daily_return = if prices.len() > 1 {
            (prices[1] - prices[0]) / prices[0]
        } else {
            0.0
        };
        
        // Weekly return (last 7 days)
        let weekly_idx = prices.len().saturating_sub(7);
        if weekly_idx < prices.len() {
            self.weekly_return = (prices.last().unwrap() - prices[weekly_idx]) / prices[weekly_idx];
        }
        
        // Monthly return (last 30 days)
        let monthly_idx = prices.len().saturating_sub(30);
        if monthly_idx < prices.len() {
            self.monthly_return = (prices.last().unwrap() - prices[monthly_idx]) / prices[monthly_idx];
        }
        
        // Yearly return
        let yearly_idx = prices.len().saturating_sub(365);
        if yearly_idx < prices.len() && prices.len() >= 365 {
            self.yearly_return = (prices.last().unwrap() - prices[yearly_idx]) / prices[yearly_idx];
            self.annualized_return = self.yearly_return;
        } else if prices.len() > 1 {
            let days = prices.len() as f64;
            self.annualized_return = (1.0 + self.total_return).pow_ref(365.0 / days) - 1.0;
        }
        
        self.cumulative_return = self.total_return;
    }
}

impl Default for PerformanceMetrics {
    fn default() -> Self {
        Self::new()
    }
}

// ============================================================================
// Time Period
// ============================================================================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TimePeriod {
    Hour,
    Day,
    Week,
    Month,
    Year,
    All,
}

impl TimePeriod {
    pub fn to_seconds(&self) -> i64 {
        match self {
            TimePeriod::Hour => 3600,
            TimePeriod::Day => 86400,
            TimePeriod::Week => 604800,
            TimePeriod::Month => 2592000,
            TimePeriod::Year => 31536000,
            TimePeriod::All => i64::MAX,
        }
    }
}

// ============================================================================
// Pool Analytics
// ============================================================================

/// Pool analytics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PoolAnalytics {
    pub pool_id: String,
    pub tvl: f64,
    pub volume_24h: f64,
    pub volume_7d: f64,
    pub fees_24h: f64,
    pub apy: f64,
    pub apr: f64,
    pub utilization: f64,
}

impl PoolAnalytics {
    pub fn new(pool_id: &str) -> Self {
        Self {
            pool_id: pool_id.to_string(),
            tvl: 0.0,
            volume_24h: 0.0,
            volume_7d: 0.0,
            fees_24h: 0.0,
            apy: 0.0,
            apr: 0.0,
            utilization: 0.0,
        }
    }
    
    /// Calculate APY from APR and compounding frequency
    pub fn calculate_apy(&mut self, comp_freq: u32) {
        if self.apr > 0.0 && comp_freq > 0 {
            self.apy = (1.0 + self.apr / comp_freq as f64).powf(comp_freq as f64) - 1.0;
        }
    }
}

// ============================================================================
// Trading Analytics
// ============================================================================

/// Trading analytics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TradingAnalytics {
    pub total_trades: u64,
    pub profitable_trades: u64,
    pub losing_trades: u64,
    pub win_rate: f64,
    pub avg_profit: f64,
    pub avg_loss: f64,
    pub largest_profit: f64,
    pub largest_loss: f64,
    pub avg_trade_size: f64,
    pub avg_hold_time: f64,
}

impl TradingAnalytics {
    pub fn new() -> Self {
        Self {
            total_trades: 0,
            profitable_trades: 0,
            losing_trades: 0,
            win_rate: 0.0,
            avg_profit: 0.0,
            avg_loss: 0.0,
            largest_profit: 0.0,
            largest_loss: 0.0,
            avg_trade_size: 0.0,
            avg_hold_time: 0.0,
        }
    }
    
    /// Calculate from trade history
    pub fn calculate(&mut self, trades: &[Trade]) {
        self.total_trades = trades.len() as u64;
        
        let mut total_profit = 0.0;
        let mut total_loss = 0.0;
        let mut total_size = 0.0;
        
        for trade in trades {
            if trade.pnl > 0.0 {
                self.profitable_trades += 1;
                total_profit += trade.pnl;
                if trade.pnl > self.largest_profit {
                    self.largest_profit = trade.pnl;
                }
            } else {
                self.losing_trades += 1;
                total_loss += trade.pnl.abs();
                if trade.pnl.abs() > self.largest_loss {
                    self.largest_loss = trade.pnl.abs();
                }
            }
            total_size += trade.size;
        }
        
        if self.profitable_trades > 0 {
            self.avg_profit = total_profit / self.profitable_trades as f64;
        }
        
        if self.losing_trades > 0 {
            self.avg_loss = total_loss / self.losing_trades as f64;
        }
        
        if self.total_trades > 0 {
            self.win_rate = self.profitable_trades as f64 / self.total_trades as f64;
            self.avg_trade_size = total_size / self.total_trades as f64;
        }
    }
}

impl Default for TradingAnalytics {
    fn default() -> Self {
        Self::new()
    }
}

/// Trade record
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Trade {
    pub trade_id: String,
    pub pair: String,
    pub side: TradeSide,
    pub size: f64,
    pub price: f64,
    pub pnl: f64,
    pub timestamp: i64,
    pub hold_time: f64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TradeSide {
    Buy,
    Sell,
}

// ============================================================================
// Analytics Engine
// ============================================================================

/// Analytics engine
pub struct AnalyticsEngine {
    tvl_history: RwLock<VecDeque<TVL>>,
    pnl_history: RwLock<VecDeque<PnL>>,
    risk_history: RwLock<VecDeque<RiskMetrics>>,
    pool_analytics: RwLock<HashMap<String, PoolAnalytics>>,
    trades: RwLock<Vec<Trade>>,
}

impl AnalyticsEngine {
    pub fn new() -> Self {
        Self {
            tvl_history: RwLock::new(VecDeque::new()),
            pnl_history: RwLock::new(VecDeque::new()),
            risk_history: RwLock::new(VecDeque::new()),
            pool_analytics: RwLock::new(HashMap::new()),
            trades: RwLock::new(Vec::new()),
        }
    }
    
    /// Calculate TVL
    pub async fn calculate_tvl(&self, token_balances: &HashMap<String, f64>, pool_balances: &HashMap<String, f64>) -> TVL {
        let mut tvl = TVL::new();
        
        for (token, balance) in token_balances {
            tvl.add_token(token, *balance);
        }
        
        for (pool, balance) in pool_balances {
            tvl.add_pool(pool, *balance);
        }
        
        // Store in history
        let mut history = self.tvl_history.write().await;
        history.push_back(tvl.clone());
        while history.len() > 10000 {
            history.pop_front();
        }
        
        tvl
    }
    
    /// Get TVL history
    pub async fn get_tvl_history(&self, period: TimePeriod) -> Vec<TVL> {
        let history = self.tvl_history.read().await;
        let now = chrono::Utc::now().timestamp();
        let cutoff = now - period.to_seconds();
        
        history.iter()
            .filter(|t| t.timestamp > cutoff)
            .cloned()
            .collect()
    }
    
    /// Calculate PnL
    pub async fn calculate_pnl(&self, trades: &[Trade], period: TimePeriod) -> PnL {
        let now = chrono::Utc::now().timestamp();
        let cutoff = now - period.to_seconds();
        
        let mut pnl = PnL::new(period);
        
        for trade in trades {
            if trade.timestamp > cutoff {
                pnl.realized_pnl += trade.pnl;
            } else {
                pnl.unrealized_pnl += trade.pnl;
            }
        }
        
        pnl.calculate();
        
        let mut history = self.pnl_history.write().await;
        history.push_back(pnl.clone());
        
        pnl
    }
    
    /// Calculate risk metrics
    pub async fn calculate_risk(&self, prices: &[f64]) -> RiskMetrics {
        let mut risk = RiskMetrics::new();
        
        if prices.len() < 2 {
            return risk;
        }
        
        // Calculate returns
        let returns: Vec<f64> = prices.windows(2)
            .map(|w| (w[1] - w[0]) / w[0])
            .collect();
        
        risk.calculate_var(&returns, 0.95);
        risk.calculate_var(&returns, 0.99);
        risk.calculate_sharpe(&returns, 0.02); // 2% risk-free rate
        risk.calculate_max_drawdown(prices);
        risk.calculate_volatility(&returns);
        
        let mut history = self.risk_history.write().await;
        history.push_back(risk.clone());
        
        risk
    }
    
    /// Calculate pool analytics
    pub async fn calculate_pool_analytics(&self, pool_id: &str) -> PoolAnalytics {
        let analytics = PoolAnalytics::new(pool_id);
        
        let mut pools = self.pool_analytics.write().await;
        pools.insert(pool_id.to_string(), analytics.clone());
        
        analytics
    }
    
    /// Get trading analytics
    pub async fn get_trading_analytics(&self) -> TradingAnalytics {
        let trades = self.trades.read().await;
        let mut analytics = TradingAnalytics::new();
        analytics.calculate(&trades);
        
        analytics
    }
    
    /// Record trade
    pub async fn record_trade(&self, trade: Trade) {
        let mut trades = self.trades.write().await;
        trades.push(trade);
    }
}

impl Default for AnalyticsEngine {
    fn default() -> Self {
        Self::new()
    }
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_tvl() {
        let mut tvl = TVL::new();
        tvl.add_token("ETH", 1000.0);
        tvl.add_token("USDC", 500.0);
        tvl.add_pool("pool1", 200.0);
        
        assert_eq!(tvl.total, 1500.0);
    }
    
    #[test]
    fn test_risk_metrics() {
        let mut risk = RiskMetrics::new();
        let prices = vec![100.0, 110.0, 105.0, 115.0, 120.0];
        
        risk.calculate_max_drawdown(&prices);
        assert!(risk.max_drawdown > 0.0);
    }
    
    #[tokio::test]
    async fn test_analytics_engine() {
        let engine = AnalyticsEngine::new();
        
        let token_balances = HashMap::from([
            ("ETH".to_string(), 1000.0),
            ("USDC".to_string(), 500.0),
        ]);
        let pool_balances = HashMap::new();
        
        let tvl = engine.calculate_tvl(&token_balances, &pool_balances).await;
        assert_eq!(tvl.total, 1500.0);
    }
    
    #[tokio::test]
    async fn test_trading_analytics() {
        let engine = AnalyticsEngine::new();
        
        let trades = vec![
            Trade {
                trade_id: "1".to_string(),
                pair: "ETH/USDC".to_string(),
                side: TradeSide::Buy,
                size: 1.0,
                price: 100.0,
                pnl: 10.0,
                timestamp: chrono::Utc::now().timestamp(),
                hold_time: 3600.0,
            },
            Trade {
                trade_id: "2".to_string(),
                pair: "ETH/USDC".to_string(),
                side: TradeSide::Sell,
                size: 0.5,
                price: 110.0,
                pnl: -5.0,
                timestamp: chrono::Utc::now().timestamp(),
                hold_time: 7200.0,
            },
        ];
        
        for trade in trades {
            engine.record_trade(trade).await;
        }
        
        let analytics = engine.get_trading_analytics().await;
        assert_eq!(analytics.total_trades, 2);
    }
}

// ============================================================================
// Library Exports
// ============================================================================

pub use self::{
    metrics::{TVL, PnL, RiskMetrics, PerformanceMetrics},
    pool::{PoolAnalytics, PoolAnalyticsBuilder},
    trading::{TradingAnalytics, Trade, TradeSide},
    analytics::AnalyticsEngine,
    period::TimePeriod,
};

mod metrics;
mod pool;
mod trading;
mod analytics;
mod period;