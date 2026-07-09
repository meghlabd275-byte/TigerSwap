//! MPC Coordinator
//! 
//! Coordinates the distributed key generation and signing operations.

use crate::error::MpcError;
use crate::key_gen::{generate_dkg, combine_shares};
use crate::key_share::{decrypt_share, encrypt_share, verify_share};
use crate::threshold::{sign_partial, combine_signatures, verify_threshold_signature};
use crate::types::*;
use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

/// MPC Coordinator for managing key generation and signing
pub struct MpcCoordinator {
    /// Active key generation sessions
    key_gen_sessions: Arc<RwLock<HashMap<String, KeyGenSession>>>,
    /// Active signing sessions
    signing_sessions: Arc<RwLock<HashMap<String, SigningSession>>>,
    /// Cached key shares
    key_shares: Arc<RwLock<HashMap<String, Vec<KeyShare>>>>,
    /// Network nodes
    nodes: Arc<RwLock<HashMap<String, MpcNode>>>,
}

struct KeyGenSession {
    wallet_address: String,
    threshold: u32,
    total_shares: u32,
    participants: Vec<String>,
    commitments: Vec<Vec<u8>>,
    shares: Vec<Option<Vec<u8>>>,
    status: NetworkRoundStatus,
    started_at: i64,
}

struct SigningSession {
    wallet_address: String,
    message_hash: Vec<u8>,
    threshold: u32,
    participants: Vec<String>,
    partial_signatures: Vec<PartialSignature>,
    status: NetworkRoundStatus,
    started_at: i64,
}

impl MpcCoordinator {
    pub fn new() -> Self {
        Self {
            key_gen_sessions: Arc::new(RwLock::new(HashMap::new())),
            signing_sessions: Arc::new(RwLock::new(HashMap::new())),
            key_shares: Arc::new(RwLock::new(HashMap::new())),
            nodes: Arc::new(RwLock::new(HashMap::new())),
        }
    }

    /// Start a new key generation session
    pub async fn start_key_gen(
        &self,
        wallet_address: String,
        threshold: u32,
        total_shares: u32,
        guardians: Vec<String>,
    ) -> Result<String, MpcError> {
        if threshold > total_shares {
            return Err(MpcError::InvalidParameters(
                "Threshold must be less than or equal to total shares".to_string(),
            ));
        }

        let session_id = uuid::Uuid::new_v4().to_string();
        
        // Generate DKG
        let (_, params) = generate_dkg(threshold, total_shares)?;
        
        let session = KeyGenSession {
            wallet_address: wallet_address.clone(),
            threshold,
            total_shares,
            participants: guardians.clone(),
            commitments: params.commitment,
            shares: vec![None; total_shares as usize],
            status: NetworkRoundStatus::KeyGen,
            started_at: chrono::Utc::now().timestamp(),
        };

        self.key_gen_sessions.write().await.insert(session_id.clone(), session);

        // Store public key parameters
        // In production, save to database
        
        Ok(session_id)
    }

    /// Submit a share for key generation
    pub async fn submit_share(
        &self,
        session_id: &str,
        participant_id: &str,
        encrypted_share: &[u8],
        password: &str,
    ) -> Result<(), MpcError> {
        let mut sessions = self.key_gen_sessions.write().await;
        let session = sessions
            .get_mut(session_id)
            .ok_or_else(|| MpcError::InvalidParameters("Session not found".to_string()))?;

        if session.status != NetworkRoundStatus::KeyGen {
            return Err(MpcError::InvalidParameters("Session not in KeyGen state".to_string()));
        }

        // Decrypt share
        let share = decrypt_share(encrypted_share, password)?;
        
        // Verify share
        if !verify_share(&share)? {
            return Err(MpcError::InvalidShare("Share verification failed".to_string()));
        }

        // Store share
        let index = (share.share_index - 1) as usize;
        if index < session.shares.len() {
            session.shares[index] = Some(share.share_value.clone());
        }

        // Check if all shares received
        if session.shares.iter().filter(|s| s.is_some()).count() >= session.threshold as usize {
            session.status = NetworkRoundStatus::Complete;
        }

        Ok(())
    }

    /// Complete key generation and get the wallet key
    pub async fn complete_key_gen(
        &self,
        session_id: &str,
    ) -> Result<Vec<u8>, MpcError> {
        let sessions = self.key_gen_sessions.read().await;
        let session = sessions
            .get(session_id)
            .ok_or_else(|| MpcError::InvalidParameters("Session not found".to_string()))?;

        if session.status != NetworkRoundStatus::Complete {
            return Err(MpcError::InvalidParameters("Key generation not complete".to_string()));
        }

        // Combine shares
        let shares: Vec<(u32, Vec<u8>)> = session
            .shares
            .iter()
            .enumerate()
            .filter_map(|(i, s)| s.as_ref().map(|v| (i as u32 + 1, v.clone())))
            .collect();

        let secret = combine_shares(&shares, session.threshold)?;
        
        Ok(secret.to_vec())
    }

    /// Start a signing session
    pub async fn start_signing(
        &self,
        wallet_address: String,
        message_hash: Vec<u8>,
        threshold: u32,
    ) -> Result<String, MpcError> {
        let session_id = uuid::Uuid::new_v4().to_string();
        
        let session = SigningSession {
            wallet_address: wallet_address.clone(),
            message_hash: message_hash.clone(),
            threshold,
            participants: Vec::new(),
            partial_signatures: Vec::new(),
            status: NetworkRoundStatus::Signing,
            started_at: chrono::Utc::now().timestamp(),
        };

        self.signing_sessions.write().await.insert(session_id.clone(), session);
        
        Ok(session_id)
    }

    /// Submit a partial signature
    pub async fn submit_partial_signature(
        &self,
        session_id: &str,
        node_id: &str,
        partial_signature: Vec<u8>,
    ) -> Result<(), MpcError> {
        let mut sessions = self.signing_sessions.write().await;
        let session = sessions
            .get_mut(session_id)
            .ok_or_else(|| MpcError::InvalidParameters("Session not found".to_string()))?;

        session.partial_signatures.push(PartialSignature {
            node_id: node_id.to_string(),
            signature: partial_signature,
        });

        // Check if we have enough signatures
        if session.partial_signatures.len() >= session.threshold as usize {
            session.status = NetworkRoundStatus::Complete;
        }

        Ok(())
    }

    /// Complete signing and get the final signature
    pub async fn complete_signing(
        &self,
        session_id: &str,
    ) -> Result<SignatureResponse, MpcError> {
        let sessions = self.signing_sessions.read().await;
        let session = sessions
            .get(session_id)
            .ok_or_else(|| MpcError::InvalidParameters("Session not found".to_string()))?;

        if session.status != NetworkRoundStatus::Complete {
            return Err(MpcError::InvalidParameters("Signing not complete".to_string()));
        }

        let result = combine_signatures(
            &session.partial_signatures,
            session.threshold,
        )?;

        Ok(result)
    }

    /// Register a node in the network
    pub async fn register_node(&self, node: MpcNode) -> Result<(), MpcError> {
        self.nodes.write().await.insert(node.node_id.clone(), node);
        Ok(())
    }

    /// Get active nodes
    pub async fn get_active_nodes(&self) -> Result<Vec<MpcNode>, MpcError> {
        let nodes = self.nodes.read().await;
        let active: Vec<MpcNode> = nodes
            .values()
            .filter(|n| n.status == NodeStatus::Active)
            .cloned()
            .collect();
        
        Ok(active)
    }
}

impl Default for MpcCoordinator {
    fn default() -> Self {
        Self::new()
    }
}
