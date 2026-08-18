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
