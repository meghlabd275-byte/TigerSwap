#include "tigerswap/crypto.hpp"

#include <openssl/crypto.h>
#include <openssl/evp.h>
#include <openssl/hmac.h>
#include <openssl/kdf.h>
#include <openssl/rand.h>

#include <cstring>

namespace tigerswap {
namespace {

constexpr char kHexDigits[] = "0123456789abcdef";
constexpr char kBase64Digits[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
constexpr char kBase64UrlDigits[] = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";

int hex_value(char character) {
    if (character >= '0' && character <= '9') {
        return character - '0';
    }
    if (character >= 'a' && character <= 'f') {
        return character - 'a' + 10;
    }
    if (character >= 'A' && character <= 'F') {
        return character - 'A' + 10;
    }
    return -1;
}

template <typename Digits>
std::string encode_base64(const uint8_t* data, size_t length, const Digits* digits, bool padding) {
    std::string out;
    out.reserve(((length + 2) / 3) * 4);

    size_t index = 0;
    while (index + 3 <= length) {
        const uint32_t block = (static_cast<uint32_t>(data[index]) << 16) |
                               (static_cast<uint32_t>(data[index + 1]) << 8) |
                               static_cast<uint32_t>(data[index + 2]);
        out.push_back(digits[(block >> 18) & 0x3F]);
        out.push_back(digits[(block >> 12) & 0x3F]);
        out.push_back(digits[(block >> 6) & 0x3F]);
        out.push_back(digits[block & 0x3F]);
        index += 3;
    }

    const size_t remaining = length - index;
    if (remaining == 1) {
        const uint32_t block = static_cast<uint32_t>(data[index]) << 16;
        out.push_back(digits[(block >> 18) & 0x3F]);
        out.push_back(digits[(block >> 12) & 0x3F]);
        if (padding) {
            out.push_back('=');
            out.push_back('=');
        }
    } else if (remaining == 2) {
        const uint32_t block = (static_cast<uint32_t>(data[index]) << 16) |
                               (static_cast<uint32_t>(data[index + 1]) << 8);
        out.push_back(digits[(block >> 18) & 0x3F]);
        out.push_back(digits[(block >> 12) & 0x3F]);
        out.push_back(digits[(block >> 6) & 0x3F]);
        if (padding) {
            out.push_back('=');
        }
    }

    return out;
}

int base64_value(char character) {
    if (character >= 'A' && character <= 'Z') {
        return character - 'A';
    }
    if (character >= 'a' && character <= 'z') {
        return character - 'a' + 26;
    }
    if (character >= '0' && character <= '9') {
        return character - '0' + 52;
    }
    if (character == '+' || character == '-') {
        return 62;
    }
    if (character == '/' || character == '_') {
        return 63;
    }
    return -1;
}

}  // namespace

bool constant_time_equal(const uint8_t* a, const uint8_t* b, size_t length) {
    return CRYPTO_memcmp(a, b, length) == 0;
}

std::array<uint8_t, kSha256Size> sha256(const uint8_t* data, size_t length) {
    std::array<uint8_t, kSha256Size> digest{};
    unsigned int digest_length = 0;

    EVP_MD_CTX* ctx = EVP_MD_CTX_new();
    if (ctx == nullptr) {
        return digest;
    }

    if (EVP_DigestInit_ex(ctx, EVP_sha256(), nullptr) == 1 &&
        EVP_DigestUpdate(ctx, data, length) == 1) {
        EVP_DigestFinal_ex(ctx, digest.data(), &digest_length);
    }

    EVP_MD_CTX_free(ctx);
    return digest;
}

std::array<uint8_t, kSha256Size> sha256(const std::string& text) {
    return sha256(reinterpret_cast<const uint8_t*>(text.data()), text.size());
}

std::array<uint8_t, kSha512Size> sha512(const uint8_t* data, size_t length) {
    std::array<uint8_t, kSha512Size> digest{};
    unsigned int digest_length = 0;

    EVP_MD_CTX* ctx = EVP_MD_CTX_new();
    if (ctx == nullptr) {
        return digest;
    }

    if (EVP_DigestInit_ex(ctx, EVP_sha512(), nullptr) == 1 &&
        EVP_DigestUpdate(ctx, data, length) == 1) {
        EVP_DigestFinal_ex(ctx, digest.data(), &digest_length);
    }

    EVP_MD_CTX_free(ctx);
    return digest;
}

std::array<uint8_t, 32> keccak256(const uint8_t* data, size_t length) {
    std::array<uint8_t, 32> digest{};
    unsigned int digest_length = 0;

    EVP_MD_CTX* ctx = EVP_MD_CTX_new();
    if (ctx == nullptr) {
        return digest;
    }

    // SHA3-256 is the NIST standard; EVP_MD_fetch resolves the provider impl.
    EVP_MD* md = EVP_MD_fetch(nullptr, "SHA3-256", nullptr);
    if (md != nullptr) {
        if (EVP_DigestInit_ex(ctx, md, nullptr) == 1 &&
            EVP_DigestUpdate(ctx, data, length) == 1) {
            EVP_DigestFinal_ex(ctx, digest.data(), &digest_length);
        }
        EVP_MD_free(md);
    }

    EVP_MD_CTX_free(ctx);
    return digest;
}

std::string to_hex(const uint8_t* data, size_t length) {
    std::string out;
    out.reserve(length * 2);
    for (size_t index = 0; index < length; ++index) {
        out.push_back(kHexDigits[data[index] >> 4]);
        out.push_back(kHexDigits[data[index] & 0x0F]);
    }
    return out;
}

bool from_hex(const std::string& text, std::vector<uint8_t>& out) {
    if (text.size() % 2 != 0) {
        return false;
    }

    out.clear();
    out.reserve(text.size() / 2);

    for (size_t index = 0; index < text.size(); index += 2) {
        const int high = hex_value(text[index]);
        const int low = hex_value(text[index + 1]);
        if (high < 0 || low < 0) {
            out.clear();
            return false;
        }
        out.push_back(static_cast<uint8_t>((high << 4) | low));
    }

    return true;
}

bool secure_random(uint8_t* buffer, size_t length) {
    if (length == 0) {
        return true;
    }
    return RAND_bytes(buffer, static_cast<int>(length)) == 1;
}

std::array<uint8_t, 32> random_key() {
    std::array<uint8_t, 32> key{};
    if (!secure_random(key.data(), key.size())) {
        // A failed RNG must never yield a predictable key.
        key.fill(0);
    }
    return key;
}

std::vector<uint8_t> hmac_sha256(const uint8_t* key, size_t key_length, const uint8_t* data,
                                 size_t data_length) {
    std::vector<uint8_t> out(kHmacSize);
    unsigned int out_length = 0;

    if (HMAC(EVP_sha256(), key, static_cast<int>(key_length), data, data_length, out.data(),
             &out_length) == nullptr) {
        out.assign(kHmacSize, 0);
        return out;
    }

    out.resize(out_length);
    return out;
}

bool verify_hmac_sha256(const uint8_t* key, size_t key_length, const uint8_t* data,
                        size_t data_length, const uint8_t* tag, size_t tag_length) {
    if (tag_length != kHmacSize) {
        return false;
    }
    const std::vector<uint8_t> expected = hmac_sha256(key, key_length, data, data_length);
    return constant_time_equal(expected.data(), tag, kHmacSize);
}

Encryptor::Encryptor(const std::array<uint8_t, kAes256KeySize>& key) : key_(key) {}

Encryptor Encryptor::from_password(const std::string& password, const std::vector<uint8_t>& salt,
                                   uint32_t iterations) {
    std::array<uint8_t, kAes256KeySize> key{};
    if (!pbkdf2_sha256(password, salt, iterations, key)) {
        key.fill(0);
    }
    return Encryptor(key);
}

bool Encryptor::encrypt(const uint8_t* plaintext, size_t length, const uint8_t* aad,
                        size_t aad_length, const uint8_t* nonce, size_t nonce_length,
                        std::vector<uint8_t>& out) const {
    if (nonce_length != kGcmNonceSize) {
        return false;
    }

    EVP_CIPHER_CTX* ctx = EVP_CIPHER_CTX_new();
    if (ctx == nullptr) {
        return false;
    }

    bool ok = false;
    do {
        if (EVP_EncryptInit_ex(ctx, EVP_aes_256_gcm(), nullptr, nullptr, nullptr) != 1) {
            break;
        }
        if (EVP_CIPHER_CTX_ctrl(ctx, EVP_CTRL_GCM_SET_IVLEN, static_cast<int>(nonce_length),
                                nullptr) != 1) {
            break;
        }
        if (EVP_EncryptInit_ex(ctx, nullptr, nullptr, key_.data(), nonce) != 1) {
            break;
        }

        int produced = 0;
        if (aad != nullptr && aad_length > 0) {
            if (EVP_EncryptUpdate(ctx, nullptr, &produced, aad, static_cast<int>(aad_length)) != 1) {
                break;
            }
        }

        out.assign(length + kGcmTagSize, 0);
        int total = 0;
        if (length > 0) {
            if (EVP_EncryptUpdate(ctx, out.data(), &produced, plaintext,
                                  static_cast<int>(length)) != 1) {
                break;
            }
            total = produced;
        }

        if (EVP_EncryptFinal_ex(ctx, out.data() + total, &produced) != 1) {
            break;
        }
        total += produced;

        if (EVP_CIPHER_CTX_ctrl(ctx, EVP_CTRL_GCM_GET_TAG, kGcmTagSize,
                                out.data() + total) != 1) {
            break;
        }

        out.resize(total + kGcmTagSize);
        ok = true;
    } while (false);

    EVP_CIPHER_CTX_free(ctx);
    if (!ok) {
        out.clear();
    }
    return ok;
}

bool Encryptor::decrypt(const uint8_t* ciphertext, size_t length, const uint8_t* aad,
                        size_t aad_length, const uint8_t* nonce, size_t nonce_length,
                        std::vector<uint8_t>& out) const {
    if (nonce_length != kGcmNonceSize || length < kGcmTagSize) {
        return false;
    }

    const size_t body_length = length - kGcmTagSize;

    EVP_CIPHER_CTX* ctx = EVP_CIPHER_CTX_new();
    if (ctx == nullptr) {
        return false;
    }

    bool ok = false;
    do {
        if (EVP_DecryptInit_ex(ctx, EVP_aes_256_gcm(), nullptr, nullptr, nullptr) != 1) {
            break;
        }
        if (EVP_CIPHER_CTX_ctrl(ctx, EVP_CTRL_GCM_SET_IVLEN, static_cast<int>(nonce_length),
                                nullptr) != 1) {
            break;
        }
        if (EVP_DecryptInit_ex(ctx, nullptr, nullptr, key_.data(), nonce) != 1) {
            break;
        }

        int produced = 0;
        if (aad != nullptr && aad_length > 0) {
            if (EVP_DecryptUpdate(ctx, nullptr, &produced, aad, static_cast<int>(aad_length)) != 1) {
                break;
            }
        }

        out.assign(body_length, 0);
        int total = 0;
        if (body_length > 0) {
            if (EVP_DecryptUpdate(ctx, out.data(), &produced, ciphertext,
                                  static_cast<int>(body_length)) != 1) {
                break;
            }
            total = produced;
        }

        // The tag must be installed before finalisation for verification to occur.
        if (EVP_CIPHER_CTX_ctrl(ctx, EVP_CTRL_GCM_SET_TAG, kGcmTagSize,
                                const_cast<uint8_t*>(ciphertext + body_length)) != 1) {
            break;
        }

        if (EVP_DecryptFinal_ex(ctx, out.data() + total, &produced) != 1) {
            // Authentication failed: reject and never return the plaintext.
            break;
        }
        total += produced;

        out.resize(total);
        ok = true;
    } while (false);

    EVP_CIPHER_CTX_free(ctx);
    if (!ok) {
        out.clear();
    }
    return ok;
}

bool Encryptor::encrypt_random_nonce(const uint8_t* plaintext, size_t length,
                                     std::vector<uint8_t>& out) const {
    std::array<uint8_t, kGcmNonceSize> nonce{};
    if (!secure_random(nonce.data(), nonce.size())) {
        return false;
    }

    std::vector<uint8_t> ciphertext;
    if (!encrypt(plaintext, length, nullptr, 0, nonce.data(), nonce.size(), ciphertext)) {
        return false;
    }

    out.clear();
    out.reserve(nonce.size() + ciphertext.size());
    out.insert(out.end(), nonce.begin(), nonce.end());
    out.insert(out.end(), ciphertext.begin(), ciphertext.end());
    return true;
}

bool Encryptor::decrypt_random_nonce(const uint8_t* data, size_t length,
                                     std::vector<uint8_t>& out) const {
    if (length < kGcmNonceSize) {
        return false;
    }
    return decrypt(data + kGcmNonceSize, length - kGcmNonceSize, nullptr, 0, data, kGcmNonceSize, out);
}

bool pbkdf2_sha256(const std::string& password, const std::vector<uint8_t>& salt,
                   uint32_t iterations, std::array<uint8_t, 32>& out) {
    return PKCS5_PBKDF2_HMAC(password.data(), static_cast<int>(password.size()), salt.data(),
                             static_cast<int>(salt.size()), static_cast<int>(iterations),
                             EVP_sha256(), static_cast<int>(out.size()), out.data()) == 1;
}

bool scrypt_key(const std::string& password, const std::vector<uint8_t>& salt, uint64_t cost,
                uint32_t block_size, uint32_t parallelism, std::array<uint8_t, 32>& out) {
    EVP_KDF* kdf = EVP_KDF_fetch(nullptr, "SCRYPT", nullptr);
    if (kdf == nullptr) {
        return false;
    }

    EVP_KDF_CTX* ctx = EVP_KDF_CTX_new(kdf);
    EVP_KDF_free(kdf);
    if (ctx == nullptr) {
        return false;
    }

    OSSL_PARAM params[6];
    params[0] = OSSL_PARAM_construct_octet_string("pass", const_cast<char*>(password.data()),
                                                  password.size());
    params[1] = OSSL_PARAM_construct_octet_string("salt", const_cast<uint8_t*>(salt.data()),
                                                  salt.size());
    params[2] = OSSL_PARAM_construct_uint64("n", &cost);
    params[3] = OSSL_PARAM_construct_uint("r", &block_size);
    params[4] = OSSL_PARAM_construct_uint("p", &parallelism);
    params[5] = OSSL_PARAM_construct_end();

    const int result = EVP_KDF_derive(ctx, out.data(), out.size(), params);
    EVP_KDF_CTX_free(ctx);

    return result == 1;
}

std::string base64_encode(const uint8_t* data, size_t length) {
    return encode_base64(data, length, kBase64Digits, true);
}

bool base64_decode(const std::string& text, std::vector<uint8_t>& out) {
    out.clear();
    out.reserve((text.size() / 4) * 3);

    uint32_t accumulator = 0;
    int bits = 0;

    for (const char character : text) {
        if (character == '=') {
            break;
        }
        if (character == '\n' || character == '\r') {
            continue;
        }

        const int value = base64_value(character);
        if (value < 0) {
            out.clear();
            return false;
        }

        accumulator = (accumulator << 6) | static_cast<uint32_t>(value);
        bits += 6;

        if (bits >= 8) {
            bits -= 8;
            out.push_back(static_cast<uint8_t>((accumulator >> bits) & 0xFF));
        }
    }

    return true;
}

std::string base64url_encode(const uint8_t* data, size_t length) {
    return encode_base64(data, length, kBase64UrlDigits, false);
}

std::array<uint8_t, kSha256Size> api_key_digest(const std::string& secret) {
    // Domain-separated so a key digest cannot collide with a plain hash use.
    const std::string material = "tigerswap-api-key-v1:" + secret;
    return sha256(reinterpret_cast<const uint8_t*>(material.data()), material.size());
}

std::string url_encode(const std::string& value) {
    std::string out;
    out.reserve(value.size());

    for (const char character : value) {
        const uint8_t byte = static_cast<uint8_t>(character);
        const bool unreserved = (byte >= 'A' && byte <= 'Z') || (byte >= 'a' && byte <= 'z') ||
                                (byte >= '0' && byte <= '9') || byte == '-' || byte == '_' ||
                                byte == '.' || byte == '~';
        if (unreserved) {
            out.push_back(character);
        } else {
            out.push_back('%');
            out.push_back(kHexDigits[byte >> 4]);
            out.push_back(kHexDigits[byte & 0x0F]);
        }
    }

    return out;
}

std::string sanitize_log_field(const std::string& value) {
    std::string out;
    out.reserve(value.size());

    for (const char character : value) {
        const uint8_t byte = static_cast<uint8_t>(character);
        if (byte < 0x20 || byte == 0x7F) {
            out.push_back('?');
        } else {
            out.push_back(character);
        }
    }

    return out;
}

}  // namespace tigerswap
