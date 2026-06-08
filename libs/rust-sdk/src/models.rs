//! Data models for TigerSwap SDK

use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Token information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Token {
    pub address: String,
    pub symbol: String,
    pub name: String,
    pub decimals: u8,
    pub chain_id: u64,
    pub logo_url: Option<String>,
}

/// Token pair
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TokenPair {
    pub token_in: Token,
    pub token_out: Token,
    pub chain_id: u64,
}

/// Swap quote response
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Quote {
    pub pair: TokenPair,
    pub amount_in: String,
    pub amount_out: String,
    pub price_impact: String,
    pub gas_estimate: String,
    pub route: Vec<RouteHop>,
}

/// Route hop
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RouteHop {
    pub dex: String,
    pub from_token: String,
    pub to_token: String,
    pub proportion: String,
}

/// Swap request
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SwapRequest {
    pub token_in: String,
    pub token_out: String,
    pub amount_in: String,
    pub amount_out_min: String,
    pub recipient: Option<String>,
    pub slippage_tolerance: Option<String>,
}

/// Swap response
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SwapResponse {
    pub tx_hash: String,
    pub amount_in: String,
    pub amount_out: String,
    pub gas_used: String,
    pub price_impact: String,
}

/// Order types
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum OrderType {
    Limit,
    StopLoss,
    TakeProfit,
    Market,
    Gtd,
    Ioc,
    Fok,
}

/// Order status
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum OrderStatus {
    Pending,
    PartiallyFilled,
    Filled,
    Cancelled,
    Expired,
}

/// Order side
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum Side {
    Buy,
    Sell,
}

/// Order
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Order {
    pub id: String,
    pub owner: String,
    pub token_in: String,
    pub token_out: String,
    pub amount_in: String,
    pub amount_out: String,
    pub price: String,
    pub stop_price: Option<String>,
    pub order_type: OrderType,
    pub side: Side,
    pub status: OrderStatus,
    pub created_at: String,
    pub expires_at: Option<String>,
    pub filled_amount: String,
}

/// DCA plan
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DCAPlan {
    pub id: String,
    pub owner: String,
    pub token_in: String,
    pub token_out: String,
    pub amount_per_execution: String,
    pub interval_seconds: u64,
    pub executions_completed: u64,
    pub max_executions: Option<u64>,
    pub status: String,
    pub next_execution: String,
    pub created_at: String,
}

/// Position (for perpetuals)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Position {
    pub id: String,
    pub owner: String,
    pub collateral_token: String,
    pub index_token: String,
    pub is_long: bool,
    pub size: String,
    pub collateral: String,
    pub average_price: String,
    pub unrealized_pnl: String,
    pub liquidation_price: String,
    pub status: String,
}

/// Pool info
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PoolInfo {
    pub address: String,
    pub token0: String,
    pub token1: String,
    pub reserve0: String,
    pub reserve1: String,
    pub liquidity: String,
    pub fee_tier: u32,
}

/// Token balance
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TokenBalance {
    pub token: Token,
    pub balance: String,
    pub balance_raw: String,
    pub allowance: String,
}

/// User portfolio
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Portfolio {
    pub address: String,
    pub tokens: Vec<TokenBalance>,
    pub total_value_usd: String,
}

/// Transaction receipt
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TransactionReceipt {
    pub tx_hash: String,
    pub block_number: u64,
    pub status: bool,
    pub gas_used: String,
    pub logs: Vec<TransactionLog>,
}

/// Transaction log
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TransactionLog {
    pub address: String,
    pub topics: Vec<String>,
    pub data: String,
}

/// Gas estimate
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GasEstimate {
    pub gas_price: String,
    pub gas_limit: String,
    pub total_cost: String,
    pub token: String,
}

/// Network status
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NetworkStatus {
    pub chain_id: u64,
    pub block_number: u64,
    pub synced: bool,
    pub gas_price: String,
}

/// API response wrapper
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct APIResponse<T> {
    pub success: bool,
    pub data: Option<T>,
    pub error: Option<String>,
}

/// Paginated response
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PaginatedResponse<T> {
    pub items: Vec<T>,
    pub page: u32,
    pub page_size: u32,
    pub total: u32,
    pub has_more: bool,
}