package services

import (
	"errors"
	"testing"
)

// TestCloseCopyPositionRejectsOtherAccounts pins the ownership check on the
// close path. A copy position is keyed by an id a caller can enumerate, so the
// handler must not close a position that belongs to another follower -- and it
// must not mutate one either, because the first draft of the handler closed the
// position and only then compared the owner.
func TestCloseCopyPositionRejectsOtherAccounts(t *testing.T) {
	engine, db := newCopyTestEngine(t)
	positionID := seedCopyPosition(t, db, "0xAlice", "open")

	if _, err := engine.CloseCopyPosition(positionID, "0xBob"); !errors.Is(err, errCopyPositionForbidden) {
		t.Fatalf("expected a forbidden error closing another account's position, got %v", err)
	}
	if got := loadCopyPositionStatus(t, db); got != "open" {
		t.Fatalf("a rejected close must leave the position open, got %q", got)
	}

	if _, err := engine.CloseCopyPosition(positionID, ""); !errors.Is(err, errCopyPositionForbidden) {
		t.Fatalf("expected a forbidden error for an empty caller, got %v", err)
	}

	// The owner closes it, and the address comparison is case-insensitive
	// because the follower address is stored as supplied.
	position, err := engine.CloseCopyPosition(positionID, "0xalice")
	if err != nil {
		t.Fatalf("owner close failed: %v", err)
	}
	if position.Status != "closed" || position.ClosedAt == nil {
		t.Fatalf("owner close did not settle the position: %+v", position)
	}

	if _, err := engine.CloseCopyPosition(positionID, "0xAlice"); !errors.Is(err, errCopyPositionClosed) {
		t.Fatalf("expected a closed error on a second close, got %v", err)
	}
}

// TestCloseCopyPositionMissingIsNotFound keeps an unknown id distinct from a
// forbidden one so the handler answers 404 rather than leaking that the id
// exists under a different owner.
func TestCloseCopyPositionMissingIsNotFound(t *testing.T) {
	engine, _ := newCopyTestEngine(t)

	if _, err := engine.CloseCopyPosition("nope", "0xAlice"); !errors.Is(err, errCopyPositionNotFound) {
		t.Fatalf("expected not-found for an unknown id, got %v", err)
	}
}
