//! TigerSwap insurance fund and secure key storage.
//!
//! The insurance fund tracks per-token holdings with a daily spending limit and
//! an append-only event log. Key material is encrypted with AES-256-GCM from the
//! `tigerswap-security` crate; there is no home-grown cipher here.

use std::collections::HashMap;
use std::sync::Arc;

use parking_lot::RwLock;
use tigerswap_security::{Encryptor, SecureKey, KeyType};
use zeroize::Zeroize;

/// Errors raised by the insurance fund.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum FundError {
    /// The token has no balance to draw from.
    UnknownToken(String),
    /// The requested amount exceeds the available balance.
    InsufficientBalance,
    /// The request would exceed the daily spending limit.
    DailyLimitExceeded,
    /// Decimal places did not match.
    DecimalMismatch,
    /// An arithmetic operation overflowed.
    Overflow,
    /// The stored key could not be found.
    KeyNotFound,
    /// The key material could not be encrypted or decrypted.
    CryptoFailure,
    /// The key material was malformed.
    InvalidKeyMaterial,
}

impl std::fmt::Display for FundError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            FundError::UnknownToken(token) => write!(f, "unknown token {token}"),
            FundError::InsufficientBalance => write!(f, "insufficient insurance fund balance"),
            FundError::DailyLimitExceeded => write!(f, "daily spending limit exceeded"),
            FundError::DecimalMismatch => write!(f, "decimal mismatch"),
            FundError::Overflow => write!(f, "arithmetic overflow"),
            FundError::KeyNotFound => write!(f, "key not found"),
            FundError::CryptoFailure => write!(f, "encryption or decryption failed"),
            FundError::InvalidKeyMaterial => write!(f, "invalid key material"),
        }
    }
}

impl std::error::Error for FundError {}

/// Fixed-point decimal backed by `i128` with an explicit scale.
#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord)]
pub struct FixedDecimal {
    pub value: i128,
    pub decimals: u8,
}

impl FixedDecimal {
    pub fn new(value: i128, decimals: u8) -> Self {
        Self { value, decimals }
    }

    pub fn zero(decimals: u8) -> Self {
        Self { value: 0, decimals }
    }

    fn scale(decimals: u8) -> Result<i128, FundError> {
        if decimals > 38 {
            return Err(FundError::DecimalMismatch);
        }
        Ok(10i128.pow(decimals as u32))
    }

    pub fn from_float(value: f64, decimals: u8) -> Result<Self, FundError> {
        if !value.is_finite() {
            return Err(FundError::Overflow);
        }
        let multiplier = Self::scale(decimals)? as f64;
        let scaled = value * multiplier;
        if scaled > i128::MAX as f64 || scaled < i128::MIN as f64 {
            return Err(FundError::Overflow);
        }
        Ok(Self {
            value: scaled as i128,
            decimals,
        })
    }

    pub fn to_float(&self) -> f64 {
        self.value as f64 / 10f64.powi(self.decimals as i32)
    }

    fn ensure_same_scale(&self, other: &FixedDecimal) -> Result<(), FundError> {
        if self.decimals != other.decimals {
            return Err(FundError::DecimalMismatch);
        }
        Ok(())
    }

    pub fn add(&self, other: &FixedDecimal) -> Result<Self, FundError> {
        self.ensure_same_scale(other)?;
        let value = self.value.checked_add(other.value).ok_or(FundError::Overflow)?;
        Ok(Self {
            value,
            decimals: self.decimals,
        })
    }

    pub fn sub(&self, other: &FixedDecimal) -> Result<Self, FundError> {
        self.ensure_same_scale(other)?;
        let value = self.value.checked_sub(other.value).ok_or(FundError::Overflow)?;
        Ok(Self {
            value,
            decimals: self.decimals,
        })
    }

    pub fn mul(&self, other: &FixedDecimal) -> Result<Self, FundError> {
        self.ensure_same_scale(other)?;
        let divisor = Self::scale(self.decimals)?;
        let product = self.value.checked_mul(other.value).ok_or(FundError::Overflow)?;
        Ok(Self {
            value: product / divisor,
            decimals: self.decimals,
        })
    }

    pub fn is_zero(&self) -> bool {
        self.value == 0
    }

    pub fn is_negative(&self) -> bool {
        self.value < 0
    }
}

/// Kind of insurance event recorded in the log.
#[derive(Clone, Copy, Debug, PartialEq, Eq)]
pub enum InsuranceEventType {
    Deposit,
    Withdraw,
    LiquidationCoverage,
    HackCoverage,
    FeeAllocation,
    GovernanceGrant,
}

/// An immutable record of a fund movement.
#[derive(Clone, Debug)]
pub struct InsuranceEvent {
    pub event_id: u64,
    pub event_type: InsuranceEventType,
    pub amount: FixedDecimal,
    pub token: String,
    pub reason: String,
    pub timestamp: u64,
    pub tx_hash: Option<String>,
}

/// Maximum number of events retained in memory.
const MAX_EVENTS: usize = 10_000;

/// Insurance fund covering liquidations and protocol losses.
pub struct InsuranceFund {
    holdings: Arc<RwLock<HashMap<String, FixedDecimal>>>,
    daily_limit: FixedDecimal,
    today_spent: Arc<RwLock<FixedDecimal>>,
    last_reset_day: Arc<RwLock<u64>>,
    events: Arc<RwLock<Vec<InsuranceEvent>>>,
    next_event_id: Arc<RwLock<u64>>,
}

impl InsuranceFund {
    /// Creates a fund seeded with an initial `TGR` balance.
    pub fn new(initial_deposit: FixedDecimal, daily_limit: FixedDecimal) -> Self {
        let mut holdings = HashMap::new();
        if !initial_deposit.is_zero() {
            holdings.insert("TGR".to_string(), initial_deposit);
        }

        Self {
            holdings: Arc::new(RwLock::new(holdings)),
            daily_limit,
            today_spent: Arc::new(RwLock::new(FixedDecimal::zero(daily_limit.decimals))),
            last_reset_day: Arc::new(RwLock::new(current_day())),
            events: Arc::new(RwLock::new(Vec::new())),
            next_event_id: Arc::new(RwLock::new(1)),
        }
    }

    /// Deposits `amount` of `token`, returning the event id.
    pub fn deposit(&self, token: &str, amount: FixedDecimal) -> Result<u64, FundError> {
        if amount.is_negative() {
            return Err(FundError::Overflow);
        }

        {
            let mut holdings = self.holdings.write();
            let current = holdings
                .get(token)
                .copied()
                .unwrap_or_else(|| FixedDecimal::zero(amount.decimals));
            holdings.insert(token.to_string(), current.add(&amount)?);
        }

        self.record(
            InsuranceEventType::Deposit,
            amount,
            token,
            "Insurance fund deposit",
            None,
        )
    }

    /// Withdraws `amount` of `token` for a covered event.
    pub fn withdraw(
        &self,
        token: &str,
        amount: FixedDecimal,
        reason: &str,
        tx_hash: Option<String>,
    ) -> Result<u64, FundError> {
        if amount.is_negative() {
            return Err(FundError::Overflow);
        }

        // The holdings write lock is held across the whole check-and-debit, so a
        // concurrent withdrawal cannot pass the balance check against a balance
        // this call is about to spend. Reading the balance under a read lock and
        // writing it afterwards let two withdrawals of the full balance both
        // record a payout while the fund was debited once.
        let mut holdings = self.holdings.write();
        let current = holdings
            .get(token)
            .copied()
            .ok_or_else(|| FundError::UnknownToken(token.to_string()))?;

        if current.value < amount.value {
            return Err(FundError::InsufficientBalance);
        }

        let remaining = current.sub(&amount)?;
        self.reserve_daily_budget(&amount)?;

        holdings.insert(token.to_string(), remaining);
        drop(holdings);

        self.record(
            InsuranceEventType::Withdraw,
            amount,
            token,
            reason,
            tx_hash,
        )
    }

    /// Records a fee allocation without touching the daily spending limit.
    pub fn allocate_fees(&self, token: &str, amount: FixedDecimal) -> Result<u64, FundError> {
        self.deposit_with_type(token, amount, InsuranceEventType::FeeAllocation, "Protocol fee allocation")
    }

    fn deposit_with_type(
        &self,
        token: &str,
        amount: FixedDecimal,
        event_type: InsuranceEventType,
        reason: &str,
    ) -> Result<u64, FundError> {
        {
            let mut holdings = self.holdings.write();
            let current = holdings
                .get(token)
                .copied()
                .unwrap_or_else(|| FixedDecimal::zero(amount.decimals));
            holdings.insert(token.to_string(), current.add(&amount)?);
        }

        self.record(event_type, amount, token, reason, None)
    }

    pub fn balance(&self, token: &str) -> FixedDecimal {
        self.holdings
            .read()
            .get(token)
            .copied()
            .unwrap_or_else(|| FixedDecimal::zero(18))
    }

    /// Total value locked, summed across every token in the fund.
    pub fn total_value(&self) -> Result<FixedDecimal, FundError> {
        let holdings = self.holdings.read();
        let mut total = FixedDecimal::zero(18);

        for value in holdings.values() {
            if value.decimals != total.decimals {
                return Err(FundError::DecimalMismatch);
            }
            total = total.add(value)?;
        }

        Ok(total)
    }

    pub fn daily_limit(&self) -> FixedDecimal {
        self.daily_limit
    }

    pub fn spent_today(&self) -> FixedDecimal {
        self.reset_daily_counter();
        *self.today_spent.read()
    }

    pub fn remaining_today(&self) -> Result<FixedDecimal, FundError> {
        self.reset_daily_counter();
        let spent = *self.today_spent.read();
        self.daily_limit.sub(&spent)
    }

    /// Most recent `limit` events, oldest first.
    pub fn recent_events(&self, limit: usize) -> Vec<InsuranceEvent> {
        let events = self.events.read();
        let start = events.len().saturating_sub(limit);
        events[start..].to_vec()
    }

    pub fn event_count(&self) -> usize {
        self.events.read().len()
    }

    fn reset_daily_counter(&self) {
        let today = current_day();
        let mut last = self.last_reset_day.write();

        if today > *last {
            *last = today;
            let mut spent = self.today_spent.write();
            spent.value = 0;
        }
    }

    fn reserve_daily_budget(&self, amount: &FixedDecimal) -> Result<(), FundError> {
        self.reset_daily_counter();

        if amount.decimals != self.daily_limit.decimals {
            return Err(FundError::DecimalMismatch);
        }

        let mut spent = self.today_spent.write();
        let projected = spent.add(amount)?;

        if projected.value > self.daily_limit.value {
            return Err(FundError::DailyLimitExceeded);
        }

        *spent = projected;
        Ok(())
    }

    fn record(
        &self,
        event_type: InsuranceEventType,
        amount: FixedDecimal,
        token: &str,
        reason: &str,
        tx_hash: Option<String>,
    ) -> Result<u64, FundError> {
        let event_id = {
            let mut next = self.next_event_id.write();
            let id = *next;
            *next += 1;
            id
        };

        let mut events = self.events.write();
        events.push(InsuranceEvent {
            event_id,
            event_type,
            amount,
            token: token.to_string(),
            reason: reason.to_string(),
            timestamp: current_timestamp(),
            tx_hash,
        });

        if events.len() > MAX_EVENTS {
            let excess = events.len() - MAX_EVENTS;
            events.drain(..excess);
        }

        Ok(event_id)
    }
}

/// Secure key storage backed by AES-256-GCM.
pub struct SecureKeyStore {
    keys: RwLock<HashMap<String, EncryptedKey>>,
    encryptor: Encryptor,
}

/// An encrypted key plus its metadata.
#[derive(Clone)]
pub struct EncryptedKey {
    pub id: String,
    pub encrypted_data: Vec<u8>,
    pub created_at: u64,
    pub last_used: u64,
}

impl SecureKeyStore {
    /// Builds a store whose keys are sealed with a PBKDF2-derived master key.
    pub fn from_password(master_password: &str, salt: &[u8]) -> Self {
        Self {
            keys: RwLock::new(HashMap::new()),
            encryptor: Encryptor::from_password(master_password, salt),
        }
    }

    /// Builds a store from an already-derived 32-byte master key.
    pub fn from_key(master_key: &[u8; 32]) -> Self {
        Self {
            keys: RwLock::new(HashMap::new()),
            encryptor: Encryptor::new(master_key),
        }
    }

    /// Seals and stores `key_data` under `key_id`.
    pub fn store_key(&self, key_id: &str, key_data: &[u8]) -> Result<(), FundError> {
        let encrypted = self
            .encryptor
            .encrypt(key_data)
            .map_err(|_| FundError::CryptoFailure)?;

        let now = current_timestamp();
        self.keys.write().insert(
            key_id.to_string(),
            EncryptedKey {
                id: key_id.to_string(),
                encrypted_data: encrypted,
                created_at: now,
                last_used: now,
            },
        );

        Ok(())
    }

    /// Stores a `SecureKey`, returning the sealed bytes held in the store.
    pub fn store_secure_key(&self, key_id: &str, key: &SecureKey) -> Result<(), FundError> {
        self.store_key(key_id, key.as_bytes())
    }

    /// Unseals the key stored under `key_id`.
    pub fn get_key(&self, key_id: &str) -> Result<Vec<u8>, FundError> {
        let encrypted = {
            let keys = self.keys.read();
            keys.get(key_id).cloned().ok_or(FundError::KeyNotFound)?
        };

        let plaintext = self
            .encryptor
            .decrypt(&encrypted.encrypted_data)
            .map_err(|_| FundError::CryptoFailure)?;

        if let Some(entry) = self.keys.write().get_mut(key_id) {
            entry.last_used = current_timestamp();
        }

        Ok(plaintext)
    }

    /// Unseals a 32-byte secret, rejecting any other length.
    pub fn get_secret_32(&self, key_id: &str) -> Result<[u8; 32], FundError> {
        let bytes = self.get_key(key_id)?;
        if bytes.len() != 32 {
            return Err(FundError::InvalidKeyMaterial);
        }
        let mut secret = [0u8; 32];
        secret.copy_from_slice(&bytes);
        Ok(secret)
    }

    pub fn delete_key(&self, key_id: &str) -> bool {
        self.keys.write().remove(key_id).is_some()
    }

    pub fn list_keys(&self) -> Vec<String> {
        self.keys.read().keys().cloned().collect()
    }

    pub fn contains(&self, key_id: &str) -> bool {
        self.keys.read().contains_key(key_id)
    }
}

impl Drop for SecureKeyStore {
    fn drop(&mut self) {
        for entry in self.keys.get_mut().values_mut() {
            entry.encrypted_data.zeroize();
        }
    }
}

/// Builds a `SecureKey` of the requested type for storage.
pub fn new_key(key_data: Vec<u8>, key_type: KeyType) -> SecureKey {
    SecureKey::new(key_data, key_type)
}

/// Seconds since the Unix epoch.
pub fn current_timestamp() -> u64 {
    std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .map(|duration| duration.as_secs())
        .unwrap_or(0)
}

fn current_day() -> u64 {
    current_timestamp() / 86_400
}

#[cfg(test)]
mod tests {
    use super::*;

    fn fund() -> InsuranceFund {
        InsuranceFund::new(
            FixedDecimal::new(1_000_000, 18),
            FixedDecimal::new(1_000_000, 18),
        )
    }

    #[test]
    fn fixed_decimal_arithmetic() {
        let a = FixedDecimal::new(1000, 18);
        let b = FixedDecimal::new(500, 18);

        assert_eq!(a.add(&b).unwrap().value, 1500);
        assert_eq!(a.sub(&b).unwrap().value, 500);
// 2.0 * 3.0 == 6.0 in 18-decimal fixed point.
        let two = FixedDecimal::new(2 * 10i128.pow(18), 18);
        let three = FixedDecimal::new(3 * 10i128.pow(18), 18);
        assert_eq!(two.mul(&three).unwrap().value, 6 * 10i128.pow(18));
    }

    #[test]
    fn fixed_decimal_rejects_scale_mismatch() {
        let a = FixedDecimal::new(1000, 18);
        let b = FixedDecimal::new(500, 6);
        assert_eq!(a.add(&b).unwrap_err(), FundError::DecimalMismatch);
    }

    #[test]
    fn fixed_decimal_detects_overflow() {
        let a = FixedDecimal::new(i128::MAX, 18);
        let b = FixedDecimal::new(1, 18);
        assert_eq!(a.add(&b).unwrap_err(), FundError::Overflow);
    }

    #[test]
    fn deposit_and_withdraw_update_balance() {
        let fund = fund();
        assert_eq!(fund.balance("TGR").value, 1_000_000);

        fund.deposit("TGR", FixedDecimal::new(100_000, 18)).unwrap();
        assert_eq!(fund.balance("TGR").value, 1_100_000);

        fund.withdraw(
            "TGR",
            FixedDecimal::new(50_000, 18),
            "Liquidation coverage",
            None,
        )
        .unwrap();
        assert_eq!(fund.balance("TGR").value, 1_050_000);
    }

    #[test]
    fn withdraw_beyond_balance_is_rejected() {
        let fund = InsuranceFund::new(
            FixedDecimal::new(100, 18),
            FixedDecimal::new(1_000_000, 18),
        );
        assert_eq!(
            fund.withdraw("TGR", FixedDecimal::new(200, 18), "test", None)
                .unwrap_err(),
            FundError::InsufficientBalance
        );
    }

    #[test]
    fn withdraw_of_unknown_token_is_rejected() {
        let fund = fund();
        assert!(matches!(
            fund.withdraw("NOPE", FixedDecimal::new(1, 18), "test", None)
                .unwrap_err(),
            FundError::UnknownToken(_)
        ));
    }

    /// Two threads withdrawing the whole balance at once must not both succeed.
    /// The balance check and the debit are one atomic step, so exactly one
    /// withdrawal is recorded and the fund never goes negative.
    #[test]
    fn concurrent_full_balance_withdrawals_settle_once() {
        use std::sync::Arc;

        let fund = Arc::new(InsuranceFund::new(
            FixedDecimal::new(100, 18),
            FixedDecimal::new(1_000_000, 18),
        ));
        let full = FixedDecimal::new(100, 18);

        let mut handles = Vec::new();
        for _ in 0..8 {
            let f = Arc::clone(&fund);
            let amount = full;
            handles.push(std::thread::spawn(move || {
                f.withdraw("TGR", amount, "race", None).is_ok()
            }));
        }

        let successes = handles
            .into_iter()
            .map(|h| h.join().expect("thread panicked"))
            .filter(|succeeded| *succeeded)
            .count();

        assert_eq!(successes, 1, "exactly one full-balance withdrawal may succeed");
        assert_eq!(fund.balance("TGR").value, 0);
        assert!(
            fund.balance("TGR").value >= 0,
            "the fund must never be overdrawn"
        );
        // One withdrawal event, no double payout recorded.
        let withdrawals = fund
            .recent_events(64)
            .into_iter()
            .filter(|e| e.event_type == InsuranceEventType::Withdraw)
            .count();
        assert_eq!(withdrawals, 1, "exactly one withdrawal event is recorded");
    }

    #[test]
    fn daily_limit_is_enforced() {
        let fund = InsuranceFund::new(
            FixedDecimal::new(10_000, 18),
            FixedDecimal::new(1_000, 18),
        );

        fund.withdraw("TGR", FixedDecimal::new(600, 18), "a", None).unwrap();
        assert_eq!(
            fund.withdraw("TGR", FixedDecimal::new(600, 18), "b", None)
                .unwrap_err(),
            FundError::DailyLimitExceeded
        );

        assert_eq!(fund.spent_today().value, 600);
        assert_eq!(fund.remaining_today().unwrap().value, 400);
    }

    #[test]
    fn deposit_does_not_consume_the_daily_limit() {
        let fund = InsuranceFund::new(
            FixedDecimal::zero(18),
            FixedDecimal::new(1_000, 18),
        );
        fund.deposit("TGR", FixedDecimal::new(5_000, 18)).unwrap();
        assert_eq!(fund.spent_today().value, 0);
    }

    #[test]
    fn fee_allocation_is_recorded_as_a_deposit_event() {
        let fund = InsuranceFund::new(FixedDecimal::zero(18), FixedDecimal::new(1_000, 18));
        fund.allocate_fees("RUSD", FixedDecimal::new(250, 18)).unwrap();

        let events = fund.recent_events(10);
        assert_eq!(events.len(), 1);
        assert_eq!(events[0].event_type, InsuranceEventType::FeeAllocation);
        assert_eq!(fund.balance("RUSD").value, 250);
    }

    #[test]
    fn event_ids_are_monotonic() {
        let fund = fund();
        let first = fund.deposit("TGR", FixedDecimal::new(1, 18)).unwrap();
        let second = fund.deposit("TGR", FixedDecimal::new(1, 18)).unwrap();
        assert!(second > first);
        assert_eq!(fund.event_count(), 2);
    }

    #[test]
    fn total_value_sums_holdings() {
        let fund = InsuranceFund::new(FixedDecimal::zero(18), FixedDecimal::new(1_000_000, 18));
        fund.deposit("TGR", FixedDecimal::new(100, 18)).unwrap();
        fund.deposit("RUSD", FixedDecimal::new(250, 18)).unwrap();
        assert_eq!(fund.total_value().unwrap().value, 350);
    }

    #[test]
    fn secure_key_store_round_trips() {
        let store = SecureKeyStore::from_password("correct horse battery staple", b"tigerswap-salt");
        store.store_key("signer", &[7u8; 32]).unwrap();

        assert_eq!(store.get_key("signer").unwrap(), vec![7u8; 32]);
        assert_eq!(store.get_secret_32("signer").unwrap(), [7u8; 32]);
    }

    #[test]
    fn secure_key_store_does_not_store_plaintext() {
        let store = SecureKeyStore::from_key(&[9u8; 32]);
        let secret = [0xABu8; 32];
        store.store_key("k", &secret).unwrap();

        let stored = store.keys.read().get("k").cloned().unwrap();
        assert_ne!(stored.encrypted_data, secret.to_vec());
        // AES-GCM output is nonce (12 bytes) plus ciphertext and a 16-byte tag.
        assert_eq!(stored.encrypted_data.len(), 12 + secret.len() + 16);
    }

    #[test]
    fn secure_key_store_rejects_wrong_password() {
        let store = SecureKeyStore::from_password("password-one", b"salt");
        store.store_key("k", &[1u8; 32]).unwrap();

        let attacker = SecureKeyStore::from_password("password-two", b"salt");
        // A different master key cannot open the sealed material.
        let sealed = store.keys.read().get("k").cloned().unwrap();
        attacker
            .keys
            .write()
            .insert("k".to_string(), sealed);
        assert_eq!(attacker.get_key("k").unwrap_err(), FundError::CryptoFailure);
    }

    #[test]
    fn secure_key_store_delete_and_list() {
        let store = SecureKeyStore::from_key(&[3u8; 32]);
        store.store_key("a", &[1u8; 32]).unwrap();
        store.store_key("b", &[2u8; 32]).unwrap();

        let mut ids = store.list_keys();
        ids.sort();
        assert_eq!(ids, vec!["a".to_string(), "b".to_string()]);

        assert!(store.delete_key("a"));
        assert!(!store.delete_key("a"));
        assert!(store.contains("b"));
        assert_eq!(store.get_key("a").unwrap_err(), FundError::KeyNotFound);
    }

    #[test]
    fn secure_key_store_rejects_wrong_secret_length() {
        let store = SecureKeyStore::from_key(&[4u8; 32]);
        store.store_key("short", &[1u8; 8]).unwrap();
        assert_eq!(
            store.get_secret_32("short").unwrap_err(),
            FundError::InvalidKeyMaterial
        );
    }

    #[test]
    fn secure_key_store_handles_secure_key_type() {
        let store = SecureKeyStore::from_key(&[5u8; 32]);
        let key = new_key(vec![0x42u8; 32], KeyType::EthKey);
        store.store_secure_key("signer", &key).unwrap();
        assert_eq!(store.get_key("signer").unwrap(), vec![0x42u8; 32]);
    }
}

#[cfg(test)]
mod regression_tests {
    use super::*;

    /// A withdrawal rejected for insufficient balance must not consume the daily
    /// spending allowance. It previously did, letting a caller exhaust the limit
    /// with requests that could never succeed and block real coverage.
    #[test]
    fn failed_withdraw_does_not_consume_daily_budget() {
        let fund = InsuranceFund::new(FixedDecimal::new(100, 18), FixedDecimal::new(1_000, 18));

        assert_eq!(
            fund.withdraw("TGR", FixedDecimal::new(500, 18), "oversize", None),
            Err(FundError::InsufficientBalance)
        );
        assert_eq!(fund.spent_today().value, 0, "no budget consumed on failure");
        assert_eq!(fund.balance("TGR").value, 100, "holdings untouched");

        // The full allowance is still available for a legitimate withdrawal.
        assert!(fund.withdraw("TGR", FixedDecimal::new(50, 18), "legit", None).is_ok());
        assert_eq!(fund.spent_today().value, 50);
        assert_eq!(fund.balance("TGR").value, 50);
    }

    /// An unknown token must also fail without spending budget.
    #[test]
    fn unknown_token_withdraw_does_not_consume_daily_budget() {
        let fund = InsuranceFund::new(FixedDecimal::new(100, 18), FixedDecimal::new(1_000, 18));
        assert_eq!(
            fund.withdraw("NOPE", FixedDecimal::new(1, 18), "x", None),
            Err(FundError::UnknownToken("NOPE".to_string()))
        );
        assert_eq!(fund.spent_today().value, 0);
    }
}
