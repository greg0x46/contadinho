package recurrences

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

// ErrNotFound is returned by Get/Update/SetActive/Delete when id has no
// matching row.
var ErrNotFound = errors.New("recurring commitment not found")

// ErrLinkedToAutomationRule is returned by Delete when the commitment is
// still targeted by an automation rule's reconcile action (ON DELETE
// RESTRICT on automation_rule_actions.recurring_commitment_id).
var ErrLinkedToAutomationRule = errors.New("recurring commitment is targeted by an automation rule's reconcile action")

// Querier is satisfied by both *sql.DB and *sql.Tx.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// dateLayout is the on-disk format for the date-only start_date/end_date
// columns — mirrors internal/scenarios' dateLayout.
const dateLayout = "2006-01-02"

func formatDate(t time.Time) string         { return t.UTC().Format(dateLayout) }
func parseDate(s string) (time.Time, error) { return time.Parse(dateLayout, s) }

const columns = `id, name, kind, amount, category_id, account_id,
	cadence, day_of_month, month_of_year, start_date, end_date, is_active, created_at, updated_at`

// Write is the fields a create/update request supplies.
type Write struct {
	Name        string
	Kind        Kind
	Amount      decimal.Decimal
	CategoryID  string
	AccountID   *string
	Cadence     Cadence
	DayOfMonth  int
	MonthOfYear *int
	StartDate   time.Time
	EndDate     *time.Time
	IsActive    bool
}

// Create inserts a new recurring commitment in one transaction. Callers are
// responsible for calling RecurringCommitment.Validate() first (the HTTP
// layer does this before ever reaching the store).
func Create(ctx context.Context, conn *sql.DB, write Write) (RecurringCommitment, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return RecurringCommitment{}, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	c := RecurringCommitment{
		ID: uuid.NewString(), Name: write.Name, Kind: write.Kind, Amount: write.Amount,
		CategoryID: write.CategoryID, AccountID: write.AccountID, Cadence: write.Cadence,
		DayOfMonth: write.DayOfMonth, MonthOfYear: write.MonthOfYear, StartDate: write.StartDate,
		EndDate: write.EndDate, IsActive: write.IsActive, CreatedAt: now, UpdatedAt: now,
	}
	_, err = tx.ExecContext(ctx, `
		INSERT INTO recurring_commitments (`+columns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.ID, c.Name, string(c.Kind), money.CanonicalDecimal(c.Amount),
		c.CategoryID, c.AccountID, string(c.Cadence), c.DayOfMonth, c.MonthOfYear,
		formatDate(c.StartDate), formatDatePtr(c.EndDate), boolToInt(c.IsActive),
		db.FormatTime(now), db.FormatTime(now),
	)
	if err != nil {
		return RecurringCommitment{}, err
	}
	if err := createScenarioProjection(ctx, tx, c); err != nil {
		return RecurringCommitment{}, err
	}
	if err := tx.Commit(); err != nil {
		return RecurringCommitment{}, err
	}
	// Return the compatibility DTO enriched with its canonical Scenario ID;
	// old callers ignore the optional field, while new automation clients can
	// target the scenario immediately after creation.
	return Get(ctx, conn, c.ID)
}

// Update replaces every field of an existing commitment, including its
// condition list (cleared and reinserted, same as internal/automation's
// Update).
func Update(ctx context.Context, conn *sql.DB, id string, write Write) (RecurringCommitment, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return RecurringCommitment{}, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, `
		UPDATE recurring_commitments SET
			name = ?, kind = ?, amount = ?, category_id = ?, account_id = ?,
			cadence = ?, day_of_month = ?, month_of_year = ?, start_date = ?, end_date = ?, is_active = ?,
			updated_at = ?
		WHERE id = ?`,
		write.Name, string(write.Kind), money.CanonicalDecimal(write.Amount),
		write.CategoryID, write.AccountID, string(write.Cadence), write.DayOfMonth, write.MonthOfYear,
		formatDate(write.StartDate), formatDatePtr(write.EndDate), boolToInt(write.IsActive),
		db.FormatTime(now), id,
	)
	if err != nil {
		return RecurringCommitment{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return RecurringCommitment{}, ErrNotFound
	}
	if err := syncScenarioProjection(ctx, tx, RecurringCommitment{
		ID: id, Name: write.Name, Kind: write.Kind, Amount: write.Amount,
		CategoryID: write.CategoryID, AccountID: write.AccountID, Cadence: write.Cadence,
		DayOfMonth: write.DayOfMonth, MonthOfYear: write.MonthOfYear, StartDate: write.StartDate,
		EndDate: write.EndDate, IsActive: write.IsActive, UpdatedAt: now,
	}); err != nil {
		return RecurringCommitment{}, err
	}
	if err := tx.Commit(); err != nil {
		return RecurringCommitment{}, err
	}
	return Get(ctx, conn, id)
}

// SetActive toggles is_active without touching any other field.
func SetActive(ctx context.Context, q Querier, id string, isActive bool) (RecurringCommitment, error) {
	now := db.FormatTime(time.Now())
	res, err := q.ExecContext(ctx, `UPDATE recurring_commitments SET is_active = ?, updated_at = ? WHERE id = ?`,
		boolToInt(isActive), now, id)
	if err != nil {
		return RecurringCommitment{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return RecurringCommitment{}, ErrNotFound
	}
	if err := setScenarioProjectionActive(ctx, q, id, isActive); err != nil {
		return RecurringCommitment{}, err
	}
	return Get(ctx, q, id)
}

// Delete removes a commitment. Fails with ErrLinkedToAutomationRule if an
// automation rule's reconcile action still targets it (ON DELETE RESTRICT
// on automation_rule_actions.recurring_commitment_id) — remove or repoint
// that rule's action first.
func Delete(ctx context.Context, q Querier, id string) error {
	// New automation rules may retain only the canonical scenario_id. Check
	// both target columns so the legacy endpoint reports the same actionable
	// conflict regardless of which API created the rule.
	var linked int
	if err := q.QueryRowContext(ctx, `
		SELECT COUNT(1)
		FROM automation_rule_actions a
		LEFT JOIN recurring_commitment_scenario_map m
		  ON m.scenario_id = a.scenario_id
		WHERE a.action_type = 'reconcile'
		  AND (a.recurring_commitment_id = ? OR m.recurring_commitment_id = ?)`, id, id).Scan(&linked); err != nil {
		return err
	}
	if linked > 0 {
		return ErrLinkedToAutomationRule
	}
	res, err := q.ExecContext(ctx, `DELETE FROM recurring_commitments WHERE id = ?`, id)
	if err != nil {
		if db.IsForeignKeyViolation(err) {
			return ErrLinkedToAutomationRule
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanCommitment(scan func(dest ...any) error) (RecurringCommitment, error) {
	var (
		c                          RecurringCommitment
		kindRaw, cadenceRaw        string
		amountRaw                  string
		accountID                  sql.NullString
		monthOfYear                sql.NullInt64
		startDateRaw               string
		endDateRaw                 sql.NullString
		isActive                   int
		createdAtRaw, updatedAtRaw string
	)
	if err := scan(
		&c.ID, &c.Name, &kindRaw, &amountRaw, &c.CategoryID, &accountID,
		&cadenceRaw, &c.DayOfMonth, &monthOfYear, &startDateRaw, &endDateRaw, &isActive,
		&createdAtRaw, &updatedAtRaw,
	); err != nil {
		return RecurringCommitment{}, err
	}
	c.Kind = Kind(kindRaw)
	c.Cadence = Cadence(cadenceRaw)
	c.IsActive = isActive != 0
	if accountID.Valid {
		c.AccountID = &accountID.String
	}
	if monthOfYear.Valid {
		m := int(monthOfYear.Int64)
		c.MonthOfYear = &m
	}
	var err error
	if c.Amount, err = decimal.NewFromString(amountRaw); err != nil {
		return RecurringCommitment{}, err
	}
	if c.StartDate, err = parseDate(startDateRaw); err != nil {
		return RecurringCommitment{}, err
	}
	if endDateRaw.Valid {
		end, err := parseDate(endDateRaw.String)
		if err != nil {
			return RecurringCommitment{}, err
		}
		c.EndDate = &end
	}
	if c.CreatedAt, err = db.ParseTime(createdAtRaw); err != nil {
		return RecurringCommitment{}, err
	}
	if c.UpdatedAt, err = db.ParseTime(updatedAtRaw); err != nil {
		return RecurringCommitment{}, err
	}
	return c, nil
}

// Get fetches a single commitment by id.
func Get(ctx context.Context, q Querier, id string) (RecurringCommitment, error) {
	row := q.QueryRowContext(ctx, `SELECT `+columns+` FROM recurring_commitments WHERE id = ?`, id)
	c, err := scanCommitment(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return RecurringCommitment{}, ErrNotFound
	}
	if err != nil {
		return RecurringCommitment{}, err
	}
	if err := attachScenarioID(ctx, q, &c); err != nil {
		return RecurringCommitment{}, err
	}
	return c, nil
}

func listWhere(ctx context.Context, q Querier, where string) ([]RecurringCommitment, error) {
	query := `SELECT ` + columns + ` FROM recurring_commitments`
	if where != "" {
		query += " WHERE " + where
	}
	query += " ORDER BY name"
	rows, err := q.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	var commitments []RecurringCommitment
	for rows.Next() {
		c, err := scanCommitment(rows.Scan)
		if err != nil {
			return nil, err
		}
		commitments = append(commitments, c)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	for i := range commitments {
		if err := attachScenarioID(ctx, q, &commitments[i]); err != nil {
			return nil, err
		}
	}
	return commitments, nil
}

func attachScenarioID(ctx context.Context, q Querier, c *RecurringCommitment) error {
	var scenarioID string
	err := q.QueryRowContext(ctx,
		`SELECT scenario_id FROM recurring_commitment_scenario_map WHERE recurring_commitment_id = ?`, c.ID,
	).Scan(&scenarioID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	c.ScenarioID = &scenarioID
	return nil
}

// List returns every commitment, ordered by name.
func List(ctx context.Context, q Querier) ([]RecurringCommitment, error) {
	return listWhere(ctx, q, "")
}

// ListActive returns only commitments with is_active = 1 — the set
// internal/timeline projects occurrences from.
func ListActive(ctx context.Context, q Querier) ([]RecurringCommitment, error) {
	return listWhere(ctx, q, "is_active = 1")
}

func formatDatePtr(t *time.Time) any {
	if t == nil {
		return nil
	}
	return formatDate(*t)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func createScenarioProjection(ctx context.Context, q Querier, c RecurringCommitment) error {
	now := c.CreatedAt
	scenarioID := uuid.NewString()
	if _, err := q.ExecContext(ctx, `
		INSERT INTO scenarios (id, kind, name, payable_id, is_active, is_accounting_source, created_at, updated_at)
		VALUES (?, 'recurring', ?, NULL, ?, 0, ?, ?)`,
		scenarioID, c.Name, boolToInt(c.IsActive), db.FormatTime(now), db.FormatTime(c.UpdatedAt),
	); err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx, `
		INSERT INTO recurring_commitment_scenario_map (recurring_commitment_id, scenario_id)
		VALUES (?, ?)`, c.ID, scenarioID); err != nil {
		return err
	}
	return insertScenarioSchedule(ctx, q, scenarioID, c)
}

func syncScenarioProjection(ctx context.Context, q Querier, c RecurringCommitment) error {
	var scenarioID string
	err := q.QueryRowContext(ctx,
		`SELECT scenario_id FROM recurring_commitment_scenario_map WHERE recurring_commitment_id = ?`, c.ID,
	).Scan(&scenarioID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx,
		`UPDATE scenarios SET name = ?, is_active = ?, updated_at = ? WHERE id = ?`,
		c.Name, boolToInt(c.IsActive), db.FormatTime(c.UpdatedAt), scenarioID,
	); err != nil {
		return err
	}
	if _, err := q.ExecContext(ctx, `DELETE FROM scenario_recurring_schedules WHERE scenario_id = ?`, scenarioID); err != nil {
		return err
	}
	return insertScenarioSchedule(ctx, q, scenarioID, c)
}

func setScenarioProjectionActive(ctx context.Context, q Querier, commitmentID string, active bool) error {
	_, err := q.ExecContext(ctx, `
		UPDATE scenarios SET is_active = ?, updated_at = ?
		WHERE id = (SELECT scenario_id FROM recurring_commitment_scenario_map WHERE recurring_commitment_id = ?)`,
		boolToInt(active), db.FormatTime(time.Now().UTC()), commitmentID,
	)
	return err
}

func insertScenarioSchedule(ctx context.Context, q Querier, scenarioID string, c RecurringCommitment) error {
	categoryID := c.CategoryID
	_, err := q.ExecContext(ctx, `
		INSERT INTO scenario_recurring_schedules (
			scenario_id, cashflow_kind, amount, category_id, account_id, cadence,
			day_of_month, month_of_year, start_date, end_date
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		scenarioID, string(c.Kind), money.CanonicalDecimal(c.Amount), categoryID, c.AccountID,
		string(c.Cadence), c.DayOfMonth, c.MonthOfYear, formatDate(c.StartDate), formatDatePtr(c.EndDate),
	)
	return err
}
