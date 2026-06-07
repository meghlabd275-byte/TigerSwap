//! Intent Settlement Binary

use intent_settlement::{IntentSettlementNetwork, Intent, Solver};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("TigerSwap Intent Settlement Network");
    println!("===============================");
    
    // Initialize network
    let network = IntentSettlementNetwork::new();
    
    println!("Network initialized");
    
    // Register solvers
    let solvers = vec![
        ("0xsolver1".to_string(), 20_000u128),
        ("0xsolver2".to_string(), 30_000u128),
        ("0xsolver3".to_string(), 25_000u128),
    ];
    
    let mut solver_ids = Vec::new();
    for (address, stake) in solvers {
        match network.register_solver(address.clone(), stake * 10u128.pow(18)).await {
            Ok(id) => {
                println!("Registered solver: {}", address);
                solver_ids.push(id);
            }
            Err(e) => {
                println!("Failed to register solver {}: {}", address, e);
            }
        }
    }
    
    println!("\nRegistered {} solvers", solver_ids.len());
    
    // Submit intents
    let intents = vec![
        ("0xowner1".to_string(), "ETH".to_string(), "USDC".to_string(), 1000u128, 900u128),
        ("0xowner2".to_string(), "BTC".to_string(), "ETH".to_string(), 100u128, 2000u128),
        ("0xowner3".to_string(), "USDC".to_string(), "DAI".to_string(), 5000u128, 4900u128),
    ];
    
    let mut intent_ids = Vec::new();
    for (owner, sell, buy, amount, min_amount) in intents {
        let intent = Intent::new(owner, sell, buy, amount, min_amount);
        let id = network.submit_intent(intent).await;
        println!("Submitted intent: {} -> {} ({} {})", sell, buy, amount, min_amount);
        intent_ids.push(id);
    }
    
    println!("\nSubmitted {} intents", intent_ids.len());
    
    // Fill intents
    for (intent_id, solver_id) in intent_ids.iter().zip(solver_ids.iter()) {
        match network.fill_intent(intent_id, solver_id, 1000).await {
            Ok(fill) => {
                println!("Filled intent {}: amount={}", intent_id, fill.fill_amount);
            }
            Err(e) => {
                println!("Failed to fill {}: {}", intent_id, e);
            }
        }
    }
    
    println!("\nIntent Settlement Network ready!");
    
    Ok(())
}