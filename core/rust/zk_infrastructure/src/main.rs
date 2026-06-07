//! ZK Prover Binary

use std::sync::Arc;

use zk_infrastructure::{
    ZKManager, ZKCircuit, ZKProofInputs, ProofType,
};

use tracing::{info, error};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("TigerSwap ZK Infrastructure");
    println!("=============================");
    
    // Initialize ZK manager
    let manager = Arc::new(ZKManager::new());
    
    info!("ZK Manager initialized");
    
    // Register circuits
    let circuits = vec![
        ("swap".to_string(), "Swap Circuit".to_string(), 4),
        ("bridge".to_string(), "Bridge Circuit".to_string(), 2),
        ("identity".to_string(), "Identity Circuit".to_string(), 1),
        ("compression".to_string(), "Compression Circuit".to_string(), 1),
    ];
    
    for (id, name, inputs) in circuits {
        let circuit = ZKCircuit::new(id, name, inputs);
        manager.prover().register_circuit(circuit).await;
    }
    
    info!("Registered {} circuits", circuits.len());
    
    // Setup circuits
    for (id, _, _) in &circuits {
        if let Err(e) = manager.prover().setup(id).await {
            error!("Failed to setup {}: {}", id, e);
        }
    }
    
    info!("Setup completed for all circuits");
    
    // Generate test proof
    let inputs = ZKProofInputs::new()
        .with_public(vec![vec![1, 2, 3, 4]])
        .with_private(vec![vec![5, 6, 7, 8]]);
    
    match manager.prover().prove("swap", inputs).await {
        Ok(proof) => {
            info!("Proof generated: {}", proof.proof_id);
            
            match manager.prover().verify(&proof).await {
                Ok(valid) => {
                    info!("Proof verification: {}", if valid { "VALID" } else { "INVALID" });
                }
                Err(e) => {
                    error!("Verification failed: {}", e);
                }
            }
        }
        Err(e) => {
            error!("Proof generation failed: {}", e);
        }
    }
    
    println!("\nZK Infrastructure ready!");
    
    Ok(())
}