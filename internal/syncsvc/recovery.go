package syncsvc

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
)

// RecoverStaleRuns mirrors recover_stale_runs, simplified for this app's
// single-process model: the reference distinguishes "worker_unavailable" (no
// worker ever claimed the run within staleAfter of its start) from
// "interrupted" (a worker claimed it, then its heartbeat went stale) because
// multiple worker processes can race to claim from a shared Postgres
// database. Here there is exactly one worker goroutine in this binary, so
// any run still 'in_progress' when the process starts up can only mean a
// previous instance of this same process died mid-sync — there is no
// separate "never claimed" case to distinguish. Every recovered run is
// therefore marked "interrupted".
func RecoverStaleRuns(ctx context.Context, conn *sql.DB, now time.Time) ([]string, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `SELECT id FROM sync_runs WHERE status = 'in_progress' ORDER BY started_at, id`)
	if err != nil {
		return nil, err
	}
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	nowText := db.FormatTime(now)
	message := SafeMessage("interrupted")
	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO sync_failures (id, sync_run_id, stage, error_code, safe_message, created_at)
			VALUES (?, ?, 'interrupted', 'interrupted', ?, ?)`,
			uuid.NewString(), id, message, nowText,
		); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE sync_runs SET status = 'failed', finished_at = ?, general_error_code = 'interrupted', general_error_message = ?
			WHERE id = ?`,
			nowText, message, id,
		); err != nil {
			return nil, err
		}
	}
	return ids, tx.Commit()
}
