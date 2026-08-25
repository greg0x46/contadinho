package db

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
)

// downMigrationsUnderTest is how far back the round trip rolls: the three
// migrations that dropped the legacy allocation, reconciliation and
// recurring-commitment tables. They are the ones whose Down blocks have to
// rebuild a schema rather than just drop what they made, so they are the ones
// worth exercising.
const downMigrationsUnderTest = 3

// TestDownUpRoundTrip applies every migration, rolls the drop migrations back
// and re-applies them. Nothing else in the suite runs a Down block at all, so
// without this a broken rollback would only be discovered by someone actually
// needing to roll back.
//
// It asserts that the round trip *runs*, not that it is lossless — it is not,
// deliberately: a recurring scenario with no category has no representation in
// the old recurring_commitments shape, which each Down block documents.
func TestDownUpRoundTrip(t *testing.T) {
	conn, err := Open(filepath.Join(t.TempDir(), "roundtrip.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := Migrate(conn); err != nil {
		t.Fatalf("initial up: %v", err)
	}

	migrations, err := fs.Sub(sqliteMigrationsFS, "migrations/sqlite")
	if err != nil {
		t.Fatalf("sub fs: %v", err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, migrations)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	ctx := context.Background()
	for i := 0; i < downMigrationsUnderTest; i++ {
		if _, err := provider.Down(ctx); err != nil {
			t.Fatalf("down %d: %v", i+1, err)
		}
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("re-up after rollback: %v", err)
	}
}

// TestPostgresDownUpRoundTrip is the same round trip on the other dialect,
// where the drop migrations use ALTER TABLE instead of a table rebuild.
func TestPostgresDownUpRoundTrip(t *testing.T) {
	dsn := os.Getenv("CONTADINHO_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("CONTADINHO_TEST_POSTGRES_DSN not set; skipping Postgres integration test")
	}
	// Open already applies the Postgres migrations, so the tree is fully
	// migrated by the time the rollback below starts.
	conn, err := Open(dsn)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	migrations, err := fs.Sub(postgresMigrationsFS, "migrations/postgres")
	if err != nil {
		t.Fatalf("sub fs: %v", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, conn, migrations)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	ctx := context.Background()
	for i := 0; i < downMigrationsUnderTest; i++ {
		if _, err := provider.Down(ctx); err != nil {
			t.Fatalf("down %d: %v", i+1, err)
		}
	}
	if _, err := provider.Up(ctx); err != nil {
		t.Fatalf("re-up after rollback: %v", err)
	}
}
