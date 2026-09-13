package worker

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"contadinho-go/internal/datasources"
	"contadinho-go/internal/db"
	"contadinho-go/internal/syncsvc"
)

// Schedule is a daily wall-clock time at which every active connection is
// enqueued for sync, replacing the external cron that used to POST
// /api/sync-runs — that route now requires a browser session, and the
// worker no longer depends on anyone having logged in, so the trigger can
// live inside the process too.
type Schedule struct {
	Hour, Minute int
	Location     *time.Location
}

// ParseSchedule reads CONTADINHO_SYNC_SCHEDULE: "HH:MM" in the process's
// local time, optionally followed by an IANA zone ("06:00 America/Sao_Paulo").
// An empty value disables the schedule.
func ParseSchedule(value string) (Schedule, bool, error) {
	fields := strings.Fields(value)
	if len(fields) == 0 {
		return Schedule{}, false, nil
	}
	if len(fields) > 2 {
		return Schedule{}, false, fmt.Errorf("CONTADINHO_SYNC_SCHEDULE deve ser \"HH:MM\" ou \"HH:MM Zona/IANA\"")
	}
	clock, err := time.Parse("15:04", fields[0])
	if err != nil {
		return Schedule{}, false, fmt.Errorf("CONTADINHO_SYNC_SCHEDULE: horário inválido %q (use HH:MM)", fields[0])
	}
	s := Schedule{Hour: clock.Hour(), Minute: clock.Minute(), Location: time.Local}
	if len(fields) == 2 {
		if s.Location, err = time.LoadLocation(fields[1]); err != nil {
			return Schedule{}, false, fmt.Errorf("CONTADINHO_SYNC_SCHEDULE: fuso inválido %q", fields[1])
		}
	}
	return s, true, nil
}

func (s Schedule) at(day time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), s.Hour, s.Minute, 0, 0, s.Location)
}

// Previous is the most recent occurrence at or before now.
func (s Schedule) Previous(now time.Time) time.Time {
	now = now.In(s.Location)
	if due := s.at(now); !due.After(now) {
		return due
	}
	return s.at(now.AddDate(0, 0, -1))
}

// Next is the first occurrence strictly after now.
func (s Schedule) Next(now time.Time) time.Time {
	return s.Previous(now).AddDate(0, 0, 1)
}

// EnqueueDue enqueues every active connection that has not started a run
// since the schedule's latest occurrence, and returns the run ids it created.
// Skipping connections that already have a run since then is what makes the
// loop safe to re-enter: a restart shortly after the scheduled time neither
// repeats the sync nor loses it, and a manual "Sincronizar agora" that
// happened after the slot counts as the day's sync.
func EnqueueDue(ctx context.Context, conn *sql.DB, s Schedule, now time.Time) ([]string, error) {
	sources, err := datasources.ListActive(ctx, conn)
	if err != nil {
		return nil, err
	}
	due := db.FormatTime(s.Previous(now))
	var created []string
	for _, source := range sources {
		var n int
		if err := conn.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sync_runs WHERE source_id = ? AND started_at >= ?`, source.ID, due,
		).Scan(&n); err != nil {
			return created, err
		}
		if n > 0 {
			continue
		}
		runID, busy, err := syncsvc.Enqueue(ctx, conn, source.ID, now)
		if err != nil {
			return created, fmt.Errorf("enqueue %s: %w", source.ID, err)
		}
		if busy {
			continue
		}
		log.Printf("sync_run_scheduled run_id=%s source_id=%s", runID, source.ID)
		created = append(created, runID)
	}
	return created, nil
}

// RunSchedule enqueues the connections due now (catching up a slot missed
// while the process was down), then once per day at the scheduled time,
// until ctx is cancelled. It only creates rows; Run executes them.
func RunSchedule(ctx context.Context, conn *sql.DB, s Schedule) {
	for {
		wait := time.Until(s.Next(time.Now()))
		if _, err := EnqueueDue(ctx, conn, s, time.Now()); err != nil {
			// EnqueueDue is idempotent, so a transient database error can
			// simply be retried shortly instead of skipping the day.
			log.Printf("sync_schedule_failed: %v", err)
			wait = time.Minute
		}
		sleep(ctx, wait)
		if ctx.Err() != nil {
			return
		}
	}
}
