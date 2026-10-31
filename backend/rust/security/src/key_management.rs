//! Key Management Module
//!
//! Secure secp256k1 key generation, storage, signing and verification for the
//! TigerSwap DEX. Private key material is held in a zeroizing buffer and never
//! leaves the manager except through explicit accessor calls.

use k256::ecdsa::{Signature, SigningKey, VerifyingKey};
use rand::rngs::OsRng;
use sha3::{Digest, Keccak256};
use std::collections::HashMap;
use std::sync::Arc;
use thiserror::Error;
use tokio::sync::RwLock;
use zeroize::Zeroize;

use crate::encryption::{EncryptionError, Encryptor};

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

impl From<EncryptionError> for KeyError {
    fn from(e: EncryptionError) -> Self {
        KeyError::EncryptionError(e.to_string())
    }
}

/// Type of cryptographic key
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum KeyType {
    /// Ethereum-style secp256k1 key
    EthKey,
    /// Master seed for HD wallets
    MasterSeed,
    /// Encryption key
    Encryption,
    /// API key
    ApiKey,
}

/// Securely stores a cryptographic key in memory.
///
/// `key_data` is zeroized on drop. Only byte buffers that can hold secrets are
/// zeroized; the enum and map carry no secret material.
pub struct SecureKey {
    key_data: Vec<u8>,
    key_type: KeyType,
    created_at: u64,
    metadata: HashMap<String, String>,
}

impl Drop for SecureKey {
    fn drop(&mut self) {
        self.key_data.zeroize();
        self.metadata.clear();
    }
}

impl SecureKey {
    /// Create a new secure key
    pub fn new(key_data: Vec<u8>, key_type: KeyType) -> Self {
        Self {
            key_data,
            key_type,
            created_at: std::time::SystemTime::now()
                .duration_since(std::time::UNIX_EPOCH)
                .map(|d| d.as_secs())
                .unwrap_or(0),
            metadata: HashMap::new(),
        }
    }

    /// Get the key type
    pub fn key_type(&self) -> KeyType {
        self.key_type
    }

    /// Creation timestamp (unix seconds)
    pub fn created_at(&self) -> u64 {
        self.created_at
    }

    /// Get key data as bytes. Handle with care: the returned slice is secret.
    pub fn as_bytes(&self) -> &[u8] {
        &self.key_data
    }

    /// Add metadata to the key
    pub fn add_metadata(&mut self, key: impl Into<String>, value: impl Into<String>) {
        self.metadata.insert(key.into(), value.into());
    }

    /// Read a metadata value
    pub fn metadata(&self, key: &str) -> Option<&String> {
        self.metadata.get(key)
    }

    /// Encrypts `key_data` in place with AES-256-GCM under `encryptor`.
    ///
    /// The stored buffer is replaced by `nonce || ciphertext || tag`, so a
    /// memory dump taken after this call does not reveal the private key. The
    /// original plaintext buffer is zeroized before being dropped.
    pub fn encrypt_at_rest(&mut self, encryptor: &Encryptor) -> Result<(), EncryptionError> {
        let sealed = encryptor.encrypt(&self.key_data)?;
        self.key_data.zeroize();
        self.key_data = sealed;
        Ok(())
    }

    /// Reverses [`SecureKey::encrypt_at_rest`], replacing the stored ciphertext
    /// with the recovered plaintext key material.
    pub fn decrypt_at_rest(&mut self, encryptor: &Encryptor) -> Result<(), EncryptionError> {
        let plaintext = encryptor.decrypt(&self.key_data)?;
        self.key_data.zeroize();
        self.key_data = plaintext;
        Ok(())
    }
}

/// Key manager for handling multiple keys
pub struct KeyManager {
    keys: Arc<RwLock<HashMap<String, SecureKey>>>,
    /// AES-256-GCM encryptor used to seal private keys at rest. `None` keeps
    /// keys in plaintext memory (the historical behaviour); setting an
    /// encryption key makes the protection real rather than declarative.
    encryptor: Option<Encryptor>,
}

impl KeyManager {
    /// Create a new key manager
    pub fn new() -> Self {
        Self {
            keys: Arc::new(RwLock::new(HashMap::new())),
            encryptor: None,
        }
    }

    /// Generate a new Ethereum-compatible key.
    ///
    /// Returns the checksummed-lowercase address, the 32-byte private key and
    /// the uncompressed public key.
    pub async fn generate_eth_key(&self) -> Result<(String, Vec<u8>), KeyError> {
        let signing_key = SigningKey::random(&mut OsRng);
        self.store_signing_key(signing_key).await
    }

    async fn store_signing_key(&self, signing_key: SigningKey) -> Result<(String, Vec<u8>), KeyError> {
        let verifying_key = *signing_key.verifying_key();
        let private_key_bytes = signing_key.to_bytes().to_vec();
        let public_key_bytes = verifying_key.to_encoded_point(false).as_bytes().to_vec();
        let address = derive_address(&public_key_bytes);

        let mut key = SecureKey::new(private_key_bytes.clone(), KeyType::EthKey);
        // Encrypt the private key before it enters the shared map when an
        // encryption key is configured, so the key is never held as plaintext.
        if let Some(enc) = &self.encryptor {
            key.encrypt_at_rest(enc)?;
        }
        self.keys.write().await.insert(address.clone(), key);

        Ok((address, private_key_bytes))
    }

    /// Import an existing Ethereum key. The address is derived from the key
    /// itself so callers cannot register a mismatched address.
    pub async fn import_eth_key(&self, private_key: &[u8]) -> Result<String, KeyError> {
        if private_key.len() != 32 {
            return Err(KeyError::InvalidFormat(
                "Private key must be 32 bytes".to_string(),
            ));
        }

        let signing_key = SigningKey::from_slice(private_key)
            .map_err(|e| KeyError::InvalidFormat(e.to_string()))?;

        let (address, _private_key) = self.store_signing_key(signing_key).await?;
        Ok(address)
    }

    /// Sign a 32-byte digest with a stored key.
    ///
    /// Returns a 65-byte Ethereum-style signature (r || s || v) where `v` is the
    /// recovery parity offset by 27. The digest is treated as a prehash and is
    /// not hashed again, so the result verifies against both
    /// [`KeyManager::verify_signature`] and
    /// [`crate::signature::verify_signature`].
    pub async fn sign(&self, address: &str, digest: &[u8]) -> Result<Vec<u8>, KeyError> {
        if digest.len() != 32 {
            return Err(KeyError::InvalidFormat(format!(
                "Digest must be 32 bytes, got {}",
                digest.len()
            )));
        }

        let keys = self.keys.read().await;
        let key = keys
            .get(address)
            .ok_or_else(|| KeyError::NotFound(format!("Key not found for address: {address}")))?;

        if key.key_type() != KeyType::EthKey {
            return Err(KeyError::InvalidFormat(
                "Wrong key type for signing".to_string(),
            ));
        }

        // When the key is held encrypted at rest, decrypt a local copy for the
        // signing operation only. The copy zeroizes itself when dropped.
        let signing_key = if let Some(enc) = &self.encryptor {
            let mut plain = SecureKey::new(key.as_bytes().to_vec(), key.key_type());
            plain.decrypt_at_rest(enc)?;
            let sk = SigningKey::from_slice(plain.as_bytes())
                .map_err(|e| KeyError::InvalidFormat(e.to_string()))?;
            drop(plain);
            sk
        } else {
            SigningKey::from_slice(key.as_bytes())
                .map_err(|e| KeyError::InvalidFormat(e.to_string()))?
        };

        let (signature, recovery_id) = signing_key
            .sign_prehash_recoverable(digest)
            .map_err(|e| KeyError::SigningError(e.to_string()))?;

        let mut raw = signature.to_bytes().to_vec();
        raw.push(recovery_id.to_byte() + 27);
        Ok(raw)
    }

    /// Verify an ECDSA signature against a public key and a 32-byte digest.
    ///
    /// Accepts both the 65-byte Ethereum form (r || s || v) produced by
    /// [`KeyManager::sign`] and a bare 64-byte (r || s) signature. The digest is
    /// a prehash and is not hashed again.
    pub fn verify_signature(
        public_key: &[u8],
        digest: &[u8],
        signature: &[u8],
    ) -> Result<bool, KeyError> {
        if digest.len() != 32 {
            return Err(KeyError::InvalidFormat(format!(
                "Digest must be 32 bytes, got {}",
                digest.len()
            )));
        }

        let verifying_key = VerifyingKey::from_sec1_bytes(public_key)
            .map_err(|_| KeyError::InvalidFormat("Invalid public key".to_string()))?;

        let compact = match signature.len() {
            65 => &signature[..64],
            64 => signature,
            other => {
                return Err(KeyError::InvalidFormat(format!(
                    "Signature must be 64 or 65 bytes, got {other}"
                )))
            }
        };

        let signature = Signature::from_slice(compact)
            .map_err(|_| KeyError::InvalidFormat("Invalid signature".to_string()))?;

        use k256::ecdsa::signature::hazmat::PrehashVerifier;
        verifying_key
            .verify_prehash(digest, &signature)
            .map_err(|_| KeyError::VerificationFailed)?;

        Ok(true)
    }

    /// Delete a key
    pub async fn delete_key(&self, address: &str) -> Result<(), KeyError> {
        let mut keys = self.keys.write().await;
        if keys.remove(address).is_none() {
            return Err(KeyError::NotFound(format!("Key not found: {address}")));
        }
        Ok(())
    }

    /// List all stored key addresses
    pub async fn list_keys(&self) -> Vec<String> {
        self.keys.read().await.keys().cloned().collect()
    }

    /// Check if a key exists
    pub async fn has_key(&self, address: &str) -> bool {
        self.keys.read().await.contains_key(address)
    }

    /// Enables encryption of private keys at rest with AES-256-GCM.
    ///
    /// The supplied key must be exactly 32 bytes. Keys already held by the
    /// manager are re-encrypted immediately, so the protection applies to the
    /// whole store and not only to keys added afterwards.
    pub async fn set_encryption_key(&mut self, key: &[u8; 32]) -> Result<(), KeyError> {
        let encryptor = Encryptor::new(key);
        let mut keys = self.keys.write().await;
        for secure_key in keys.values_mut() {
            secure_key.encrypt_at_rest(&encryptor)?;
        }
        self.encryptor = Some(encryptor);
        Ok(())
    }

    /// Removes at-rest encryption, returning all keys to plaintext memory.
    pub async fn clear_encryption_key(&mut self) -> Result<(), KeyError> {
        if let Some(enc) = &self.encryptor {
            let mut keys = self.keys.write().await;
            for secure_key in keys.values_mut() {
                secure_key.decrypt_at_rest(enc)?;
            }
        }
        self.encryptor = None;
        Ok(())
    }

    /// Whether private keys are encrypted at rest.
    pub fn has_encryption_at_rest(&self) -> bool {
        self.encryptor.is_some()
    }

    /// Number of stored keys
    pub async fn len(&self) -> usize {
        self.keys.read().await.len()
    }

    /// Whether the manager holds no keys
    pub async fn is_empty(&self) -> bool {
        self.keys.read().await.is_empty()
    }
}

impl Default for KeyManager {
    fn default() -> Self {
        Self::new()
    }
}

/// Hash data using Keccak-256 (Ethereum-style)
pub fn keccak256(data: &[u8]) -> [u8; 32] {
    let mut hasher = Keccak256::new();
    hasher.update(data);
    hasher.finalize().into()
}

/// Derive an Ethereum address from an uncompressed secp256k1 public key.
pub fn derive_address(public_key: &[u8]) -> String {
    // Accept both compressed (33-byte) and uncompressed (65-byte) keys by
    // normalising through the curve point.
    let point = k256::EncodedPoint::from_bytes(public_key)
        .ok()
        .and_then(|p| VerifyingKey::from_encoded_point(&p).ok())
        .map(|vk| vk.to_encoded_point(false));

    let uncompressed = match point {
        Some(p) => p.as_bytes().to_vec(),
        None => public_key.to_vec(),
    };

    let body = if uncompressed.len() == 65 && uncompressed[0] == 0x04 {
        &uncompressed[1..]
    } else {
        &uncompressed[..]
    };

    let hash = keccak256(body);
    format!("0x{}", hex::encode(&hash[12..]))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[tokio::test]
    async fn test_generate_key_derives_real_address() {
        let manager = KeyManager::new();
        let (address, private_key) = manager.generate_eth_key().await.unwrap();

        assert!(address.starts_with("0x"));
        assert_eq!(address.len(), 42);
        assert_eq!(private_key.len(), 32);
        assert!(manager.has_key(&address).await);
    }

    #[tokio::test]
    async fn test_import_derives_matching_address() {
        let manager = KeyManager::new();
        let (address, private_key) = manager.generate_eth_key().await.unwrap();
        let imported = manager.import_eth_key(&private_key).await.unwrap();
        assert_eq!(address, imported);
    }

    #[tokio::test]
    async fn test_import_rejects_wrong_length() {
        let manager = KeyManager::new();
        assert!(manager.import_eth_key(&[0u8; 31]).await.is_err());
    }

    #[tokio::test]
    async fn test_sign_and_verify_roundtrip() {
        let manager = KeyManager::new();
        let (address, private_key) = manager.generate_eth_key().await.unwrap();

        let digest = keccak256(b"Hello, TigerSwap!");
        let signature = manager.sign(&address, &digest).await.unwrap();
        assert_eq!(signature.len(), 65, "r || s || v");

        let signing_key = SigningKey::from_slice(&private_key).unwrap();
        let public_key = signing_key.verifying_key().to_encoded_point(false);

        assert!(
            KeyManager::verify_signature(public_key.as_bytes(), &digest, &signature).unwrap()
        );
        // The sibling `signature` module must accept the same signature.
        assert!(
            crate::signature::verify_signature(public_key.as_bytes(), &digest, &signature).unwrap()
        );

        let tampered = keccak256(b"tampered");
        assert!(KeyManager::verify_signature(public_key.as_bytes(), &tampered, &signature).is_err());
    }

    #[test]
    fn test_keccak256_known_vector() {
        // keccak256("") is a well-known constant.
        let hash = keccak256(b"");
        assert_eq!(
            hex::encode(hash),
            "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"
        );
    }

    #[test]
    fn test_derive_address_matches_known_key() {
        // secp256k1 key = 1 => address 0x7e5f4552091a69125d5dfcb7b8c2659029395bdf
        let signing_key = SigningKey::from_slice(&{
            let mut k = [0u8; 32];
            k[31] = 1;
            k
        })
        .unwrap();
        let public_key = signing_key.verifying_key().to_encoded_point(false);
        assert_eq!(
            derive_address(public_key.as_bytes()),
            "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf"
        );
    }

    #[tokio::test]
    async fn test_encryption_at_rest_hides_private_key_and_still_signs() {
        let mut manager = KeyManager::new();
        let (address, private_key) = manager.generate_eth_key().await.unwrap();

        let mut enc_key = [0u8; 32];
        for (i, b) in enc_key.iter_mut().enumerate() {
            *b = i as u8;
        }
        manager.set_encryption_key(&enc_key).await.unwrap();
        assert!(manager.has_encryption_at_rest());

        // The stored buffer must no longer equal the plaintext private key.
        let stored = {
            let keys = manager.keys.read().await;
            keys.get(&address).unwrap().as_bytes().to_vec()
        };
        assert_ne!(stored, private_key, "private key must not be stored in the clear");
        assert!(stored.len() > private_key.len(), "nonce and tag are prepended/appended");

        // Signing must transparently decrypt and still produce a valid signature.
        let digest = keccak256(b"encrypted at rest");
        let signature = manager.sign(&address, &digest).await.unwrap();
        let signing_key = SigningKey::from_slice(&private_key).unwrap();
        let public_key = signing_key.verifying_key().to_encoded_point(false);
        assert!(KeyManager::verify_signature(public_key.as_bytes(), &digest, &signature).unwrap());
    }

    #[tokio::test]
    async fn test_set_encryption_key_reencrypts_existing_keys() {
        let mut manager = KeyManager::new();
        let (address, private_key) = manager.generate_eth_key().await.unwrap();
        // Key exists in plaintext before encryption is enabled.
        {
            let keys = manager.keys.read().await;
            assert_eq!(keys.get(&address).unwrap().as_bytes(), &private_key[..]);
        }

        let mut enc_key = [7u8; 32];
        enc_key[0] = 1;
        manager.set_encryption_key(&enc_key).await.unwrap();

        {
            let keys = manager.keys.read().await;
            assert_ne!(keys.get(&address).unwrap().as_bytes(), &private_key[..]);
        }

        // Clearing restores plaintext and signing keeps working.
        manager.clear_encryption_key().await.unwrap();
        assert!(!manager.has_encryption_at_rest());
        {
            let keys = manager.keys.read().await;
            assert_eq!(keys.get(&address).unwrap().as_bytes(), &private_key[..]);
        }
        let digest = keccak256(b"after clear");
        assert!(manager.sign(&address, &digest).await.is_ok());
    }
}
