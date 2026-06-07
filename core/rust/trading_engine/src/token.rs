use serde::{Deserialize, Serialize};
use rust_decimal::Decimal;
use std::collections::HashMap;
use std::sync::Arc;
use parking_lot::RwLock;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
pub enum TokenType {
    #[default] Unknown,
    Native,
    Erc20,
    Stable,
    Governance,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Token {
    pub address: String,
    pub symbol: String,
    pub name: String,
    pub decimals: u8,
    pub chain_id: u64,
    pub token_type: TokenType,
    pub price_usd: Option<Decimal>,
    pub is_verified: bool,
}

impl Token {
    pub fn new(address: impl Into<String>, symbol: impl Into<String>, name: impl Into<String>, decimals: u8, chain_id: u64) -> Self {
        Self {
            address: address.into(),
            symbol: symbol.into().to_uppercase(),
            name: name.into(),
            decimals,
            chain_id,
            token_type: TokenType::Unknown,
            price_usd: None,
            is_verified: false,
        }
    }

    pub fn stable(address: impl Into<String>, symbol: impl Into<String>, name: impl Into<String>, decimals: u8, chain_id: u64) -> Self {
        let mut token = Self::new(address, symbol, name, decimals, chain_id);
        token.token_type = TokenType::Stable;
        token.is_verified = true;
        token
    }

    pub fn native(chain_id: u64, symbol: &str, name: &str) -> Self {
        Self::new("0x0", symbol, name, 18, chain_id)
    }

    pub fn is_stable(&self) -> bool {
        self.token_type == TokenType::Stable
    }
}

#[derive(Debug, Clone, Default)]
pub struct TokenRegistry {
    tokens: Arc<RwLock<HashMap<String, Token>>>,
}

impl TokenRegistry {
    pub fn new() -> Self {
        Self { tokens: Arc::new(RwLock::new(HashMap::new())) }
    }

    pub fn register(&self, token: Token) {
        let key = format!("{}:{}", token.chain_id, token.address.to_lowercase());
        self.tokens.write().insert(key, token);
    }

    pub fn get(&self, chain_id: u64, address: &str) -> Option<Token> {
        let key = format!("{}:{}", chain_id, address.to_lowercase());
        self.tokens.read().get(&key).cloned()
    }

    pub fn len(&self) -> usize {
        self.tokens.read().len()
    }
}

impl TokenRegistry {
    pub fn with_defaults() -> Self {
        let registry = Self::new();
        registry.register(Token::native(1, "ETH", "Ethereum"));
        registry.register(Token::stable("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", "USDC", "USD Coin", 6, 1));
        registry.register(Token::stable("0xdAC17F958D2ee523a2206206994597C13D831ec7", "USDT", "Tether USD", 6, 1));
        registry.register(Token::native(56, "BNB", "BNB Chain"));
        registry.register(Token::native(137, "MATIC", "Polygon"));
        registry
    }
}

pub fn pool_key(token_a: &str, token_b: &str) -> String {
    let mut a = token_a.to_lowercase();
    let mut b = token_b.to_lowercase();
    if a > b { std::mem::swap(&mut a, &mut b); }
    format!("{}:{}", a, b)
}