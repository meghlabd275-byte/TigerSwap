//! Math utilities for AMM calculations
//! 
//! Uses Q96 and Q128 fixed-point arithmetic for precision

use num_bigint::{BigUint, BigInt};
use num_traits::{One, Zero};

/// Q96 constant (2^96)
pub const Q96: BigUint = BigUint::from(1u64) << 96;
/// Q128 constant (2^128)
pub const Q128: BigUint = BigUint::from(1u64) << 128;
/// Max uint256
pub const MAX_UINT256: BigUint = BigUint::parse_bytes(
    b"115792089237316195423570985008687907853269984665640564039457584007913129639935",
    10
).unwrap();

/// Full math library for precise calculations
pub struct FullMath;

impl FullMath {
    /// Multiply and divide with rounding up
    pub fn mul_div_rounding_up(a: &BigUint, b: &BigUint, divisor: &BigUint) -> BigUint {
        if divisor.is_zero() {
            panic!("Division by zero");
        }
        
        let result = a * b / divisor;
        if (a * b) % divisor > BigUint::zero() {
            result + BigUint::one()
        } else {
            result
        }
    }

    /// Multiply and divide rounding down
    pub fn mul_div_floor(a: &BigUint, b: &BigUint, divisor: &BigUint) -> BigUint {
        if divisor.is_zero() {
            panic!("Division by zero");
        }
        a * b / divisor
    }

    /// Clip ratio to prevent overflow
    pub fn clip_ratio(value: &BigInt, max_val: &BigInt) -> BigInt {
        if value > max_val {
            max_val.clone()
        } else if value < &-max_val.clone() {
            -max_val.clone()
        } else {
            value.clone()
        }
    }
}

/// Bit math utilities
pub struct BitMath;

impl BitMath {
    /// Find most significant bit
    pub fn most_significant_bit(x: &BigUint) -> u32 {
        if x.is_zero() {
            return 0;
        }
        
        let mut msb = 0u32;
        let mut x256 = x.clone();
        
        if &x256 >= &(BigUint::from(1u64) << 128) {
            x256 >>= 128;
            msb += 128;
        }
        if &x256 >= &(BigUint::from(1u64) << 64) {
            x256 >>= 64;
            msb += 64;
        }
        if &x256 >= &(BigUint::from(1u64) << 32) {
            x256 >>= 32;
            msb += 32;
        }
        if &x256 >= &(BigUint::from(1u64) << 16) {
            x256 >>= 16;
            msb += 16;
        }
        if &x256 >= &(BigUint::from(1u64) << 8) {
            x256 >>= 8;
            msb += 8;
        }
        if &x256 >= &(BigUint::from(1u64) << 4) {
            x256 >>= 4;
            msb += 4;
        }
        if &x256 >= &(BigUint::from(1u64) << 2) {
            x256 >>= 2;
            msb += 2;
        }
        if &x256 >= &(BigUint::from(1u64) << 1) {
            msb += 1;
        }
        
        msb
    }

    /// Find least significant bit
    pub fn least_significant_bit(x: &BigUint) -> u32 {
        if x.is_zero() {
            return 255;
        }
        
        let mut lsb = 0u32;
        let mut x256 = x.clone();
        
        while x256.is_even() {
            x256 >>= 1;
            lsb += 1;
        }
        
        lsb
    }
}

/// Price math utilities
pub struct PriceMath;

impl PriceMath {
    /// Get sqrt price from tick
    pub fn get_sqrt_price_at_tick(tick: i32) -> BigUint {
        let abs_tick = tick.abs() as u32;
        let mut ratio = BigUint::from(1u64) << 96;
        
        // Binary decomposition of tick
        if abs_tick & 0x01 != 0 { ratio = (ratio * BigUint::parse_bytes(b"0xfffcb933bd6fad37aa2d162d", 16).unwrap()) >> 96; }
        if abs_tick & 0x02 != 0 { ratio = (ratio * BigUint::parse_bytes(b"0xfffffffffffffffe5f83b8d41aecc0000", 16).unwrap()) >> 96; }
        if abs_tick & 0x04 != 0 { ratio = (ratio * BigUint::parse_bytes(b"0xffffffffffff993a3dc967a00048000000", 16).unwrap()) >> 96; }
        if abs_tick & 0x08 != 0 { ratio = (ratio * BigUint::parse_bytes(b"0xffffffffffeb1c7cd700006c6800000000", 16).unwrap()) >> 96; }
        if abs_tick & 0x10 != 0 { ratio = (ratio * BigUint::parse_bytes(b"0xfffe910d040000000000000000000000000", 16).unwrap()) >> 96; }
        if abs_tick & 0x20 != 0 { ratio = (ratio * BigUint::parse_bytes(b"0xfffc6ecf00000000000000000000000000000", 16).unwrap()) >> 96; }
        if abs_tick & 0x40 != 0 { ratio = (ratio * BigUint::parse_bytes(b"0xfffe8898000000000000000000000000000000", 16).unwrap()) >> 96; }
        if abs_tick & 0x80 != 0 { ratio = (ratio * BigUint::parse_bytes(b"0xfffc9b180000000000000000000000000000000", 16).unwrap()) >> 96; }
        if abs_tick & 0x100 != 0 { ratio = (ratio * BigUint::parse_bytes(b"0xfffc979d00000000000000000000000000000000", 16).unwrap()) >> 96; }
        if abs_tick & 0x200 != 0 { ratio = (ratio * BigUint::parse_bytes(b"0xfffc86c8000000000000000000000000000000000", 16).unwrap()) >> 96; }
        
        if tick >= 0 {
            ratio
        } else {
            Q96.clone() / ratio
        }
    }

    /// Get tick from sqrt price
    pub fn get_tick_at_sqrt_price(sqrt_price_x96: &BigUint) -> i32 {
        if sqrt_price_x96 < &Q96 {
            let ratio = &Q96 / sqrt_price_x96;
            let msb = BitMath::most_significant_bit(&ratio);
            -((msb as i32 - 96) * 2 + 1)
        } else {
            let msb = BitMath::most_significant_bit(sqrt_price_x96);
            (msb as i32 - 96) * 2
        }
    }

    /// Get amount0 delta
    pub fn get_amount0_delta(
        sqrt_ratio_ax96: &BigUint,
        sqrt_ratio_bx96: &BigUint,
        liquidity: &BigUint,
        round_up: bool,
    ) -> BigUint {
        let (ratio_a, ratio_b) = if sqrt_ratio_ax96 < sqrt_ratio_bx96 {
            (sqrt_ratio_ax96.clone(), sqrt_ratio_bx96.clone())
        } else {
            (sqrt_ratio_bx96.clone(), sqrt_ratio_ax96.clone())
        };

        let numerator1 = liquidity * Q96.clone();
        let numerator2 = ratio_b - ratio_a;

        if round_up {
            FullMath::mul_div_rounding_up(
                &FullMath::mul_div_rounding_up(&numerator1, &numerator2, &ratio_b),
                &BigUint::one(),
                &ratio_a,
            )
        } else {
            FullMath::mul_div_floor(&numerator1, &numerator2, &ratio_b) / &ratio_a
        }
    }

    /// Get amount1 delta
    pub fn get_amount1_delta(
        sqrt_ratio_ax96: &BigUint,
        sqrt_ratio_bx96: &BigUint,
        liquidity: &BigUint,
        round_up: bool,
    ) -> BigUint {
        let (ratio_a, ratio_b) = if sqrt_ratio_ax96 < sqrt_ratio_bx96 {
            (sqrt_ratio_ax96.clone(), sqrt_ratio_bx96.clone())
        } else {
            (sqrt_ratio_bx96.clone(), sqrt_ratio_ax96.clone())
        };

        if round_up {
            FullMath::mul_div_rounding_up(liquidity, &(ratio_b - ratio_a), &Q96)
        } else {
            FullMath::mul_div_floor(liquidity, &(ratio_b - ratio_a), &Q96)
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_mul_div_rounding_up() {
        let a = BigUint::from(10u64);
        let b = BigUint::from(3u64);
        let divisor = BigUint::from(3u64);
        
        let result = FullMath::mul_div_rounding_up(&a, &b, &divisor);
        assert_eq!(result, BigUint::from(10u64));
    }

    #[test]
    fn test_msb() {
        assert_eq!(BitMath::most_significant_bit(&BigUint::from(1u64) << 10), 10);
        assert_eq!(BitMath::most_significant_bit(&BigUint::from(1u64)), 0);
    }
}