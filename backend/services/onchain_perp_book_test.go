package services

import (
	"math/big"
	"testing"

	"tigerswap/backend/go/orderbook"
)

func wad(s string) *big.Int {
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("bad integer " + s)
	}
	return v
}

// TestMarkFromBookUsesMidWhenTwoSided pins the mark to the book mid when both
// sides are present: that is what makes the funding rate non-zero.
func TestMarkFromBookUsesMidWhenTwoSided(t *testing.T) {
	index := wad("100000000000000000000") // 100
	bid := wad("99900000000000000000")    // 99.9
	ask := wad("100100000000000000000")   // 100.1
	got := markFromBook(bid, ask, index)
	want := wad("100000000000000000000") // mid = 100
	if got.Cmp(want) != 0 {
		t.Fatalf("mark = %s, want %s", got, want)
	}
}

// TestMarkFromBookFallsBackToIndexWhenOneSided keeps funding inert rather than
// deriving a mark from a one-sided quote.
func TestMarkFromBookFallsBackToIndexWhenOneSided(t *testing.T) {
	index := wad("100000000000000000000")
	for _, tc := range []struct {
		name     string
		bid, ask *big.Int
	}{
		{"no bid", big.NewInt(0), wad("100100000000000000000")},
		{"no ask", wad("99900000000000000000"), big.NewInt(0)},
		{"both empty", big.NewInt(0), big.NewInt(0)},
	} {
		got := markFromBook(tc.bid, tc.ask, index)
		if got.Cmp(index) != 0 {
			t.Errorf("%s: mark = %s, want index %s", tc.name, got, index)
		}
	}
}

// TestMarkFromBookClampsToBand stops a dislocated book from moving the mark
// arbitrarily far from the index.
func TestMarkFromBookClampsToBand(t *testing.T) {
	index := wad("100000000000000000000")
	high := wad("200000000000000000000")
	low := wad("10000000000000000000")

	got := markFromBook(high, high, index)
	wantHigh := wad("105000000000000000000") // +5%
	if got.Cmp(wantHigh) != 0 {
		t.Errorf("high mid mark = %s, want %s", got, wantHigh)
	}

	got = markFromBook(low, low, index)
	wantLow := wad("95000000000000000000") // -5%
	if got.Cmp(wantLow) != 0 {
		t.Errorf("low mid mark = %s, want %s", got, wantLow)
	}
}

// TestMarkFromBookZeroIndexIsZero refuses to invent a mark when the index is
// missing, so the caller never accrues funding on a fabricated price.
func TestMarkFromBookZeroIndexIsZero(t *testing.T) {
	if got := markFromBook(wad("1"), wad("1"), big.NewInt(0)); got.Sign() != 0 {
		t.Fatalf("mark = %s, want 0", got)
	}
	if got := markFromBook(wad("1"), wad("1"), nil); got.Sign() != 0 {
		t.Fatalf("mark = %s, want 0", got)
	}
}

// TestBookBelowMaintenanceMirrorsContract pins the keeper's health check to the
// contract's: equity below `abs(size) * mark / WAD * maintenanceMargin / WAD`.
// A size of 1 at mark 100 with a 5% maintenance requires 5 of equity.
func TestBookBelowMaintenanceMirrorsContract(t *testing.T) {
	market := &orderbook.Market{MaintenanceMargin: wad("50000000000000000")} // 5%
	size := wad("1000000000000000000")                                       // 1
	mark := wad("100000000000000000000")                                     // 100

	if !bookBelowMaintenance(wad("4999999999999999999"), size, market, mark) {
		t.Fatal("equity just under maintenance must be liquidatable")
	}
	if bookBelowMaintenance(wad("5000000000000000000"), size, market, mark) {
		t.Fatal("equity exactly at maintenance must not be liquidatable")
	}
	if bookBelowMaintenance(wad("6000000000000000000"), size, market, mark) {
		t.Fatal("equity above maintenance must not be liquidatable")
	}
}

// TestBookBelowMaintenanceGuardsEmptyInputs ensures a missing size, equity or
// mark never reports a liquidation the contract would reject.
func TestBookBelowMaintenanceGuardsEmptyInputs(t *testing.T) {
	market := &orderbook.Market{MaintenanceMargin: wad("50000000000000000")}
	size := wad("1000000000000000000")
	mark := wad("100000000000000000000")

	if bookBelowMaintenance(nil, size, market, mark) {
		t.Fatal("nil equity must not be liquidatable")
	}
	if bookBelowMaintenance(wad("1"), big.NewInt(0), market, mark) {
		t.Fatal("zero size must not be liquidatable")
	}
	if bookBelowMaintenance(wad("1"), size, market, big.NewInt(0)) {
		t.Fatal("zero mark must not be liquidatable")
	}
}
