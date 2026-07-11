/**
 * TigerSwap DEX Aggregator - Production Routing Engine
 * Smart routing with MEV protection and gas optimization
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

#ifndef TIGERSWAP_ROUTING_ENGINE_HPP
#define TIGERSWAP_ROUTING_ENGINE_HPP

#include <algorithm>
#include <array>
#include <cstdint>
#include <limits>
#include <queue>
#include <stack>
#include <unordered_map>
#include <unordered_set>
#include <vector>
#include <string>
#include <optional>
#include <functional>
#include <memory>

namespace tigerswap {

// ============================================================================
// Constants
// ============================================================================

constexpr uint64_t MAX_HOPS = 4;
constexpr uint64_t MAX_ROUTES = 10;
constexpr uint32_t MAX_POOLS_PER_TOKEN = 100;
constexpr double DEFAULT_SLIPPAGE = 0.005;
constexpr double MAX_SLIPPAGE = 0.50;
constexpr uint64_t CACHE_TTL_MS = 5000;

// ============================================================================
// Types
// ============================================================================

enum class DEXType : uint8_t {
    UNISWAP_V2 = 0,
    UNISWAP_V3 = 1,
    SUSHI_SWAP = 2,
    CURVE = 3,
    BALANCER = 4,
    PANCAKE_SWAP = 5,
    QUICK_SWAP = 6,
    AERODROME = 7,
    VELODROME = 8,
    CAMELOT = 9,
    RAYDIUM = 10,
    ORCA = 11,
    MAVERICK = 12,
    KYBER = 13,
    DODO = 14,
    WOOFi = 15,
    STARGATE = 16,
    LAYERZERO = 17
};

enum class TradeType : uint8_t {
    EXACT_INPUT = 0,
    EXACT_OUTPUT = 1
};

enum class RouteType : uint8_t {
    SINGLE = 0,
    MULTI_HOP = 1,
    SPLIT = 2,
    BRIDGE = 3
};

struct Token {
    std::string address;
    std::string symbol;
    std::string name;
    uint8_t decimals;
    uint64_t chain_id;
    
    bool operator==(const Token& other) const {
        return address == other.address && chain_id == other.chain_id;
    }
};

struct Pool {
    std::string pool_address;
    DEXType dex_type;
    Token token_a;
    Token token_b;
    uint64_t reserve_a;
    uint64_t reserve_b;
    uint64_t liquidity_usd;
    uint32_t fee_bps;
    uint64_t last_updated;
    bool is_stable;
    double apr;
};

struct SwapRoute {
    std::vector<Pool> pools;
    std::vector<Token> path;
    uint64_t input_amount;
    uint64_t output_amount;
    double price_impact;
    uint64_t gas_estimate;
    RouteType route_type;
    DEXType primary_dex;
};

struct QuoteRequest {
    Token token_in;
    Token token_out;
    uint64_t amount_in;
    double slippage;
    TradeType trade_type;
    std::vector<DEXType> included_dexes;
    std::vector<DEXType> excluded_dexes;
    bool enable_mev_protection;
    bool enable_gas_optimization;
    uint64_t deadline;
};

struct QuoteResult {
    SwapRoute route;
    uint64_t amount_out;
    uint64_t amount_in;
    double price_impact;
    uint64_t gas_estimate;
    double gas_fee_usd;
    double total_fee_usd;
    double execution_time_ms;
    bool mev_protected;
    std::string tx_data;
};

struct SplitRoute {
    std::vector<SwapRoute> routes;
    std::vector<double> percentages;
    uint64_t total_input;
    uint64_t total_output;
    double improvement_bps;
};

struct PriceData {
    Token token_a;
    Token token_b;
    double price;
    double price_change_24h;
    uint64_t volume_24h;
    uint64_t liquidity_usd;
    uint64_t updated_at;
    std::vector<DEXType> sources;
};

// ============================================================================
// Routing Engine
// ============================================================================

class RoutingEngine {
public:
    RoutingEngine(uint64_t chain_id) : chain_id_(chain_id) {}
    ~RoutingEngine() = default;
    
    void add_pool(const Pool& pool);
    void remove_pool(const std::string& pool_address);
    void update_pool(const Pool& pool);
    std::vector<Pool> get_pools(const Token& token_a, const Token& token_b);
    
    std::optional<QuoteResult> get_quote(const QuoteRequest& request);
    std::optional<SplitRoute> get_split_quote(const QuoteRequest& request);
    
    std::vector<SwapRoute> find_routes(const Token& token_in, const Token& token_out, 
                                        uint64_t amount_in, uint64_t max_hops);
    std::vector<SwapRoute> find_best_routes(const QuoteRequest& request);
    
    PriceData get_price(const Token& token_a, const Token& token_b);
    
    void clear_cache();
    void set_cache_ttl(uint64_t ttl_ms);
    void set_max_hops(uint64_t hops);
    void set_max_routes(uint64_t routes);
    void set_default_slippage(double slippage);
    
private:
    uint64_t chain_id_;
    uint64_t max_hops_ = MAX_HOPS;
    uint64_t max_routes_ = MAX_ROUTES;
    double default_slippage_ = DEFAULT_SLIPPAGE;
    uint64_t cache_ttl_ = CACHE_TTL_MS;
    
    std::unordered_map<std::string, std::vector<Pool>> pools_;
    std::unordered_map<std::string, PriceData> price_cache_;
    std::unordered_map<std::string, QuoteResult> quote_cache_;
    
    std::string get_pool_key(const Token& a, const Token& b) const;
    std::string get_price_key(const Token& a, const Token& b) const;
    
    std::vector<SwapRoute> dijkstra_route(const Token& start, const Token& end, 
                                          uint64_t amount_in);
    
    uint64_t calculate_amount_out(uint64_t amount_in, const Pool& pool);
    double calculate_price_impact(const SwapRoute& route);
    uint64_t estimate_gas_for_route(const SwapRoute& route);
    
    SplitRoute calculate_split_routes(const std::vector<SwapRoute>& routes, 
                                      uint64_t total_amount);
    
    bool is_valid_pool(const Pool& pool) const;
    bool is_valid_route(const SwapRoute& route) const;
};

// ============================================================================
// Price Aggregator
// ============================================================================

class PriceAggregator {
public:
    PriceAggregator() = default;
    ~PriceAggregator() = default;
    
    void add_source(DEXType dex, double weight);
    void remove_source(DEXType dex);
    double get_aggregated_price(const Token& a, const Token& b);
    void update_price(DEXType dex, const Token& a, const Token& b, double price);
    double calculate_impact(const Token& a, const Token& b, uint64_t amount);
    
private:
    struct PriceSource {
        DEXType dex;
        double weight;
        bool active;
    };
    
    std::vector<PriceSource> sources_;
    std::unordered_map<std::string, std::unordered_map<DEXType, double>> prices_;
    
    std::string get_key(const Token& a, const Token& b) const;
};

// ============================================================================
// Gas Optimizer
// ============================================================================

class GasOptimizer {
public:
    GasOptimizer() = default;
    ~GasOptimizer() = default;
    
    uint64_t estimate_swap_gas(DEXType dex, bool is_multi_hop);
    uint64_t estimate_approve_gas();
    
    struct GasSettings {
        uint64_t gas_limit;
        uint64_t max_fee_per_gas;
        uint64_t max_priority_fee_per_gas;
        bool use_eip1559;
    };
    
    GasSettings get_eip1559_settings();
    double calculate_total_fee(uint64_t gas_limit, uint64_t gas_price);
    
private:
    uint64_t avg_gas_price_ = 20000000000;
};

// ============================================================================
// MEV Protection Engine
// ============================================================================

class MEVProtectionEngine {
public:
    MEVProtectionEngine() = default;
    ~MEVProtectionEngine() = default;
    
    bool is_mev_opportunity(const SwapRoute& route, uint64_t amount);
    double estimate_mev_extractable(const SwapRoute& route);
    SwapRoute apply_protection(const SwapRoute& route);
    bool detect_sandwich_risk(const SwapRoute& route);
    SwapRoute mitigate_sandwich_risk(const SwapRoute& route);
    uint64_t get_protected_gas_price(uint64_t base_price);
    
private:
    struct MEVAnalysis {
        bool sandwich_risk;
        bool front_run_risk;
        double estimated_mev;
    };
    
    MEVAnalysis analyze_route(const SwapRoute& route);
};

// ============================================================================
// Swap Executor
// ============================================================================

class SwapExecutor {
public:
    SwapExecutor(RoutingEngine* routing_engine, GasOptimizer* gas_optimizer);
    ~SwapExecutor() = default;
    
    struct Transaction {
        std::string from;
        std::string to;
        std::string data;
        uint64_t value;
        uint64_t gas_limit;
        uint64_t gas_price;
        uint64_t max_fee_per_gas;
        uint64_t max_priority_fee_per_gas;
        uint64_t nonce;
        uint64_t chain_id;
    };
    
    Transaction build_transaction(const QuoteResult& quote, const std::string& from, 
                                 const std::string& to, uint64_t nonce);
    std::string encode_swap_data(const QuoteResult& quote);
    
private:
    RoutingEngine* routing_engine_;
    GasOptimizer* gas_optimizer_;
};

// ============================================================================
// Inline Implementations
// ============================================================================

inline std::string RoutingEngine::get_pool_key(const Token& a, const Token& b) const {
    if (a.address < b.address) {
        return a.address + "_" + b.address;
    }
    return b.address + "_" + a.address;
}

inline std::string RoutingEngine::get_price_key(const Token& a, const Token& b) const {
    return a.address + "_" + b.address + "_" + std::to_string(a.chain_id);
}

inline uint64_t RoutingEngine::calculate_amount_out(uint64_t amount_in, const Pool& pool) {
    if (amount_in == 0 || pool.reserve_a == 0 || pool.reserve_b == 0) {
        return 0;
    }
    
    uint128_t amount_in_with_fee = static_cast<uint128_t>(amount_in) * (10000 - pool.fee_bps);
    uint128_t numerator = amount_in_with_fee * pool.reserve_b;
    uint128_t denominator = static_cast<uint128_t>(pool.reserve_a) * 10000 + amount_in_with_fee;
    
    return static_cast<uint64_t>(numerator / denominator);
}

inline double RoutingEngine::calculate_price_impact(const SwapRoute& route) {
    if (route.input_amount == 0 || route.output_amount == 0) {
        return 0.0;
    }
    
    double spot_price = 1.0;
    if (!route.pools.empty()) {
        const auto& pool = route.pools[0];
        if (pool.reserve_a > 0 && pool.reserve_b > 0) {
            spot_price = static_cast<double>(pool.reserve_b) / pool.reserve_a;
        }
    }
    
    double execution_price = static_cast<double>(route.output_amount) / route.input_amount;
    double impact = (spot_price - execution_price) / spot_price * 10000;
    
    return impact;
}

inline uint64_t RoutingEngine::estimate_gas_for_route(const SwapRoute& route) {
    uint64_t base_gas = 21000;
    uint64_t hop_gas = 50000;
    base_gas += route.pools.size() * hop_gas;
    
    double multiplier = 1.0;
    if (route.route_type == RouteType::SPLIT) multiplier = 1.2;
    else if (route.route_type == RouteType::BRIDGE) multiplier = 1.5;
    
    return static_cast<uint64_t>(base_gas * multiplier);
}

} // namespace tigerswap

#endif
