//! MPC Types for TigerWallet

use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Public parameters for MPC key generation
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PublicParameters {
    /// The threshold required for signature (t of n)
    pub threshold: u32,
    /// Total number of shares (n)
    pub total_shares: u32,
    /// The generated public key (compressed)
    pub public_key: Vec<u8>,
    /// Commitment to the polynomial
    pub commitment: Vec<Vec<u8>>,
}

/// Secret share for a participant
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KeyShare {
    /// Unique identifier for this share
    pub share_id: String,
    /// The wallet address this share belongs to
    pub wallet_address: String,
    /// Index of this share in the threshold scheme
    pub share_index: u32,
    /// The actual share value (encrypted)
    pub share_value: Vec<u8>,
    /// Encrypted backup of the share
    pub encrypted_backup: Vec<u8>,
    /// Creation timestamp
    pub created_at: i64,
    /// Last rotation timestamp
    pub rotated_at: Option<i64>,
}

/// Request for generating new MPC keys
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KeyGenRequest {
    /// The wallet address to generate keys for
    pub wallet_address: String,
    /// Number of shares to generate
    pub total_shares: u32,
    /// Threshold required for signing
    pub threshold: u32,
    /// Optional guardian addresses for social recovery
    pub guardians: Option<Vec<String>>,
}

/// Response from key generation
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KeyGenResponse {
    /// The generated wallet address
    pub wallet_address: String,
    /// The public key
    pub public_key: Vec<u8>,
    /// Encrypted shares for each participant
    pub shares: Vec<EncryptedShare>,
    /// Quantum-resistant backup
    pub backup: Vec<u8>,
}

/// Encrypted share for a participant
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EncryptedShare {
    pub share_id: String,
    pub recipient_id: String,
    pub encrypted_value: Vec<u8>,
    pub ephemeral_public_key: Vec<u8>,
}

/// Signing request
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SignRequest {
    /// The message to sign (hash)
    pub message_hash: Vec<u8>,
    /// Wallet address
    pub wallet_address: String,
    /// Partial signatures from participants
    pub partial_signatures: Vec<PartialSignature>,
}

/// Partial signature from one MPC node
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PartialSignature {
    /// Node ID
    pub node_id: String,
    /// The partial signature (R || s)
    pub signature: Vec<u8>,
}

/// Complete signature
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SignatureResponse {
    /// The final signature
    pub signature: Vec<u8>,
    /// Public key used
    pub public_key: Vec<u8>,
    /// Recovery ID
    pub recovery_id: u8,
}

/// Key rotation request
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KeyRotationRequest {
    pub wallet_address: String,
    pub old_shares: Vec<String>,
    pub new_guardians: Vec<String>,
}

/// Recovery request
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RecoveryRequest {
    pub wallet_address: String,
    pub guardian_signatures: Vec<GuardianSignature>,
    pub new_owner_public_key: Vec<u8>,
}

/// Signature from a guardian
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GuardianSignature {
    pub guardian_address: String,
    pub signature: Vec<u8>,
}

/// Node information for MPC network
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MpcNode {
    pub node_id: String,
    pub public_key: Vec<u8>,
    pub endpoint: String,
    pub status: NodeStatus,
    pub region: String,
    pub load: f32,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum NodeStatus {
    Active,
    Inactive,
    Maintenance,
    Joing,
}

/// MPC network state
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NetworkState {
    pub round_id: String,
    pub participants: Vec<String>,
    pub status: NetworkRoundStatus,
    pub started_at: i64,
    pub expires_at: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub enum NetworkRoundStatus {
    Waiting,
    KeyGen,
    Signing,
    Complete,
    Failed,
}

/// DKG (Distributed Key Generation) message
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DkgMessage {
    pub round_id: String,
    pub from_node: String,
    pub to_node: Option<String>,
    pub message_type: DkgMessageType,
    pub payload: Vec<u8>,
    pub timestamp: i64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum DkgMessageType {
    Commit,
    Share,
    Complaint,
    Justification,
    Finalize,
}
