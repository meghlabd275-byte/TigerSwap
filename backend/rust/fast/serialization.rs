//! TigerSwap High-Performance Serialization
//! Zero-copy encoding/decoding for trading data

use std::io::{Read, Write, Result};
use std::mem::size_of;

// ============== CONSTANTS ==============
const MAX_BUFFER_SIZE: usize = 1024 * 1024; // 1MB

// ============== BINARY ENCODER ==============

/// High-performance binary encoder with zero-copy semantics
pub struct BinaryEncoder {
    buffer: Vec<u8>,
    position: usize,
}

impl BinaryEncoder {
    pub fn new(capacity: usize) -> Self {
        Self {
            buffer: Vec::with_capacity(capacity),
            position: 0,
        }
    }
    
    pub fn with_buffer(buffer: Vec<u8>) -> Self {
        let position = 0;
        Self { buffer, position }
    }
    
    pub fn into_inner(self) -> Vec<u8> {
        self.buffer
    }
    
    pub fn capacity(&self) -> usize {
        self.buffer.capacity()
    }
    
    pub fn len(&self) -> usize {
        self.position
    }
    
    pub fn is_empty(&self) -> bool {
        self.position == 0
    }
    
    pub fn clear(&mut self) {
        self.position = 0;
        self.buffer.clear();
    }
    
    // Encode integers
    #[inline]
    pub fn encode_u8(&mut self, value: u8) {
        self.ensure_capacity(1);
        self.buffer[self.position] = value;
        self.position += 1;
    }
    
    #[inline]
    pub fn encode_i8(&mut self, value: i8) {
        self.encode_u8(value as u8);
    }
    
    #[inline]
    pub fn encode_u16(&mut self, value: u16) {
        self.ensure_capacity(2);
        self.buffer[self.position..self.position + 2].copy_from_slice(&value.to_le_bytes());
        self.position += 2;
    }
    
    #[inline]
    pub fn encode_i16(&mut self, value: i16) {
        self.encode_u16(value as u16);
    }
    
    #[inline]
    pub fn encode_u32(&mut self, value: u32) {
        self.ensure_capacity(4);
        self.buffer[self.position..self.position + 4].copy_from_slice(&value.to_le_bytes());
        self.position += 4;
    }
    
    #[inline]
    pub fn encode_i32(&mut self, value: i32) {
        self.encode_u32(value as u32);
    }
    
    #[inline]
    pub fn encode_u64(&mut self, value: u64) {
        self.ensure_capacity(8);
        self.buffer[self.position..self.position + 8].copy_from_slice(&value.to_le_bytes());
        self.position += 8;
    }
    
    #[inline]
    pub fn encode_i64(&mut self, value: i64) {
        self.encode_u64(value as u64);
    }
    
    #[inline]
    pub fn encode_f32(&mut self, value: f32) {
        self.encode_u32(value.to_bits());
    }
    
    #[inline]
    pub fn encode_f64(&mut self, value: f64) {
        self.encode_u64(value.to_bits());
    }
    
    // Varint encoding for variable-length integers
    #[inline]
    pub fn encode_varint(&mut self, mut value: u64) {
        loop {
            self.ensure_capacity(1);
            let mut byte = (value & 0x7F) as u8;
            value >>= 7;
            if value == 0 {
                self.buffer[self.position] = byte;
                self.position += 1;
                break;
            }
            byte |= 0x80;
            self.buffer[self.position] = byte;
            self.position += 1;
        }
    }
    
    // Encode bytes
    #[inline]
    pub fn encode_bytes(&mut self, value: &[u8]) {
        self.encode_varint(value.len() as u64);
        self.ensure_capacity(value.len());
        self.buffer[self.position..self.position + value.len()].copy_from_slice(value);
        self.position += value.len();
    }
    
    // Encode string
    #[inline]
    pub fn encode_str(&mut self, value: &str) {
        self.encode_bytes(value.as_bytes());
    }
    
    // Encode boolean
    #[inline]
    pub fn encode_bool(&mut self, value: bool) {
        self.encode_u8(if value { 1 } else { 0 });
    }
    
    #[inline]
    fn ensure_capacity(&mut self, additional: usize) {
        while self.position + additional > self.buffer.capacity() {
            self.buffer.reserve(256);
        }
    }
}

// ============== BINARY DECODER ==============

/// High-performance binary decoder
pub struct BinaryDecoder {
    buffer: Vec<u8>,
    position: usize,
}

impl BinaryDecoder {
    pub fn new(buffer: Vec<u8>) -> Self {
        Self { buffer, position: 0 }
    }
    
    pub fn from_slice(slice: &[u8]) -> Self {
        Self {
            buffer: slice.to_vec(),
            position: 0,
        }
    }
    
    pub fn remaining(&self) -> usize {
        self.buffer.len() - self.position
    }
    
    pub fn position(&self) -> usize {
        self.position
    }
    
    pub fn set_position(&mut self, pos: usize) {
        self.position = pos;
    }
    
    // Decode integers
    #[inline]
    pub fn decode_u8(&mut self) -> u8 {
        let value = self.buffer[self.position];
        self.position += 1;
        value
    }
    
    #[inline]
    pub fn decode_i8(&mut self) -> i8 {
        self.decode_u8() as i8
    }
    
    #[inline]
    pub fn decode_u16(&mut self) -> u16 {
        let value = u16::from_le_bytes([
            self.buffer[self.position],
            self.buffer[self.position + 1],
        ]);
        self.position += 2;
        value
    }
    
    #[inline]
    pub fn decode_i16(&mut self) -> i16 {
        self.decode_u16() as i16
    }
    
    #[inline]
    pub fn decode_u32(&mut self) -> u32 {
        let value = u32::from_le_bytes([
            self.buffer[self.position],
            self.buffer[self.position + 1],
            self.buffer[self.position + 2],
            self.buffer[self.position + 3],
        ]);
        self.position += 4;
        value
    }
    
    #[inline]
    pub fn decode_i32(&mut self) -> i32 {
        self.decode_u32() as i32
    }
    
    #[inline]
    pub fn decode_u64(&mut self) -> u64 {
        let mut bytes = [0u8; 8];
        bytes.copy_from_slice(&self.buffer[self.position..self.position + 8]);
        self.position += 8;
        u64::from_le_bytes(bytes)
    }
    
    #[inline]
    pub fn decode_i64(&mut self) -> i64 {
        self.decode_u64() as i64
    }
    
    #[inline]
    pub fn decode_f32(&mut self) -> f32 {
        f32::from_bits(self.decode_u32())
    }
    
    #[inline]
    pub fn decode_f64(&mut self) -> f64 {
        f64::from_bits(self.decode_u64())
    }
    
    // Varint decoding
    #[inline]
    pub fn decode_varint(&mut self) -> u64 {
        let mut result = 0u64;
        let mut shift = 0;
        
        loop {
            let byte = self.buffer[self.position];
            self.position += 1;
            result |= ((byte & 0x7F) as u64) << shift;
            
            if byte & 0x80 == 0 {
                break;
            }
            shift += 7;
        }
        
        result
    }
    
    // Decode bytes
    #[inline]
    pub fn decode_bytes(&mut self) -> Vec<u8> {
        let len = self.decode_varint() as usize;
        let data = self.buffer[self.position..self.position + len].to_vec();
        self.position += len;
        data
    }
    
    // Decode string
    #[inline]
    pub fn decode_str(&mut self) -> String {
        let bytes = self.decode_bytes();
        String::from_utf8(bytes).unwrap_or_default()
    }
    
    // Decode boolean
    #[inline]
    pub fn decode_bool(&mut self) -> bool {
        self.decode_u8() != 0
    }
}

// ============== MESSAGE PACKER ==============

/// Trait for serializable types
pub trait Serializable {
    fn serialize(&self, encoder: &mut BinaryEncoder);
    fn deserialize(decoder: &mut BinaryDecoder) -> Self;
}

/// Order serialization
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
    
    fn deserialize(decoder: &mut BinaryDecoder) -> Self {
        Self {
            order_id: decoder.decode_u64(),
            trader_id: decoder.decode_u64(),
            pair_id: decoder.decode_u64(),
            price: decoder.decode_u64(),
            quantity: decoder.decode_u64(),
            filled: decoder.decode_u64(),
            side: decoder.decode_u8(),
            order_type: decoder.decode_u8(),
            status: decoder.decode_u8(),
            created_at: decoder.decode_u64(),
        }
    }
}

#[derive(Debug, Clone)]
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

// ============== FFI EXPORTS ==============

#[no_mangle]
pub extern "C" fn create_encoder(capacity: usize) -> *mut BinaryEncoder {
    Box::into_raw(Box::new(BinaryEncoder::new(capacity)))
}

#[no_mangle]
pub extern "C" fn destroy_encoder(encoder: *mut BinaryEncoder) {
    if !encoder.is_null() {
        unsafe { Box::from_raw(encoder) };
    }
}

#[no_mangle]
pub extern "C" fn encoder_encode_u64(encoder: *mut BinaryEncoder, value: u64) {
    unsafe { (*encoder).encode_u64(value) };
}

#[no_mangle]
pub extern "C" fn encoder_encode_bytes(encoder: *mut BinaryEncoder, data: *const u8, len: usize) {
    unsafe {
        let slice = std::slice::from_raw_parts(data, len);
        (*encoder).encode_bytes(slice);
    }
}

#[no_mangle]
pub extern "C" fn encoder_into_inner(encoder: *mut BinaryEncoder) -> *mut Vec<u8> {
    unsafe {
        let encoder = Box::from_raw(encoder);
        Box::into_raw(Box::new(encoder.into_inner()))
    }
}

#[no_mangle]
pub extern "C" fn create_decoder(buffer: *mut Vec<u8>) -> *mut BinaryDecoder {
    unsafe {
        let buffer = Box::from_raw(buffer);
        Box::into_raw(Box::new(BinaryDecoder::new(*buffer)))
    }
}

#[no_mangle]
pub extern "C" fn destroy_decoder(decoder: *mut BinaryDecoder) {
    if !decoder.is_null() {
        unsafe { Box::from_raw(decoder) };
    }
}

#[no_mangle]
pub extern "C" fn decoder_decode_u64(decoder: *mut BinaryDecoder) -> u64 {
    unsafe { (*decoder).decode_u64() }
}

// ============== TESTS ==============

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_encode_decode_u64() {
        let mut encoder = BinaryEncoder::new(64);
        encoder.encode_u64(1234567890);
        
        let buffer = encoder.into_inner();
        let mut decoder = BinaryDecoder::new(buffer);
        
        assert_eq!(decoder.decode_u64(), 1234567890);
    }
    
    #[test]
    fn test_varint() {
        let mut encoder = BinaryEncoder::new(64);
        
        encoder.encode_varint(0);
        encoder.encode_varint(127);
        encoder.encode_varint(128);
        encoder.encode_varint(300);
        encoder.encode_varint(1000000);
        
        let buffer = encoder.into_inner();
        let mut decoder = BinaryDecoder::new(buffer);
        
        assert_eq!(decoder.decode_varint(), 0);
        assert_eq!(decoder.decode_varint(), 127);
        assert_eq!(decoder.decode_varint(), 128);
        assert_eq!(decoder.decode_varint(), 300);
        assert_eq!(decoder.decode_varint(), 1000000);
    }
    
    #[test]
    fn test_order_serialization() {
        let order = OrderData {
            order_id: 1,
            trader_id: 100,
            pair_id: 1,
            price: 50000,
            quantity: 100,
            filled: 0,
            side: 0,
            order_type: 1,
            status: 1,
            created_at: 1234567890,
        };
        
        let mut encoder = BinaryEncoder::new(256);
        order.serialize(&mut encoder);
        
        let buffer = encoder.into_inner();
        let mut decoder = BinaryDecoder::new(buffer);
        let decoded = OrderData::deserialize(&mut decoder);
        
        assert_eq!(order.order_id, decoded.order_id);
        assert_eq!(order.price, decoded.price);
    }
}
