/**
 * TigerSwap High-Performance TCP/UDP Networking
 * Ultra-low latency network stack with DPDK-like performance
 */

#ifndef TIGERSWAP_NETWORK_HPP
#define TIGERSWAP_NETWORK_HPP

#include <iostream>
#include <vector>
#include <array>
#include <queue>
#include <thread>
#include <atomic>
#include <mutex>
#include <condition_variable>
#include <chrono>
#include <cstring>
#include <unistd.h>
#include <sys/socket.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <arpa/inet.h>
#include <fcntl.h>
#include <errno.h>
#include <poll.h>

// ============== CONSTANTS ==============
constexpr size_t MAX_CONNECTIONS = 100000;
constexpr size_t SEND_BUFFER_SIZE = 256 * 1024;
constexpr size_t RECV_BUFFER_SIZE = 256 * 1024;
constexpr size_t MAX_PACKET_SIZE = 65536;
constexpr int MAX_EVENTS = 1024;

// ============== PACKET ==============
#pragma pack(push, 1)
struct Packet {
    uint32_t length;
    uint32_t type;
    uint64_t timestamp;
    uint8_t data[];
};
#pragma pack(pop)

struct Connection {
    int fd;
    uint64_t id;
    uint64_t last_active;
    uint32_t requests;
    uint64_t bytes_in;
    uint64_t bytes_out;
    bool registered;
};

// ============== TCP SERVER ==============
class TCPServer {
private:
    int server_fd_;
    int epoll_fd_;
    bool running_;
    std::vector<std::thread> worker_threads_;
    
    std::array<Connection, MAX_CONNECTIONS> connections_;
    std::atomic<uint64_t> next_conn_id_{1};
    std::atomic<uint64_t> active_connections_{0};
    
    // Statistics
    std::atomic<uint64_t> total_requests_{0};
    std::atomic<uint64_t> total_bytes_in_{0};
    std::atomic<uint64_t> total_bytes_out_{0};
    
    // Callbacks
    std::function<void(Connection&, const uint8_t*, size_t)> on_message_;
    std::function<void(Connection&)> on_connect_;
    std::function<void(Connection&)> on_disconnect_;
    
public:
    TCPServer() : server_fd_(-1), epoll_fd_(-1), running_(false) {}
    
    ~TCPServer() { stop(); }
    
    bool start(const char* ip, int port, int workers = 4) {
        // Create server socket
        server_fd_ = socket(AF_INET, SOCK_STREAM, 0);
        if (server_fd_ < 0) return false;
        
        // Set socket options
        int opt = 1;
        setsockopt(server_fd_, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));
        setsockopt(server_fd_, SOL_SOCKET, SO_REUSEPORT, &opt, sizeof(opt));
        setsockopt(server_fd_, IPPROTO_TCP, TCP_NODELAY, &opt, sizeof(opt));
        
        // Set send/recv buffers
        setsockopt(server_fd_, SOL_SOCKET, SO_SNDBUF, &(int){SEND_BUFFER_SIZE}, sizeof(int));
        setsockopt(server_fd_, SOL_SOCKET, SO_RCVBUF, &(int){RECV_BUFFER_SIZE}, sizeof(int));
        
        // Bind
        struct sockaddr_in addr;
        memset(&addr, 0, sizeof(addr));
        addr.sin_family = AF_INET;
        addr.sin_port = htons(port);
        inet_pton(AF_INET, ip, &addr.sin_addr);
        
        if (bind(server_fd_, (struct sockaddr*)&addr, sizeof(addr)) < 0) return false;
        
        // Listen
        if (listen(server_fd_, SOMAXCONN) < 0) return false;
        
        // Set non-blocking
        int flags = fcntl(server_fd_, F_GETFL, 0);
        fcntl(server_fd_, F_SETFL, flags | O_NONBLOCK);
        
        // Create epoll
        epoll_fd_ = epoll_create1(0);
        if (epoll_fd_ < 0) return false;
        
        // Register server socket
        struct epoll_event ev;
        ev.events = EPOLLIN | EPOLLET;
        ev.data.u32 = 0; // Server socket marker
        epoll_ctl(epoll_fd_, EPOLL_CTL_ADD, server_fd_, &ev);
        
        running_ = true;
        
        // Start worker threads
        for (int i = 0; i < workers; i++) {
            worker_threads_.emplace_back([this]() { workerLoop(); });
        }
        
        return true;
    }
    
    void stop() {
        running_ = false;
        
        if (epoll_fd_ >= 0) close(epoll_fd_);
        if (server_fd_ >= 0) close(server_fd_);
        
        for (auto& t : worker_threads_) {
            if (t.joinable()) t.join();
        }
    }
    
    void setCallbacks(
        std::function<void(Connection&, const uint8_t*, size_t)> on_message,
        std::function<void(Connection&)> on_connect = nullptr,
        std::function<void(Connection&)> on_disconnect = nullptr
    ) {
        on_message_ = on_message;
        on_connect_ = on_connect;
        on_disconnect_ = on_disconnect;
    }

private:
    void workerLoop() {
        struct epoll_event events[MAX_EVENTS];
        
        while (running_) {
            int n = epoll_wait(epoll_fd_, events, MAX_EVENTS, 1);
            if (n < 0) continue;
            
            for (int i = 0; i < n; i++) {
                uint32_t conn_id = events[i].data.u32;
                
                if (conn_id == 0) {
                    // Server socket - accept new connections
                    while (true) {
                        struct sockaddr_in client_addr;
                        socklen_t addr_len = sizeof(client_addr);
                        int client_fd = accept(server_fd_, (struct sockaddr*)&client_addr, &addr_len);
                        
                        if (client_fd < 0) break;
                        
                        handleAccept(client_fd);
                    }
                } else {
                    // Client socket - handle IO
                    if (events[i].events & (EPOLLERR | EPOLLHUP)) {
                        handleDisconnect(conn_id);
                    } else if (events[i].events & EPOLLIN) {
                        handleRead(conn_id);
                    }
                }
            }
        }
    }
    
    void handleAccept(int client_fd) {
        // Set socket options
        int opt = 1;
        setsockopt(client_fd, IPPROTO_TCP, TCP_NODELAY, &opt, sizeof(opt));
        setsockopt(client_fd, SOL_SOCKET, SO_SNDBUF, &(int){SEND_BUFFER_SIZE}, sizeof(int));
        setsockopt(client_fd, SOL_SOCKET, SO_RCVBUF, &(int){RECV_BUFFER_SIZE}, sizeof(int));
        
        // Set non-blocking
        int flags = fcntl(client_fd, F_GETFL, 0);
        fcntl(client_fd, F_SETFL, flags | O_NONBLOCK);
        
        // Create connection
        uint64_t conn_id = next_conn_id_++;
        
        Connection& conn = connections_[conn_id % MAX_CONNECTIONS];
        conn = {client_fd, conn_id, 0, 0, 0, 0, true};
        
        // Register with epoll
        struct epoll_event ev;
        ev.events = EPOLLIN | EPOLLOUT | EPOLLET;
        ev.data.u32 = conn_id % MAX_CONNECTIONS;
        epoll_ctl(epoll_fd_, EPOLL_CTL_ADD, client_fd, &ev);
        
        active_connections_++;
        
        if (on_connect_) {
            on_connect_(conn);
        }
    }
    
    void handleRead(uint32_t conn_id) {
        Connection& conn = connections_[conn_id % MAX_CONNECTIONS];
        
        uint8_t buffer[RECV_BUFFER_SIZE];
        ssize_t n = recv(conn.fd, buffer, RECV_BUFFER_SIZE, 0);
        
        if (n <= 0) {
            handleDisconnect(conn_id);
            return;
        }
        
        conn.bytes_in += n;
        total_bytes_in_ += n;
        conn.requests++;
        total_requests_++;
        
        if (on_message_) {
            on_message_(conn, buffer, n);
        }
    }
    
    void handleDisconnect(uint32_t conn_id) {
        Connection& conn = connections_[conn_id % MAX_CONNECTIONS];
        
        if (conn.registered) {
            close(conn.fd);
            conn.registered = false;
            active_connections_--;
            
            if (on_disconnect_) {
                on_disconnect_(conn);
            }
        }
    }
    
public:
    bool sendTo(uint64_t conn_id, const uint8_t* data, size_t len) {
        Connection& conn = connections_[conn_id % MAX_CONNECTIONS];
        
        if (!conn.registered) return false;
        
        ssize_t n = send(conn.fd, data, len, 0);
        if (n > 0) {
            conn.bytes_out += n;
            total_bytes_out_ += n;
            return true;
        }
        return false;
    }
    
    void broadcast(const uint8_t* data, size_t len) {
        for (auto& conn : connections_) {
            if (conn.registered) {
                send(conn.id, data, len);
            }
        }
    }
    
    struct Stats {
        uint64_t active_connections;
        uint64_t total_requests;
        uint64_t total_bytes_in;
        uint64_t total_bytes_out;
    };
    
    Stats getStats() const {
        return {
            active_connections_.load(),
            total_requests_.load(),
            total_bytes_in_.load(),
            total_bytes_out_.load()
        };
    }
};

// ============== UDP SERVER (Market Data) ==============
class UDPServer {
private:
    int socket_fd_;
    bool running_;
    std::thread recv_thread_;
    
    std::atomic<uint64_t> total_packets_{0};
    std::atomic<uint64_t> total_bytes_{0};
    
    std::function<void(const uint8_t*, size_t, struct sockaddr_in&)> on_packet_;
    
public:
    UDPServer() : socket_fd_(-1), running_(false) {}
    ~UDPServer() { stop(); }
    
    bool start(const char* ip, int port) {
        socket_fd_ = socket(AF_INET, SOCK_DGRAM, 0);
        if (socket_fd_ < 0) return false;
        
        int opt = 1;
        setsockopt(socket_fd_, SOL_SOCKET, SO_REUSEADDR, &opt, sizeof(opt));
        
        struct sockaddr_in addr;
        memset(&addr, 0, sizeof(addr));
        addr.sin_family = AF_INET;
        addr.sin_port = htons(port);
        inet_pton(AF_INET, ip, &addr.sin_addr);
        
        if (bind(socket_fd_, (struct sockaddr*)&addr, sizeof(addr)) < 0) return false;
        
        running_ = true;
        recv_thread_ = std::thread([this]() { recvLoop(); });
        
        return true;
    }
    
    void stop() {
        running_ = false;
        if (socket_fd_ >= 0) close(socket_fd_);
        if (recv_thread_.joinable()) recv_thread_.join();
    }
    
    void setOnPacket(std::function<void(const uint8_t*, size_t, struct sockaddr_in&)> cb) {
        on_packet_ = cb;
    }
    
private:
    void recvLoop() {
        uint8_t buffer[MAX_PACKET_SIZE];
        struct sockaddr_in client_addr;
        socklen_t addr_len = sizeof(client_addr);
        
        while (running_) {
            ssize_t n = recvfrom(socket_fd_, buffer, MAX_PACKET_SIZE, 0,
                                 (struct sockaddr*)&client_addr, &addr_len);
            
            if (n > 0) {
                total_packets_++;
                total_bytes_ += n;
                
                if (on_packet_) {
                    on_packet_(buffer, n, client_addr);
                }
            }
        }
    }
    
public:
    bool sendTo(const char* ip, int port, const uint8_t* data, size_t len) {
        struct sockaddr_in addr;
        memset(&addr, 0, sizeof(addr));
        addr.sin_family = AF_INET;
        addr.sin_port = htons(port);
        inet_pton(AF_INET, ip, &addr.sin_addr);
        
        return sendto(socket_fd_, data, len, 0, (struct sockaddr*)&addr, sizeof(addr)) > 0;
    }
    
    struct Stats {
        uint64_t total_packets;
        uint64_t total_bytes;
    };
    
    Stats getStats() const {
        return {total_packets_.load(), total_bytes_.load()};
    }
};

// ============== FFI ==============
extern "C" {

void* create_tcp_server() {
    return new TCPServer();
}

void destroy_tcp_server(void* server) {
    delete static_cast<TCPServer*>(server);
}

int tcp_start(void* server, const char* ip, int port, int workers) {
    return static_cast<TCPServer*>(server)->start(ip, port, workers) ? 1 : 0;
}

void tcp_stop(void* server) {
    static_cast<TCPServer*>(server)->stop();
}

int tcp_send(void* server, uint64_t conn_id, const uint8_t* data, size_t len) {
    return static_cast<TCPServer*>(server)->sendTo(conn_id, data, len) ? 1 : 0;
}

void* create_udp_server() {
    return new UDPServer();
}

void destroy_udp_server(void* server) {
    delete static_cast<UDPServer*>(server);
}

int udp_start(void* server, const char* ip, int port) {
    return static_cast<UDPServer*>(server)->start(ip, port) ? 1 : 0;
}

void udp_stop(void* server) {
    static_cast<UDPServer*>(server)->stop();
}

} // extern "C"

#endif // TIGERSWAP_NETWORK_HPP
