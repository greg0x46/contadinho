package dates_test

import (
	"testing"
	"time"

	"contadinho-go/internal/dates"
)

// TestDaySameWallClockDateInDifferentZonesCompareEqual is the whole reason
// this package exists: two engines can arrive at the same calendar date by
// different routes — one parsing a UTC date column, one computing in
// time.Local — and the results must compare equal. Leaving each caller's own
// zone on the truncated value would produce two different instants for what
// both engines consider the same date.
func TestDaySameWallClockDateInDifferentZonesCompareEqual(t *testing.T) {
	utc := time.Date(2026, 5, 10, 3, 0, 0, 0, time.UTC)
	east := time.Date(2026, 5, 10, 23, 30, 0, 0, time.FixedZone("UTC+9", 9*3600))

	if !dates.Day(utc).Equal(dates.Day(east)) {
		t.Errorf("Day(%v) = %v, Day(%v) = %v — the same calendar day must compare equal",
			utc, dates.Day(utc), east, dates.Day(east))
	}
	if got := dates.Day(utc); got != time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC) {
		t.Errorf("Day = %v, want midnight UTC on 2026-05-10", got)
	}
}

// TestDayKeepsTheWallClockDate pins that Day reads the date off the time as
// the caller sees it rather than converting to UTC first — 2026-05-10 late
// evening in a +09:00 zone is the 10th, not the 9th.
func TestDayKeepsTheWallClockDate(t *testing.T) {
	east := time.Date(2026, 5, 10, 23, 30, 0, 0, time.FixedZone("UTC+9", 9*3600))
	if got := dates.Day(east); got.Day() != 10 || got.Month() != time.May {
		t.Errorf("Day = %v, want 2026-05-10", got)
	}
}

// TestDayOfOneInstantDiffersAcrossZones pins the corollary of the rule
// above, which is the sharp edge a caller has to know about: Day is not an
// instant-to-day mapping. One moment in time is a different calendar date
// depending on the zone the time.Time carries, and Day faithfully reports
// each. A caller that means "the day this instant fell on in zone Z" has to
// convert to Z before calling.
func TestDayOfOneInstantDiffersAcrossZones(t *testing.T) {
	instant := time.Date(2026, 5, 11, 1, 0, 0, 0, time.UTC)
	west := instant.In(time.FixedZone("UTC-5", -5*3600)) // 2026-05-10 20:00

	if dates.Day(instant).Equal(dates.Day(west)) {
		t.Errorf("Day(%v) = %v and Day(%v) = %v — the same instant is a different wall-clock date in each zone, and Day must report each faithfully",
			instant, dates.Day(instant), west, dates.Day(west))
	}
	if got := dates.Day(west); got != time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC) {
		t.Errorf("Day(west) = %v, want midnight UTC on 2026-05-10", got)
	}
}

// TestDayIsIdempotent pins that re-truncating an already-truncated day is a
// no-op — callers layer Day over each other freely (BuildSeries truncates its
// window, then ListPlanInstallments truncates it again).
func TestDayIsIdempotent(t *testing.T) {
	once := dates.Day(time.Date(2026, 5, 10, 14, 22, 9, 500, time.UTC))
	if twice := dates.Day(once); !twice.Equal(once) {
		t.Errorf("Day(Day(t)) = %v, want %v", twice, once)
	}
}

// TestDaysInMonthIsZoneIndependent pins the property that let DaysInMonth
// drop the *time.Location parameter the three copies it replaced disagreed
// about (one keyed on time.Local, two on UTC): how many days a month has is
// calendar arithmetic, not a point in time, so no zone can change it.
func TestDaysInMonthIsZoneIndependent(t *testing.T) {
	// Zones on both sides of UTC, plus one that historically shifted its
	// clocks at midnight — the case that makes the day-zero normalization
	// ambiguous if it were done in a local zone rather than UTC.
	for _, name := range []string{"UTC", "America/Sao_Paulo", "Asia/Tokyo", "America/Los_Angeles"} {
		loc, err := time.LoadLocation(name)
		if err != nil {
			t.Skipf("zone %s unavailable: %v", name, err)
		}
		for month := time.January; month <= time.December; month++ {
			want := dates.DaysInMonth(2026, month)
			got := time.Date(2026, month+1, 0, 0, 0, 0, 0, loc).Day()
			if got != want {
				t.Errorf("%s: %v 2026 has %d days, DaysInMonth says %d", name, month, got, want)
			}
		}
	}
}

// TestDaysInMonthLeapYear pins February, the only month whose answer moves.
func TestDaysInMonthLeapYear(t *testing.T) {
	for _, tc := range []struct {
		year int
		want int
	}{
		{2026, 28},
		{2024, 29},
		{2000, 29}, // divisible by 400 — a leap year
		{1900, 28}, // divisible by 100 but not 400 — not one
	} {
		if got := dates.DaysInMonth(tc.year, time.February); got != tc.want {
			t.Errorf("DaysInMonth(%d, February) = %d, want %d", tc.year, got, tc.want)
		}
	}
}

// TestDaysInMonthClampsAMonthlyCadence exercises what every caller actually
// uses it for: a commitment or card closing on the 31st has to land on the
// last day of a shorter month, never spill into the next one.
func TestDaysInMonthClampsAMonthlyCadence(t *testing.T) {
	for _, tc := range []struct {
		month time.Month
		want  int
	}{
		{time.November, 30},
		{time.February, 28},
		{time.December, 31},
	} {
		day := 31
		if max := dates.DaysInMonth(2026, tc.month); day > max {
			day = max
		}
		if day != tc.want {
			t.Errorf("day 31 clamped to %v 2026 = %d, want %d", tc.month, day, tc.want)
		}
	}
}
