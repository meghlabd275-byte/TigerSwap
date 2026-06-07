//! Reconciliation Module - State reconciliation

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

use crate::{AccountState, TokenBalance, StateChange};

/// State reconciliation
pub struct StateReconciliation {
    states: RwLock<HashMap<String, AccountState>>,
    token_balances: RwLock<HashMap<String, TokenBalance>>,
}

impl StateReconciliation {
    pub fn new() -> Self {
        Self {
            states: RwLock::new(HashMap::new()),
            token_balances: RwLock::new(HashMap::new()),
        }
    }
    
    /// Update account state
    pub async fn update_state(&self, address: &str, state: AccountState) {
        let mut states = self.states.write().await;
        states.insert(address.to_string(), state);
    }
    
    /// Get account state
    pub async fn get_state(&self, address: &str) -> Option<AccountState> {
        let states = self.states.read().await;
        states.get(address).cloned()
    }
    
    /// Update token balance
    pub async fn update_token_balance(&self, address: &str, token: &str, balance: TokenBalance) {
        let key = format!("{}:{}", address, token);
        let mut balances = self.token_balances.write().await;
        balances.insert(key, balance);
    }
    
    /// Get token balance
    pub async fn get_token_balance(&self, address: &str, token: &str) -> Option<TokenBalance> {
        let key = format!("{}:{}", address, token);
        let balances = self.token_balances.read().await;
        balances.get(&key).cloned()
    }
    
    /// Reconcile state changes
    pub async fn reconcile(&self, old_state: &AccountState, new_state: &AccountState) -> Vec<StateChange> {
        let mut changes = Vec::new();
        
        if old_state.balance != new_state.balance {
            changes.push(StateChange {
                field: "balance".to_string(),
                old_value: old_state.balance.clone(),
                new_value: new_state.balance.clone(),
            });
        }
        
        if old_state.nonce != new_state.nonce {
            changes.push(StateChange {
                field: "nonce".to_string(),
                old_value: old_state.nonce.to_string(),
                new_value: new_state.nonce.to_string(),
            });
        }
        
        if old_state.code_hash != new_state.code_hash {
            changes.push(StateChange {
                field: "code_hash".to_string(),
                old_value: old_state.code_hash.clone(),
                new_value: new_state.code_hash.clone(),
            });
        }
        
        changes
    }
}

impl Default for StateReconciliation {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[tokio::test]
    async fn test_reconciliation() {
        let reconciliation = StateReconciliation::new();
        
        let old_state = AccountState::new("0x123");
        let mut new_state = AccountState::new("0x123");
        new_state.balance = "100".to_string();
        
        let changes = reconciliation.reconcile(&old_state, &new_state).await;
        assert!(!changes.is_empty());
    }
}