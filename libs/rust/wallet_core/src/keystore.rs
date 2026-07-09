//! Keystore module for secure wallet storage

use std::collections::HashMap;
use serde::{Deserialize, Serialize};
use zeroize::Zeroize;
use aes_gcm::{
    aead::{Aead, KeyInit, OsRng},
    Aes256Gcm, Nonce,
};
use argon2::{
    password_hash::{PasswordHasher, SaltString},
    Argon2,
};
use rand::RngCore;

use crate::crypto::{DerivedKey, Mnemonic};

/// Encrypted keystore entry
pub struct KeystoreEntry {
    pub id: String,
    pub name: String,
    pub encrypted_data: Vec<u8>,
    pub salt: Vec<u8>,
    pub nonce: Vec<u8>,
    pub created_at: i64,
}

/// Keystore manager
pub struct Keystore {
    entries: HashMap<String, KeystoreEntry>,
    master_key: Option<[u8; 32]>,
}

impl Keystore {
    pub fn new() -> Self {
        Self {
            entries: HashMap::new(),
            master_key: None,
        }
    }
    
    /// Unlock keystore with password
    pub fn unlock(&mut self, password: &str, salt: &[u8]) -> Result<(), &'static str> {
        // Derive key from password using Argon2
        let argon2 = Argon2::default();
        let salt_str = base64::encode(salt);
        
        let password_hash = argon2.hash_password(password.as_bytes(), &SaltString::from_b64(&salt_str).unwrap())
            .map_err(|_| "Failed to hash password")?;
        
        let hash_bytes = password_hash.hash.unwrap();
        let mut key = [0u8; 32];
        key.copy_from_slice(&hash_bytes.as_bytes()[..32]);
        
        self.master_key = Some(key);
        Ok(())
    }
    
    /// Lock keystore
    pub fn lock(&mut self) {
        if let Some(ref mut key) = self.master_key {
            key.zeroize();
        }
        self.master_key = None;
    }
    
    /// Check if unlocked
    pub fn is_unlocked(&self) -> bool {
        self.master_key.is_some()
    }
    
    /// Add account to keystore
    pub async fn add_account(&mut self, id: String, account: crate::types::WalletAccount) -> Result<(), &'static str> {
        if self.master_key.is_none() {
            return Err("Keystore is locked");
        }
        
        let key = self.master_key.unwrap();
        
        // Serialize account data
        let data = serde_json::to_vec(&account).unwrap();
        
        // Encrypt data
        let (encrypted, nonce) = Self::encrypt(&data, &key)?;
        
        let entry = KeystoreEntry {
            id: id.clone(),
            name: account.name.clone(),
            encrypted_data: encrypted,
            salt: vec![0u8; 16], // Would use proper salt
            nonce,
            created_at: chrono::Utc::now().timestamp(),
        };
        
        self.entries.insert(id, entry);
        Ok(())
    }
    
    /// Encrypt data
    fn encrypt(data: &[u8], key: &[u8; 32]) -> Result<(Vec<u8>, Vec<u8>), &'static str> {
        let cipher = Aes256Gcm::new_from_slice(key)
            .map_err(|_| "Invalid key")?;
        
        let mut nonce_bytes = [0u8; 12];
        OsRng.fill_bytes(&mut nonce_bytes);
        let nonce = Nonce::from_slice(&nonce_bytes);
        
        let ciphertext = cipher.encrypt(nonce, data)
            .map_err(|_| "Encryption failed")?;
        
        Ok((ciphertext, nonce_bytes.to_vec()))
    }
    
    /// Decrypt data
    fn decrypt(ciphertext: &[u8], key: &[u8; 32], nonce: &[u8]) -> Result<Vec<u8>, &'static str> {
        let cipher = Aes256Gcm::new_from_slice(key)
            .map_err(|_| "Invalid key")?;
        
        let nonce = Nonce::from_slice(nonce);
        
        let plaintext = cipher.decrypt(nonce, ciphertext)
            .map_err(|_| "Decryption failed")?;
        
        Ok(plaintext)
    }
}

impl Default for Keystore {
    fn default() -> Self {
        Self::new()
    }
}
