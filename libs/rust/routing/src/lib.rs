//! TigerSwap Routing Engine - High-Performance DEX Aggregator Router
//! 
//! This is the heart of the DEX aggregator - finds optimal swap paths
//! across multiple DEXs with support for multi-hop routing and split routes.

mod graph;
mod dijkstra;
mod pool;
mod route;
mod allocator;

pub use graph::RoutingGraph;
pub use pool::{Pool, PoolKey, DexConfig, DEX_CONFIGS};
pub use route::{Route, RouteStep, SplitRoute, QuoteRequest, QuoteResult};
pub use allocator::RouteAllocator;
pub use dijkstra::DijkstraRouter;

/// Maximum number of hops in a route
pub const MAX_HOPS: usize = 3;
/// Maximum number of split routes
pub const MAX_SPLITS: usize = 3;
/// Default gas price in wei (30 gwei)
pub const DEFAULT_GAS_PRICE: u64 = 30_000_000_000;
/// Default native price in USD
pub const DEFAULT_NATIVE_PRICE_USD: f64 = 2000.0;

/// Fee tiers in basis points
pub mod fee {
    pub const UNISWAP_V2: u32 = 300;
    pub const UNISWAP_V3: u32 = 500;
    pub const SUSHISWAP: u32 = 300;
    pub const PANCAKESWAP: u32 = 200;
    pub const QUICKSWAP: u32 = 300;
    pub const CURVE_STABLE: u32 = 40;
    pub const CURVE_VOLATILE: u32 = 400;
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_pool_creation() {
        let pool = Pool::new(
            "0x1234".to_string(),
            "0xab".to_string(),
            "0xcd".to_string(),
            1_000_000u64.into(),
            1_000_000u64.into(),
            fee::UNISWAP_V2,
            "uniswap_v2".to_string(),
        );
        assert_eq!(pool.fee_bps, fee::UNISWAP_V2);
    }

    #[test]
    fn test_route_calculation() {
        let mut graph = RoutingGraph::new();
        
        // Add test pools
        let pool1 = Pool::new(
            "tokenA".to_string(),
            "tokenB".to_string(),
            "0xpool1".to_string(),
            10_000_000u64.into(),
            10_000_000u64.into(),
            fee::UNISWAP_V2,
            "uniswap_v2".to_string(),
        );
        graph.add_pool(pool1);
        
        let request = QuoteRequest {
            token_in: "tokenA".to_string(),
            token_out: "tokenB".to_string(),
            amount_in: 1_000_000u64.into(),
            slippage_bps: 50,
            gas_price: DEFAULT_GAS_PRICE,
            native_price_usd: DEFAULT_NATIVE_PRICE_USD,
            max_hops: MAX_HOPS,
        };
        
        let result = graph.find_best_route(&request);
        assert!(result.best_route.is_some());
    }
}