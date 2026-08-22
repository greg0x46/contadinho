package recurrences

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/dates"
	"contadinho-go/internal/db"
	"contadinho-go/internal/transactions"
)

// ErrOverrideNotFound is returned by DeleteOverride when the occurrence
// carries no manual decision to undo.
var ErrOverrideNotFound = errors.New("recurrence reconciliation not found")

// ErrTransactionAlreadyReconciled is returned by PutOverride when the chosen
// transaction is already linked to another occurrence (UNIQUE (transaction_id)).
// One transaction settles at most one occurrence — otherwise the same money
// would suppress two projections.
var ErrTransactionAlreadyReconciled = errors.New("transaction is already reconciled to another occurrence")

const overrideColumns = `id, recurring_commitment_id, occurrence_date, state, transaction_id, created_at`

func scanOverride(scan func(dest ...any) error) (Override, error) {
	var (
		o                 Override
		occurrenceDateRaw string
		stateRaw          string
		transactionID     sql.NullString
		createdAtRaw      string
	)
	if err := scan(&o.ID, &o.RecurringCommitmentID, &occurrenceDateRaw, &stateRaw, &transactionID, &createdAtRaw); err != nil {
		return Override{}, err
	}
	o.State = OverrideState(stateRaw)
	if transactionID.Valid {
		o.TransactionID = &transactionID.String
	}
	var err error
	if o.OccurrenceDate, err = parseDate(occurrenceDateRaw); err != nil {
		return Override{}, err
	}
	if o.CreatedAt, err = db.ParseTime(createdAtRaw); err != nil {
		return Override{}, err
	}
	return o, nil
}

// ListOverrides loads every manual decision belonging to any of
// commitmentIDs whose occurrence falls in [from, to], keyed by commitment id.
//
// It takes the whole set rather than one id because every interesting read is
// per-period, not per-commitment: the Timeline resolves every active
// commitment in one pass. One query for the whole set keeps that O(1) in
// round trips instead of O(number of commitments) — the same reasoning behind
// scenarios.realizationsFor.
func ListOverrides(ctx context.Context, q Querier, commitmentIDs []string, from, to time.Time) (map[string][]Override, error) {
	result := make(map[string][]Override, len(commitmentIDs))
	if len(commitmentIDs) == 0 {
		return result, nil
	}
	in, args := db.InClause(commitmentIDs)
	args = append(args, formatDate(dates.Day(from)), formatDate(dates.Day(to)))
	rows, err := q.QueryContext(ctx, `
		SELECT `+overrideColumns+`
		FROM recurrence_reconciliations
		WHERE recurring_commitment_id IN (`+in+`)
			AND occurrence_date >= ? AND occurrence_date <= ?
		ORDER BY occurrence_date`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		o, err := scanOverride(rows.Scan)
		if err != nil {
			return nil, err
		}
		result[o.RecurringCommitmentID] = append(result[o.RecurringCommitmentID], o)
	}
	return result, rows.Err()
}

// GetOverride reads the manual decision on one occurrence, if any.
func GetOverride(ctx context.Context, q Querier, commitmentID string, occurrenceDate time.Time) (Override, bool, error) {
	row := q.QueryRowContext(ctx, `
		SELECT `+overrideColumns+` FROM recurrence_reconciliations
		WHERE recurring_commitment_id = ? AND occurrence_date = ?`,
		commitmentID, formatDate(dates.Day(occurrenceDate)))
	o, err := scanOverride(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return Override{}, false, nil
	}
	if err != nil {
		return Override{}, false, err
	}
	return o, true, nil
}

// OverrideForTransaction finds the occurrence a transaction is hand-linked
// to, if any — the reverse lookup the transaction detail view needs.
func OverrideForTransaction(ctx context.Context, q Querier, transactionID string) (Override, bool, error) {
	row := q.QueryRowContext(ctx, `
		SELECT `+overrideColumns+` FROM recurrence_reconciliations WHERE transaction_id = ?`, transactionID)
	o, err := scanOverride(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return Override{}, false, nil
	}
	if err != nil {
		return Override{}, false, err
	}
	return o, true, nil
}

// LinkedTransactionIDs returns every transaction currently hand-linked to
// some occurrence, so a candidate list can exclude them in one lookup rather
// than one query per candidate.
func LinkedTransactionIDs(ctx context.Context, q Querier) (map[string]bool, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT transaction_id FROM recurrence_reconciliations WHERE transaction_id IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	linked := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		linked[id] = true
	}
	return linked, rows.Err()
}

// PutOverride records the user's decision about one occurrence, replacing
// whatever decision was there before: (commitment, occurrence_date) is
// UNIQUE, and the three UI actions are all one write to that one row —
// linking a transaction, detaching the occurrence, or replacing one manual
// link with another. Delete + insert rather than an upsert because the two
// dialects spell ON CONFLICT differently and the pair runs in one
// transaction anyway.
//
// Callers are responsible for having validated that occurrenceDate really is
// an occurrence of this commitment, and that transactionID is eligible — the
// HTTP layer does both before reaching here.
func PutOverride(ctx context.Context, conn *sql.DB, commitmentID string, occurrenceDate time.Time, state OverrideState, transactionID *string) (Override, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return Override{}, err
	}
	defer tx.Rollback()

	day := dates.Day(occurrenceDate)
	var scenarioID sql.NullString
	if err := tx.QueryRowContext(ctx,
		`SELECT scenario_id FROM recurring_commitment_scenario_map WHERE recurring_commitment_id = ?`, commitmentID,
	).Scan(&scenarioID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Override{}, err
	}
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM recurrence_reconciliations WHERE recurring_commitment_id = ? AND occurrence_date = ?`,
		commitmentID, formatDate(day),
	); err != nil {
		return Override{}, err
	}
	if scenarioID.Valid {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM scenario_realizations
			WHERE scenario_id = ? AND relation_type = 'reconciliation' AND occurrence_date = ?`,
			scenarioID.String, formatDate(day)); err != nil {
			return Override{}, err
		}
	}

	o := Override{
		ID:                    uuid.NewString(),
		RecurringCommitmentID: commitmentID,
		OccurrenceDate:        day,
		State:                 state,
		TransactionID:         transactionID,
		CreatedAt:             time.Now().UTC(),
	}
	if state == StateDetached {
		o.TransactionID = nil
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO recurrence_reconciliations (
			id, recurring_commitment_id, scenario_id, occurrence_date, state, transaction_id, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		o.ID, o.RecurringCommitmentID, nullableScenarioID(scenarioID), formatDate(o.OccurrenceDate), string(o.State),
		o.TransactionID, db.FormatTime(o.CreatedAt),
	); err != nil {
		// UNIQUE (transaction_id): another occurrence already claims this
		// transaction. The HTTP layer checks for that first, so reaching here
		// means a concurrent writer won the race.
		return Override{}, ErrTransactionAlreadyReconciled
	}
	if scenarioID.Valid {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO scenario_realizations (
				id, scenario_id, occurrence_date, transaction_id, relation_type,
				state, origin, created_at
			) VALUES (?, ?, ?, ?, 'reconciliation', ?, 'manual', ?)`,
			o.ID, scenarioID.String, formatDate(o.OccurrenceDate), o.TransactionID,
			string(o.State), db.FormatTime(o.CreatedAt)); err != nil {
			return Override{}, ErrTransactionAlreadyReconciled
		}
	}
	if err := tx.Commit(); err != nil {
		return Override{}, err
	}
	return o, nil
}

// DeleteOverride drops the manual decision on one occurrence, handing it back
// to the automation rule ("voltar ao automático"). This is not the same as
// detaching: detaching is a decision that persists, this forgets that a
// decision was ever made.
func DeleteOverride(ctx context.Context, q Querier, commitmentID string, occurrenceDate time.Time) error {
	var scenarioID sql.NullString
	err := q.QueryRowContext(ctx,
		`SELECT scenario_id FROM recurring_commitment_scenario_map WHERE recurring_commitment_id = ?`, commitmentID,
	).Scan(&scenarioID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	res, err := q.ExecContext(ctx,
		`DELETE FROM recurrence_reconciliations WHERE recurring_commitment_id = ? AND occurrence_date = ?`,
		commitmentID, formatDate(dates.Day(occurrenceDate)))
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrOverrideNotFound
	}
	if scenarioID.Valid {
		_, err = q.ExecContext(ctx, `
			DELETE FROM scenario_realizations
			WHERE scenario_id = ? AND relation_type = 'reconciliation' AND occurrence_date = ?`,
			scenarioID.String, formatDate(dates.Day(occurrenceDate)))
		if err != nil {
			return err
		}
	}
	return nil
}

// UnlinkIfPresent drops any manual link a transaction holds. Its signature
// matches transactions.OnIgnoredHook exactly — same as
// payables.UnlinkIfPresent — so it can be wired directly into every
// inclusion-changing path: a transaction the user marked as ignored is
// excluded from the totals, so it can't be the thing carrying a commitment's
// money.
//
// Only 'linked' rows have a transaction_id, so a detached occurrence is
// untouched by this — the user's decision to detach survives an unrelated
// transaction being ignored.
func UnlinkIfPresent(ctx context.Context, q transactions.Querier, transactionID string) error {
	if _, err := q.ExecContext(ctx, `DELETE FROM recurrence_reconciliations WHERE transaction_id = ?`, transactionID); err != nil {
		return err
	}
	_, err := q.ExecContext(ctx, `
		DELETE FROM scenario_realizations
		WHERE relation_type = 'reconciliation' AND transaction_id = ?`, transactionID)
	return err
}

func nullableScenarioID(value sql.NullString) any {
	if !value.Valid {
		return nil
	}
	return value.String
}
