//! MPC Service Errors

use thiserror::Error;

#[derive(Error, Debug)]
pub enum MpcError {
    #[error("Key generation failed: {0}")]
    KeyGenerationFailed(String),

    #[error("Signing failed: {0}")]
    SigningFailed(String),

    #[error("Invalid share: {0}")]
    InvalidShare(String),

    #[error("Threshold not met: need {need}, have {have}")]
    ThresholdNotMet { need: u32, have: u32 },

    #[error("Invalid signature: {0}")]
    InvalidSignature(String),

    #[error("Network error: {0}")]
    NetworkError(String),

    #[error("Encryption error: {0}")]
    EncryptionError(String),

    #[error("Decryption error: {0}")]
    DecryptionError(String),

    #[error("Node not found: {0}")]
    NodeNotFound(String),

    #[error("Invalid parameters: {0}")]
    InvalidParameters(String),

    #[error("Timeout: {0}")]
    Timeout(String),

    #[error("Database error: {0}")]
    DatabaseError(String),

    #[error("Serialization error: {0}")]
    SerializationError(String),

    #[error("Recovery failed: {0}")]
    RecoveryFailed(String),

    #[error("Rotation failed: {0}")]
    RotationFailed(String),
}

impl From<secp256k1::Error> for MpcError {
    fn from(e: secp256k1::Error) -> Self {
        MpcError::SigningFailed(e.to_string())
    }
}

impl From<serde_json::Error> for MpcError {
    fn from(e: serde_json::Error) -> Self {
        MpcError::SerializationError(e.to_string())
    }
}

impl From<base64::DecodeError> for MpcError {
    fn from(e: base64::DecodeError) -> Self {
        MpcError::DecryptionError(e.to_string())
    }
}

impl From<hex::FromHexError> for MpcError {
    fn from(e: hex::FromHexError) -> Self {
        MpcError::SerializationError(e.to_string())
    }
}
