package services

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/google/uuid"

	"tigerswap/backend/models"
)

// newGovernanceTestDB builds an isolated in-memory database with the governance
// tables migrated. SQLite is the test driver only; production runs PostgreSQL.
func newGovernanceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&govTestDBSeq, 1))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("db handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { sqlDB.Close() })

	if err := db.AutoMigrate(
		&models.VeLock{}, &models.Gauge{}, &models.GaugeVote{}, &models.Bribe{}, &models.BribeClaim{},
	); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	return db
}

var govTestDBSeq uint64

// callAs runs a handler with the authenticated user id placed in the gin
// context, exactly as AuthRequired does.
func callAs(t *testing.T, userID string, handler gin.HandlerFunc, method, path string, body interface{}, params gin.Params) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var reader *bytesReader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = &bytesReader{data: raw}
	} else {
		reader = &bytesReader{}
	}

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(method, path, reader)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Params = params
	if userID != "" {
		c.Set("user_id", userID)
	}
	handler(c)
	return rec
}

// bytesReader is a minimal io.Reader over a byte slice.
type bytesReader struct {
	data []byte
	pos  int
}

func (r *bytesReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var out T
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode response %q: %v", rec.Body.String(), err)
	}
	return out
}

// ---- Locking ---------------------------------------------------------------

func TestCreateLockStoresAmountAndDecays(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	rec := callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/api/v1/governance/locks", gin.H{
		"amount":           "4000000000000000000000",
		"duration_seconds": maxLockSeconds,
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var lock models.VeLock
	if err := db.Where("user_id = ?", "user-1").First(&lock).Error; err != nil {
		t.Fatalf("lock not persisted: %v", err)
	}
	if lock.Amount != "4000000000000000000000" {
		t.Fatalf("unexpected amount %s", lock.Amount)
	}

	// A maximum-term lock starts at full power.
	power := votingPower(lock, time.Now())
	expected, _ := new(big.Int).SetString("4000000000000000000000", 10)
	diff := new(big.Int).Sub(power, expected)
	diff.Abs(diff)
	// Allow a small tolerance for the seconds elapsed during the test.
	tol, _ := new(big.Int).SetString("1000000000000000000000", 10)
	if diff.Cmp(tol) > 0 {
		t.Fatalf("voting power %s too far from %s", power, expected)
	}
}

func TestVotingPowerDecaysToZeroAtUnlock(t *testing.T) {
	now := time.Now()
	lock := models.VeLock{
		Amount:     "1000000000000000000000",
		StartTime:  now,
		UnlockTime: now.Add(100 * time.Hour),
	}

	// Halfway through, power is half.
	half := votingPower(lock, now.Add(50*time.Hour))
	expected, _ := new(big.Int).SetString("500000000000000000000", 10)
	if half.Cmp(expected) != 0 {
		t.Fatalf("expected %s at halfway, got %s", expected, half)
	}

	// At the unlock time, power is zero.
	if votingPower(lock, lock.UnlockTime).Sign() != 0 {
		t.Fatal("expected zero power at unlock time")
	}
}

func TestVotingPowerIsZeroForWithdrawnLock(t *testing.T) {
	now := time.Now()
	lock := models.VeLock{
		Amount:     "1000000000000000000000",
		StartTime:  now,
		UnlockTime: now.Add(100 * time.Hour),
		Withdrawn:  true,
	}
	if votingPower(lock, now.Add(time.Hour)).Sign() != 0 {
		t.Fatal("withdrawn lock must have no power")
	}
}

func TestCreateLockRejectsShortDuration(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	rec := callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/api/v1/governance/locks", gin.H{
		"amount":           "1000",
		"duration_seconds": minLockSeconds - 1,
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateLockRejectsLongDuration(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	rec := callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/api/v1/governance/locks", gin.H{
		"amount":           "1000",
		"duration_seconds": maxLockSeconds + 1,
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateLockRejectsNonNumericAmount(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	for _, bad := range []string{"abc", "-5", "1.5", ""} {
		rec := callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/api/v1/governance/locks", gin.H{
			"amount":           bad,
			"duration_seconds": minLockSeconds,
		}, nil)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("amount %q: expected 400, got %d", bad, rec.Code)
		}
	}
}

func TestCreateLockRejectsSecondActiveLock(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	first := callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/api/v1/governance/locks", gin.H{
		"amount":           "1000",
		"duration_seconds": maxLockSeconds,
	}, nil)
	if first.Code != http.StatusCreated {
		t.Fatalf("first lock: expected 201, got %d: %s", first.Code, first.Body.String())
	}

	second := callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/api/v1/governance/locks", gin.H{
		"amount":           "1000",
		"duration_seconds": maxLockSeconds,
	}, nil)
	if second.Code != http.StatusConflict {
		t.Fatalf("second lock: expected 409, got %d: %s", second.Code, second.Body.String())
	}
}

func TestCreateLockRequiresAuthentication(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	rec := callAs(t, "", svc.CreateLock, http.MethodPost, "/api/v1/governance/locks", gin.H{
		"amount":           "1000",
		"duration_seconds": maxLockSeconds,
	}, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestGetLocksReturnsTotalVotingPower(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/api/v1/governance/locks", gin.H{
		"amount":           "1000000000000000000000",
		"duration_seconds": maxLockSeconds,
	}, nil)

	rec := callAs(t, "user-1", svc.GetLocks, http.MethodGet, "/api/v1/governance/locks", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var out struct {
		Locks            []map[string]interface{} `json:"locks"`
		TotalVotingPower string                   `json:"total_voting_power"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Locks) != 1 {
		t.Fatalf("expected 1 lock, got %d", len(out.Locks))
	}
	total, ok := new(big.Int).SetString(out.TotalVotingPower, 10)
	if !ok || total.Sign() <= 0 {
		t.Fatalf("expected positive voting power, got %q", out.TotalVotingPower)
	}
}

func TestGetLocksScopesToCaller(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/api/v1/governance/locks", gin.H{
		"amount": "1000", "duration_seconds": maxLockSeconds,
	}, nil)
	callAs(t, "user-2", svc.CreateLock, http.MethodPost, "/api/v1/governance/locks", gin.H{
		"amount": "1000", "duration_seconds": maxLockSeconds,
	}, nil)

	rec := callAs(t, "user-1", svc.GetLocks, http.MethodGet, "/api/v1/governance/locks", nil, nil)
	var out struct {
		Locks []map[string]interface{} `json:"locks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Locks) != 1 {
		t.Fatalf("user-1 must see only their own lock, got %d", len(out.Locks))
	}
}

func TestWithdrawLockRejectsUnexpired(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/api/v1/governance/locks", gin.H{
		"amount": "1000", "duration_seconds": maxLockSeconds,
	}, nil)

	var lock models.VeLock
	db.Where("user_id = ?", "user-1").First(&lock)

	rec := callAs(t, "user-1", svc.WithdrawLock, http.MethodPost, "/x", nil,
		gin.Params{{Key: "id", Value: lock.ID.String()}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWithdrawLockRejectsOtherUsersLock(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/api/v1/governance/locks", gin.H{
		"amount": "1000", "duration_seconds": maxLockSeconds,
	}, nil)

	var lock models.VeLock
	db.Where("user_id = ?", "user-1").First(&lock)

	// user-2 must not be able to release user-1's lock.
	rec := callAs(t, "user-2", svc.WithdrawLock, http.MethodPost, "/x", nil,
		gin.Params{{Key: "id", Value: lock.ID.String()}})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestWithdrawLockSucceedsAfterExpiry(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/api/v1/governance/locks", gin.H{
		"amount": "1000", "duration_seconds": minLockSeconds,
	}, nil)

	var lock models.VeLock
	db.Where("user_id = ?", "user-1").First(&lock)

	// Force the lock into the past to simulate expiry.
	lock.UnlockTime = time.Now().Add(-time.Hour)
	if err := db.Save(&lock).Error; err != nil {
		t.Fatalf("save: %v", err)
	}

	rec := callAs(t, "user-1", svc.WithdrawLock, http.MethodPost, "/x", nil,
		gin.Params{{Key: "id", Value: lock.ID.String()}})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	db.Where("id = ?", lock.ID).First(&lock)
	if !lock.Withdrawn {
		t.Fatal("lock should be marked withdrawn")
	}
}

// ---- Gauges and voting -----------------------------------------------------

func createGauge(t *testing.T, svc *GovernanceService, name, gaugeType string) models.Gauge {
	t.Helper()
	rec := callAs(t, "admin", svc.CreateGauge, http.MethodPost, "/api/v1/admin/gauges", gin.H{
		"name": name, "gauge_type": gaugeType,
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create gauge: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Gauge models.Gauge `json:"gauge"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return out.Gauge
}

func TestCreateGaugeRejectsUnknownType(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	rec := callAs(t, "admin", svc.CreateGauge, http.MethodPost, "/x", gin.H{
		"name": "bad", "gauge_type": "not-a-type",
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCreateGaugeRejectsEmptyName(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	rec := callAs(t, "admin", svc.CreateGauge, http.MethodPost, "/x", gin.H{
		"name": "   ", "gauge_type": models.GaugeTypePool,
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestVoteRequiresVotingPower(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)
	gauge := createGauge(t, svc, "TGR/RUSD", models.GaugeTypePool)

	// No lock, so no power to vote with.
	rec := callAs(t, "user-1", svc.Vote, http.MethodPost, "/x", gin.H{
		"gauge_id": gauge.ID.String(), "weight": "1000",
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestVoteSucceedsWithinVotingPower(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)
	gauge := createGauge(t, svc, "TGR/RUSD", models.GaugeTypePool)

	callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/x", gin.H{
		"amount": "1000000000000000000000", "duration_seconds": maxLockSeconds,
	}, nil)

	power := svc.totalVotingPower("user-1", time.Now())
	half := new(big.Int).Div(power, big.NewInt(2))

	rec := callAs(t, "user-1", svc.Vote, http.MethodPost, "/x", gin.H{
		"gauge_id": gauge.ID.String(), "weight": half.String(),
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var vote models.GaugeVote
	if err := db.Where("user_id = ? AND gauge_id = ?", "user-1", gauge.ID).First(&vote).Error; err != nil {
		t.Fatalf("vote not persisted: %v", err)
	}
	if vote.Weight != half.String() {
		t.Fatalf("expected weight %s, got %s", half, vote.Weight)
	}
}

func TestVoteCannotExceedTotalPowerAcrossGauges(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)
	gaugeA := createGauge(t, svc, "Pool A", models.GaugeTypePool)
	gaugeB := createGauge(t, svc, "Pool B", models.GaugeTypePool)

	callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/x", gin.H{
		"amount": "1000000000000000000000", "duration_seconds": maxLockSeconds,
	}, nil)

	power := svc.totalVotingPower("user-1", time.Now())
	twoThirds := new(big.Int).Div(new(big.Int).Mul(power, big.NewInt(2)), big.NewInt(3))

	first := callAs(t, "user-1", svc.Vote, http.MethodPost, "/x", gin.H{
		"gauge_id": gaugeA.ID.String(), "weight": twoThirds.String(),
	}, nil)
	if first.Code != http.StatusOK {
		t.Fatalf("first vote: expected 200, got %d: %s", first.Code, first.Body.String())
	}

	// Another two thirds would exceed the total power.
	second := callAs(t, "user-1", svc.Vote, http.MethodPost, "/x", gin.H{
		"gauge_id": gaugeB.ID.String(), "weight": twoThirds.String(),
	}, nil)
	if second.Code != http.StatusBadRequest {
		t.Fatalf("second vote: expected 400, got %d: %s", second.Code, second.Body.String())
	}
}

func TestVoteCanBeReducedAndRedistributed(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)
	gaugeA := createGauge(t, svc, "Pool A", models.GaugeTypePool)
	gaugeB := createGauge(t, svc, "Pool B", models.GaugeTypePool)

	callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/x", gin.H{
		"amount": "1000000000000000000000", "duration_seconds": maxLockSeconds,
	}, nil)

	power := svc.totalVotingPower("user-1", time.Now())

	// All power on A, then move it to B by voting A down to zero.
	callAs(t, "user-1", svc.Vote, http.MethodPost, "/x", gin.H{
		"gauge_id": gaugeA.ID.String(), "weight": power.String(),
	}, nil)
	callAs(t, "user-1", svc.Vote, http.MethodPost, "/x", gin.H{
		"gauge_id": gaugeA.ID.String(), "weight": "0",
	}, nil)

	rec := callAs(t, "user-1", svc.Vote, http.MethodPost, "/x", gin.H{
		"gauge_id": gaugeB.ID.String(), "weight": power.String(),
	}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var gaugeAVote, gaugeBVote models.GaugeVote
	db.Where("user_id = ? AND gauge_id = ?", "user-1", gaugeA.ID).First(&gaugeAVote)
	db.Where("user_id = ? AND gauge_id = ?", "user-1", gaugeB.ID).First(&gaugeBVote)

	if gaugeAVote.Weight != "0" {
		t.Fatalf("expected gauge A weight 0, got %s", gaugeAVote.Weight)
	}
	if gaugeBVote.Weight != power.String() {
		t.Fatalf("expected gauge B weight %s, got %s", power, gaugeBVote.Weight)
	}
}

func TestVoteRejectsUnknownGauge(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/x", gin.H{
		"amount": "1000000000000000000000", "duration_seconds": maxLockSeconds,
	}, nil)

	rec := callAs(t, "user-1", svc.Vote, http.MethodPost, "/x", gin.H{
		"gauge_id": "00000000-0000-0000-0000-000000000001", "weight": "1",
	}, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- Bribes ----------------------------------------------------------------

func TestBribeIsSplitProRataByVoteWeight(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)
	gauge := createGauge(t, svc, "TGR/RUSD", models.GaugeTypePool)

	// user-1 locks 4x what user-2 locks.
	callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/x", gin.H{
		"amount": "4000000000000000000000", "duration_seconds": maxLockSeconds,
	}, nil)
	callAs(t, "user-2", svc.CreateLock, http.MethodPost, "/x", gin.H{
		"amount": "1000000000000000000000", "duration_seconds": maxLockSeconds,
	}, nil)

	power1 := svc.totalVotingPower("user-1", time.Now())
	power2 := svc.totalVotingPower("user-2", time.Now())

	callAs(t, "user-1", svc.Vote, http.MethodPost, "/x", gin.H{
		"gauge_id": gauge.ID.String(), "weight": power1.String(),
	}, nil)
	callAs(t, "user-2", svc.Vote, http.MethodPost, "/x", gin.H{
		"gauge_id": gauge.ID.String(), "weight": power2.String(),
	}, nil)

	rec := callAs(t, "funder", svc.AddBribe, http.MethodPost, "/x", gin.H{
		"gauge_id": gauge.ID.String(), "token": "RUSD", "amount": "1000000000000000000000",
	}, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("add bribe: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var bribe models.Bribe
	if err := db.Where("gauge_id = ?", gauge.ID).First(&bribe).Error; err != nil {
		t.Fatalf("bribe not persisted: %v", err)
	}

	// user-1 has roughly 4x user-2's weight, so their share should be ~4x.
	rec1 := callAs(t, "user-1", svc.ClaimBribe, http.MethodPost, "/x", nil,
		gin.Params{{Key: "id", Value: bribe.ID.String()}})
	rec2 := callAs(t, "user-2", svc.ClaimBribe, http.MethodPost, "/x", nil,
		gin.Params{{Key: "id", Value: bribe.ID.String()}})

	if rec1.Code != http.StatusOK || rec2.Code != http.StatusOK {
		t.Fatalf("claims failed: %d %s / %d %s", rec1.Code, rec1.Body.String(), rec2.Code, rec2.Body.String())
	}

	var out1, out2 struct {
		Amount string `json:"amount"`
	}
	if err := json.Unmarshal(rec1.Body.Bytes(), &out1); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &out2); err != nil {
		t.Fatalf("decode: %v", err)
	}

	share1, _ := new(big.Int).SetString(out1.Amount, 10)
	share2, _ := new(big.Int).SetString(out2.Amount, 10)
	total := new(big.Int).Add(share1, share2)

	bribeTotal, _ := new(big.Int).SetString("1000000000000000000000", 10)
	if total.Cmp(bribeTotal) > 0 {
		t.Fatalf("shares %s exceed the bribe", total)
	}
	// share1 should be close to 4x share2.
	ratio := new(big.Int).Div(share1, share2)
	if ratio.Cmp(big.NewInt(3)) < 0 || ratio.Cmp(big.NewInt(5)) > 0 {
		t.Fatalf("expected share ratio near 4, got %s (share1=%s share2=%s)", ratio, share1, share2)
	}
}

func TestBribeCannotBeClaimedTwice(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)
	gauge := createGauge(t, svc, "TGR/RUSD", models.GaugeTypePool)

	callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/x", gin.H{
		"amount": "1000000000000000000000", "duration_seconds": maxLockSeconds,
	}, nil)
	power := svc.totalVotingPower("user-1", time.Now())
	callAs(t, "user-1", svc.Vote, http.MethodPost, "/x", gin.H{
		"gauge_id": gauge.ID.String(), "weight": power.String(),
	}, nil)

	callAs(t, "funder", svc.AddBribe, http.MethodPost, "/x", gin.H{
		"gauge_id": gauge.ID.String(), "token": "RUSD", "amount": "1000",
	}, nil)
	var bribe models.Bribe
	db.Where("gauge_id = ?", gauge.ID).First(&bribe)

	first := callAs(t, "user-1", svc.ClaimBribe, http.MethodPost, "/x", nil,
		gin.Params{{Key: "id", Value: bribe.ID.String()}})
	if first.Code != http.StatusOK {
		t.Fatalf("first claim: expected 200, got %d: %s", first.Code, first.Body.String())
	}

	second := callAs(t, "user-1", svc.ClaimBribe, http.MethodPost, "/x", nil,
		gin.Params{{Key: "id", Value: bribe.ID.String()}})
	if second.Code != http.StatusConflict {
		t.Fatalf("second claim: expected 409, got %d: %s", second.Code, second.Body.String())
	}
}

func TestBribeClaimRejectsNonVoter(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)
	gauge := createGauge(t, svc, "TGR/RUSD", models.GaugeTypePool)

	callAs(t, "funder", svc.AddBribe, http.MethodPost, "/x", gin.H{
		"gauge_id": gauge.ID.String(), "token": "RUSD", "amount": "1000",
	}, nil)
	var bribe models.Bribe
	db.Where("gauge_id = ?", gauge.ID).First(&bribe)

	// user-9 never voted, so there is nothing to claim.
	rec := callAs(t, "user-9", svc.ClaimBribe, http.MethodPost, "/x", nil,
		gin.Params{{Key: "id", Value: bribe.ID.String()}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAddBribeRejectsUnsupportedToken(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)
	gauge := createGauge(t, svc, "TGR/RUSD", models.GaugeTypePool)

	rec := callAs(t, "funder", svc.AddBribe, http.MethodPost, "/x", gin.H{
		"gauge_id": gauge.ID.String(), "token": "SHIB", "amount": "1000",
	}, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAddBribeRejectsUnknownGauge(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	rec := callAs(t, "funder", svc.AddBribe, http.MethodPost, "/x", gin.H{
		"gauge_id": "00000000-0000-0000-0000-000000000002", "token": "RUSD", "amount": "1000",
	}, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- Boost -----------------------------------------------------------------

func TestBoostIsOneXWithoutVeTGR(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	rec := callAs(t, "user-1", svc.GetBoost, http.MethodGet, "/x?staked_amount=1000", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var out struct {
		BoostBps string `json:"boost_bps"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.BoostBps != "10000" {
		t.Fatalf("expected 10000 bps, got %s", out.BoostBps)
	}
}

func TestBoostCapsAtTwoPointFiveX(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/x", gin.H{
		"amount": "1000000000000000000000000", "duration_seconds": maxLockSeconds,
	}, nil)

	// Staked amount far below voting power must saturate at 2.5x.
	rec := callAs(t, "user-1", svc.GetBoost, http.MethodGet, "/x?staked_amount=1", nil, nil)
	var out struct {
		BoostBps string `json:"boost_bps"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.BoostBps != "25000" {
		t.Fatalf("expected 25000 bps, got %s", out.BoostBps)
	}
}

func TestBoostScalesWithRatio(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	callAs(t, "user-1", svc.CreateLock, http.MethodPost, "/x", gin.H{
		"amount": "1000000000000000000000", "duration_seconds": maxLockSeconds,
	}, nil)

	power := svc.totalVotingPower("user-1", time.Now())
	// staked == power => 1x + 1.5x = 2.5x
	rec := callAs(t, "user-1", svc.GetBoost, http.MethodGet, "/x?staked_amount="+power.String(), nil, nil)
	var out struct {
		BoostBps string `json:"boost_bps"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.BoostBps != "25000" {
		t.Fatalf("expected 25000 bps at equal ratio, got %s", out.BoostBps)
	}
}

func TestBoostRejectsMissingStake(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	rec := callAs(t, "user-1", svc.GetBoost, http.MethodGet, "/x", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// ---- Gauge admin -----------------------------------------------------------

func TestSetGaugeWeightUpdatesGauge(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)
	gauge := createGauge(t, svc, "TGR/RUSD", models.GaugeTypePool)

	rec := callAs(t, "admin", svc.SetGaugeWeight, http.MethodPut, "/x", gin.H{
		"weight": "5000000000000000000000",
	}, gin.Params{{Key: "id", Value: gauge.ID.String()}})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated models.Gauge
	db.Where("id = ?", gauge.ID).First(&updated)
	if updated.Weight != "5000000000000000000000" {
		t.Fatalf("expected updated weight, got %s", updated.Weight)
	}
}

func TestSetGaugeWeightCanDeactivate(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)
	gauge := createGauge(t, svc, "TGR/RUSD", models.GaugeTypePool)

	active := false
	rec := callAs(t, "admin", svc.SetGaugeWeight, http.MethodPut, "/x", gin.H{
		"weight": "0", "active": active,
	}, gin.Params{{Key: "id", Value: gauge.ID.String()}})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var updated models.Gauge
	db.Where("id = ?", gauge.ID).First(&updated)
	if updated.Active {
		t.Fatal("gauge should be deactivated")
	}
}

func TestSetGaugeWeightRejectsUnknownGauge(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	rec := callAs(t, "admin", svc.SetGaugeWeight, http.MethodPut, "/x", gin.H{
		"weight": "100",
	}, gin.Params{{Key: "id", Value: "00000000-0000-0000-0000-000000000003"}})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestGetGaugesFiltersActive(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)
	gauge := createGauge(t, svc, "Active Pool", models.GaugeTypePool)
	createGauge(t, svc, "Other Pool", models.GaugeTypePool)

	// Deactivate one.
	rec := callAs(t, "admin", svc.SetGaugeWeight, http.MethodPut, "/x", gin.H{
		"weight": "0", "active": false,
	}, gin.Params{{Key: "id", Value: gauge.ID.String()}})
	if rec.Code != http.StatusOK {
		t.Fatalf("deactivate: %d %s", rec.Code, rec.Body.String())
	}

	rec = callAs(t, "", svc.GetGauges, http.MethodGet, "/x?active=true", nil, nil)
	var out struct {
		Gauges []models.Gauge `json:"gauges"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Gauges) != 1 {
		t.Fatalf("expected 1 active gauge, got %d", len(out.Gauges))
	}
}

// ---- Helpers ---------------------------------------------------------------

func TestFormatBpsRendersMultiplier(t *testing.T) {
	cases := map[string]string{
		"10000": "1",
		"15000": "1.5",
		"25000": "2.5",
		"12345": "1.2345",
	}
	for in, want := range cases {
		bps, _ := new(big.Int).SetString(in, 10)
		if got := formatBps(bps); got != want {
			t.Fatalf("formatBps(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestCurrentEpochIsWeekly(t *testing.T) {
	base := time.Unix(int64(weekSeconds)*10, 0)
	if CurrentEpoch(base) != 10 {
		t.Fatalf("expected epoch 10, got %d", CurrentEpoch(base))
	}
	if CurrentEpoch(base.Add(time.Duration(weekSeconds)*time.Second)) != 11 {
		t.Fatal("epoch should advance after one week")
	}
}

// TestGetBribesPublicRoute exercises the authenticated read route the
// frontend's gauge panel uses: GET /governance/bribes?gauge_id=...
func TestGetBribesPublicRoute(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)
	userID := uuid.NewString()

	gauge := models.Gauge{Name: "RUSD/TGR", GaugeType: "liquidity", Active: true}
	if err := db.Create(&gauge).Error; err != nil {
		t.Fatalf("create gauge: %v", err)
	}

	for _, amount := range []string{"1000000000000000000000", "2500000000000000000000"} {
		rec := callAs(t, userID, svc.AddBribe, http.MethodPost, "/governance/bribes", map[string]string{
			"gauge_id": gauge.ID.String(), "token": "RUSD", "amount": amount,
		}, nil)
		if rec.Code != http.StatusOK && rec.Code != http.StatusCreated {
			t.Fatalf("add bribe: status %d body %s", rec.Code, rec.Body.String())
		}
	}

	rec := callAs(t, userID, svc.GetBribes, http.MethodGet,
		"/governance/bribes?gauge_id="+gauge.ID.String(), nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get bribes: status %d body %s", rec.Code, rec.Body.String())
	}

	var payload struct {
		Success bool           `json:"success"`
		Bribes  []models.Bribe `json:"bribes"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode bribes: %v", err)
	}
	if !payload.Success {
		t.Fatalf("expected success envelope, got %s", rec.Body.String())
	}
	if len(payload.Bribes) != 2 {
		t.Fatalf("expected 2 bribes, got %d", len(payload.Bribes))
	}
	// Newest epoch first.
	if payload.Bribes[0].Amount != "2500000000000000000000" {
		t.Fatalf("expected newest bribe first, got %s", payload.Bribes[0].Amount)
	}
}

// TestGetBribesRejectsMissingGaugeID checks the handler rejects a request with
// no gauge filter rather than returning every bribe in the database.
func TestGetBribesRejectsMissingGaugeID(t *testing.T) {
	db := newGovernanceTestDB(t)
	svc := NewGovernanceService(db)

	rec := callAs(t, "user-x", svc.GetBribes, http.MethodGet, "/governance/bribes", nil, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing gauge id, got %d", rec.Code)
	}
}
