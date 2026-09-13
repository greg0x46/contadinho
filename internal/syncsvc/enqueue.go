package syncsvc

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
)

// Enqueue inserts a new in_progress run for sourceID for the worker to pick
// up. busy reports that uq_sync_runs_active_source rejected it because the
// connection is already mid-sync — the expected outcome for a busy
// connection, not an error, so callers enqueueing several connections can
// skip it and carry on with the rest.
func Enqueue(ctx context.Context, conn *sql.DB, sourceID string, now time.Time) (runID string, busy bool, err error) {
	runID = uuid.NewString()
	_, err = conn.ExecContext(ctx,
		`INSERT INTO sync_runs (id, source_id, status, started_at) VALUES (?, ?, 'in_progress', ?)`,
		runID, sourceID, db.FormatTime(now))
	if db.IsUniqueViolationOn(err, db.ConstraintActiveSyncRunSQLite, db.ConstraintActiveSyncRunPostgres) {
		return "", true, nil
	}
	if err != nil {
		return "", false, err
	}
	return runID, false, nil
}
