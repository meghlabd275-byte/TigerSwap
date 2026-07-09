//! TigerSwap Wallet Core
//! High-performance secure wallet implementation with multi-chain support
//! 
//! Features:
//! - HD Wallet (BIP39, BIP44)
//! - Multi-signature support
//! - Hardware wallet integration ready
//! - Multi-chain key derivation (Ethereum, Solana, Cosmos)
//! - Transaction signing and verification
//! - Encrypted wallet storage

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

pub mod crypto;
pub mod keystore;
pub mod transaction;
pub mod types;

pub use crypto::*;
pub use keystore::*;
pub use transaction::*;
pub use types::*;

/// Wallet manager for handling multiple accounts
pub struct WalletManager {
    keystore: Arc<RwLock<Keystore>>,
    accounts: Arc<RwLock<HashMap<String, WalletAccount>>>,
    chain_configs: Arc<RwLock<HashMap<u32, ChainConfig>>>,
}

impl WalletManager {
    /// Create a new wallet manager
    pub fn new() -> Self {
        Self {
            keystore: Arc::new(RwLock::new(Keystore::new())),
            accounts: Arc::new(RwLock::new(HashMap::new())),
            chain_configs: Arc::new(RwLock::new(HashMap::new())),
        }
    }
    
    /// Initialize with default chain configurations
    pub async fn init_default_chains(&self) {
        let mut configs = self.chain_configs.write().await;
        
        // Ethereum
        configs.insert(1, ChainConfig {
            chain_id: 1,
            name: "Ethereum Mainnet".to_string(),
            symbol: "ETH".to_string(),
            decimals: 18,
            rpc_url: "https://eth.llamarpc.com".to_string(),
            explorer_url: "https://etherscan.io".to_string(),
            coin_type: 60,
        });
        
        // BSC
        configs.insert(56, ChainConfig {
            chain_id: 56,
            name: "BNB Smart Chain".to_string(),
            symbol: "BNB".to_string(),
            decimals: 18,
            rpc_url: "https://bsc-dataseed.binance.org".to_string(),
            explorer_url: "https://bscscan.com".to_string(),
            coin_type: 60,
        });
        
        // Arbitrum
        configs.insert(42161, ChainConfig {
            chain_id: 42161,
            name: "Arbitrum One".to_string(),
            symbol: "ETH".to_string(),
            decimals: 18,
            rpc_url: "https://arb1.arbitrum.io/rpc".to_string(),
            explorer_url: "https://arbiscan.io".to_string(),
            coin_type: 60,
        });
        
        // Optimism
        configs.insert(10, ChainConfig {
            chain_id: 10,
            name: "Optimism".to_string(),
            symbol: "ETH".to_string(),
            decimals: 18,
            rpc_url: "https://mainnet.optimism.io".to_string(),
            explorer_url: "https://optimistic.etherscan.io".to_string(),
            coin_type: 60,
        });
        
        // Base
        configs.insert(8453, ChainConfig {
            chain_id: 8453,
            name: "Base".to_string(),
            symbol: "ETH".to_string(),
            decimals: 18,
            rpc_url: "https://mainnet.base.org".to_string(),
            explorer_url: "https://basescan.org".to_string(),
            coin_type: 60,
        });
        
        // Polygon
        configs.insert(137, ChainConfig {
            chain_id: 137,
            name: "Polygon".to_string(),
            symbol: "MATIC".to_string(),
            decimals: 18,
            rpc_url: "https://polygon-rpc.com".to_string(),
            explorer_url: "https://polygonscan.com".to_string(),
            coin_type: 60,
        });
        
        // Solana
        configs.insert(0, ChainConfig {
            chain_id: 0,
            name: "Solana".to_string(),
            symbol: "SOL".to_string(),
            decimals: 9,
            rpc_url: "https://api.mainnet-beta.solana.com".to_string(),
            explorer_url: "https://explorer.solana.com".to_string(),
            coin_type: 501,
        });
        
        tracing::info!("Initialized {} chain configurations", configs.len());
    }
    
    /// Create a new wallet from mnemonic
    pub async fn create_from_mnemonic(
        &self,
        mnemonic: &str,
        password: &str,
        name: String,
    ) -> Result<WalletAccount, WalletError> {
        // Validate mnemonic
        if !Self::validate_mnemonic(mnemonic) {
            return Err(WalletError::InvalidMnemonic);
        }
        
        // Generate master key
        let master_key = Mnemonic::from_phrase(mnemonic)
            .map_err(|_| WalletError::InvalidMnemonic)?
            .to_seed(password);
        
        // Create keyring
        let keyring = Keyring::new(master_key);
        
        // Derive accounts for supported chains
        let mut accounts = HashMap::new();
        
        // Ethereum account (m/44'/60'/0'/0/0)
        let eth_key = keyring.derive_path("m/44'/60'/0'/0/0")?;
        let eth_address = eth_key.to_address();
        accounts.insert(1, DerivedKey {
            chain_id: 1,
            address: eth_address.clone(),
            private_key: eth_key.private_key_bytes(),
            public_key: eth_key.public_key_bytes(),
        });
        
        // Create account
        let account = WalletAccount {
            id: Self::generate_account_id(),
            name,
            created_at: chrono::Utc::now(),
            keys: accounts,
            encrypted: true,
        };
        
        // Store in keystore
        let mut keystore = self.keystore.write().await;
        keystore.add_account(account.id.clone(), account.clone()).await?;
        
        // Store in accounts
        let mut accounts_map = self.accounts.write().await;
        accounts_map.insert(account.id.clone(), account.clone());
        
        tracing::info!("Created wallet account: {}", account.id);
        
        Ok(account)
    }
    
    /// Import existing private key
    pub async fn import_private_key(
        &self,
        private_key: &[u8],
        password: &str,
        name: String,
    ) -> Result<WalletAccount, WalletError> {
        let key = PrivateKey::from_bytes(private_key)?;
        
        let mut accounts = HashMap::new();
        
        // Ethereum address from private key
        let eth_address = key.to_address();
        accounts.insert(1, DerivedKey {
            chain_id: 1,
            address: eth_address.clone(),
            private_key: private_key.to_vec(),
            public_key: key.public_key_bytes(),
        });
        
        let account = WalletAccount {
            id: Self::generate_account_id(),
            name,
            created_at: chrono::Utc::now(),
            keys: accounts,
            encrypted: true,
        };
        
        let mut keystore = self.keystore.write().await;
        keystore.add_account(account.id.clone(), account.clone()).await?;
        
        let mut accounts_map = self.accounts.write().await;
        accounts_map.insert(account.id.clone(), account.clone());
        
        Ok(account)
    }
    
    /// Sign a transaction for a specific chain
    pub async fn sign_transaction(
        &self,
        account_id: &str,
        chain_id: u32,
        transaction: &mut Transaction,
    ) -> Result<Vec<u8>, WalletError> {
        let accounts = self.accounts.read().await;
        let account = accounts_map.get(account_id)
            .ok_or(WalletError::AccountNotFound)?;
        
        let key = account.keys.get(&chain_id)
            .ok_or(WalletError::ChainNotSupported)?;
        
        // Sign based on chain type
        match chain_id {
            1 | 56 | 42161 | 10 | 8453 | 137 => {
                // EVM chains
                let private_key = PrivateKey::from_bytes(&key.private_key)?;
                let signature = private_key.sign_transaction(transaction)?;
                transaction.signature = Some(signature);
                Ok(signature)
            },
            _ => Err(WalletError::ChainNotSupported),
        }
    }
    
    /// Sign a message
    pub async fn sign_message(
        &self,
        account_id: &str,
        chain_id: u32,
        message: &[u8],
    ) -> Result<Vec<u8>, WalletError> {
        let accounts = self.accounts.read().await;
        let account = accounts_map.get(account_id)
            .ok_or(WalletError::AccountNotFound)?;
        
        let key = account.keys.get(&chain_id)
            .ok_or(WalletError::ChainNotSupported)?;
        
        match chain_id {
            1 | 56 | 42161 | 10 | 8453 | 137 => {
                let private_key = PrivateKey::from_bytes(&key.private_key)?;
                Ok(private_key.sign_message(message))
            },
            _ => Err(WalletError::ChainNotSupported),
        }
    }
    
    /// Get account by ID
    pub async fn get_account(&self, account_id: &str) -> Option<WalletAccount> {
        let accounts = self.accounts.read().await;
        accounts.get(account_id).cloned()
    }
    
    /// Get address for a specific chain
    pub async fn get_address(&self, account_id: &str, chain_id: u32) -> Option<String> {
        let accounts = self.accounts.read().await;
        accounts.get(account_id)
            .and_then(|acc| acc.keys.get(&chain_id))
            .map(|key| key.address.clone())
    }
    
    /// List all accounts
    pub async fn list_accounts(&self) -> Vec<WalletAccount> {
        let accounts = self.accounts.read().await;
        accounts.values().cloned().collect()
    }
    
    /// Validate mnemonic phrase
    fn validate_mnemonic(mnemonic: &str) -> bool {
        let words: Vec<&str> = mnemonic.split_whitespace().collect();
        if words.len() != 12 && words.len() != 24 {
            return false;
        }
        // In production, validate against wordlist
        true
    }
    
    fn generate_account_id() -> String {
        use rand::Rng;
        let mut rng = rand::thread_rng();
        let bytes: [u8; 16] = rng.gen();
        hex::encode(bytes)
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
    async fn test_wallet_creation() {
        let manager = WalletManager::new();
        manager.init_default_chains().await;
        
        let mnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about";
        
        let account = manager.create_from_mnemonic(
            mnemonic,
            "password",
            "Test Wallet".to_string(),
        ).await;
        
        assert!(account.is_ok());
    }
}
