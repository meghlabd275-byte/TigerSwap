//! ZK Bridge Module - Cross-chain messaging with privacy

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

use serde::{Deserialize, Serialize};

use crate::{ZKProver, ZKProof, MessageStatus};

/// ZK Bridge
pub struct ZKBridge {
    prover: Arc<ZKProver>,
    messages: RwLock<HashMap<String, ZKBridgeMessage>>,
    proofs: RwLock<HashMap<String, ZKProof>>,
}

impl ZKBridge {
    pub fn new() -> Self {
        Self {
            prover: Arc::new(ZKProver::new()),
            messages: RwLock::new(HashMap::new()),
            proofs: RwLock::new(HashMap::new()),
        }
    }
    
    /// Send message to another chain
    pub async fn send_message(
        &self,
        dest_chain: u64,
        message: Vec<u8>,
        sender: &str,
    ) -> Result<String, crate::ZKError> {
        let bridge_message = ZKBridgeMessage {
            message_id: uuid::Uuid::new_v4().to_string(),
            source_chain: 1, // Ethereum
            dest_chain,
            sender: sender.to_string(),
            message: message.clone(),
            status: MessageStatus::Pending,
            created_at: chrono::Utc::now().timestamp(),
        };
        
        let message_id = bridge_message.message_id.clone();
        
        // Generate proof of message
        let inputs = crate::ZKProofInputs::new()
            .with_public(vec![message])
            .with_private(vec![]);
        
        let proof = self.prover.prove("bridge", inputs).await?;
        
        let mut messages = self.messages.write().await;
        messages.insert(message_id.clone(), bridge_message);
        
        let mut proofs = self.proofs.write().await;
        proofs.insert(message_id.clone(), proof);
        
        Ok(message_id)
    }
    
    /// Verify message
    pub async fn verify_message(&self, message_id: &str) -> Result<bool, crate::ZKError> {
        let proofs = self.proofs.read().await;
        
        if let Some(proof) = proofs.get(message_id) {
            self.prover.verify(proof).await
        } else {
            Err(crate::ZKError::VerificationFailed)
        }
    }
    
    /// Get message status
    pub async fn get_status(&self, message_id: &str) -> Option<MessageStatus> {
        let messages = self.messages.read().await;
        messages.get(message_id).map(|m| m.status)
    }
    
    /// Update message status
    pub async fn update_status(&self, message_id: &str, status: MessageStatus) {
        let mut messages = self.messages.write().await;
        if let Some(msg) = messages.get_mut(message_id) {
            msg.status = status;
        }
    }
}

impl Default for ZKBridge {
    fn default() -> Self {
        Self::new()
    }
}

/// Bridge message
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ZKBridgeMessage {
    pub message_id: String,
    pub source_chain: u64,
    pub dest_chain: u64,
    pub sender: String,
    pub message: Vec<u8>,
    pub status: MessageStatus,
    pub created_at: i64,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[tokio::test]
    async fn test_bridge() {
        let bridge = ZKBridge::new();
        
        let message_id = bridge
            .send_message(42161, vec![1, 2, 3], "sender")
            .await
            .unwrap();
        
        assert!(!message_id.is_empty());
    }
}