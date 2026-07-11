#pragma once

#include <atomic>
#include <chrono>
#include <memory>
#include <mutex>
#include <queue>
#include <shared_mutex>
#include <string>
#include <unordered_map>
#include <vector>

namespace tigerswap {

// High-precision timestamp using steady clock
using Timestamp = std::chrono::steady_clock::time_point;
using Duration = std::chrono::microseconds;

// Order types
enum class OrderType : uint8_t {
    MARKET = 0,
    LIMIT = 1,
    STOP_LOSS = 2,
    TAKE_PROFIT = 3,
    STOP_LIMIT = 4
};

// Order side
enum class OrderSide : uint8_t {
    BUY = 0,
    SELL = 1
};

// Order status
enum class OrderStatus : uint8_t {
    PENDING = 0,
    OPEN = 1,
    PARTIALLY_FILLED = 2,
    FILLED = 3,
    CANCELLED = 4,
    REJECTED = 5
};

// Order structure with minimal memory footprint for cache efficiency
struct alignas(64) Order {
    uint64_t order_id;
    uint64_t user_id;
    uint64_t pair_id;
    OrderType type;
    OrderSide side;
    OrderStatus status;
    int64_t price;        // Scaled by 1e8
    int64_t quantity;    // Token units
    int64_t filled;
    int64_t remaining;
    int64_t stop_price;  // For stop orders
    uint32_t pair_hash;
    Timestamp created_at;
    Timestamp updated_at;
    Timestamp expires_at;
    uint8_t time_in_force;
    bool is_post_only;
    char _padding[6];
    
    Order() : order_id(0), user_id(0), pair_id(0), type(OrderType::LIMIT),
              side(OrderSide::BUY), status(OrderStatus::PENDING), price(0),
              quantity(0), filled(0), remaining(0), stop_price(0), pair_hash(0),
              time_in_force(1), is_post_only(false) {}
};

// Trade execution structure
struct Trade {
    uint64_t trade_id;
    uint64_t order_id;
    uint64_t counter_order_id;
    uint64_t maker_order_id;
    uint64_t taker_order_id;
    int64_t price;
    int64_t quantity;
    int64_t fee;
    Timestamp executed_at;
    uint8_t side; // 0 = buy, 1 = sell
    char _padding[7];
};

// Market data structure
struct MarketData {
    uint64_t pair_id;
    int64_t last_price;
    int64_t bid_price;
    int64_t ask_price;
    int64_t bid_quantity;
    int64_t ask_quantity;
    int64_t volume_24h;
    int64_t high_24h;
    int64_t low_24h;
    int64_t open_interest;
    Timestamp timestamp;
};

// Order book level
struct Level {
    int64_t price;
    int64_t quantity;
    uint32_t order_count;
};

// Order book side (bids or asks)
struct OrderBookSide {
    std::vector<Level> levels;
    std::unordered_map<uint64_t, Order> orders;
    mutable std::shared_mutex mutex;
    
    void add_order(const Order& order);
    void remove_order(uint64_t order_id);
    void modify_order(uint64_t order_id, int64_t new_quantity);
    std::vector<Level> get_top_levels(size_t count) const;
};

// Order book for a trading pair
class OrderBook {
public:
    OrderBook(uint64_t pair_id);
    
    // Order management
    bool add_order(const Order& order);
    bool cancel_order(uint64_t order_id);
    bool modify_order(uint64_t order_id, int64_t new_quantity);
    
    // Matching
    std::vector<Trade> match_orders();
    
    // Getters
    const OrderBookSide& bids() const { return bids_; }
    const OrderBookSide& asks() const { return asks_; }
    MarketData get_market_data() const;
    
    // Price improvement
    int64_t get_best_bid() const;
    int64_t get_best_ask() const;
    int64_t get_mid_price() const;
    int64_t get_spread() const;

private:
    uint64_t pair_id_;
    OrderBookSide bids_;
    OrderBookSide asks_;
    std::atomic<uint64_t> last_trade_id_{0};
    mutable std::shared_mutex mutex_;
    
    bool can_match(const Order& buy_order, const Order& sell_order) const;
    Trade create_trade(const Order& maker, const Order& taker, int64_t price, int64_t quantity);
};

// Trading pair configuration
struct TradingPair {
    uint64_t pair_id;
    std::string base_token;
    std::string quote_token;
    uint32_t chain_id;
    int decimals_base;
    int decimals_quote;
    int price_precision;
    int quantity_precision;
    int64_t min_quantity;
    int64_t max_quantity;
    int64_t min_notional;
    uint32_t maker_fee;
    uint32_t taker_fee;
    bool is_trading_enabled;
    bool is_margin_enabled;
    uint8_t price_feed_source;
};

// Risk parameters
struct RiskParams {
    int64_t max_order_size;
    int64_t max_notional_value;
    int64_t max_daily_volume;
    uint32_t max_open_orders;
    int64_t max_position;
    uint8_t risk_level; // 0 = low, 1 = medium, 2 = high
    bool enable_circuit_breaker;
    int64_t circuit_breaker_threshold;
    uint32_t circuit_breaker_duration;
};

// Order book manager
class OrderBookManager {
public:
    static OrderBookManager& instance();
    
    OrderBook* get_or_create_orderbook(uint64_t pair_id);
    OrderBook* get_orderbook(uint64_t pair_id);
    void remove_orderbook(uint64_t pair_id);
    
    void set_trading_pair(const TradingPair& pair);
    const TradingPair* get_trading_pair(uint64_t pair_id) const;
    
    std::vector<uint64_t> get_active_pairs() const;
    size_t get_orderbook_count() const;

private:
    OrderBookManager() = default;
    ~OrderBookManager() = default;
    OrderBookManager(const OrderBookManager&) = delete;
    OrderBookManager& operator=(const OrderBookManager&) = delete;
    
    std::unordered_map<uint64_t, std::unique_ptr<OrderBook>> orderbooks_;
    std::unordered_map<uint64_t, TradingPair> trading_pairs_;
    mutable std::shared_mutex mutex_;
};

// Configuration
struct EngineConfig {
    std::string host;
    uint16_t port;
    uint32_t worker_threads;
    uint32_t max_connections;
    bool enable_matching;
    bool enable_risk_check;
    bool enable_price_feed;
    uint32_t matching_interval_us;
    uint32_t stats_interval_ms;
    std::string log_level;
    std::string data_dir;
};

} // namespace tigerswap
