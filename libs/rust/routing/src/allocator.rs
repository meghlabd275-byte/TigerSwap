//! Route allocator - distributes trades across multiple routes for best execution

use crate::route::{Route, SplitRoute, QuoteRequest};
use num_bigint::BigUint;

/// Route allocator for split orders
pub struct RouteAllocator {
    max_splits: usize,
    min_improvement_bps: u32,
}

impl RouteAllocator {
    pub fn new() -> Self {
        Self {
            max_splits: 3,
            min_improvement_bps: 5, // Require at least 0.05% improvement
        }
    }

    /// Calculate optimal split percentages
    pub fn calculate_splits(
        &self,
        routes: &[Route],
        total_amount_in: &BigUint,
    ) -> Option<SplitRoute> {
        if routes.len() < 2 || routes.len() > self.max_splits {
            return None;
        }

        // Simple equal weight split (50/50 for two routes)
        // In production, would use more sophisticated optimization
        let percentages = self.equal_split(routes.len());
        
        let split_routes: Vec<Route> = routes.iter().take(self.max_splits).cloned().collect();
        
        let split = SplitRoute::new(split_routes, percentages);
        
        // Check if split improves over single route
        if self.is_split_better(&split, &routes[0]) {
            Some(split)
        } else {
            None
        }
    }

    /// Calculate equal split percentages
    fn equal_split(&self, count: usize) -> Vec<u32> {
        let base = 100 / count as u32;
        let remainder = 100 - base * count as u32;
        
        (0..count)
            .map(|i| if i == 0 { base + remainder } else { base })
            .collect()
    }

    /// Check if split is better than single route
    fn is_split_better(&self, split: &SplitRoute, best_route: &Route) -> bool {
        let improvement = if best_route.total_amount_out == BigUint::from(0u64) {
            0.0
        } else {
            let split_out = split.total_amount_out.to_f64().unwrap_or(0.0);
            let best_out = best_route.total_amount_out.to_f64().unwrap_or(1.0);
            ((split_out - best_out) / best_out * 10000.0) as u32
        };
        
        improvement >= self.min_improvement_bps
    }

    /// Allocate amount across routes based on percentages
    pub fn allocate(&self, route: &Route, percentages: &[u32]) -> Vec<(Route, BigUint)> {
        let mut result = Vec::new();
        let total_pct: u32 = percentages.iter().sum();
        
        for (i, _) in percentages.iter().enumerate() {
            if i < route.steps.len() {
                let pct = percentages[i] as f64 / total_pct as f64;
                let amount = (route.total_amount_out.clone() * BigUint::from((pct * 1000.0) as u64)) / BigUint::from(1000u64);
                result.push((route.clone(), amount));
            }
        }
        
        result
    }

    /// Set max splits
    pub fn with_max_splits(mut self, splits: usize) -> Self {
        self.max_splits = splits;
        self
    }

    /// Set minimum improvement threshold
    pub fn with_min_improvement(mut self, bps: u32) -> Self {
        self.min_improvement_bps = bps;
        self
    }
}

impl Default for RouteAllocator {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_equal_split() {
        let allocator = RouteAllocator::new();
        let split = allocator.equal_split(2);
        assert_eq!(split, vec![50, 50]);
        
        let split3 = allocator.equal_split(3);
        assert_eq!(split3, vec![34, 33, 33]);
    }

    #[test]
    fn test_allocator_creation() {
        let allocator = RouteAllocator::new();
        assert_eq!(allocator.max_splits, 3);
        assert_eq!(allocator.min_improvement_bps, 5);
    }
}