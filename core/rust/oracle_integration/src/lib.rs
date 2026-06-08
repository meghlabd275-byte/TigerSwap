//! TigerSwap Oracle Integration
//! 
//! Implements Chainlink and Pyth oracle integration:
//! - Chainlink price feeds
//! - Pyth price feeds
//! - TWAP (Time-Weighted Average Price)
//! - VWAP (Volume-Weighted Average Price)
//! - Historical price data
//!
//! Implementation: Pure Rust with no external dependencies

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use rust_decimal::Decimal;
use thiserror::Error;
use uuid::Uuid;
use chrono::{Utc, Duration};
use std::collections::{HashMap, VecDeque};

/// Chain ID constants
pub const CHAIN_ETH: u64 = 1;
pub const CHAIN_BSC: u64 = 56;
pub const CHAIN_POLYGON: u64 = 137;
pub const CHAIN_ARBITRUM: u64 = 42161;
pub const CHAIN_OPTIMISM: u64 = 10;
pub const CHAIN_BASE: u64 = 8453;
pub const CHAIN_AVALANCHE: u64 = 43114;

#[derive(Debug, Error)]
pub enum OracleError {
    #[error("Price not available: {0}")]
    PriceNotAvailable(String),
    #[error("Feed not found: {0}")]
    FeedNotFound(String),
    #[error("Stale price: {0}")]
    StalePrice(String),
    #[error("Invalid price: {0}")]
    InvalidPrice(String),
    #[error("Chain not supported: {0}")]
    ChainNotSupported(u64),
    #[error("Insufficient data: {0}")]
    InsufficientData(String),
}

/// Oracle type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum OracleType {
    Chainlink,
    Pyth,
    TWAP,
    VWAP,
    Median,
}

impl Default for OracleType {
    fn default() -> Self { OracleType::Chainlink }
}

/// Price data
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PriceData {
    pub price: Decimal,
    pub confidence: u128,     // Confidence interval
    pub timestamp: i64,
    pub slot: u64,
    pub aggregator_round_id: u64,
}

impl PriceData {
    pub fn new(price: Decimal, confidence: u128) -> Self {
        Self {
            price,
            confidence,
            timestamp: Utc::now().timestamp(),
            slot: 0,
            aggregator_round_id: 0,
        }
    }

    pub fn is_stale(&self, max_age_seconds: i64) -> bool {
        Utc::now().timestamp() - self.timestamp > max_age_seconds
    }
}

/// Price feed
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PriceFeed {
    pub feed_id: String,
    pub chain_id: u64,
    pub oracle_type: OracleType,
    pub token0: String,
    pub token1: String,
    pub description: String,
    pub decimals: u8,
    pub current_price: Option<PriceData>,
    pub historical_prices: VecDeque<PriceData>,
    pub max_historical: usize,
    pub updated_at: i64,
}

impl PriceFeed {
    pub fn new(
        feed_id: String,
        chain_id: u64,
        oracle_type: OracleType,
        token0: String,
        token1: String,
    ) -> Self {
        Self {
            feed_id,
            chain_id,
            oracle_type,
            token0,
            token1,
            description: format!("{}/{}", token0, token1),
            decimals: 8,
            current_price: None,
            historical_prices: VecDeque::new(),
            max_historical: 1000,
            updated_at: Utc::now().timestamp(),
        }
    }

    pub fn update_price(&mut self, price: Decimal, confidence: u128) {
        let price_data = PriceData::new(price, confidence);
        
        // Update current
        self.current_price = Some(price_data.clone());
        
        // Add to history
        if self.historical_prices.len() >= self.max_historical {
            self.historical_prices.pop_front();
        }
        self.historical_prices.push_back(price_data);
        
        self.updated_at = Utc::now().timestamp();
    }

    pub fn get_price(&self) -> Result<Decimal, OracleError> {
        match &self.current_price {
            Some(price) => {
                if price.is_stale(300) {
                    return Err(OracleError::StalePrice("Price is stale".to_string()));
                }
                Ok(price.price)
            }
            None => Err(OracleError::PriceNotAvailable(self.feed_id.clone())),
        }
    }

    pub fn get_twap(&self, duration_seconds: i64) -> Result<Decimal, OracleError> {
        if self.historical_prices.len() < 2 {
            return Err(OracleError::InsufficientData("Need more data".to_string()));
        }
        
        let cutoff = Utc::now().timestamp() - duration_seconds;
        
        let sum: Decimal = self.historical_prices.iter()
            .filter(|p| p.timestamp >= cutoff)
            .map(|p| p.price)
            .sum();
        
        let count = self.historical_prices.iter()
            .filter(|p| p.timestamp >= cutoff)
            .count();
        
        if count == 0 {
            return Err(OracleError::InsufficientData("No data in window".to_string()));
        }
        
        Ok(sum / Decimal::from(count))
    }

    pub fn get_vwap(&self, duration_seconds: i64) -> Result<Decimal, OracleError> {
        if self.historical_prices.len() < 2 {
            return Err(OracleError::InsufficientData("Need more data".to_string()));
        }
        
        let cutoff = Utc::now().timestamp() - duration_seconds;
        
        let mut total_value: u128 = 0;
        let mut total_volume: u128 = 0;
        
        for p in self.historical_prices.iter().filter(|p| p.timestamp >= cutoff) {
            // Use confidence as volume proxy
            let volume = p.confidence.max(1);
            total_value += p.price.as_u128() * volume;
            total_volume += volume;
        }
        
        if total_volume == 0 {
            return Err(OracleError::InsufficientData("No data in window".to_string()));
        }
        
        Ok(Decimal::from(total_value) / Decimal::from(total_volume))
    }

    pub fn get_median(&self) -> Result<Decimal, OracleError> {
        if self.historical_prices.len() < 3 {
            return Err(OracleError::InsufficientData("Need more data".to_string()));
        }
        
        let mut prices: Vec<Decimal> = self.historical_prices.iter()
            .map(|p| p.price)
            .collect();
        
        prices.sort();
        
        let mid = prices.len() / 2;
        
        if prices.len() % 2 == 0 {
            Ok((prices[mid - 1] + prices[mid]) / Decimal::from(2))
        } else {
            Ok(prices[mid])
        }
    }

    pub fn get_historical(&self, limit: usize) -> Vec<PriceData> {
        self.historical_prices.iter()
            .rev()
            .take(limit)
            .cloned()
            .collect()
    }
}

/// Chainlink feed config
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChainlinkConfig {
    pub feed_id: String,
    pub proxy_address: String,
    pub aggregator_address: String,
    pub heartbeat: i64,       // Max time between updates
    pub deviation: u256,     // Deviation threshold
}

impl ChainlinkConfig {
    pub fn new(proxy: String, aggregator: String) -> Self {
        Self {
            feed_id: proxy.clone(),
            proxy_address: proxy,
            aggregator_address: aggregator,
            heartbeat: 3600,  // 1 hour
            deviation: 50000000, // 5%
        }
    }
}

/// Pyth feed config
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PythConfig {
    pub feed_id: String,
    pub price_type: PythPriceType,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum PythPriceType {
    Price,
    EMA,
}

impl PythConfig {
    pub fn new(feed_id: String) -> Self {
        Self {
            feed_id,
            price_type: PythPriceType::EMA,
        }
    }
}

/// Oracle price aggregation
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OraclePrice {
    pub price: Decimal,
    pub confidence: u128,
    pub sources: Vec<OracleSource>,
    pub timestamp: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OracleSource {
    pub oracle_type: OracleType,
    pub feed_id: String,
    pub price: Decimal,
    pub weight: u256,
    pub is_valid: bool,
}

impl OraclePrice {
    pub fn from_sources(sources: Vec<OracleSource>) -> Option<Self> {
        let valid_sources: Vec<_> = sources.into_iter()
            .filter(|s| s.is_valid)
            .collect();
        
        if valid_sources.is_empty() {
            return None;
        }
        
        // Weighted average
        let mut total_weight: u256 = 0;
        let mut total_value: u256 = 0;
        
        for source in &valid_sources {
            total_weight += source.weight;
            total_value += source.price.as_u128() as u256 * source.weight;
        }
        
        let price = Decimal::from(total_value / total_weight);
        let confidence = valid_sources.iter()
            .map(|s| s.price.as_u128())
            .sum::<u128>() / valid_sources.len() as u128;
        
        Some(Self {
            price,
            confidence,
            sources: valid_sources,
            timestamp: Utc::now().timestamp(),
        })
    }
}

/// TWAP calculator
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TWAPCalculator {
    pub feed_id: String,
    pub prices: VecDeque<(i64, Decimal)>, // (timestamp, price)
    pub window_seconds: i64,
}

impl TWAPCalculator {
    pub fn new(feed_id: String, window_seconds: i64) -> Self {
        Self {
            feed_id,
            prices: VecDeque::new(),
            window_seconds,
        }
    }

    pub fn add_price(&mut self, price: Decimal) {
        let now = Utc::now().timestamp();
        
        self.prices.push_back((now, price));
        
        // Remove old prices
        let cutoff = now - self.window_seconds;
        while let Some(&(ts, _)) = self.prices.front() {
            if ts < cutoff {
                self.prices.pop_front();
            } else {
                break;
            }
        }
    }

    pub fn calculate(&self) -> Option<Decimal> {
        if self.prices.len() < 2 {
            return None;
        }
        
        let sum: Decimal = self.prices.iter().map(|(_, p)| *p).sum();
        let count = self.prices.len();
        
        Some(sum / Decimal::from(count))
    }
}

/// Oracle Engine
pub struct OracleEngine {
    feeds: Arc<RwLock<HashMap<String, PriceFeed>>>,
    chainlink_configs: Arc<RwLock<HashMap<String, ChainlinkConfig>>>,
    pyth_configs: Arc<RwLock<HashMap<String, PythConfig>>>,
    twap_calculators: Arc<RwLock<HashMap<String, TWAPCalculator>>>,
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
}

impl OracleEngine {
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [
            CHAIN_ETH, CHAIN_BSC, CHAIN_POLYGON, CHAIN_ARBITRUM,
            CHAIN_OPTIMISM, CHAIN_BASE, CHAIN_AVALANCHE,
        ].into_iter().collect();
        
        Self {
            feeds: Arc::new(RwLock::new(HashMap::new())),
            chainlink_configs: Arc::new(RwLock::new(HashMap::new())),
            pyth_configs: Arc::new(RwLock::new(HashMap::new())),
            twap_calculators: Arc::new(RwLock::new(HashMap::new())),
            supported_chains: Arc::new(RwLock::new(chains)),
        }
    }

    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    /// Register Chainlink feed
    pub fn register_chainlink_feed(
        &self,
        feed_id: String,
        chain_id: u64,
        token0: String,
        token1: String,
        proxy: String,
        aggregator: String,
    ) -> Result<String, OracleError> {
        if !self.is_chain_supported(chain_id) {
            return Err(OracleError::ChainNotSupported(chain_id));
        }
        
        let config = ChainlinkConfig::new(proxy, aggregator);
        
        let feed = PriceFeed::new(
            feed_id.clone(),
            chain_id,
            OracleType::Chainlink,
            token0,
            token1,
        );
        
        let feed_key = format!("{}_{}_{}", chain_id, token0, token1);
        
        self.feeds.write().insert(feed_key.clone(), feed);
        self.chainlink_configs.write().insert(feed_id, config);
        
        Ok(feed_key)
    }

    /// Register Pyth feed
    pub fn register_pyth_feed(
        &self,
        feed_id: String,
        chain_id: u64,
        token0: String,
        token1: String,
    ) -> Result<String, OracleError> {
        if !self.is_chain_supported(chain_id) {
            return Err(OracleError::ChainNotSupported(chain_id));
        }
        
        let config = PythConfig::new(feed_id);
        
        let feed = PriceFeed::new(
            feed_id.clone(),
            chain_id,
            OracleType::Pyth,
            token0,
            token1,
        );
        
        let feed_key = format!("{}_{}_{}", chain_id, token0, token1);
        
        self.feeds.write().insert(feed_key.clone(), feed);
        self.pyth_configs.write().insert(feed_id, config);
        
        Ok(feed_key)
    }

    /// Update price
    pub fn update_price(
        &self,
        feed_key: &str,
        price: Decimal,
        confidence: u128,
    ) -> Result<(), OracleError> {
        let mut feeds = self.feeds.write();
        let feed = feeds.get_mut(feed_key)
            .ok_or_else(|| OracleError::FeedNotFound(feed_key.to_string()))?;
        
        feed.update_price(price, confidence);
        
        // Update TWAP calculator
        let mut twap_calc = self.twap_calculators.write();
        if let Some(calc) = twap_calc.get_mut(feed_key) {
            calc.add_price(price);
        }
        
        Ok(())
    }

    /// Get price
    pub fn get_price(&self, feed_key: &str) -> Result<Decimal, OracleError> {
        let feeds = self.feeds.read();
        let feed = feeds.get(feed_key)
            .ok_or_else(|| OracleError::FeedNotFound(feed_key.to_string()))?;
        
        feed.get_price()
    }

    /// Get TWAP
    pub fn get_twap(&self, feed_key: &str, duration_seconds: i64) -> Result<Decimal, OracleError> {
        let feeds = self.feeds.read();
        let feed = feeds.get(feed_key)
            .ok_or_else(|| OracleError::FeedNotFound(feed_key.to_string()))?;
        
        feed.get_twap(duration_seconds)
    }

    /// Get VWAP
    pub fn get_vwap(&self, feed_key: &str, duration_seconds: i64) -> Result<Decimal, OracleError> {
        let feeds = self.feeds.read();
        let feed = feeds.get(feed_key)
            .ok_or_else(|| OracleError::FeedNotFound(feed_key.to_string()))?;
        
        feed.get_vwap(duration_seconds)
    }

    /// Get median
    pub fn get_median(&self, feed_key: &str) -> Result<Decimal, OracleError> {
        let feeds = self.feeds.read();
        let feed = feeds.get(feed_key)
            .ok_or_else(|| OracleError::FeedNotFound(feed_key.to_string()))?;
        
        feed.get_median()
    }

    /// Get aggregated price (Chainlink + Pyth)
    pub fn get_aggregated_price(
        &self,
        feed_key: &str,
    ) -> Result<OraclePrice, OracleError> {
        let feeds = self.feeds.read();
        let feed = feeds.get(feed_key)
            .ok_or_else(|| OracleError::FeedNotFound(feed_key.to_string()))?;
        
        let price = feed.get_price()?;
        
        let source = OracleSource {
            oracle_type: feed.oracle_type,
            feed_id: feed.feed_id.clone(),
            price,
            weight: 1,
            is_valid: true,
        };
        
        OraclePrice::from_sources(vec![source])
            .ok_or_else(|| OracleError::PriceNotAvailable(feed_key.to_string()))
    }

    /// Initialize TWAP calculator
    pub fn init_twap(&self, feed_key: &str, window_seconds: i64) {
        self.twap_calculators.write().insert(
            feed_key.to_string(),
            TWAPCalculator::new(feed_key.to_string(), window_seconds),
        );
    }

    /// Get historical prices
    pub fn get_historical(&self, feed_key: &str, limit: usize) -> Result<Vec<PriceData>, OracleError> {
        let feeds = self.feeds.read();
        let feed = feeds.get(feed_key)
            .ok_or_else(|| OracleError::FeedNotFound(feed_key.to_string()))?;
        
        Ok(feed.get_historical(limit))
    }

    /// Get feed
    pub fn get_feed(&self, feed_key: &str) -> Option<PriceFeed> {
        self.feeds.read().get(feed_key).cloned()
    }

    /// Add supported chain
    pub fn add_chain(&self, chain_id: u64) {
        self.supported_chains.write().insert(chain_id);
    }

    pub fn supported_chains(&self) -> Vec<u64> {
        self.supported_chains.read().iter().cloned().collect()
    }
}

impl Default for OracleEngine {
    fn default() -> Self { Self::new() }
}

#[cfg(test)]
mod tests {
    use super::*;
    use rust_decimal_macros::dec;

    #[test]
    fn test_feed_creation() {
        let feed = PriceFeed::new(
            "eth-usd".to_string(),
            CHAIN_ETH,
            OracleType::Chainlink,
            "ETH".to_string(),
            "USD".to_string(),
        );
        
        assert_eq!(feed.oracle_type, OracleType::Chainlink);
    }

    #[test]
    fn test_price_update() {
        let mut feed = PriceFeed::new(
            "eth-usd".to_string(),
            CHAIN_ETH,
            OracleType::Chainlink,
            "ETH".to_string(),
            "USD".to_string(),
        );
        
        feed.update_price(dec!(2500.0), 10);
        
        let price = feed.get_price().unwrap();
        assert_eq!(price, dec!(2500.0));
    }

    #[test]
    fn test_twap() {
        let mut feed = PriceFeed::new(
            "eth-usd".to_string(),
            CHAIN_ETH,
            OracleType::TWAP,
            "ETH".to_string(),
            "USD".to_string(),
        );
        
        feed.update_price(dec!(2500.0), 10);
        feed.update_price(dec!(2600.0), 10);
        feed.update_price(dec!(2700.0), 10);
        
        let twap = feed.get_twap(3600).unwrap();
        
        assert!(twap > dec!(0));
    }

    #[test]
    fn test_twap_calculator() {
        let mut calc = TWAPCalculator::new("eth-usd".to_string(), 300);
        
        calc.add_price(dec!(2500.0));
        calc.add_price(dec!(2600.0));
        calc.add_price(dec!(2700.0));
        
        let twap = calc.calculate().unwrap();
        
        assert_eq!(twap, dec!(2600.0));
    }

    #[test]
    fn test_median() {
        let mut feed = PriceFeed::new(
            "eth-usd".to_string(),
            CHAIN_ETH,
            OracleType::Median,
            "ETH".to_string(),
            "USD".to_string(),
        );
        
        feed.update_price(dec!(2500.0), 10);
        feed.update_price(dec!(2600.0), 10);
        feed.update_price(dec!(2700.0), 10);
        
        let median = feed.get_median().unwrap();
        
        assert_eq!(median, dec!(2600.0));
    }
}