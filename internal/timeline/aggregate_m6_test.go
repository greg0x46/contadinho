package timeline_test

import (
	"testing"

	"contadinho-go/internal/timeline"
)

func TestCategoryEvolutionReconcilesWithCategoryBreakdown(t *testing.T) {
	cat := categorySupermercado
	series := timeline.Series{
		Entries: []timeline.Entry{
			{Date: date(t, "2026-07-01"), Amount: dec(t, "-100"), CategoryID: &cat, CategoryName: "Supermercado"},
			{Date: date(t, "2026-08-01"), Amount: dec(t, "-300"), CategoryID: &cat, CategoryName: "Supermercado"},
			{Date: date(t, "2026-08-15"), Amount: dec(t, "-50")}, // Sem categoria, must not leak into Supermercado's evolution
		},
	}

	evolution := timeline.CategoryEvolution(series, &cat)
	if len(evolution) != 2 {
		t.Fatalf("CategoryEvolution() = %+v, want 2 months", evolution)
	}
	if !evolution[0].Month.Equal(date(t, "2026-07-01")) || evolution[0].Amount.String() != "100" {
		t.Errorf("July = %+v", evolution[0])
	}
	if !evolution[1].Month.Equal(date(t, "2026-08-01")) || evolution[1].Amount.String() != "300" {
		t.Errorf("August = %+v", evolution[1])
	}

	// Reconciliation: August's CategoryEvolution amount must equal the
	// matching row of that month's CategoryBreakdown.
	breakdown := timeline.CategoryBreakdown(series, date(t, "2026-08-01"))
	var supermercadoAmount string
	for _, row := range breakdown {
		if row.CategoryID != nil && *row.CategoryID == cat {
			supermercadoAmount = row.Amount.String()
		}
	}
	if supermercadoAmount != evolution[1].Amount.String() {
		t.Errorf("CategoryBreakdown Supermercado = %s, CategoryEvolution August = %s, want equal", supermercadoAmount, evolution[1].Amount)
	}
}

func TestCategoryEvolutionForSemCategoria(t *testing.T) {
	cat := categorySupermercado
	series := timeline.Series{
		Entries: []timeline.Entry{
			{Date: date(t, "2026-08-01"), Amount: dec(t, "-50")},
			{Date: date(t, "2026-08-05"), Amount: dec(t, "-100"), CategoryID: &cat, CategoryName: "Supermercado"},
		},
	}
	evolution := timeline.CategoryEvolution(series, nil)
	if len(evolution) != 1 || evolution[0].Amount.String() != "50" {
		t.Errorf("CategoryEvolution(nil) = %+v, want [{2026-08-01, 50}]", evolution)
	}
}

func TestMonthOverMonth(t *testing.T) {
	breakdown := []timeline.MonthSummary{
		{Month: date(t, "2026-07-01"), Income: dec(t, "2000"), Expense: dec(t, "1500"), Result: dec(t, "500")},
		{Month: date(t, "2026-08-01"), Income: dec(t, "2000"), Expense: dec(t, "1000"), Result: dec(t, "1000")},
	}
	comparison, ok := timeline.MonthOverMonth(breakdown, date(t, "2026-08-01"))
	if !ok {
		t.Fatal("MonthOverMonth() ok = false, want true")
	}
	if comparison.Current.String() != "1000" || comparison.Previous.String() != "500" || comparison.DeltaPercent.String() != "100" {
		t.Errorf("comparison = %+v, want Current=1000 Previous=500 DeltaPercent=100", comparison)
	}
}

func TestMonthOverMonthWithoutPreviousMonthReturnsNotOK(t *testing.T) {
	breakdown := []timeline.MonthSummary{
		{Month: date(t, "2026-08-01"), Income: dec(t, "2000"), Expense: dec(t, "1000"), Result: dec(t, "1000")},
	}
	_, ok := timeline.MonthOverMonth(breakdown, date(t, "2026-08-01"))
	if ok {
		t.Error("MonthOverMonth() ok = true, want false (no July row at all)")
	}
}

func TestYearOverYear(t *testing.T) {
	current := timeline.Series{
		Entries: []timeline.Entry{
			{Date: date(t, "2026-01-15"), Amount: dec(t, "1000")},
			{Date: date(t, "2026-02-15"), Amount: dec(t, "500")},
		},
	}
	prior := timeline.Series{
		Entries: []timeline.Entry{
			{Date: date(t, "2025-01-15"), Amount: dec(t, "800")},
			{Date: date(t, "2025-02-15"), Amount: dec(t, "200")},
		},
	}
	comparison, ok := timeline.YearOverYear(current, prior, date(t, "2026-02-01"))
	if !ok {
		t.Fatal("YearOverYear() ok = false, want true")
	}
	if comparison.Current.String() != "1500" || comparison.Previous.String() != "1000" {
		t.Errorf("comparison = %+v, want Current=1500 Previous=1000", comparison)
	}
}

func TestYearOverYearWithoutPriorYearDataReturnsNotOK(t *testing.T) {
	current := timeline.Series{
		Entries: []timeline.Entry{{Date: date(t, "2026-01-15"), Amount: dec(t, "1000")}},
	}
	prior := timeline.Series{}
	_, ok := timeline.YearOverYear(current, prior, date(t, "2026-01-01"))
	if ok {
		t.Error("YearOverYear() ok = true, want false (no prior-year data)")
	}
}
