//! Fixed-point arithmetic for bot pricing and sizing.
//!
//! Bots compare prices and compute sizes on every tick, so the arithmetic must
//! be exact and must never depend on the host float unit. `Fixed` is a scaled
//! `i128`; every operation reports overflow instead of wrapping.

use std::fmt;

/// An error raised by a bot or by the arithmetic it performs.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum BotError {
    /// A scaled operation overflowed `i128`.
    Overflow,
    /// The two operands did not share a scale.
    ScaleMismatch,
    /// Division by zero.
    DivisionByZero,
    /// A value fell outside the range the caller allows.
    OutOfRange(&'static str),
    /// The configuration was rejected.
    InvalidConfig(&'static str),
    /// The market snapshot was unusable.
    InvalidMarket(&'static str),
    /// The bot has no work to do on this tick.
    NoAction,
}

impl fmt::Display for BotError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            BotError::Overflow => write!(f, "arithmetic overflow"),
            BotError::ScaleMismatch => write!(f, "scale mismatch"),
            BotError::DivisionByZero => write!(f, "division by zero"),
            BotError::OutOfRange(what) => write!(f, "{what} out of range"),
            BotError::InvalidConfig(what) => write!(f, "invalid config: {what}"),
            BotError::InvalidMarket(what) => write!(f, "invalid market: {what}"),
            BotError::NoAction => write!(f, "no action"),
        }
    }
}

impl std::error::Error for BotError {}

pub type BotResult<T> = Result<T, BotError>;

/// The scale every price and quantity in this crate uses.
pub const SCALE: u32 = 18;

const SCALE_FACTOR: i128 = 10i128.pow(SCALE);

/// A fixed-point decimal scaled by `10^18`.
#[derive(Clone, Copy, Debug, PartialEq, Eq, PartialOrd, Ord, Default)]
pub struct Fixed {
    value: i128,
}

impl Fixed {
    pub const ZERO: Fixed = Fixed { value: 0 };
    pub const ONE: Fixed = Fixed { value: SCALE_FACTOR };

    /// Builds a value from its raw scaled representation.
    pub const fn from_raw(value: i128) -> Self {
        Self { value }
    }

    /// The raw scaled representation.
    pub const fn raw(self) -> i128 {
        self.value
    }

    /// Builds a value from a whole number.
    pub fn from_integer(whole: i128) -> BotResult<Self> {
        whole
            .checked_mul(SCALE_FACTOR)
            .map(Self::from_raw)
            .ok_or(BotError::Overflow)
    }

    /// Builds a value from a numerator and denominator, rounding toward zero.
    ///
    /// Used for ratios such as fee fractions, where the exact decimal
    /// expansion does not terminate.
    pub fn ratio(numerator: i128, denominator: i128) -> BotResult<Self> {
        if denominator == 0 {
            return Err(BotError::DivisionByZero);
        }
        numerator
            .checked_mul(SCALE_FACTOR)
            .ok_or(BotError::Overflow)
            .map(|scaled| Self::from_raw(scaled / denominator))
    }

    /// Parses a decimal string such as `"0.001"` or `"-12.5"`.
    ///
    /// Rejects anything that is not a plain decimal so a malformed operator
    /// setting cannot silently become zero.
    pub fn parse(text: &str) -> BotResult<Self> {
        let text = text.trim();
        if text.is_empty() {
            return Err(BotError::OutOfRange("empty decimal"));
        }

        let (negative, digits) = match text.strip_prefix('-') {
            Some(rest) => (true, rest),
            None => (false, text),
        };

        let (whole_text, fraction_text) = match digits.split_once('.') {
            Some((whole, fraction)) => (whole, fraction),
            None => (digits, ""),
        };

        if whole_text.is_empty() && fraction_text.is_empty() {
            return Err(BotError::OutOfRange("decimal has no digits"));
        }
        if !whole_text.chars().all(|c| c.is_ascii_digit())
            || !fraction_text.chars().all(|c| c.is_ascii_digit())
        {
            return Err(BotError::OutOfRange("decimal has non-digit characters"));
        }
        if fraction_text.len() as u32 > SCALE {
            return Err(BotError::OutOfRange("decimal has too many places"));
        }

        let whole: i128 = if whole_text.is_empty() {
            0
        } else {
            whole_text.parse().map_err(|_| BotError::Overflow)?
        };

        let mut fraction: i128 = if fraction_text.is_empty() {
            0
        } else {
            fraction_text.parse().map_err(|_| BotError::Overflow)?
        };

        // Pad the fraction out to the full scale.
        for _ in fraction_text.len() as u32..SCALE {
            fraction = fraction.checked_mul(10).ok_or(BotError::Overflow)?;
        }

        let scaled = whole
            .checked_mul(SCALE_FACTOR)
            .and_then(|w| w.checked_add(fraction))
            .ok_or(BotError::Overflow)?;

        Ok(Self::from_raw(if negative { -scaled } else { scaled }))
    }

    pub fn is_zero(self) -> bool {
        self.value == 0
    }

    pub fn is_negative(self) -> bool {
        self.value < 0
    }

    pub fn is_positive(self) -> bool {
        self.value > 0
    }

    pub fn abs(self) -> BotResult<Self> {
        self.value
            .checked_abs()
            .map(Self::from_raw)
            .ok_or(BotError::Overflow)
    }

    pub fn add(self, other: Self) -> BotResult<Self> {
        self.value
            .checked_add(other.value)
            .map(Self::from_raw)
            .ok_or(BotError::Overflow)
    }

    pub fn sub(self, other: Self) -> BotResult<Self> {
        self.value
            .checked_sub(other.value)
            .map(Self::from_raw)
            .ok_or(BotError::Overflow)
    }

    /// Multiplies two values, rounding toward zero.
    ///
    /// The product is formed in 256 bits before the scale is divided out, so
    /// ordinary magnitudes such as `60000 * 0.01` cannot overflow.
    pub fn mul(self, other: Self) -> BotResult<Self> {
        crate::wide::mul_div(self.value, other.value, SCALE_FACTOR)
            .map(Self::from_raw)
            .ok_or(BotError::Overflow)
    }

    /// Divides two values, rounding toward zero.
    pub fn div(self, other: Self) -> BotResult<Self> {
        if other.value == 0 {
            return Err(BotError::DivisionByZero);
        }
        crate::wide::mul_div(self.value, SCALE_FACTOR, other.value)
            .map(Self::from_raw)
            .ok_or(BotError::Overflow)
    }

    /// Applies a basis-point adjustment: `self * (10_000 + bps) / 10_000`.
    ///
    /// A negative `bps` reduces the value, which is how a bot derives a bid
    /// below the mid from a positive spread.
    pub fn apply_bps(self, bps: i64) -> BotResult<Self> {
        let numerator = 10_000i128
            .checked_add(bps as i128)
            .ok_or(BotError::Overflow)?;
        if numerator < 0 {
            return Err(BotError::OutOfRange("bps adjustment makes value negative"));
        }
        let scaled = self
            .value
            .checked_mul(numerator)
            .ok_or(BotError::Overflow)?;
        Ok(Self::from_raw(scaled / 10_000))
    }

    /// Converts a whole-number basis-point count into a `Fixed` fraction.
    ///
    /// `Fixed` stores fractions, so `10_000` basis points is `1`. Comparing a
    /// `Fixed` result against a raw `i64` basis-point count without this
    /// conversion compares a scaled value against a plain integer, which is the
    /// wrong magnitude by `10^18`.
    pub fn bps(count: i64) -> BotResult<Self> {
        Self::ratio(count as i128, 10_000)
    }

    /// Converts a `Fixed` fraction back to a whole basis-point count.
    ///
    /// Truncates toward zero, which is the safe direction when the result
    /// gates a trade: a 0.9 bp edge reports as 0 and is refused.
    pub fn to_bps(self) -> BotResult<i64> {
        let scaled = self.mul(Self::from_integer(10_000)?)?;
        i64::try_from(scaled.raw() / SCALE_FACTOR).map_err(|_| BotError::Overflow)
    }

    /// Clamps into `[low, high]`.
    pub fn clamp(self, low: Self, high: Self) -> BotResult<Self> {
        if low > high {
            return Err(BotError::InvalidConfig("clamp bounds are inverted"));
        }
        Ok(if self < low {
            low
        } else if self > high {
            high
        } else {
            self
        })
    }
}

impl fmt::Display for Fixed {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let negative = self.value < 0;
        let magnitude = self.value.unsigned_abs();
        let factor = SCALE_FACTOR as u128;
        let whole = magnitude / factor;
        let fraction = magnitude % factor;
        if negative {
            write!(f, "-")?;
        }
        if fraction == 0 {
            return write!(f, "{whole}");
        }
        let mut fraction_text = format!("{fraction:0width$}", width = SCALE as usize);
        while fraction_text.ends_with('0') {
            fraction_text.pop();
        }
        write!(f, "{whole}.{fraction_text}")
    }
}
