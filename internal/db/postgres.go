package db

import (
	"context"
	"database/sql/driver"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
)

// pgConnector wraps pgx's stdlib connector so every store package can keep
// writing the same SQLite-style `?` positional placeholders it already uses
// — rewriting `?` to Postgres' `$1, $2, ...` here is far less invasive than
// changing the ~20 files (and their tests) that build raw SQL strings
// directly against *sql.DB/*sql.Tx.
type pgConnector struct {
	inner driver.Connector
}

func newPGConnector(dsn string) (driver.Connector, error) {
	cfg, err := pgx.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	return pgConnector{inner: stdlib.GetConnector(*cfg)}, nil
}

func (c pgConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	// stdlib.GetConnector always hands back a *stdlib.Conn; the type switch
	// is defensive rather than expected to ever miss.
	stdlibConn, ok := conn.(*stdlib.Conn)
	if !ok {
		return conn, nil
	}
	return &pgConn{Conn: stdlibConn}, nil
}

func (c pgConnector) Driver() driver.Driver {
	return c.inner.Driver()
}

// pgConn shadows only the three methods that take a raw SQL string,
// rewriting placeholders before delegating to the embedded *stdlib.Conn.
// Every other driver capability (Ping, BeginTx, CheckNamedValue,
// ResetSession, ...) is inherited unchanged through embedding.
type pgConn struct {
	*stdlib.Conn
}

func rebindPositional(query string) string {
	return sqlx.Rebind(sqlx.DOLLAR, query)
}

func (c *pgConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	return c.Conn.PrepareContext(ctx, rebindPositional(query))
}

func (c *pgConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	return c.Conn.QueryContext(ctx, rebindPositional(query), args)
}

func (c *pgConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	return c.Conn.ExecContext(ctx, rebindPositional(query), args)
}
