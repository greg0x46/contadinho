package worker_test

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/datasources"
	"contadinho-go/internal/db"
	"contadinho-go/internal/pluggy"
	"contadinho-go/internal/settings"
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

// TestProcessClaimUsesTheRunsOwnConnection pins the rule that makes several
// connections safe: the item to fetch comes from the run's source_id, not from
// global config. Reading a single configured item id here would have synced
// one bank's data into another bank's run.
func TestProcessClaimUsesTheRunsOwnConnection(t *testing.T) {
	conn := newTestConn(t)
	insertSourceAndRun(t, conn, "item-first", "in_progress", time.Now().Add(-time.Hour))
	sourceID, runID := insertSourceAndRun(t, conn, "item-second", "in_progress", time.Now())

	var requested []string
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.RequestURI())
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/auth" {
			_, _ = w.Write([]byte(`{"apiKey":"test-key"}`))
			return
		}
		// Anything past auth can fail: this test is about which item was
		// asked for, and the run failing cleanly is a valid outcome.
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer provider.Close()

	ctx := context.Background()
	key, err := settings.Setup(ctx, conn, "correct horse battery staple")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	for name, value := range map[string]string{"pluggy.client_id": "cid", "pluggy.client_secret": "csecret"} {
		if err := settings.Set(ctx, conn, name, value, true, key); err != nil {
			t.Fatalf("Set %s: %v", name, err)
		}
	}
	session := settings.NewSession()
	session.Unlock(key)

	cfg := worker.Config{Pluggy: pluggy.DefaultConfig()}
	cfg.Pluggy.BaseURL = provider.URL
	cfg.Pluggy.MaxAttempts = 1
	_ = worker.ProcessClaim(ctx, conn, session, cfg, runID, sourceID)

	var asked bool
	for _, uri := range requested {
		if strings.Contains(uri, "item-first") {
			t.Fatalf("asked the provider for another connection's item: %v", requested)
		}
		if uri == "/items/item-second" {
			asked = true
		}
	}
	if !asked {
		t.Errorf("never asked for the run's own item: %v", requested)
	}
}

// TestProcessClaimRejectsAnUnknownConnection guards the other direction: a run
// whose connection vanished must fail loudly instead of syncing whatever item
// happened to be configured.
func TestProcessClaimRejectsAnUnknownConnection(t *testing.T) {
	conn := newTestConn(t)
	_, runID := insertSourceAndRun(t, conn, "item-1", "in_progress", time.Now())

	ctx := context.Background()
	key, err := settings.Setup(ctx, conn, "correct horse battery staple")
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	for name, value := range map[string]string{"pluggy.client_id": "cid", "pluggy.client_secret": "csecret"} {
		if err := settings.Set(ctx, conn, name, value, true, key); err != nil {
			t.Fatalf("Set %s: %v", name, err)
		}
	}
	session := settings.NewSession()
	session.Unlock(key)

	err = worker.ProcessClaim(ctx, conn, session, worker.Config{Pluggy: pluggy.DefaultConfig()},
		runID, "does-not-exist")
	if !errors.Is(err, datasources.ErrNotFound) {
		t.Errorf("err = %v, want datasources.ErrNotFound", err)
	}
}
