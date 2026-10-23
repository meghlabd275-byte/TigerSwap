//! TigerSwap ultra-low-latency trading engine.
//!
//! This crate provides the primitives the Rust matching engine is built from:
//! sound lock-free queues, a serialization codec, an order book and an
//! epoll-based network loop. Every primitive here is memory-safe; the only
//! `unsafe` blocks are the FFI exports and the documented atomic pointer reads
//! in the MPMC queue.

pub mod data_structures;
pub mod network;
pub mod protocol;
pub mod serialization;
pub mod trading_engine;

pub use data_structures::{Bitset, MPMCQueue, SPSCQueue, TimingWheel};
pub use network::{EpollServer, ServerStats, ZeroCopyBuffer};
pub use protocol::{WSFrame, WSMessage, MessageType};
pub use serialization::{BinaryDecoder, BinaryEncoder, Serializable};
pub use trading_engine::{FastOrder, FastOrderBook, FastTrade, OrderBookStats, PriceLevel};
