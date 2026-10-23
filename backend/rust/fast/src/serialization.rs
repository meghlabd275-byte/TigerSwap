//! Binary codec for TigerSwap wire messages.
//!
//! The encoder is length-prefixed and little-endian; the decoder is strictly
//! bounds-checked so that malformed input returns an error instead of panicking.

use std::fmt;

/// Error returned when a decode operation reads past the end of the buffer.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DecodeError {
    pub needed: usize,
    pub remaining: usize,
}

impl fmt::Display for DecodeError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "buffer underflow: needed {} bytes but {} remain",
            self.needed, self.remaining
        )
    }
}

impl std::error::Error for DecodeError {}

/// Length-prefixed little-endian binary encoder.
#[derive(Default)]
pub struct BinaryEncoder {
    buffer: Vec<u8>,
}

impl BinaryEncoder {
    pub fn new(capacity: usize) -> Self {
        Self {
            buffer: Vec::with_capacity(capacity),
        }
    }

    pub fn with_buffer(buffer: Vec<u8>) -> Self {
        Self { buffer }
    }

    pub fn into_inner(self) -> Vec<u8> {
        self.buffer
    }

    pub fn as_slice(&self) -> &[u8] {
        &self.buffer
    }

    pub fn capacity(&self) -> usize {
        self.buffer.capacity()
    }

    pub fn len(&self) -> usize {
        self.buffer.len()
    }

    pub fn is_empty(&self) -> bool {
        self.buffer.is_empty()
    }

    pub fn clear(&mut self) {
        self.buffer.clear();
    }

    #[inline]
    pub fn encode_u8(&mut self, value: u8) {
        self.buffer.push(value);
    }

    #[inline]
    pub fn encode_i8(&mut self, value: i8) {
        self.buffer.push(value as u8);
    }

    #[inline]
    pub fn encode_u16(&mut self, value: u16) {
        self.buffer.extend_from_slice(&value.to_le_bytes());
    }

    #[inline]
    pub fn encode_i16(&mut self, value: i16) {
        self.buffer.extend_from_slice(&value.to_le_bytes());
    }

    #[inline]
    pub fn encode_u32(&mut self, value: u32) {
        self.buffer.extend_from_slice(&value.to_le_bytes());
    }

    #[inline]
    pub fn encode_i32(&mut self, value: i32) {
        self.buffer.extend_from_slice(&value.to_le_bytes());
    }

    #[inline]
    pub fn encode_u64(&mut self, value: u64) {
        self.buffer.extend_from_slice(&value.to_le_bytes());
    }

    #[inline]
    pub fn encode_i64(&mut self, value: i64) {
        self.buffer.extend_from_slice(&value.to_le_bytes());
    }

    #[inline]
    pub fn encode_f32(&mut self, value: f32) {
        self.buffer.extend_from_slice(&value.to_bits().to_le_bytes());
    }

    #[inline]
    pub fn encode_f64(&mut self, value: f64) {
        self.buffer.extend_from_slice(&value.to_bits().to_le_bytes());
    }

    /// Base-128 varint, least significant group first.
    #[inline]
    pub fn encode_varint(&mut self, mut value: u64) {
        loop {
            let byte = (value & 0x7F) as u8;
            value >>= 7;
            if value == 0 {
                self.buffer.push(byte);
                break;
            }
            self.buffer.push(byte | 0x80);
        }
    }

    /// Length-prefixed byte string.
    #[inline]
    pub fn encode_bytes(&mut self, value: &[u8]) {
        self.encode_varint(value.len() as u64);
        self.buffer.extend_from_slice(value);
    }

    #[inline]
    pub fn encode_str(&mut self, value: &str) {
        self.encode_bytes(value.as_bytes());
    }

    #[inline]
    pub fn encode_bool(&mut self, value: bool) {
        self.buffer.push(if value { 1 } else { 0 });
    }
}

/// Bounds-checked little-endian binary decoder.
pub struct BinaryDecoder<'a> {
    buffer: &'a [u8],
    position: usize,
}

impl<'a> BinaryDecoder<'a> {
    pub fn new(buffer: &'a [u8]) -> Self {
        Self { buffer, position: 0 }
    }

    pub fn remaining(&self) -> usize {
        self.buffer.len().saturating_sub(self.position)
    }

    pub fn position(&self) -> usize {
        self.position
    }

    pub fn is_empty(&self) -> bool {
        self.remaining() == 0
    }

    /// Moves the read cursor, returning false when it would pass the end.
    pub fn set_position(&mut self, position: usize) -> bool {
        if position > self.buffer.len() {
            return false;
        }
        self.position = position;
        true
    }

    fn take(&mut self, count: usize) -> Result<&'a [u8], DecodeError> {
        if self.remaining() < count {
            return Err(DecodeError {
                needed: count,
                remaining: self.remaining(),
            });
        }
        let slice = &self.buffer[self.position..self.position + count];
        self.position += count;
        Ok(slice)
    }

    pub fn decode_u8(&mut self) -> Result<u8, DecodeError> {
        Ok(self.take(1)?[0])
    }

    pub fn decode_i8(&mut self) -> Result<i8, DecodeError> {
        Ok(self.decode_u8()? as i8)
    }

    pub fn decode_u16(&mut self) -> Result<u16, DecodeError> {
        let mut bytes = [0u8; 2];
        bytes.copy_from_slice(self.take(2)?);
        Ok(u16::from_le_bytes(bytes))
    }

    pub fn decode_i16(&mut self) -> Result<i16, DecodeError> {
        Ok(self.decode_u16()? as i16)
    }

    pub fn decode_u32(&mut self) -> Result<u32, DecodeError> {
        let mut bytes = [0u8; 4];
        bytes.copy_from_slice(self.take(4)?);
        Ok(u32::from_le_bytes(bytes))
    }

    pub fn decode_i32(&mut self) -> Result<i32, DecodeError> {
        Ok(self.decode_u32()? as i32)
    }

    pub fn decode_u64(&mut self) -> Result<u64, DecodeError> {
        let mut bytes = [0u8; 8];
        bytes.copy_from_slice(self.take(8)?);
        Ok(u64::from_le_bytes(bytes))
    }

    pub fn decode_i64(&mut self) -> Result<i64, DecodeError> {
        Ok(self.decode_u64()? as i64)
    }

    pub fn decode_f32(&mut self) -> Result<f32, DecodeError> {
        Ok(f32::from_bits(self.decode_u32()?))
    }

    pub fn decode_f64(&mut self) -> Result<f64, DecodeError> {
        Ok(f64::from_bits(self.decode_u64()?))
    }

    pub fn decode_varint(&mut self) -> Result<u64, DecodeError> {
        let mut result = 0u64;
        let mut shift = 0u32;

        loop {
            if shift >= 64 {
                return Err(DecodeError {
                    needed: 1,
                    remaining: self.remaining(),
                });
            }
            let byte = self.decode_u8()?;
            result |= ((byte & 0x7F) as u64) << shift;
            if byte & 0x80 == 0 {
                break;
            }
            shift += 7;
        }

        Ok(result)
    }

    pub fn decode_bytes(&mut self) -> Result<Vec<u8>, DecodeError> {
        let length = self.decode_varint()? as usize;
        Ok(self.take(length)?.to_vec())
    }

    pub fn decode_str(&mut self) -> Result<String, DecodeError> {
        let bytes = self.decode_bytes()?;
        String::from_utf8(bytes).map_err(|_| DecodeError {
            needed: 0,
            remaining: self.remaining(),
        })
    }

    pub fn decode_bool(&mut self) -> Result<bool, DecodeError> {
        Ok(self.decode_u8()? != 0)
    }
}

/// A type that can be written to and read from the wire format.
pub trait Serializable: Sized {
    fn serialize(&self, encoder: &mut BinaryEncoder);
    fn deserialize(decoder: &mut BinaryDecoder<'_>) -> Result<Self, DecodeError>;
}

/// Order payload carried on the wire.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct OrderData {
    pub order_id: u64,
    pub trader_id: u64,
    pub pair_id: u64,
    pub price: u64,
    pub quantity: u64,
    pub filled: u64,
    pub side: u8,
    pub order_type: u8,
    pub status: u8,
    pub created_at: u64,
}

impl Serializable for OrderData {
    fn serialize(&self, encoder: &mut BinaryEncoder) {
        encoder.encode_u64(self.order_id);
        encoder.encode_u64(self.trader_id);
        encoder.encode_u64(self.pair_id);
        encoder.encode_u64(self.price);
        encoder.encode_u64(self.quantity);
        encoder.encode_u64(self.filled);
        encoder.encode_u8(self.side);
        encoder.encode_u8(self.order_type);
        encoder.encode_u8(self.status);
        encoder.encode_u64(self.created_at);
    }

    fn deserialize(decoder: &mut BinaryDecoder<'_>) -> Result<Self, DecodeError> {
        Ok(Self {
            order_id: decoder.decode_u64()?,
            trader_id: decoder.decode_u64()?,
            pair_id: decoder.decode_u64()?,
            price: decoder.decode_u64()?,
            quantity: decoder.decode_u64()?,
            filled: decoder.decode_u64()?,
            side: decoder.decode_u8()?,
            order_type: decoder.decode_u8()?,
            status: decoder.decode_u8()?,
            created_at: decoder.decode_u64()?,
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn round_trips_scalars() {
        let mut encoder = BinaryEncoder::new(64);
        encoder.encode_u8(7);
        encoder.encode_i8(-7);
        encoder.encode_u16(1000);
        encoder.encode_i16(-1000);
        encoder.encode_u32(4_000_000_000);
        encoder.encode_i32(-4_000_000);
        encoder.encode_u64(u64::MAX);
        encoder.encode_i64(-1234567890123);
        encoder.encode_f32(3.5);
        encoder.encode_f64(-2.25);
        encoder.encode_bool(true);
        encoder.encode_bool(false);

        let buffer = encoder.into_inner();
        let mut decoder = BinaryDecoder::new(&buffer);

        assert_eq!(decoder.decode_u8().unwrap(), 7);
        assert_eq!(decoder.decode_i8().unwrap(), -7);
        assert_eq!(decoder.decode_u16().unwrap(), 1000);
        assert_eq!(decoder.decode_i16().unwrap(), -1000);
        assert_eq!(decoder.decode_u32().unwrap(), 4_000_000_000);
        assert_eq!(decoder.decode_i32().unwrap(), -4_000_000);
        assert_eq!(decoder.decode_u64().unwrap(), u64::MAX);
        assert_eq!(decoder.decode_i64().unwrap(), -1234567890123);
        assert_eq!(decoder.decode_f32().unwrap(), 3.5);
        assert_eq!(decoder.decode_f64().unwrap(), -2.25);
        assert!(decoder.decode_bool().unwrap());
        assert!(!decoder.decode_bool().unwrap());
        assert!(decoder.is_empty());
    }

    #[test]
    fn varint_round_trips_boundaries() {
        let values = [0u64, 1, 127, 128, 300, 16_383, 16_384, u32::MAX as u64, u64::MAX];

        let mut encoder = BinaryEncoder::new(64);
        for value in values {
            encoder.encode_varint(value);
        }

        let buffer = encoder.into_inner();
        let mut decoder = BinaryDecoder::new(&buffer);
        for value in values {
            assert_eq!(decoder.decode_varint().unwrap(), value);
        }
    }

    #[test]
    fn varint_is_compact_for_small_values() {
        let mut encoder = BinaryEncoder::new(8);
        encoder.encode_varint(127);
        assert_eq!(encoder.len(), 1);
    }

    #[test]
    fn round_trips_bytes_and_strings() {
        let mut encoder = BinaryEncoder::new(32);
        encoder.encode_bytes(&[1, 2, 3, 4, 5]);
        encoder.encode_str("TigerSwap");

        let buffer = encoder.into_inner();
        let mut decoder = BinaryDecoder::new(&buffer);

        assert_eq!(decoder.decode_bytes().unwrap(), vec![1, 2, 3, 4, 5]);
        assert_eq!(decoder.decode_str().unwrap(), "TigerSwap");
    }

    #[test]
    fn round_trips_order_data() {
        let order = OrderData {
            order_id: 1,
            trader_id: 100,
            pair_id: 1,
            price: 50_000,
            quantity: 100,
            filled: 25,
            side: 0,
            order_type: 1,
            status: 2,
            created_at: 1_234_567_890,
        };

        let mut encoder = BinaryEncoder::new(256);
        order.serialize(&mut encoder);

        let buffer = encoder.into_inner();
        let mut decoder = BinaryDecoder::new(&buffer);
        assert_eq!(OrderData::deserialize(&mut decoder).unwrap(), order);
    }

    #[test]
    fn decoder_rejects_truncated_input() {
        let buffer = [1u8, 2, 3];
        let mut decoder = BinaryDecoder::new(&buffer);
        assert!(decoder.decode_u64().is_err());
        assert_eq!(decoder.position(), 0);
    }

    #[test]
    fn decoder_rejects_oversized_length_prefix() {
        let mut encoder = BinaryEncoder::new(16);
        encoder.encode_varint(1024);
        let buffer = encoder.into_inner();

        let mut decoder = BinaryDecoder::new(&buffer);
        assert!(decoder.decode_bytes().is_err());
    }

    #[test]
    fn decoder_rejects_invalid_utf8() {
        let mut encoder = BinaryEncoder::new(8);
        encoder.encode_bytes(&[0xFF, 0xFE]);
        let buffer = encoder.into_inner();

        let mut decoder = BinaryDecoder::new(&buffer);
        assert!(decoder.decode_str().is_err());
    }

    #[test]
    fn decoder_rejects_runaway_varint() {
        let buffer = [0x80u8; 12];
        let mut decoder = BinaryDecoder::new(&buffer);
        assert!(decoder.decode_varint().is_err());
    }

    #[test]
    fn set_position_is_bounds_checked() {
        let buffer = [1u8, 2, 3, 4];
        let mut decoder = BinaryDecoder::new(&buffer);
        assert!(decoder.set_position(2));
        assert_eq!(decoder.decode_u16().unwrap(), 0x0403);
        assert!(!decoder.set_position(99));
    }

    #[test]
    fn clear_resets_the_encoder() {
        let mut encoder = BinaryEncoder::new(16);
        encoder.encode_u64(1);
        assert_eq!(encoder.len(), 8);
        encoder.clear();
        assert!(encoder.is_empty());
    }
}
