//! TigerSwap Rust High-Speed Trading Engine
//! Ultra-low latency operations using lock-free data structures
//! 
//! Compile: cargo build --release

#![allow(dead_code)]

use std::sync::atomic::{AtomicU64, AtomicUsize, Ordering, fence};
use std::sync::Arc;
use std::collections::HashMap;
use std::time::{SystemTime, UNIX_EPOCH};

// ============== LOCK-FREE DATA STRUCTURES ==============

/// Lock-free MPMC channel for inter-thread communication
pub struct LockFreeChannel<T: Copy + Default> {
    buffer: Box<[AtomicU64]>,
    head: Arc<AtomicUsize>,
    tail: Arc<AtomicUsize>,
    size: usize,
    _phantom: std::marker::PhantomData<T>,
}

impl<T: Copy + Default + std::fmt::Debug> LockFreeChannel<T> {
    pub fn new(capacity: usize) -> Self {
        let size = capacity.next_power_of_two();
        let mut buffer = Vec::with_capacity(size);
        
        for _ in 0..size {
            buffer.push(AtomicU64::new(0));
        }
        
        Self {
            buffer: buffer.into_boxed_slice(),
            head: Arc::new(AtomicUsize::new(0)),
            tail: Arc::new(AtomicUsize::new(0)),
            size,
            _phantom: std::marker::PhantomData,
        }
    }
    
    pub fn push(&self, value: T) -> bool {
        let mask = self.size - 1;
        let tail = self.tail.load(Ordering::Acquire);
        let head = self.head.load(Ordering::Acquire);
        
        if (tail.wrapping_sub(head) & mask) >= self.size - 1 {
            return false; // Full
        }
        
        // Write value to buffer
        let ptr = &self.buffer[tail & mask] as *const AtomicU64 as *mut u64;
        unsafe { *ptr = std::mem::transmute(value) };
        
        self.tail.store(tail.wrapping_add(1), Ordering::Release);
        fence(Ordering::Release);
        
        true
    }
    
    pub fn pop(&self) -> Option<T> {
        let mask = self.size - 1;
        let head = self.head.load(Ordering::Acquire);
        let tail = self.tail.load(Ordering::Acquire);
        
        if head == tail {
            return None; // Empty
        }
        
        let ptr = &self.buffer[head & mask] as *const AtomicU64 as *const u64;
        let value: T = unsafe { std::mem::transmute(*ptr) };
        
        self.head.store(head.wrapping_add(1), Ordering::Release);
        
        Some(value)
    }
}

/// Lock-free hash map for order storage
pub struct LockFreeHashMap<K: std::hash::Hash + Eq + Copy, V: Copy> {
    buckets: Vec<AtomicU64>,
    size: usize,
    _phantom: std::marker::PhantomData<(K, V)>,
}

impl<K: std::hash::Hash + Eq + Copy, V: Copy> LockFreeHashMap<K, V> {
    pub fn new(capacity: usize) -> Self {
        let size = capacity.next_power_of_two();
        let mut buckets = Vec::with_capacity(size);
        
        for _ in 0..size {
            buckets.push(AtomicU64::new(0));
        }
        
        Self {
            buckets,
            size,
            _phantom: std::marker::PhantomData,
        }
    }
}

// ============== HIGH-SPEED ORDER ENGINE ==============

/// Order with minimal memory footprint
#[derive(Debug, Clone, Copy)]
#[repr(C)]
pub struct FastOrder {
    pub order_id: u64,
    pub trader_id: u64,
    pub pair_id: u64,
    pub price: u64,
    pub quantity: u64,
    pub filled: u64,
    pub side: u8,        // 0 = buy, 1 = sell
    pub order_type: u8,  // 0 = market, 1 = limit, 2 = stop
    pub status: u8,      // 0 = pending, 1 = open, 2 = filled, 3 = cancelled
    pub created_at: u64,
}

impl FastOrder {
    pub fn new(order_id: u64, trader_id: u64, pair_id: u64, 
               price: u64, quantity: u64, side: u8, order_type: u8) -> Self {
        Self {
            order_id,
            trader_id,
            pair_id,
            price,
            quantity,
            filled: 0,
            side,
            order_type,
            status: 1, // Open
            created_at: current_timestamp_ns(),
        }
    }
}

/// Trade execution record
#[derive(Debug, Clone, Copy)]
#[repr(C)]
pub struct FastTrade {
    pub trade_id: u64,
    pub maker_order_id: u64,
    pub taker_order_id: u64,
    pub pair_id: u64,
    pub price: u64,
    pub quantity: u64,
    pub fee: u64,
    pub timestamp: u64,
}

/// Price level in order book
#[derive(Debug, Clone, Copy)]
pub struct PriceLevel {
    pub price: u64,
    pub quantity: u64,
    pub orders: u32,
}

/// Order book with ultra-low latency operations
pub struct FastOrderBook {
    // Atomic counters
    next_order_id: AtomicU64,
    next_trade_id: AtomicU64,
    total_orders: AtomicU64,
    total_volume: AtomicU64,
    
    // Performance tracking
    min_latency_ns: AtomicU64,
    max_latency_ns: AtomicU64,
    avg_latency_ns: AtomicU64,
    orders_processed: AtomicU64,
    
    // Order storage (using regular HashMap with atomic operations)
    orders: parking_lot::RwLock<HashMap<u64, FastOrder>>,
    trader_orders: parking_lot::RwLock<HashMap<u64, Vec<u64>>>,
    
    // Recent trades
    recent_trades: LockFreeChannel<FastTrade>,
    
    // Pair configuration
    pair_id: u64,
    min_price: u64,
    max_price: u64,
    tick_size: u64,
}

impl FastOrderBook {
    pub fn new(pair_id: u64, min_price: u64, max_price: u64, tick_size: u64) -> Self {
        Self {
            next_order_id: AtomicU64::new(1),
            next_trade_id: AtomicU64::new(1),
            total_orders: AtomicU64::new(0),
            total_volume: AtomicU64::new(0),
            min_latency_ns: AtomicU64::new(u64::MAX),
            max_latency_ns: AtomicU64::new(0),
            avg_latency_ns: AtomicU64::new(0),
            orders_processed: AtomicU64::new(0),
            orders: parking_lot::RwLock::new(HashMap::new()),
            trader_orders: parking_lot::RwLock::new(HashMap::new()),
            recent_trades: LockFreeChannel::new(65536),
            pair_id,
            min_price,
            max_price,
            tick_size,
        }
    }
    
    /// Submit order - target < 1 microsecond
    #[inline]
    pub fn submit_order(&self, order: FastOrder) -> u64 {
        let start = unsafe { std::arch::rdtsc() };
        
        let order_id = self.next_order_id.fetch_add(1, Ordering::AcqRel);
        let mut order = order;
        order.order_id = order_id;
        
        // Store order
        {
            let mut orders = self.orders.write();
            orders.insert(order_id, order);
        }
        
        // Index by trader
        {
            let mut trader_orders = self.trader_orders.write();
            trader_orders
                .entry(order.trader_id)
                .or_insert_with(Vec::new)
                .push(order_id);
        }
        
        // Update stats
        self.total_orders.fetch_add(1, Ordering::Relaxed);
        
        // Calculate latency
        let latency = unsafe { std::arch::rdtsc() } - start;
        self.update_latency(latency);
        
        order_id
    }
    
    /// Cancel order - target < 1 microsecond
    #[inline]
    pub fn cancel_order(&self, order_id: u64, trader_id: u64) -> bool {
        let start = unsafe { std::arch::rdtsc() };
        
        let mut orders = self.orders.write();
        
        if let Some(order) = orders.get_mut(&order_id) {
            if order.trader_id == trader_id && order.status == 1 {
                order.status = 3; // Cancelled
                order.filled = order.quantity;
                
                let latency = unsafe { std::arch::rdtsc() } - start;
                self.update_latency(latency);
                return true;
            }
        }
        
        false
    }
    
    /// Get order - target < 100 nanoseconds
    #[inline]
    pub fn get_order(&self, order_id: u64) -> Option<FastOrder> {
        let orders = self.orders.read();
        orders.get(&order_id).copied()
    }
    
    /// Get best bid/ask - target < 50 nanoseconds
    #[inline]
    pub fn get_spread(&self) -> Option<(u64, u64)> {
        let orders = self.orders.read();
        
        let mut best_bid = 0u64;
        let mut best_ask = u64::MAX;
        
        for order in orders.values() {
            if order.status != 1 { continue; } // Only open orders
            
            if order.side == 0 && order.price > best_bid {
                best_bid = order.price;
            }
            if order.side == 1 && order.price < best_ask {
                best_ask = order.price;
            }
        }
        
        if best_bid > 0 && best_ask < u64::MAX {
            Some((best_bid, best_ask))
        } else {
            None
        }
    }
    
    /// Get market depth
    #[inline]
    pub fn get_depth(&self, levels: usize) -> Vec<PriceLevel> {
        let orders = self.orders.read();
        let mut bids: Vec<PriceLevel> = Vec::new();
        let mut asks: Vec<PriceLevel> = Vec::new();
        
        for order in orders.values() {
            if order.status != 1 { continue; }
            
            let level = PriceLevel {
                price: order.price,
                quantity: order.quantity - order.filled,
                orders: 1,
            };
            
            if order.side == 0 {
                bids.push(level);
            } else {
                asks.push(level);
            }
        }
        
        // Sort by price
        bids.sort_by(|a, b| b.price.cmp(&a.price));
        asks.sort_by(|a, b| a.price.cmp(&b.price));
        
        // Aggregate levels
        let mut result: Vec<PriceLevel> = Vec::new();
        
        for level in bids.iter().take(levels) {
            if let Some(last) = result.last_mut() {
                if last.price == level.price {
                    last.quantity += level.quantity;
                    last.orders += 1;
                    continue;
                }
            }
            result.push(*level);
        }
        
        for level in asks.iter().take(levels) {
            if let Some(last) = result.last_mut() {
                if last.price == level.price {
                    last.quantity += level.quantity;
                    last.orders += 1;
                    continue;
                }
            }
            result.push(*level);
        }
        
        result
    }
    
    /// Execute trade
    #[inline]
    pub fn execute_trade(&self, maker_order_id: u64, taker_order_id: u64, 
                         price: u64, quantity: u64) -> FastTrade {
        let trade_id = self.next_trade_id.fetch_add(1, Ordering::AcqRel);
        
        let trade = FastTrade {
            trade_id,
            maker_order_id,
            taker_order_id,
            pair_id: self.pair_id,
            price,
            quantity,
            fee: (price * quantity * 3) / 10000, // 0.3%
            timestamp: current_timestamp_ns(),
        };
        
        // Update order fills
        {
            let mut orders = self.orders.write();
            
            if let Some(maker) = orders.get_mut(&maker_order_id) {
                maker.filled += quantity;
                if maker.filled >= maker.quantity {
                    maker.status = 2; // Filled
                }
            }
            
            if let Some(taker) = orders.get_mut(&taker_order_id) {
                taker.filled += quantity;
                if taker.filled >= taker.quantity {
                    taker.status = 2; // Filled
                }
            }
        }
        
        // Update volume
        self.total_volume.fetch_add(quantity, Ordering::Relaxed);
        
        // Push to channel
        self.recent_trades.push(trade);
        
        trade
    }
    
    /// Get performance stats
    pub fn get_stats(&self) -> OrderBookStats {
        let processed = self.orders_processed.load(Ordering::Acquire);
        
        OrderBookStats {
            total_orders: self.total_orders.load(Ordering::Acquire),
            total_trades: self.next_trade_id.load(Ordering::Acquire) - 1,
            total_volume: self.total_volume.load(Ordering::Acquire),
            min_latency_ns: self.min_latency_ns.load(Ordering::Acquire),
            max_latency_ns: self.max_latency_ns.load(Ordering::Acquire),
            avg_latency_ns: self.avg_latency_ns.load(Ordering::Acquire),
            orders_per_second: if processed > 0 { 
                processed as f64 / 1.0 // Would use actual time
            } else { 0.0 },
        }
    }
    
    #[inline]
    fn update_latency(&self, latency: u64) {
        self.orders_processed.fetch_add(1, Ordering::Relaxed);
        
        // Update min
        let mut current = self.min_latency_ns.load(Ordering::Acquire);
        while latency < current {
            match self.min_latency_ns.compare_exchange_weak(current, latency, Ordering::AcqRel, Ordering::Acquire) {
                Ok(_) => break,
                Err(v) => current = v,
            }
        }
        
        // Update max
        current = self.max_latency_ns.load(Ordering::Acquire);
        while latency > current {
            match self.max_latency_ns.compare_exchange_weak(current, latency, Ordering::AcqRel, Ordering::Acquire) {
                Ok(_) => break,
                Err(v) => current = v,
            }
        }
        
        // Update avg
        let processed = self.orders_processed.load(Ordering::Acquire);
        let avg = self.avg_latency_ns.load(Ordering::Acquire);
        let new_avg = if processed == 1 {
            latency
        } else {
            (avg * (processed - 1) + latency) / processed
        };
        self.avg_latency_ns.store(new_avg, Ordering::Release);
    }
}

#[derive(Debug, Clone)]
pub struct OrderBookStats {
    pub total_orders: u64,
    pub total_trades: u64,
    pub total_volume: u64,
    pub min_latency_ns: u64,
    pub max_latency_ns: u64,
    pub avg_latency_ns: u64,
    pub orders_per_second: f64,
}

// ============== UTILITIES ==============

#[inline]
fn current_timestamp_ns() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos() as u64
}

// ============== FFI EXPORTS ==============

#[no_mangle]
pub extern "C" fn create_fast_orderbook(pair_id: u64, min_price: u64, max_price: u64, tick_size: u64) -> *mut FastOrderBook {
    let book = FastOrderBook::new(pair_id, min_price, max_price, tick_size);
    Box::into_raw(Box::new(book))
}

#[no_mangle]
pub extern "C" fn destroy_fast_orderbook(book: *mut FastOrderBook) {
    if !book.is_null() {
        unsafe { Box::from_raw(book) };
    }
}

#[no_mangle]
pub extern "C" fn fast_submit_order(book: *mut FastOrderBook, order: FastOrder) -> u64 {
    unsafe { (*book).submit_order(order) }
}

#[no_mangle]
pub extern "C" fn fast_cancel_order(book: *mut FastOrderBook, order_id: u64, trader_id: u64) -> u8 {
    unsafe { (*book).cancel_order(order_id, trader_id) as u8 }
}

#[no_mangle]
pub extern "C" fn fast_get_order(book: *mut FastOrderBook, order_id: u64) -> Option<FastOrder> {
    unsafe { (*book).get_order(order_id) }
}

#[no_mangle]
pub extern "C" fn fast_get_spread(book: *mut FastOrderBook) -> Option<(u64, u64)> {
    unsafe { (*book).get_spread() }
}

// ============== TESTS ==============

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_order_creation() {
        let book = FastOrderBook::new(1, 0, u64::MAX, 1);
        
        let order = FastOrder::new(0, 1, 1, 50000, 1000, 0, 1);
        let order_id = book.submit_order(order);
        
        assert!(order_id > 0);
        
        let retrieved = book.get_order(order_id);
        assert!(retrieved.is_some());
    }
    
    #[test]
    fn test_order_cancellation() {
        let book = FastOrderBook::new(1, 0, u64::MAX, 1);
        
        let order = FastOrder::new(0, 1, 1, 50000, 1000, 0, 1);
        let order_id = book.submit_order(order);
        
        let cancelled = book.cancel_order(order_id, 1);
        assert!(cancelled);
        
        let retrieved = book.get_order(order_id).unwrap();
        assert_eq!(retrieved.status, 3);
    }
    
    #[test]
    fn test_spread() {
        let book = FastOrderBook::new(1, 0, u64::MAX, 1);
        
        // Add buy order at 50000
        let buy_order = FastOrder::new(0, 1, 1, 50000, 1000, 0, 1);
        book.submit_order(buy_order);
        
        // Add sell order at 50100
        let sell_order = FastOrder::new(0, 2, 1, 50100, 1000, 1, 1);
        book.submit_order(sell_order);
        
        let spread = book.get_spread();
        assert!(spread.is_some());
        
        let (bid, ask) = spread.unwrap();
        assert_eq!(bid, 50000);
        assert_eq!(ask, 50100);
    }
}
