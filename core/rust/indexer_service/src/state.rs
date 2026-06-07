//! State Module

use serde::{Deserialize, Serialize};

/// Account state
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct AccountState {
    pub address: String,
    pub balance: String,
    pub nonce: u64,
    pub code_hash: String,
    pub storage_root: String,
}

impl AccountState {
    pub fn new(address: &str) -> Self {
        Self {
            address: address.to_string(),
            balance: "0".to_string(),
            nonce: 0,
            code_hash: "0x".to_string(),
            storage_root: "0x".to_string(),
        }
    }
}

/// Token balance
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TokenBalance {
    pub address: String,
    pub token: String,
    pub balance: String,
    pub block_number: u64,
}

impl TokenBalance {
    pub fn new(address: &str, token: &str) -> Self {
        Self {
            address: address.to_string(),
            token: token.to_string(),
            balance: "0".to_string(),
            block_number: 0,
        }
    }
}

/// State change
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StateChange {
    pub field: String,
    pub old_value: String,
    pub new_value: String,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_account_state() {
        let state = AccountState::new("0x123");
        assert_eq!(state.balance, "0");
    }
}