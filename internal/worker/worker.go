// Package worker ports worker.py's background sync loop: it recovers any
// sync run an earlier process left in_progress, then repeatedly claims and
// executes the oldest unclaimed run.
//
// Unlike the reference (designed for several worker processes sharing one
// Postgres database, hence claim_next_run's SKIP LOCKED and the
// heartbeat/worker_id staleness split in recovery), this is one worker
// goroutine inside the single binary that also serves HTTP — there is only
// ever one claimant, so the claim step is really just "pick the oldest
// unclaimed run" with no contention to resolve.
package worker

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/datasources"
	"contadinho-go/internal/db"
	"contadinho-go/internal/pluggy"
	"contadinho-go/internal/settings"
	"contadinho-go/internal/syncsvc"
)

// Config mirrors the reference's worker-relevant Settings fields.
type Config struct {
	PollInterval          time.Duration
	Pluggy                pluggy.Config
	OnTransactionUpserted syncsvc.TransactionUpsertedHook
}

// WorkerID identifies this process for sync_runs.worker_id, kept even
// though nothing else contends for a claim, so a run's row still records
// which process executed it.
func WorkerID() string {
	host, _ := os.Hostname()
	return fmt.Sprintf("%s:%d:%s", host, os.Getpid(), uuid.NewString()[:8])
}

// ClaimNextRun mirrors claim_next_run.
func ClaimNextRun(ctx context.Context, conn *sql.DB, workerID string) (syncRunID, sourceID string, ok bool, err error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return "", "", false, err
	}
	defer tx.Rollback()

	err = tx.QueryRowContext(ctx, `
		SELECT id, source_id FROM sync_runs
		WHERE status = 'in_progress' AND worker_id IS NULL
		ORDER BY started_at, id LIMIT 1`,
	).Scan(&syncRunID, &sourceID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}

	now := db.FormatTime(time.Now())
	if _, err := tx.ExecContext(ctx,
		`UPDATE sync_runs SET worker_id = ?, heartbeat_at = ? WHERE id = ?`,
		workerID, now, syncRunID,
	); err != nil {
		return "", "", false, err
	}
	if err := tx.Commit(); err != nil {
		return "", "", false, err
	}
	log.Printf("sync_run_claimed run_id=%s worker_id=%s", syncRunID, workerID)
	return syncRunID, sourceID, true, nil
}

// pluggyCredentials reads the decrypted Pluggy API credentials from settings;
// returns ok=false if the server key or Pluggy credentials are unavailable, in which
// case the caller should wait rather than fail the run.
//
// These are application-wide: every connection is an item under the same
// Pluggy application. Which item a run covers is not read here — it comes
// from the run's own source_id, see ProcessClaim.
func pluggyCredentials(ctx context.Context, conn *sql.DB, secrets *settings.Secrets) (clientID, clientSecret string, ok bool) {
	key, available := secrets.Key()
	if !available {
		return "", "", false
	}
	clientID, found, err := settings.Get(ctx, conn, "pluggy.client_id", key)
	if err != nil || !found {
		return "", "", false
	}
	clientSecret, found, err = settings.Get(ctx, conn, "pluggy.client_secret", key)
	if err != nil || !found {
		return "", "", false
	}
	return clientID, clientSecret, true
}

// ProcessClaim mirrors process_claim (minus the heartbeat-maintaining
// goroutine, which only matters for detecting a worker that's still alive
// but stuck — not needed here since a hung sync in this process would hang
// the whole binary, which is its own, more visible, failure mode).
func ProcessClaim(ctx context.Context, conn *sql.DB, secrets *settings.Secrets, cfg Config, syncRunID, sourceID string) error {
	clientID, clientSecret, ok := pluggyCredentials(ctx, conn, secrets)
	if !ok {
		return fmt.Errorf("pluggy credentials unavailable")
	}
	// The item comes from the connection this run was created for, not from
	// global config: with several connections registered, reading a single
	// configured item id here would sync one bank's data into another's run.
	source, err := datasources.Get(ctx, conn, sourceID)
	if err != nil {
		return fmt.Errorf("resolve data source %s: %w", sourceID, err)
	}
	// The connection can be deactivated after ClaimNextRun claims this run
	// but before we get here — ClaimNextRun only looks at unclaimed rows, so
	// it has no way to see that. Check here, before syncing an item the user
	// no longer wants synced.
	if !source.IsActive {
		return syncsvc.FailRun(ctx, conn, syncRunID, "connection_inactive")
	}
	pluggyConfig := cfg.Pluggy
	pluggyConfig.ClientID = clientID
	pluggyConfig.ClientSecret = clientSecret
	pluggyConfig.ItemID = source.ExternalItemID

	writer := &syncsvc.RawImportWriter{DB: conn, SyncRunID: syncRunID, SourceID: sourceID}
	adapter := pluggy.NewAdapter(pluggyConfig, writer, &http.Client{Timeout: pluggyConfig.ConnectTimeout + pluggyConfig.ReadTimeout})
	service := &syncsvc.Service{
		DB: conn, Provider: adapter, SyncRunID: syncRunID, SourceID: sourceID,
		OnTransactionUpserted: cfg.OnTransactionUpserted,
	}
	return service.Execute(ctx)
}

// Run mirrors run_worker: recovers stale runs once at startup, then loops
// claiming and executing runs until ctx is cancelled.
func Run(ctx context.Context, conn *sql.DB, secrets *settings.Secrets, cfg Config) {
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = time.Second
	}
	if recovered, err := syncsvc.RecoverStaleRuns(ctx, conn, time.Now()); err != nil {
		log.Printf("recover_stale_runs failed: %v", err)
	} else {
		for _, id := range recovered {
			log.Printf("sync_run_recovered run_id=%s", id)
		}
	}

	workerID := WorkerID()
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Check credentials before claiming: a run claimed here but blocked
		// before credentials exist would sit forever with worker_id set, since
		// ClaimNextRun only looks at unclaimed rows. Waiting to claim until
		// we can actually proceed keeps every claimed run either running or
		// finished.
		if _, _, ok := pluggyCredentials(ctx, conn, secrets); !ok {
			sleep(ctx, cfg.PollInterval)
			continue
		}

		syncRunID, sourceID, ok, err := ClaimNextRun(ctx, conn, workerID)
		if err != nil {
			log.Printf("claim_next_run failed: %v", err)
			sleep(ctx, cfg.PollInterval)
			continue
		}
		if !ok {
			sleep(ctx, cfg.PollInterval)
			continue
		}
		if err := ProcessClaim(ctx, conn, secrets, cfg, syncRunID, sourceID); err != nil {
			log.Printf("sync_run_processing_failed run_id=%s: %v", syncRunID, err)
		}
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
