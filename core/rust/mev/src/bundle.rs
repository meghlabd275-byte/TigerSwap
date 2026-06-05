//! Bundle Builder for MEV Protection

use crate::detection::TransactionData;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BundleTransaction {
    pub to: String,
    pub data: String,
    pub value: String,
    pub max_fee_per_gas: String,
    pub max_priority_fee_per_gas: String,
    pub nonce: u64,
    pub chain_id: u64,
}

pub struct BundleBuilder {
    transactions: Vec<TransactionData>,
    enable_randomization: bool,
}

impl BundleBuilder {
    pub fn new() -> Self {
        Self {
            transactions: Vec::new(),
            enable_randomization: true,
        }
    }

    pub fn add_transaction(&mut self, tx: TransactionData) {
        self.transactions.push(tx);
    }

    pub fn randomize_order(&mut self) {
        if self.enable_randomization {
            // Simple Fisher-Yates shuffle
            for i in (1..self.transactions.len()).rev() {
                let j = (i as u64 % (i as u64 + 1)) as usize;
                self.transactions.swap(i, j);
            }
        }
    }

    pub fn build(&mut self) -> Vec<BundleTransaction> {
        self.randomize_order();
        
        self.transactions.iter().map(|tx| BundleTransaction {
            to: tx.to.clone(),
            data: tx.data.clone(),
            value: tx.value.clone(),
            max_fee_per_gas: "100".to_string(),
            max_priority_fee_per_gas: "2".to_string(),
            nonce: 0,
            chain_id: 1,
        }).collect()
    }

    pub fn clear(&mut self) {
        self.transactions.clear();
    }

    pub fn len(&self) -> usize {
        self.transactions.len()
    }
}

impl Default for BundleBuilder {
    fn default() -> Self {
        Self::new()
    }
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BundleResult {
    pub success: bool,
    pub bundle_hash: Option<String>,
    pub transaction_hash: Option<String>,
    pub error: Option<String>,
    pub gas_used: Option<String>,
    pub effective_gas_price: Option<String>,
    pub block_number: Option<u64>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_bundle_builder() {
        let mut builder = BundleBuilder::new();
        builder.add_transaction(TransactionData {
            from: "0xA".to_string(),
            to: "0xB".to_string(),
            data: "0x".to_string(),
            value: "0".to_string(),
            gas_price: "50".to_string(),
            hash: Some("0x1".to_string()),
        });
        
        assert_eq!(builder.len(), 1);
        
        let bundle = builder.build();
        assert_eq!(bundle.len(), 1);
    }
}