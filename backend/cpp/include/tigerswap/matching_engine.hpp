// Multi-pair matching engine.
//
// The engine owns one order book per pair plus a pre-trade risk gate. Books are
// touched from the engine thread only; readers take a shared lock when they need
// a consistent snapshot, so matching itself stays lock-free of the read path.

#pragma once

#include <algorithm>
#include <atomic>
#include <memory>
#include <mutex>
#include <shared_mutex>
#include <string>
#include <unordered_map>
#include <vector>

#include "tigerswap/clock.hpp"
#include "tigerswap/order_book.hpp"
#include "tigerswap/types.hpp"

namespace tigerswap {

/// Per-pair risk limits applied before an order reaches the book.
struct RiskLimits {
    uint64_t max_order_quantity = 1'000'000'000'000ULL;
    uint64_t max_order_notional = 10'000'000'000'000ULL;
    uint32_t max_open_orders = 100'000;
    uint64_t min_order_quantity = 1;
};

/// Result of the pre-trade risk check.
struct RiskDecision {
    bool allowed = true;
    RejectReason reason = RejectReason::None;
};

/// Tracks a trader's exposure per pair.
struct TraderState {
    uint64_t open_orders = 0;
    uint64_t buy_notional = 0;
    uint64_t sell_notional = 0;
};

/// Engine-level counters.
struct EngineStats {
    uint64_t orders_accepted = 0;
    uint64_t orders_rejected = 0;
    uint64_t orders_cancelled = 0;
    uint64_t trades = 0;
    uint64_t volume = 0;
    uint64_t pairs = 0;
    uint64_t open_orders = 0;
    uint64_t latency_p50_ns = 0;
    uint64_t latency_p99_ns = 0;
    uint64_t latency_max_ns = 0;
};

/// Configures a tradable pair.
struct PairConfig {
    uint64_t pair_id = 0;
    std::string symbol;
    uint64_t tick_size = 1;
    uint32_t maker_fee_bps = 0;
    uint32_t taker_fee_bps = 2;
    RiskLimits risk;
};

/// One pair: its book, its risk limits and its latency histogram.
struct PairState {
    PairConfig config;
    std::unique_ptr<OrderBook> book;
    LatencyHistogram latency;
};

/// The matching engine. All mutation happens on the caller's thread.
class MatchingEngine {
public:
    MatchingEngine() = default;
    MatchingEngine(const MatchingEngine&) = delete;
    MatchingEngine& operator=(const MatchingEngine&) = delete;

    /// Registers a pair. Returns false if the pair already exists.
    bool add_pair(const PairConfig& config) {
        std::unique_lock lock(mutex_);
        if (pairs_.count(config.pair_id) != 0) {
            return false;
        }

        auto state = std::make_unique<PairState>();
        state->config = config;
        state->book = std::make_unique<OrderBook>(config.pair_id, config.tick_size,
                                                  config.maker_fee_bps, config.taker_fee_bps);
        pairs_.emplace(config.pair_id, std::move(state));
        return true;
    }

    /// Removes a pair and its book.
    bool remove_pair(uint64_t pair_id) {
        std::unique_lock lock(mutex_);
        return pairs_.erase(pair_id) > 0;
    }

    bool has_pair(uint64_t pair_id) const {
        std::shared_lock lock(mutex_);
        return pairs_.count(pair_id) != 0;
    }

    std::vector<PairConfig> list_pairs() const {
        std::shared_lock lock(mutex_);
        std::vector<PairConfig> out;
        out.reserve(pairs_.size());
        for (const auto& entry : pairs_) {
            out.push_back(entry.second->config);
        }
        return out;
    }

    /// Submits an order, applying risk checks, matching, and book updates.
    MatchResult submit(uint64_t pair_id, const Order& order, TimeInForce tif) {
        MatchResult result;

        PairState* pair = nullptr;
        {
            std::shared_lock lock(mutex_);
            auto it = pairs_.find(pair_id);
            if (it == pairs_.end()) {
                result.report.reason = RejectReason::UnknownOrder;
                return result;
            }
            pair = it->second.get();
        }

        const uint64_t start = CycleClock::cycles();

        Order candidate = order;
        candidate.pair = pair_id;

        const RiskDecision decision = check_risk(pair->config, candidate);
        if (!decision.allowed) {
            result.report.reason = decision.reason;
            rejected_.fetch_add(1, std::memory_order_relaxed);
            return result;
        }

        {
            std::unique_lock lock(mutex_);
            result = pair->book->submit(candidate, tif);
        }

        pair->latency.record(CycleClock::instance().to_ns(CycleClock::cycles() - start));

        if (result.report.accepted) {
            accepted_.fetch_add(1, std::memory_order_relaxed);
            trades_.fetch_add(result.trades.size(), std::memory_order_relaxed);
            uint64_t volume = 0;
            for (const auto& trade : result.trades) {
                volume += trade.quantity;
            }
            volume_.fetch_add(volume, std::memory_order_relaxed);
        } else {
            rejected_.fetch_add(1, std::memory_order_relaxed);
        }

        return result;
    }

    /// Cancels an order on a pair.
    ExecReport cancel(uint64_t pair_id, uint64_t order_id, uint64_t trader) {
        std::unique_lock lock(mutex_);

        auto it = pairs_.find(pair_id);
        if (it == pairs_.end()) {
            ExecReport report;
            report.reason = RejectReason::UnknownOrder;
            return report;
        }

        ExecReport report = it->second->book->cancel(order_id, trader);
        if (report.accepted) {
            cancelled_.fetch_add(1, std::memory_order_relaxed);
        }
        return report;
    }

    /// Applies a trade replayed from the log so market data survives a restart.
    ///
    /// This only updates last price and cumulative volume; it never touches the
    /// book, because the resting orders are restored separately.
    void record_recovered_trade(uint64_t pair_id, uint64_t price, uint64_t quantity) {
        std::unique_lock lock(mutex_);
        auto it = pairs_.find(pair_id);
        if (it == pairs_.end()) {
            return;
        }
        it->second->book->apply_recovered_trade(price, quantity);
        trades_.fetch_add(1, std::memory_order_relaxed);
        volume_.fetch_add(quantity, std::memory_order_relaxed);
    }

    /// Best bid and ask for a pair. Either value is 0 when that side is empty.
    std::pair<uint64_t, uint64_t> spread(uint64_t pair_id) const {
        std::shared_lock lock(mutex_);
        auto it = pairs_.find(pair_id);
        if (it == pairs_.end()) {
            return {0, 0};
        }
        return {it->second->book->best_bid(), it->second->book->best_ask()};
    }

    /// Aggregated depth for a pair.
    std::vector<PriceLevel> depth(uint64_t pair_id, size_t levels) const {
        std::shared_lock lock(mutex_);
        auto it = pairs_.find(pair_id);
        if (it == pairs_.end()) {
            return {};
        }
        return it->second->book->depth(levels);
    }

    /// Last traded price for a pair, or 0 when nothing has traded.
    uint64_t last_price(uint64_t pair_id) const {
        std::shared_lock lock(mutex_);
        auto it = pairs_.find(pair_id);
        return it == pairs_.end() ? 0 : it->second->book->last_price();
    }

    /// Finds an order on a pair.
    const Order* find_order(uint64_t pair_id, uint64_t order_id) const {
        std::shared_lock lock(mutex_);
        auto it = pairs_.find(pair_id);
        return it == pairs_.end() ? nullptr : it->second->book->find(order_id);
    }

    EngineStats stats() const {
        std::shared_lock lock(mutex_);

        EngineStats out;
        out.orders_accepted = accepted_.load(std::memory_order_relaxed);
        out.orders_rejected = rejected_.load(std::memory_order_relaxed);
        out.orders_cancelled = cancelled_.load(std::memory_order_relaxed);
        out.trades = trades_.load(std::memory_order_relaxed);
        out.volume = volume_.load(std::memory_order_relaxed);
        out.pairs = pairs_.size();

        uint64_t p50 = 0;
        uint64_t p99 = 0;
        uint64_t max_ns = 0;

        for (const auto& entry : pairs_) {
            const PairState& pair = *entry.second;
            out.open_orders += pair.book->open_orders();
            p50 = std::max(p50, pair.latency.percentile_ns(0.50));
            p99 = std::max(p99, pair.latency.percentile_ns(0.99));
            max_ns = std::max(max_ns, pair.latency.max_ns());
        }

        out.latency_p50_ns = p50;
        out.latency_p99_ns = p99;
        out.latency_max_ns = max_ns;
        return out;
    }

    /// Total number of registered pairs.
    size_t pair_count() const {
        std::shared_lock lock(mutex_);
        return pairs_.size();
    }

private:
    RiskDecision check_risk(const PairConfig& config, const Order& order) const {
        const RiskLimits& limits = config.risk;

        if (order.quantity < limits.min_order_quantity) {
            return {false, RejectReason::InvalidOrder};
        }
        if (order.quantity > limits.max_order_quantity) {
            return {false, RejectReason::InvalidOrder};
        }

        if (order.type == OrderType::Limit) {
            const uint64_t notional = order.price * order.quantity;
            if (notional > limits.max_order_notional) {
                return {false, RejectReason::InvalidOrder};
            }
        }

        return {};
    }

    mutable std::shared_mutex mutex_;
    std::unordered_map<uint64_t, std::unique_ptr<PairState>> pairs_;

    std::atomic<uint64_t> accepted_{0};
    std::atomic<uint64_t> rejected_{0};
    std::atomic<uint64_t> cancelled_{0};
    std::atomic<uint64_t> trades_{0};
    std::atomic<uint64_t> volume_{0};
};

}  // namespace tigerswap
