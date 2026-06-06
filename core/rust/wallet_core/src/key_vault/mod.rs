//! Key Vault - Secure Key Storage and Management
//! 
//! Provides secure storage for sensitive key material with encryption at rest,
//! access control, audit logging, and key rotation support.

use std::collections::BTreeMap;
use std::sync::{Arc, RwLock};
use std::time::{SystemTime, UNIX_EPOCH};
use thiserror::Error;

#[derive(Error, Debug)]
pub enum KeyVaultError {
    #[error("Key not found: {0}")]
    KeyNotFound(String),
    #[error("Encryption failed: {0}")]
    EncryptionFailed(String),
    #[error("Decryption failed: {0}")]
    DecryptionFailed(String),
    #[error("Access denied: {0}")]
    AccessDenied(String),
    #[error("Invalid key: {0}")]
    InvalidKey(String),
    #[error("Vault locked: {0}")]
    VaultLocked(String),
    #[error("Key expired: {0}")]
    KeyExpired(String),
}

// ============================================================================
// Types
// ============================================================================

/// Key type
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum KeyType {
    PrivateKey,
    PublicKey,
    Mnemonic,
    Seed,
    Secret,
}

/// Key status
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum KeyStatus {
    Active,
    Rotating,
    Revoked,
    Expired,
}

/// Key metadata
#[derive(Debug, Clone)]
pub struct KeyMetadata {
    pub key_id: String,
    pub key_type: KeyType,
    pub created_at: u64,
    pub expires_at: Option<u64>,
    pub last_used: Option<u64>,
    pub rotation_period: Option<u64>,
    pub status: KeyStatus,
    pub tags: BTreeMap<String, String>,
}

/// Stored encrypted key
#[derive(Debug, Clone)]
pub struct StoredKey {
    pub metadata: KeyMetadata,
    pub encrypted_data: Vec<u8>,
    pub nonce: [u8; 12],
}

/// Access policy
#[derive(Debug, Clone)]
pub struct AccessPolicy {
    pub allowed_principals: Vec<String>,
    pub allowed_ips: Vec<String>,
    pub max_daily_usage: Option<u64>,
    pub require_mfa: bool,
}

impl AccessPolicy {
    pub fn new() -> Self {
        Self {
            allowed_principals: Vec::new(),
            allowed_ips: Vec::new(),
            max_daily_usage: None,
            require_mfa: false,
        }
    }
    
    pub fn with_principal(mut self, principal: String) -> Self {
        self.allowed_principals.push(principal);
        self
    }
    
    pub fn with_ip(mut self, ip: String) -> Self {
        self.allowed_ips.push(ip);
        self
    }
    
    pub fn with_daily_limit(mut self, limit: u64) -> Self {
        self.max_daily_usage = Some(limit);
        self
    }
    
    pub fn with_mfa(mut self) -> Self {
        self.require_mfa = true;
        self
    }
}

/// Audit log entry
#[derive(Debug, Clone)]
pub struct AuditEntry {
    pub timestamp: u64,
    pub principal: String,
    pub action: String,
    pub key_id: String,
    pub success: bool,
    pub ip: Option<String>,
    pub details: String,
}

// ============================================================================
// Key Vault
// ============================================================================

pub struct KeyVault {
    keys: RwLock<BTreeMap<String, StoredKey>>,
    policies: RwLock<BTreeMap<String, AccessPolicy>>,
    audit_log: RwLock<Vec<AuditEntry>>,
    master_key: RwLock<Option<[u8; 32]>>,
    locked: RwLock<bool>,
}

impl KeyVault {
    pub fn new() -> Self {
        Self {
            keys: RwLock::new(BTreeMap::new()),
            policies: RwLock::new(BTreeMap::new()),
            audit_log: RwLock::new(Vec::new()),
            master_key: RwLock::new(None),
            locked: RwLock::new(true),
        }
    }
    
    /// Unlock vault with master key
    pub fn unlock(&self, master_key: [u8; 32]) -> Result<(), KeyVaultError> {
        *self.master_key.write().unwrap() = Some(master_key);
        *self.locked.write().unwrap() = false;
        Ok(())
    }
    
    /// Lock vault
    pub fn lock(&self) -> Result<(), KeyVaultError> {
        *self.master_key.write().unwrap() = None;
        *self.locked.write().unwrap() = true;
        Ok(())
    }
    
    /// Check if vault is locked
    pub fn is_locked(&self) -> bool {
        *self.locked.read().unwrap()
    }
    
    /// Store a key
    pub fn store_key(
        &self,
        key_id: String,
        key_type: KeyType,
        data: &[u8],
        metadata: Option<KeyMetadata>,
    ) -> Result<(), KeyVaultError> {
        if self.is_locked() {
            return Err(KeyVaultError::VaultLocked("vault is locked".to_string()));
        }
        
        // Encrypt key data
        let (encrypted, nonce) = self.encrypt(data)?;
        
        let meta = metadata.unwrap_or(KeyMetadata {
            key_id: key_id.clone(),
            key_type,
            created_at: current_timestamp(),
            expires_at: None,
            last_used: None,
            rotation_period: None,
            status: KeyStatus::Active,
            tags: BTreeMap::new(),
        });
        
        let stored = StoredKey {
            metadata: meta,
            encrypted_data: encrypted,
            nonce,
        };
        
        self.keys.write().unwrap().insert(key_id, stored);
        
        Ok(())
    }
    
    /// Retrieve a key
    pub fn get_key(&self, key_id: &str, principal: &str) -> Result<Vec<u8>, KeyVaultError> {
        if self.is_locked() {
            return Err(KeyVaultError::VaultLocked("vault is locked".to_string()));
        }
        
        // Check access policy
        self.check_access(key_id, principal)?;
        
        let keys = self.keys.read().unwrap();
        let stored = keys.get(key_id)
            .ok_or_else(|| KeyVaultError::KeyNotFound(key_id.to_string()))?;
        
        // Check expiration
        if let Some(expires) = stored.metadata.expires_at {
            if current_timestamp() > expires {
                return Err(KeyVaultError::KeyExpired(key_id.to_string()));
            }
        }
        
        // Decrypt key data
        let decrypted = self.decrypt(&stored.encrypted_data, stored.nonce)?;
        
        // Update last used
        drop(keys);
        if let Some(key) = self.keys.write().unwrap().get_mut(key_id) {
            key.metadata.last_used = Some(current_timestamp());
        }
        
        // Audit
        self.log_audit(principal, "key_read", key_id, true, None);
        
        Ok(decrypted)
    }
    
    /// Delete a key
    pub fn delete_key(&self, key_id: &str, principal: &str) -> Result<(), KeyVaultError> {
        if self.is_locked() {
            return Err(KeyVaultError::VaultLocked("vault is locked".to_string()));
        }
        
        self.check_access(key_id, principal)?;
        
        let mut keys = self.keys.write().unwrap();
        keys.remove(key_id)
            .ok_or_else(|| KeyVaultError::KeyNotFound(key_id.to_string()))?;
        
        drop(keys);
        self.log_audit(principal, "key_delete", key_id, true, None);
        
        Ok(())
    }
    
    /// Set access policy for a key
    pub fn set_policy(&self, key_id: &str, policy: AccessPolicy) -> Result<(), KeyVaultError> {
        if self.is_locked() {
            return Err(KeyVaultError::VaultLocked("vault is locked".to_string()));
        }
        
        // Verify key exists
        if !self.keys.read().unwrap().contains_key(key_id) {
            return Err(KeyVaultError::KeyNotFound(key_id.to_string()));
        }
        
        self.policies.write().unwrap().insert(key_id.to_string(), policy);
        Ok(())
    }
    
    /// Get key metadata
    pub fn get_metadata(&self, key_id: &str) -> Result<KeyMetadata, KeyVaultError> {
        let keys = self.keys.read().unwrap();
        let stored = keys.get(key_id)
            .ok_or_else(|| KeyVaultError::KeyNotFound(key_id.to_string()))?;
        Ok(stored.metadata.clone())
    }
    
    /// List all key IDs
    pub fn list_keys(&self) -> Vec<String> {
        self.keys.read().unwrap().keys().cloned().collect()
    }
    
    /// Rotate a key
    pub fn rotate_key(&self, key_id: &str, new_data: &[u8], principal: &str) -> Result<(), KeyVaultError> {
        if self.is_locked() {
            return Err(KeyVaultError::VaultLocked("vault is locked".to_string()));
        }
        
        self.check_access(key_id, principal)?;
        
        // Mark old key as rotating
        {
            let mut keys = self.keys.write().unwrap();
            if let Some(key) = keys.get_mut(key_id) {
                key.metadata.status = KeyStatus::Rotating;
            }
        }
        
        // Store new key
        self.store_key(key_id.to_string(), KeyType::PrivateKey, new_data, None)?;
        
        // Mark new key as active
        {
            let mut keys = self.keys.write().unwrap();
            if let Some(key) = keys.get_mut(key_id) {
                key.metadata.status = KeyStatus::Active;
            }
        }
        
        self.log_audit(principal, "key_rotate", key_id, true, None);
        Ok(())
    }
    
    /// Revoke a key
    pub fn revoke_key(&self, key_id: &str, principal: &str) -> Result<(), KeyVaultError> {
        if self.is_locked() {
            return Err(KeyVaultError::VaultLocked("vault is locked".to_string()));
        }
        
        self.check_access(key_id, principal)?;
        
        let mut keys = self.keys.write().unwrap();
        if let Some(key) = keys.get_mut(key_id) {
            key.metadata.status = KeyStatus::Revoked;
        }
        
        drop(keys);
        self.log_audit(principal, "key_revoke", key_id, true, None);
        
        Ok(())
    }
    
    /// Get audit log
    pub fn get_audit_log(&self, key_id: Option<&str>) -> Vec<AuditEntry> {
        let log = self.audit_log.read().unwrap();
        
        if let Some(id) = key_id {
            log.iter()
                .filter(|e| e.key_id == id)
                .cloned()
                .collect()
        } else {
            log.clone()
        }
    }
    
    // ============================================================================
    // Internal Methods
    // ============================================================================
    
    fn encrypt(&self, data: &[u8]) -> Result<(Vec<u8>, [u8; 12]), KeyVaultError> {
        // Simplified encryption
        // In production: use AES-256-GCM with master key
        
        let master_key = self.master_key.read().unwrap();
        let key = master_key.as_ref()
            .ok_or_else(|| KeyVaultError::EncryptionFailed("no master key".to_string()))?;
        
        let mut nonce = [0u8; 12];
        for (i, byte) in nonce.iter_mut().enumerate() {
            *byte = key[i % 32];
        }
        
        let mut encrypted = data.to_vec();
        for (i, byte) in encrypted.iter_mut().enumerate() {
            *byte ^= key[i % 32];
        }
        
        Ok((encrypted, nonce))
    }
    
    fn decrypt(&self, data: &[u8], nonce: [u8; 12]) -> Result<Vec<u8>, KeyVaultError> {
        let master_key = self.master_key.read().unwrap();
        let key = master_key.as_ref()
            .ok_or_else(|| KeyVaultError::DecryptionFailed("no master key".to_string()))?;
        
        // Decrypt (XOR is symmetric)
        let mut decrypted = data.to_vec();
        for (i, byte) in decrypted.iter_mut().enumerate() {
            *byte ^= key[i % 32];
        }
        
        Ok(decrypted)
    }
    
    fn check_access(&self, key_id: &str, principal: &str) -> Result<(), KeyVaultError> {
        let policies = self.policies.read().unwrap();
        
        if let Some(policy) = policies.get(key_id) {
            // Check principal
            if !policy.allowed_principals.is_empty() && 
               !policy.allowed_principals.contains(&principal.to_string()) {
                return Err(KeyVaultError::AccessDenied(
                    format!("principal {} not allowed for key {}", principal, key_id)
                ));
            }
            
            // Check MFA requirement
            if policy.require_mfa {
                // In production: verify MFA token
            }
        }
        
        Ok(())
    }
    
    fn log_audit(&self, principal: &str, action: &str, key_id: &str, success: bool, ip: Option<String>) {
        let entry = AuditEntry {
            timestamp: current_timestamp(),
            principal: principal.to_string(),
            action: action.to_string(),
            key_id: key_id.to_string(),
            success,
            ip,
            details: String::new(),
        };
        
        self.audit_log.write().unwrap().push(entry);
    }
}

// ============================================================================
// Key Rotation
// ============================================================================

pub struct KeyRotation {
    vault: Arc<KeyVault>,
    rotation_interval: u64,
}

impl KeyRotation {
    pub fn new(vault: Arc<KeyVault>, rotation_interval_days: u64) -> Self {
        Self {
            vault,
            rotation_interval: rotation_interval_days * 86400,
        }
    }
    
    /// Check if key needs rotation
    pub fn needs_rotation(&self, key_id: &str) -> bool {
        if let Ok(meta) = self.vault.get_metadata(key_id) {
            if let Some(period) = meta.rotation_period {
                if let Some(created) = Some(meta.created_at) {
                    return current_timestamp() - created > period;
                }
            }
        }
        false
    }
    
    /// Auto-rotate keys that need rotation
    pub fn auto_rotate(&self, new_key_data: &[u8]) -> Vec<String> {
        let mut rotated = Vec::new();
        
        for key_id in self.vault.list_keys() {
            if self.needs_rotation(&key_id) {
                if self.vault.rotate_key(&key_id, new_key_data, "system").is_ok() {
                    rotated.push(key_id);
                }
            }
        }
        
        rotated
    }
}

// ============================================================================
// Helper Functions
// ============================================================================

fn current_timestamp() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

// ============================================================================
// Tests
// ============================================================================

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_key_vault_creation() {
        let vault = KeyVault::new();
        assert!(vault.is_locked());
    }

    #[test]
    fn test_key_vault_unlock() {
        let vault = KeyVault::new();
        let key = [0u8; 32];
        vault.unlock(key).unwrap();
        assert!(!vault.is_locked());
    }

    #[test]
    fn test_store_and_retrieve_key() {
        let vault = KeyVault::new();
        let key = [0u8; 32];
        vault.unlock(key).unwrap();
        
        let secret = b"my-secret-key";
        vault.store_key("test-key".to_string(), KeyType::Secret, secret, None).unwrap();
        
        let retrieved = vault.get_key("test-key", "admin").unwrap();
        assert_eq!(retrieved, secret);
    }

    #[test]
    fn test_key_not_found() {
        let vault = KeyVault::new();
        let key = [0u8; 32];
        vault.unlock(key).unwrap();
        
        let result = vault.get_key("non-existent", "admin");
        assert!(result.is_err());
    }

    #[test]
    fn test_delete_key() {
        let vault = KeyVault::new();
        let master_key = [0u8; 32];
        vault.unlock(master_key).unwrap();
        
        vault.store_key("test-key".to_string(), KeyType::Secret, b"secret", None).unwrap();
        vault.delete_key("test-key", "admin").unwrap();
        
        assert!(!vault.list_keys().contains(&"test-key".to_string()));
    }

    #[test]
    fn test_access_policy() {
        let policy = AccessPolicy::new()
            .with_principal("admin".to_string())
            .with_ip("192.168.1.0/24".to_string())
            .with_daily_limit(1000)
            .with_mfa();
        
        assert!(policy.require_mfa);
        assert_eq!(policy.allowed_principals.len(), 1);
    }

    #[test]
    fn test_audit_log() {
        let vault = KeyVault::new();
        let key = [0u8; 32];
        vault.unlock(key).unwrap();
        
        vault.store_key("test-key".to_string(), KeyType::Secret, b"secret", None).unwrap();
        vault.get_key("test-key", "admin").unwrap();
        
        let log = vault.get_audit_log(Some("test-key"));
        assert!(!log.is_empty());
    }

    #[test]
    fn test_key_rotation() {
        let vault = Arc::new(KeyVault::new());
        let master_key = [0u8; 32];
        vault.unlock(master_key).unwrap();
        
        let rotator = KeyRotation::new(vault.clone(), 30);
        assert!(!rotator.needs_rotation("test-key"));
    }
}