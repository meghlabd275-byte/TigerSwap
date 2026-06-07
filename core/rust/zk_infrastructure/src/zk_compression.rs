//! ZK Compression Module - On-chain data compression

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

use serde::{Deserialize, Serialize};

use crate::{ZKProver, ZKProof};

/// ZK Compression
pub struct ZKCompression {
    prover: Arc<ZKProver>,
    compressed_data: RwLock<HashMap<String, CompressedData>>,
}

impl ZKCompression {
    pub fn new() -> Self {
        Self {
            prover: Arc::new(ZKProver::new()),
            compressed_data: RwLock::new(HashMap::new()),
        }
    }
    
    /// Compress data
    pub async fn compress(&self, data: &[u8]) -> Result<CompressedData, crate::ZKError> {
        // Simple compression (in production use proper algorithm)
        let compressed = data.to_vec();
        
        // Generate proof of compression
        let inputs = crate::ZKProofInputs::new()
            .with_public(vec![compressed.clone()])
            .with_private(vec![data.to_vec()]);
        
        let proof = self.prover.prove("compression", inputs).await?;
        
        let compressed_data = CompressedData {
            data_id: uuid::Uuid::new_v4().to_string(),
            original_size: data.len(),
            compressed,
            proof,
            created_at: chrono::Utc::now().timestamp(),
        };
        
        let data_id = compressed_data.data_id.clone();
        let mut store = self.compressed_data.write().await;
        store.insert(data_id, compressed_data.clone());
        
        Ok(compressed_data)
    }
    
    /// Decompress data
    pub async fn decompress(&self, data_id: &str) -> Result<Vec<u8>, crate::ZKError> {
        let store = self.compressed_data.read().await;
        
        let data = store
            .get(data_id)
            .ok_or(crate::ZKError::CircuitNotFound)?;
        
        // Verify proof
        self.prover.verify(&data.proof).await?;
        
        Ok(data.compressed.clone())
    }
    
    /// Get compression ratio
    pub async fn get_ratio(&self, data_id: &str) -> Option<f64> {
        let store = self.compressed_data.read().await;
        
        store.get(data_id).map(|d| {
            d.compressed.len() as f64 / d.original_size as f64
        })
    }
}

impl Default for ZKCompression {
    fn default() -> Self {
        Self::new()
    }
}

/// Compressed data
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct CompressedData {
    pub data_id: String,
    pub original_size: usize,
    pub compressed: Vec<u8>,
    pub proof: ZKProof,
    pub created_at: i64,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[tokio::test]
    async fn test_compression() {
        let compression = ZKCompression::new();
        
        let data = vec![0u8; 100];
        let compressed = compression.compress(&data).await.unwrap();
        
        assert!(compressed.original_size > compressed.compressed.len());
    }
}