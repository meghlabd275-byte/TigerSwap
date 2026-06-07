//! Pool Module

use serde::{Deserialize, Serialize};

/// Pool Analytics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PoolAnalytics {
    pub pool_id: String,
    pub tvl: f64,
    pub volume_24h: f64,
    pub volume_7d: f64,
    pub fees_24h: f64,
    pub apy: f64,
    pub apr: f64,
    pub utilization: f64,
}

impl PoolAnalytics {
    pub fn new(pool_id: &str) -> Self {
        Self {
            pool_id: pool_id.to_string(),
            tvl: 0.0,
            volume_24h: 0.0,
            volume_7d: 0.0,
            fees_24h: 0.0,
            apy: 0.0,
            apr: 0.0,
            utilization: 0.0,
        }
    }
}

/// Pool Analytics Builder
pub struct PoolAnalyticsBuilder {
    pool_id: String,
    tvl: f64,
    volume_24h: f64,
    volume_7d: f64,
    fees_24h: f64,
    apr: f64,
    utilization: f64,
}

impl PoolAnalyticsBuilder {
    pub fn new(pool_id: &str) -> Self {
        Self {
            pool_id: pool_id.to_string(),
            tvl: 0.0,
            volume_24h: 0.0,
            volume_7d: 0.0,
            fees_24h: 0.0,
            apr: 0.0,
            utilization: 0.0,
        }
    }
    
    pub fn with_tvl(mut self, tvl: f64) -> Self {
        self.tvl = tvl;
        self
    }
    
    pub fn with_volume_24h(mut self, volume: f64) -> Self {
        self.volume_24h = volume;
        self
    }
    
    pub fn with_fees_24h(mut self, fees: f64) -> Self {
        self.fees_24h = fees;
        self
    }
    
    pub fn with_apr(mut self, apr: f64) -> Self {
        self.apr = apr;
        self
    }
    
    pub fn build(self) -> PoolAnalytics {
        let apy = if self.apr > 0.0 {
            (1.0 + self.apr / 365.0).powf(365.0) - 1.0
        } else {
            0.0
        };
        
        PoolAnalytics {
            pool_id: self.pool_id,
            tvl: self.tvl,
            volume_24h: self.volume_24h,
            volume_7d: self.volume_7d,
            fees_24h: self.fees_24h,
            apy,
            apr: self.apr,
            utilization: self.utilization,
        }
    }
}