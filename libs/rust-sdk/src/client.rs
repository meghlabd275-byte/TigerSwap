//! TigerSwap Client - Main SDK Client

use crate::errors::{Result, TigerSwapError};
use crate::models::*;
use std::sync::Arc;

/// TigerSwap API client configuration
#[derive(Debug, Clone)]
pub struct Config {
    /// API base URL
    pub base_url: String,
    /// API key for authentication
    pub api_key: Option<String>,
    /// Timeout for requests
    pub timeout: std::time::Duration,
    /// Maximum retries
    pub max_retries: u32,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            base_url: "https://api.tigerswap.io".to_string(),
            api_key: None,
            timeout: std::time::Duration::from_secs(30),
            max_retries: 3,
        }
    }
}

/// Main TigerSwap client
#[derive(Debug, Clone)]
pub struct TigerSwapClient {
    config: Config,
    http_client: reqwest::Client,
}

impl TigerSwapClient {
    /// Create a new client
    pub fn new(config: Config) -> Result<Self> {
        let http_client = reqwest::Client::builder()
            .timeout(config.timeout)
            .build()
            .map_err(|e| TigerSwapError::Network(e.to_string()))?;

        Ok(Self {
            config,
            http_client,
        })
    }

    /// Get token balance
    pub async fn get_balance(&self, owner: &str, token: &str) -> Result<TokenBalance> {
        let url = format!("{}/v1/balance/{}", self.config.base_url, owner);
        
        let response = self.http_client
            .get(&url)
            .query(&[("token", token)])
            .send()
            .await
            .map_err(|e| TigerSwapError::Network(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::Api(format!("Status: {}", response.status())));
        }

        let api_response: APIResponse<TokenBalance> = response
            .json()
            .await
            .map_err(|e| TigerSwapError::Serialization(e.to_string()))?;

        api_response.data.ok_or_else(|| TigerSwapError::Api("No data".to_string()))
    }

    /// Get swap quote
    pub async fn get_quote(
        &self,
        token_in: &str,
        token_out: &str,
        amount_in: &str,
    ) -> Result<Quote> {
        let url = format!("{}/v1/quote", self.config.base_url);

        let response = self.http_client
            .get(&url)
            .query(&[
                ("token_in", token_in),
                ("token_out", token_out),
                ("amount_in", amount_in),
            ])
            .send()
            .await
            .map_err(|e| TigerSwapError::Network(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::Api(format!("Status: {}", response.status())));
        }

        let api_response: APIResponse<Quote> = response
            .json()
            .await
            .map_err(|e| TigerSwapError::Serialization(e.to_string()))?;

        api_response.data.ok_or_else(|| TigerSwapError::Api("No data".to_string()))
    }

    /// Execute swap
    pub async fn swap(&self, request: SwapRequest) -> Result<SwapResponse> {
        let url = format!("{}/v1/swap", self.config.base_url);

        let response = self.http_client
            .post(&url)
            .json(&request)
            .send()
            .await
            .map_err(|e| TigerSwapError::Network(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::Api(format!("Status: {}", response.status())));
        }

        let api_response: APIResponse<SwapResponse> = response
            .json()
            .await
            .map_err(|e| TigerSwapError::Serialization(e.to_string()))?;

        api_response.data.ok_or_else(|| TigerSwapError::Api("No data".to_string()))
    }

    /// Create limit order
    pub async fn create_order(&self, order: Order) -> Result<Order> {
        let url = format!("{}/v1/orders", self.config.base_url);

        let response = self.http_client
            .post(&url)
            .json(&order)
            .send()
            .await
            .map_err(|e| TigerSwapError::Network(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::Api(format!("Status: {}", response.status())));
        }

        let api_response: APIResponse<Order> = response
            .json()
            .await
            .map_err(|e| TigerSwapError::Serialization(e.to_string()))?;

        api_response.data.ok_or_else(|| TigerSwapError::Api("No data".to_string()))
    }

    /// Cancel order
    pub async fn cancel_order(&self, order_id: &str) -> Result<Order> {
        let url = format!("{}/v1/orders/{}", self.config.base_url, order_id);

        let response = self.http_client
            .delete(&url)
            .send()
            .await
            .map_err(|e| TigerSwapError::Network(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::Api(format!("Status: {}", response.status())));
        }

        let api_response: APIResponse<Order> = response
            .json()
            .await
            .map_err(|e| TigerSwapError::Serialization(e.to_string()))?;

        api_response.data.ok_or_else(|| TigerSwapError::Api("No data".to_string()))
    }

    /// Get order
    pub async fn get_order(&self, order_id: &str) -> Result<Order> {
        let url = format!("{}/v1/orders/{}", self.config.base_url, order_id);

        let response = self.http_client
            .get(&url)
            .send()
            .await
            .map_err(|e| TigerSwapError::Network(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::Api(format!("Status: {}", response.status())));
        }

        let api_response: APIResponse<Order> = response
            .json()
            .await
            .map_err(|e| TigerSwapError::Serialization(e.to_string()))?;

        api_response.data.ok_or_else(|| TigerSwapError::Api("No data".to_string()))
    }

    /// Create DCA plan
    pub async fn create_dca(&self, plan: DCAPlan) -> Result<DCAPlan> {
        let url = format!("{}/v1/dca", self.config.base_url);

        let response = self.http_client
            .post(&url)
            .json(&plan)
            .send()
            .await
            .map_err(|e| TigerSwapError::Network(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::Api(format!("Status: {}", response.status())));
        }

        let api_response: APIResponse<DCAPlan> = response
            .json()
            .await
            .map_err(|e| TigerSwapError::Serialization(e.to_string()))?;

        api_response.data.ok_or_else(|| TigerSwapError::Api("No data".to_string()))
    }

    /// Get pool info
    pub async fn get_pool(&self, token_a: &str, token_b: &str) -> Result<PoolInfo> {
        let url = format!("{}/v1/pool/{}", self.config.base_url, format!("{}/{}", token_a, token_b));

        let response = self.http_client
            .get(&url)
            .send()
            .await
            .map_err(|e| TigerSwapError::Network(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::Api(format!("Status: {}", response.status())));
        }

        let api_response: APIResponse<PoolInfo> = response
            .json()
            .await
            .map_err(|e| TigerSwapError::Serialization(e.to_string()))?;

        api_response.data.ok_or_else(|| TigerSwapError::Api("No data".to_string()))
    }

    /// Get network status
    pub async fn get_network_status(&self, chain_id: u64) -> Result<NetworkStatus> {
        let url = format!("{}/v1/network/{}", self.config.base_url, chain_id);

        let response = self.http_client
            .get(&url)
            .send()
            .await
            .map_err(|e| TigerSwapError::Network(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::Api(format!("Status: {}", response.status())));
        }

        let api_response: APIResponse<NetworkStatus> = response
            .json()
            .await
            .map_err(|e| TigerSwapError::Serialization(e.to_string()))?;

        api_response.data.ok_or_else(|| TigerSwapError::Api("No data".to_string()))
    }

    /// Estimate gas
    pub async fn estimate_gas(&self, request: &SwapRequest) -> Result<GasEstimate> {
        let url = format!("{}/v1/gas/estimate", self.config.base_url);

        let response = self.http_client
            .post(&url)
            .json(request)
            .send()
            .await
            .map_err(|e| TigerSwapError::Network(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::Api(format!("Status: {}", response.status())));
        }

        let api_response: APIResponse<GasEstimate> = response
            .json()
            .await
            .map_err(|e| TigerSwapError::Serialization(e.to_string()))?;

        api_response.data.ok_or_else(|| TigerSwapError::Api("No data".to_string()))
    }
}

impl Default for TigerSwapClient {
    fn default() -> Self {
        Self::new(Config::default()).expect("Failed to create client")
    }
}