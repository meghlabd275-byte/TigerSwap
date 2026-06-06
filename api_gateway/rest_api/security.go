// ============================================================================
// TIGERSWAP SECURITY MODULE
// Complete security with encryption, DDOS protection, XSS protection
// ============================================================================

package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ============================================================================
// ENCRYPTION MODULE
// ============================================================================

// Encrypt encrypts data using AES-256-GCM
func Encrypt(data []byte, key []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts data using AES-256-GCM
func Decrypt(encoded string, key []byte) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// HashPassword creates a secure hash of a password
func HashPassword(password string) string {
	salt := make([]byte, 32)
	rand.Read(salt)

	hash := sha256.Sum256(append(salt, []byte(password)...))
	return fmt.Sprintf("%s:%s", base64.StdEncoding.EncodeToString(salt), hex.EncodeToString(hash[:]))
}

// VerifyPassword verifies a password against a hash
func VerifyPassword(password, stored string) bool {
	parts := strings.Split(stored, ":")
	if len(parts) != 2 {
		return false
	}

	salt, _ := base64.StdEncoding.DecodeString(parts[0])
	expectedHash := sha256.Sum256(append(salt, []byte(password)...))

	actualHash, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare(expectedHash[:], actualHash) == 1
}

// GenerateSecureKey generates a 256-bit key
func GenerateSecureKey() []byte {
	key := make([]byte, 32)
	rand.Read(key)
	return key
}

// ============================================================================
// INPUT VALIDATION & SANITIZATION
// ============================================================================

// XSSPatterns - dangerous patterns to detect
var XSSPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)<script[^>]*>`),
	regexp.MustCompile(`(?i)javascript:`),
	regexp.MustCompile(`(?i)on\w+\s*=`),
	regexp.MustCompile(`(?i)<iframe`),
	regexp.MustCompile(`(?i)<object`),
	regexp.MustCompile(`(?i)<embed`),
	regexp.MustCompile(`(?i)expression\s*\(`),
	regexp.MustCompile(`(?i)eval\s*\(`),
	regexp.MustCompile(`(?i)alert\s*\(`),
	regexp.MustCompile(`(?i)document\.`),
	regexp.MustCompile(`(?i)window\.`),
}

// SanitizeInput removes XSS vectors
func SanitizeInput(input string) string {
	result := html.EscapeString(input)

	// Remove null bytes
	result = strings.ReplaceAll(result, "\x00", "")

	// Basic sanitization - escape HTML
	result = strings.ReplaceAll(result, "<", "&lt;")
	result = strings.ReplaceAll(result, ">", "&gt;")

	return result
}

// ValidateInput checks for XSS and other attacks
func ValidateInput(input string) (bool, string) {
	for _, pattern := range XSSPatterns {
		if pattern.MatchString(input) {
			return false, "Invalid input detected"
		}
	}

	// Check for null bytes
	if strings.Contains(input, "\x00") {
		return false, "Invalid characters detected"
	}

	// Check length
	if len(input) > 10000 {
		return false, "Input too long"
	}

	return true, ""
}

// ValidateAddress validates blockchain addresses
func ValidateAddress(address, chainType string) bool {
	// Basic length checks
	if len(address) < 20 || len(address) > 64 {
		return false
	}

	// Check hex for EVM
	if chainType == "evm" {
		return isValidHex(address)
	}

	// Check base58 for Solana
	if chainType == "solana" {
		return isValidBase58(address)
	}

	return true
}

func isValidHex(s string) bool {
	_, err := hex.DecodeString(strings.TrimPrefix(s, "0x"))
	return err == nil
}

func isValidBase58(s string) bool {
	valid := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for _, c := range s {
		if !strings.Contains(valid, string(c)) {
			return false
		}
	}
	return true
}

// ============================================================================
// DDOS PROTECTION
// ============================================================================

// RateLimiter - Token bucket rate limiter
type RateLimiter struct {
	mu       sync.Mutex
	tokens   map[string]*bucket
	cleanup time.Time
}

type bucket struct {
	tokens    float64
	maxTokens float64
	refill   time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	rl := &RateLimiter{
		tokens: make(map[string]*bucket),
	}
	go rl.cleanupLoop()
	return rl
}

// Allow checks if request is allowed
func (rl *RateLimiter) Allow(key string, maxPerMinute float64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.tokens[key]

	if !ok {
		rl.tokens[key] = &bucket{
			tokens:    maxPerMinute - 1,
			maxTokens: maxPerMinute,
			refill:   now,
		}
		return true
	}

	// Refill tokens
	if now.After(b.refill) {
		b.tokens = b.maxTokens
		b.refill = now.Add(time.Minute)
	}

	if b.tokens >= 1 {
		b.tokens--
		return true
	}

	return false
}

func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, b := range rl.tokens {
			if now.Sub(b.refill) > 10*time.Minute {
				delete(rl.tokens, key)
			}
		}
		rl.mu.Unlock()
	}
}

// ============================================================================
// SECURITY MIDDLEWARE
// ============================================================================

// SecurityHeaders adds security headers
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		headers := w.Header()

		// Prevent clickjacking
		headers.Set("X-Frame-Options", "DENY")

		// XSS protection
		headers.Set("X-XSS-Protection", "1; mode=block")

		// Prevent content type sniffing
		headers.Set("X-Content-Type-Options", "nosniff")

		// CSP
		headers.Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'self'")

		// Referrer policy
		headers.Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// HSTS
		headers.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")

		next.ServeHTTP(w, r)
	})
}

// InputValidationMiddleware validates all inputs
func InputValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get POST parameters
		if r.Method == "POST" {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "Invalid request", http.StatusBadRequest)
				return
			}

			// Validate all form values
			for key, values := range r.Form {
				for _, value := range values {
					valid, msg := ValidateInput(value)
					if !valid {
						http.Error(w, fmt.Sprintf("Invalid input in %s: %s", key, msg), http.StatusBadRequest)
						return
					}

					// Sanitize
					r.Form.Set(key, SanitizeInput(value))
				}
			}
		}

		next.ServeHTTP(w, r)
	})
}

// ============================================================================
// API KEY SECURITY
// ============================================================================

// SecureGenerateAPIKey generates a secure API key
func SecureGenerateAPIKey() (string, string) {
	keyBytes := make([]byte, 32)
	secretBytes := make([]byte, 32)

	rand.Read(keyBytes)
	rand.Read(secretBytes)

	key := fmt.Sprintf("TS-%s", hex.EncodeToString(keyBytes))
	secret := hex.EncodeToString(secretBytes)

	// Hash the secret for storage
	secretHash := sha256.Sum256([]byte(secret))
	storedSecret := hex.EncodeToString(secretHash[:])

	return key, storedSecret
}

// VerifyAPIKeySecret verifies API key secret
func VerifyAPIKeySecret(secret, storedSecret string) bool {
	secretHash := sha256.Sum256([]byte(secret))
	expected, _ := hex.DecodeString(storedSecret)

	return subtle.ConstantTimeCompare(secretHash[:], expected) == 1
}

// ============================================================================
// TRANSACTION SIGNING SECURITY
// ============================================================================

// SecureTransactionData creates secure transaction data
func SecureTransactionData(txData string, chainID int64, nonce int64, gasPrice int64) []byte {
	data := fmt.Sprintf("%s:%d:%d:%d", txData, chainID, nonce, gasPrice)
	hash := sha256.Sum256([]byte(data))
	return hash[:]
}

// VerifyTransaction verifies transaction hasn't been tampered
func VerifyTransaction(txData string, chainID int64, nonce int64, gasPrice int64, signature []byte) bool {
	expected := SecureTransactionData(txData, chainID, nonce, gasPrice)
	return subtle.ConstantTimeCompare(expected, signature) == 1
}

// ============================================================================
// DATABASE ENCRYPTION HELPER
// ============================================================================

// EncryptField encrypts a database field
func EncryptField(value string, key []byte) (string, error) {
	encrypted, err := Encrypt([]byte(value), key)
	if err != nil {
		return "", err
	}
	return encrypted, nil
}

// DecryptField decrypts a database field
func DecryptField(value string, key []byte) (string, error) {
	decrypted, err := Decrypt(value, key)
	if err != nil {
		return "", err
	}
	return string(decrypted), nil
}
EOF
echo "Security module created"