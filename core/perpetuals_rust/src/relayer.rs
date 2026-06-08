//! TigerSwap Gasless Relayer - Meta-transactions
//! 
//! Enables gasless transactions for users

#![deny(unsafe_code)]

use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::time::{SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};
use thiserror::Error;

#[derive(Error, Debug, Clone)]
pub enum RelayerError {
    #[error("Invalid signature")]
    InvalidSignature,
    #[error("Nonce expired")]
    NonceExpired,
    #[error("Insufficient gas")]
    InsufficientGas,
    #[error("Transaction failed")]
    TransactionFailed,
    #[error("Unauthorized")]
    Unauthorized,
}

// ============ Meta Transaction ============

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MetaTransaction {
    pub from: String,
    pub to: String,
    pub value: u128,
    pub gas: u64,
    pub nonce: u64,
    pub data: Vec<u8>,
    pub expiry: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SignedTransaction {
    pub transaction: MetaTransaction,
    pub signature: Vec<u8>,
    pub gas_price: u128,
}

// ============ Relayer ============

pub struct GaslessRelayer {
    // Nonce management
    nonces: RwLock<HashMap<String, u64>>,
    
    // Fee configuration
    protocol_fee_bps: u64,
    max_gas_price: u128,
    
    // Authorized signers
    authorized_signers: RwLock<Vec<String>>,
    
    // Transaction history
    history: RwLock<HashMap<String, Vec<TransactionRecord>>>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TransactionRecord {
    pub tx_hash: String,
    pub from: String,
    pub to: String,
    pub gas_used: u64,
    pub gas_price: u128,
    pub timestamp: u64,
    pub status: TransactionStatus,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
pub enum TransactionStatus {
    Pending,
    Included,
    Failed,
}

impl GaslessRelayer {
    pub fn new() -> Self {
        Self {
            nonces: RwLock::new(HashMap::new()),
            protocol_fee_bps: 10, // 0.1%
            max_gas_price: 100_000_000_000, // 100 gwei
            authorized_signers: RwLock::new(Vec::new()),
            history: RwLock::new(HashMap::new()),
        }
    }
    
    /// Process a meta-transaction
    pub fn process_transaction(
        &self,
        signed_tx: &SignedTransaction,
    ) -> Result<String, RelayerError> {
        // Verify signature
        if signed_tx.signature.is_empty() {
            return Err(RelayerError::InvalidSignature);
        }
        
        // Check nonce
        let nonce_key = signed_tx.transaction.from.clone();
        let mut nonces = self.nonces.write().unwrap();
        let current_nonce = nonces.get(&nonce_key).copied().unwrap_or(0);
        
        if signed_tx.transaction.nonce < current_nonce {
            return Err(RelayerError::NonceExpired);
        }
        
        // Update nonce
        nonces.insert(nonce_key.clone(), signed_tx.transaction.nonce + 1);
        drop(nonces);
        
        // Check expiry
        let now = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_secs();
        
        if signed_tx.transaction.expiry < now {
            return Err(RelayerError::NonceExpired);
        }
        
        // Simulate transaction
        // In production, this would actually execute the transaction
        
        // Record transaction
        let tx_hash = self.record_transaction(
            &signed_tx.transaction.from,
            &signed_tx.transaction.to,
            signed_tx.transaction.gas,
            signed_tx.gas_price,
        )?;
        
        Ok(tx_hash)
    }
    
    /// Verify meta-transaction signature
    pub fn verify_signature(
        &self,
        tx: &MetaTransaction,
        signature: &[u8],
    ) -> Result<bool, RelayerError> {
        if signature.is_empty() {
            return Err(RelayerError::InvalidSignature);
        }
        
        // In production, verify using ecrecover
        Ok(true)
    }
    
    /// Get nonce for address
    pub fn get_nonce(&self, address: &str) -> u64 {
        let nonces = self.nonces.read().unwrap();
        *nonces.get(address).unwrap_or(&0)
    }
    
    /// Set protocol fee
    pub fn set_protocol_fee(&mut self, fee_bps: u64) {
        self.protocol_fee_bps = fee_bps;
    }
    
    /// Authorize signer
    pub fn authorize_signer(&self, address: &str) {
        let mut signers = self.authorized_signers.write().unwrap();
        if !signers.contains(&address.to_string()) {
            signers.push(address.to_string());
        }
    }
    
    /// Revoke signer
    pub fn revoke_signer(&self, address: &str) {
        let mut signers = self.authorized_signers.write().unwrap();
        signers.retain(|s| s != address);
    }
    
    /// Check if authorized
    pub fn is_authorized(&self, address: &str) -> bool {
        let signers = self.authorized_signers.read().unwrap();
        signers.contains(&address.to_string())
    }
    
    /// Get transaction history
    pub fn get_history(&self, address: &str) -> Vec<TransactionRecord> {
        let history = self.history.read().unwrap();
        history.get(address).cloned().unwrap_or_default()
    }
    
    fn record_transaction(
        &self,
        from: &str,
        to: &str,
        gas: u64,
        gas_price: u128,
    ) -> Result<String, RelayerError> {
        let tx_hash = format!("0x{:x}", rand_hash());
        
        let record = TransactionRecord {
            tx_hash: tx_hash.clone(),
            from: from.to_string(),
            to: to.to_string(),
            gas_used: gas,
            gas_price,
            timestamp: SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap()
                .as_secs(),
            status: TransactionStatus::Pending,
        };
        
        let mut history = self.history.write().unwrap();
        history.entry(from.to_string())
            .or_insert_with(Vec::new)
            .push(record);
        
        Ok(tx_hash)
    }
}

fn rand_hash() -> u64 {
    use std::time::{SystemTime, UNIX_EPOCH};
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .subsec_nanos() as u64;
    nanos
}

// ============ Forwarder ============

pub struct Forwarder {
    relayer: Arc<GaslessRelayer>,
    domain: String,
    chain_id: u64,
}

impl Forwarder {
    pub fn new(relayer: Arc<GaslessRelayer>, domain: &str, chain_id: u64) -> Self {
        Self {
            relayer,
            domain: domain.to_string(),
            chain_id,
        }
    }
    
    /// Build meta-transaction
    pub fn build_transaction(
        &self,
        from: &str,
        to: &str,
        data: Vec<u8>,
        gas: u64,
    ) -> MetaTransaction {
        let nonce = self.relayer.get_nonce(from);
        
        MetaTransaction {
            from: from.to_string(),
            to: to.to_string(),
            value: 0,
            gas,
            nonce,
            data,
            expiry: (SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap()
                .as_secs() + 3600), // 1 hour
        }
    }
    
    /// Execute meta-transaction
    pub fn execute(
        &self,
        signed_tx: SignedTransaction,
    ) -> Result<String, RelayerError> {
        // Build domain separator
        let domain_separator = self.build_domain_separator();
        
        // Verify signature
        let message = self.build_message(&signed_tx.transaction);
        if !self.relayer.verify_signature(&signed_tx.transaction, &signed_tx.signature)? {
            return Err(RelayerError::InvalidSignature);
        }
        
        // Process transaction
        self.relayer.process_transaction(&signed_tx)
    }
    
    fn build_domain_separator(&self) -> Vec<u8> {
        format!("{}:{}", self.domain, self.chain_id).as_bytes().to_vec()
    }
    
    fn build_message(&self, tx: &MetaTransaction) -> Vec<u8> {
        // EIP-712 domain separator
        format!(
            "{}:{}:{}:{}:{}",
            tx.from, tx.to, tx.value, tx.nonce, tx.expiry
        ).as_bytes().to_vec()
    }
}

// ============ Payment Token ============

#[derive(Debug, Clone)]
pub struct PaymentToken {
    pub address: String,
    pub min_amount: u128,
    pub max_amount: u128,
    pub fee_bps: u64,
}

impl PaymentToken {
    pub fn new(address: &str) -> Self {
        Self {
            address: address.to_string(),
            min_amount: 0,
            max_amount: u128::MAX,
            fee_bps: 0,
        }
    }
    
    pub fn with_limits(mut self, min: u128, max: u128) -> Self {
        self.min_amount = min;
        self.max_amount = max;
        self
    }
    
    pub fn with_fee(mut self, fee_bps: u64) -> Self {
        self.fee_bps = fee_bps;
        self
    }
}

// ============ Gas Station ============

pub struct GasStation {
    relayer: Arc<GaslessRelayer>,
    native_token: String,
    payment_tokens: RwLock<Vec<PaymentToken>>,
    min_gas_balance: u128,
}

impl GasStation {
    pub fn new(relayer: Arc<GaslessRelayer>, native_token: &str) -> Self {
        Self {
            relayer,
            native_token: native_token.to_string(),
            payment_tokens: RwLock::new(Vec::new()),
            min_gas_balance: 1_000_000_000_000_000_000, // 1 ETH
        }
    }
    
    /// Add payment token
    pub fn add_payment_token(&self, token: PaymentToken) {
        let mut tokens = self.payment_tokens.write().unwrap();
        tokens.push(token);
    }
    
    /// Get supported tokens
    pub fn get_supported_tokens(&self) -> Vec<PaymentToken> {
        let tokens = self.payment_tokens.read().unwrap();
        tokens.clone()
    }
    
    /// Estimate gas cost
    pub fn estimate_gas_cost(&self, gas: u64, gas_price: u128) -> u128 {
        (gas as u128) * gas_price
    }
    
    /// Check if user needs gas
    pub fn needs_gas(&self, balance: u128) -> bool {
        balance < self.min_gas_balance
    }
}

// ============ Tests ============

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_relayer_nonce() {
        let relayer = GaslessRelayer::new();
        
        assert_eq!(relayer.get_nonce("0x123"), 0);
        
        relayer.nonces.write().unwrap()
            .insert("0x123".to_string(), 5);
        
        assert_eq!(relayer.get_nonce("0x123"), 5);
    }
    
    #[test]
    fn test_authorization() {
        let relayer = GaslessRelayer::new();
        
        assert!(!relayer.is_authorized("0x123"));
        
        relayer.authorize_signer("0x123");
        
        assert!(relayer.is_authorized("0x123"));
        
        relayer.revoke_signer("0x123");
        
        assert!(!relayer.is_authorized("0x123"));
    }
    
    #[test]
    fn test_payment_token() {
        let token = PaymentToken::new("0xABC")
            .with_limits(1000, 1_000_000)
            .with_fee(10);
        
        assert_eq!(token.address, "0xABC");
        assert_eq!(token.min_amount, 1000);
        assert_eq!(token.max_amount, 1_000_000);
        assert_eq!(token.fee_bps, 10);
    }
}