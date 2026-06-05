//! Dijkstra-based routing algorithm for optimal path finding

use crate::pool::Pool;
use crate::route::{Route, RouteStep};
use ahash::AHashMap;
use num_bigint::BigUint;
use std::cmp::Ordering;
use std::collections::BinaryHeap;

/// Edge in the routing graph
#[derive(Clone)]
struct Edge {
    pool: Pool,
    amount_out: BigUint,
    price_impact: f64,
}

/// State for Dijkstra search
#[derive(Clone)]
struct DijkstraState {
    token: String,
    amount_out: BigUint,
    gas_cost: u64,
    path: Vec<RouteStep>,
}

impl DijkstraState {
    fn new(token: String) -> Self {
        Self {
            token,
            amount_out: BigUint::from(0u64),
            gas_cost: 0,
            path: Vec::new(),
        }
    }
}

impl Ord for DijkstraState {
    fn cmp(&self, other: &Self) -> Ordering {
        // Compare by amount out (higher is better, so reverse)
        other.amount_out.cmp(&self.amount_out)
    }
}

impl PartialOrd for DijkstraState {
    fn partial_cmp(&self, other: &Self) -> Option<Ordering> {
        Some(self.cmp(other))
    }
}

impl Eq for DijkstraState {}
impl PartialEq for DijkstraState {
    fn eq(&self, other: &Self) -> bool {
        self.token == other.token && self.amount_out == other.amount_out
    }
}

/// Dijkstra router for finding optimal swap paths
pub struct DijkstraRouter {
    max_hops: usize,
    gas_per_hop: u64,
}

impl DijkstraRouter {
    pub fn new() -> Self {
        Self {
            max_hops: 3,
            gas_per_hop: 150000,
        }
    }

    /// Find optimal path using Dijkstra's algorithm
    pub fn find_path(
        &self,
        start_token: &str,
        end_token: &str,
        amount_in: &BigUint,
        pools: &AHashMap<String, Vec<Pool>>,
        visited: &mut AHashMap<String, bool>,
    ) -> Option<Route> {
        let mut heap = BinaryHeap::new();
        
        // Initialize with starting state
        let start = DijkstraState::new(start_token.to_string());
        heap.push(start);

        while let Some(current) = heap.pop() {
            // Check if we reached the destination
            if current.token.to_lowercase() == end_token.to_lowercase() {
                return Some(Route::new(current.path));
            }

            // Check max hops
            if current.path.len() >= self.max_hops {
                continue;
            }

            // Get adjacent pools
            if let Some(adjacent_pools) = pools.get(&current.token) {
                for pool in adjacent_pools {
                    // Determine direction
                    let (token_in, token_out, reserve_in, reserve_out) = if pool.token0.to_lowercase() == current.token.to_lowercase() {
                        (pool.token0.clone(), pool.token1.clone(), pool.reserve0.clone(), pool.reserve1.clone())
                    } else {
                        (pool.token1.clone(), pool.token0.clone(), pool.reserve1.clone(), pool.reserve0.clone())
                    };

                    // Skip if already visited this pool
                    if visited.contains_key(&pool.address) {
                        continue;
                    }

                    // Calculate amount out
                    let input_amount = if current.path.is_empty() {
                        amount_in.clone()
                    } else {
                        current.amount_out.clone()
                    };

                    let amount_out = self.calculate_amount_out(&pool, &input_amount, reserve_in, reserve_out);
                    
                    if amount_out == BigUint::from(0u64) {
                        continue;
                    }

                    let price_impact = self.calculate_price_impact(&pool, &input_amount, reserve_in, reserve_out, &amount_out);

                    let step = RouteStep::new(
                        pool.address.clone(),
                        token_in,
                        token_out,
                        pool.dex.clone(),
                        pool.dex.clone(),
                        reserve_in,
                        reserve_out,
                        input_amount,
                        amount_out.clone(),
                        pool.fee_bps,
                    );

                    let mut new_path = current.path.clone();
                    new_path.push(step);

                    let mut new_state = DijkstraState {
                        token: token_out,
                        amount_out,
                        gas_cost: current.gas_cost + self.gas_per_hop,
                        path: new_path,
                    };

                    heap.push(new_state);
                }
            }
        }

        None
    }

    /// Calculate amount out from a pool
    fn calculate_amount_out(
        &self,
        pool: &Pool,
        amount_in: &BigUint,
        _reserve_in: BigUint,
        _reserve_out: BigUint,
    ) -> BigUint {
        pool.get_amount_out(amount_in)
    }

    /// Calculate price impact
    fn calculate_price_impact(
        &self,
        pool: &Pool,
        amount_in: &BigUint,
        _reserve_in: BigUint,
        _reserve_out: BigUint,
        amount_out: &BigUint,
    ) -> f64 {
        pool.price_impact(amount_in)
    }

    /// Set max hops
    pub fn with_max_hops(mut self, hops: usize) -> Self {
        self.max_hops = hops;
        self
    }

    /// Set gas per hop
    pub fn with_gas_per_hop(mut self, gas: u64) -> Self {
        self.gas_per_hop = gas;
        self
    }
}

impl Default for DijkstraRouter {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_dijkstra_creation() {
        let router = DijkstraRouter::new();
        assert_eq!(router.max_hops, 3);
        assert_eq!(router.gas_per_hop, 150000);
    }

    #[test]
    fn test_dijkstra_with_custom_hops() {
        let router = DijkstraRouter::new().with_max_hops(5);
        assert_eq!(router.max_hops, 5);
    }
}