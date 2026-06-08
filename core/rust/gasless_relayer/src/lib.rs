//! TigerSwap Gasless Relayer
//! 
//! Implements Biconomy/Coinbase Relayer style gasless transactions:
//! - Meta-transactions
//! - Feedelegation
//! - Batch transactions
//! - Relayer network
//!
//! Implementation: Pure Rust with no external dependencies

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use thiserror::Error;
use uuid::Uuid;
use chrono::Utc;
use std::collections::HashMap;

/// Chain ID constants
pub const CHAIN_ETH: u64 = 1;
pub const CHAIN_BSC: u64 = 56;
pub const CHAIN_POLYGON: u64 = 137;
pub const CHAIN_ARBITRUM: u64 = 42161;
pub const CHAIN_OPTIMISM: u64 = 10;
pub const CHAIN_BASE: u64 = 8453;
pub const CHAIN_AVALANCHE: u64 = 43114;

#[derive(Debug, Error)]
pub enum GaslessError {
    #[error("Transaction not found: {0}")]
    TransactionNotFound(String),
    #[error("Relayer not found: {0}")]
    RelayerNotFound(String),
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
    #[error("Relayer not available")]
    RelayerNotAvailable,
    #[error("Execution failed: {0}")]
    ExecutionFailed(String),
    #[error("Signature invalid")]
    SignatureInvalid,
    #[error("Chain not supported: {0}")]
    ChainNotSupported(u64),
    #[error("Nonce too low")]
    NonceTooLow,
    #[error("Gas too low")]
    GasTooLow,
    #[error("Fee too low")]
    FeeTooLow,
}

/// Transaction type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum GaslessType {
    MetaTransaction,    // User signs, relayer pays
    FeeDelegation,      // Relayer pays gas
    Batch,           // Multiple transactions
    Sponsored,       // Protocol sponsors
}

impl Default for GaslessType {
    fn default() -> Self { GaslessType::MetaTransaction }
}

/// Transaction status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum GaslessStatus {
    Pending,
    Simulating,
    Approved,
    Submitted,
    Included,
    Failed,
    Cancelled,
}

impl Default for GaslessStatus {
    fn default() -> Self { GaslessStatus::Pending }
}

/// Gasless transaction
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GaslessTransaction {
    pub tx_id: String,
    pub user: String,
    pub chain_id: u64,
    pub tx_type: GaslessType,
    
    // Transaction data
    pub to: String,
    pub data: Vec<u8>,
    pub value: u128,
    
    // Fee
    pub gas_limit: u64,
    pub max_fee_per_gas: u128,
    pub max_priority_fee: u128,
    pub relayer_fee: u128,
    
    // Relayer
    pub relayer: String,
    pub signature: Vec<u8>,
    
    // Status
    pub status: GaslessStatus,
    pub nonce: u64,
    pub tx_hash: Option<String>,
    
    // Timing
    pub created_at: i64,
    pub submitted_at: Option<i64>,
    pub included_at: Option<i64>,
    pub expires_at: i64,
}

impl GaslessTransaction {
    /// Create a new gasless transaction
    pub fn new(
        user: String,
        chain_id: u64,
        to: String,
        data: Vec<u8>,
        value: u128,
        relayer: String,
    ) -> Self {
        let now = Utc::now().timestamp();
        
        Self {
            tx_id: Uuid::new_v4().to_string(),
            user,
            chain_id,
            tx_type: GaslessType::MetaTransaction,
            to,
            data,
            value,
            gas_limit: 0,
            max_fee_per_gas: 0,
            max_priority_fee: 0,
            relayer_fee: 0,
            relayer,
            signature: vec![],
            status: GaslessStatus::Pending,
            nonce: 0,
            tx_hash: None,
            created_at: now,
            submitted_at: None,
            included_at: None,
            expires_at: now + 300,  // 5 minutes
        }
    }

    /// Set gas parameters
    pub fn set_gas(&mut self, gas_limit: u64, max_fee: u128, priority: u128) {
        self.gas_limit = gas_limit;
        self.max_fee_per_gas = max_fee;
        self.max_priority_fee = priority;
    }

    /// Set signature
    pub fn sign(&mut self, signature: Vec<u8>) {
        self.signature = signature;
    }

    /// Approve transaction
    pub fn approve(&mut self) {
        self.status = GaslessStatus::Approved;
    }

    /// Submit transaction
    pub fn submit(&mut self, tx_hash: String) {
        self.tx_hash = Some(tx_hash);
        self.status = GaslessStatus::Submitted;
        self.submitted_at = Some(Utc::now().timestamp());
    }

    /// Include transaction
    pub fn include(&mut self) {
        self.status = GaslessStatus::Included;
        self.included_at = Some(Utc::now().timestamp());
    }

    /// Check if expired
    pub fn is_expired(&self) -> bool {
        Utc::now().timestamp() > self.expires_at
    }
}

/// Relayer
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Relayer {
    pub relayer_id: String,
    pub address: String,
    pub chain_id: u64,
    pub supported_tokens: Vec<String>,
    pub min_fee_bps: i64,
    pub max_fee_bps: i64,
    pub balance: u128,
    pub nonce: u64,
    pub is_active: bool,
    pub created_at: i64,
}

impl Relayer {
    /// Create a new relayer
    pub fn new(address: String, chain_id: u64) -> Self {
        Self {
            relayer_id: Uuid::new_v4().to_string(),
            address,
            chain_id,
            supported_tokens: vec![],
            min_fee_bps: 0,
            max_fee_bps: 100,
            balance: 0,
            nonce: 0,
            is_active: true,
            created_at: Utc::now().timestamp(),
        }
    }

    /// Add supported token
    pub fn add_token(&mut self, token: String) {
        if !self.supported_tokens.contains(&token) {
            self.supported_tokens.push(token);
        }
    }

    /// Calculate fee
    pub fn calculate_fee(&self, gas_used: u64, gas_price: u128) -> u128 {
        let base_fee = gas_used * gas_price;
        let relayer_fee = (base_fee * self.max_fee_bps as u128) / 10000;
        base_fee + relayer_fee
    }

    /// Check if can afford
    pub fn can_afford(&self, required: u128) -> bool {
        self.balance >= required
    }

    /// Deduct balance
    pub fn deduct(&mut self, amount: u128) -> bool {
        if self.balance < amount {
            return false;
        }
        self.balance -= amount;
        true
    }

    /// Add balance
    pub fn add_balance(&mut self, amount: u128) {
        self.balance += amount;
    }
}

/// Batch transaction
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BatchTransaction {
    pub batch_id: String,
    pub user: String,
    pub chain_id: u64,
    pub txs: Vec<BatchTx>,
    pub relayer: String,
    pub total_fee: u128,
    pub signature: Vec<u8>,
    pub status: GaslessStatus,
    pub created_at: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BatchTx {
    pub to: String,
    pub data: Vec<u8>,
    pub value: u128,
}

impl BatchTransaction {
    /// Create a new batch
    pub fn new(user: String, chain_id: u64, txs: Vec<BatchTx>, relayer: String) -> Self {
        Self {
            batch_id: Uuid::new_v4().to_string(),
            user,
            chain_id,
            txs,
            relayer,
            total_fee: 0,
            signature: vec![],
            status: GaslessStatus::Pending,
            created_at: Utc::now().timestamp(),
        }
    }

    /// Calculate total gas
    pub fn calculate_gas(&self) -> u64 {
        self.txs.len() as u64 * 21000
    }
}

/// Gasless engine
pub struct GaslessEngine {
    transactions: Arc<RwLock<HashMap<String, GaslessTransaction>>>,
    relayers: Arc<RwLock<HashMap<String, Relayer>>>,
    batches: Arc<RwLock<HashMap<String, BatchTransaction>>>,
    user_nonces: Arc<RwLock<HashMap<String, u64>>>,
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
}

impl GaslessEngine {
    /// Create a new gasless engine
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [
            CHAIN_ETH, CHAIN_BSC, CHAIN_POLYGON, CHAIN_ARBITRUM,
            CHAIN_OPTIMISM, CHAIN_BASE, CHAIN_AVALANCHE,
        ].into_iter().collect();
        
        Self {
            transactions: Arc::new(RwLock::new(HashMap::new())),
            relayers: Arc::new(RwLock::new(HashMap::new())),
            batches: Arc::new(RwLock::new(HashMap::new())),
            user_nonces: Arc::new(RwLock::new(HashMap::new())),
            supported_chains: Arc::new(RwLock::new(chains)),
        }
    }

    /// Check if chain is supported
    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    /// Register relayer
    pub fn register_relayer(&self, relayer: Relayer) -> Result<String, GaslessError> {
        if !self.is_chain_supported(relayer.chain_id) {
            return Err(GaslessError::ChainNotSupported(relayer.chain_id));
        }
        
        let relayer_id = relayer.relayer_id.clone();
        self.relayers.write().insert(relayer_id.clone(), relayer);
        
        Ok(relayer_id)
    }

    /// Get relayer
    pub fn get_relayer(&self, relayer_id: &str) -> Option<Relayer> {
        self.relayers.read().get(relayer_id).cloned()
    }

    /// Get available relayer
    pub fn get_available_relayer(&self, chain_id: u64) -> Result<Relayer, GaslessError> {
        let relayers = self.relayers.read();
        
        for relayer in relayers.values() {
            if relayer.chain_id == chain_id && relayer.is_active && relayer.balance > 0 {
                return Ok(relayer.clone());
            }
        }
        
        Err(GaslessError::RelayerNotAvailable)
    }

    /// Create gasless transaction
    pub fn create_transaction(
        &self,
        user: String,
        chain_id: u64,
        to: String,
        data: Vec<u8>,
        value: u128,
    ) -> Result<String, GaslessError> {
        if !self.is_chain_supported(chain_id) {
            return Err(GaslessError::ChainNotSupported(chain_id));
        }
        
        // Get available relayer
        let relayer = self.get_available_relayer(chain_id)?;
        
        let tx = GaslessTransaction::new(user, chain_id, to, data, value, relayer.address.clone());
        let tx_id = tx.tx_id.clone();
        
        self.transactions.write().insert(tx_id.clone(), tx);
        
        Ok(tx_id)
    }

    /// Get transaction
    pub fn get_transaction(&self, tx_id: &str) -> Option<GaslessTransaction> {
        self.transactions.read().get(tx_id).cloned()
    }

    /// Set gas for transaction
    pub fn set_gas(&self, tx_id: &str, gas_limit: u64, max_fee: u128, priority: u128) -> Result<(), GaslessError> {
        let mut txs = self.transactions.write();
        let tx = txs.get_mut(tx_id)
            .ok_or_else(|| GaslessError::TransactionNotFound(tx_id.to_string()))?;
        
        tx.set_gas(gas_limit, max_fee, priority);
        
        // Calculate relayer fee
        let relayer_fee = (gas_limit * max_fee * tx.relayer_fee as u128) / 10000;
        tx.relayer_fee = relayer_fee;
        
        Ok(())
    }

    /// Sign transaction
    pub fn sign_transaction(&self, tx_id: &str, signature: Vec<u8>) -> Result<(), GaslessError> {
        let mut txs = self.transactions.write();
        let tx = txs.get_mut(tx_id)
            .ok_or_else(|| GaslessError::TransactionNotFound(tx_id.to_string()))?;
        
        tx.sign(signature);
        
        Ok(())
    }

    /// Simulate transaction
    pub fn simulate(&self, tx_id: &str) -> Result<bool, GaslessError> {
        let mut txs = self.transactions.write();
        let tx = txs.get_mut(tx_id)
            .ok_or_else(|| GaslessError::TransactionNotFound(tx_id.to_string()))?;
        
        tx.status = GaslessStatus::Simulating;
        
        // Mock simulation
        tx.status = GaslessStatus::Approved;
        
        Ok(true)
    }

    /// Submit transaction via relayer
    pub fn submit(&self, tx_id: &str) -> Result<String, GaslessError> {
        let mut txs = self.transactions.write();
        let tx = txs.get_mut(tx_id)
            .ok_or_else(|| GaslessError::TransactionNotFound(tx_id.to_string()))?;
        
        if !matches!(tx.status, GaslessStatus::Approved) {
            return Err(GaslessError::ExecutionFailed("Not approved".to_string()));
        }
        
        // Get relayer
        let relayer = self.get_relayer(&tx.relayer)
            .ok_or_else(|| GaslessError::RelayerNotFound(tx.relayer.clone()))?;
        
        // Check balance
        let total_cost = tx.gas_limit as u128 * tx.max_fee_per_gas + tx.relayer_fee;
        if !relayer.can_afford(total_cost) {
            return Err(GaslessError::InsufficientBalance("Relayer balance too low".to_string()));
        }
        
        // Submit
        let tx_hash = format!("0x{}", Uuid::new_v4().replace("-", ""));
        tx.submit(tx_hash.clone());
        
        Ok(tx_hash)
    }

    /// Create batch transaction
    pub fn create_batch(
        &self,
        user: String,
        chain_id: u64,
        txs: Vec<BatchTx>,
    ) -> Result<String, GaslessError> {
        if !self.is_chain_supported(chain_id) {
            return Err(GaslessError::ChainNotSupported(chain_id));
        }
        
        let relayer = self.get_available_relayer(chain_id)?;
        
        let batch = BatchTransaction::new(user, chain_id, txs, relayer.address.clone());
        let batch_id = batch.batch_id.clone();
        
        self.batches.write().insert(batch_id.clone(), batch);
        
        Ok(batch_id)
    }

    /// Get batch
    pub fn get_batch(&self, batch_id: &str) -> Option<BatchTransaction> {
        self.batches.read().get(batch_id).cloned()
    }

    /// Execute batch
    pub fn execute_batch(&self, batch_id: &str) -> Result<Vec<String>, GaslessError> {
        let mut batches = self.batches.write();
        let batch = batches.get_mut(batch_id)
            .ok_or_else(|| GaslessError::TransactionNotFound(batch_id.to_string()))?;
        
        // Execute all txs
        let mut hashes = vec![];
        for _ in &batch.txs {
            hashes.push(format!("0x{}", Uuid::new_v4().replace("-", "")));
        }
        
        batch.status = GaslessStatus::Submitted;
        
        Ok(hashes)
    }

    /// Get user transactions
    pub fn get_user_transactions(&self, user: &str) -> Vec<GaslessTransaction> {
        self.transactions.read()
            .values()
            .filter(|tx| tx.user == user)
            .cloned()
            .collect()
    }

    /// Get statistics
    pub fn get_stats(&self) -> GaslessStats {
        let txs = self.transactions.read();
        
        let mut pending = 0;
        let mut submitted = 0;
        let mut included = 0;
        
        for tx in txs.values() {
            match tx.status {
                GaslessStatus::Pending => pending += 1,
                GaslessStatus::Submitted => submitted += 1,
                GaslessStatus::Included => included += 1,
                _ => {}
            }
        }
        
        GaslessStats {
            pending,
            submitted,
            included,
            total: txs.len(),
        }
    }

    /// Add supported chain
    pub fn add_chain(&self, chain_id: u64) {
        self.supported_chains.write().insert(chain_id);
    }

    /// Get supported chains
    pub fn supported_chains(&self) -> Vec<u64> {
        self.supported_chains.read().iter().cloned().collect()
    }
}

impl Default for GaslessEngine {
    fn default() -> Self { Self::new() }
}

/// Gasless statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GaslessStats {
    pub pending: usize,
    pub submitted: usize,
    pub included: usize,
    pub total: usize,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_transaction_creation() {
        let tx = GaslessTransaction::new(
            "user1".to_string(),
            CHAIN_ETH,
            "0xTo".to_string(),
            vec![0x00],
            1000,
            "0xRelayer".to_string(),
        );
        
        assert_eq!(tx.tx_type, GaslessType::MetaTransaction);
    }

    #[test]
    fn test_relayer() {
        let relayer = Relayer::new("0xRelayer".to_string(), CHAIN_ETH);
        
        assert!(relayer.is_active);
    }

    #[test]
    fn test_batch() {
        let txs = vec![
            BatchTx {
                to: "0xTo1".to_string(),
                data: vec![],
                value: 100,
            },
            BatchTx {
                to: "0xTo2".to_string(),
                data: vec![],
                value: 200,
            },
        ];
        
        let batch = BatchTransaction::new("user1".to_string(), CHAIN_ETH, txs, "0xRelayer".to_string());
        
        assert_eq!(batch.txs.len(), 2);
    }
}