package services

import (
	"testing"

	"tigerswap/backend/models"
)

// Collected fees must survive a restart. The records and the revenue report
// used to live only in the collector's process memory, so every fee and the
// whole report were erased the moment the API restarted.
func TestFeeRecordsSurviveRestart(t *testing.T) {
	db := newListingTestDB(t)
	if err := db.AutoMigrate(&models.FeeRecordModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	collector := NewFeeCollectorService()
	collector.SetDB(db)
	if _, err := collector.CollectSwapFee("RUSD", "USDT", "1000", "0xabc", 1, "0xuser", 1, 1); err != nil {
		t.Fatalf("collect swap fee: %v", err)
	}

	// A fresh collector over the same database is what a restart looks like.
	restarted := NewFeeCollectorService()
	restarted.SetDB(db)

	records := restarted.GetFeeRecords("swap")
	if len(records) != 1 {
		t.Fatalf("fee record lost on restart: got %d", len(records))
	}
	if records[0].FeeCharged != 3 {
		t.Fatalf("fee charged = %v, want 3", records[0].FeeCharged)
	}
	if got := restarted.GetTotalRevenue(); got != 3 {
		t.Fatalf("total revenue = %v, want 3", got)
	}
	if report := restarted.GetRevenueReport("all"); report.TotalSwapFees != 3 {
		t.Fatalf("revenue report total swap fees = %v, want 3", report.TotalSwapFees)
	}
}

func TestCalculateFeeExactUsesPercentage(t *testing.T) {
	// 0.3% of 1000 is exactly 3.
	fee, err := calculateFeeExact("1000", 0.3, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := feeAmountString(fee); got != "3.00000000" {
		t.Fatalf("expected 3.00000000, got %q", got)
	}
}

func TestCalculateFeeExactAppliesFixedFloor(t *testing.T) {
	// 0.3% of 1 is 0.003, below the 0.01 floor.
	fee, err := calculateFeeExact("1", 0.3, 0.01)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := feeAmountString(fee); got != "0.01000000" {
		t.Fatalf("expected the fixed floor 0.01000000, got %q", got)
	}
}

// A malformed amount used to be read as zero by fmt.Sscanf, which silently
// produced a zero fee instead of surfacing bad input.
func TestCalculateFeeExactRejectsMalformedAmount(t *testing.T) {
	for _, bad := range []string{"", "abc", "-5", "1e3", "0x10", "1/2"} {
		if _, err := calculateFeeExact(bad, 0.3, 0); err == nil {
			t.Fatalf("amount %q must be rejected", bad)
		}
	}
}

// Small amounts must not drift: 0.3% of 0.1 is exactly 0.0003.
func TestCalculateFeeExactIsPreciseForSmallAmounts(t *testing.T) {
	fee, err := calculateFeeExact("0.1", 0.3, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := feeAmountString(fee); got != "0.00030000" {
		t.Fatalf("expected 0.00030000, got %q", got)
	}
}
