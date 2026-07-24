/**
 * TigerSwap Order Book - Ultra Low Latency C++ Implementation
 * High-performance central limit order book (CLOB) for perpetual trading
 * Compile: g++ -O3 -march=native -o order_book order_book.cpp -lpthread
 */

#ifndef TIGERSWAP_ORDER_BOOK_HPP
#define TIGERSWAP_ORDER_BOOK_HPP

#include <iostream>
#include <fstream>
#include <sstream>
#include <string>
#include <vector>
#include <map>
#include <unordered_map>
#include <set>
#include <mutex>
#include <shared_mutex>
#include <atomic>
#include <chrono>
#include <thread>
#include <queue>
#include <optional>
#include <cstring>
#include <algorithm>
#include <iomanip>
#include <csignal>

// ============== CONSTANTS ==============
constexpr int MAX_ORDERS = 1000000;
constexpr int MAX_PRICE_LEVELS = 10000;
constexpr uint64_t NANOSECONDS_PER_SECOND = 1000000000ULL;
constexpr uint64_t MAX_ORDER_LIFETIME_MS = 86400000; // 24 hours

// Order types
enum class OrderType : uint8_t {
    MARKET = 0,
    LIMIT = 1,
    STOP_LOSS = 2,
    STOP_LOSS_LIMIT = 3,
    TAKE_PROFIT = 4,
    TAKE_PROFIT_LIMIT = 5
};

// Order side
enum class Side : uint8_t {
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

// Time in force
enum class TimeInForce : uint8_t {
    GTC = 0,  // Good till cancel
    IOC = 1,  // Immediate or cancel
    FOK = 2,  // Fill or kill
    GTD = 3   // Good till date
};

// ============== PRICE LEVEL ==============
struct PriceLevel {
    uint64_t price;
    uint64_t quantity;
    uint32_t orders;

    bool operator<(const PriceLevel& other) const { return price < other.price; }
    bool operator>(const PriceLevel& other) const { return price > other.price; }
};

// ============== ORDER ==============
struct Order {
    uint64_t order_id;
    std::string trader_address;
    std::string pair;           // e.g., "BTC-USD", "ETH-USD"
    Side side;
    OrderType type;
    TimeInForce tif;
    uint64_t price;
    uint64_t quantity;
    uint64_t filled_quantity;
    uint64_t leaves_quantity;
    uint64_t stop_price;
    uint64_t created_at;
    uint64_t expires_at;
    OrderStatus status;
    uint8_t priority;           // For price-time priority

    Order() : order_id(0), price(0), quantity(0), filled_quantity(0), 
              leaves_quantity(0), stop_price(0), created_at(0), expires_at(0),
              status(OrderStatus::PENDING), priority(0) {}
};

// ============== TRADE ==============
struct Trade {
    uint64_t trade_id;
    uint64_t order_id;
    uint64_t maker_order_id;
    uint64_t taker_order_id;
    std::string pair;
    Side side;
    uint64_t price;
    uint64_t quantity;
    uint64_t fee;
    uint64_t created_at;

    Trade() : trade_id(0), order_id(0), maker_order_id(0), taker_order_id(0),
              price(0), quantity(0), fee(0), created_at(0) {}
};

// ============== ORDER BOOK ==============
class OrderBook {
private:
    // Price-time priority book (best price first, then time)
    std::map<uint64_t, PriceLevel, std::greater<uint64_t>> bids_;   // Descending for buys
    std::map<uint64_t, PriceLevel, std::less<uint64_t>> asks_;      // Ascending for sells
    
    // Order storage - O(1) lookup
    std::unordered_map<uint64_t, Order> orders_;
    
    // Index by trader for quick lookup
    std::unordered_map<std::string, std::vector<uint64_t>> trader_orders_;
    
    // Recent trades
    std::vector<Trade> recent_trades_;
    
    // Market data
    std::string current_pair_;
    uint64_t last_trade_price_;
    uint64_t last_24h_volume_;
    uint64_t last_24h_high_;
    uint64_t last_24h_low_;
    
    // Counters
    std::atomic<uint64_t> next_order_id_{1};
    std::atomic<uint64_t> next_trade_id_{1};
    std::atomic<uint64_t> total_orders_{0};
    std::atomic<uint64_t> total_trades_{0};
    
    // Statistics
    std::atomic<uint64_t> orders_processed_{0};
    std::chrono::high_resolution_clock::time_point start_time_;
    
    // Thread safety
    mutable std::shared_mutex mutex_;
    
    // Persistence
    std::ofstream trade_log_;
    std::mutex log_mutex_;

public:
    OrderBook(const std::string& pair = "BTC-USD") 
        : current_pair_(pair), last_trade_price_(0), last_24h_volume_(0),
          last_24h_high_(0), last_24h_low_(0) {
        start_time_ = std::chrono::high_resolution_clock::now();
        
        // Open trade log
        trade_log_.open("/tmp/tigerswap_trades.log", std::ios::app);
    }
    
    ~OrderBook() {
        if (trade_log_.is_open()) {
            trade_log_.close();
        }
    }

    // ============== ORDER OPERATIONS ==============
    
    /**
     * Submit a new order - ultra low latency
     * @return Order ID if successful, 0 if rejected
     */
    uint64_t submitOrder(
        const std::string& trader,
        const std::string& pair,
        Side side,
        OrderType type,
        uint64_t price,
        uint64_t quantity,
        TimeInForce tif = TimeInForce::GTC,
        uint64_t stop_price = 0,
        uint64_t expires_at = 0
    ) {
        auto start = std::chrono::high_resolution_clock::now();
        
        std::unique_lock lock(mutex_);
        
        // Validate order
        if (quantity == 0 || quantity > MAX_ORDERS) {
            return 0;
        }
        
        if (type == OrderType::LIMIT && price == 0) {
            return 0;
        }
        
        if ((type == OrderType::STOP_LOSS || type == OrderType::STOP_LOSS_LIMIT ||
             type == OrderType::TAKE_PROFIT || type == OrderType::TAKE_PROFIT_LIMIT) 
            && stop_price == 0) {
            return 0;
        }
        
        // Generate order ID
        uint64_t order_id = next_order_id_.fetch_add(1);
        
        // Get current timestamp
        auto now = std::chrono::duration_cast<std::chrono::milliseconds>(
            std::chrono::system_clock::now().time_since_epoch()
        ).count();
        
        // Create order
        Order order;
        order.order_id = order_id;
        order.trader_address = trader;
        order.pair = pair;
        order.side = side;
        order.type = type;
        order.tif = tif;
        order.price = price;
        order.quantity = quantity;
        order.filled_quantity = 0;
        order.leaves_quantity = quantity;
        order.stop_price = stop_price;
        order.created_at = now;
        order.expires_at = expires_at > 0 ? expires_at : (now + MAX_ORDER_LIFETIME_MS);
        order.status = OrderStatus::OPEN;
        order.priority = static_cast<uint8_t>(orders_processed_.load() & 0xFF);
        
        // Store order
        orders_[order_id] = order;
        trader_orders_[trader].push_back(order_id);
        
        // Process based on type
        bool success = false;
        if (type == OrderType::MARKET) {
            success = matchMarketOrder(order);
        } else if (type == OrderType::LIMIT) {
            success = addLimitOrder(order);
        } else {
            // Stop loss / take profit - add to watch list
            success = true;
        }
        
        if (!success) {
            order.status = OrderStatus::REJECTED;
            orders_[order_id] = order;
            return 0;
        }
        
        total_orders_.fetch_add(1);
        orders_processed_.fetch_add(1);
        
        auto end = std::chrono::high_resolution_clock::now();
        auto duration = std::chrono::duration_cast<std::chrono::nanoseconds>(end - start).count();
        
        // Log latency
        if (duration > 10000) { // > 10 microseconds
            std::cerr << "HIGH LATENCY: " << duration << "ns for order " << order_id << std::endl;
        }
        
        return order_id;
    }
    
    /**
     * Cancel an order
     */
    bool cancelOrder(uint64_t order_id, const std::string& trader) {
        std::unique_lock lock(mutex_);
        
        auto it = orders_.find(order_id);
        if (it == orders_.end()) {
            return false;
        }
        
        Order& order = it->second;
        
        // Verify ownership
        if (order.trader_address != trader) {
            return false;
        }
        
        if (order.status != OrderStatus::OPEN && 
            order.status != OrderStatus::PARTIALLY_FILLED) {
            return false;
        }
        
        // Remove from book if limit order
        if (order.type == OrderType::LIMIT) {
            removeFromBook(order);
        }
        
        order.status = OrderStatus::CANCELLED;
        order.leaves_quantity = 0;
        
        return true;
    }
    
    /**
     * Modify an order (cancel and replace)
     */
    uint64_t modifyOrder(uint64_t order_id, const std::string& trader, 
                         uint64_t new_price, uint64_t new_quantity) {
        // Cancel existing
        if (!cancelOrder(order_id, trader)) {
            return 0;
        }
        
        // Get original order details
        std::shared_lock lock(mutex_);
        auto it = orders_.find(order_id);
        if (it == orders_.end()) {
            return 0;
        }
        
        Order original = it->second;
        lock.unlock();
        
        // Submit new order
        return submitOrder(
            trader,
            original.pair,
            original.side,
            original.type,
            new_price,
            new_quantity,
            original.tif,
            original.stop_price,
            original.expires_at
        );
    }

    // ============== MARKET DATA ==============
    
    /**
     * Get best bid price
     */
    std::optional<uint64_t> getBestBid() const {
        std::shared_lock lock(mutex_);
        if (bids_.empty()) return std::nullopt;
        return bids_.begin()->first;
    }
    
    /**
     * Get best ask price
     */
    std::optional<uint64_t> getBestAsk() const {
        std::shared_lock lock(mutex_);
        if (asks_.empty()) return std::nullopt;
        return asks_.begin()->first;
    }
    
    /**
     * Get spread
     */
    double getSpread() const {
        std::shared_lock lock(mutex_);
        if (bids_.empty() || asks_.empty()) return 0;
        return static_cast<double>(asks_.begin()->first - bids_.begin()->first);
    }
    
    /**
     * Get order book depth
     */
    std::vector<PriceLevel> getDepth(int levels = 10) const {
        std::shared_lock lock(mutex_);
        
        std::vector<PriceLevel> depth;
        
        // Bids
        int count = 0;
        for (auto& [price, level] : bids_) {
            if (count++ >= levels) break;
            depth.push_back(level);
        }
        
        // Asks
        count = 0;
        for (auto& [price, level] : asks_) {
            if (count++ >= levels) break;
            depth.push_back(level);
        }
        
        return depth;
    }
    
    /**
     * Get recent trades
     */
    std::vector<Trade> getRecentTrades(int limit = 100) const {
        std::shared_lock lock(mutex_);
        
        std::vector<Trade> result;
        int start = recent_trades_.size() > limit ? recent_trades_.size() - limit : 0;
        
        for (size_t i = start; i < recent_trades_.size(); ++i) {
            result.push_back(recent_trades_[i]);
        }
        
        return result;
    }
    
    /**
     * Get order details
     */
    std::optional<Order> getOrder(uint64_t order_id) const {
        std::shared_lock lock(mutex_);
        
        auto it = orders_.find(order_id);
        if (it == orders_.end()) {
            return std::nullopt;
        }
        
        return it->second;
    }
    
    /**
     * Get all orders for a trader
     */
    std::vector<Order> getTraderOrders(const std::string& trader) const {
        std::shared_lock lock(mutex_);
        
        std::vector<Order> result;
        
        auto it = trader_orders_.find(trader);
        if (it != trader_orders_.end()) {
            for (uint64_t order_id : it->second) {
                auto order_it = orders_.find(order_id);
                if (order_it != orders_.end()) {
                    result.push_back(order_it->second);
                }
            }
        }
        
        return result;
    }

    // ============== STATISTICS ==============
    
    /**
     * Get market statistics
     */
    struct MarketStats {
        uint64_t last_price;
        uint64_t volume_24h;
        uint64_t high_24h;
        uint64_t low_24h;
        uint64_t total_orders;
        uint64_t total_trades;
        double avg_latency_ns;
        uint64_t orders_per_second;
    };
    
    MarketStats getStats() const {
        std::shared_lock lock(mutex_);
        
        auto now = std::chrono::high_resolution_clock::now();
        auto uptime = std::chrono::duration_cast<std::chrono::seconds>(now - start_time_).count();
        
        double avg_latency = 0; // Would calculate from logged latencies
        
        uint64_t ops = orders_processed_.load();
        uint64_t ops_per_sec = uptime > 0 ? ops / uptime : 0;
        
        return MarketStats{
            last_trade_price_,
            last_24h_volume_,
            last_24h_high_,
            last_24h_low_,
            total_orders_.load(),
            total_trades_.load(),
            avg_latency,
            ops_per_sec
        };
    }

private:
    // ============== MATCHING ENGINE ==============
    
    bool addLimitOrder(Order& order) {
        auto& book = order.side == Side::BUY ? bids_ : asks_;
        auto it = book.find(order.price);
        
        if (it == book.end()) {
            book[order.price] = {order.price, order.leaves_quantity, 1};
        } else {
            it->second.quantity += order.leaves_quantity;
            it->second.orders += 1;
        }
        
        return true;
    }
    
    void removeFromBook(const Order& order) {
        auto& book = order.side == Side::BUY ? bids_ : asks_;
        auto it = book.find(order.price);
        
        if (it != book.end()) {
            if (it->second.orders <= 1) {
                book.erase(it);
            } else {
                it->second.orders -= 1;
                if (it->second.quantity >= order.leaves_quantity) {
                    it->second.quantity -= order.leaves_quantity;
                }
            }
        }
    }
    
    bool matchMarketOrder(Order& order) {
        auto& book = order.side == Side::BUY ? asks_ : bids_;
        auto& opposite_book = order.side == Side::BUY ? bids_ : asks_;
        
        while (order.leaves_quantity > 0 && !book.empty()) {
            auto it = book.begin();
            uint64_t match_price = it->first;
            
            // Check if we can match
            if (order.type == OrderType::MARKET) {
                executeTrade(order, match_price, 
                           std::min(order.leaves_quantity, it->second.quantity));
            } else {
                // Limit order - check price
                if ((order.side == Side::BUY && match_price > order.price) ||
                    (order.side == Side::SELL && match_price < order.price)) {
                    break;
                }
                executeTrade(order, match_price,
                           std::min(order.leaves_quantity, it->second.quantity));
            }
            
            // Remove filled price level
            if (it->second.quantity == 0) {
                book.erase(it);
            }
        }
        
        // If order still has quantity and is limit, add to book
        if (order.leaves_quantity > 0 && order.type == OrderType::LIMIT) {
            addLimitOrder(order);
        }
        
        return true;
    }
    
    void executeTrade(Order& order, uint64_t price, uint64_t quantity) {
        uint64_t trade_id = next_trade_id_.fetch_add(1);
        
        // Create trade
        Trade trade;
        trade.trade_id = trade_id;
        trade.order_id = order.order_id;
        trade.pair = order.pair;
        trade.side = order.side;
        trade.price = price;
        trade.quantity = quantity;
        trade.fee = calculateFee(quantity, price);
        trade.created_at = std::chrono::duration_cast<std::chrono::milliseconds>(
            std::chrono::system_clock::now().time_since_epoch()
        ).count();
        
        // Update order
        order.filled_quantity += quantity;
        order.leaves_quantity -= quantity;
        
        if (order.leaves_quantity == 0) {
            order.status = OrderStatus::FILLED;
        } else {
            order.status = OrderStatus::PARTIALLY_FILLED;
        }
        
        // Update market data
        last_trade_price_ = price;
        last_24h_volume_ += quantity;
        if (last_24h_high_ == 0 || price > last_24h_high_) {
            last_24h_high_ = price;
        }
        if (last_24h_low_ == 0 || price < last_24h_low_) {
            last_24h_low_ = price;
        }
        
        // Store trade
        recent_trades_.push_back(trade);
        total_trades_.fetch_add(1);
        
        // Log trade
        logTrade(trade);
    }
    
    uint64_t calculateFee(uint64_t quantity, uint64_t price) {
        // 0.3% fee (matches TigerSwap)
        return (quantity * price * 3) / 1000;
    }
    
    void logTrade(const Trade& trade) {
        std::lock_guard lock(log_mutex_);
        
        if (trade_log_.is_open()) {
            trade_log_ << trade.trade_id << ","
                      << trade.order_id << ","
                      << trade.pair << ","
                      << (trade.side == Side::BUY ? "BUY" : "SELL") << ","
                      << trade.price << ","
                      << trade.quantity << ","
                      << trade.fee << ","
                      << trade.created_at << "\n";
            trade_log_.flush();
        }
    }
};

// ============== GLOBAL INSTANCE ==============
static OrderBook* g_order_book = nullptr;

extern "C" {

void* create_order_book(const char* pair) {
    if (g_order_book != nullptr) {
        delete g_order_book;
    }
    g_order_book = new OrderBook(pair);
    return g_order_book;
}

void destroy_order_book() {
    if (g_order_book != nullptr) {
        delete g_order_book;
        g_order_book = nullptr;
    }
}

uint64_t submit_order(
    const char* trader,
    const char* pair,
    uint8_t side,
    uint8_t type,
    uint64_t price,
    uint64_t quantity,
    uint8_t tif,
    uint64_t stop_price,
    uint64_t expires_at
) {
    if (g_order_book == nullptr) return 0;
    
    return g_order_book->submitOrder(
        trader, pair,
        static_cast<Side>(side),
        static_cast<OrderType>(type),
        price, quantity,
        static_cast<TimeInForce>(tif),
        stop_price, expires_at
    );
}

uint8_t cancel_order(uint64_t order_id, const char* trader) {
    if (g_order_book == nullptr) return 0;
    return g_order_book->cancelOrder(order_id, trader) ? 1 : 0;
}

uint64_t modify_order(uint64_t order_id, const char* trader, 
                      uint64_t new_price, uint64_t new_quantity) {
    if (g_order_book == nullptr) return 0;
    return g_order_book->modifyOrder(order_id, trader, new_price, new_quantity);
}

uint64_t get_best_bid() {
    if (g_order_book == nullptr) return 0;
    auto bid = g_order_book->getBestBid();
    return bid.value_or(0);
}

uint64_t get_best_ask() {
    if (g_order_book == nullptr) return 0;
    auto ask = g_order_book->getBestAsk();
    return ask.value_or(0);
}

double get_spread() {
    if (g_order_book == nullptr) return 0;
    return g_order_book->getSpread();
}

} // extern "C"

#endif // TIGERSWAP_ORDER_BOOK_HPP
