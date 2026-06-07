//! MPC Node Module - Individual signing node implementation

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::net::SocketAddr;
use std::sync::Arc;
use tokio::sync::RwLock;
use uuid::Uuid;

/// MPC Node - Individual signing node in the network
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct MPCNode {
    pub id: String,
    pub address: SocketAddr,
    pub public_key: Vec<u8>,
    pub is_active: bool,
    pub last_heartbeat: i64,
    pub reputation_score: f64,
    pub total_signatures: u64,
    pub successful_signatures: u64,
    pub failed_signatures: u64,
    pub average_latency_ms: u64,
    pub joined_at: i64,
    pub metadata: HashMap<String, String>,
}

impl MPCNode {
    /// Create new MPC node
    pub fn new(id: String, address: SocketAddr) -> Self {
        Self {
            id,
            address,
            public_key: Vec::new(),
            is_active: true,
            last_heartbeat: chrono::Utc::now().timestamp(),
            reputation_score: 100.0,
            total_signatures: 0,
            successful_signatures: 0,
            failed_signatures: 0,
            average_latency_ms: 0,
            joined_at: chrono::Utc::now().timestamp(),
            metadata: HashMap::new(),
        }
    }
    
    /// Update heartbeat timestamp
    pub fn update_heartbeat(&mut self) {
        self.last_heartbeat = chrono::Utc::now().timestamp();
    }
    
    /// Record successful signature
    pub fn record_success(&mut self, latency_ms: u64) {
        self.total_signatures += 1;
        self.successful_signatures += 1;
        
        // Update average latency
        let total = self.total_signatures;
        self.average_latency_ms = ((self.average_latency_ms * (total - 1)) + latency_ms) / total;
        
        // Update reputation
        self.reputation_score = (self.successful_signatures as f64 / self.total_signatures as f64) * 100.0;
    }
    
    /// Record failed signature
    pub fn record_failure(&mut self) {
        self.total_signatures += 1;
        self.failed_signatures += 1;
        self.reputation_score = (self.successful_signatures as f64 / self.total_signatures as f64) * 100.0;
    }
    
    /// Check if node is responsive (heartbeat within 5 minutes)
    pub fn is_responsive(&self) -> bool {
        let now = chrono::Utc::now().timestamp();
        now - self.last_heartbeat < 300
    }
    
    /// Check if node is healthy
    pub fn is_healthy(&self) -> bool {
        self.is_active && self.is_responsive() && self.reputation_score >= 50.0
    }
    
    /// Get uptime in seconds
    pub fn uptime(&self) -> i64 {
        chrono::Utc::now().timestamp() - self.joined_at
    }
    
    /// Get success rate
    pub fn success_rate(&self) -> f64 {
        if self.total_signatures == 0 {
            return 100.0;
        }
        (self.successful_signatures as f64 / self.total_signatures as f64) * 100.0
    }
    
    /// Set metadata
    pub fn set_metadata(&mut self, key: &str, value: &str) {
        self.metadata.insert(key.to_string(), value.to_string());
    }
    
    /// Get metadata
    pub fn get_metadata(&self, key: &str) -> Option<&String> {
        self.metadata.get(key)
    }
    
    /// Deactivate node
    pub fn deactivate(&mut self) {
        self.is_active = false;
    }
    
    /// Reactivate node
    pub fn activate(&mut self) {
        self.is_active = true;
        self.update_heartbeat();
    }
}

/// Node statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NodeStats {
    pub node_id: String,
    pub uptime_seconds: i64,
    pub total_signatures: u64,
    pub successful_signatures: u64,
    pub failed_signatures: u64,
    pub success_rate: f64,
    pub reputation_score: f64,
    pub average_latency_ms: u64,
    pub last_active: i64,
}

impl From<&MPCNode> for NodeStats {
    fn from(node: &MPCNode) -> Self {
        Self {
            node_id: node.id.clone(),
            uptime_seconds: node.uptime(),
            total_signatures: node.total_signatures,
            successful_signatures: node.successful_signatures,
            failed_signatures: node.failed_signatures,
            success_rate: node.success_rate(),
            reputation_score: node.reputation_score,
            average_latency_ms: node.average_latency_ms,
            last_active: node.last_heartbeat,
        }
    }
}

/// Node configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NodeConfig {
    pub node_id: String,
    pub listen_address: SocketAddr,
    pub coordinator_addresses: Vec<SocketAddr>,
    pub timeout_seconds: u64,
    pub max_retries: u32,
    pub tls_enabled: bool,
    pub certificate_path: Option<String>,
    pub private_key_path: Option<String>,
}

impl Default for NodeConfig {
    fn default() -> Self {
        Self {
            node_id: Uuid::new_v4().to_string(),
            listen_address: "0.0.0.0:9000".parse().unwrap(),
            coordinator_addresses: Vec::new(),
            timeout_seconds: 30,
            max_retries: 3,
            tls_enabled: false,
            certificate_path: None,
            private_key_path: None,
        }
    }
}

/// Node runtime state
pub struct NodeRuntime {
    pub node: Arc<RwLock<MPCNode>>,
    pub config: NodeConfig,
    pub active_sessions: RwLock<HashMap<String, SessionState>>,
    pub key_shares: RwLock<HashMap<String, Vec<u8>>>,
}

impl NodeRuntime {
    pub fn new(config: NodeConfig) -> Self {
        let node = MPCNode::new(
            config.node_id.clone(),
            config.listen_address,
        );
        
        Self {
            node: Arc::new(RwLock::new(node)),
            config,
            active_sessions: RwLock::new(HashMap::new()),
            key_shares: RwLock::new(HashMap::new()),
        }
    }
    
    /// Get node ID
    pub async fn node_id(&self) -> String {
        self.node.read().await.id.clone()
    }
    
    /// Check if node is healthy
    pub async fn is_healthy(&self) -> bool {
        self.node.read().await.is_healthy()
    }
    
    /// Store key share
    pub async fn store_share(&self, session_id: &str, share: Vec<u8>) {
        let mut shares = self.key_shares.write().await;
        shares.insert(session_id.to_string(), share);
    }
    
    /// Get key share
    pub async fn get_share(&self, session_id: &str) -> Option<Vec<u8>> {
        let shares = self.key_shares.read().await;
        shares.get(session_id).cloned()
    }
    
    /// Add signing session
    pub async fn add_session(&self, session_id: &str, state: SessionState) {
        let mut sessions = self.active_sessions.write().await;
        sessions.insert(session_id.to_string(), state);
    }
    
    /// Get signing session
    pub async fn get_session(&self, session_id: &str) -> Option<SessionState> {
        let sessions = self.active_sessions.read().await;
        sessions.get(session_id).cloned()
    }
    
    /// Remove signing session
    pub async fn remove_session(&self, session_id: &str) {
        let mut sessions = self.active_sessions.write().await;
        sessions.remove(session_id);
    }
    
    /// Update heartbeat
    pub async fn heartbeat(&self) {
        self.node.read().await.update_heartbeat();
    }
}

/// Session state for signing
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SessionState {
    pub session_id: String,
    pub round: u32,
    pub status: SessionStatus,
    pub started_at: i64,
    pub last_update: i64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum SessionStatus {
    Pending,
    InProgress,
    Awaiting,
    Completed,
    Failed,
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::net::ToSocketAddrs;
    
    #[test]
    fn test_node_creation() {
        let node = MPCNode::new(
            "node1".to_string(),
            "127.0.0.1:8080".parse().unwrap(),
        );
        
        assert_eq!(node.id, "node1");
        assert!(node.is_active);
        assert!(node.is_responsive());
    }
    
    #[test]
    fn test_node_stats() {
        let node = MPCNode::new(
            "node1".to_string(),
            "127.0.0.1:8080".parse().unwrap(),
        );
        
        node.record_success(100);
        node.record_success(200);
        node.record_failure();
        
        let stats = NodeStats::from(&node);
        
        assert_eq!(stats.total_signatures, 3);
        assert_eq!(stats.successful_signatures, 2);
        assert!(approx_eq!(stats.success_rate, 66.67, 0.01));
    }
    
    fn approx_eq(a: f64, b: f64, eps: f64) -> bool {
        (a - b).abs() < eps
    }
}