//! Manager Module - Wallet management

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

use crate::{WalletError, Chain, HDWallet};

/// Wallet record stored in database
#[derive(Debug, Clone)]
pub struct WalletRecord {
    pub id: String,
    pub name: String,
    pub address: String,
    pub chain: Chain,
    pub created_at: i64,
    pub is_primary: bool,
}

/// Wallet info for API responses
#[derive(Debug, Clone)]
pub struct WalletInfo {
    pub wallet_id: String,
    pub name: String,
    pub address: String,
    pub chain: Chain,
    pub mnemonic: String,
}

/// Balance information
#[derive(Debug, Clone)]
pub struct Balance {
    pub address: String,
    pub chain: Chain,
    pub balances: Vec<TokenBalance>,
}

/// Token balance
#[derive(Debug, Clone)]
pub struct TokenBalance {
    pub symbol: String,
    pub balance: String,
    pub balance_raw: String,
}

/// Wallet Manager
pub struct WalletManager {
    wallets: RwLock<HashMap<String, WalletRecord>>,
}

impl WalletManager {
    pub fn new() -> Self {
        Self {
            wallets: RwLock::new(HashMap::new()),
        }
    }
    
    /// Create new wallet
    pub async fn create_wallet(&self, name: String, chain: Chain) -> Result<WalletInfo, WalletError> {
        let wallet = HDWallet::generate(chain);
        let address = wallet.default_address()?;
        let mnemonic = wallet.mnemonic_phrase();
        
        let record = WalletRecord {
            id: uuid::Uuid::new_v4().to_string(),
            name: name.clone(),
            address: address.clone(),
            chain,
            created_at: chrono::Utc::now().timestamp(),
            is_primary: false,
        };
        
        let wallet_id = record.id.clone();
        let mut wallets = self.wallets.write().await;
        wallets.insert(wallet_id.clone(), record);
        
        Ok(WalletInfo {
            wallet_id,
            name,
            address,
            chain,
            mnemonic,
        })
    }
    
    /// Import wallet from mnemonic
    pub async fn import_wallet(&self, name: String, mnemonic: &str, chain: Chain) -> Result<WalletInfo, WalletError> {
        let mnemonic = crate::Mnemonic::from_phrase(mnemonic)?;
        let wallet = HDWallet::from_mnemonic(mnemonic, chain);
        let address = wallet.default_address()?;
        
        let record = WalletRecord {
            id: uuid::Uuid::new_v4().to_string(),
            name: name.clone(),
            address: address.clone(),
            chain,
            created_at: chrono::Utc::now().timestamp(),
            is_primary: false,
        };
        
        let wallet_id = record.id.clone();
        let mut wallets = self.wallets.write().await;
        wallets.insert(wallet_id.clone(), record);
        
        Ok(WalletInfo {
            wallet_id,
            name,
            address,
            chain,
            mnemonic: mnemonic.phrase(),
        })
    }
    
    /// Get wallet
    pub async fn get_wallet(&self, wallet_id: &str) -> Option<WalletInfo> {
        let wallets = self.wallets.read().await;
        wallets.get(wallet_id).map(|w| WalletInfo {
            wallet_id: w.id.clone(),
            name: w.name.clone(),
            address: w.address.clone(),
            chain: w.chain,
            mnemonic: String::new(),
        })
    }
    
    /// List wallets
    pub async fn list_wallets(&self) -> Vec<WalletInfo> {
        let wallets = self.wallets.read().await;
        wallets.values().map(|w| WalletInfo {
            wallet_id: w.id.clone(),
            name: w.name.clone(),
            address: w.address.clone(),
            chain: w.chain,
            mnemonic: String::new(),
        }).collect()
    }
    
    /// Sign transaction
    pub async fn sign_transaction(&self, wallet_id: &str, message: &[u8]) -> Result<String, WalletError> {
        let wallets = self.wallets.read().await;
        let wallet_record = wallets.get(wallet_id).ok_or(WalletError::WalletNotFound)?;
        
        let wallet = HDWallet::generate(wallet_record.chain);
        let signature = wallet.sign(message)?;
        
        Ok(hex::encode(signature))
    }
    
    /// Get balance (simulated)
    pub async fn get_balance(&self, wallet_id: &str) -> Result<Balance, WalletError> {
        let wallets = self.wallets.read().await;
        let wallet_record = wallets.get(wallet_id).ok_or(WalletError::WalletNotFound)?;
        
        Ok(Balance {
            address: wallet_record.address.clone(),
            chain: wallet_record.chain,
            balances: vec![TokenBalance {
                symbol: "ETH".to_string(),
                balance: "0.0".to_string(),
                balance_raw: "0",
            }],
        })
    }
    
    /// Delete wallet
    pub async fn delete_wallet(&self, wallet_id: &str) -> Result<(), WalletError> {
        let mut wallets = self.wallets.write().await;
        wallets.remove(wallet_id).ok_or(WalletError::WalletNotFound)?;
        Ok(())
    }
}

impl Default for WalletManager {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[tokio::test]
    async fn test_create_wallet() {
        let manager = WalletManager::new();
        
        let wallet = manager.create_wallet("Test".to_string(), Chain::Ethereum).await.unwrap();
        
        assert!(!wallet.wallet_id.is_empty());
        assert!(!wallet.address.is_empty());
    }
    
    #[tokio::test]
    async fn test_import_wallet() {
        let manager = WalletManager::new();
        
        let mnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about";
        let wallet = manager.import_wallet("Imported".to_string(), mnemonic, Chain::Ethereum).await.unwrap();
        
        assert!(!wallet.wallet_id.is_empty());
    }
    
    #[tokio::test]
    async fn test_sign_transaction() {
        let manager = WalletManager::new();
        
        let wallet = manager.create_wallet("Test".to_string(), Chain::Ethereum).await.unwrap();
        
        let signature = manager.sign_transaction(&wallet.wallet_id, b"test message").await.unwrap();
        
        assert!(!signature.is_empty());
    }
}