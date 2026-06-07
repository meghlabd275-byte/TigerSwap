//! Order Module

use serde::{Deserialize, Serialize};

/// Order side
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderSide {
    Buy,
    Sell,
}

/// Order status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderStatus {
    Pending,
    Filled,
    Cancelled,
    Failed,
}

/// Order
#[derive(Debug, Clone)]
pub struct Order {
    pub order_id: String,
    pub side: OrderSide,
    pub price: f64,
    pub size: f64,
    pub status: OrderStatus,
}

impl Order {
    pub fn new(side: OrderSide, order_id: &str, price: f64, size: f64) -> Self {
        Self {
            order_id: order_id.to_string(),
            side,
            price,
            size,
            status: OrderStatus::Pending,
        }
    }
}

/// Trade
#[derive(Debug, Clone)]
pub struct Trade {
    pub trade_id: String,
    pub pair: String,
    pub side: OrderSide,
    pub size: f64,
    pub price: f64,
    pub pnl: f64,
}

impl Trade {
    pub fn new(trade_id: &str, pair: String, side: OrderSide, size: f64, price: f64, pnl: f64) -> Self {
        Self {
            trade_id: trade_id.to_string(),
            pair,
            side,
            size,
            price,
            pnl,
        }
    }
}