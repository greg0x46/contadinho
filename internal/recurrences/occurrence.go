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

// OccurrencesInRange is pure: it generates every occurrence of schedule
// within [from, to] (inclusive). A schedule is a calendar and nothing else —
// it has no notion of being paused, which is why the paused-state gate lives on
// RecurringCommitment.Occurrences instead of here.
func OccurrencesInRange(schedule RecurringSchedule, from, to time.Time) []Occurrence {
	if to.Before(from) {
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
