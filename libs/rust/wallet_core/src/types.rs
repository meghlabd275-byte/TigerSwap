//! Type definitions for TigerSwap wallet

use std::collections::HashMap;
use chrono::{DateTime, Utc};

/// Wallet account
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct WalletAccount {
    pub id: String,
    pub name: String,
    pub created_at: DateTime<Utc>,
    pub keys: HashMap<u32, DerivedKey>,
    pub encrypted: bool,
}

/// Chain configuration
#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct ChainConfig {
    pub chain_id: u32,
    pub name: String,
    pub symbol: String,
    pub decimals: u8,
    pub rpc_url: String,
    pub explorer_url: String,
    pub coin_type: u32,
}

/// Wallet error types
#[derive(Debug, thiserror::Error)]
pub enum WalletError {
    #[error("Invalid mnemonic phrase")]
    InvalidMnemonic,
    
    #[error("Account not found")]
    AccountNotFound,
    
    #[error("Chain not supported")]
    ChainNotSupported,
    
    #[error("Invalid private key")]
    InvalidPrivateKey,
    
    #[error("Signing failed")]
    SigningFailed,
    
    #[error("Encryption failed")]
    EncryptionFailed,
    
    #[error("Decryption failed")]
    DecryptionFailed,
    
    #[error("Keystore locked")]
    KeystoreLocked,
    
    #[error("Invalid password")]
    InvalidPassword,
}

use crate::crypto::DerivedKey;
