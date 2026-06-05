//! Core routing graph that maintains pools and finds optimal paths

use crate::pool::{Pool, PoolKey};
use crate::route::{QuoteRequest, QuoteResult, Route, RouteStep, SplitRoute};
use crate::dijkstra::DijkstraRouter;
use crate::fee;
use ahash::AHashMap;
use num_bigint::BigUint;
use parking_lot::RwLock;
use std::sync::Arc;

/// Common base tokens by chain ID (used for multi-hop routing)
const COMMON_BASE_TOKENS: &[&str] = &[
    "0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2", // WETH
    "0xdAC17F958D2ee523a2206206994597C13D831ec7", // USDT
    "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", // USDC
    "0x6B175474E89094C44Da98b954EedeAC495271d0F", // DAI
];

/// Thread-safe routing graph
pub struct RoutingGraph {
    /// Pools indexed by token pair
    pools_by_pair: RwLock<AHashMap<(String, String), Vec<Pool>>>,
    /// All pools indexed by address
    pools_by_address: RwLock<AHashMap<String, Pool>>,
    /// Token adjacency list for fast lookup
    adjacency: RwLock<AHashMap<String, AHashMap<String, Vec<Pool>>>>,
    /// Dijkstra router for pathfinding
    dijkstra: DijkstraRouter,
}

impl RoutingGraph {
    /// Create a new routing graph
    pub fn new() -> Self {
        Self {
            pools_by_pair: RwLock::new(AHashMap::default()),
            pools_by_address: RwLock::new(AHashMap::default()),
            adjacency: RwLock::new(AHashMap::default()),
            dijkstra: DijkstraRouter::new(),
        }
    }

    /// Add a pool to the graph
    pub fn add_pool(&self, pool: Pool) {
        let address = pool.address.clone();
        let token0 = pool.token0.clone();
        let token1 = pool.token1.clone();

        // Add to address index
        {
            let mut by_address = self.pools_by_address.write();
            by_address.insert(address.clone(), pool.clone());
        }

        // Normalize pair key (alphabetically ordered)
        let pair_key = Self::pair_key(&token0, &token1);
        
        // Add to pair index
        {
            let mut by_pair = self.pools_by_pair.write();
            let pools = by_pair.entry(pair_key.clone()).or_insert_with(Vec::new);
            // Replace or add pool
            if let Some(existing) = pools.iter_mut().find(|p| p.address == address) {
                *existing = pool.clone();
            } else {
                pools.push(pool.clone());
            }
        }

        // Update adjacency list
        {
            let mut adj = self.adjacency.write();
            adj.entry(token0.clone())
                .or_insert_with(AHashMap::new)
                .entry(token1.clone())
                .or_insert_with(Vec::new)
                .push(pool.clone());
            
            // Add reverse direction
            adj.entry(token1.clone())
                .or_insert_with(AHashMap::new)
                .entry(token0.clone())
                .or_insert_with(Vec::new)
                .push(pool);
        }
    }

    /// Remove a pool from the graph
    pub fn remove_pool(&self, address: &str) {
        let pool = {
            let by_address = self.pools_by_address.read();
            by_address.get(address).cloned()
        };

        if let Some(pool) = pool {
            let pair_key = Self::pair_key(&pool.token0, &pool.token1);
            
            // Remove from pair index
            {
                let mut by_pair = self.pools_by_pair.write();
                if let Some(pools) = by_pair.get_mut(&pair_key) {
                    pools.retain(|p| p.address != address);
                }
            }

            // Remove from adjacency
            {
                let mut adj = self.adjacency.write();
                if let Some(token_map) = adj.get_mut(&pool.token0) {
                    if let Some(pools) = token_map.get_mut(&pool.token1) {
                        pools.retain(|p| p.address != address);
                    }
                }
                if let Some(token_map) = adj.get_mut(&pool.token1) {
                    if let Some(pools) = token_map.get_mut(&pool.token0) {
                        pools.retain(|p| p.address != address);
                    }
                }
            }

            // Remove from address index
            let mut by_address = self.pools_by_address.write();
            by_address.remove(address);
        }
    }

    /// Get pools for a token pair
    pub fn get_pools(&self, token0: &str, token1: &str) -> Vec<Pool> {
        let pair_key = Self::pair_key(token0, token1);
        let by_pair = self.pools_by_pair.read();
        by_pair.get(&pair_key).cloned().unwrap_or_default()
    }

    /// Get all pools for a token
    pub fn get_pools_for_token(&self, token: &str) -> Vec<Pool> {
        let adj = self.adjacency.read();
        let mut pools = Vec::new();
        
        if let Some(token_map) = adj.get(token) {
            for (_, pool_list) in token_map.iter() {
                pools.extend(pool_list.iter().cloned());
            }
        }
        
        pools
    }

    /// Find the best route for a quote request
    pub fn find_best_route(&self, request: &QuoteRequest) -> QuoteResult {
        let direct_routes = self.find_direct_routes(request);
        let multi_hop_routes = self.find_multi_hop_routes(request);
        
        let mut all_routes: Vec<Route> = direct_routes
            .into_iter()
            .chain(multi_hop_routes.into_iter())
            .collect();

        // Sort by gas-adjusted output
        all_routes.sort_by(|a, b| {
            let value_a = Self::route_value(a, request.gas_price, request.native_price_usd);
            let value_b = Self::route_value(b, request.gas_price, request.native_price_usd);
            value_b.partial_cmp(&value_a).unwrap_or(std::cmp::Ordering::Equal)
        });

        let best_route = all_routes.first().cloned();
        
        let mut result = QuoteResult::new(best_route, all_routes);
        
        // Calculate split routes if beneficial
        if all_routes.len() >= 2 {
            if let Some(split) = self.calculate_split_routes(&all_routes, &request.amount_in, request.slippage_bps) {
                result = result.with_split_route(split);
            }
        }
        
        result
    }

    /// Find direct (single-hop) routes
    fn find_direct_routes(&self, request: &QuoteRequest) -> Vec<Route> {
        let pools = self.get_pools(&request.token_in, &request.token_out);
        
        let mut routes: Vec<Route> = pools
            .into_iter()
            .filter_map(|pool| {
                let amount_out = pool.get_amount_out(&request.amount_in);
                if amount_out == BigUint::from(0u64) {
                    return None;
                }

                let step = RouteStep::new(
                    pool.address.clone(),
                    request.token_in.clone(),
                    request.token_out.clone(),
                    pool.dex.clone(),
                    pool.dex.clone(),
                    pool.reserve0.clone(),
                    pool.reserve1.clone(),
                    request.amount_in.clone(),
                    amount_out.clone(),
                    pool.fee_bps,
                );

                let mut route = Route::new(vec![step]);
                route.with_slippage(request.slippage_bps);
                route.with_gas_cost(request.gas_price, request.native_price_usd);
                
                Some(route)
            })
            .collect();

        routes.sort_by(|a, b| {
            let value_a = Self::route_value(a, request.gas_price, request.native_price_usd);
            let value_b = Self::route_value(b, request.gas_price, request.native_price_usd);
            value_b.partial_cmp(&value_a).unwrap_or(std::cmp::Ordering::Equal)
        });

        routes
    }

    /// Find multi-hop routes through base tokens
    fn find_multi_hop_routes(&self, request: &QuoteRequest) -> Vec<Route> {
        let mut routes = Vec::new();
        
        for base_token in COMMON_BASE_TOKENS.iter() {
            // Skip if base token is same as input or output
            if *base_token == request.token_in || *base_token == request.token_out {
                continue;
            }

            // Find route: input -> base -> output
            let hop1_pools = self.get_pools(&request.token_in, base_token);
            let hop2_pools = self.get_pools(base_token, &request.token_out);

            if hop1_pools.is_empty() || hop2_pools.is_empty() {
                continue;
            }

            // Find best combination
            let mut best_hop1_out = BigUint::from(0u64);
            let mut best_hop1_pool: Option<&Pool> = None;

            for pool in &hop1_pools {
                let out = pool.get_amount_out(&request.amount_in);
                if out > best_hop1_out {
                    best_hop1_out = out;
                    best_hop1_pool = Some(pool);
                }
            }

            if let Some(pool1) = best_hop1_pool {
                let mut best_final_out = BigUint::from(0u64);
                let mut best_hop2_pool: Option<&Pool> = None;

                for pool2 in &hop2_pools {
                    let out = pool2.get_amount_out(&best_hop1_out);
                    if out > best_final_out {
                        best_final_out = out;
                        best_hop2_pool = Some(pool2);
                    }
                }

                if let Some(pool2) = best_hop2_pool {
                    let step1 = RouteStep::new(
                        pool1.address.clone(),
                        request.token_in.clone(),
                        base_token.to_string(),
                        pool1.dex.clone(),
                        pool1.dex.clone(),
                        pool1.reserve0.clone(),
                        pool1.reserve1.clone(),
                        request.amount_in.clone(),
                        best_hop1_out.clone(),
                        pool1.fee_bps,
                    );

                    let step2 = RouteStep::new(
                        pool2.address.clone(),
                        base_token.to_string(),
                        request.token_out.clone(),
                        pool2.dex.clone(),
                        pool2.dex.clone(),
                        pool2.reserve0.clone(),
                        pool2.reserve1.clone(),
                        best_hop1_out.clone(),
                        best_final_out.clone(),
                        pool2.fee_bps,
                    );

                    let mut route = Route::new(vec![step1, step2]);
                    route.with_slippage(request.slippage_bps);
                    route.with_gas_cost(request.gas_price, request.native_price_usd);
                    routes.push(route);
                }
            }
        }

        routes
    }

    /// Calculate split routes if beneficial
    fn calculate_split_routes(
        &self,
        routes: &[Route],
        amount_in: &BigUint,
        slippage_bps: u32,
    ) -> Option<SplitRoute> {
        if routes.len() < 2 {
            return None;
        }

        let best = &routes[0];
        let second = &routes[1];

        // Calculate 50/50 split
        let split_amount = amount_in.clone() / BigUint::from(2u64);
        
        // For simplicity, just use original amounts
        // In production, would scale routes proportionally
        
        let split_routes = vec![best.clone(), second.clone()];
        let percentages = vec![50, 50];
        
        let split = SplitRoute::new(split_routes, percentages);
        
        // Only use split if it improves output
        if split.total_amount_out > best.total_amount_out {
            Some(split)
        } else {
            None
        }
    }

    /// Calculate gas-adjusted route value
    fn route_value(route: &Route, gas_price: u64, native_price_usd: f64) -> f64 {
        let output_usd = route.total_amount_out.to_f64().unwrap_or(0.0) / 1e18;
        let gas_cost_usd = route.total_gas_fee_usd;
        output_usd - gas_cost_usd
    }

    /// Normalize pair key (alphabetically ordered)
    fn pair_key(token0: &str, token1: &str) -> (String, String) {
        if token0.to_lowercase() < token1.to_lowercase() {
            (token0.to_string(), token1.to_string())
        } else {
            (token1.to_string(), token0.to_string())
        }
    }

    /// Get pool by address
    pub fn get_pool(&self, address: &str) -> Option<Pool> {
        let by_address = self.pools_by_address.read();
        by_address.get(address).cloned()
    }

    /// Get total pool count
    pub fn pool_count(&self) -> usize {
        let by_address = self.pools_by_address.read();
        by_address.len()
    }

    /// Clear all pools
    pub fn clear(&self) {
        let mut by_pair = self.pools_by_pair.write();
        let mut by_address = self.pools_by_address.write();
        let mut adj = self.adjacency.write();
        
        by_pair.clear();
        by_address.clear();
        adj.clear();
    }
}

impl Default for RoutingGraph {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_add_and_find_pool() {
        let graph = RoutingGraph::new();
        
        let pool = Pool::new(
            "A".to_string(),
            "B".to_string(),
            "0xpool".to_string(),
            BigUint::from(1_000_000u64),
            BigUint::from(1_000_000u64),
            fee::UNISWAP_V2,
            "uniswap_v2".to_string(),
        );
        
        graph.add_pool(pool);
        
        let pools = graph.get_pools("A", "B");
        assert_eq!(pools.len(), 1);
        assert_eq!(graph.pool_count(), 1);
    }

    #[test]
    fn test_direct_route() {
        let graph = RoutingGraph::new();
        
        let pool = Pool::new(
            "A".to_string(),
            "B".to_string(),
            "0xpool".to_string(),
            BigUint::from(10_000_000u64),
            BigUint::from(10_000_000u64),
            fee::UNISWAP_V2,
            "uniswap_v2".to_string(),
        );
        
        graph.add_pool(pool);
        
        let request = QuoteRequest::new(
            "A".to_string(),
            "B".to_string(),
            BigUint::from(1000u64),
        );
        
        let result = graph.find_best_route(&request);
        assert!(result.best_route.is_some());
    }
}