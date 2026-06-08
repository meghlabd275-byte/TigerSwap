//! TigerSwap SDK Errors
//! 
//! Error types for the TigerSwap SDK

use thiserror::Error;

/// TigerSwap SDK errors
#[derive(Debug, Error)]
pub enum TigerSwapError {
    /// Configuration error
    #[error("Configuration error: {0}")]
    ConfigError(String),
    
    /// Request error
    #[error("Request error: {0}")]
    RequestError(String),
    
    /// API error
    #[error("API error ({0}): {1}")]
    ApiError(u16, String),
    
    /// Parse error
    #[error("Parse error: {0}")]
    ParseError(String),
    
    /// Authentication error
    #[error("Authentication error: {0}")]
    AuthError(String),
    
    /// Validation error
    #[error("Validation error: {0}")]
    ValidationError(String),
    
    /// Network error
    #[error("Network error: {0}")]
    NetworkError(String),
    
    /// Rate limit error
    #[error("Rate limit exceeded")]
    RateLimitError,
    
    /// Not found error
    #[error("Resource not found: {0}")]
    NotFound(String),
    
    /// Insufficient balance error
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
    
    /// Slippage exceeded error
    #[error("Slippage exceeded: {0}")]
    SlippageExceeded(String),
    
    /// Transaction error
    #[error("Transaction error: {0}")]
    TransactionError(String),
    
    /// Wallet error
    #[error("Wallet error: {0}")]
    WalletError(String),
    
    /// Signing error
    #[error("Signing error: {0}")]
    SigningError(String),
    
    /// Unknown error
    #[error("Unknown error: {0}")]
    Unknown(String),
}

impl From<reqwest::Error> for TigerSwapError {
    fn from(err: reqwest::Error) -> Self {
        if err.is_timeout() {
            TigerSwapError::NetworkError("Request timeout".to_string())
        } else if err.is_connect() {
            TigerSwapError::NetworkError("Connection failed".to_string())
        } else {
            TigerSwapError::RequestError(err.to_string())
        }
    }
}

impl From<serde_json::Error> for TigerSwapError {
    fn from(err: serde_json::Error) -> Self {
        TigerSwapError::ParseError(err.to_string())
    }
}

impl std::fmt::Display for TigerSwapError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self)
    }
}

impl std::error::Error for TigerSwapError {}
