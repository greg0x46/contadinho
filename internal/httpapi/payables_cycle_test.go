package httpapi

import (
	"testing"
	"time"
)

func TestCurrentCreditCardCycleIsHalfOpenInApplicationTimezone(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}

	closing := time.Date(2026, time.August, 2, 0, 0, 0, 0, loc)
	bills := cardBillClosingDates{byAccount: map[string][]time.Time{"card-1": {closing}}}
	cycle, ok := currentCreditCardCycle(bills, "card-1", time.Date(2026, time.August, 15, 12, 0, 0, 0, loc), loc)
	if !ok {
		t.Fatal("currentCreditCardCycle returned no cycle")
	}

	wantStart := time.Date(2026, time.August, 2, 0, 0, 0, 0, loc)
	wantEnd := time.Date(2026, time.September, 2, 0, 0, 0, 0, loc)
	if !cycle.start.Equal(wantStart) || !cycle.end.Equal(wantEnd) {
		t.Fatalf("cycle = [%s, %s), want [%s, %s)", cycle.start, cycle.end, wantStart, wantEnd)
	}

	beforeStart := time.Date(2026, time.August, 1, 23, 59, 59, 0, loc)
	atStart := wantStart
	dayBeforeEnd := time.Date(2026, time.September, 1, 12, 0, 0, 0, loc)
	atEnd := wantEnd
	for name, transactionDate := range map[string]time.Time{
		"before start":   beforeStart,
		"at start":       atStart,
		"day before end": dayBeforeEnd,
		"at end":         atEnd,
	} {
		got := cardTransactionIsInCycle(&transactionDate, cycle)
		want := name == "at start" || name == "day before end"
		if got != want {
			t.Errorf("%s: in cycle = %v, want %v", name, got, want)
		}
	}
}

func TestCurrentCreditCardCycleHandlesYearAndShortMonthBoundaries(t *testing.T) {
	loc := time.UTC
	t.Run("year rollover", func(t *testing.T) {
		closings := []time.Time{
			time.Date(2025, time.November, 2, 0, 0, 0, 0, loc),
			time.Date(2025, time.December, 2, 0, 0, 0, 0, loc),
		}
		cycle, ok := currentCreditCardCycle(
			cardBillClosingDates{byAccount: map[string][]time.Time{"card-1": closings}},
			"card-1", time.Date(2025, time.December, 20, 0, 0, 0, 0, loc), loc,
		)
		if !ok {
			t.Fatal("currentCreditCardCycle returned no cycle")
		}
		if got, want := cycle.end, time.Date(2026, time.January, 2, 0, 0, 0, 0, loc); !got.Equal(want) {
			t.Errorf("end = %s, want %s", got, want)
		}
	})

	t.Run("february clamps a nominal day 31", func(t *testing.T) {
		closings := []time.Time{
			time.Date(2026, time.January, 31, 0, 0, 0, 0, loc),
			time.Date(2026, time.February, 28, 0, 0, 0, 0, loc),
		}
		cycle, ok := currentCreditCardCycle(
			cardBillClosingDates{byAccount: map[string][]time.Time{"card-1": closings}},
			"card-1", time.Date(2026, time.March, 1, 0, 0, 0, 0, loc), loc,
		)
		if !ok {
			t.Fatal("currentCreditCardCycle returned no cycle")
		}
		if got, want := cycle.start, time.Date(2026, time.February, 28, 0, 0, 0, 0, loc); !got.Equal(want) {
			t.Errorf("start = %s, want %s", got, want)
		}
		if got, want := cycle.end, time.Date(2026, time.March, 31, 0, 0, 0, 0, loc); !got.Equal(want) {
			t.Errorf("end = %s, want %s", got, want)
		}
	})
}

func TestCurrentCreditCardCycleUsesLocalCalendarDay(t *testing.T) {
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		t.Fatalf("LoadLocation: %v", err)
	}
	closing := time.Date(2026, time.August, 2, 0, 0, 0, 0, loc)
	bills := cardBillClosingDates{byAccount: map[string][]time.Time{"card-1": {closing}}}
	cycle, ok := currentCreditCardCycle(bills, "card-1", time.Date(2026, time.August, 3, 12, 0, 0, 0, loc), loc)
	if !ok {
		t.Fatal("currentCreditCardCycle returned no cycle")
	}

	// 02:30Z is still August 1st at 23:30 in São Paulo, while 03:00Z is
	// exactly the local opening of August 2nd. A UTC-only date comparison
	// would classify the first transaction incorrectly.
	beforeLocalStart := time.Date(2026, time.August, 2, 2, 30, 0, 0, time.UTC)
	atLocalStart := time.Date(2026, time.August, 2, 3, 0, 0, 0, time.UTC)
	if cardTransactionIsInCycle(&beforeLocalStart, cycle) {
		t.Error("transaction before local closing boundary was included")
	}
	if !cardTransactionIsInCycle(&atLocalStart, cycle) {
		t.Error("transaction at local closing boundary was excluded")
	}
}

func TestCurrentCreditCardCycleIsConservativeWithoutClosingHistory(t *testing.T) {
	loc := time.UTC
	for name, bills := range map[string]cardBillClosingDates{
		"no bills": {byAccount: map[string][]time.Time{}},
		"only future closing": {byAccount: map[string][]time.Time{
			"card-1": {time.Date(2026, time.September, 2, 0, 0, 0, 0, loc)},
		}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, ok := currentCreditCardCycle(bills, "card-1", time.Date(2026, time.August, 15, 0, 0, 0, 0, loc), loc); ok {
				t.Fatal("currentCreditCardCycle returned a cycle without a reliable lower bound")
			}
		})
	}
}
