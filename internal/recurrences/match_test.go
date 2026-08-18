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
// be: a 1500 reference amount with 10% tolerance, and an absolute
// day-of-month window spanning day 28 through day 3 (wraps around day 31).
// Both carry their own reference directly — independent of any commitment.
func baseConditions() []rules.Condition {
	return []rules.Condition{
		{Field: rules.FieldAmount, Operator: rules.OperatorWithinPercent, Value: "1500.00:10"},
		{Field: rules.FieldDayOfMonth, Operator: rules.OperatorDayRange, Value: "28:3"},
	}
}

func TestResolveOccurrenceMatchesWithinToleranceAndDayWindow(t *testing.T) {
	candidates := []transactions.Item{itemAt(t, "2026-01-29", "1520.00")}

	matched, ok := recurrences.ResolveOccurrence(baseConditions(), rules.LogicAnd, candidates)
	if !ok {
		t.Fatal("expected a match within tolerance")
	}
	if matched.ID != "tx-2026-01-29" {
		t.Errorf("unexpected matched item: %+v", matched)
	}
}

func TestResolveOccurrenceRejectsAmountOutsideTolerance(t *testing.T) {
	candidates := []transactions.Item{itemAt(t, "2026-01-30", "2000.00")}

	if _, ok := recurrences.ResolveOccurrence(baseConditions(), rules.LogicAnd, candidates); ok {
		t.Error("expected no match: amount too far from expected")
	}
}

func TestResolveOccurrenceRejectsDayOutsideWindow(t *testing.T) {
	candidates := []transactions.Item{itemAt(t, "2026-01-10", "1500.00")}

	if _, ok := recurrences.ResolveOccurrence(baseConditions(), rules.LogicAnd, candidates); ok {
		t.Error("expected no match: day too far from expected")
	}
}

func TestResolveOccurrenceWithoutDayConditionMatchesRegardlessOfDay(t *testing.T) {
	conditions := []rules.Condition{
		{Field: rules.FieldAmount, Operator: rules.OperatorWithinPercent, Value: "1500.00:10"},
	}
	// Far from the 28:3 window — would fail baseConditions, but there's no
	// day_of_month condition here.
	candidates := []transactions.Item{itemAt(t, "2026-01-10", "1500.00")}

	if _, ok := recurrences.ResolveOccurrence(conditions, rules.LogicAnd, candidates); !ok {
		t.Error("expected a match: day is irrelevant when there's no day_of_month condition")
	}
}

func TestResolveOccurrenceNoCandidatesNeverMatches(t *testing.T) {
	if _, ok := recurrences.ResolveOccurrence(baseConditions(), rules.LogicAnd, nil); ok {
		t.Error("expected no match with zero candidates")
	}
}

func TestResolveOccurrenceUsesAbsoluteValueOfSignedAmount(t *testing.T) {
	candidates := []transactions.Item{itemAt(t, "2026-01-31", "-1500.00")}

	if _, ok := recurrences.ResolveOccurrence(baseConditions(), rules.LogicAnd, candidates); !ok {
		t.Error("expected a match: sign should not matter, only magnitude")
	}
}

func TestResolveOccurrencePicksEarliestMatchingCandidate(t *testing.T) {
	candidates := []transactions.Item{
		itemAt(t, "2026-02-01", "1500.00"),
		itemAt(t, "2026-01-30", "1500.00"),
	}

	matched, ok := recurrences.ResolveOccurrence(baseConditions(), rules.LogicAnd, candidates)
	if !ok {
		t.Fatal("expected a match")
	}
	if matched.ID != "tx-2026-01-30" {
		t.Errorf("expected the earliest matching candidate, got %s", matched.ID)
	}
}
