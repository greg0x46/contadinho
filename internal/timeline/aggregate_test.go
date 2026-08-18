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

func TestMonthlyBreakdown(t *testing.T) {
	cat := categorySupermercado
	series := timeline.Series{
		Entries: []timeline.Entry{
			{Date: date(t, "2026-08-01"), Amount: dec(t, "-100"), CategoryID: &cat, CategoryName: "Supermercado"},
			{Date: date(t, "2026-08-15"), Amount: dec(t, "2000")},
			{Date: date(t, "2026-09-01"), Amount: dec(t, "-50")},
		},
	}
	got := timeline.MonthlyBreakdown(series)
	if len(got) != 2 {
		t.Fatalf("MonthlyBreakdown() = %+v, want 2 months", got)
	}
	if !got[0].Month.Equal(date(t, "2026-08-01")) || got[0].Income.String() != "2000" || got[0].Expense.String() != "100" || got[0].Result.String() != "1900" {
		t.Errorf("August = %+v", got[0])
	}
	if !got[1].Month.Equal(date(t, "2026-09-01")) || got[1].Expense.String() != "50" {
		t.Errorf("September = %+v", got[1])
	}
}

func TestCategoryBreakdownIncludesSemCategoriaEvenAtZero(t *testing.T) {
	cat := categorySupermercado
	series := timeline.Series{
		Entries: []timeline.Entry{
			{Date: date(t, "2026-08-01"), Amount: dec(t, "-300"), CategoryID: &cat, CategoryName: "Supermercado"},
			{Date: date(t, "2026-08-15"), Amount: dec(t, "1000")}, // income, ignored by CategoryBreakdown
		},
	}
	got := timeline.CategoryBreakdown(series, date(t, "2026-08-01"))
	if len(got) != 2 {
		t.Fatalf("CategoryBreakdown() = %+v, want 2 rows (Supermercado + Sem categoria)", got)
	}
	if got[0].CategoryName != "Supermercado" || got[0].Amount.String() != "300" || got[0].Percentage.String() != "100" {
		t.Errorf("Supermercado row = %+v", got[0])
	}
	if got[1].CategoryName != "Sem categoria" || got[1].CategoryID != nil || !got[1].Amount.IsZero() {
		t.Errorf("Sem categoria row = %+v, want zero and present", got[1])
	}
}

func TestCategoryBreakdownOrdersByDescendingImpact(t *testing.T) {
	small := "11111111-1111-4111-8111-111111111111"
	big := "22222222-2222-4222-8222-222222222222"
	series := timeline.Series{
		Entries: []timeline.Entry{
			{Date: date(t, "2026-08-01"), Amount: dec(t, "-50"), CategoryID: &small, CategoryName: "Pequena"},
			{Date: date(t, "2026-08-02"), Amount: dec(t, "-500"), CategoryID: &big, CategoryName: "Grande"},
		},
	}
	got := timeline.CategoryBreakdown(series, date(t, "2026-08-01"))
	if got[0].CategoryName != "Grande" || got[1].CategoryName != "Pequena" {
		t.Errorf("order = %+v, want Grande before Pequena", got)
	}
}

func TestCategoryBreakdownOnlyIncludesEntriesInMonth(t *testing.T) {
	cat := categorySupermercado
	series := timeline.Series{
		Entries: []timeline.Entry{
			{Date: date(t, "2026-08-01"), Amount: dec(t, "-100"), CategoryID: &cat, CategoryName: "Supermercado"},
			{Date: date(t, "2026-09-01"), Amount: dec(t, "-999"), CategoryID: &cat, CategoryName: "Supermercado"},
		},
	}
	got := timeline.CategoryBreakdown(series, date(t, "2026-08-01"))
	if got[0].Amount.String() != "100" {
		t.Errorf("August Supermercado = %+v, want 100 (September entry excluded)", got[0])
	}
}
