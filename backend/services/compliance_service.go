package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// ComplianceService owns identity verification (KYC), sanctions screening and
// geographic restriction.
//
// The controls are enforced on the mutating paths through middleware so an
// unverified account cannot trade or withdraw, and a screened destination or
// jurisdiction is refused rather than allowed. Every decision is read from the
// database per request, so an operator change takes effect without a restart.
//
// Nothing here fabricates a result: a screening lookup that fails is an error
// and the caller is denied, never silently allowed.
type ComplianceService struct {
	db *gorm.DB

	// EnforceKYC gates trading and withdrawal on an approved profile. It is on
	// unless COMPLIANCE_ENFORCE_KYC is explicitly "false", so a production
	// deployment is compliant by default and a development one can opt out.
	enforceKYC bool
}

// ErrComplianceUnavailable is returned when the compliance store cannot be read.
var ErrComplianceUnavailable = errors.New("compliance check unavailable")

// NewComplianceService builds the service. The enforcement switch defaults to
// on; COMPLIANCE_ENFORCE_KYC=false turns it off for local development.
func NewComplianceService(db *gorm.DB) *ComplianceService {
	enforce := true
	if raw := strings.TrimSpace(os.Getenv("COMPLIANCE_ENFORCE_KYC")); strings.EqualFold(raw, "false") {
		enforce = false
	}
	return &ComplianceService{db: db, enforceKYC: enforce}
}

// EnforcesKYC reports whether an approved profile is required on the mutating
// paths.
func (s *ComplianceService) EnforcesKYC() bool { return s.enforceKYC }

// settings loads the singleton compliance row, creating it on first use with the
// safe defaults: KYC required and no restricted countries until an operator sets
// them.
func (s *ComplianceService) settings() (*models.ComplianceSettings, error) {
	var settings models.ComplianceSettings
	err := s.db.Where("key = ?", "default").First(&settings).Error
	if err == nil {
		return &settings, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	settings = models.ComplianceSettings{
		Key:                     "default",
		RequireKycForTrading:    true,
		RequireKycForWithdrawal: true,
	}
	if err := s.db.Create(&settings).Error; err != nil {
		return nil, err
	}
	return &settings, nil
}

// Settings exposes the policy row to the admin handlers.
func (s *ComplianceService) Settings() (*models.ComplianceSettings, error) {
	return s.settings()
}

// profileFor loads one account's profile, returning a nil profile (not an error)
// when the account has never submitted one.
func (s *ComplianceService) profileFor(userID string) (*models.KycProfile, error) {
	var profile models.KycProfile
	err := s.db.Where("user_id = ?", userID).First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

// KYCStatus returns the account's verification status, defaulting to
// unverified when no profile exists.
func (s *ComplianceService) KYCStatus(userID string) (string, error) {
	profile, err := s.profileFor(userID)
	if err != nil {
		return "", err
	}
	if profile == nil {
		return models.KycStatusUnverified, nil
	}
	return profile.Status, nil
}

// IsVerified reports whether the account has an approved profile.
func (s *ComplianceService) IsVerified(userID string) (bool, error) {
	status, err := s.KYCStatus(userID)
	if err != nil {
		return false, err
	}
	return status == models.KycStatusApproved, nil
}

// normalizeCountry upper-cases and trims an ISO 3166-1 alpha-2 code.
func normalizeCountry(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// normalizeName lower-cases and collapses internal whitespace so two spellings
// of one name screen alike.
func normalizeName(name string) string {
	return strings.Join(strings.Fields(strings.ToLower(name)), " ")
}

// restrictedCountries parses the comma-separated policy list into a set.
func restrictedCountries(settings *models.ComplianceSettings) map[string]bool {
	set := map[string]bool{}
	for _, raw := range strings.Split(settings.RestrictedCountries, ",") {
		code := normalizeCountry(raw)
		if code != "" {
			set[code] = true
		}
	}
	return set
}

// CountryAllowed reports whether a country may use the platform under the
// current policy. An empty code is not allowed: the platform cannot know the
// jurisdiction, and a missing value must not read as permitted.
func (s *ComplianceService) CountryAllowed(code string) (bool, error) {
	settings, err := s.settings()
	if err != nil {
		return false, err
	}
	normalized := normalizeCountry(code)
	if normalized == "" {
		return false, nil
	}
	if restrictedCountries(settings)[normalized] {
		return false, nil
	}
	// A denylisted country entry is an independent block that survives a policy
	// change, so an operator can stop a jurisdiction without editing the list.
	sanctioned, err := s.isDenylisted(models.SanctionsKindCountry, normalized)
	if err != nil {
		return false, err
	}
	return !sanctioned, nil
}

// IsSanctionedAddress reports whether an address is on the denylist. The check
// is case-insensitive; an empty address is refused.
func (s *ComplianceService) IsSanctionedAddress(address string) (bool, error) {
	if strings.TrimSpace(address) == "" {
		return true, nil
	}
	return s.isDenylisted(models.SanctionsKindAddress, strings.ToLower(strings.TrimSpace(address)))
}

// isDenylisted reports whether an active entry exists for a normalised value.
func (s *ComplianceService) isDenylisted(kind, value string) (bool, error) {
	var count int64
	err := s.db.Model(&models.SanctionsEntry{}).
		Where("kind = ? AND value = ? AND active = ?", kind, value, true).
		Count(&count).Error
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ScreenWithdrawalDestination applies the sanctions and geographic policy to a
// prospective withdrawal. It returns a non-empty reason when the destination is
// refused. A database error is returned so the caller can fail closed.
func (s *ComplianceService) ScreenWithdrawalDestination(userID, address string) (string, error) {
	sanctioned, err := s.IsSanctionedAddress(address)
	if err != nil {
		return "", err
	}
	if sanctioned {
		return "destination address is on the sanctions denylist", nil
	}

	profile, err := s.profileFor(userID)
	if err != nil {
		return "", err
	}
	if profile != nil && profile.Country != "" {
		allowed, err := s.CountryAllowed(profile.Country)
		if err != nil {
			return "", err
		}
		if !allowed {
			return "your jurisdiction is not permitted to withdraw", nil
		}
	}
	return "", nil
}

// hashDocument returns the SHA-256 digest of a document number. The number is
// never stored; the digest lets an operator match a resubmission without
// holding the document itself.
func hashDocument(number string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(number)))
	return hex.EncodeToString(sum[:])
}

// lastFour returns the trailing four characters of a document number.
func lastFour(number string) string {
	trimmed := strings.TrimSpace(number)
	if len(trimmed) <= 4 {
		return trimmed
	}
	return trimmed[len(trimmed)-4:]
}

// SubmitKYC records or replaces the caller's identity submission and moves the
// profile to pending. A resubmission after a rejection is allowed; a submission
// from a restricted jurisdiction is refused here, at the boundary, so a
// restricted applicant never enters the review queue.
func (s *ComplianceService) SubmitKYC(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	var req struct {
		FullName     string `json:"full_name" binding:"required"`
		Country      string `json:"country" binding:"required"`
		DocumentType string `json:"document_type" binding:"required"`
		DocumentNo   string `json:"document_number" binding:"required"`
		DateOfBirth  string `json:"date_of_birth"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	name := normalizeName(req.FullName)
	if len(name) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "full_name is required"})
		return
	}
	country := normalizeCountry(req.Country)
	if len(country) != 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "country must be an ISO 3166-1 alpha-2 code"})
		return
	}
	if strings.TrimSpace(req.DocumentNo) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "document_number is required"})
		return
	}

	// Geographic restriction is enforced before the record is written.
	allowed, err := s.CountryAllowed(country)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "your jurisdiction is not permitted to use this platform"})
		return
	}

	// A denylisted identity is refused at submission, like a restricted country.
	sanctioned, err := s.isDenylisted(models.SanctionsKindName, name)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	if sanctioned {
		c.JSON(http.StatusForbidden, gin.H{"error": "identity screening failed"})
		return
	}

	profile, err := s.profileFor(userID)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	if profile == nil {
		profile = &models.KycProfile{UserID: userID}
	}
	profile.FullName = name
	profile.Country = country
	profile.DocumentType = strings.TrimSpace(req.DocumentType)
	profile.DocumentHash = hashDocument(req.DocumentNo)
	profile.DocumentLast4 = lastFour(req.DocumentNo)
	profile.DateOfBirth = strings.TrimSpace(req.DateOfBirth)
	profile.Status = models.KycStatusPending
	profile.SubmittedAt = time.Now().UTC()
	// A resubmission clears the previous decision so the queue shows it afresh.
	profile.ReviewerID = nil
	profile.ReviewedAt = nil
	profile.RejectionReason = ""

	if err := s.db.Save(profile).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"profile": profile})
}

// GetMyKYC returns the caller's own verification record.
func (s *ComplianceService) GetMyKYC(c *gin.Context) {
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	profile, err := s.profileFor(userID)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	if profile == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":       models.KycStatusUnverified,
			"full_name":    "",
			"country":      "",
			"submitted_at": nil,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":           profile.Status,
		"full_name":        profile.FullName,
		"country":          profile.Country,
		"document_type":    profile.DocumentType,
		"document_last4":   profile.DocumentLast4,
		"rejection_reason": profile.RejectionReason,
		"submitted_at":     profile.SubmittedAt,
		"reviewed_at":      profile.ReviewedAt,
	})
}

// ListKYCSubmissions returns the review queue (admin only), newest first.
func (s *ComplianceService) ListKYCSubmissions(c *gin.Context) {
	query := s.db.Model(&models.KycProfile{})
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		switch status {
		case models.KycStatusUnverified, models.KycStatusPending, models.KycStatusApproved, models.KycStatusRejected:
			query = query.Where("status = ?", status)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown status"})
			return
		}
	}
	var profiles []models.KycProfile
	if err := query.Order("submitted_at desc").Find(&profiles).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"submissions": profiles})
}

// decideKYC applies an operator decision to one profile.
func (s *ComplianceService) decideKYC(c *gin.Context, approve bool) {
	userID := strings.TrimSpace(c.Param("user_id"))
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id is required"})
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			respondBadRequestError(c, err)
			return
		}
	}

	profile, err := s.profileFor(userID)
	if err != nil {
		respondInternalError(c, err)
		return
	}
	if profile == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no KYC submission for that account"})
		return
	}

	now := time.Now().UTC()
	profile.ReviewedAt = &now
	if reviewer, ok := authenticatedUserID(c); ok {
		if parsed, parseErr := uuid.Parse(reviewer); parseErr == nil {
			profile.ReviewerID = &parsed
		}
	}
	if approve {
		profile.Status = models.KycStatusApproved
		profile.RejectionReason = ""
	} else {
		profile.Status = models.KycStatusRejected
		profile.RejectionReason = strings.TrimSpace(req.Reason)
		if profile.RejectionReason == "" {
			profile.RejectionReason = "rejected by operator"
		}
	}
	if err := s.db.Save(profile).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": profile.Status})
}

// ApproveKYC marks a submission approved (admin only).
func (s *ComplianceService) ApproveKYC(c *gin.Context) { s.decideKYC(c, true) }

// RejectKYC marks a submission rejected (admin only).
func (s *ComplianceService) RejectKYC(c *gin.Context) { s.decideKYC(c, false) }

// AddSanctionsEntry adds a denylisted value (admin only). The value is
// normalised by kind so a duplicate add is a no-op.
func (s *ComplianceService) AddSanctionsEntry(c *gin.Context) {
	var req struct {
		Kind   string `json:"kind" binding:"required"`
		Value  string `json:"value" binding:"required"`
		Source string `json:"source"`
		Note   string `json:"note"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	var value string
	switch req.Kind {
	case models.SanctionsKindAddress:
		if !common.IsHexAddress(strings.TrimSpace(req.Value)) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "value must be a hex address"})
			return
		}
		value = strings.ToLower(strings.TrimSpace(req.Value))
	case models.SanctionsKindCountry:
		value = normalizeCountry(req.Value)
		if len(value) != 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "value must be an ISO 3166-1 alpha-2 code"})
			return
		}
	case models.SanctionsKindName:
		value = normalizeName(req.Value)
		if value == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "value is required"})
			return
		}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind must be address, country or name"})
		return
	}

	entry := models.SanctionsEntry{
		Kind:   req.Kind,
		Value:  value,
		Source: strings.TrimSpace(req.Source),
		Note:   strings.TrimSpace(req.Note),
		Active: true,
	}
	// Insert through a map so the `active` default does not overwrite the value
	// and the timestamps are set explicitly, the same way the other admin
	// creates handle a defaulted boolean column.
	row := map[string]interface{}{
		"id":         uuid.New(),
		"kind":       entry.Kind,
		"value":      entry.Value,
		"source":     entry.Source,
		"note":       entry.Note,
		"active":     entry.Active,
		"created_at": time.Now().UTC(),
		"updated_at": time.Now().UTC(),
	}
	if err := s.db.Model(&models.SanctionsEntry{}).Create(row).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	var saved models.SanctionsEntry
	if err := s.db.Where("kind = ? AND value = ?", entry.Kind, entry.Value).First(&saved).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"entry": saved})
}

// ListSanctionsEntries returns the denylist (admin only), optionally filtered by
// kind.
func (s *ComplianceService) ListSanctionsEntries(c *gin.Context) {
	query := s.db.Model(&models.SanctionsEntry{})
	if kind := strings.TrimSpace(c.Query("kind")); kind != "" {
		switch kind {
		case models.SanctionsKindAddress, models.SanctionsKindCountry, models.SanctionsKindName:
			query = query.Where("kind = ?", kind)
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown kind"})
			return
		}
	}
	var entries []models.SanctionsEntry
	if err := query.Order("created_at desc").Find(&entries).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

// DeleteSanctionsEntry removes a denylist entry (admin only).
func (s *ComplianceService) DeleteSanctionsEntry(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	if _, err := uuid.Parse(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := s.db.Where("id = ?", id).Delete(&models.SanctionsEntry{}).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetSettings returns the compliance policy (admin only).
func (s *ComplianceService) GetSettings(c *gin.Context) {
	settings, err := s.settings()
	if err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

// UpdateSettings replaces the compliance policy (admin only).
func (s *ComplianceService) UpdateSettings(c *gin.Context) {
	var req struct {
		RestrictedCountries     string `json:"restricted_countries"`
		RequireKycForTrading    *bool  `json:"require_kyc_for_trading"`
		RequireKycForWithdrawal *bool  `json:"require_kyc_for_withdrawal"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBadRequestError(c, err)
		return
	}

	// Validate every code before writing any of them, so a bad list cannot
	// half-apply.
	cleaned := make([]string, 0)
	for _, raw := range strings.Split(req.RestrictedCountries, ",") {
		code := normalizeCountry(raw)
		if code == "" {
			continue
		}
		if len(code) != 2 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "restricted_countries must be comma-separated ISO 3166-1 alpha-2 codes"})
			return
		}
		cleaned = append(cleaned, code)
	}

	settings, err := s.settings()
	if err != nil {
		respondInternalError(c, err)
		return
	}
	settings.RestrictedCountries = strings.Join(cleaned, ",")
	if req.RequireKycForTrading != nil {
		settings.RequireKycForTrading = *req.RequireKycForTrading
	}
	if req.RequireKycForWithdrawal != nil {
		settings.RequireKycForWithdrawal = *req.RequireKycForWithdrawal
	}
	settings.UpdatedAt = time.Now().UTC()
	if err := s.db.Save(settings).Error; err != nil {
		respondInternalError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

// RequireVerifiedTrading is middleware that refuses a trading request from an
// account without an approved profile, when the policy requires it. It fails
// closed: a store error denies the request rather than allowing it through.
func (s *ComplianceService) RequireVerifiedTrading() gin.HandlerFunc {
	return s.requireVerified(func(settings *models.ComplianceSettings) bool {
		return settings.RequireKycForTrading
	})
}

// RequireVerifiedWithdrawal is middleware that refuses a withdrawal from an
// account without an approved profile, when the policy requires it.
func (s *ComplianceService) RequireVerifiedWithdrawal() gin.HandlerFunc {
	return s.requireVerified(func(settings *models.ComplianceSettings) bool {
		return settings.RequireKycForWithdrawal
	})
}

func (s *ComplianceService) requireVerified(when func(*models.ComplianceSettings) bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !s.enforceKYC {
			c.Next()
			return
		}
		settings, err := s.settings()
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "compliance settings unavailable"})
			return
		}
		if !when(settings) {
			c.Next()
			return
		}
		userID := c.GetString("user_id")
		if userID == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authenticated session required"})
			return
		}
		status, err := s.KYCStatus(userID)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "compliance check unavailable"})
			return
		}
		if status != models.KycStatusApproved {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "identity verification is required before this action"})
			return
		}
		c.Next()
	}
}
