use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::Arc;
use parking_lot::RwLock;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
pub enum MEVType { #[default] None, FrontRun, Sandwich, BackRun, FlashLoan }

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MEVDetection { pub mev_type: MEVType, pub confidence: u8 }

impl MEVDetection { pub fn clean() -> Self { Self { mev_type: MEVType::None, confidence: 100 } } }

pub struct MEVProtection {
    known_bots: Arc<RwLock<HashMap<String, bool>>>,
}

impl MEVProtection {
    pub fn new() -> Self {
        let mut bots = HashMap::new();
        bots.insert("0x47176b1afb3c3174794b0e7d42c0c5d5ce91c6f2c".to_string(), true);
        Self { known_bots: Arc::new(RwLock::new(bots)) }
    }
    pub fn detect(&self, _tx_data: &str, from: &str) -> MEVDetection {
        if self.known_bots.read().contains_key(&from.to_lowercase()) { MEVDetection { mev_type: MEVType::FrontRun, confidence: 95 } }
        else { MEVDetection::clean() }
    }
}

impl Default for MEVProtection { fn default() -> Self { Self::new() } }