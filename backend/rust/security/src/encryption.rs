//! Encryption Module
//! 
//! Provides AES-GCM encryption for sensitive data

use aes_gcm::{
    aead::{Aead, KeyInit, OsRng},
    Aes256Gcm, Nonce,
};
use argon2::{Argon2, password_hash::{PasswordHasher, SaltString}};
use pbkdf2::pbkdf2_hmac;
use sha2::Sha256;
use rand::RngCore;
use thiserror::Error;
use zeroize::Zeroize;

#[derive(Error, Debug)]
pub enum EncryptionError {
    #[error("Encryption failed: {0}")]
    EncryptionFailed(String),
    #[error("Decryption failed: {0}")]
    DecryptionFailed(String),
    #[error("Invalid key")]
    InvalidKey,
    #[error("Invalid data")]
    InvalidData,
}

/// AES-256-GCM encryptor
pub struct Encryptor {
    key: [u8; 32],
}

impl Encryptor {
    /// Create a new encryptor with a 32-byte key
    pub fn new(key: &[u8; 32]) -> Self {
        Self { key: *key }
    }
    
    /// Create from password using PBKDF2
    pub fn from_password(password: &str, salt: &[u8]) -> Self {
        let mut key = [0u8; 32];
        pbkdf2_hmac::<Sha256>(password.as_bytes(), salt, 100_000, &mut key);
        Self { key }
    }
    
    /// Encrypt data using AES-256-GCM
    pub fn encrypt(&self, plaintext: &[u8]) -> Result<Vec<u8>, EncryptionError> {
        let cipher = Aes256Gcm::new_from_slice(&self.key)
            .map_err(|_| EncryptionError::InvalidKey)?;
        
        let mut nonce_bytes = [0u8; 12];
        OsRng.fill_bytes(&mut nonce_bytes);
        let nonce = Nonce::from_slice(&nonce_bytes);
        
        let ciphertext = cipher.encrypt(nonce, plaintext)
            .map_err(|e| EncryptionError::EncryptionFailed(e.to_string()))?;
        
        let mut result = Vec::with_capacity(12 + ciphertext.len());
        result.extend_from_slice(&nonce_bytes);
        result.extend(ciphertext);
        
        Ok(result)
    }
    
    /// Decrypt data using AES-256-GCM
    pub fn decrypt(&self, data: &[u8]) -> Result<Vec<u8>, EncryptionError> {
        if data.len() < 12 {
            return Err(EncryptionError::InvalidData);
        }
        
        let cipher = Aes256Gcm::new_from_slice(&self.key)
            .map_err(|_| EncryptionError::InvalidKey)?;
        
        let nonce = Nonce::from_slice(&data[..12]);
        let ciphertext = &data[12..];
        
        cipher.decrypt(nonce, ciphertext)
            .map_err(|e| EncryptionError::DecryptionFailed(e.to_string()))
    }
}

impl Drop for Encryptor {
    fn drop(&mut self) {
        self.key.zeroize();
    }
}

/// Hash password using Argon2
pub fn hash_password(password: &str) -> Result<String, EncryptionError> {
    let salt = SaltString::generate(&mut OsRng);
    let argon2 = Argon2::default();
    
    let hash = argon2.hash_password(password.as_bytes(), &salt)
        .map_err(|e| EncryptionError::EncryptionFailed(e.to_string()))?;
    
    Ok(hash.to_string())
}

/// Verify password against hash
pub fn verify_password(password: &str, hash: &str) -> Result<bool, EncryptionError> {
    let parsed_hash = argon2::PasswordHash::new(hash)
        .map_err(|_| EncryptionError::InvalidData)?;
    
    let argon2 = Argon2::default();
    
    Ok(argon2.verify_password(password.as_bytes(), &parsed_hash).is_ok())
}

/// Generate secure random bytes
pub fn generate_random_bytes(length: usize) -> Vec<u8> {
    let mut bytes = vec![0u8; length];
    OsRng.fill_bytes(&mut bytes);
    bytes
}

/// Generate a secure random hex string
pub fn generate_random_hex(length: usize) -> String {
    let bytes = generate_random_bytes(length / 2);
    hex::encode(bytes)
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_encrypt_decrypt() {
        let key = [0u8; 32];
        let encryptor = Encryptor::new(&key);
        
        let plaintext = b"Hello, TigerSwap!";
        let encrypted = encryptor.encrypt(plaintext).unwrap();
        let decrypted = encryptor.decrypt(&encrypted).unwrap();
        
        assert_eq!(plaintext.as_slice(), decrypted.as_slice());
    }
    
    #[test]
    fn test_password_hash() {
        let password = "secure_password_123";
        let hash = hash_password(password).unwrap();
        
        assert!(verify_password(password, &hash).unwrap());
        assert!(!verify_password("wrong_password", &hash).unwrap());
    }
}
