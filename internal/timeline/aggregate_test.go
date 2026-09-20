package timeline_test

import (
	"testing"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/timeline"
)

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestTotalsForPeriod(t *testing.T) {
	cat := categorySupermercado
	zero := decimal.Zero
	series := timeline.Series{
		Entries: []timeline.Entry{
			{Date: date(t, "2026-08-01"), Amount: dec(t, "-100"), CategoryID: &cat, CategoryName: "Supermercado"},
			{Date: date(t, "2026-08-15"), Amount: dec(t, "2000")},
			// A transfer between the user's own accounts: it moves cash
			// (Amount is non-zero) but reports as neither income nor
			// expense.
			{Date: date(t, "2026-09-20"), Amount: dec(t, "-4895"), ReportableAmount: &zero},
			{Date: date(t, "2026-09-01"), Amount: dec(t, "-50")},
		},
	}
	got := timeline.TotalsForPeriod(series)
	if got.Income.String() != "2000" {
		t.Errorf("Income = %s, want 2000", got.Income.String())
	}
	if got.Expense.String() != "150" {
		t.Errorf("Expense = %s, want 150 (the transfer is excluded)", got.Expense.String())
	}
	if got.Result.String() != "1850" {
		t.Errorf("Result = %s, want 1850", got.Result.String())
	}
}
