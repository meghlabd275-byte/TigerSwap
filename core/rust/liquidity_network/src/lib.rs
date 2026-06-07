//! Professional Liquidity Network
//! 
//! Provides connections to professional market makers, OTC desks, and institutional liquidity

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::Arc;
use parking_lot::RwLock;

/// Liquidity provider type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ProviderType {
    MarketMaker,
    OTCDeck,
    Institutional,
    HedgeFund,
    PrimeBroker,
}

/// Liquidity provider
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LiquidityProvider {
    pub id: String,
    pub name: String,
    pub provider_type: ProviderType,
    pub is_active: bool,
    pub supported_tokens: Vec<String>,
    pub min_order_size: u128,
    pub max_order_size: u128,
    pub fee_bps: i64,
    pub regions: Vec<String>,
}

impl LiquidityProvider {
    pub fn new(id: String, name: String, provider_type: ProviderType) -> Self {
        Self {
            id,
            name,
            provider_type,
            is_active: true,
            supported_tokens: vec![],
            min_order_size: 10_000_000_000, // $10k
            max_order_size: 10_000_000_000_000, // $10M
            fee_bps: 2, // 0.02%
            regions: vec!["global".to_string()],
        }
    }
}

/// Liquidity quote
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LiquidityQuote {
    pub quote_id: String,
    pub provider_id: String,
    pub token_in: String,
    pub token_out: String,
    pub amount_in: u128,
    pub amount_out: u128,
    pub fee: u128,
    pub expires_at: i64,
}

/// Liquidity Network
pub struct LiquidityNetwork {
    providers: Arc<RwLock<HashMap<String, LiquidityProvider>>>,
    quotes: Arc<RwLock<HashMap<String, Vec<LiquidityQuote>>>>,
}

impl LiquidityNetwork {
    pub fn new() -> Self {
        Self {
            providers: Arc::new(RwLock::new(HashMap::new())),
            quotes: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    pub fn register_provider(&self, provider: LiquidityProvider) {
        let mut providers = self.providers.write();
        providers.insert(provider.id.clone(), provider);
    }

    pub fn get_providers(&self) -> Vec<LiquidityProvider> {
        let providers = self.providers.read();
        providers.values()
            .filter(|p| p.is_active)
            .cloned()
            .collect()
    }

    pub fn get_provider(&self, id: &str) -> Option<LiquidityProvider> {
        let providers = self.providers.read();
        providers.get(id).cloned()
    }

    pub fn add_quote(&self, quote: LiquidityQuote) {
        let mut quotes = self.quotes.write();
        let key = format!("{}:{}", quote.token_in, quote.token_out);
        let provider_quotes = quotes.entry(key).or_insert_with(Vec::new);
        provider_quotes.push(quote);
    }

    pub fn get_best_quote(&self, token_in: &str, token_out: &str, amount: u128) -> Option<LiquidityQuote> {
        let quotes = self.quotes.read();
        let key = format!("{}:{}", token_in, token_out);
        let provider_quotes = quotes.get(&key)?;
        
        provider_quotes.iter()
            .filter(|q| q.amount_in >= amount)
            .max_by_key(|q| q.amount_out)
            .cloned()
    }

    pub fn provider_count(&self) -> usize {
        let providers = self.providers.read();
        providers.len()
    }
}

impl Default for LiquidityNetwork {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_liquidity_network() {
        let network = LiquidityNetwork::new();
        
        let provider = LiquidityProvider::new(
            "mm1".to_string(),
            "Jane Street".to_string(),
            ProviderType::MarketMaker,
        );
        
        network.register_provider(provider);
        assert_eq!(network.provider_count(), 1);
    }
}