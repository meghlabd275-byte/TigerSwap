//! Ticker Module

use serde::{Deserialize, Serialize};

/// Ticker
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Ticker {
    pub pair: String,
    pub last_price: f64,
    pub best_bid: f64,
    pub best_ask: f64,
    pub volume_24h: f64,
    pub change_24h: f64,
}