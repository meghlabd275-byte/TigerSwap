package services

import (
	"bytes"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

// newWithdrawalTestEnv builds a withdrawal service over a real database and
// ledger. The blockchain client is present but no key is configured, so a send
// attempt fails after the policy has already been applied.
func newWithdrawalTestEnv(t *testing.T, requireAllowlist bool) (*WithdrawalService, *LedgerService, *gorm.DB) {
	t.Helper()
	db := newWalletTestDB(t)
	if err := db.AutoMigrate(
		&models.AccountBalance{},
		&models.WithdrawalAddress{},
		&models.WithdrawalLimit{},
		&models.WithdrawalRequest{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ledger := NewLedgerService(db)
	svc := &WithdrawalService{db: db, ledger: ledger, blockchain: NewBlockchainClient(), requireAllowlist: requireAllowlist, cooldown: defaultWithdrawalCooldown}
	return svc, ledger, db
}

// runHandlerWithMethod invokes a handler with an arbitrary HTTP method and query
// string, so a GET route can be exercised the way the router calls it.
func runHandlerWithMethod(t *testing.T, handler gin.HandlerFunc, method, target, sessionUserID string, body interface{}, params gin.Params) *httptest.ResponseRecorder {
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
	c.Request = httptest.NewRequest(method, target, reader)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	if sessionUserID != "" {
		c.Set("user_id", sessionUserID)
	}
	handler(c)
	return rec
}

// TestWithdrawalRejectsAddressNotOnAllowlist proves that with the allow-list on,
// a destination the caller never added cannot receive funds.
func TestWithdrawalRejectsAddressNotOnAllowlist(t *testing.T) {
	policy, _, _ := newWithdrawalTestEnv(t, true)
	svc := &WalletService{db: policy.db, blockchain: NewBlockchainClient(), ledger: policy.ledger}
	svc.SetWithdrawalPolicy(policy)
	if err := policy.ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}

	rec := runAuthenticatedHandler(t, svc.Withdraw, "trader", map[string]interface{}{
		"to_address": "0x1111111111111111111111111111111111111111",
		"chain_id":   1,
		"token":      "native",
		"amount":     "100",
	}, nil)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for a non-allow-listed destination, got %d: %s", rec.Code, rec.Body.String())
	}
	available, _ := policy.ledger.Available("trader", 1, "native")
	if available.Cmp(big.NewInt(1000)) != 0 {
		t.Fatalf("a rejected withdrawal must not touch the balance, got %s", available)
	}
}

// TestWithdrawalAllowsCooledDownAddress proves the destination becomes usable
// once its cooldown has passed.
func TestWithdrawalAllowsCooledDownAddress(t *testing.T) {
	policy, ledger, db := newWithdrawalTestEnv(t, true)
	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}
	// Insert an address whose cooldown is already in the past.
	if err := db.Create(&models.WithdrawalAddress{
		UserID: "trader", ChainID: 1, Address: "0x1111111111111111111111111111111111111111",
		ReadyAt: time.Now().UTC().Add(-time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed address: %v", err)
	}

	decision, err := policy.Evaluate("trader", 1, "native", "0x1111111111111111111111111111111111111111", big.NewInt(100))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if !decision.Allowed {
		t.Fatalf("a cooled-down address must be allowed, got %q", decision.Reason)
	}
}

// TestWithdrawalRejectsAddressInCooldown proves a freshly added address cannot be
// used in the same window it was added, which is what stops a stolen session from
// draining an account.
func TestWithdrawalRejectsAddressInCooldown(t *testing.T) {
	policy, _, db := newWithdrawalTestEnv(t, true)
	if err := db.Create(&models.WithdrawalAddress{
		UserID: "trader", ChainID: 1, Address: "0x1111111111111111111111111111111111111111",
		ReadyAt: time.Now().UTC().Add(23 * time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed address: %v", err)
	}

	decision, err := policy.Evaluate("trader", 1, "native", "0x1111111111111111111111111111111111111111", big.NewInt(100))
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if decision.Allowed {
		t.Fatal("an address still in cooldown must not be allowed")
	}
	if !strings.Contains(decision.Reason, "cooldown") {
		t.Fatalf("the reason must mention the cooldown, got %q", decision.Reason)
	}
}

// TestWithdrawalAboveLimitRequiresApproval proves a request over the asset limit
// is queued and the funds are reserved by the debit.
func TestWithdrawalAboveLimitRequiresApproval(t *testing.T) {
	policy, ledger, db := newWithdrawalTestEnv(t, false)
	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}
	if err := db.Create(&models.WithdrawalLimit{ChainID: 1, TokenAddress: "native", MaxAmount: "500"}).Error; err != nil {
		t.Fatalf("seed limit: %v", err)
	}
	svc := &WalletService{db: db, blockchain: NewBlockchainClient(), ledger: ledger}
	svc.SetWithdrawalPolicy(policy)

	rec := runAuthenticatedHandler(t, svc.Withdraw, "trader", map[string]interface{}{
		"to_address": "0x1111111111111111111111111111111111111111",
		"chain_id":   1,
		"token":      "native",
		"amount":     "700",
	}, nil)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for a queued withdrawal, got %d: %s", rec.Code, rec.Body.String())
	}
	var body struct {
		Status    string `json:"status"`
		RequestID string `json:"request_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Status != "pending_approval" || body.RequestID == "" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}

	// The debit reserves the funds while the request waits.
	available, _ := ledger.Available("trader", 1, "native")
	if available.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("the queued amount must be reserved, balance is %s", available)
	}

	var request models.WithdrawalRequest
	if err := db.Where("id = ?", body.RequestID).First(&request).Error; err != nil {
		t.Fatalf("the request row must exist: %v", err)
	}
	if request.Status != models.WithdrawalStatusPending {
		t.Fatalf("expected a pending request, got %q", request.Status)
	}
}

// TestWithdrawalWithinLimitProceedsToSigner proves a request at or under the
// limit is not queued; it reaches the signer, which fails because no key is set.
func TestWithdrawalWithinLimitProceedsToSigner(t *testing.T) {
	policy, ledger, db := newWithdrawalTestEnv(t, false)
	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}
	if err := db.Create(&models.WithdrawalLimit{ChainID: 1, TokenAddress: "native", MaxAmount: "500"}).Error; err != nil {
		t.Fatalf("seed limit: %v", err)
	}
	svc := &WalletService{db: db, blockchain: NewBlockchainClient(), ledger: ledger}
	svc.SetWithdrawalPolicy(policy)

	rec := runAuthenticatedHandler(t, svc.Withdraw, "trader", map[string]interface{}{
		"to_address": "0x1111111111111111111111111111111111111111",
		"chain_id":   1,
		"token":      "native",
		"amount":     "500",
	}, nil)

	// The signer is unavailable, so the withdrawal fails after reserving; it must
	// not have been queued.
	if rec.Code == http.StatusAccepted {
		t.Fatalf("a within-limit withdrawal must not be queued: %s", rec.Body.String())
	}
	var count int64
	db.Model(&models.WithdrawalRequest{}).Count(&count)
	if count != 0 {
		t.Fatalf("expected no queued request, found %d", count)
	}
}

// TestRefuseWithdrawalReturnsFunds proves refusing a queued withdrawal credits the
// reserved amount back and closes the request.
func TestRefuseWithdrawalReturnsFunds(t *testing.T) {
	policy, ledger, db := newWithdrawalTestEnv(t, false)
	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}
	id, err := policy.CreatePendingRequest("trader", 1, "native", "0x1111111111111111111111111111111111111111", big.NewInt(700))
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	reserved, _ := ledger.Available("trader", 1, "native")
	if reserved.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("expected the amount reserved, got %s", reserved)
	}

	rec := runHandlerWithMethod(t, policy.RefuseRequest, http.MethodPost, "/x", "", map[string]interface{}{"reason": "manual review"}, gin.Params{{Key: "id", Value: id.String()}})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 refusing, got %d: %s", rec.Code, rec.Body.String())
	}
	available, _ := ledger.Available("trader", 1, "native")
	if available.Cmp(big.NewInt(1000)) != 0 {
		t.Fatalf("the refused amount must be returned, balance is %s", available)
	}
	var request models.WithdrawalRequest
	if err := db.Where("id = ?", id).First(&request).Error; err != nil {
		t.Fatalf("load request: %v", err)
	}
	if request.Status != models.WithdrawalStatusRefused {
		t.Fatalf("expected a refused request, got %q", request.Status)
	}
}

// TestRefuseDecidedWithdrawalIsRejected proves a request that was already decided
// cannot be refused again, which would credit the funds twice.
func TestRefuseDecidedWithdrawalIsRejected(t *testing.T) {
	policy, ledger, db := newWithdrawalTestEnv(t, false)
	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}
	id, err := policy.CreatePendingRequest("trader", 1, "native", "0x1111111111111111111111111111111111111111", big.NewInt(700))
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	if err := db.Model(&models.WithdrawalRequest{}).Where("id = ?", id).Update("status", models.WithdrawalStatusRefused).Error; err != nil {
		t.Fatalf("decide: %v", err)
	}

	rec := runHandlerWithMethod(t, policy.RefuseRequest, http.MethodPost, "/x", "", nil, gin.Params{{Key: "id", Value: id.String()}})
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409 for an already-decided request, got %d: %s", rec.Code, rec.Body.String())
	}
	available, _ := ledger.Available("trader", 1, "native")
	if available.Cmp(big.NewInt(300)) != 0 {
		t.Fatalf("a second refusal must not credit again, balance is %s", available)
	}
}

// TestApproveIsExclusive proves a second concurrent approval cannot claim a
// request that the first already took.
func TestApproveIsExclusive(t *testing.T) {
	policy, ledger, _ := newWithdrawalTestEnv(t, false)
	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}
	id, err := policy.CreatePendingRequest("trader", 1, "native", "0x1111111111111111111111111111111111111111", big.NewInt(700))
	if err != nil {
		t.Fatalf("queue: %v", err)
	}

	first, err := policy.claimRequest(id.String())
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if first.Status != "processing" {
		t.Fatalf("a claimed request must be processing, got %q", first.Status)
	}
	if _, err := policy.claimRequest(id.String()); err == nil {
		t.Fatal("a second claim must fail because the request is already claimed")
	}
}

// TestAddAddressRejectsMalformedAndNormalizes proves the allow-list stores a
// lower-cased address and refuses a malformed one.
func TestAddAddressRejectsMalformedAndNormalizes(t *testing.T) {
	policy, _, db := newWithdrawalTestEnv(t, true)

	bad := runAuthenticatedHandler(t, policy.AddAddress, "trader", map[string]interface{}{
		"chain_id": 1, "address": "nope",
	}, nil)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a malformed address, got %d: %s", bad.Code, bad.Body.String())
	}

	good := runAuthenticatedHandler(t, policy.AddAddress, "trader", map[string]interface{}{
		"chain_id": 1, "address": "0xAbCd00000000000000000000000000000000Ef12", "label": "cold",
	}, nil)
	if good.Code != http.StatusCreated {
		t.Fatalf("expected 201 adding an address, got %d: %s", good.Code, good.Body.String())
	}

	var stored models.WithdrawalAddress
	if err := db.Where("user_id = ?", "trader").First(&stored).Error; err != nil {
		t.Fatalf("the address must be stored: %v", err)
	}
	if stored.Address != strings.ToLower(stored.Address) {
		t.Fatalf("the address must be lower-cased, got %q", stored.Address)
	}
	if !stored.ReadyAt.After(time.Now().UTC()) {
		t.Fatalf("a new address must start its cooldown, ready_at is %s", stored.ReadyAt)
	}

	// A duplicate add returns the existing row rather than failing.
	dup := runAuthenticatedHandler(t, policy.AddAddress, "trader", map[string]interface{}{
		"chain_id": 1, "address": stored.Address,
	}, nil)
	if dup.Code != http.StatusOK {
		t.Fatalf("expected 200 for a duplicate add, got %d: %s", dup.Code, dup.Body.String())
	}
}

// TestDeleteAddressIsScopedToCaller proves one account cannot delete another
// account's allow-list entry.
func TestDeleteAddressIsScopedToCaller(t *testing.T) {
	policy, _, db := newWithdrawalTestEnv(t, true)
	entry := models.WithdrawalAddress{UserID: "victim", ChainID: 1, Address: "0x1111111111111111111111111111111111111111", ReadyAt: time.Now().UTC()}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	rec := runHandlerWithMethod(t, policy.DeleteAddress, http.MethodDelete, "/x", "attacker", nil, gin.Params{{Key: "id", Value: entry.ID.String()}})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for another account's entry, got %d: %s", rec.Code, rec.Body.String())
	}
	var count int64
	db.Model(&models.WithdrawalAddress{}).Where("id = ?", entry.ID).Count(&count)
	if count != 1 {
		t.Fatal("another account's entry must not be deleted")
	}
}

// TestListRequestsRejectsUnknownStatus proves a malformed filter is rejected
// rather than silently returning everything.
func TestListRequestsRejectsUnknownStatus(t *testing.T) {
	policy, _, _ := newWithdrawalTestEnv(t, false)
	rec := runHandlerWithMethod(t, policy.ListRequests, http.MethodGet, "/x?status=bogus", "admin", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for an unknown status, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSetLimitRejectsNegativeAmount proves the limit endpoint validates its input.
func TestSetLimitRejectsNegativeAmount(t *testing.T) {
	policy, _, _ := newWithdrawalTestEnv(t, false)
	rec := runAuthenticatedHandler(t, policy.SetLimit, "admin", map[string]interface{}{
		"chain_id": 1, "token_address": "native", "max_amount": "-1",
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for a negative limit, got %d: %s", rec.Code, rec.Body.String())
	}
}

// TestSetLimitUpserts proves re-setting a limit for an asset replaces the old one
// rather than creating a second row.
func TestSetLimitUpserts(t *testing.T) {
	policy, _, db := newWithdrawalTestEnv(t, false)
	for _, amount := range []string{"100", "200"} {
		rec := runAuthenticatedHandler(t, policy.SetLimit, "admin", map[string]interface{}{
			"chain_id": 1, "token_address": "native", "max_amount": amount,
		}, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 setting a limit, got %d: %s", rec.Code, rec.Body.String())
		}
	}
	var limits []models.WithdrawalLimit
	if err := db.Find(&limits).Error; err != nil {
		t.Fatalf("load limits: %v", err)
	}
	if len(limits) != 1 {
		t.Fatalf("expected one limit row, found %d", len(limits))
	}
	if limits[0].MaxAmount != "200" {
		t.Fatalf("expected the latest amount, got %q", limits[0].MaxAmount)
	}
}

// TestWithdrawalWithoutPolicyFailsClosed proves a server that was never wired with
// a policy refuses to send rather than sending unchecked.
func TestWithdrawalWithoutPolicyFailsClosed(t *testing.T) {
	db := newWalletTestDB(t)
	if err := db.AutoMigrate(&models.AccountBalance{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ledger := NewLedgerService(db)
	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}
	svc := &WalletService{db: db, blockchain: NewBlockchainClient(), ledger: ledger}

	rec := runAuthenticatedHandler(t, svc.Withdraw, "trader", map[string]interface{}{
		"to_address": "0x1111111111111111111111111111111111111111",
		"chain_id":   1,
		"token":      "native",
		"amount":     "100",
	}, nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without a policy, got %d: %s", rec.Code, rec.Body.String())
	}
	available, _ := ledger.Available("trader", 1, "native")
	if available.Cmp(big.NewInt(1000)) != 0 {
		t.Fatalf("balance must be untouched, got %s", available)
	}
}

// TestApproveWithoutSignerIsRefused proves an approval cannot proceed when the
// platform signer is not configured.
func TestApproveWithoutSignerIsRefused(t *testing.T) {
	policy, ledger, _ := newWithdrawalTestEnv(t, false)
	// Remove the blockchain client so the handler refuses before touching chain.
	policy.blockchain = nil
	if err := ledger.Credit("trader", 1, "native", "ETH", 18, big.NewInt(1000)); err != nil {
		t.Fatalf("credit: %v", err)
	}
	id, err := policy.CreatePendingRequest("trader", 1, "native", "0x1111111111111111111111111111111111111111", big.NewInt(700))
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	rec := runHandlerWithMethod(t, policy.ApproveRequest, http.MethodPost, "/x", "admin", nil, gin.Params{{Key: "id", Value: id.String()}})
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 without a signer, got %d: %s", rec.Code, rec.Body.String())
	}
	// The request must still be pending, not stranded in processing.
	var request models.WithdrawalRequest
	if err := policy.db.Where("id = ?", id).First(&request).Error; err != nil {
		t.Fatalf("load request: %v", err)
	}
	if request.Status != models.WithdrawalStatusPending {
		t.Fatalf("expected a pending request, got %q", request.Status)
	}
}

// TestCreatePendingRequestRejectsUnfundedCaller proves the queue cannot reserve
// more than the caller holds.
func TestCreatePendingRequestRejectsUnfundedCaller(t *testing.T) {
	policy, _, _ := newWithdrawalTestEnv(t, false)
	_, err := policy.CreatePendingRequest("trader", 1, "native", "0x1111111111111111111111111111111111111111", big.NewInt(700))
	if err == nil {
		t.Fatal("a caller with no balance must not be able to queue a withdrawal")
	}
}

// TestWithdrawalAddressIDsAreGenerated proves a created address gets a UUID so the
// delete route can reference it.
func TestWithdrawalAddressIDsAreGenerated(t *testing.T) {
	_, _, db := newWithdrawalTestEnv(t, true)
	entry := models.WithdrawalAddress{UserID: "trader", ChainID: 1, Address: "0x1111111111111111111111111111111111111111", ReadyAt: time.Now().UTC()}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	if entry.ID == uuid.Nil {
		t.Fatal("a created address must have a generated id")
	}
}
