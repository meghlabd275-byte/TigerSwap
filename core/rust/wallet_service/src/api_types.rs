//! API Types Module

use serde::{Deserialize, Serialize};

/// Create wallet request
#[derive(Debug, Deserialize)]
pub struct CreateWalletRequest {
    pub name: Option<String>,
    pub chain: Option<String>,
}

/// Import wallet request
#[derive(Debug, Deserialize)]
pub struct ImportWalletRequest {
    pub name: Option<String>,
    pub mnemonic: String,
    pub chain: Option<String>,
}

/// Sign transaction request
#[derive(Debug, Deserialize)]
pub struct SignRequest {
    pub wallet_id: String,
    pub message: String,
}

/// Wallet response
#[derive(Debug, Serialize)]
pub struct WalletResponse {
    pub wallet_id: String,
    pub name: String,
    pub address: String,
    pub chain: String,
    pub mnemonic: Option<String>,
}

/// Sign response
#[derive(Debug, Serialize)]
pub struct SignResponse {
    pub signature: String,
    pub message: String,
}

/// Error response
#[derive(Debug, Serialize)]
pub struct ErrorResponse {
    pub error: String,
    pub code: i32,
}

impl ErrorResponse {
    pub fn new(error: String, code: i32) -> Self {
        Self { error, code }
    }
}

/// Health check response
#[derive(Debug, Serialize)]
pub struct HealthResponse {
    pub status: String,
    pub version: String,
}

impl HealthResponse {
    pub fn new() -> Self {
        Self {
            status: "ok".to_string(),
            version: "1.0.0".to_string(),
        }
    }
}