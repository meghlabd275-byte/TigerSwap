//! Order Book Module

use std::collections::VecDeque;
use serde::{Deserialize, Serialize};

use crate::{Order, OrderSide};

/// Order book
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OrderBook {
    pub pair: String,
    pub bids: VecDeque<Order>,
    pub asks: VecDeque<Order>,
}

impl OrderBook {
    pub fn new(pair: &str) -> Self {
        Self {
            pair: pair.to_string(),
            bids: VecDeque::new(),
            asks: VecDeque::new(),
        }
    }
    
    pub fn add_order(&mut self, order: Order) {
        match order.side {
            OrderSide::Buy => {
                let mut inserted = false;
                for (i, existing) in self.bids.iter().enumerate() {
                    if existing.price < order.price {
                        self.bids.insert(i, order);
                        inserted = true;
                        break;
                    }
                }
                if !inserted {
                    self.bids.push_back(order);
                }
            }
            OrderSide::Sell => {
                let mut inserted = false;
                for (i, existing) in self.asks.iter().enumerate() {
                    if existing.price > order.price {
                        self.asks.insert(i, order);
                        inserted = true;
                        break;
                    }
                }
                if !inserted {
                    self.asks.push_back(order);
                }
            }
        }
    }
    
    pub fn best_bid(&self) -> Option<&Order> {
        self.bids.front()
    }
    
    pub fn best_ask(&self) -> Option<&Order> {
        self.asks.front()
    }
    
    pub fn spread(&self) -> Option<f64> {
        match (self.best_bid(), self.best_ask()) {
            (Some(bid), Some(ask)) => Some(ask.price - bid.price),
            _ => None,
        }
    }
}