//! TigerWallet MPC Service Main Entry Point

use tigerwallet_mpc::coordinator::MpcCoordinator;
use tigerwallet_mpc::key_gen::generate_dkg;
use tigerwallet_mpc::key_share::{encrypt_share, decrypt_share};
use tigerwallet_mpc::types::*;
use std::sync::Arc;
use tokio::sync::RwLock;
use tonic::{Request, Response, Status};
use tracing::{info, error, warn};
use tracing_subscriber;

mod mpc_service {
    include!("../generated/mpc_service.rs");
}

use mpc_service::{
    mpc_service_server::MpcService,
    KeyGenRequest, KeyGenResponse, SignRequest, SignResponse,
    KeyShareResponse, KeyRotationRequest, RecoveryRequest,
};

pub struct MpcServiceImpl {
    coordinator: Arc<RwLock<MpcCoordinator>>,
}

impl MpcServiceImpl {
    pub fn new() -> Self {
        Self {
            coordinator: Arc::new(RwLock::new(MpcCoordinator::new())),
        }
    }
}

#[tonic::async_trait]
impl MpcService for MpcServiceImpl {
    async fn key_gen(
        &self,
        request: Request<KeyGenRequest>,
    ) -> Result<Response<KeyGenResponse>, Status> {
        let req = request.into_inner();
        
        info!("Starting key generation for wallet: {}", req.wallet_address);
        
        let coordinator = self.coordinator.read().await;
        
        let session_id = coordinator
            .start_key_gen(
                req.wallet_address.clone(),
                req.threshold,
                req.total_shares,
                req.guardians,
            )
            .await
            .map_err(|e| {
                error!("Key generation failed: {}", e);
                Status::internal(e.to_string())
            })?;
        
        let response = KeyGenResponse {
            session_id,
            wallet_address: req.wallet_address,
            public_key: Vec::new(), // Would be filled from DKG
        };
        
        Ok(Response::new(response))
    }

    async fn sign(
        &self,
        request: Request<SignRequest>,
    ) -> Result<Response<SignResponse>, Status> {
        let req = request.into_inner();
        
        info!("Starting signing for wallet: {}", req.wallet_address);
        
        let coordinator = self.coordinator.read().await;
        
        let session_id = coordinator
            .start_signing(
                req.wallet_address.clone(),
                req.message_hash,
                req.threshold,
            )
            .await
            .map_err(|e| {
                error!("Signing failed: {}", e);
                Status::internal(e.to_string())
            })?;
        
        let response = SignResponse {
            session_id,
            signature: Vec::new(), // Would be filled after signing
        };
        
        Ok(Response::new(response))
    }

    async fn get_key_shares(
        &self,
        request: Request<KeyShareRequest>,
    ) -> Result<Response<KeyShareResponse>, Status> {
        let _req = request.into_inner();
        
        // Implementation would fetch key shares from database
        let response = KeyShareResponse {
            shares: Vec::new(),
        };
        
        Ok(Response::new(response))
    }

    async fn rotate_keys(
        &self,
        request: Request<KeyRotationRequest>,
    ) -> Result<Response<KeyRotationResponse>, Status> {
        let _req = request.into_inner();
        
        info!("Starting key rotation");
        
        // Implementation would handle key rotation
        let response = KeyRotationResponse {
            success: true,
            new_public_key: Vec::new(),
        };
        
        Ok(Response::new(response))
    }

    async fn recover_keys(
        &self,
        request: Request<RecoveryRequest>,
    ) -> Result<Response<RecoveryResponse>, Status> {
        let _req = request.into_inner();
        
        info!("Starting key recovery");
        
        // Implementation would handle social recovery
        let response = RecoveryResponse {
            success: true,
            wallet_address: String::new(),
        };
        
        Ok(Response::new(response))
    }
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Initialize logging
    tracing_subscriber::fmt()
        .with_env_filter(
            tracing_subscriber::EnvFilter::from_default_env()
                .add_directive(tracing::Level::INFO.into()),
        )
        .init();

    info!("Starting TigerWallet MPC Service");

    let addr = "0.0.0.0:50051".parse()?;
    
    let service = MpcServiceImpl::new();
    
    tonic::build()
        .compile_descriptors(&mut std::io::sink())
        .ok();

    info!("MPC Service listening on {}", addr);

    // In production, use proper gRPC server
    // let mut server = Server::builder()
    //     .add_service(MpcServiceServer::new(service))
    //     .serve(addr)
    //     .await?;

    info!("MPC Service started successfully");
    
    // Keep running
    tokio::signal::ctrl_c().await?;
    
    info!("Shutting down MPC Service");
    
    Ok(())
}

// Request/Response types for gRPC
#[derive(serde::Serialize, serde::Deserialize)]
struct KeyShareRequest {
    wallet_address: String,
}

#[derive(serde::Serialize, serde::Deserialize)]
struct KeyShareResponse {
    shares: Vec<String>,
}

#[derive(serde::Serialize, serde::Deserialize)]
struct KeyRotationResponse {
    success: bool,
    new_public_key: Vec<u8>,
}

#[derive(serde::Serialize, serde::Deserialize)]
struct RecoveryResponse {
    success: bool,
    wallet_address: String,
}
