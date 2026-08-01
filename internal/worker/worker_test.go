package worker_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
	"contadinho-go/internal/worker"
)

func newTestConn(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func insertSourceAndRun(t *testing.T, conn *sql.DB, itemID, status string, startedAt time.Time) (sourceID, runID string) {
	t.Helper()
	sourceID, runID = uuid.NewString(), uuid.NewString()
	now := db.FormatTime(time.Now())
	if _, err := conn.Exec(`INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at)
		VALUES (?, 'pluggy', ?, ?, ?)`, sourceID, itemID, now, now); err != nil {
		t.Fatalf("insert data_source: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO sync_runs (id, source_id, status, started_at) VALUES (?, ?, ?, ?)`,
		runID, sourceID, status, db.FormatTime(startedAt)); err != nil {
		t.Fatalf("insert sync_run: %v", err)
	}
	return sourceID, runID
}

func TestClaimNextRunPicksOldestUnclaimed(t *testing.T) {
	conn := newTestConn(t)
	_, older := insertSourceAndRun(t, conn, "item-1", "in_progress", time.Now().Add(-time.Hour))
	_, _ = insertSourceAndRun(t, conn, "item-2", "in_progress", time.Now())

	runID, _, ok, err := worker.ClaimNextRun(context.Background(), conn, "worker-1")
	if err != nil {
		t.Fatalf("ClaimNextRun: %v", err)
	}
	if !ok || runID != older {
		t.Errorf("runID = %s, ok=%v, want %s/true", runID, ok, older)
	}

	var workerID sql.NullString
	conn.QueryRow(`SELECT worker_id FROM sync_runs WHERE id = ?`, runID).Scan(&workerID)
	if !workerID.Valid || workerID.String != "worker-1" {
		t.Errorf("worker_id = %v, want worker-1", workerID)
	}
}

func TestClaimNextRunSkipsAlreadyClaimedRuns(t *testing.T) {
	conn := newTestConn(t)
	_, runID := insertSourceAndRun(t, conn, "item-1", "in_progress", time.Now())
	if _, err := conn.Exec(`UPDATE sync_runs SET worker_id = 'other-worker' WHERE id = ?`, runID); err != nil {
		t.Fatalf("update: %v", err)
	}

	_, _, ok, err := worker.ClaimNextRun(context.Background(), conn, "worker-1")
	if err != nil {
		t.Fatalf("ClaimNextRun: %v", err)
	}
	if ok {
		t.Error("should not claim a run another worker already holds")
	}
}

func TestClaimNextRunNoneAvailable(t *testing.T) {
	conn := newTestConn(t)
	_, _, ok, err := worker.ClaimNextRun(context.Background(), conn, "worker-1")
	if err != nil {
		t.Fatalf("ClaimNextRun: %v", err)
	}
	if ok {
		t.Error("expected no run to claim on an empty database")
	}
}
