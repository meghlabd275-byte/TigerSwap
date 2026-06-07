//! Analytics Service Binary

use analytics_service::{AnalyticsEngine, TVL, Trade, TradeSide};
use std::collections::HashMap;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("TigerSwap Analytics Service");
    println!("=====================");
    
    // Initialize analytics engine
    let engine = AnalyticsEngine::new();
    
    println!("Analytics Engine initialized");
    
    // Calculate TVL
    println!("\nCalculating TVL...");
    let mut token_balances = HashMap::new();
    token_balances.insert("ETH".to_string(), 1000000.0);
    token_balances.insert("BTC".to_string(), 500000.0);
    token_balances.insert("USDC".to_string(), 2000000.0);
    
    let tvl = engine.calculate_tvl(&token_balances).await;
    println!("Total TVL: ${:.2}", tvl.total);
    
    // Record trades
    println!("\nRecording trades...");
    let trades = vec![
        Trade {
            trade_id: "1".to_string(),
            pair: "ETH/USDC".to_string(),
            side: TradeSide::Buy,
            size: 1.0,
            price: 100.0,
            pnl: 10.0,
            timestamp: chrono::Utc::now().timestamp(),
            hold_time: 3600.0,
        },
        Trade {
            trade_id: "2".to_string(),
            pair: "ETH/USDC".to_string(),
            side: TradeSide::Sell,
            size: 0.5,
            price: 110.0,
            pnl: -5.0,
            timestamp: chrono::Utc::now().timestamp(),
            hold_time: 7200.0,
        },
        Trade {
            trade_id: "3".to_string(),
            pair: "BTC/USDC".to_string(),
            side: TradeSide::Buy,
            size: 0.1,
            price: 50000.0,
            pnl: 100.0,
            timestamp: chrono::Utc::now().timestamp(),
            hold_time: 1800.0,
        },
    ];
    
    for trade in trades {
        engine.record_trade(trade).await;
    }
    
    // Get trading analytics
    println!("\nCalculating trading analytics...");
    let analytics = engine.get_trading_analytics().await;
    
    println!("Total Trades: {}", analytics.total_trades);
    println!("Win Rate: {:.2}%", analytics.win_rate * 100.0);
    println!("Average Profit: ${:.2}", analytics.avg_profit);
    println!("Average Loss: ${:.2}", analytics.avg_loss);
    println!("Largest Profit: ${:.2}", analytics.largest_profit);
    println!("Largest Loss: ${:.2}", analytics.largest_loss);
    
    println!("\nAnalytics Service ready!");
    
    Ok(())
}