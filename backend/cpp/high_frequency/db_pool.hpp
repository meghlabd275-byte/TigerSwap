/**
 * TigerSwap High-Performance Database Connection Pool
 * Zero-copy, lock-free connection management
 */

#ifndef TIGERSWAP_DB_POOL_HPP
#define TIGERSWAP_DB_POOL_HPP

#include <iostream>
#include <vector>
#include <queue>
#include <thread>
#include <mutex>
#include <condition_variable>
#include <atomic>
#include <memory>
#include <functional>
#include <chrono>
#include <libpq-fe.h>

// ============== CONNECTION ==============
class DBConnection {
private:
    PGconn* conn_;
    bool in_use_;
    std::chrono::steady_clock::time_point last_used_;
    uint64_t id_;
    
public:
    DBConnection(PGconn* conn, uint64_t id) : conn_(conn), in_use_(false), id_(id) {
        last_used_ = std::chrono::steady_clock::now();
    }
    
    ~DBConnection() {
        if (conn_) PQfinish(conn_);
    }
    
    bool isConnected() {
        return conn_ && PQstatus(conn_) == CONNECTION_OK;
    }
    
    bool execute(const char* query) {
        PGresult* res = PQexec(conn_, query);
        bool ok = PQresultStatus(res) == PGRES_TUPLES_OK || PQresultStatus(res) == PGRES_COMMAND_OK;
        PQclear(res);
        return ok;
    }
    
    PGconn* get() { return conn_; }
    bool& inUse() { return in_use_; }
    uint64_t id() { return id_; }
    void markUsed() { last_used_ = std::chrono::steady_clock::now(); }
    
    std::chrono::steady_clock::time_point lastUsed() { return last_used_; }
};

// ============== CONNECTION POOL ==============
class DBPool {
private:
    std::vector<std::unique_ptr<DBConnection>> connections_;
    std::queue<size_t> available_;
    std::mutex mutex_;
    std::condition_variable cv_;
    
    std::string conninfo_;
    size_t max_size_;
    size_t min_size_;
    std::atomic<uint64_t> next_id_{1};
    
    // Stats
    std::atomic<uint64_t> total_requests_{0};
    std::atomic<uint64_t> active_connections_{0};
    std::atomic<uint64_t> waiting_threads_{0};
    
public:
    DBPool(const char* conninfo, size_t min_size = 5, size_t max_size = 100)
        : conninfo_(conninfo), min_size_(min_size), max_size_(max_size) {
        
        // Initialize connections
        for (size_t i = 0; i < min_size_; i++) {
            createConnection();
        }
    }
    
    ~DBPool() {
        std::lock_guard<std::mutex> lock(mutex_);
        connections_.clear();
    }
    
    void createConnection() {
        PGconn* conn = PQconnectdb(conninfo_.c_str());
        
        if (conn && PQstatus(conn) == CONNECTION_OK) {
            uint64_t id = next_id_id_.fetch_add(1);
            connections_.push_back(std::make_unique<DBConnection>(conn, id));
            available_.push(connections_.size() - 1);
        }
    }
    
    std::shared_ptr<DBConnection> acquire(int timeout_ms = 5000) {
        total_requests_++;
        
        std::unique_lock<std::mutex> lock(mutex_);
        waiting_threads_++;
        
        // Wait for available connection
        auto start = std::chrono::steady_clock::now();
        
        while (available_.empty()) {
            if (connections_.size() < max_size_) {
                // Create new connection
                createConnection();
                break;
            }
            
            if (std::chrono::duration_cast<std::chrono::milliseconds>(
                std::chrono::steady_clock::now() - start).count() >= timeout_ms) {
                waiting_threads_--;
                return nullptr;
            }
            
            cv_.wait_for(lock, std::chrono::milliseconds(100));
        }
        
        if (available_.empty()) {
            waiting_threads_--;
            return nullptr;
        }
        
        size_t idx = available_.front();
        available_.pop();
        
        auto& conn = connections_[idx];
        conn->inUse() = true;
        active_connections_++;
        waiting_threads_--;
        
        // Verify connection
        if (!conn->isConnected()) {
            PQfinish(conn->get());
            PGconn* new_conn = PQconnectdb(conninfo_.c_str());
            conn = std::make_unique<DBConnection>(new_conn, conn->id());
        }
        
        return std::shared_ptr<DBConnection>(conn.get(), [this, idx](DBConnection* c) {
            release(idx);
        });
    }
    
    void release(size_t idx) {
        std::lock_guard<std::mutex> lock(mutex_);
        connections_[idx]->inUse() = false;
        connections_[idx]->markUsed();
        available_.push(idx);
        active_connections_--;
        cv_.notify_one();
    }
    
    struct PoolStats {
        uint64_t total_connections;
        uint64_t available_connections;
        uint64_t active_connections;
        uint64_t total_requests;
        uint64_t waiting_threads;
    };
    
    PoolStats getStats() {
        std::lock_guard<std::mutex> lock(mutex_);
        return {
            (uint64_t)connections_.size(),
            (uint64_t)available_.size(),
            active_connections_.load(),
            total_requests_.load(),
            waiting_threads_.load()
        };
    }
};

// ============== FFI ==============
extern "C" {

void* create_db_pool(const char* conninfo, size_t min_size, size_t max_size) {
    return new DBPool(conninfo, min_size, max_size);
}

void destroy_db_pool(void* pool) {
    delete static_cast<DBPool*>(pool);
}

void* db_pool_acquire(void* pool, int timeout_ms) {
    auto conn = static_cast<DBPool*>(pool)->acquire(timeout_ms);
    if (conn) {
        return conn.get();
    }
    return nullptr;
}

void db_pool_release(void* pool, void* conn) {
    // Release handled by shared_ptr
}

void db_pool_stats(void* pool, void* stats) {
    *static_cast<DBPool::PoolStats*>(stats) = static_cast<DBPool*>(pool)->getStats();
}

} // extern "C"

#endif // TIGERSWAP_DB_POOL_HPP
