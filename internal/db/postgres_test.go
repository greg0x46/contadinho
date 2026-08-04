package db_test

import (
	"database/sql"
	"os"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"contadinho-go/internal/db"
)

// openPostgresTest skips unless CONTADINHO_TEST_POSTGRES_DSN points at a
// reachable Postgres server (see README "Rodando com Postgres" for a local
// docker one-liner). It wipes the public schema before opening so every test
// function starts from a clean, freshly migrated database.
func openPostgresTest(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("CONTADINHO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("CONTADINHO_TEST_POSTGRES_DSN not set; skipping Postgres integration test")
	}

	raw, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open raw pgx connection: %v", err)
	}
	if _, err := raw.Exec("DROP SCHEMA public CASCADE; CREATE SCHEMA public;"); err != nil {
		raw.Close()
		t.Fatalf("reset public schema: %v", err)
	}
	raw.Close()

	conn, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func TestPostgresMigrateAppliesAllTables(t *testing.T) {
	conn := openPostgresTest(t)

	tables := []string{
		"data_sources", "sync_runs", "raw_imports", "financial_accounts",
		"financial_transactions", "normalization_events", "sync_failures",
		"automation_rules", "automation_rule_conditions",
		"transaction_inclusion_decisions", "transaction_inclusion_events",
		"categories", "transaction_category_decisions", "transaction_category_events",
		"debts", "debt_transaction_links", "settings", "app_auth",
		"receivables", "receivable_transaction_links", "scenarios",
		"scenario_transactions", "scenario_transaction_realizations",
		"financial_investments", "financial_investment_transactions",
	}
	for _, table := range tables {
		var name string
		err := conn.QueryRow(
			"SELECT tablename FROM pg_tables WHERE schemaname = 'public' AND tablename = ?", table,
		).Scan(&name)
		if err != nil {
			t.Errorf("table %s missing: %v", table, err)
		}
	}
}

func TestPostgresMigrateIsIdempotent(t *testing.T) {
	openPostgresTest(t) // first Open already applied every migration once

	dsn := os.Getenv("CONTADINHO_TEST_POSTGRES_DSN")
	conn2, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	conn2.Close()
}

func TestPostgresCategoriesAreSeeded(t *testing.T) {
	conn := openPostgresTest(t)

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

func TestPostgresForeignKeysAreEnforced(t *testing.T) {
	conn := openPostgresTest(t)

	_, err := conn.Exec(
		`INSERT INTO sync_runs (id, source_id, status, started_at, finished_at)
		 VALUES ('r1', 'missing-source', 'completed', '2026-01-01T00:00:00Z', '2026-01-01T00:00:01Z')`,
	)
	if err == nil {
		t.Fatal("expected foreign key violation inserting sync_runs with missing source_id")
	}
}

func TestPostgresOnlyOneActiveSyncRunPerSource(t *testing.T) {
	conn := openPostgresTest(t)

	mustExec(t, conn, `INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at)
		VALUES ('s1', 'pluggy', 'item-1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO sync_runs (id, source_id, status, started_at, finished_at)
		VALUES ('r1', 's1', 'in_progress', '2026-01-01T00:00:00Z', NULL)`)

	_, err := conn.Exec(`INSERT INTO sync_runs (id, source_id, status, started_at, finished_at)
		VALUES ('r2', 's1', 'in_progress', '2026-01-01T00:00:01Z', NULL)`)
	if err == nil {
		t.Fatal("expected unique violation for a second in_progress sync run on the same source (uq_sync_runs_active_source)")
	}
}

func TestPostgresAppendOnlyTablesRejectUpdateAndDelete(t *testing.T) {
	conn := openPostgresTest(t)

	mustExec(t, conn, `INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at)
		VALUES ('s1', 'pluggy', 'item-1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO sync_runs (id, source_id, status, started_at, finished_at)
		VALUES ('r1', 's1', 'completed', '2026-01-01T00:00:00Z', '2026-01-01T00:00:01Z')`)
	mustExec(t, conn, `INSERT INTO raw_imports (
		id, sync_run_id, source_id, scope, page_sequence, request_attempt,
		request_method, request_path, http_status, response_headers, payload,
		payload_sha256, received_at
	) VALUES (
		'ri1', 'r1', 's1', 'item', 1, 1, 'GET', '/items/1', 200, '{}', ?,
		'deadbeef', '2026-01-01T00:00:00Z'
	)`, []byte{0})

	if _, err := conn.Exec("UPDATE raw_imports SET http_status = 500 WHERE id = 'ri1'"); err == nil {
		t.Error("expected UPDATE on raw_imports to be rejected")
	}
	if _, err := conn.Exec("DELETE FROM raw_imports WHERE id = 'ri1'"); err == nil {
		t.Error("expected DELETE on raw_imports to be rejected")
	}
}

// TestPostgresOnConflictReturning exercises the ON CONFLICT (...) DO NOTHING
// ... RETURNING pattern syncsvc uses (e.g. internal/syncsvc/service.go) —
// this is the shape most sensitive to the qmark-to-$N rewrite in
// internal/db/postgres.go, since it mixes a RETURNING clause with a
// conflict target.
func TestPostgresOnConflictReturning(t *testing.T) {
	conn := openPostgresTest(t)

	insert := `INSERT INTO data_sources (id, provider, external_item_id, display_name, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (provider, external_item_id) DO NOTHING
		RETURNING id`

	var id string
	if err := conn.QueryRow(insert, "s1", "pluggy", "item-1", "Nubank", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z").Scan(&id); err != nil {
		t.Fatalf("first insert should return the new id: %v", err)
	}
	if id != "s1" {
		t.Errorf("got id %q, want s1", id)
	}

	err := conn.QueryRow(insert, "s2", "pluggy", "item-1", "Nubank (duplicate)", "2026-01-01T00:00:00Z", "2026-01-01T00:00:00Z").Scan(&id)
	if err != sql.ErrNoRows {
		t.Errorf("conflicting insert should return no rows, got err=%v", err)
	}
}

func TestPostgresPartialAndExpressionIndexesExist(t *testing.T) {
	conn := openPostgresTest(t)

	indexes := []string{"uq_sync_runs_active_source", "uq_raw_imports_identity"}
	for _, idx := range indexes {
		var name string
		err := conn.QueryRow(
			"SELECT indexname FROM pg_indexes WHERE schemaname = 'public' AND indexname = ?", idx,
		).Scan(&name)
		if err != nil {
			t.Errorf("index %s missing: %v", idx, err)
		}
	}
}
