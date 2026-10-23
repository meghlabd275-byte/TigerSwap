// Epoll-based HTTP/1.1 server for the TigerSwap matching engine.
//
// The parser is deliberately strict: request line, header count, header size,
// body size and framing are all bounded, and any deviation closes the
// connection. Responses always carry `X-Content-Type-Options: nosniff` and a
// restrictive `Content-Security-Policy`, so a browser cannot be tricked into
// reinterpreting a response body.

#pragma once

#include <atomic>
#include <cstdint>
#include <functional>
#include <map>
#include <string>
#include <unordered_map>
#include <vector>

namespace tigerswap {

/// Limits that bound every connection's resource use.
struct ServerLimits {
    size_t max_request_line = 8192;
    size_t max_header_bytes = 16384;
    size_t max_body_bytes = 1 << 20;
    size_t max_headers = 100;
    size_t max_connections = 10000;
    size_t read_buffer_cap = 1 << 21;
    size_t write_buffer_cap = 1 << 21;
    uint32_t idle_timeout_ms = 30000;
    uint32_t requests_per_second = 500;
};

/// A parsed HTTP request.
struct HttpRequest {
    std::string method;
    std::string path;
    std::string query;
    std::string version;
    std::unordered_map<std::string, std::string> headers;
    std::string body;
    std::string remote_address;

    /// Returns a header value, or an empty string when absent.
    std::string header(const std::string& name) const;

    /// Returns a query parameter, or an empty string when absent.
    std::string param(const std::string& name) const;
};

/// A response under construction.
struct HttpResponse {
    int status = 200;
    std::string content_type = "application/json";
    std::string body;
    std::map<std::string, std::string> headers;

    static HttpResponse json(const std::string& payload, int status = 200);
    static HttpResponse text(const std::string& payload, int status = 200);
    static HttpResponse empty(int status);

    /// Serialises the response, including the standard security headers.
    std::string serialize(bool keep_alive) const;
};

/// Reason a request could not be parsed.
enum class ParseStatus {
    Complete,
    Incomplete,
    BadRequest,
    TooLarge,
    Unsupported,
};

/// Incremental HTTP/1.1 request parser.
class HttpParser {
public:
    explicit HttpParser(const ServerLimits& limits) : limits_(limits) {}

    /// Consumes bytes and reports whether a full request is available.
    ParseStatus consume(const char* data, size_t length, HttpRequest& request);

    /// Attempts to parse a request from already-buffered bytes.
    ParseStatus parse(HttpRequest& request);

    /// Bytes consumed for the last completed request.
    size_t consumed() const { return consumed_; }

    /// Clears state for the next request on the same connection.
    void reset();

    /// True once a non-keep-alive request has been parsed.
    bool should_close() const { return should_close_; }

private:
    ServerLimits limits_;
    std::string buffer_;
    size_t consumed_ = 0;
    bool headers_parsed_ = false;
    size_t header_end_ = 0;
    size_t content_length_ = 0;
    bool should_close_ = false;
};

/// Handler for a routed request.
using RequestHandler = std::function<HttpResponse(const HttpRequest&)>;

/// Minimal router with exact-match path registration.
class Router {
public:
    void add(const std::string& method, const std::string& path, RequestHandler handler);
    HttpResponse dispatch(const HttpRequest& request) const;

private:
    struct Route {
        std::string method;
        std::string path;
        RequestHandler handler;
    };
    std::vector<Route> routes_;
};

/// Statistics for the server.
struct NetworkStats {
    uint64_t connections_accepted = 0;
    uint64_t connections_closed = 0;
    uint64_t connections_rejected = 0;
    uint64_t requests = 0;
    uint64_t bad_requests = 0;
    uint64_t rate_limited = 0;
    uint64_t bytes_in = 0;
    uint64_t bytes_out = 0;
    size_t active_connections = 0;
};

/// Non-blocking epoll server.
class HttpServer {
public:
    HttpServer(const std::string& address, uint16_t port, ServerLimits limits, Router router);
    ~HttpServer();

    HttpServer(const HttpServer&) = delete;
    HttpServer& operator=(const HttpServer&) = delete;

    /// Binds and starts listening. Returns false on failure.
    bool start();

    /// Runs one poll iteration. Returns the number of events handled, or -1 on error.
    int poll_once(int timeout_ms);

    /// Runs until `stop` is called.
    void run();

    /// Requests a clean shutdown.
    void stop() { running_.store(false); }

    /// Bound port, useful when the caller requested port 0.
    uint16_t port() const { return port_; }

    NetworkStats stats() const;

private:
    struct Connection;

    void accept_connections();
    void handle_readable(int fd);
    void handle_writable(int fd);
    void close_connection(int fd, bool count_as_closed);
    bool flush(Connection& connection);
    bool rate_limit_ok(Connection& connection, uint64_t now_ms);

    std::string address_;
    uint16_t port_;
    ServerLimits limits_;
    Router router_;

    int listen_fd_ = -1;
    int epoll_fd_ = -1;
    std::unordered_map<int, Connection*> connections_;

    std::atomic<bool> running_{false};
    std::atomic<uint64_t> connections_accepted_{0};
    std::atomic<uint64_t> connections_closed_{0};
    std::atomic<uint64_t> connections_rejected_{0};
    std::atomic<uint64_t> requests_{0};
    std::atomic<uint64_t> bad_requests_{0};
    std::atomic<uint64_t> rate_limited_{0};
    std::atomic<uint64_t> bytes_in_{0};
    std::atomic<uint64_t> bytes_out_{0};
};

/// Escapes a string for safe embedding in a JSON string literal.
std::string json_escape(const std::string& value);

/// Decodes percent-encoding and `+` in a query parameter value.
std::string url_decode(const std::string& value);

/// Parses a JSON object of string or number values into a flat map.
///
/// This is intentionally a small, strict reader for request bodies: it rejects
/// trailing data, unterminated strings and control characters, and it never
/// allocates unboundedly.
bool parse_flat_json(const std::string& body, std::unordered_map<std::string, std::string>& out);

}  // namespace tigerswap
