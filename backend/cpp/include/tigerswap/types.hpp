// TigerSwap order types shared by the matching engine and the network layer.
#pragma once

#include <cstdint>

namespace tigerswap {

/// Order side.
enum class Side : uint8_t { Buy = 0, Sell = 1 };

/// Order type.
enum class OrderType : uint8_t { Market = 0, Limit = 1, Stop = 2, StopLimit = 3 };

/// Time in force.
enum class TimeInForce : uint8_t {
    Gtc = 0,       ///< Rest until filled or cancelled.
    Ioc = 1,       ///< Fill what is possible, cancel the remainder.
    Fok = 2,       ///< Fill completely or reject.
    PostOnly = 3,  ///< Only add liquidity.
};

/// Order status.
enum class OrderStatus : uint8_t {
    Open = 0,
    PartiallyFilled = 1,
    Filled = 2,
    Cancelled = 3,
    Rejected = 4,
};

/// A limit or market order.
struct Order {
    uint64_t id = 0;
    uint64_t trader = 0;
    uint64_t pair = 0;
    uint64_t price = 0;
    uint64_t quantity = 0;
    uint64_t filled = 0;
    uint64_t timestamp = 0;
    Side side = Side::Buy;
    OrderType type = OrderType::Limit;
    OrderStatus status = OrderStatus::Open;

    uint64_t remaining() const { return quantity - filled; }
    bool is_open() const {
        return status == OrderStatus::Open || status == OrderStatus::PartiallyFilled;
    }
};

/// A completed match between a resting maker and an incoming taker.
struct Trade {
    uint64_t id = 0;
    uint64_t maker_order = 0;
    uint64_t taker_order = 0;
    uint64_t pair = 0;
    uint64_t price = 0;
    uint64_t quantity = 0;
    uint64_t taker_fee = 0;
    uint64_t timestamp = 0;
};

/// Aggregated resting quantity at one price.
struct PriceLevel {
    uint64_t price = 0;
    uint64_t quantity = 0;
    uint32_t orders = 0;
};

/// Reason an order was not accepted.
enum class RejectReason : uint8_t {
    None = 0,
    InvalidOrder,
    InvalidSide,
    InvalidType,
    PriceNotOnTick,
    PostOnlyWouldCross,
    FillOrKillUnfillable,
    NoLiquidity,
    StopRequiresTrigger,
    UnknownOrder,
    NotOrderOwner,
    AlreadyClosed,
};

/// Outcome of a submit or cancel call.
struct ExecReport {
    bool accepted = false;
    RejectReason reason = RejectReason::None;
    uint64_t order_id = 0;
    uint64_t filled = 0;
    uint64_t remaining = 0;
};

}  // namespace tigerswap
