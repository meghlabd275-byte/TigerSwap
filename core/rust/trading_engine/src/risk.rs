use serde::{Deserialize, Serialize};
use rust_decimal::Decimal;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize, Default)]
pub enum RiskLevel { #[default] Low, Medium, High, Critical, Blocked }

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RiskCheck {
    pub name: String,
    pub level: RiskLevel,
    pub value: Decimal,
    pub threshold: Decimal,
    pub passed: bool,
}

impl RiskCheck {
    pub fn new(name: &str, value: Decimal, threshold: Decimal) -> Self {
        let passed = value <= threshold;
        Self { name: name.to_string(), level: if passed { RiskLevel::Low } else { RiskLevel::High }, value, threshold, passed }
    }
}

pub struct RiskEngine {
    max_leverage: Decimal,
    max_price_impact: Decimal,
}

impl RiskEngine {
    pub fn new() -> Self { Self { max_leverage: Decimal::from(50), max_price_impact: Decimal::from(500) } }
    pub fn check_price_impact(&self, price_impact_bps: i64) -> RiskCheck {
        RiskCheck::new("price_impact", Decimal::from(price_impact_bps), self.max_price_impact)
    }
}

impl Default for RiskEngine { fn default() -> Self { Self::new() } }