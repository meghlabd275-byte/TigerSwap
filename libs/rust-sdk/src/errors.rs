//! Error types for TigerSwap SDK

use thiserror::Error;

#[derive(Error, Debug)]
pub enum TigerSwapError {
    #[error("API error: {0}")]
    Api(String),
    
    #[error("Invalid request: {0}")]
    InvalidRequest(String),
    
    #[error("Authentication error: {0}")]
    Auth(String),
    
    #[error("Network error: {0}")]
    Network(String),
    
    #[error("Serialization error: {0}")]
    Serialization(String),
    
    #[error("Wallet error: {0}")]
    Wallet(String),
    
    #[error("Transaction error: {0}")]
    Transaction(String),
    
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
    
    #[error("Slippage exceeded")]
    SlippageExceeded,
    
    #[error("Price changed")]
    PriceChanged,
    
    #[error("Order not found: {0}")]
    OrderNotFound(String),
    
    #[error("Invalid signature")]
    InvalidSignature,
    
    #[error("Timeout")]
    Timeout,
    
    #[error("Rate limited")]
    RateLimited,
}

pub type Result<T> = std::result::Result<T, TigerSwapError>;