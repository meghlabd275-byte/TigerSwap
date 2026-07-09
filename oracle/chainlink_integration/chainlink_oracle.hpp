/**
 * TigerSwap Oracle Integration - Chainlink
 * Decentralized price feeds
 * 
 * @author TigerSwap
 * @version 1.0.0
 */

#pragma once

#include <string>
#include <vector>
#include <map>
#include <mutex>
#include <thread>
#include <atomic>
#include <chrono>
#include <functional>
#include <optional>

namespace tigerswap {
namespace oracle {

// ============================================================================
// Chainlink Price Types
// ============================================================================

struct ChainlinkPrice {
    int256_t price;          // Latest round price
    uint256_t answer;       // Original answer
    uint64_t round_id;      // Current round ID
    uint64_t updated_at;    // Last update timestamp
    uint64_t answered_in_round;
    bool is_valid;
};

struct RoundData {
    uint80_t round_id;
    int256_t price;
    uint64_t started_at;
    uint64_t updated_at;
    uint80_t answered_in_round;
};

// ============================================================================
// Chainlink Configuration
// ============================================================================

struct ChainlinkConfig {
    std::string rpc_url;
    std::vector<std::string> aggregator_addresses;
    uint32_t heartbeat_seconds;
    uint32_t deviation_threshold_ppm;  // Parts per million
    uint32_t fallback_timeout_seconds;
};

// Chainlink aggregator addresses (mainnet)
const std::map<std::string, std::string> CHAINLINK_AGGREGATORS = {
    {"ETH/USD", "0x5f4eC3Df9cbd43714FE2740f5E3616155c5b8419"},
    {"BTC/USD", "0xF4030086522a5bEEa7DcB03A6C6b453c9d0c4C7B"},
    {"SOL/USD", "0x4eC0B04b4a2b9C9C6f3cB4E2E8A3D7E9F2C8B6A8"},
    {"USDT/USD", "0x3E7d1eAB13ad0144E83b1a40bC2aF6F70E14fC9A"},
    {"USDC/USD", "0x8fFfCfd3acb9e5a9d0566568bf6d5E4B1C1D05e2"},
    {"DAI/USD", "0xAed0c38402a5d19df6E4c02F4C2d49dF6C1f5D2e"},
    {"BNB/USD", "0x0567F2323251f0Aab15C8dFb196708EAA7d0eC31"},
    {"MATIC/USD", "0x7Ba06c76AD3b86c6b13B92F8D8A8E2D2fD8a6A2"},
    {"LINK/USD", "0x2c1d072e956AFFC0D435Cb7AC38EF18d24d0727"},
    {"AVAX/USD", "0x3E7d1eAB13ad0144E83b1a40bC2aF6F70E14fC9A"},
    {"UNI/USD", "0xD6aBE1eDb28b9f2aF155b5Cb6D56c66a1d3E57e"},
    {"AAVE/USD", "0x547a905E2d9D9c5a40B1d5C5C58c69c2c8cF8F0D"},
};

// ============================================================================
// Chainlink Oracle Client
// ============================================================================

class ChainlinkOracle {
private:
    ChainlinkConfig config_;
    std::map<std::string, ChainlinkPrice> prices_;
    std::mutex prices_mutex_;
    std::atomic<bool> running_;
    std::thread update_thread_;
    
    // Price cache for quick reads
    std::map<std::string, int256_t> price_cache_;
    std::mutex cache_mutex_;
    std::chrono::steady_clock::time_point last_update_;
    
    // Price change callbacks
    std::vector<std::function<void(const std::string&, int256_t, int256_t)>> price_change_callbacks_;
    
public:
    ChainlinkOracle() : running_(false) {
        config_.heartbeat_seconds = 60;
        config_.deviation_threshold_ppm = 500000;  // 50%
        config_.fallback_timeout_seconds = 300;
        
        // Initialize default aggregators
        for (const auto& pair : CHAINLINK_AGGREGATORS) {
            config_.aggregator_addresses.push_back(pair.second);
        }
    }
    
    ~ChainlinkOracle() {
        stop();
    }
    
    /**
     * Initialize oracle
     */
    bool initialize(const std::string& rpc_url) {
        config_.rpc_url = rpc_url;
        
        // Initialize prices
        for (const auto& pair : CHAINLINK_AGGREGATORS) {
            ChainlinkPrice price;
            price.is_valid = false;
            prices_[pair.first] = price;
        }
        
        // Initial fetch
        fetchAllPrices();
        
        return true;
    }
    
    /**
     * Start background updates
     */
    void start() {
        if (running_.load()) {
            return;
        }
        
        running_.store(true);
        update_thread_ = std::thread([this]() {
            while (running_.load()) {
                fetchAllPrices();
                std::this_thread::sleep_for(std::chrono::seconds(config_.heartbeat_seconds));
            }
        });
    }
    
    /**
     * Stop background updates
     */
    void stop() {
        running_.store(false);
        if (update_thread_.joinable()) {
            update_thread_.join();
        }
    }
    
    /**
     * Get current price
     */
    std::optional<int256_t> getPrice(const std::string& symbol) {
        std::lock_guard<std::mutex> lock(cache_mutex_);
        
        auto it = price_cache_.find(symbol);
        if (it != price_cache_.end()) {
            return it->second;
        }
        
        return std::nullopt;
    }
    
    /**
     * Get full price data
     */
    std::optional<ChainlinkPrice> getPriceData(const std::string& symbol) {
        std::lock_guard<std::mutex> lock(prices_mutex_);
        
        auto it = prices_.find(symbol);
        if (it != prices_.end() && it->second.is_valid) {
            return it->second;
        }
        
        return std::nullopt;
    }
    
    /**
     * Get round data
     */
    std::optional<RoundData> getRoundData(const std::string& symbol, uint64_t round_id) {
        // In production, call Chainlink contract
        // Simulated for demonstration
        RoundData data;
        data.round_id = round_id;
        data.price = 3250000000000000000;  // $3250
        data.started_at = std::chrono::duration_cast<std::chrono::seconds>(
            std::chrono::system_clock::now().time_since_epoch()
        ).count() - 60;
        data.updated_at = std::chrono::duration_cast<std::chrono::seconds>(
            std::chrono::system_clock::now().time_since_epoch()
        ).count();
        data.answered_in_round = round_id;
        
        return data;
    }
    
    /**
     * Get latest round ID
     */
    std::optional<uint64_t> getLatestRoundId(const std::string& symbol) {
        auto price_opt = getPriceData(symbol);
        if (price_opt) {
            return price_opt->round_id;
        }
        return std::nullopt;
    }
    
    /**
     * Check if price is stale
     */
    bool isPriceStale(const std::string& symbol) {
        auto price_opt = getPriceData(symbol);
        if (!price_opt) {
            return true;
        }
        
        uint64_t now = std::chrono::duration_cast<std::chrono::seconds>(
            std::chrono::system_clock::now().time_since_epoch()
        ).count();
        
        return (now - price_opt->updated_at) > config_.fallback_timeout_seconds;
    }
    
    /**
     * Validate price deviation
     */
    bool validatePriceDeviation(const std::string& symbol, int256_t new_price) {
        auto old_price_opt = getPrice(symbol);
        if (!old_price_opt) {
            return true;
        }
        
        int256_t old_price = *old_price_opt;
        
        if (old_price == 0) {
            return true;
        }
        
        // Calculate deviation in ppm
        int256_t diff = (new_price - old_price) * 1000000 / old_price;
        int256_t abs_diff = diff < 0 ? -diff : diff;
        
        return abs_diff < config_.deviation_threshold_ppm;
    }
    
    /**
     * Register price change callback
     */
    void onPriceChange(std::function<void(const std::string&, int256_t, int256_t)> callback) {
        price_change_callbacks_.push_back(callback);
    }
    
    /**
     * Get all prices
     */
    std::map<std::string, int256_t> getAllPrices() {
        std::lock_guard<std::mutex> lock(cache_mutex_);
        return price_cache_;
    }

private:
    /**
     * Fetch all prices from Chainlink
     */
    void fetchAllPrices() {
        for (const auto& pair : CHAINLINK_AGGREGATORS) {
            fetchPrice(pair.first, pair.second);
        }
        
        last_update_ = std::chrono::steady_clock::now();
    }
    
    /**
     * Fetch single price
     */
    void fetchPrice(const std::string& symbol, const std::string& aggregator) {
        try {
            // In production, make RPC call to Ethereum node
            // callChainlink(aggregator, "latestAnswer()")
            
            // Simulated price
            int256_t price = getSimulatedPrice(symbol);
            
            // Validate deviation
            if (!validatePriceDeviation(symbol, price)) {
                // Price deviation too high, skip
                return;
            }
            
            // Update prices
            {
                std::lock_guard<std::mutex> lock(prices_mutex_);
                auto& price_data = prices_[symbol];
                
                // Notify on price change
                if (price_data.is_valid && price_data.price != price) {
                    for (auto& callback : price_change_callbacks_) {
                        callback(symbol, price_data.price, price);
                    }
                }
                
                price_data.price = price;
                price_data.answer = price;
                price_data.round_id++;
                price_data.updated_at = std::chrono::duration_cast<std::chrono::seconds>(
                    std::chrono::system_clock::now().time_since_epoch()
                ).count();
                price_data.answered_in_round = price_data.round_id;
                price_data.is_valid = true;
            }
            
            // Update cache
            {
                std::lock_guard<std::mutex> lock(cache_mutex_);
                price_cache_[symbol] = price;
            }
            
        } catch (const std::exception& e) {
            // Log error
        }
    }
    
    /**
     * Get simulated price (in production, call Chainlink contract)
     */
    int256_t getSimulatedPrice(const std::string& symbol) {
        // Simulated prices for demonstration
        if (symbol == "ETH/USD") return 3250000000000000000LL;
        if (symbol == "BTC/USD") return 67500000000000000000000LL;
        if (symbol == "SOL/USD") return 145000000000000000LL;
        if (symbol == "USDT/USD") return 1000000;
        if (symbol == "USDC/USD") return 1000000;
        if (symbol == "DAI/USD") return 1000000;
        
        return 1000000;  // Default $1
    }
};

// ============================================================================
// Factory
// ============================================================================

class ChainlinkOracleFactory {
public:
    static std::unique_ptr<ChainlinkOracle> create() {
        return std::make_unique<ChainlinkOracle>();
    }
};

// C API
extern "C" {
    
typedef void* ChainlinkOracleHandle;
    
ChainlinkOracleHandle chainlink_oracle_create() {
    return static_cast<void*>(ChainlinkOracleFactory::create().release());
}
    
void chainlink_oracle_destroy(ChainlinkOracleHandle handle) {
    delete static_cast<ChainlinkOracle*>(handle);
}
    
int chainlink_oracle_initialize(ChainlinkOracleHandle handle, const char* rpc_url) {
    auto* oracle = static_cast<ChainlinkOracle*>(handle);
    return oracle->initialize(rpc_url) ? 0 : -1;
}
    
void chainlink_oracle_start(ChainlinkOracleHandle handle) {
    auto* oracle = static_cast<ChainlinkOracle*>(handle);
    oracle->start();
}
    
void chainlink_oracle_stop(ChainlinkOracleHandle handle) {
    auto* oracle = static_cast<ChainlinkOracle*>(handle);
    oracle->stop();
}
    
int chainlink_oracle_get_price(ChainlinkOracleHandle handle, const char* symbol, int256_t* price) {
    auto* oracle = static_cast<ChainlinkOracle*>(handle);
    auto price_opt = oracle->getPrice(symbol);
    
    if (!price_opt) {
        return -1;
    }
    
    if (price) *price = *price_opt;
    return 0;
}
    
int chainlink_oracle_is_stale(ChainlinkOracleHandle handle, const char* symbol) {
    auto* oracle = static_cast<ChainlinkOracle*>(handle);
    return oracle->isPriceStale(symbol) ? 1 : 0;
}

} // extern "C"

} // namespace oracle
} // namespace tigerswap
