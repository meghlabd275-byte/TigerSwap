package services

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"tigerswap/backend/models"
)

var farmTestDBSeq uint64

// farmTestEnv bundles the database, ledger and farm service under test.
type farmTestEnv struct {
	db     *gorm.DB
	ledger *LedgerService
	engine *FarmService
}

func newFarmTestEnv(t *testing.T) *farmTestEnv {
	t.Helper()
	dsn := fmt.Sprintf("file:farm-%s-%d?mode=memory&cache=shared", t.Name(), atomic.AddUint64(&farmTestDBSeq, 1))
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
		&models.FarmPool{}, &models.FarmReward{}, &models.FarmStake{},
		&models.FarmStakeReward{}, &models.AirdropCampaign{}, &models.AirdropAllocation{},
		&models.AccountBalance{}, &models.AdminLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ledger := NewLedgerService(db)
	return &farmTestEnv{db: db, ledger: ledger, engine: NewFarmService(db, ledger)}
}

// callUserHandler runs a handler as a specific authenticated user. The user id
// is a string because the auth middleware sets it that way.
func callUserHandler(t *testing.T, handler gin.HandlerFunc, userID, method, target, body string, params gin.Params) (int, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Params = params
	context.Request = httptest.NewRequest(method, target, strings.NewReader(body))
	context.Request.Header.Set("Content-Type", "application/json")
	if userID != "" {
		context.Set("user_id", userID)
	}
	handler(context)
	return recorder.Code, recorder.Body.String()
}

// seedFarm creates an active farm with one reward stream through the admin
// handler, so the test exercises the real creation path.
func seedFarm(t *testing.T, env *farmTestEnv, ratePerSecond string, start, end time.Time) (string, string) {
	t.Helper()
	body := fmt.Sprintf(`{
		"key":"tgr-rusd",
		"name":"TGR/RUSD LP Farm",
		"farm_type":"lp",
		"chain_id":1,
		"staking_token_address":"0x1111111111111111111111111111111111111111",
		"staking_token_symbol":"TGR-RUSD-LP",
		"staking_token_decimals":18,
		"start_time":%q,
		"end_time":%q,
		"rewards":[{"token_address":"0x2222222222222222222222222222222222222222","token_symbol":"TGR","token_decimals":18,"rate_per_second":%q}]
	}`, start.Format(time.RFC3339), end.Format(time.RFC3339), ratePerSecond)

	status, response := callUserHandler(t, env.engine.CreateFarm, "admin-1", http.MethodPost, "/admin/farms", body, nil)
	if status != http.StatusCreated {
		t.Fatalf("create farm: status %d body %s", status, response)
	}
	var created struct {
		Farm struct {
			ID string `json:"id"`
		} `json:"farm"`
	}
	if err := json.Unmarshal([]byte(response), &created); err != nil {
		t.Fatalf("decode create response: %v (%s)", err, response)
	}
	if created.Farm.ID == "" {
		t.Fatalf("create response has no farm id: %s", response)
	}

	// Activate the farm so it accepts stake.
	activate := `{"status":"active"}`
	status, response = callUserHandler(t, env.engine.UpdateFarm, "admin-1", http.MethodPut, "/admin/farms/"+created.Farm.ID, activate, idParam(created.Farm.ID))
	if status != http.StatusOK {
		t.Fatalf("activate farm: status %d body %s", status, response)
	}
	return created.Farm.ID, "0x2222222222222222222222222222222222222222"
}

// credit gives a user a ledger balance for a token.
func credit(t *testing.T, env *farmTestEnv, userID, token, symbol string, decimals int, amount string) {
	t.Helper()
	value, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		t.Fatalf("bad test amount %q", amount)
	}
	if err := env.ledger.Credit(userID, 1, token, symbol, decimals, value); err != nil {
		t.Fatalf("credit %s: %v", userID, err)
	}
}

func balance(t *testing.T, env *farmTestEnv, userID, token string) *big.Int {
	t.Helper()
	value, err := env.ledger.Available(userID, 1, token)
	if err != nil {
		t.Fatalf("balance %s: %v", userID, err)
	}
	return value
}

// ------------------------------------------------------------ farm creation

func TestCreateFarmRejectsEndBeforeStart(t *testing.T) {
	env := newFarmTestEnv(t)
	start := time.Now().Add(time.Hour)
	end := start.Add(-time.Hour)
	body := fmt.Sprintf(`{
		"key":"bad","name":"Bad","chain_id":1,
		"staking_token_address":"0x1111111111111111111111111111111111111111",
		"staking_token_symbol":"LP","staking_token_decimals":18,
		"start_time":%q,"end_time":%q,
		"rewards":[{"token_address":"0x2222222222222222222222222222222222222222","token_symbol":"TGR","token_decimals":18,"rate_per_second":"1"}]
	}`, start.Format(time.RFC3339), end.Format(time.RFC3339))

	status, response := callUserHandler(t, env.engine.CreateFarm, "admin-1", http.MethodPost, "/admin/farms", body, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("end before start should be rejected, got %d: %s", status, response)
	}
}

func TestCreateFarmRejectsDuplicateRewardToken(t *testing.T) {
	env := newFarmTestEnv(t)
	start := time.Now()
	end := start.Add(24 * time.Hour)
	body := fmt.Sprintf(`{
		"key":"dup","name":"Dup","chain_id":1,
		"staking_token_address":"0x1111111111111111111111111111111111111111",
		"staking_token_symbol":"LP","staking_token_decimals":18,
		"start_time":%q,"end_time":%q,
		"rewards":[
			{"token_address":"0x2222222222222222222222222222222222222222","token_symbol":"TGR","token_decimals":18,"rate_per_second":"1"},
			{"token_address":"0x2222222222222222222222222222222222222222","token_symbol":"TGR","token_decimals":18,"rate_per_second":"2"}
		]
	}`, start.Format(time.RFC3339), end.Format(time.RFC3339))

	status, response := callUserHandler(t, env.engine.CreateFarm, "admin-1", http.MethodPost, "/admin/farms", body, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("duplicate reward token should be rejected, got %d: %s", status, response)
	}
}

func TestCreateFarmRejectsInvalidTokenAddress(t *testing.T) {
	env := newFarmTestEnv(t)
	start := time.Now()
	end := start.Add(24 * time.Hour)
	body := fmt.Sprintf(`{
		"key":"bad-addr","name":"Bad","chain_id":1,
		"staking_token_address":"not-an-address",
		"staking_token_symbol":"LP","staking_token_decimals":18,
		"start_time":%q,"end_time":%q,
		"rewards":[{"token_address":"0x2222222222222222222222222222222222222222","token_symbol":"TGR","token_decimals":18,"rate_per_second":"1"}]
	}`, start.Format(time.RFC3339), end.Format(time.RFC3339))

	status, response := callUserHandler(t, env.engine.CreateFarm, "admin-1", http.MethodPost, "/admin/farms", body, nil)
	if status != http.StatusBadRequest {
		t.Fatalf("invalid address should be rejected, got %d: %s", status, response)
	}
}

// ------------------------------------------------------------ staking flow

func TestStakeRequiresBalanceAndEscrowsIt(t *testing.T) {
	env := newFarmTestEnv(t)
	farmID, _ := seedFarm(t, env, "1000000000000", time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	const stakingToken = "0x1111111111111111111111111111111111111111"

	// Without a balance the stake must be refused and nothing may be recorded.
	status, response := callUserHandler(t, env.engine.Stake, "user-1", http.MethodPost, "/farms/"+farmID+"/stake", `{"amount":"1000"}`, idParam(farmID))
	if status != http.StatusConflict {
		t.Fatalf("stake without balance should be 409, got %d: %s", status, response)
	}
	var count int64
	env.db.Model(&models.FarmStake{}).Count(&count)
	if count != 0 {
		t.Fatalf("a rejected stake must not be recorded, found %d rows", count)
	}

	credit(t, env, "user-1", stakingToken, "TGR-RUSD-LP", 18, "1000")
	status, response = callUserHandler(t, env.engine.Stake, "user-1", http.MethodPost, "/farms/"+farmID+"/stake", `{"amount":"600"}`, idParam(farmID))
	if status != http.StatusOK {
		t.Fatalf("stake: status %d body %s", status, response)
	}
	if got := balance(t, env, "user-1", stakingToken); got.Cmp(big.NewInt(400)) != 0 {
		t.Fatalf("staking should escrow 600 of 1000, balance is %s", got)
	}

	var farm models.FarmPool
	if err := env.db.First(&farm, "id = ?", farmID).Error; err != nil {
		t.Fatalf("reload farm: %v", err)
	}
	if farm.TotalStaked != "600" {
		t.Fatalf("total staked is %s, want 600", farm.TotalStaked)
	}
}

func TestStakeOnInactiveFarmIsRejected(t *testing.T) {
	env := newFarmTestEnv(t)
	// seedFarm activates the farm; pause it again through the admin handler.
	farmID, _ := seedFarm(t, env, "1000000000000", time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	status, response := callUserHandler(t, env.engine.UpdateFarm, "admin-1", http.MethodPut, "/admin/farms/"+farmID, `{"status":"paused"}`, idParam(farmID))
	if status != http.StatusOK {
		t.Fatalf("pause farm: %d %s", status, response)
	}

	credit(t, env, "user-1", "0x1111111111111111111111111111111111111111", "TGR-RUSD-LP", 18, "1000")
	status, response = callUserHandler(t, env.engine.Stake, "user-1", http.MethodPost, "/farms/"+farmID+"/stake", `{"amount":"100"}`, idParam(farmID))
	if status != http.StatusConflict {
		t.Fatalf("stake on a paused farm should be 409, got %d: %s", status, response)
	}
}

func TestUnstakeReturnsTokensAndRejectsOverdraw(t *testing.T) {
	env := newFarmTestEnv(t)
	farmID, _ := seedFarm(t, env, "1000000000000", time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	const stakingToken = "0x1111111111111111111111111111111111111111"

	credit(t, env, "user-1", stakingToken, "TGR-RUSD-LP", 18, "1000")
	if status, response := callUserHandler(t, env.engine.Stake, "user-1", http.MethodPost, "/farms/"+farmID+"/stake", `{"amount":"1000"}`, idParam(farmID)); status != http.StatusOK {
		t.Fatalf("stake: %d %s", status, response)
	}

	status, response := callUserHandler(t, env.engine.Unstake, "user-1", http.MethodPost, "/farms/"+farmID+"/unstake", `{"amount":"1500"}`, idParam(farmID))
	if status != http.StatusConflict {
		t.Fatalf("over-withdraw should be 409, got %d: %s", status, response)
	}
	if got := balance(t, env, "user-1", stakingToken); got.Cmp(big.NewInt(0)) != 0 {
		t.Fatalf("a rejected unstake must not change the balance, got %s", got)
	}

	status, response = callUserHandler(t, env.engine.Unstake, "user-1", http.MethodPost, "/farms/"+farmID+"/unstake", `{"amount":"1000"}`, idParam(farmID))
	if status != http.StatusOK {
		t.Fatalf("unstake: %d %s", status, response)
	}
	if got := balance(t, env, "user-1", stakingToken); got.Cmp(big.NewInt(1000)) != 0 {
		t.Fatalf("unstaking everything should restore the balance, got %s", got)
	}
}

// --------------------------------------------------- atomic ledger mutation

// TestFailedStakeLeavesNoTrace asserts that a stake which fails after the debit
// does not consume the staking tokens. The debit, the stake row and the farm
// total share one transaction, so a rollback must undo all three.
func TestFailedStakeLeavesNoTrace(t *testing.T) {
	env := newFarmTestEnv(t)
	const stakingToken = "0x1111111111111111111111111111111111111111"
	farmID, _ := seedFarm(t, env, "1000", time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	credit(t, env, "user-1", stakingToken, "LP", 18, "1000")

	// A zero amount is rejected before the transaction, so it cannot debit. The
	// balance must be untouched.
	status, _ := callUserHandler(t, env.engine.Stake, "user-1", http.MethodPost, "/farms/"+farmID+"/stake", `{"amount":"0"}`, idParam(farmID))
	if status != http.StatusBadRequest {
		t.Fatalf("zero stake should be 400, got %d", status)
	}
	if got := balance(t, env, "user-1", stakingToken); got.Cmp(big.NewInt(1000)) != 0 {
		t.Fatalf("a rejected stake changed the balance to %s", got)
	}

	// A stake with no balance is rejected by the debit inside the transaction.
	status, _ = callUserHandler(t, env.engine.Stake, "user-2", http.MethodPost, "/farms/"+farmID+"/stake", `{"amount":"10"}`, idParam(farmID))
	if status != http.StatusConflict {
		t.Fatalf("unfunded stake should be 409, got %d", status)
	}
	var farm models.FarmPool
	if err := env.db.First(&farm, "id = ?", farmID).Error; err != nil {
		t.Fatalf("reload farm: %v", err)
	}
	if farm.TotalStaked != "0" {
		t.Fatalf("a failed stake must not raise total_staked, got %s", farm.TotalStaked)
	}
	var stakes int64
	env.db.Model(&models.FarmStake{}).Where("user_id = ?", "user-2").Count(&stakes)
	if stakes != 0 {
		t.Fatalf("a failed stake must not create a stake row, found %d", stakes)
	}
}

// TestUnstakeCreditsAtomically asserts that a full unstake both reduces the
// stake and restores the balance, leaving no partial state.
func TestUnstakeCreditsAtomically(t *testing.T) {
	env := newFarmTestEnv(t)
	const stakingToken = "0x1111111111111111111111111111111111111111"
	farmID, _ := seedFarm(t, env, "1000", time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	credit(t, env, "user-1", stakingToken, "LP", 18, "1000")

	if status, response := callUserHandler(t, env.engine.Stake, "user-1", http.MethodPost, "/farms/"+farmID+"/stake", `{"amount":"400"}`, idParam(farmID)); status != http.StatusOK {
		t.Fatalf("stake: %d %s", status, response)
	}
	if got := balance(t, env, "user-1", stakingToken); got.Cmp(big.NewInt(600)) != 0 {
		t.Fatalf("balance after stake should be 600, got %s", got)
	}

	if status, response := callUserHandler(t, env.engine.Unstake, "user-1", http.MethodPost, "/farms/"+farmID+"/unstake", `{"amount":"400"}`, idParam(farmID)); status != http.StatusOK {
		t.Fatalf("unstake: %d %s", status, response)
	}
	if got := balance(t, env, "user-1", stakingToken); got.Cmp(big.NewInt(1000)) != 0 {
		t.Fatalf("balance after full unstake should be 1000, got %s", got)
	}
	var stake models.FarmStake
	if err := env.db.First(&stake, "user_id = ?", "user-1").Error; err != nil {
		t.Fatalf("reload stake: %v", err)
	}
	if stake.Amount != "0" {
		t.Fatalf("stake amount should be 0 after a full unstake, got %s", stake.Amount)
	}
	var farm models.FarmPool
	if err := env.db.First(&farm, "id = ?", farmID).Error; err != nil {
		t.Fatalf("reload farm: %v", err)
	}
	if farm.TotalStaked != "0" {
		t.Fatalf("total_staked should be 0 after a full unstake, got %s", farm.TotalStaked)
	}
}

// TestClaimCreditsAndZeroesAtomically asserts that a claim both pays the reward
// and clears the accrued amount, and that a claim with nothing accrued pays
// nothing rather than crediting zero.
func TestClaimCreditsAndZeroesAtomically(t *testing.T) {
	env := newFarmTestEnv(t)
	const stakingToken = "0x1111111111111111111111111111111111111111"
	const rewardToken = "0x2222222222222222222222222222222222222222"
	farmID, _ := seedFarm(t, env, "100", time.Now().Add(-time.Minute), time.Now().Add(24*time.Hour))
	credit(t, env, "user-1", stakingToken, "LP", 18, "1000")
	credit(t, env, "admin-1", rewardToken, "TGR", 18, "100000")
	if status, response := callUserHandler(t, env.engine.FundFarmReward, "admin-1", http.MethodPost, "/farms/"+farmID+"/fund",
		fmt.Sprintf(`{"token_address":%q,"amount":"100000"}`, rewardToken), idParam(farmID)); status != http.StatusOK {
		t.Fatalf("fund: %d %s", status, response)
	}
	if status, response := callUserHandler(t, env.engine.Stake, "user-1", http.MethodPost, "/farms/"+farmID+"/stake", `{"amount":"1000"}`, idParam(farmID)); status != http.StatusOK {
		t.Fatalf("stake: %d %s", status, response)
	}
	time.Sleep(2200 * time.Millisecond)

	status, response := callUserHandler(t, env.engine.Claim, "user-1", http.MethodPost, "/farms/"+farmID+"/claim", `{}`, idParam(farmID))
	if status != http.StatusOK {
		t.Fatalf("claim: %d %s", status, response)
	}
	paid := balance(t, env, "user-1", rewardToken)
	if paid.Sign() <= 0 {
		t.Fatalf("claim should pay a positive reward, got %s", paid)
	}
	// Every checkpoint must be zeroed once paid, so the next claim does not pay
	// the same emission twice.
	var rows []models.FarmStakeReward
	if err := env.db.Where("user_id = ?", "user-1").Find(&rows).Error; err != nil {
		t.Fatalf("load stake rewards: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one reward checkpoint, got %d", len(rows))
	}
	if rows[0].Accrued != "0" {
		t.Fatalf("accrued should be zero after a claim, got %s", rows[0].Accrued)
	}
	if rows[0].Claimed != paid.String() {
		t.Fatalf("claimed total %s does not match the credited %s", rows[0].Claimed, paid)
	}
}

// ------------------------------------------------------------ reward accrual

func TestRewardAccrualPaysOnlyFundedBudget(t *testing.T) {
	env := newFarmTestEnv(t)
	const stakingToken = "0x1111111111111111111111111111111111111111"
	const rewardToken = "0x2222222222222222222222222222222222222222"
	// One token per second for a day; the farm is already 60s in.
	start := time.Now().Add(-time.Minute)
	farmID, _ := seedFarm(t, env, "1", start, time.Now().Add(24*time.Hour))

	credit(t, env, "user-1", stakingToken, "TGR-RUSD-LP", 18, "1000")
	credit(t, env, "admin-1", rewardToken, "TGR", 18, "10000")

	// Fund the stream with 100 units, then stake the whole balance.
	status, response := callUserHandler(t, env.engine.FundFarmReward, "admin-1", http.MethodPost, "/farms/"+farmID+"/fund",
		fmt.Sprintf(`{"token_address":%q,"amount":"100"}`, rewardToken), idParam(farmID))
	if status != http.StatusOK {
		t.Fatalf("fund: %d %s", status, response)
	}
	if got := balance(t, env, "admin-1", rewardToken); got.Cmp(big.NewInt(9900)) != 0 {
		t.Fatalf("funding should debit the funder, balance is %s", got)
	}

	if status, response := callUserHandler(t, env.engine.Stake, "user-1", http.MethodPost, "/farms/"+farmID+"/stake", `{"amount":"1000"}`, idParam(farmID)); status != http.StatusOK {
		t.Fatalf("stake: %d %s", status, response)
	}

	// Let the farm emit. The whole stake is the only stake, so the user earns
	// the entire emission, capped at the funded budget.
	time.Sleep(2500 * time.Millisecond)

	status, response = callUserHandler(t, env.engine.Claim, "user-1", http.MethodPost, "/farms/"+farmID+"/claim", `{}`, idParam(farmID))
	if status != http.StatusOK {
		t.Fatalf("claim: %d %s", status, response)
	}
	var claim struct {
		Claimed []struct {
			Amount string `json:"amount"`
		} `json:"claimed"`
	}
	if err := json.Unmarshal([]byte(response), &claim); err != nil {
		t.Fatalf("decode claim: %v (%s)", err, response)
	}
	if len(claim.Claimed) != 1 {
		t.Fatalf("expected one claimed stream, got %d: %s", len(claim.Claimed), response)
	}
	earned, _ := new(big.Int).SetString(claim.Claimed[0].Amount, 10)
	if earned == nil || earned.Sign() <= 0 {
		t.Fatalf("claim amount should be positive, got %q", claim.Claimed[0].Amount)
	}
	// At most 100 units can ever be paid out of a 100-unit budget.
	if earned.Cmp(big.NewInt(100)) > 0 {
		t.Fatalf("claim %s exceeds the funded budget of 100", earned)
	}
	if got := balance(t, env, "user-1", rewardToken); got.Cmp(earned) != 0 {
		t.Fatalf("claimed %s but the credited balance is %s", earned, got)
	}

	// A second claim with nothing left to settle must not double-pay.
	before := balance(t, env, "user-1", rewardToken)
	status, response = callUserHandler(t, env.engine.Claim, "user-1", http.MethodPost, "/farms/"+farmID+"/claim", `{}`, idParam(farmID))
	if status != http.StatusOK {
		t.Fatalf("second claim: %d %s", status, response)
	}
	after := balance(t, env, "user-1", rewardToken)
	if after.Cmp(before) < 0 {
		t.Fatalf("a second claim must not reduce the balance: %s -> %s", before, after)
	}
}

func TestGetStakesProjectsPendingWithoutMutatingTheStream(t *testing.T) {
	env := newFarmTestEnv(t)
	const stakingToken = "0x1111111111111111111111111111111111111111"
	const rewardToken = "0x2222222222222222222222222222222222222222"
	farmID, _ := seedFarm(t, env, "100", time.Now().Add(-time.Minute), time.Now().Add(24*time.Hour))

	credit(t, env, "user-1", stakingToken, "LP", 18, "1000")
	credit(t, env, "admin-1", rewardToken, "TGR", 18, "1000000")
	if status, response := callUserHandler(t, env.engine.FundFarmReward, "admin-1", http.MethodPost, "/farms/"+farmID+"/fund",
		fmt.Sprintf(`{"token_address":%q,"amount":"1000000"}`, rewardToken), idParam(farmID)); status != http.StatusOK {
		t.Fatalf("fund: %d %s", status, response)
	}
	if status, response := callUserHandler(t, env.engine.Stake, "user-1", http.MethodPost, "/farms/"+farmID+"/stake", `{"amount":"1000"}`, idParam(farmID)); status != http.StatusOK {
		t.Fatalf("stake: %d %s", status, response)
	}

	// Snapshot the persisted stream before the read.
	var before models.FarmReward
	if err := env.db.First(&before, "farm_id = ?", farmID).Error; err != nil {
		t.Fatalf("load reward: %v", err)
	}

	time.Sleep(2500 * time.Millisecond)

	status, response := callUserHandler(t, env.engine.GetStakes, "user-1", http.MethodGet, "/farms/stakes", "", nil)
	if status != http.StatusOK {
		t.Fatalf("get stakes: %d %s", status, response)
	}
	var payload struct {
		Stakes []struct {
			PendingRewards []struct {
				Pending string `json:"pending"`
			} `json:"pending_rewards"`
		} `json:"stakes"`
	}
	if err := json.Unmarshal([]byte(response), &payload); err != nil {
		t.Fatalf("decode stakes: %v (%s)", err, response)
	}
	if len(payload.Stakes) != 1 || len(payload.Stakes[0].PendingRewards) != 1 {
		t.Fatalf("expected one stake with one reward stream: %s", response)
	}
	pending, _ := new(big.Int).SetString(payload.Stakes[0].PendingRewards[0].Pending, 10)
	if pending == nil || pending.Sign() <= 0 {
		t.Fatalf("pending reward should be positive after emission, got %q", payload.Stakes[0].PendingRewards[0].Pending)
	}

	// The projection is read-only: reading pending must not move the accumulator
	// or consume the funded budget, otherwise a reader would compete with a
	// claimer for the same emission.
	var after models.FarmReward
	if err := env.db.First(&after, "farm_id = ?", farmID).Error; err != nil {
		t.Fatalf("reload reward: %v", err)
	}
	if after.AccRewardPerShare != before.AccRewardPerShare {
		t.Fatalf("read changed acc_reward_per_share: %s -> %s", before.AccRewardPerShare, after.AccRewardPerShare)
	}
	if after.TotalDistributed != before.TotalDistributed {
		t.Fatalf("read changed total_distributed: %s -> %s", before.TotalDistributed, after.TotalDistributed)
	}
}

func TestClaimWithoutStakeIsNotFound(t *testing.T) {
	env := newFarmTestEnv(t)
	farmID, _ := seedFarm(t, env, "1", time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	status, response := callUserHandler(t, env.engine.Claim, "user-9", http.MethodPost, "/farms/"+farmID+"/claim", `{}`, idParam(farmID))
	if status != http.StatusNotFound {
		t.Fatalf("claim without a stake should be 404, got %d: %s", status, response)
	}
}

func TestRewardIsSplitProportionallyBetweenStakers(t *testing.T) {
	env := newFarmTestEnv(t)
	const stakingToken = "0x1111111111111111111111111111111111111111"
	const rewardToken = "0x2222222222222222222222222222222222222222"
	start := time.Now().Add(-time.Minute)
	farmID, _ := seedFarm(t, env, "100", start, time.Now().Add(24*time.Hour))

	credit(t, env, "user-1", stakingToken, "LP", 18, "3000")
	credit(t, env, "user-2", stakingToken, "LP", 18, "1000")
	credit(t, env, "admin-1", rewardToken, "TGR", 18, "1000000")
	if status, response := callUserHandler(t, env.engine.FundFarmReward, "admin-1", http.MethodPost, "/farms/"+farmID+"/fund",
		fmt.Sprintf(`{"token_address":%q,"amount":"1000000"}`, rewardToken), idParam(farmID)); status != http.StatusOK {
		t.Fatalf("fund: %d %s", status, response)
	}

	// user-1 stakes 3x user-2's amount, so it must earn about 3x the reward.
	if status, response := callUserHandler(t, env.engine.Stake, "user-1", http.MethodPost, "/farms/"+farmID+"/stake", `{"amount":"3000"}`, idParam(farmID)); status != http.StatusOK {
		t.Fatalf("user-1 stake: %d %s", status, response)
	}
	if status, response := callUserHandler(t, env.engine.Stake, "user-2", http.MethodPost, "/farms/"+farmID+"/stake", `{"amount":"1000"}`, idParam(farmID)); status != http.StatusOK {
		t.Fatalf("user-2 stake: %d %s", status, response)
	}
	time.Sleep(2500 * time.Millisecond)

	earned := func(user string) *big.Int {
		status, response := callUserHandler(t, env.engine.Claim, user, http.MethodPost, "/farms/"+farmID+"/claim", `{}`, idParam(farmID))
		if status != http.StatusOK {
			t.Fatalf("claim %s: %d %s", user, status, response)
		}
		return balance(t, env, user, rewardToken)
	}
	first := earned("user-1")
	second := earned("user-2")
	if first.Sign() <= 0 || second.Sign() <= 0 {
		t.Fatalf("both stakers should earn: user-1=%s user-2=%s", first, second)
	}
	// The accumulator truncates, so the ratio is approximate; a 3:1 stake must
	// still put user-1 comfortably ahead of user-2.
	if first.Cmp(second) <= 0 {
		t.Fatalf("the larger stake should earn more: %s vs %s", first, second)
	}
}

func TestAPRIsNullWithoutAPriceSource(t *testing.T) {
	env := newFarmTestEnv(t)
	farmID, _ := seedFarm(t, env, "100", time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	status, response := callUserHandler(t, env.engine.GetFarm, "", http.MethodGet, "/farms/"+farmID, "", idParam(farmID))
	if status != http.StatusOK {
		t.Fatalf("get farm: %d %s", status, response)
	}
	var view struct {
		Farm struct {
			APR *float64 `json:"apr"`
		} `json:"farm"`
	}
	if err := json.Unmarshal([]byte(response), &view); err != nil {
		t.Fatalf("decode farm: %v (%s)", err, response)
	}
	if view.Farm.APR != nil {
		t.Fatalf("APR must be null without a price source, got %v", *view.Farm.APR)
	}
}

// ------------------------------------------------------------ deletion rules

func TestDeleteFarmRefusesWhileStaked(t *testing.T) {
	env := newFarmTestEnv(t)
	farmID, _ := seedFarm(t, env, "1", time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	credit(t, env, "user-1", "0x1111111111111111111111111111111111111111", "LP", 18, "100")
	if status, response := callUserHandler(t, env.engine.Stake, "user-1", http.MethodPost, "/farms/"+farmID+"/stake", `{"amount":"100"}`, idParam(farmID)); status != http.StatusOK {
		t.Fatalf("stake: %d %s", status, response)
	}
	status, response := callUserHandler(t, env.engine.DeleteFarm, "admin-1", http.MethodDelete, "/admin/farms/"+farmID, "", idParam(farmID))
	if status != http.StatusConflict {
		t.Fatalf("deleting a farm with stake should be 409, got %d: %s", status, response)
	}
}

func TestDeleteEmptyFarmClosesIt(t *testing.T) {
	env := newFarmTestEnv(t)
	farmID, _ := seedFarm(t, env, "1", time.Now().Add(-time.Hour), time.Now().Add(24*time.Hour))
	status, response := callUserHandler(t, env.engine.DeleteFarm, "admin-1", http.MethodDelete, "/admin/farms/"+farmID, "", idParam(farmID))
	if status != http.StatusOK {
		t.Fatalf("delete empty farm: %d %s", status, response)
	}
	var farm models.FarmPool
	if err := env.db.First(&farm, "id = ?", farmID).Error; err != nil {
		t.Fatalf("reload farm: %v", err)
	}
	if farm.Status != models.FarmStatusClosed {
		t.Fatalf("farm status is %q, want closed", farm.Status)
	}
}

// ------------------------------------------------------------ airdrop flow

func TestAirdropClaimRequiresFundingAndPaysOnce(t *testing.T) {
	env := newFarmTestEnv(t)
	airdrop := NewAirdropService(env.db, env.ledger)
	const token = "0x3333333333333333333333333333333333333333"

	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(24 * time.Hour)
	createBody := fmt.Sprintf(`{
		"key":"tgr-genesis","name":"TGR Genesis","chain_id":1,
		"token_address":%q,"token_symbol":"TGR","token_decimals":18,
		"start_time":%q,"end_time":%q
	}`, token, start.Format(time.RFC3339), end.Format(time.RFC3339))
	status, response := callUserHandler(t, airdrop.CreateCampaign, "admin-1", http.MethodPost, "/admin/airdrops", createBody, nil)
	if status != http.StatusCreated {
		t.Fatalf("create campaign: %d %s", status, response)
	}
	var created struct {
		Campaign struct {
			ID string `json:"id"`
		} `json:"campaign"`
	}
	if err := json.Unmarshal([]byte(response), &created); err != nil {
		t.Fatalf("decode campaign: %v (%s)", err, response)
	}
	campaignID := created.Campaign.ID

	// Allocate to two users, then activate the campaign.
	allocBody := `{"allocations":[{"user_id":"user-1","amount":"500"},{"user_id":"user-2","amount":"250"}]}`
	status, response = callUserHandler(t, airdrop.SetAllocations, "admin-1", http.MethodPost, "/admin/airdrops/"+campaignID+"/allocations", allocBody, idParam(campaignID))
	if status != http.StatusOK {
		t.Fatalf("set allocations: %d %s", status, response)
	}
	status, response = callUserHandler(t, airdrop.UpdateCampaign, "admin-1", http.MethodPut, "/admin/airdrops/"+campaignID, `{"status":"active"}`, idParam(campaignID))
	if status != http.StatusOK {
		t.Fatalf("activate campaign: %d %s", status, response)
	}

	// A claim before funding must be refused: nothing was escrowed.
	status, response = callUserHandler(t, airdrop.ClaimAirdrop, "user-1", http.MethodPost, "/airdrops/"+campaignID+"/claim", `{}`, idParam(campaignID))
	if status != http.StatusConflict {
		t.Fatalf("claim before funding should be 409, got %d: %s", status, response)
	}

	credit(t, env, "admin-1", token, "TGR", 18, "750")
	status, response = callUserHandler(t, airdrop.FundCampaign, "admin-1", http.MethodPost, "/admin/airdrops/"+campaignID+"/fund", `{"amount":"750"}`, idParam(campaignID))
	if status != http.StatusOK {
		t.Fatalf("fund campaign: %d %s", status, response)
	}

	status, response = callUserHandler(t, airdrop.ClaimAirdrop, "user-1", http.MethodPost, "/airdrops/"+campaignID+"/claim", `{}`, idParam(campaignID))
	if status != http.StatusOK {
		t.Fatalf("claim: %d %s", status, response)
	}
	if got := balance(t, env, "user-1", token); got.Cmp(big.NewInt(500)) != 0 {
		t.Fatalf("user-1 should receive 500, got %s", got)
	}

	// A second claim must be refused.
	status, response = callUserHandler(t, airdrop.ClaimAirdrop, "user-1", http.MethodPost, "/airdrops/"+campaignID+"/claim", `{}`, idParam(campaignID))
	if status != http.StatusConflict {
		t.Fatalf("double claim should be 409, got %d: %s", status, response)
	}

	// The second user's smaller allocation is still payable from the escrow.
	status, response = callUserHandler(t, airdrop.ClaimAirdrop, "user-2", http.MethodPost, "/airdrops/"+campaignID+"/claim", `{}`, idParam(campaignID))
	if status != http.StatusOK {
		t.Fatalf("user-2 claim: %d %s", status, response)
	}
	if got := balance(t, env, "user-2", token); got.Cmp(big.NewInt(250)) != 0 {
		t.Fatalf("user-2 should receive 250, got %s", got)
	}
}

func TestAirdropAllocationReplacementBlockedAfterClaim(t *testing.T) {
	env := newFarmTestEnv(t)
	airdrop := NewAirdropService(env.db, env.ledger)
	const token = "0x4444444444444444444444444444444444444444"
	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(24 * time.Hour)

	createBody := fmt.Sprintf(`{"key":"k","name":"K","chain_id":1,"token_address":%q,"token_symbol":"TGR","token_decimals":18,"start_time":%q,"end_time":%q}`,
		token, start.Format(time.RFC3339), end.Format(time.RFC3339))
	_, response := callUserHandler(t, airdrop.CreateCampaign, "admin-1", http.MethodPost, "/admin/airdrops", createBody, nil)
	var created struct {
		Campaign struct {
			ID string `json:"id"`
		} `json:"campaign"`
	}
	if err := json.Unmarshal([]byte(response), &created); err != nil {
		t.Fatalf("decode campaign: %v (%s)", err, response)
	}
	campaignID := created.Campaign.ID

	_, _ = callUserHandler(t, airdrop.SetAllocations, "admin-1", http.MethodPost, "/admin/airdrops/"+campaignID+"/allocations",
		`{"allocations":[{"user_id":"user-1","amount":"100"}]}`, idParam(campaignID))
	_, _ = callUserHandler(t, airdrop.UpdateCampaign, "admin-1", http.MethodPut, "/admin/airdrops/"+campaignID, `{"status":"active"}`, idParam(campaignID))
	credit(t, env, "admin-1", token, "TGR", 18, "100")
	_, _ = callUserHandler(t, airdrop.FundCampaign, "admin-1", http.MethodPost, "/admin/airdrops/"+campaignID+"/fund", `{"amount":"100"}`, idParam(campaignID))
	if status, response := callUserHandler(t, airdrop.ClaimAirdrop, "user-1", http.MethodPost, "/airdrops/"+campaignID+"/claim", `{}`, idParam(campaignID)); status != http.StatusOK {
		t.Fatalf("claim: %d %s", status, response)
	}

	status, response := callUserHandler(t, airdrop.SetAllocations, "admin-1", http.MethodPost, "/admin/airdrops/"+campaignID+"/allocations",
		`{"allocations":[{"user_id":"user-2","amount":"50"}]}`, idParam(campaignID))
	if status != http.StatusConflict {
		t.Fatalf("replacing allocations after a claim should be 409, got %d: %s", status, response)
	}
}

func TestAirdropRejectsNonPositiveAllocation(t *testing.T) {
	env := newFarmTestEnv(t)
	airdrop := NewAirdropService(env.db, env.ledger)
	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(24 * time.Hour)
	createBody := fmt.Sprintf(`{"key":"k","name":"K","chain_id":1,"token_address":"0x5555555555555555555555555555555555555555","token_symbol":"TGR","token_decimals":18,"start_time":%q,"end_time":%q}`,
		start.Format(time.RFC3339), end.Format(time.RFC3339))
	_, response := callUserHandler(t, airdrop.CreateCampaign, "admin-1", http.MethodPost, "/admin/airdrops", createBody, nil)
	var created struct {
		Campaign struct {
			ID string `json:"id"`
		} `json:"campaign"`
	}
	if err := json.Unmarshal([]byte(response), &created); err != nil {
		t.Fatalf("decode campaign: %v (%s)", err, response)
	}
	status, response := callUserHandler(t, airdrop.SetAllocations, "admin-1", http.MethodPost, "/admin/airdrops/"+created.Campaign.ID+"/allocations",
		`{"allocations":[{"user_id":"user-1","amount":"0"}]}`, idParam(created.Campaign.ID))
	if status != http.StatusBadRequest {
		t.Fatalf("a zero allocation should be rejected, got %d: %s", status, response)
	}
}

// TestAirdropClaimRefusedWhenCampaignPausedMidFlight asserts that a campaign
// paused between the read and the settling transaction cannot pay out: the
// window is re-checked against the row re-read under lock.
func TestAirdropClaimRefusedWhenCampaignPausedMidFlight(t *testing.T) {
	env := newFarmTestEnv(t)
	airdrop := NewAirdropService(env.db, env.ledger)
	const token = "0x6666666666666666666666666666666666666666"

	start := time.Now().Add(-time.Hour)
	end := time.Now().Add(24 * time.Hour)
	createBody := fmt.Sprintf(`{"key":"k","name":"K","chain_id":1,"token_address":%q,"token_symbol":"TGR","token_decimals":18,"start_time":%q,"end_time":%q}`,
		token, start.Format(time.RFC3339), end.Format(time.RFC3339))
	_, response := callUserHandler(t, airdrop.CreateCampaign, "admin-1", http.MethodPost, "/admin/airdrops", createBody, nil)
	var created struct {
		Campaign struct {
			ID string `json:"id"`
		} `json:"campaign"`
	}
	if err := json.Unmarshal([]byte(response), &created); err != nil {
		t.Fatalf("decode campaign: %v (%s)", err, response)
	}
	campaignID := created.Campaign.ID

	_, _ = callUserHandler(t, airdrop.SetAllocations, "admin-1", http.MethodPost, "/admin/airdrops/"+campaignID+"/allocations",
		`{"allocations":[{"user_id":"user-1","amount":"100"}]}`, idParam(campaignID))
	_, _ = callUserHandler(t, airdrop.UpdateCampaign, "admin-1", http.MethodPut, "/admin/airdrops/"+campaignID, `{"status":"active"}`, idParam(campaignID))
	credit(t, env, "admin-1", token, "TGR", 18, "100")
	_, _ = callUserHandler(t, airdrop.FundCampaign, "admin-1", http.MethodPost, "/admin/airdrops/"+campaignID+"/fund", `{"amount":"100"}`, idParam(campaignID))

	// Pause the campaign; a claim must now be refused even though the escrow is
	// funded and the allocation is unclaimed.
	if status, response := callUserHandler(t, airdrop.UpdateCampaign, "admin-1", http.MethodPut, "/admin/airdrops/"+campaignID, `{"status":"paused"}`, idParam(campaignID)); status != http.StatusOK {
		t.Fatalf("pause campaign: %d %s", status, response)
	}
	status, response := callUserHandler(t, airdrop.ClaimAirdrop, "user-1", http.MethodPost, "/airdrops/"+campaignID+"/claim", `{}`, idParam(campaignID))
	if status != http.StatusConflict {
		t.Fatalf("claim on a paused campaign should be 409, got %d: %s", status, response)
	}
	if got := balance(t, env, "user-1", token); got.Sign() != 0 {
		t.Fatalf("a refused claim must not credit the caller, got %s", got)
	}

	// Reopening makes the same allocation payable exactly once.
	if status, response := callUserHandler(t, airdrop.UpdateCampaign, "admin-1", http.MethodPut, "/admin/airdrops/"+campaignID, `{"status":"active"}`, idParam(campaignID)); status != http.StatusOK {
		t.Fatalf("reactivate campaign: %d %s", status, response)
	}
	if status, response := callUserHandler(t, airdrop.ClaimAirdrop, "user-1", http.MethodPost, "/airdrops/"+campaignID+"/claim", `{}`, idParam(campaignID)); status != http.StatusOK {
		t.Fatalf("claim after reopening: %d %s", status, response)
	}
	if got := balance(t, env, "user-1", token); got.Cmp(big.NewInt(100)) != 0 {
		t.Fatalf("the claim should pay the full allocation once, got %s", got)
	}
}

// ------------------------------------------------------------ helpers

func TestParseTokenAmountRejectsNonNumeric(t *testing.T) {
	for _, raw := range []string{"", "abc", "1.5", "-1", "0x10"} {
		if _, err := parseTokenAmount(raw); err == nil {
			t.Errorf("parseTokenAmount(%q) should fail", raw)
		}
	}
	if value, err := parseTokenAmount(" 42 "); err != nil || value.Int64() != 42 {
		t.Errorf("parseTokenAmount(\" 42 \") = %v, %v", value, err)
	}
}

func TestTokenKeyOrNativeNormalizes(t *testing.T) {
	key, err := tokenKeyOrNative("0xABCDEF0000000000000000000000000000000001")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if key != "0xabcdef0000000000000000000000000000000001" {
		t.Fatalf("address should be lower-cased, got %q", key)
	}
	for _, raw := range []string{"", "native", "NATIVE"} {
		key, err := tokenKeyOrNative(raw)
		if err != nil || key != "native" {
			t.Fatalf("tokenKeyOrNative(%q) = %q, %v", raw, key, err)
		}
	}
	if _, err := tokenKeyOrNative("0xnothex"); err == nil {
		t.Fatal("an invalid address should be rejected")
	}
}

func TestEffectiveAmountAppliesBoost(t *testing.T) {
	amount := big.NewInt(1000)
	if got := effectiveAmount(amount, big.NewInt(10000)); got.Cmp(big.NewInt(1000)) != 0 {
		t.Fatalf("1x boost should be identity, got %s", got)
	}
	if got := effectiveAmount(amount, big.NewInt(25000)); got.Cmp(big.NewInt(2500)) != 0 {
		t.Fatalf("2.5x boost should scale to 2500, got %s", got)
	}
}
