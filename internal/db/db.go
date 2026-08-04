// Package db opens the application's database and applies migrations
// embedded into the binary, so a single executable needs no external schema
// or migration files at runtime. SQLite is the zero-config default; a
// "postgres://" or "postgresql://" DSN opts into Postgres instead, for
// deployments that run several contadinho instances against one shared
// database — see README "Rodando com Postgres".
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"
	"strings"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/sqlite/*.sql
var sqliteMigrationsFS embed.FS

//go:embed migrations/postgres/*.sql
var postgresMigrationsFS embed.FS

// Open opens the application database at dsn (a plain filesystem path, or
// ":memory:", selects SQLite; a "postgres://"/"postgresql://" URL selects
// Postgres) and applies the migrations embedded for that dialect.
func Open(dsn string) (*sql.DB, error) {
	if isPostgresDSN(dsn) {
		return openPostgres(dsn)
	}
	return openSQLite(dsn)
}

func isPostgresDSN(dsn string) bool {
	return strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://")
}

// openSQLite opens the SQLite database at path with WAL journaling, foreign
// keys enforced, and a busy timeout so concurrent access from the HTTP
// server and the sync worker doesn't fail immediately under lock contention.
func openSQLite(path string) (*sql.DB, error) {
	dsn := path
	if path != ":memory:" {
		dsn = "file:" + path + "?" + url.Values{
			"_foreign_keys": {"1"},
			"_journal_mode": {"WAL"},
			"_busy_timeout": {"5000"},
			"_synchronous":  {"NORMAL"},
		}.Encode()
	}

	conn, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite has no real concurrent-writer story: serialize all access
	// through a single connection so WAL readers never race a writer that
	// goose or the app opened on a second pooled connection.
	conn.SetMaxOpenConns(1)

	if err := migrate(conn, goose.DialectSQLite3, sqliteMigrationsFS, "migrations/sqlite"); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// openPostgres opens conn through the qmark-rewriting connector in
// postgres.go, so callers keep writing the same `?` placeholders they use
// for SQLite. Unlike SQLite, Postgres has a real concurrent-writer story —
// exactly what lets several contadinho instances share one database — so
// the pool isn't pinned to a single connection.
func openPostgres(dsn string) (*sql.DB, error) {
	connector, err := newPGConnector(dsn)
	if err != nil {
		return nil, err
	}
	conn := sql.OpenDB(connector)
	conn.SetMaxOpenConns(10)

	if err := migrate(conn, goose.DialectPostgres, postgresMigrationsFS, "migrations/postgres"); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// Migrate applies all pending embedded SQLite migrations to conn.
func Migrate(conn *sql.DB) error {
	return migrate(conn, goose.DialectSQLite3, sqliteMigrationsFS, "migrations/sqlite")
}

func migrate(conn *sql.DB, dialect goose.Dialect, migrationsFS embed.FS, root string) error {
	migrations, err := fs.Sub(migrationsFS, root)
	if err != nil {
		return fmt.Errorf("root migrations fs: %w", err)
	}
	provider, err := goose.NewProvider(dialect, conn, migrations)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
