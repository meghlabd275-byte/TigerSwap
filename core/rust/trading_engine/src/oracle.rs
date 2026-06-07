use serde::{Deserialize, Serialize};
use rust_decimal::Decimal;
use std::collections::VecDeque;
use std::sync::Arc;
use parking_lot::RwLock;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum OracleError { #[error("No observations")] NoObservations }

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Observation { pub timestamp: i64, pub price: Decimal }

pub struct TWAPOracle {
    window_seconds: u32,
    observations: Arc<RwLock<VecDeque<Observation>>>,
}

impl TWAPOracle {
    pub fn new(window_seconds: u32) -> Self { Self { window_seconds, observations: Arc::new(RwLock::new(VecDeque::new())) } }
    pub fn add_observation(&self, price: Decimal) {
        let obs = Observation { timestamp: chrono::Utc::now().timestamp(), price };
        self.observations.write().push_back(obs);
    }
    pub fn get_twap(&self) -> Result<Decimal, OracleError> {
        let obs = self.observations.read();
        if obs.is_empty() { return Err(OracleError::NoObservations); }
        let sum: Decimal = obs.iter().map(|o| o.price).sum();
        Ok(sum / Decimal::from(obs.len()))
    }
}