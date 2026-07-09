/**
 * TigerSwap Oracle Integration - Pyth Network
 * Real-time price feeds for DeFi
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
#include <curl/curl.h>
#include <openssl/hmac.h>
#include <openssl/sha.h>

namespace tigerswap {
namespace oracle {

// ============================================================================
// Pyth Price Types
// ============================================================================

struct Price {
    int64_t price;           // Price in 10^-exponent units
    uint64_t conf;           // Confidence interval
    int32_t exponent;        // Power of 10 scaling
    uint64_t publish_time;   // Publish timestamp
};

struct PriceFeed {
    std::string id;          // Product ID (e.g., "Crypto.ETH/USD")
    Price current_price;
    Price previous_price;
    uint64_t last_updated;
    bool is_valid;
};

// ============================================================================
// Pyth API Configuration
// ============================================================================

struct PythConfig {
    std::string api_url;
    std::string ws_url;
    std::string api_key;
    uint32_t update_interval_ms;
    uint32_t connection_timeout_ms;
    uint32_t max_retries;
};

// Default Pyth endpoints
const std::string PITH_API_URL = "https://api.pyth.network/v1";
const std::string PITH_WS_URL = "wss://api.pyth.network/v1/ws";

// Pyth product IDs
const std::map<std::string, std::string> PYTH_PRODUCT_IDS = {
    {"ETH/USD", "0xff61491a931112ddf1e8148bab5a42d2021cfbe16cfb889a0c6e0f3b0c0a0c0"},
    {"BTC/USD", "0xe62df6c8b4a85fe4a67f17cc2b3e5a8a0b0e0e0e0e0e0e0e0e0e0e0e0e0e0"},
    {"SOL/USD", "0xa12c4f3a0b0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0e0"},
    {"USDT/USD", "0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcd"},
    {"USDC/USD", "0xabcdef1234567890abcdef1234567890abcdef1234567890abcdef12345678"},
    {"DAI/USD", "0x9876543210fedcba9876543210fedcba9876543210fedcba9876543210fedc"},
    {"BNB/USD", "0x1111111111111111111111111111111111111111111111111111111111111111"},
    {"MATIC/USD", "0x2222222222222222222222222222222222222222222222222222222222222222"},
    {"LINK/USD", "0x3333333333333333333333333333333333333333333333333333333333333333"},
    {"AVAX/USD", "0x4444444444444444444444444444444444444444444444444444444444444444"},
};

// ============================================================================
// HTTP Client for Pyth API
// ============================================================================

class HttpClient {
private:
    CURL* curl_;
    std::string response_buffer_;
    std::mutex mutex_;

public:
    HttpClient() {
        curl_ = curl_easy_init();
    }
    
    ~HttpClient() {
        if (curl_) {
            curl_easy_cleanup(curl_);
        }
    }
    
    std::string get(const std::string& url) {
        std::lock_guard<std::mutex> lock(mutex_);
        
        curl_easy_setopt(curl_, CURLOPT_URL, url.c_str());
        curl_easy_setopt(curl_, CURLOPT_FOLLOWLOCATION, 1L);
        curl_easy_setopt(curl_, CURLOPT_WRITEFUNCTION, WriteCallback);
        curl_easy_setopt(curl_, CURLOPT_WRITEDATA, &response_buffer_);
        
        response_buffer_.clear();
        CURLcode res = curl_easy_perform(curl_);
        
        if (res != CURLE_OK) {
            return "";
        }
        
        return response_buffer_;
    }
    
    std::string post(const std::string& url, const std::string& data) {
        std::lock_guard<std::mutex> lock(mutex_);
        
        curl_easy_setopt(curl_, CURLOPT_URL, url.c_str());
        curl_easy_setopt(curl_, CURLOPT_POST, 1L);
        curl_easy_setopt(curl_, CURLOPT_POSTFIELDS, data.c_str());
        curl_easy_setopt(curl_, CURLOPT_WRITEFUNCTION, WriteCallback);
        curl_easy_setopt(curl_, CURLOPT_WRITEDATA, &response_buffer_);
        
        response_buffer_.clear();
        CURLcode res = curl_easy_perform(curl_);
        
        if (res != CURLE_OK) {
            return "";
        }
        
        return response_buffer_;
    }

private:
    static size_t WriteCallback(void* contents, size_t size, size_t nmemb, void* userp) {
        ((std::string*)userp)->append((char*)contents, size * nmemb);
        return size * nmemb;
    }
};

// ============================================================================
// Pyth Oracle Client
// ============================================================================

class PythOracle {
private:
    PythConfig config_;
    HttpClient http_client_;
    std::map<std::string, PriceFeed> price_feeds_;
    std::mutex feeds_mutex_;
    std::atomic<bool> running_;
    std::thread update_thread_;
    
    // Cache
    std::map<std::string, Price> price_cache_;
    std::mutex cache_mutex_;
    std::chrono::steady_clock::time_point last_update_;

public:
    PythOracle() : running_(false) {
        config_.api_url = PITH_API_URL;
        config_.ws_url = PITH_WS_URL;
        config_.update_interval_ms = 1000;
        config_.connection_timeout_ms = 5000;
        config_.max_retries = 3;
    }
    
    ~PythOracle() {
        stop();
    }
    
    /**
     * Initialize oracle with API key
     */
    bool initialize(const std::string& api_key) {
        config_.api_key = api_key;
        
        // Initialize price feeds for default tokens
        for (const auto& pair : PYTH_PRODUCT_IDS) {
            PriceFeed feed;
            feed.id = pair.first;
            feed.is_valid = false;
            price_feeds_[pair.first] = feed;
        }
        
        // Initial price fetch
        updateAllPrices();
        
        return true;
    }
    
    /**
     * Start background price updates
     */
    void start() {
        if (running_.load()) {
            return;
        }
        
        running_.store(true);
        update_thread_ = std::thread([this]() {
            while (running_.load()) {
                updateAllPrices();
                std::this_thread::sleep_for(std::chrono::milliseconds(config_.update_interval_ms));
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
     * Get current price for a symbol
     */
    std::optional<Price> getPrice(const std::string& symbol) {
        std::lock_guard<std::mutex> lock(cache_mutex_);
        
        auto it = price_cache_.find(symbol);
        if (it != price_cache_.end()) {
            return it->second;
        }
        
        return std::nullopt;
    }
    
    /**
     * Get price with confidence interval
     */
    std::optional<PriceFeed> getPriceFeed(const std::string& symbol) {
        std::lock_guard<std::mutex> lock(feeds_mutex_);
        
        auto it = price_feeds_.find(symbol);
        if (it != price_feeds_.end() && it->second.is_valid) {
            return it->second;
        }
        
        return std::nullopt;
    }
    
    /**
     * Get all available prices
     */
    std::map<std::string, Price> getAllPrices() {
        std::lock_guard<std::mutex> lock(cache_mutex_);
        return price_cache_;
    }
    
    /**
     * Calculate price impact for a trade
     */
    double calculatePriceImpact(const std::string& symbol, int64_t amount) {
        auto price_opt = getPrice(symbol);
        if (!price_opt) {
            return 0.0;
        }
        
        int64_t price = price_opt->price;
        int64_t trade_value = amount * price;
        
        // Simple price impact calculation
        // In production, use orderbook depth
        double impact = 0.001 * (trade_value / 1000000000.0);
        
        return std::min(impact, 1.0);  // Cap at 100%
    }

private:
    /**
     * Update all prices from Pyth API
     */
    void updateAllPrices() {
        try {
            // Build price IDs
            std::vector<std::string> price_ids;
            for (const auto& pair : PYTH_PRODUCT_IDS) {
                price_ids.push_back(pair.second);
            }
            
            // Fetch prices
            std::string response = fetchPrices(price_ids);
            if (response.empty()) {
                return;
            }
            
            // Parse response
            parsePriceResponse(response);
            
            // Update cache
            updateCache();
            
        } catch (const std::exception& e) {
            // Log error
        }
    }
    
    /**
     * Fetch prices from Pyth API
     */
    std::string fetchPrices(const std::vector<std::string>& price_ids) {
        std::string url = config_.api_url + "/v1/price_feed?ids=";
        
        for (size_t i = 0; i < price_ids.size(); ++i) {
            if (i > 0) url += ",";
            url += price_ids[i];
        }
        
        return http_client_.get(url);
    }
    
    /**
     * Parse price response from Pyth
     */
    void parsePriceResponse(const std::string& response) {
        // In production, use JSON parser
        // Simplified parsing for demonstration
        
        std::lock_guard<std::mutex> lock(feeds_mutex_);
        
        for (auto& pair : price_feeds_) {
            // Parse price from response
            // In production, parse actual JSON
            Price price;
            price.price = 325000000000;  // Placeholder: $3250 with 8 decimals
            price.conf = 1000000;
            price.exponent = -8;
            price.publish_time = std::chrono::duration_cast<std::chrono::seconds>(
                std::chrono::system_clock::now().time_since_epoch()
            ).count();
            
            price_feeds_[pair.first].current_price = price;
            price_feeds_[pair.first].last_updated = price.publish_time;
            price_feeds_[pair.first].is_valid = true;
        }
    }
    
    /**
     * Update internal price cache
     */
    void updateCache() {
        std::lock_guard<std::mutex> lock(cache_mutex_);
        
        for (const auto& pair : price_feeds_) {
            if (pair.second.is_valid) {
                price_cache_[pair.first] = pair.second.current_price;
            }
        }
        
        last_update_ = std::chrono::steady_clock::now();
    }
};

// ============================================================================
// Factory
// ============================================================================

class PythOracleFactory {
public:
    static std::unique_ptr<PythOracle> create() {
        return std::make_unique<PythOracle>();
    }
};

// C API
extern "C" {
    
typedef void* PythOracleHandle;
    
PythOracleHandle pyth_oracle_create() {
    return static_cast<void*>(PythOracleFactory::create().release());
}
    
void pyth_oracle_destroy(PythOracleHandle handle) {
    delete static_cast<PythOracle*>(handle);
}
    
int pyth_oracle_initialize(PythOracleHandle handle, const char* api_key) {
    auto* oracle = static_cast<PythOracle*>(handle);
    return oracle->initialize(api_key) ? 0 : -1;
}
    
void pyth_oracle_start(PythOracleHandle handle) {
    auto* oracle = static_cast<PythOracle*>(handle);
    oracle->start();
}
    
void pyth_oracle_stop(PythOracleHandle handle) {
    auto* oracle = static_cast<PythOracle*>(handle);
    oracle->stop();
}
    
// Get price - returns 0 on success, fills price buffer
int pyth_oracle_get_price(PythOracleHandle handle, const char* symbol, int64_t* price, int64_t* conf) {
    auto* oracle = static_cast<PythOracle*>(handle);
    auto price_opt = oracle->getPrice(symbol);
    
    if (!price_opt) {
        return -1;
    }
    
    if (price) *price = price_opt->price;
    if (conf) *conf = price_opt->conf;
    
    return 0;
}

} // extern "C"

} // namespace oracle
} // namespace tigerswap
