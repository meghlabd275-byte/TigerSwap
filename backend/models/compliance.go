package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Compliance: identity verification, sanctions screening and geographic
// restriction.
//
// A platform that lets an unverified account move funds cannot be operated
// commercially, so the controls are explicit and enforced on the mutating
// paths rather than left to the operator. Screening is deterministic against a
// denylist the operator maintains; nothing is inferred from a third party and a
// screening failure denies the action instead of allowing it.

// KYC verification statuses. An account starts unverified; a submission moves
// it to pending; an operator approves or rejects it.
const (
	KycStatusUnverified = "unverified"
	KycStatusPending    = "pending"
	KycStatusApproved   = "approved"
	KycStatusRejected   = "rejected"
)

// KycProfile is one account's identity record. The document number is never
// stored in the clear: only its SHA-256 digest and the last four characters are
// kept, so a database read cannot recover the document while an operator can
// still match a resubmission.
type KycProfile struct {
	ID     uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	UserID string    `gorm:"uniqueIndex;size:100" json:"user_id"`

	Status string `gorm:"index;size:20;default:'unverified'" json:"status"`

	// FullName and Country are the declared identity. Country is an ISO 3166-1
	// alpha-2 code, upper-cased, and is what the geographic check reads.
	FullName string `gorm:"size:200" json:"full_name"`
	Country  string `gorm:"index;size:2" json:"country"`

	DocumentType   string `gorm:"size:32" json:"document_type"`
	DocumentHash   string `gorm:"size:64" json:"-"`
	DocumentLast4  string `gorm:"size:4" json:"document_last4,omitempty"`
	DateOfBirth    string `gorm:"size:10" json:"date_of_birth,omitempty"`

	// ReviewerID, ReviewedAt and RejectionReason record the operator decision.
	ReviewerID      *uuid.UUID `gorm:"type:uuid" json:"reviewer_id,omitempty"`
	ReviewedAt      *time.Time `json:"reviewed_at,omitempty"`
	RejectionReason string     `gorm:"size:500" json:"rejection_reason,omitempty"`

	// SubmittedAt is when the applicant last submitted, which is what an operator
	// sorts the review queue by.
	SubmittedAt time.Time      `json:"submitted_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

func (k *KycProfile) BeforeCreate(tx *gorm.DB) error {
	if k.ID == uuid.Nil {
		k.ID = uuid.New()
	}
	return nil
}

// Sanctions entry kinds. An address is screened for a withdrawal destination, a
// country for geographic restriction, and a name for an identity match.
const (
	SanctionsKindAddress = "address"
	SanctionsKindCountry = "country"
	SanctionsKindName    = "name"
)

// SanctionsEntry is one denylisted value. An entry is unique per (kind, value)
// so re-adding the same value is a no-op rather than a duplicate row.
type SanctionsEntry struct {
	ID uuid.UUID `gorm:"type:uuid;primary_key" json:"id"`
	// Kind is one of SanctionsKindAddress, SanctionsKindCountry, SanctionsKindName.
	Kind string `gorm:"index:idx_sanctions_entry,unique;size:16" json:"kind"`
	// Value is normalised before storage: an address is lower-cased, a country is
	// upper-cased, a name is lower-cased and whitespace-collapsed.
	Value  string `gorm:"index:idx_sanctions_entry,unique;size:200" json:"value"`
	Source string `gorm:"size:100" json:"source"`
	Note   string `gorm:"size:500" json:"note,omitempty"`
	// Active is stored through an explicit insert so a `false` value is honoured
	// rather than overwritten by the column default.
	Active    bool      `gorm:"default:true" json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *SanctionsEntry) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

// ComplianceSettings is the singleton geographic and KYC policy row.
type ComplianceSettings struct {
	ID  uint   `gorm:"primaryKey" json:"id"`
	Key string `gorm:"uniqueIndex;size:32" json:"key"`
	// RestrictedCountries is a comma-separated list of ISO 3166-1 alpha-2 codes
	// that may not use the platform.
	RestrictedCountries string `gorm:"size:1000" json:"restricted_countries"`
	// RequireKycForTrading gates the mutating trading paths on an approved
	// profile.
	RequireKycForTrading bool `gorm:"default:true" json:"require_kyc_for_trading"`
	// RequireKycForWithdrawal gates the withdrawal path on an approved profile.
	RequireKycForWithdrawal bool      `gorm:"default:true" json:"require_kyc_for_withdrawal"`
	UpdatedAt               time.Time `json:"updated_at"`
}
