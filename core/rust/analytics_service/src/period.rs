//! Period Module

use serde::{Deserialize, Serialize};

/// Time Period
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum TimePeriod {
    Hour,
    Day,
    Week,
    Month,
    Year,
    All,
}

impl TimePeriod {
    pub fn to_seconds(&self) -> i64 {
        match self {
            TimePeriod::Hour => 3600,
            TimePeriod::Day => 86400,
            TimePeriod::Week => 604800,
            TimePeriod::Month => 2592000,
            TimePeriod::Year => 31536000,
            TimePeriod::All => i64::MAX,
        }
    }
}