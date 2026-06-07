//! Analytics Module

use std::collections::{HashMap, VecDeque};
use std::sync::Arc;
use tokio::sync::RwLock;

use crate::{TVL, PnL, RiskMetrics, TradingAnalytics, Trade};

/// Analytics Engine
pub struct AnalyticsEngine {
    tvl_history: RwLock<VecDeque<TVL>>,
    pnl_history: RwLock<VecDeque<PnL>>,
    risk_history: RwLock<VecDeque<RiskMetrics>>,
    trades: RwLock<Vec<Trade>>,
}

impl AnalyticsEngine {
    pub fn new() -> Self {
        Self {
            tvl_history: RwLock::new(VecDeque::new()),
            pnl_history: RwLock::new(VecDeque::new()),
            risk_history: RwLock::new(VecDeque::new()),
            trades: RwLock::new(Vec::new()),
        }
    }
    
    /// Calculate TVL
    pub async fn calculate_tvl(&self, token_balances: &HashMap<String, f64>) -> TVL {
        let mut tvl = TVL::new();
        
        for (token, balance) in token_balances {
            tvl.by_token.insert(token.clone(), *balance);
            tvl.total += *balance;
        }
        
        let mut history = self.tvl_history.write().await;
        history.push_back(tvl.clone());
        
        tvl
    }
    
    /// Record trade
    pub async fn record_trade(&self, trade: Trade) {
        let mut trades = self.trades.write().await;
        trades.push(trade);
    }
    
    /// Get trading analytics
    pub async fn get_trading_analytics(&self) -> TradingAnalytics {
        let trades = self.trades.read().await;
        let mut analytics = TradingAnalytics::new();
        analytics.calculate(&trades);
        analytics
    }
}

impl Default for AnalyticsEngine {
    fn default() -> Self {
        Self::new()
    }
}