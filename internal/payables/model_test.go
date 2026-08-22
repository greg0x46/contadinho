// This file is an internal test (package payables, not payables_test): it
// exercises the pure settled/remaining/status math directly, and that math
// is unexported so nothing outside the package assembles it by hand instead
// of going through Summarize.
package payables

import (
	"testing"

	"github.com/shopspring/decimal"
)

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestSettledAmountSumsStartingPlusLinks(t *testing.T) {
	got := settledAmount(dec(t, "100.00"), []decimal.Decimal{dec(t, "50.00"), dec(t, "25.00")})
	if !got.Equal(dec(t, "175.00")) {
		t.Errorf("settledAmount = %s, want 175.00", got)
	}
}

func TestSettledAmountWithNoLinks(t *testing.T) {
	got := settledAmount(dec(t, "100.00"), nil)
	if !got.Equal(dec(t, "100.00")) {
		t.Errorf("settledAmount = %s, want 100.00", got)
	}
}

func TestRemainingAmountClampedToZero(t *testing.T) {
	got := remainingAmount(dec(t, "100.00"), dec(t, "150.00"))
	if !got.IsZero() {
		t.Errorf("remainingAmount = %s, want 0 (overpaid clamps to zero)", got)
	}
}

func TestRemainingAmountClampedToTotal(t *testing.T) {
	got := remainingAmount(dec(t, "100.00"), dec(t, "-10.00"))
	if !got.Equal(dec(t, "100.00")) {
		t.Errorf("remainingAmount = %s, want 100.00 (negative settled clamps to total)", got)
	}
}

func TestRemainingAmountNormalCase(t *testing.T) {
	got := remainingAmount(dec(t, "1000.00"), dec(t, "400.00"))
	if !got.Equal(dec(t, "600.00")) {
		t.Errorf("remainingAmount = %s, want 600.00", got)
	}
}

func TestStatusForSettledOnlyWhenExactlyZeroRemaining(t *testing.T) {
	if statusFor(dec(t, "0")) != StatusSettled {
		t.Error("zero remaining should be settled")
	}
	if statusFor(dec(t, "0.01")) != StatusOpen {
		t.Error("any positive remaining should be open")
	}
}
