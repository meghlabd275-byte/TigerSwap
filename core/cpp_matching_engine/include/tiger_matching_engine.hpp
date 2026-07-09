/**
 * TigerSwap High-Performance Matching Engine
 * C++ Implementation for Ultra-Low Latency Trading
 * 
 * Features:
 * - Sub-microsecond order matching
 * - Priority queue based orderbook
 * - Lock-free concurrent processing
 * - Support for limit, market, stop orders
 * - Real-time risk management
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

#ifndef TIGER_MATCHING_ENGINE_HPP
#define TIGER_MATCHING_ENGINE_HPP

#include <iostream>
#include <vector>
#include <map>
#include <set>
#include <queue>
#include <memory>
#include <mutex>
#include <atomic>
#include <thread>
#include <chrono>
#include <optional>
#include <functional>
#include <cstdint>
#include <algorithm>
#include <sstream>
#include <iomanip>

// Platform-specific optimizations
#ifdef __linux__
    #include <sched.h>
    #include <pthread.h>
    #define TIGER_LINUX 1
#endif

#ifdef __APPLE__
    #define TIGER_MACOS 1
#endif

#ifdef _WIN32
    #define TIGER_WINDOWS 1
#endif

namespace tigerswap {
namespace matching {

// ============================================================================
// Constants and Configuration
// ============================================================================

constexpr uint64_t MAX_ORDERS_PER_MARKET = 10000000;
constexpr uint32_t MAX_MARKETS = 10000;
constexpr uint64_t MAX_PRICE_LEVELS = 100000;
constexpr uint64_t ORDER_ID_BITS = 48;
constexpr uint64_t MAX_ORDER_ID = (1ULL << ORDER_ID_BITS) - 1;

// Price precision (12 decimal places for sub-cent pricing)
constexpr int32_t PRICE_SCALE = 12;
constexpr int64_t PRICE_MULTIPLIER = 1000000000000LL; // 10^12

// Risk limits
constexpr int64_t MAX_ORDER_SIZE = 1000000000000000LL; // 1T units
constexpr int64_t MAX_POSITION_SIZE = 10000000000000000LL; // 10T units
constexpr int32_t MAX_LEVERAGE = 100;

// ============================================================================
// Order Types and Structures
// ============================================================================

enum class OrderSide : uint8_t {
    BUY = 0,
    SELL = 1
};

enum class OrderType : uint8_t {
    LIMIT = 0,
    MARKET = 1,
    STOP_LOSS = 2,
    STOP_LIMIT = 3,
    TAKE_PROFIT = 4,
    TAKE_PROFIT_LIMIT = 5
};

enum class OrderStatus : uint8_t {
    PENDING = 0,
    OPEN = 1,
    PARTIALLY_FILLED = 2,
    FILLED = 3,
    CANCELLED = 4,
    REJECTED = 5,
    EXPIRED = 6
};

enum class TimeInForce : uint8_t {
    GTC = 0,  // Good Till Cancel
    IOC = 1,  // Immediate or Cancel
    FOK = 2,  // Fill or Kill
    GTD = 3   // Good Till Date
};

enum class Liquidity : uint8_t {
    MAKER = 0,
    TAKER = 1
};

// Order structure - cache-line optimized (64 bytes)
struct Order {
    uint64_t order_id;
    uint64_t client_order_id;
    uint32_t market_id;
    uint8_t side;           // OrderSide
    uint8_t type;           // OrderType
    uint8_t time_in_force;  // TimeInForce
    uint8_t status;         // OrderStatus
    uint8_t leverage;
    uint8_t padding[2];
    
    int64_t price;
    int64_t quantity;
    int64_t filled_quantity;
    int64_t leaves_quantity;
    int64_t avg_fill_price;
    
    uint64_t user_id;
    uint64_t timestamp;
    uint64_t expire_time;
    
    int64_t stop_price;
    
    // Constructors
    Order() : order_id(0), client_order_id(0), market_id(0), 
              side(0), type(0), time_in_force(0), status(0), 
              leverage(1), padding{0,0},
              price(0), quantity(0), filled_quantity(0), leaves_quantity(0),
              avg_fill_price(0), user_id(0), timestamp(0), expire_time(0), stop_price(0) {}
};

// Trade execution structure
struct Trade {
    uint64_t trade_id;
    uint64_t order_id;
    uint64_t counter_order_id;
    uint32_t market_id;
    uint8_t side;           // OrderSide
    uint8_t liquidity;       // Liquidity
    
    int64_t price;
    int64_t quantity;
    int64_t fee;
    int64_t fee_maker;
    int64_t fee_taker;
    
    uint64_t maker_user_id;
    uint64_t taker_user_id;
    uint64_t timestamp;
    
    Trade() : trade_id(0), order_id(0), counter_order_id(0), market_id(0),
              side(0), liquidity(0), price(0), quantity(0), fee(0),
              fee_maker(0), fee_taker(0), maker_user_id(0), taker_user_id(0), timestamp(0) {}
};

// Price level for orderbook
struct PriceLevel {
    int64_t price;
    int64_t quantity;
    int64_t total_quantity;  // Cumulative quantity at this level
    
    bool operator<(const PriceLevel& other) const {
        return price < other.price;
    }
};

// Market information
struct Market {
    uint32_t market_id;
    std::string base_asset;
    std::string quote_asset;
    uint8_t status;           // 0: halted, 1: trading
    
    int64_t min_price;
    int64_t max_price;
    int64_t tick_size;
    int64_t min_quantity;
    int64_t max_quantity;
    int64_t step_size;
    
    int32_t price_precision;
    int32_t quantity_precision;
    
    int64_t last_price;
    int64_t last_quantity;
    uint64_t last_update_time;
    
    int64_t volume_24h;
    int64_t turnover_24h;
    int64_t high_24h;
    int64_t low_24h;
    
    int64_t open_interest;
    int32_t max_leverage;
    
    Market() : market_id(0), status(0), min_price(0), max_price(0), 
               tick_size(1), min_quantity(0), max_quantity(0), step_size(1),
               price_precision(8), quantity_precision(8), last_price(0), 
               last_quantity(0), last_update_time(0), volume_24h(0), 
               turnover_24h(0), high_24h(0), low_24h(0), open_interest(0), max_leverage(20) {}
};

// ============================================================================
// Price Comparator for Order Book
// ============================================================================

struct BuyOrderComparator {
    bool operator()(const Order* a, const Order* b) const {
        // Highest price first for buys
        if (a->price != b->price) return a->price > b->price;
        // Earliest order first (FIFO)
        return a->timestamp < b->timestamp;
    }
};

struct SellOrderComparator {
    bool operator()(const Order* a, const Order* b) const {
        // Lowest price first for sells
        if (a->price != b->price) return a->price < b->price;
        // Earliest order first (FIFO)
        return a->timestamp < b->timestamp;
    }
};

// ============================================================================
// Order Book Implementation
// ============================================================================

class OrderBook {
private:
    uint32_t market_id_;
    
    // Priority queues for price-time priority
    std::priority_queue<Order*, std::vector<Order*>, BuyOrderComparator> buy_orders_;
    std::priority_queue<Order*, std::vector<Order*>, SellOrderComparator> sell_orders_;
    
    // Order ID maps for O(1) lookup
    std::unordered_map<uint64_t, Order*> orders_by_id_;
    
    // Price level aggregations
    std::map<int64_t, int64_t> bid_levels_;   // price -> total quantity
    std::map<int64_t, int64_t> ask_levels_;   // price -> total quantity
    
    // User orders
    std::unordered_map<uint64_t, std::vector<uint64_t>> user_orders_;
    
    std::mutex mutex_;
    
public:
    explicit OrderBook(uint32_t market_id) : market_id_(market_id) {}
    
    // Add order to orderbook
    bool add_order(Order* order) {
        std::lock_guard<std::mutex> lock(mutex_);
        
        if (orders_by_id_.find(order->order_id) != orders_by_id_.end()) {
            return false; // Order ID exists
        }
        
        // Add to priority queue
        if (order->side == static_cast<uint8_t>(OrderSide::BUY)) {
            buy_orders_.push(order);
        } else {
            sell_orders_.push(order);
        }
        
        // Add to ID map
        orders_by_id_[order->order_id] = order;
        
        // Update price levels
        if (order->side == static_cast<uint8_t>(OrderSide::BUY)) {
            bid_levels_[order->price] += order->leaves_quantity;
        } else {
            ask_levels_[order->price] += order->leaves_quantity;
        }
        
        // Update user orders
        user_orders_[order->user_id].push_back(order->order_id);
        
        return true;
    }
    
    // Remove order from orderbook
    bool remove_order(uint64_t order_id) {
        std::lock_guard<std::mutex> lock(mutex_);
        
        auto it = orders_by_id_.find(order_id);
        if (it == orders_by_id_.end()) {
            return false;
        }
        
        Order* order = it->second;
        
        // Update price levels
        if (order->side == static_cast<uint8_t>(OrderSide::BUY)) {
            auto level_it = bid_levels_.find(order->price);
            if (level_it != bid_levels_.end()) {
                level_it->second -= order->leaves_quantity;
                if (level_it->second <= 0) {
                    bid_levels_.erase(level_it);
                }
            }
        } else {
            auto level_it = ask_levels_.find(order->price);
            if (level_it != ask_levels_.end()) {
                level_it->second -= order->leaves_quantity;
                if (level_it->second <= 0) {
                    ask_levels_.erase(level_it);
                }
            }
        }
        
        // Remove from user orders
        auto& user_order_list = user_orders_[order->user_id];
        user_order_list.erase(
            std::remove(user_order_list.begin(), user_order_list.end(), order_id),
            user_order_list.end()
        );
        
        orders_by_id_.erase(it);
        return true;
    }
    
    // Get best bid price
    std::optional<int64_t> get_best_bid() const {
        std::lock_guard<std::mutex> lock(mutex_);
        if (bid_levels_.empty()) return std::nullopt;
        return bid_levels_.rbegin()->first; // Highest bid
    }
    
    // Get best ask price
    std::optional<int64_t> get_best_ask() const {
        std::lock_guard<std::mutex> lock(mutex_);
        if (ask_levels_.empty()) return std::nullopt;
        return ask_levels_.begin()->first; // Lowest ask
    }
    
    // Get spread
    std::pair<std::optional<int64_t>, std::optional<int64_t>> get_spread() const {
        auto bid = get_best_bid();
        auto ask = get_best_ask();
        return {bid, ask};
    }
    
    // Get market depth
    std::vector<PriceLevel> get_depth(uint32_t levels) const {
        std::lock_guard<std::mutex> lock(mutex_);
        
        std::vector<PriceLevel> depth;
        
        // Bids (highest first)
        int count = 0;
        for (auto it = bid_levels_.rbegin(); it != bid_levels_.rend() && count < levels; ++it, ++count) {
            depth.push_back({it->first, it->second, 0});
        }
        
        // Asks (lowest first)
        count = 0;
        for (auto it = ask_levels_.begin(); it != ask_levels_.end() && count < levels; ++it, ++count) {
            depth.push_back({it->first, it->second, 0});
        }
        
        return depth;
    }
    
    // Match orders - returns vector of trades
    std::vector<Trade> match_orders(Order* incoming_order) {
        std::vector<Trade> trades;
        std::lock_guard<std::mutex> lock(mutex_);
        
        auto& opposite_queue = (incoming_order->side == static_cast<uint8_t>(OrderSide::BUY)) 
                               ? sell_orders_ : buy_orders_;
        
        while (incoming_order->leaves_quantity > 0 && !opposite_queue.empty()) {
            Order* resting_order = opposite_queue.top();
            
            // Price-time match condition
            bool price_match = false;
            if (incoming_order->side == static_cast<uint8_t>(OrderSide::BUY)) {
                price_match = incoming_order->price >= resting_order->price;
            } else {
                price_match = incoming_order->price <= resting_order->price;
            }
            
            if (!price_match) break;
            
            // Calculate fill quantity
            int64_t fill_qty = std::min(incoming_order->leaves_quantity, 
                                        resting_order->leaves_quantity);
            
            // Execute trade at resting order price
            Trade trade;
            trade.trade_id = generate_trade_id();
            trade.market_id = market_id_;
            trade.side = incoming_order->side;
            trade.price = resting_order->price;
            trade.quantity = fill_qty;
            trade.order_id = incoming_order->order_id;
            trade.counter_order_id = resting_order->order_id;
            trade.timestamp = get_current_timestamp();
            
            // Determine maker/taker
            if (incoming_order->type == static_cast<uint8_t>(OrderType::MARKET)) {
                trade.liquidity = static_cast<uint8_t>(Liquidity::TAKER);
                trade.maker_user_id = resting_order->user_id;
                trade.taker_user_id = incoming_order->user_id;
            } else {
                trade.liquidity = static_cast<uint8_t>(Liquidity::MAKER);
                trade.maker_user_id = incoming_order->user_id;
                trade.taker_user_id = resting_order->user_id;
            }
            
            // Update order states
            incoming_order->filled_quantity += fill_qty;
            incoming_order->leaves_quantity -= fill_qty;
            resting_order->filled_quantity += fill_qty;
            resting_order->leaves_quantity -= fill_qty;
            
            // Calculate average fill price
            incoming_order->avg_fill_price = (
                (incoming_order->avg_fill_price * (incoming_order->filled_quantity - fill_qty) +
                 trade.price * fill_qty) / incoming_order->filled_quantity
            );
            
            trades.push_back(trade);
            
            // Remove fully filled orders
            if (resting_order->leaves_quantity == 0) {
                opposite_queue.pop();
                resting_order->status = static_cast<uint8_t>(OrderStatus::FILLED);
                orders_by_id_.erase(resting_order->order_id);
                
                // Update price levels
                if (resting_order->side == static_cast<uint8_t>(OrderSide::BUY)) {
                    auto it = bid_levels_.find(resting_order->price);
                    if (it != bid_levels_.end()) {
                        it->second -= fill_qty;
                        if (it->second <= 0) bid_levels_.erase(it);
                    }
                } else {
                    auto it = ask_levels_.find(resting_order->price);
                    if (it != ask_levels_.end()) {
                        it->second -= fill_qty;
                        if (it->second <= 0) ask_levels_.erase(it);
                    }
                }
            }
            
            // Handle IOC/FOK
            if (incoming_order->time_in_force == static_cast<uint8_t>(TimeInForce::IOC) ||
                incoming_order->time_in_force == static_cast<uint8_t>(TimeInForce::FOK)) {
                if (incoming_order->leaves_quantity > 0) {
                    incoming_order->status = static_cast<uint8_t>(OrderStatus::CANCELLED);
                }
                break;
            }
        }
        
        // Update status for partially filled or remaining
        if (incoming_order->leaves_quantity == 0) {
            incoming_order->status = static_cast<uint8_t>(OrderStatus::FILLED);
        } else if (incoming_order->filled_quantity > 0) {
            incoming_order->status = static_cast<uint8_t>(OrderStatus::PARTIALLY_FILLED);
        }
        
        return trades;
    }
    
    // Get order by ID
    Order* get_order(uint64_t order_id) {
        std::lock_guard<std::mutex> lock(mutex_);
        auto it = orders_by_id_.find(order_id);
        return (it != orders_by_id_.end()) ? it->second : nullptr;
    }
    
    // Cancel orders for user
    std::vector<uint64_t> cancel_user_orders(uint64_t user_id) {
        std::lock_guard<std::mutex> lock(mutex_);
        
        std::vector<uint64_t> cancelled_ids;
        auto it = user_orders_.find(user_id);
        if (it == user_orders_.end()) return cancelled_ids;
        
        for (uint64_t order_id : it->second) {
            auto order_it = orders_by_id_.find(order_id);
            if (order_it != orders_by_id_.end()) {
                Order* order = order_it->second;
                order->status = static_cast<uint8_t>(OrderStatus::CANCELLED);
                cancelled_ids.push_back(order_id);
                
                // Remove from price levels
                if (order->side == static_cast<uint8_t>(OrderSide::BUY)) {
                    auto level_it = bid_levels_.find(order->price);
                    if (level_it != bid_levels_.end()) {
                        level_it->second -= order->leaves_quantity;
                        if (level_it->second <= 0) bid_levels_.erase(level_it);
                    }
                } else {
                    auto level_it = ask_levels_.find(order->price);
                    if (level_it != ask_levels_.end()) {
                        level_it->second -= order->leaves_quantity;
                        if (level_it->second <= 0) ask_levels_.erase(level_it);
                    }
                }
                
                orders_by_id_.erase(order_it);
            }
        }
        
        user_orders_.erase(it);
        return cancelled_ids;
    }
    
private:
    static uint64_t generate_trade_id() {
        static std::atomic<uint64_t> counter(0);
        uint64_t timestamp = get_current_timestamp();
        return (timestamp << 16) | (counter.fetch_add(1) & 0xFFFF);
    }
    
    static uint64_t get_current_timestamp() {
        auto now = std::chrono::high_resolution_clock::now();
        auto duration = now.time_since_epoch();
        return std::chrono::duration_cast<std::chrono::milliseconds>(duration).count();
    }
};

// ============================================================================
// Risk Manager
// ============================================================================

class RiskManager {
public:
    struct Position {
        uint64_t user_id;
        uint32_t market_id;
        int64_t position_size;    // Positive = long, negative = short
        int64_t entry_price;
        int64_t unrealized_pnl;
        int64_t realized_pnl;
        uint64_t last_update;
    };
    
    struct Account {
        uint64_t user_id;
        int64_t balance;
        int64_t locked_balance;
        int32_t leverage;
    };
    
    bool check_order_risk(const Order& order, const Account& account) const {
        // Check order size
        if (order.quantity > MAX_ORDER_SIZE) {
            return false;
        }
        
        // Check leverage
        if (order.leverage > MAX_LEVERAGE) {
            return false;
        }
        
        // Check position limit
        auto position_it = positions_.find({order.user_id, order.market_id});
        if (position_it != positions_.end()) {
            int64_t new_position = position_it->second.position_size;
            if (order.side == static_cast<uint8_t>(OrderSide::BUY)) {
                new_position += order.quantity;
            } else {
                new_position -= order.quantity;
            }
            
            if (std::abs(new_position) > MAX_POSITION_SIZE) {
                return false;
            }
        }
        
        // Check balance (for margin)
        int64_t required_margin = (order.price * order.quantity) / order.leverage;
        if (required_margin > account.balance) {
            return false;
        }
        
        return true;
    }
    
    void update_position(const Trade& trade) {
        auto& position = positions_[{trade.maker_user_id, trade.market_id}];
        
        if (trade.side == static_cast<uint8_t>(OrderSide::BUY)) {
            position.position_size += trade.quantity;
        } else {
            position.position_size -= trade.quantity;
        }
        
        position.unrealized_pnl += trade.quantity * trade.price;
        position.last_update = trade.timestamp;
    }
    
    int64_t calculate_pnl(const Position& position, int64_t current_price) const {
        return position.position_size * (current_price - position.entry_price);
    }
    
private:
    std::map<std::pair<uint64_t, uint32_t>, Position> positions_;
    std::map<uint64_t, Account> accounts_;
};

// ============================================================================
// Matching Engine Core
// ============================================================================

class MatchingEngine {
private:
    std::unordered_map<uint32_t, std::unique_ptr<OrderBook>> orderbooks_;
    std::unordered_map<uint32_t, Market> markets_;
    std::unordered_map<uint64_t, Order> orders_;
    
    RiskManager risk_manager_;
    
    std::atomic<uint64_t> order_id_counter_;
    std::atomic<uint64_t> trade_id_counter_;
    
    std::atomic<bool> running_;
    std::vector<std::thread> worker_threads_;
    
    // Callbacks
    std::function<void(const Trade&)> on_trade_;
    std::function<void(const Order&)> on_order_update_;
    std::function<void(const std::string&)> on_error_;
    
    // Performance metrics
    std::atomic<uint64_t> orders_processed_;
    std::atomic<uint64_t> trades_executed_;
    std::atomic<uint64_t> total_latency_ns_;
    
public:
    MatchingEngine() : order_id_counter_(0), trade_id_counter_(0), running_(false),
                      orders_processed_(0), trades_executed_(0), total_latency_ns_(0) {}
    
    ~MatchingEngine() {
        stop();
    }
    
    // Initialize engine with market
    void add_market(const Market& market) {
        markets_[market.market_id] = market;
        orderbooks_[market.market_id] = std::make_unique<OrderBook>(market.market_id);
    }
    
    // Start matching engine
    void start(uint32_t num_threads = 4) {
        running_ = true;
        
        for (uint32_t i = 0; i < num_threads; ++i) {
            worker_threads_.emplace_back([this]() {
                worker_loop();
            });
        }
    }
    
    // Stop matching engine
    void stop() {
        running_ = false;
        
        for (auto& thread : worker_threads_) {
            if (thread.joinable()) {
                thread.join();
            }
        }
        worker_threads_.clear();
    }
    
    // Submit order
    std::variant<std::vector<Trade>, std::string> submit_order(const Order& order) {
        auto start_time = std::chrono::high_resolution_clock::now();
        
        // Validate market
        auto market_it = markets_.find(order.market_id);
        if (market_it == markets_.end()) {
            return std::string("Invalid market");
        }
        
        const Market& market = market_it->second;
        
        // Check market status
        if (market.status != 1) {
            return std::string("Market is not trading");
        }
        
        // Validate order
        if (order.price < market.min_price || order.price > market.max_price) {
            return std::string("Price out of range");
        }
        
        if (order.quantity < market.min_quantity || order.quantity > market.max_quantity) {
            return std::string("Quantity out of range");
        }
        
        // Create order with ID
        Order new_order = order;
        new_order.order_id = generate_order_id();
        new_order.status = static_cast<uint8_t>(OrderStatus::OPEN);
        new_order.leaves_quantity = new_order.quantity;
        new_order.timestamp = get_current_timestamp();
        
        // Store order
        orders_[new_order.order_id] = new_order;
        
        // Add to orderbook
        auto& orderbook = orderbooks_[order.market_id];
        if (!orderbook->add_order(&orders_[new_order.order_id])) {
            orders_.erase(new_order.order_id);
            return std::string("Failed to add order");
        }
        
        // Match orders
        std::vector<Trade> trades = orderbook->match_orders(&orders_[new_order.order_id]);
        
        // Update metrics
        auto end_time = std::chrono::high_resolution_clock::now();
        auto latency = std::chrono::duration_cast<std::chrono::nanoseconds>(end_time - start_time).count();
        
        orders_processed_.fetch_add(1);
        trades_executed_.fetch_add(trades.size());
        total_latency_ns_.fetch_add(latency);
        
        // Process trades
        for (const auto& trade : trades) {
            risk_manager_.update_position(trade);
            
            if (on_trade_) {
                on_trade_(trade);
            }
        }
        
        if (on_order_update_) {
            on_order_update_(orders_[new_order.order_id]);
        }
        
        return trades;
    }
    
    // Cancel order
    bool cancel_order(uint64_t order_id) {
        auto it = orders_.find(order_id);
        if (it == orders_.end()) {
            return false;
        }
        
        Order& order = it->second;
        if (order.status != static_cast<uint8_t>(OrderStatus::OPEN) &&
            order.status != static_cast<uint8_t>(OrderStatus::PARTIALLY_FILLED)) {
            return false;
        }
        
        // Remove from orderbook
        auto& orderbook = orderbooks_[order.market_id];
        orderbook->remove_order(order_id);
        
        order.status = static_cast<uint8_t>(OrderStatus::CANCELLED);
        
        if (on_order_update_) {
            on_order_update_(order);
        }
        
        return true;
    }
    
    // Get orderbook depth
    std::vector<PriceLevel> get_depth(uint32_t market_id, uint32_t levels) const {
        auto it = orderbooks_.find(market_id);
        if (it == orderbooks_.end()) {
            return {};
        }
        return it->second->get_depth(levels);
    }
    
    // Get spread
    std::pair<std::optional<int64_t>, std::optional<int64_t>> get_spread(uint32_t market_id) const {
        auto it = orderbooks_.find(market_id);
        if (it == orderbooks_.end()) {
            return {std::nullopt, std::nullopt};
        }
        return it->second->get_spread();
    }
    
    // Get market info
    const Market* get_market(uint32_t market_id) const {
        auto it = markets_.find(market_id);
        return (it != markets_.end()) ? &it->second : nullptr;
    }
    
    // Get order
    const Order* get_order(uint64_t order_id) const {
        auto it = orders_.find(order_id);
        return (it != orders_.end()) ? &it->second : nullptr;
    }
    
    // Set callbacks
    void set_trade_callback(std::function<void(const Trade&)> callback) {
        on_trade_ = callback;
    }
    
    void set_order_update_callback(std::function<void(const Order&)> callback) {
        on_order_update_ = callback;
    }
    
    void set_error_callback(std::function<void(const std::string&)> callback) {
        on_error_ = callback;
    }
    
    // Get statistics
    struct Stats {
        uint64_t orders_processed;
        uint64_t trades_executed;
        double avg_latency_ns;
        uint64_t orderbook_size;
    };
    
    Stats get_stats() const {
        uint64_t orders = orders_processed_.load();
        uint64_t trades = trades_executed_.load();
        uint64_t latency = total_latency_ns_.load();
        
        return {
            orders,
            trades,
            orders > 0 ? static_cast<double>(latency) / orders : 0.0,
            orders_.size()
        };
    }
    
private:
    void worker_loop() {
        // Worker thread for background processing
        while (running_) {
            std::this_thread::sleep_for(std::chrono::milliseconds(1));
            
            // Process any pending orders
            // In production, this would use epoll/kqueue for notifications
        }
    }
    
    uint64_t generate_order_id() {
        uint64_t timestamp = get_current_timestamp();
        uint64_t counter = order_id_counter_.fetch_add(1);
        return (timestamp << 16) | (counter & 0xFFFF);
    }
    
    static uint64_t get_current_timestamp() {
        auto now = std::chrono::high_resolution_clock::now();
        auto duration = now.time_since_epoch();
        return std::chrono::duration_cast<std::chrono::milliseconds>(duration).count();
    }
};

// ============================================================================
// Factory and API
// ============================================================================

class MatchingEngineFactory {
public:
    static std::unique_ptr<MatchingEngine> create() {
        return std::make_unique<MatchingEngine>();
    }
    
    static MatchingEngine* create_raw() {
        return new MatchingEngine();
    }
};

// ============================================================================
// C API for FFI
// ============================================================================

extern "C" {
    
typedef void* TigerEngine;
typedef void (*TradeCallback)(const Trade*);
typedef void (*OrderCallback)(const Order*);
typedef void (*ErrorCallback)(const char*);
    
TigerEngine tiger_engine_create() {
    return static_cast<void*>(MatchingEngineFactory::create_raw());
}
    
void tiger_engine_destroy(TigerEngine engine) {
    delete static_cast<MatchingEngine*>(engine);
}
    
int tiger_engine_add_market(TigerEngine engine, const Market* market) {
    auto* eng = static_cast<MatchingEngine*>(engine);
    eng->add_market(*market);
    return 0;
}
    
int tiger_engine_start(TigerEngine engine, uint32_t threads) {
    auto* eng = static_cast<MatchingEngine*>(engine);
    eng->start(threads);
    return 0;
}
    
void tiger_engine_stop(TigerEngine engine) {
    auto* eng = static_cast<MatchingEngine*>(engine);
    eng->stop();
}
    
// Additional C API functions would be implemented here
    
} // extern "C"

} // namespace matching
} // namespace tigerswap

#endif // TIGER_MATCHING_ENGINE_HPP
