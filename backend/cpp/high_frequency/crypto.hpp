/**
 * TigerSwap High-Performance Cryptographic Operations
 * Hardware-accelerated encryption for ultra-low latency
 */

#ifndef TIGERSWAP_CRYPTO_HPP
#define TIGERSWAP_CRYPTO_HPP

#include <iostream>
#include <vector>
#include <array>
#include <cstring>
#include <openssl/evp.h>
#include <openssl/rand.h>
#include <openssl/hmac.h>
#include <openssl/sha.h>
#include <openssl/ripemd.h>
#include <openssl/ec.h>
#include <openssl/obj_mac.h>
#include <openssl/bn.h>

// ============== CONSTANTS ==============
constexpr size_t SHA256_SIZE = 32;
constexpr size_t SHA512_SIZE = 64;
constexpr size_t AES_BLOCK_SIZE = 16;
constexpr size_t AES_256_KEY_SIZE = 32;
constexpr size_t HMAC_SIZE = 32;
constexpr size_t EC_SIGNATURE_SIZE = 72;

// ============== HASH FUNCTIONS ==============

class SHA256 {
private:
    EVP_MD_CTX* ctx_;
    
public:
    SHA256() {
        ctx_ = EVP_MD_CTX_new();
    }
    
    ~SHA256() {
        if (ctx_) EVP_MD_CTX_free(ctx_);
    }
    
    void init() {
        EVP_DigestInit_ex(ctx_, EVP_sha256(), nullptr);
    }
    
    void update(const uint8_t* data, size_t len) {
        EVP_DigestUpdate(ctx_, data, len);
    }
    
    void final(uint8_t* hash) {
        unsigned int len = SHA256_SIZE;
        EVP_DigestFinal_ex(ctx_, hash, &len);
        EVP_DigestInit_ex(ctx_, EVP_sha256(), nullptr);
    }
    
    static std::array<uint8_t, SHA256_SIZE> hash(const uint8_t* data, size_t len) {
        SHA256 hasher;
        hasher.init();
        hasher.update(data, len);
        std::array<uint8_t, SHA256_SIZE> hash;
        hasher.final(hash.data());
        return hash;
    }
};

class SHA512 {
private:
    EVP_MD_CTX* ctx_;
    
public:
    SHA512() {
        ctx_ = EVP_MD_CTX_new();
    }
    
    ~SHA512() {
        if (ctx_) EVP_MD_CTX_free(ctx_);
    }
    
    void init() {
        EVP_DigestInit_ex(ctx_, EVP_sha512(), nullptr);
    }
    
    void update(const uint8_t* data, size_t len) {
        EVP_DigestUpdate(ctx_, data, len);
    }
    
    void final(uint8_t* hash) {
        unsigned int len = SHA512_SIZE;
        EVP_DigestFinal_ex(ctx_, hash, &len);
    }
    
    static std::array<uint8_t, SHA512_SIZE> hash(const uint8_t* data, size_t len) {
        SHA512 hasher;
        hasher.init();
        hasher.update(data, len);
        std::array<uint8_t, SHA512_SIZE> hash;
        hasher.final(hash.data());
        return hash;
    }
};

// ============== HMAC ==============

class HMAC_SHA256 {
private:
    std::array<uint8_t, AES_256_KEY_SIZE> key_;
    
public:
    void setKey(const uint8_t* key, size_t len) {
        // If key is longer than block size, hash it
        if (len > AES_BLOCK_SIZE) {
            auto h = SHA256::hash(key, len);
            key_ = h;
        } else {
            std::memcpy(key_.data(), key, len);
        }
    }
    
    std::array<uint8_t, HMAC_SIZE> sign(const uint8_t* data, size_t len) {
        std::array<uint8_t, HMAC_SIZE> result;
        
        HMAC(EVP_sha256(), key_.data(), key_.size(), data, len, result.data(), nullptr);
        
        return result;
    }
    
    bool verify(const uint8_t* data, size_t len, const uint8_t* signature) {
        auto computed = sign(data, len);
        return std::memcmp(computed.data(), signature, HMAC_SIZE) == 0;
    }
};

// ============== AES-256-GCM ==============

class AES256GCM {
private:
    std::array<uint8_t, AES_256_KEY_SIZE> key_;
    std::array<uint8_t, 12> nonce_;
    EVP_CIPHER_CTX* ctx_;
    
public:
    AES256GCM() {
        ctx_ = EVP_CIPHER_CTX_new();
    }
    
    ~AES256GCM() {
        if (ctx_) EVP_CIPHER_CTX_free(ctx_);
    }
    
    void setKey(const uint8_t* key) {
        std::memcpy(key_.data(), key, AES_256_KEY_SIZE);
    }
    
    void setNonce(const uint8_t* nonce) {
        std::memcpy(nonce_.data(), nonce, 12);
    }
    
    // Encrypt with AEAD (authenticates additional data)
    std::vector<uint8_t> encrypt(const uint8_t* plaintext, size_t len, const uint8_t* aad = nullptr, size_t aad_len = 0) {
        std::vector<uint8_t> ciphertext(len + AES_BLOCK_SIZE + 16);
        int out_len = 0;
        
        EVP_EncryptInit_ex(ctx_, EVP_aes_256_gcm(), nullptr, nullptr, nullptr);
        EVP_CIPHER_CTX_ctrl(ctx_, EVP_CTRL_GCM_SET_IVLEN, 12, nullptr);
        EVP_EncryptInit_ex(ctx_, nullptr, nullptr, key_.data(), nonce_.data());
        
        // Add additional authenticated data
        if (aad && aad_len > 0) {
            EVP_EncryptUpdate(ctx_, nullptr, &out_len, aad, aad_len);
        }
        
        // Encrypt
        EVP_EncryptUpdate(ctx_, ciphertext.data(), &out_len, plaintext, len);
        int total_len = out_len;
        
        // Finalize (adds auth tag)
        EVP_EncryptFinal_ex(ctx_, ciphertext.data() + out_len, &out_len);
        total_len += out_len;
        
        // Get auth tag
        std::array<uint8_t, 16> tag;
        EVP_CIPHER_CTX_ctrl(ctx_, EVP_CTRL_GCM_GET_TAG, 16, tag.data());
        
        // Append tag to ciphertext
        ciphertext.resize(total_len + 16);
        std::memcpy(ciphertext.data() + total_len, tag.data(), 16);
        
        return ciphertext;
    }
    
    // Decrypt with AEAD verification
    std::vector<uint8_t> decrypt(const uint8_t* ciphertext, size_t len, const uint8_t* aad = nullptr, size_t aad_len = 0) {
        if (len < 16) return {};
        
        std::vector<uint8_t> plaintext(len);
        int out_len = 0;
        int total_len = 0;
        
        // Extract tag
        std::array<uint8_t, 16> tag;
        std::memcpy(tag.data(), ciphertext + len - 16, 16);
        
        EVP_DecryptInit_ex(ctx_, EVP_aes_256_gcm(), nullptr, nullptr, nullptr);
        EVP_CIPHER_CTX_ctrl(ctx_, EVP_CTRL_GCM_SET_IVLEN, 12, nullptr);
        EVP_DecryptInit_ex(ctx_, nullptr, nullptr, key_.data(), nonce_.data());
        
        // Set expected tag
        EVP_CIPHER_CTX_ctrl(ctx_, EVP_CTRL_GCM_SET_TAG, 16, tag.data());
        
        // Add additional authenticated data
        if (aad && aad_len > 0) {
            EVP_DecryptUpdate(ctx_, nullptr, &out_len, aad, aad_len);
        }
        
        // Decrypt
        size_t ct_len = len - 16;
        EVP_DecryptUpdate(ctx_, plaintext.data(), &out_len, ciphertext, ct_len);
        total_len = out_len;
        
        // Finalize and verify tag
        int ret = EVP_DecryptFinal_ex(ctx_, plaintext.data() + out_len, &out_len);
        
        if (ret > 0) {
            total_len += out_len;
            plaintext.resize(total_len);
            return plaintext;
        }
        
        return {};
    }
};

// ============== ECDSA ==============

class ECDSASigner {
private:
    EC_KEY* key_;
    int curve_;
    
public:
    ECDSASigner(int curve = NID_secp256k1) : curve_(curve) {
        key_ = EC_KEY_new_by_curve_name(curve);
        EC_KEY_generate_key(key_);
    }
    
    ~ECDSASigner() {
        if (key_) EC_KEY_free(key_);
    }
    
    std::vector<uint8_t> sign(const uint8_t* msg, size_t len) {
        // Hash message
        auto hash = SHA256::hash(msg, len);
        
        std::vector<uint8_t> signature(ECDSA_size(key_));
        unsigned int sig_len = 0;
        
        ECDSA_sign(0, hash.data(), hash.size(), signature.data(), &sig_len, key_);
        signature.resize(sig_len);
        
        return signature;
    }
    
    bool verify(const uint8_t* msg, size_t len, const uint8_t* sig, size_t sig_len) {
        auto hash = SHA256::hash(msg, len);
        return ECDSA_verify(0, hash.data(), hash.size(), sig, sig_len, key_) == 1;
    }
    
    std::array<uint8_t, 64> getPublicKey() {
        const EC_POINT* point = EC_KEY_get0_public_key(key_);
        const EC_GROUP* group = EC_KEY_get0_group(key_);
        
        std::array<uint8_t, 64> pubkey;
        BN_CTX* ctx = BN_CTX_new();
        
        EC_POINT_point2oct(group, point, POINT_CONVERSION_UNCOMPRESSED, pubkey.data(), 64 + 1, ctx);
        
        BN_CTX_free(ctx);
        
        return pubkey;
    }
};

// ============== KEY DERIVATION ==============

class PBKDF2 {
public:
    static std::vector<uint8_t> derive(const char* password, size_t pass_len,
                                         const uint8_t* salt, size_t salt_len,
                                         uint32_t iterations = 100000,
                                         size_t key_len = 32) {
        std::vector<uint8_t> key(key_len);
        
        PKCS5_PBKDF2_HMAC(password, pass_len, salt, salt_len, iterations, EVP_sha256(), key_len, key.data());
        
        return key;
    }
};

class Scrypt {
public:
    static std::vector<uint8_t> derive(const char* password, size_t pass_len,
                                        const uint8_t* salt, size_t salt_len,
                                        uint64_t N = 16384, uint32_t r = 8, uint32_t p = 1,
                                        size_t key_len = 32) {
        std::vector<uint8_t> key(key_len);
        
        // Note: OpenSSL 1.1+ required for scrypt
        // EVP_KDF with scrypt would be used here
        // For now, use PBKDF2 as fallback
        return PBKDF2::derive(password, pass_len, salt, salt_len, iterations, key_len);
    }
};

// ============== RANDOM ==============

class SecureRandom {
public:
    static void bytes(uint8_t* buffer, size_t len) {
        RAND_bytes(buffer, len);
    }
    
    static uint32_t uint32() {
        uint32_t val;
        RAND_bytes((uint8_t*)&val, 4);
        return val;
    }
    
    static uint64_t uint64() {
        uint64_t val;
        RAND_bytes((uint8_t*)&val, 8);
        return val;
    }
};

// ============== FFI ==============
extern "C" {

void* create_aes256() {
    return new AES256GCM();
}

void destroy_aes256(void* aes) {
    delete static_cast<AES256GCM*>(aes);
}

void aes256_set_key(void* aes, const uint8_t* key) {
    static_cast<AES256GCM*>(aes)->setKey(key);
}

void aes256_set_nonce(void* aes, const uint8_t* nonce) {
    static_cast<AES256GCM*>(aes)->setNonce(nonce);
}

int aes256_encrypt(void* aes, const uint8_t* in, size_t len, uint8_t* out) {
    auto result = static_cast<AES256GCM*>(aes)->encrypt(in, len);
    if (result.size() > 0) {
        std::memcpy(out, result.data(), result.size());
        return result.size();
    }
    return -1;
}

int aes256_decrypt(void* aes, const uint8_t* in, size_t len, uint8_t* out) {
    auto result = static_cast<AES256GCM*>(aes)->decrypt(in, len);
    if (result.size() > 0) {
        std::memcpy(out, result.data(), result.size());
        return result.size();
    }
    return -1;
}

void sha256_hash(const uint8_t* data, size_t len, uint8_t* out) {
    auto h = SHA256::hash(data, len);
    std::memcpy(out, h.data(), 32);
}

void sha512_hash(const uint8_t* data, size_t len, uint8_t* out) {
    auto h = SHA512::hash(data, len);
    std::memcpy(out, h.data(), 64);
}

void secure_random(uint8_t* buffer, size_t len) {
    SecureRandom::bytes(buffer, len);
}

} // extern "C"

#endif // TIGERSWAP_CRYPTO_HPP
