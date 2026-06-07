//! Encrypted Share Module - Secure encryption for MPC key shares

use aes_gcm::{aead::{Aead, KeyInit, OsRng}, Aes256Gcm, Nonce};
use chacha20poly1305::{ChaCha20Poly1305, Key as ChaChaKey};
use rand::RngCore;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

/// Encrypted key share with metadata
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EncryptedShare {
    pub ciphertext: Vec<u8>,
    pub nonce: [u8; 12],
    pub node_id: String,
    pub session_id: String,
    pub created_at: i64,
    pub version: u32,
}

impl EncryptedShare {
    /// Encrypt key share for specific node
    pub fn encrypt(share: &[u8], node_id: &str, session_id: &str) -> Result<Self, &'static str> {
        // Derive encryption key from node_id and session_id
        let key = Self::derive_key(node_id, session_id);
        
        let cipher = Aes256Gcm::new_from_slice(&key).map_err(|_| "Invalid key")?;
        
        let mut nonce_bytes = [0u8; 12];
        OsRng.fill_bytes(&mut nonce_bytes);
        let nonce = Nonce::from_slice(&nonce_bytes);
        
        let ciphertext = cipher
            .encrypt(nonce, share)
            .map_err(|_| "Encryption failed")?;
        
        Ok(Self {
            ciphertext,
            nonce: nonce_bytes,
            node_id: node_id.to_string(),
            session_id: session_id.to_string(),
            created_at: chrono::Utc::now().timestamp(),
            version: 1,
        })
    }
    
    /// Decrypt key share
    pub fn decrypt(&self) -> Result<Vec<u8>, &'static str> {
        let key = Self::derive_key(&self.node_id, &self.session_id);
        
        let cipher = Aes256Gcm::new_from_slice(&key).map_err(|_| "Invalid key")?;
        
        let nonce = Nonce::from_slice(&self.nonce);
        
        let plaintext = cipher
            .decrypt(nonce, self.ciphertext.as_ref())
            .map_err(|_| "Decryption failed")?;
        
        Ok(plaintext)
    }
    
    /// Derive encryption key from node_id and session_id
    fn derive_key(node_id: &str, session_id: &str) -> [u8; 32] {
        let mut hasher = Sha256::new();
        hasher.update(b"tigerswap_mpc_key_v1");
        hasher.update(session_id.as_bytes());
        hasher.update(node_id.as_bytes());
        hasher.finalize().into()
    }
    
    /// Re-encrypt for new node
    pub fn reencrypt(&self, new_node_id: &str) -> Result<Self, &'static str> {
        let plaintext = self.decrypt()?;
        self.encrypt(&plaintext, new_node_id, &self.session_id)
    }
    
    /// Verify integrity
    pub fn verify(&self) -> bool {
        self.decrypt().is_ok()
    }
}

/// Double-encrypted share (for extra security)
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DoubleEncryptedShare {
    pub outer: EncryptedShare,
    pub inner: EncryptedShare,
}

impl DoubleEncryptedShare {
    pub fn encrypt(share: &[u8], node_id: &str, session_id: &str) -> Result<Self, &'static str> {
        let inner = EncryptedShare::encrypt(share, node_id, session_id)?;
        let outer = EncryptedShare::encrypt(&inner.ciphertext, node_id, session_id)?;
        
        Ok(Self { outer, inner })
    }
    
    pub fn decrypt(&self) -> Result<Vec<u8>, &'static str> {
        let inner_ciphertext = self.outer.decrypt()?;
        let mut inner_share = EncryptedShare {
            ciphertext: inner_ciphertext,
            nonce: self.inner.nonce,
            node_id: self.inner.node_id.clone(),
            session_id: self.inner.session_id.clone(),
            created_at: self.inner.created_at,
            version: self.inner.version,
        };
        inner_share.decrypt()
    }
}

/// Share encryption key material
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ShareEncryptionKey {
    pub key_id: String,
    pub node_id: String,
    pub session_id: String,
    pub public_key: Vec<u8>,
    pub created_at: i64,
    pub expires_at: i64,
}

impl ShareEncryptionKey {
    pub fn new(node_id: String, session_id: String) -> Self {
        Self {
            key_id: uuid::Uuid::new_v4().to_string(),
            node_id,
            session_id,
            public_key: Vec::new(),
            created_at: chrono::Utc::now().timestamp(),
            expires_at: chrono::Utc::now().timestamp() + 86400, // 24 hours
        }
    }
    
    pub fn is_expired(&self) -> bool {
        chrono::Utc::now().timestamp() > self.expires_at
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_encrypted_share() {
        let share_data = b"test_key_share_data_32_bytes!".to_vec();
        let encrypted = EncryptedShare::encrypt(&share_data, "node1", "session1").unwrap();
        
        let decrypted = encrypted.decrypt().unwrap();
        assert_eq!(decrypted, share_data);
    }
    
    #[test]
    fn test_double_encryption() {
        let share_data = b"super_secret_key_share_data".to_vec();
        let encrypted = DoubleEncryptedShare::encrypt(&share_data, "node1", "session1").unwrap();
        
        let decrypted = encrypted.decrypt().unwrap();
        assert_eq!(decrypted, share_data);
    }
    
    #[test]
    fn test_reencryption() {
        let share_data = b"test_key_share_data".to_vec();
        let encrypted = EncryptedShare::encrypt(&share_data, "node1", "session1").unwrap();
        
        let reencrypted = encrypted.reencrypt("node2").unwrap();
        
        let decrypted = reencrypted.decrypt().unwrap();
        assert_eq!(decrypted, share_data);
    }
}