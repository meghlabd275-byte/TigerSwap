//! TigerSwap Price Oracle - Rust Implementation
//! 
//! Multi-source price feeds: Chainlink, DEX pools, TWAP calculations

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use parking_lot::RwLock;

/// Price data from a source
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PriceData {
    pub price: f64,
    pub timestamp: u64,
    pub source: PriceSource,
    pub confidence: f64,
    pub previous_price: Option<f64>,
}

/// Price source
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum PriceSource {
    Chainlink,
    DEX,
    TWAP,
    Coinbase,
}

/// Price result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PriceResult {
    pub base_token: String,
    pub quote_token: String,
    pub price: f64,
    pub previous_price: Option<f64>,
    pub change_24h: Option<f64>,
    pub change_1h: Option<f64>,
    pub high_24h: Option<f64>,
    pub low_24h: Option<f64>,
    pub volume_24h: Option<f64>,
    pub sources: Vec<PriceData>,
    pub timestamp: u64,
    pub provider: String,
}

/// Historical price data
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct HistoricalPrice {
    pub timestamp: u64,
    pub open: f64,
    pub high: f64,
    pub low: f64,
    pub close: f64,
    pub volume: f64,
}

/// Chainlink price feed addresses by chain
const CHAINLINK_FEEDS: &[(&str, &str, &str)] = &[
    ("ETH/USD", "1", "0x5f4eC3Df9cbd43714FE2740f5E3617235d868879"),
    ("BTC/USD", "1", "0xF4030086522a5bEEa4988F8cA5B36dbC97BeE88c"),
    ("LINK/USD", "1", "0x2c1d072e956affc02f810a2d70d6f371ea4b1d8c"),
    ("USDC/USD", "1", "0x8fffffd085591743496e568d2398187d1ba14bac"),
    ("ETH/USD", "56", "0x9ef1B8cE5E96FeD8b63Cb2EaADc66e4b4236cB85"),
    ("BTC/USD", "56", "0x264990fbd0A4796A3E8d8BbC90280fF41eB0C1C2"),
    ("ETH/USD", "137", "0xF9680D99D6C9589e2a93a78A04A279e509205225"),
    ("MATIC/USD", "137", "0xAB594600376Ec9fD91F8e885dADF0CE036862dE0"),
];

/// Price Oracle Service
pub struct PriceOracle {
    cache: RwLock<HashMap<String, CachedPrice>>,
    cache_expiry_ms: u64,
    historical_prices: RwLock<HashMap<String, Vec<HistoricalPrice>>>,
}

#[derive(Debug, Clone)]
struct CachedPrice {
    data: PriceData,
    expiry: u64,
}

impl PriceOracle {
    pub fn new() -> Self {
        Self {
            cache: RwLock::new(HashMap::new()),
            cache_expiry_ms: 60_000, // 1 minute default
            historical_prices: RwLock::new(HashMap::new()),
        }
    }

    /// Get price from multiple sources
    pub async fn get_price(&self, base_token: &str, quote_token: &str, chain_id: u64) -> Result<PriceResult, String> {
        let sources = vec![PriceSource::Chainlink, PriceSource::DEX, PriceSource::Coinbase];
        let mut prices: Vec<PriceData> = Vec::new();
        let mut best_price: Option<f64> = None;
        let mut best_source = "chainlink";

        for source in sources {
            if let Some(price_data) = self.get_price_from_source(source, base_token, quote_token, chain_id).await {
                prices.push(price_data.clone());
                
                if best_price.is_none() || price_data.confidence > prices.iter().find(|p| p.source == source).unwrap().confidence {
                    best_price = Some(price_data.price);
                    best_source = match source {
                        PriceSource::Chainlink => "chainlink",
                        PriceSource::DEX => "dex",
                        PriceSource::TWAP => "twap",
                        PriceSource::Coinbase => "coinbase",
                    };
                }
            }
        }

        let price = best_price.ok_or_else(|| format!("No price available for {}/{}", base_token, quote_token))?;

        Ok(PriceResult {
            base_token: base_token.to_string(),
            quote_token: quote_token.to_string(),
            price,
            previous_price: prices.first().and_then(|p| p.previous_price),
            change_24h: None,
            change_1h: None,
            high_24h: None,
            low_24h: None,
            volume_24h: None,
            sources: prices,
            timestamp: current_timestamp(),
            provider: best_source.to_string(),
        })
    }

    /// Get price from a specific source
    async fn get_price_from_source(&self, source: PriceSource, base_token: &str, quote_token: &str, chain_id: u64) -> Option<PriceData> {
        let cache_key = format!("{:?}:{}:{}:{}", source, base_token, quote_token, chain_id);
        
        // Check cache first
        {
            let cache = self.cache.read();
            if let Some(cached) = cache.get(&cache_key) {
                if cached.expiry > current_timestamp() {
                    return Some(cached.data.clone());
                }
            }
        }

        // Get price based on source
        let price = match source {
            PriceSource::Chainlink => self.get_chainlink_price(base_token, quote_token, chain_id).await,
            PriceSource::DEX => self.get_dex_price(base_token, quote_token, chain_id).await,
            PriceSource::TWAP => self.get_twap_price(base_token, quote_token, chain_id, 30).await,
            PriceSource::Coinbase => self.get_coinbase_price(base_token, quote_token).await,
        };

        if let Some(price) = price {
            let price_data = PriceData {
                price,
                timestamp: current_timestamp(),
                source,
                confidence: self.get_confidence_for_source(source),
                previous_price: None,
            };

            // Update cache
            {
                let mut cache = self.cache.write();
                cache.insert(cache_key, CachedPrice {
                    data: price_data.clone(),
                    expiry: current_timestamp() + self.cache_expiry_ms,
                });
            }

            Some(price_data)
        } else {
            None
        }
    }

    /// Get Chainlink price
    async fn get_chainlink_price(&self, base_token: &str, quote_token: &str, chain_id: u64) -> Option<f64> {
        // Find the feed address
        let pair = format!("{}/{}", base_token, quote_token);
        let chain_str = chain_id.to_string();
        
        for (feed_pair, feed_chain, _feed_addr) in CHAINLINK_FEEDS {
            if feed_pair == pair && feed_chain == chain_str {
                // In production, would call Chainlink feed
                // For now, return mock price
                return Some(self.get_mock_price(base_token));
            }
        }
        
        None
    }

    /// Get DEX price
    async fn get_dex_price(&self, base_token: &str, _quote_token: &str, _chain_id: u64) -> Option<f64> {
        // In production, would query DEX pools
        Some(self.get_mock_price(base_token) * 1.001) // Slight premium
    }

    /// Get TWAP price
    async fn get_twap_price(&self, base_token: &str, _quote_token: &str, _chain_id: u64, window_minutes: u32) -> Option<f64> {
        let key = format!("{}:{}", base_token, window_minutes);
        let hist = self.historical_prices.read();
        
        if let Some(prices) = hist.get(&key) {
            if !prices.is_empty() {
                let sum: f64 = prices.iter().map(|p| p.close).sum();
                return Some(sum / prices.len() as f64);
            }
        }
        
        Some(self.get_mock_price(base_token))
    }

    /// Get Coinbase price
    async fn get_coinbase_price(&self, base_token: &str, quote_token: &str) -> Option<f64> {
        // In production, would call Coinbase API
        // For now, return mock price with slight variation
        let base = self.get_mock_price(base_token);
        Some(base * (0.9995 + (current_timestamp() % 1000) as f64 / 100000.0))
    }

    /// Get mock price for testing
    fn get_mock_price(&self, token: &str) -> f64 {
        match token.to_uppercase().as_str() {
            "ETH" | "WETH" => 2450.0,
            "BTC" | "WBTC" => 62500.0,
            "LINK" => 18.5,
            "UNI" => 12.5,
            "AAVE" => 285.0,
            "USDC" | "USDT" | "DAI" => 1.0,
            "BNB" => 310.0,
            "MATIC" | "WMATIC" => 0.85,
            _ => 1.0,
        }
    }

    /// Get confidence score for a source
    fn get_confidence_for_source(&self, source: PriceSource) -> f64 {
        match source {
            PriceSource::Chainlink => 0.95,
            PriceSource::TWAP => 0.90,
            PriceSource::Coinbase => 0.88,
            PriceSource::DEX => 0.85,
        }
    }

    /// Get historical prices
    pub fn get_historical_prices(&self, token: &str, days: u32) -> Vec<HistoricalPrice> {
        let key = format!("{}:{}", token, days);
        self.historical_prices.read()
            .get(&key)
            .cloned()
            .unwrap_or_else(|| self.generate_mock_historical_prices(days))
    }

    /// Generate mock historical prices
    fn generate_mock_historical_prices(&self, days: u32) -> Vec<HistoricalPrice> {
        let mut prices = Vec::new();
        let now = current_timestamp();
        let interval_ms = 5 * 60 * 1000; // 5 minutes
        let points = (days as u64 * 24 * 60) / 5;
        
        let mut price = 2000.0 + (now % 1000) as f64;
        
        for i in (0..points).rev() {
            let timestamp = now - (i * interval_ms);
            let change = (i % 10) as f64 / 100.0 - 0.05;
            price = price * (1.0 + change);
            
            prices.push(HistoricalPrice {
                timestamp,
                open: price * 0.999,
                high: price * 1.001,
                low: price * 0.998,
                close: price,
                volume: (i % 100) as f64 * 10000.0,
            });
        }
        
        prices
    }

    /// Clear cache
    pub fn clear_cache(&self) {
        self.cache.write().clear();
    }

    /// Set cache expiry
    pub fn set_cache_expiry(&self, ms: u64) {
        self.cache_expiry_ms = ms;
    }
}

impl Default for PriceOracle {
    fn default() -> Self {
        Self::new()
    }
}

fn current_timestamp() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_millis() as u64
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_get_price() {
        let oracle = PriceOracle::new();
        
        let result = oracle.get_price("ETH", "USD", 1).await;
        assert!(result.is_ok());
        
        let price = result.unwrap();
        assert!(price.price > 0.0);
        assert_eq!(price.base_token, "ETH");
    }

    #[test]
    fn test_mock_prices() {
        let oracle = PriceOracle::new();
        
        assert_eq!(oracle.get_mock_price("ETH"), 2450.0);
        assert_eq!(oracle.get_mock_price("BTC"), 62500.0);
        assert_eq!(oracle.get_mock_price("USDC"), 1.0);
    }
}