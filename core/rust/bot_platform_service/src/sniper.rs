//! Sniper Module

use serde::{Deserialize, Serialize};

/// Sniper configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SniperConfig {
    pub target_tokens: Vec<String>,
    pub max_slippage: f64,
    pub max_gas_price: f64,
}

impl SniperConfig {
    pub fn new(target_tokens: Vec<String>) -> Self {
        Self {
            target_tokens,
            max_slippage: 0.01,
            max_gas_price: 100.0,
        }
    }
}

/// Sniper target
#[derive(Debug, Clone)]
pub struct SniperTarget {
    pub token: String,
    pub pool_address: String,
    pub target_size: f64,
    pub detected_at: i64,
}

/// Sniper bot
#[derive(Debug, Clone)]
pub struct SniperBot {
    pub bot_id: String,
    pub config: SniperConfig,
    pub status: super::BotStatus,
    pub targets_detected: u64,
    pub successful_snipes: u64,
}

impl SniperBot {
    pub fn new(bot_id: String, config: SniperConfig) -> Self {
        Self {
            bot_id,
            config,
            status: super::BotStatus::Stopped,
            targets_detected: 0,
            successful_snipes: 0,
        }
    }
}