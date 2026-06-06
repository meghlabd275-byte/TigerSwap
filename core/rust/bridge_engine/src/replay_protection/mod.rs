//! Replay Protection
//! 
//! Prevents replay attacks on cross-chain messages.

use std::collections::{HashSet, VecDeque};
use std::sync::{Arc, RwLock};
use std::time::{SystemTime, UNIX_EPOCH};

const MAX_NONCE_AGE: usize = 1000;

pub struct ReplayProtection {
    used_nonces: RwLock<HashSet<(u32, u64)>>,
    recent_nonces: RwLock<VecDeque<(u32, u64)>>,
    chain_windows: RwLock<HashMap<u32, u64>>,
}

impl ReplayProtection {
    pub fn new() -> Self {
        Self {
            used_nonces: RwLock::new(HashSet::new()),
            recent_nonces: RwLock::new(VecDeque::new()),
            chain_windows: RwLock::new(HashMap::new()),
        }
    }
    
    pub fn check_and_record(&self, chain: u32, nonce: u64) -> bool {
        // Check if nonce is within valid window
        let windows = self.chain_windows.read().unwrap();
        let window_start = windows.get(&chain).copied().unwrap_or(0);
        
        if nonce < window_start {
            return false;
        }
        
        // Check if already used
        if self.used_nonces.read().unwrap().contains(&(chain, nonce)) {
            return false;
        }
        
        // Record nonce
        self.used_nonces.write().unwrap().insert((chain, nonce));
        self.recent_nonces.write().unwrap().push_back((chain, nonce));
        
        // Prune old nonces
        let mut recent = self.recent_nonces.write().unwrap();
        while recent.len() > MAX_NONCE_AGE {
            if let Some((c, n)) = recent.pop_front() {
                self.used_nonces.write().unwrap().remove(&(c, n));
            }
        }
        
        true
    }
    
    pub fn set_window(&self, chain: u32, window_start: u64) {
        self.chain_windows.write().unwrap().insert(chain, window_start);
    }
    
    pub fn get_window(&self, chain: u32) -> u64 {
        self.chain_windows.read().unwrap().get(&chain).copied().unwrap_or(0)
    }
    
    pub fn cleanup(&self, chain: u32, current_nonce: u64) {
        let cutoff = current_nonce.saturating_sub(MAX_NONCE_AGE as u64);
        let mut used = self.used_nonces.write().unwrap();
        used.retain(|(c, n)| *c != chain || *n > cutoff);
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_check_nonce() {
        let protection = ReplayProtection::new();
        assert!(protection.check_and_record(1, 1));
        assert!(!protection.check_and_record(1, 1)); // Replay!
    }
    
    #[test]
    fn test_window() {
        let protection = ReplayProtection::new();
        protection.set_window(1, 100);
        assert!(!protection.check_and_record(1, 50)); // Before window
        assert!(protection.check_and_record(1, 150));
    }
}