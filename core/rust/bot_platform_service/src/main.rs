//! Bot Platform Service Binary

use bot_platform_service::{
    BotManager, BotType, BotStatus, BotInfo,
    GridConfig, MarketMakingConfig, ArbitrageConfig, SniperConfig,
};

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    println!("TigerSwap Bot Platform Service");
    println!("=====================");
    
    // Initialize bot manager
    let mut manager = BotManager::new();
    
    println!("Bot Manager initialized");
    
    // Create grid bot
    println!("\nCreating Grid Trading Bot...");
    let grid_config = GridConfig::new(10, 0.01, 100.0);
    let grid_bot = bot_platform_service::grid::GridBot::new("grid-1".to_string(), grid_config);
    
    manager.register(BotInfo {
        bot_id: grid_bot.bot_id.clone(),
        bot_type: BotType::Grid,
        status: BotStatus::Stopped,
        pnl: 0.0,
    });
    println!("Created Grid Bot with {} levels", grid_bot.levels.len());
    
    // Create market making bot
    println!("\nCreating Market Making Bot...");
    let mm_config = MarketMakingConfig::new(0.001, 1.0);
    let mm_bot = bot_platform_service::market_making::MarketMakingBot::new("mm-1".to_string(), mm_config);
    let (bid, ask) = mm_bot.calculate_quotes(100.0);
    
    manager.register(BotInfo {
        bot_id: mm_bot.bot_id.clone(),
        bot_type: BotType::MarketMaking,
        status: BotStatus::Stopped,
        pnl: 0.0,
    });
    println!("Created MM Bot with bid={:.4}, ask={:.4}", bid, ask);
    
    // Create arbitrage bot
    println!("\nCreating Arbitrage Bot...");
    let arb_config = ArbitrageConfig::new(1.0);
    let mut arb_bot = bot_platform_service::arbitrage::ArbitrageBot::new("arb-1".to_string(), arb_config);
    
    manager.register(BotInfo {
        bot_id: arb_bot.bot_id.clone(),
        bot_type: BotType::Arbitrage,
        status: BotStatus::Stopped,
        pnl: 0.0,
    });
    println!("Created Arbitrage Bot");
    
    // Create sniper bot
    println!("\nCreating Sniper Bot...");
    let sniper_config = SniperConfig::new(vec!["0xnew_token".to_string()]);
    let sniper_bot = bot_platform_service::sniper::SniperBot::new("sniper-1".to_string(), sniper_config);
    
    manager.register(BotInfo {
        bot_id: sniper_bot.bot_id.clone(),
        bot_type: BotType::Sniper,
        status: BotStatus::Stopped,
        pnl: 0.0,
    });
    println!("Created Sniper Bot for {} tokens", sniper_bot.config.target_tokens.len());
    
    // List all bots
    println!("\nListing all bots...");
    let bots = manager.list();
    for bot in &bots {
        println!("- {} ({:?}) - {:?}", bot.bot_id, bot.bot_type, bot.status);
    }
    
    println!("\nBot Platform Service ready!");
    
    Ok(())
}