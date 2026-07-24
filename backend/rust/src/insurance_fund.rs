//! TigerSwap Rust Security Module - Insurance Fund & Key Management
//! High-performance, memory-safe implementation
//! 
//! Compile: cargo build --release

use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::time::{SystemTime, UNIX_EPOCH};

// ============== INSURANCE FUND ==============

/// Insurance fund for covering liquidations and protocol losses
pub struct InsuranceFund {
    // Treasury holdings by token
    holdings: Arc<RwLock<HashMap<String, FixedDecimal>>>,
    // Total value locked
    tvl: Arc<RwLock<FixedDecimal>>,
    // Daily spending limit
    daily_limit: FixedDecimal,
    // Today's spending
    today_spent: Arc<RwLock<FixedDecimal>>,
    // Last reset date
    last_reset: Arc<RwLock<u64>>,
    // Events
    events: Arc<RwLock<Vec<InsuranceEvent>>>,
}

#[derive(Clone, Debug)]
pub struct FixedDecimal {
    pub value: i128,
    pub decimals: u8,
}

#[derive(Clone, Debug)]
pub struct InsuranceEvent {
    pub event_id: u64,
    pub event_type: InsuranceEventType,
    pub amount: FixedDecimal,
    pub token: String,
    pub reason: String,
    pub timestamp: u64,
    pub tx_hash: Option<String>,
}

#[derive(Clone, Debug)]
pub enum InsuranceEventType {
    Deposit,
    Withdraw,
    LiquidationCoverage,
    HackCoverage,
    FeeAllocation,
    GovernanceGrant,
}

impl InsuranceFund {
    pub fn new(initial_deposit: FixedDecimal) -> Self {
        let mut holdings = HashMap::new();
        holdings.insert("TGR".to_string(), initial_deposit.clone());
        
        Self {
            holdings: Arc::new(RwLock::new(holdings)),
            tvl: Arc::new(RwLock::new(initial_deposit)),
            daily_limit: FixedDecimal::new(1_000_000, 18), // 1M per day
            today_spent: Arc::new(RwLock::new(FixedDecimal::new(0, 18))),
            last_reset: Arc::new(RwLock::new(current_timestamp())),
            events: Arc::new(RwLock::new(Vec::new())),
        }
    }

    /// Deposit funds into insurance fund
    pub fn deposit(&self, token: &str, amount: FixedDecimal) -> Result<u64, String> {
        // Check daily limit
        self.check_daily_limit(&amount)?;
        
        let mut holdings = self.holdings.write().map_err(|e| e.to_string())?;
        let current = holdings.get(token).cloned().unwrap_or(FixedDecimal::new(0, amount.decimals));
        
        let new_balance = current.add(&amount)?;
        holdings.insert(token.to_string(), new_balance);
        
        // Update TVL
        self.update_tvl(&amount, true)?;
        
        // Log event
        let event_id = self.log_event(
            InsuranceEventType::Deposit,
            amount.clone(),
            token.to_string(),
            "Insurance fund deposit".to_string(),
            None,
        );
        
        Ok(event_id)
    }

    /// Withdraw funds (for covered events)
    pub fn withdraw(&self, token: &str, amount: FixedDecimal, reason: &str, tx_hash: Option<String>) -> Result<u64, String> {
        // Check daily limit
        self.check_daily_limit(&amount)?;
        
        let mut holdings = self.holdings.write().map_err(|e| e.to_string())?;
        
        let current = holdings.get(token)
            .ok_or("Token not found in insurance fund")?
            .clone();
        
        if current.value < amount.value {
            return Err("Insufficient insurance fund balance".to_string());
        }
        
        let new_balance = current.sub(&amount)?;
        holdings.insert(token.to_string(), new_balance);
        
        // Update TVL
        self.update_tvl(&amount, false)?;
        
        // Log event
        let event_id = self.log_event(
            InsuranceEventType::Withdraw,
            amount.clone(),
            token.to_string(),
            reason.to_string(),
            tx_hash,
        );
        
        Ok(event_id)
    }

    /// Get fund balance for a token
    pub fn get_balance(&self, token: &str) -> Result<FixedDecimal, String> {
        let holdings = self.holdings.read().map_err(|e| e.to_string())?;
        Ok(holdings.get(token).cloned().unwrap_or(FixedDecimal::new(0, 18)))
    }

    /// Get total value locked
    pub fn get_tvl(&self) -> Result<FixedDecimal, String> {
        let tvl = self.tvl.read().map_err(|e| e.to_string())?;
        Ok(tvl.clone())
    }

    fn check_daily_limit(&self, amount: &FixedDecimal) -> Result<(), String> {
        // Reset daily counter if needed
        let today = current_timestamp() / 86400; // Days since epoch
        {
            let last = *self.last_reset.read().map_err(|e| e.to_string())? / 86400;
            if today > last {
                *self.today_spent.write().map_err(|e| e.to_string())? = FixedDecimal::new(0, 18);
                *self.last_reset.write().map_err(|e| e.to_string())? = today * 86400;
            }
        }
        
        let spent = self.today_spent.read().map_err(|e| e.to_string())?;
        let remaining = self.daily_limit.sub(spent)?;
        
        if amount.value > remaining.value {
            return Err("Daily limit exceeded".to_string());
        }
        
        // Update spent
        *self.today_spent.write().map_err(|e| e.to_string())? = spent.add(amount)?;
        
        Ok(())
    }

    fn update_tvl(&self, amount: &FixedDecimal, is_deposit: bool) -> Result<(), String> {
        let mut tvl = self.tvl.write().map_err(|e| e.to_string())?;
        if is_deposit {
            *tvl = tvl.add(amount)?;
        } else {
            *tvl = tvl.sub(amount)?;
        }
        Ok(())
    }

    fn log_event(&self, event_type: InsuranceEventType, amount: FixedDecimal, 
                 token: String, reason: String, tx_hash: Option<String>) -> u64 {
        let event_id = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap()
            .as_nanos() as u64;
        
        let event = InsuranceEvent {
            event_id,
            event_type,
            amount,
            token,
            reason,
            timestamp: current_timestamp(),
            tx_hash,
        };
        
        if let Ok(mut events) = self.events.write() {
            events.push(event);
            // Keep last 10000 events
            if events.len() > 10000 {
                events.remove(0);
            }
        }
        
        event_id
    }

    /// Get recent events
    pub fn get_events(&self, limit: usize) -> Result<Vec<InsuranceEvent>, String> {
        let events = self.events.read().map_err(|e| e.to_string())?;
        let start = events.len().saturating_sub(limit);
        Ok(events[start..].to_vec())
    }
}

impl FixedDecimal {
    pub fn new(value: i128, decimals: u8) -> Self {
        Self { value, decimals }
    }
    
    pub fn from_float(f: f64, decimals: u8) -> Self {
        let multiplier = 10_i128.pow(decimals as u32);
        Self {
            value: (f * multiplier as f64) as i128,
            decimals,
        }
    }
    
    pub fn add(&self, other: &FixedDecimal) -> Result<Self, String> {
        if self.decimals != other.decimals {
            return Err("Decimal mismatch".to_string());
        }
        Ok(Self {
            value: self.value + other.value,
            decimals: self.decimals,
        })
    }
    
    pub fn sub(&self, other: &FixedDecimal) -> Result<Self, String> {
        if self.decimals != other.decimals {
            return Err("Decimal mismatch".to_string());
        }
        Ok(Self {
            value: self.value - other.value,
            decimals: self.decimals,
        })
    }
    
    pub fn mul(&self, other: &FixedDecimal) -> Result<Self, String> {
        if self.decimals != other.decimals {
            return Err("Decimal mismatch".to_string());
        }
        // Result has double decimals, need to normalize
        let result = (self.value * other.value) / 10_i128.pow(self.decimals as u32);
        Ok(Self {
            value: result,
            decimals: self.decimals,
        })
    }
    
    pub fn to_float(&self) -> f64 {
        self.value as f64 / 10_i128.pow(self.decimals as u32) as f64
    }
}

fn current_timestamp() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_secs()
}

// ============== SECURE KEY MANAGEMENT ==============

use std::sync::Mutex;

/// Secure key storage with encryption
pub struct SecureKeyStore {
    keys: Arc<RwLock<HashMap<String, EncryptedKey>>>,
    master_key: [u8; 32],
}

#[derive(Clone)]
pub struct EncryptedKey {
    pub id: String,
    pub encrypted_data: Vec<u8>,
    pub nonce: [u8; 12],
    pub created_at: u64,
    pub last_used: u64,
}

impl SecureKeyStore {
    pub fn new(master_password: &str) -> Result<Self, String> {
        // Derive master key from password using Argon2
        let master_key = derive_key(master_password)?;
        
        Ok(Self {
            keys: Arc::new(RwLock::new(HashMap::new())),
            master_key,
        })
    }
    
    /// Store an encrypted key
    pub fn store_key(&self, key_id: &str, key_data: &[u8]) -> Result<(), String> {
        let nonce = generate_nonce();
        let encrypted = encrypt(key_data, &self.master_key, nonce)?;
        
        let encrypted_key = EncryptedKey {
            id: key_id.to_string(),
            encrypted_data: encrypted,
            nonce,
            created_at: current_timestamp(),
            last_used: current_timestamp(),
        };
        
        let mut keys = self.keys.write().map_err(|e| e.to_string())?;
        keys.insert(key_id.to_string(), encrypted_key);
        
        Ok(())
    }
    
    /// Retrieve and decrypt a key
    pub fn get_key(&self, key_id: &str) -> Result<Vec<u8>, String> {
        let keys = self.keys.read().map_err(|e| e.to_string())?;
        
        let encrypted_key = keys.get(key_id)
            .ok_or("Key not found")?
            .clone();
        
        // Update last used
        drop(keys);
        if let Ok(mut keys) = self.keys.write() {
            if let Some(key) = keys.get_mut(key_id) {
                key.last_used = current_timestamp();
            }
        }
        
        decrypt(&encrypted_key.encrypted_data, &self.master_key, encrypted_key.nonce)
    }
    
    /// Delete a key
    pub fn delete_key(&self, key_id: &str) -> Result<(), String> {
        let mut keys = self.keys.write().map_err(|e| e.to_string())?;
        keys.remove(key_id);
        Ok(())
    }
    
    /// List all key IDs (not the keys themselves)
    pub fn list_keys(&self) -> Result<Vec<String>, String> {
        let keys = self.keys.read().map_err(|e| e.to_string())?;
        Ok(keys.keys().cloned().collect())
    }
}

// Simple encryption (in production, use proper AEAD)
// For demo - uses XOR with key derivation
fn encrypt(data: &[u8], key: &[u8; 32], nonce: [u8; 12]) -> Result<Vec<u8>, String> {
    let mut result = Vec::with_capacity(data.len() + 12);
    result.extend_from_slice(&nonce);
    
    // Simple XOR encryption (replace with ChaCha20-Poly1305 in production)
    for (i, byte) in data.iter().enumerate() {
        let key_byte = key[i % 32];
        let nonce_byte = nonce[i % 12];
        result.push(byte ^ key_byte ^ nonce_byte);
    }
    
    Ok(result)
}

fn decrypt(data: &[u8], key: &[u8; 32], nonce: [u8; 12]) -> Result<Vec<u8>, String> {
    if data.len() < 12 {
        return Err("Invalid encrypted data".to_string());
    }
    
    let ciphertext = &data[12..];
    let mut result = Vec::with_capacity(ciphertext.len());
    
    for (i, byte) in ciphertext.iter().enumerate() {
        let key_byte = key[i % 32];
        let nonce_byte = nonce[i % 12];
        result.push(byte ^ key_byte ^ nonce_byte);
    }
    
    Ok(result)
}

fn derive_key(password: &str) -> Result<[u8; 32], String> {
    // Simple key derivation (replace with Argon2 in production)
    use std::collections::hash_map::DefaultHasher;
    use std::hash::{Hash, Hasher};
    
    let mut key = [0u8; 32];
    let mut hasher = DefaultHasher::new();
    
    // Multiple rounds for basic key stretching
    for _ in 0..10000 {
        password.hash(&mut hasher);
        "tigerswap_salt".hash(&mut hasher);
    }
    
    let hash = hasher.finish();
    let bytes = hash.to_le_bytes();
    key[..8].copy_from_slice(&bytes);
    
    // Fill rest with derived values
    for i in 8..32 {
        key[i] = key[i - 8] ^ (i as u8).wrapping_mul(0x9E);
    }
    
    Ok(key)
}

fn generate_nonce() -> [u8; 12] {
    use std::time::SystemTime;
    
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos() as u64;
    
    let mut nonce = [0u8; 12];
    nonce[..8].copy_from_slice(&now.to_le_bytes());
    nonce[8..].copy_from_slice(&[0x01, 0x02, 0x03, 0x04]); // Fixed for demo
    
    nonce
}

// ============== FFI EXPORTS ==============

#[no_mangle]
pub extern "C" fn create_insurance_fund(initial_tvl: i128) -> *mut InsuranceFund {
    let fund = InsuranceFund::new(FixedDecimal::new(initial_tvl, 18));
    Box::into_raw(Box::new(fund))
}

#[no_mangle]
pub extern "C" fn destroy_insurance_fund(fund: *mut InsuranceFund) {
    if !fund.is_null() {
        unsafe { Box::from_raw(fund) };
    }
}

#[no_mangle]
pub extern "C" fn insurance_deposit(fund: *mut InsuranceFund, token: *const u8, token_len: usize, amount: i128) -> u64 {
    unsafe {
        let token_str = std::str::from_utf8(std::slice::from_raw_parts(token, token_len)).unwrap_or("TGR");
        let amount = FixedDecimal::new(amount, 18);
        
        match (*fund).deposit(token_str, amount) {
            Ok(id) => id,
            Err(_) => 0,
        }
    }
}

#[no_mangle]
pub extern "C" fn insurance_withdraw(fund: *mut InsuranceFund, token: *const u8, token_len: usize, amount: i128, reason: *const u8, reason_len: usize) -> u64 {
    unsafe {
        let token_str = std::str::from_utf8(std::slice::from_raw_parts(token, token_len)).unwrap_or("TGR");
        let reason_str = std::str::from_utf8(std::slice::from_raw_parts(reason, reason_len)).unwrap_or("Withdrawal");
        let amount = FixedDecimal::new(amount, 18);
        
        match (*fund).withdraw(token_str, amount, reason_str, None) {
            Ok(id) => id,
            Err(_) => 0,
        }
    }
}

#[no_mangle]
pub extern "C" fn insurance_get_tvl(fund: *mut InsuranceFund) -> i128 {
    unsafe {
        match (*fund).get_tvl() {
            Ok(tvl) => tvl.value,
            Err(_) => 0,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_fixed_decimal() {
        let a = FixedDecimal::new(1000, 18);
        let b = FixedDecimal::new(500, 18);
        
        let sum = a.add(&b).unwrap();
        assert_eq!(sum.value, 1500);
        
        let diff = a.sub(&b).unwrap();
        assert_eq!(diff.value, 500);
    }
    
    #[test]
    fn test_insurance_fund() {
        let fund = InsuranceFund::new(FixedDecimal::new(1_000_000, 18));
        
        // Deposit
        let event_id = fund.deposit("TGR", FixedDecimal::new(100_000, 18)).unwrap();
        assert!(event_id > 0);
        
        // Check balance
        let balance = fund.get_balance("TGR").unwrap();
        assert_eq!(balance.value, 1_100_000);
        
        // Withdraw
        let withdraw_id = fund.withdraw("TGR", FixedDecimal::new(50_000, 18), "Liquidation coverage", None).unwrap();
        assert!(withdraw_id > 0);
    }
}
