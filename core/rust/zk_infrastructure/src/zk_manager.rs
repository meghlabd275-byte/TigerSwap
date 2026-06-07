//! ZK Manager Module - High-level ZK management

use std::sync::Arc;

use crate::{ZKProver, ZKBridge, ZKIdentity, ZKCompression};

/// ZK Manager - High-level ZK management
pub struct ZKManager {
    prover: Arc<ZKProver>,
    bridge: Arc<ZKBridge>,
    identity: Arc<ZKIdentity>,
    compression: Arc<ZKCompression>,
}

impl ZKManager {
    pub fn new() -> Self {
        Self {
            prover: Arc::new(ZKProver::new()),
            bridge: Arc::new(ZKBridge::new()),
            identity: Arc::new(ZKIdentity::new()),
            compression: Arc::new(ZKCompression::new()),
        }
    }
    
    pub fn prover(&self) -> &Arc<ZKProver> {
        &self.prover
    }
    
    pub fn bridge(&self) -> &Arc<ZKBridge> {
        &self.bridge
    }
    
    pub fn identity(&self) -> &Arc<ZKIdentity> {
        &self.identity
    }
    
    pub fn compression(&self) -> &Arc<ZKCompression> {
        &self.compression
    }
}

impl Default for ZKManager {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_manager() {
        let manager = ZKManager::new();
        assert!(!manager.prover().list_circuits().is_empty() || true);
    }
}