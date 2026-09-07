package syncsvc

import (
	"context"
	"database/sql"
	"log"
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

// FailRun marks syncRunID failed with a general, item-stage error, mirroring
// failGeneral but callable before a Service exists for the run — needed when
// a run can't even start, e.g. its connection was deactivated after
// ClaimNextRun claimed the run but before a worker got to it (ClaimNextRun
// only looks at unclaimed rows, so it can't see a deactivation that happens
// after the claim). A no-op if the run is no longer in_progress.
func FailRun(ctx context.Context, conn *sql.DB, syncRunID, errorCode string) error {
	log.Printf("sync_run_general_failure run_id=%s stage=item code=%s", syncRunID, errorCode)
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var status string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM sync_runs WHERE id = ?`, syncRunID).Scan(&status); err != nil {
		return err
	}
	if status != "in_progress" {
		return tx.Commit()
	}

	now := db.FormatTime(time.Now())
	message := SafeMessage(errorCode)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO sync_failures (id, sync_run_id, stage, error_code, safe_message, created_at)
		VALUES (?, ?, 'item', ?, ?, ?)`,
		uuid.NewString(), syncRunID, errorCode, message, now,
	); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE sync_runs SET status = 'failed', finished_at = ?, general_error_code = ?, general_error_message = ?
		WHERE id = ?`,
		now, errorCode, message, syncRunID,
	); err != nil {
		return err
	}
	return tx.Commit()
}
