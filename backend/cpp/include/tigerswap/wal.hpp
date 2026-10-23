// Durable write-ahead log for order and trade events.
//
// Records are framed as [length:u32][crc32c:u32][payload], where the checksum
// covers the length and the payload. On recovery a torn or corrupt tail is
// truncated, so a crash mid-write can never produce a partially applied record.

#pragma once

#include <cstdint>
#include <string>
#include <vector>

namespace tigerswap {

/// Event kinds persisted by the engine.
enum class WalEventType : uint16_t {
    OrderAccepted = 1,
    OrderRejected = 2,
    OrderCancelled = 3,
    TradeExecuted = 4,
    PairAdded = 5,
    PairRemoved = 6,
};

/// A decoded log record.
struct WalRecord {
    WalEventType type = WalEventType::OrderAccepted;
    uint64_t sequence = 0;
    uint64_t timestamp = 0;
    std::vector<uint8_t> payload;
};

/// Computes CRC-32C (Castagnoli) over a buffer.
uint32_t crc32c(const uint8_t* data, size_t length);

/// Append-only, checksummed event log.
class WriteAheadLog {
public:
    WriteAheadLog() = default;
    WriteAheadLog(const WriteAheadLog&) = delete;
    WriteAheadLog& operator=(const WriteAheadLog&) = delete;

    ~WriteAheadLog();

    /// Opens (creating if needed) the log at `path`.
    bool open(const std::string& path);

    /// Closes the log, flushing any pending data.
    void close();

    bool is_open() const { return fd_ >= 0; }

    /// Appends a record. When `sync` is true the write is flushed to disk first.
    bool append(WalEventType type, const std::vector<uint8_t>& payload, bool sync = false);

    /// Flushes buffered writes to the operating system.
    bool flush();

    /// Forces buffered writes through to the storage device.
    bool sync();

    /// Replays every valid record, stopping at the first corrupt or torn entry.
    bool recover(std::vector<WalRecord>& out);

    /// Number of records appended by this process.
    uint64_t appended() const { return appended_; }

    /// Bytes currently held in the write buffer.
    size_t buffered_bytes() const { return buffer_.size(); }

    /// Truncates the log, discarding all records. Used after a snapshot.
    bool reset();

private:
    bool write_all(const uint8_t* data, size_t length);
    bool read_all(uint8_t* data, size_t length);

    int fd_ = -1;
    std::string path_;
    std::vector<uint8_t> buffer_;
    uint64_t appended_ = 0;
    uint64_t sequence_ = 0;
};

}  // namespace tigerswap
