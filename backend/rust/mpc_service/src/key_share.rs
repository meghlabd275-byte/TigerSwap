//! Key Share Management
//! 
//! This module handles encryption, storage, and retrieval of key shares.

use crate::error::MpcError;
use crate::types::KeyShare;
use aes_gcm::{
    aead::{Aead, KeyInit, OsRng},
    Aes256Gcm, Nonce,
};
use base64::{engine::general_purpose::STANDARD as BASE64, Engine};
use rand::RngCore;
use sha2::{Digest, Sha256};

const NONCE_SIZE: usize = 12;

/// Encrypt a key share with a password-derived key
pub fn encrypt_share(
    share: &KeyShare,
    password: &str,
) -> Result<Vec<u8>, MpcError> {
    // Derive key from password
    let key = derive_key(password);
    
    // Serialize share
    let plaintext = serde_json::to_vec(share)
        .map_err(|e| MpcError::EncryptionError(e.to_string()))?;
    
    // Generate random nonce
    let mut nonce_bytes = [0u8; NONCE_SIZE];
    OsRng.fill_bytes(&mut nonce_bytes);
    let nonce = Nonce::from_slice(&nonce_bytes);
    
    // Encrypt
    let cipher = Aes256Gcm::new_from_slice(&key)
        .map_err(|e| MpcError::EncryptionError(e.to_string()))?;
    
    let ciphertext = cipher
        .encrypt(nonce, plaintext.as_ref())
        .map_err(|e| MpcError::EncryptionError(e.to_string()))?;
    
    // Combine nonce + ciphertext
    let mut result = nonce_bytes.to_vec();
    result.extend(ciphertext);
    
    Ok(result)
}

/// Decrypt a key share
pub fn decrypt_share(
    encrypted_data: &[u8],
    password: &str,
) -> Result<KeyShare, MpcError> {
    if encrypted_data.len() < NONCE_SIZE {
        return Err(MpcError::DecryptionError("Data too short".to_string()));
    }
    
    // Extract nonce and ciphertext
    let nonce = Nonce::from_slice(&encrypted_data[..NONCE_SIZE]);
    let ciphertext = &encrypted_data[NONCE_SIZE..];
    
    // Derive key from password
    let key = derive_key(password);
    
    // Decrypt
    let cipher = Aes256Gcm::new_from_slice(&key)
        .map_err(|e| MpcError::DecryptionError(e.to_string()))?;
    
    let plaintext = cipher
        .decrypt(nonce, ciphertext)
        .map_err(|e| MpcError::DecryptionError(e.to_string()))?;
    
    // Deserialize share
    let share: KeyShare = serde_json::from_slice(&plaintext)
        .map_err(|e| MpcError::DecryptionError(e.to_string()))?;
    
    Ok(share)
}

/// Derive a 256-bit key from a password using SHA-256
fn derive_key(password: &str) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(password.as_bytes());
    let result = hasher.finalize();
    
    let mut key = [0u8; 32];
    key.copy_from_slice(&result);
    key
}

/// Create an encrypted backup of a key share
pub fn create_backup(
    share: &KeyShare,
    wallet_address: &str,
) -> Result<Vec<u8>, MpcError> {
    // Use wallet address + timestamp as password for backup
    let password = format!("{}_{}", wallet_address, share.created_at);
    encrypt_share(share, &password)
}

/// Verify that a share is valid
pub fn verify_share(share: &KeyShare) -> Result<bool, MpcError> {
    // Check share has required fields
    if share.share_id.is_empty() {
        return Ok(false);
    }
    
    if share.share_value.is_empty() {
        return Ok(false);
    }
    
    // Check share index is valid (1-based)
    if share.share_index == 0 {
        return Ok(false);
    }
    
    Ok(true)
}

/// Rotate a key share
pub fn rotate_share(
    old_share: &KeyShare,
    new_share_value: Vec<u8>,
) -> Result<KeyShare, MpcError> {
    let new_share = KeyShare {
        share_id: format!("{}_rotated", old_share.share_id),
        wallet_address: old_share.wallet_address.clone(),
        share_index: old_share.share_index,
        share_value: new_share_value,
        encrypted_backup: old_share.encrypted_backup.clone(),
        created_at: old_share.created_at,
        rotated_at: Some(chrono::Utc::now().timestamp()),
    };
    
    Ok(new_share)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_encrypt_decrypt() {
        let share = KeyShare {
            share_id: "test_share".to_string(),
            wallet_address: "0x1234".to_string(),
            share_index: 1,
            share_value: vec![1, 2, 3, 4],
            encrypted_backup: Vec::new(),
            created_at: 1234567890,
            rotated_at: None,
        };
        
        let password = "test_password";
        
        let encrypted = encrypt_share(&share, password).unwrap();
        let decrypted = decrypt_share(&encrypted, password).unwrap();
        
        assert_eq!(decrypted.share_id, share.share_id);
        assert_eq!(decrypted.share_value, share.share_value);
    }
}
