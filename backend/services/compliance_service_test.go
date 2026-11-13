package services

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// newComplianceTestEnv builds a compliance service over a real database.
func newComplianceTestEnv(t *testing.T) (*ComplianceService, *gorm.DB) {
	t.Helper()
	db := newWalletTestDB(t)
	if err := db.AutoMigrate(
		&models.KycProfile{},
		&models.SanctionsEntry{},
		&models.ComplianceSettings{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return &ComplianceService{db: db, enforceKYC: true}, db
}

// runKycHandler invokes a handler with the session user id and an optional body,
// the way the auth middleware sets it.
func runKycHandler(t *testing.T, handler gin.HandlerFunc, sessionUserID string, body interface{}, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", reader)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	if sessionUserID != "" {
		c.Set("user_id", sessionUserID)
	}
	handler(c)
	return rec
}

// runMiddleware drives a middleware with a session user id, returning the
// recorder. A middleware that neither aborts nor writes a status leaves the
// default 200, which is how a "passes" case is detected.
func runMiddleware(t *testing.T, mw gin.HandlerFunc, sessionUserID string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	if sessionUserID != "" {
		c.Set("user_id", sessionUserID)
	}
	mw(c)
	return rec
}

// submitKYCForTest drives the real SubmitKYC handler for a user and country.
func submitKYCForTest(t *testing.T, svc *ComplianceService, userID, country string) {
	t.Helper()
	rec := runKycHandler(t, svc.SubmitKYC, userID, map[string]string{
		"full_name":       "Test Applicant",
		"country":         country,
		"document_type":   "passport",
		"document_number": "T0000001",
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("submit kyc: %d %s", rec.Code, rec.Body.String())
	}
}

// updateComplianceSettingsForTest drives the real UpdateSettings handler.
func updateComplianceSettingsForTest(t *testing.T, svc *ComplianceService, body map[string]interface{}) {
	t.Helper()
	rec := runKycHandler(t, svc.UpdateSettings, "admin-1", body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("update settings: %d %s", rec.Code, rec.Body.String())
	}
}

func TestSubmitKYCMovesProfileToPending(t *testing.T) {
	svc, db := newComplianceTestEnv(t)

	rec := runKycHandler(t, svc.SubmitKYC, "user-1", map[string]string{
		"full_name":       "Ada  Lovelace",
		"country":         "gb",
		"document_type":   "passport",
		"document_number": "X1234567",
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var profile models.KycProfile
	if err := db.Where("user_id = ?", "user-1").First(&profile).Error; err != nil {
		t.Fatalf("profile not persisted: %v", err)
	}
	if profile.Status != models.KycStatusPending {
		t.Fatalf("expected pending, got %q", profile.Status)
	}
	if profile.Country != "GB" {
		t.Fatalf("country must be upper-cased, got %q", profile.Country)
	}
	if profile.FullName != "ada lovelace" {
		t.Fatalf("name must be normalised, got %q", profile.FullName)
	}
	// The document number must never be stored in the clear.
	if profile.DocumentHash == "" || profile.DocumentHash == "X1234567" {
		t.Fatalf("document must be hashed, got %q", profile.DocumentHash)
	}
	if profile.DocumentLast4 != "4567" {
		t.Fatalf("expected last four 4567, got %q", profile.DocumentLast4)
	}
}

func TestSubmitKYCRefusesRestrictedCountry(t *testing.T) {
	svc, _ := newComplianceTestEnv(t)
	updateComplianceSettingsForTest(t, svc, map[string]interface{}{
		"restricted_countries": "KP,IR",
	})

	rec := runKycHandler(t, svc.SubmitKYC, "user-1", map[string]string{
		"full_name":       "Grace Hopper",
		"country":         "KP",
		"document_type":   "passport",
		"document_number": "Y7654321",
	}, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a restricted country, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSubmitKYCRefusesDenylistedIdentity(t *testing.T) {
	svc, _ := newComplianceTestEnv(t)
	if err := svc.db.Model(&models.SanctionsEntry{}).Create(map[string]interface{}{
		"kind": models.SanctionsKindName, "value": "john doe", "active": true,
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := runKycHandler(t, svc.SubmitKYC, "user-1", map[string]string{
		"full_name":       "John   Doe",
		"country":         "US",
		"document_type":   "passport",
		"document_number": "Z1111111",
	}, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a denylisted identity, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestApproveKYCLetsTradingThrough(t *testing.T) {
	svc, _ := newComplianceTestEnv(t)

	submitKYCForTest(t, svc, "user-1", "US")
	verify := svc.RequireVerifiedTrading()
	// A pending account is refused.
	if rec := runMiddleware(t, verify, "user-1"); rec.Code != http.StatusForbidden {
		t.Fatalf("pending account must be refused, got %d", rec.Code)
	}

	rec := runKycHandler(t, svc.ApproveKYC, "admin-1", nil, gin.Params{{Key: "user_id", Value: "user-1"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", rec.Code, rec.Body.String())
	}
	// An approved account passes.
	if rec := runMiddleware(t, verify, "user-1"); rec.Code != http.StatusOK {
		t.Fatalf("approved account must pass, got %d", rec.Code)
	}
}

func TestRejectKYCKeepsAccountGated(t *testing.T) {
	svc, _ := newComplianceTestEnv(t)
	submitKYCForTest(t, svc, "user-1", "US")

	rec := runKycHandler(t, svc.RejectKYC, "admin-1", map[string]string{"reason": "blurry document"},
		gin.Params{{Key: "user_id", Value: "user-1"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("reject: %d %s", rec.Code, rec.Body.String())
	}

	status, err := svc.KYCStatus("user-1")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status != models.KycStatusRejected {
		t.Fatalf("expected rejected, got %q", status)
	}
	if rec := runMiddleware(t, svc.RequireVerifiedTrading(), "user-1"); rec.Code != http.StatusForbidden {
		t.Fatalf("rejected account must stay gated, got %d", rec.Code)
	}
}

func TestRequireVerifiedFailsClosedWithoutProfile(t *testing.T) {
	svc, _ := newComplianceTestEnv(t)
	// No profile at all: the account is unverified and must be refused.
	if rec := runMiddleware(t, svc.RequireVerifiedTrading(), "stranger"); rec.Code != http.StatusForbidden {
		t.Fatalf("unverified account must be refused, got %d", rec.Code)
	}
}

func TestRequireVerifiedSkippedWhenPolicyOff(t *testing.T) {
	svc, _ := newComplianceTestEnv(t)
	updateComplianceSettingsForTest(t, svc, map[string]interface{}{
		"require_kyc_for_trading": false,
	})
	if rec := runMiddleware(t, svc.RequireVerifiedTrading(), "unverified"); rec.Code != http.StatusOK {
		t.Fatalf("policy off must allow the request, got %d", rec.Code)
	}
}

func TestSanctionedAddressIsRefusedForWithdrawal(t *testing.T) {
	svc, _ := newComplianceTestEnv(t)
	address := "0x1111111111111111111111111111111111111111"
	if err := svc.db.Model(&models.SanctionsEntry{}).Create(map[string]interface{}{
		"kind": models.SanctionsKindAddress, "value": address, "active": true,
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	sanctioned, err := svc.IsSanctionedAddress(address)
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if !sanctioned {
		t.Fatal("a denylisted address must screen as sanctioned")
	}

	reason, err := svc.ScreenWithdrawalDestination("user-1", address)
	if err != nil {
		t.Fatalf("screen withdrawal: %v", err)
	}
	if reason == "" {
		t.Fatal("a sanctioned destination must be refused")
	}
}

func TestInactiveSanctionsEntryIsIgnored(t *testing.T) {
	svc, _ := newComplianceTestEnv(t)
	address := "0x2222222222222222222222222222222222222222"
	// Insert with active=false through a map so the column default does not win.
	if err := svc.db.Model(&models.SanctionsEntry{}).Create(map[string]interface{}{
		"kind": models.SanctionsKindAddress, "value": address, "active": false,
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	sanctioned, err := svc.IsSanctionedAddress(address)
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if sanctioned {
		t.Fatal("an inactive entry must not screen as sanctioned")
	}
}

func TestEmptyCountryIsNotAllowed(t *testing.T) {
	svc, _ := newComplianceTestEnv(t)
	allowed, err := svc.CountryAllowed("")
	if err != nil {
		t.Fatalf("country: %v", err)
	}
	if allowed {
		t.Fatal("an unknown jurisdiction must not read as allowed")
	}
}

func TestScreenWithdrawalRefusesRestrictedJurisdiction(t *testing.T) {
	svc, _ := newComplianceTestEnv(t)
	updateComplianceSettingsForTest(t, svc, map[string]interface{}{
		"restricted_countries": "IR",
	})
	// Seed the profile directly so a restricted applicant exists even though
	// SubmitKYC would refuse the jurisdiction.
	if err := svc.db.Create(&models.KycProfile{
		UserID: "user-1", Status: models.KycStatusApproved, Country: "IR",
	}).Error; err != nil {
		t.Fatalf("seed profile: %v", err)
	}

	reason, err := svc.ScreenWithdrawalDestination("user-1", "0x3333333333333333333333333333333333333333")
	if err != nil {
		t.Fatalf("screen: %v", err)
	}
	if reason == "" {
		t.Fatal("a restricted jurisdiction must be refused")
	}
}

func TestAddSanctionsEntryNormalisesValue(t *testing.T) {
	svc, db := newComplianceTestEnv(t)
	rec := runKycHandler(t, svc.AddSanctionsEntry, "admin-1", map[string]string{
		"kind":   models.SanctionsKindAddress,
		"value":  "0xABCDEF0000000000000000000000000000000001",
		"source": "manual",
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("add entry: %d %s", rec.Code, rec.Body.String())
	}

	var entry models.SanctionsEntry
	if err := db.Where("kind = ?", models.SanctionsKindAddress).First(&entry).Error; err != nil {
		t.Fatalf("entry not persisted: %v", err)
	}
	if entry.Value != "0xabcdef0000000000000000000000000000000001" {
		t.Fatalf("address must be lower-cased, got %q", entry.Value)
	}
	if !entry.Active {
		t.Fatal("a new entry must be active")
	}
}

func TestAddSanctionsEntryRejectsBadAddress(t *testing.T) {
	svc, _ := newComplianceTestEnv(t)
	rec := runKycHandler(t, svc.AddSanctionsEntry, "admin-1", map[string]string{
		"kind":  models.SanctionsKindAddress,
		"value": "not-an-address",
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a bad address, got %d", rec.Code)
	}
}
