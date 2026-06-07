//! MPC Coordinator Module - Orchestrates key generation and signing ceremonies

use serde::{Deserialize, Serialize};
use std::collections::{HashMap, HashSet};
use std::sync::Arc;
use std::time::Duration;
use tokio::sync::RwLock;
use uuid::Uuid;

use crate::{KeyGenParams, KeyGenState, KeyGenStatus, MPCError, MPCNode, SigningSession, SigningStatus};

/// MPC Coordinator - orchestrates key generation and signing ceremonies
pub struct MPCCoordinator {
    nodes: RwLock<HashMap<String, Arc<RwLock<MPCNode>>>>,
    active_sessions: RwLock<HashMap<String, KeyGenState>>,
    signing_sessions: RwLock<HashMap<String, SigningSession>>,
    secret_shares: RwLock<HashMap<String, Vec<u8>>>,
    threshold_config: RwLock<HashMap<String, (u32, u32)>>,
    completed_keys: RwLock<HashMap<String, Vec<u8>>>,
}

impl MPCCoordinator {
    /// Create new MPC coordinator
    pub fn new() -> Self {
        Self {
            nodes: RwLock::new(HashMap::new()),
            active_sessions: RwLock::new(HashMap::new()),
            signing_sessions: RwLock::new(HashMap::new()),
            secret_shares: RwLock::new(HashMap::new()),
            threshold_config: RwLock::new(HashMap::new()),
            completed_keys: RwLock::new(HashMap::new()),
        }
    }
    
    /// Register an MPC node
    pub async fn register_node(&self, node: MPCNode) {
        let node_id = node.id.clone();
        let mut nodes = self.nodes.write().await;
        nodes.insert(node_id, Arc::new(RwLock::new(node)));
    }
    
    /// Get all registered nodes
    pub async fn get_nodes(&self) -> Vec<MPCNode> {
        let nodes = self.nodes.read().await;
        nodes.values().map(|n| n.read().await.clone()).collect()
    }
    
    /// Get active nodes (responsive and healthy)
    pub async fn get_active_nodes(&self) -> Vec<String> {
        let nodes = self.nodes.read().await;
        let now = chrono::Utc::now().timestamp();
        let mut result = Vec::new();
        
        for (id, node) in nodes.iter() {
            let node = node.read().await;
            if now - node.last_heartbeat < 300 && node.is_active {
                result.push(id.clone());
            }
        }
        
        result
    }
    
    /// Get node by ID
    pub async fn get_node(&self, node_id: &str) -> Option<MPCNode> {
        let nodes = self.nodes.read().await;
        nodes.get(node_id).map(|n| n.read().await.clone())
    }
    
    /// Remove node
    pub async fn remove_node(&self, node_id: &str) {
        let mut nodes = self.nodes.write().await;
        nodes.remove(node_id);
    }
    
    /// Start MPC key generation ceremony
    pub async fn start_key_generation(
        &self,
        params: KeyGenParams,
    ) -> Result<String, MPCError> {
        let active_nodes = self.get_active_nodes().await;
        
        if active_nodes.len() < params.total_nodes as usize {
            return Err(MPCError::InvalidThreshold(
                params.total_nodes as usize,
                active_nodes.len(),
            ));
        }
        
        let state = KeyGenState::new(params.clone(), active_nodes);
        
        let mut sessions = self.active_sessions.write().await;
        sessions.insert(params.session_id.clone(), state);
        
        // Store threshold configuration
        let mut th = self.threshold_config.write().await;
        th.insert(params.session_id.clone(), (params.threshold, params.total_nodes));
        
        Ok(params.session_id)
    }
    
    /// Get key generation state
    pub async fn get_keygen_state(&self, session_id: &str) -> Option<KeyGenState> {
        let sessions = self.active_sessions.read().await;
        sessions.get(session_id).cloned()
    }
    
    /// Submit commitment from node
    pub async fn submit_commitment(
        &self,
        session_id: &str,
        node_id: &str,
        commitment: Vec<u8>,
    ) -> Result<(), MPCError> {
        let mut sessions = self.active_sessions.write().await;
        
        let session = sessions
            .get_mut(session_id)
            .ok_or(MPCError::KeyNotFound)?;
        
        if !session.participants.contains(node_id) {
            return Err(MPCError::InvalidParticipant);
        }
        
        session.commitments.insert(node_id.to_string(), commitment);
        
        // Check if we can advance
        if session.all_submitted("commitments") {
            session.advance_round();
        }
        
        Ok(())
    }
    
    /// Submit key share from node
    pub async fn submit_share(
        &self,
        session_id: &str,
        node_id: &str,
        share: Vec<u8>,
    ) -> Result<(), MPCError> {
        let mut sessions = self.active_sessions.write().await;
        
        let session = sessions
            .get_mut(session_id)
            .ok_or(MPCError::KeyNotFound)?;
        
        if !session.participants.contains(node_id) {
            return Err(MPCError::InvalidParticipant);
        }
        
        session.shares.insert(node_id.to_string(), share);
        
        // Check if we can advance
        if session.has_quorum("shares") {
            session.advance_round();
        }
        
        Ok(())
    }
    
    /// Compute final public key
    pub async fn compute_public_key(
        &self,
        session_id: &str,
    ) -> Result<Vec<u8>, MPCError> {
        let mut sessions = self.active_sessions.write().await;
        
        let session = sessions
            .get_mut(session_id)
            .ok_or(MPCError::KeyNotFound)?;
        
        if !session.has_quorum("shares") {
            return Err(MPCError::InsufficientSignatures(
                session.params.threshold as usize,
                session.shares.len(),
            ));
        }
        
        // Compute combined public key
        let mut public_key = [0u8; 33];
        for (_, share) in session.shares.iter() {
            for (i, byte) in share.iter().enumerate().take(33) {
                public_key[i] ^= byte;
            }
        }
        
        session.public_key = Some(public_key.to_vec());
        session.advance_round();
        
        // Store completed key
        let mut keys = self.completed_keys.write().await;
        keys.insert(session_id.to_string(), public_key.to_vec());
        
        Ok(public_key.to_vec())
    }
    
    /// Get completed public key
    pub async fn get_public_key(&self, session_id: &str) -> Option<Vec<u8>> {
        let keys = self.completed_keys.read().await;
        keys.get(session_id).cloned()
    }
    
    /// Store secret share for node
    pub async fn store_share(&self, session_id: &str, node_id: &str, share: Vec<u8>) {
        let key = format!("{}:{}", session_id, node_id);
        let mut shares = self.secret_shares.write().await;
        shares.insert(key, share);
    }
    
    /// Get secret share for node
    pub async fn get_share(&self, session_id: &str, node_id: &str) -> Option<Vec<u8>> {
        let key = format!("{}:{}", session_id, node_id);
        let shares = self.secret_shares.read().await;
        shares.get(&key).cloned()
    }
    
    /// Start threshold signing ceremony
    pub async fn start_signing(
        &self,
        session_id: &str,
        message: Vec<u8>,
    ) -> Result<String, MPCError> {
        use sha2::{Digest, Sha256};
        
        let th = self.threshold_config.read().await;
        let (threshold, total) = th
            .get(session_id)
            .ok_or(MPCError::KeyNotFound)?;
        
        let message_hash: [u8; 32] = Sha256::digest(&message).into();
        
        let signing_session = SigningSession {
            id: Uuid::new_v4().to_string(),
            session_id: session_id.to_string(),
            message,
            message_hash,
            threshold: *threshold,
            total_nodes: *total,
            partial_signatures: HashMap::new(),
            started_at: chrono::Utc::now().timestamp(),
            status: SigningStatus::Pending,
        };
        
        let signing_id = signing_session.id.clone();
        let mut sessions = self.signing_sessions.write().await;
        sessions.insert(signing_id.clone(), signing_session);
        
        Ok(signing_id)
    }
    
    /// Submit partial signature
    pub async fn submit_partial_signature(
        &self,
        signing_session_id: &str,
        node_id: &str,
        partial_sig: crate::PartialSignature,
    ) -> Result<(), MPCError> {
        let mut sessions = self.signing_sessions.write().await;
        
        let session = sessions
            .get_mut(signing_session_id)
            .ok_or(MPCError::KeyNotFound)?;
        
        if session.status == SigningStatus::Completed {
            return Err(MPCError::CeremonyFailed("Already completed".to_string()));
        }
        
        session.partial_signatures.insert(node_id.to_string(), partial_sig);
        session.status = SigningStatus::Collecting;
        
        // Check if we have enough
        if session.partial_signatures.len() >= session.threshold as usize {
            session.status = SigningStatus::Ready;
        }
        
        Ok(())
    }
    
    /// Get signing session
    pub async fn get_signing_session(&self, signing_id: &str) -> Option<SigningSession> {
        let sessions = self.signing_sessions.read().await;
        sessions.get(signing_id).cloned()
    }
    
    /// Combine partial signatures
    pub async fn combine_signatures(
        &self,
        signing_session_id: &str,
    ) -> Result<crate::ThresholdSignature, MPCError> {
        let sessions = self.signing_sessions.read().await;
        
        let session = sessions
            .get(signing_session_id)
            .ok_or(MPCError::KeyNotFound)?;
        
        if session.status != SigningStatus::Ready {
            return Err(MPCError::InsufficientSignatures(
                session.threshold as usize,
                session.partial_signatures.len(),
            ));
        }
        
        let signatures: Vec<crate::PartialSignature> = session.partial_signatures
            .values()
            .cloned()
            .collect();
        
        Ok(crate::ThresholdSignature {
            signatures,
            message_hash: session.message_hash,
            threshold: session.threshold,
            total_shares: session.total_nodes,
        })
    }
    
    /// Get active session count
    pub async fn active_session_count(&self) -> usize {
        let sessions = self.active_sessions.read().await;
        sessions.len()
    }
    
    /// Get signing session count
    pub async fn signing_session_count(&self) -> usize {
        let sessions = self.signing_sessions.read().await;
        sessions.len()
    }
    
    /// Clean up completed sessions
    pub async fn cleanup(&self, max_age_seconds: i64) {
        let now = chrono::Utc::now().timestamp();
        
        // Clean up key generation sessions
        {
            let mut sessions = self.active_sessions.write().await;
            sessions.retain(|_, s| {
                s.status != KeyGenStatus::Completed || 
                now - s.completed_at.unwrap_or(0) < max_age_seconds
            });
        }
        
        // Clean up signing sessions
        {
            let mut sessions = self.signing_sessions.write().await;
            sessions.retain(|_, s| {
                s.status != SigningStatus::Completed || 
                now - s.started_at < max_age_seconds
            });
        }
    }
}

impl Default for MPCCoordinator {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[tokio::test]
    async fn test_coordinator_registration() {
        let coordinator = MPCCoordinator::new();
        
        let node = MPCNode::new(
            "node1".to_string(),
            "127.0.0.1:8080".parse().unwrap(),
        );
        
        coordinator.register_node(node).await;
        
        let nodes = coordinator.get_nodes().await;
        assert_eq!(nodes.len(), 1);
    }
    
    #[tokio::test]
    async fn test_key_generation() {
        let coordinator = MPCCoordinator::new();
        
        // Register nodes
        for i in 1..=3 {
            let node = MPCNode::new(
                format!("node{}", i),
                format!("127.0.0.1:{}", 8080 + i).parse().unwrap(),
            );
            coordinator.register_node(node).await;
        }
        
        let params = KeyGenParams::new(2, 3);
        let session_id = coordinator.start_key_generation(params).await.unwrap();
        
        assert!(!session_id.is_empty());
    }
    
    #[tokio::test]
    async fn test_signing_ceremony() {
        let coordinator = MPCCoordinator::new();
        
        // Register nodes
        for i in 1..=3 {
            let node = MPCNode::new(
                format!("node{}", i),
                format!("127.0.0.1:{}", 8080 + i).parse().unwrap(),
            );
            coordinator.register_node(node).await;
        }
        
        // Store threshold config
        {
            let mut th = coordinator.threshold_config.write().await;
            th.insert("session1".to_string(), (2, 3));
        }
        
        let signing_id = coordinator
            .start_signing("session1", b"test message".to_vec())
            .await
            .unwrap();
        
        assert!(!signing_id.is_empty());
    }
}