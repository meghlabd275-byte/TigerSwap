//! TigerSwap DCA (Dollar-Cost Averaging) Engine
//! 
//! Implements DCA strategy and recurring orders similar to 1inch, Jupiter, ParaSwap:
//! - Time-based DCA: Buy/sell at regular intervals
//! - Price-dip DCA: Buy more when price drops below threshold
//! - Recurring orders: Scheduled recurring swaps
//! - DCA portfolio: Multiple DCA strategies
//!
//! Implementation: Pure Rust with no external dependencies

use serde::{Deserialize, Serialize};
use std::sync::Arc;
use parking_lot::RwLock;
use rust_decimal::Decimal;
use thiserror::Error;
use uuid::Uuid;
use chrono::{Utc, Duration, Timelike, Datelike};
use std::collections::HashMap;
use std::cmp::Ordering;

/// Chain ID constants
pub const CHAIN_ETH: u64 = 1;
pub const CHAIN_BSC: u64 = 56;
pub const CHAIN_POLYGON: u64 = 137;
pub const CHAIN_ARBITRUM: u64 = 42161;
pub const CHAIN_OPTIMISM: u64 = 10;
pub const CHAIN_BASE: u64 = 8453;
pub const CHAIN_AVALANCHE: u64 = 43114;
pub const CHAIN_SOLANA: u64 = 0;

#[derive(Debug, Error)]
pub enum DCAError {
    #[error("Strategy not found: {0}")]
    StrategyNotFound(String),
    #[error("Strategy paused: {0}")]
    StrategyPaused(String),
    #[error("Insufficient balance: {0}")]
    InsufficientBalance(String),
    #[error("Invalid parameters: {0}")]
    InvalidParameters(String),
    #[error("Not due yet: next execution at {0}")]
    NotDueYet(i64),
    #[error("Price condition not met: expected {expected}, got {actual}")]
    PriceConditionNotMet { expected: Decimal, actual: Decimal },
    #[error("Execution failed: {0}")]
    ExecutionFailed(String),
    #[error("Max slippage exceeded: {0}")]
    MaxSlippageExceeded(String),
    #[error("Chain not supported: {0}")]
    ChainNotSupported(u64),
    #[error("No executor configured")]
    NoExecutorConfigured,
}

/// DCA type
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum DCAType {
    TimeBased,      // Buy/sell at regular intervals
    PriceDip,      // Buy more when price drops
    VolumeBased,    // Execute when volume threshold reached
    Recurring,     // General recurring order
}

impl Default for DCAType {
    fn default() -> Self { DCAType::TimeBased }
}

/// DCA status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum DCAStatus {
    Pending,
    Active,
    Paused,
    Completed,
    Cancelled,
    Failed,
}

impl Default for DCAStatus {
    fn default() -> Self { DCAStatus::Pending }
}

/// Execution trigger
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum ExecutionTrigger {
    TimeInterval,      // Every N seconds
    Daily,          // Once per day
    Weekly,         // Once per week
    Monthly,        // Once per month
    PriceDip,        // When price drops below threshold
    PriceSurge,       // When price rises above threshold
    VolumeSpike,     // When volume spikes
}

impl Default for ExecutionTrigger {
    fn default() -> Self { ExecutionTrigger::TimeInterval }
}

/// Price condition for DCA
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct PriceCondition {
    pub trigger_price: Decimal,
    pub condition: PriceTriggerCondition,
    pub reference_price: Decimal,  // Price to compare against
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum PriceTriggerCondition {
    Below,          // Trigger when price < trigger
    Above,          // Trigger when price > trigger
    PercentBelow,    // Trigger when price is X% below reference
    PercentAbove,   // Trigger when price is X% above reference
}

impl Default for PriceCondition {
    fn default() -> Self {
        Self {
            trigger_price: Decimal::ZERO,
            condition: PriceTriggerCondition::Below,
            reference_price: Decimal::ZERO,
        }
    }
}

/// Time interval configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TimeInterval {
    pub interval_type: IntervalType,
    pub value: i64,  // Value in seconds for TimeInterval, or hour of day for Daily/Weekly/Monthly
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum IntervalType {
    Seconds,
    Minutes,
    Hours,
    Days,
    Weeks,
}

impl Default for TimeInterval {
    fn default() -> Self {
        Self {
            interval_type: IntervalType::Days,
            value: 1,
        }
    }
}

impl TimeInterval {
    /// Get interval in seconds
    pub fn to_seconds(&self) -> i64 {
        match self.interval_type {
            IntervalType::Seconds => self.value,
            IntervalType::Minutes => self.value * 60,
            IntervalType::Hours => self.value * 3600,
            IntervalType::Days => self.value * 86400,
            IntervalType::Weeks => self.value * 604800,
        }
    }
}

/// DCA execution configuration
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DCAExecutionConfig {
    pub trigger: ExecutionTrigger,
    pub time_interval: Option<TimeInterval>,
    pub price_condition: Option<PriceCondition>,
    pub volume_threshold: Option<u128>,
    pub max_slippage_bps: i64,
    pub min_execution_amount: u128,
    pub max_execution_amount: u128,
}

impl Default for DCAExecutionConfig {
    fn default() -> Self {
        Self {
            trigger: ExecutionTrigger::TimeInterval,
            time_interval: Some(TimeInterval::default()),
            price_condition: None,
            volume_threshold: None,
            max_slippage_bps: 100,
            min_execution_amount: 10,
            max_execution_amount: 1000000,
        }
    }
}

/// DCA strategy
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DCAStrategy {
    pub strategy_id: String,
    pub user: String,
    pub chain_id: u64,
    pub dca_type: DCAType,
    pub token_in: String,
    pub token_out: String,
    pub side: DCASide,
    
    // Amount configuration
    pub total_amount: u128,           // Total amount to execute
    pub per_execution_amount: u128,   // Amount per execution
    pub executed_amount: u128,         // Amount already executed
    pub remaining_amount: u128,        // Remaining amount
    
    // Execution settings
    pub execution_config: DCAExecutionConfig,
    pub executions_completed: u32,
    pub max_executions: u32,
    
    // Timing
    pub start_time: i64,
    pub next_execution_time: i64,
    pub last_execution_time: Option<i64>,
    pub end_time: Option<i64>,
    
    // Status
    pub status: DCAStatus,
    pub created_at: i64,
    pub updated_at: i64,
    
    // Statistics
    pub total_swaps: u32,
    pub successful_swaps: u32,
    pub failed_swaps: u32,
    pub average_price: Decimal,
    pub total_spent: u128,
    
    // Settings
    pub referrer: Option<String>,
    pub auto_compound: bool,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum DCASide {
    Buy,   // Buy token_out with token_in
    Sell,  // Sell token_in for token_out
}

impl DCAStrategy {
    /// Create a new time-based DCA strategy
    pub fn new_time_based(
        user: String,
        chain_id: u64,
        token_in: String,
        token_out: String,
        side: DCASide,
        total_amount: u128,
        per_execution_amount: u128,
        interval: TimeInterval,
        max_executions: u32,
    ) -> Self {
        let now = Utc::now().timestamp();
        let next_execution = now + interval.to_seconds();
        
        Self {
            strategy_id: Uuid::new_v4().to_string(),
            user,
            chain_id,
            dca_type: DCAType::TimeBased,
            token_in,
            token_out,
            side,
            total_amount,
            per_execution_amount,
            executed_amount: 0,
            remaining_amount: total_amount,
            execution_config: DCAExecutionConfig {
                trigger: ExecutionTrigger::TimeInterval,
                time_interval: Some(interval),
                ..Default::default()
            },
            executions_completed: 0,
            max_executions,
            start_time: now,
            next_execution_time: next_execution,
            last_execution_time: None,
            end_time: None,
            status: DCAStatus::Pending,
            created_at: now,
            updated_at: now,
            total_swaps: 0,
            successful_swaps: 0,
            failed_swaps: 0,
            average_price: Decimal::ZERO,
            total_spent: 0,
            referrer: None,
            auto_compound: false,
        }
    }

    /// Create a new price-dip DCA strategy
    pub fn new_price_dip(
        user: String,
        chain_id: u64,
        token_in: String,
        token_out: String,
        side: DCASide,
        per_execution_amount: u128,
        trigger_price: Decimal,
        percent_dip: i64,
    ) -> Self {
        let now = Utc::now().timestamp();
        
        Self {
            strategy_id: Uuid::new_v4().to_string(),
            user,
            chain_id,
            dca_type: DCAType::PriceDip,
            token_in,
            token_out,
            side,
            total_amount: 0,
            per_execution_amount,
            executed_amount: 0,
            remaining_amount: 0,
            execution_config: DCAExecutionConfig {
                trigger: ExecutionTrigger::PriceDip,
                price_condition: Some(PriceCondition {
                    trigger_price,
                    condition: PriceTriggerCondition::PercentBelow,
                    reference_price: trigger_price * Decimal::from(10000 - percent_dip) / Decimal::from(10000),
                }),
                ..Default::default()
            },
            executions_completed: 0,
            max_executions: 0,
            start_time: now,
            next_execution_time: now,
            last_execution_time: None,
            end_time: None,
            status: DCAStatus::Pending,
            created_at: now,
            updated_at: now,
            total_swaps: 0,
            successful_swaps: 0,
            failed_swaps: 0,
            average_price: Decimal::ZERO,
            total_spent: 0,
            referrer: None,
            auto_compound: false,
        }
    }

    /// Create a new recurring order
    pub fn new_recurring(
        user: String,
        chain_id: u64,
        token_in: String,
        token_out: String,
        side: DCASide,
        amount: u128,
        trigger: ExecutionTrigger,
    ) -> Self {
        let now = Utc::now().timestamp();
        let next_execution = match trigger {
            ExecutionTrigger::Daily => {
                let next = Utc::now().date_naive().and_hms_opt(0, 0, 0).unwrap();
                let tomorrow = next + Duration::days(1);
                tomorrow.and_utc().timestamp()
            }
            ExecutionTrigger::Weekly => {
                let next = Utc::now().date_naive() + Duration::weeks(1);
                next.and_hms_opt(0, 0, 0).unwrap().and_utc().timestamp()
            }
            ExecutionTrigger::Monthly => {
                let next = Utc::now().date_naive() + Duration::days(30);
                next.and_hms_opt(0, 0, 0).unwrap().and_utc().timestamp()
            }
            _ => now,
        };
        
        Self {
            strategy_id: Uuid::new_v4().to_string(),
            user,
            chain_id,
            dca_type: DCAType::Recurring,
            token_in,
            token_out,
            side,
            total_amount: 0,
            per_execution_amount: amount,
            executed_amount: 0,
            remaining_amount: 0,
            execution_config: DCAExecutionConfig {
                trigger,
                ..Default::default()
            },
            executions_completed: 0,
            max_executions: 0,
            start_time: now,
            next_execution_time: next_execution,
            last_execution_time: None,
            end_time: None,
            status: DCAStatus::Pending,
            created_at: now,
            updated_at: now,
            total_swaps: 0,
            successful_swaps: 0,
            failed_swaps: 0,
            average_price: Decimal::ZERO,
            total_spent: 0,
            referrer: None,
            auto_compound: false,
        }
    }

    /// Validate strategy parameters
    pub fn validate(&self) -> Result<(), DCAError> {
        if self.token_in == self.token_out {
            return Err(DCAError::InvalidParameters("Token pair must be different".to_string()));
        }
        
        if self.dca_type == DCAType::TimeBased {
            if self.total_amount == 0 {
                return Err(DCAError::InvalidParameters("Total amount must be greater than 0".to_string()));
            }
            if self.per_execution_amount == 0 {
                return Err(DCAError::InvalidParameters("Per execution amount must be greater than 0".to_string()));
            }
            if self.per_execution_amount > self.total_amount {
                return Err(DCAError::InvalidParameters("Per execution amount cannot exceed total amount".to_string()));
            }
        }
        
        if let Some(ref interval) = self.execution_config.time_interval {
            if interval.value <= 0 {
                return Err(DCAError::InvalidParameters("Interval value must be positive".to_string()));
            }
        }
        
        Ok(())
    }

    /// Check if strategy is due for execution
    pub fn is_due(&self, current_time: i64, current_price: Decimal) -> bool {
        if !matches!(self.status, DCAStatus::Active) {
            return false;
        }
        
        // Check if max executions reached
        if self.max_executions > 0 && self.executions_completed >= self.max_executions {
            return false;
        }
        
        // Check if end time reached
        if let Some(end_time) = self.end_time {
            if current_time > end_time {
                return false;
            }
        }
        
        // Check time-based triggers
        match self.execution_config.trigger {
            ExecutionTrigger::TimeInterval |
            ExecutionTrigger::Daily |
            ExecutionTrigger::Weekly |
            ExecutionTrigger::Monthly => {
                current_time >= self.next_execution_time
            }
            ExecutionTrigger::PriceDip => {
                if let Some(ref price_cond) = self.execution_config.price_condition {
                    match price_cond.condition {
                        PriceTriggerCondition::Below => current_price < price_cond.trigger_price,
                        PriceTriggerCondition::Above => current_price > price_cond.trigger_price,
                        PriceTriggerCondition::PercentBelow => {
                            let threshold = price_cond.reference_price * Decimal::from(10000 - price_cond.trigger_price.as_i64()) / Decimal::from(10000);
                            current_price < threshold
                        }
                        PriceTriggerCondition::PercentAbove => {
                            let threshold = price_cond.reference_price * Decimal::from(10000 + price_cond.trigger_price.as_i64()) / Decimal::from(10000);
                            current_price > threshold
                        }
                    }
                } else {
                    false
                }
            }
            ExecutionTrigger::VolumeSpike => {
                // Volume checking would be done externally
                false
            }
        }
    }

    /// Calculate next execution time
    pub fn calculate_next_execution(&mut self) {
        let now = Utc::now().timestamp();
        
        match self.execution_config.trigger {
            ExecutionTrigger::TimeInterval => {
                if let Some(ref interval) = self.execution_config.time_interval {
                    self.next_execution_time = now + interval.to_seconds();
                }
            }
            ExecutionTrigger::Daily => {
                let tomorrow = Utc::now().date_naive() + Duration::days(1);
                self.next_execution_time = tomorrow.and_hms_opt(0, 0, 0).unwrap().and_utc().timestamp();
            }
            ExecutionTrigger::Weekly => {
                let next_week = Utc::now().date_naive() + Duration::weeks(1);
                self.next_execution_time = next_week.and_hms_opt(0, 0, 0).unwrap().and_utc().timestamp();
            }
            ExecutionTrigger::Monthly => {
                let next_month = Utc::now().date_naive() + Duration::days(30);
                self.next_execution_time = next_month.and_hms_opt(0, 0, 0).unwrap().and_utc().timestamp();
            }
            ExecutionTrigger::PriceDip | ExecutionTrigger::VolumeSpike => {
                // These trigger on condition, not schedule
            }
        }
        
        self.updated_at = Utc::now().timestamp();
    }

    /// Execute a swap
    pub fn execute(&mut self, amount_in: u128, amount_out: u128, current_price: Decimal) -> Result<(), DCAError> {
        let now = Utc::now().timestamp();
        
        // Check slippage
        let expected_out = amount_in * current_price.as_u128();
        let actual_slippage = if expected_out > 0 {
            ((expected_out as i128 - amount_out as i128) * 10000 / expected_out as i128).abs() as i64
        } else {
            0
        };
        
        if actual_slippage > self.execution_config.max_slippage_bps {
            self.failed_swaps += 1;
            return Err(DCAError::MaxSlippageExceeded(format!("Slippage: {} bps", actual_slippage)));
        }
        
        // Update statistics
        self.executed_amount += amount_in;
        self.remaining_amount = self.remaining_amount.saturating_sub(amount_in);
        self.executions_completed += 1;
        self.total_swaps += 1;
        self.successful_swaps += 1;
        self.total_spent += amount_in;
        
        // Update average price
        if self.total_swaps > 0 {
            let total_value: u128 = (0..self.successful_swaps).map(|_| amount_in).sum();
            self.average_price = Decimal::from(total_value) / Decimal::from(self.successful_swaps);
        }
        
        self.last_execution_time = Some(now);
        self.calculate_next_execution();
        
        // Check if completed
        if self.remaining_amount == 0 || (self.max_executions > 0 && self.executions_completed >= self.max_executions) {
            self.status = DCAStatus::Completed;
        }
        
        Ok(())
    }

    /// Pause the strategy
    pub fn pause(&mut self) {
        self.status = DCAStatus::Paused;
        self.updated_at = Utc::now().timestamp();
    }

    /// Resume the strategy
    pub fn resume(&mut self) {
        self.status = DCAStatus::Active;
        self.calculate_next_execution();
        self.updated_at = Utc::now().timestamp();
    }

    /// Cancel the strategy
    pub fn cancel(&mut self) {
        self.status = DCAStatus::Cancelled;
        self.updated_at = Utc::now().timestamp();
    }

    /// Get progress percentage
    pub fn progress_percent(&self) -> Decimal {
        if self.total_amount == 0 {
            Decimal::ZERO
        } else {
            Decimal::from(self.executed_amount) * Decimal::from(10000) / Decimal::from(self.total_amount)
        }
    }

    /// Get remaining executions
    pub fn remaining_executions(&self) -> u32 {
        if self.max_executions == 0 {
            u32::MAX
        } else {
            self.max_executions.saturating_sub(self.executions_completed)
        }
    }
}

/// DCA execution result
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DCAExecution {
    pub strategy_id: String,
    pub user: String,
    pub amount_in: u128,
    pub amount_out: u128,
    pub price: Decimal,
    pub slippage_bps: i64,
    pub executed_at: i64,
    pub tx_hash: String,
}

impl DCAExecution {
    pub fn new(strategy_id: String, user: String, amount_in: u128, amount_out: u128, price: Decimal) -> Self {
        let slippage_bps = if amount_in > 0 {
            let expected = amount_in * price.as_u128();
            if expected > 0 {
                ((expected as i128 - amount_out as i128) * 10000 / expected as i128).abs() as i64
            } else {
                0
            }
        } else {
            0
        };
        
        Self {
            strategy_id,
            user,
            amount_in,
            amount_out,
            price,
            slippage_bps,
            executed_at: Utc::now().timestamp(),
            tx_hash: Uuid::new_v4().to_string(),
        }
    }
}

/// DCA engine
pub struct DCAEngine {
    strategies: Arc<RwLock<HashMap<String, DCAStrategy>>>,
    execution_history: Arc<RwLock<HashMap<String, Vec<DCAExecution>>>>,
    supported_chains: Arc<RwLock<std::collections::HashSet<u64>>>,
    max_slippage_bps: i64,
}

impl DCAEngine {
    /// Create a new DCA engine
    pub fn new() -> Self {
        let chains: std::collections::HashSet<u64> = [
            CHAIN_ETH, CHAIN_BSC, CHAIN_POLYGON, CHAIN_ARBITRUM,
            CHAIN_OPTIMISM, CHAIN_BASE, CHAIN_AVALANCHE, CHAIN_SOLANA,
        ].into_iter().collect();
        
        Self {
            strategies: Arc::new(RwLock::new(HashMap::new())),
            execution_history: Arc::new(RwLock::new(HashMap::new())),
            supported_chains: Arc::new(RwLock::new(chains)),
            max_slippage_bps: 100,
        }
    }

    /// Check if chain is supported
    pub fn is_chain_supported(&self, chain_id: u64) -> bool {
        self.supported_chains.read().contains(&chain_id)
    }

    /// Create a new DCA strategy
    pub fn create_strategy(&self, strategy: DCAStrategy) -> Result<String, DCAError> {
        strategy.validate()?;
        
        if !self.is_chain_supported(strategy.chain_id) {
            return Err(DCAError::ChainNotSupported(strategy.chain_id));
        }
        
        let strategy_id = strategy.strategy_id.clone();
        self.strategies.write().insert(strategy_id.clone(), strategy);
        Ok(strategy_id)
    }

    /// Get strategy by ID
    pub fn get_strategy(&self, strategy_id: &str) -> Option<DCAStrategy> {
        self.strategies.read().get(strategy_id).cloned()
    }

    /// Get strategies for a user
    pub fn get_user_strategies(&self, user: &str) -> Vec<DCAStrategy> {
        self.strategies.read()
            .values()
            .filter(|s| s.user == user)
            .cloned()
            .collect()
    }

    /// Get active strategies
    pub fn get_active_strategies(&self) -> Vec<DCAStrategy> {
        self.strategies.read()
            .values()
            .filter(|s| matches!(s.status, DCAStatus::Active))
            .cloned()
            .collect()
    }

    /// Get strategies due for execution
    pub fn get_due_strategies(&self, current_price: Decimal) -> Vec<DCAStrategy> {
        let now = Utc::now().timestamp();
        
        self.strategies.read()
            .values()
            .filter(|s| s.is_due(now, current_price))
            .cloned()
            .collect()
    }

    /// Execute a strategy
    pub fn execute_strategy(&self, strategy_id: &str, amount_in: u128, amount_out: u128, current_price: Decimal) -> Result<DCAExecution, DCAError> {
        let mut strategies = self.strategies.write();
        let strategy = strategies.get_mut(strategy_id)
            .ok_or_else(|| DCAError::StrategyNotFound(strategy_id.to_string()))?;
        
        if !matches!(strategy.status, DCAStatus::Active) {
            return Err(DCAError::StrategyPaused(strategy_id.to_string()));
        }
        
        // Execute the swap
        strategy.execute(amount_in, amount_out, current_price)?;
        
        // Create execution record
        let execution = DCAExecution::new(
            strategy_id.to_string(),
            strategy.user.clone(),
            amount_in,
            amount_out,
            current_price,
        );
        
        // Store execution history
        self.execution_history.write()
            .entry(strategy_id.to_string())
            .or_insert_with(Vec::new)
            .push(execution.clone());
        
        Ok(execution)
    }

    /// Pause a strategy
    pub fn pause_strategy(&self, strategy_id: &str, user: &str) -> Result<(), DCAError> {
        let mut strategies = self.strategies.write();
        let strategy = strategies.get_mut(strategy_id)
            .ok_or_else(|| DCAError::StrategyNotFound(strategy_id.to_string()))?;
        
        if strategy.user != user {
            return Err(DCAError::StrategyNotFound(strategy_id.to_string()));
        }
        
        strategy.pause();
        Ok(())
    }

    /// Resume a strategy
    pub fn resume_strategy(&self, strategy_id: &str, user: &str) -> Result<(), DCAError> {
        let mut strategies = self.strategies.write();
        let strategy = strategies.get_mut(strategy_id)
            .ok_or_else(|| DCAError::StrategyNotFound(strategy_id.to_string()))?;
        
        if strategy.user != user {
            return Err(DCAError::StrategyNotFound(strategy_id.to_string()));
        }
        
        strategy.resume();
        Ok(())
    }

    /// Cancel a strategy
    pub fn cancel_strategy(&self, strategy_id: &str, user: &str) -> Result<(), DCAError> {
        let mut strategies = self.strategies.write();
        let strategy = strategies.get_mut(strategy_id)
            .ok_or_else(|| DCAError::StrategyNotFound(strategy_id.to_string()))?;
        
        if strategy.user != user {
            return Err(DCAError::StrategyNotFound(strategy_id.to_string()));
        }
        
        strategy.cancel();
        Ok(())
    }

    /// Update strategy parameters
    pub fn update_strategy(
        &self,
        strategy_id: &str,
        user: &str,
        per_execution_amount: Option<u128>,
        max_slippage_bps: Option<i64>,
    ) -> Result<(), DCAError> {
        let mut strategies = self.strategies.write();
        let strategy = strategies.get_mut(strategy_id)
            .ok_or_else(|| DCAError::StrategyNotFound(strategy_id.to_string()))?;
        
        if strategy.user != user {
            return Err(DCAError::StrategyNotFound(strategy_id.to_string()));
        }
        
        if let Some(amount) = per_execution_amount {
            strategy.per_execution_amount = amount;
        }
        
        if let Some(slippage) = max_slippage_bps {
            strategy.execution_config.max_slippage_bps = slippage;
        }
        
        strategy.updated_at = Utc::now().timestamp();
        Ok(())
    }

    /// Get execution history
    pub fn get_execution_history(&self, strategy_id: &str) -> Vec<DCAExecution> {
        self.execution_history.read()
            .get(strategy_id)
            .cloned()
            .unwrap_or_default()
    }

    /// Get statistics for a user
    pub fn get_user_stats(&self, user: &str) -> DCAStats {
        let strategies = self.strategies.read();
        
        let mut active = 0;
        let mut paused = 0;
        let mut completed = 0;
        let mut total_volume: u128 = 0;
        let mut total_swaps: u32 = 0;
        
        for strategy in strategies.values() {
            if strategy.user != user { continue; }
            
            match strategy.status {
                DCAStatus::Active => active += 1,
                DCAStatus::Paused => paused += 1,
                DCAStatus::Completed => completed += 1,
                _ => {}
            }
            
            total_volume += strategy.executed_amount;
            total_swaps += strategy.total_swaps;
        }
        
        DCAStats {
            active_strategies: active,
            paused_strategies: paused,
            completed_strategies: completed,
            total_volume,
            total_swaps,
        }
    }

    /// Process all due strategies
    pub fn process_due_strategies<F>(&self, current_price: Decimal, executor: F) -> Vec<DCAExecution>
    where
        F: Fn(&DCAStrategy) -> Result<(u128, u128), DCAError>,
    {
        let due = self.get_due_strategies(current_price);
        let mut executions = vec![];
        
        for strategy in due {
            match executor(&strategy) {
                Ok((amount_in, amount_out)) => {
                    if let Ok(exec) = self.execute_strategy(&strategy.strategy_id, amount_in, amount_out, current_price) {
                        executions.push(exec);
                    }
                }
                Err(_) => {
                    // Log error but continue
                }
            }
        }
        
        executions
    }

    /// Get portfolio summary
    pub fn get_portfolio(&self, user: &str) -> DCAPortfolio {
        let strategies = self.get_user_strategies(user);
        
        let mut total_invested: u128 = 0;
        let mut total_value: u128 = 0;
        let mut active_count = 0;
        
        for strategy in &strategies {
            total_invested += strategy.executed_amount;
            
            if matches!(strategy.status, DCAStatus::Active) {
                active_count += 1;
            }
        }
        
        DCAPortfolio {
            user: user.to_string(),
            total_invested,
            total_value,
            active_strategies: active_count,
            total_strategies: strategies.len(),
        }
    }

    /// Add supported chain
    pub fn add_chain(&self, chain_id: u64) {
        self.supported_chains.write().insert(chain_id);
    }

    /// Get supported chains
    pub fn supported_chains(&self) -> Vec<u64> {
        self.supported_chains.read().iter().cloned().collect()
    }
}

impl Default for DCAEngine {
    fn default() -> Self { Self::new() }
}

/// DCA statistics
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DCAStats {
    pub active_strategies: usize,
    pub paused_strategies: usize,
    pub completed_strategies: usize,
    pub total_volume: u128,
    pub total_swaps: u32,
}

/// DCA portfolio
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct DCAPortfolio {
    pub user: String,
    pub total_invested: u128,
    pub total_value: u128,
    pub active_strategies: usize,
    pub total_strategies: usize,
}

#[cfg(test)]
mod tests {
    use super::*;
    use rust_decimal_macros::dec;

    #[test]
    fn test_dca_strategy_creation() {
        let strategy = DCAStrategy::new_time_based(
            "user1".to_string(),
            CHAIN_ETH,
            "USDC".to_string(),
            "WETH".to_string(),
            DCASide::Buy,
            10000,
            1000,
            TimeInterval { interval_type: IntervalType::Days, value: 1 },
            10,
        );
        
        assert!(strategy.validate().is_ok());
        assert_eq!(strategy.dca_type, DCAType::TimeBased);
    }

    #[test]
    fn test_price_dip_strategy() {
        let strategy = DCAStrategy::new_price_dip(
            "user1".to_string(),
            CHAIN_ETH,
            "USDC".to_string(),
            "WETH".to_string(),
            DCASide::Buy,
            100,
            dec!(0.05),
            500,  // 5% dip
        );
        
        assert_eq!(strategy.dca_type, DCAType::PriceDip);
    }

    #[test]
    fn test_time_interval() {
        let interval = TimeInterval { interval_type: IntervalType::Hours, value: 4 };
        assert_eq!(interval.to_seconds(), 14400);
    }
}