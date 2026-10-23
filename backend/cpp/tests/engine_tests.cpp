// Test suite for the TigerSwap matching engine.
//
// These tests exercise real code paths: matching, partial fills, time in force,
// self-trade prevention, risk rejection, the write-ahead log (including
// corruption recovery) and the cryptographic primitives.

#include <cstdio>
#include <cstdlib>
#include <unistd.h>
#include <string>
#include <atomic>
#include <thread>
#include <vector>

#include "tigerswap/crypto.hpp"
#include "tigerswap/matching_engine.hpp"
#include "tigerswap/network.hpp"
#include "tigerswap/order_book.hpp"
#include "tigerswap/wal.hpp"

using namespace tigerswap;

namespace {

int g_failures = 0;
int g_checks = 0;

#define CHECK(condition)                                                            \
    do {                                                                            \
        ++g_checks;                                                                 \
        if (!(condition)) {                                                         \
            ++g_failures;                                                           \
            std::printf("FAIL %s:%d: %s\n", __FILE__, __LINE__, #condition);        \
        }                                                                           \
    } while (false)

#define CHECK_EQ(actual, expected)                                                              \
    do {                                                                                        \
        ++g_checks;                                                                             \
        const auto actual_value = (actual);                                                     \
        const auto expected_value = (expected);                                                 \
        if (!(actual_value == expected_value)) {                                                \
            ++g_failures;                                                                       \
            std::printf("FAIL %s:%d: %s == %s\n", __FILE__, __LINE__, #actual, #expected);      \
        }                                                                                       \
    } while (false)

Order make_limit(uint64_t id, uint64_t trader, uint64_t price, uint64_t quantity, Side side) {
    Order order;
    order.id = id;
    order.trader = trader;
    order.price = price;
    order.quantity = quantity;
    order.side = side;
    order.type = OrderType::Limit;
    return order;
}

Order make_market(uint64_t id, uint64_t trader, uint64_t quantity, Side side) {
    Order order;
    order.id = id;
    order.trader = trader;
    order.quantity = quantity;
    order.side = side;
    order.type = OrderType::Market;
    return order;
}

// ---- order book ----

void test_resting_orders() {
    OrderBook book(1, 1, 0, 2);

    const auto bid = book.submit(make_limit(1, 100, 50'000, 10, Side::Buy));
    CHECK(bid.report.accepted);
    CHECK_EQ(book.best_bid(), 50'000u);

    const auto ask = book.submit(make_limit(2, 101, 50'100, 5, Side::Sell));
    CHECK(ask.report.accepted);
    CHECK_EQ(book.best_ask(), 50'100u);
    CHECK_EQ(book.open_orders(), 2u);
}

void test_maker_price_execution() {
    OrderBook book(1, 1, 0, 2);
    book.submit(make_limit(1, 100, 50'000, 10, Side::Sell));

    const auto result = book.submit(make_limit(2, 101, 50'500, 4, Side::Buy));
    CHECK(result.report.accepted);
    CHECK_EQ(result.trades.size(), 1u);
    CHECK_EQ(result.trades[0].price, 50'000u);
    CHECK_EQ(result.trades[0].quantity, 4u);
    CHECK_EQ(result.trades[0].taker_fee, 50'000u * 4u * 2u / 10'000u);
    CHECK_EQ(book.last_price(), 50'000u);
}

void test_partial_fill_rests() {
    OrderBook book(1, 1, 0, 2);
    book.submit(make_limit(1, 100, 50'000, 10, Side::Sell));

    const auto result = book.submit(make_limit(2, 101, 50'000, 25, Side::Buy));
    CHECK_EQ(result.report.filled, 10u);
    CHECK_EQ(result.report.remaining, 15u);

    const auto depth = book.depth(5);
    CHECK_EQ(depth.size(), 1u);
    CHECK_EQ(depth[0].price, 50'000u);
    CHECK_EQ(depth[0].quantity, 15u);
}

void test_ioc_cancels_remainder() {
    OrderBook book(1, 1, 0, 2);
    book.submit(make_limit(1, 100, 50'000, 5, Side::Sell));

    const auto result = book.submit(make_limit(2, 101, 50'000, 20, Side::Buy), TimeInForce::Ioc);
    CHECK_EQ(result.trades.size(), 1u);
    CHECK_EQ(result.report.remaining, 0u);
    CHECK(book.depth(5).empty());
}

void test_fok_rejects_thin_book() {
    OrderBook book(1, 1, 0, 2);
    book.submit(make_limit(1, 100, 50'000, 5, Side::Sell));

    const auto result = book.submit(make_limit(2, 101, 50'000, 20, Side::Buy), TimeInForce::Fok);
    CHECK(!result.report.accepted);
    CHECK(result.report.reason == RejectReason::FillOrKillUnfillable);
    CHECK_EQ(book.total_trades(), 0u);
}

void test_fok_fills_when_liquid() {
    OrderBook book(1, 1, 0, 2);
    book.submit(make_limit(1, 100, 50'000, 30, Side::Sell));

    const auto result = book.submit(make_limit(2, 101, 50'000, 20, Side::Buy), TimeInForce::Fok);
    CHECK(result.report.accepted);
    CHECK_EQ(result.report.remaining, 0u);
}

void test_post_only() {
    OrderBook book(1, 1, 0, 2);
    book.submit(make_limit(1, 100, 50'000, 5, Side::Sell));

    const auto crossing = book.submit(make_limit(2, 101, 50'000, 5, Side::Buy), TimeInForce::PostOnly);
    CHECK(!crossing.report.accepted);
    CHECK(crossing.report.reason == RejectReason::PostOnlyWouldCross);

    const auto resting = book.submit(make_limit(3, 101, 49'900, 5, Side::Buy), TimeInForce::PostOnly);
    CHECK(resting.report.accepted);
    CHECK_EQ(book.best_bid(), 49'900u);
}

void test_market_sweeps_levels() {
    OrderBook book(1, 1, 0, 2);
    book.submit(make_limit(1, 100, 50'000, 5, Side::Sell));
    book.submit(make_limit(2, 101, 50'100, 5, Side::Sell));
    book.submit(make_limit(3, 102, 50'200, 5, Side::Sell));

    const auto result = book.submit(make_market(4, 200, 12, Side::Buy));
    CHECK(result.report.accepted);
    CHECK_EQ(result.trades.size(), 3u);
    CHECK_EQ(result.trades[0].price, 50'000u);
    CHECK_EQ(result.trades[1].price, 50'100u);
    CHECK_EQ(result.trades[2].price, 50'200u);

    const auto depth = book.depth(10);
    CHECK_EQ(depth.size(), 1u);
    CHECK_EQ(depth[0].price, 50'200u);
    CHECK_EQ(depth[0].quantity, 3u);
}

void test_market_without_liquidity_rejected() {
    OrderBook book(1, 1, 0, 2);
    const auto result = book.submit(make_market(1, 100, 10, Side::Buy));
    CHECK(!result.report.accepted);
    CHECK(result.report.reason == RejectReason::NoLiquidity);
}

void test_self_trade_prevented() {
    OrderBook book(1, 1, 0, 2);
    book.submit(make_limit(1, 100, 50'000, 10, Side::Sell));

    const auto result = book.submit(make_limit(2, 100, 50'000, 10, Side::Buy));
    CHECK(result.report.accepted);
    CHECK(result.trades.empty());
    CHECK_EQ(book.total_trades(), 0u);
}

void test_cancel() {
    OrderBook book(1, 1, 0, 2);
    const auto placed = book.submit(make_limit(1, 100, 50'000, 10, Side::Buy));
    CHECK(placed.report.accepted);

    const auto wrong_owner = book.cancel(placed.report.order_id, 999);
    CHECK(!wrong_owner.accepted);
    CHECK(wrong_owner.reason == RejectReason::NotOrderOwner);

    const auto cancelled = book.cancel(placed.report.order_id, 100);
    CHECK(cancelled.accepted);
    CHECK(book.depth(5).empty());

    const auto again = book.cancel(placed.report.order_id, 100);
    CHECK(!again.accepted);
}

void test_tick_validation() {
    OrderBook book(1, 100, 0, 2);
    const auto result = book.submit(make_limit(1, 100, 50'050, 10, Side::Buy));
    CHECK(!result.report.accepted);
    CHECK(result.report.reason == RejectReason::PriceNotOnTick);

    const auto valid = book.submit(make_limit(2, 100, 50'100, 10, Side::Buy));
    CHECK(valid.report.accepted);
}

void test_zero_quantity_rejected() {
    OrderBook book(1, 1, 0, 2);
    const auto result = book.submit(make_limit(1, 100, 50'000, 0, Side::Buy));
    CHECK(!result.report.accepted);
    CHECK(result.report.reason == RejectReason::InvalidOrder);
}

void test_stop_not_marketable() {
    OrderBook book(1, 1, 0, 2);
    Order stop = make_limit(1, 100, 50'000, 10, Side::Buy);
    stop.type = OrderType::Stop;
    const auto result = book.submit(stop);
    CHECK(!result.report.accepted);
    CHECK(result.report.reason == RejectReason::StopRequiresTrigger);
}

void test_time_priority_preserved() {
    OrderBook book(1, 1, 0, 2);
    book.submit(make_limit(1, 100, 50'000, 5, Side::Sell));
    book.submit(make_limit(2, 101, 50'000, 5, Side::Sell));

    const auto result = book.submit(make_market(3, 200, 5, Side::Buy));
    CHECK_EQ(result.trades.size(), 1u);
    CHECK_EQ(result.trades[0].maker_order, 1u);
}

void test_depth_aggregates() {
    OrderBook book(1, 1, 0, 2);
    book.submit(make_limit(1, 100, 50'000, 10, Side::Buy));
    book.submit(make_limit(2, 101, 50'000, 15, Side::Buy));

    const auto depth = book.depth(5);
    CHECK_EQ(depth.size(), 1u);
    CHECK_EQ(depth[0].quantity, 25u);
    CHECK_EQ(depth[0].orders, 2u);
}

void test_aggressive_sell_orders_bids() {
    OrderBook book(1, 1, 0, 2);
    book.submit(make_limit(1, 100, 50'000, 5, Side::Buy));
    book.submit(make_limit(2, 101, 49'900, 5, Side::Buy));

    const auto result = book.submit(make_limit(3, 200, 49'900, 10, Side::Sell));
    CHECK_EQ(result.trades.size(), 2u);
    CHECK_EQ(result.trades[0].price, 50'000u);
    CHECK_EQ(result.trades[1].price, 49'900u);
}

void test_cancel_then_rebook() {
    OrderBook book(1, 1, 0, 2);
    const auto first = book.submit(make_limit(1, 100, 50'000, 10, Side::Buy));
    CHECK(book.cancel(first.report.order_id, 100).accepted);

    book.submit(make_limit(2, 101, 50'000, 7, Side::Buy));
    const auto depth = book.depth(5);
    CHECK_EQ(depth.size(), 1u);
    CHECK_EQ(depth[0].quantity, 7u);
}

// ---- matching engine ----

PairConfig pair(uint64_t id, const std::string& symbol) {
    PairConfig config;
    config.pair_id = id;
    config.symbol = symbol;
    config.tick_size = 1;
    config.maker_fee_bps = 0;
    config.taker_fee_bps = 2;
    return config;
}

void test_engine_pair_lifecycle() {
    MatchingEngine engine;
    CHECK(engine.add_pair(pair(1, "BTC-RUSD")));
    CHECK(!engine.add_pair(pair(1, "BTC-RUSD")));
    CHECK(engine.has_pair(1));
    CHECK_EQ(engine.list_pairs().size(), 1u);
    CHECK(engine.remove_pair(1));
    CHECK(!engine.has_pair(1));
}

void test_engine_rejects_oversized_order() {
    MatchingEngine engine;
    PairConfig config = pair(1, "BTC-RUSD");
    config.risk.max_order_quantity = 100;
    engine.add_pair(config);

    const auto too_big = engine.submit(1, make_limit(1, 100, 50'000, 500, Side::Buy), TimeInForce::Gtc);
    CHECK(!too_big.report.accepted);
    CHECK(too_big.report.reason == RejectReason::InvalidOrder);

    const auto ok = engine.submit(1, make_limit(2, 100, 50'000, 50, Side::Buy), TimeInForce::Gtc);
    CHECK(ok.report.accepted);
}

void test_engine_stats() {
    MatchingEngine engine;
    engine.add_pair(pair(1, "BTC-RUSD"));

    engine.submit(1, make_limit(1, 100, 50'000, 10, Side::Sell), TimeInForce::Gtc);
    engine.submit(1, make_limit(2, 101, 50'000, 4, Side::Buy), TimeInForce::Gtc);

    const auto stats = engine.stats();
    CHECK_EQ(stats.orders_accepted, 2u);
    CHECK_EQ(stats.trades, 1u);
    CHECK_EQ(stats.volume, 4u);
    CHECK_EQ(stats.pairs, 1u);
}

void test_engine_cancel() {
    MatchingEngine engine;
    engine.add_pair(pair(1, "BTC-RUSD"));

    const auto placed = engine.submit(1, make_limit(1, 100, 50'000, 10, Side::Buy), TimeInForce::Gtc);
    const auto cancelled = engine.cancel(1, placed.report.order_id, 100);
    CHECK(cancelled.accepted);
    CHECK(engine.depth(1, 5).empty());
}

void test_recovered_trade_restores_market_data() {
    MatchingEngine engine;
    engine.add_pair(pair(1, "BTC-RUSD"));

    engine.record_recovered_trade(1, 50'000, 40);

    CHECK_EQ(engine.last_price(1), 50'000u);
    const auto stats = engine.stats();
    CHECK_EQ(stats.trades, 1u);
    CHECK_EQ(stats.volume, 40u);
}

void test_recovery_restores_remaining_size() {
    MatchingEngine engine;
    engine.add_pair(pair(1, "BTC-RUSD"));

    // Simulate recovery: an order of 100 with 40 already filled is restored at 60.
    Order restored;
    restored.id = 1;
    restored.trader = 1;
    restored.price = 50'000;
    restored.quantity = 60;
    restored.type = OrderType::Limit;
    restored.side = Side::Sell;

    const auto result = engine.submit(1, restored, TimeInForce::PostOnly);
    CHECK(result.report.accepted);

    const auto depth = engine.depth(1, 5);
    CHECK_EQ(depth.size(), 1u);
    CHECK_EQ(depth[0].quantity, 60u);
}

void test_recovery_never_fabricates_trades() {
    MatchingEngine engine;
    engine.add_pair(pair(1, "BTC-RUSD"));

    // Two crossing orders replayed post-only must both rest, never trade.
    Order bid;
    bid.id = 1;
    bid.trader = 1;
    bid.price = 50'100;
    bid.quantity = 10;
    bid.type = OrderType::Limit;
    bid.side = Side::Buy;

    Order ask;
    ask.id = 2;
    ask.trader = 2;
    ask.price = 50'000;
    ask.quantity = 10;
    ask.type = OrderType::Limit;
    ask.side = Side::Sell;

    CHECK(engine.submit(1, bid, TimeInForce::PostOnly).report.accepted);

    // The ask would cross the restored bid. Post-only rejects it rather than
    // trading, so recovery cannot fabricate a fill from replayed orders.
    const auto ask_result = engine.submit(1, ask, TimeInForce::PostOnly);
    CHECK(!ask_result.report.accepted);
    CHECK(ask_result.report.reason == RejectReason::PostOnlyWouldCross);

    CHECK_EQ(engine.stats().trades, 0u);
}

// ---- write-ahead log ----

std::string temp_path(const std::string& name) {
    return "/tmp/tigerswap_test_" + name + "_" + std::to_string(::getpid());
}

void test_wal_round_trip() {
    const std::string path = temp_path("roundtrip");
    std::remove(path.c_str());

    {
        WriteAheadLog log;
        CHECK(log.open(path));
        std::vector<uint8_t> payload = {1, 2, 3, 4, 5};
        CHECK(log.append(WalEventType::OrderAccepted, payload, true));
        CHECK(log.append(WalEventType::TradeExecuted, payload));
        CHECK(log.flush());
    }

    WriteAheadLog log;
    CHECK(log.open(path));
    std::vector<WalRecord> records;
    CHECK(log.recover(records));
    CHECK_EQ(records.size(), 2u);
    if (records.size() == 2) {
        CHECK(records[0].type == WalEventType::OrderAccepted);
        CHECK(records[1].type == WalEventType::TradeExecuted);
        CHECK_EQ(records[0].payload.size(), 5u);
        CHECK(records[0].sequence < records[1].sequence);
    }

    std::remove(path.c_str());
}

void test_wal_truncates_corrupt_tail() {
    const std::string path = temp_path("corrupt");
    std::remove(path.c_str());

    {
        WriteAheadLog log;
        CHECK(log.open(path));
        CHECK(log.append(WalEventType::OrderAccepted, {9, 9, 9}, true));
        CHECK(log.append(WalEventType::OrderCancelled, {7, 7, 7}, true));
    }

    // Flip a byte inside the second record's payload.
    {
        FILE* file = std::fopen(path.c_str(), "r+b");
        CHECK(file != nullptr);
        if (file != nullptr) {
            std::fseek(file, 0, SEEK_END);
            const long size = std::ftell(file);
            std::fseek(file, size - 1, SEEK_SET);
            std::fputc(0xFF, file);
            std::fclose(file);
        }
    }

    WriteAheadLog log;
    CHECK(log.open(path));
    std::vector<WalRecord> records;
    CHECK(log.recover(records));
    // Only the intact prefix survives.
    CHECK_EQ(records.size(), 1u);

    std::remove(path.c_str());
}

void test_wal_detects_payload_change() {
    const std::string path = temp_path("checksum");
    std::remove(path.c_str());

    {
        WriteAheadLog log;
        CHECK(log.open(path));
        CHECK(log.append(WalEventType::OrderAccepted, {1, 2, 3, 4}, true));
    }

    {
        FILE* file = std::fopen(path.c_str(), "r+b");
        CHECK(file != nullptr);
        if (file != nullptr) {
            std::fseek(file, 10, SEEK_SET);
            std::fputc(0xAB, file);
            std::fclose(file);
        }
    }

    WriteAheadLog log;
    CHECK(log.open(path));
    std::vector<WalRecord> records;
    CHECK(log.recover(records));
    CHECK_EQ(records.size(), 0u);

    std::remove(path.c_str());
}

void test_wal_reset() {
    const std::string path = temp_path("reset");
    std::remove(path.c_str());

    WriteAheadLog log;
    CHECK(log.open(path));
    CHECK(log.append(WalEventType::OrderAccepted, {1}, true));
    CHECK(log.reset());
    CHECK(log.flush());

    std::vector<WalRecord> records;
    CHECK(log.recover(records));
    CHECK_EQ(records.size(), 0u);

    std::remove(path.c_str());
}

// ---- crypto ----

void test_sha256_known_vector() {
    const auto digest = sha256(std::string("abc"));
    const std::string expected =
        "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad";
    CHECK_EQ(to_hex(digest.data(), digest.size()), expected);
}

void test_hmac_verifies() {
    const std::string key = "secret-key";
    const std::string message = "tigerswap";

    const auto tag = hmac_sha256(reinterpret_cast<const uint8_t*>(key.data()), key.size(),
                                 reinterpret_cast<const uint8_t*>(message.data()), message.size());
    CHECK_EQ(tag.size(), kHmacSize);

    CHECK(verify_hmac_sha256(reinterpret_cast<const uint8_t*>(key.data()), key.size(),
                             reinterpret_cast<const uint8_t*>(message.data()), message.size(),
                             tag.data(), tag.size()));

    auto tampered = tag;
    tampered[0] ^= 0x01;
    CHECK(!verify_hmac_sha256(reinterpret_cast<const uint8_t*>(key.data()), key.size(),
                              reinterpret_cast<const uint8_t*>(message.data()), message.size(),
                              tampered.data(), tampered.size()));
}

void test_aes_gcm_round_trip() {
    const auto key = random_key();
    const Encryptor encryptor(key);

    const std::string plaintext = "tigerswap order: BTC/RUSD 1.5 @ 50000";
    const auto nonce = random_key();

    std::vector<uint8_t> ciphertext;
    CHECK(encryptor.encrypt(reinterpret_cast<const uint8_t*>(plaintext.data()), plaintext.size(),
                            nullptr, 0, nonce.data(), kGcmNonceSize, ciphertext));
    CHECK_EQ(ciphertext.size(), plaintext.size() + kGcmTagSize);

    std::vector<uint8_t> decrypted;
    CHECK(encryptor.decrypt(ciphertext.data(), ciphertext.size(), nullptr, 0, nonce.data(),
                            kGcmNonceSize, decrypted));
    CHECK_EQ(std::string(decrypted.begin(), decrypted.end()), plaintext);
}

void test_aes_gcm_rejects_tampering() {
    const auto key = random_key();
    const Encryptor encryptor(key);
    const auto nonce = random_key();

    const std::string plaintext = "authenticated payload";
    std::vector<uint8_t> ciphertext;
    CHECK(encryptor.encrypt(reinterpret_cast<const uint8_t*>(plaintext.data()), plaintext.size(),
                            nullptr, 0, nonce.data(), kGcmNonceSize, ciphertext));

    ciphertext[0] ^= 0x80;

    std::vector<uint8_t> decrypted;
    CHECK(!encryptor.decrypt(ciphertext.data(), ciphertext.size(), nullptr, 0, nonce.data(),
                             kGcmNonceSize, decrypted));
    CHECK(decrypted.empty());
}

void test_aes_gcm_aad_is_authenticated() {
    const auto key = random_key();
    const Encryptor encryptor(key);
    const auto nonce = random_key();

    const std::string plaintext = "payload";
    const std::string aad = "order-42";

    std::vector<uint8_t> ciphertext;
    CHECK(encryptor.encrypt(reinterpret_cast<const uint8_t*>(plaintext.data()), plaintext.size(),
                            reinterpret_cast<const uint8_t*>(aad.data()), aad.size(), nonce.data(),
                            kGcmNonceSize, ciphertext));

    std::vector<uint8_t> decrypted;
    // Decrypting without the AAD that was bound at encryption time must fail.
    CHECK(!encryptor.decrypt(ciphertext.data(), ciphertext.size(), nullptr, 0, nonce.data(),
                             kGcmNonceSize, decrypted));

    const std::string wrong_aad = "order-43";
    CHECK(!encryptor.decrypt(ciphertext.data(), ciphertext.size(),
                             reinterpret_cast<const uint8_t*>(wrong_aad.data()), wrong_aad.size(),
                             nonce.data(), kGcmNonceSize, decrypted));

    CHECK(encryptor.decrypt(ciphertext.data(), ciphertext.size(),
                            reinterpret_cast<const uint8_t*>(aad.data()), aad.size(), nonce.data(),
                            kGcmNonceSize, decrypted));
}

void test_random_nonce_encryption() {
    const Encryptor encryptor(random_key());
    const std::string plaintext = "random nonce path";

    std::vector<uint8_t> sealed;
    CHECK(encryptor.encrypt_random_nonce(reinterpret_cast<const uint8_t*>(plaintext.data()),
                                         plaintext.size(), sealed));

    std::vector<uint8_t> opened;
    CHECK(encryptor.decrypt_random_nonce(sealed.data(), sealed.size(), opened));
    CHECK_EQ(std::string(opened.begin(), opened.end()), plaintext);
}

void test_base64_round_trip() {
    const std::string input = "tigerswap\u0000binary";
    const auto encoded = base64_encode(reinterpret_cast<const uint8_t*>(input.data()), input.size());

    std::vector<uint8_t> decoded;
    CHECK(base64_decode(encoded, decoded));
    CHECK_EQ(std::string(decoded.begin(), decoded.end()), input);

    std::vector<uint8_t> bad;
    CHECK(!base64_decode("not base64!!!", bad));
}

void test_hex_round_trip() {
    const std::vector<uint8_t> bytes = {0x00, 0x0F, 0xA5, 0xFF};
    const std::string encoded = to_hex(bytes.data(), bytes.size());
    CHECK_EQ(encoded, "000fa5ff");

    std::vector<uint8_t> decoded;
    CHECK(from_hex(encoded, decoded));
    CHECK(decoded == bytes);

    CHECK(!from_hex("abc", decoded));
    CHECK(!from_hex("zz", decoded));
}

void test_pbkdf2_is_deterministic() {
    std::array<uint8_t, 32> first{};
    std::array<uint8_t, 32> second{};
    const std::vector<uint8_t> salt = {1, 2, 3, 4};

    CHECK(pbkdf2_sha256("password", salt, 1000, first));
    CHECK(pbkdf2_sha256("password", salt, 1000, second));
    CHECK(constant_time_equal(first.data(), second.data(), first.size()));

    std::array<uint8_t, 32> different{};
    CHECK(pbkdf2_sha256("password", salt, 1001, different));
    CHECK(!constant_time_equal(first.data(), different.data(), first.size()));
}

void test_api_key_digest_is_domain_separated() {
    const auto digest = api_key_digest("tiger_key_123");
    const auto plain = sha256(std::string("tiger_key_123"));
    CHECK(!constant_time_equal(digest.data(), plain.data(), digest.size()));
}

// ---- network parsing ----

void test_http_parser_accepts_valid_request() {
    ServerLimits limits;
    HttpParser parser(limits);
    HttpRequest request;

    const std::string raw =
        "POST /api/v1/order?symbol=BTC-RUSD HTTP/1.1\r\n"
        "Host: localhost\r\n"
        "Content-Type: application/json\r\n"
        "Content-Length: 13\r\n"
        "\r\n"
        "{\"a\":\"bbbbb\"}";

    CHECK(parser.consume(raw.data(), raw.size(), request) == ParseStatus::Complete);
    CHECK_EQ(request.method, "POST");
    CHECK_EQ(request.path, "/api/v1/order");
    CHECK_EQ(request.param("symbol"), "BTC-RUSD");
    CHECK_EQ(request.body, "{\"a\":\"bbbbb\"}");
}

void test_http_parser_rejects_missing_host() {
    ServerLimits limits;
    HttpParser parser(limits);
    HttpRequest request;

    const std::string raw = "GET /health HTTP/1.1\r\nContent-Length: 0\r\n\r\n";
    CHECK(parser.consume(raw.data(), raw.size(), request) == ParseStatus::BadRequest);
}

void test_http_parser_rejects_smuggling() {
    ServerLimits limits;
    HttpParser parser(limits);
    HttpRequest request;

    const std::string raw =
        "POST /api/v1/order HTTP/1.1\r\n"
        "Host: localhost\r\n"
        "Transfer-Encoding: chunked\r\n"
        "\r\n";
    CHECK(parser.consume(raw.data(), raw.size(), request) == ParseStatus::Unsupported);
}

void test_http_parser_rejects_header_injection() {
    ServerLimits limits;
    HttpParser parser(limits);
    HttpRequest request;

    const std::string raw =
        "GET /health HTTP/1.1\r\n"
        "Host: localhost\r\n"
        "X-Evil: value\rInjected: yes\r\n"
        "\r\n";
    CHECK(parser.consume(raw.data(), raw.size(), request) == ParseStatus::BadRequest);
}

void test_http_parser_rejects_absolute_target() {
    ServerLimits limits;
    HttpParser parser(limits);
    HttpRequest request;

    const std::string raw =
        "GET http://attacker.example/steal HTTP/1.1\r\nHost: localhost\r\n\r\n";
    CHECK(parser.consume(raw.data(), raw.size(), request) == ParseStatus::BadRequest);
}

void test_http_parser_handles_partial_input() {
    ServerLimits limits;
    HttpParser parser(limits);
    HttpRequest request;

    const std::string head = "POST /api/v1/order HTTP/1.1\r\nHost: localhost\r\nContent-Length: 7\r\n\r\n";
    CHECK(parser.consume(head.data(), head.size(), request) == ParseStatus::Incomplete);

    const std::string tail = "{\"a\":1}";
    CHECK(parser.consume(tail.data(), tail.size(), request) == ParseStatus::Complete);
    CHECK_EQ(request.body, "{\"a\":1}");
}

void test_response_has_security_headers() {
    const HttpResponse response = HttpResponse::json("{\"ok\":true}");
    const std::string serialized = response.serialize(true);

    CHECK(serialized.find("X-Content-Type-Options: nosniff\r\n") != std::string::npos);
    CHECK(serialized.find("X-Frame-Options: DENY\r\n") != std::string::npos);
    CHECK(serialized.find("Content-Security-Policy:") != std::string::npos);
    CHECK(serialized.find("Cache-Control: no-store\r\n") != std::string::npos);
}

void test_json_escape_blocks_script() {
    const std::string escaped = json_escape("</script><script>alert(1)</script>");
    CHECK(escaped.find("<script>") == std::string::npos);
    CHECK(escaped.find("\\u0000") == std::string::npos);

    const std::string control = json_escape(std::string("a\x01""b"));
    CHECK(control.find("\\u0001") != std::string::npos);
}

void test_flat_json_parser() {
    std::unordered_map<std::string, std::string> fields;

    CHECK(parse_flat_json("{\"symbol\":\"BTC-RUSD\",\"quantity\":10,\"price\":50000}", fields));
    CHECK_EQ(fields["symbol"], "BTC-RUSD");
    CHECK_EQ(fields["quantity"], "10");

    CHECK(parse_flat_json("{}", fields));
    CHECK(fields.empty());

    CHECK(!parse_flat_json("{\"symbol\":\"BTC-RUSD\"", fields));
    CHECK(!parse_flat_json("{\"symbol\":\"BTC-RUSD\"} trailing", fields));
    CHECK(!parse_flat_json("[1,2,3]", fields));
    CHECK(!parse_flat_json("{\"a\":\"\\q\"}", fields));
}

void test_url_decode() {
    CHECK_EQ(url_decode("BTC%2DRUSD"), "BTC-RUSD");
    CHECK_EQ(url_decode("a+b"), "a b");
    CHECK_EQ(url_decode("plain"), "plain");
    CHECK_EQ(url_decode("%zz"), "%zz");
}

// ---- concurrency ----

void test_engine_under_concurrent_readers() {
    MatchingEngine engine;
    engine.add_pair(pair(1, "BTC-RUSD"));

    std::atomic<bool> stop{false};
    std::vector<std::thread> readers;

    for (int index = 0; index < 4; ++index) {
        readers.emplace_back([&engine, &stop]() {
            while (!stop.load()) {
                engine.stats();
                engine.depth(1, 10);
                engine.spread(1);
            }
        });
    }

    for (uint64_t index = 0; index < 2000; ++index) {
        const uint64_t price = 50'000 + (index % 10);
        engine.submit(1, make_limit(index * 2 + 1, 100 + index, price, 1, Side::Sell), TimeInForce::Gtc);
        engine.submit(1, make_limit(index * 2 + 2, 900 + index, price, 1, Side::Buy), TimeInForce::Gtc);
    }

    stop.store(true);
    for (auto& reader : readers) {
        reader.join();
    }

    const auto stats = engine.stats();
    CHECK(stats.trades > 0);
}

}  // namespace

int main() {
    std::printf("TigerSwap C++ engine test suite\n");

    test_resting_orders();
    test_maker_price_execution();
    test_partial_fill_rests();
    test_ioc_cancels_remainder();
    test_fok_rejects_thin_book();
    test_fok_fills_when_liquid();
    test_post_only();
    test_market_sweeps_levels();
    test_market_without_liquidity_rejected();
    test_self_trade_prevented();
    test_cancel();
    test_tick_validation();
    test_zero_quantity_rejected();
    test_stop_not_marketable();
    test_time_priority_preserved();
    test_depth_aggregates();
    test_aggressive_sell_orders_bids();
    test_cancel_then_rebook();

    test_engine_pair_lifecycle();
    test_engine_rejects_oversized_order();
    test_engine_stats();
    test_engine_cancel();
    test_recovered_trade_restores_market_data();
    test_recovery_restores_remaining_size();
    test_recovery_never_fabricates_trades();

    test_wal_round_trip();
    test_wal_truncates_corrupt_tail();
    test_wal_detects_payload_change();
    test_wal_reset();

    test_sha256_known_vector();
    test_hmac_verifies();
    test_aes_gcm_round_trip();
    test_aes_gcm_rejects_tampering();
    test_aes_gcm_aad_is_authenticated();
    test_random_nonce_encryption();
    test_base64_round_trip();
    test_hex_round_trip();
    test_pbkdf2_is_deterministic();
    test_api_key_digest_is_domain_separated();

    test_http_parser_accepts_valid_request();
    test_http_parser_rejects_missing_host();
    test_http_parser_rejects_smuggling();
    test_http_parser_rejects_header_injection();
    test_http_parser_rejects_absolute_target();
    test_http_parser_handles_partial_input();
    test_response_has_security_headers();
    test_json_escape_blocks_script();
    test_flat_json_parser();
    test_url_decode();

    test_engine_under_concurrent_readers();

    std::printf("%d checks, %d failures\n", g_checks, g_failures);
    return g_failures == 0 ? 0 : 1;
}
