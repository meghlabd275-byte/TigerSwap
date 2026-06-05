//! TigerSwap MEV Protection - Rust Implementation
//! 
//! Provides Flashbots-style MEV protection for all swaps
//! Detects and prevents sandwich attacks, front-running, and arbitrage

use serde::{Deserialize, Serialize};
use std::collections::{HashMap, HashSet};

mod detection;
mod bundle;
mod protection;

pub use detection::{SandwichAttack, SuspiciousPattern, AttackType};
pub use bundle::{BundleBuilder, BundleTransaction, BundleResult};
pub use protection::{MEVProtectionService, MEVConfig};

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_mev_config_default() {
        let config = MEVConfig::default();
        assert!(config.enabled);
        assert!(config.use_flashbots);
    }
}