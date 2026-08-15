package payables_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/payables"
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
	got := payables.SettledAmount(dec(t, "100.00"), []decimal.Decimal{dec(t, "50.00"), dec(t, "25.00")})
	if !got.Equal(dec(t, "175.00")) {
		t.Errorf("SettledAmount = %s, want 175.00", got)
	}
}

func TestSettledAmountWithNoLinks(t *testing.T) {
	got := payables.SettledAmount(dec(t, "100.00"), nil)
	if !got.Equal(dec(t, "100.00")) {
		t.Errorf("SettledAmount = %s, want 100.00", got)
	}
}

func TestRemainingAmountClampedToZero(t *testing.T) {
	got := payables.RemainingAmount(dec(t, "100.00"), dec(t, "150.00"))
	if !got.IsZero() {
		t.Errorf("RemainingAmount = %s, want 0 (overpaid clamps to zero)", got)
	}
}

func TestRemainingAmountClampedToTotal(t *testing.T) {
	got := payables.RemainingAmount(dec(t, "100.00"), dec(t, "-10.00"))
	if !got.Equal(dec(t, "100.00")) {
		t.Errorf("RemainingAmount = %s, want 100.00 (negative settled clamps to total)", got)
	}
}

func TestRemainingAmountNormalCase(t *testing.T) {
	got := payables.RemainingAmount(dec(t, "1000.00"), dec(t, "400.00"))
	if !got.Equal(dec(t, "600.00")) {
		t.Errorf("RemainingAmount = %s, want 600.00", got)
	}
}

func TestStatusForSettledOnlyWhenExactlyZeroRemaining(t *testing.T) {
	if payables.StatusFor(dec(t, "0")) != payables.StatusSettled {
		t.Error("zero remaining should be settled")
	}
	if payables.StatusFor(dec(t, "0.01")) != payables.StatusOpen {
		t.Error("any positive remaining should be open")
	}
}
