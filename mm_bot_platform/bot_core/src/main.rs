// TigerSwap MM Bot - Market Making Bot Platform
// Ultra-low latency Rust implementation for competitive DEX trading

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::time::{SystemTime, UNIX_EPOCH};

// ============================================================================
// TOP 20 DEXs (by trading volume & speed - matching competitors like Uniswap, Hyperliquid)
// ============================================================================
pub const TOP_DEXS: [&str; 20] = [
    "uniswap_v4",        // Ethereum - Highest volume
    "uniswap_v3",        // Ethereum, Arbitrum, etc.
    "pancakeswap_v4",    // BNB Chain
    "curve_finance",     // Stablecoin specialist
    "sushiswap",         // Multi-chain
    "hyperliquid",       // Perpetuals leader
    "dydx_v4",           // Perpetuals orderbook
    "jupiter",           // Solana aggregator
    "raydium",           // Solana AMM
    "orca",              // Solana concentrated liquidity
    "balancer_v2",       // Weighted pools
    "1inch",             // Aggregator
    "odos",              // Aggregator
    "maverick",          // Movement AMM
    "velodrome_v3",      // Optimism
    "aerodrome",         // Base
    "odos_aggregator",   // Multi-chain
    "woofi",             // Cross-chain
    "spirit_swap",       // Fantom
    "spookyswap",        // Fantom
];

// ============================================================================
// TOP 200 CEXs (major centralized exchanges)
pub const TOP_CEXS: [&str; 200] = [
    "binance", "coinbase", "kraken", "okx", "bybit", "kucoin", "htx", "gateio",
    "bitget", "mexc", "binance_us", "crypto_com", "lbank", "bitmart", "bitex",
    "cryptology", "luno", "valr", "bit2c", "koinearth", "bitso", "btcmex",
    "coinex", "whitebit", "hotcoin", "bitrue", "pex", "digifinex", "bitbank",
    "fmfw", "bitforex", "oceanex", "zbg", "tidex", "btcbox", "btcturk",
    "coinw", "indodax", "probit", "bitinka", "latoken", "btcusd", "vinex",
    "exmo", "coinbene", "stex", "crex24", "safe_coin", "dsx", "localbitcoins",
    "acx", "aax", "aofg", "bequant", "bigONE", "bitci", "bithumb", "bitmas",
    "bitmax", "bitopro", "bitsdaq", "bkex", "blofin", "cex", "chainex",
    "chipmixer", "clf", "cmc", "cob", "coinall", "coineal", "coinfield",
    "coingi", "coinlist", "coinmate", "coinmetro", "coinsbit", "cointiger",
    "cryptobadge", "cryptocom", "cryptoforce", "depo", "deribit", "digifinex",
    "drift", "dxcm", "emirex", "enclave", "eternal", "excambior", "exio",
    "fasset", "finexbox", "ftx", "gbg", "gemini", "hbtc", "hks", "hkcex",
    "huobi", "idex", "ifiny", "incor", "iohk", "joy", "joytec", "kanga",
    "kann", "kersa", "kiyoung", "koyn", "kraken", "kuna", "latoken",
    "liquid", "luno", "lykke", "mercado", "mercadobitcoin", "mx", "nak",
    "nbc", "nexo", "nocks", "novadax", "nt", "oceanswap", "okcoin",
    "okex", "otc", "paymium", "pex", "pika", "poloniex", "qtrade",
    "quadency", "ripio", "safecoin", "satoshi", "simplex", "simex",
    "slex", "southxchange", "stacker", "stream", "sistemkoin", "taiko",
    "terr", "texit", "theRock", "tidex", "timex", "tokerextract", "trezor",
    "trubit", "txbit", "ubt", "uncjy", "uphold", "usd", "utorg",
    "valr", "vcc", "virgo", "wazirx", "whirl", "wings", "xbtpro",
    "xt", "yobit", "za", "zbg", "zb", "zeon", "zipmex", "zonda",
];

// ============================================================================
// Bot Types (All strategies)
// ============================================================================
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum BotType {
    MarketMaker,      // Standard MM - earn spread
    Arbitrage,        // Cross-exchange/arbitrage
    Sniper,           // Fast trade execution
    Liquidity,        // Provide liquidity
    FrontRun,         // Anticipate large trades (MEV)
    MevBot,           // MEV extraction
    Sandwich,        // Sandwich attacks
    FlashLoan,       // Flash loan strategies
    CrossChain,       // Bridge arbitrage
    PerpHedge,        // Perpetual hedging
}

impl BotType {
    pub fn as_str(&self) -> &'static str {
        match self {
            BotType::MarketMaker => "Market Maker",
            BotType::Arbitrage => "Arbitrage",
            BotType::Sniper => "Sniper",
            BotType::Liquidity => "Liquidity Provider",
            BotType::FrontRun => "Front Run",
            BotType::MevBot => "MEV Bot",
            BotType::Sandwich => "Sandwich",
            BotType::FlashLoan => "Flash Loan",
            BotType::CrossChain => "Cross-Chain",
            BotType::PerpHedge => "Perpetual Hedge",
        }
    }
}

// ============================================================================
// Fee Configuration
// ============================================================================
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FeeConfig {
    pub monthly_fee_usdt: f64,
    pub per_exchange_fee_usdt: f64,
    pub description: String,
}

impl FeeConfig {
    pub fn new(bot_type: BotType) -> Self {
        match bot_type {
            BotType::MarketMaker => FeeConfig {
                monthly_fee_usdt: 5000.0,
                per_exchange_fee_usdt: 1000.0,
                description: "Market Maker Bot - $5000/month + $1000 per exchange".to_string(),
            },
            _ => FeeConfig {
                monthly_fee_usdt: 2500.0,
                per_exchange_fee_usdt: 500.0,
                description: "Standard Bot - $2500/month + $500 per exchange".to_string(),
            }
        }
    }
    
    pub fn total_fee(&self, num_exchanges: u32) -> f64 {
        self.monthly_fee_usdt + (self.per_exchange_fee_usdt * num_exchanges as f64)
    }
}

// ============================================================================
// Strategy Configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct StrategyConfig {
    pub id: String,
    pub name: String,
    pub bot_type: BotType,
    pub pair: (String, String),
    pub chain_id: u32,
    pub dexes: Vec<String>,
    pub cexes: Vec<String>,
    pub enabled: bool,
    pub base_spread_bps: u32,
    pub spread_adjustment: f64,
    pub max_spread_bps: u32,
    pub min_spread_bps: u32,
    pub inventory_balance_limit: f64,
    pub inventory_skew_threshold: f64,
    pub order_size_min: f64,
    pub order_size_max: f64,
    pub max_orders_per_side: u32,
    pub max_position_usd: f64,
    pub latency_target_us: u64,  // Target latency in microseconds
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Order {
    pub id: String,
    pub side: String,
    pub pair: (String, String),
    pub price: f64,
    pub size: f64,
    pub status: String,
    pub exchange: String,
    pub created_at: i64,
    pub execution_latency_us: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BotStats {
    pub total_pnl: f64,
    pub daily_pnl: f64,
    pub total_volume: f64,
    pub filled_orders: u64,
    pub open_orders: u32,
    pub avg_execution_latency_us: u64,
    pub total_saved_fees: f64,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BotInstance {
    pub id: String,
    pub bot_type: BotType,
    pub strategies: Vec<StrategyConfig>,
    pub connected_dexes: Vec<String>,
    pub connected_cexes: Vec<String>,
    pub is_running: bool,
    pub stats: BotStats,
    pub orders: HashMap<String, Order>,
    pub fee_config: FeeConfig,
}

impl BotInstance {
    pub fn new(id: String, bot_type: BotType) -> Self {
        let fee_config = FeeConfig::new(bot_type);
        Self {
            id,
            bot_type,
            strategies: Vec::new(),
            connected_dexes: Vec::new(),
            connected_cexes: Vec::new(),
            is_running: false,
            stats: BotStats {
                total_pnl: 0.0,
                daily_pnl: 0.0,
                total_volume: 0.0,
                filled_orders: 0,
                open_orders: 0,
                avg_execution_latency_us: 0,
                total_saved_fees: 0.0,
            },
            orders: HashMap::new(),
            fee_config,
        }
    }

    pub fn add_strategy(&mut self, config: StrategyConfig) {
        self.strategies.push(config);
    }

    pub fn connect_dex(&mut self, dex: String) {
        if !self.connected_dexes.contains(&dex) {
            self.connected_dexes.push(dex);
        }
    }

    pub fn connect_cex(&mut self, cex: String) {
        if !self.connected_cexes.contains(&cex) {
            self.connected_cexes.push(cex);
        }
    }

    pub fn connect_all_top_dexs(&mut self) {
        for dex in TOP_DEXS.iter() {
            self.connect_dex(dex.to_string());
        }
    }

    pub fn connect_all_top_cexes(&mut self) {
        for cex in TOP_CEXS.iter() {
            self.connect_cex(cex.to_string());
        }
    }

    pub fn start(&mut self) {
        self.is_running = true;
    }

    pub fn stop(&mut self) {
        self.is_running = false;
    }

    pub fn calculate_spread(&self, strategy: &StrategyConfig, volatility: f64) -> f64 {
        let base = strategy.base_spread_bps as f64 / 10000.0;
        let adjusted = base + (volatility * strategy.spread_adjustment);
        adjusted.max(strategy.min_spread_bps as f64 / 10000.0)
               .min(strategy.max_spread_bps as f64 / 10000.0)
    }

    pub fn calculate_bid_price(&self, mid_price: f64, spread: f64) -> f64 {
        mid_price * (1.0 - spread)
    }

    pub fn calculate_ask_price(&self, mid_price: f64, spread: f64) -> f64 {
        mid_price * (1.0 + spread)
    }

    pub fn execute_order(&mut self, side: String, pair: (String, String), price: f64, size: f64, exchange: String) -> Order {
        let start = SystemTime::now();
        let timestamp = start.duration_since(UNIX_EPOCH).unwrap().as_millis() as i64;
        
        let order = Order {
            id: format!("{}_{}_{}", self.id, timestamp, rand_id()),
            side,
            pair: pair.clone(),
            price,
            size,
            status: "filled".to_string(),
            exchange,
            created_at: timestamp,
            execution_latency_us: 0, // Will be calculated
        };
        
        let elapsed = start.elapsed().unwrap().as_micros() as u64;
        let mut final_order = order;
        final_order.execution_latency_us = elapsed;
        
        self.stats.filled_orders += 1;
        self.stats.total_volume += size * price;
        
        // Update average latency
        let total = self.stats.avg_execution_latency_us * (self.stats.filled_orders - 1) + elapsed;
        self.stats.avg_execution_latency_us = total / self.stats.filled_orders;
        
        self.orders.insert(final_order.id.clone(), final_order.clone());
        final_order
    }
}

fn rand_id() -> String {
    use std::time::SystemTime;
    let now = SystemTime::now().duration_since(UNIX_EPOCH).unwrap();
    format!("{:x}", now.subsec_nanos())
}

// ============================================================================
// MM Bot Engine - Main orchestrator
// ============================================================================
pub struct MMBotEngine {
    pub id: String,
    pub bots: HashMap<String, Arc<RwLock<BotInstance>>>,
    pub connected_dexes: HashMap<String, bool>,
    pub connected_cexes: HashMap<String, bool>,
    pub is_running: bool,
}

impl MMBotEngine {
    pub fn new(id: String) -> Self {
        Self {
            id,
            bots: HashMap::new(),
            connected_dexes: HashMap::new(),
            connected_cexes: HashMap::new(),
            is_running: false,
        }
    }

    pub fn create_bot(&mut self, id: String, bot_type: BotType) -> Arc<RwLock<BotInstance>> {
        let bot = BotInstance::new(id.clone(), bot_type);
        let bot_arc = Arc::new(RwLock::new(bot));
        self.bots.insert(id.clone(), bot_arc.clone());
        bot_arc
    }

    pub fn create_mm_bot(&mut self, id: String) -> Arc<RwLock<BotInstance>> {
        self.create_bot(id, BotType::MarketMaker)
    }

    pub fn create_arbitrage_bot(&mut self, id: String) -> Arc<RwLock<BotInstance>> {
        self.create_bot(id, BotType::Arbitrage)
    }

    pub fn create_sniper_bot(&mut self, id: String) -> Arc<RwLock<BotInstance>> {
        self.create_bot(id, BotType::Sniper)
    }

    pub fn get_bot(&self, id: &str) -> Option<Arc<RwLock<BotInstance>>> {
        self.bots.get(id).cloned()
    }

    pub fn start_all(&mut self) {
        self.is_running = true;
        for (_, bot) in &self.bots {
            if let Ok(mut b) = bot.write() {
                b.start();
            }
        }
    }

    pub fn stop_all(&mut self) {
        self.is_running = false;
        for (_, bot) in &self.bots {
            if let Ok(mut b) = bot.write() {
                b.stop();
            }
        }
    }

    pub fn get_total_stats(&self) -> BotStats {
        let mut total = BotStats {
            total_pnl: 0.0,
            daily_pnl: 0.0,
            total_volume: 0.0,
            filled_orders: 0,
            open_orders: 0,
            avg_execution_latency_us: 0,
            total_saved_fees: 0.0,
        };

        for (_, bot) in &self.bots {
            if let Ok(b) = bot.read() {
                total.total_pnl += b.stats.total_pnl;
                total.daily_pnl += b.stats.daily_pnl;
                total.total_volume += b.stats.total_volume;
                total.filled_orders += b.stats.filled_orders;
                total.open_orders += b.stats.open_orders;
                total.total_saved_fees += b.stats.total_saved_fees;
            }
        }

        if total.filled_orders > 0 {
            let mut latency_sum: u64 = 0;
            let mut count = 0;
            for (_, bot) in &self.bots {
                if let Ok(b) = bot.read() {
                    latency_sum += b.stats.avg_execution_latency_us * b.stats.filled_orders;
                    count += b.stats.filled_orders;
                }
            }
            if count > 0 {
                total.avg_execution_latency_us = latency_sum / count;
            }
        }

        total
    }
}

// ============================================================================
// Latency Performance Tracker
// ============================================================================
#[derive(Debug, Clone)]
pub struct LatencyTracker {
    pub dex_latencies: HashMap<String, Vec<u64>>,
    pub cex_latencies: HashMap<String, Vec<u64>>,
}

impl LatencyTracker {
    pub fn new() -> Self {
        Self {
            dex_latencies: HashMap::new(),
            cex_latencies: HashMap::new(),
        }
    }

    pub fn record_dex_latency(&mut self, dex: &str, latency_us: u64) {
        self.dex_latencies
            .entry(dex.to_string())
            .or_insert_with(Vec::new)
            .push(latency_us);
    }

    pub fn record_cex_latency(&mut self, cex: &str, latency_us: u64) {
        self.cex_latencies
            .entry(cex.to_string())
            .or_insert_with(Vec::new)
            .push(latency_us);
    }

    pub fn get_avg_latency(&self, name: &str, is_dex: bool) -> u64 {
        let latencies = if is_dex {
            self.dex_latencies.get(name)
        } else {
            self.cex_latencies.get(name)
        };

        if let Some(lats) = latencies {
            if lats.is_empty() {
                return 0;
            }
            let sum: u64 = lats.iter().sum();
            sum / lats.len() as u64
        } else {
            0
        }
    }
}

fn main() {
    println!("===========================================");
    println!("  TigerSwap MM Bot Platform v1.0");
    println!("  Ultra-low latency trading infrastructure");
    println!("===========================================");
    println!();
    
    let mut engine = MMBotEngine::new("main_engine".to_string());
    
    // Create Market Maker Bot
    println!("[+] Creating Market Maker Bot...");
    let mm_bot = engine.create_mm_bot("mm_bot_001".to_string());
    
    // Configure MM bot with full features
    {
        let mut bot = mm_bot.write().unwrap();
        bot.connect_all_top_dexs();
        bot.connect_all_top_cexes();
        
        // Add multiple trading strategies
        let strategies = vec![
            ("ETH/USDT", "uniswap_v4", 1),
            ("BTC/USDT", "pancakeswap_v4", 56),
            ("SOL/USDT", "jupiter", 101),
            ("ETH/USDT", "hyperliquid", 42161),
        ];
        
        for (pair_str, dex, chain_id) in strategies {
            let pair: Vec<&str> = pair_str.split('/').collect();
            let strategy = StrategyConfig {
                id: format!("{}_{}", dex, pair_str.replace("/", "_")),
                name: format!("{} on {}", pair_str, dex),
                bot_type: BotType::MarketMaker,
                pair: (pair[0].to_string(), pair[1].to_string()),
                chain_id,
                dexes: vec![dex.to_string()],
                cexes: vec![],
                enabled: true,
                base_spread_bps: 50,
                spread_adjustment: 0.5,
                max_spread_bps: 200,
                min_spread_bps: 10,
                inventory_balance_limit: 50000.0,
                inventory_skew_threshold: 0.3,
                order_size_min: 100.0,
                order_size_max: 10000.0,
                max_orders_per_side: 5,
                max_position_usd: 100000.0,
                latency_target_us: 1000, // 1ms target
            };
            bot.add_strategy(strategy);
        }
        
        bot.start();
        println!("  -> Connected to {} DEXs", bot.connected_dexes.len());
        println!("  -> Connected to {} CEXs", bot.connected_cexes.len());
        println!("  -> Monthly Fee: ${:.0}", bot.fee_config.monthly_fee_usdt);
        println!("  -> Per Exchange Fee: ${:.0}", bot.fee_config.per_exchange_fee_usdt);
    }
    
    // Create Arbitrage Bot
    println!("\n[+] Creating Arbitrage Bot...");
    let arb_bot = engine.create_arbitrage_bot("arb_bot_001".to_string());
    {
        let mut bot = arb_bot.write().unwrap();
        bot.connect_all_top_dexs();
        bot.connect_all_top_cexes();
        bot.start();
        println!("  -> Connected to {} DEXs", bot.connected_dexes.len());
        println!("  -> Connected to {} CEXs", bot.connected_cexes.len());
    }
    
    // Create Sniper Bot
    println!("\n[+] Creating Sniper Bot...");
    let sniper_bot = engine.create_sniper_bot("sniper_bot_001".to_string());
    {
        let mut bot = sniper_bot.write().unwrap();
        bot.connect_all_top_dexs();
        bot.connect_all_top_cexes();
        bot.start();
        println!("  -> Low latency target: <500μs");
    }
    
    engine.start_all();
    
    // Print summary
    println!("\n===========================================");
    println!("  Engine Summary");
    println!("===========================================");
    println!("  Total Bots: {}", engine.bots.len());
    println!("  Supported DEXs: {}", TOP_DEXS.len());
    println!("  Supported CEXs: {}", TOP_CEXS.len());
    println!("  Bot Types: {:?}", BotType::iter_all());
    
    let stats = engine.get_total_stats();
    println!("  Total Volume: ${:.2}", stats.total_volume);
    println!("  Total Orders: {}", stats.filled_orders);
    println!("  Avg Latency: {}μs", stats.avg_execution_latency_us);
    println!("===========================================");
    println!("  FEE SCHEDULE:");
    println!("  - Market Maker Bot: $5000/month");
    println!("  - Per Exchange Fee: $1000/month");
    println!("  - Other Bots: $2500/month + $500/exchange");
    println!("===========================================");
}

impl BotType {
    pub fn iter_all() -> Vec<&'static str> {
        vec![
            "Market Maker",
            "Arbitrage",
            "Sniper",
            "Liquidity",
            "Front Run",
            "MEV Bot",
            "Sandwich",
            "Flash Loan",
            "Cross-Chain",
            "Perpetual Hedge",
        ]
    }
}