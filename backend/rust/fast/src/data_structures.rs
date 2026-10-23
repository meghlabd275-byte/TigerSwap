//! Lock-free data structures for the TigerSwap trading engine.
//!
//! All containers here are safe: they own their storage, use atomic indices for
//! coordination and never reinterpret memory. The MPMC queue reserves slots
//! with `MaybeUninit` so that a producer and a consumer never touch the same
//! cell at once.

use std::cell::UnsafeCell;
use std::mem::MaybeUninit;
use std::sync::atomic::{fence, AtomicBool, AtomicU64, AtomicUsize, Ordering};

// ============== SPSC RING BUFFER ==============

/// Single-producer single-consumer ring buffer.
///
/// `push` must only be called from one thread and `pop` from one other thread.
pub struct SPSCQueue<T> {
    slots: Box<[UnsafeCell<MaybeUninit<T>>]>,
    read_idx: AtomicUsize,
    write_idx: AtomicUsize,
    mask: usize,
    capacity: usize,
}

// Safety: the producer only writes the slot at `write_idx` and the consumer
// only reads the slot at `read_idx`. Indices are published with Release stores
// and read with Acquire loads, so the payload is never accessed concurrently.
unsafe impl<T: Send> Send for SPSCQueue<T> {}
unsafe impl<T: Send> Sync for SPSCQueue<T> {}

impl<T> SPSCQueue<T> {
    /// Creates a queue holding at least `capacity` items, rounded up to a power of two.
    pub fn new(capacity: usize) -> Self {
        let capacity = capacity.max(1).next_power_of_two();

        let mut slots = Vec::with_capacity(capacity);
        for _ in 0..capacity {
            slots.push(UnsafeCell::new(MaybeUninit::uninit()));
        }

        Self {
            slots: slots.into_boxed_slice(),
            read_idx: AtomicUsize::new(0),
            write_idx: AtomicUsize::new(0),
            mask: capacity - 1,
            capacity,
        }
    }

    /// Attempts to enqueue a value, returning false when the queue is full.
    #[inline]
    pub fn push(&self, value: T) -> bool {
        let write = self.write_idx.load(Ordering::Relaxed);
        let read = self.read_idx.load(Ordering::Acquire);

        if write.wrapping_sub(read) >= self.capacity {
            return false;
        }

        // Safety: this slot is free because the distance between the indices is
        // below the capacity, so the consumer has already drained it.
        unsafe {
            (*self.slots[write & self.mask].get()).write(value);
        }

        self.write_idx.store(write.wrapping_add(1), Ordering::Release);
        true
    }

    /// Attempts to dequeue a value, returning None when the queue is empty.
    #[inline]
    pub fn pop(&self) -> Option<T> {
        let read = self.read_idx.load(Ordering::Relaxed);
        let write = self.write_idx.load(Ordering::Acquire);

        if read == write {
            return None;
        }

        // Safety: the producer published this slot with a Release store and the
        // consumer now owns it until it advances `read_idx`.
        let value = unsafe { (*self.slots[read & self.mask].get()).assume_init_read() };

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

    pub fn capacity(&self) -> usize {
        self.capacity
    }
}

impl<T> Drop for SPSCQueue<T> {
    fn drop(&mut self) {
        // Drop every element that was pushed but never popped.
        while self.pop().is_some() {}
    }
}

// ============== MPMC QUEUE ==============

/// Multi-producer multi-consumer bounded queue using per-slot sequence numbers.
pub struct MPMCQueue<T> {
    slots: Box<[Slot<T>]>,
    mask: usize,
    capacity: usize,
    enqueue_pos: AtomicUsize,
    dequeue_pos: AtomicUsize,
}

struct Slot<T> {
    sequence: AtomicUsize,
    value: UnsafeCell<MaybeUninit<T>>,
}

// Safety: a slot is only written while its sequence number reserves it for a
// single producer, and only read while it is reserved for a single consumer.
unsafe impl<T: Send> Send for MPMCQueue<T> {}
unsafe impl<T: Send> Sync for MPMCQueue<T> {}

impl<T> MPMCQueue<T> {
    pub fn new(capacity: usize) -> Self {
        let capacity = capacity.max(1).next_power_of_two();

        let mut slots = Vec::with_capacity(capacity);
        for index in 0..capacity {
            slots.push(Slot {
                sequence: AtomicUsize::new(index),
                value: UnsafeCell::new(MaybeUninit::uninit()),
            });
        }

        Self {
            slots: slots.into_boxed_slice(),
            mask: capacity - 1,
            capacity,
            enqueue_pos: AtomicUsize::new(0),
            dequeue_pos: AtomicUsize::new(0),
        }
    }

    /// Attempts to enqueue a value, returning false when the queue is full.
    pub fn enqueue(&self, value: T) -> bool {
        let mut pos = self.enqueue_pos.load(Ordering::Relaxed);

        loop {
            let slot = &self.slots[pos & self.mask];
            let sequence = slot.sequence.load(Ordering::Acquire);
            let diff = sequence as isize - pos as isize;

            if diff == 0 {
                if self
                    .enqueue_pos
                    .compare_exchange_weak(pos, pos.wrapping_add(1), Ordering::Relaxed, Ordering::Relaxed)
                    .is_ok()
                {
                    // Safety: this producer reserved the slot by advancing the
                    // enqueue position; no other thread can write it.
                    unsafe {
                        (*slot.value.get()).write(value);
                    }
                    slot.sequence.store(pos.wrapping_add(1), Ordering::Release);
                    return true;
                }
                pos = self.enqueue_pos.load(Ordering::Relaxed);
            } else if diff < 0 {
                return false;
            } else {
                pos = self.enqueue_pos.load(Ordering::Relaxed);
            }
        }
    }

    /// Attempts to dequeue a value, returning None when the queue is empty.
    pub fn dequeue(&self) -> Option<T> {
        let mut pos = self.dequeue_pos.load(Ordering::Relaxed);

        loop {
            let slot = &self.slots[pos & self.mask];
            let sequence = slot.sequence.load(Ordering::Acquire);
            let diff = sequence as isize - pos.wrapping_add(1) as isize;

            if diff == 0 {
                if self
                    .dequeue_pos
                    .compare_exchange_weak(pos, pos.wrapping_add(1), Ordering::Relaxed, Ordering::Relaxed)
                    .is_ok()
                {
                    // Safety: the slot was published by a producer and reserved
                    // here for this consumer alone.
                    let value = unsafe { (*slot.value.get()).assume_init_read() };
                    slot.sequence
                        .store(pos.wrapping_add(self.capacity), Ordering::Release);
                    return Some(value);
                }
                pos = self.dequeue_pos.load(Ordering::Relaxed);
            } else if diff < 0 {
                return None;
            } else {
                pos = self.dequeue_pos.load(Ordering::Relaxed);
            }
        }
    }

    #[inline]
    pub fn len(&self) -> usize {
        let enqueue = self.enqueue_pos.load(Ordering::Acquire);
        let dequeue = self.dequeue_pos.load(Ordering::Acquire);
        enqueue.saturating_sub(dequeue)
    }

    #[inline]
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }

    pub fn capacity(&self) -> usize {
        self.capacity
    }
}

impl<T> Drop for MPMCQueue<T> {
    fn drop(&mut self) {
        while self.dequeue().is_some() {}
    }
}

// ============== BITSET ==============

/// Lock-free bitset for order and pool tracking.
pub struct Bitset {
    words: Box<[AtomicU64]>,
    len: usize,
}

impl Bitset {
    pub fn new(len: usize) -> Self {
        let words = len.div_ceil(64);
        let mut storage = Vec::with_capacity(words);
        for _ in 0..words {
            storage.push(AtomicU64::new(0));
        }

        Self {
            words: storage.into_boxed_slice(),
            len,
        }
    }

    pub fn len(&self) -> usize {
        self.len
    }

    pub fn is_empty(&self) -> bool {
        self.len == 0
    }

    /// Sets a bit, returning false when the index is out of range.
    #[inline]
    pub fn set(&self, index: usize) -> bool {
        if index >= self.len {
            return false;
        }
        self.words[index / 64].fetch_or(1u64 << (index % 64), Ordering::AcqRel);
        true
    }

    /// Clears a bit, returning false when the index is out of range.
    #[inline]
    pub fn clear(&self, index: usize) -> bool {
        if index >= self.len {
            return false;
        }
        self.words[index / 64].fetch_and(!(1u64 << (index % 64)), Ordering::AcqRel);
        true
    }

    #[inline]
    pub fn test(&self, index: usize) -> bool {
        if index >= self.len {
            return false;
        }
        self.words[index / 64].load(Ordering::Acquire) & (1u64 << (index % 64)) != 0
    }

    /// Sets a contiguous run of bits `[start, start + count)`.
    pub fn set_batch(&self, start: usize, count: usize) -> usize {
        let mut set = 0;
        for index in start..start.saturating_add(count).min(self.len) {
            if self.set(index) {
                set += 1;
            }
        }
        set
    }

    /// Counts the set bits.
    pub fn count(&self) -> usize {
        self.words
            .iter()
            .map(|word| word.load(Ordering::Acquire).count_ones() as usize)
            .sum()
    }
}

// ============== TIMING WHEEL ==============

/// Hashed timing wheel for expiry and timeout handling.
pub struct TimingWheel {
    buckets: Vec<Vec<u64>>,
    tick_ms: u64,
    current_tick: AtomicU64,
    rotation: usize,
}

impl TimingWheel {
    /// Builds a wheel with `slots` buckets and `tick_ms` milliseconds per tick.
    pub fn new(slots: usize, tick_ms: u64) -> Self {
        let slots = slots.max(1);
        let mut buckets = Vec::with_capacity(slots);
        for _ in 0..slots {
            buckets.push(Vec::new());
        }

        Self {
            buckets,
            tick_ms: tick_ms.max(1),
            current_tick: AtomicU64::new(0),
            rotation: 0,
        }
    }

    /// Schedules an id to be released after `delay_ms` milliseconds.
    pub fn schedule(&mut self, id: u64, delay_ms: u64) -> usize {
        let ticks = (delay_ms / self.tick_ms).max(1) as usize;
        let slot = (self.rotation + ticks) % self.buckets.len();
        self.buckets[slot].push(id);
        slot
    }

    /// Advances the wheel by `ticks` and returns the ids that expired.
    pub fn tick(&mut self, ticks: u64) -> Vec<u64> {
        let mut expired = Vec::new();

        for _ in 0..ticks {
            self.rotation = (self.rotation + 1) % self.buckets.len();
            expired.append(&mut self.buckets[self.rotation]);
            self.current_tick.fetch_add(1, Ordering::AcqRel);
        }

        expired
    }

    pub fn current_tick(&self) -> u64 {
        self.current_tick.load(Ordering::Acquire)
    }
}

// ============== SPIN LOCK ==============

/// Simple test-and-set spin lock for short critical sections.
pub struct SpinLock {
    locked: AtomicBool,
}

impl SpinLock {
    pub const fn new() -> Self {
        Self {
            locked: AtomicBool::new(false),
        }
    }

    #[inline]
    pub fn lock(&self) {
        while self
            .locked
            .compare_exchange_weak(false, true, Ordering::Acquire, Ordering::Relaxed)
            .is_err()
        {
            std::hint::spin_loop();
        }
    }

    #[inline]
    pub fn unlock(&self) {
        self.locked.store(false, Ordering::Release);
    }

    #[inline]
    pub fn try_lock(&self) -> bool {
        self.locked
            .compare_exchange(false, true, Ordering::Acquire, Ordering::Relaxed)
            .is_ok()
    }
}

impl Default for SpinLock {
    fn default() -> Self {
        Self::new()
    }
}

// ============== SEQUENCE BARRIER ==============

/// Publishes monotonically increasing sequence numbers to readers.
pub struct SequenceBarrier {
    sequence: AtomicU64,
}

impl SequenceBarrier {
    pub fn new(initial: u64) -> Self {
        Self {
            sequence: AtomicU64::new(initial),
        }
    }

    pub fn publish(&self, sequence: u64) {
        fence(Ordering::Release);
        self.sequence.store(sequence, Ordering::Release);
    }

    pub fn current(&self) -> u64 {
        self.sequence.load(Ordering::Acquire)
    }

    /// Waits until the sequence reaches `target` or `spins` is exhausted.
    pub fn wait_for(&self, target: u64, spins: u32) -> bool {
        for _ in 0..spins {
            if self.current() >= target {
                return true;
            }
            std::hint::spin_loop();
        }
        self.current() >= target
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::Arc;

    #[test]
    fn spsc_preserves_order() {
        let queue = SPSCQueue::new(8);
        for value in 0..8u64 {
            assert!(queue.push(value));
        }
        assert!(queue.is_full());

        for value in 0..8u64 {
            assert_eq!(queue.pop(), Some(value));
        }
        assert!(queue.is_empty());
    }

    #[test]
    fn spsc_round_trips_owned_strings() {
        let queue = SPSCQueue::new(4);
        assert!(queue.push(String::from("tigerswap")));
        assert_eq!(queue.pop().as_deref(), Some("tigerswap"));
    }

    #[test]
    fn spsc_across_threads() {
        const COUNT: u64 = 100_000;
        let queue = Arc::new(SPSCQueue::new(1024));

        let producer = {
            let queue = Arc::clone(&queue);
            std::thread::spawn(move || {
                let mut sent = 0;
                while sent < COUNT {
                    if queue.push(sent) {
                        sent += 1;
                    } else {
                        std::hint::spin_loop();
                    }
                }
            })
        };

        let consumer = {
            let queue = Arc::clone(&queue);
            std::thread::spawn(move || {
                let mut received = Vec::with_capacity(COUNT as usize);
                while received.len() < COUNT as usize {
                    if let Some(value) = queue.pop() {
                        received.push(value);
                    } else {
                        std::hint::spin_loop();
                    }
                }
                received
            })
        };

        producer.join().unwrap();
        let received = consumer.join().unwrap();
        assert_eq!(received.len(), COUNT as usize);
        assert!(received.iter().enumerate().all(|(i, v)| *v == i as u64));
    }

    #[test]
    fn mpmc_round_trip() {
        let queue = MPMCQueue::new(4);
        assert!(queue.enqueue(1u32));
        assert!(queue.enqueue(2u32));
        assert_eq!(queue.dequeue(), Some(1));
        assert_eq!(queue.dequeue(), Some(2));
        assert_eq!(queue.dequeue(), None);
    }

    #[test]
    fn mpmc_under_contention() {
        const THREADS: usize = 4;
        const PER_THREAD: usize = 10_000;

        let queue = Arc::new(MPMCQueue::new(256));
        let mut handles = Vec::new();

        for _ in 0..THREADS {
            let queue = Arc::clone(&queue);
            handles.push(std::thread::spawn(move || {
                let mut produced = 0;
                while produced < PER_THREAD {
                    if queue.enqueue(produced as u64) {
                        produced += 1;
                    } else {
                        std::hint::spin_loop();
                    }
                }
            }));
        }

        let total = THREADS * PER_THREAD;
        let consumer_queue = Arc::clone(&queue);
        let consumer = std::thread::spawn(move || {
            let mut consumed = 0;
            while consumed < total {
                if consumer_queue.dequeue().is_some() {
                    consumed += 1;
                } else {
                    std::hint::spin_loop();
                }
            }
            consumed
        });

        for handle in handles {
            handle.join().unwrap();
        }
        assert_eq!(consumer.join().unwrap(), total);
    }

    #[test]
    fn bitset_operations() {
        let bitset = Bitset::new(130);
        assert!(bitset.set(0));
        assert!(bitset.set(129));
        assert!(bitset.test(0));
        assert!(bitset.test(129));
        assert!(!bitset.test(64));
        assert!(!bitset.set(200));
        assert_eq!(bitset.count(), 2);

        bitset.clear(0);
        assert!(!bitset.test(0));
        assert_eq!(bitset.count(), 1);
    }

    #[test]
    fn bitset_set_batch() {
        let bitset = Bitset::new(64);
        assert_eq!(bitset.set_batch(10, 20), 20);
        assert_eq!(bitset.count(), 20);
        assert!(bitset.test(10));
        assert!(bitset.test(29));
        assert!(!bitset.test(30));
    }

    #[test]
    fn timing_wheel_expires_in_order() {
        let mut wheel = TimingWheel::new(8, 10);
        wheel.schedule(1, 10);
        wheel.schedule(2, 20);

        assert!(wheel.tick(1).contains(&1));
        assert!(wheel.tick(1).contains(&2));
        assert_eq!(wheel.current_tick(), 2);
    }

    #[test]
    fn spin_lock_serializes() {
        let lock = SpinLock::new();
        assert!(lock.try_lock());
        assert!(!lock.try_lock());
        lock.unlock();
        assert!(lock.try_lock());
        lock.unlock();
    }

    #[test]
    fn sequence_barrier_waits_for_publication() {
        let barrier = SequenceBarrier::new(0);
        assert!(!barrier.wait_for(1, 8));
        barrier.publish(1);
        assert!(barrier.wait_for(1, 8));
    }
}
