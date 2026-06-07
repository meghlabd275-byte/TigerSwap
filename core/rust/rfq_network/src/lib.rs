//! RFQ (Request for Quote) Network
//! 
//! Provides RFQ functionality for professional market makers (similar to 1inch Fusion, CoW Swap)

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::Arc;
use parking_lot::RwLock;
use thiserror::Error;
use uuid::Uuid;
use chrono::Utc;

/// RFQ errors
#[derive(Debug, Error)]
pub enum RFQError {
    #[error("Quote expired")]
    QuoteExpired,
    #[error("Invalid quote")]
    InvalidQuote,
    #[error("Market maker not found")]
    MarketMakerNotFound,
    #[error("No quotes available")]
    NoQuotes,
}

/// RFQ Request
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RFQRequest {
    /// Request ID
    pub request_id: String,
    /// User address
    pub user: String,
    /// Buy token
    pub buy_token: String,
    /// Sell token
    pub sell_token: String,
    /// Sell amount
    pub sell_amount: u128,
    /// Deadline
    pub deadline: i64,
    /// Created at
    pub created_at: i64,
}

impl RFQRequest {
    pub fn new(
        user: String,
        buy_token: String,
        sell_token: String,
        sell_amount: u128,
    ) -> Self {
        Self {
            request_id: Uuid::new_v4().to_string(),
            user,
            buy_token,
            sell_token,
            sell_amount,
            deadline: Utc::now().timestamp() + 30, // 30 sec
            created_at: Utc::now().timestamp(),
        }
    }
}

/// RFQ Quote
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RFQQuote {
    /// Quote ID
    pub quote_id: String,
    /// Market maker address
    pub market_maker: String,
    /// Request ID
    pub request_id: String,
    /// Buy token
    pub buy_token: String,
    /// Sell token
    pub sell_token: String,
    /// Buy amount
    pub buy_amount: u128,
    /// Sell amount
    pub sell_amount: u128,
    /// Fee (in buy token)
    pub fee: u128,
    /// Expiration
    pub expires_at: i64,
    /// Signature
    pub signature: String,
}

impl RFQQuote {
    pub fn new(
        market_maker: String,
        request_id: String,
        buy_amount: u128,
        sell_amount: u128,
        fee: u128,
    ) -> Self {
        Self {
            quote_id: Uuid::new_v4().to_string(),
            market_maker,
            request_id,
            buy_token: String::new(),
            sell_token: String::new(),
            buy_amount,
            sell_amount,
            fee,
            expires_at: Utc::now().timestamp() + 60,
            signature: String::new(),
        }
    }

    pub fn is_valid(&self) -> bool {
        Utc::now().timestamp() < self.expires_at && !self.signature.is_empty()
    }
}

/// Market Maker
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MarketMaker {
    pub address: String,
    pub name: String,
    pub is_active: bool,
    pub regions: Vec<String>,
    pub chains: Vec<u64>,
    pub quote_timeout_ms: u64,
    pub min_quote_amount: u128,
}

impl MarketMaker {
    pub fn new(address: String, name: String) -> Self {
        Self {
            address,
            name,
            is_active: true,
            regions: vec!["global".to_string()],
            chains: vec![1, 56, 137, 42161, 10],
            quote_timeout_ms: 5000,
            min_quote_amount: 100_000_000,
        }
    }
}

/// RFQ Network
pub struct RFQNetwork {
    market_makers: Arc<RwLock<HashMap<String, MarketMaker>>>,
    requests: Arc<RwLock<HashMap<String, RFQRequest>>>,
    quotes: Arc<RwLock<HashMap<String, Vec<RFQQuote>>>>,
}

impl RFQNetwork {
    pub fn new() -> Self {
        Self {
            market_makers: Arc::new(RwLock::new(HashMap::new())),
            requests: Arc::new(RwLock::new(HashMap::new())),
            quotes: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    pub fn register_market_maker(&self, mm: MarketMaker) {
        let mut makers = self.market_makers.write();
        makers.insert(mm.address.clone(), mm);
    }

    pub fn create_request(&self, request: RFQRequest) -> String {
        let id = request.request_id.clone();
        let mut requests = self.requests.write();
        requests.insert(id.clone(), request);
        id
    }

    pub fn submit_quote(&self, quote: RFQQuote) -> Result<(), RFQError> {
        if !quote.is_valid() {
            return Err(RFQError::InvalidQuote);
        }
        
        let mut quotes = self.quotes.write();
        let request_quotes = quotes.entry(quote.request_id.clone()).or_insert_with(Vec::new);
        request_quotes.push(quote);
        Ok(())
    }

    pub fn get_best_quote(&self, request_id: &str) -> Option<RFQQuote> {
        let quotes = self.quotes.read();
        let request_quotes = quotes.get(request_id)?;
        
        request_quotes.iter()
            .filter(|q| q.is_valid())
            .max_by_key(|q| q.buy_amount)
            .cloned()
    }

    pub fn get_market_makers(&self) -> Vec<MarketMaker> {
        let makers = self.market_makers.read();
        makers.values()
            .filter(|m| m.is_active)
            .cloned()
            .collect()
    }
}

impl Default for RFQNetwork {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_rfq() {
        let network = RFQNetwork::new();
        
        let mm = MarketMaker::new("0xMM1".to_string(), "TestMM".to_string());
        network.register_market_maker(mm);
        
        let request = RFQRequest::new(
            "0xuser".to_string(),
            "0xUSDC".to_string(),
            "0xETH".to_string(),
            1_000_000_000_000_000_000,
        );
        
        let id = network.create_request(request);
        assert!(!id.is_empty());
    }
}