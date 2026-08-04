package scenarios

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"contadinho-go/internal/db"
	"contadinho-go/internal/money"
)

// ErrScenarioNotFound is returned by GetScenario/DeleteScenario when a
// scenario id has no matching row.
var ErrScenarioNotFound = errors.New("scenario not found")

// ErrTransactionNotFound is returned by GetScenarioTransaction/
// UpdateScenarioTransaction/DeleteScenarioTransaction when a scenario
// transaction id has no matching row.
var ErrTransactionNotFound = errors.New("scenario transaction not found")

// Querier is satisfied by both *sql.DB and *sql.Tx — mirrors debts.Querier.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// dateLayout is the on-disk format for the date-only projected_at column —
// deliberately narrower than db.TimeLayout (no time-of-day, no timezone)
// since a planned installment's date is a calendar date, not an instant.
const dateLayout = "2006-01-02"

func formatDate(t time.Time) string         { return t.UTC().Format(dateLayout) }
func parseDate(s string) (time.Time, error) { return time.Parse(dateLayout, s) }

// CreateScenario mirrors creating a Scenario row. Callers are responsible
// for enforcing kind-specific rules (e.g. debt_id required for
// KindDebtPlan, receivable_id required for KindReceivablePlan) before
// calling this — the schema also enforces it via CHECK constraints as
// defense in depth.
func CreateScenario(ctx context.Context, q Querier, kind Kind, name string, debtID, receivableID *string) (Scenario, error) {
	now := time.Now().UTC()
	s := Scenario{ID: uuid.NewString(), Kind: kind, Name: name, DebtID: debtID, ReceivableID: receivableID, CreatedAt: now, UpdatedAt: now}
	_, err := q.ExecContext(ctx, `
		INSERT INTO scenarios (id, kind, name, debt_id, receivable_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		s.ID, string(s.Kind), s.Name, s.DebtID, s.ReceivableID, db.FormatTime(now), db.FormatTime(now),
	)
	if err != nil {
		return Scenario{}, err
	}
	return s, nil
}

func scanScenario(row *sql.Row) (Scenario, error) {
	var (
		s                          Scenario
		kind                       string
		debtID, receivableID       sql.NullString
		createdAtRaw, updatedAtRaw string
	)
	if err := row.Scan(&s.ID, &kind, &s.Name, &debtID, &receivableID, &createdAtRaw, &updatedAtRaw); err != nil {
		return Scenario{}, err
	}
	s.Kind = Kind(kind)
	if debtID.Valid {
		s.DebtID = &debtID.String
	}
	if receivableID.Valid {
		s.ReceivableID = &receivableID.String
	}
	var err error
	if s.CreatedAt, err = db.ParseTime(createdAtRaw); err != nil {
		return Scenario{}, err
	}
	if s.UpdatedAt, err = db.ParseTime(updatedAtRaw); err != nil {
		return Scenario{}, err
	}
	return s, nil
}

// GetScenario mirrors reading a single Scenario by id.
func GetScenario(ctx context.Context, q Querier, id string) (Scenario, error) {
	row := q.QueryRowContext(ctx, `SELECT id, kind, name, debt_id, receivable_id, created_at, updated_at FROM scenarios WHERE id = ?`, id)
	s, err := scanScenario(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Scenario{}, ErrScenarioNotFound
	}
	return s, err
}

// ListScenariosByDebt mirrors listing every scenario attached to a debt,
// newest first.
func ListScenariosByDebt(ctx context.Context, q Querier, debtID string) ([]Scenario, error) {
	return listScenariosByColumn(ctx, q, "debt_id", debtID)
}

// ListScenariosByReceivable mirrors listing every scenario attached to a
// receivable, newest first.
func ListScenariosByReceivable(ctx context.Context, q Querier, receivableID string) ([]Scenario, error) {
	return listScenariosByColumn(ctx, q, "receivable_id", receivableID)
}

func listScenariosByColumn(ctx context.Context, q Querier, column, value string) ([]Scenario, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, kind, name, debt_id, receivable_id, created_at, updated_at FROM scenarios WHERE `+column+` = ? ORDER BY created_at DESC`, value)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []Scenario
	for rows.Next() {
		var (
			s                          Scenario
			kind                       string
			debtID, receivableID       sql.NullString
			createdAtRaw, updatedAtRaw string
		)
		if err := rows.Scan(&s.ID, &kind, &s.Name, &debtID, &receivableID, &createdAtRaw, &updatedAtRaw); err != nil {
			return nil, err
		}
		s.Kind = Kind(kind)
		if debtID.Valid {
			s.DebtID = &debtID.String
		}
		if receivableID.Valid {
			s.ReceivableID = &receivableID.String
		}
		if s.CreatedAt, err = db.ParseTime(createdAtRaw); err != nil {
			return nil, err
		}
		if s.UpdatedAt, err = db.ParseTime(updatedAtRaw); err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

// DeleteScenario mirrors deleting a Scenario (scenario_transactions cascades
// via the schema's ON DELETE CASCADE).
func DeleteScenario(ctx context.Context, q Querier, id string) error {
	res, err := q.ExecContext(ctx, `DELETE FROM scenarios WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrScenarioNotFound
	}
	return nil
}

// CreateScenarioTransaction mirrors inserting a single planned installment.
func CreateScenarioTransaction(ctx context.Context, q Querier, scenarioID, description string, amount decimal.Decimal, projectedAt time.Time, category *string) (ScenarioTransaction, error) {
	now := time.Now().UTC()
	st := ScenarioTransaction{
		ID: uuid.NewString(), ScenarioID: scenarioID, Description: description,
		Amount: amount, ProjectedAt: projectedAt, Category: category,
		CreatedAt: now, UpdatedAt: now,
	}
	_, err := q.ExecContext(ctx, `
		INSERT INTO scenario_transactions (id, scenario_id, description, amount, projected_at, category, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		st.ID, st.ScenarioID, st.Description, money.CanonicalDecimal(st.Amount), formatDate(st.ProjectedAt), st.Category,
		db.FormatTime(now), db.FormatTime(now),
	)
	if err != nil {
		return ScenarioTransaction{}, err
	}
	return st, nil
}

func scanScenarioTransaction(row *sql.Row) (ScenarioTransaction, error) {
	var (
		st                         ScenarioTransaction
		amountRaw, projectedAtRaw  string
		category                   sql.NullString
		createdAtRaw, updatedAtRaw string
	)
	if err := row.Scan(&st.ID, &st.ScenarioID, &st.Description, &amountRaw, &projectedAtRaw, &category, &createdAtRaw, &updatedAtRaw); err != nil {
		return ScenarioTransaction{}, err
	}
	if category.Valid {
		st.Category = &category.String
	}
	var err error
	if st.Amount, err = decimal.NewFromString(amountRaw); err != nil {
		return ScenarioTransaction{}, err
	}
	if st.ProjectedAt, err = parseDate(projectedAtRaw); err != nil {
		return ScenarioTransaction{}, err
	}
	if st.CreatedAt, err = db.ParseTime(createdAtRaw); err != nil {
		return ScenarioTransaction{}, err
	}
	if st.UpdatedAt, err = db.ParseTime(updatedAtRaw); err != nil {
		return ScenarioTransaction{}, err
	}
	return st, nil
}

const scenarioTransactionColumns = `id, scenario_id, description, amount, projected_at, category, created_at, updated_at`

// GetScenarioTransaction mirrors reading a single planned installment by id.
func GetScenarioTransaction(ctx context.Context, q Querier, id string) (ScenarioTransaction, error) {
	row := q.QueryRowContext(ctx, `SELECT `+scenarioTransactionColumns+` FROM scenario_transactions WHERE id = ?`, id)
	st, err := scanScenarioTransaction(row)
	if errors.Is(err, sql.ErrNoRows) {
		return ScenarioTransaction{}, ErrTransactionNotFound
	}
	return st, err
}

// ListScenarioTransactions mirrors listing every planned installment of a
// scenario, ordered by projected_at (earliest first — the natural reading
// order for a payment plan).
func ListScenarioTransactions(ctx context.Context, q Querier, scenarioID string) ([]ScenarioTransaction, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT `+scenarioTransactionColumns+` FROM scenario_transactions WHERE scenario_id = ? ORDER BY projected_at, created_at`, scenarioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []ScenarioTransaction
	for rows.Next() {
		var (
			st                         ScenarioTransaction
			amountRaw, projectedAtRaw  string
			category                   sql.NullString
			createdAtRaw, updatedAtRaw string
		)
		if err := rows.Scan(&st.ID, &st.ScenarioID, &st.Description, &amountRaw, &projectedAtRaw, &category, &createdAtRaw, &updatedAtRaw); err != nil {
			return nil, err
		}
		if category.Valid {
			st.Category = &category.String
		}
		if st.Amount, err = decimal.NewFromString(amountRaw); err != nil {
			return nil, err
		}
		if st.ProjectedAt, err = parseDate(projectedAtRaw); err != nil {
			return nil, err
		}
		if st.CreatedAt, err = db.ParseTime(createdAtRaw); err != nil {
			return nil, err
		}
		if st.UpdatedAt, err = db.ParseTime(updatedAtRaw); err != nil {
			return nil, err
		}
		list = append(list, st)
	}
	return list, rows.Err()
}

// UpdateScenarioTransaction mirrors editing a planned installment's fields
// directly (used both by manual edits and, later, by the readjustment flow).
func UpdateScenarioTransaction(ctx context.Context, q Querier, id, description string, amount decimal.Decimal, projectedAt time.Time, category *string) (ScenarioTransaction, error) {
	now := db.FormatTime(time.Now())
	res, err := q.ExecContext(ctx, `
		UPDATE scenario_transactions SET description = ?, amount = ?, projected_at = ?, category = ?, updated_at = ?
		WHERE id = ?`,
		description, money.CanonicalDecimal(amount), formatDate(projectedAt), category, now, id)
	if err != nil {
		return ScenarioTransaction{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ScenarioTransaction{}, ErrTransactionNotFound
	}
	return GetScenarioTransaction(ctx, q, id)
}

// DeleteScenarioTransaction mirrors removing a single planned installment
// (scenario_transaction_realizations cascades via ON DELETE CASCADE).
func DeleteScenarioTransaction(ctx context.Context, q Querier, id string) error {
	res, err := q.ExecContext(ctx, `DELETE FROM scenario_transactions WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrTransactionNotFound
	}
	return nil
}
