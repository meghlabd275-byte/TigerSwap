//! Key Share Module - Secure key share storage with zeroization

use aes_gcm::{aead::{Aead, KeyInit, OsRng}, Aes256Gcm, Nonce};
use chacha20poly1305::{ChaCha20Poly1305, Key as ChaChaKey};
use rand::RngCore;
use sha2::{Digest, Sha256};
use zeroize::{Zeroize, ZeroizeOnDrop};

/// 256-bit key share with secure memory handling
#[derive(Clone, Zeroize, ZeroizeOnDrop)]
pub struct KeyShare([u8; 32]);

impl KeyShare {
    /// Create new key share from cryptographically secure random bytes
    pub fn new() -> Self {
        let mut key = [0u8; 32];
        OsRng.fill_bytes(&mut key);
        Self(key)
    }
    
    /// Create key share from existing bytes
    pub fn from_bytes(bytes: [u8; 32]) -> Self {
        Self(bytes)
    }
    
    /// Import key share from hex string
    pub fn from_hex(hex: &str) -> Result<Self, &'static str> {
        let bytes = hex::decode(hex).map_err(|_| "Invalid hex")?;
        if bytes.len() != 32 {
            return Err("Invalid key length");
        }
        let mut key = [0u8; 32];
        key.copy_from_slice(&bytes);
        Ok(Self(key))
    }
    
    /// Get raw key bytes
    pub fn as_bytes(&self) -> &[u8; 32] {
        &self.0
    }
    
    /// Get key as hex string
    pub fn to_hex(&self) -> String {
        hex::encode(self.0)
    }
    
    /// Derive encryption key from key share
    pub fn derive_key(&self, context: &[u8]) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(&self.0);
        hasher.update(context);
        hasher.finalize().into()
    }
    
    /// Encrypt data using this key share
    pub fn encrypt(&self, plaintext: &[u8]) -> Vec<u8> {
        let key = self.derive_key(b"tigerswap_encrypt");
        let cipher = Aes256Gcm::new_from_slice(&key).unwrap();
        
        let mut nonce_bytes = [0u8; 12];
        OsRng.fill_bytes(&mut nonce_bytes);
        let nonce = Nonce::from_slice(&nonce_bytes);
        
        let mut ciphertext = cipher.encrypt(nonce, plaintext).unwrap();
        
        // Prepend nonce
        let mut result = nonce_bytes.to_vec();
        result.append(&mut ciphertext);
        result
    }
    
    /// Decrypt data using this key share
    pub fn decrypt(&self, ciphertext: &[u8]) -> Result<Vec<u8>, &'static str> {
        if ciphertext.len() < 12 {
            return Err("Ciphertext too short");
        }
        
        let key = self.derive_key(b"tigerswap_encrypt");
        let cipher = Aes256Gcm::new_from_slice(&key).map_err(|_| "Invalid key")?;
        
        let nonce = Nonce::from_slice(&ciphertext[..12]);
        let ciphertext = &ciphertext[12..];
        
        cipher.decrypt(nonce, ciphertext).map_err(|_| "Decryption failed")
    }
}

impl Default for KeyShare {
    fn default() -> Self {
        Self::new()
    }
}

impl core::fmt::Debug for KeyShare {
    fn fmt(&self, f: &mut core::fmt::Formatter<'_>) -> core::fmt::Result {
        f.debug_struct("KeyShare")
            .field("len", &32)
            .finish()
    }
}

impl PartialEq for KeyShare {
    fn eq(&self, other: &Self) -> bool {
        self.0 == other.0
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_key_share_generation() {
        let share1 = KeyShare::new();
        let share2 = KeyShare::new();
        
        // Different shares each time
        assert_ne!(share1.as_bytes(), share2.as_bytes());
    }
    
    #[test]
    fn test_hex_conversion() {
        let share = KeyShare::new();
        let hex = share.to_hex();
        let recovered = KeyShare::from_hex(&hex).unwrap();
        
        assert_eq!(share.as_bytes(), recovered.as_bytes());
    }
    
    #[test]
    fn test_encryption() {
        let share = KeyShare::new();
        let data = b"Hello, TigerSwap!";
        
        let encrypted = share.encrypt(data);
        let decrypted = share.decrypt(&encrypted).unwrap();
        
        assert_eq!(decrypted, data);
    }
    
    #[test]
    fn test_key_derivation() {
        let share = KeyShare::new();
        let key1 = share.derive_key(b"context1");
        let key2 = share.derive_key(b"context2");
        
        // Different context = different key
        assert_ne!(key1, key2);
    }
}