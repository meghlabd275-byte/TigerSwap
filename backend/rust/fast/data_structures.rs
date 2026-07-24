//! TigerSwap High-Performance Data Structures
//! Lock-free containers for trading engine

use std::sync::atomic::{AtomicU64, AtomicUsize, Ordering, fence};
use std::sync::Arc;
use std::collections::HashMap;

// ============== LOCK-FREE MAP ==============

/// Lock-free hash map using atomic operations
pub struct LockFreeMap<K, V> 
where 
    K: std::hash::Hash + Eq + Clone,
    V: Copy + Default,
{
    buckets: Vec<AtomicU64>,
    size: usize,
    _phantom: std::marker::PhantomData<(K, V)>,
}

impl<K, V> LockFreeMap<K, V> 
where 
    K: std::hash::Hash + Eq + Clone,
    V: Copy + Default,
{
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
    
    #[inline]
    fn bucket_index(&self, key: &K) -> usize {
        use std::collections::hash_map::DefaultHasher;
        use std::hash::{Hash, Hasher};
        
        let mut hasher = DefaultHasher::new();
        key.hash(&mut hasher);
        (hasher.finish() as usize) % self.size
    }
}

// ============== RING BUFFER ==============

/// Single-producer single-consumer ring buffer
pub struct SPSCQueue<T: Copy + Default> {
    buffer: Vec<T>,
    read_idx: AtomicUsize,
    write_idx: AtomicUsize,
    capacity: usize,
}

impl<T: Copy + Default> SPSCQueue<T> {
    pub fn new(capacity: usize) -> Self {
        let capacity = capacity.next_power_of_two();
        let mut buffer = Vec::with_capacity(capacity);
        
        for _ in 0..capacity {
            buffer.push(T::default());
        }
        
        Self {
            buffer,
            read_idx: AtomicUsize::new(0),
            write_idx: AtomicUsize::new(0),
            capacity,
        }
    }
    
    #[inline]
    pub fn push(&self, value: T) -> bool {
        let write = self.write_idx.load(Ordering::Acquire);
        let read = self.read_idx.load(Ordering::Acquire);
        
        if (write.wrapping_sub(read)) >= self.capacity {
            return false; // Full
        }
        
        self.buffer[write & (self.capacity - 1)] = value;
        self.write_idx.store(write.wrapping_add(1), Ordering::Release);
        
        true
    }
    
    #[inline]
    pub fn pop(&self) -> Option<T> {
        let read = self.read_idx.load(Ordering::Acquire);
        let write = self.write_idx.load(Ordering::Acquire);
        
        if read == write {
            return None; // Empty
        }
        
        let value = self.buffer[read & (self.capacity - 1)];
        self.read_idx.store(read.wrapping_add(1), Ordering::Release);
        
        Some(value)
    }
    
    #[inline]
    pub fn len(&self) -> usize {
        let write = self.write_idx.load(Ordering::Acquire);
        let read = self.read_idx.load(Ordering::Acquire);
        write.wrapping_sub(read)
    }
    
    #[inline]
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }
    
    #[inline]
    pub fn is_full(&self) -> bool {
        self.len() >= self.capacity
    }
}

// ============== MPMC QUEUE ==============

/// Multi-producer multi-consumer queue
pub struct MPMCQueue<T: Copy + Default> {
    buffer: Vec<T>,
    read_idx: AtomicUsize,
    write_idx: AtomicUsize,
    capacity: usize,
}

impl<T: Copy + Default> MPMCQueue<T> {
    pub fn new(capacity: usize) -> Self {
        let capacity = capacity.next_power_of_two();
        let mut buffer = Vec::with_capacity(capacity);
        
        for _ in 0..capacity {
            buffer.push(T::default());
        }
        
        Self {
            buffer,
            read_idx: AtomicUsize::new(0),
            write_idx: AtomicUsize::new(0),
            capacity,
        }
    }
    
    #[inline]
    pub fn enqueue(&self, value: T) -> bool {
        loop {
            let write = self.write_idx.load(Ordering::Acquire);
            let read = self.read_idx.load(Ordering::Acquire);
            
            if (write.wrapping_sub(read)) >= self.capacity {
                return false;
            }
            
            let new_write = write.wrapping_add(1);
            
            if self.write_idx.compare_exchange_weak(
                write, new_write, 
                Ordering::Release, 
                Ordering::Acquire
            ).is_ok() {
                self.buffer[write & (self.capacity - 1)] = value;
                fence(Ordering::Release);
                return true;
            }
        }
    }
    
    #[inline]
    pub fn dequeue(&self) -> Option<T> {
        loop {
            let read = self.read_idx.load(Ordering::Acquire);
            let write = self.write_idx.load(Ordering::Acquire);
            
            if read == write {
                return None;
            }
            
            let new_read = read.wrapping_add(1);
            
            if self.read_idx.compare_exchange_weak(
                read, new_read,
                Ordering::Release,
                Ordering::Acquire
            ).is_ok() {
                let value = self.buffer[read & (self.capacity - 1)];
                fence(Ordering::Release);
                return Some(value);
            }
        }
    }
    
    #[inline]
    pub fn len(&self) -> usize {
        let write = self.write_idx.load(Ordering::Acquire);
        let read = self.read_idx.load(Ordering::Acquire);
        write.wrapping_sub(read)
    }
}

// ============== BITSET ==============

/// Lock-free bitset for order tracking
pub struct Bitset {
    words: Vec<u64>,
    len: usize,
}

impl Bitset {
    pub fn new(bits: usize) -> Self {
        let words = (bits + 63) / 64;
        Self {
            words: vec![0; words],
            len: bits,
        }
    }
    
    #[inline]
    pub fn set(&self, index: usize) {
        if index >= self.len { return; }
        let word = index / 64;
        let bit = index % 64;
        self.words[word] |= 1 << bit;
    }
    
    #[inline]
    pub fn clear(&self, index: usize) {
        if index >= self.len { return; }
        let word = index / 64;
        let bit = index % 64;
        self.words[word] &= !(1 << bit);
    }
    
    #[inline]
    pub fn test(&self, index: usize) -> bool {
        if index >= self.len { return false; }
        let word = index / 64;
        let bit = index % 64;
        (self.words[word] & (1 << bit)) != 0
    }
    
    #[inline]
    pub fn set_batch(&self, start: usize, count: usize) {
        for i in 0..count {
            self.set(start + i);
        }
    }
    
    pub fn count(&self) -> usize {
        self.words.iter().map(|w| w.count_ones() as usize).sum()
    }
}

// ============== TIMING WHEEL ==============

/// Timing wheel for scheduled tasks
pub struct TimingWheel {
    wheel: Vec<Vec<ScheduledTask>>,
    current_slot: usize,
    tick_ms: u64,
    slots: usize,
}

struct ScheduledTask {
    delay_ms: u64,
    task_id: u64,
}

impl TimingWheel {
    pub fn new(tick_ms: u64, slots: usize) -> Self {
        let mut wheel = Vec::with_capacity(slots);
        
        for _ in 0..slots {
            wheel.push(Vec::new());
        }
        
        Self {
            wheel,
            current_slot: 0,
            tick_ms,
            slots,
        }
    }
    
    pub fn schedule(&mut self, delay_ms: u64, task_id: u64) {
        let slots_ahead = (delay_ms / self.tick_ms) as usize % self.slots;
        let slot = (self.current_slot + slots_ahead) % self.slots;
        
        self.wheel[slot].push(ScheduledTask {
            delay_ms,
            task_id,
        });
    }
    
    pub fn tick(&mut self) -> Vec<u64> {
        let mut ready = Vec::new();
        
        for task in self.wheel[self.current_slot].drain(..) {
            ready.push(task.task_id);
        }
        
        self.current_slot = (self.current_slot + 1) % self.slots;
        
        ready
    }
}

// ============== FFI EXPORTS ==============

#[no_mangle]
pub extern "C" fn create_spsc_queue(capacity: usize) -> *mut SPSCQueue<u64> {
    Box::into_raw(Box::new(SPSCQueue::new(capacity)))
}

#[no_mangle]
pub extern "C" fn spsc_push(queue: *mut SPSCQueue<u64>, value: u64) -> bool {
    unsafe { (*queue).push(value) }
}

#[no_mangle]
pub extern "C" fn spsc_pop(queue: *mut SPSCQueue<u64>) -> Option<u64> {
    unsafe { (*queue).pop() }
}

#[no_mangle]
pub extern "C" fn create_bitset(bits: usize) -> *mut Bitset {
    Box::into_raw(Box::new(Bitset::new(bits)))
}

#[no_mangle]
pub extern "C" fn bitset_set(bitset: *mut Bitset, index: usize) {
    unsafe { (*bitset).set(index) }
}

#[no_mangle]
pub extern "C" fn bitset_test(bitset: *const Bitset, index: usize) -> bool {
    unsafe { (*bitset).test(index) }
}

// ============== TESTS ==============

#[cfg(test)]
mod tests {
    use super::*;
    
    #[test]
    fn test_spsc_queue() {
        let queue = SPSCQueue::new(10);
        
        assert!(queue.push(1));
        assert!(queue.push(2));
        
        assert_eq!(queue.pop(), Some(1));
        assert_eq!(queue.pop(), Some(2));
        assert_eq!(queue.pop(), None);
    }
    
    #[test]
    fn test_mpmc_queue() {
        let queue = MPMCQueue::new(10);
        
        assert!(queue.enqueue(1));
        assert!(queue.enqueue(2));
        
        assert_eq!(queue.dequeue(), Some(1));
        assert_eq!(queue.dequeue(), Some(2));
        assert_eq!(queue.dequeue(), None);
    }
    
    #[test]
    fn test_bitset() {
        let bitset = Bitset::new(100);
        
        bitset.set(0);
        bitset.set(50);
        bitset.set(99);
        
        assert!(bitset.test(0));
        assert!(bitset.test(50));
        assert!(bitset.test(99));
        assert!(!bitset.test(1));
        
        bitset.clear(50);
        assert!(!bitset.test(50));
    }
}
