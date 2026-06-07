//! Key Generation Module - DKG (Distributed Key Generation) protocol

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use uuid::Uuid;

/// Key Generation Parameters
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KeyGenParams {
    pub threshold: u32,           // t-of-n threshold
    pub total_nodes: u32,          // Total number of nodes n
    pub session_id: String,       // Unique session identifier
    pub key_type: KeyType,         // secp256k1 or ed25519
    pub created_at: i64,
}

impl KeyGenParams {
    pub fn new(threshold: u32, total_nodes: u32) -> Self {
        assert!(threshold <= total_nodes, "Threshold cannot exceed total nodes");
        assert!(threshold > 0, "Threshold must be at least 1");
        
        Self {
            threshold,
            total_nodes,
            session_id: Uuid::new_v4().to_string(),
            key_type: KeyType::Secp256k1,
            created_at: chrono::Utc::now().timestamp(),
        }
    }
    
    pub fn with_key_type(mut self, key_type: KeyType) -> Self {
        self.key_type = key_type;
        self
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum KeyType {
    Secp256k1,
    Ed25519,
}

impl Default for KeyType {
    fn default() -> Self {
        KeyType::Secp256k1
    }
}

/// Key Generation Status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum KeyGenStatus {
    Pending,
    Round1,      // Commitments distributed
    Round2,      // Shares distributed
    Round3,      // Public key computed
    Completed,
    Failed,
}

impl Default for KeyGenStatus {
    fn default() -> Self {
        KeyGenStatus::Pending
    }
}

/// Key Generation State Machine
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KeyGenState {
    pub session_id: String,
    pub params: KeyGenParams,
    pub round: u32,
    pub participants: Vec<String>,
    pub commitments: HashMap<String, Vec<u8>>,
    pub shares: HashMap<String, Vec<u8>>,
    pub public_key: Option<Vec<u8>>,
    pub started_at: i64,
    pub status: KeyGenStatus,
    pub completed_at: Option<i64>,
}

impl KeyGenState {
    pub fn new(params: KeyGenParams, participants: Vec<String>) -> Self {
        Self {
            session_id: params.session_id.clone(),
            params,
            round: 0,
            participants,
            commitments: HashMap::new(),
            shares: HashMap::new(),
            public_key: None,
            started_at: chrono::Utc::now().timestamp(),
            status: KeyGenStatus::Pending,
            completed_at: None,
        }
    }
    
    /// Advance to next round
    pub fn advance_round(&mut self) -> bool {
        match self.status {
            KeyGenStatus::Pending => {
                self.round = 1;
                self.status = KeyGenStatus::Round1;
                true
            }
            KeyGenStatus::Round1 => {
                self.round = 2;
                self.status = KeyGenStatus::Round2;
                true
            }
            KeyGenStatus::Round2 => {
                self.round = 3;
                self.status = KeyGenStatus::Round3;
                true
            }
            KeyGenStatus::Round3 => {
                self.status = KeyGenStatus::Completed;
                self.completed_at = Some(chrono::Utc::now().timestamp());
                true
            }
            KeyGenStatus::Completed | KeyGenStatus::Failed => false,
        }
    }
    
    /// Check if all participants have submitted
    pub fn all_submitted(&self, field: &str) -> bool {
        match field {
            "commitments" => self.commitments.len() == self.participants.len(),
            "shares" => self.shares.len() == self.participants.len(),
            _ => false,
        }
    }
    
    /// Get required participants for threshold
    pub fn required_participants(&self) -> usize {
        self.params.threshold as usize
    }
    
    /// Check if we have enough submissions
    pub fn has_quorum(&self, field: &str) -> bool {
        let count = match field {
            "commitments" => self.commitments.len(),
            "shares" => self.shares.len(),
            _ => 0,
        };
        count >= self.params.threshold as usize
    }
    
    /// Mark as failed
    pub fn mark_failed(&mut self) {
        self.status = KeyGenStatus::Failed;
    }
    
    /// Get duration in seconds
    pub fn duration(&self) -> i64 {
        let end = self.completed_at.unwrap_or_else(|| chrono::Utc::now().timestamp());
        end - self.started_at
    }
}

/// Commitment for VSS (Verifiable Secret Sharing)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Commitment {
    pub id: String,
    pub node_id: String,
    pub round: u32,
    pub commitments: Vec<Vec<u8>>,
    pub proof: Option<Vec<u8>>,
    pub timestamp: i64,
}

impl Commitment {
    pub fn new(node_id: String, round: u32, commitments: Vec<Vec<u8>>) -> Self {
        Self {
            id: Uuid::new_v4().to_string(),
            node_id,
            round,
            commitments,
            proof: None,
            timestamp: chrono::Utc::now().timestamp(),
        }
    }
}

/// Key Share for participant
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KeyShare {
    pub id: String,
    pub session_id: String,
    pub node_id: String,
    pub share_index: u32,
    pub share_data: Vec<u8>,
    pub encrypted: bool,
    pub created_at: i64,
}

impl KeyShare {
    pub fn new(session_id: String, node_id: String, share_index: u32, share_data: Vec<u8>) -> Self {
        Self {
            id: Uuid::new_v4().to_string(),
            session_id,
            node_id,
            share_index,
            share_data,
            encrypted: false,
            created_at: chrono::Utc::now().timestamp(),
        }
    }
    
    pub fn encrypt(&mut self) {
        self.encrypted = true;
    }
}

/// DKG Result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DKGResult {
    pub session_id: String,
    pub public_key: Vec<u8>,
    pub threshold: u32,
    pub total_nodes: u32,
    pub key_type: KeyType,
    pub created_at: i64,
}

impl DKGResult {
    pub fn to_address(&self) -> String {
        use sha2::{Digest, Sha256};
        let hash = Sha256::digest(&self.public_key);
        let address = &hash[12..];
        format!("0x{}", hex::encode(address))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_keygen_params() {
        let params = KeyGenParams::new(2, 3);
        
        assert_eq!(params.threshold, 2);
        assert_eq!(params.total_nodes, 3);
        assert!(!params.session_id.is_empty());
    }
    
    #[test]
    fn test_keygen_state() {
        let params = KeyGenParams::new(2, 3);
        let participants = vec!["node1".to_string(), "node2".to_string(), "node3".to_string()];
        
        let state = KeyGenState::new(params, participants);
        
        assert_eq!(state.status, KeyGenStatus::Pending);
        assert!(state.advance_round());
        assert_eq!(state.status, KeyGenStatus::Round1);
    }
    
    #[test]
    fn test_dkg_result() {
        let result = DKGResult {
            session_id: "test".to_string(),
            public_key: vec![1u8; 33],
            threshold: 2,
            total_nodes: 3,
            key_type: KeyType::Secp256k1,
            created_at: 1234567890,
        };
        
        let address = result.to_address();
        assert!(address.starts_with("0x"));
    }
}