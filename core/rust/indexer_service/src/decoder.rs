//! Decoder Module - Block and transaction decoding

use crate::{IndexerError, Block, Transaction};

/// Block decoder
pub struct BlockDecoder;

impl BlockDecoder {
    pub fn new() -> Self {
        Self
    }
    
    /// Decode block from raw data
    pub fn decode_block(&self, data: &[u8]) -> Result<Block, IndexerError> {
        if data.len() < 100 {
            return Err(IndexerError::InvalidBlock);
        }
        
        let block_number = u64::from_le_bytes([
            data.get(0).copied().unwrap_or(0),
            data.get(1).copied().unwrap_or(0),
            data.get(2).copied().unwrap_or(0),
            data.get(3).copied().unwrap_or(0),
            data.get(4).copied().unwrap_or(0),
            data.get(5).copied().unwrap_or(0),
            data.get(6).copied().unwrap_or(0),
            data.get(7).copied().unwrap_or(0),
        ]);
        
        Ok(Block::new(block_number))
    }
    
    /// Decode transaction from raw data
    pub fn decode_transaction(&self, data: &[u8]) -> Result<Transaction, IndexerError> {
        if data.is_empty() {
            return Err(IndexerError::InvalidBlock);
        }
        
        // Simplified transaction decoding
        Ok(Transaction::new(
            "0x".to_string(),
            "0x".to_string(),
            "0",
        ))
    }
    
    /// Decode receipt
    pub fn decode_receipt(&self, data: &[u8]) -> Result<bool, IndexerError> {
        if data.is_empty() {
            return Err(IndexerError::InvalidBlock);
        }
        
        // Check status byte
        let status = data.get(0).copied().unwrap_or(1);
        Ok(status == 1)
    }
}

impl Default for BlockDecoder {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_decode_block() {
        let decoder = BlockDecoder::new();
        let data = vec![0u8; 100];
        let block = decoder.decode_block(&data);
        assert!(block.is_ok());
    }
}