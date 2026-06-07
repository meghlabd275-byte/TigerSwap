//! Market Making Module

use serde::{Deserialize, Serialize};

/// Market making configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MarketMakingConfig {
    pub spread: f64,
    pub order_size: f64,
    pub max_position: f64,
    pub refresh_rate_ms: u64,
}

impl MarketMakingConfig {
    pub fn new(spread: f64, order_size: f64) -> Self {
        Self {
            spread,
            order_size,
            max_position: 10.0,
            refresh_rate_ms: 1000,
        }
    }
}

/// Market making bot
#[derive(Debug, Clone)]
pub struct MarketMakingBot {
    pub bot_id: String,
    pub config: MarketMakingConfig,
    pub status: super::BotStatus,
    pub position: f64,
    pub realized_pnl: f64,
}

impl MarketMakingBot {
    pub fn new(bot_id: String, config: MarketMakingConfig) -> Self {
        Self {
            bot_id,
            config,
            status: super::BotStatus::Stopped,
            position: 0.0,
            realized_pnl: 0.0,
        }
    }
    
    pub fn calculate_quotes(&self, mid_price: f64) -> (f64, f64) {
        let spread = mid_price * self.config.spread;
        (mid_price - spread / 2.0, mid_price + spread / 2.0)
    }
}