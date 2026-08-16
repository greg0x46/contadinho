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
	if err := tx.Commit(); err != nil {
		return RecurringCommitment{}, err
	}
	return c, nil
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
	return Get(ctx, q, id)
}

// Delete removes a commitment. Fails with ErrLinkedToAutomationRule if an
// automation rule's reconcile action still targets it (ON DELETE RESTRICT
// on automation_rule_actions.recurring_commitment_id) — remove or repoint
// that rule's action first.
func Delete(ctx context.Context, q Querier, id string) error {
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
	defer rows.Close()
	var commitments []RecurringCommitment
	for rows.Next() {
		c, err := scanCommitment(rows.Scan)
		if err != nil {
			return nil, err
		}
		commitments = append(commitments, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return commitments, nil
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
