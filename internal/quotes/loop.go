package quotes

import (
	"context"
	"database/sql"
	"log"
	"time"

	"contadinho-go/internal/settings"
)

// RunSchedule runs RefreshAll once per loop iteration and then sleeps until
// the next scheduled time, until ctx is cancelled — same shape as
// worker.RunSchedule (compute the wait, do the work, sleep, check
// ctx.Err(), repeat), so a slot missed while the process was down still
// gets a run as soon as it comes back up, the same catch-up behavior
// EnqueueDue gives the sync schedule. RefreshAll's own same-day dedupe is
// what makes running it an extra time (at startup, or from a second
// process restart the same day) harmless.
//
// The brapi token is re-read from settings on every tick rather than once
// at startup, so changing it through PUT /api/settings/quotes takes effect
// without a restart.
func RunSchedule(ctx context.Context, conn *sql.DB, secrets *settings.Secrets, schedule Schedule) {
	for {
		wait := time.Until(schedule.Next(time.Now()))
		registry := NewDefaultRegistry(currentBrapiToken(ctx, conn, secrets))
		if _, err := RefreshAll(ctx, conn, registry, time.Now()); err != nil {
			log.Printf("quote_refresh_failed: %v", err)
		}
		sleep(ctx, wait)
		if ctx.Err() != nil {
			return
		}
	}
}

// currentBrapiToken reads the decrypted brapi token from settings, treating
// "not configured" and "server key unavailable" the same way: an empty
// token, which BrapiConnector sends unauthenticated (matching brapi's free
// tier) rather than treating as an error.
func currentBrapiToken(ctx context.Context, conn *sql.DB, secrets *settings.Secrets) string {
	key, ok := secrets.Key()
	if !ok {
		return ""
	}
	token, found, err := settings.GetBrapiToken(ctx, conn, key)
	if err != nil || !found {
		return ""
	}
	return token
}

func sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
