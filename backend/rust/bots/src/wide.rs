//! 256-bit intermediate arithmetic for scaled multiply and divide.
//!
//! A price scaled by `10^18` and a quantity scaled by `10^18` have a product
//! around `10^45`, which does not fit in `i128` (about `1.7 * 10^38`). Doing the
//! multiply first and dividing afterwards would therefore overflow on entirely
//! ordinary values: a 1% price move computes `600 * 10^18 * 10^18`, which
//! overflows. These helpers form the full 256-bit product and divide it down,
//! so the result is exact whenever it fits.

const LIMB: u128 = 1 << 64;
const LIMB_MASK: u128 = LIMB - 1;

/// Multiplies two 128-bit values into a 256-bit product, returned high-first.
///
/// The inner accumulation cannot overflow: the largest partial sum is
/// `(2^64 - 1)^2 + (2^64 - 1) + (2^64 - 1) = 2^128 - 1`, which is exactly
/// `u128::MAX`.
fn wide_mul(x: u128, y: u128) -> (u128, u128) {
    let x_limbs = [x & LIMB_MASK, x >> 64];
    let y_limbs = [y & LIMB_MASK, y >> 64];
    let mut acc = [0u128; 4];

    for i in 0..2 {
        let mut carry = 0u128;
        for j in 0..2 {
            let current = acc[i + j] + x_limbs[i] * y_limbs[j] + carry;
            acc[i + j] = current & LIMB_MASK;
            carry = current >> 64;
        }
        acc[i + 2] = carry;
    }

    let lo = acc[0] | (acc[1] << 64);
    let hi = acc[2] | (acc[3] << 64);
    (hi, lo)
}

/// Divides a 256-bit value by a 128-bit divisor, returning the 256-bit
/// quotient high-first. The remainder is discarded.
///
/// Binary long division. The remainder stays below the divisor, so the shift
/// below can lose the top bit; that bit is captured in `overflow` and folded
/// back in by the wrapping subtraction.
fn div_wide(hi: u128, lo: u128, divisor: u128) -> Option<(u128, u128)> {
    if divisor == 0 {
        return None;
    }
    if hi == 0 {
        return Some((0, lo / divisor));
    }

    let mut remainder: u128 = 0;
    let mut quotient_hi: u128 = 0;
    let mut quotient_lo: u128 = 0;

    for bit_index in (0..256).rev() {
        let bit = if bit_index >= 128 {
            (hi >> (bit_index - 128)) & 1
        } else {
            (lo >> bit_index) & 1
        };

        let overflow = remainder >> 127;
        remainder = (remainder << 1) | bit;

        let mut quotient_bit = 0u128;
        if overflow != 0 || remainder >= divisor {
            remainder = remainder.wrapping_sub(divisor);
            quotient_bit = 1;
        }

        if bit_index >= 128 {
            quotient_hi = (quotient_hi << 1) | quotient_bit;
        } else {
            quotient_lo = (quotient_lo << 1) | quotient_bit;
        }
    }

    Some((quotient_hi, quotient_lo))
}

/// Computes `a * b / d` exactly, truncating toward zero.
///
/// Returns `None` when `d` is zero or when the result does not fit in `i128`,
/// so a caller can never observe a wrapped value.
pub fn mul_div(a: i128, b: i128, d: i128) -> Option<i128> {
    if d == 0 {
        return None;
    }

    let negative = (a < 0) ^ (b < 0) ^ (d < 0);
    let (product_hi, product_lo) = wide_mul(a.unsigned_abs(), b.unsigned_abs());
    let (quotient_hi, quotient_lo) = div_wide(product_hi, product_lo, d.unsigned_abs())?;

    if quotient_hi != 0 {
        return None;
    }

    const MIN_MAGNITUDE: u128 = (i128::MAX as u128) + 1;

    if negative {
        if quotient_lo > MIN_MAGNITUDE {
            return None;
        }
        if quotient_lo == MIN_MAGNITUDE {
            return Some(i128::MIN);
        }
        Some(-(quotient_lo as i128))
    } else {
        if quotient_lo > i128::MAX as u128 {
            return None;
        }
        Some(quotient_lo as i128)
    }
}
