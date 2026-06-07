use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use rust_decimal::Decimal;
use thiserror::Error;
use uuid::Uuid;
use chrono::Utc;

use crate::amm::{AMM, AMMPool};
use crate::pool::PoolManager;
use crate::token::{TokenRegistry, pool_key};

#[derive(Debug, Error)]
pub enum TradingError {
    #[error("Invalid token pair")]
    InvalidTokenPair,
    #[error("Insufficient liquidity: {0}")]
    InsufficientLiquidity(String),
    #[error("Price impact too high: {0} bps")]
    PriceImpactTooHigh(i64),
    #[error("Slippage exceeded")]
    SlippageExceeded,
    #[error("Invalid amount")]
    InvalidAmount,
    #[error("Chain not supported: {0}")]
    ChainNotSupported(u64),
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SwapRequest {
    pub request_id: String,
    pub chain_id: u64,
    pub token_in: String,
    pub token_out: String,
    pub amount_in: u128,
    pub min_amount_out: u128,
    pub recipient: String,
    pub deadline: i64,
    pub slippage_bps: i64,
}

impl SwapRequest {
    pub fn new(chain_id: u64, token_in: String, token_out: String, amount_in: u128, recipient: String) -> Self {
        Self {
            request_id: Uuid::new_v4().to_string(),
            chain_id,
            token_in,
            token_out,
            amount_in,
            min_amount_out: 0,
            recipient,
            deadline: Utc::now().timestamp() + 600,
            slippage_bps: 50,
        }
    }

    pub fn validate(&self) -> Result<(), TradingError> {
        if self.amount_in == 0 { return Err(TradingError::InvalidAmount); }
        if self.token_in == self.token_out { return Err(TradingError::InvalidTokenPair); }
        Ok(())
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SwapResult {
    pub request_id: String,
    pub success: bool,
    pub amount_in: u128,
    pub amount_out: u128,
    pub price_impact_bps: i64,
    pub effective_price: Decimal,
    pub gas_used: u64,
    pub latency_ms: i64,
    pub error: Option<String>,
}

impl SwapResult {
    pub fn success(request_id: String, amount_in: u128, amount_out: u128, price_impact_bps: i64, latency_ms: i64) -> Self {
        Self {
            request_id,
            success: true,
            amount_in,
            amount_out,
            price_impact_bps,
            effective_price: if amount_in > 0 { Decimal::from(amount_out) / Decimal::from(amount_in) } else { Decimal::ZERO },
            gas_used: 200000,
            latency_ms,
            error: None,
        }
    }

    pub fn failure(request_id: String, error: String, latency_ms: i64) -> Self {
        Self {
            request_id,
            success: false,
            amount_in: 0,
            amount_out: 0,
            price_impact_bps: 0,
            effective_price: Decimal::ZERO,
            gas_used: 0,
            latency_ms,
            error: Some(error),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QuoteRequest {
    pub chain_id: u64,
    pub token_in: String,
    pub token_out: String,
    pub amount_in: u128,
    pub slippage_bps: i64,
}

impl QuoteRequest {
    pub fn new(chain_id: u64, token_in: String, token_out: String, amount_in: u128) -> Self {
        Self { chain_id, token_in, token_out, amount_in, slippage_bps: 50 }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct QuoteResult {
    pub amount_in: u128,
    pub amount_out: u128,
    pub amount_out_min: u128,
    pub price_impact_bps: i64,
    pub effective_price: Decimal,
    pub gas_estimate: u64,
}

impl QuoteResult {
    pub fn from_swap(amount_in: u128, amount_out: u128, price_impact_bps: i64) -> Self {
        Self {
            amount_in,
            amount_out,
            amount_out_min: amount_out * 995 / 1000,
            price_impact_bps,
            effective_price: if amount_in > 0 { Decimal::from(amount_out) / Decimal::from(amount_in) } else { Decimal::ZERO },
            gas_estimate: 150000,
        }
    }
}

pub struct TradingEngine {
    amm: Arc<AMM>,
    pool_manager: Arc<PoolManager>,
    token_registry: Arc<TokenRegistry>,
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
}

impl TradingEngine {
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [1, 56, 137, 42161, 10, 8453, 43114, 0].into_iter().collect();
        Self {
            amm: Arc::new(AMM::new()),
            pool_manager: Arc::new(PoolManager::new()),
            token_registry: Arc::new(TokenRegistry::with_defaults()),
            supported_chains: Arc::new(RwLock::new(chains)),
        }
    }

    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    pub fn get_quote(&self, req: &QuoteRequest) -> Result<QuoteResult, TradingError> {
        if !self.is_chain_supported(req.chain_id) { return Err(TradingError::ChainNotSupported(req.chain_id)); }
        
        let (amount_out, price_impact_bps) = self.amm.quote(&req.token_in, &req.token_out, req.amount_in)
            .ok_or_else(|| TradingError::InsufficientLiquidity("Pool not found".to_string()))?;
        
        Ok(QuoteResult::from_swap(req.amount_in, amount_out, price_impact_bps))
    }

    pub fn execute_swap(&self, req: &SwapRequest) -> Result<SwapResult, TradingError> {
        req.validate()?;
        if !self.is_chain_supported(req.chain_id) { return Err(TradingError::ChainNotSupported(req.chain_id)); }
        
        let start = std::time::Instant::now();
        
        let (amount_out, price_impact_bps) = self.amm.quote(&req.token_in, &req.token_out, req.amount_in)
            .ok_or_else(|| TradingError::InsufficientLiquidity("Pool not found".to_string()))?;
        
        let min_required = req.amount_in * (10000 - req.slippage_bps as u128) / 10000;
        if amount_out < min_required { return Err(TradingError::SlippageExceeded); }
        
        let latency_ms = start.elapsed().as_millis() as i64;
        Ok(SwapResult::success(req.request_id.clone(), req.amount_in, amount_out, price_impact_bps, latency_ms))
    }

    pub fn register_pool(&self, pool: AMMPool) { self.amm.register_pool(pool); }
    pub fn supported_chains(&self) -> Vec<u64> { self.supported_chains.read().iter().cloned().collect() }
}

impl Default for TradingEngine { fn default() -> Self { Self::new() } }