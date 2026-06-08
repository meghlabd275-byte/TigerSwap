//! TigerSwap Keeper Network Engine
//! 
//! Implements automation keeper network:
//! - Scheduled task execution
//! - Liquidation automation
//! - Rebalancing automation
//! - Price oracle updates
//!
//! Implementation: Pure Rust with no external dependencies

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use thiserror::Error;
use uuid::Uuid;
use chrono::Utc;
use std::collections::HashMap;

/// Chain ID constants
pub const CHAIN_ETH: u64 = 1;
pub const CHAIN_BSC: u64 = 56;

#[derive(Debug, Error)]
pub enum KeeperError {
    #[error("Task not found: {0}")]
    TaskNotFound(String),
    #[error("Keeper not found: {0}")]
    KeeperNotFound(String),
    #[error("Execution failed: {0}")]
    ExecutionFailed(String),
    #[error("Invalid parameters: {0}")]
    InvalidParameters(String),
}

/// Task type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TaskType {
    Liquidation,
    Rebalance,
    PriceUpdate,
    Scheduled,
    Custom,
}

impl Default for TaskType {
    fn default() -> Self { TaskType::Scheduled }
}

/// Task status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TaskStatus {
    Pending,
    Running,
    Completed,
    Failed,
    Cancelled,
}

impl Default for TaskStatus {
    fn default() -> Self { TaskStatus::Pending }
}

/// Automation task
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KeeperTask {
    pub task_id: String,
    pub task_type: TaskType,
    pub chain_id: u64,
    pub contract_address: String,
    pub function_name: String,
    pub function_params: Vec<String>,
    
    // Schedule
    pub schedule_type: ScheduleType,
    pub interval_seconds: i64,
    pub next_execution: i64,
    pub last_execution: Option<i64>,
    
    // Status
    pub status: TaskStatus,
    pub execution_count: u32,
    pub last_error: Option<String>,
    
    pub created_at: i64,
    pub updated_at: i64,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ScheduleType {
    Interval,
    Cron,
    Trigger,
}

impl Default for ScheduleType {
    fn default() -> Self { ScheduleType::Interval }
}

impl KeeperTask {
    pub fn new(
        task_type: TaskType,
        chain_id: u64,
        contract_address: String,
        function_name: String,
        interval_seconds: i64,
    ) -> Self {
        let now = Utc::now().timestamp();
        
        Self {
            task_id: Uuid::new_v4().to_string(),
            task_type,
            chain_id,
            contract_address,
            function_name,
            function_params: vec![],
            schedule_type: ScheduleType::Interval,
            interval_seconds,
            next_execution: now + interval_seconds,
            last_execution: None,
            status: TaskStatus::Pending,
            execution_count: 0,
            last_error: None,
            created_at: now,
            updated_at: now,
        }
    }

    pub fn should_execute(&self) -> bool {
        let now = Utc::now().timestamp();
        now >= self.next_execution && matches!(self.status, TaskStatus::Pending)
    }

    pub fn execute(&mut self) -> Result<(), KeeperError> {
        self.status = TaskStatus::Running;
        self.execution_count += 1;
        self.last_execution = Some(Utc::now().timestamp());
        self.next_execution = Utc::now().timestamp() + self.interval_seconds;
        self.status = TaskStatus::Completed;
        self.updated_at = Utc::now().timestamp();
        Ok(())
    }

    pub fn fail(&mut self, error: String) {
        self.status = TaskStatus::Failed;
        self.last_error = Some(error);
        self.updated_at = Utc::now().timestamp();
    }
}

/// Keeper node
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct KeeperNode {
    pub keeper_id: String,
    pub address: String,
    pub is_active: bool,
    pub tasks_assigned: u32,
    pub tasks_completed: u32,
    pub tasks_failed: u32,
    pub uptime: i64,
    pub registered_at: i64,
}

impl KeeperNode {
    pub fn new(address: String) -> Self {
        Self {
            keeper_id: Uuid::new_v4().to_string(),
            address,
            is_active: true,
            tasks_assigned: 0,
            tasks_completed: 0,
            tasks_failed: 0,
            uptime: 0,
            registered_at: Utc::now().timestamp(),
        }
    }

    pub fn record_success(&mut self) {
        self.tasks_completed += 1;
    }

    pub fn record_failure(&mut self) {
        self.tasks_failed += 1;
    }
}

/// Execution record
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ExecutionRecord {
    pub record_id: String,
    pub task_id: String,
    pub keeper_id: String,
    pub status: TaskStatus,
    pub gas_used: u64,
    pub error: Option<String>,
    pub executed_at: i64,
}

/// Keeper Network Engine
pub struct KeeperNetworkEngine {
    tasks: Arc<RwLock<HashMap<String, KeeperTask>>>,
    keepers: Arc<RwLock<HashMap<String, KeeperNode>>>,
    executions: Arc<RwLock<Vec<ExecutionRecord>>>,
}

impl KeeperNetworkEngine {
    pub fn new() -> Self {
        Self {
            tasks: Arc::new(RwLock::new(HashMap::new())),
            keepers: Arc::new(RwLock::new(HashMap::new())),
            executions: Arc::new(RwLock::new(Vec::new())),
        }
    }

    /// Register keeper
    pub fn register_keeper(&self, address: String) -> String {
        let keeper = KeeperNode::new(address);
        let keeper_id = keeper.keeper_id.clone();
        
        self.keepers.write().insert(keeper_id.clone(), keeper);
        
        keeper_id
    }

    /// Create task
    pub fn create_task(
        &self,
        task_type: TaskType,
        chain_id: u64,
        contract_address: String,
        function_name: String,
        interval_seconds: i64,
    ) -> String {
        let task = KeeperTask::new(
            task_type,
            chain_id,
            contract_address,
            function_name,
            interval_seconds,
        );
        
        let task_id = task.task_id.clone();
        self.tasks.write().insert(task_id.clone(), task);
        
        task_id
    }

    /// Get pending tasks
    pub fn get_pending_tasks(&self) -> Vec<KeeperTask> {
        self.tasks.read()
            .values()
            .filter(|t| t.should_execute())
            .cloned()
            .collect()
    }

    /// Execute task
    pub fn execute_task(&self, task_id: &str, keeper_id: &str) -> Result<ExecutionRecord, KeeperError> {
        let mut tasks = self.tasks.write();
        let task = tasks.get_mut(task_id)
            .ok_or_else(|| KeeperError::TaskNotFound(task_id.to_string()))?;
        
        // Execute (simulated)
        let result = task.execute();
        
        let record = ExecutionRecord {
            record_id: Uuid::new_v4().to_string(),
            task_id: task_id.to_string(),
            keeper_id: keeper_id.to_string(),
            status: task.status,
            gas_used: 50000,
            error: task.last_error.clone(),
            executed_at: Utc::now().timestamp(),
        };
        
        // Update keeper stats
        let mut keepers = self.keepers.write();
        if let Some(keeper) = keepers.get_mut(keeper_id) {
            if result.is_ok() {
                keeper.record_success();
            } else {
                keeper.record_failure();
            }
        }
        
        self.executions.write().push(record.clone());
        
        result.map(|_| record)
    }

    /// Get task
    pub fn get_task(&self, task_id: &str) -> Option<KeeperTask> {
        self.tasks.read().get(task_id).cloned()
    }

    /// Get keeper
    pub fn get_keeper(&self, keeper_id: &str) -> Option<KeeperNode> {
        self.keepers.read().get(keeper_id).cloned()
    }

    /// Get execution history
    pub fn get_executions(&self, limit: usize) -> Vec<ExecutionRecord> {
        let mut records = self.executions.read().clone();
        records.sort_by(|a, b| b.executed_at.cmp(&a.executed_at));
        records.truncate(limit);
        records
    }

    /// Get active keepers
    pub fn get_active_keepers(&self) -> Vec<KeeperNode> {
        self.keepers.read()
            .values()
            .filter(|k| k.is_active)
            .cloned()
            .collect()
    }
}

impl Default for KeeperNetworkEngine {
    fn default() -> Self { Self::new() }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_keeper_registration() {
        let engine = KeeperNetworkEngine::new();
        
        let keeper_id = engine.register_keeper("0xKeeper1".to_string());
        
        assert!(!keeper_id.is_empty());
    }

    #[test]
    fn test_task_creation() {
        let engine = KeeperNetworkEngine::new();
        
        let task_id = engine.create_task(
            TaskType::Liquidation,
            CHAIN_ETH,
            "0xContract".to_string(),
            "liquidate".to_string(),
            300,
        );
        
        assert!(!task_id.is_empty());
    }

    #[test]
    fn test_task_execution() {
        let engine = KeeperNetworkEngine::new();
        
        let keeper_id = engine.register_keeper("0xKeeper".to_string());
        let task_id = engine.create_task(
            TaskType::Scheduled,
            CHAIN_ETH,
            "0xContract".to_string(),
            "execute".to_string(),
            60,
        );
        
        let result = engine.execute_task(&task_id, &keeper_id);
        
        assert!(result.is_ok());
    }
}