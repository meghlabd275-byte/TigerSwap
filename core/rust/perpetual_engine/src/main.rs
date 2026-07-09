//! TigerSwap Perpetual Engine CLI
//! Command-line interface for perpetual trading engine

use clap::{Parser, Subcommand};
use tiger_perpetual_engine::{PerpetualEngine, Market, OrderSide, OrderType, TimeInForce};
use rust_decimal::Decimal;
use chrono::Utc;

#[derive(Parser)]
#[command(name = "perpetual-engine")]
#[command(about = "TigerSwap Perpetual Trading Engine", long_about = None)]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Start the engine
    Start {
        port: Option<u16>,
    },
    
    /// Add a new market
    AddMarket {
        id: String,
        symbol: String,
        base: String,
        quote: String,
        max_leverage: u32,
    },
    
    /// List all markets
    ListMarkets,
    
    /// Create an order
    CreateOrder {
        market_id: String,
        user_id: String,
        side: String,
        order_type: String,
        price: f64,
        quantity: f64,
        leverage: u32,
    },
    
    /// Get position
    GetPosition {
        user_id: String,
        market_id: String,
    },
    
    /// Get account
    GetAccount {
        user_id: String,
    },
    
    /// Update price
    UpdatePrice {
        market_id: String,
        price: f64,
    },
}

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let cli = Cli::parse();
    let engine = std::sync::Arc::new(PerpetualEngine::new());
    
    // Initialize default markets
    init_default_markets(&engine);
    
    match cli.command {
        Commands::Start { port } => {
            let port = port.unwrap_or(8080);
            println!("Starting Perpetual Engine on port {}", port);
            // In production, start gRPC/REST server
            loop {
                tokio::time::sleep(tokio::time::Duration::from_secs(1)).await;
            }
        },
        
        Commands::AddMarket { id, symbol, base, quote, max_leverage } => {
            let market = Market {
                id: id.clone(),
                symbol: symbol.clone(),
                base_asset: base,
                quote_asset: quote,
                mark_price: Decimal::ZERO,
                index_price: Decimal::ZERO,
                last_price: Decimal::ZERO,
                funding_rate: Decimal::ZERO,
                funding_timestamp: 0,
                next_funding_time: 0,
                max_leverage,
                max_position_size: Decimal::from(1_000_000),
                maintenance_margin_rate: Decimal::from(50000),  // 5%
                initial_margin_rate: Decimal::from(100000),    // 10%
                is_active: true,
                is_paused: false,
            };
            
            engine.add_market(market);
            println!("Added market: {}", id);
        },
        
        Commands::ListMarkets => {
            let markets = engine.list_markets();
            for market in markets {
                println!("{} ({}) - Leverage: {}x", market.symbol, market.id, market.max_leverage);
            }
        },
        
        Commands::CreateOrder { market_id, user_id, side, order_type, price, quantity, leverage } => {
            let side = if side.to_lowercase() == "buy" { OrderSide::Buy } else { OrderSide::Sell };
            let order_type = match order_type.to_lowercase().as_str() {
                "market" => OrderType::Market,
                "limit" => OrderType::Limit,
                "stop_loss" => OrderType::StopLoss,
                "take_profit" => OrderType::TakeProfit,
                _ => OrderType::Limit,
            };
            
            let order = engine.create_order(
                &market_id,
                &user_id,
                side,
                order_type,
                Decimal::from(price),
                Decimal::from(quantity),
                leverage,
                TimeInForce::GTC,
                None,
            );
            
            match order {
                Ok(o) => println!("Order created: {} - {} {} @ {}",
                    o.id, o.side.debug(), o.quantity, o.price),
                Err(e) => println!("Error: {:?}", e),
            }
        },
        
        Commands::GetPosition { user_id, market_id } => {
            let position = engine.get_position(&user_id, &market_id);
            match position {
                Some(p) => {
                    println!("Position: {} {} @ {}",
                        p.side.debug(), p.size, p.entry_price);
                    println!("PnL: {}", p.unrealized_pnl);
                },
                None => println!("No position found"),
            }
        },
        
        Commands::GetAccount { user_id } => {
            let account = engine.get_account(&user_id);
            match account {
                Some(a) => {
                    println!("Account: {}", a.user_id);
                    println!("Collateral: {}", a.total_collateral);
                    println!("Available: {}", a.available_collateral);
                    println!("Status: {:?}", a.account_status);
                },
                None => println!("No account found"),
            }
        },
        
        Commands::UpdatePrice { market_id, price } => {
            engine.update_price(&market_id, Decimal::from(price));
            println!("Updated {} to {}", market_id, price);
        },
    }
    
    Ok(())
}

fn init_default_markets(engine: &PerpetualEngine) {
    let markets = vec![
        Market {
            id: "ETH-PERP".to_string(),
            symbol: "ETH-PERP".to_string(),
            base_asset: "ETH".to_string(),
            quote_asset: "USD".to_string(),
            mark_price: Decimal::from(3250000000000000000i64),
            index_price: Decimal::from(3250000000000000000i64),
            last_price: Decimal::from(3250000000000000000i64),
            funding_rate: Decimal::ZERO,
            funding_timestamp: Utc::now().timestamp(),
            next_funding_time: Utc::now().timestamp() + 28800,  // 8 hours
            max_leverage: 50,
            max_position_size: Decimal::from(1_000_000),
            maintenance_margin_rate: Decimal::from(50000),  // 5%
            initial_margin_rate: Decimal::from(100000),     // 10%
            is_active: true,
            is_paused: false,
        },
        Market {
            id: "BTC-PERP".to_string(),
            symbol: "BTC-PERP".to_string(),
            base_asset: "BTC".to_string(),
            quote_asset: "USD".to_string(),
            mark_price: Decimal::from(67500000000000000000000i64),
            index_price: Decimal::from(67500000000000000000000i64),
            last_price: Decimal::from(67500000000000000000000i64),
            funding_rate: Decimal::ZERO,
            funding_timestamp: Utc::now().timestamp(),
            next_funding_time: Utc::now().timestamp() + 28800,
            max_leverage: 50,
            max_position_size: Decimal::from(10_000_000),
            maintenance_margin_rate: Decimal::from(50000),
            initial_margin_rate: Decimal::from(100000),
            is_active: true,
            is_paused: false,
        },
        Market {
            id: "SOL-PERP".to_string(),
            symbol: "SOL-PERP".to_string(),
            base_asset: "SOL".to_string(),
            quote_asset: "USD".to_string(),
            mark_price: Decimal::from(145000000000000000i64),
            index_price: Decimal::from(145000000000000000i64),
            last_price: Decimal::from(145000000000000000i64),
            funding_rate: Decimal::ZERO,
            funding_timestamp: Utc::now().timestamp(),
            next_funding_time: Utc::now().timestamp() + 28800,
            max_leverage: 50,
            max_position_size: Decimal::from(5_000_000),
            maintenance_margin_rate: Decimal::from(50000),
            initial_margin_rate: Decimal::from(100000),
            is_active: true,
            is_paused: false,
        },
    ];
    
    for market in markets {
        engine.add_market(market);
    }
}
