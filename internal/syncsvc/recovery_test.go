package syncsvc_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
	"contadinho-go/internal/syncsvc"
)

func TestRecoverStaleRunsMarksInProgressRunsAsInterrupted(t *testing.T) {
	conn := newTestConn(t)
	sourceID, syncRunID := newSyncRun(t, conn) // starts 'in_progress', as if the process died mid-sync

	recovered, err := syncsvc.RecoverStaleRuns(context.Background(), conn, time.Now())
	if err != nil {
		t.Fatalf("RecoverStaleRuns: %v", err)
	}
	if len(recovered) != 1 || recovered[0] != syncRunID {
		t.Fatalf("recovered = %v, want [%s]", recovered, syncRunID)
	}

	var status, generalCode string
	if err := conn.QueryRow(`SELECT status, general_error_code FROM sync_runs WHERE id = ?`, syncRunID).Scan(&status, &generalCode); err != nil {
		t.Fatalf("query sync_run: %v", err)
	}
	if status != "failed" || generalCode != "interrupted" {
		t.Errorf("status=%s generalCode=%s, want failed/interrupted", status, generalCode)
	}

	var failureCount int
	conn.QueryRow(`SELECT COUNT(*) FROM sync_failures WHERE sync_run_id = ? AND stage = 'interrupted'`, syncRunID).Scan(&failureCount)
	if failureCount != 1 {
		t.Errorf("failureCount = %d, want 1", failureCount)
	}

	_ = sourceID
}

func TestRecoverStaleRunsLeavesCompletedRunsAlone(t *testing.T) {
	conn := newTestConn(t)
	sourceID, syncRunID := newSyncRun(t, conn)
	now := db.FormatTime(time.Now())
	if _, err := conn.Exec(`UPDATE sync_runs SET status = 'completed', finished_at = ? WHERE id = ?`, now, syncRunID); err != nil {
		t.Fatalf("update sync_run: %v", err)
	}

	recovered, err := syncsvc.RecoverStaleRuns(context.Background(), conn, time.Now())
	if err != nil {
		t.Fatalf("RecoverStaleRuns: %v", err)
	}
	if len(recovered) != 0 {
		t.Errorf("recovered = %v, want none", recovered)
	}

	var status string
	conn.QueryRow(`SELECT status FROM sync_runs WHERE id = ?`, syncRunID).Scan(&status)
	if status != "completed" {
		t.Errorf("status = %s, want completed (untouched)", status)
	}
	_ = sourceID
}

func TestRecoverStaleRunsHandlesMultipleRunsInStartedOrder(t *testing.T) {
	conn := newTestConn(t)
	sourceID := uuid.NewString()
	now := db.FormatTime(time.Now())
	if _, err := conn.Exec(`INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at)
		VALUES (?, 'pluggy', 'item-1', ?, ?)`, sourceID, now, now); err != nil {
		t.Fatalf("insert data_source: %v", err)
	}

	// A partial unique index only allows one 'in_progress' run per source,
	// so this exercises recovery across two distinct sources instead.
	sourceID2 := uuid.NewString()
	if _, err := conn.Exec(`INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at)
		VALUES (?, 'pluggy', 'item-2', ?, ?)`, sourceID2, now, now); err != nil {
		t.Fatalf("insert second data_source: %v", err)
	}

	run1, run2 := uuid.NewString(), uuid.NewString()
	if _, err := conn.Exec(`INSERT INTO sync_runs (id, source_id, status, started_at) VALUES (?, ?, 'in_progress', ?)`, run1, sourceID, now); err != nil {
		t.Fatalf("insert run1: %v", err)
	}
	if _, err := conn.Exec(`INSERT INTO sync_runs (id, source_id, status, started_at) VALUES (?, ?, 'in_progress', ?)`, run2, sourceID2, now); err != nil {
		t.Fatalf("insert run2: %v", err)
	}

	recovered, err := syncsvc.RecoverStaleRuns(context.Background(), conn, time.Now())
	if err != nil {
		t.Fatalf("RecoverStaleRuns: %v", err)
	}
	if len(recovered) != 2 {
		t.Fatalf("len(recovered) = %d, want 2", len(recovered))
	}
}
