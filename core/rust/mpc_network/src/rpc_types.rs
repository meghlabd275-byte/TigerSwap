//! RPC Types Module - gRPC communication types

use serde::{Deserialize, Serialize};

/// Node request
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NodeRequest {
    pub request_id: String,
    pub session_id: String,
    pub node_id: String,
    pub payload: Vec<u8>,
    pub timestamp: i64,
    pub request_type: RequestType,
}

impl NodeRequest {
    pub fn new(session_id: String, node_id: String, payload: Vec<u8>, request_type: RequestType) -> Self {
        Self {
            request_id: uuid::Uuid::new_v4().to_string(),
            session_id,
            node_id,
            payload,
            timestamp: chrono::Utc::now().timestamp(),
            request_type,
        }
    }
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum RequestType {
    KeyGenCommitment,
    KeyGenShare,
    SigningStart,
    SigningPartial,
    Heartbeat,
    KeyRotation,
}

/// Node response
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NodeResponse {
    pub request_id: String,
    pub node_id: String,
    pub success: bool,
    pub payload: Option<Vec<u8>>,
    pub error: Option<String>,
    pub timestamp: i64,
}

impl NodeResponse {
    pub fn success(request_id: String, node_id: String, payload: Vec<u8>) -> Self {
        Self {
            request_id,
            node_id,
            success: true,
            payload: Some(payload),
            error: None,
            timestamp: chrono::Utc::now().timestamp(),
        }
    }
    
    pub fn error(request_id: String, node_id: String, error: String) -> Self {
        Self {
            request_id,
            node_id,
            success: false,
            payload: None,
            error: Some(error),
            timestamp: chrono::Utc::now().timestamp(),
        }
    }
}

/// Batch request for multiple nodes
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BatchRequest {
    pub batch_id: String,
    pub requests: Vec<NodeRequest>,
    pub created_at: i64,
}

impl BatchRequest {
    pub fn new(requests: Vec<NodeRequest>) -> Self {
        Self {
            batch_id: uuid::Uuid::new_v4().to_string(),
            requests,
            created_at: chrono::Utc::now().timestamp(),
        }
    }
}

/// Batch response
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BatchResponse {
    pub batch_id: String,
    pub responses: Vec<NodeResponse>,
    pub completed_at: i64,
}

/// gRPC service message
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GRPCMessage {
    pub message_id: String,
    pub message_type: MessageType,
    pub payload: Vec<u8>,
    pub sender_id: String,
    pub receiver_id: Option<String>,
    pub timestamp: i64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum MessageType {
    Request,
    Response,
    Error,
    Heartbeat,
    Notification,
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_node_request() {
        let request = NodeRequest::new(
            "session1".to_string(),
            "node1".to_string(),
            vec![1, 2, 3],
            RequestType::KeyGenCommitment,
        );
        
        assert!(!request.request_id.is_empty());
    }
    
    #[test]
    fn test_node_response_success() {
        let response = NodeResponse::success(
            "req1".to_string(),
            "node1".to_string(),
            vec![1, 2, 3],
        );
        
        assert!(response.success);
        assert!(response.payload.is_some());
    }
    
    #[test]
    fn test_node_response_error() {
        let response = NodeResponse::error(
            "req1".to_string(),
            "node1".to_string(),
            "Test error".to_string(),
        );
        
        assert!(!response.success);
        assert!(response.error.is_some());
    }
}