package services

import (
	"log"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// revocationStore keeps the set of revoked JWT identifiers. Revocations are
// written to the database so they survive a restart, and mirrored in memory so
// the token validation path does not hit the database on every request.
//
// It also tracks a per-user "revoked before" timestamp so deactivating an
// account invalidates every session that account already holds, not just the
// one token that was logged out.
type revocationStore struct {
	db *gorm.DB

	mu     sync.RWMutex
	byJTI  map[string]time.Time
	byUser map[string]time.Time
}

// userRevocationJTI is the sentinel primary key used to persist a user-level
// revocation in the same table as per-token revocations.
func userRevocationJTI(userID string) string {
	return "user:" + userID
}

func newRevocationStore(db *gorm.DB) *revocationStore {
	return &revocationStore{
		db:     db,
		byJTI:  make(map[string]time.Time),
		byUser: make(map[string]time.Time),
	}
}

// load pulls unexpired revocations into memory and prunes expired rows.
func (s *revocationStore) load() {
	if s.db == nil {
		return
	}
	now := time.Now()
	if err := s.db.Where("expires_at < ?", now).Delete(&models.RevokedToken{}).Error; err != nil {
		log.Printf("failed to prune revoked tokens: %v", err)
	}

	var rows []models.RevokedToken
	if err := s.db.Where("expires_at >= ?", now).Find(&rows).Error; err != nil {
		log.Printf("failed to load revoked tokens: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, row := range rows {
		if strings.HasPrefix(row.JTI, "user:") {
			// The cutoff is the moment the user was revoked, not the sentinel's
			// expiry; a token issued after that moment stays valid.
			s.byUser[strings.TrimPrefix(row.JTI, "user:")] = row.RevokedAt
			continue
		}
		s.byJTI[row.JTI] = row.ExpiresAt
	}
}

// isRevoked reports whether a token identifier has been revoked.
func (s *revocationStore) isRevoked(jti string) bool {
	s.mu.RLock()
	expiry, found := s.byJTI[jti]
	s.mu.RUnlock()
	if !found {
		return false
	}
	// A revocation that outlives the token itself is irrelevant; drop it.
	if time.Now().After(expiry) {
		s.mu.Lock()
		delete(s.byJTI, jti)
		s.mu.Unlock()
		return false
	}
	return true
}

// revoke records a token identifier as revoked until expiresAt.
func (s *revocationStore) revoke(jti string, expiresAt time.Time) error {
	if s.db != nil {
		row := models.RevokedToken{
			JTI:       jti,
			ExpiresAt: expiresAt,
			RevokedAt: time.Now(),
		}
		if err := s.db.Create(&row).Error; err != nil {
			return err
		}
	}

	s.mu.Lock()
	s.byJTI[jti] = expiresAt
	s.mu.Unlock()
	return nil
}

// isUserRevoked reports whether a user-level revocation was recorded at or after
// the token's issue time, which means the session predates the revocation and
// must be refused. issuedAt is the token's `iat` claim in Unix seconds.
func (s *revocationStore) isUserRevoked(userID string, issuedAt time.Time) bool {
	s.mu.RLock()
	revokedAt, found := s.byUser[userID]
	s.mu.RUnlock()
	if !found {
		return false
	}
	// A token issued in the same second as the revocation is treated as revoked,
	// because the two are indistinguishable at second resolution.
	return !issuedAt.After(revokedAt)
}

// revokeUser invalidates every token a user already holds. It is used when an
// account is deactivated so an already-issued session cannot outlive the ban.
// The record is persisted under a sentinel key with a long horizon; a fresh
// token issued after the ban carries a later `iat` and therefore stays valid.
func (s *revocationStore) revokeUser(userID string) error {
	now := time.Now()
	if s.db != nil {
		row := models.RevokedToken{
			JTI:       userRevocationJTI(userID),
			UserID:    userID,
			ExpiresAt: now.Add(100 * 365 * 24 * time.Hour),
			RevokedAt: now,
		}
		// Upsert so re-banning a user refreshes the cutoff rather than failing on
		// the primary-key conflict.
		if err := s.db.Where("jti = ?", row.JTI).
			Assign(map[string]interface{}{"revoked_at": row.RevokedAt, "expires_at": row.ExpiresAt}).
			FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}

	s.mu.Lock()
	s.byUser[userID] = now
	s.mu.Unlock()
	return nil
}
