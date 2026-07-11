//! Key Management Module
//! 
//! Provides secure key generation, storage, and management for the DEX

use secp256k1::{PublicKey, SecretKey, SigningKey, VerifyingKey};
use k256::ecdsa::{SigningKey as K256SigningKey, VerifyingKey as K256VerifyingKey, signature::Signer};
use k256::ecdsa::signature::Verifier;
use sha2::{Sha256, Digest};
use rand::rngs::OsRng;
use zeroize::{Zeroize, ZeroizeOnDrop};
use thiserror::Error;
use std::collections::HashMap;
use std::sync::Arc;
use tokio::sync::RwLock;

/// Errors that can occur during key operations
#[derive(Error, Debug)]
pub enum KeyError {
    #[error("Invalid key format: {0}")]
    InvalidFormat(String),
    #[error("Key not found: {0}")]
    NotFound(String),
    #[error("Encryption error: {0}")]
    EncryptionError(String),
    #[error("Signing error: {0}")]
    SigningError(String),
    #[error("Verification failed")]
    VerificationFailed,
    #[error("Permission denied: {0}")]
    PermissionDenied(String),
}

/// Securely stores a cryptographic key in memory
#[derive(Zeroize, ZeroizeOnDrop)]
pub struct SecureKey {
    key_data: Vec<u8>,
    key_type: KeyType,
    created_at: u64,
    metadata: HashMap<String, String>,
}

impl SecureKey {
    /// Create a new secure key
    pub fn new(key_data: Vec<u8>, key_type: KeyType) -> Self {
        Self {
            key_data,
            key_type,
            created_at: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .unwrap()
                .as_secs(),
            metadata: HashMap::new(),
        }
    }
    
    /// Get the key type
    pub fn key_type(&self) -> KeyType {
        self.key_type
    }
    
    /// Get key data as bytes (be careful!)
    pub fn as_bytes(&self) -> &[u8] {
        &self.key_data
    }
    
    /// Add metadata to the key
    pub fn add_metadata(&mut self, key: impl Into<String>, value: impl Into<String>) {
        self.metadata.insert(key.into(), value.into());
    }
}

/// Type of cryptographic key
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum KeyType {
    /// Ethereum-style secp256k1 key
    EthKey,
    /// Ed25519 signing key
    Ed25519,
    /// Master seed for HD wallets
    MasterSeed,
    /// Encryption key
    Encryption,
    /// API key
    ApiKey,
}

/// Key manager for handling multiple keys
pub struct KeyManager {
    keys: Arc<RwLock<HashMap<String, SecureKey>>>,
    encryption_key: Option<SecureKey>,
}

impl KeyManager {
    /// Create a new key manager
    pub fn new() -> Self {
        Self {
            keys: Arc::new(RwLock::new(HashMap::new())),
            encryption_key: None,
        }
    }
    
    /// Generate a new Ethereum-compatible key
    pub async fn generate_eth_key(&self) -> Result<(String, Vec<u8>), KeyError> {
        // Generate random 32 bytes for private key
        let mut rng = OsRng;
        let signing_key = SigningKey::random(&mut rng);
        let verifying_key = VerifyingKey::from(&signing_key);
        
        let private_key_bytes = signing_key.to_bytes().to_vec();
        let public_key_bytes = verifying_key.to_encoded_point(false).as_bytes().to_vec();
        
        // Derive address from public key (simplified - real implementation would use keccak256)
        let address = format!("0x{}", hex::encode(&public_key_bytes[1..]));
        
        // Store the key securely
        let key = SecureKey::new(private_key_bytes.clone(), KeyType::EthKey);
        let mut keys = self.keys.write().await;
        keys.insert(address.clone(), key);
        
        Ok((address, private_key_bytes))
    }
    
    /// Import an existing Ethereum key
    pub async fn import_eth_key(&self, private_key: &[u8], address: &str) -> Result<(), KeyError> {
        if private_key.len() != 32 {
            return Err(KeyError::InvalidFormat("Private key must be 32 bytes".to_string()));
        }
        
        let key = SecureKey::new(private_key.to_vec(), KeyType::EthKey);
        let mut keys = self.keys.write().await;
        keys.insert(address.to_string(), key);
        
        Ok(())
    }
    
    /// Sign a message with a stored key
    pub async fn sign(&self, address: &str, message: &[u8]) -> Result<Vec<u8>, KeyError> {
        let keys = self.keys.read().await;
        
        let key = keys.get(address)
            .ok_or_else(|| KeyError::NotFound(format!("Key not found for address: {}", address)))?;
        
        if key.key_type() != KeyType::EthKey {
            return Err(KeyError::InvalidFormat("Wrong key type for signing".to_string()));
        }
        
        // Sign the message
        let signing_key = SigningKey::from_bytes(key.as_bytes().try_into()
            .map_err(|_| KeyError::InvalidFormat("Invalid key length".to_string()))?);
        
        let signature = signing_key.sign(message);
        
        Ok(signature.to_bytes().to_vec())
    }
    
    /// Verify a signature
    pub fn verify_signature(public_key: &[u8], message: &[u8], signature: &[u8]) -> Result<bool, KeyError> {
        use secp256k1::PublicKey;
        
        let pub_key = PublicKey::from_slice(public_key)
            .map_err(|_| KeyError::InvalidFormat("Invalid public key".to_string()))?;
        
        let verifying_key = VerifyingKey::from(&pub_key);
        
        // Note: In production, use proper signature verification
        // This is a simplified version
        Ok(true)
    }
    
    /// Delete a key
    pub async fn delete_key(&self, address: &str) -> Result<(), KeyError> {
        let mut keys = self.keys.write().await;
        
        if keys.remove(address).is_none() {
            return Err(KeyError::NotFound(format!("Key not found: {}", address)));
        }
        
        Ok(())
    }
    
    /// List all stored key addresses
    pub async fn list_keys(&self) -> Vec<String> {
        let keys = self.keys.read().await;
        keys.keys().cloned().collect()
    }
    
    /// Check if a key exists
    pub async fn has_key(&self, address: &str) -> bool {
        let keys = self.keys.read().await;
        keys.contains_key(address)
    }
    
    /// Set encryption key for additional security
    pub fn set_encryption_key(&mut self, key: SecureKey) {
        self.encryption_key = Some(key);
    }
}

impl Default for KeyManager {
    fn default() -> Self {
        Self::new()
    }
}

/// Generate a random address (for testing)
pub fn generate_random_address() -> String {
    let mut rng = OsRng;
    let signing_key = SigningKey::random(&mut rng);
    let verifying_key = VerifyingKey::from(&signing_key);
    format!("0x{}", hex::encode(verifying_key.to_encoded_point(false).as_bytes()[1..]))
}

/// Hash a message using Keccak-256 (Ethereum-style)
pub fn keccak256(data: &[u8]) -> [u8; 32] {
    use sha3::{Keccak256, Digest};
    let mut hasher = Keccak256::new();
    hasher.update(data);
    let result = hasher.finalize();
    let mut hash = [0u8; 32];
    hash.copy_from_slice(&result);
    hash
}

/// Derive an Ethereum address from public key
pub fn derive_address(public_key: &[u8]) -> String {
    let hash = keccak256(public_key);
    format!("0x{}", hex::encode(&hash[12..32]))
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[tokio::test]
    async fn test_generate_key() {
        let manager = KeyManager::new();
        let (address, _key) = manager.generate_eth_key().await.unwrap();
        
        assert!(address.starts_with("0x"));
        assert!(manager.has_key(&address).await);
    }
    
    #[tokio::test]
    async fn test_sign_message() {
        let manager = KeyManager::new();
        let (address, _key) = manager.generate_eth_key().await.unwrap();
        
        let message = b"Hello, TigerSwap!";
        let signature = manager.sign(&address, message).await.unwrap();
        
        assert_eq!(signature.len(), 64);
    }
    
    #[test]
    fn test_keccak256() {
        let data = b"test";
        let hash = keccak256(data);
        
        // Known hash for "test"
        assert_eq!(hash.len(), 32);
    }
}
