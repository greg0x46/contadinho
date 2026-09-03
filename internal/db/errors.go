package db

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"modernc.org/sqlite"
)

// IsForeignKeyViolation reports whether err is a foreign-key constraint
// violation raised by either supported dialect's driver, so store packages
// can turn an ON DELETE RESTRICT/insert-against-a-missing-row failure into a
// typed sentinel error instead of leaking a raw driver error.
func IsForeignKeyViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		// modernc.org/sqlite reports RESTRICT-triggered FK failures under
		// varying extended result codes (observed: SQLITE_CONSTRAINT_TRIGGER,
		// not SQLITE_CONSTRAINT_FOREIGNKEY) — the message text is the
		// reliable signal SQLite itself gives for this constraint class.
		return strings.Contains(sqliteErr.Error(), "FOREIGN KEY constraint failed")
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23503"
	}
	return false
}

// SQLite extended result codes for the two spellings of "this row already
// exists". Unlike the FK case above, modernc.org/sqlite reports these
// reliably, so the class check does not have to read English error text.
const (
	sqliteConstraintPrimaryKey = 1555
	sqliteConstraintUnique     = 2067
)

// The partial unique index that lets a real transaction satisfy at most one
// complete planned event. Two spellings because the drivers name it
// differently — SQLite reports the indexed column, Postgres the index — and
// both are the same schema fact, created by the migrations this package
// owns. Pass both to IsUniqueViolationOn.
const (
	ConstraintOneCompleteEventTransactionSQLite   = "scenario_realizations.transaction_id"
	ConstraintOneCompleteEventTransactionPostgres = "uq_scenario_realizations_one_complete_event_transaction"
)

// The partial unique index that allows one in-progress run per connection.
// Hitting it is the expected answer to "sync this bank" while that bank is
// already syncing, so the caller has to tell it apart from any other insert
// failure on the same table.
const (
	ConstraintActiveSyncRunSQLite   = "sync_runs.source_id"
	ConstraintActiveSyncRunPostgres = "uq_sync_runs_active_source"
)

// data_sources' UNIQUE (provider, external_item_id): the same Pluggy item may
// only be registered once, since a second row would give its accounts a
// second identity rather than a second connection. Postgres names the
// implicit constraint after the table and columns; SQLite lists the columns.
const (
	ConstraintDataSourceItemSQLite   = "data_sources.external_item_id"
	ConstraintDataSourceItemPostgres = "data_sources_provider_external_item_id_key"
)

// IsUniqueViolation reports whether err is a unique-constraint violation, so
// a store can tell "someone else already claims this" apart from every other
// insert failure — including the foreign-key violation an unknown parent id
// raises, which would otherwise be reported as a conflict the caller cannot
// resolve.
func IsUniqueViolation(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		code := sqliteErr.Code()
		return code == sqliteConstraintUnique || code == sqliteConstraintPrimaryKey
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

// IsUniqueViolationOn narrows IsUniqueViolation to a specific constraint, for
// a store whose table carries several unique constraints and must map only
// one of them onto a typed conflict.
//
// Which constraint was hit is the one part neither driver exposes portably:
// Postgres names the index (pgErr.ConstraintName), SQLite names the columns
// ("table.column") and only inside the message. So the class check stays
// typed and only the identification falls back to text — pass every spelling
// the constraint answers to.
//
// The text fallback is reached only when nothing better is available. When
// Postgres did name the constraint, that name is the answer: a miss there is
// a definitive "some other constraint", and scanning the message afterwards
// could only turn it into a false positive — reporting an unrelated conflict
// as one the user could resolve by picking a different row.
func IsUniqueViolationOn(err error, names ...string) bool {
	if !IsUniqueViolation(err) {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName != "" {
		for _, name := range names {
			if strings.EqualFold(pgErr.ConstraintName, name) {
				return true
			}
		}
		return false
	}
	message := strings.ToLower(err.Error())
	for _, name := range names {
		if strings.Contains(message, strings.ToLower(name)) {
			return true
		}
	}
	return false
}
