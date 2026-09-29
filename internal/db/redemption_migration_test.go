package db

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

func TestRedemptionMigrationPreservesOperationsAndLinks(t *testing.T) {
	conn, err := Open(filepath.Join(t.TempDir(), "redemption.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	migrations, err := fs.Sub(sqliteMigrationsFS, "migrations/sqlite")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, migrations)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := provider.DownTo(ctx, 37); err != nil {
		t.Fatal(err)
	}
	seedProviderInvestments(t, conn, []byte("{}"))
	mustExec(t, conn, `INSERT INTO investment_accounts (id,name,kind,currency_code,created_at,updated_at)
        VALUES ('custody','Custody','manual','BRL','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO investment_operations (id,account_id,kind,occurred_on,amount,notes,created_at,updated_at)
        VALUES ('op','custody','deposit','2026-01-01','1400','original','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`)
	mustExec(t, conn, `INSERT INTO investment_reconciliations (id,operation_id,financial_investment_transaction_id,amount,created_at)
        VALUES ('link','op','mov-cdb','1400','2026-01-01T00:00:00Z')`)
	check := func() {
		t.Helper()
		assertRow(t, conn, "investment_operations", "op", "amount", "1400")
		assertRow(t, conn, "investment_operations", "op", "notes", "original")
		assertRow(t, conn, "investment_reconciliations", "link", "operation_id", "op")
		assertRow(t, conn, "investment_reconciliations", "link", "financial_investment_transaction_id", "mov-cdb")
		rows, err := conn.Query("PRAGMA foreign_key_check")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		if rows.Next() {
			t.Fatal("broken foreign key after rebuild")
		}
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	check()
	assertRow(t, conn, "investment_operations", "op", "principal_amount", "0")
	assertRow(t, conn, "investment_operations", "op", "income_amount", "0")
	if _, err := provider.DownTo(ctx, 37); err != nil {
		t.Fatal(err)
	}
	check()
	if _, err := provider.Up(ctx); err != nil {
		t.Fatal(err)
	}
	check()
	mustExec(t, conn, `INSERT INTO investment_operations (id,account_id,kind,occurred_on,amount,principal_amount,income_amount,created_at,updated_at)
        VALUES ('redemption','custody','redemption','2026-01-02','1100','1000','100','2026-01-02T00:00:00Z','2026-01-02T00:00:00Z')`)
	if _, err := provider.DownTo(ctx, 37); err == nil {
		t.Fatal("rollback discarded detailed redemption")
	}
	check()
	assertRow(t, conn, "investment_operations", "redemption", "principal_amount", "1000")
}
