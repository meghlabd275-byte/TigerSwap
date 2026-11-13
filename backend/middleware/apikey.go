package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// ParseAPIKey extracts a key from the X-API-Key header, falling back to the
// Authorization: Bearer scheme, then a query parameter. The header wins so an
// explicit credential is never overridden by a query string.
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

// MaskKey renders a key for logs without exposing it. Only a short prefix and a
// hash fragment are shown, so a log line cannot leak a usable credential.
func MaskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	hash := sha256.Sum256([]byte(key))
	return key[:4] + "..." + hex.EncodeToString(hash[:4])
}
