//! Rate Limiter Module
//! 
//! Rate limiting and abuse prevention for the DEX API

use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Duration, Instant};
use tokio::sync::RwLock;
use thiserror::Error;

#[derive(Error, Debug)]
pub enum RateLimitError {
    #[error("Rate limit exceeded. Try again in {0} seconds")]
    RateLimitExceeded(u64),
    #[error("Invalid request")]
    InvalidRequest,
}

/// Rate limit configuration
#[derive(Debug, Clone)]
pub struct RateLimitConfig {
    pub max_requests: u32,
    pub window_seconds: u64,
    pub burst_size: u32,
    /// Upper bound on the number of tracked identifiers.
    ///
    /// The identifier is caller-controlled (typically a client IP), so without a
    /// bound a client that rotates its key grows the map without limit and
    /// exhausts memory. When the map is full the least recently active entry is
    /// evicted, which only ever resets a bucket that is not currently being
    /// used.
    pub max_identifiers: usize,
}

impl Default for RateLimitConfig {
    fn default() -> Self {
        Self {
            max_requests: 100,
            window_seconds: 60,
            burst_size: 10,
            max_identifiers: 100_000,
        }
    }
}

/// Token bucket rate limiter
pub struct TokenBucket {
    tokens: f64,
    max_tokens: f64,
    refill_rate: f64,
    last_refill: Instant,
}

impl TokenBucket {
    pub fn new(max_tokens: u32, refill_per_second: f64) -> Self {
        let tokens = max_tokens as f64;
        Self {
            tokens,
            max_tokens: tokens,
            refill_rate: refill_per_second,
            last_refill: Instant::now(),
        }
    }
    
    /// Try to consume tokens, returns true if successful
    pub fn try_consume(&mut self, tokens: u32) -> bool {
        self.refill();
        
        if self.tokens >= tokens as f64 {
            self.tokens -= tokens as f64;
            true
        } else {
            false
        }
    }
    
    /// Refill tokens based on elapsed time
    fn refill(&mut self) {
        let now = Instant::now();
        let elapsed = now.duration_since(self.last_refill);
        let tokens_to_add = elapsed.as_secs_f64() * self.refill_rate;
        
        self.tokens = (self.tokens + tokens_to_add).min(self.max_tokens);
        self.last_refill = now;
    }
    
    /// Get remaining tokens
    pub fn remaining(&self) -> u32 {
        self.tokens as u32
    }

    /// True once the bucket has refilled to its ceiling, so the entry can be
    /// dropped without changing behaviour.
    fn is_idle(&self) -> bool {
        self.tokens >= self.max_tokens
    }
}

/// Rate limiter for API endpoints
pub struct RateLimiter {
    buckets: Arc<RwLock<HashMap<String, TokenBucket>>>,
    config: RateLimitConfig,
}

impl RateLimiter {
    pub fn new(config: RateLimitConfig) -> Self {
        Self {
            buckets: Arc::new(RwLock::new(HashMap::new())),
            config,
        }
    }
    
    /// Check if a request is allowed
    pub async fn check(&self, identifier: &str) -> Result<(), RateLimitError> {
        let mut buckets = self.buckets.write().await;
        if !buckets.contains_key(identifier) {
            self.make_room(&mut buckets);
        }
        
        let bucket = buckets.entry(identifier.to_string()).or_insert_with(|| {
            let refill_rate = self.config.max_requests as f64 / self.config.window_seconds as f64;
            TokenBucket::new(self.config.burst_size, refill_rate)
        });
        
        if bucket.try_consume(1) {
            Ok(())
        } else {
            // Calculate retry after time
            let retry_after = (self.config.window_seconds as f64 * 
                (1.0 - bucket.remaining() as f64 / self.config.max_requests as f64)) as u64;
            Err(RateLimitError::RateLimitExceeded(retry_after.max(1)))
        }
    }
    
    /// Frees space for a new identifier once the map is at capacity.
    ///
    /// Fully refilled buckets are dropped first because they no longer carry any
    /// state. If every bucket is still draining, the least recently used one is
    /// evicted so the map cannot grow past `max_identifiers`.
    fn make_room(&self, buckets: &mut HashMap<String, TokenBucket>) {
        let limit = self.config.max_identifiers.max(1);
        if buckets.len() < limit {
            return;
        }

        let idle: Vec<String> = buckets
            .iter()
            .filter(|(_, bucket)| bucket.is_idle())
            .map(|(key, _)| key.clone())
            .collect();
        for key in idle {
            buckets.remove(&key);
        }

        while buckets.len() >= limit {
            let oldest = buckets
                .iter()
                .min_by_key(|(_, bucket)| bucket.last_refill)
                .map(|(key, _)| key.clone());
            match oldest {
                Some(key) => {
                    buckets.remove(&key);
                }
                None => break,
            }
        }
    }

    /// Get remaining requests for an identifier
    pub async fn remaining(&self, identifier: &str) -> u32 {
        let buckets = self.buckets.read().await;
        
        if let Some(bucket) = buckets.get(identifier) {
            bucket.remaining()
        } else {
            self.config.burst_size
        }
    }
    
    /// Reset rate limit for an identifier
    pub async fn reset(&self, identifier: &str) {
        let mut buckets = self.buckets.write().await;
        buckets.remove(identifier);
    }

    /// Number of identifiers currently tracked, for capacity checks.
    pub async fn bucket_count(&self) -> usize {
        self.buckets.read().await.len()
    }
}

/// Sliding window rate limiter
pub struct SlidingWindowRateLimiter {
    requests: Arc<RwLock<HashMap<String, Vec<Instant>>>>,
    max_requests: u32,
    window: Duration,
    /// Upper bound on tracked identifiers; the identifier is caller-controlled,
    /// so an unbounded map is a memory-exhaustion vector.
    max_identifiers: usize,
}

impl SlidingWindowRateLimiter {
    pub fn new(max_requests: u32, window: Duration) -> Self {
        Self {
            requests: Arc::new(RwLock::new(HashMap::new())),
            max_requests,
            window,
            max_identifiers: 100_000,
        }
    }

    /// Overrides the maximum number of tracked identifiers.
    pub fn with_max_identifiers(mut self, max_identifiers: usize) -> Self {
        self.max_identifiers = max_identifiers.max(1);
        self
    }
    
    /// Number of identifiers currently tracked, for capacity checks.
    pub async fn tracked_identifiers(&self) -> usize {
        self.requests.read().await.len()
    }

    pub async fn check(&self, identifier: &str) -> Result<(), RateLimitError> {
        let mut requests = self.requests.write().await;
        let now = Instant::now();
        
        if !requests.contains_key(identifier) {
            // Drop windows that have fully drained, then the least recently
            // touched one, so the map cannot grow past `max_identifiers`.
            if requests.len() >= self.max_identifiers {
                requests.retain(|_, stamps| {
                    stamps.retain(|&t| now.duration_since(t) < self.window);
                    !stamps.is_empty()
                });
            }
            while requests.len() >= self.max_identifiers {
                let oldest = requests
                    .iter()
                    .min_by_key(|(_, stamps)| stamps.last().copied())
                    .map(|(key, _)| key.clone());
                match oldest {
                    Some(key) => {
                        requests.remove(&key);
                    }
                    None => break,
                }
            }
        }

        let timestamps = requests.entry(identifier.to_string()).or_insert_with(Vec::new);
        
        // Remove old timestamps
        timestamps.retain(|&t| now.duration_since(t) < self.window);
        
        if (timestamps.len() as u32) < self.max_requests {
            timestamps.push(now);
            Ok(())
        } else {
            let retry_after = if let Some(oldest) = timestamps.first() {
                (self.window - now.duration_since(*oldest)).as_secs()
            } else {
                self.window.as_secs()
            };
            
            Err(RateLimitError::RateLimitExceeded(retry_after.max(1)))
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::thread;
    use std::time::Duration as StdDuration;
    
    #[test]
    fn test_token_bucket() {
        let mut bucket = TokenBucket::new(10, 2.0);
        
        // Should allow first 10 requests
        for _ in 0..10 {
            assert!(bucket.try_consume(1));
        }
        
        // Should fail 11th request
        assert!(!bucket.try_consume(1));
        
        // Wait for refill
        thread::sleep(StdDuration::from_millis(600));
        
        // Should allow more requests
        assert!(bucket.try_consume(1));
    }
    
    #[tokio::test]
    async fn test_rate_limiter() {
        let limiter = RateLimiter::new(RateLimitConfig {
            max_requests: 5,
            window_seconds: 60,
            burst_size: 5,
            max_identifiers: 1000,
        });
        
        for _ in 0..5 {
            assert!(limiter.check("test_user").await.is_ok());
        }
        
        assert!(limiter.check("test_user").await.is_err());
    }

    /// The identifier is caller-controlled, so the bucket map must not grow
    /// without limit. With a capacity of two, a third identifier evicts the
    /// least recently used one instead of allocating a new entry forever.
    #[tokio::test]
    async fn test_rate_limiter_bounds_identifier_count() {
        let limiter = RateLimiter::new(RateLimitConfig {
            max_requests: 5,
            window_seconds: 60,
            burst_size: 5,
            max_identifiers: 2,
        });

        assert!(limiter.check("a").await.is_ok());
        assert!(limiter.check("b").await.is_ok());
        assert!(limiter.check("c").await.is_ok());

        assert_eq!(limiter.bucket_count().await, 2, "map is capped at max_identifiers");
        // "a" was evicted, so it starts from a full bucket again.
        assert_eq!(limiter.remaining("a").await, 5);
        assert!(limiter.remaining("c").await < 5, "the newest key keeps its usage");
    }

    #[tokio::test]
    async fn test_sliding_window_bounds_identifier_count() {
        let limiter = SlidingWindowRateLimiter::new(5, Duration::from_secs(60))
            .with_max_identifiers(2);

        for key in ["a", "b", "c", "d"] {
            assert!(limiter.check(key).await.is_ok());
        }

        assert_eq!(limiter.tracked_identifiers().await, 2);
    }
}
