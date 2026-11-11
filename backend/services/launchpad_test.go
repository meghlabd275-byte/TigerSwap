package services

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"tigerswap/backend/models"
)

func TestCreateProjectHandlerIgnoresClientSuppliedCreator(t *testing.T) {
	svc, db := newLaunchpadWithDB(t)
	user := seedVoter(t, db, "0x1111111111111111111111111111111111111111", "0", 24*time.Hour)

	start := time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	end := time.Now().Add(time.Hour).UTC().Format(time.RFC3339)

	rec := runAuthenticatedHandler(t, svc.CreateProjectHandler, user.ID.String(), gin.H{
		"name": "Project", "token_address": "0xToken", "token_symbol": "PRJ",
		"token_name": "Project Token", "total_supply": "1000000", "token_price": "1",
		"payment_token": "RUSD", "min_purchase": "1", "max_purchase": "100",
		"soft_cap": "1000", "hard_cap": "100000", "start_time": start, "end_time": end,
		// A spoofed creator must be ignored in favour of the session identity.
		"creator": "0xSomeoneElse",
	}, nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	project := decodeMarket[LaunchpadProject](t, rec)
	if project.CreatedBy != "0x1111111111111111111111111111111111111111" {
		t.Fatalf("created_by = %q, want the session wallet", project.CreatedBy)
	}
}

func TestParticipateHandlerUsesSessionWallet(t *testing.T) {
	svc, db := newLaunchpadWithDB(t)
	user := seedVoter(t, db, "0x2222222222222222222222222222222222222222", "0", 24*time.Hour)

	project, err := svc.CreateProject(context.Background(), "0xOperator", "P", "", "0xToken", "PRJ", "Project",
		"1000000", "1", "RUSD", "1", "1000", "10000", "1000000", time.Now().Add(-time.Hour), time.Now().Add(time.Hour), "", "", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := svc.UpdateProjectStatus(project.ID, "active"); err != nil {
		t.Fatalf("UpdateProjectStatus: %v", err)
	}

	rec := runAuthenticatedHandler(t, svc.ParticipateHandler, user.ID.String(), gin.H{
		"amount": "100",
		// A spoofed address must not be used as the participant.
		"user_address": "0xAttacker",
	}, gin.Params{{Key: "id", Value: project.ID}})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	allocation := decodeMarket[Allocation](t, rec)
	if allocation.UserAddress != "0x2222222222222222222222222222222222222222" {
		t.Fatalf("allocation user = %q, want the session wallet", allocation.UserAddress)
	}
}

func TestParticipateHandlerRejectsAnonymousCaller(t *testing.T) {
	svc, _ := newLaunchpadWithDB(t)

	rec := runAuthenticatedHandler(t, svc.ParticipateHandler, "", gin.H{"amount": "100"},
		gin.Params{{Key: "id", Value: "proj_x"}})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestCreateProjectHandlerRejectsMalformedTimes(t *testing.T) {
	svc, db := newLaunchpadWithDB(t)
	user := seedVoter(t, db, "0x3333333333333333333333333333333333333333", "0", 24*time.Hour)

	rec := runAuthenticatedHandler(t, svc.CreateProjectHandler, user.ID.String(), gin.H{
		"name": "Project", "token_address": "0xToken", "token_symbol": "PRJ",
		"token_name": "Project Token", "total_supply": "1000000", "token_price": "1",
		"payment_token": "RUSD", "start_time": "not-a-time", "end_time": "also-bad",
	}, nil)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for malformed timestamps", rec.Code)
	}
}

func newTestLaunchpad(t *testing.T) *LaunchpadService {
	t.Helper()
	svc, _ := newLaunchpadWithDB(t)
	return svc
}

func TestCreateProjectRejectsInvertedWindow(t *testing.T) {
	svc := newTestLaunchpad(t)
	start := time.Now().Add(time.Hour)
	end := time.Now()

	_, err := svc.CreateProject(
		context.Background(), "creator-1", "Tiger Raise", "", "0xabc", "TGR", "Tiger",
		"1000000", "0.5", "USDC", "100", "10000", "50000", "250000",
		start, end, "", "", "",
	)
	if err == nil {
		t.Fatal("expected an error when end time precedes start time")
	}
}

func TestCreateProjectStartsUpcomingAndIsListed(t *testing.T) {
	svc := newTestLaunchpad(t)
	start := time.Now().Add(time.Hour)
	end := start.Add(48 * time.Hour)

	project, err := svc.CreateProject(
		context.Background(), "creator-1", "Tiger Raise", "desc", "0xabc", "TGR", "Tiger",
		"1000000", "0.5", "USDC", "100", "10000", "50000", "250000",
		start, end, "https://example.com", "https://example.com/wp.pdf", "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if project.Status != "upcoming" {
		t.Fatalf("expected new project status upcoming, got %q", project.Status)
	}
	if project.RaisedAmount != "0" || project.Participants != 0 {
		t.Fatalf("expected zeroed raise counters, got raised=%q participants=%d", project.RaisedAmount, project.Participants)
	}

	all := svc.GetProjects("")
	if len(all) != 1 || all[0].ID != project.ID {
		t.Fatalf("expected the created project to be listed, got %d projects", len(all))
	}
	if filtered := svc.GetProjects("active"); len(filtered) != 0 {
		t.Fatalf("expected no active projects, got %d", len(filtered))
	}
}

func TestUpdateProjectStatusRejectsUnknownProject(t *testing.T) {
	svc := newTestLaunchpad(t)
	if err := svc.UpdateProjectStatus("missing", "active"); err == nil {
		t.Fatal("expected an error when updating an unknown project")
	}
}

func TestParticipateRequiresActiveProject(t *testing.T) {
	svc := newTestLaunchpad(t)
	start := time.Now().Add(-time.Hour)
	end := start.Add(48 * time.Hour)

	project, err := svc.CreateProject(
		context.Background(), "creator-1", "Tiger Raise", "", "0xabc", "TGR", "Tiger",
		"1000000", "0.5", "USDC", "100", "10000", "50000", "250000",
		start, end, "", "", "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := svc.Participate(context.Background(), "0xuser", project.ID, "500"); err == nil {
		t.Fatal("expected an error participating in an upcoming project")
	}

	if err := svc.UpdateProjectStatus(project.ID, "active"); err != nil {
		t.Fatalf("unexpected error activating project: %v", err)
	}
	if _, err := svc.Participate(context.Background(), "0xuser", project.ID, "500"); err != nil {
		t.Fatalf("unexpected error participating in an active project: %v", err)
	}
}

func TestValidLaunchpadStatusesIsClosedSet(t *testing.T) {
	for _, status := range []string{"upcoming", "active", "completed", "cancelled"} {
		if !validLaunchpadStatuses[status] {
			t.Fatalf("expected %q to be a valid launchpad status", status)
		}
	}
	for _, status := range []string{"", "paused", "delisted", "ACTIVE"} {
		if validLaunchpadStatuses[status] {
			t.Fatalf("expected %q to be rejected as a launchpad status", status)
		}
	}
}

func newLaunchpadWithDB(t *testing.T) (*LaunchpadService, *gorm.DB) {
	t.Helper()
	db := newListingTestDB(t)
	if err := db.AutoMigrate(&models.User{}, &models.VeLock{}, &models.LaunchpadProject{}, &models.LaunchpadAllocation{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewLaunchpadService(db, nil, nil), db
}

// endProject moves a project's end time into the past directly in the store, so
// a test can exercise the post-raise claim path without waiting out the window.
func endProject(t *testing.T, svc *LaunchpadService, projectID string) {
	t.Helper()
	if err := svc.db.Model(&models.LaunchpadProject{}).Where("id = ?", projectID).
		Update("end_time", time.Now().Add(-time.Second)).Error; err != nil {
		t.Fatalf("end project: %v", err)
	}
}

// Projects and allocations must survive a restart. The service used to keep both
// in process memory, so every project and commitment vanished the moment the API
// restarted; a second service instance on the same database must still see them.
func TestLaunchpadStateSurvivesRestart(t *testing.T) {
	svc, db := newLaunchpadWithDB(t)
	start := time.Now().Add(-time.Hour)
	project, err := svc.CreateProject(
		context.Background(), "creator-1", "Tiger Raise", "", "0xabc", "TGR", "Tiger",
		"1000000", "0.5", "USDC", "1", "10000", "50000", "250000",
		start, start.Add(48*time.Hour), "", "", "",
	)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := svc.UpdateProjectStatus(project.ID, "active"); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if _, err := svc.Participate(context.Background(), "0xuser", project.ID, "250"); err != nil {
		t.Fatalf("participate: %v", err)
	}

	// A fresh instance over the same database is what a restart looks like.
	restarted := NewLaunchpadService(db, nil, nil)

	reloaded, err := restarted.GetProject(project.ID)
	if err != nil {
		t.Fatalf("reload project: %v", err)
	}
	if reloaded.RaisedAmount != "250.00000000" || reloaded.Participants != 1 {
		t.Fatalf("project state lost on restart: raised=%q participants=%d", reloaded.RaisedAmount, reloaded.Participants)
	}

	allocation, err := restarted.GetUserAllocation("0xuser", project.ID)
	if err != nil {
		t.Fatalf("reload allocation: %v", err)
	}
	if allocation.AllocatedAmount != "250.00000000" {
		t.Fatalf("allocation state lost on restart: %q", allocation.AllocatedAmount)
	}
}

// seedVoter creates a user with a wallet address and a veTGR lock of `amount`.
func seedVoter(t *testing.T, db *gorm.DB, address, amount string, unlockIn time.Duration) models.User {
	t.Helper()
	user := models.User{
		Email:         address + "@example.com",
		Username:      address,
		WalletAddress: strings.ToLower(address),
		IsActive:      true,
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	lock := models.VeLock{
		UserID:     user.ID.String(),
		Amount:     amount,
		StartTime:  time.Now().Add(-24 * time.Hour),
		UnlockTime: time.Now().Add(unlockIn),
	}
	if err := db.Create(&lock).Error; err != nil {
		t.Fatalf("create lock: %v", err)
	}
	return user
}

// The tier was previously a hardcoded "bronze" for every participant, so the
// configured staking thresholds had no effect.
func TestDetermineTierUsesRealStake(t *testing.T) {
	svc, db := newLaunchpadWithDB(t)

	if got := svc.determineTier("0xNoSuchWallet"); got != "bronze" {
		t.Fatalf("unknown wallet should be bronze, got %q", got)
	}

	// A 4-year lock keeps full power, so the amount maps directly to a tier.
	seedVoter(t, db, "0xGold", "20000", 4*365*24*time.Hour)
	seedVoter(t, db, "0xSilver", "2000", 4*365*24*time.Hour)
	seedVoter(t, db, "0xDiamond", "200000", 4*365*24*time.Hour)

	cases := map[string]string{
		"0xGold":    "gold",
		"0xSilver":  "silver",
		"0xDiamond": "diamond",
		// Case-insensitive lookup: the address is stored lowercase.
		"0XGOLD": "gold",
	}
	for address, want := range cases {
		if got := svc.determineTier(address); got != want {
			t.Fatalf("tier for %s = %q, want %q", address, got, want)
		}
	}
}

// A lock close to expiry holds less voting power than its face value, so the
// tier must follow the decayed power rather than the raw amount.
func TestDetermineTierFollowsDecayedPower(t *testing.T) {
	svc, db := newLaunchpadWithDB(t)

	// The helper opens the lock 24h ago, so a 6h remaining window leaves about
	// 20% of the power: 20000 face value decays to roughly 4000, which is below
	// the 10000 gold threshold but above silver's 1000.
	seedVoter(t, db, "0xDecaying", "20000", 6*time.Hour)

	if got := svc.determineTier("0xDecaying"); got != "silver" {
		t.Fatalf("decayed lock should be silver, got %q", got)
	}
}

// A malformed amount used to be coerced to zero by fmt.Sscanf, which produced a
// misleading "below minimum purchase" error.
func TestParticipateRejectsMalformedAmount(t *testing.T) {
	svc := newTestLaunchpad(t)
	start := time.Now().Add(-time.Hour)
	project, err := svc.CreateProject(
		context.Background(), "creator-1", "Tiger Raise", "", "0xabc", "TGR", "Tiger",
		"1000000", "0.5", "USDC", "100", "10000", "50000", "250000",
		start, start.Add(48*time.Hour), "", "", "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := svc.UpdateProjectStatus(project.ID, "active"); err != nil {
		t.Fatalf("activate: %v", err)
	}

	for _, bad := range []string{"abc", "", "-500", "1e5", "0x10", "5 0", "1/2", "  "} {
		if _, err := svc.Participate(context.Background(), "0xuser", project.ID, bad); err == nil {
			t.Fatalf("amount %q must be rejected", bad)
		}
	}
}

// Raised amounts must accumulate exactly; float rounding used to drift.
func TestParticipateAccumulatesRaisedExactly(t *testing.T) {
	svc := newTestLaunchpad(t)
	start := time.Now().Add(-time.Hour)
	project, err := svc.CreateProject(
		context.Background(), "creator-1", "Tiger Raise", "", "0xabc", "TGR", "Tiger",
		"1000000", "0.5", "USDC", "0.1", "10000", "50000", "250000",
		start, start.Add(48*time.Hour), "", "", "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := svc.UpdateProjectStatus(project.ID, "active"); err != nil {
		t.Fatalf("activate: %v", err)
	}

	for i := 0; i < 3; i++ {
		if _, err := svc.Participate(context.Background(), "0xuser", project.ID, "0.1"); err != nil {
			t.Fatalf("participate: %v", err)
		}
	}
	// The counters are read back from the store: three commitments from one wallet
	// accumulate to 0.3 exactly and count as a single participant.
	stored, err := svc.GetProject(project.ID)
	if err != nil {
		t.Fatalf("reload project: %v", err)
	}
	if stored.RaisedAmount != "0.30000000" {
		t.Fatalf("expected exact raised amount 0.30000000, got %q", stored.RaisedAmount)
	}
	if stored.Participants != 1 {
		t.Fatalf("expected 1 distinct participant, got %d", stored.Participants)
	}
}

// A zero token price used to divide by zero and yield an infinite claim.
func TestClaimRejectsZeroPrice(t *testing.T) {
	svc := newTestLaunchpad(t)
	start := time.Now().Add(-time.Hour)
	project, err := svc.CreateProject(
		context.Background(), "creator-1", "Tiger Raise", "", "0xabc", "TGR", "Tiger",
		"1000000", "0", "USDC", "100", "10000", "50000", "250000",
		start, start.Add(48*time.Hour), "", "", "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := svc.UpdateProjectStatus(project.ID, "active"); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if _, err := svc.Participate(context.Background(), "0xuser", project.ID, "500"); err != nil {
		t.Fatalf("participate: %v", err)
	}

	// Move the project end into the past so claiming is allowed.
	endProject(t, svc, project.ID)

	if _, err := svc.ClaimTokens("0xuser", project.ID); err == nil {
		t.Fatal("a zero token price must not produce a claim")
	}
}

// The exact claim amount must be the payment divided by the price.
func TestClaimComputesExactAmount(t *testing.T) {
	svc := newTestLaunchpad(t)
	start := time.Now().Add(-time.Hour)
	project, err := svc.CreateProject(
		context.Background(), "creator-1", "Tiger Raise", "", "0xabc", "TGR", "Tiger",
		"1000000", "0.5", "USDC", "100", "10000", "50000", "250000",
		start, start.Add(48*time.Hour), "", "", "",
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := svc.UpdateProjectStatus(project.ID, "active"); err != nil {
		t.Fatalf("activate: %v", err)
	}
	if _, err := svc.Participate(context.Background(), "0xuser", project.ID, "1000"); err != nil {
		t.Fatalf("participate: %v", err)
	}
	endProject(t, svc, project.ID)

	claimed, err := svc.ClaimTokens("0xuser", project.ID)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if claimed != "2000.00000000" {
		t.Fatalf("expected 2000.00000000 tokens, got %q", claimed)
	}
}

func TestParseDecimalRejectsBadInput(t *testing.T) {
	for _, bad := range []string{"", " ", "abc", "-1", "+1", "1.2.3", "1e5", "0x10", ".", "1/2", "NaN"} {
		if _, err := parseDecimal(bad); err == nil {
			t.Fatalf("parseDecimal(%q) must fail", bad)
		}
	}
	for input, want := range map[string]string{"0": "0", "1.5": "3/2", "100": "100"} {
		got, err := parseDecimal(input)
		if err != nil {
			t.Fatalf("parseDecimal(%q): %v", input, err)
		}
		if got.RatString() != want {
			t.Fatalf("parseDecimal(%q) = %s, want %s", input, got.RatString(), want)
		}
	}
}

func TestFormatDecimalTruncates(t *testing.T) {
	value, err := parseDecimal("1.999999999")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := formatDecimal(value, 4); got != "1.9999" {
		t.Fatalf("expected truncation to 1.9999, got %q", got)
	}
	small, _ := parseDecimal("0.5")
	if got := formatDecimal(small, 8); got != "0.50000000" {
		t.Fatalf("expected 0.50000000, got %q", got)
	}
}
