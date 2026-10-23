package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strings"
	"sync"
	"time"
)

// APIKey is a stored credential. Only the hash is kept, so a database or
// memory dump does not disclose usable keys.
type APIKey struct {
	Hash       [32]byte
	Label      string
	Scopes     map[string]bool
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastUsedAt time.Time
	Revoked    bool
}

// HasScope reports whether the key is allowed to perform `scope`.
func (k *APIKey) HasScope(scope string) bool {
	if k.Scopes == nil {
		return false
	}
	if k.Scopes["*"] {
		return true
	}
	return k.Scopes[scope]
}

// Expired reports whether the key is past its expiry. A zero ExpiresAt means
// the key does not expire.
func (k *APIKey) Expired(now time.Time) bool {
	return !k.ExpiresAt.IsZero() && now.After(k.ExpiresAt)
}

// ErrUnknownKey is returned when no stored key matches the presented value.
var ErrUnknownKey = errors.New("unknown api key")

// APIKeyStore validates API keys. It is safe for concurrent use.
type APIKeyStore struct {
	mu   sync.RWMutex
	keys map[[32]byte]*APIKey
}

// NewAPIKeyStore returns an empty store.
func NewAPIKeyStore() *APIKeyStore {
	return &APIKeyStore{keys: make(map[[32]byte]*APIKey)}
}

// HashKey derives the storage hash for a presented key.
func HashKey(key string) [32]byte {
	return sha256.Sum256([]byte(key))
}

// Add registers a key with the given scopes. It returns the hash so the caller
// can revoke the key later without retaining the plaintext.
func (s *APIKeyStore) Add(key, label string, scopes []string, ttl time.Duration) [32]byte {
	hash := HashKey(key)

	scopeSet := make(map[string]bool, len(scopes))
	for _, scope := range scopes {
		scopeSet[scope] = true
	}

	entry := &APIKey{
		Hash:      hash,
		Label:     label,
		Scopes:    scopeSet,
		CreatedAt: time.Now(),
	}
	if ttl > 0 {
		entry.ExpiresAt = entry.CreatedAt.Add(ttl)
	}

	s.mu.Lock()
	s.keys[hash] = entry
	s.mu.Unlock()

	return hash
}

// Revoke marks a key unusable.
func (s *APIKeyStore) Revoke(hash [32]byte) {
	s.mu.Lock()
	if entry, ok := s.keys[hash]; ok {
		entry.Revoked = true
	}
	s.mu.Unlock()
}

// Validate checks a presented key and returns the stored entry.
//
// The lookup hashes the candidate and compares hashes with a constant-time
// comparison, so timing does not leak information about stored keys.
func (s *APIKeyStore) Validate(presented string) (*APIKey, error) {
	if presented == "" {
		return nil, ErrUnknownKey
	}

	candidate := HashKey(presented)
	now := time.Now()

	s.mu.RLock()
	entry, ok := s.keys[candidate]
	s.mu.RUnlock()

	if !ok {
		// Burn comparable time so a miss is not faster than a hit.
		subtle.ConstantTimeCompare(candidate[:], candidate[:])
		return nil, ErrUnknownKey
	}
	if subtle.ConstantTimeCompare(entry.Hash[:], candidate[:]) != 1 {
		return nil, ErrUnknownKey
	}
	if entry.Revoked {
		return nil, ErrUnknownKey
	}
	if entry.Expired(now) {
		return nil, ErrUnknownKey
	}

	s.mu.Lock()
	entry.LastUsedAt = now
	s.mu.Unlock()

	return entry, nil
}

// ParseAPIKey extracts a key from the X-API-Key header, falling back to the
// Authorization: Bearer scheme.
func ParseAPIKey(header, authorization, query string) string {
	if header != "" {
		return strings.TrimSpace(header)
	}

	if authorization != "" {
		const prefix = "Bearer "
		if len(authorization) > len(prefix) && strings.EqualFold(authorization[:len(prefix)], prefix) {
			return strings.TrimSpace(authorization[len(prefix):])
		}
	}

	return strings.TrimSpace(query)
}

// MaskKey renders a key for logs without exposing it.
func MaskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	hash := HashKey(key)
	return key[:4] + "..." + hex.EncodeToString(hash[:4])
}
