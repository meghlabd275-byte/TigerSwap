//! TigerSwap ultra-low-latency trading engine.
//!
//! This crate provides the primitives the Rust matching engine is built from:
//! sound lock-free queues, a serialization codec, an order book and an
//! epoll-based network loop. The `unsafe` code is confined to three places:
//! the `UnsafeCell` slot access in the SPSC and MPMC queues, where the atomic
//! indices guarantee that a slot is only touched by one side at a time, and the
//! `libc` epoll and socket calls in the network loop. Nothing here reinterprets
//! foreign memory or exposes an unchecked public API.

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
