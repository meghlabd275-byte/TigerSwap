#include "trading_engine.hpp"
#include <algorithm>
#include <iostream>
#include <fstream>
#include <sstream>

namespace tigerswap {

// OrderBook implementation
OrderBook::OrderBook(uint64_t pair_id) : pair_id_(pair_id) {}

bool OrderBook::add_order(const Order& order) {
    std::unique_lock lock(mutex_);
    
    if (order.side == OrderSide::BUY) {
        bids_.add_order(order);
    } else {
        asks_.add_order(order);
    }
    
    return true;
}

bool OrderBook::cancel_order(uint64_t order_id) {
    std::unique_lock lock(mutex_);
    
    // Search in bids
    if (bids_.orders.find(order_id) != bids_.orders.end()) {
        bids_.remove_order(order_id);
        return true;
    }
    
    // Search in asks
    if (asks_.orders.find(order_id) != asks_.orders.end()) {
        asks_.remove_order(order_id);
        return true;
    }
    
    return false;
}

bool OrderBook::modify_order(uint64_t order_id, int64_t new_quantity) {
    std::unique_lock lock(mutex_);
    
    // Search in bids
    if (bids_.orders.find(order_id) != bids_.orders.end()) {
        bids_.modify_order(order_id, new_quantity);
        return true;
    }
    
    // Search in asks
    if (asks_.orders.find(order_id) != asks_.orders.end()) {
        asks_.modify_order(order_id, new_quantity);
        return true;
    }
    
    return false;
}

bool OrderBook::can_match(const Order& buy_order, const Order& sell_order) const {
    if (buy_order.side != OrderSide::BUY || sell_order.side != OrderSide::SELL) {
        return false;
    }
    
    // For market orders, always match
    if (buy_order.type == OrderType::MARKET || sell_order.type == OrderType::MARKET) {
        return true;
    }
    
    // For limit orders, price must cross
    return buy_order.price >= sell_order.price;
}

Trade OrderBook::create_trade(const Order& maker, const Order& taker, 
                               int64_t price, int64_t quantity) {
    Trade trade;
    trade.trade_id = ++last_trade_id_;
    trade.order_id = taker.order_id;
    trade.counter_order_id = maker.order_id;
    trade.maker_order_id = maker.order_id;
    trade.taker_order_id = taker.order_id;
    trade.price = price;
    trade.quantity = quantity;
    trade.executed_at = std::chrono::steady_clock::now();
    trade.side = static_cast<uint8_t>(taker.side);
    
    return trade;
}

std::vector<Trade> OrderBook::match_orders() {
    std::vector<Trade> trades;
    std::unique_lock lock(mutex_);
    
    while (true) {
        if (bids_.orders.empty() || asks_.orders.empty()) {
            break;
        }
        
        // Get best bid and ask
        auto best_bid = bids_.levels.empty() ? nullptr : &bids_.levels.front();
        auto best_ask = asks_.levels.empty() ? nullptr : &asks_.levels.front();
        
        if (!best_bid || !best_ask) break;
        
        // Check if they can match
        if (best_bid->price < best_ask->price) {
            break; // No crossing
        }
        
        // Execute trade at ask price (maker's price)
        int64_t trade_price = best_ask->price;
        int64_t trade_quantity = std::min(best_bid->quantity, best_ask->quantity);
        
        // Find and update orders
        // In production, this would iterate through order maps
        // For now, create a simple trade
        Trade trade;
        trade.trade_id = ++last_trade_id_;
        trade.price = trade_price;
        trade.quantity = trade_quantity;
        trade.executed_at = std::chrono::steady_clock::now();
        
        trades.push_back(trade);
        
        // Update quantities (simplified)
        best_bid->quantity -= trade_quantity;
        best_ask->quantity -= trade_quantity;
        
        // Remove empty levels
        if (best_bid->quantity <= 0) {
            bids_.levels.erase(bids_.levels.begin());
        }
        if (best_ask->quantity <= 0) {
            asks_.levels.erase(asks_.levels.begin());
        }
    }
    
    return trades;
}

int64_t OrderBook::get_best_bid() const {
    std::shared_lock lock(mutex_);
    if (bids_.levels.empty()) return 0;
    return bids_.levels.front().price;
}

int64_t OrderBook::get_best_ask() const {
    std::shared_lock lock(mutex_);
    if (asks_.levels.empty()) return 0;
    return asks_.levels.front().price;
}

int64_t OrderBook::get_mid_price() const {
    int64_t bid = get_best_bid();
    int64_t ask = get_best_ask();
    if (bid == 0 || ask == 0) return 0;
    return (bid + ask) / 2;
}

int64_t OrderBook::get_spread() const {
    return get_best_ask() - get_best_bid();
}

MarketData OrderBook::get_market_data() const {
    std::shared_lock lock(mutex_);
    
    MarketData data;
    data.pair_id = pair_id_;
    data.bid_price = get_best_bid();
    data.ask_price = get_best_ask();
    data.last_price = get_mid_price();
    data.timestamp = std::chrono::steady_clock::now();
    
    return data;
}

// OrderBookSide implementation
void OrderBookSide::add_order(const Order& order) {
    std::unique_lock lock(mutex_);
    orders[order.order_id] = order;
    
    // Add or update level
    bool found = false;
    for (auto& level : levels) {
        if (level.price == order.price) {
            level.quantity += order.quantity;
            level.order_count++;
            found = true;
            break;
        }
    }
    
    if (!found) {
        Level level;
        level.price = order.price;
        level.quantity = order.quantity;
        level.order_count = 1;
        
        // Insert in sorted order
        auto it = std::lower_bound(levels.begin(), levels.end(), level,
            [](const Level& a, const Level& b) { return a.price > b.price; });
        levels.insert(it, level);
    }
}

void OrderBookSide::remove_order(uint64_t order_id) {
    std::unique_lock lock(mutex_);
    
    auto it = orders.find(order_id);
    if (it == orders.end()) return;
    
    const Order& order = it->second;
    
    // Update level
    for (auto& level : levels) {
        if (level.price == order.price) {
            level.quantity -= order.quantity;
            level.order_count--;
            if (level.quantity <= 0) {
                levels.erase(std::remove_if(levels.begin(), levels.end(),
                    [&level](const Level& l) { return l.price == level.price; }), levels.end());
            }
            break;
        }
    }
    
    orders.erase(it);
}

void OrderBookSide::modify_order(uint64_t order_id, int64_t new_quantity) {
    std::unique_lock lock(mutex_);
    
    auto it = orders.find(order_id);
    if (it == orders.end()) return;
    
    Order& order = it->second;
    int64_t old_quantity = order.quantity;
    order.quantity = new_quantity;
    order.remaining = new_quantity;
    
    // Update level
    for (auto& level : levels) {
        if (level.price == order.price) {
            level.quantity = level.quantity - old_quantity + new_quantity;
            break;
        }
    }
}

std::vector<Level> OrderBookSide::get_top_levels(size_t count) const {
    std::shared_lock lock(mutex_);
    size_t size = std::min(count, levels.size());
    return std::vector<Level>(levels.begin(), levels.begin() + size);
}

// OrderBookManager implementation
OrderBookManager& OrderBookManager::instance() {
    static OrderBookManager instance;
    return instance;
}

OrderBook* OrderBookManager::get_or_create_orderbook(uint64_t pair_id) {
    std::unique_lock lock(mutex_);
    
    auto it = orderbooks_.find(pair_id);
    if (it != orderbooks_.end()) {
        return it->second.get();
    }
    
    auto book = std::make_unique<OrderBook>(pair_id);
    OrderBook* ptr = book.get();
    orderbooks_[pair_id] = std::move(book);
    
    return ptr;
}

OrderBook* OrderBookManager::get_orderbook(uint64_t pair_id) {
    std::shared_lock lock(mutex_);
    
    auto it = orderbooks_.find(pair_id);
    if (it != orderbooks_.end()) {
        return it->second.get();
    }
    
    return nullptr;
}

void OrderBookManager::remove_orderbook(uint64_t pair_id) {
    std::unique_lock lock(mutex_);
    orderbooks_.erase(pair_id);
}

void OrderBookManager::set_trading_pair(const TradingPair& pair) {
    std::unique_lock lock(mutex_);
    trading_pairs_[pair.pair_id] = pair;
    
    // Create orderbook if not exists
    if (orderbooks_.find(pair.pair_id) == orderbooks_.end()) {
        orderbooks_[pair.pair_id] = std::make_unique<OrderBook>(pair.pair_id);
    }
}

const TradingPair* OrderBookManager::get_trading_pair(uint64_t pair_id) const {
    std::shared_lock lock(mutex_);
    
    auto it = trading_pairs_.find(pair_id);
    if (it != trading_pairs_.end()) {
        return &it->second;
    }
    
    return nullptr;
}

std::vector<uint64_t> OrderBookManager::get_active_pairs() const {
    std::shared_lock lock(mutex_);
    
    std::vector<uint64_t> pairs;
    pairs.reserve(trading_pairs_.size());
    
    for (const auto& pair : trading_pairs_) {
        pairs.push_back(pair.first);
    }
    
    return pairs;
}

size_t OrderBookManager::get_orderbook_count() const {
    std::shared_lock lock(mutex_);
    return orderbooks_.size();
}

} // namespace tigerswap
