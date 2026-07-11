/**
 * TigerSwap Production Matching Engine - Implementation
 * Ultra-low latency C++ matching engine for high-frequency trading
 * 
 * @author TigerSwap
 * @version 1.0.0 Production
 */

#include "matching_engine.hpp"
#include <algorithm>
#include <cassert>
#include <chrono>
#include <iostream>
#include <sstream>

namespace tigerswap {

// ============================================================================
// OrderBook Implementation
// ============================================================================

OrderBook::OrderBook(uint64_t market_id) : market_id_(market_id) {}

OrderBook::~OrderBook() = default;

bool OrderBook::add_order(const Order& order) {
    std::unique_lock<std::shared_mutex> lock(mutex_);
    
    if (order.side == Side::BUY) {
        // Check if price level exists
        auto it = bid_tree_.find(order.price);
        if (it == bid_tree_.end()) {
            bid_tree_[order.price] = std::vector<std::shared_ptr<Order>>();
            bid_levels_[order.price] = 0;
        }
        
        auto new_order = std::make_shared<Order>(order);
        bid_tree_[order.price].push_back(new_order);
        bid_levels_[order.price] += order.quantity;
    } else {
        auto it = ask_tree_.find(order.price);
        if (it == ask_tree_.end()) {
            ask_tree_[order.price] = std::vector<std::shared_ptr<Order>>();
            ask_levels_[order.price] = 0;
        }
        
        auto new_order = std::make_shared<Order>(order);
        ask_tree_[order.price].push_back(new_order);
        ask_levels_[order.price] += order.quantity;
    }
    
    orders_[order.order_id] = std::make_shared<Order>(order);
    
    return true;
}

bool OrderBook::cancel_order(uint64_t order_id) {
    std::unique_lock<std::shared_mutex> lock(mutex_);
    
    auto it = orders_.find(order_id);
    if (it == orders_.end()) {
        return false;
    }
    
    const Order& order = *it->second;
    remove_from_levels(order);
    
    it->second->status = OrderStatus::CANCELLED;
    orders_.erase(it);
    
    return true;
}

bool OrderBook::modify_order(uint64_t order_id, uint64_t new_price, uint64_t new_quantity) {
    std::unique_lock<std::shared_mutex> lock(mutex_);
    
    auto it = orders_.find(order_id);
    if (it == orders_.end()) {
        return false;
    }
    
    Order& order = *it->second;
    if (order.status != OrderStatus::OPEN && order.status != OrderStatus::PENDING) {
        return false;
    }
    
    // Remove old price level
    remove_from_levels(order);
    
    // Update order
    order.price = new_price;
    order.quantity = new_quantity;
    order.remaining_quantity = new_quantity - order.filled_quantity;
    
    // Add to new price level
    add_to_levels(order);
    
    return true;
}

std::vector<Trade> OrderBook::match_order(const Order& order) {
    std::unique_lock<std::shared_mutex> lock(mutex_);
    
    if (order.side == Side::BUY) {
        return match_buy_order(order);
    } else {
        return match_sell_order(order);
    }
}

std::vector<Trade> OrderBook::match_buy_order(const Order& order) {
    std::vector<Trade> trades;
    
    if (ask_tree_.empty()) {
        return trades;
    }
    
    // Find matching asks (price <= order.price for buy)
    auto it = ask_tree_.begin();
    uint64_t remaining = order.quantity;
    
    while (it != ask_tree_.end() && remaining > 0 && it->first <= order.price) {
        auto& price_level = it->second;
        
        // Process orders at this price level
        for (auto& maker_order_ptr : price_level) {
            if (remaining == 0) break;
            
            Order& maker_order = *maker_order_ptr;
            if (maker_order.status != OrderStatus::OPEN) continue;
            
            uint64_t fill_quantity = std::min(remaining, maker_order.remaining_quantity);
            
            // Create trade
            Trade trade;
            trade.trade_id = 0;  // Will be assigned by engine
            trade.market_id = market_id_;
            trade.maker_order_id = maker_order.order_id;
            trade.taker_order_id = order.order_id;
            trade.price = it->first;  // Maker's price
            trade.quantity = fill_quantity;
            trade.timestamp = std::chrono::duration_cast<std::chrono::milliseconds>(
                std::chrono::system_clock::now().time_since_epoch()
            ).count();
            
            trades.push_back(trade);
            
            // Update orders
            maker_order.filled_quantity += fill_quantity;
            maker_order.remaining_quantity -= fill_quantity;
            remaining -= fill_quantity;
            
            if (maker_order.remaining_quantity == 0) {
                maker_order.status = OrderStatus::FILLED;
            } else {
                maker_order.status = OrderStatus::PARTIALLY_FILLED;
            }
            
            // Update levels
            ask_levels_[it->first] -= fill_quantity;
        }
        
        // Remove empty price levels
        if (ask_levels_[it->first] == 0) {
            ask_levels_.erase(it->first);
            it = ask_tree_.erase(it);
        } else {
            ++it;
        }
    }
    
    return trades;
}

std::vector<Trade> OrderBook::match_sell_order(const Order& order) {
    std::vector<Trade> trades;
    
    if (bid_tree_.empty()) {
        return trades;
    }
    
    // Find matching bids (price >= order.price for sell)
    auto it = bid_tree_.rbegin();
    uint64_t remaining = order.quantity;
    
    while (remaining > 0) {
        // Convert reverse iterator to normal
        if (it == bid_tree_.rend()) break;
        
        uint64_t price = it->first;
        if (price < order.price) break;
        
        auto& price_level = it->second;
        
        for (auto& maker_order_ptr : price_level) {
            if (remaining == 0) break;
            
            Order& maker_order = *maker_order_ptr;
            if (maker_order.status != OrderStatus::OPEN) continue;
            
            uint64_t fill_quantity = std::min(remaining, maker_order.remaining_quantity);
            
            Trade trade;
            trade.trade_id = 0;
            trade.market_id = market_id_;
            trade.maker_order_id = maker_order.order_id;
            trade.taker_order_id = order.order_id;
            trade.price = price;
            trade.quantity = fill_quantity;
            trade.timestamp = std::chrono::duration_cast<std::chrono::milliseconds>(
                std::chrono::system_clock::now().time_since_epoch()
            ).count();
            
            trades.push_back(trade);
            
            maker_order.filled_quantity += fill_quantity;
            maker_order.remaining_quantity -= fill_quantity;
            remaining -= fill_quantity;
            
            if (maker_order.remaining_quantity == 0) {
                maker_order.status = OrderStatus::FILLED;
            } else {
                maker_order.status = OrderStatus::PARTIALLY_FILLED;
            }
            
            bid_levels_[price] -= fill_quantity;
        }
        
        if (bid_levels_[price] == 0) {
            bid_levels_.erase(price);
            // Handle reverse iterator erasure
            auto it_erase = std::next(it).base();
            bid_tree_.erase(--it_erase);
            it = std::reverse_iterator(bid_tree_.rbegin());
        } else {
            ++it;
        }
    }
    
    return trades;
}

void OrderBook::remove_from_levels(const Order& order) {
    auto& levels = (order.side == Side::BUY) ? bid_levels_ : ask_levels_;
    auto& tree = (order.side == Side::BUY) ? bid_tree_ : ask_tree_;
    
    auto level_it = levels.find(order.price);
    if (level_it != levels.end()) {
        level_it->second -= order.remaining_quantity;
        if (level_it->second == 0) {
            levels.erase(level_it);
            tree.erase(order.price);
        }
    }
}

void OrderBook::add_to_levels(const Order& order) {
    auto& levels = (order.side == Side::BUY) ? bid_levels_ : ask_levels_;
    auto& tree = (order.side == Side::BUY) ? bid_tree_ : ask_tree_;
    
    levels[order.price] += order.remaining_quantity;
    tree[order.price].push_back(std::make_shared<Order>(order));
}

std::vector<PriceLevel> OrderBook::get_ask_levels(uint32_t limit) const {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    std::vector<PriceLevel> result;
    result.reserve(limit);
    
    uint32_t count = 0;
    for (const auto& [price, quantity] : ask_levels_) {
        if (count++ >= limit) break;
        
        PriceLevel level;
        level.price = price;
        level.quantity = quantity;
        level.timestamp = std::chrono::duration_cast<std::chrono::milliseconds>(
            std::chrono::system_clock::now().time_since_epoch()
        ).count();
        
        result.push_back(level);
    }
    
    return result;
}

std::vector<PriceLevel> OrderBook::get_bid_levels(uint32_t limit) const {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    std::vector<PriceLevel> result;
    result.reserve(limit);
    
    uint32_t count = 0;
    for (const auto& [price, quantity] : bid_levels_) {
        if (count++ >= limit) break;
        
        PriceLevel level;
        level.price = price;
        level.quantity = quantity;
        level.timestamp = std::chrono::duration_cast<std::chrono::milliseconds>(
            std::chrono::system_clock::now().time_since_epoch()
        ).count();
        
        result.push_back(level);
    }
    
    return result;
}

uint64_t OrderBook::get_best_bid() const {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    if (bid_levels_.empty()) return 0;
    
    return bid_levels_.rbegin()->first;  // Highest bid
}

uint64_t OrderBook::get_best_ask() const {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    if (ask_levels_.empty()) return 0;
    
    return ask_levels_.begin()->first;  // Lowest ask
}

uint64_t OrderBook::get_mid_price() const {
    uint64_t bid = get_best_bid();
    uint64_t ask = get_best_ask();
    
    if (bid == 0 || ask == 0) return 0;
    
    return (bid + ask) / 2;
}

uint64_t OrderBook::get_spread() const {
    uint64_t bid = get_best_bid();
    uint64_t ask = get_best_ask();
    
    if (bid == 0 || ask == 0) return 0;
    
    return ask - bid;
}

uint64_t OrderBook::get_depth(Side side, uint64_t price) const {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    const auto& levels = (side == Side::BUY) ? bid_levels_ : ask_levels_;
    
    uint64_t depth = 0;
    if (side == Side::BUY) {
        for (const auto& [p, q] : levels) {
            if (p >= price) depth += q;
        }
    } else {
        for (const auto& [p, q] : levels) {
            if (p <= price) depth += q;
        }
    }
    
    return depth;
}

uint64_t OrderBook::get_total_bid_volume() const {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    uint64_t total = 0;
    for (const auto& [price, quantity] : bid_levels_) {
        total += quantity;
    }
    return total;
}

uint64_t OrderBook::get_total_ask_volume() const {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    uint64_t total = 0;
    for (const auto& [price, quantity] : ask_levels_) {
        total += quantity;
    }
    return total;
}

uint64_t OrderBook::get_order_count(Side side) const {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    uint64_t count = 0;
    const auto& tree = (side == Side::BUY) ? bid_tree_ : ask_tree_;
    
    for (const auto& [price, orders] : tree) {
        for (const auto& order : orders) {
            if (order->status == OrderStatus::OPEN) {
                count++;
            }
        }
    }
    
    return count;
}

// ============================================================================
// MatchingEngine Implementation
// ============================================================================

MatchingEngine::MatchingEngine() : running_(false) {
    stats_.start_time = std::chrono::steady_clock::now();
}

MatchingEngine::~MatchingEngine() {
    stop();
}

bool MatchingEngine::create_market(const MarketConfig& config) {
    std::unique_lock<std::shared_mutex> lock(mutex_);
    
    if (markets_.find(config.market_id) != markets_.end()) {
        return false;
    }
    
    markets_[config.market_id] = config;
    order_books_[config.market_id] = std::make_unique<OrderBook>(config.market_id);
    
    return true;
}

bool MatchingEngine::update_market(const MarketConfig& config) {
    std::unique_lock<std::shared_mutex> lock(mutex_);
    
    auto it = markets_.find(config.market_id);
    if (it == markets_.end()) {
        return false;
    }
    
    it->second = config;
    return true;
}

bool MatchingEngine::delete_market(uint64_t market_id) {
    std::unique_lock<std::shared_mutex> lock(mutex_);
    
    auto it = markets_.find(market_id);
    if (it == markets_.end()) {
        return false;
    }
    
    markets_.erase(it);
    order_books_.erase(market_id);
    
    return true;
}

MarketConfig* MatchingEngine::get_market_config(uint64_t market_id) {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    auto it = markets_.find(market_id);
    if (it == markets_.end()) {
        return nullptr;
    }
    
    return &it->second;
}

uint64_t MatchingEngine::create_order(const Order& order) {
    std::unique_lock<std::shared_mutex> lock(mutex_);
    
    // Validate order
    if (!validate_order(order)) {
        stats_.rejected_orders++;
        return 0;
    }
    
    auto market = find_market(order.market_id);
    if (!market || market->status != MarketStatus::ACTIVE) {
        stats_.rejected_orders++;
        return 0;
    }
    
    // Generate order ID
    uint64_t order_id = next_order_id_.fetch_add(1);
    
    Order new_order = order;
    new_order.order_id = order_id;
    new_order.timestamp = std::chrono::duration_cast<std::chrono::milliseconds>(
        std::chrono::system_clock::now().time_since_epoch()
    ).count();
    new_order.status = OrderStatus::OPEN;
    new_order.remaining_quantity = new_order.quantity;
    
    // Add to order book
    auto book_it = order_books_.find(order.market_id);
    if (book_it == order_books_.end()) {
        stats_.rejected_orders++;
        return 0;
    }
    
    book_it->second->add_order(new_order);
    
    // Match order
    auto trades = book_it->second->match_order(new_order);
    
    // Process trades
    for (auto& trade : trades) {
        trade.trade_id = next_trade_id_.fetch_add(1);
        trade.block_number = stats_.last_block.load();
        
        stats_.total_trades++;
        stats_.total_volume += trade.quantity * trade.price;
        
        if (trade_callback_) {
            trade_callback_(trade);
        }
    }
    
    stats_.total_orders++;
    stats_.matched_orders++;
    
    if (order_callback_) {
        order_callback_(new_order);
    }
    
    return order_id;
}

bool MatchingEngine::cancel_order(uint64_t market_id, uint64_t order_id) {
    std::unique_lock<std::shared_mutex> lock(mutex_);
    
    auto book_it = order_books_.find(market_id);
    if (book_it == order_books_.end()) {
        return false;
    }
    
    bool result = book_it->second->cancel_order(order_id);
    
    if (result) {
        stats_.cancelled_orders++;
        
        auto order = get_order(market_id, order_id);
        if (order && order_callback_) {
            order_callback_(*order);
        }
    }
    
    return result;
}

bool MatchingEngine::modify_order(uint64_t market_id, uint64_t order_id, uint64_t new_price, uint64_t new_quantity) {
    std::unique_lock<std::shared_mutex> lock(mutex_);
    
    auto book_it = order_books_.find(market_id);
    if (book_it == order_books_.end()) {
        return false;
    }
    
    return book_it->second->modify_order(order_id, new_price, new_quantity);
}

Order* MatchingEngine::get_order(uint64_t market_id, uint64_t order_id) {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    auto book_it = order_books_.find(market_id);
    if (book_it == order_books_.end()) {
        return nullptr;
    }
    
    // This would need proper implementation to expose orders
    return nullptr;
}

std::vector<Order> MatchingEngine::get_open_orders(uint64_t market_id, uint64_t user_id) {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    std::vector<Order> result;
    
    auto book_it = order_books_.find(market_id);
    if (book_it == order_books_.end()) {
        return result;
    }
    
    // Implementation would iterate through order book
    return result;
}

std::vector<Order> MatchingEngine::get_order_history(uint64_t market_id, uint64_t user_id, uint32_t limit) {
    // Would need order history storage
    return {};
}

std::vector<Trade> MatchingEngine::get_trades(uint64_t market_id, uint64_t user_id, uint32_t limit) {
    // Would need trade history storage
    return {};
}

std::vector<Trade> MatchingEngine::get_recent_trades(uint64_t market_id, uint32_t limit) {
    // Would need trade history storage
    return {};
}

std::vector<PriceLevel> MatchingEngine::get_market_depth(uint64_t market_id, uint32_t limit) {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    auto book_it = order_books_.find(market_id);
    if (book_it == order_books_.end()) {
        return {};
    }
    
    auto bids = book_it->second->get_bid_levels(limit);
    auto asks = book_it->second->get_ask_levels(limit);
    
    // Combine and return
    std::vector<PriceLevel> result;
    result.insert(result.end(), bids.begin(), bids.end());
    result.insert(result.end(), asks.begin(), asks.end());
    
    return result;
}

uint64_t MatchingEngine::get_market_price(uint64_t market_id) {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    auto book_it = order_books_.find(market_id);
    if (book_it == order_books_.end()) {
        return 0;
    }
    
    return book_it->second->get_mid_price();
}

void MatchingEngine::start() {
    if (running_.exchange(true)) {
        return;  // Already running
    }
    
    // Create worker threads
    uint32_t num_threads = std::thread::hardware_concurrency();
    for (uint32_t i = 0; i < num_threads; ++i) {
        worker_threads_.emplace_back([this, i]() {
            // Worker thread for processing
            while (running_.load()) {
                std::this_thread::sleep_for(std::chrono::milliseconds(1));
            }
        });
    }
}

void MatchingEngine::stop() {
    if (!running_.exchange(false)) {
        return;  // Already stopped
    }
    
    // Wait for worker threads
    for (auto& thread : worker_threads_) {
        if (thread.joinable()) {
            thread.join();
        }
    }
    worker_threads_.clear();
}

bool MatchingEngine::validate_order(const Order& order) const {
    if (order.quantity < MIN_QUANTITY || order.quantity > MAX_QUANTITY) {
        return false;
    }
    
    if (order.price == 0) {
        if (order.order_type != OrderType::MARKET) {
            return false;
        }
    }
    
    if (order.order_type == OrderType::STOP_LOSS || order.order_type == OrderType::STOP_LIMIT) {
        if (order.expire_time == 0) {
            return false;
        }
    }
    
    return true;
}

MarketConfig* MatchingEngine::find_market(uint64_t market_id) {
    auto it = markets_.find(market_id);
    if (it == markets_.end()) {
        return nullptr;
    }
    return &it->second;
}

Order* MatchingEngine::find_order(uint64_t market_id, uint64_t order_id) {
    auto book_it = order_books_.find(market_id);
    if (book_it == order_books_.end()) {
        return nullptr;
    }
    
    // Would need to expose order lookup
    return nullptr;
}

void MatchingEngine::process_market(uint64_t market_id) {
    // Worker thread processing for market
}

// ============================================================================
// PriceCalculator Implementation
// ============================================================================

uint64_t PriceCalculator::calculate_sqrt_price(uint64_t amount_a, uint64_t amount_b) {
    if (amount_a == 0 || amount_b == 0) {
        return 0;
    }
    
    // Using Babylonian method for sqrt
    uint128_t product = static_cast<uint128_t>(amount_a) * FIXED_POINT_96 / amount_b;
    uint64_t x = static_cast<uint64_t>(product);
    uint64_t y = (x + product / x) >> 1;
    y = (y + product / y) >> 1;
    y = (y + product / y) >> 1;
    y = (y + product / y) >> 1;
    
    return y;
}

uint64_t PriceCalculator::get_sqrt_ratio_at_tick(int24 tick) {
    // Calculate sqrt ratio for a given tick
    // This is a simplified version - production would use full tick math
    uint128_t ratio = FIXED_POINT_96;
    
    int24 abs_tick = std::abs(tick);
    uint64_t multiplier = 1;
    
    // Using binary exponentiation for efficiency
    for (int i = 0; i < 10; ++i) {
        if (abs_tick & (1 << i)) {
            multiplier *= (10001ULL / 10000ULL);  // Approximation
        }
    }
    
    return static_cast<uint64_t>(ratio * multiplier);
}

int24 PriceCalculator::get_tick_at_sqrt_ratio(uint64_t ratio) {
    // Inverse of get_sqrt_ratio_at_tick
    // Simplified version
    return static_cast<int24>(std::log2(ratio) * 10000 / std::log2(1.0001));
}

uint64_t PriceCalculator::compute_swap_step(
    uint64_t current_price,
    uint64_t target_price,
    uint64_t liquidity,
    uint64_t amount_remaining,
    uint64_t fee_bps
) {
    bool zero_for_one = current_price > target_price;
    
    uint128_t sqrt_ratio_next;
    uint128_t amount_input;
    uint128_t amount_output;
    uint128_t fee_amount;
    
    // Apply fee
    uint128_t amount_remaining_less_fee = 
        static_cast<uint128_t>(amount_remaining) * (10000 - fee_bps) / 10000;
    
    // Calculate next sqrt price
    if (liquidity > 0) {
        uint128_t delta_y = amount_remaining_less_fee * liquidity;
        
        if (zero_for_one) {
            delta_y = delta_y * current_price / FIXED_POINT_96;
            sqrt_ratio_next = current_price - delta_y / liquidity;
            if (sqrt_ratio_next > target_price) {
                sqrt_ratio_next = target_price;
            }
        } else {
            sqrt_ratio_next = current_price + delta_y / liquidity;
            if (sqrt_ratio_next < target_price) {
                sqrt_ratio_next = target_price;
            }
        }
    } else {
        sqrt_ratio_next = target_price;
    }
    
    // Calculate amounts
    uint128_t price_delta = current_price - sqrt_ratio_next;
    
    if (zero_for_one) {
        amount_input = liquidity * price_delta / current_price * FIXED_POINT_96 / liquidity;
        amount_output = liquidity * price_delta / sqrt_ratio_next;
    } else {
        amount_input = liquidity * price_delta / FIXED_POINT_96;
        amount_output = liquidity * price_delta / current_price / sqrt_ratio_next;
    }
    
    fee_amount = static_cast<uint128_t>(amount_input) * fee_bps / 10000;
    
    return static_cast<uint64_t>(amount_output);
}

// ============================================================================
// RiskManager Implementation
// ============================================================================

RiskManager::RiskManager() : initial_margin_ratio_(10000), maintenance_margin_ratio_(5000) {}

bool RiskManager::check_order_risk(const Order& order, uint64_t account_balance) {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    // Check order size against max
    auto max_order_it = max_order_sizes_.find(order.market_id);
    if (max_order_it != max_order_sizes_.end()) {
        if (order.quantity > max_order_it->second) {
            return false;
        }
    }
    
    // Check position size
    auto max_pos_it = max_position_sizes_.find(order.market_id);
    if (max_pos_it != max_position_sizes_.end()) {
        auto pos_it = positions_.find(order.user_id);
        if (pos_it != positions_.end()) {
            uint64_t current_size = pos_it->second.size;
            uint64_t new_size = current_size + order.quantity;
            if (new_size > max_pos_it->second) {
                return false;
            }
        }
    }
    
    // Check account balance
    if (order.quantity * order.price > account_balance) {
        return false;
    }
    
    return true;
}

bool RiskManager::check_trade_risk(const Trade& trade, uint64_t account_balance) {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    uint64_t trade_value = trade.quantity * trade.price;
    
    if (trade_value > account_balance) {
        return false;
    }
    
    return true;
}

bool RiskManager::check_position_risk(uint64_t user_id, uint64_t market_id) {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    auto pos_it = positions_.find(user_id);
    if (pos_it == positions_.end()) {
        return true;  // No position, no risk
    }
    
    const Position& pos = pos_it->second;
    
    // Check maintenance margin
    if (pos.margin < maintenance_margin_ratio_) {
        return false;
    }
    
    return true;
}

void RiskManager::set_max_position_size(uint64_t market_id, uint64_t max_size) {
    std::unique_lock<std::shared_mutex> lock(mutex_);
    max_position_sizes_[market_id] = max_size;
}

void RiskManager::set_max_order_size(uint64_t market_id, uint64_t max_size) {
    std::unique_lock<std::shared_mutex> lock(mutex_);
    max_order_sizes_[market_id] = max_size;
}

void RiskManager::set_max_daily_volume(uint64_t market_id, uint64_t max_volume) {
    std::unique_lock<std::shared_mutex> lock(mutex_);
    max_daily_volumes_[market_id] = max_volume;
}

void RiskManager::set_initial_margin_ratio(uint64_t ratio) {
    std::unique_lock<std::shared_mutex> lock(mutex_);
    initial_margin_ratio_ = ratio;
}

void RiskManager::set_maintenance_margin_ratio(uint64_t ratio) {
    std::unique_lock<std::shared_mutex> lock(mutex_);
    maintenance_margin_ratio_ = ratio;
}

bool RiskManager::check_liquidation(uint64_t user_id, uint64_t market_id) {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    auto pos_it = positions_.find(user_id);
    if (pos_it == positions_.end()) {
        return false;
    }
    
    const Position& pos = pos_it->second;
    
    // Calculate margin ratio
    uint128_t margin_ratio = (static_cast<uint128_t>(pos.margin) * 10000) / 
                              (pos.size * pos.entry_price);
    
    return margin_ratio < maintenance_margin_ratio_;
}

std::vector<uint64_t> RiskManager::get_liquidatable_positions() {
    std::shared_lock<std::shared_mutex> lock(mutex_);
    
    std::vector<uint64_t> result;
    
    for (const auto& [user_id, position] : positions_) {
        uint128_t margin_ratio = (static_cast<uint128_t>(position.margin) * 10000) /
                                 (position.size * position.entry_price);
        
        if (margin_ratio < maintenance_margin_ratio_) {
            result.push_back(user_id);
        }
    }
    
    return result;
}

// ============================================================================
// TradeExecutor Implementation
// ============================================================================

TradeExecutor::TradeExecutor(MatchingEngine* engine, RiskManager* risk_manager)
    : engine_(engine), risk_manager_(risk_manager) {}

std::vector<Trade> TradeExecutor::execute_order(const Order& order, uint64_t account_balance) {
    // Check risk
    if (!risk_manager_->check_order_risk(order, account_balance)) {
        return {};
    }
    
    // Execute through matching engine
    // Note: This would need to properly handle the order execution
    return {};
}

std::vector<Trade> TradeExecutor::execute_batch(const std::vector<Order>& orders, uint64_t account_balance) {
    std::vector<Trade> all_trades;
    
    for (const auto& order : orders) {
        auto trades = execute_order(order, account_balance);
        all_trades.insert(all_trades.end(), trades.begin(), trades.end());
    }
    
    return all_trades;
}

bool TradeExecutor::settle_trade(const Trade& trade) {
    if (!validate_settlement(trade)) {
        return false;
    }
    
    return process_trade_settlement(trade);
}

bool TradeExecutor::settle_batch(const std::vector<Trade>& trades) {
    for (const auto& trade : trades) {
        if (!settle_trade(trade)) {
            return false;
        }
    }
    return true;
}

bool TradeExecutor::validate_settlement(const Trade& trade) {
    if (trade.quantity == 0 || trade.price == 0) {
        return false;
    }
    
    return true;
}

bool TradeExecutor::process_trade_settlement(const Trade& trade) {
    if (settlement_callback_) {
        return settlement_callback_(trade);
    }
    
    // Default settlement processing
    return true;
}

} // namespace tigerswap
