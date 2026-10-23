#include "tigerswap/network.hpp"

#include <arpa/inet.h>
#include <fcntl.h>
#include <netinet/in.h>
#include <netinet/tcp.h>
#include <sys/epoll.h>
#include <sys/socket.h>
#include <unistd.h>

#include <algorithm>
#include <cerrno>
#include <cctype>
#include <cstdlib>
#include <cstring>
#include <ctime>

#include "tigerswap/clock.hpp"

namespace tigerswap {
namespace {

constexpr size_t kMaxEvents = 1024;

std::string to_lower(const std::string& value) {
    std::string out = value;
    std::transform(out.begin(), out.end(), out.begin(), [](unsigned char character) {
        return static_cast<char>(std::tolower(character));
    });
    return out;
}

std::string trim(const std::string& value) {
    size_t begin = 0;
    while (begin < value.size() && (value[begin] == ' ' || value[begin] == '\t')) {
        ++begin;
    }
    size_t end = value.size();
    while (end > begin && (value[end - 1] == ' ' || value[end - 1] == '\t')) {
        --end;
    }
    return value.substr(begin, end - begin);
}

bool is_valid_method(const std::string& method) {
    return method == "GET" || method == "POST" || method == "PUT" || method == "DELETE" ||
           method == "PATCH" || method == "OPTIONS" || method == "HEAD";
}

bool is_valid_header_name(const std::string& name) {
    if (name.empty()) {
        return false;
    }
    for (const char character : name) {
        const unsigned char byte = static_cast<unsigned char>(character);
        const bool token = (byte >= 'a' && byte <= 'z') || (byte >= 'A' && byte <= 'Z') ||
                           (byte >= '0' && byte <= '9') || byte == '-' || byte == '_';
        if (!token) {
            return false;
        }
    }
    return true;
}

std::string status_text(int status) {
    switch (status) {
        case 200: return "OK";
        case 201: return "Created";
        case 204: return "No Content";
        case 400: return "Bad Request";
        case 401: return "Unauthorized";
        case 403: return "Forbidden";
        case 404: return "Not Found";
        case 405: return "Method Not Allowed";
        case 408: return "Request Timeout";
        case 413: return "Payload Too Large";
        case 415: return "Unsupported Media Type";
        case 429: return "Too Many Requests";
        case 500: return "Internal Server Error";
        case 503: return "Service Unavailable";
        default: return "OK";
    }
}

uint64_t now_ms() {
    return static_cast<uint64_t>(
        std::chrono::duration_cast<std::chrono::milliseconds>(
            std::chrono::steady_clock::now().time_since_epoch())
            .count());
}

int hex_digit(char character) {
    if (character >= '0' && character <= '9') return character - '0';
    if (character >= 'a' && character <= 'f') return character - 'a' + 10;
    if (character >= 'A' && character <= 'F') return character - 'A' + 10;
    return -1;
}

}  // namespace

std::string HttpRequest::header(const std::string& name) const {
    auto it = headers.find(to_lower(name));
    return it == headers.end() ? std::string() : it->second;
}

std::string HttpRequest::param(const std::string& name) const {
    if (query.empty()) {
        return {};
    }

    size_t position = 0;
    while (position <= query.size()) {
        const size_t ampersand = query.find('&', position);
        const std::string pair =
            query.substr(position, ampersand == std::string::npos ? std::string::npos : ampersand - position);

        const size_t equals = pair.find('=');
        if (equals != std::string::npos) {
            const std::string key = url_decode(pair.substr(0, equals));
            if (key == name) {
                return url_decode(pair.substr(equals + 1));
            }
        } else if (url_decode(pair) == name) {
            return {};
        }

        if (ampersand == std::string::npos) {
            break;
        }
        position = ampersand + 1;
    }

    return {};
}

HttpResponse HttpResponse::json(const std::string& payload, int status) {
    HttpResponse response;
    response.status = status;
    response.content_type = "application/json; charset=utf-8";
    response.body = payload;
    return response;
}

HttpResponse HttpResponse::text(const std::string& payload, int status) {
    HttpResponse response;
    response.status = status;
    response.content_type = "text/plain; charset=utf-8";
    response.body = payload;
    return response;
}

HttpResponse HttpResponse::empty(int status) {
    HttpResponse response;
    response.status = status;
    response.content_type = "application/json; charset=utf-8";
    response.body = "{}";
    return response;
}

std::string HttpResponse::serialize(bool keep_alive) const {
    std::string out;
    out.reserve(body.size() + 256);

    out += "HTTP/1.1 " + std::to_string(status) + " " + status_text(status) + "\r\n";
    out += "Content-Type: " + content_type + "\r\n";
    out += "Content-Length: " + std::to_string(body.size()) + "\r\n";
    out += "Connection: " + std::string(keep_alive ? "keep-alive" : "close") + "\r\n";

    // Defence in depth: these are set on every response so a mis-typed content
    // type can never let a browser execute a body as script.
    out += "X-Content-Type-Options: nosniff\r\n";
    out += "X-Frame-Options: DENY\r\n";
    out += "Content-Security-Policy: default-src 'none'; frame-ancestors 'none'\r\n";
    out += "Referrer-Policy: no-store\r\n";
    out += "Cache-Control: no-store\r\n";

    for (const auto& entry : headers) {
        out += entry.first + ": " + entry.second + "\r\n";
    }

    out += "\r\n";
    out += body;
    return out;
}

void HttpParser::reset() {
    consumed_ = 0;
    headers_parsed_ = false;
    header_end_ = 0;
    content_length_ = 0;
    should_close_ = false;
}

ParseStatus HttpParser::consume(const char* data, size_t length, HttpRequest& request) {
    if (buffer_.size() + length > limits_.max_header_bytes + limits_.max_body_bytes) {
        return ParseStatus::TooLarge;
    }

    buffer_.append(data, length);
    return parse(request);
}

ParseStatus HttpParser::parse(HttpRequest& request) {
    if (!headers_parsed_) {
        header_end_ = buffer_.find("\r\n\r\n");
        if (header_end_ == std::string::npos) {
            if (buffer_.size() > limits_.max_header_bytes) {
                return ParseStatus::TooLarge;
            }
            return ParseStatus::Incomplete;
        }

        const size_t header_bytes = header_end_ + 4;
        if (header_bytes > limits_.max_header_bytes) {
            return ParseStatus::TooLarge;
        }

        const std::string head = buffer_.substr(0, header_end_);
        size_t line_start = 0;
        bool first_line = true;

        while (line_start <= head.size()) {
            size_t line_end = head.find("\r\n", line_start);
            if (line_end == std::string::npos) {
                line_end = head.size();
            }

            const std::string line = head.substr(line_start, line_end - line_start);

            if (first_line) {
                first_line = false;

                const size_t first_space = line.find(' ');
                const size_t second_space =
                    first_space == std::string::npos ? std::string::npos : line.find(' ', first_space + 1);

                if (first_space == std::string::npos || second_space == std::string::npos) {
                    return ParseStatus::BadRequest;
                }

                request.method = line.substr(0, first_space);
                std::string target = line.substr(first_space + 1, second_space - first_space - 1);
                request.version = trim(line.substr(second_space + 1));

                if (!is_valid_method(request.method)) {
                    return ParseStatus::Unsupported;
                }
                if (request.version != "HTTP/1.1" && request.version != "HTTP/1.0") {
                    return ParseStatus::BadRequest;
                }
                if (target.empty() || target[0] != '/') {
                    return ParseStatus::BadRequest;
                }
                // Reject absolute-form and authority-form targets, which are
                // how a proxy-style request could be aimed at another host.
                if (target.find("//") == 0 || target.find("://") != std::string::npos) {
                    return ParseStatus::BadRequest;
                }
                if (line.size() > limits_.max_request_line) {
                    return ParseStatus::TooLarge;
                }

                const size_t question = target.find('?');
                if (question == std::string::npos) {
                    request.path = target;
                } else {
                    request.path = target.substr(0, question);
                    request.query = target.substr(question + 1);
                }
            } else if (!line.empty()) {
                if (request.headers.size() >= limits_.max_headers) {
                    return ParseStatus::TooLarge;
                }

                const size_t colon = line.find(':');
                if (colon == std::string::npos) {
                    return ParseStatus::BadRequest;
                }

                const std::string name = trim(line.substr(0, colon));
                const std::string value = trim(line.substr(colon + 1));

                if (!is_valid_header_name(name)) {
                    return ParseStatus::BadRequest;
                }

                // Header values must not contain control characters: a CR or LF
                // here is a request-splitting attempt.
                for (const char character : value) {
                    const unsigned char byte = static_cast<unsigned char>(character);
                    if (byte < 0x20 && byte != '\t') {
                        return ParseStatus::BadRequest;
                    }
                }

                const std::string lower = to_lower(name);
                if (lower == "content-length") {
                    if (value.empty() ||
                        !std::all_of(value.begin(), value.end(),
                                     [](unsigned char c) { return c >= '0' && c <= '9'; })) {
                        return ParseStatus::BadRequest;
                    }
                    const unsigned long long parsed = std::strtoull(value.c_str(), nullptr, 10);
                    if (parsed > limits_.max_body_bytes) {
                        return ParseStatus::TooLarge;
                    }
                    if (request.headers.count("content-length") != 0 &&
                        request.headers["content-length"] != value) {
                        // Conflicting lengths are a smuggling attempt.
                        return ParseStatus::BadRequest;
                    }
                    content_length_ = static_cast<size_t>(parsed);
                }
                if (lower == "transfer-encoding") {
                    // Chunked bodies are not supported; refusing is safer than
                    // guessing at framing.
                    return ParseStatus::Unsupported;
                }
                if (lower == "connection") {
                    const std::string lowered = to_lower(value);
                    should_close_ = lowered.find("close") != std::string::npos;
                }

                request.headers[lower] = value;
            }

            if (line_end == head.size()) {
                break;
            }
            line_start = line_end + 2;
        }

        if (request.headers.count("host") == 0 && request.version == "HTTP/1.1") {
            return ParseStatus::BadRequest;
        }

        headers_parsed_ = true;
        header_end_ = header_bytes;
    }

    if (buffer_.size() < header_end_ + content_length_) {
        return ParseStatus::Incomplete;
    }

    request.body = buffer_.substr(header_end_, content_length_);
    consumed_ = header_end_ + content_length_;

    // Drop what was consumed, keeping any pipelined bytes for the next request.
    buffer_.erase(0, consumed_);
    reset();

    return ParseStatus::Complete;
}

void Router::add(const std::string& method, const std::string& path, RequestHandler handler) {
    routes_.push_back(Route{method, path, std::move(handler)});
}

HttpResponse Router::dispatch(const HttpRequest& request) const {
    for (const auto& route : routes_) {
        if (route.method == request.method && route.path == request.path) {
            return route.handler(request);
        }
    }

    HttpResponse response = HttpResponse::json("{\"error\":\"not found\"}", 404);
    return response;
}

struct HttpServer::Connection {
    int fd = -1;
    std::string remote_address;
    HttpParser parser;
    std::string write_buffer;
    uint64_t last_active_ms = 0;
    uint64_t window_start_ms = 0;
    uint32_t window_requests = 0;
    bool close_after_write = false;

    explicit Connection(int descriptor, const ServerLimits& limits)
        : fd(descriptor), parser(limits) {}
};

HttpServer::HttpServer(const std::string& address, uint16_t port, ServerLimits limits,
                       Router router)
    : address_(address), port_(port), limits_(limits), router_(std::move(router)) {}

HttpServer::~HttpServer() {
    stop();

    for (auto& entry : connections_) {
        ::close(entry.first);
        delete entry.second;
    }
    connections_.clear();

    if (listen_fd_ >= 0) {
        ::close(listen_fd_);
    }
    if (epoll_fd_ >= 0) {
        ::close(epoll_fd_);
    }
}

bool HttpServer::start() {
    listen_fd_ = ::socket(AF_INET, SOCK_STREAM | SOCK_NONBLOCK | SOCK_CLOEXEC, 0);
    if (listen_fd_ < 0) {
        return false;
    }

    int reuse = 1;
    ::setsockopt(listen_fd_, SOL_SOCKET, SO_REUSEADDR, &reuse, sizeof(reuse));

    sockaddr_in address{};
    address.sin_family = AF_INET;
    address.sin_port = htons(port_);
    if (::inet_pton(AF_INET, address_.c_str(), &address.sin_addr) != 1) {
        ::close(listen_fd_);
        listen_fd_ = -1;
        return false;
    }

    if (::bind(listen_fd_, reinterpret_cast<sockaddr*>(&address), sizeof(address)) != 0) {
        ::close(listen_fd_);
        listen_fd_ = -1;
        return false;
    }

    if (::listen(listen_fd_, 512) != 0) {
        ::close(listen_fd_);
        listen_fd_ = -1;
        return false;
    }

    sockaddr_in bound{};
    socklen_t bound_length = sizeof(bound);
    if (::getsockname(listen_fd_, reinterpret_cast<sockaddr*>(&bound), &bound_length) == 0) {
        port_ = ntohs(bound.sin_port);
    }

    epoll_fd_ = ::epoll_create1(EPOLL_CLOEXEC);
    if (epoll_fd_ < 0) {
        ::close(listen_fd_);
        listen_fd_ = -1;
        return false;
    }

    epoll_event event{};
    event.events = EPOLLIN;
    event.data.fd = listen_fd_;
    if (::epoll_ctl(epoll_fd_, EPOLL_CTL_ADD, listen_fd_, &event) != 0) {
        return false;
    }

    return true;
}

void HttpServer::accept_connections() {
    while (true) {
        sockaddr_in peer{};
        socklen_t peer_length = sizeof(peer);

        const int client =
            ::accept4(listen_fd_, reinterpret_cast<sockaddr*>(&peer), &peer_length,
                      SOCK_NONBLOCK | SOCK_CLOEXEC);
        if (client < 0) {
            if (errno == EAGAIN || errno == EWOULDBLOCK || errno == EINTR) {
                return;
            }
            return;
        }

        if (connections_.size() >= limits_.max_connections) {
            connections_rejected_.fetch_add(1, std::memory_order_relaxed);
            ::close(client);
            continue;
        }

        int nodelay = 1;
        ::setsockopt(client, IPPROTO_TCP, TCP_NODELAY, &nodelay, sizeof(nodelay));

        auto* connection = new Connection(client, limits_);
        char text[INET_ADDRSTRLEN] = {};
        ::inet_ntop(AF_INET, &peer.sin_addr, text, sizeof(text));
        connection->remote_address = text;
        connection->last_active_ms = now_ms();
        connection->window_start_ms = connection->last_active_ms;

        epoll_event event{};
        event.events = EPOLLIN | EPOLLRDHUP | EPOLLET;
        event.data.fd = client;
        if (::epoll_ctl(epoll_fd_, EPOLL_CTL_ADD, client, &event) != 0) {
            delete connection;
            ::close(client);
            continue;
        }

        connections_.emplace(client, connection);
        connections_accepted_.fetch_add(1, std::memory_order_relaxed);
    }
}

bool HttpServer::rate_limit_ok(Connection& connection, uint64_t now) {
    if (now - connection.window_start_ms >= 1000) {
        connection.window_start_ms = now;
        connection.window_requests = 0;
    }

    ++connection.window_requests;
    return connection.window_requests <= limits_.requests_per_second;
}

void HttpServer::handle_readable(int fd) {
    auto it = connections_.find(fd);
    if (it == connections_.end()) {
        return;
    }

    Connection& connection = *it->second;
    char scratch[16384];

    while (true) {
        const ssize_t received = ::recv(fd, scratch, sizeof(scratch), 0);
        if (received == 0) {
            close_connection(fd, true);
            return;
        }
        if (received < 0) {
            if (errno == EINTR) {
                continue;
            }
            if (errno == EAGAIN || errno == EWOULDBLOCK) {
                break;
            }
            close_connection(fd, true);
            return;
        }

        bytes_in_.fetch_add(static_cast<uint64_t>(received), std::memory_order_relaxed);
        connection.last_active_ms = now_ms();

        HttpRequest request;
        const ParseStatus status = connection.parser.consume(scratch, static_cast<size_t>(received), request);

        switch (status) {
            case ParseStatus::Incomplete:
                continue;

            case ParseStatus::TooLarge:
            case ParseStatus::BadRequest:
            case ParseStatus::Unsupported: {
                bad_requests_.fetch_add(1, std::memory_order_relaxed);
                const int code = status == ParseStatus::TooLarge ? 413
                                 : status == ParseStatus::Unsupported ? 415 : 400;
                connection.write_buffer += HttpResponse::json("{\"error\":\"bad request\"}", code)
                                               .serialize(false);
                connection.close_after_write = true;
                break;
            }

            case ParseStatus::Complete: {
                if (!rate_limit_ok(connection, connection.last_active_ms)) {
                    rate_limited_.fetch_add(1, std::memory_order_relaxed);
                    connection.write_buffer +=
                        HttpResponse::json("{\"error\":\"rate limited\"}", 429).serialize(false);
                    connection.close_after_write = true;
                    break;
                }

                request.remote_address = connection.remote_address;
                requests_.fetch_add(1, std::memory_order_relaxed);

                HttpResponse response;
                try {
                    response = router_.dispatch(request);
                } catch (const std::exception&) {
                    response = HttpResponse::json("{\"error\":\"internal error\"}", 500);
                }

                const bool keep_alive =
                    !connection.close_after_write && !connection.parser.should_close();
                connection.write_buffer += response.serialize(keep_alive);
                break;
            }
        }

        if (connection.write_buffer.size() > limits_.write_buffer_cap) {
            close_connection(fd, true);
            return;
        }
    }

    flush(connection);

    epoll_event event{};
    event.events = EPOLLIN | EPOLLRDHUP | EPOLLET;
    if (!connection.write_buffer.empty()) {
        event.events |= EPOLLOUT;
    }
    event.data.fd = fd;
    ::epoll_ctl(epoll_fd_, EPOLL_CTL_MOD, fd, &event);

    if (connection.close_after_write && connection.write_buffer.empty()) {
        close_connection(fd, true);
    }
}

bool HttpServer::flush(Connection& connection) {
    while (!connection.write_buffer.empty()) {
        const ssize_t written = ::send(connection.fd, connection.write_buffer.data(),
                                       connection.write_buffer.size(), MSG_NOSIGNAL);
        if (written > 0) {
            connection.write_buffer.erase(0, static_cast<size_t>(written));
            bytes_out_.fetch_add(static_cast<uint64_t>(written), std::memory_order_relaxed);
            continue;
        }
        if (written < 0 && (errno == EAGAIN || errno == EWOULDBLOCK)) {
            return false;
        }
        if (written < 0 && errno == EINTR) {
            continue;
        }
        return false;
    }

    return true;
}

void HttpServer::handle_writable(int fd) {
    auto it = connections_.find(fd);
    if (it == connections_.end()) {
        return;
    }

    Connection& connection = *it->second;
    flush(connection);

    if (connection.close_after_write && connection.write_buffer.empty()) {
        close_connection(fd, true);
        return;
    }

    epoll_event event{};
    event.events = EPOLLIN | EPOLLRDHUP | EPOLLET;
    if (!connection.write_buffer.empty()) {
        event.events |= EPOLLOUT;
    }
    event.data.fd = fd;
    ::epoll_ctl(epoll_fd_, EPOLL_CTL_MOD, fd, &event);
}

void HttpServer::close_connection(int fd, bool count_as_closed) {
    auto it = connections_.find(fd);
    if (it == connections_.end()) {
        return;
    }

    ::epoll_ctl(epoll_fd_, EPOLL_CTL_DEL, fd, nullptr);
    ::close(fd);
    delete it->second;
    connections_.erase(it);

    if (count_as_closed) {
        connections_closed_.fetch_add(1, std::memory_order_relaxed);
    }
}

int HttpServer::poll_once(int timeout_ms) {
    epoll_event events[kMaxEvents];

    const int ready = ::epoll_wait(epoll_fd_, events, kMaxEvents, timeout_ms);
    if (ready < 0) {
        if (errno == EINTR) {
            return 0;
        }
        return -1;
    }

    const uint64_t now = now_ms();

    for (int index = 0; index < ready; ++index) {
        const int fd = events[index].data.fd;

        if (fd == listen_fd_) {
            accept_connections();
            continue;
        }

        if (events[index].events & (EPOLLHUP | EPOLLERR)) {
            close_connection(fd, true);
            continue;
        }

        if (events[index].events & EPOLLIN) {
            handle_readable(fd);
        }
        if (connections_.count(fd) != 0 && (events[index].events & EPOLLOUT)) {
            handle_writable(fd);
        }
    }

    // Reap idle connections.
    std::vector<int> stale;
    for (auto& entry : connections_) {
        if (now - entry.second->last_active_ms > limits_.idle_timeout_ms) {
            stale.push_back(entry.first);
        }
    }
    for (const int fd : stale) {
        close_connection(fd, true);
    }

    return ready;
}

void HttpServer::run() {
    running_.store(true);
    while (running_.load()) {
        poll_once(100);
    }
}

NetworkStats HttpServer::stats() const {
    NetworkStats out;
    out.connections_accepted = connections_accepted_.load(std::memory_order_relaxed);
    out.connections_closed = connections_closed_.load(std::memory_order_relaxed);
    out.connections_rejected = connections_rejected_.load(std::memory_order_relaxed);
    out.requests = requests_.load(std::memory_order_relaxed);
    out.bad_requests = bad_requests_.load(std::memory_order_relaxed);
    out.rate_limited = rate_limited_.load(std::memory_order_relaxed);
    out.bytes_in = bytes_in_.load(std::memory_order_relaxed);
    out.bytes_out = bytes_out_.load(std::memory_order_relaxed);
    out.active_connections = connections_.size();
    return out;
}

std::string json_escape(const std::string& value) {
    std::string out;
    out.reserve(value.size() + 8);

    for (const char character : value) {
        switch (character) {
            case '"': out += "\\\""; break;
            case '\\': out += "\\\\"; break;
            case '\b': out += "\\b"; break;
            case '\f': out += "\\f"; break;
            case '\n': out += "\\n"; break;
            case '\r': out += "\\r"; break;
            case '\t': out += "\\t"; break;
            // Escaping these keeps the payload inert if a page embeds the JSON
            // inside a script element.
            case '<': out += "\\u003c"; break;
            case '>': out += "\\u003e"; break;
            case '&': out += "\\u0026"; break;
            case '\'': out += "\\u0027"; break;
            default: {
                const unsigned char byte = static_cast<unsigned char>(character);
                if (byte < 0x20) {
                    // Control characters are escaped rather than emitted raw.
                    constexpr char digits[] = "0123456789abcdef";
                    out += "\\u00";
                    out.push_back(digits[byte >> 4]);
                    out.push_back(digits[byte & 0x0F]);
                } else {
                    out.push_back(character);
                }
            }
        }
    }

    return out;
}

std::string url_decode(const std::string& value) {
    std::string out;
    out.reserve(value.size());

    for (size_t index = 0; index < value.size(); ++index) {
        if (value[index] == '%' && index + 2 < value.size()) {
            const int high = hex_digit(value[index + 1]);
            const int low = hex_digit(value[index + 2]);
            if (high >= 0 && low >= 0) {
                out.push_back(static_cast<char>((high << 4) | low));
                index += 2;
                continue;
            }
        }
        if (value[index] == '+') {
            out.push_back(' ');
        } else {
            out.push_back(value[index]);
        }
    }

    return out;
}

bool parse_flat_json(const std::string& body, std::unordered_map<std::string, std::string>& out) {
    out.clear();

    size_t index = 0;
    auto skip_space = [&]() {
        while (index < body.size() &&
               (body[index] == ' ' || body[index] == '\t' || body[index] == '\n' || body[index] == '\r')) {
            ++index;
        }
    };

    auto parse_string = [&](std::string& result) -> bool {
        if (index >= body.size() || body[index] != '"') {
            return false;
        }
        ++index;
        result.clear();

        while (index < body.size()) {
            const char character = body[index];
            if (character == '"') {
                ++index;
                return true;
            }
            if (character == '\\') {
                if (index + 1 >= body.size()) {
                    return false;
                }
                const char escaped = body[index + 1];
                switch (escaped) {
                    case '"': result.push_back('"'); break;
                    case '\\': result.push_back('\\'); break;
                    case '/': result.push_back('/'); break;
                    case 'b': result.push_back('\b'); break;
                    case 'f': result.push_back('\f'); break;
                    case 'n': result.push_back('\n'); break;
                    case 'r': result.push_back('\r'); break;
                    case 't': result.push_back('\t'); break;
                    case 'u': {
                        if (index + 5 >= body.size()) {
                            return false;
                        }
                        unsigned code = 0;
                        for (int offset = 0; offset < 4; ++offset) {
                            const int digit = hex_digit(body[index + 2 + offset]);
                            if (digit < 0) {
                                return false;
                            }
                            code = (code << 4) | static_cast<unsigned>(digit);
                        }
                        // Encode as UTF-8. Surrogates are rejected rather than
                        // half-decoded.
                        if (code >= 0xD800 && code <= 0xDFFF) {
                            return false;
                        }
                        if (code < 0x80) {
                            result.push_back(static_cast<char>(code));
                        } else if (code < 0x800) {
                            result.push_back(static_cast<char>(0xC0 | (code >> 6)));
                            result.push_back(static_cast<char>(0x80 | (code & 0x3F)));
                        } else {
                            result.push_back(static_cast<char>(0xE0 | (code >> 12)));
                            result.push_back(static_cast<char>(0x80 | ((code >> 6) & 0x3F)));
                            result.push_back(static_cast<char>(0x80 | (code & 0x3F)));
                        }
                        index += 4;
                        break;
                    }
                    default:
                        return false;
                }
                index += 2;
                continue;
            }
            const unsigned char byte = static_cast<unsigned char>(character);
            if (byte < 0x20) {
                return false;
            }
            result.push_back(character);
            ++index;
        }

        return false;
    };

    skip_space();
    if (index >= body.size() || body[index] != '{') {
        return false;
    }
    ++index;
    skip_space();

    if (index < body.size() && body[index] == '}') {
        ++index;
        skip_space();
        return index == body.size();
    }

    while (index < body.size()) {
        std::string key;
        if (!parse_string(key)) {
            return false;
        }

        skip_space();
        if (index >= body.size() || body[index] != ':') {
            return false;
        }
        ++index;
        skip_space();

        std::string value;
        if (index < body.size() && body[index] == '"') {
            if (!parse_string(value)) {
                return false;
            }
        } else {
            const size_t start = index;
            while (index < body.size() && body[index] != ',' && body[index] != '}' &&
                   body[index] != ' ' && body[index] != '\n' && body[index] != '\r' &&
                   body[index] != '\t') {
                ++index;
            }
            if (index == start) {
                return false;
            }
            value = body.substr(start, index - start);
        }

        out[key] = value;

        skip_space();
        if (index >= body.size()) {
            return false;
        }
        if (body[index] == ',') {
            ++index;
            skip_space();
            continue;
        }
        if (body[index] == '}') {
            ++index;
            skip_space();
            return index == body.size();
        }
        return false;
    }

    return false;
}

}  // namespace tigerswap
