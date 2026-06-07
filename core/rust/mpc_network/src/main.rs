//! MPC Node Binary - Main entry point for MPC node

use std::net::SocketAddr;
use std::sync::Arc;
use std::time::Duration;

use mpc_network::{
    MPCNetwork, MPCNode, MPCCoordinator, NodeConfig, NodeRuntime, KeyGenParams,
};

use tokio::time;
use tracing::{info, error, Level};
use tracing_subscriber::FmtSubscriber;

/// Node configuration
#[derive(Debug, Clone)]
struct Config {
    node_id: String,
    listen_address: SocketAddr,
    coordinator_addresses: Vec<SocketAddr>,
    threshold: u32,
    total_nodes: u32,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            node_id: uuid::Uuid::new_v4().to_string(),
            listen_address: "0.0.0.0:9000".parse().unwrap(),
            coordinator_addresses: Vec::new(),
            threshold: 2,
            total_nodes: 3,
        }
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Initialize logging
    let subscriber = FmtSubscriber::builder()
        .with_max_level(Level::INFO)
        .with_target(false)
        .with_thread_ids(false)
        .with_file(true)
        .with_line_number(true)
        .finish();
    
    tracing::subscriber::set_global_default(subscriber)
        .expect("Failed to set tracing subscriber");
    
    info!("Starting MPC Node");
    
    // Load configuration
    let config = Config::default();
    
    info!("Node ID: {}", config.node_id);
    info!("Listen address: {}", config.listen_address);
    
    // Create MPC network
    let network = Arc::new(MPCNetwork::new(
        config.node_id.clone(),
        false, // Not coordinator
    ));
    
    // Create and register this node
    let node = MPCNode::new(
        config.node_id.clone(),
        config.listen_address,
    );
    
    network.register_node(node).await;
    
    info!("Node registered successfully");
    
    // Start key generation if this is coordinator
    if config.total_nodes >= 2 {
        // Register additional nodes for demo
        for i in 1..=config.total_nodes as usize - 1 {
            let node = MPCNode::new(
                format!("node-{}", i),
                format!("127.0.0.1:{}", 9000 + i).parse().unwrap(),
            );
            network.register_node(node).await;
        }
        
        info!("All {} nodes registered", config.total_nodes);
        
        // Start key generation
        match network.generate_key(config.threshold, config.total_nodes).await {
            Ok(session_id) => {
                info!("Key generation started: session_id={}", session_id);
            }
            Err(e) => {
                error!("Failed to start key generation: {}", e);
            }
        }
    }
    
    // Heartbeat loop
    let mut interval = time::interval(Duration::from_secs(30));
    
    loop {
        interval.tick().await;
        
        let active_count = network.active_node_count().await;
        let session_count = network.active_session_count().await;
        
        info!(
            "Status: active_nodes={}, active_sessions={}",
            active_count, session_count
        );
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_config_default() {
        let config = Config::default();
        
        assert!(!config.node_id.is_empty());
        assert_eq!(config.threshold, 2);
        assert_eq!(config.total_nodes, 3);
    }
}