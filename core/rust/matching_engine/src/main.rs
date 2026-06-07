//! Matching Engine Binary

use matching_engine::{MatchingEngine, Order, OrderSide};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("TigerSwap Matching Engine");
    println!("======================");
    
    // Initialize matching engine
    let engine = MatchingEngine::new();
    
    println!("Matching Engine initialized");
    
    // Place sample orders
    println!("\nPlacing sample orders...");
    
    let orders = vec![
        Order::new_limit("1", "ETH/USDC", OrderSide::Buy, 100.0, 1.0, "user1"),
        Order::new_limit("2", "ETH/USDC", OrderSide::Buy, 101.0, 2.0, "user2"),
        Order::new_limit("3", "ETH/USDC", OrderSide::Buy, 102.0, 1.5, "user3"),
        Order::new_limit("4", "ETH/USDC", OrderSide::Sell, 103.0, 1.0, "user4"),
        Order::new_limit("5", "ETH/USDC", OrderSide::Sell, 104.0, 2.0, "user5"),
    ];
    
    for order in orders {
        println!("Placed {} order: {} {} {} @ {}",
            if order.side == OrderSide::Buy { "BUY" } else { "SELL" },
            order.amount,
            order.pair,
            order.price
        );
    }
    
    // Get order book
    println!("\nGetting order book...");
    let book = engine.get_order_book("ETH/USDC").await;
    if let Some(b) = book {
        if let Some(bid) = b.best_bid() {
            println!("Best bid: {}", bid.price);
        }
        if let Some(ask) = b.best_ask() {
            println!("Best ask: {}", ask.price);
        }
        if let Some(spread) = b.spread() {
            println!("Spread: {}", spread);
        }
    }
    
    // Get stats
    println!("\nGetting stats...");
    let stats = engine.get_stats().await;
    println!("Total orders: {}", stats.total_orders);
    println!("Total trades: {}", stats.total_trades);
    
    println!("\nMatching Engine ready!");
    
    Ok(())
}