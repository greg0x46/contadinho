package automation

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
)

// ErrNotFound is returned by Get/Update/SetActive/Delete when id has no
// matching row.
var ErrNotFound = errors.New("automation rule not found")

// Rule mirrors AutomationRule (with its conditions eager-loaded, as the
// reference always fetches them together).
type Rule struct {
	ID            string
	Name          string
	IsActive      bool
	LogicOperator LogicOperator
	Conditions    []Condition
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// Write mirrors RuleWrite: the fields a create/update request supplies.
type Write struct {
	Name          string
	IsActive      bool
	LogicOperator LogicOperator
	Conditions    []Condition
}

// Querier is satisfied by both *sql.DB and *sql.Tx.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func insertConditions(ctx context.Context, q Querier, ruleID string, conditions []Condition) error {
	for i, c := range conditions {
		if _, err := q.ExecContext(ctx, `
			INSERT INTO automation_rule_conditions (id, rule_id, field, operator, value, position)
			VALUES (?, ?, ?, ?, ?, ?)`,
			uuid.NewString(), ruleID, string(c.Field), string(c.Operator), c.Value, i,
		); err != nil {
			return err
		}
	}
	return nil
}

func loadConditions(ctx context.Context, q Querier, ruleID string) ([]Condition, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT field, operator, value FROM automation_rule_conditions WHERE rule_id = ? ORDER BY position`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var conditions []Condition
	for rows.Next() {
		var c Condition
		if err := rows.Scan(&c.Field, &c.Operator, &c.Value); err != nil {
			return nil, err
		}
		conditions = append(conditions, c)
	}
	return conditions, rows.Err()
}

// Create mirrors create_rule.
func Create(ctx context.Context, conn *sql.DB, write Write) (Rule, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return Rule{}, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	id := uuid.NewString()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO automation_rules (id, name, is_active, logic_operator, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		id, write.Name, boolToInt(write.IsActive), string(write.LogicOperator), db.FormatTime(now), db.FormatTime(now),
	)
	if err != nil {
		return Rule{}, err
	}
	if err := insertConditions(ctx, tx, id, write.Conditions); err != nil {
		return Rule{}, err
	}
	if err := tx.Commit(); err != nil {
		return Rule{}, err
	}
	return Rule{ID: id, Name: write.Name, IsActive: write.IsActive, LogicOperator: write.LogicOperator, Conditions: write.Conditions, CreatedAt: now, UpdatedAt: now}, nil
}

// Update mirrors update_rule: it replaces every condition rather than
// diffing them, same as the reference's clear-then-reassign.
func Update(ctx context.Context, conn *sql.DB, id string, write Write) (Rule, error) {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return Rule{}, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	res, err := tx.ExecContext(ctx, `
		UPDATE automation_rules SET name = ?, is_active = ?, logic_operator = ?, updated_at = ? WHERE id = ?`,
		write.Name, boolToInt(write.IsActive), string(write.LogicOperator), db.FormatTime(now), id,
	)
	if err != nil {
		return Rule{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Rule{}, ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM automation_rule_conditions WHERE rule_id = ?`, id); err != nil {
		return Rule{}, err
	}
	if err := insertConditions(ctx, tx, id, write.Conditions); err != nil {
		return Rule{}, err
	}

	var createdAtRaw string
	if err := tx.QueryRowContext(ctx, `SELECT created_at FROM automation_rules WHERE id = ?`, id).Scan(&createdAtRaw); err != nil {
		return Rule{}, err
	}
	createdAt, err := db.ParseTime(createdAtRaw)
	if err != nil {
		return Rule{}, err
	}
	if err := tx.Commit(); err != nil {
		return Rule{}, err
	}
	return Rule{ID: id, Name: write.Name, IsActive: write.IsActive, LogicOperator: write.LogicOperator, Conditions: write.Conditions, CreatedAt: createdAt, UpdatedAt: now}, nil
}

// SetActive mirrors set_rule_active.
func SetActive(ctx context.Context, conn *sql.DB, id string, isActive bool) (Rule, error) {
	now := db.FormatTime(time.Now())
	res, err := conn.ExecContext(ctx, `UPDATE automation_rules SET is_active = ?, updated_at = ? WHERE id = ?`,
		boolToInt(isActive), now, id)
	if err != nil {
		return Rule{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Rule{}, ErrNotFound
	}
	return Get(ctx, conn, id)
}

// Delete mirrors delete_rule (automation_rule_conditions cascades via the
// schema's ON DELETE CASCADE).
func Delete(ctx context.Context, conn *sql.DB, id string) error {
	res, err := conn.ExecContext(ctx, `DELETE FROM automation_rules WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanRule(row *sql.Row) (Rule, error) {
	var (
		r            Rule
		isActive     int
		createdAtRaw string
		updatedAtRaw string
	)
	if err := row.Scan(&r.ID, &r.Name, &isActive, &r.LogicOperator, &createdAtRaw, &updatedAtRaw); err != nil {
		return Rule{}, err
	}
	r.IsActive = isActive != 0
	var err error
	if r.CreatedAt, err = db.ParseTime(createdAtRaw); err != nil {
		return Rule{}, err
	}
	if r.UpdatedAt, err = db.ParseTime(updatedAtRaw); err != nil {
		return Rule{}, err
	}
	return r, nil
}

// Get mirrors get_rule.
func Get(ctx context.Context, q Querier, id string) (Rule, error) {
	row := q.QueryRowContext(ctx, `SELECT id, name, is_active, logic_operator, created_at, updated_at FROM automation_rules WHERE id = ?`, id)
	r, err := scanRule(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Rule{}, ErrNotFound
	}
	if err != nil {
		return Rule{}, err
	}
	if r.Conditions, err = loadConditions(ctx, q, id); err != nil {
		return Rule{}, err
	}
	return r, nil
}

func listWhere(ctx context.Context, q Querier, where string) ([]Rule, error) {
	query := `SELECT id, name, is_active, logic_operator, created_at, updated_at FROM automation_rules`
	if where != "" {
		query += " WHERE " + where
	}
	query += " ORDER BY created_at"
	rows, err := q.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []Rule
	for rows.Next() {
		var (
			r            Rule
			isActive     int
			createdAtRaw string
			updatedAtRaw string
		)
		if err := rows.Scan(&r.ID, &r.Name, &isActive, &r.LogicOperator, &createdAtRaw, &updatedAtRaw); err != nil {
			return nil, err
		}
		r.IsActive = isActive != 0
		if r.CreatedAt, err = db.ParseTime(createdAtRaw); err != nil {
			return nil, err
		}
		if r.UpdatedAt, err = db.ParseTime(updatedAtRaw); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range rules {
		conditions, err := loadConditions(ctx, q, rules[i].ID)
		if err != nil {
			return nil, err
		}
		rules[i].Conditions = conditions
	}
	return rules, nil
}

// List mirrors list_rules.
func List(ctx context.Context, q Querier) ([]Rule, error) {
	return listWhere(ctx, q, "")
}

// ListActive mirrors load_active_rules.
func ListActive(ctx context.Context, q Querier) ([]Rule, error) {
	return listWhere(ctx, q, "is_active = 1")
}

// ListConditionOptions mirrors list_condition_options: the distinct
// account labels and card numbers available across existing data, to
// populate the rule-builder UI's autocomplete.
func ListConditionOptions(ctx context.Context, q Querier) (accounts []string, cards []string, err error) {
	rows, err := q.QueryContext(ctx, `SELECT name, institution FROM financial_accounts ORDER BY name, institution`)
	if err != nil {
		return nil, nil, err
	}
	accountSet := make(map[string]bool)
	for rows.Next() {
		var name, institution sql.NullString
		if err := rows.Scan(&name, &institution); err != nil {
			rows.Close()
			return nil, nil, err
		}
		label := strings.TrimSpace(name.String)
		if label == "" {
			label = strings.TrimSpace(institution.String)
		}
		if label != "" {
			accountSet[label] = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()

	rows, err = q.QueryContext(ctx, `SELECT credit_card_metadata FROM financial_transactions WHERE credit_card_metadata IS NOT NULL`)
	if err != nil {
		return nil, nil, err
	}
	cardSet := make(map[string]bool)
	for rows.Next() {
		var metadata string
		if err := rows.Scan(&metadata); err != nil {
			rows.Close()
			return nil, nil, err
		}
		if card := extractCardNumber(metadata); card != "" {
			cardSet[card] = true
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, nil, err
	}
	rows.Close()

	accounts = setToSortedSlice(accountSet)
	cards = setToSortedSlice(cardSet)
	return accounts, cards, nil
}

func setToSortedSlice(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for v := range set {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
