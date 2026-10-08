package db

import (
	"strings"
	"testing"
)

// TestPostgresAssetQuoteOriginMigrationClassifiesExistingPricesBySource
// mirrors TestAssetQuoteOriginMigrationClassifiesExistingPricesBySource for
// the Postgres dialect (where the migration is 00043): it seeds prices of
// every source into the pre-origin schema, applies the migration, and checks
// the classification, the CHECK, the DEFAULT, the rollback and a second up.
func TestPostgresAssetQuoteOriginMigrationClassifiesExistingPricesBySource(t *testing.T) {
	conn, provider := postgresMigrationProviderAt(t, 42)
	ctx := t.Context()

	mustExec(t, conn, `INSERT INTO investment_assets
		(id, canonical_key, name, asset_type, currency_code, created_at, updated_at)
		VALUES ('asset-1', 'name:acao:petr4', 'Petrobras PN', 'Ação', 'BRL', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	cases := []struct{ source, origin string }{
		{"pluggy", "sync"}, {"issue", "issue"},
		{"yahoo", "market"}, {"brapi", "market"}, {"coingecko", "market"}, {"manual", "market"},
	}
	for i, tc := range cases {
		mustExec(t, conn, `INSERT INTO investment_asset_quotes
			(asset_id, quoted_on, price, source, created_at, updated_at)
			VALUES ('asset-1', ?, '10', ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`,
			"2026-09-0"+string(rune('1'+i)), tc.source)
	}

	if _, err := provider.UpTo(ctx, 43); err != nil {
		t.Fatalf("up to 43: %v", err)
	}
	assertOrigins := func(stage string) {
		t.Helper()
		for _, tc := range cases {
			var origin string
			if err := conn.QueryRow(`SELECT origin FROM investment_asset_quotes WHERE source = ?`, tc.source).Scan(&origin); err != nil {
				t.Fatalf("%s: %s: %v", stage, tc.source, err)
			}
			if origin != tc.origin {
				t.Errorf("%s: a %s price became %q, want %q", stage, tc.source, origin, tc.origin)
			}
		}
	}
	assertOrigins("after up")

	// The column is NOT NULL with DEFAULT 'market' as far as the catalog goes.
	var nullable, columnDefault string
	if err := conn.QueryRow(`SELECT is_nullable, column_default FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'investment_asset_quotes' AND column_name = 'origin'`).Scan(&nullable, &columnDefault); err != nil {
		t.Fatalf("describe origin: %v", err)
	}
	if nullable != "NO" || !strings.Contains(columnDefault, "'market'") {
		t.Errorf("origin nullable=%q default=%q, want NO and 'market'", nullable, columnDefault)
	}

	// A writer that does not say what its price is gets the least privileged
	// kind, the one a market quote may replace.
	mustExec(t, conn, `INSERT INTO investment_asset_quotes
		(asset_id, quoted_on, price, source, created_at, updated_at)
		VALUES ('asset-1', '2026-09-21', '10', 'unknown-writer', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	var defaulted string
	if err := conn.QueryRow(`SELECT origin FROM investment_asset_quotes WHERE source = 'unknown-writer'`).Scan(&defaulted); err != nil || defaulted != "market" {
		t.Errorf("a price written without an origin became %q (%v), want market", defaulted, err)
	}
	mustExec(t, conn, `DELETE FROM investment_asset_quotes WHERE source = 'unknown-writer'`)

	// New rows must say what they are, and only one of the three kinds.
	_, err := conn.Exec(`INSERT INTO investment_asset_quotes
		(asset_id, quoted_on, price, source, origin, created_at, updated_at)
		VALUES ('asset-1', '2026-09-20', '10', 'pluggy', 'guess', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`)
	if err == nil {
		t.Error("an unknown origin was accepted")
	} else if !strings.Contains(err.Error(), "23514") && !strings.Contains(err.Error(), "check constraint") {
		t.Errorf("an unknown origin failed, but not on the CHECK: %v", err)
	}
	// NULL is refused too (NOT NULL), not silently defaulted.
	if _, err := conn.Exec(`INSERT INTO investment_asset_quotes
		(asset_id, quoted_on, price, source, origin, created_at, updated_at)
		VALUES ('asset-1', '2026-09-19', '10', 'pluggy', NULL, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err == nil {
		t.Error("a NULL origin was accepted")
	}

	// Rolling back drops only the column; the prices stay and classify again.
	if _, err := provider.Down(ctx); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	var count int
	if err := conn.QueryRow("SELECT COUNT(*) FROM investment_asset_quotes").Scan(&count); err != nil || count != len(cases) {
		t.Fatalf("rollback lost prices: %d, %v", count, err)
	}
	if _, err := conn.Exec("SELECT origin FROM investment_asset_quotes"); err == nil {
		t.Error("origin survived the rollback")
	}
	if version, err := provider.GetDBVersion(ctx); err != nil || version != 42 {
		t.Errorf("version after rollback = %d (%v), want 42", version, err)
	}
	if _, err := provider.UpTo(ctx, 43); err != nil {
		t.Fatalf("re-up: %v", err)
	}
	assertOrigins("after re-up")
}
