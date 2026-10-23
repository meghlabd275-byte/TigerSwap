// Cryptographic primitives for the TigerSwap engine.
//
// Everything here is backed by OpenSSL EVP. Two properties matter for this
// layer:
//   * AEAD tags are always verified before plaintext is returned, so a
//     tampered ciphertext yields an error rather than attacker-chosen data.
//   * Secret comparison is constant time, so MAC checks do not leak length or
//     content through timing.

#pragma once

#include <array>
#include <cstdint>
#include <string>
#include <vector>

namespace tigerswap {

constexpr size_t kSha256Size = 32;
constexpr size_t kSha512Size = 64;
constexpr size_t kAes256KeySize = 32;
constexpr size_t kGcmNonceSize = 12;
constexpr size_t kGcmTagSize = 16;
constexpr size_t kHmacSize = 32;

/// Constant-time comparison of two byte ranges.
bool constant_time_equal(const uint8_t* a, const uint8_t* b, size_t length);

/// SHA-256 over a buffer.
std::array<uint8_t, kSha256Size> sha256(const uint8_t* data, size_t length);

/// SHA-256 of a string.
std::array<uint8_t, kSha256Size> sha256(const std::string& text);

/// SHA-512 over a buffer.
std::array<uint8_t, kSha512Size> sha512(const uint8_t* data, size_t length);

/// Keccak-256, as used by Ethereum for address derivation.
std::array<uint8_t, 32> keccak256(const uint8_t* data, size_t length);

/// Lowercase hex encoding.
std::string to_hex(const uint8_t* data, size_t length);

/// Decodes hex, returning false on malformed input.
bool from_hex(const std::string& text, std::vector<uint8_t>& out);

/// Fills `buffer` with cryptographically secure random bytes.
bool secure_random(uint8_t* buffer, size_t length);

/// A fresh 32-byte random value, e.g. for a key or a nonce seed.
std::array<uint8_t, 32> random_key();

/// HMAC-SHA-256.
std::vector<uint8_t> hmac_sha256(const uint8_t* key, size_t key_length, const uint8_t* data,
                                 size_t data_length);

/// Verifies an HMAC-SHA-256 tag in constant time.
bool verify_hmac_sha256(const uint8_t* key, size_t key_length, const uint8_t* data,
                        size_t data_length, const uint8_t* tag, size_t tag_length);

/// AES-256-GCM with a caller-supplied nonce.
///
/// A nonce must never repeat for the same key: that would break confidentiality
/// and authenticity. `Encryptor::encrypt_random_nonce` handles this by
/// prepending a fresh random nonce to the output.
class Encryptor {
public:
    /// Builds an encryptor from a 32-byte key.
    explicit Encryptor(const std::array<uint8_t, kAes256KeySize>& key);

    /// Builds an encryptor from a key derived from `password` and `salt`.
    static Encryptor from_password(const std::string& password, const std::vector<uint8_t>& salt,
                                   uint32_t iterations = 200'000);

    /// Encrypts with the given nonce. The tag is appended to the ciphertext.
    bool encrypt(const uint8_t* plaintext, size_t length, const uint8_t* aad, size_t aad_length,
                 const uint8_t* nonce, size_t nonce_length, std::vector<uint8_t>& out) const;

    /// Decrypts and verifies. Returns false if the tag does not match.
    bool decrypt(const uint8_t* ciphertext, size_t length, const uint8_t* aad, size_t aad_length,
                 const uint8_t* nonce, size_t nonce_length, std::vector<uint8_t>& out) const;

    /// Encrypts, prepending a fresh random nonce to the returned buffer.
    bool encrypt_random_nonce(const uint8_t* plaintext, size_t length, std::vector<uint8_t>& out) const;

    /// Decrypts a buffer produced by `encrypt_random_nonce`.
    bool decrypt_random_nonce(const uint8_t* data, size_t length, std::vector<uint8_t>& out) const;

    const std::array<uint8_t, kAes256KeySize>& key() const { return key_; }

private:
    std::array<uint8_t, kAes256KeySize> key_;
};

/// Derives a 32-byte key with PBKDF2-HMAC-SHA256.
bool pbkdf2_sha256(const std::string& password, const std::vector<uint8_t>& salt,
                   uint32_t iterations, std::array<uint8_t, 32>& out);

/// Derives a 32-byte key with scrypt.
bool scrypt_key(const std::string& password, const std::vector<uint8_t>& salt, uint64_t cost,
                uint32_t block_size, uint32_t parallelism, std::array<uint8_t, 32>& out);

/// Base64 encoding and decoding, used for API-key headers and signatures.
std::string base64_encode(const uint8_t* data, size_t length);
bool base64_decode(const std::string& text, std::vector<uint8_t>& out);

/// Base64url encoding without padding, used in JWT segments.
std::string base64url_encode(const uint8_t* data, size_t length);

/// Constant-time check that `candidate` matches the SHA-256 of `secret`.
///
/// API keys are stored as digests, so a leaked store does not expose usable
/// credentials.
std::array<uint8_t, kSha256Size> api_key_digest(const std::string& secret);

/// URL-safe percent encoding for building outbound requests.
std::string url_encode(const std::string& value);

/// Strips control characters that could break a log or a header.
std::string sanitize_log_field(const std::string& value);

}  // namespace tigerswap
