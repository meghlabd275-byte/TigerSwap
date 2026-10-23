// TigerSwap ultra-low-latency matching engine.
//
// Exposes a strict HTTP/1.1 API over the in-memory matching engine, with an
// append-only write-ahead log for durability. Authentication uses API keys that
// are stored only as SHA-256 digests and compared in constant time.

#include <atomic>
#include <csignal>
#include <cstdlib>
#include <iostream>
#include <string>
#include <thread>
#include <unordered_map>
#include <vector>

#include "tigerswap/crypto.hpp"
#include "tigerswap/matching_engine.hpp"
#include "tigerswap/network.hpp"
#include "tigerswap/order_book.hpp"
#include "tigerswap/wal.hpp"

namespace {

using tigerswap::ExecReport;
using tigerswap::HttpRequest;
using tigerswap::HttpResponse;
using tigerswap::HttpServer;
using tigerswap::MatchResult;
using tigerswap::MatchingEngine;
using tigerswap::Order;
using tigerswap::OrderStatus;
using tigerswap::OrderType;
using tigerswap::PairConfig;
using tigerswap::RejectReason;
using tigerswap::RiskLimits;
using tigerswap::Router;
using tigerswap::ServerLimits;
using tigerswap::Side;
using tigerswap::TimeInForce;
using tigerswap::WalEventType;
using tigerswap::WriteAheadLog;

std::atomic<bool> g_shutdown{false};

void handle_signal(int) { g_shutdown.store(true); }

/// Scope required to call an endpoint.
enum class Scope { Read, Trade, Admin };

/// An API key: only the digest is retained.
struct ApiKey {
    std::string id;
    std::array<uint8_t, 32> digest{};
    Scope scope = Scope::Read;
    bool enabled = true;
};

/// Constant-time API key lookup.
class Authenticator {
public:
    void add_key(const std::string& id, const std::string& secret, Scope scope) {
        ApiKey key;
        key.id = id;
        key.digest = tigerswap::api_key_digest(secret);
        key.scope = scope;
        keys_.push_back(key);
    }

    bool empty() const { return keys_.empty(); }

    /// Authenticates a bearer token. Returns the scope when the key is valid.
    bool authenticate(const std::string& token, Scope required) const {
        if (token.empty()) {
            return false;
        }

        const std::array<uint8_t, 32> candidate = tigerswap::api_key_digest(token);

        // Compare against every key so timing does not reveal which id matched.
        bool matched = false;
        Scope matched_scope = Scope::Read;

        for (const auto& key : keys_) {
            const bool equal =
                tigerswap::constant_time_equal(candidate.data(), key.digest.data(), candidate.size());
            if (equal && key.enabled) {
                matched = true;
                matched_scope = key.scope;
            }
        }

        if (!matched) {
            return false;
        }

        switch (required) {
            case Scope::Read:
                return true;
            case Scope::Trade:
                return matched_scope == Scope::Trade || matched_scope == Scope::Admin;
            case Scope::Admin:
                return matched_scope == Scope::Admin;
        }
        return false;
    }

private:
    std::vector<ApiKey> keys_;
};

/// Engine service: engine + log + auth + JSON encoding.
class EngineService {
public:
    EngineService(WriteAheadLog& log, Authenticator& auth) : log_(log), auth_(auth) {}

    MatchingEngine& engine() { return engine_; }
    Authenticator& auth() { return auth_; }

    /// Loads pair configuration from the environment.
    void configure_pairs() {
        // Each entry is symbol:tick:maker_bps:taker_bps.
        const char* raw = std::getenv("TIGERSWAP_PAIRS");
        const std::string specification =
            raw != nullptr ? raw
                           : "BTC-RUSD:1:0:2,ETH-RUSD:1:0:2,SOL-RUSD:1:0:2,TGR-RUSD:1:0:2,"
                             "BTC-USDT:1:0:2,ETH-USDT:1:0:2,BTC-USDC:1:0:2,ETH-USDC:1:0:2";

        size_t position = 0;
        uint64_t next_pair_id = 1;

        while (position <= specification.size()) {
            const size_t comma = specification.find(',', position);
            const std::string entry =
                specification.substr(position, comma == std::string::npos ? std::string::npos
                                                                         : comma - position);

            if (!entry.empty()) {
                std::vector<std::string> fields;
                size_t start = 0;
                while (start <= entry.size()) {
                    const size_t colon = entry.find(':', start);
                    fields.push_back(entry.substr(
                        start, colon == std::string::npos ? std::string::npos : colon - start));
                    if (colon == std::string::npos) {
                        break;
                    }
                    start = colon + 1;
                }

                if (fields.size() == 4) {
                    PairConfig config;
                    config.pair_id = next_pair_id++;
                    config.symbol = fields[0];
                    config.tick_size = std::strtoull(fields[1].c_str(), nullptr, 10);
                    config.maker_fee_bps = static_cast<uint32_t>(std::strtoul(fields[2].c_str(), nullptr, 10));
                    config.taker_fee_bps = static_cast<uint32_t>(std::strtoul(fields[3].c_str(), nullptr, 10));
                    config.risk = RiskLimits{};

                    if (engine_.add_pair(config)) {
                        symbols_[config.pair_id] = config.symbol;
                        symbol_to_pair_[config.symbol] = config.pair_id;
                        pair_configs_[config.pair_id] = config;

                        std::vector<uint8_t> payload;
                        append_u64(payload, config.pair_id);
                        payload.insert(payload.end(), config.symbol.begin(), config.symbol.end());
                        log_.append(WalEventType::PairAdded, payload);
                    }
                }
            }

            if (comma == std::string::npos) {
                break;
            }
            position = comma + 1;
        }
    }

    /// Resolves a symbol to a pair id.
    bool pair_for_symbol(const std::string& symbol, uint64_t& pair_id) const {
        auto it = symbol_to_pair_.find(symbol);
        if (it == symbol_to_pair_.end()) {
            return false;
        }
        pair_id = it->second;
        return true;
    }

    std::string symbol_for_pair(uint64_t pair_id) const {
        auto it = symbols_.find(pair_id);
        return it == symbols_.end() ? std::string() : it->second;
    }

    const std::unordered_map<uint64_t, PairConfig>& pair_configs() const { return pair_configs_; }

    /// Records an accepted order in the log.
    void log_order(const Order& order, const ExecReport& report) {
        std::vector<uint8_t> payload;
        append_u64(payload, order.id);
        append_u64(payload, order.trader);
        append_u64(payload, order.pair);
        append_u64(payload, order.price);
        append_u64(payload, order.quantity);
        append_u64(payload, report.filled);
        payload.push_back(static_cast<uint8_t>(order.side));
        payload.push_back(static_cast<uint8_t>(order.type));
        payload.push_back(static_cast<uint8_t>(order.status));
        log_.append(WalEventType::OrderAccepted, payload);
    }

    void log_cancel(const ExecReport& report) {
        std::vector<uint8_t> payload;
        append_u64(payload, report.order_id);
        append_u64(payload, report.filled);
        log_.append(WalEventType::OrderCancelled, payload);
    }

    void log_trades(const std::vector<tigerswap::Trade>& trades) {
        for (const auto& trade : trades) {
            std::vector<uint8_t> payload;
            append_u64(payload, trade.id);
            append_u64(payload, trade.maker_order);
            append_u64(payload, trade.taker_order);
            append_u64(payload, trade.pair);
            append_u64(payload, trade.price);
            append_u64(payload, trade.quantity);
            append_u64(payload, trade.taker_fee);
            append_u64(payload, trade.timestamp);
            log_.append(WalEventType::TradeExecuted, payload);
        }
    }

    /// Replays the log to rebuild in-memory state after a restart.
    bool recover() {
        std::vector<tigerswap::WalRecord> records;
        if (!log_.recover(records)) {
            return false;
        }

        // Last record wins per order id: an order that was partially filled
        // later has a newer record with a larger `filled` value.
        std::unordered_map<uint64_t, Order> latest;
        std::vector<uint64_t> order_sequence;
        uint64_t highest_order_id = 0;

        for (const auto& record : records) {
            if (record.type == WalEventType::OrderAccepted && record.payload.size() >= 51) {
                Order order;
                order.id = read_u64(record.payload, 0);
                order.trader = read_u64(record.payload, 8);
                order.pair = read_u64(record.payload, 16);
                order.price = read_u64(record.payload, 24);
                order.quantity = read_u64(record.payload, 32);
                order.filled = read_u64(record.payload, 40);
                order.side = static_cast<Side>(record.payload[48]);
                order.type = static_cast<OrderType>(record.payload[49]);
                order.status = static_cast<OrderStatus>(record.payload[50]);

                highest_order_id = std::max(highest_order_id, order.id);
                if (latest.find(order.id) == latest.end()) {
                    order_sequence.push_back(order.id);
                }
                latest[order.id] = order;
            }
        }

        // Reinsert in first-seen order so time priority is preserved.
        for (const uint64_t order_id : order_sequence) {
            const Order& order = latest[order_id];
            if (order.status != OrderStatus::Open && order.status != OrderStatus::PartiallyFilled) {
                continue;
            }

            const uint64_t remaining = order.quantity > order.filled ? order.quantity - order.filled : 0;
            if (remaining == 0) {
                continue;
            }

            // Post-only guarantees recovery can never cross two recovered
            // orders and fabricate a trade.
            Order working;
            working.id = order.id;
            working.trader = order.trader;
            working.price = order.price;
            working.quantity = remaining;
            working.side = order.side;
            working.type = OrderType::Limit;

            engine_.submit(order.pair, working, TimeInForce::PostOnly);
        }

        // Replay trades so last price and volume match the pre-restart state.
        for (const auto& record : records) {
            if (record.type != WalEventType::TradeExecuted || record.payload.size() < 56) {
                continue;
            }

            tigerswap::Trade trade;
            trade.id = read_u64(record.payload, 0);
            trade.maker_order = read_u64(record.payload, 8);
            trade.taker_order = read_u64(record.payload, 16);
            trade.pair = read_u64(record.payload, 24);
            trade.price = read_u64(record.payload, 32);
            trade.quantity = read_u64(record.payload, 40);
            trade.taker_fee = read_u64(record.payload, 48);

            engine_.record_recovered_trade(trade.pair, trade.price, trade.quantity);
        }

        recovered_order_id_ = highest_order_id;
        return true;
    }

    /// Allocates the next order id, continuing past anything recovered.
    uint64_t next_order_id() {
        const uint64_t candidate = next_order_id_.fetch_add(1, std::memory_order_relaxed);
        return candidate > recovered_order_id_ ? candidate : ++recovered_order_id_;
    }

    static void append_u64(std::vector<uint8_t>& buffer, uint64_t value) {
        for (int index = 0; index < 8; ++index) {
            buffer.push_back(static_cast<uint8_t>((value >> (index * 8)) & 0xFF));
        }
    }

    static uint64_t read_u64(const std::vector<uint8_t>& buffer, size_t offset) {
        uint64_t value = 0;
        for (int index = 0; index < 8; ++index) {
            value |= static_cast<uint64_t>(buffer[offset + index]) << (index * 8);
        }
        return value;
    }

private:
    MatchingEngine engine_;
    WriteAheadLog& log_;
    Authenticator& auth_;

    std::unordered_map<uint64_t, std::string> symbols_;
    std::unordered_map<std::string, uint64_t> symbol_to_pair_;
    std::unordered_map<uint64_t, PairConfig> pair_configs_;

    std::atomic<uint64_t> next_order_id_{1};
    uint64_t recovered_order_id_ = 0;
};

/// Extracts the bearer token from the Authorization header.
std::string bearer_token(const HttpRequest& request) {
    const std::string header = request.header("authorization");
    const std::string prefix = "Bearer ";
    if (header.size() <= prefix.size() || header.compare(0, prefix.size(), prefix) != 0) {
        return {};
    }
    return header.substr(prefix.size());
}

HttpResponse unauthorized() {
    return HttpResponse::json("{\"error\":\"unauthorized\"}", 401);
}

HttpResponse forbidden() {
    return HttpResponse::json("{\"error\":\"forbidden\"}", 403);
}

std::string reason_name(RejectReason reason) {
    switch (reason) {
        case RejectReason::None: return "none";
        case RejectReason::InvalidOrder: return "invalid_order";
        case RejectReason::InvalidSide: return "invalid_side";
        case RejectReason::InvalidType: return "invalid_type";
        case RejectReason::PriceNotOnTick: return "price_not_on_tick";
        case RejectReason::PostOnlyWouldCross: return "post_only_would_cross";
        case RejectReason::FillOrKillUnfillable: return "fill_or_kill_unfillable";
        case RejectReason::NoLiquidity: return "no_liquidity";
        case RejectReason::StopRequiresTrigger: return "stop_requires_trigger";
        case RejectReason::UnknownOrder: return "unknown_order";
        case RejectReason::NotOrderOwner: return "not_order_owner";
        case RejectReason::AlreadyClosed: return "already_closed";
    }
    return "unknown";
}

bool parse_u64(const std::string& text, uint64_t& out) {
    if (text.empty()) {
        return false;
    }
    for (const char character : text) {
        if (character < '0' || character > '9') {
            return false;
        }
    }
    out = std::strtoull(text.c_str(), nullptr, 10);
    return true;
}

bool parse_side(const std::string& text, Side& out) {
    if (text == "buy" || text == "BUY" || text == "0") {
        out = Side::Buy;
        return true;
    }
    if (text == "sell" || text == "SELL" || text == "1") {
        out = Side::Sell;
        return true;
    }
    return false;
}

bool parse_order_type(const std::string& text, OrderType& out) {
    if (text.empty() || text == "limit" || text == "LIMIT") {
        out = OrderType::Limit;
        return true;
    }
    if (text == "market" || text == "MARKET") {
        out = OrderType::Market;
        return true;
    }
    return false;
}

bool parse_tif(const std::string& text, TimeInForce& out) {
    if (text.empty() || text == "gtc" || text == "GTC") {
        out = TimeInForce::Gtc;
        return true;
    }
    if (text == "ioc" || text == "IOC") {
        out = TimeInForce::Ioc;
        return true;
    }
    if (text == "fok" || text == "FOK") {
        out = TimeInForce::Fok;
        return true;
    }
    if (text == "post_only" || text == "POST_ONLY") {
        out = TimeInForce::PostOnly;
        return true;
    }
    return false;
}

/// Builds the router with every endpoint.
Router build_router(EngineService& service) {
    Router router;

    router.add("GET", "/health", [](const HttpRequest&) {
        return HttpResponse::json("{\"status\":\"ok\",\"engine\":\"tigerswap\"}");
    });

    router.add("GET", "/api/v1/pairs", [&service](const HttpRequest& request) {
        if (!service.auth().authenticate(bearer_token(request), Scope::Read)) {
            return unauthorized();
        }

        std::string body = "{\"pairs\":[";
        bool first = true;
        for (const auto& entry : service.pair_configs()) {
            if (!first) {
                body += ",";
            }
            first = false;
            body += "{\"pair_id\":" + std::to_string(entry.first);
            body += ",\"symbol\":\"" + tigerswap::json_escape(entry.second.symbol) + "\"";
            body += ",\"tick_size\":" + std::to_string(entry.second.tick_size);
            body += ",\"maker_fee_bps\":" + std::to_string(entry.second.maker_fee_bps);
            body += ",\"taker_fee_bps\":" + std::to_string(entry.second.taker_fee_bps);
            body += "}";
        }
        body += "]}";
        return HttpResponse::json(body);
    });

    router.add("GET", "/api/v1/depth", [&service](const HttpRequest& request) {
        if (!service.auth().authenticate(bearer_token(request), Scope::Read)) {
            return unauthorized();
        }

        uint64_t pair_id = 0;
        if (!service.pair_for_symbol(request.param("symbol"), pair_id)) {
            return HttpResponse::json("{\"error\":\"unknown symbol\"}", 404);
        }

        uint64_t levels = 20;
        parse_u64(request.param("levels"), levels);
        if (levels == 0 || levels > 500) {
            levels = 20;
        }

        const auto depth = service.engine().depth(pair_id, static_cast<size_t>(levels));
        std::string body = "{\"symbol\":\"" + tigerswap::json_escape(request.param("symbol")) + "\",";
        body += "\"last_price\":" + std::to_string(service.engine().last_price(pair_id)) + ",";
        body += "\"levels\":[";

        for (size_t index = 0; index < depth.size(); ++index) {
            if (index != 0) {
                body += ",";
            }
            body += "{\"price\":" + std::to_string(depth[index].price);
            body += ",\"quantity\":" + std::to_string(depth[index].quantity);
            body += ",\"orders\":" + std::to_string(depth[index].orders) + "}";
        }

        body += "]}";
        return HttpResponse::json(body);
    });

    router.add("POST", "/api/v1/order", [&service](const HttpRequest& request) {
        if (!service.auth().authenticate(bearer_token(request), Scope::Trade)) {
            return unauthorized();
        }

        std::unordered_map<std::string, std::string> fields;
        if (!tigerswap::parse_flat_json(request.body, fields)) {
            return HttpResponse::json("{\"error\":\"malformed json body\"}", 400);
        }

        uint64_t pair_id = 0;
        auto symbol_it = fields.find("symbol");
        if (symbol_it == fields.end() || !service.pair_for_symbol(symbol_it->second, pair_id)) {
            return HttpResponse::json("{\"error\":\"unknown symbol\"}", 400);
        }

        Order order;
        order.pair = pair_id;

        auto trader_it = fields.find("trader");
        if (trader_it == fields.end() || !parse_u64(trader_it->second, order.trader)) {
            return HttpResponse::json("{\"error\":\"trader must be an unsigned integer\"}", 400);
        }

        Side side = Side::Buy;
        auto side_it = fields.find("side");
        if (side_it == fields.end() || !parse_side(side_it->second, side)) {
            return HttpResponse::json("{\"error\":\"side must be buy or sell\"}", 400);
        }
        order.side = side;

        OrderType type = OrderType::Limit;
        auto type_it = fields.find("type");
        if (type_it != fields.end() && !parse_order_type(type_it->second, type)) {
            return HttpResponse::json("{\"error\":\"unsupported order type\"}", 400);
        }
        order.type = type;

        TimeInForce tif = TimeInForce::Gtc;
        auto tif_it = fields.find("tif");
        if (tif_it != fields.end() && !parse_tif(tif_it->second, tif)) {
            return HttpResponse::json("{\"error\":\"unsupported time in force\"}", 400);
        }

        auto quantity_it = fields.find("quantity");
        if (quantity_it == fields.end() || !parse_u64(quantity_it->second, order.quantity)) {
            return HttpResponse::json("{\"error\":\"quantity must be an unsigned integer\"}", 400);
        }

        if (type == OrderType::Limit) {
            auto price_it = fields.find("price");
            if (price_it == fields.end() || !parse_u64(price_it->second, order.price)) {
                return HttpResponse::json("{\"error\":\"price must be an unsigned integer\"}", 400);
            }
        }

        order.id = service.next_order_id();

        const MatchResult result = service.engine().submit(pair_id, order, tif);

        if (!result.report.accepted) {
            return HttpResponse::json(
                "{\"error\":\"rejected\",\"reason\":\"" + reason_name(result.report.reason) + "\"}", 400);
        }

        service.log_order(order, result.report);
        // Resting makers changed size, so persist their new state as well.
        // Recovery keeps the last record per order id, which restores the exact
        // remaining quantity.
        for (const auto& trade : result.trades) {
            const Order* maker = service.engine().find_order(pair_id, trade.maker_order);
            if (maker != nullptr) {
                ExecReport maker_report;
                maker_report.accepted = true;
                maker_report.order_id = maker->id;
                maker_report.filled = maker->filled;
                maker_report.remaining = maker->remaining();
                service.log_order(*maker, maker_report);
            }
        }
        service.log_trades(result.trades);

        std::string body = "{\"order_id\":" + std::to_string(result.report.order_id);
        body += ",\"filled\":" + std::to_string(result.report.filled);
        body += ",\"remaining\":" + std::to_string(result.report.remaining);
        body += ",\"trades\":[";
        for (size_t index = 0; index < result.trades.size(); ++index) {
            if (index != 0) {
                body += ",";
            }
            body += "{\"trade_id\":" + std::to_string(result.trades[index].id);
            body += ",\"price\":" + std::to_string(result.trades[index].price);
            body += ",\"quantity\":" + std::to_string(result.trades[index].quantity);
            body += ",\"fee\":" + std::to_string(result.trades[index].taker_fee) + "}";
        }
        body += "]}";
        return HttpResponse::json(body, 201);
    });

    router.add("DELETE", "/api/v1/order", [&service](const HttpRequest& request) {
        if (!service.auth().authenticate(bearer_token(request), Scope::Trade)) {
            return unauthorized();
        }

        std::unordered_map<std::string, std::string> fields;
        if (!tigerswap::parse_flat_json(request.body, fields)) {
            return HttpResponse::json("{\"error\":\"malformed json body\"}", 400);
        }

        uint64_t pair_id = 0;
        auto symbol_it = fields.find("symbol");
        if (symbol_it == fields.end() || !service.pair_for_symbol(symbol_it->second, pair_id)) {
            return HttpResponse::json("{\"error\":\"unknown symbol\"}", 400);
        }

        uint64_t order_id = 0;
        auto order_it = fields.find("order_id");
        if (order_it == fields.end() || !parse_u64(order_it->second, order_id)) {
            return HttpResponse::json("{\"error\":\"order_id must be an unsigned integer\"}", 400);
        }

        uint64_t trader = 0;
        auto trader_it = fields.find("trader");
        if (trader_it == fields.end() || !parse_u64(trader_it->second, trader)) {
            return HttpResponse::json("{\"error\":\"trader must be an unsigned integer\"}", 400);
        }

        const ExecReport report = service.engine().cancel(pair_id, order_id, trader);
        if (!report.accepted) {
            return HttpResponse::json(
                "{\"error\":\"cancelled failed\",\"reason\":\"" + reason_name(report.reason) + "\"}", 400);
        }

        service.log_cancel(report);
        return HttpResponse::json("{\"order_id\":" + std::to_string(order_id) + ",\"status\":\"cancelled\"}");
    });

    router.add("GET", "/api/v1/order", [&service](const HttpRequest& request) {
        if (!service.auth().authenticate(bearer_token(request), Scope::Read)) {
            return unauthorized();
        }

        uint64_t pair_id = 0;
        if (!service.pair_for_symbol(request.param("symbol"), pair_id)) {
            return HttpResponse::json("{\"error\":\"unknown symbol\"}", 404);
        }

        uint64_t order_id = 0;
        if (!parse_u64(request.param("order_id"), order_id)) {
            return HttpResponse::json("{\"error\":\"order_id must be an unsigned integer\"}", 400);
        }

        const Order* order = service.engine().find_order(pair_id, order_id);
        if (order == nullptr) {
            return HttpResponse::json("{\"error\":\"order not found\"}", 404);
        }

        std::string body = "{\"order_id\":" + std::to_string(order->id);
        body += ",\"trader\":" + std::to_string(order->trader);
        body += ",\"price\":" + std::to_string(order->price);
        body += ",\"quantity\":" + std::to_string(order->quantity);
        body += ",\"filled\":" + std::to_string(order->filled);
        body += ",\"status\":" + std::to_string(static_cast<int>(order->status));
        body += "}";
        return HttpResponse::json(body);
    });

    router.add("GET", "/api/v1/stats", [&service](const HttpRequest& request) {
        if (!service.auth().authenticate(bearer_token(request), Scope::Read)) {
            return unauthorized();
        }

        const auto stats = service.engine().stats();
        std::string body = "{\"orders_accepted\":" + std::to_string(stats.orders_accepted);
        body += ",\"orders_rejected\":" + std::to_string(stats.orders_rejected);
        body += ",\"orders_cancelled\":" + std::to_string(stats.orders_cancelled);
        body += ",\"trades\":" + std::to_string(stats.trades);
        body += ",\"volume\":" + std::to_string(stats.volume);
        body += ",\"pairs\":" + std::to_string(stats.pairs);
        body += ",\"open_orders\":" + std::to_string(stats.open_orders);
        body += ",\"latency_p50_ns\":" + std::to_string(stats.latency_p50_ns);
        body += ",\"latency_p99_ns\":" + std::to_string(stats.latency_p99_ns);
        body += ",\"latency_max_ns\":" + std::to_string(stats.latency_max_ns);
        body += "}";
        return HttpResponse::json(body);
    });

    return router;
}

}  // namespace

int main() {
    std::signal(SIGPIPE, SIG_IGN);
    std::signal(SIGINT, handle_signal);
    std::signal(SIGTERM, handle_signal);

    const char* address = std::getenv("TIGERSWAP_BIND");
    const char* port_text = std::getenv("TIGERSWAP_PORT");
    const char* log_path = std::getenv("TIGERSWAP_WAL");
    const char* key_spec = std::getenv("TIGERSWAP_API_KEYS");

    const std::string bind_address = address != nullptr ? address : "127.0.0.1";
    const uint16_t port = port_text != nullptr ? static_cast<uint16_t>(std::strtoul(port_text, nullptr, 10))
                                               : 8081;
    const std::string wal_path = log_path != nullptr ? log_path : "/var/lib/tigerswap/engine.wal";

    Authenticator auth;

    // Keys are supplied as id:secret:scope entries. No key has a default value:
    // an unconfigured engine refuses every authenticated call.
    if (key_spec != nullptr) {
        const std::string specification = key_spec;
        size_t position = 0;
        while (position <= specification.size()) {
            const size_t comma = specification.find(',', position);
            const std::string entry = specification.substr(
                position, comma == std::string::npos ? std::string::npos : comma - position);

            if (!entry.empty()) {
                const size_t first = entry.find(':');
                const size_t second = first == std::string::npos ? std::string::npos : entry.find(':', first + 1);
                if (first != std::string::npos && second != std::string::npos) {
                    const std::string id = entry.substr(0, first);
                    const std::string secret = entry.substr(first + 1, second - first - 1);
                    const std::string scope = entry.substr(second + 1);

                    Scope parsed = Scope::Read;
                    if (scope == "trade") {
                        parsed = Scope::Trade;
                    } else if (scope == "admin") {
                        parsed = Scope::Admin;
                    }

                    auth.add_key(id, secret, parsed);
                }
            }

            if (comma == std::string::npos) {
                break;
            }
            position = comma + 1;
        }
    }

    WriteAheadLog log;
    if (!log.open(wal_path)) {
        std::cerr << "tigerswap: cannot open write-ahead log at " << wal_path << "\n";
        return 1;
    }

    EngineService service(log, auth);
    service.configure_pairs();

    if (!service.recover()) {
        std::cerr << "tigerswap: write-ahead log recovery failed\n";
        return 1;
    }

    ServerLimits limits;
    if (const char* connections = std::getenv("TIGERSWAP_MAX_CONNECTIONS")) {
        limits.max_connections = std::strtoull(connections, nullptr, 10);
    }
    if (const char* rps = std::getenv("TIGERSWAP_RATE_LIMIT")) {
        limits.requests_per_second = static_cast<uint32_t>(std::strtoul(rps, nullptr, 10));
    }

    HttpServer server(bind_address, port, limits, build_router(service));
    if (!server.start()) {
        std::cerr << "tigerswap: cannot bind " << bind_address << ":" << port << "\n";
        return 1;
    }

    std::cout << "tigerswap engine listening on " << bind_address << ":" << server.port()
              << " with " << service.pair_configs().size() << " pairs" << std::endl;

    while (!g_shutdown.load()) {
        server.poll_once(100);
    }

    log.flush();
    log.sync();
    std::cout << "tigerswap engine stopped" << std::endl;
    return 0;
}
