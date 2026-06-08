#include "trading_engine.hpp"
#include <algorithm>
#include <cmath>
#include <iostream>

namespace tigerswap {

// ============ Routing Engine Implementation ============

RoutingEngine::RoutingEngine(
    std::shared_ptr<PriceEngine> price_engine,
    std::shared_ptr<LiquidityScanner> liquidity_scanner
) : price_engine_(price_engine),
    liquidity_scanner_(liquidity_scanner) {}

RoutingEngine::~RoutingEngine() = default;

std::optional<Route> RoutingEngine::get_route(
    const TokenPair& pair,
    uint64_t amount_in,
    uint64_t max_slippage_bps
) {
    // Get quotes from all DEXs
    auto pools = liquidity_scanner_->scan_pools(pair);
    
    if (pools.empty()) {
        return std::nullopt;
    }
    
    // Get price for price impact calculation
    auto price = price_engine_->get_price(pair);
    uint64_t price_impact = price_engine_->calculate_price_impact(pair, amount_in);
    
    // Find best DEX
    Route best_route;
    best_route.amount_out = 0;
    
    for (const auto& pool : pools) {
        // Calculate output amount
        uint64_t amount_out = calculate_output(
            amount_in,
            pool.reserve0,
            pool.reserve1,
            pool.fee_tier
        );
        
        // Apply price impact
        uint64_t impact_adjustment = amount_out * price_impact / kPricePrecision;
        amount_out -= impact_adjustment;
        
        // Check slippage
        uint64_t min_acceptable = amount_in * (kPricePrecision - max_slippage_bps * 100) / kPricePrecision;
        
        if (amount_out >= min_acceptable && amount_out > best_route.amount_out) {
            best_route.amount_out = amount_out;
            best_route.path = {pair.to_string()};
            best_route.gas_used = 150000;  // Estimated
            best_route.input_amount = amount_in;
            best_route.quotes = {{pair, amount_in, amount_out, 0, 150000, pool.dex_name, {pair.to_string()}}};
        }
    }
    
    if (best_route.amount_out == 0) {
        return std::nullopt;
    }
    
    return best_route;
}

std::vector<Route> RoutingEngine::get_routes(
    const TokenPair& pair,
    uint64_t amount_in,
    uint32_t max_routes
) {
    auto pools = liquidity_scanner_->scan_pools(pair);
    std::vector<Route> routes;
    
    for (const auto& pool : pools) {
        if (routes.size() >= max_routes) break;
        
        uint64_t amount_out = calculate_output(
            amount_in,
            pool.reserve0,
            pool.reserve1,
            pool.fee_tier
        );
        
        Route route;
        route.path = {pair.to_string()};
        route.amount_out = amount_out;
        route.gas_used = 150000;
        route.input_amount = amount_in;
        
        routes.push_back(route);
    }
    
    // Sort by amount out (descending)
    std::sort(routes.begin(), routes.end(), 
        [](const Route& a, const Route& b) {
            return a.amount_out > b.amount_out;
        });
    
    return routes;
}

std::optional<Route> RoutingEngine::get_split_route(
    const TokenPair& pair,
    uint64_t amount_in,
    uint32_t max_splits
) {
    auto pools = liquidity_scanner_->scan_pools(pair);
    
    if (pools.empty()) {
        return std::nullopt;
    }
    
    // Calculate optimal splits based on liquidity
    uint64_t total_liquidity = 0;
    for (const auto& pool : pools) {
        total_liquidity += pool.liquidity;
    }
    
    Route split_route;
    split_route.amount_out = 0;
    split_route.gas_used = 0;
    
    for (const auto& pool : pools) {
        if (split_route.quotes.size() >= max_splits) break;
        
        // Proportional split
        uint64_t split_amount = amount_in * pool.liquidity / total_liquidity;
        if (split_amount < kMinOrderSize) continue;
        
        uint64_t amount_out = calculate_output(
            split_amount,
            pool.reserve0,
            pool.reserve1,
            pool.fee_tier
        );
        
        Quote quote;
        quote.pair = pair;
        quote.amount_in = split_amount;
        quote.amount_out = amount_out;
        quote.dex_name = pool.dex_name;
        quote.path = {pair.to_string()};
        
        split_route.quotes.push_back(quote);
        split_route.amount_out += amount_out;
        split_route.gas_used += 150000;
    }
    
    if (split_route.quotes.empty()) {
        return std::nullopt;
    }
    
    split_route.path = {pair.to_string()};
    split_route.input_amount = amount_in;
    
    return split_route;
}

std::optional<Route> RoutingEngine::get_multihop_route(
    const std::vector<TokenPair>& path,
    uint64_t amount_in
) {
    if (path.size() < 2) {
        return std::nullopt;
    }
    
    Route route;
    route.input_amount = amount_in;
    uint64_t current_amount = amount_in;
    
    for (size_t i = 0; i < path.size() - 1; ++i) {
        const auto& pair = path[i];
        auto route_opt = get_route(pair, current_amount);
        
        if (!route_opt) {
            return std::nullopt;
        }
        
        current_amount = route_opt->amount_out;
        route.quotes.push_back(route_opt->quotes[0]);
    }
    
    route.amount_out = current_amount;
    route.path.reserve(path.size());
    for (const auto& pair : path) {
        route.path.push_back(pair.to_string());
    }
    
    return route;
}

void RoutingEngine::add_dex(const std::string& dex_name, void* connector) {
    dex_connectors_[dex_name] = connector;
}

uint64_t RoutingEngine::calculate_output(
    uint64_t amount_in,
    uint64_t reserve0,
    uint64_t reserve1,
    uint32_t fee_tier
) {
    if (reserve0 == 0 || reserve1 == 0) return 0;
    
    // Apply fee
    uint64_t amount_in_after_fee = amount_in * (10000 - fee_tier) / 10000;
    
    // Constant product formula: amountOut = (amountIn * reserve1 * reserve0) / (reserve0 + amountIn)
    // Simplified: amountOut = amountIn * reserve1 / reserve0
    uint256_t product = static_cast<uint256_t>(amount_in_after_fee) * reserve1;
    uint64_t amount_out = static_cast<uint64_t>(product / reserve0);
    
    return amount_out;
}

// ============ Order Executor Implementation ============

OrderExecutor::OrderExecutor(
    std::shared_ptr<RoutingEngine> routing_engine,
    std::shared_ptr<OrderBook> order_book
) : routing_engine_(routing_engine),
    order_book_(order_book) {}

OrderExecutor::~OrderExecutor() = default;

std::string OrderExecutor::execute_order(
    const Order& order,
    const std::string& private_key
) {
    // Get route
    auto route = routing_engine_->get_route(
        order.pair,
        order.amount_in
    );
    
    if (!route) {
        return "";
    }
    
    // Apply slippage check
    uint64_t min_out = order.amount_in * order.amount_out_min / order.amount_in;
    if (route->amount_out < min_out) {
        return "";
    }
    
    return sign_and_send(order, route->amount_out, private_key);
}

std::vector<std::string> OrderExecutor::execute_orders(
    const std::vector<Order>& orders,
    const std::string& private_key
) {
    std::vector<std::string> results;
    
    for (const auto& order : orders) {
        if (order.status == OrderStatus::kPending) {
            std::string tx_hash = execute_order(order, private_key);
            if (!tx_hash.empty()) {
                results.push_back(tx_hash);
            }
        }
    }
    
    return results;
}

uint64_t OrderExecutor::fill_order_book(
    const TokenPair& pair,
    uint64_t amount_in,
    const std::string& private_key
) {
    uint64_t total_filled = 0;
    auto buy_orders = order_book_->get_orders(Side::kBuy, kOrderBookDepth);
    
    for (const auto& order : buy_orders) {
        if (amount_in == 0) break;
        
        uint64_t fill_amount = std::min(
            amount_in,
            order.amount_in - order.executed_amount_in
        );
        
        if (fill_amount > 0) {
            std::string tx_hash = sign_and_send(order, fill_amount, private_key);
            if (!tx_hash.empty()) {
                Order updated = order;
                updated.executed_amount_in += fill_amount;
                updated.status = OrderStatus::kFilled;
                order_book_->update_order(updated);
                
                total_filled += fill_amount;
                amount_in -= fill_amount;
            }
        }
    }
    
    return total_filled;
}

void OrderExecutor::set_executor(const std::string& address) {
    executor_address_ = address;
}

std::vector<std::string> OrderExecutor::get_pending_txs() const {
    return pending_txs_;
}

std::string OrderExecutor::sign_and_send(
    const Order& order,
    uint64_t amount_in,
    const std::string& private_key
) {
    // In production, this would:
    // 1. Sign the transaction with the private key
    // 2. Send to the network
    // 3. Return the transaction hash
    
    // For now, return empty string (placeholder)
    return "";
}

// ============ Trading Engine Implementation ============

TradingEngine::TradingEngine() 
    : next_order_id_(0),
    running_(false) {}

TradingEngine::~TradingEngine() {
    if (running_) {
        stop();
    }
}

bool TradingEngine::initialize(const std::string& config_path) {
    // Initialize components
    price_engine_ = std::make_shared<PriceEngine>();
    liquidity_scanner_ = std::make_shared<LiquidityScanner>();
    routing_engine_ = std::make_shared<RoutingEngine>(
        price_engine_,
        liquidity_scanner_
    );
    order_book_ = std::make_shared<OrderBook>();
    order_executor_ = std::make_shared<OrderExecutor>(
        routing_engine_,
        order_book_
    );
    
    return true;
}

void TradingEngine::start() {
    if (running_) return;
    running_ = true;
    
    std::cout << "[TradingEngine] Started" << std::endl;
}

void TradingEngine::stop() {
    if (!running_) return;
    running_ = false;
    
    std::cout << "[TradingEngine] Stopped" << std::endl;
}

OrderId TradingEngine::create_order(const Order& order) {
    Order new_order = order;
    new_order.id = ++next_order_id_;
    new_order.created_at = std::chrono::duration_cast<Timestamp>(
        std::chrono::system_clock::now().time_since_epoch()
    );
    new_order.status = OrderStatus::kPending;
    
    order_book_->add_order(new_order);
    
    stats_.total_orders++;
    
    return new_order.id;
}

bool TradingEngine::cancel_order(OrderId order_id) {
    auto order = order_book_->get_order(order_id);
    if (!order) return false;
    
    order->status = OrderStatus::kCancelled;
    order_book_->update_order(*order);
    
    return true;
}

std::optional<Order> TradingEngine::get_order(OrderId order_id) const {
    return order_book_->get_order(order_id);
}

std::vector<Order> TradingEngine::get_pending_orders() const {
    return order_book_->get_orders(Side::kBuy, 1000);
}

std::vector<Order> TradingEngine::get_order_book(
    const TokenPair& pair,
    Side side,
    uint64_t limit
) const {
    return order_book_->get_orders(side, limit);
}

std::optional<Quote> TradingEngine::get_quote(
    const TokenPair& pair,
    uint64_t amount_in
) {
    auto route = routing_engine_->get_route(pair, amount_in);
    
    if (!route) {
        return std::nullopt;
    }
    
    Quote quote;
    quote.pair = pair;
    quote.amount_in = amount_in;
    quote.amount_out = route->amount_out;
    quote.price = route->amount_out * kPricePrecision / amount_in;
    quote.gas_used = route->gas_used;
    quote.timestamp = std::chrono::duration_cast<Timestamp>(
        std::chrono::system_clock::now().time_since_epoch()
    );
    
    return quote;
}

std::optional<Route> TradingEngine::get_route(
    const TokenPair& pair,
    uint64_t amount_in
) {
    return routing_engine_->get_route(pair, amount_in);
}

std::string TradingEngine::execute_swap(
    const TokenPair& pair,
    uint64_t amount_in,
    uint64_t amount_out_min,
    const std::string& recipient,
    const std::string& private_key
) {
    auto route = routing_engine_->get_route(pair, amount_in);
    
    if (!route) {
        return "";
    }
    
    if (route->amount_out < amount_out_min) {
        return "";
    }
    
    Order order;
    order.pair = pair;
    order.amount_in = amount_in;
    order.amount_out_min = amount_out_min;
    order.side = Side::kSell;
    order.order_type = OrderType::kLimit;
    
    return order_executor_->execute_order(order, private_key);
}

TradingEngine::Stats TradingEngine::get_stats() const {
    return stats_;
}

} // namespace tigerswap