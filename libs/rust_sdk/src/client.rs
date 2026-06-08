//! TigerSwap Client
//! 
//! HTTP client for TigerSwap API

use crate::types::*;
use crate::errors::TigerSwapError;
use reqwest::Client;
use std::sync::Arc;
use tokio::sync::RwLock;

/// TigerSwap API Client
pub struct TigerSwapClient {
    client: Client,
    config: crate::Config,
    auth_token: Arc<RwLock<Option<String>>>,
}

impl TigerSwapClient {
    /// Create a new client
    pub async fn new(config: crate::Config) -> Result<Self, TigerSwapError> {
        let client = Client::builder()
            .timeout(std::time::Duration::from_secs(config.timeout))
            .build()
            .map_err(|e| TigerSwapError::ConfigError(e.to_string()))?;

        Ok(Self {
            client,
            config,
            auth_token: Arc::new(RwLock::new(None)),
        })
    }

    /// Set authentication token
    pub async fn set_auth_token(&self, token: String) {
        let mut auth = self.auth_token.write().await;
        *auth = Some(token);
    }

    /// Get token quote
    pub async fn get_quote(&self, from: &str, to: &str, amount: &str) -> Result<Quote, TigerSwapError> {
        let url = format!("{}/v1/quote?from={}&to={}&amount={}", 
            self.config.endpoint, from, to, amount);
        
        let response = self.client.get(&url)
            .send()
            .await
            .map_err(|e| TigerSwapError::RequestError(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::ApiError(response.status().as_u16(), "Quote failed".to_string()));
        }

        response.json().await
            .map_err(|e| TigerSwapError::ParseError(e.to_string()))
    }

    /// Execute swap
    pub async fn swap(&self, request: &SwapRequest) -> Result<SwapResponse, TigerSwapError> {
        let url = format!("{}/v1/swap", self.config.endpoint);
        
        let response = self.client.post(&url)
            .json(request)
            .send()
            .await
            .map_err(|e| TigerSwapError::RequestError(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::ApiError(response.status().as_u16(), "Swap failed".to_string()));
        }

        response.json().await
            .map_err(|e| TigerSwapError::ParseError(e.to_string()))
    }

    /// Get supported tokens
    pub async fn get_tokens(&self, chain_id: u64) -> Result<Vec<Token>, TigerSwapError> {
        let url = format!("{}/v1/tokens?chain_id={}", self.config.endpoint, chain_id);
        
        let response = self.client.get(&url)
            .send()
            .await
            .map_err(|e| TigerSwapError::RequestError(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::ApiError(response.status().as_u16(), "Failed to get tokens".to_string()));
        }

        response.json().await
            .map_err(|e| TigerSwapError::ParseError(e.to_string()))
    }

    /// Get order book
    pub async fn get_orderbook(&self, pair: &str) -> Result<OrderBook, TigerSwapError> {
        let url = format!("{}/v1/orderbook/{}", self.config.endpoint, pair);
        
        let response = self.client.get(&url)
            .send()
            .await
            .map_err(|e| TigerSwapError::RequestError(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::ApiError(response.status().as_u16(), "Failed to get orderbook".to_string()));
        }

        response.json().await
            .map_err(|e| TigerSwapError::ParseError(e.to_string()))
    }

    /// Get market data
    pub async fn get_market(&self, pair: &str) -> Result<MarketData, TigerSwapError> {
        let url = format!("{}/v1/market/{}", self.config.endpoint, pair);
        
        let response = self.client.get(&url)
            .send()
            .await
            .map_err(|e| TigerSwapError::RequestError(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::ApiError(response.status().as_u16(), "Failed to get market".to_string()));
        }

        response.json().await
            .map_err(|e| TigerSwapError::ParseError(e.to_string()))
    }

    /// Get user's orders
    pub async fn get_orders(&self, user: &str) -> Result<Vec<Order>, TigerSwapError> {
        let url = format!("{}/v1/orders?user={}", self.config.endpoint, user);
        
        let response = self.client.get(&url)
            .send()
            .await
            .map_err(|e| TigerSwapError::RequestError(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::ApiError(response.status().as_u16(), "Failed to get orders".to_string()));
        }

        response.json().await
            .map_err(|e| TigerSwapError::ParseError(e.to_string()))
    }

    /// Create order
    pub async fn create_order(&self, order: &Order) -> Result<Order, TigerSwapError> {
        let url = format!("{}/v1/orders", self.config.endpoint);
        
        let response = self.client.post(&url)
            .json(order)
            .send()
            .await
            .map_err(|e| TigerSwapError::RequestError(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::ApiError(response.status().as_u16(), "Failed to create order".to_string()));
        }

        response.json().await
            .map_err(|e| TigerSwapError::ParseError(e.to_string()))
    }

    /// Cancel order
    pub async fn cancel_order(&self, order_id: &str) -> Result<(), TigerSwapError> {
        let url = format!("{}/v1/orders/{}", self.config.endpoint, order_id);
        
        let response = self.client.delete(&url)
            .send()
            .await
            .map_err(|e| TigerSwapError::RequestError(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::ApiError(response.status().as_u16(), "Failed to cancel order".to_string()));
        }

        Ok(())
    }

    /// Get positions
    pub async fn get_positions(&self, user: &str) -> Result<Vec<Position>, TigerSwapError> {
        let url = format!("{}/v1/positions?user={}", self.config.endpoint, user);
        
        let response = self.client.get(&url)
            .send()
            .await
            .map_err(|e| TigerSwapError::RequestError(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::ApiError(response.status().as_u16(), "Failed to get positions".to_string()));
        }

        response.json().await
            .map_err(|e| TigerSwapError::ParseError(e.to_string()))
    }

    /// Get portfolio
    pub async fn get_portfolio(&self, user: &str) -> Result<Portfolio, TigerSwapError> {
        let url = format!("{}/v1/portfolio?user={}", self.config.endpoint, user);
        
        let response = self.client.get(&url)
            .send()
            .await
            .map_err(|e| TigerSwapError::RequestError(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::ApiError(response.status().as_u16(), "Failed to get portfolio".to_string()));
        }

        response.json().await
            .map_err(|e| TigerSwapError::ParseError(e.to_string()))
    }

    /// Get pools
    pub async fn get_pools(&self) -> Result<Vec<Pool>, TigerSwapError> {
        let url = format!("{}/v1/pools", self.config.endpoint);
        
        let response = self.client.get(&url)
            .send()
            .await
            .map_err(|e| TigerSwapError::RequestError(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::ApiError(response.status().as_u16(), "Failed to get pools".to_string()));
        }

        response.json().await
            .map_err(|e| TigerSwapError::ParseError(e.to_string()))
    }

    /// Get supported chains
    pub async fn get_chains(&self) -> Result<Vec<Chain>, TigerSwapError> {
        let url = format!("{}/v1/chains", self.config.endpoint);
        
        let response = self.client.get(&url)
            .send()
            .await
            .map_err(|e| TigerSwapError::RequestError(e.to_string()))?;

        if !response.status().is_success() {
            return Err(TigerSwapError::ApiError(response.status().as_u16(), "Failed to get chains".to_string()));
        }

        response.json().await
            .map_err(|e| TigerSwapError::ParseError(e.to_string()))
    }
}
