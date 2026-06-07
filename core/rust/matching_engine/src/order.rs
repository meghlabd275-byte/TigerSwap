//! Order Module

use serde::{Deserialize, Serialize};

/// Order side
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderSide {
    Buy,
    Sell,
}

impl OrderSide {
    pub fn opposite(&self) -> Self {
        match self {
            OrderSide::Buy => OrderSide::Sell,
            OrderSide::Sell => OrderSide::Buy,
        }
    }
}

/// Order type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderType {
    Limit,
    Market,
    StopLoss,
    StopLimit,
}

/// Order status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OrderStatus {
    Pending,
    Open,
    PartiallyFilled,
    Filled,
    Cancelled,
    Expired,
}

/// Order
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Order {
    pub order_id: String,
    pub pair: String,
    pub side: OrderSide,
    pub order_type: OrderType,
    pub price: f64,
    pub amount: f64,
    pub filled_amount: f64,
    pub remaining_amount: f64,
    pub status: OrderStatus,
    pub user_id: String,
    pub created_at: i64,
    pub updated_at: i64,
    pub expires_at: Option<i64>,
    pub stop_price: Option<f64>,
}

impl Order {
    pub fn new_limit(order_id: &str, pair: &str, side: OrderSide, price: f64, amount: f64, user_id: &str) -> Self {
        let now = chrono::Utc::now().timestamp();
        
        Self {
            order_id: order_id.to_string(),
            pair: pair.to_string(),
            side,
            order_type: OrderType::Limit,
            price,
            amount,
            filled_amount: 0.0,
            remaining_amount: amount,
            status: OrderStatus::Open,
            user_id: user_id.to_string(),
            created_at: now,
            updated_at: now,
            expires_at: None,
            stop_price: None,
        }
    }
    
    pub fn fill(&mut self, amount: f64) {
        self.filled_amount += amount;
        self.remaining_amount -= amount;
        
        if self.remaining_amount <= 0.0 {
            self.status = OrderStatus::Filled;
        } else {
            self.status = OrderStatus::PartiallyFilled;
        }
        
        self.updated_at = chrono::Utc::now().timestamp();
    }
    
    pub fn cancel(&mut self) {
        self.status = OrderStatus::Cancelled;
        self.updated_at = chrono::Utc::now().timestamp();
    }
}