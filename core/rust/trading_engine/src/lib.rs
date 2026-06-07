//! TigerSwap Trading Engine - High-Performance DEX Core

pub mod token;
pub mod amm;
pub mod pool;
pub mod trading;
pub mod risk;
pub mod oracle;
pub mod mev;
pub mod chain;

pub use token::{Token, TokenType, TokenRegistry};
pub use amm::{AMM, AMMPool};
pub use pool::PoolManager;
pub use trading::{TradingEngine, SwapRequest, SwapResult, QuoteRequest, QuoteResult};
pub use risk::{RiskEngine, RiskLevel};
pub use oracle::TWAPOracle;
pub use mev::MEVProtection;
pub use chain::{ChainConfig, ChainRegistry};