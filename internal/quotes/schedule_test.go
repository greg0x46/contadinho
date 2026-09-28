package quotes

import (
	"testing"
	"time"
)

// Mirrors internal/worker/schedule_test.go's TestParseSchedule/
// TestScheduleOccurrences: this package's Schedule/ParseSchedule/Next/
// Previous are a deliberate local copy of worker's, so the tests are too.

func TestParseSchedule(t *testing.T) {
	if _, enabled, err := ParseSchedule("  "); err != nil || enabled {
		t.Fatalf("empty schedule: enabled=%v err=%v", enabled, err)
	}
	s, enabled, err := ParseSchedule("06:30 America/Sao_Paulo")
	if err != nil || !enabled || s.Hour != 6 || s.Minute != 30 || s.Location.String() != "America/Sao_Paulo" {
		t.Fatalf("got %+v enabled=%v err=%v", s, enabled, err)
	}
	if s, _, err := ParseSchedule("23:05"); err != nil || s.Location != time.Local {
		t.Fatalf("local schedule: %+v %v", s, err)
	}
	for _, invalid := range []string{"6", "24:00", "06:00 Mars/Olympus", "06:00 UTC extra"} {
		if _, _, err := ParseSchedule(invalid); err == nil {
			t.Errorf("accepted %q", invalid)
		}
	}
}

func TestScheduleOccurrences(t *testing.T) {
	sp, _ := time.LoadLocation("America/Sao_Paulo")
	s := Schedule{Hour: 6, Minute: 0, Location: sp}
	before := time.Date(2026, 9, 15, 5, 59, 0, 0, sp)
	if got := s.Previous(before); !got.Equal(time.Date(2026, 9, 14, 6, 0, 0, 0, sp)) {
		t.Errorf("Previous(before) = %v", got)
	}
	if got := s.Next(before); !got.Equal(time.Date(2026, 9, 15, 6, 0, 0, 0, sp)) {
		t.Errorf("Next(before) = %v", got)
	}
	exact := time.Date(2026, 9, 15, 6, 0, 0, 0, sp)
	if got := s.Previous(exact); !got.Equal(exact) {
		t.Errorf("Previous(exact) = %v", got)
	}
	if got := s.Next(exact); !got.Equal(exact.AddDate(0, 0, 1)) {
		t.Errorf("Next(exact) = %v", got)
	}
	// A caller in another zone still gets the schedule's own wall-clock time.
	utc := time.Date(2026, 9, 15, 9, 30, 0, 0, time.UTC) // 06:30 in São Paulo
	if got := s.Previous(utc); !got.Equal(exact) {
		t.Errorf("Previous(utc) = %v", got)
	}
}
