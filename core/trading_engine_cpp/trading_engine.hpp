#pragma once

#include <cstdint>
#include <string>
#include <vector>
#include <memory>
#include <functional>
#include <optional>
#include <chrono>
#include <unordered_map>
#include <array>

namespace tigerswap {

// Forward declarations
class PriceEngine;
class LiquidityScanner;
class RoutingEngine;
class OrderExecutor;

// ============ Constants ============
constexpr uint64_t kMinOrderSize = 1e6;          // 0.000001 token units
constexpr uint64_t kMaxSlippageBPS = 5000;       // 50% max slippage
constexpr uint64_t kPricePrecision = 1e8;          // 1e8 precision for prices
constexpr uint64_t kQuoteTimeoutMs = 50;           // 50ms quote timeout
constexpr uint64_t kOrderBookDepth = 100;          // Order book depth

// ============ Enums ============
enum class OrderType : uint8_t {
    kLimit = 0,
    kStopLoss = 1,
    kTakeProfit = 2,
    kStopLossLimit = 3,  // OCO
    kGTD = 4,            // Good Till Date
    kIOC = 5,            // Immediate or Cancel
    kFOK = 6             // Fill or Kill
};

enum class OrderStatus : uint8_t {
    kPending = 0,
    kFilled = 1,
    kCancelled = 2,
    kExpired = 3,
    kPartialFill = 4
};

enum class Side : uint8_t {
    kBuy = 0,
    kSell = 1
};

enum class Chain : uint16_t {
    kEthereum = 1,
    kBNBChain = 56,
    kPolygon = 137,
    kArbitrum = 42161,
    kOptimism = 10,
    kBase = 8453,
    kAvalanche = 43114,
    kSolana = 101,
    kSui = 784,
    kAptos = 0,
    kInjective = 4102
};

// ============ Types ============
using Timestamp = std::chrono::milliseconds;
using OrderId = uint64_t;
using AccountId = uint64_t;
using PoolId = uint64_t;

// ============ Structures ============
struct Token {
    std::string address;
    std::string symbol;
    std::string name;
    uint8_t decimals;
    Chain chain;
};

struct TokenPair {
    Token token_in;
    Token token_out;
    Chain chain;
    
    std::string to_string() const {
        return token_in.symbol + "/" + token_out.symbol;
    }
    
    bool operator==(const TokenPair& other) const {
        return token_in.address == other.token_in.address && 
               token_out.address == other.token_out.address &&
               chain == other.chain;
    }
};

struct Order {
    OrderId id;
    AccountId owner;
    TokenPair pair;
    uint64_t amount_in;
    uint64_t amount_out_min;
    uint64_t price;
    uint64_t stop_price;
    uint64_t executed_amount_in;
    uint64_t executed_amount_out;
    OrderType order_type;
    OrderStatus status;
    Side side;
    Timestamp created_at;
    Timestamp expires_at;
    Timestamp updated_at;
    bool is_native;
};

struct Pool {
    PoolId id;
    TokenPair pair;
    std::string dex_name;
    uint64_t reserve0;
    uint64_t reserve1;
    uint64_t liquidity;
    uint32_t fee_tier;  // in basis points (e.g., 3000 = 0.3%)
    uint64_t tick_current;
    int tick_lower;
    int tick_upper;
};

struct Quote {
    TokenPair pair;
    uint64_t amount_in;
    uint64_t amount_out;
    uint64_t price;
    uint64_t gas_used;
    std::string dex_name;
    std::vector<std::string> path;
    Timestamp timestamp;
    
    bool operator<(const Quote& other) const {
        return amount_out < other.amount_out;
    }
};

struct Route {
    std::vector<std::string> path;
    uint64_t amount_out;
    uint64_t gas_used;
    uint64_t input_amount;
    std::vector<Quote> quotes;
};

struct PriceTick {
    TokenPair pair;
    uint64_t bid;    // Best bid price
    uint64_t ask;    // Best ask price
    uint64_t mid;    // Mid price
    uint64_t volume_24h;
    Timestamp timestamp;
};

struct TickInfo {
    int index;
    int64_t liquidity_net;
    uint64_t liquidity_gross;
    uint64_t fee_growth0;
    uint64_t fee_growth1;
};

// ============ Order Book ============
class OrderBook {
public:
    OrderBook() = default;
    
    void add_order(const Order& order);
    void remove_order(OrderId order_id);
    void update_order(const Order& order);
    
    std::vector<Order> get_orders(Side side, uint64_t limit) const;
    std::optional<Order> get_order(OrderId order_id) const;
    std::vector<Order> get_orders_by_price(Side side, uint64_t price, uint64_t limit) const;
    
    std::optional<uint64_t> get_best_price(Side side) const;
    uint64_t get_total_liquidity(Side side, uint64_t price) const;
    
    void expire_orders(Timestamp now);
    void process_stop_orders(const PriceTick& price);
    
private:
    std::unordered_map<OrderId, Order> orders_;
    std::unordered_map<std::string, std::vector<OrderId>> price_index_;
    std::array<std::vector<OrderId>, 2> best_prices_;
};

// ============ Price Engine (C++ for sub-millisecond pricing) ============
class PriceEngine {
public:
    PriceEngine();
    ~PriceEngine();
    
    // Get price with sub-millisecond latency
    std::optional<PriceTick> get_price(const TokenPair& pair);
    
    // Get prices for multiple pairs
    std::vector<PriceTick> get_prices(const std::vector<TokenPair>& pairs);
    
    // Update price from oracle
    void update_price(const TokenPair& pair, uint64_t bid, uint64_t ask);
    
    // Calculate price impact
    uint64_t calculate_price_impact(const TokenPair& pair, uint64_t amount_in);
    
    // TWAP calculation
    uint64_t calculate_twap(const TokenPair& pair, uint32_t window_seconds);
    
    // Volatility calculation
    double calculate_volatility(const TokenPair& pair, uint32_t window_seconds);
    
private:
    std::unordered_map<std::string, PriceTick> prices_;
    std::unordered_map<std::string, std::vector<PriceTick>> price_history_;
};

// ============ Liquidity Scanner (C++ for real-time liquidity) ============
class LiquidityScanner {
public:
    LiquidityScanner();
    ~LiquidityScanner();
    
    // Scan all DEX pools for a token pair
    std::vector<Pool> scan_pools(const TokenPair& pair);
    
    // Get pool by ID
    std::optional<Pool> get_pool(PoolId pool_id);
    
    // Get aggregated liquidity
    uint64_t get_total_liquidity(const TokenPair& pair);
    
    // Get liquidity by price range (for concentrated liquidity)
    uint64_t get_liquidity_in_range(const TokenPair& pair, int tick_lower, int tick_upper);
    
    // Get tick info
    std::optional<TickInfo> get_tick(const TokenPair& pair, int tick_index);
    
    // Update pool data
    void update_pool(const Pool& pool);
    
private:
    std::unordered_map<PoolId, Pool> pools_;
    std::unordered_map<std::string, std::vector<PoolId>> pools_by_pair_;
};

// ============ Routing Engine (C++ for pathfinding) ============
class RoutingEngine {
public:
    RoutingEngine(
        std::shared_ptr<PriceEngine> price_engine,
        std::shared_ptr<LiquidityScanner> liquidity_scanner
    );
    ~RoutingEngine();
    
    // Get best single route
    std::optional<Route> get_route(
        const TokenPair& pair,
        uint64_t amount_in,
        uint64_t max_slippage_bps = 50
    );
    
    // Get multiple routes
    std::vector<Route> get_routes(
        const TokenPair& pair,
        uint64_t amount_in,
        uint32_t max_routes = 3
    );
    
    // Get split route (optimal split across DEXs)
    std::optional<Route> get_split_route(
        const TokenPair& pair,
        uint64_t amount_in,
        uint32_t max_splits = 5
    );
    
    // Multi-hop routing
    std::optional<Route> get_multihop_route(
        const std::vector<TokenPair>& path,
        uint64_t amount_in
    );
    
    // Cross-chain routing
    std::optional<Route> get_crosschain_route(
        const TokenPair& source_pair,
        const TokenPair& dest_pair,
        uint64_t amount_in
    );
    
    // Add DEX connector
    void add_dex(const std::string& dex_name, void* connector);
    
private:
    struct RouteNode {
        TokenPair pair;
        uint64_t amount_out;
        uint64_t gas_used;
        std::string dex_name;
        std::vector<RouteNode> next;
    };
    
    std::optional<Route> dijkstra(
        const TokenPair& pair,
        uint64_t amount_in,
        uint32_t max_hops
    );
    
    std::shared_ptr<PriceEngine> price_engine_;
    std::shared_ptr<LiquidityScanner> liquidity_scanner_;
    std::unordered_map<std::string, void*> dex_connectors_;
};

// ============ Order Executor (C++ for execution) ============
class OrderExecutor {
public:
    OrderExecutor(
        std::shared_ptr<RoutingEngine> routing_engine,
        std::shared_ptr<OrderBook> order_book
    );
    ~OrderExecutor();
    
    // Execute a single order
    std::string execute_order(
        const Order& order,
        const std::string& private_key
    );
    
    // Execute multiple orders (batch)
    std::vector<std::string> execute_orders(
        const std::vector<Order>& orders,
        const std::string& private_key
    );
    
    // Fill order book orders
    uint64_t fill_order_book(
        const TokenPair& pair,
        uint64_t amount_in,
        const std::string& private_key
    );
    
    // Set executor address
    void set_executor(const std::string& address);
    
    // Get pending transactions
    std::vector<std::string> get_pending_txs() const;
    
private:
    std::string sign_and_send(
        const Order& order,
        uint64_t amount_in,
        const std::string& private_key
    );
    
    std::shared_ptr<RoutingEngine> routing_engine_;
    std::shared_ptr<OrderBook> order_book_;
    std::string executor_address_;
    std::vector<std::string> pending_txs_;
};

// ============ Trading Engine (Main C++ Class) ============
class TradingEngine {
public:
    TradingEngine();
    ~TradingEngine();
    
    // Initialize with configuration
    bool initialize(const std::string& config_path);
    
    // Start the engine
    void start();
    
    // Stop the engine
    void stop();
    
    // Create order
    OrderId create_order(const Order& order);
    
    // Cancel order
    bool cancel_order(OrderId order_id);
    
    // Get order
    std::optional<Order> get_order(OrderId order_id) const;
    
    // Get pending orders
    std::vector<Order> get_pending_orders() const;
    
    // Get order book
    std::vector<Order> get_order_book(
        const TokenPair& pair,
        Side side,
        uint64_t limit
    ) const;
    
    // Get quote
    std::optional<Quote> get_quote(
        const TokenPair& pair,
        uint64_t amount_in
    );
    
    // Get route
    std::optional<Route> get_route(
        const TokenPair& pair,
        uint64_t amount_in
    );
    
    // Execute swap
    std::string execute_swap(
        const TokenPair& pair,
        uint64_t amount_in,
        uint64_t amount_out_min,
        const std::string& recipient,
        const std::string& private_key
    );
    
    // Execute DCA
    std::string execute_dca(
        const std::string& plan_id,
        const std::string& private_key
    );
    
    // Get statistics
    struct Stats {
        uint64_t total_orders;
        uint64_t filled_orders;
        uint64_t total_volume;
        uint64_t total_fees;
    };
    
    Stats get_stats() const;
    
private:
    std::shared_ptr<PriceEngine> price_engine_;
    std::shared_ptr<LiquidityScanner> liquidity_scanner_;
    std::shared_ptr<RoutingEngine> routing_engine_;
    std::shared_ptr<OrderBook> order_book_;
    std::shared_ptr<OrderExecutor> order_executor_;
    
    OrderId next_order_id_;
    Stats stats_;
    bool running_;
};

// ============ Inline Implementations ============

inline void OrderBook::add_order(const Order& order) {
    orders_[order.id] = order;
    
    std::string key = order.pair.to_string() + std::to_string(order.price);
    price_index_[key].push_back(order.id);
    
    if (order.side == Side::kBuy) {
        if (best_prices_[0].empty() || order.price < best_prices_[0][0]) {
            best_prices_[0] = {order.id};
        } else {
            best_prices_[0].push_back(order.id);
        }
    } else {
        if (best_prices_[1].empty() || order.price > best_prices_[1][0]) {
            best_prices_[1] = {order.id};
        } else {
            best_prices_[1].push_back(order.id);
        }
    }
}

inline void OrderBook::remove_order(OrderId order_id) {
    auto it = orders_.find(order_id);
    if (it != orders_.end()) {
        const Order& order = it->second;
        std::string key = order.pair.to_string() + std::to_string(order.price);
        auto& list = price_index_[key];
        list.erase(std::remove(list.begin(), list.end(), order_id), list.end());
        orders_.erase(it);
    }
}

inline void OrderBook::update_order(const Order& order) {
    orders_[order.id] = order;
}

inline std::vector<Order> OrderBook::get_orders(Side side, uint64_t limit) const {
    std::vector<Order> result;
    for (const auto& [id, order] : orders_) {
        if (order.side == side && order.status == OrderStatus::kPending) {
            result.push_back(order);
            if (result.size() >= limit) break;
        }
    }
    return result;
}

inline std::optional<Order> OrderBook::get_order(OrderId order_id) const {
    auto it = orders_.find(order_id);
    if (it != orders_.end()) {
        return it->second;
    }
    return std::nullopt;
}

inline std::optional<uint64_t> OrderBook::get_best_price(Side side) const {
    const auto& ids = best_prices_[static_cast<uint8_t>(side)];
    if (ids.empty()) return std::nullopt;
    
    auto it = orders_.find(ids[0]);
    if (it != orders_.end()) {
        return it->second.price;
    }
    return std::nullopt;
}

inline void OrderBook::expire_orders(Timestamp now) {
    for (auto& [id, order] : orders_) {
        if (order.status == OrderStatus::kPending && order.expires_at <= now) {
            order.status = OrderStatus::kExpired;
        }
    }
}

inline void OrderBook::process_stop_orders(const PriceTick& price) {
    for (auto& [id, order] : orders_) {
        if (order.status != OrderStatus::kPending) continue;
        if (order.order_type != OrderType::kStopLoss && 
            order.order_type != OrderType::kTakeProfit) continue;
        
        bool should_trigger = false;
        
        if (order.side == Side::kBuy) {
            if (order.order_type == OrderType::kStopLoss && price.bid <= order.stop_price) {
                should_trigger = true;
            } else if (order.order_type == OrderType::kTakeProfit && price.ask >= order.stop_price) {
                should_trigger = true;
            }
        } else {
            if (order.order_type == OrderType::kStopLoss && price.ask >= order.stop_price) {
                should_trigger = true;
            } else if (order.order_type == OrderType::kTakeProfit && price.bid <= order.stop_price) {
                should_trigger = true;
            }
        }
        
        if (should_trigger) {
            order.status = OrderStatus::kFilled;
        }
    }
}

// ============ Price Engine Inline Methods ============

inline PriceEngine::PriceEngine() = default;
inline PriceEngine::~PriceEngine() = default;

inline std::optional<PriceTick> PriceEngine::get_price(const TokenPair& pair) {
    auto it = prices_.find(pair.to_string());
    if (it != prices_.end()) {
        return it->second;
    }
    return std::nullopt;
}

inline std::vector<PriceTick> PriceEngine::get_prices(const std::vector<TokenPair>& pairs) {
    std::vector<PriceTick> result;
    result.reserve(pairs.size());
    
    for (const auto& pair : pairs) {
        if (auto price = get_price(pair)) {
            result.push_back(*price);
        }
    }
    
    return result;
}

inline void PriceEngine::update_price(const TokenPair& pair, uint64_t bid, uint64_t ask) {
    PriceTick tick;
    tick.pair = pair;
    tick.bid = bid;
    tick.ask = ask;
    tick.mid = (bid + ask) / 2;
    tick.timestamp = std::chrono::duration_cast<Timestamp>(
        std::chrono::system_clock::now().time_since_epoch()
    );
    
    prices_[pair.to_string()] = tick;
    price_history_[pair.to_string()].push_back(tick);
}

inline uint64_t PriceEngine::calculate_price_impact(const TokenPair& pair, uint64_t amount_in) {
    auto price = get_price(pair);
    if (!price) return 0;
    
    // Simplified price impact calculation
    // In production, use more sophisticated model
    double impact = static_cast<double>(amount_in) / 
                  static_cast<double>(price->volume_24h);
    return static_cast<uint64_t>(impact * kPricePrecision);
}

// ============ Liquidity Scanner Inline Methods ============

inline LiquidityScanner::LiquidityScanner() = default;
inline LiquidityScanner::~LiquidityScanner() = default;

inline std::vector<Pool> LiquidityScanner::scan_pools(const TokenPair& pair) {
    std::vector<Pool> result;
    auto it = pools_by_pair_.find(pair.to_string());
    
    if (it != pools_by_pair_.end()) {
        for (PoolId pool_id : it->second) {
            auto pool_it = pools_.find(pool_id);
            if (pool_it != pools_.end()) {
                result.push_back(pool_it->second);
            }
        }
    }
    
    return result;
}

inline std::optional<Pool> LiquidityScanner::get_pool(PoolId pool_id) {
    auto it = pools_.find(pool_id);
    if (it != pools_.end()) {
        return it->second;
    }
    return std::nullopt;
}

inline uint64_t LiquidityScanner::get_total_liquidity(const TokenPair& pair) {
    uint64_t total = 0;
    auto pools = scan_pools(pair);
    
    for (const auto& pool : pools) {
        total += pool.liquidity;
    }
    
    return total;
}

inline void LiquidityScanner::update_pool(const Pool& pool) {
    pools_[pool.id] = pool;
    pools_by_pair_[pool.pair.to_string()].push_back(pool.id);
}

} // namespace tigerswap