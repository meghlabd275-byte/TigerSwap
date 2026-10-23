// Low-latency clock helpers for the TigerSwap matching engine.
#pragma once

#include <atomic>
#include <chrono>
#include <cstdint>

#if defined(__x86_64__) || defined(__i386__)
#include <x86intrin.h>
#define TIGERSWAP_HAS_RDTSC 1
#else
#define TIGERSWAP_HAS_RDTSC 0
#endif

namespace tigerswap {

/// Nanoseconds since the Unix epoch.
inline uint64_t wall_ns() {
    return static_cast<uint64_t>(
        std::chrono::duration_cast<std::chrono::nanoseconds>(
            std::chrono::system_clock::now().time_since_epoch())
            .count());
}

/// Monotonic nanoseconds, suitable for latency measurement.
inline uint64_t monotonic_ns() {
    return static_cast<uint64_t>(
        std::chrono::duration_cast<std::chrono::nanoseconds>(
            std::chrono::steady_clock::now().time_since_epoch())
            .count());
}

/// Serialising read of the CPU cycle counter.
///
/// `_mm_lfence` keeps the read from being reordered ahead of earlier loads, so
/// a pair of reads around a region measures that region and not surrounding
/// work. On non-x86 targets this falls back to the monotonic clock.
class CycleClock {
public:
    CycleClock() { calibrate(); }

    static CycleClock& instance() {
        static CycleClock clock;
        return clock;
    }

    /// Raw cycle counter value.
    static uint64_t cycles() {
#if TIGERSWAP_HAS_RDTSC
        _mm_lfence();
        const uint64_t value = __rdtsc();
        _mm_lfence();
        return value;
#else
        return monotonic_ns();
#endif
    }

    /// Converts a cycle delta to nanoseconds using the calibrated frequency.
    uint64_t to_ns(uint64_t cycles) const {
        if (ns_per_cycle_ == 0.0) {
            return cycles;
        }
        return static_cast<uint64_t>(static_cast<double>(cycles) * ns_per_cycle_);
    }

    double ns_per_cycle() const { return ns_per_cycle_; }

private:
    void calibrate() {
        constexpr uint64_t sample_ns = 20'000'000;  // 20 ms
        const uint64_t start_cycles = cycles();
        const uint64_t start_ns = monotonic_ns();

        uint64_t elapsed_ns = 0;
        while (elapsed_ns < sample_ns) {
            elapsed_ns = monotonic_ns() - start_ns;
        }

        const uint64_t end_cycles = cycles();
        const uint64_t cycle_delta = end_cycles - start_cycles;

        ns_per_cycle_ =
            cycle_delta == 0 ? 0.0 : static_cast<double>(elapsed_ns) / static_cast<double>(cycle_delta);
    }

    double ns_per_cycle_ = 0.0;
};

/// Measures a scope in nanoseconds using the calibrated cycle clock.
class ScopedTimer {
public:
    ScopedTimer() : start_(CycleClock::cycles()) {}

    uint64_t elapsed_ns() const {
        return CycleClock::instance().to_ns(CycleClock::cycles() - start_);
    }

private:
    uint64_t start_;
};

/// Lock-free latency histogram with fixed microsecond buckets.
class LatencyHistogram {
public:
    static constexpr size_t kBuckets = 20;

    void record(uint64_t ns) {
        size_t bucket = 0;
        uint64_t bound = 100;  // 100 ns
        while (bucket + 1 < kBuckets && ns > bound) {
            bound *= 2;
            ++bucket;
        }

        buckets_[bucket].fetch_add(1, std::memory_order_relaxed);
        count_.fetch_add(1, std::memory_order_relaxed);
        total_ns_.fetch_add(ns, std::memory_order_relaxed);

        uint64_t current = min_ns_.load(std::memory_order_relaxed);
        while (ns < current &&
               !min_ns_.compare_exchange_weak(current, ns, std::memory_order_relaxed)) {
        }

        current = max_ns_.load(std::memory_order_relaxed);
        while (ns > current &&
               !max_ns_.compare_exchange_weak(current, ns, std::memory_order_relaxed)) {
        }
    }

    uint64_t count() const { return count_.load(std::memory_order_relaxed); }
    uint64_t min_ns() const { return min_ns_.load(std::memory_order_relaxed); }
    uint64_t max_ns() const { return max_ns_.load(std::memory_order_relaxed); }

    uint64_t mean_ns() const {
        const uint64_t count = count_.load(std::memory_order_relaxed);
        return count == 0 ? 0 : total_ns_.load(std::memory_order_relaxed) / count;
    }

    /// Approximate percentile from the power-of-two bucket layout.
    uint64_t percentile_ns(double percentile) const {
        const uint64_t count = count_.load(std::memory_order_relaxed);
        if (count == 0) {
            return 0;
        }

        const uint64_t target = static_cast<uint64_t>(static_cast<double>(count) * percentile);
        uint64_t running = 0;
        uint64_t bound = 100;

        for (size_t bucket = 0; bucket < kBuckets; ++bucket) {
            running += buckets_[bucket].load(std::memory_order_relaxed);
            if (running >= target) {
                return bound;
            }
            bound *= 2;
        }

        return bound;
    }

    void reset() {
        for (auto& bucket : buckets_) {
            bucket.store(0, std::memory_order_relaxed);
        }
        count_.store(0, std::memory_order_relaxed);
        total_ns_.store(0, std::memory_order_relaxed);
        min_ns_.store(UINT64_MAX, std::memory_order_relaxed);
        max_ns_.store(0, std::memory_order_relaxed);
    }

private:
    std::atomic<uint64_t> buckets_[kBuckets] = {};
    std::atomic<uint64_t> count_{0};
    std::atomic<uint64_t> total_ns_{0};
    std::atomic<uint64_t> min_ns_{UINT64_MAX};
    std::atomic<uint64_t> max_ns_{0};
};

}  // namespace tigerswap
