package services

import (
	"context"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"tigerswap/backend/models"
)

func newLaunchpadEscrowProject(t *testing.T, svc *LaunchpadService) *LaunchpadProject {
	t.Helper()
	project, err := svc.CreateProject(context.Background(), "0xOperator", "P", "", "0xToken", "PRJ", "Project",
		"1000000", "1", "RUSD", "1", "10000", "10000", "1000000",
		time.Now().Add(-time.Hour), time.Now().Add(time.Hour), "", "", "")
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := svc.UpdateProjectStatus(project.ID, "active"); err != nil {
		t.Fatalf("UpdateProjectStatus: %v", err)
	}
	return project
}

// TestParticipateEscrowsCommitment pins that a commitment debits the
// participant's own platform balance by the committed amount.
func TestParticipateEscrowsCommitment(t *testing.T) {
	svc, _ := newLaunchpadWithDB(t)
	project := newLaunchpadEscrowProject(t, svc)

	before, err := svc.ledger.Available("0xuser", launchpadTestChainID, launchpadTestToken)
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	if _, err := svc.Participate(context.Background(), "0xuser", "0xuser", project.ID, "500"); err != nil {
		t.Fatalf("Participate: %v", err)
	}
	after, err := svc.ledger.Available("0xuser", launchpadTestChainID, launchpadTestToken)
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	spent := new(big.Int).Sub(before, after)
	want := new(big.Int).Mul(big.NewInt(500), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
	if spent.Cmp(want) != 0 {
		t.Fatalf("escrowed %s, want %s", spent, want)
	}
}

// TestParticipateRefusedWithoutBalance pins that a commitment with no funds is
// rejected and neither the allocation nor the raised total is written.
func TestParticipateRefusedWithoutBalance(t *testing.T) {
	svc, db := newLaunchpadWithDB(t)
	project := newLaunchpadEscrowProject(t, svc)

	_, err := svc.Participate(context.Background(), "0xbroker", "0xbroker", project.ID, "500")
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("err = %v, want ErrInsufficientBalance", err)
	}
	var count int64
	if err := db.Model(&models.LaunchpadAllocation{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("a refused commitment wrote %d allocation rows", count)
	}
	stored, err := svc.GetProject(project.ID)
	if err != nil {
		t.Fatalf("reload project: %v", err)
	}
	if stored.RaisedAmount != "0" {
		t.Fatalf("raised amount = %q, want 0", stored.RaisedAmount)
	}
}

// TestCancelledProjectRefundsEscrow pins that cancelling a project credits each
// commitment back exactly once and marks it refunded.
func TestCancelledProjectRefundsEscrow(t *testing.T) {
	svc, db := newLaunchpadWithDB(t)
	project := newLaunchpadEscrowProject(t, svc)
	fundLaunchpadAccount(t, svc, "0xparticipant")

	before, err := svc.ledger.Available("0xparticipant", launchpadTestChainID, launchpadTestToken)
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	if _, err := svc.Participate(context.Background(), "0xparticipant", "0xparticipant", project.ID, "750"); err != nil {
		t.Fatalf("Participate: %v", err)
	}

	if err := svc.UpdateProjectStatus(project.ID, "cancelled"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	after, err := svc.ledger.Available("0xparticipant", launchpadTestChainID, launchpadTestToken)
	if err != nil {
		t.Fatalf("available: %v", err)
	}
	if after.Cmp(before) != 0 {
		t.Fatalf("balance after refund = %s, want %s (fully refunded)", after, before)
	}

	var row models.LaunchpadAllocation
	if err := db.Where("project_id = ?", project.ID).First(&row).Error; err != nil {
		t.Fatalf("reload allocation: %v", err)
	}
	if !row.Refunded {
		t.Fatal("allocation not marked refunded")
	}

	// A second cancel must not pay again.
	if err := svc.UpdateProjectStatus(project.ID, "cancelled"); err != nil {
		t.Fatalf("second cancel: %v", err)
	}
	again, _ := svc.ledger.Available("0xparticipant", launchpadTestChainID, launchpadTestToken)
	if again.Cmp(before) != 0 {
		t.Fatalf("second cancel changed balance: %s", again)
	}
}

// TestParticipateHandlerReportsInsufficientBalance pins the HTTP path: a
// participant with no balance gets 409, not a recorded commitment.
func TestParticipateHandlerReportsInsufficientBalance(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc, db := newLaunchpadWithDB(t)
	user := seedVoter(t, db, "0x3333333333333333333333333333333333333333", "0", 24*time.Hour)
	project := newLaunchpadEscrowProject(t, svc)

	rec := runAuthenticatedHandler(t, svc.ParticipateHandler, user.ID.String(), gin.H{
		"amount": "100",
	}, gin.Params{{Key: "id", Value: project.ID}})

	if rec.Code != 409 {
		t.Fatalf("status = %d, want 409 (no funds): %s", rec.Code, rec.Body.String())
	}
}
