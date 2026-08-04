package receivables_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/receivables"
)

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestReceivedAmountSumsStartingPlusLinks(t *testing.T) {
	got := receivables.ReceivedAmount(dec(t, "100.00"), []decimal.Decimal{dec(t, "50.00"), dec(t, "25.00")})
	if !got.Equal(dec(t, "175.00")) {
		t.Errorf("ReceivedAmount = %s, want 175.00", got)
	}
}

func TestReceivedAmountWithNoLinks(t *testing.T) {
	got := receivables.ReceivedAmount(dec(t, "100.00"), nil)
	if !got.Equal(dec(t, "100.00")) {
		t.Errorf("ReceivedAmount = %s, want 100.00", got)
	}
}

func TestRemainingAmountClampedToZero(t *testing.T) {
	got := receivables.RemainingAmount(dec(t, "100.00"), dec(t, "150.00"))
	if !got.IsZero() {
		t.Errorf("RemainingAmount = %s, want 0 (over-received clamps to zero)", got)
	}
}

func TestRemainingAmountClampedToTotal(t *testing.T) {
	got := receivables.RemainingAmount(dec(t, "100.00"), dec(t, "-10.00"))
	if !got.Equal(dec(t, "100.00")) {
		t.Errorf("RemainingAmount = %s, want 100.00 (negative received clamps to total)", got)
	}
}

func TestRemainingAmountNormalCase(t *testing.T) {
	got := receivables.RemainingAmount(dec(t, "1000.00"), dec(t, "400.00"))
	if !got.Equal(dec(t, "600.00")) {
		t.Errorf("RemainingAmount = %s, want 600.00", got)
	}
}

func TestStatusForSettledOnlyWhenExactlyZeroRemaining(t *testing.T) {
	if receivables.StatusFor(dec(t, "0")) != receivables.StatusSettled {
		t.Error("zero remaining should be settled")
	}
	if receivables.StatusFor(dec(t, "0.01")) != receivables.StatusOpen {
		t.Error("any positive remaining should be open")
	}
}
