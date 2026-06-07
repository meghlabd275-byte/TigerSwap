//! Trading Module

use serde::{Deserialize, Serialize};

/// Trade
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

/// Trade Side
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TradeSide {
    Buy,
    Sell,
}

/// Trading Analytics
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
    
    pub fn calculate(&mut self, trades: &[Trade]) {
        self.total_trades = trades.len() as u64;
        
        if self.total_trades == 0 {
            return;
        }
        
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
        
        self.win_rate = self.profitable_trades as f64 / self.total_trades as f64;
        self.avg_trade_size = total_size / self.total_trades as f64;
    }
}

impl Default for TradingAnalytics {
    fn default() -> Self {
        Self::new()
    }
}