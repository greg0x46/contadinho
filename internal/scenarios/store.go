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

// Querier is satisfied by both *sql.DB and *sql.Tx — mirrors payables.Querier.
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
// for enforcing kind-specific rules (payableID required, and its kind must
// agree with kind) before calling this — the schema also enforces
// payableID being non-null via a CHECK constraint as defense in depth.
func CreateScenario(ctx context.Context, q Querier, kind Kind, name string, payableID *string) (Scenario, error) {
	now := time.Now().UTC()
	s := Scenario{ID: uuid.NewString(), Kind: kind, Name: name, PayableID: payableID, CreatedAt: now, UpdatedAt: now}
	_, err := q.ExecContext(ctx, `
		INSERT INTO scenarios (id, kind, name, payable_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		s.ID, string(s.Kind), s.Name, s.PayableID, db.FormatTime(now), db.FormatTime(now),
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
		payableID                  sql.NullString
		createdAtRaw, updatedAtRaw string
	)
	if err := row.Scan(&s.ID, &kind, &s.Name, &payableID, &createdAtRaw, &updatedAtRaw); err != nil {
		return Scenario{}, err
	}
	s.Kind = Kind(kind)
	if payableID.Valid {
		s.PayableID = &payableID.String
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
	row := q.QueryRowContext(ctx, `SELECT id, kind, name, payable_id, created_at, updated_at FROM scenarios WHERE id = ?`, id)
	s, err := scanScenario(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Scenario{}, ErrScenarioNotFound
	}
	return s, err
}

// ListScenariosByPayable mirrors listing every scenario attached to a
// payable, newest first.
func ListScenariosByPayable(ctx context.Context, q Querier, payableID string) ([]Scenario, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, kind, name, payable_id, created_at, updated_at FROM scenarios WHERE payable_id = ? ORDER BY created_at DESC`, payableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanScenarioRows(rows)
}

// listPayablePlanScenarios mirrors listing every Scenario{Kind: debt_plan
// or receivable_plan} — i.e. every scenario backed by a payable, regardless
// of which one — fed to internal/timeline's M4 projection.
func listPayablePlanScenarios(ctx context.Context, q Querier) ([]Scenario, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, kind, name, payable_id, created_at, updated_at FROM scenarios WHERE kind IN (?, ?) ORDER BY created_at DESC`,
		string(KindDebtPlan), string(KindReceivablePlan))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanScenarioRows(rows)
}

// ListStandaloneScenarios mirrors listing every Scenario{Kind: standalone},
// newest first — the "what if" scenarios with no payable_id, fed to the
// M5 multi-select.
func ListStandaloneScenarios(ctx context.Context, q Querier) ([]Scenario, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, kind, name, payable_id, created_at, updated_at FROM scenarios WHERE kind = ? ORDER BY created_at DESC`, string(KindStandalone))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanScenarioRows(rows)
}

// ListScenariosByIDs mirrors fetching a batch of scenarios by id, in no
// particular order — used to load just the scenarios active in a
// simulation. An empty ids returns an empty slice without touching the
// database.
func ListScenariosByIDs(ctx context.Context, q Querier, ids []string) ([]Scenario, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	in, args := db.InClause(ids)
	query := `SELECT id, kind, name, payable_id, created_at, updated_at FROM scenarios WHERE id IN (` + in + `)`
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanScenarioRows(rows)
}

func scanScenarioRows(rows *sql.Rows) ([]Scenario, error) {
	var list []Scenario
	for rows.Next() {
		var (
			s                          Scenario
			kind                       string
			payableID                  sql.NullString
			createdAtRaw, updatedAtRaw string
		)
		if err := rows.Scan(&s.ID, &kind, &s.Name, &payableID, &createdAtRaw, &updatedAtRaw); err != nil {
			return nil, err
		}
		s.Kind = Kind(kind)
		if payableID.Valid {
			s.PayableID = &payableID.String
		}
		var err error
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
	byScenario, err := scenarioTransactionsFor(ctx, q, []string{scenarioID})
	if err != nil {
		return nil, err
	}
	return byScenario[scenarioID], nil
}

// scenarioTransactionsFor loads the installments of any number of scenarios
// in one query, keyed by scenario_id and ordered by projected_at within
// each key — the same batching shape realizationsFor uses, for the same
// reason: ListPlanInstallments walks every payable-backed plan at once, and
// a query per plan would make the timeline's cost grow with how many plans
// the user has open. A scenario with no installments is simply absent from
// the map.
func scenarioTransactionsFor(ctx context.Context, q Querier, scenarioIDs []string) (map[string][]ScenarioTransaction, error) {
	result := make(map[string][]ScenarioTransaction, len(scenarioIDs))
	if len(scenarioIDs) == 0 {
		return result, nil
	}
	in, args := db.InClause(scenarioIDs)
	rows, err := q.QueryContext(ctx,
		`SELECT `+scenarioTransactionColumns+` FROM scenario_transactions
		 WHERE scenario_id IN (`+in+`)
		 ORDER BY projected_at, created_at`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
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
		result[st.ScenarioID] = append(result[st.ScenarioID], st)
	}
	return result, rows.Err()
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
