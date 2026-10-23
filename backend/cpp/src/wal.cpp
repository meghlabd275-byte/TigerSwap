#include "tigerswap/wal.hpp"

#include <fcntl.h>
#include <unistd.h>

#include <cerrno>
#include <cstring>

#include "tigerswap/clock.hpp"

namespace tigerswap {
namespace {

// CRC-32C lookup table, generated once at start-up.
struct Crc32cTable {
    uint32_t values[256];

    Crc32cTable() {
        constexpr uint32_t polynomial = 0x82F63B78u;  // reversed Castagnoli
        for (uint32_t index = 0; index < 256; ++index) {
            uint32_t crc = index;
            for (int bit = 0; bit < 8; ++bit) {
                crc = (crc & 1u) ? (crc >> 1) ^ polynomial : crc >> 1;
            }
            values[index] = crc;
        }
    }
};

const Crc32cTable& crc_table() {
    static const Crc32cTable table;
    return table;
}

constexpr size_t kHeaderSize = 8;  // length (4) + crc (4)
constexpr uint32_t kMaxRecordSize = 1u << 20;

}  // namespace

uint32_t crc32c(const uint8_t* data, size_t length) {
    const Crc32cTable& table = crc_table();
    uint32_t crc = 0xFFFFFFFFu;

    for (size_t index = 0; index < length; ++index) {
        crc = table.values[(crc ^ data[index]) & 0xFFu] ^ (crc >> 8);
    }

    return crc ^ 0xFFFFFFFFu;
}

WriteAheadLog::~WriteAheadLog() { close(); }

bool WriteAheadLog::open(const std::string& path) {
    close();
    path_ = path;

    fd_ = ::open(path.c_str(), O_RDWR | O_CREAT | O_APPEND | O_CLOEXEC, 0644);
    return fd_ >= 0;
}

void WriteAheadLog::close() {
    if (fd_ >= 0) {
        flush();
        ::fsync(fd_);
        ::close(fd_);
        fd_ = -1;
    }
    buffer_.clear();
}

bool WriteAheadLog::append(WalEventType type, const std::vector<uint8_t>& payload, bool sync_after) {
    if (fd_ < 0) {
        return false;
    }

    std::vector<uint8_t> record;
    record.reserve(kHeaderSize + 2 + 8 + 8 + payload.size());

    const uint32_t payload_length = static_cast<uint32_t>(2 + 8 + 8 + payload.size());
    record.resize(kHeaderSize);
    record[0] = static_cast<uint8_t>(payload_length & 0xFF);
    record[1] = static_cast<uint8_t>((payload_length >> 8) & 0xFF);
    record[2] = static_cast<uint8_t>((payload_length >> 16) & 0xFF);
    record[3] = static_cast<uint8_t>((payload_length >> 24) & 0xFF);

    const uint16_t type_value = static_cast<uint16_t>(type);
    record.push_back(static_cast<uint8_t>(type_value & 0xFF));
    record.push_back(static_cast<uint8_t>((type_value >> 8) & 0xFF));

    const uint64_t sequence = ++sequence_;
    for (int index = 0; index < 8; ++index) {
        record.push_back(static_cast<uint8_t>((sequence >> (index * 8)) & 0xFF));
    }

    const uint64_t timestamp = wall_ns();
    for (int index = 0; index < 8; ++index) {
        record.push_back(static_cast<uint8_t>((timestamp >> (index * 8)) & 0xFF));
    }

    record.insert(record.end(), payload.begin(), payload.end());

    const uint32_t checksum = crc32c(record.data(), record.size());
    record[4] = static_cast<uint8_t>(checksum & 0xFF);
    record[5] = static_cast<uint8_t>((checksum >> 8) & 0xFF);
    record[6] = static_cast<uint8_t>((checksum >> 16) & 0xFF);
    record[7] = static_cast<uint8_t>((checksum >> 24) & 0xFF);

    buffer_.insert(buffer_.end(), record.begin(), record.end());
    ++appended_;

    if (buffer_.size() >= (1u << 20) || sync_after) {
        if (!flush()) {
            return false;
        }
        if (sync_after) {
            return sync();
        }
    }

    return true;
}

bool WriteAheadLog::flush() {
    if (fd_ < 0 || buffer_.empty()) {
        return true;
    }

    if (!write_all(buffer_.data(), buffer_.size())) {
        return false;
    }

    buffer_.clear();
    return true;
}

bool WriteAheadLog::sync() {
    if (fd_ < 0) {
        return false;
    }
    return ::fsync(fd_) == 0;
}

bool WriteAheadLog::write_all(const uint8_t* data, size_t length) {
    size_t written = 0;
    while (written < length) {
        const ssize_t result = ::write(fd_, data + written, length - written);
        if (result < 0) {
            if (errno == EINTR) {
                continue;
            }
            return false;
        }
        written += static_cast<size_t>(result);
    }
    return true;
}

bool WriteAheadLog::read_all(uint8_t* data, size_t length) {
    size_t read_bytes = 0;
    while (read_bytes < length) {
        const ssize_t result = ::read(fd_, data + read_bytes, length - read_bytes);
        if (result == 0) {
            return false;
        }
        if (result < 0) {
            if (errno == EINTR) {
                continue;
            }
            return false;
        }
        read_bytes += static_cast<size_t>(result);
    }
    return true;
}

bool WriteAheadLog::recover(std::vector<WalRecord>& out) {
    out.clear();

    if (path_.empty()) {
        return false;
    }

    const int read_fd = ::open(path_.c_str(), O_RDONLY | O_CLOEXEC);
    if (read_fd < 0) {
        return false;
    }

    off_t offset = 0;
    while (true) {
        uint8_t header[kHeaderSize];
        ssize_t got = ::pread(read_fd, header, kHeaderSize, offset);
        if (got == 0) {
            break;
        }
        if (got < 0) {
            if (errno == EINTR) {
                continue;
            }
            ::close(read_fd);
            return false;
        }
        if (got < static_cast<ssize_t>(kHeaderSize)) {
            // Torn header: the tail is incomplete and is dropped.
            break;
        }

        uint32_t payload_length = 0;
        for (int index = 0; index < 4; ++index) {
            payload_length |= static_cast<uint32_t>(header[index]) << (index * 8);
        }
        uint32_t checksum = 0;
        for (int index = 0; index < 4; ++index) {
            checksum |= static_cast<uint32_t>(header[4 + index]) << (index * 8);
        }

        if (payload_length < 18 || payload_length > kMaxRecordSize) {
            break;
        }

        std::vector<uint8_t> record(kHeaderSize + payload_length);
        std::memcpy(record.data(), header, kHeaderSize);

        const ssize_t body = ::pread(read_fd, record.data() + kHeaderSize, payload_length, offset + kHeaderSize);
        if (body < static_cast<ssize_t>(payload_length)) {
            // Torn payload: drop the partial record.
            break;
        }

        // The writer computed the checksum with the checksum field zeroed, so
        // zero it here too before recomputing.
        record[4] = 0;
        record[5] = 0;
        record[6] = 0;
        record[7] = 0;

        if (crc32c(record.data(), record.size()) != checksum) {
            // Checksum mismatch marks the end of the valid region.
            break;
        }

        WalRecord decoded;
        decoded.type = static_cast<WalEventType>(
            static_cast<uint16_t>(record[kHeaderSize]) |
            (static_cast<uint16_t>(record[kHeaderSize + 1]) << 8));

        for (int index = 0; index < 8; ++index) {
            decoded.sequence |= static_cast<uint64_t>(record[kHeaderSize + 2 + index]) << (index * 8);
            decoded.timestamp |= static_cast<uint64_t>(record[kHeaderSize + 10 + index]) << (index * 8);
        }

        decoded.payload.assign(record.begin() + kHeaderSize + 18, record.end());
        out.push_back(std::move(decoded));

        offset += kHeaderSize + payload_length;
    }

    ::close(read_fd);

    // Continue numbering after the recovered prefix.
    sequence_ = out.empty() ? 0 : out.back().sequence;
    return true;
}

bool WriteAheadLog::reset() {
    if (fd_ < 0) {
        return false;
    }

    buffer_.clear();
    sequence_ = 0;

    if (::ftruncate(fd_, 0) != 0) {
        return false;
    }
    return ::lseek(fd_, 0, SEEK_SET) >= 0;
}

}  // namespace tigerswap
