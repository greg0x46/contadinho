package db_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"contadinho-go/internal/db"
)

func openTest(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestMigrateAppliesAllMigrations(t *testing.T) {
	conn := openTest(t)

	tables := []string{
		"data_sources", "sync_runs", "raw_imports", "financial_accounts",
		"financial_transactions", "normalization_events", "sync_failures",
		"automation_rules", "automation_rule_conditions",
		"transaction_inclusion_decisions", "transaction_inclusion_events",
		"categories", "transaction_category_decisions", "transaction_category_events",
		"debts", "debt_transaction_links", "settings",
	}
	for _, table := range tables {
		var name string
		err := conn.QueryRow(
			"SELECT name FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %s missing: %v", table, err)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	conn, err := db.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	conn.Close()

	conn2, err := db.Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	conn2.Close()
}

func TestCategoriesAreSeeded(t *testing.T) {
	conn := openTest(t)

	var count int
	if err := conn.QueryRow("SELECT COUNT(*) FROM categories").Scan(&count); err != nil {
		t.Fatalf("count categories: %v", err)
	}
	if count != 28 {
		t.Errorf("got %d seeded categories, want 28", count)
	}

	var kind string
	err := conn.QueryRow(
		"SELECT kind FROM categories WHERE id = ?", "533d9187-99b6-542b-a2f3-6eb9cbb299ce",
	).Scan(&kind)
	if err != nil {
		t.Fatalf("query transfer category: %v", err)
	}
	if kind != "transfer" {
		t.Errorf("got kind %q, want transfer", kind)
	}
}

func TestForeignKeysAreEnforced(t *testing.T) {
	conn := openTest(t)

	_, err := conn.Exec(
		`INSERT INTO sync_runs (id, source_id, started_at, finished_at)
		 VALUES ('r1', 'missing-source', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
	)
	if err == nil {
		t.Fatal("expected foreign key violation inserting sync_runs with missing source_id")
	}
}

func TestOnlyOneActiveSyncRunPerSource(t *testing.T) {
	conn := openTest(t)

	mustExec(t, conn, `INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at)
		VALUES ('s1', 'pluggy', 'item-1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO sync_runs (id, source_id, status, started_at, finished_at)
		VALUES ('r1', 's1', 'in_progress', '2026-01-01T00:00:00Z', NULL)`)

	_, err := conn.Exec(`INSERT INTO sync_runs (id, source_id, status, started_at, finished_at)
		VALUES ('r2', 's1', 'in_progress', '2026-01-01T00:00:01Z', NULL)`)
	if err == nil {
		t.Fatal("expected unique violation for a second in_progress sync run on the same source")
	}
}

func TestAppendOnlyTablesRejectUpdateAndDelete(t *testing.T) {
	conn := openTest(t)

	mustExec(t, conn, `INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at)
		VALUES ('s1', 'pluggy', 'item-1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO sync_runs (id, source_id, status, started_at, finished_at)
		VALUES ('r1', 's1', 'completed', '2026-01-01T00:00:00Z', '2026-01-01T00:00:01Z')`)
	mustExec(t, conn, `INSERT INTO raw_imports (
		id, sync_run_id, source_id, scope, page_sequence, request_attempt,
		request_method, request_path, http_status, response_headers, payload,
		payload_sha256, received_at
	) VALUES (
		'ri1', 'r1', 's1', 'item', 1, 1, 'GET', '/items/1', 200, '{}', x'00',
		'deadbeef', '2026-01-01T00:00:00Z'
	)`)

	if _, err := conn.Exec("UPDATE raw_imports SET http_status = 500 WHERE id = 'ri1'"); err == nil {
		t.Error("expected UPDATE on raw_imports to be rejected")
	}
	if _, err := conn.Exec("DELETE FROM raw_imports WHERE id = 'ri1'"); err == nil {
		t.Error("expected DELETE on raw_imports to be rejected")
	}
}

func mustExec(t *testing.T, conn *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
