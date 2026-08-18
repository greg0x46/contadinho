package recurrences

import (
	"time"

	"github.com/shopspring/decimal"
)

// Occurrence is one expected instance of a RecurringCommitment on the
// calendar, before any reconciliation against real transactions.
type Occurrence struct {
	Date           time.Time
	ExpectedAmount decimal.Decimal
}

func daysInMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// occurrenceDate clamps DayOfMonth to the last day of a short month (e.g.
// day 31 in February becomes the 28th or 29th), matching how a real
// calendar bill actually lands.
func occurrenceDate(year int, month time.Month, dayOfMonth int) time.Time {
	day := dayOfMonth
	if last := daysInMonth(year, month); day > last {
		day = last
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// OccurrencesInRange is pure: it generates every expected occurrence of
// commitment within [from, to] (inclusive), respecting StartDate/EndDate and
// IsActive, and — for an annual cadence — only emitting in MonthOfYear.
func OccurrencesInRange(commitment RecurringCommitment, from, to time.Time) []Occurrence {
	if !commitment.IsActive || to.Before(from) {
		return nil
	}

	var occurrences []Occurrence
	cursor := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC)
	for !cursor.After(last) {
		if commitment.Cadence == CadenceAnnual && int(cursor.Month()) != *commitment.MonthOfYear {
			cursor = cursor.AddDate(0, 1, 0)
			continue
		}
		date := occurrenceDate(cursor.Year(), cursor.Month(), commitment.DayOfMonth)
		if date.Before(dateOnly(commitment.StartDate)) {
			cursor = cursor.AddDate(0, 1, 0)
			continue
		}
		if commitment.EndDate != nil && date.After(dateOnly(*commitment.EndDate)) {
			cursor = cursor.AddDate(0, 1, 0)
			continue
		}
		if date.Before(dateOnly(from)) || date.After(dateOnly(to)) {
			cursor = cursor.AddDate(0, 1, 0)
			continue
		}
		occurrences = append(occurrences, Occurrence{Date: date, ExpectedAmount: commitment.Amount})
		cursor = cursor.AddDate(0, 1, 0)
	}
	return occurrences
}

func dateOnly(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
