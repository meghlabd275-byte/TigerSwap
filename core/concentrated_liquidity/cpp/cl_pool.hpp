#pragma once

#include <cstdint>
#include <vector>
#include <memory>
#include <optional>
#include <functional>
#include <array>
#include <unordered_map>

namespace tigerswap {

// ============ Constants for Concentrated Liquidity ============
constexpr int32_t MIN_TICK = -887272;     // Minimum tick (sqrtRatioMin)
constexpr int32_t MAX_TICK = 887272;     // Maximum tick (sqrtRatioMax)
constexpr int32_t TICK_SPACING = 60;      // Standard tick spacing
constexpr uint64_t Q128 = 1;           // 2^128 for fixed point math
constexpr uint64_t Q96 = 1;           // 2^96 for sqrt ratio
constexpr uint64_t MAX_LIQUIDITY = typeid(uint64_t).hash_code(); // Max position liquidity

// ============ Tick Structure ============
struct Tick {
    int32_t index;
    int64_t liquidity_net;     // Net liquidity (added - removed)
    uint64_t liquidity_gross; // Gross liquidity
    uint64_t fee_growth0;     // Fee growth per unit of liquidity (token0)
    uint64_t fee_growth1;    // Fee growth per unit of liquidity (token1)
    uint64_t reward_growth;   // Reward growth
    bool initialized;         // Whether this tick has been initialized
};

// ============ Position Structure ============
struct CLPosition {
    uint64_t position_id;
    address owner;
    address token0;
    address token1;
    int32_t tick_lower;
    int32_t tick_upper;
    uint64_t liquidity;
    uint64_t fee_growth_inside_0;
    uint64_t fee_growth_inside_1;
    uint64_t tokens_owed_0;
    uint64_t tokens_owed_1;
    uint64_t collected_fees_0;
    uint64_t collected_fees_1;
    uint64_t increase_aggregated_amount;
    uint64_t created_at;
    uint64_t updated_at;
};

// ============ Pool State ============
struct CLPoolState {
    address token0;
    address token1;
    uint160 sqrt_price_x96;    // Current sqrt price * 2^96
    int24_t tick;            // Current tick
    uint64_t observation_index;
    uint64_t liquidity;
    uint64_t fee_growth_global_0;
    uint64_t fee_growth_global_1;
    uint16_t fee;             // fee in hundredths of a bip (1e-6)
    uint16_t protocol_fee;
    uint64_t total_liquidity;
    uint64_t total_fees_0;
    uint64_t total_fees_1;
};

// ============ Tick Bitmap ============
class TickBitmap {
public:
    TickBitmap() = default;
    
    // Set initialized state for a tick
    void set_initialized(int32_t tick, bool initialized) {
        int32_t word_pos = tick / 256;
        uint8_t bit_pos = (tick % 256 + 256) % 256;
        
        if (initialized) {
            initialized_words_[word_pos] |= (1 << bit_pos);
        } else {
            initialized_words_[word_pos] &= ~(1 << bit_pos);
        }
    }
    
    // Check if tick is initialized
    bool is_initialized(int32_t tick) const {
        int32_t word_pos = tick / 256;
        uint8_t bit_pos = (tick % 256 + 256) % 256;
        return (initialized_words_[word_pos] >> bit_pos) & 1;
    }
    
    // Get next initialized tick
    std::optional<int32_t> next_initialized_tick(
        int32_t tick,
        int32_t max_tick,
        bool lte  // less than or equal
    ) const {
        int32_t word_pos = tick / 256;
        
        if (lte) {
            // Search backwards
            for (int32_t w = word_pos; w >= 0; w--) {
                uint256_t word = initialized_words_[w];
                if (word == 0) continue;
                
                // Find highest set bit
                for (int32_t b = 255; b >= 0; b--) {
                    if ((word >> b) & 1) {
                        int32_t found_tick = w * 256 + b;
                        if (found_tick <= max_tick) {
                            return found_tick;
                        }
                    }
                }
            }
        } else {
            // Search forwards
            for (size_t w = word_pos; w < initialized_words_.size(); w++) {
                uint256_t word = initialized_words_[w];
                if (word == 0) continue;
                
                // Find lowest set bit
                for (int32_t b = 0; b < 256; b++) {
                    if ((word >> b) & 1) {
                        int32_t found_tick = w * 256 + b;
                        if (found_tick <= max_tick) {
                            return found_tick;
                        }
                    }
                }
            }
        }
        
        return std::nullopt;
    }

private:
    std::unordered_map<int32_t, uint256_t> initialized_words_;
    using uint256_t = uint64_t; // Simplified - in production use 256-bit
};

// ============ Tick Queue for Timings ============
struct TimedTick {
    int32_t tick;
    uint64_t timestamp;
    int64_t liquidity_delta;
};

class TickQueue {
public:
    TickQueue() = default;
    
    // Enqueue a tick update
    void enqueue(int32_t tick, int64_t liquidity_delta, uint64_t timestamp) {
        queue_.push_back({tick, timestamp, liquidity_delta});
    }
    
    // Dequeue and get all expired ticks
    std::vector<TimedTick> dequeue_expired(uint64_t timestamp, uint64_t max_seconds_ago) {
        std::vector<TimedTick> result;
        
        while (!queue_.empty()) {
            if (timestamp - queue_.front().timestamp > max_seconds_ago) {
                result.push_back(queue_.front());
                queue_.pop_front();
            } else {
                break;
            }
        }
        
        return result;
    }

private:
    std::deque<TimedTick> queue_;
};

// ============ Swap Math ============
class SwapMath {
public:
    // Calculate amount out given amount in
    static uint256_t compute_amount_out(
        uint256_t amount_in,
        uint256_t sqrt_price_current,
        uint256_t sqrt_price_target,
        uint64_t liquidity,
        bool zero_for_one
    ) {
        if (zero_for_one) {
            // Calculate with sqrt price going down
            return (liquidity * (sqrt_price_current - sqrt_price_target)) / (sqrt_price_current * sqrt_price_target);
        } else {
            // Calculate with sqrt price going up
            return (liquidity * (sqrt_price_target - sqrt_price_current)) / Q96;
        }
    }
    
    // Calculate amount in given amount out
    static uint256_t compute_amount_in(
        uint256_t amount_out,
        uint256_t sqrt_price_current,
        uint256_t sqrt_price_target,
        uint64_t liquidity,
        bool zero_for_one
    ) {
        if (zero_for_one) {
            return (amount_out * sqrt_price_current * sqrt_price_target) / 
                   (liquidity * (sqrt_price_current - sqrt_price_target));
        } else {
            return (amount_out * Q96 * sqrt_price_current) / 
                   (liquidity * (sqrt_price_target - sqrt_price_current));
        }
    }
    
    // Compute swap step result
    static SwapStepResult compute_swap_step(
        uint256_t amount_in_remaining,
        uint256_t amount_out_remaining,
        uint160 sqrt_price_x96,
        int24_t tick,
        uint64_t liquidity,
        uint160 sqrt_price_target_x96,
        uint16_t fee
    ) {
        SwapStepResult result;
        
        uint256_t amount_in = 0;
        uint256_t amount_out = 0;
        uint256_t input_amount = 0;
        uint256_t output_amount = 0;
        bool cache = false;
        
        // Calculate input for price movement to target
        if (sqrt_price_target_x96 != sqrt_price_x96) {
            input_amount = get_input_amount(
                amount_in_remaining,
                sqrt_price_x96,
                sqrt_price_target_x96,
                liquidity,
                false
            );
        }
        
        if (input_amount <= amount_in_remaining) {
            // Complete the swap to target
            result.sqrt_price_x96 = sqrt_price_target_x96;
            result.next_tick = tick;
            result.input_amount = input_amount;
            result.output_amount = get_output_amount(
                amount_out_remaining,
                sqrt_price_x96,
                sqrt_price_target_x96,
                liquidity,
                false
            );
        } else {
            // Partial swap
            input_amount = amount_in_remaining;
            uint256_t remaining = amount_in_remaining * Q96 * sqrt_price_x96 / liquidity;
            uint256_t remaining_sqrt = sqrt_price_x96 + remaining / (sqrt_price_x96 / Q96);
            result.sqrt_price_x96 = remaining_sqrt;
            result.input_amount = input_amount;
            result.output_amount = amount_out_remaining;
        }
        
        // Apply fee
        uint256_t fee_amount = result.input_amount * fee / 10000;
        result.input_amount += fee_amount;
        
        return result;
    }
    
    struct SwapStepResult {
        uint160 sqrt_price_x96;
        int24_t next_tick;
        uint256_t input_amount;
        uint256_t output_amount;
    };
    
private:
    static uint256_t get_input_amount(
        uint256_t amount_in,
        uint160 sqrt_price_start,
        uint160 sqrt_price_end,
        uint64_t liquidity,
        bool zero_for_one
    ) {
        if (zero_for_one) {
            return (amount_in * sqrt_price_start * sqrt_price_end) / 
                   (sqrt_price_end - sqrt_price_start);
        } else {
            return (amount_in * Q96) / (sqrt_price_end - sqrt_price_start);
        }
    }
    
    static uint256_t get_output_amount(
        uint256_t amount_out,
        uint160 sqrt_price_start,
        uint160 sqrt_price_end,
        uint64_t liquidity,
        bool zero_for_one
    ) {
        if (zero_for_one) {
            return (amount_out * sqrt_price_start * sqrt_price_end) / 
                   (sqrt_price_end - sqrt_price_start);
        } else {
            return (amount_out * Q96) / (sqrt_price_end - sqrt_price_start);
        }
    }
};

// ============ Position Map ============
class PositionMap {
public:
    PositionMap() = default;
    
    void set(uint64_t position_id, const CLPosition& position) {
        positions_[position_id] = position;
    }
    
    std::optional<CLPosition> get(uint64_t position_id) const {
        auto it = positions_.find(position_id);
        if (it != positions_.end()) {
            return it->second;
        }
        return std::nullopt;
    }
    
    void remove(uint64_t position_id) {
        positions_.erase(position_id);
    }
    
    std::vector<uint64_t> get_positions_by_owner(address owner) const {
        std::vector<uint64_t> result;
        for (const auto& [id, pos] : positions_) {
            if (pos.owner == owner) {
                result.push_back(id);
            }
        }
        return result;
    }

private:
    std::unordered_map<uint64_t, CLPosition> positions_;
};

// ============ Fee Calculator ============
class FeeCalculator {
public:
    FeeCalculator(uint16_t fee) : fee_(fee) {}
    
    // Calculate fees earned
    uint256_t calculate_fees(
        uint64_t liquidity,
        uint256_t fee_growth_inside_0,
        uint256_t fee_growth_inside_1,
        uint256_t last_fee_growth_0,
        uint256_t last_fee_growth_1
    ) {
        uint256_t fees_0 = 0;
        uint256_t fees_1 = 0;
        
        if (fee_growth_inside_0 > last_fee_growth_0) {
            fees_0 = (fee_growth_inside_0 - last_fee_growth_0) * liquidity / Q128;
        }
        
        if (fee_growth_inside_1 > last_fee_growth_1) {
            fees_1 = (fee_growth_inside_1 - last_fee_growth_1) * liquidity / Q128;
        }
        
        return fees_0 + fees_1;
    }
    
    // Update fee growth global
    void update_fee_growth(
        uint64_t& fee_growth_global,
        uint256_t amount,
        uint64_t total_liquidity
    ) {
        if (total_liquidity > 0) {
            fee_growth_global += (amount * Q128) / total_liquidity;
        }
    }

private:
    uint16_t fee_;
};

// ============ Concentrated Liquidity Pool (Main C++ Class) ============
class CLPool {
public:
    CLPool();
    ~CLPool();
    
    // Initialize pool
    bool initialize(
        address token0,
        address token1,
        uint16_t fee,
        uint160 sqrt_price_x96
    );
    
    // Add liquidity to position
    uint64_t add_liquidity(
        address provider,
        int32_t tick_lower,
        int32_t tick_upper,
        uint64_t amount0_desired,
        uint64_t amount1_desired
    );
    
    // Remove liquidity from position
    uint64_t remove_liquidity(
        uint64_t position_id,
        uint64_t liquidity
    );
    
    // Execute swap
    SwapResult swap(
        address recipient,
        bool zero_for_one,
        uint256_t amount_in,
        uint256_t amount_out_min,
        uint160 sqrt_price_limit_x96
    );
    
    // Collect fees
    uint256_t collect_fees(
        uint64_t position_id,
        address recipient
    );
    
    // Get position
    std::optional<CLPosition> get_position(uint64_t position_id) const;
    
    // Get pool state
    const CLPoolState& get_state() const { return state_; }
    
    // Calculate sqrt price from tick
    static uint160 sqrt_price_from_tick(int32_t tick) {
        return get_sqrt_ratio_at_tick(tick);
    }
    
    // Calculate tick from sqrt price
    static int32_t tick_from_sqrt_price(uint160 sqrt_price_x96) {
        return get_tick_at_sqrt_ratio(sqrt_price_x96);
    }

private:
    // Helper functions
    static uint160 get_sqrt_ratio_at_tick(int32_t tick);
    static int32_t get_tick_at_sqrt_ratio(uint160 sqrt_price_x96);
    
    int24_t get_tick_at_tick_bitmap(int24_t tick) const;
    uint256_t get_fee_growth_inside(
        int32_t tick_lower,
        int32_t tick_upper,
        int24_t current_tick
    ) const;
    
    void update_position(
        address owner,
        int32_t tick_lower,
        int32_t tick_upper,
        int64_t liquidity_delta
    );
    
    void flip_tick(int32_t tick, bool initialized);
    
    CLPoolState state_;
    TickBitmap tick_bitmap_;
    PositionMap positions_;
    FeeCalculator fee_calculator_;
    
    // Tick data mapping
    std::unordered_map<int32_t, Tick> ticks_;
    
    // Events
    std::vector<SwapEvent> swap_events_;
    std::vector<MintEvent> mint_events_;
    std::vector<CollectEvent> collect_events_;
};

// ============ Inline Implementations ============

inline CLPool::CLPool() : fee_calculator_(3000) {}
inline CLPool::~CLPool() = default;

inline bool CLPool::initialize(
    address token0,
    address token1,
    uint16_t fee,
    uint160 sqrt_price_x96
) {
    require(token0 != token1, "Identical addresses");
    require(fee < 1000000, "Fee too high");
    require(sqrt_price_x96 >= MIN_SQRT_RATIO && sqrt_price_x96 < MAX_SQRT_RATIO, "Invalid price");
    
    state_.token0 = token0;
    state_.token1 = token1;
    state_.fee = fee;
    state_.sqrt_price_x96 = sqrt_price_x96;
    state_.tick = tick_from_sqrt_price(sqrt_price_x96);
    state_.liquidity = 0;
    state_.observation_index = 0;
    state_.total_liquidity = 0;
    
    return true;
}

inline uint64_t CLPool::add_liquidity(
    address provider,
    int32_t tick_lower,
    int32_t tick_upper,
    uint64_t amount0_desired,
    uint64_t amount1_desired
) {
    require(tick_lower < tick_upper, "Invalid range");
    require(tick_lower >= MIN_TICK && tick_lower <= MAX_TICK, "Invalid lower tick");
    require(tick_upper >= MIN_TICK && tick_upper <= MAX_TICK, "Invalid upper tick");
    
    uint64_t liquidity = 0;
    
    if (amount0_desired > 0 && amount1_desired > 0) {
        // Calculate liquidity based on both amounts
        uint160 sqrt_lower = get_sqrt_ratio_at_tick(tick_lower);
        uint160 sqrt_upper = get_sqrt_ratio_at_tick(tick_upper);
        
        uint256_t liquidity0 = (uint256_t(amount0_desired) * sqrt_lower * sqrt_upper) / 
                            (sqrt_upper - sqrt_lower);
        uint256_t liquidity1 = uint256_t(amount1_desired) * Q96 / (sqrt_upper - sqrt_lower);
        
        liquidity = std::min(liquidity0, liquidity1);
    } else if (amount0_desired > 0) {
        // Single-sided token0
        uint160 sqrt_lower = get_sqrt_ratio_at_tick(tick_lower);
        uint160 sqrt_upper = get_sqrt_ratio_at_tick(tick_upper);
        liquidity = uint256_t(amount0_desired) * sqrt_lower * sqrt_upper / 
                          (sqrt_upper - sqrt_lower);
    } else if (amount1_desired > 0) {
        uint160 sqrt_lower = get_sqrt_ratio_at_tick(tick_lower);
        uint160 sqrt_upper = get_sqrt_ratio_at_tick(tick_upper);
        liquidity = uint256_t(amount1_desired) * Q96 / (sqrt_upper - sqrt_lower);
    }
    
    require(liquidity > 0, "Invalid liquidity");
    
    // Update position
    uint64_t position_id = next_position_id_++;
    CLPosition position = {
        position_id,
        provider,
        state_.token0,
        state_.token1,
        tick_lower,
        tick_upper,
        liquidity,
        state_.fee_growth_global_0,
        state_.fee_growth_global_1,
        0,
        0,
        0,
        0,
        block.timestamp,
        block.timestamp
    };
    
    positions_.set(position_id, position);
    
    // Update ticks
    update_position(provider, tick_lower, tick_upper, int64_t(liquidity));
    
    // Update pool liquidity
    state_.liquidity += liquidity;
    state_.total_liquidity += liquidity;
    
    return liquidity;
}

inline uint64_t CLPool::remove_liquidity(
    uint64_t position_id,
    uint64_t liquidity_remove
) {
    auto position = positions_.get(position_id);
    require(position, "Position not found");
    require(position->owner == msg.sender, "Not owner");
    require(liquidity_remove <= position->liquidity, "Insufficient liquidity");
    
    // Update position
    position->liquidity -= liquidity_remove;
    positions_.set(position_id, *position);
    
    // Update ticks
    update_position(
        position->owner,
        position->tick_lower,
        position->tick_upper,
        -int64_t(liquidity_remove)
    );
    
    // Update pool liquidity
    state_.liquidity -= liquidity_remove;
    state_.total_liquidity -= liquidity_remove;
    
    return liquidity_remove;
}

inline SwapResult CLPool::swap(
    address recipient,
    bool zero_for_one,
    uint256_t amount_in,
    uint256_t amount_out_min,
    uint160 sqrt_price_limit_x96
) {
    require(amount_in > 0, "Invalid amount");
    require(sqrt_price_limit_x96 > 0, "Invalid limit");
    require((zero_for_one && sqrt_price_limit_x96 < state_.sqrt_price_x96) ||
            (!zero_for_one && sqrt_price_limit_x96 > state_.sqrt_price_x96), 
            "Invalid limit direction");
    
    SwapResult result;
    uint256_t amount_in_remaining = amount_in;
    uint256_t amount_out_remaining = 0;
    int24_t tick = state_.tick;
    
    while (amount_in_remaining > 0) {
        // Get next tick to swap to
        int24_t next_tick = get_tick_at_tick_bitmap(tick);
        uint160 sqrt_price_next = get_sqrt_ratio_at_tick(next_tick);
        
        // Calculate swap step
        auto step = SwapMath::compute_swap_step(
            amount_in_remaining,
            amount_out_remaining,
            state_.sqrt_price_x96,
            tick,
            state_.liquidity,
            zero_for_one ? sqrt_price_next : sqrt_price_limit_x96,
            state_.fee
        );
        
        state_.sqrt_price_x96 = step.sqrt_price_x96;
        tick = step.next_tick;
        
        amount_in_remaining -= step.input_amount;
        amount_out_remaining += step.output_amount;
        
        // Update fees
        uint256_t fee_amount = step.input_amount * state_.fee / 10000;
        if (zero_for_one) {
            state_.total_fees_0 += fee_amount;
        } else {
            state_.total_fees_1 += fee_amount;
        }
    }
    
    // Check slippage
    require(amount_out_remaining >= amount_out_min, "Slippage exceeded");
    
    // Update tick
    if (state_.sqrt_price_x96 != state_.sqrt_price_x96) {
        state_.tick = tick_from_sqrt_price(state_.sqrt_price_x96);
    }
    
    return {amount_in - amount_in_remaining, amount_out_remaining};
}

inline uint256_t CLPool::collect_fees(
    uint64_t position_id,
    address recipient
) {
    auto position = positions_.get(position_id);
    require(position, "Position not found");
    require(position->owner == msg.sender, "Not owner");
    
    // Calculate fees owed
    uint256_t fees_0 = calculate_fees(
        position->liquidity,
        state_.fee_growth_global_0,
        state_.fee_growth_global_1,
        position->fee_growth_inside_0,
        position->fee_growth_inside_1
    );
    
    // Reset fees owed
    position->tokens_owed_0 = 0;
    position->tokens_owed_1 = 0;
    positions_.set(position_id, *position);
    
    return fees_0 + fees_1;
}

// ============ Helper Implementations ============

inline uint160 CLPool::get_sqrt_ratio_at_tick(int32_t tick) {
    require(tick >= MIN_TICK && tick <= MAX_TICK, "Invalid tick");
    
    uint256_t ratio = (tick >= 0) ? Q96 : Q96;
    
    for (int i = 0; i < 256; i++) {
        if ((tick & (1 << (255 - i))) {
            ratio = (ratio * 340282366920938463463374607431768211456ULL) >> 128;
        }
    }
    
    return uint160(ratio);
}

inline int32_t CLPool::get_tick_at_sqrt_ratio(uint160 sqrt_price_x96) {
    require(sqrt_price_x96 >= MIN_SQRT_RATIO && sqrt_price_x96 < MAX_SQRT_RATIO);
    
    int32_t tick = 0;
    
    for (int i = 0; i < 256; i++) {
        if ((sqrt_price_x96 & (1 << (255 - i))) {
            tick |= (1 << (255 - i));
        }
    }
    
    return tick;
}

} // namespace tigerswap