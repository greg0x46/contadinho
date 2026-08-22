package recurrences

import (
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/dates"
)

// Occurrence is one expected instance of a recurring schedule on the
// calendar, before any reconciliation against real transactions.
type Occurrence struct {
	Date           time.Time
	ExpectedAmount decimal.Decimal
}

// occurrenceDate clamps DayOfMonth to the last day of a short month (e.g.
// day 31 in February becomes the 28th or 29th), matching how a real
// calendar bill actually lands.
func occurrenceDate(year int, month time.Month, dayOfMonth int) time.Time {
	day := dayOfMonth
	if last := dates.DaysInMonth(year, month); day > last {
		day = last
	}
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// OccurrencesInRange is pure: it accepts either the legacy
// RecurringCommitment DTO or a RecurringSchedule and generates every
// occurrence within [from, to] (inclusive). The compatibility-shaped input
// keeps old callers source-compatible while the data model moves schedules
// under Scenario.
func OccurrencesInRange(input any, from, to time.Time) []Occurrence {
	schedule, active, ok := normalizeSchedule(input)
	if !ok || !active || to.Before(from) {
		return nil
	}

	var occurrences []Occurrence
	cursor := time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(to.Year(), to.Month(), 1, 0, 0, 0, 0, time.UTC)
	for !cursor.After(last) {
		if schedule.Cadence == CadenceAnnual && (schedule.MonthOfYear == nil || int(cursor.Month()) != *schedule.MonthOfYear) {
			cursor = cursor.AddDate(0, 1, 0)
			continue
		}
		date := occurrenceDate(cursor.Year(), cursor.Month(), schedule.DayOfMonth)
		if date.Before(dates.Day(schedule.StartDate)) {
			cursor = cursor.AddDate(0, 1, 0)
			continue
		}
		if schedule.EndDate != nil && date.After(dates.Day(*schedule.EndDate)) {
			cursor = cursor.AddDate(0, 1, 0)
			continue
		}
		if date.Before(dates.Day(from)) || date.After(dates.Day(to)) {
			cursor = cursor.AddDate(0, 1, 0)
			continue
		}
		occurrences = append(occurrences, Occurrence{Date: date, ExpectedAmount: schedule.Amount})
		cursor = cursor.AddDate(0, 1, 0)
	}
	return occurrences
}

func normalizeSchedule(input any) (RecurringSchedule, bool, bool) {
	switch value := input.(type) {
	case RecurringCommitment:
		return value.Schedule(), value.IsActive, true
	case *RecurringCommitment:
		if value == nil {
			return RecurringSchedule{}, false, false
		}
		return value.Schedule(), value.IsActive, true
	case RecurringSchedule:
		return value, true, true
	case *RecurringSchedule:
		if value == nil {
			return RecurringSchedule{}, false, false
		}
		return *value, true, true
	default:
		return RecurringSchedule{}, false, false
	}
}
