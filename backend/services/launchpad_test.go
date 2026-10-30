package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"

	"tigerswap/backend/models"
)

func newTestLaunchpad() *LaunchpadService {
	return NewLaunchpadService(nil, nil, nil)
}

func TestCreateProjectRejectsInvertedWindow(t *testing.T) {
	svc := newTestLaunchpad()
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
	svc := newTestLaunchpad()
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
	svc := newTestLaunchpad()
	if err := svc.UpdateProjectStatus("missing", "active"); err == nil {
		t.Fatal("expected an error when updating an unknown project")
	}
}

func TestParticipateRequiresActiveProject(t *testing.T) {
	svc := newTestLaunchpad()
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
	if err := db.AutoMigrate(&models.User{}, &models.VeLock{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return NewLaunchpadService(db, nil, nil), db
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
	svc := newTestLaunchpad()
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
	svc := newTestLaunchpad()
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
	if project.RaisedAmount != "0.30000000" {
		t.Fatalf("expected exact raised amount 0.30000000, got %q", project.RaisedAmount)
	}
	if project.Participants != 3 {
		t.Fatalf("expected 3 participants, got %d", project.Participants)
	}
}

// A zero token price used to divide by zero and yield an infinite claim.
func TestClaimRejectsZeroPrice(t *testing.T) {
	svc := newTestLaunchpad()
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
	project.EndTime = time.Now().Add(-time.Second)

	if _, err := svc.ClaimTokens("0xuser", project.ID); err == nil {
		t.Fatal("a zero token price must not produce a claim")
	}
}

// The exact claim amount must be the payment divided by the price.
func TestClaimComputesExactAmount(t *testing.T) {
	svc := newTestLaunchpad()
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
	project.EndTime = time.Now().Add(-time.Second)

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
