//! ZK Identity Module - Anonymous authentication

use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

use serde::{Deserialize, Serialize};

use crate::{ZKProver, ZKProof};

/// ZK Identity
pub struct ZKIdentity {
    prover: Arc<ZKProver>,
    identities: RwLock<HashMap<String, ZKIdentityRecord>>,
}

impl ZKIdentity {
    pub fn new() -> Self {
        Self {
            prover: Arc::new(ZKProver::new()),
            identities: RwLock::new(HashMap::new()),
        }
    }
    
    /// Register identity
    pub async fn register(&self, identity: ZKIdentityRecord) -> Result<String, crate::ZKError> {
        let mut identities = self.identities.write().await;
        let id = identity.identity_id.clone();
        identities.insert(id.clone(), identity);
        Ok(id)
    }
    
    /// Generate proof of identity
    pub async fn prove_identity(
        &self,
        identity_id: &str,
        challenge: &[u8],
    ) -> Result<ZKProof, crate::ZKError> {
        let identities = self.identities.read().await;
        
        let identity = identities
            .get(identity_id)
            .ok_or(crate::ZKError::CircuitNotFound)?;
        
        let inputs = crate::ZKProofInputs::new()
            .with_public(vec![challenge.to_vec()])
            .with_private(vec![identity.secret.clone()]);
        
        self.prover.prove("identity", inputs).await
    }
    
    /// Verify identity proof
    pub async fn verify_identity(&self, proof: &ZKProof) -> Result<bool, crate::ZKError> {
        self.prover.verify(proof).await
    }
    
    /// Get identity
    pub async fn get_identity(&self, identity_id: &str) -> Option<ZKIdentityRecord> {
        let identities = self.identities.read().await;
        identities.get(identity_id).cloned()
    }
}

impl Default for ZKIdentity {
    fn default() -> Self {
        Self::new()
    }
}

/// Identity record
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ZKIdentityRecord {
    pub identity_id: String,
    pub secret: Vec<u8>,
    pub attributes: HashMap<String, String>,
    pub created_at: i64,
    pub verified: bool,
}

impl ZKIdentityRecord {
    pub fn new(secret: Vec<u8>) -> Self {
        Self {
            identity_id: uuid::Uuid::new_v4().to_string(),
            secret,
            attributes: HashMap::new(),
            created_at: chrono::Utc::now().timestamp(),
            verified: false,
        }
    }
    
    pub fn with_attribute(mut self, key: &str, value: &str) -> Self {
        self.attributes.insert(key.to_string(), value.to_string());
        self
    }
    
    pub fn verify(&mut self) {
        self.verified = true;
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[tokio::test]
    async fn test_identity() {
        let identity = ZKIdentity::new();
        
        let record = ZKIdentityRecord::new(vec![1, 2, 3, 4]);
        let identity_id = identity.register(record).await.unwrap();
        
        let proof = identity
            .prove_identity(&identity_id, b"challenge")
            .await
            .unwrap();
        
        assert!(identity.verify_identity(&proof).await.unwrap());
    }
}