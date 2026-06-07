//! Disaster Recovery System
//! 
//! Region failover, backup, cross-region replication, and chaos testing

use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::sync::Arc;
use parking_lot::RwLock;
use chrono::{DateTime, Utc};

/// Region status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum RegionStatus {
    Active,
    Standby,
    Failed,
    Recovering,
}

/// Region
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Region {
    pub region_id: String,
    pub name: String,
    pub status: RegionStatus,
    pub is_primary: bool,
    pub endpoint: String,
    pub health_check_url: String,
    pub last_health_check: i64,
    pub latency_ms: u64,
}

impl Region {
    pub fn new(region_id: String, name: String, is_primary: bool) -> Self {
        Self {
            region_id,
            name,
            status: RegionStatus::Standby,
            is_primary,
            endpoint: String::new(),
            health_check_url: String::new(),
            last_health_check: 0,
            latency_ms: 0,
        }
    }

    pub fn mark_active(&mut self) {
        self.status = RegionStatus::Active;
    }

    pub fn mark_failed(&mut self) {
        self.status = RegionStatus::Failed;
    }
}

/// Failover event
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FailoverEvent {
    pub event_id: String,
    pub from_region: String,
    pub to_region: String,
    pub timestamp: i64,
    pub reason: String,
    pub duration_ms: u64,
}

/// Replication status
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ReplicationStatus {
    pub region_id: String,
    pub lag_ms: u64,
    pub last_synced: i64,
    pub pending_writes: u64,
}

/// Disaster Recovery
pub struct DisasterRecovery {
    regions: Arc<RwLock<HashMap<String, Region>>>,
    failover_events: Arc<RwLock<Vec<FailoverEvent>>>,
    replication_status: Arc<RwLock<HashMap<String, ReplicationStatus>>>,
    auto_failover_enabled: bool,
}

impl DisasterRecovery {
    pub fn new() -> Self {
        Self {
            regions: Arc::new(RwLock::new(HashMap::new())),
            failover_events: Arc::new(RwLock::new(Vec::new())),
            replication_status: Arc::new(RwLock::new(HashMap::new())),
            auto_failover_enabled: true,
        }
    }

    /// Register region
    pub fn register_region(&self, region: Region) {
        let mut regions = self.regions.write();
        regions.insert(region.region_id.clone(), region);
    }

    /// Get primary region
    pub fn get_primary_region(&self) -> Option<Region> {
        let regions = self.regions.read();
        regions.values()
            .find(|r| r.is_primary && r.status == RegionStatus::Active)
            .cloned()
    }

    /// Get all active regions
    pub fn get_active_regions(&self) -> Vec<Region> {
        let regions = self.regions.read();
        regions.values()
            .filter(|r| r.status == RegionStatus::Active)
            .cloned()
            .collect()
    }

    /// Check region health
    pub fn check_region_health(&self, region_id: &str) -> bool {
        let regions = self.regions.read();
        if let Some(region) = regions.get(region_id) {
            region.status == RegionStatus::Active
        } else {
            false
        }
    }

    /// Trigger failover
    pub fn trigger_failover(&self, from_region: &str, to_region: &str, reason: &str) -> Result<FailoverEvent, String> {
        let start = Utc::now().timestamp_millis();
        
        // Update region statuses
        let mut regions = self.regions.write();
        
        if let Some(from) = regions.get_mut(from_region) {
            from.mark_failed();
        }
        
        if let Some(to) = regions.get_mut(to_region) {
            to.mark_active();
        }
        
        let event = FailoverEvent {
            event_id: uuid::Uuid::new_v4().to_string(),
            from_region: from_region.to_string(),
            to_region: to_region.to_string(),
            timestamp: start,
            reason: reason.to_string(),
            duration_ms: 0,
        };
        
        let mut events = self.failover_events.write();
        events.push(event.clone());
        
        Ok(event)
    }

    /// Update replication status
    pub fn update_replication(&self, status: ReplicationStatus) {
        let mut replication = self.replication_status.write();
        replication.insert(status.region_id.clone(), status);
    }

    /// Get replication lag
    pub fn get_replication_lag(&self, region_id: &str) -> Option<u64> {
        let replication = self.replication_status.read();
        replication.get(region_id).map(|r| r.lag_ms)
    }

    /// Enable/disable auto failover
    pub fn set_auto_failover(&mut self, enabled: bool) {
        self.auto_failover_enabled = enabled;
    }

    /// Get failover history
    pub fn get_failover_history(&self, limit: usize) -> Vec<FailoverEvent> {
        let events = self.failover_events.read();
        events.iter()
            .rev()
            .take(limit)
            .cloned()
            .collect()
    }
}

impl Default for DisasterRecovery {
    fn default() -> Self {
        Self::new()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_disaster_recovery() {
        let dr = DisasterRecovery::new();
        
        let region = Region::new("us-east".to_string(), "US East".to_string(), true);
        dr.register_region(region);
        
        let primary = dr.get_primary_region();
        assert!(primary.is_some());
    }

    #[test]
    fn test_failover() {
        let dr = DisasterRecovery::new();
        
        let from = Region::new("us-east".to_string(), "US East".to_string(), true);
        let to = Region::new("us-west".to_string(), "US West".to_string(), false);
        
        dr.register_region(from);
        dr.register_region(to);
        
        let result = dr.trigger_failover("us-east", "us-west", "Primary failed");
        assert!(result.is_ok());
    }
}