// Price-time priority order book for one trading pair.
//
// Design notes:
//   * Price levels live in a `std::map` keyed by price. Bids use `std::greater`
//     so `begin()` is always the best bid; asks use `std::less` so `begin()` is
//     the best ask.
//   * Each level holds an intrusive doubly linked FIFO of resting orders, so
//     time priority is exact and cancellation is O(1) once the order is found.
//   * Orders are stored in an open-addressing table indexed by order id, which
//     keeps lookup cache-friendly and avoids per-node allocation.
//
// The book is single-threaded by design: the matching engine owns one book per
// pair and serialises access, which is how real venues achieve determinism.

#pragma once

#include <cstdint>
#include <map>
#include <vector>

#include "tigerswap/clock.hpp"
#include "tigerswap/types.hpp"

namespace tigerswap {

/// Fixed-capacity open-addressing order index.
class OrderIndex {
public:
    explicit OrderIndex(size_t capacity = 1 << 16) { resize(capacity); }

    void resize(size_t capacity) {
        capacity_ = 1;
        while (capacity_ < capacity) {
            capacity_ <<= 1;
        }
        mask_ = capacity_ - 1;
        slots_.assign(capacity_, Slot{});
        size_ = 0;
    }

    size_t size() const { return size_; }
    size_t capacity() const { return capacity_; }

    /// Inserts an order. Returns false if the id already exists.
    bool insert(const Order& order) {
        if (size_ * 4 >= capacity_ * 3) {
            rehash(capacity_ * 2);
        }

        size_t slot = hash(order.id) & mask_;
        while (slots_[slot].occupied) {
            if (slots_[slot].order.id == order.id) {
                return false;
            }
            slot = (slot + 1) & mask_;
        }

        slots_[slot].occupied = true;
        slots_[slot].order = order;
        ++size_;
        return true;
    }

    Order* find(uint64_t id) {
        size_t slot = hash(id) & mask_;
        while (slots_[slot].occupied) {
            if (slots_[slot].order.id == id) {
                return &slots_[slot].order;
            }
            slot = (slot + 1) & mask_;
        }
        return nullptr;
    }

    const Order* find(uint64_t id) const {
        return const_cast<OrderIndex*>(this)->find(id);
    }

    bool erase(uint64_t id) {
        size_t slot = hash(id) & mask_;
        while (slots_[slot].occupied) {
            if (slots_[slot].order.id == id) {
                break;
            }
            slot = (slot + 1) & mask_;
        }

        if (!slots_[slot].occupied) {
            return false;
        }

        // Backward-shift deletion keeps probe chains intact without tombstones.
        slots_[slot].occupied = false;
        --size_;

        size_t next = (slot + 1) & mask_;
        while (slots_[next].occupied) {
            const Order displaced = slots_[next].order;
            slots_[next].occupied = false;
            --size_;

            size_t target = hash(displaced.id) & mask_;
            while (slots_[target].occupied) {
                target = (target + 1) & mask_;
            }
            slots_[target].occupied = true;
            slots_[target].order = displaced;
            ++size_;

            next = (next + 1) & mask_;
        }

        return true;
    }

    /// Visits every stored order.
    template <typename Fn>
    void for_each(Fn&& fn) const {
        for (const auto& slot : slots_) {
            if (slot.occupied) {
                fn(slot.order);
            }
        }
    }

private:
    struct Slot {
        Order order;
        bool occupied = false;
    };

    static uint64_t hash(uint64_t key) {
        // splitmix64 finaliser: good avalanche, no multiply overflow issues.
        key += 0x9E3779B97F4A7C15ULL;
        key = (key ^ (key >> 30)) * 0xBF58476D1CE4E5B9ULL;
        key = (key ^ (key >> 27)) * 0x94D049BB133111EBULL;
        return key ^ (key >> 31);
    }

    void rehash(size_t capacity) {
        std::vector<Slot> old;
        old.swap(slots_);
        const size_t old_capacity = capacity_;

        capacity_ = capacity;
        mask_ = capacity_ - 1;
        slots_.assign(capacity_, Slot{});
        size_ = 0;

        for (size_t index = 0; index < old_capacity; ++index) {
            if (old[index].occupied) {
                insert(old[index].order);
            }
        }
    }

    std::vector<Slot> slots_;
    size_t capacity_ = 0;
    size_t mask_ = 0;
    size_t size_ = 0;
};

/// FIFO queue of resting orders at one price.
class LevelQueue {
public:
    struct Node {
        uint64_t order_id = 0;
        uint64_t quantity = 0;
        uint64_t trader = 0;
        Node* prev = nullptr;
        Node* next = nullptr;
    };

    LevelQueue() = default;
    LevelQueue(const LevelQueue&) = delete;
    LevelQueue& operator=(const LevelQueue&) = delete;

    LevelQueue(LevelQueue&& other) noexcept { move_from(other); }

    LevelQueue& operator=(LevelQueue&& other) noexcept {
        if (this != &other) {
            release();
            move_from(other);
        }
        return *this;
    }

    ~LevelQueue() { release(); }

    /// Appends an order to the back of the queue, preserving time priority.
    void push_back(uint64_t order_id, uint64_t quantity, uint64_t trader) {
        Node* node = new Node{order_id, quantity, trader, tail_, nullptr};
        if (tail_ != nullptr) {
            tail_->next = node;
        } else {
            head_ = node;
        }
        tail_ = node;
        total_quantity_ += quantity;
        ++count_;
    }

    /// Unlinks a node and frees it. The caller must have obtained it from this queue.
    void erase(Node* node) {
        if (node == nullptr) {
            return;
        }
        if (node->prev != nullptr) {
            node->prev->next = node->next;
        } else {
            head_ = node->next;
        }
        if (node->next != nullptr) {
            node->next->prev = node->prev;
        } else {
            tail_ = node->prev;
        }
        total_quantity_ -= node->quantity;
        --count_;
        delete node;
    }

    /// Subtracts a fill from the front node, removing it when exhausted.
    void reduce_front(uint64_t filled) {
        if (head_ == nullptr) {
            return;
        }
        const uint64_t applied = filled > head_->quantity ? head_->quantity : filled;
        head_->quantity -= applied;
        total_quantity_ -= applied;
        if (head_->quantity == 0) {
            erase(head_);
        }
    }

    Node* front() const { return head_; }
    uint64_t total_quantity() const { return total_quantity_; }
    uint32_t count() const { return count_; }
    bool empty() const { return head_ == nullptr; }

    /// Drops every node without touching the book's index.
    void clear() {
        Node* node = head_;
        while (node != nullptr) {
            Node* next = node->next;
            delete node;
            node = next;
        }
        head_ = nullptr;
        tail_ = nullptr;
        total_quantity_ = 0;
        count_ = 0;
    }

private:
    void release() { clear(); }

    void move_from(LevelQueue& other) {
        head_ = other.head_;
        tail_ = other.tail_;
        total_quantity_ = other.total_quantity_;
        count_ = other.count_;
        other.head_ = nullptr;
        other.tail_ = nullptr;
        other.total_quantity_ = 0;
        other.count_ = 0;
    }

    Node* head_ = nullptr;
    Node* tail_ = nullptr;
    uint64_t total_quantity_ = 0;
    uint32_t count_ = 0;
};

/// One price level: its FIFO queue plus a back-pointer into that queue.
struct Level {
    LevelQueue queue;
    uint64_t price = 0;
};

/// Outcome of a single order submission.
struct MatchResult {
    ExecReport report;
    std::vector<Trade> trades;
};

/// Price-time priority limit order book.
class OrderBook {
public:
    using BidMap = std::map<uint64_t, Level, std::greater<uint64_t>>;
    using AskMap = std::map<uint64_t, Level, std::less<uint64_t>>;

    OrderBook(uint64_t pair_id, uint64_t tick_size, uint32_t maker_fee_bps,
              uint32_t taker_fee_bps)
        : pair_id_(pair_id),
          tick_size_(tick_size == 0 ? 1 : tick_size),
          maker_fee_bps_(maker_fee_bps),
          taker_fee_bps_(taker_fee_bps) {}

    uint64_t pair_id() const { return pair_id_; }

    /// Submits an order with good-till-cancelled semantics.
    MatchResult submit(const Order& order) { return submit(order, TimeInForce::Gtc); }

    /// Submits an order, applying the given time in force.
    MatchResult submit(const Order& incoming, TimeInForce tif) {
        MatchResult result;

        if (incoming.quantity == 0 || incoming.side > Side::Sell) {
            result.report.reason = RejectReason::InvalidOrder;
            return result;
        }

        if (incoming.type != OrderType::Market && incoming.type != OrderType::Limit) {
            // Stop orders are armed by the trigger monitor, never marketable here.
            result.report.reason = RejectReason::StopRequiresTrigger;
            return result;
        }

        if (incoming.type == OrderType::Limit &&
            (incoming.price == 0 || incoming.price % tick_size_ != 0)) {
            result.report.reason = RejectReason::PriceNotOnTick;
            return result;
        }

        if (tif == TimeInForce::PostOnly && would_cross(incoming)) {
            result.report.reason = RejectReason::PostOnlyWouldCross;
            return result;
        }

        if (tif == TimeInForce::Fok && fillable_quantity(incoming) < incoming.quantity) {
            result.report.reason = RejectReason::FillOrKillUnfillable;
            return result;
        }

        Order working = incoming;
        working.timestamp = wall_ns();
        working.status = OrderStatus::Open;
        working.filled = 0;

        match(working, result.trades);

        const uint64_t remaining = working.remaining();

        if (working.type == OrderType::Market) {
            if (working.filled == 0) {
                result.report.reason = RejectReason::NoLiquidity;
                return result;
            }
            working.status = remaining == 0 ? OrderStatus::Filled : OrderStatus::Cancelled;
            working.filled = working.quantity;
            index_.insert(working);
            result.report.accepted = true;
            result.report.order_id = working.id;
            result.report.filled = working.quantity;
            result.report.remaining = 0;
            return result;
        }

        if (remaining > 0 && (tif == TimeInForce::Ioc || tif == TimeInForce::Fok)) {
            working.status =
                working.filled > 0 ? OrderStatus::PartiallyFilled : OrderStatus::Cancelled;
            working.filled = working.quantity;
            index_.insert(working);
            result.report.accepted = true;
            result.report.order_id = working.id;
            result.report.filled = working.filled;
            result.report.remaining = 0;
            return result;
        }

        if (remaining > 0) {
            working.status =
                working.filled > 0 ? OrderStatus::PartiallyFilled : OrderStatus::Open;
            index_.insert(working);
            rest(working);
        } else {
            working.status = OrderStatus::Filled;
            index_.insert(working);
        }

        result.report.accepted = true;
        result.report.order_id = working.id;
        result.report.filled = working.filled;
        result.report.remaining = remaining;
        return result;
    }

    /// Cancels a resting order owned by `trader`. Returns the reason on failure.
    ExecReport cancel(uint64_t order_id, uint64_t trader) {
        ExecReport report;
        report.order_id = order_id;

        Order* order = index_.find(order_id);
        if (order == nullptr) {
            report.reason = RejectReason::UnknownOrder;
            return report;
        }
        if (order->trader != trader) {
            report.reason = RejectReason::NotOrderOwner;
            return report;
        }
        if (!order->is_open()) {
            report.reason = RejectReason::AlreadyClosed;
            return report;
        }

        detach(*order);
        order->status = OrderStatus::Cancelled;
        order->filled = order->quantity;

        report.accepted = true;
        report.filled = order->filled;
        report.remaining = 0;
        return report;
    }

    const Order* find(uint64_t order_id) const { return index_.find(order_id); }

    /// Best bid price, or 0 when there is no bid.
    uint64_t best_bid() const { return bids_.empty() ? 0 : bids_.begin()->first; }

    /// Best ask price, or 0 when there is no ask.
    uint64_t best_ask() const { return asks_.empty() ? 0 : asks_.begin()->first; }

    bool has_bid() const { return !bids_.empty(); }
    bool has_ask() const { return !asks_.empty(); }

    /// Asks ascending followed by bids descending.
    std::vector<PriceLevel> depth(size_t levels) const {
        std::vector<PriceLevel> out;

        size_t emitted = 0;
        for (auto it = asks_.begin(); it != asks_.end() && emitted < levels; ++it, ++emitted) {
            out.push_back(PriceLevel{it->first, it->second.queue.total_quantity(),
                                     it->second.queue.count()});
        }

        emitted = 0;
        for (auto it = bids_.begin(); it != bids_.end() && emitted < levels; ++it, ++emitted) {
            out.push_back(PriceLevel{it->first, it->second.queue.total_quantity(),
                                     it->second.queue.count()});
        }

        return out;
    }

    /// Restores market data from a replayed trade without matching.
    void apply_recovered_trade(uint64_t price, uint64_t quantity) {
        last_price_ = price;
        total_volume_ += quantity;
        ++total_trades_;
    }

    uint64_t last_price() const { return last_price_; }
    uint64_t total_volume() const { return total_volume_; }
    uint64_t total_trades() const { return total_trades_; }
    size_t open_orders() const { return index_.size(); }

    uint32_t maker_fee_bps() const { return maker_fee_bps_; }
    uint32_t taker_fee_bps() const { return taker_fee_bps_; }

    /// Every order known to the book, including closed ones.
    template <typename Fn>
    void for_each_order(Fn&& fn) const {
        index_.for_each(fn);
    }

private:
    uint64_t next_trade_id_ = 1;

    bool would_cross(const Order& order) const {
        if (order.side == Side::Buy) {
            return has_ask() && order.price >= best_ask();
        }
        return has_bid() && order.price <= best_bid();
    }

    uint64_t fillable_quantity(const Order& order) const {
        uint64_t available = 0;

        if (order.side == Side::Buy) {
            for (auto it = asks_.begin(); it != asks_.end(); ++it) {
                if (it->first > order.price) {
                    break;
                }
                available += it->second.queue.total_quantity();
                if (available >= order.quantity) {
                    return available;
                }
            }
        } else {
            for (auto it = bids_.begin(); it != bids_.end(); ++it) {
                if (it->first < order.price) {
                    break;
                }
                available += it->second.queue.total_quantity();
                if (available >= order.quantity) {
                    return available;
                }
            }
        }

        return available;
    }

    void match(Order& taker, std::vector<Trade>& trades) {
        if (taker.side == Side::Buy) {
            match_side(taker, asks_, trades);
        } else {
            match_side(taker, bids_, trades);
        }

        if (!trades.empty()) {
            last_price_ = trades.back().price;
            total_volume_ += trades.back().quantity;
        }
    }

    /// Sweeps the opposite book until the taker is filled or price no longer crosses.
    template <typename Book>
    void match_side(Order& taker, Book& book, std::vector<Trade>& trades) {
        while (taker.remaining() > 0 && !book.empty()) {
            auto level_it = book.begin();
            const uint64_t price = level_it->first;

            if (taker.type == OrderType::Limit) {
                const bool crosses =
                    taker.side == Side::Buy ? price <= taker.price : price >= taker.price;
                if (!crosses) {
                    break;
                }
            }

            if (!match_at_level(taker, level_it->second, price, trades)) {
                // The level held only the taker's own liquidity.
                break;
            }

            if (level_it->second.queue.empty()) {
                book.erase(level_it);
            }
        }
    }

    /// Fills `taker` against one level. Returns false when no foreign liquidity was touched.
    bool match_at_level(Order& taker, Level& level, uint64_t price, std::vector<Trade>& trades) {
        bool progressed = false;

        while (taker.remaining() > 0 && level.queue.front() != nullptr) {
            LevelQueue::Node* maker_node = level.queue.front();

            if (maker_node->trader == taker.trader) {
                // Self-trade prevention: stop sweeping rather than trade with ourselves.
                break;
            }

            const uint64_t fill =
                maker_node->quantity < taker.remaining() ? maker_node->quantity : taker.remaining();

            const uint64_t maker_order_id = maker_node->order_id;
            Order* maker = index_.find(maker_order_id);

            Trade trade;
            trade.id = next_trade_id_++;
            trade.maker_order = maker_order_id;
            trade.taker_order = taker.id;
            trade.pair = pair_id_;
            trade.price = price;
            trade.quantity = fill;
            trade.taker_fee = (price * fill * taker_fee_bps_) / 10'000;
            trade.timestamp = wall_ns();
            trades.push_back(trade);

            taker.filled += fill;
            level.queue.reduce_front(fill);

            if (maker != nullptr) {
                maker->filled += fill;
                if (maker->remaining() == 0) {
                    maker->status = OrderStatus::Filled;
                } else {
                    maker->status = OrderStatus::PartiallyFilled;
                }
            }

            ++total_trades_;
            progressed = true;
        }

        return progressed;
    }

    void rest(Order& order) {
        if (order.side == Side::Buy) {
            Level& level = bids_[order.price];
            level.price = order.price;
            level.queue.push_back(order.id, order.remaining(), order.trader);
        } else {
            Level& level = asks_[order.price];
            level.price = order.price;
            level.queue.push_back(order.id, order.remaining(), order.trader);
        }
    }

    /// Removes an order from its level queue.
    void detach(Order& order) {
        if (order.side == Side::Buy) {
            detach_from(bids_, order);
        } else {
            detach_from(asks_, order);
        }
    }

    template <typename Book>
    void detach_from(Book& book, Order& order) {
        auto it = book.find(order.price);
        if (it == book.end()) {
            return;
        }

        for (LevelQueue::Node* node = it->second.queue.front(); node != nullptr; node = node->next) {
            if (node->order_id == order.id) {
                it->second.queue.erase(node);
                break;
            }
        }

        if (it->second.queue.empty()) {
            book.erase(it);
        }
    }

    uint64_t pair_id_;
    uint64_t tick_size_;
    uint32_t maker_fee_bps_;
    uint32_t taker_fee_bps_;

    BidMap bids_;
    AskMap asks_;
    OrderIndex index_{1 << 16};

    uint64_t last_price_ = 0;
    uint64_t total_volume_ = 0;
    uint64_t total_trades_ = 0;
};

}  // namespace tigerswap
