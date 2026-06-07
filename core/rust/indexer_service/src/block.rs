//! Block Module

use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

/// Block
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Block {
    pub block_number: u64,
    pub block_hash: String,
    pub parent_hash: String,
    pub timestamp: i64,
    pub miner: String,
    pub gas_limit: u64,
    pub gas_used: u64,
    pub transaction_count: u64,
    pub base_fee_per_gas: Option<u128>,
}

impl Block {
    pub fn new(block_number: u64) -> Self {
        let hash = Sha256::digest(&block_number.to_le_bytes());
        Self {
            block_number,
            block_hash: format!("0x{}", hex::encode(hash)),
            parent_hash: format!("0x{:x}", Sha256::digest(&((block_number.saturating_sub(1)).to_le_bytes()))),
            timestamp: chrono::Utc::now().timestamp(),
            miner: String::new(),
            gas_limit: 30_000_000,
            gas_used: 0,
            transaction_count: 0,
            base_fee_per_gas: Some(10000000000),
        }
    }
    
    pub fn with_miner(mut self, miner: String) -> Self {
        self.miner = miner;
        self
    }
    
    pub fn with_gas_used(mut self, gas: u64) -> Self {
        self.gas_used = gas;
        self
    }
}

/// Transaction
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Transaction {
    pub tx_hash: String,
    pub block_number: u64,
    pub from: String,
    pub to: String,
    pub value: String,
    pub gas_price: u128,
    pub gas_limit: u64,
    pub data: Vec<u8>,
    pub nonce: u64,
    pub transaction_type: TransactionType,
    pub status: TransactionStatus,
    pub log_index: u64,
}

impl Transaction {
    pub fn new(from: String, to: String, value: &str) -> Self {
        let data = format!("{}:{}:{}", from, to, value);
        Self {
            tx_hash: format!("0x{:x}", Sha256::digest(data.as_bytes())),
            block_number: 0,
            from,
            to,
            value: value.to_string(),
            gas_price: 0,
            gas_limit: 21000,
            data: Vec::new(),
            nonce: 0,
            transaction_type: TransactionType::Legacy,
            status: TransactionStatus::Pending,
            log_index: 0,
        }
    }
    
    pub fn is_internal_transfer(&self) -> bool {
        self.data.is_empty() && !self.to.is_empty()
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TransactionType {
    Legacy,
    EIP2930,
    EIP1559,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TransactionStatus {
    Pending,
    Success,
    Failed,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_block() {
        let block = Block::new(12345);
        assert_eq!(block.block_number, 12345);
    }
    
    #[test]
    fn test_transaction() {
        let tx = Transaction::new("0xfrom".to_string(), "0xto".to_string(), "100");
        assert_eq!(tx.value, "100");
    }
}