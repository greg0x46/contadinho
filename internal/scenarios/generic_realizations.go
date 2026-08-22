package scenarios

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"contadinho-go/internal/db"
	"contadinho-go/internal/money"
	"contadinho-go/internal/recurrences"
	"contadinho-go/internal/transactions"
)

type RealizationRelation string

const (
	RelationSettlement       RealizationRelation = "settlement"
	RelationAllocation       RealizationRelation = "allocation"
	RelationReconciliation   RealizationRelation = "reconciliation"
	RealizationStateLinked   string              = "linked"
	RealizationStateDetached string              = "detached"
)

var (
	ErrInvalidEventKey            = errors.New("invalid planned event key")
	ErrInvalidRealization         = errors.New("invalid scenario realization")
	ErrPlannedRealizationNotFound = errors.New("planned event realization not found")
	ErrTransactionUnavailable     = errors.New("real transaction not found")
	ErrTransactionAlreadyRealized = errors.New("real transaction already satisfies another complete event")
)

// GenericRealization is the read model of scenario_realizations. It is
// intentionally independent of the old payable-link/allocation DTOs.
type GenericRealization struct {
	ID                    string
	ScenarioID            string
	ScenarioTransactionID *string
	OccurrenceDate        *time.Time
	TransactionID         *string
	RelationType          RealizationRelation
	State                 string
	Origin                string
	AllocatedAmount       *decimal.Decimal
	LinkedAmount          *decimal.Decimal
	CreatedAt             time.Time
}

type RealizationWrite struct {
	State           string
	TransactionID   *string
	AllocatedAmount decimal.Decimal
	Origin          string
}

// ListRealizations is the compatibility/read API for all generic relations
// owned by one scenario. It intentionally returns the normalized relation
// shape instead of exposing one of the legacy allocation/reconciliation
// tables.
func ListRealizations(ctx context.Context, q Querier, scenarioID string) ([]GenericRealization, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, scenario_id, scenario_transaction_id, occurrence_date, transaction_id,
		       relation_type, state, origin, allocated_amount, linked_amount, created_at
		FROM scenario_realizations
		WHERE scenario_id = ?
		ORDER BY created_at, id`, scenarioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []GenericRealization
	for rows.Next() {
		var (
			value                                  GenericRealization
			transactionID, scenarioTransactionID   sql.NullString
			occurrenceRaw, allocatedRaw, linkedRaw sql.NullString
			createdAtRaw                           string
			relationRaw                            string
		)
		if err := rows.Scan(
			&value.ID, &value.ScenarioID, &scenarioTransactionID, &occurrenceRaw, &transactionID,
			&relationRaw, &value.State, &value.Origin, &allocatedRaw, &linkedRaw, &createdAtRaw,
		); err != nil {
			return nil, err
		}
		value.RelationType = RealizationRelation(relationRaw)
		if scenarioTransactionID.Valid {
			value.ScenarioTransactionID = &scenarioTransactionID.String
		}
		if occurrenceRaw.Valid {
			parsed, err := time.Parse(dateLayout, occurrenceRaw.String)
			if err != nil {
				return nil, err
			}
			value.OccurrenceDate = &parsed
		}
		if transactionID.Valid {
			value.TransactionID = &transactionID.String
		}
		var parseErr error
		if allocatedRaw.Valid {
			parsed, err := decimal.NewFromString(allocatedRaw.String)
			if err != nil {
				return nil, err
			}
			value.AllocatedAmount = &parsed
		}
		if linkedRaw.Valid {
			parsed, err := decimal.NewFromString(linkedRaw.String)
			if err != nil {
				return nil, err
			}
			value.LinkedAmount = &parsed
		}
		value.CreatedAt, parseErr = db.ParseTime(createdAtRaw)
		if parseErr != nil {
			return nil, parseErr
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

// RealizeEvent creates or replaces the realization for one stable event key.
// It is the service used by both the new generic endpoint and compatibility
// handlers. A linked event always references a real financial transaction;
// a detached event explicitly carries no transaction and blocks automatic
// recurrence matching.
func RealizeEvent(ctx context.Context, conn *sql.DB, scenarioID, eventKey string, write RealizationWrite) (GenericRealization, error) {
	scenario, err := GetScenario(ctx, conn, scenarioID)
	if err != nil {
		return GenericRealization{}, err
	}
	if write.Origin == "" {
		write.Origin = "manual"
	}
	if write.State != RealizationStateLinked && write.State != RealizationStateDetached {
		return GenericRealization{}, fmt.Errorf("%w: state", ErrInvalidRealization)
	}
	if write.State == RealizationStateDetached {
		write.TransactionID = nil
		write.AllocatedAmount = decimal.Zero
	}
	if write.State == RealizationStateLinked && (write.TransactionID == nil || strings.TrimSpace(*write.TransactionID) == "") {
		return GenericRealization{}, fmt.Errorf("%w: transaction_id is required", ErrInvalidRealization)
	}
	if write.State == RealizationStateLinked {
		var exists int
		if err := conn.QueryRowContext(ctx, `SELECT COUNT(1) FROM financial_transactions WHERE id = ?`, *write.TransactionID).Scan(&exists); err != nil {
			return GenericRealization{}, err
		}
		if exists == 0 {
			return GenericRealization{}, ErrTransactionUnavailable
		}
	}

	relation, transactionID, occurrenceDate, scenarioTransactionID, err := eventIdentity(ctx, conn, scenario, eventKey)
	if err != nil {
		return GenericRealization{}, err
	}
	if relation == RelationAllocation && write.State == RealizationStateDetached {
		return GenericRealization{}, fmt.Errorf("%w: allocations must be linked or deleted", ErrInvalidRealization)
	}
	if relation == RelationAllocation && scenario.PayableID != nil {
		var linked int
		if err := conn.QueryRowContext(ctx, `
			SELECT COUNT(1) FROM payable_transaction_links
			WHERE payable_id = ? AND transaction_id = ?`, *scenario.PayableID, *write.TransactionID).Scan(&linked); err != nil {
			return GenericRealization{}, err
		}
		if linked == 0 {
			return GenericRealization{}, fmt.Errorf("%w: transaction is not linked to the scenario payable", ErrInvalidRealization)
		}
	}
	if relation == RelationAllocation {
		if write.State == RealizationStateLinked && !write.AllocatedAmount.IsPositive() {
			// A generic event link may omit an allocation amount (the common
			// case for standalone scenarios). In that case the event's own
			// magnitude is the amount being realized; payable-plan clients can
			// still provide a positive partial allocation explicitly.
			var plannedAmountRaw string
			if err := conn.QueryRowContext(ctx,
				`SELECT amount FROM scenario_transactions WHERE id = ?`, *scenarioTransactionID,
			).Scan(&plannedAmountRaw); err != nil {
				return GenericRealization{}, err
			}
			plannedAmount, err := decimal.NewFromString(plannedAmountRaw)
			if err != nil {
				return GenericRealization{}, err
			}
			write.AllocatedAmount = plannedAmount.Abs()
		}
		if !write.AllocatedAmount.IsPositive() && write.State == RealizationStateLinked {
			return GenericRealization{}, fmt.Errorf("%w: allocated_amount must be positive", ErrInvalidRealization)
		}
	}
	if relation == RelationReconciliation && write.State == RealizationStateLinked {
		if err := validateRecurringTransaction(ctx, conn, scenario.ID, *write.TransactionID); err != nil {
			return GenericRealization{}, err
		}
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return GenericRealization{}, err
	}
	defer tx.Rollback()

	if relation == RelationAllocation {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM scenario_realizations
			WHERE scenario_id = ? AND relation_type = 'allocation' AND scenario_transaction_id = ?`,
			scenarioID, *scenarioTransactionID); err != nil {
			return GenericRealization{}, err
		}
		// Keep the old installment detail endpoint coherent while it is
		// still supported. A new generic write replaces the old allocation
		// rows for this event; the generic table remains authoritative.
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM scenario_transaction_realizations WHERE scenario_transaction_id = ?`, *scenarioTransactionID); err != nil {
			return GenericRealization{}, err
		}
	} else {
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM scenario_realizations
			WHERE scenario_id = ? AND relation_type = 'reconciliation' AND occurrence_date = ?`,
			scenarioID, formatDate(*occurrenceDate)); err != nil {
			return GenericRealization{}, err
		}
		if _, err := tx.ExecContext(ctx, `
			DELETE FROM recurrence_reconciliations
			WHERE scenario_id = ? AND occurrence_date = ?`,
			scenarioID, formatDate(*occurrenceDate)); err != nil {
			return GenericRealization{}, err
		}
	}

	now := time.Now().UTC()
	row := GenericRealization{
		ID: uuid.NewString(), ScenarioID: scenarioID, ScenarioTransactionID: scenarioTransactionID,
		OccurrenceDate: occurrenceDate, TransactionID: transactionID, RelationType: relation,
		State: write.State, Origin: write.Origin, CreatedAt: now,
	}
	if write.State == RealizationStateLinked {
		row.TransactionID = write.TransactionID
		if relation == RelationAllocation {
			amount := write.AllocatedAmount
			row.AllocatedAmount = &amount
		} else {
			// The transaction below owns the only SQLite connection; reading
			// through conn here would wait forever with the application's
			// single-connection pool. Use the already-open transaction.
			row.LinkedAmount = linkedAmountSnapshot(ctx, tx, write.TransactionID)
		}
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO scenario_realizations (
			id, scenario_id, scenario_transaction_id, occurrence_date, transaction_id,
			relation_type, state, origin, allocated_amount, linked_amount, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.ID, row.ScenarioID, row.ScenarioTransactionID, nullableDate(row.OccurrenceDate), row.TransactionID,
		string(row.RelationType), row.State, row.Origin, decimalPtr(row.AllocatedAmount), decimalPtr(row.LinkedAmount), db.FormatTime(now),
	); err != nil {
		if isCompleteEventTransactionConflict(err) {
			return GenericRealization{}, ErrTransactionAlreadyRealized
		}
		return GenericRealization{}, err
	}
	if relation == RelationAllocation && write.State == RealizationStateLinked && scenario.PayableID != nil {
		var payableLinkID string
		linkErr := tx.QueryRowContext(ctx, `
			SELECT id FROM payable_transaction_links
			WHERE payable_id = ? AND transaction_id = ?`, *scenario.PayableID, *write.TransactionID).Scan(&payableLinkID)
		if linkErr == nil {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO scenario_transaction_realizations (
					id, scenario_transaction_id, payable_link_id, allocated_amount, created_at
				) VALUES (?, ?, ?, ?, ?)`, row.ID, *scenarioTransactionID, payableLinkID,
				money.CanonicalDecimal(*row.AllocatedAmount), db.FormatTime(now)); err != nil {
				return GenericRealization{}, err
			}
		} else if !errors.Is(linkErr, sql.ErrNoRows) {
			return GenericRealization{}, linkErr
		}
	}
	if relation == RelationReconciliation {
		var commitmentID string
		mapErr := tx.QueryRowContext(ctx, `
			SELECT recurring_commitment_id
			FROM recurring_commitment_scenario_map
			WHERE scenario_id = ?`, scenarioID).Scan(&commitmentID)
		if mapErr == nil {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO recurrence_reconciliations (
					id, recurring_commitment_id, scenario_id, occurrence_date, state, transaction_id, created_at
				) VALUES (?, ?, ?, ?, ?, ?, ?)`, row.ID, commitmentID, scenarioID,
				formatDate(*occurrenceDate), row.State, row.TransactionID, db.FormatTime(now)); err != nil {
				if isCompleteEventTransactionConflict(err) {
					return GenericRealization{}, ErrTransactionAlreadyRealized
				}
				return GenericRealization{}, err
			}
		} else if !errors.Is(mapErr, sql.ErrNoRows) {
			return GenericRealization{}, mapErr
		}
	}
	if err := tx.Commit(); err != nil {
		return GenericRealization{}, err
	}
	return row, nil
}

// validateRecurringTransaction keeps the canonical event endpoint aligned
// with the legacy occurrence handler: a recurring occurrence can only be
// satisfied by a considered BRL transaction flowing in the schedule's
// direction. The database constraint still handles the cross-occurrence
// uniqueness race; this check supplies the domain-level rejection for an
// ignored, wrong-direction, or non-BRL transaction before opening the write
// transaction.
func validateRecurringTransaction(ctx context.Context, q Querier, scenarioID, transactionID string) error {
	var kindRaw string
	err := q.QueryRowContext(ctx, `
		SELECT cashflow_kind
		FROM scenario_recurring_schedules
		WHERE scenario_id = ?`, scenarioID).Scan(&kindRaw)
	if errors.Is(err, sql.ErrNoRows) {
		var commitmentID string
		if lookupErr := q.QueryRowContext(ctx, `
			SELECT recurring_commitment_id
			FROM recurring_commitment_scenario_map
			WHERE scenario_id = ?`, scenarioID).Scan(&commitmentID); lookupErr != nil {
			return fmt.Errorf("%w: recurring schedule not found", ErrInvalidRealization)
		}
		commitment, lookupErr := recurrences.Get(ctx, q, commitmentID)
		if lookupErr != nil {
			return fmt.Errorf("%w: recurring schedule not found", ErrInvalidRealization)
		}
		kindRaw = string(commitment.Kind)
	} else if err != nil {
		return err
	}

	item, found, err := transactions.GetItem(ctx, q, transactionID)
	if err != nil {
		return err
	}
	if !found {
		return ErrTransactionUnavailable
	}
	if eligibility := recurrences.EligibilityForItem(recurrences.Kind(kindRaw), item, false); !eligibility.Eligible {
		return fmt.Errorf("%w: transaction is not eligible for this recurring event", ErrInvalidRealization)
	}
	return nil
}

func isCompleteEventTransactionConflict(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "scenario_realizations.transaction_id") ||
		strings.Contains(message, "uq_scenario_realizations_one_complete_event_transaction") ||
		strings.Contains(message, "recurrence_reconciliations.transaction_id") ||
		strings.Contains(message, "duplicate key") && strings.Contains(message, "transaction_id")
}

func eventIdentity(ctx context.Context, q Querier, scenario Scenario, eventKey string) (RealizationRelation, *string, *time.Time, *string, error) {
	prefix := "scenario:" + scenario.ID + ":"
	if !strings.HasPrefix(eventKey, prefix) {
		return "", nil, nil, nil, ErrInvalidEventKey
	}
	rest := strings.TrimPrefix(eventKey, prefix)
	switch {
	case strings.HasPrefix(rest, "transaction:"):
		id := strings.TrimPrefix(rest, "transaction:")
		if id == "" {
			return "", nil, nil, nil, ErrInvalidEventKey
		}
		if scenario.Kind != KindDebtPlan && scenario.Kind != KindReceivablePlan && scenario.Kind != KindStandalone {
			return "", nil, nil, nil, ErrInvalidEventKey
		}
		planned, err := GetScenarioTransaction(ctx, q, id)
		if err != nil {
			return "", nil, nil, nil, err
		}
		if planned.ScenarioID != scenario.ID {
			return "", nil, nil, nil, ErrInvalidEventKey
		}
		return RelationAllocation, nil, nil, &id, nil
	case strings.HasPrefix(rest, "occurrence:"):
		if scenario.Kind != KindRecurring {
			return "", nil, nil, nil, ErrInvalidEventKey
		}
		dateRaw := strings.TrimPrefix(rest, "occurrence:")
		date, err := time.Parse(dateLayout, dateRaw)
		if err != nil {
			return "", nil, nil, nil, ErrInvalidEventKey
		}
		if !isRecurringOccurrence(ctx, q, scenario.ID, date) {
			return "", nil, nil, nil, ErrInvalidEventKey
		}
		return RelationReconciliation, nil, &date, nil, nil
	default:
		return "", nil, nil, nil, ErrInvalidEventKey
	}
}

// isRecurringOccurrence keeps the generic event endpoint from persisting a
// realization for a date that the schedule can never emit. Occurrence dates
// are derived identities, so accepting an arbitrary date would create a row
// that neither the projector nor the compatibility reconciliation API could
// ever observe.
func isRecurringOccurrence(ctx context.Context, q Querier, scenarioID string, date time.Time) bool {
	var (
		cashflowKind, amountRaw, cadence, startRaw string
		categoryID, accountID                      sql.NullString
		dayOfMonth                                 int
		monthOfYear                                sql.NullInt64
		endRaw                                     sql.NullString
	)
	err := q.QueryRowContext(ctx, `
		SELECT cashflow_kind, amount, category_id, account_id, cadence,
		       day_of_month, month_of_year, start_date, end_date
		FROM scenario_recurring_schedules
		WHERE scenario_id = ?`, scenarioID).Scan(
		&cashflowKind, &amountRaw, &categoryID, &accountID, &cadence,
		&dayOfMonth, &monthOfYear, &startRaw, &endRaw,
	)
	if err != nil {
		return false
	}
	amount, err := decimal.NewFromString(amountRaw)
	if err != nil {
		return false
	}
	start, err := time.Parse(dateLayout, startRaw)
	if err != nil {
		return false
	}
	var end *time.Time
	if endRaw.Valid {
		parsed, err := time.Parse(dateLayout, endRaw.String)
		if err != nil {
			return false
		}
		end = &parsed
	}
	var month *int
	if monthOfYear.Valid {
		value := int(monthOfYear.Int64)
		month = &value
	}
	schedule := recurrences.RecurringSchedule{
		CashflowKind: recurrences.Kind(cashflowKind), Amount: amount,
		CategoryID: nullableString(categoryID), AccountID: nullableString(accountID),
		Cadence: recurrences.Cadence(cadence), DayOfMonth: dayOfMonth,
		MonthOfYear: month, StartDate: start, EndDate: end,
	}
	return len(recurrences.OccurrencesInRange(schedule, date, date)) == 1
}

func nullableString(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func linkedAmountSnapshot(ctx context.Context, q Querier, transactionID *string) *decimal.Decimal {
	if transactionID == nil {
		return nil
	}
	var amountRaw, amountAccountRaw, currencyRaw, accountCurrencyRaw sql.NullString
	err := q.QueryRowContext(ctx, `
		SELECT ft.amount, ft.amount_in_account_currency, ft.currency_code, fa.currency_code
		FROM financial_transactions ft JOIN financial_accounts fa ON fa.id = ft.account_id
		WHERE ft.id = ?`, *transactionID).Scan(&amountRaw, &amountAccountRaw, &currencyRaw, &accountCurrencyRaw)
	if err != nil {
		return nil
	}
	var amount *decimal.Decimal
	if amountAccountRaw.Valid && accountCurrencyRaw.Valid && accountCurrencyRaw.String == "BRL" {
		if parsed, err := decimal.NewFromString(amountAccountRaw.String); err == nil {
			amount = &parsed
		}
	} else if amountRaw.Valid && currencyRaw.Valid && currencyRaw.String == "BRL" {
		if parsed, err := decimal.NewFromString(amountRaw.String); err == nil {
			amount = &parsed
		}
	}
	if amount == nil {
		return nil
	}
	value := amount.Abs()
	return &value
}

func decimalPtr(value *decimal.Decimal) any {
	if value == nil {
		return nil
	}
	return money.CanonicalDecimal(*value)
}

func nullableDate(value *time.Time) any {
	if value == nil {
		return nil
	}
	return formatDate(*value)
}

// DeleteRealization removes the realization represented by eventKey without
// touching the scenario transaction, financial transaction, or any payable.
func DeletePlannedEventRealization(ctx context.Context, q Querier, scenarioID, eventKey string) error {
	scenario, err := GetScenario(ctx, q, scenarioID)
	if err != nil {
		return err
	}
	relation, _, occurrence, transaction, err := eventIdentity(ctx, q, scenario, eventKey)
	if err != nil {
		return err
	}
	var result sql.Result
	if relation == RelationAllocation {
		result, err = q.ExecContext(ctx, `
			DELETE FROM scenario_realizations
			WHERE scenario_id = ? AND relation_type = 'allocation' AND scenario_transaction_id = ?`, scenarioID, *transaction)
	} else {
		result, err = q.ExecContext(ctx, `
			DELETE FROM scenario_realizations
			WHERE scenario_id = ? AND relation_type = 'reconciliation' AND occurrence_date = ?`, scenarioID, formatDate(*occurrence))
	}
	if err != nil {
		return err
	}
	if relation == RelationAllocation {
		if _, err := q.ExecContext(ctx,
			`DELETE FROM scenario_transaction_realizations WHERE scenario_transaction_id = ?`, *transaction); err != nil {
			return err
		}
	}
	if relation == RelationReconciliation {
		if _, err := q.ExecContext(ctx, `
			DELETE FROM recurrence_reconciliations
			WHERE scenario_id = ? AND occurrence_date = ?`, scenarioID, formatDate(*occurrence)); err != nil {
			return err
		}
	}
	if n, _ := result.RowsAffected(); n == 0 {
		return ErrPlannedRealizationNotFound
	}
	return nil
}
