//! Grid Module

use serde::{Deserialize, Serialize};

/// Grid configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GridConfig {
    pub grid_levels: u32,
    pub grid_spacing: f64,
    pub min_order_size: f64,
    pub max_order_size: f64,
    pub base_price: f64,
    pub upper_bound: f64,
    pub lower_bound: f64,
}

impl GridConfig {
    pub fn new(grid_levels: u32, grid_spacing: f64, base_price: f64) -> Self {
        Self {
            grid_levels,
            grid_spacing,
            min_order_size: 0.01,
            max_order_size: 1.0,
            base_price,
            upper_bound: base_price * 1.05,
            lower_bound: base_price * 0.95,
        }
    }
}

/// Grid level
#[derive(Debug, Clone)]
pub struct GridLevel {
    pub price: f64,
    pub active: bool,
}

impl GridLevel {
    pub fn new(price: f64) -> Self {
        Self {
            price,
            active: false,
        }
    }
}

/// Grid bot
#[derive(Debug, Clone)]
pub struct GridBot {
    pub bot_id: String,
    pub config: GridConfig,
    pub levels: Vec<GridLevel>,
    pub status: super::BotStatus,
    pub pnl: f64,
    pub total_trades: u64,
}

impl GridBot {
    pub fn new(bot_id: String, config: GridConfig) -> Self {
        let levels = (0..config.grid_levels)
            .map(|i| {
                let range = config.upper_bound - config.lower_bound;
                let step = range / config.grid_levels as f64;
                let price = config.lower_bound + (step * i as f64);
                GridLevel::new(price)
            })
            .collect();
        
        Self {
            bot_id,
            config,
            levels,
            status: super::BotStatus::Stopped,
            pnl: 0.0,
            total_trades: 0,
        }
    }
}