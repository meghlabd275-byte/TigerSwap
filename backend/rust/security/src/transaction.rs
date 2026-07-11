//! Transaction Module
//! 
//! Transaction signing and verification for the DEX

use crate::key_management::{KeyManager, KeyError};
use serde::{Serialize, Deserialize};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum TransactionError {
    #[error("Invalid transaction: {0}")]
    InvalidTransaction(String),
    #[error("Signing error: {0}")]
    SigningError(#[from] KeyError),
    #[error("Validation error: {0}")]
    ValidationError(String),
}

/// Transaction types
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum TransactionType {
    /// Basic token transfer
    Transfer {
        to: String,
        value: String,
        data: Option<String>,
    },
    /// Token swap
    Swap {
        path: Vec<String>,
        amount_in: String,
        amount_out_min: String,
    },
    /// Add liquidity
    AddLiquidity {
        token_a: String,
        token_b: String,
        amount_a_desired: String,
        amount_b_desired: String,
        amount_a_min: String,
        amount_b_min: String,
    },
    /// Remove liquidity
    RemoveLiquidity {
        token_a: String,
        token_b: String,
        liquidity: String,
        amount_a_min: String,
        amount_b_min: String,
    },
    /// Cross-chain bridge
    Bridge {
        to_chain: u64,
        token: String,
        amount: String,
        recipient: String,
    },
}

/// Unsigned transaction
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct UnsignedTransaction {
    pub chain_id: u64,
    pub nonce: u64,
    pub to: String,
    pub value: String,
    pub data: String,
    pub gas_limit: u64,
    pub gas_price: u64,
    pub transaction_type: TransactionType,
}

/// Signed transaction
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SignedTransaction {
    pub unsigned: UnsignedTransaction,
    pub signature: Vec<u8>,
    pub sender: String,
}

impl UnsignedTransaction {
    /// Get the transaction hash (for signing)
    pub fn hash(&self) -> [u8; 32] {
        use sha3::{Keccak256, Digest};
        let mut hasher = Keccak256::new();
        
        // Simplified hash calculation
        hasher.update(self.chain_id.to_le_bytes());
        hasher.update(self.nonce.to_le_bytes());
        hasher.update(&self.to);
        hasher.update(&self.value);
        hasher.update(&self.data);
        
        let result = hasher.finalize();
        let mut hash = [0u8; 32];
        hash.copy_from_slice(&result);
        hash
    }
    
    /// Encode as RLP (simplified)
    pub fn encode_rlp(&self) -> Vec<u8> {
        // In production, implement proper RLP encoding
        let mut encoded = Vec::new();
        encoded.extend_from_slice(&self.chain_id.to_le_bytes());
        encoded.extend_from_slice(&self.nonce.to_le_bytes());
        encoded.extend_from_slice(&self.to.as_bytes());
        encoded
    }
}

impl SignedTransaction {
    /// Sign an unsigned transaction
    pub async fn sign(
        unsigned: UnsignedTransaction,
        key_manager: &KeyManager,
        sender: &str,
    ) -> Result<Self, TransactionError> {
        let hash = unsigned.hash();
        let signature = key_manager.sign(sender, &hash)
            .await
            .map_err(|e| TransactionError::SigningError(e))?;
        
        Ok(Self {
            unsigned,
            signature,
            sender: sender.to_string(),
        })
    }
    
    /// Verify transaction signature
    pub fn verify(&self) -> bool {
        // In production, implement proper signature verification
        !self.signature.is_empty()
    }
}

/// Builder for transactions
pub struct TransactionBuilder {
    chain_id: u64,
    nonce: u64,
    to: String,
    value: String,
    data: String,
    gas_limit: u64,
    gas_price: u64,
}

impl TransactionBuilder {
    pub fn new(chain_id: u64) -> Self {
        Self {
            chain_id,
            nonce: 0,
            to: String::new(),
            value: "0".to_string(),
            data: String::new(),
            gas_limit: 21000,
            gas_price: 0,
        }
    }
    
    pub fn to(mut self, to: impl Into<String>) -> Self {
        self.to = to.into();
        self
    }
    
    pub fn value(mut self, value: impl Into<String>) -> Self {
        self.value = value.into();
        self
    }
    
    pub fn data(mut self, data: impl Into<String>) -> Self {
        self.data = data.into();
        self
    }
    
    pub fn gas_limit(mut self, limit: u64) -> Self {
        self.gas_limit = limit;
        self
    }
    
    pub fn gas_price(mut self, price: u64) -> Self {
        self.gas_price = price;
        self
    }
    
    pub fn nonce(mut self, nonce: u64) -> Self {
        self.nonce = nonce;
        self
    }
    
    pub fn build(self, transaction_type: TransactionType) -> UnsignedTransaction {
        UnsignedTransaction {
            chain_id: self.chain_id,
            nonce: self.nonce,
            to: self.to,
            value: self.value,
            data: self.data,
            gas_limit: self.gas_limit,
            gas_price: self.gas_price,
            transaction_type,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_transaction_builder() {
        let tx = TransactionBuilder::new(1)
            .to("0x742d35Cc6634C0532925a3b844Bc9e7595f0fEb1")
            .value("1000000000000000000")
            .gas_limit(21000)
            .gas_price(1000000000)
            .build(TransactionType::Transfer {
                to: "0x742d35Cc6634C0532925a3b844Bc9e7595f0fEb1".to_string(),
                value: "1000000000000000000".to_string(),
                data: None,
            });
        
        assert_eq!(tx.chain_id, 1);
        assert_eq!(tx.gas_limit, 21000);
    }
}
