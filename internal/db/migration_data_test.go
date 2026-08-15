package db

import (
	"context"
	"database/sql"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

// TestPayablesMigrationPreservesData seeds rows into the pre-00013 (SQLite)
// debts/receivables/scenarios schema, applies migration 00013_payables, and
// asserts every row survived into payables/payable_transaction_links with
// the same ids, amounts, and FKs repointed correctly.
func TestPayablesMigrationPreservesData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	// Mirrors openSQLite's single-connection pinning in db.go: PRAGMA
	// foreign_keys is per-connection, so a pool bigger than one risks the
	// migration's OFF/ON toggle landing on a different physical connection
	// than the statements it's meant to guard.
	conn.SetMaxOpenConns(1)
	if _, err := conn.Exec("PRAGMA foreign_keys = ON"); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	migrations, err := fs.Sub(sqliteMigrationsFS, "migrations/sqlite")
	if err != nil {
		t.Fatalf("root migrations fs: %v", err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, migrations)
	if err != nil {
		t.Fatalf("new provider: %v", err)
	}

	ctx := context.Background()
	if _, err := provider.UpTo(ctx, 12); err != nil {
		t.Fatalf("migrate up to 12: %v", err)
	}

	mustExec(t, conn, `INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at)
		VALUES ('src1', 'pluggy', 'item-1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO sync_runs (id, source_id, status, started_at, finished_at)
		VALUES ('run1', 'src1', 'completed', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO raw_imports (
			id, sync_run_id, source_id, scope, page_sequence, request_attempt,
			request_method, request_path, http_status, response_headers, payload,
			payload_sha256, received_at
		) VALUES ('ri1', 'run1', 'src1', 'transactions', 1, 1, 'GET', '/x', 200, '{}', x'00', 'sha', '2026-01-01T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO financial_accounts (
			id, source_id, external_id, currency_code, current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES ('acc1', 'src1', 'acc1', 'BRL', 'ri1', 'hash', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO financial_transactions (
			id, source_id, account_id, external_id, description, amount, amount_in_account_currency,
			currency_code, occurred_at, provider_status, movement_type, current_raw_import_id,
			normalized_hash, created_at, updated_at
		) VALUES ('tx1', 'src1', 'acc1', 'tx1', 'Pagamento dívida', '-100.00', '-100.00', 'BRL', '2026-01-05T00:00:00Z', 'POSTED', 'DEBIT', 'ri1', 'hash', '2026-01-05T00:00:00Z', '2026-01-05T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO financial_transactions (
			id, source_id, account_id, external_id, description, amount, amount_in_account_currency,
			currency_code, occurred_at, provider_status, movement_type, current_raw_import_id,
			normalized_hash, created_at, updated_at
		) VALUES ('tx2', 'src1', 'acc1', 'tx2', 'Recebimento', '200.00', '200.00', 'BRL', '2026-01-06T00:00:00Z', 'POSTED', 'CREDIT', 'ri1', 'hash', '2026-01-06T00:00:00Z', '2026-01-06T00:00:00Z')`)

	mustExec(t, conn, `INSERT INTO debts (id, name, total_amount, starting_paid_amount, created_at, updated_at)
		VALUES ('debt1', 'Empréstimo carro', '1000.00', '50.00', '2026-01-01T00:00:00Z', '2026-01-02T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO debt_transaction_links (id, debt_id, transaction_id, linked_amount, linked_at)
		VALUES ('dlink1', 'debt1', 'tx1', '100.00', '2026-01-05T00:00:00Z')`)

	mustExec(t, conn, `INSERT INTO receivables (id, name, total_amount, starting_received_amount, created_at, updated_at)
		VALUES ('recv1', 'Empréstimo para amigo', '2000.00', '100.00', '2026-02-01T00:00:00Z', '2026-02-02T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO receivable_transaction_links (id, receivable_id, transaction_id, linked_amount, linked_at)
		VALUES ('rlink1', 'recv1', 'tx2', '200.00', '2026-01-06T00:00:00Z')`)

	mustExec(t, conn, `INSERT INTO scenarios (id, kind, name, debt_id, created_at, updated_at)
		VALUES ('scen1', 'debt_plan', 'Plano dívida', 'debt1', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO scenarios (id, kind, name, receivable_id, created_at, updated_at)
		VALUES ('scen2', 'receivable_plan', 'Plano recebível', 'recv1', '2026-02-01T00:00:00Z', '2026-02-01T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO scenario_transactions (id, scenario_id, description, amount, projected_at, created_at, updated_at)
		VALUES ('stx1', 'scen1', 'Parcela 1', '100.00', '2026-01-05T00:00:00Z', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO scenario_transactions (id, scenario_id, description, amount, projected_at, created_at, updated_at)
		VALUES ('stx2', 'scen2', 'Recebimento 1', '200.00', '2026-01-06T00:00:00Z', '2026-02-01T00:00:00Z', '2026-02-01T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO scenario_transaction_realizations (id, scenario_transaction_id, debt_link_id, allocated_amount, created_at)
		VALUES ('real1', 'stx1', 'dlink1', '100.00', '2026-01-05T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO scenario_transaction_realizations (id, scenario_transaction_id, receivable_link_id, allocated_amount, created_at)
		VALUES ('real2', 'stx2', 'rlink1', '200.00', '2026-01-06T00:00:00Z')`)

	if _, err := provider.UpTo(ctx, 13); err != nil {
		t.Fatalf("migrate up to 13: %v", err)
	}

	assertRow(t, conn, "payables", "debt1", "kind", "debt")
	assertRow(t, conn, "payables", "debt1", "total_amount", "1000.00")
	assertRow(t, conn, "payables", "debt1", "starting_settled_amount", "50.00")
	assertRow(t, conn, "payables", "recv1", "kind", "receivable")
	assertRow(t, conn, "payables", "recv1", "starting_settled_amount", "100.00")

	assertRow(t, conn, "payable_transaction_links", "dlink1", "payable_id", "debt1")
	assertRow(t, conn, "payable_transaction_links", "dlink1", "linked_amount", "100.00")
	assertRow(t, conn, "payable_transaction_links", "rlink1", "payable_id", "recv1")

	assertRow(t, conn, "scenarios", "scen1", "payable_id", "debt1")
	assertRow(t, conn, "scenarios", "scen2", "payable_id", "recv1")

	assertRow(t, conn, "scenario_transaction_realizations", "real1", "payable_link_id", "dlink1")
	assertRow(t, conn, "scenario_transaction_realizations", "real2", "payable_link_id", "rlink1")

	for _, table := range []string{"debts", "receivables", "debt_transaction_links", "receivable_transaction_links"} {
		var name string
		err := conn.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != sql.ErrNoRows {
			t.Errorf("expected table %s to be dropped, got err=%v", table, err)
		}
	}
}

func assertRow(t *testing.T, conn *sql.DB, table, id, column, want string) {
	t.Helper()
	var got string
	query := "SELECT " + column + " FROM " + table + " WHERE id = ?"
	if err := conn.QueryRow(query, id).Scan(&got); err != nil {
		t.Fatalf("query %s.%s for id=%s: %v", table, column, id, err)
	}
	if got != want {
		t.Errorf("%s.%s for id=%s = %q, want %q", table, column, id, got, want)
	}
}

func mustExec(t *testing.T, conn *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(query, args...); err != nil {
		t.Fatalf("exec %q: %v", query, err)
	}
}
