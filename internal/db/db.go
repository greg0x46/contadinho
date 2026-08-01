// Package db opens the application's SQLite database and applies migrations
// embedded into the binary, so a single executable needs no external schema
// or migration files at runtime.
package db

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"net/url"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Open opens the SQLite database at path (use ":memory:" for an ephemeral
// in-process database) with WAL journaling, foreign keys enforced, and a
// busy timeout so concurrent access from the HTTP server and the sync
// worker doesn't fail immediately under lock contention.
func Open(path string) (*sql.DB, error) {
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

	if err := Migrate(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}

// Migrate applies all pending embedded migrations to db.
func Migrate(conn *sql.DB) error {
	migrations, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("root migrations fs: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, migrations)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := provider.Up(context.Background()); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
