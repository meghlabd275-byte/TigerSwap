//! TigerSwap SDK Types
//! 
//! Core data types for the TigerSwap SDK

use serde::{Deserialize, Serialize};

/// Token pair information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Token {
    pub address: String,
    pub symbol: String,
    pub name: String,
    pub decimals: u8,
    pub chain_id: u64,
    pub logo_url: Option<String>,
}

/// Trading pair
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Pair {
    pub token_a: Token,
    pub token_b: Token,
    pub address: String,
    pub factory: String,
}

/// Swap quote
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Quote {
    pub from_token: String,
    pub to_token: String,
    pub from_amount: String,
    pub to_amount: String,
    pub price_impact: String,
    pub gas_estimate: String,
    pub route: Vec<String>,
}

/// Order side
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum OrderSide {
    Buy,
    Sell,
}

/// Order type
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
pub enum OrderType {
    Market,
    Limit,
    StopLoss,
    TakeProfit,
    StopLimit,
}

/// Order status
#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum OrderStatus {
    Pending,
    Open,
    PartiallyFilled,
    Filled,
    Cancelled,
    Expired,
}

/// Order details
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Order {
    pub id: String,
    pub user: String,
    pub pair: String,
    pub side: OrderSide,
    pub order_type: OrderType,
    pub price: String,
    pub quantity: String,
    pub filled: String,
    pub status: OrderStatus,
    pub created_at: u64,
    pub expires_at: u64,
}

/// Position for perpetual trading
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Position {
    pub id: String,
    pub user: String,
    pub pair: String,
    pub side: String, // "long" or "short"
    pub size: String,
    pub collateral: String,
    pub leverage: String,
    pub entry_price: String,
    pub mark_price: String,
    pub pnl: String,
    pub roe: String,
    pub liquidation_price: String,
    pub status: String,
}

/// Portfolio information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Portfolio {
    pub user: String,
    pub total_value: String,
    pub positions: Vec<Position>,
    pub balances: Vec<TokenBalance>,
}

/// Token balance
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TokenBalance {
    pub token: Token,
    pub balance: String,
    pub value_usd: String,
}

/// Market data
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MarketData {
    pub pair: String,
    pub price: String,
    pub price_24h_ago: String,
    pub change_24h: String,
    pub volume_24h: String,
    pub liquidity: String,
    pub high_24h: String,
    pub low_24h: String,
}

/// Pool information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Pool {
    pub address: String,
    pub token_a: String,
    pub token_b: String,
    pub reserve_a: String,
    pub reserve_b: String,
    pub total_supply: String,
    pub fee: String,
}

/// Liquidity position
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LiquidityPosition {
    pub id: String,
    pub user: String,
    pub pool: String,
    pub liquidity: String,
    pub token_a_balance: String,
    pub token_b_balance: String,
    pub fee_earned: String,
}

/// Swap transaction request
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SwapRequest {
    pub from_token: String,
    pub to_token: String,
    pub amount: String,
    pub slippage_tolerance: u32, // basis points
    pub to: String,
}

/// Swap transaction response
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SwapResponse {
    pub tx_hash: String,
    pub from_token: String,
    pub to_token: String,
    pub from_amount: String,
    pub to_amount: String,
    pub status: String,
}

/// Chain information
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Chain {
    pub id: u64,
    pub name: String,
    pub symbol: String,
    pub rpc_url: String,
    pub explorer_url: String,
    pub native_token: String,
    pub is_active: bool,
}

/// Price ticker
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Ticker {
    pub pair: String,
    pub price: String,
    pub change_1h: String,
    pub change_24h: String,
    pub volume_24h: String,
    pub high_24h: String,
    pub low_24h: String,
}

/// Order book entry
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OrderBookEntry {
    pub price: String,
    pub quantity: String,
}

/// Order book
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OrderBook {
    pub pair: String,
    pub bids: Vec<OrderBookEntry>,
    pub asks: Vec<OrderBookEntry>,
    pub updated_at: u64,
}

/// Trade history
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Trade {
    pub id: String,
    pub pair: String,
    pub side: OrderSide,
    pub price: String,
    pub quantity: String,
    pub timestamp: u64,
    pub tx_hash: String,
}

/// User transactions
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Transaction {
    pub hash: String,
    pub from: String,
    pub to: String,
    pub token: String,
    pub amount: String,
    pub status: String,
    pub timestamp: u64,
    pub block_number: u64,
    pub gas_used: String,
    pub gas_price: String,
}
