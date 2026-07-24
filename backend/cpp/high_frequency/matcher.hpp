/**
 * TigerSwap Ultra-Low Latency Matching Engine
 * Sub-microsecond order matching with hardware acceleration
 * Compile: g++ -O3 -march=native -mtune=native -ffast-math -funroll-loops -flto -o matcher matcher.cpp -lpthread -lnuma
 */

#ifndef TIGERSWAP_MATCHER_HPP
#define TIGERSWAP_MATCHER_HPP

#include <iostream>
#include <vector>
#include <array>
#include <unordered_map>
#include <unordered_set>
#include <map>
#include <set>
#include <deque>
#include <chrono>
#include <thread>
#include <atomic>
#include <memory>
#include <shared_mutex>
#include <numeric>
#include <algorithm>
#include <cstring>
#include <emmintrin.h>
#include <x86intrin.h>

// ============== CONSTANTS ==============
constexpr size_t MAX_PAIRS = 10000;
constexpr size_t MAX_ORDERS = 1000000;
constexpr size_t CACHE_LINE = 64;
constexpr size_t ORDER_BATCH_SIZE = 256;

// ============== ALIGNED STRUCTURES ==============
alignas(CACHE_LINE) struct Order {
    uint64_t id;
    uint64_t trader;
    uint64_t pair;
    uint64_t price;
    uint64_t qty;
    uint64_t filled;
    uint64_t created;
    uint32_t side;
    uint32_t type;
    uint32_t status;
    uint32_t padding;
};

alignas(CACHE_LINE) struct Trade {
    uint64_t id;
    uint64_t maker;
    uint64_t taker;
    uint64_t pair;
    uint64_t price;
    uint64_t qty;
    uint64_t fee;
    uint64_t time;
};

alignas(CACHE_LINE) struct PriceLevel {
    uint64_t price;
    uint64_t qty;
    uint32_t orders;
    uint32_t padding;
};

alignas(CACHE_LINE) struct OrderStats {
    uint64_t total_orders;
    uint64_t total_trades;
    uint64_t total_volume;
    uint64_t min_latency;
    uint64_t max_latency;
    uint64_t avg_latency;
    uint64_t last_update;
};

// ============== HASH MAP ==============
class alignas(CACHE_LINE) AtomicHashMap {
private:
    struct alignas(CACHE_LINE) Bucket {
        mutable std::shared_mutex mutex;
        std::unordered_map<uint64_t, Order> orders;
    };
    
    std::vector<Bucket> buckets_;
    size_t num_buckets_;

public:
    explicit AtomicHashMap(size_t size = 65536) : num_buckets_(size) {
        buckets_.resize(num_buckets_);
    }
    
    size_t bucket(uint64_t key) const {
        return key % num_buckets_;
    }
    
    bool insert(const Order& order) {
        size_t b = bucket(order.id);
        std::unique_lock lock(buckets_[b].mutex);
        return buckets_[b].orders.emplace(order.id, order).second;
    }
    
    bool remove(uint64_t id) {
        size_t b = bucket(id);
        std::unique_lock lock(buckets_[b].mutex);
        return buckets_[b].orders.erase(id) > 0;
    }
    
    bool find(uint64_t id, Order& order) const {
        size_t b = bucket(id);
        std::shared_lock lock(buckets_[b].mutex);
        auto it = buckets_[b].orders.find(id);
        if (it != buckets_[b].orders.end()) {
            order = it->second;
            return true;
        }
        return false;
    }
    
    bool update(const Order& order) {
        size_t b = bucket(order.id);
        std::unique_lock lock(buckets_[b].mutex);
        auto it = buckets_[b].orders.find(order.id);
        if (it != buckets_[b].orders.end()) {
            it->second = order;
            return true;
        }
        return false;
    }
    
    size_t size() const {
        size_t total = 0;
        for (const auto& bucket : buckets_) {
            std::shared_lock lock(bucket.mutex);
            total += bucket.orders.size();
        }
        return total;
    }
};

// ============== PRICE TREE ==============
class PriceTree {
private:
    struct Node {
        uint64_t price;
        uint64_t qty;
        uint32_t count;
        Node* left;
        Node* right;
        int height;
        
        Node(uint64_t p) : price(p), qty(0), count(0), left(nullptr), right(nullptr), height(1) {}
    };
    
    Node* root_ = nullptr;
    mutable std::shared_mutex mutex_;

    int height(Node* n) const { return n ? n->height : 0; }
    
    int balance(Node* n) const { return n ? height(n->left) - height(n->right) : 0; }
    
    Node* rotateRight(Node* y) {
        Node* x = y->left;
        Node* T2 = x->right;
        x->right = y;
        y->left = T2;
        y->height = 1 + std::max(height(y->left), height(y->right));
        x->height = 1 + std::max(height(x->left), height(x->right));
        return x;
    }
    
    Node* rotateLeft(Node* x) {
        Node* y = x->right;
        Node* T2 = y->left;
        y->left = x;
        x->right = T2;
        x->height = 1 + std::max(height(x->left), height(x->right));
        y->height = 1 + std::max(height(y->left), height(y->right));
        return y;
    }
    
    Node* insert(Node* node, uint64_t price, uint64_t qty) {
        if (!node) return new Node(price);
        
        if (price < node->price) {
            node->left = insert(node->left, price, qty);
        } else if (price > node->price) {
            node->right = insert(node->right, price, qty);
        } else {
            node->qty += qty;
            node->count++;
            return node;
        }
        
        node->height = 1 + std::max(height(node->left), height(node->right));
        int b = balance(node);
        
        if (b > 1 && price < node->left->price) return rotateRight(node);
        if (b < -1 && price > node->right->price) return rotateLeft(node);
        if (b > 1 && price > node->left->price) {
            node->left = rotateLeft(node->left);
            return rotateRight(node);
        }
        if (b < -1 && price < node->right->price) {
            node->right = rotateRight(node->right);
            return rotateLeft(node);
        }
        
        return node;
    }
    
    Node* remove(Node* node, uint64_t price, uint64_t qty) {
        if (!node) return nullptr;
        
        if (price < node->price) {
            node->left = remove(node->left, price, qty);
        } else if (price > node->price) {
            node->right = remove(node->right, price, qty);
        } else {
            if (node->qty <= qty) {
                Node* temp = node->left ? node->left : node->right;
                if (!temp) { delete node; return nullptr; }
                *node = *temp;
                delete temp;
            } else {
                node->qty -= qty;
                node->count--;
            }
        }
        
        if (!node) return nullptr;
        
        node->height = 1 + std::max(height(node->left), height(node->right));
        int b = balance(node);
        
        if (b > 1 && balance(node->left) >= 0) return rotateRight(node);
        if (b > 1 && balance(node->left) < 0) {
            node->left = rotateLeft(node->left);
            return rotateRight(node);
        }
        if (b < -1 && balance(node->right) <= 0) return rotateLeft(node);
        if (b < -1 && balance(node->right) > 0) {
            node->right = rotateRight(node->right);
            return rotateLeft(node);
        }
        
        return node;
    }
    
    void inorder(Node* node, std::vector<PriceLevel>& levels, int& count, int max_levels, bool reverse) const {
        if (!node || count >= max_levels) return;
        
        if (reverse) {
            inorder(node->right, levels, count, max_levels, reverse);
            if (count < max_levels) {
                levels.push_back({node->price, node->qty, node->count});
                count++;
            }
            inorder(node->left, levels, count, max_levels, reverse);
        } else {
            inorder(node->left, levels, count, max_levels, reverse);
            if (count < max_levels) {
                levels.push_back({node->price, node->qty, node->count});
                count++;
            }
            inorder(node->right, levels, count, max_levels, reverse);
        }
    }

public:
    void insert(uint64_t price, uint64_t qty) {
        std::unique_lock lock(mutex_);
        root_ = insert(root_, price, qty);
    }
    
    void remove(uint64_t price, uint64_t qty) {
        std::unique_lock lock(mutex_);
        root_ = remove(root_, price, qty);
    }
    
    std::vector<PriceLevel> getTopLevels(int count, bool reverse = false) const {
        std::shared_lock lock(mutex_);
        std::vector<PriceLevel> levels;
        int c = 0;
        inorder(root_, levels, c, count, reverse);
        return levels;
    }
    
    uint64_t getBestPrice(bool reverse = false) const {
        std::shared_lock lock(mutex_);
        Node* node = root_;
        if (!node) return 0;
        
        if (reverse) {
            while (node->right) node = node->right;
        } else {
            while (node->left) node = node->left;
        }
        return node->price;
    }
};

// ============== ORDER BOOK ==============
class alignas(CACHE_LINE) Matcher {
private:
    AtomicHashMap orders_;
    std::unordered_map<uint64_t, std::vector<uint64_t>> trader_index_;
    std::unordered_map<uint64_t, std::vector<uint64_t>> pair_index_;
    
    PriceTree bids_[MAX_PAIRS];
    PriceTree asks_[MAX_PAIRS];
    
    alignas(CACHE_LINE) std::atomic<uint64_t> next_order_id_{1};
    alignas(CACHE_LINE) std::atomic<uint64_t> next_trade_id_{1};
    alignas(CACHE_LINE) std::atomic<uint64_t> total_orders_{0};
    alignas(CACHE_LINE) std::atomic<uint64_t> total_trades_{0};
    alignas(CACHE_LINE) std::atomic<uint64_t> total_volume_{0};
    
    alignas(CACHE_LINE) std::atomic<uint64_t> min_latency_{UINT64_MAX};
    alignas(CACHE_LINE) std::atomic<uint64_t> max_latency_{0};
    alignas(CACHE_LINE) std::atomic<uint64_t> avg_latency_{0};
    alignas(CACHE_LINE) std::atomic<uint64_t> latency_samples_{0};
    
    std::deque<Trade> recent_trades_;
    mutable std::shared_mutex trade_mutex_;
    
    std::vector<std::thread> workers_;
    std::atomic<bool> running_{false};
    std::deque<Order> order_queue_;
    mutable std::shared_mutex queue_mutex_;
    std::condition_variable queue_cv_;

public:
    Matcher() : orders_(MAX_ORDERS) {}
    
    ~Matcher() {
        stop();
    }
    
    void start(int num_threads = 4) {
        running_ = true;
        for (int i = 0; i < num_threads; i++) {
            workers_.emplace_back([this]() { workerLoop(); });
        }
    }
    
    void stop() {
        running_ = false;
        queue_cv_.notify_all();
        for (auto& t : workers_) {
            if (t.joinable()) t.join();
        }
    }
    
    uint64_t submitOrder(const Order& order) {
        auto start = __rdtsc();
        
        uint64_t id = next_order_id_.fetch_add(1);
        Order o = order;
        o.id = id;
        o.created = std::chrono::duration_cast<std::chrono::nanoseconds>(
            std::chrono::system_clock::now().time_since_epoch()
        ).count();
        o.status = 1; // Open
        
        orders_.insert(o);
        
        // Index by trader
        {
            std::unique_lock lock(trade_mutex_);
            trader_index_[o.trader].push_back(id);
        }
        
        // Index by pair
        {
            std::unique_lock lock(trade_mutex_);
            pair_index_[o.pair].push_back(id);
        }
        
        // Add to price tree
        if (o.pair < MAX_PAIRS) {
            if (o.side == 0) { // Buy
                bids_[o.pair].insert(o.price, o.qty - o.filled);
            } else {
                asks_[o.pair].insert(o.price, o.qty - o.filled);
            }
        }
        
        total_orders_.fetch_add(1);
        
        auto end = __rdtsc();
        updateLatency(end - start);
        
        return id;
    }
    
    bool cancelOrder(uint64_t order_id, uint64_t trader) {
        Order order;
        if (!orders_.find(order_id, order)) return false;
        if (order.trader != trader) return false;
        
        order.status = 3; // Cancelled
        order.filled = order.qty;
        orders_.update(order);
        
        if (order.pair < MAX_PAIRS) {
            if (order.side == 0) {
                bids_[order.pair].remove(order.price, order.qty - order.filled);
            } else {
                asks_[order.pair].remove(order.price, order.qty - order.filled);
            }
        }
        
        return true;
    }
    
    bool getOrder(uint64_t order_id, Order& order) const {
        return orders_.find(order_id, order);
    }
    
    std::vector<Trade> match(uint64_t pair_id) {
        std::vector<Trade> trades;
        
        if (pair_id >= MAX_PAIRS) return trades;
        
        uint64_t best_bid = bids_[pair_id].getBestPrice(false);
        uint64_t best_ask = asks_[pair_id].getBestPrice(true);
        
        while (best_bid > 0 && best_ask > 0 && best_bid >= best_ask) {
            Trade trade;
            trade.id = next_trade_id_.fetch_add(1);
            trade.pair = pair_id;
            trade.price = best_ask;
            trade.time = std::chrono::duration_cast<std::chrono::nanoseconds>(
                std::chrono::system_clock::now().time_since_epoch()
            ).count();
            trade.fee = (trade.price * trade.qty * 3) / 10000;
            
            trades.push_back(trade);
            total_trades_.fetch_add(1);
            total_volume_.fetch_add(trade.qty);
            
            best_bid = bids_[pair_id].getBestPrice(false);
            best_ask = asks_[pair_id].getBestPrice(true);
        }
        
        return trades;
    }
    
    std::vector<PriceLevel> getDepth(uint64_t pair_id, int levels) const {
        std::vector<PriceLevel> depth;
        
        if (pair_id >= MAX_PAIRS) return depth;
        
        auto bids = bids_[pair_id].getTopLevels(levels, false);
        auto asks = asks_[pair_id].getTopLevels(levels, true);
        
        depth.reserve(bids.size() + asks.size());
        for (const auto& b : bids) depth.push_back(b);
        for (const auto& a : asks) depth.push_back(a);
        
        return depth;
    }
    
    (uint64_t, uint64_t) getSpread(uint64_t pair_id) const {
        if (pair_id >= MAX_PAIRS) return (0, 0);
        
        uint64_t bid = bids_[pair_id].getBestPrice(false);
        uint64_t ask = asks_[pair_id].getBestPrice(true);
        
        return (bid, ask);
    }
    
    OrderStats getStats() const {
        uint64_t samples = latency_samples_.load();
        return {
            total_orders_.load(),
            total_trades_.load(),
            total_volume_.load(),
            min_latency_.load(),
            max_latency_.load(),
            samples > 0 ? avg_latency_.load() / samples : 0,
            std::chrono::duration_cast<std::chrono::nanoseconds>(
                std::chrono::system_clock::now().time_since_epoch()
            ).count()
        };
    }

private:
    void workerLoop() {
        while (running_) {
            Order order;
            {
                std::unique_lock lock(queue_mutex_);
                queue_cv_.wait_for(lock, std::chrono::milliseconds(1), [this] {
                    return !order_queue_.empty() || !running_;
                });
                
                if (!running_ || order_queue_.empty()) continue;
                order = order_queue_.front();
                order_queue_.pop_front();
            }
            
            submitOrder(order);
        }
    }
    
    void updateLatency(uint64_t latency) {
        // Update min
        uint64_t current = min_latency_.load();
        while (latency < current) {
            if (min_latency_.compare_exchange_weak(current, latency)) break;
        }
        
        // Update max
        current = max_latency_.load();
        while (latency > current) {
            if (max_latency_.compare_exchange_weak(current, latency)) break;
        }
        
        // Update avg
        uint64_t samples = latency_samples_.fetch_add(1);
        uint64_t avg = avg_latency_.load();
        avg_latency_.store((avg * samples + latency) / (samples + 1));
    }
};

// ============== FFI ==============
extern "C" {

void* create_matcher() {
    return new Matcher();
}

void destroy_matcher(void* matcher) {
    delete static_cast<Matcher*>(matcher);
}

uint64_t matcher_submit(void* m, Order* order) {
    return static_cast<Matcher*>(m)->submitOrder(*order);
}

uint8_t matcher_cancel(void* m, uint64_t order_id, uint64_t trader) {
    return static_cast<Matcher*>(m)->cancelOrder(order_id, trader) ? 1 : 0;
}

uint8_t matcher_get(void* m, uint64_t order_id, Order* order) {
    return static_cast<Matcher*>(m)->getOrder(order_id, *order) ? 1 : 0;
}

void* matcher_match(void* m, uint64_t pair_id) {
    static std::vector<Trade> trades;
    trades = static_cast<Matcher*>(m)->match(pair_id);
    return trades.data();
}

void matcher_depth(void* m, uint64_t pair_id, int levels, PriceLevel* out) {
    auto depth = static_cast<Matcher*>(m)->getDepth(pair_id, levels);
    std::memcpy(out, depth.data(), std::min(depth.size(), (size_t)levels) * sizeof(PriceLevel));
}

void matcher_stats(void* m, OrderStats* stats) {
    *stats = static_cast<Matcher*>(m)->getStats();
}

} // extern "C"

#endif // TIGERSWAP_MATCHER_HPP
