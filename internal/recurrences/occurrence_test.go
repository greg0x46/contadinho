package recurrences_test

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/recurrences"
)

func date(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func baseCommitment(t *testing.T) recurrences.RecurringCommitment {
	return recurrences.RecurringCommitment{
		ID: "c1", Name: "Aluguel", Kind: recurrences.KindExpense,
		Amount:     dec(t, "1500.00"),
		CategoryID: "cat-aluguel", Cadence: recurrences.CadenceMonthly, DayOfMonth: 31,
		StartDate: date(t, "2026-01-01"), IsActive: true,
	}
}

func TestOccurrencesInRangeClampsDayInShortMonths(t *testing.T) {
	c := baseCommitment(t)
	occurrences := recurrences.OccurrencesInRange(c, date(t, "2026-01-01"), date(t, "2026-03-31"))
	if len(occurrences) != 3 {
		t.Fatalf("expected 3 occurrences, got %d", len(occurrences))
	}
	if got := occurrences[1].Date; !got.Equal(date(t, "2026-02-28")) {
		t.Errorf("expected February occurrence clamped to the 28th, got %v", got)
	}
}

func TestOccurrencesInRangeRespectsStartAndEndDate(t *testing.T) {
	c := baseCommitment(t)
	c.DayOfMonth = 15
	c.StartDate = date(t, "2026-03-01")
	end := date(t, "2026-05-20")
	c.EndDate = &end

	occurrences := recurrences.OccurrencesInRange(c, date(t, "2026-01-01"), date(t, "2026-12-31"))
	if len(occurrences) != 3 {
		t.Fatalf("expected occurrences for Mar/Apr/May only, got %d", len(occurrences))
	}
	if occurrences[0].Date.Month() != time.March || occurrences[2].Date.Month() != time.May {
		t.Errorf("unexpected occurrence range: %+v", occurrences)
	}
}

func TestOccurrencesInRangeInactiveCommitmentYieldsNone(t *testing.T) {
	c := baseCommitment(t)
	c.IsActive = false
	occurrences := recurrences.OccurrencesInRange(c, date(t, "2026-01-01"), date(t, "2026-12-31"))
	if len(occurrences) != 0 {
		t.Errorf("expected no occurrences for an inactive commitment, got %d", len(occurrences))
	}
}

func TestOccurrencesInRangeAnnualCadenceOnlyEmitsInConfiguredMonth(t *testing.T) {
	c := baseCommitment(t)
	c.Cadence = recurrences.CadenceAnnual
	month := 6
	c.MonthOfYear = &month
	c.DayOfMonth = 10

	occurrences := recurrences.OccurrencesInRange(c, date(t, "2026-01-01"), date(t, "2027-12-31"))
	if len(occurrences) != 2 {
		t.Fatalf("expected 2 yearly occurrences, got %d", len(occurrences))
	}
	for _, o := range occurrences {
		if o.Date.Month() != time.June || o.Date.Day() != 10 {
			t.Errorf("expected June 10th occurrence, got %v", o.Date)
		}
	}
}

func TestOccurrencesInRangeExpectedAmountMatchesCommitment(t *testing.T) {
	c := baseCommitment(t)
	occurrences := recurrences.OccurrencesInRange(c, date(t, "2026-01-01"), date(t, "2026-01-31"))
	if len(occurrences) != 1 {
		t.Fatalf("expected 1 occurrence, got %d", len(occurrences))
	}
	if !occurrences[0].ExpectedAmount.Equal(c.Amount) {
		t.Errorf("expected occurrence amount to match commitment amount, got %s", occurrences[0].ExpectedAmount)
	}
}
