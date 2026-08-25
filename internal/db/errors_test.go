package db

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// pgUnique builds the error pgx reports for a unique violation: the SQLSTATE
// and the constraint name arrive as fields, and the rendered message repeats
// the name — which is exactly the overlap that makes a text fallback risky
// once the fields are available.
func pgUnique(constraint string) error {
	return &pgconn.PgError{
		Code:           "23505",
		Message:        `duplicate key value violates unique constraint "` + constraint + `"`,
		ConstraintName: constraint,
	}
}

func TestIsUniqueViolationOnMatchesTheNamedConstraint(t *testing.T) {
	err := pgUnique(ConstraintOneCompleteEventTransactionPostgres)
	if !IsUniqueViolationOn(err, ConstraintOneCompleteEventTransactionSQLite, ConstraintOneCompleteEventTransactionPostgres) {
		t.Error("IsUniqueViolationOn = false, want true for the constraint the driver named")
	}
}

// A different unique constraint on the same table must not be reported as the
// one the caller asked about: PutOverride maps a match onto "pick another
// transaction", which is the wrong thing to tell a user whose write lost a
// race on (scenario_id, occurrence_date) instead.
func TestIsUniqueViolationOnRejectsAnotherConstraintOnTheSameTable(t *testing.T) {
	err := pgUnique("uq_scenario_realizations_reconciliation_event")
	if IsUniqueViolationOn(err, ConstraintOneCompleteEventTransactionSQLite, ConstraintOneCompleteEventTransactionPostgres) {
		t.Error("IsUniqueViolationOn = true, want false for a different constraint")
	}
}

// Once Postgres has named the constraint, that name is the whole answer. This
// pins that the text fallback cannot re-open the decision: the message here
// mentions the caller's SQLite spelling while the field names a different
// constraint, and the field has to win.
func TestIsUniqueViolationOnPrefersTheConstraintFieldOverTheMessage(t *testing.T) {
	err := &pgconn.PgError{
		Code:           "23505",
		Message:        `duplicate key value violates unique constraint on scenario_realizations.transaction_id somewhere`,
		ConstraintName: "uq_scenario_realizations_reconciliation_event",
	}
	if IsUniqueViolationOn(err, ConstraintOneCompleteEventTransactionSQLite) {
		t.Error("IsUniqueViolationOn = true, want false — the named constraint must outrank the message text")
	}
}

// SQLite is the case the text fallback exists for: the driver reports the
// indexed columns only inside the message.
func TestIsUniqueViolationOnFallsBackToTextForSQLite(t *testing.T) {
	err := errors.New("constraint failed: UNIQUE constraint failed: scenario_realizations.transaction_id (2067)")
	// A bare error carries no driver type, so the class check rejects it
	// before the name ever matters — this is what keeps an arbitrary error
	// mentioning the column from being read as a conflict.
	if IsUniqueViolationOn(err, ConstraintOneCompleteEventTransactionSQLite) {
		t.Error("IsUniqueViolationOn = true, want false for an error no driver classified as a unique violation")
	}
}

func TestIsUniqueViolationOnRejectsNonUniqueErrors(t *testing.T) {
	if IsUniqueViolationOn(errors.New("boom"), ConstraintOneCompleteEventTransactionSQLite) {
		t.Error("IsUniqueViolationOn = true, want false for an unrelated error")
	}
	if IsUniqueViolation(nil) {
		t.Error("IsUniqueViolation(nil) = true, want false")
	}
}
