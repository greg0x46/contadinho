package quotes

import (
	"context"
	"database/sql"
	"log"
	"time"

	"contadinho-go/internal/investments"
	"contadinho-go/internal/marketdata"
	"contadinho-go/internal/settings"
)

// backfillRequests carries "something that changes what needs pricing was
// just saved" from the HTTP handlers to the scheduler. One slot is enough:
// the scan it triggers looks at everything, so any number of requests that
// arrive while one is pending are the same request.
var backfillRequests = make(chan struct{}, 1)

// backfillDebounce lets a burst of saves (a position, then its operations)
// settle before the scan looks at them.
const backfillDebounce = 2 * time.Second

// RequestBackfill asks the running scheduler to look for price history that is
// missing — after a position or an operation was saved with an earlier date,
// an asset got a quote source, and so on. It never blocks and never fails: when
// quoting is not scheduled (no CONTADINHO_QUOTES_SCHEDULE) nothing is listening
// and the request is dropped, so a fresh install does not start market requests.
func RequestBackfill() {
	select {
	case backfillRequests <- struct{}{}:
	default:
	}
}

// RunSchedule prices every quoted asset once a day and fills in the history
// that is missing, until ctx is cancelled — same shape as worker.RunSchedule
// (do the work, sleep until the next slot, check ctx.Err(), repeat), so a
// slot missed while the process was down still gets a run as soon as it comes
// back up, the same catch-up behavior EnqueueDue gives the sync schedule. The
// daily run's own same-day dedupe is what makes running it an extra time (at
// startup, or from a second process restart the same day) harmless, and the
// history scan asks nothing of a provider for a range it has already asked
// about.
//
// Between daily runs the loop also wakes for RequestBackfill and prices what
// has no price today yet and fills in the missing history: that is how a
// position registered with a past date gets its prices within seconds instead
// of at the next scheduled slot.
//
// The service is shared across runs. Its brapi token callback reads settings
// per request, so changing the token takes effect without a restart.
func RunSchedule(ctx context.Context, conn *sql.DB, service *marketdata.Service, schedule Schedule) {
	nextDaily := time.Now()
	for {
		now := time.Now()
		// "Today" is the Brazilian calendar day, like everywhere rendimento
		// reads the price series, whatever zone the host runs in: a quote
		// stamped with the host's day could land after the day it is read on.
		today := investments.ProviderDay(now)
		if !now.Before(nextDaily) {
			if _, err := RefreshAll(ctx, conn, service, today); err != nil {
				log.Printf("quote_refresh_failed: %v", err)
			}
			nextDaily = schedule.Next(now)
		} else if _, err := RefreshMissing(ctx, conn, service, today); err != nil && ctx.Err() == nil {
			// Woken by a save: price what has no price today yet.
			log.Printf("quote_refresh_failed: %v", err)
		}
		if _, err := RefreshHistory(ctx, conn, service, today); err != nil && ctx.Err() == nil {
			log.Printf("quote_history_failed: %v", err)
		}

		if !waitForWork(ctx, time.Until(nextDaily)) {
			return
		}
	}
}

// waitForWork sleeps until the next daily slot or a backfill request,
// whichever comes first, and reports false once ctx is cancelled.
func waitForWork(ctx context.Context, untilDaily time.Duration) bool {
	timer := time.NewTimer(untilDaily)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	case <-backfillRequests:
		// Let the rest of a burst of saves land, then take them all at once.
		sleep(ctx, backfillDebounce)
		select {
		case <-backfillRequests:
		default:
		}
		return ctx.Err() == nil
	}
}

// BrapiToken reads the decrypted brapi token from settings, treating
// "not configured" and "server key unavailable" the same way: an empty
// token, which the brapi provider sends unauthenticated (matching brapi's free
// tier) rather than treating as an error.
func BrapiToken(conn *sql.DB, secrets *settings.Secrets) func(context.Context) string {
	return func(ctx context.Context) string {
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
}

func sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
