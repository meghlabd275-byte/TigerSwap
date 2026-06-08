//! TigerSwap Oracle Aggregator
//! 
//! Multi-source price oracle aggregation:
//! - Chainlink integration
//! - Uniswap TWAP
//! - Band Protocol
//! - Median deviation protection
//! - Staleness protection
//!
//! Uses Rust for high performance

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use serde::{Deserialize, Serialize};
use uint::construct_uint;

construct_uint! {
    pub struct U256(4);
}

// ==================== ORACLE TYPES ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum OracleSource {
    Chainlink,
    UniswapTWAP,
    BandProtocol,
    TigerSwap,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PriceData {
    pub value: U256,
    pub timestamp: u64,
    pub source: OracleSource,
    pub confidence: U256,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AggregatedPrice {
    pub value: U256,
    pub timestamp: u64,
    pub sources: Vec<OracleSource>,
    pub deviation: U256,
    pub is_stale: bool,
}

// ==================== ORACLE CONFIG ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OracleConfig {
    pub max_staleness_seconds: u64,
    pub max_deviation_bps: u64,
    pub min_sources: u8,
    pub heartbeat_seconds: u64,
}

impl Default for OracleConfig {
    fn default() -> Self {
        Self {
            max_staleness_seconds: 300, // 5 minutes
            max_deviation_bps: 500, // 5%
            min_sources: 2,
            heartbeat_seconds: 60,
        }
    }
}

// ==================== PRICE FEED ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PriceFeed {
    pub pair: [u8; 20],  // Trading pair (tokenA-tokenB)
    pub prices: HashMap<OracleSource, PriceData>,
    pub last_update: u64,
}

impl PriceFeed {
    pub fn new(pair: [u8; 20]) -> Self {
        Self {
            pair,
            prices: HashMap::new(),
            last_update: 0,
        }
    }
    
    pub fn update_price(&mut self, source: OracleSource, value: U256, confidence: U256) {
        self.prices.insert(source, PriceData {
            value,
            timestamp: current_timestamp(),
            source,
            confidence,
        });
        self.last_update = current_timestamp();
    }
    
    pub fn get_price(&self, source: OracleSource) -> Option<&PriceData> {
        self.prices.get(&source)
    }
    
    pub fn is_stale(&self, max_staleness: u64) -> bool {
        if self.prices.is_empty() {
            return true;
        }
        
        current_timestamp() - self.last_update > max_staleness
    }
    
    pub fn calculate_median(&self) -> Option<U256> {
        if self.prices.is_empty() {
            return None;
        }
        
        let mut values: Vec<U256> = self.prices.values()
            .map(|p| p.value)
            .collect();
        
        values.sort();
        
        let len = values.len();
        if len % 2 == 0 {
            (values[len/2 - 1] + values[len/2]) / U256::from(2)
        } else {
            values[len/2]
        }
    }
    
    pub fn remove_outliers(&mut self, max_deviation_bps: u64) {
        if let Some(median) = self.calculate_median() {
            let mut prices_to_remove = Vec::new();
            
            for (source, data) in &self.prices {
                let deviation = calculate_deviation(&data.value, &median);
                if deviation > U256::from(max_deviation_bps) {
                    prices_to_remove.push(*source);
                }
            }
            
            for source in prices_to_remove {
                self.prices.remove(&source);
            }
        }
    }
}

// ==================== ORACLE AGGREGATOR ====================

pub struct OracleAggregator {
    feeds: Arc<RwLock<HashMap<[u8; 20], PriceFeed>>>,
    config: OracleConfig,
    chainlink_addresses: HashMap<[u8; 20], address>,
    uniswap_pools: HashMap<[u8; 20], address>,
    fallback_enabled: bool,
}

impl OracleAggregator {
    pub fn new(config: OracleConfig) -> Self {
        Self {
            feeds: Arc::new(RwLock::new(HashMap::new())),
            config,
            chainlink_addresses: HashMap::new(),
            uniswap_pools: HashMap::new(),
            fallback_enabled: true,
        }
    }
    
    // ==================== CONFIGURATION ====================
    
    pub fn add_chainlink_feed(&mut self, pair: [u8; 20], chainlink_address: address) {
        self.chainlink_addresses.insert(pair, chainlink_address);
    }
    
    pub fn add_uniswap_pool(&mut self, pair: [u8; 20], pool_address: address) {
        self.uniswap_pools.insert(pair, pool_address);
    }
    
    // ==================== PRICE UPDATES ====================
    
    pub async fn update_price(&self, pair: [u8; 20], source: OracleSource, value: U256, confidence: U256) {
        let mut feeds = self.feeds.write().await;
        
        let feed = feeds.entry(pair).or_insert_with(|| PriceFeed::new(pair));
        feed.update_price(source, value, confidence);
    }
    
    // Simulate Chainlink update (in production, would call Chainlink oracle)
    pub async fn update_from_chainlink(&self, pair: [u8; 20], value: U256, confidence: U256) {
        self.update_price(pair, OracleSource::Chainlink, value, confidence).await;
    }
    
    // Simulate Uniswap TWAP update
    pub async fn update_from_uniswap(&self, pair: [u8; 20], value: U256) {
        self.update_price(pair, OracleSource::UniswapTWAP, value, U256::from(50)).await;
    }
    
    // Simulate Band Protocol update
    pub async fn update_from_band(&self, pair: [u8; 20], value: U256) {
        self.update_price(pair, OracleSource::BandProtocol, value, U256::from(30)).await;
    }
    
    // ==================== PRICE QUERIES ====================
    
    pub async fn get_price(&self, pair: [u8; 20]) -> Option<AggregatedPrice> {
        let feeds = self.feeds.read().await;
        
        if let Some(feed) = feeds.get(&pair) {
            // Remove outliers
            let mut feed = feed.clone();
            feed.remove_outliers(self.config.max_deviation_bps);
            
            // Check staleness
            let is_stale = feed.is_stale(self.config.max_staleness_seconds);
            
            // Check minimum sources
            if feed.prices.len() < self.config.min_sources as usize && !self.fallback_enabled {
                return None;
            }
            
            // Calculate median
            let value = feed.calculate_median()?;
            
            // Get sources
            let sources: Vec<OracleSource> = feed.prices.keys().cloned().collect();
            
            // Calculate deviation
            let deviation = self.calculate_max_deviation(&feed)?;
            
            Some(AggregatedPrice {
                value,
                timestamp: current_timestamp(),
                sources,
                deviation,
                is_stale,
            })
        } else {
            None
        }
    }
    
    pub async fn get_price_from_source(&self, pair: [u8; 20], source: OracleSource) -> Option<PriceData> {
        let feeds = self.feeds.read().await;
        
        if let Some(feed) = feeds.get(&pair) {
            feed.get_price(source).cloned()
        } else {
            None
        }
    }
    
    // ==================== HELPERS ====================
    
    fn calculate_max_deviation(&self, feed: &PriceFeed) -> Option<U256> {
        if feed.prices.is_empty() {
            return None;
        }
        
        let median = feed.calculate_median()?;
        let mut max_dev = U256::zero();
        
        for data in feed.prices.values() {
            let dev = calculate_deviation(&data.value, &median);
            if dev > max_dev {
                max_dev = dev;
            }
        }
        
        Some(max_dev)
    }
    
    // ==================== HEALTH CHECKS ====================
    
    pub async fn health_check(&self, pair: [u8; 20]) -> OracleHealth {
        let feeds = self.feeds.read().await;
        
        if let Some(feed) = feeds.get(&pair) {
            let price_count = feed.prices.len() as u8;
            let is_stale = feed.is_stale(self.config.max_staleness_seconds);
            
            OracleHealth {
                pair,
                is_stale,
                source_count: price_count,
                min_sources_met: price_count >= self.config.min_sources,
            }
        } else {
            OracleHealth {
                pair,
                is_stale: true,
                source_count: 0,
                min_sources_met: false,
            }
        }
    }
    
    pub async fn get_all_pairs(&self) -> Vec<[u8; 20]> {
        let feeds = self.feeds.read().await;
        feeds.keys().cloned().collect()
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct OracleHealth {
    pub pair: [u8; 20],
    pub is_stale: bool,
    pub source_count: u8,
    pub min_sources_met: bool,
}

// ==================== TWAP CALCULATOR ====================

pub struct TWAPCalculator {
    cumulative_prices: HashMap<[u8; 20], U256>,
    last_price: HashMap<[u8; 20], U256>,
    last_update: HashMap<[u8; 20], u64>,
}

impl TWAPCalculator {
    pub fn new() -> Self {
        Self {
            cumulative_prices: HashMap::new(),
            last_price: HashMap::new(),
            last_update: HashMap::new(),
        }
    }
    
    pub fn update(&mut self, pair: [u8; 20], price: U256) {
        let now = current_timestamp();
        
        if let Some(last_ts) = self.last_update.get(&pair) {
            let time_elapsed = now - last_ts;
            
            // Update cumulative price
            let cumulative = self.cumulative_prices.get(&pair).unwrap_or(&U256::zero());
            let lp = self.last_price.get(&pair).unwrap_or(&U256::zero());
            
            // cumulative += price * time_elapsed
            self.cumulative_prices.insert(pair, *cumulative + (price * U256::from(time_elapsed)));
        }
        
        self.last_price.insert(pair, price);
        self.last_update.insert(pair, now);
    }
    
    pub fn calculate_twap(&self, pair: [u8; 20], interval_seconds: u64) -> Option<U256> {
        let now = current_timestamp();
        
        if let Some(last_ts) = self.last_update.get(&pair) {
            if now - last_ts < interval_seconds {
                return None;
            }
            
            let cumulative = self.cumulative_prices.get(&pair).unwrap_or(&U256::zero());
            
            // TWAP = cumulative / interval
            Some(*cumulative / U256::from(interval_seconds))
        } else {
            None
        }
    }
}

// ==================== HELPER FUNCTIONS ====================

fn current_timestamp() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

fn calculate_deviation(a: &U256, b: &U256) -> U256 {
    if *b == U256::zero() {
        return U256::zero();
    }
    
    let diff = if *a > *b { *a - *b } else { *b - *a };
    (diff * U256::from(10000)) / *b
}

trait U256Ext {
    fn zero() -> Self;
    fn from(n: u64) -> Self;
}

impl U256Ext for U256 {
    fn zero() -> Self { U256::from(0) }
    fn from(n: u64) -> Self { U256::from(n) }
}

// ==================== PUBLIC API ====================

pub mod api {
    use super::*;
    
    pub type OracleHandle = Arc<OracleAggregator>;
    
    pub fn create_oracle(config: OracleConfig) -> OracleHandle {
        Arc::new(OracleAggregator::new(config))
    }
}

// ==================== TESTS ====================

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_oracle_creation() {
        let config = OracleConfig::default();
        let oracle = OracleAggregator::new(config);
        
        assert_eq!(oracle.config.max_staleness_seconds, 300);
    }
    
    #[test]
    fn test_price_feed() {
        let pair = [0u8; 20];
        let mut feed = PriceFeed::new(pair);
        
        feed.update_price(OracleSource::Chainlink, U256::from(1000), U256::from(10));
        feed.update_price(OracleSource::UniswapTWAP, U256::from(1005), U256::from(20));
        
        assert!(feed.prices.len() == 2);
    }
}