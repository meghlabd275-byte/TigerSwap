//! Trade Module

use serde::{Deserialize, Serialize};

/// Trade
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Trade {
    pub trade_id: String,
    pub pair: String,
    pub maker_order_id: String,
    pub taker_order_id: String,
    pub price: f64,
    pub amount: f64,
    pub maker_fee: f64,
    pub taker_fee: f64,
    pub executed_at: i64,
}