// TigerSwap order types shared by the matching engine and the network layer.
#pragma once

#include <cstdint>
#include <limits>

namespace tigerswap {

// A 128-bit unsigned integer, used for the one place 64 bits is not enough:
// price * quantity (and the fee derived from it). `__extension__` keeps the
// type usable under `-Wpedantic`, which the engine is built with.
__extension__ typedef unsigned __int128 wide_uint;

/// Adds two unsigned 64-bit values, saturating at the maximum instead of
/// wrapping. Used when summing quantities, where a wrap would understate a
/// total and let a fill-or-kill pass a check it should fail.
inline uint64_t saturating_add(uint64_t a, uint64_t b) {
    const uint64_t max = std::numeric_limits<uint64_t>::max();
    return a > max - b ? max : a + b;
}

/// Multiplies two unsigned 64-bit values into a 128-bit result. Price and
/// quantity are both caller-supplied, so a 64-bit product can silently wrap;
/// the wide result cannot.
inline wide_uint mul_wide(uint64_t a, uint64_t b) {
    return static_cast<wide_uint>(a) * static_cast<wide_uint>(b);
}

/// Computes `a * b / divisor` without the intermediate product wrapping. The
/// divisor must be non-zero. The result is clamped to the 64-bit range so a
/// value that cannot be represented is saturated rather than truncated.
inline uint64_t mul_div(uint64_t a, uint64_t b, uint64_t divisor) {
    if (divisor == 0) {
        return std::numeric_limits<uint64_t>::max();
    }
    const wide_uint result = mul_wide(a, b) / static_cast<wide_uint>(divisor);
    const wide_uint max = std::numeric_limits<uint64_t>::max();
    return result > max ? std::numeric_limits<uint64_t>::max() : static_cast<uint64_t>(result);
}

/// Computes `a * b * c / divisor` with a 128-bit intermediate product, so the
/// fee on a large notional cannot wrap. Every argument is caller-influenced, so
/// both the 128-bit product and the final result are clamped rather than
/// truncated.
inline uint64_t mul_mul_div(uint64_t a, uint64_t b, uint64_t c, uint64_t divisor) {
    const uint64_t max64 = std::numeric_limits<uint64_t>::max();
    if (divisor == 0) {
        return max64;
    }
    const wide_uint wide_c = static_cast<wide_uint>(c);
    const wide_uint product = mul_wide(a, b);
    if (wide_c != 0 && product > ~static_cast<wide_uint>(0) / wide_c) {
        return max64;
    }
    const wide_uint result = (product * wide_c) / static_cast<wide_uint>(divisor);
    return result > static_cast<wide_uint>(max64) ? max64 : static_cast<uint64_t>(result);
}

/// True when `a * b` does not fit in 64 bits.
inline bool mul_overflows(uint64_t a, uint64_t b) {
    if (a == 0 || b == 0) {
        return false;
    }
    return mul_wide(a, b) > static_cast<wide_uint>(std::numeric_limits<uint64_t>::max());
}

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
    // The pair is already holding the maximum number of resting orders.
    TooManyOpenOrders,
    // The order could only have matched the trader's own resting liquidity, so
    // resting it would leave the book crossed.
    SelfTradePrevented,
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
