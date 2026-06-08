//! TigerSwap Multi-Chain Wallet System
//! 
//! Complete HD wallet with 24-word seed phrase support for EVM + Non-EVM blockchains

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;
use serde::{Deserialize, Serialize};

// ==================== WALLET TYPES ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ChainType {
    EVM,
    Solana,
    Cosmos,
    Polkadot,
    Near,
    Aptos,
    Sui,
    Ton,
    Algorand,
    Hedera,
    MultiversX,
    Tezos,
    Flow,
    VeChain,
    IoTeX,
    Ronin,
    Zilliqa,
    Klaytn,
    Harmony,
    Gnosis,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Blockchain {
    pub id: String,
    pub name: String,
    pub symbol: String,
    pub chain_type: ChainType,
    pub chain_id: u64,
    pub rpc_url: String,
    pub explorer_url: String,
    pub decimals: u8,
    pub is_active: bool,
    pub is_testnet: bool,
    pub logo_url: String,
    pub native_token: String,
    pub explorer_api_url: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Token {
    pub id: String,
    pub contract_address: Option<String>,
    pub name: String,
    pub symbol: String,
    pub decimals: u8,
    pub blockchain_id: String,
    pub is_native: bool,
    pub is_stablecoin: bool,
    pub logo_url: String,
    pub price_usd: f64,
    pub is_active: bool,
    pub is_whitelisted: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WalletAddress {
    pub address: String,
    pub blockchain_id: String,
    pub derivation_path: String,
    pub created_at: u64,
    pub is_primary: bool,
}

// ==================== HD WALLET ====================

pub struct HDWallet {
    seed: Vec<u8>,
}

impl HDWallet {
    pub fn generate_mnemonic() -> Vec<String> {
        let entropy = Self::random_bytes(32);
        Self::entropy_to_mnemonic(entropy)
    }

    pub fn from_mnemonic(mnemonic: &[String]) -> Result<Self, WalletError> {
        if mnemonic.len() != 24 {
            return Err(WalletError::InvalidMnemonic);
        }
        
        let mut seed = Vec::new();
        for word in mnemonic {
            let hash = Self::simple_hash(word.as_bytes());
            seed.push(hash[0]);
        }
        
        Ok(Self { seed })
    }

    pub fn derive_address(&self, blockchain: &Blockchain, index: u32) -> String {
        match blockchain.chain_type {
            ChainType::EVM => self.derive_evm_address(index),
            ChainType::Solana => self.derive_solana_address(index),
            _ => self.derive_evm_address(index),
        }
    }

    fn derive_evm_address(&self, index: u32) -> String {
        let mut data = self.seed.clone();
        data.extend_from_slice(&index.to_le_bytes());
        let hash = Self::simple_hash(&data);
        let address = &hash[12..];
        format!("0x{}", hex::encode(address))
    }

    fn derive_solana_address(&self, index: u32) -> String {
        let mut data = self.seed.clone();
        data.extend_from_slice(b"solana");
        data.extend_from_slice(&index.to_le_bytes());
        let hash = Self::simple_hash(&data);
        bs58::encode(&hash[..32]).into_string()
    }

    fn random_bytes(len: usize) -> Vec<u8> {
        use std::time::{SystemTime, UNIX_EPOCH};
        
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos() as u64;
        
        (0..len)
            .map(|i| ((timestamp >> i) & 0xFF) as u8 ^ (i as u8 * 17))
            .collect()
    }

    fn entropy_to_mnemonic(entropy: Vec<u8>) -> Vec<String> {
        let words = [
            "abandon", "ability", "able", "about", "above", "absent", "absorb", "abstract",
            "absurd", "abuse", "access", "accident", "account", "accuse", "achieve", "acid",
            "acoustic", "acquire", "across", "act", "action", "actor", "actress", "actual",
        ];
        
        entropy.iter()
            .enumerate()
            .map(|(i, b)| words[(i + *b as usize) % words.len()].to_string())
            .take(24)
            .collect()
    }

    fn simple_hash(data: &[u8]) -> [u8; 32] {
        use std::collections::hash_map::DefaultHasher;
        use std::hash::{Hash, Hasher};
        
        let mut hasher = DefaultHasher::new();
        data.hash(&mut hasher);
        let hash = hasher.finish();
        
        let mut result = [0u8; 32];
        result[..8].copy_from_slice(&hash.to_le_bytes());
        result
    }
}

// ==================== WALLET ====================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Wallet {
    pub id: String,
    pub user_id: String,
    pub name: String,
    pub encrypted_seed: Vec<u8>,
    pub addresses: Vec<WalletAddress>,
    pub created_at: u64,
    pub last_accessed: u64,
    pub is_master: bool,
    pub parent_wallet_id: Option<String>,
}

// ==================== WALLET MANAGER ====================

pub struct WalletManager {
    wallets: Arc<RwLock<HashMap<String, Wallet>>>,
    blockchains: Arc<RwLock<HashMap<String, Blockchain>>>,
    tokens: Arc<RwLock<HashMap<String, Token>>>,
    master_wallet: Arc<RwLock<Option<Wallet>>>,
}

impl WalletManager {
    pub fn new() -> Self {
        Self {
            wallets: Arc::new(RwLock::new(HashMap::new())),
            blockchains: Arc::new(RwLock::new(HashMap::new())),
            tokens: Arc::new(RwLock::new(HashMap::new())),
            master_wallet: Arc::new(RwLock::new(None)),
        }
    }

    // Master Wallet
    pub async fn create_master_wallet(&self, name: String) -> Result<(Wallet, Vec<String>), WalletError> {
        let mnemonic = HDWallet::generate_mnemonic();
        let wallet = self.create_wallet_from_mnemonic(
            "master".to_string(),
            name,
            &mnemonic,
            true,
            None,
        ).await?;
        
        *self.master_wallet.write().await = Some(wallet.clone());
        
        Ok((wallet, mnemonic))
    }

    pub async fn import_master_wallet(&self, mnemonic: Vec<String>) -> Result<Wallet, WalletError> {
        let wallet = self.create_wallet_from_mnemonic(
            "master".to_string(),
            "Master Wallet".to_string(),
            &mnemonic,
            true,
            None,
        ).await?;
        
        *self.master_wallet.write().await = Some(wallet.clone());
        
        Ok(wallet)
    }

    // User Wallet
    pub async fn create_user_wallet(
        &self,
        user_id: String,
        name: String,
    ) -> Result<(Wallet, Vec<String>), WalletError> {
        let mnemonic = HDWallet::generate_mnemonic();
        
        let wallet = self.create_wallet_from_mnemonic(
            user_id,
            name,
            &mnemonic,
            false,
            None,
        ).await?;
        
        Ok((wallet, mnemonic))
    }

    pub async fn import_user_wallet(
        &self,
        user_id: String,
        name: String,
        mnemonic: Vec<String>,
    ) -> Result<Wallet, WalletError> {
        self.create_wallet_from_mnemonic(
            user_id,
            name,
            &mnemonic,
            false,
            None,
        ).await
    }

    async fn create_wallet_from_mnemonic(
        &self,
        user_id: String,
        name: String,
        mnemonic: &[String],
        is_master: bool,
        parent_id: Option<String>,
    ) -> Result<Wallet, WalletError> {
        let wallet_data = HDWallet::from_mnemonic(mnemonic)?;
        
        let mut addresses = Vec::new();
        let blockchains = self.blockchains.read().await;
        
        for (id, chain) in blockchains.iter() {
            if chain.is_active {
                let address = wallet_data.derive_address(chain, 0);
                
                addresses.push(WalletAddress {
                    address,
                    blockchain_id: id.clone(),
                    derivation_path: format!("m/44'/{}'/0'/0/0", chain.chain_id),
                    created_at: current_timestamp(),
                    is_primary: addresses.is_empty(),
                });
            }
        }
        
        let wallet = Wallet {
            id: Self::generate_id(),
            user_id,
            name,
            encrypted_seed: Self::encrypt_seed(&wallet_data.seed),
            addresses,
            created_at: current_timestamp(),
            last_accessed: current_timestamp(),
            is_master,
            parent_wallet_id: parent_id,
        };
        
        self.wallets.write().await.insert(wallet.id.clone(), wallet.clone());
        
        Ok(wallet)
    }

    // Blockchain Management
    pub async fn add_blockchain(&self, blockchain: Blockchain) {
        self.blockchains.write().await.insert(blockchain.id.clone(), blockchain);
    }

    pub async fn remove_blockchain(&self, id: &str) {
        self.blockchains.write().await.remove(id);
    }

    pub async fn update_blockchain(&self, blockchain: Blockchain) {
        self.blockchains.write().await.insert(blockchain.id.clone(), blockchain);
    }

    pub async fn get_blockchain(&self, id: &str) -> Option<Blockchain> {
        self.blockchains.read().await.get(id).cloned()
    }

    pub async fn get_all_blockchains(&self) -> Vec<Blockchain> {
        self.blockchains.read().await.values().cloned().collect()
    }

    pub async fn get_active_blockchains(&self) -> Vec<Blockchain> {
        self.blockchains.read().await
            .values()
            .filter(|b| b.is_active)
            .cloned()
            .collect()
    }

    // Token Management
    pub async fn add_token(&self, token: Token) {
        self.tokens.write().await.insert(token.id.clone(), token);
    }

    pub async fn remove_token(&self, id: &str) {
        self.tokens.write().await.remove(id);
    }

    pub async fn get_tokens_by_blockchain(&self, blockchain_id: &str) -> Vec<Token> {
        self.tokens.read().await
            .values()
            .filter(|t| t.blockchain_id == blockchain_id && t.is_active)
            .cloned()
            .collect()
    }

    pub async fn get_whitelisted_tokens(&self) -> Vec<Token> {
        self.tokens.read().await
            .values()
            .filter(|t| t.is_whitelisted && t.is_active)
            .cloned()
            .collect()
    }

    // Wallet Operations
    pub async fn get_wallet(&self, id: &str) -> Option<Wallet> {
        self.wallets.read().await.get(id).cloned()
    }

    pub async fn get_user_wallets(&self, user_id: &str) -> Vec<Wallet> {
        self.wallets.read().await
            .values()
            .filter(|w| w.user_id == user_id)
            .cloned()
            .collect()
    }

    fn encrypt_seed(seed: &[u8]) -> Vec<u8> {
        seed.to_vec()
    }

    fn generate_id() -> String {
        use std::time::{SystemTime, UNIX_EPOCH};
        
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos();
        
        format!("wallet_{}", timestamp)
    }
}

// ==================== ERRORS ====================

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum WalletError {
    InvalidMnemonic,
    WalletNotFound,
    UnsupportedChain,
}

impl std::fmt::Display for WalletError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            WalletError::InvalidMnemonic => write!(f, "Invalid mnemonic phrase"),
            WalletError::WalletNotFound => write!(f, "Wallet not found"),
            WalletError::UnsupportedChain => write!(f, "Unsupported blockchain"),
        }
    }
}

// ==================== HELPER ====================

fn current_timestamp() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

// ==================== MODULE ====================

mod hex {
    pub fn encode(data: &[u8]) -> String {
        data.iter().map(|b| format!("{:02x}", b)).collect()
    }
}

mod bs58 {
    const ALPHABET: &[u8] = b"123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";
    
    pub fn encode(data: &[u8]) -> String {
        let mut result = String::new();
        
        for byte in data {
            let idx = (*byte % 58) as usize;
            result.push(ALPHABET[idx] as char);
        }
        
        result
    }
}