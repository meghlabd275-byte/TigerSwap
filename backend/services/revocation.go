package services

import (
	"log"
	"sync"
	"time"

	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// revocationStore keeps the set of revoked JWT identifiers. Revocations are
// written to the database so they survive a restart, and mirrored in memory so
// the token validation path does not hit the database on every request.
type revocationStore struct {
	db *gorm.DB

	mu    sync.RWMutex
	byJTI map[string]time.Time
}

func newRevocationStore(db *gorm.DB) *revocationStore {
	return &revocationStore{
		db:    db,
		byJTI: make(map[string]time.Time),
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
