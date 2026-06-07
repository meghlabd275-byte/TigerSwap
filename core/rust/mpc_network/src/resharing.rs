//! Resharing Module - Dynamic key share redistribution

use serde::{Deserialize, Serialize};
use uuid::Uuid;

/// Key resharing request
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ReshareRequest {
    pub session_id: String,
    pub old_threshold: u32,
    pub new_threshold: u32,
    pub new_nodes: Vec<String>,
    pub request_id: String,
    pub created_at: i64,
    pub status: ReshareStatus,
}

impl ReshareRequest {
    pub fn new(
        session_id: String,
        new_threshold: u32,
        new_nodes: Vec<String>,
    ) -> Self {
        Self {
            session_id,
            old_threshold: 0,
            new_threshold,
            new_nodes,
            request_id: Uuid::new_v4().to_string(),
            created_at: chrono::Utc::now().timestamp(),
            status: ReshareStatus::Pending,
        }
    }
    
    pub fn with_old_threshold(mut self, threshold: u32) -> Self {
        self.old_threshold = threshold;
        self
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ReshareStatus {
    Pending,
    InProgress,
    Completed,
    Failed,
}

/// Key rotation request
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KeyRotationRequest {
    pub session_id: String,
    pub rotation_id: String,
    pub reason: RotationReason,
    pub scheduled_at: i64,
    pub executed_at: Option<i64>,
    pub status: RotationStatus,
}

impl KeyRotationRequest {
    pub fn new(session_id: String, reason: RotationReason) -> Self {
        Self {
            session_id,
            rotation_id: Uuid::new_v4().to_string(),
            reason,
            scheduled_at: chrono::Utc::now().timestamp(),
            executed_at: None,
            status: RotationStatus::Scheduled,
        }
    }
    
    pub fn schedule_for(mut self, timestamp: i64) -> Self {
        self.scheduled_at = timestamp;
        self
    }
    
    pub fn execute(&mut self) {
        self.executed_at = Some(chrono::Utc::now().timestamp());
        self.status = RotationStatus::Executed;
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum RotationReason {
    Scheduled,
    NodeCompromise,
    NodeDeparture,
    ThresholdChange,
    SecurityAudit,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum RotationStatus {
    Scheduled,
    InProgress,
    Executed,
    Failed,
}

/// Share redistribution packet
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ShareRedistribution {
    pub request_id: String,
    pub from_node: String,
    pub to_node: String,
    pub encrypted_share: Vec<u8>,
    pub nonce: [u8; 12],
    pub round: u32,
}

impl ShareRedistribution {
    pub fn new(
        request_id: String,
        from_node: String,
        to_node: String,
        encrypted_share: Vec<u8>,
        nonce: [u8; 12],
    ) -> Self {
        Self {
            request_id,
            from_node,
            to_node,
            encrypted_share,
            nonce,
            round: 1,
        }
    }
}

/// Resharing result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ReshareResult {
    pub request_id: String,
    pub session_id: String,
    pub new_threshold: u32,
    pub new_total_nodes: u32,
    pub new_public_key: Vec<u8>,
    pub successful_nodes: Vec<String>,
    pub failed_nodes: Vec<String>,
    pub completed_at: i64,
    pub duration_ms: u64,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_reshare_request() {
        let request = ReshareRequest::new(
            "session1".to_string(),
            2,
            vec!["node1".to_string(), "node2".to_string()],
        );
        
        assert_eq!(request.status, ReshareStatus::Pending);
    }
    
    #[test]
    fn test_rotation_request() {
        let request = KeyRotationRequest::new(
            "session1".to_string(),
            RotationReason::Scheduled,
        );
        
        assert_eq!(request.status, RotationStatus::Scheduled);
        
        let mut request = request;
        request.execute();
        
        assert_eq!(request.status, RotationStatus::Executed);
        assert!(request.executed_at.is_some());
    }
}