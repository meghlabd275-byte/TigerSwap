/**
 * @file tigerswap_client.hpp
 * @brief TigerSwap C++ SDK Header
 * 
 * Enterprise-grade C++ SDK for TigerSwap DEX
 */

#ifndef TIGERSWAP_CLIENT_HPP
#define TIGERSWAP_CLIENT_HPP

#include <string>
#include <vector>
#include <map>
#include <memory>
#include <functional>
#include <curl/curl.h>
#include <nlohmann/json.hpp>

namespace tigerswap {

using json = nlohmann::json;

// ============================================================================
// Data Structures
// ============================================================================

struct Token {
    std::string address;
    std::string symbol;
    std::string name;
    uint8_t decimals;
    uint64_t chain_id;
    std::string logo_url;
};

struct Quote {
    std::string from_token;
    std::string to_token;
    std::string from_amount;
    std::string to_amount;
    std::string price;
    std::string price_impact;
    std::string gas_estimate;
    std::vector<std::string> route;
};

struct Order {
    std::string id;
    std::string user;
    std::string pair;
    std::string side;        // "buy" or "sell"
    std::string order_type;  // "limit", "market", "stop_loss", "take_profit"
    std::string price;
    std::string quantity;
    std::string filled;
    std::string status;     // "open", "filled", "cancelled"
    uint64_t created_at;
    uint64_t expires_at;
};

struct Position {
    std::string id;
    std::string user;
    std::string pair;
    std::string side;      // "long" or "short"
    std::string size;
    std::string collateral;
    std::string leverage;
    std::string entry_price;
    std::string mark_price;
    std::string pnl;
    std::string roe;
    std::string liquidation_price;
    std::string status;
};

struct Pool {
    std::string address;
    std::string token_a;
    std::string token_b;
    std::string reserve_a;
    std::string reserve_b;
    std::string total_supply;
    std::string fee;
};

struct Chain {
    uint64_t id;
    std::string name;
    std::string symbol;
    std::string rpc_url;
    std::string explorer_url;
    std::string native_token;
    bool is_active;
};

struct MarketData {
    std::string pair;
    std::string price;
    std::string price_24h_ago;
    std::string change_24h;
    std::string volume_24h;
    std::string liquidity;
    std::string high_24h;
    std::string low_24h;
};

struct OrderBookEntry {
    std::string price;
    std::string quantity;
};

struct OrderBook {
    std::string pair;
    std::vector<OrderBookEntry> bids;
    std::vector<OrderBookEntry> asks;
    uint64_t updated_at;
};

struct SwapRequest {
    std::string from_token;
    std::string to_token;
    std::string amount;
    uint32_t slippage;  // basis points
    std::string to;
};

struct SwapResponse {
    std::string tx_hash;
    std::string from_token;
    std::string to_token;
    std::string from_amount;
    std::string to_amount;
    std::string status;
};

// ============================================================================
// Client Class
// ============================================================================

class TigerSwapClient {
public:
    /**
     * @brief Construct a new TigerSwap Client
     * @param api_key API key for authentication
     * @param base_url Base URL for the API (optional)
     */
    explicit TigerSwapClient(const std::string& api_key, 
                           const std::string& base_url = "https://api.tigerswap.exchange");
    
    ~TigerSwapClient();
    
    // Delete copy constructor and assignment
    TigerSwapClient(const TigerSwapClient&) = delete;
    TigerSwapClient& operator=(const TigerSwapClient&) = delete;
    
    // Allow move
    TigerSwapClient(TigerSwapClient&&) noexcept;
    TigerSwapClient& operator=(TigerSwapClient&&) noexcept;
    
    // ========================================================================
    // Trading Methods
    // ========================================================================
    
    /**
     * @brief Get a swap quote
     * @param from Source token address
     * @param to Destination token address
     * @param amount Amount to swap
     * @return Quote object
     */
    Quote getQuote(const std::string& from, 
                   const std::string& to, 
                   const std::string& amount);
    
    /**
     * @brief Execute a swap
     * @param request Swap request parameters
     * @return Swap response with transaction hash
     */
    SwapResponse swap(const SwapRequest& request);
    
    /**
     * @brief Get supported tokens
     * @param chain_id Chain ID
     * @return Vector of supported tokens
     */
    std::vector<Token> getTokens(uint64_t chain_id);
    
    /**
     * @brief Get order book for a pair
     * @param pair Trading pair
     * @return Order book
     */
    OrderBook getOrderBook(const std::string& pair);
    
    /**
     * @brief Get market data
     * @param pair Trading pair
     * @return Market data
     */
    MarketData getMarket(const std::string& pair);
    
    // ========================================================================
    // Order Management
    // ========================================================================
    
    /**
     * @brief Get user's orders
     * @param user User address
     * @return Vector of orders
     */
    std::vector<Order> getOrders(const std::string& user);
    
    /**
     * @brief Create a new order
     * @param order Order to create
     * @return Created order
     */
    Order createOrder(const Order& order);
    
    /**
     * @brief Cancel an order
     * @param order_id Order ID to cancel
     */
    void cancelOrder(const std::string& order_id);
    
    // ========================================================================
    // Position Management
    // ========================================================================
    
    /**
     * @brief Get user's positions
     * @param user User address
     * @return Vector of positions
     */
    std::vector<Position> getPositions(const std::string& user);
    
    // ========================================================================
    // Liquidity
    // ========================================================================
    
    /**
     * @brief Get available pools
     * @return Vector of pools
     */
    std::vector<Pool> getPools();
    
    // ========================================================================
    // Network
    // ========================================================================
    
    /**
     * @brief Get supported chains
     * @return Vector of chains
     */
    std::vector<Chain> getChains();
    
private:
    std::string api_key_;
    std::string base_url_;
    CURL* curl_;
    
    // Helper methods
    std::string makeRequest(const std::string& method, 
                            const std::string& endpoint,
                            const std::string& body = "");
    
    template<typename T>
    T parseResponse(const std::string& response);
    
    static size_t writeCallback(void* contents, size_t size, size_t nmemb, void* userp);
};

// ============================================================================
// Implementation
// ============================================================================

inline TigerSwapClient::TigerSwapClient(const std::string& api_key, 
                                         const std::string& base_url)
    : api_key_(api_key), base_url_(base_url) {
    curl_ = curl_easy_init();
}

inline TigerSwapClient::~TigerSwapClient() {
    if (curl_) {
        curl_easy_cleanup(curl_);
    }
}

inline TigerSwapClient::TigerSwapClient(TigerSwapClient&& other) noexcept 
    : api_key_(std::move(other.api_key_)),
      base_url_(std::move(other.base_url_)),
      curl_(other.curl_) {
    other.curl_ = nullptr;
}

inline TigerSwapClient& TigerSwapClient::operator=(TigerSwapClient&& other) noexcept {
    if (this != &other) {
        if (curl_) {
            curl_easy_cleanup(curl_);
        }
        api_key_ = std::move(other.api_key_);
        base_url_ = std::move(other.base_url_);
        curl_ = other.curl_;
        other.curl_ = nullptr;
    }
    return *this;
}

inline Quote TigerSwapClient::getQuote(const std::string& from,
                                        const std::string& to,
                                        const std::string& amount) {
    std::string endpoint = "/v1/quote?from=" + from + "&to=" + to + "&amount=" + amount;
    std::string response = makeRequest("GET", endpoint);
    return parseResponse<Quote>(response);
}

inline SwapResponse TigerSwapClient::swap(const SwapRequest& request) {
    json body = {
        {"from_token", request.from_token},
        {"to_token", request.to_token},
        {"amount", request.amount},
        {"slippage", request.slippage},
        {"to", request.to}
    };
    std::string response = makeRequest("POST", "/v1/swap", body.dump());
    return parseResponse<SwapResponse>(response);
}

inline std::vector<Token> TigerSwapClient::getTokens(uint64_t chain_id) {
    std::string endpoint = "/v1/tokens?chain_id=" + std::to_string(chain_id);
    std::string response = makeRequest("GET", endpoint);
    return parseResponse<std::vector<Token>>(response);
}

inline OrderBook TigerSwapClient::getOrderBook(const std::string& pair) {
    std::string endpoint = "/v1/orderbook/" + pair;
    std::string response = makeRequest("GET", endpoint);
    return parseResponse<OrderBook>(response);
}

inline MarketData TigerSwapClient::getMarket(const std::string& pair) {
    std::string endpoint = "/v1/market/" + pair;
    std::string response = makeRequest("GET", endpoint);
    return parseResponse<MarketData>(response);
}

inline std::vector<Order> TigerSwapClient::getOrders(const std::string& user) {
    std::string endpoint = "/v1/orders?user=" + user;
    std::string response = makeRequest("GET", endpoint);
    return parseResponse<std::vector<Order>>(response);
}

inline Order TigerSwapClient::createOrder(const Order& order) {
    json body = {
        {"user", order.user},
        {"pair", order.pair},
        {"side", order.side},
        {"order_type", order.order_type},
        {"price", order.price},
        {"quantity", order.quantity}
    };
    std::string response = makeRequest("POST", "/v1/orders", body.dump());
    return parseResponse<Order>(response);
}

inline void TigerSwapClient::cancelOrder(const std::string& order_id) {
    makeRequest("DELETE", "/v1/orders/" + order_id);
}

inline std::vector<Position> TigerSwapClient::getPositions(const std::string& user) {
    std::string endpoint = "/v1/positions?user=" + user;
    std::string response = makeRequest("GET", endpoint);
    return parseResponse<std::vector<Position>>(response);
}

inline std::vector<Pool> TigerSwapClient::getPools() {
    std::string response = makeRequest("GET", "/v1/pools");
    return parseResponse<std::vector<Pool>>(response);
}

inline std::vector<Chain> TigerSwapClient::getChains() {
    std::string response = makeRequest("GET", "/v1/chains");
    return parseResponse<std::vector<Chain>>(response);
}

inline std::string TigerSwapClient::makeRequest(const std::string& method,
                                                const std::string& endpoint,
                                                const std::string& body) {
    std::string url = base_url_ + endpoint;
    std::string response_buffer;
    
    if (curl_) {
        curl_easy_setopt(curl_, CURLOPT_URL, url.c_str());
        curl_easy_setopt(curl_, CURLOPT_WRITEFUNCTION, writeCallback);
        curl_easy_setopt(curl_, CURLOPT_WRITEDATA, &response_buffer);
        
        // Set headers
        struct curl_slist* headers = nullptr;
        headers = curl_slist_append(headers, ("Authorization: Bearer " + api_key_).c_str());
        headers = curl_slist_append(headers, "Content-Type: application/json");
        curl_easy_setopt(curl_, CURLOPT_HTTPHEADER, headers);
        
        if (method == "POST") {
            curl_easy_setopt(curl_, CURLOPT_POST, 1);
            curl_easy_setopt(curl_, CURLOPT_POSTFIELDS, body.c_str());
        } else if (method == "DELETE") {
            curl_easy_setopt(curl_, CURLOPT_CUSTOMREQUEST, "DELETE");
        }
        
        curl_easy_perform(curl_);
        
        if (headers) {
            curl_slist_free_all(headers);
        }
    }
    
    return response_buffer;
}

inline size_t TigerSwapClient::writeCallback(void* contents, size_t size, 
                                              size_t nmemb, void* userp) {
    size_t realsize = size * nmemb;
    std::string* str = static_cast<std::string*>(userp);
    str->append(static_cast<char*>(contents), realsize);
    return realsize;
}

template<typename T>
inline T TigerSwapClient::parseResponse(const std::string& response) {
    auto j = json::parse(response);
    return j.get<T>();
}

} // namespace tigerswap

#endif // TIGERSWAP_CLIENT_HPP
