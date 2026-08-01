package debts

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"contadinho-go/internal/db"
	"contadinho-go/internal/money"
	"contadinho-go/internal/transactions"
)

// ErrNotFound is returned by Get/Update/Delete when a debt id has no
// matching row.
var ErrNotFound = errors.New("debt not found")

// ErrLinkNotFound is returned by GetLink when a debt_transaction_links id
// has no matching row.
var ErrLinkNotFound = errors.New("debt link not found")

// Querier is satisfied by both *sql.DB and *sql.Tx, and structurally by
// transactions.Querier too (same three methods), so it can also be passed
// wherever that's expected — see UnlinkIfPresent.
type Querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Create mirrors create_debt.
func Create(ctx context.Context, q Querier, name string, totalAmount, initialRemainingAmount decimal.Decimal) (Debt, error) {
	now := time.Now().UTC()
	debt := Debt{
		ID: uuid.NewString(), Name: name, TotalAmount: totalAmount,
		StartingPaidAmount: totalAmount.Sub(initialRemainingAmount),
		CreatedAt:          now, UpdatedAt: now,
	}
	_, err := q.ExecContext(ctx, `
		INSERT INTO debts (id, name, total_amount, starting_paid_amount, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		debt.ID, debt.Name, money.CanonicalDecimal(debt.TotalAmount), money.CanonicalDecimal(debt.StartingPaidAmount),
		db.FormatTime(now), db.FormatTime(now),
	)
	if err != nil {
		return Debt{}, err
	}
	return debt, nil
}

// Update mirrors update_debt (name and total_amount only — starting_paid_amount
// is set once at creation and never edited directly afterward).
func Update(ctx context.Context, q Querier, id, name string, totalAmount decimal.Decimal) (Debt, error) {
	now := db.FormatTime(time.Now())
	res, err := q.ExecContext(ctx, `UPDATE debts SET name = ?, total_amount = ?, updated_at = ? WHERE id = ?`,
		name, money.CanonicalDecimal(totalAmount), now, id)
	if err != nil {
		return Debt{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Debt{}, ErrNotFound
	}
	return Get(ctx, q, id)
}

// Delete mirrors delete_debt (debt_transaction_links cascades via the
// schema's ON DELETE CASCADE).
func Delete(ctx context.Context, q Querier, id string) error {
	res, err := q.ExecContext(ctx, `DELETE FROM debts WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanDebt(row *sql.Row) (Debt, error) {
	var (
		d                                     Debt
		totalAmountRaw, startingPaidAmountRaw string
		createdAtRaw, updatedAtRaw            string
	)
	if err := row.Scan(&d.ID, &d.Name, &totalAmountRaw, &startingPaidAmountRaw, &createdAtRaw, &updatedAtRaw); err != nil {
		return Debt{}, err
	}
	var err error
	if d.TotalAmount, err = decimal.NewFromString(totalAmountRaw); err != nil {
		return Debt{}, err
	}
	if d.StartingPaidAmount, err = decimal.NewFromString(startingPaidAmountRaw); err != nil {
		return Debt{}, err
	}
	if d.CreatedAt, err = db.ParseTime(createdAtRaw); err != nil {
		return Debt{}, err
	}
	if d.UpdatedAt, err = db.ParseTime(updatedAtRaw); err != nil {
		return Debt{}, err
	}
	return d, nil
}

// Get mirrors get_debt (without eager-loaded links — call Links separately,
// as List and the HTTP handlers do, since not every caller needs them).
func Get(ctx context.Context, q Querier, id string) (Debt, error) {
	row := q.QueryRowContext(ctx, `SELECT id, name, total_amount, starting_paid_amount, created_at, updated_at FROM debts WHERE id = ?`, id)
	d, err := scanDebt(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Debt{}, ErrNotFound
	}
	return d, err
}

// List mirrors list_debts, ordered by created_at.
func List(ctx context.Context, q Querier) ([]Debt, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, name, total_amount, starting_paid_amount, created_at, updated_at FROM debts ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var debts []Debt
	for rows.Next() {
		var (
			d                                     Debt
			totalAmountRaw, startingPaidAmountRaw string
			createdAtRaw, updatedAtRaw            string
		)
		if err := rows.Scan(&d.ID, &d.Name, &totalAmountRaw, &startingPaidAmountRaw, &createdAtRaw, &updatedAtRaw); err != nil {
			return nil, err
		}
		if d.TotalAmount, err = decimal.NewFromString(totalAmountRaw); err != nil {
			return nil, err
		}
		if d.StartingPaidAmount, err = decimal.NewFromString(startingPaidAmountRaw); err != nil {
			return nil, err
		}
		if d.CreatedAt, err = db.ParseTime(createdAtRaw); err != nil {
			return nil, err
		}
		if d.UpdatedAt, err = db.ParseTime(updatedAtRaw); err != nil {
			return nil, err
		}
		debts = append(debts, d)
	}
	return debts, rows.Err()
}

// Links mirrors accessing debt.links: every link for debtID, newest first
// (matching the reference's order_by(DebtTransactionLink.linked_at.desc())).
func Links(ctx context.Context, q Querier, debtID string) ([]Link, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT id, debt_id, transaction_id, linked_amount, linked_at FROM debt_transaction_links
		 WHERE debt_id = ? ORDER BY linked_at DESC`, debtID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var links []Link
	for rows.Next() {
		var l Link
		var linkedAmountRaw, linkedAtRaw string
		if err := rows.Scan(&l.ID, &l.DebtID, &l.TransactionID, &linkedAmountRaw, &linkedAtRaw); err != nil {
			return nil, err
		}
		if l.LinkedAmount, err = decimal.NewFromString(linkedAmountRaw); err != nil {
			return nil, err
		}
		if l.LinkedAt, err = db.ParseTime(linkedAtRaw); err != nil {
			return nil, err
		}
		links = append(links, l)
	}
	return links, rows.Err()
}

// GetLink mirrors reading a single debt_transaction_links row by id,
// regardless of which debt it belongs to — callers that need to scope it to
// a specific debt (e.g. DeleteLink) check Link.DebtID themselves.
func GetLink(ctx context.Context, q Querier, id string) (Link, error) {
	var l Link
	var linkedAmountRaw, linkedAtRaw string
	err := q.QueryRowContext(ctx,
		`SELECT id, debt_id, transaction_id, linked_amount, linked_at FROM debt_transaction_links WHERE id = ?`, id,
	).Scan(&l.ID, &l.DebtID, &l.TransactionID, &linkedAmountRaw, &linkedAtRaw)
	if errors.Is(err, sql.ErrNoRows) {
		return Link{}, ErrLinkNotFound
	}
	if err != nil {
		return Link{}, err
	}
	if l.LinkedAmount, err = decimal.NewFromString(linkedAmountRaw); err != nil {
		return Link{}, err
	}
	if l.LinkedAt, err = db.ParseTime(linkedAtRaw); err != nil {
		return Link{}, err
	}
	return l, nil
}

// LinkEffectiveAmount mirrors link_effective_amount: recomputed from the
// linked transaction's *current* amount/currency, not the link's stored
// snapshot — see the package doc comment.
func LinkEffectiveAmount(ctx context.Context, q Querier, transactionID string) (decimal.Decimal, error) {
	eff, err := effectiveMoneyFor(ctx, q, transactionID)
	if err != nil {
		return decimal.Zero, err
	}
	if eff == nil || eff.CurrencyCode != "BRL" {
		return decimal.Zero, nil
	}
	return eff.Value.Abs(), nil
}

// LinkedTransactionSummary is the display-only transaction fields a
// DebtLinkedTransaction response needs alongside the link row itself.
type LinkedTransactionSummary struct {
	OccurredAt  *time.Time
	Description *string
}

// TransactionSummaryFor loads the fields _linked_transaction_response reads
// off link.transaction directly (this package has no ORM relationship to
// walk, so the HTTP layer asks for them explicitly per link).
func TransactionSummaryFor(ctx context.Context, q Querier, transactionID string) (LinkedTransactionSummary, error) {
	s, _, err := loadTransaction(ctx, q, transactionID)
	if err != nil {
		return LinkedTransactionSummary{}, err
	}
	return LinkedTransactionSummary{OccurredAt: s.occurredAt, Description: s.description}, nil
}

type transactionSnapshot struct {
	description    *string
	occurredAt     *time.Time
	movementType   *string
	amount         *decimal.Decimal
	amountInAcct   *decimal.Decimal
	currencyCode   *string
	acctCurrency   *string
	acctName       *string
	inclusionState *string
}

func loadTransaction(ctx context.Context, q Querier, transactionID string) (transactionSnapshot, bool, error) {
	var (
		s                                       transactionSnapshot
		description, movementType, currencyCode sql.NullString
		amountRaw, amountInAcctRaw              sql.NullString
		occurredAtRaw                           sql.NullString
		acctCurrency, acctName                  sql.NullString
		inclusionState                          sql.NullString
	)
	err := q.QueryRowContext(ctx, `
		SELECT ft.description, ft.occurred_at, ft.movement_type, ft.amount, ft.amount_in_account_currency,
			ft.currency_code, fa.currency_code, fa.name, tid.state
		FROM financial_transactions ft
		JOIN financial_accounts fa ON fa.id = ft.account_id
		LEFT JOIN transaction_inclusion_decisions tid ON tid.transaction_id = ft.id
		WHERE ft.id = ?`, transactionID,
	).Scan(&description, &occurredAtRaw, &movementType, &amountRaw, &amountInAcctRaw,
		&currencyCode, &acctCurrency, &acctName, &inclusionState)
	if errors.Is(err, sql.ErrNoRows) {
		return transactionSnapshot{}, false, nil
	}
	if err != nil {
		return transactionSnapshot{}, false, err
	}
	if description.Valid {
		s.description = &description.String
	}
	if movementType.Valid {
		s.movementType = &movementType.String
	}
	if currencyCode.Valid {
		s.currencyCode = &currencyCode.String
	}
	if acctCurrency.Valid {
		s.acctCurrency = &acctCurrency.String
	}
	if acctName.Valid {
		s.acctName = &acctName.String
	}
	if inclusionState.Valid {
		s.inclusionState = &inclusionState.String
	}
	if occurredAtRaw.Valid {
		t, err := db.ParseTime(occurredAtRaw.String)
		if err != nil {
			return transactionSnapshot{}, false, err
		}
		s.occurredAt = &t
	}
	if amountRaw.Valid {
		d, err := decimal.NewFromString(amountRaw.String)
		if err != nil {
			return transactionSnapshot{}, false, err
		}
		s.amount = &d
	}
	if amountInAcctRaw.Valid {
		d, err := decimal.NewFromString(amountInAcctRaw.String)
		if err != nil {
			return transactionSnapshot{}, false, err
		}
		s.amountInAcct = &d
	}
	return s, true, nil
}

func effectiveMoneyFor(ctx context.Context, q Querier, transactionID string) (*money.EffectiveMoney, error) {
	s, ok, err := loadTransaction(ctx, q, transactionID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, nil
	}
	return money.SelectEffectiveMoney(s.amountInAcct, s.acctCurrency, s.amount, s.currencyCode), nil
}

func inclusionStateFor(s transactionSnapshot) money.InclusionState {
	if s.inclusionState != nil && *s.inclusionState == string(money.Ignored) {
		return money.Ignored
	}
	return money.Considered
}

// EligibleTransaction mirrors EligibleTransactionRow.
type EligibleTransaction struct {
	ID             string
	OccurredAt     *time.Time
	Description    *string
	AccountName    *string
	EffectiveMoney money.EffectiveMoney
}

// ListEligibleTransactions mirrors list_eligible_transactions: every
// not-yet-linked transaction eligible to pay down some debt (an ignored,
// non-outflow, or non-BRL transaction can't), optionally filtered by a
// case-insensitive description search, newest first.
func ListEligibleTransactions(ctx context.Context, q Querier, search *string, limit int) ([]EligibleTransaction, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT ft.id, ft.description, ft.occurred_at, ft.movement_type, ft.amount, ft.amount_in_account_currency,
			ft.currency_code, fa.currency_code, fa.name, tid.state
		FROM financial_transactions ft
		JOIN financial_accounts fa ON fa.id = ft.account_id
		LEFT JOIN transaction_inclusion_decisions tid ON tid.transaction_id = ft.id
		WHERE ft.id NOT IN (SELECT transaction_id FROM debt_transaction_links)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var needle string
	if search != nil {
		needle = strings.ToLower(strings.TrimSpace(*search))
	}

	type candidate struct {
		row EligibleTransaction
		key time.Time
	}
	var candidates []candidate
	for rows.Next() {
		var (
			id                                      string
			description, movementType, currencyCode sql.NullString
			amountRaw, amountInAcctRaw              sql.NullString
			occurredAtRaw                           sql.NullString
			acctCurrency, acctName                  sql.NullString
			inclusionState                          sql.NullString
		)
		if err := rows.Scan(&id, &description, &occurredAtRaw, &movementType, &amountRaw, &amountInAcctRaw,
			&currencyCode, &acctCurrency, &acctName, &inclusionState); err != nil {
			return nil, err
		}
		s := transactionSnapshot{}
		if description.Valid {
			s.description = &description.String
		}
		if movementType.Valid {
			s.movementType = &movementType.String
		}
		if currencyCode.Valid {
			s.currencyCode = &currencyCode.String
		}
		if acctCurrency.Valid {
			s.acctCurrency = &acctCurrency.String
		}
		if acctName.Valid {
			s.acctName = &acctName.String
		}
		if inclusionState.Valid {
			s.inclusionState = &inclusionState.String
		}
		if occurredAtRaw.Valid {
			t, err := db.ParseTime(occurredAtRaw.String)
			if err != nil {
				return nil, err
			}
			s.occurredAt = &t
		}
		if amountRaw.Valid {
			d, err := decimal.NewFromString(amountRaw.String)
			if err != nil {
				return nil, err
			}
			s.amount = &d
		}
		if amountInAcctRaw.Valid {
			d, err := decimal.NewFromString(amountInAcctRaw.String)
			if err != nil {
				return nil, err
			}
			s.amountInAcct = &d
		}

		classification := money.Classify(s.movementType)
		eff := money.SelectEffectiveMoney(s.amountInAcct, s.acctCurrency, s.amount, s.currencyCode)
		eligibility := EligibilityForLink(classification, inclusionStateFor(s), eff, false)
		if !eligibility.Eligible {
			continue
		}
		if needle != "" && (s.description == nil || !strings.Contains(strings.ToLower(*s.description), needle)) {
			continue
		}
		orderKey := time.Time{}
		if s.occurredAt != nil {
			orderKey = *s.occurredAt
		}
		candidates = append(candidates, candidate{
			row: EligibleTransaction{ID: id, OccurredAt: s.occurredAt, Description: s.description, AccountName: s.acctName, EffectiveMoney: *eff},
			key: orderKey,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].key.After(candidates[j].key) })
	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}
	result := make([]EligibleTransaction, len(candidates))
	for i, c := range candidates {
		result[i] = c.row
	}
	return result, nil
}

// CreateLinkStatus mirrors CreateLinkResult.status.
type CreateLinkStatus string

const (
	StatusCreated             CreateLinkStatus = "created"
	StatusDebtNotFound        CreateLinkStatus = "debt_not_found"
	StatusTransactionNotFound CreateLinkStatus = "transaction_not_found"
	StatusIneligible          CreateLinkStatus = "ineligible"
	StatusConflict            CreateLinkStatus = "conflict"
)

// CreateLinkResult mirrors CreateLinkResult.
type CreateLinkResult struct {
	Status CreateLinkStatus
	Link   *Link
	Reason *LinkIneligibilityReason
}

// CreateLink mirrors create_link.
func CreateLink(ctx context.Context, conn *sql.DB, debtID, transactionID string) (CreateLinkResult, error) {
	if _, err := Get(ctx, conn, debtID); errors.Is(err, ErrNotFound) {
		return CreateLinkResult{Status: StatusDebtNotFound}, nil
	} else if err != nil {
		return CreateLinkResult{}, err
	}

	snapshot, found, err := loadTransaction(ctx, conn, transactionID)
	if err != nil {
		return CreateLinkResult{}, err
	}
	if !found {
		return CreateLinkResult{Status: StatusTransactionNotFound}, nil
	}

	var existingLinkID sql.NullString
	if err := conn.QueryRowContext(ctx, `SELECT id FROM debt_transaction_links WHERE transaction_id = ?`, transactionID).Scan(&existingLinkID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return CreateLinkResult{}, err
	}
	alreadyLinked := existingLinkID.Valid

	classification := money.Classify(snapshot.movementType)
	eff := money.SelectEffectiveMoney(snapshot.amountInAcct, snapshot.acctCurrency, snapshot.amount, snapshot.currencyCode)
	eligibility := EligibilityForLink(classification, inclusionStateFor(snapshot), eff, alreadyLinked)
	if eligibility.Reason != nil && *eligibility.Reason == ReasonAlreadyLinked {
		return CreateLinkResult{Status: StatusConflict}, nil
	}
	if !eligibility.Eligible {
		return CreateLinkResult{Status: StatusIneligible, Reason: eligibility.Reason}, nil
	}

	link := Link{ID: uuid.NewString(), DebtID: debtID, TransactionID: transactionID, LinkedAmount: eff.Value.Abs(), LinkedAt: time.Now().UTC()}
	_, err = conn.ExecContext(ctx, `
		INSERT INTO debt_transaction_links (id, debt_id, transaction_id, linked_amount, linked_at)
		VALUES (?, ?, ?, ?, ?)`,
		link.ID, link.DebtID, link.TransactionID, money.CanonicalDecimal(link.LinkedAmount), db.FormatTime(link.LinkedAt),
	)
	if err != nil {
		// A UNIQUE violation here (transaction_id) means a concurrent
		// caller won the race despite the already-linked check above —
		// defense in depth, since SQLite's single-connection model (see
		// internal/db) makes this practically unreachable.
		return CreateLinkResult{Status: StatusConflict}, nil
	}
	return CreateLinkResult{Status: StatusCreated, Link: &link}, nil
}

// DeleteLink mirrors delete_link: returns found=false when linkID doesn't
// exist under debtID (a 404 either way at the HTTP layer, but keeps the
// scoping check explicit here rather than delegating it to a WHERE clause
// that would silently no-op on a mismatched debt_id).
func DeleteLink(ctx context.Context, conn *sql.DB, debtID, linkID string) (bool, error) {
	var storedDebtID string
	err := conn.QueryRowContext(ctx, `SELECT debt_id FROM debt_transaction_links WHERE id = ?`, linkID).Scan(&storedDebtID)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && storedDebtID != debtID) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if _, err := conn.ExecContext(ctx, `DELETE FROM debt_transaction_links WHERE id = ?`, linkID); err != nil {
		return false, err
	}
	return true, nil
}

// UnlinkIfPresent mirrors unlink_if_present. Its signature matches
// transactions.OnIgnoredHook exactly, so it can be passed directly wherever
// one is needed (transactions.SetInclusion, automation's apply paths, and
// the sync pipeline via automation.NewTransactionHook) without an adapter.
func UnlinkIfPresent(ctx context.Context, q transactions.Querier, transactionID string) error {
	_, err := q.ExecContext(ctx, `DELETE FROM debt_transaction_links WHERE transaction_id = ?`, transactionID)
	return err
}
