//! MPC Network Module - High-level network management

use std::sync::Arc;
use tokio::sync::RwLock;

use crate::{KeyGenParams, MPCError, MPCNode, MPCCoordinator, ThresholdSignature};

/// MPC Network - High-level network manager
pub struct MPCNetwork {
    coordinator: Arc<MPCCoordinator>,
    node_id: String,
    is_coordinator: bool,
}

impl MPCNetwork {
    /// Create new MPC network
    pub fn new(node_id: String, is_coordinator: bool) -> Self {
        Self {
            coordinator: Arc::new(MPCCoordinator::new()),
            node_id,
            is_coordinator,
        }
    }
    
    /// Get coordinator reference
    pub fn coordinator(&self) -> &Arc<MPCCoordinator> {
        &self.coordinator
    }
    
    /// Get node ID
    pub fn node_id(&self) -> &str {
        &self.node_id
    }
    
    /// Check if this is the coordinator
    pub fn is_coordinator(&self) -> bool {
        self.is_coordinator
    }
    
    /// Generate new MPC key with threshold t-of-n
    pub async fn generate_key(
        &self,
        threshold: u32,
        total_nodes: u32,
    ) -> Result<String, MPCError> {
        let params = KeyGenParams::new(threshold, total_nodes);
        self.coordinator.start_key_generation(params).await
    }
    
    /// Register a node
    pub async fn register_node(&self, node: MPCNode) {
        self.coordinator.register_node(node).await;
    }
    
    /// Get all nodes
    pub async fn get_nodes(&self) -> Vec<MPCNode> {
        self.coordinator.get_nodes().await
    }
    
    /// Sign message with threshold signature
    pub async fn sign(
        &self,
        session_id: &str,
        message: &[u8],
    ) -> Result<ThresholdSignature, MPCError> {
        let signing_id = self.coordinator
            .start_signing(session_id, message.to_vec())
            .await?;
        
        // In production, collect signatures from all nodes
        // This is a simplified placeholder
        
        self.coordinator.combine_signatures(&signing_id).await
    }
    
    /// Get public key for session
    pub async fn get_public_key(&self, session_id: &str) -> Option<Vec<u8>> {
        self.coordinator.get_public_key(session_id).await
    }
    
    /// Get active node count
    pub async fn active_node_count(&self) -> usize {
        self.coordinator.get_active_nodes().await.len()
    }
    
    /// Get active session count
    pub async fn active_session_count(&self) -> usize {
        self.coordinator.active_session_count().await
    }
}

/// Builder for MPC Network
pub struct MPCNetworkBuilder {
    node_id: String,
    is_coordinator: bool,
    initial_nodes: Vec<MPCNode>,
}

impl MPCNetworkBuilder {
    pub fn new(node_id: String) -> Self {
        Self {
            node_id,
            is_coordinator: false,
            initial_nodes: Vec::new(),
        }
    }
    
    pub fn as_coordinator(mut self) -> Self {
        self.is_coordinator = true;
        self
    }
    
    pub fn with_node(mut self, node: MPCNode) -> Self {
        self.initial_nodes.push(node);
        self
    }
    
    pub fn build(self) -> MPCNetwork {
        let network = MPCNetwork::new(self.node_id, self.is_coordinator);
        network
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::net::ToSocketAddrs;
    
    #[tokio::test]
    async fn test_mpc_network() {
        let network = MPCNetwork::new("test-node".to_string(), true);
        
        // Register nodes
        for i in 1..=3 {
            let node = MPCNode::new(
                format!("node{}", i),
                format!("127.0.0.1:{}", 8080 + i).parse().unwrap(),
            );
            network.register_node(node).await;
        }
        
        let nodes = network.get_nodes().await;
        assert_eq!(nodes.len(), 3);
    }
    
    #[tokio::test]
    async fn test_key_generation() {
        let network = MPCNetwork::new("coordinator".to_string(), true);
        
        // Register nodes
        for i in 1..=3 {
            let node = MPCNode::new(
                format!("node{}", i),
                format!("127.0.0.1:{}", 8080 + i).parse().unwrap(),
            );
            network.register_node(node).await;
        }
        
        let session_id = network.generate_key(2, 3).await.unwrap();
        assert!(!session_id.is_empty());
    }
}