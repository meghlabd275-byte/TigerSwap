//! TigerSwap Rust SDK
//! 
//! A comprehensive Rust SDK for interacting with TigerSwap DEX

pub mod client;
pub mod types;
pub mod errors;

pub use client::TigerSwapClient;
pub use types::*;
pub use errors::TigerSwapError;

use serde::{Deserialize, Serialize};

/// SDK version
pub const VERSION: &str = "1.0.0";

/// Default API endpoint
pub const DEFAULT_ENDPOINT: &str = "https://api.tigerswap.exchange";

/// Configuration for the TigerSwap client
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Config {
    /// API endpoint URL
    pub endpoint: String,
    /// API key for authentication
    pub api_key: Option<String>,
    /// WebSocket endpoint for real-time data
    pub ws_endpoint: String,
    /// Request timeout in seconds
    pub timeout: u64,
    /// Maximum retry attempts
    pub max_retries: u32,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            endpoint: DEFAULT_ENDPOINT.to_string(),
            api_key: None,
            ws_endpoint: DEFAULT_ENDPOINT.replace("https://", "wss://").replace("http://", "ws://") + "/ws",
            timeout: 30,
            max_retries: 3,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_config_default() {
        let config = Config::default();
        assert_eq!(config.endpoint, DEFAULT_ENDPOINT);
        assert_eq!(config.timeout, 30);
        assert_eq!(config.max_retries, 3);
    }
}
