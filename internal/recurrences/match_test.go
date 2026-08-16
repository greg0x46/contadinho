package recurrences_test

import (
	"testing"

	"contadinho-go/internal/money"
	"contadinho-go/internal/recurrences"
	"contadinho-go/internal/rules"
	"contadinho-go/internal/transactions"
)

func itemAt(t *testing.T, dateStr, amount string) transactions.Item {
	t.Helper()
	occurredAt := date(t, dateStr)
	return transactions.Item{
		ID:             "tx-" + dateStr,
		OccurredAt:     &occurredAt,
		EffectiveMoney: &transactions.EffectiveMoneyView{Value: amount, CurrencyCode: "BRL", Source: money.AccountCurrency},
	}
}

// baseConditions mirrors what a linked automation rule's conditions would
// be for baseCommitment: 10% amount tolerance + a 3-day window around the
// scheduled day.
func baseConditions() []rules.Condition {
	return []rules.Condition{
		{Field: rules.FieldAmount, Operator: rules.OperatorWithinPercent, Value: "10"},
		{Field: rules.FieldDayOfMonth, Operator: rules.OperatorNearDay, Value: "3"},
	}
}

func TestResolveOccurrenceMatchesWithinToleranceAndDayWindow(t *testing.T) {
	c := baseCommitment(t)
	occurrence := recurrences.Occurrence{Date: date(t, "2026-01-31"), ExpectedAmount: c.Amount}
	candidates := []transactions.Item{itemAt(t, "2026-01-29", "1520.00")}

	matched, ok := recurrences.ResolveOccurrence(occurrence, baseConditions(), rules.LogicAnd, candidates)
	if !ok {
		t.Fatal("expected a match within tolerance")
	}
	if matched.ID != "tx-2026-01-29" {
		t.Errorf("unexpected matched item: %+v", matched)
	}
}

func TestResolveOccurrenceRejectsAmountOutsideTolerance(t *testing.T) {
	c := baseCommitment(t)
	occurrence := recurrences.Occurrence{Date: date(t, "2026-01-31"), ExpectedAmount: c.Amount}
	candidates := []transactions.Item{itemAt(t, "2026-01-30", "2000.00")}

	if _, ok := recurrences.ResolveOccurrence(occurrence, baseConditions(), rules.LogicAnd, candidates); ok {
		t.Error("expected no match: amount too far from expected")
	}
}

func TestResolveOccurrenceRejectsDayOutsideWindow(t *testing.T) {
	c := baseCommitment(t)
	occurrence := recurrences.Occurrence{Date: date(t, "2026-01-31"), ExpectedAmount: c.Amount}
	candidates := []transactions.Item{itemAt(t, "2026-01-10", "1500.00")}

	if _, ok := recurrences.ResolveOccurrence(occurrence, baseConditions(), rules.LogicAnd, candidates); ok {
		t.Error("expected no match: day too far from expected")
	}
}

func TestResolveOccurrenceWithoutDayConditionMatchesRegardlessOfDay(t *testing.T) {
	c := baseCommitment(t)
	conditions := []rules.Condition{
		{Field: rules.FieldAmount, Operator: rules.OperatorWithinPercent, Value: "10"},
	}
	occurrence := recurrences.Occurrence{Date: date(t, "2026-01-31"), ExpectedAmount: c.Amount}
	// Far from day 31 — would fail the old hardcoded 3-day window, but
	// there's no day_of_month condition here.
	candidates := []transactions.Item{itemAt(t, "2026-01-10", "1500.00")}

	if _, ok := recurrences.ResolveOccurrence(occurrence, conditions, rules.LogicAnd, candidates); !ok {
		t.Error("expected a match: day is irrelevant when there's no day_of_month condition")
	}
}

func TestResolveOccurrenceNoCandidatesNeverMatches(t *testing.T) {
	c := baseCommitment(t)
	occurrence := recurrences.Occurrence{Date: date(t, "2026-01-31"), ExpectedAmount: c.Amount}

	if _, ok := recurrences.ResolveOccurrence(occurrence, baseConditions(), rules.LogicAnd, nil); ok {
		t.Error("expected no match with zero candidates")
	}
}

func TestResolveOccurrenceUsesAbsoluteValueOfSignedAmount(t *testing.T) {
	c := baseCommitment(t)
	occurrence := recurrences.Occurrence{Date: date(t, "2026-01-31"), ExpectedAmount: c.Amount}
	candidates := []transactions.Item{itemAt(t, "2026-01-31", "-1500.00")}

	if _, ok := recurrences.ResolveOccurrence(occurrence, baseConditions(), rules.LogicAnd, candidates); !ok {
		t.Error("expected a match: sign should not matter, only magnitude")
	}
}

func TestResolveOccurrencePicksEarliestMatchingCandidate(t *testing.T) {
	c := baseCommitment(t)
	occurrence := recurrences.Occurrence{Date: date(t, "2026-01-31"), ExpectedAmount: c.Amount}
	candidates := []transactions.Item{
		itemAt(t, "2026-02-01", "1500.00"),
		itemAt(t, "2026-01-30", "1500.00"),
	}

	matched, ok := recurrences.ResolveOccurrence(occurrence, baseConditions(), rules.LogicAnd, candidates)
	if !ok {
		t.Fatal("expected a match")
	}
	if matched.ID != "tx-2026-01-30" {
		t.Errorf("expected the earliest matching candidate, got %s", matched.ID)
	}
}
