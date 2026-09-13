package worker_test

import (
	"context"
	"testing"
	"time"

	"contadinho-go/internal/db"
	"contadinho-go/internal/worker"
)

func TestParseSchedule(t *testing.T) {
	if _, enabled, err := worker.ParseSchedule("  "); err != nil || enabled {
		t.Fatalf("empty schedule: enabled=%v err=%v", enabled, err)
	}
	s, enabled, err := worker.ParseSchedule("06:30 America/Sao_Paulo")
	if err != nil || !enabled || s.Hour != 6 || s.Minute != 30 || s.Location.String() != "America/Sao_Paulo" {
		t.Fatalf("got %+v enabled=%v err=%v", s, enabled, err)
	}
	if s, _, err := worker.ParseSchedule("23:05"); err != nil || s.Location != time.Local {
		t.Fatalf("local schedule: %+v %v", s, err)
	}
	for _, invalid := range []string{"6", "24:00", "06:00 Mars/Olympus", "06:00 UTC extra"} {
		if _, _, err := worker.ParseSchedule(invalid); err == nil {
			t.Errorf("accepted %q", invalid)
		}
	}
}

func TestScheduleOccurrences(t *testing.T) {
	sp, _ := time.LoadLocation("America/Sao_Paulo")
	s := worker.Schedule{Hour: 6, Minute: 0, Location: sp}
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

func TestEnqueueDueIsIdempotentAndSkipsInactive(t *testing.T) {
	conn := newTestConn(t)
	s := worker.Schedule{Hour: 6, Minute: 0, Location: time.UTC}
	ctx := context.Background()
	now := time.Date(2026, 9, 15, 6, 0, 5, 0, time.UTC)

	// Ran yesterday: due again. Inactive: never. Synced manually after the
	// slot: counts as today's sync.
	finished := func(runID string) {
		t.Helper()
		if _, err := conn.Exec(`UPDATE sync_runs SET status = 'completed', finished_at = started_at WHERE id = ?`, runID); err != nil {
			t.Fatal(err)
		}
	}
	fresh, run := insertSourceAndRun(t, conn, "item-fresh", "in_progress", now.AddDate(0, 0, -1))
	finished(run)
	inactive, run := insertSourceAndRun(t, conn, "item-inactive", "in_progress", now.AddDate(0, 0, -1))
	finished(run)
	if _, err := conn.Exec(`UPDATE data_sources SET is_active = 0 WHERE id = ?`, inactive); err != nil {
		t.Fatal(err)
	}
	manual, run := insertSourceAndRun(t, conn, "item-manual", "in_progress", now.Add(-2*time.Second))
	finished(run)

	created, err := worker.EnqueueDue(ctx, conn, s, now)
	if err != nil {
		t.Fatalf("EnqueueDue: %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("created %d runs, want 1", len(created))
	}
	var sourceID, status string
	if err := conn.QueryRow(`SELECT source_id, status FROM sync_runs WHERE id = ?`, created[0]).Scan(&sourceID, &status); err != nil {
		t.Fatal(err)
	}
	if sourceID != fresh || status != "in_progress" {
		t.Errorf("run for %s (%s), want %s in_progress", sourceID, status, fresh)
	}

	// Re-entering later the same day (a restart) must not enqueue again,
	// whether the run is still in progress or already finished.
	for _, step := range []string{"in_progress", "completed"} {
		if step == "completed" {
			finished(created[0])
		}
		again, err := worker.EnqueueDue(ctx, conn, s, now.Add(3*time.Hour))
		if err != nil || len(again) != 0 {
			t.Fatalf("status %s: re-enqueued %v (err %v)", step, again, err)
		}
	}

	// The next day every active connection is due again.
	tomorrow := now.AddDate(0, 0, 1)
	created, err = worker.EnqueueDue(ctx, conn, s, tomorrow)
	if err != nil || len(created) != 2 {
		t.Fatalf("next day: %d runs (err %v), want 2", len(created), err)
	}
	var count int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM sync_runs WHERE source_id = ? AND started_at >= ?`,
		manual, db.FormatTime(tomorrow)).Scan(&count); err != nil || count != 1 {
		t.Errorf("manual source next-day runs = %d (err %v), want 1", count, err)
	}
	if err := conn.QueryRow(`SELECT COUNT(*) FROM sync_runs WHERE source_id = ?`, inactive).Scan(&count); err != nil || count != 1 {
		t.Errorf("inactive source runs = %d (err %v), want the original only", count, err)
	}
}
