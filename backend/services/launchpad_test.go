package services

import (
	"context"
	"testing"
	"time"
)

func newTestLaunchpad() *LaunchpadService {
	return NewLaunchpadService(nil, nil)
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
