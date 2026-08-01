package categories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
)

// Origin distinguishes a category decision the user made explicitly from one
// the automatic source_category mapping applied.
type Origin string

const (
	OriginManual    Origin = "manual"
	OriginAutomatic Origin = "automatic"
)

// Decision is a transaction's current category assignment.
type Decision struct {
	TransactionID string
	CategoryID    string
	Revision      int
	ChangedAt     time.Time
	Origin        Origin
}

// ErrTransactionNotFound and ErrCategoryInvalid are the two failure modes of
// AssignManual, mirroring assign_transaction_category's
// AssignCategoryError literal.
var (
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrCategoryInvalid     = errors.New("category not found or inactive")
)

func getDecision(ctx context.Context, q Querier, transactionID string) (*Decision, error) {
	var (
		d            Decision
		changedAtRaw string
	)
	err := q.QueryRowContext(ctx,
		`SELECT transaction_id, category_id, revision, changed_at, origin
		 FROM transaction_category_decisions WHERE transaction_id = ?`,
		transactionID,
	).Scan(&d.TransactionID, &d.CategoryID, &d.Revision, &changedAtRaw, &d.Origin)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if d.ChangedAt, err = db.ParseTime(changedAtRaw); err != nil {
		return nil, err
	}
	return &d, nil
}

// writeDecision upserts the decision row and appends the corresponding
// append-only event, mirroring categorization.py's _write_category_decision.
// It is a no-op (changed=false) when the transaction is already assigned to
// categoryID.
func writeDecision(
	ctx context.Context, q Querier, transactionID, categoryID string, origin Origin,
) (decision Decision, changed bool, err error) {
	existing, err := getDecision(ctx, q, transactionID)
	if err != nil {
		return Decision{}, false, err
	}
	if existing != nil && existing.CategoryID == categoryID {
		return *existing, false, nil
	}

	changedAt := time.Now().UTC()
	revision := 1
	var previousCategoryID *string
	if existing != nil {
		revision = existing.Revision + 1
		previousCategoryID = &existing.CategoryID
	}

	_, err = q.ExecContext(ctx,
		`INSERT INTO transaction_category_decisions
			(transaction_id, category_id, revision, changed_at, origin)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT (transaction_id) DO UPDATE SET
			category_id = excluded.category_id,
			revision = excluded.revision,
			changed_at = excluded.changed_at,
			origin = excluded.origin`,
		transactionID, categoryID, revision, db.FormatTime(changedAt), string(origin),
	)
	if err != nil {
		return Decision{}, false, err
	}
	_, err = q.ExecContext(ctx,
		`INSERT INTO transaction_category_events
			(id, transaction_id, revision, previous_category_id, resulting_category_id, changed_at, origin)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		uuid.NewString(), transactionID, revision, previousCategoryID, categoryID, db.FormatTime(changedAt), string(origin),
	)
	if err != nil {
		return Decision{}, false, err
	}
	return Decision{
		TransactionID: transactionID,
		CategoryID:    categoryID,
		Revision:      revision,
		ChangedAt:     changedAt,
		Origin:        origin,
	}, true, nil
}

// AssignManual mirrors assign_transaction_category: always overrides any
// prior decision (manual or automatic). Returns ErrTransactionNotFound or
// ErrCategoryInvalid (category missing or inactive) instead of writing.
func AssignManual(ctx context.Context, q Querier, transactionID, categoryID string) (Decision, error) {
	var exists int
	err := q.QueryRowContext(ctx,
		`SELECT 1 FROM financial_transactions WHERE id = ?`, transactionID,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return Decision{}, ErrTransactionNotFound
	}
	if err != nil {
		return Decision{}, err
	}

	category, err := Get(ctx, q, categoryID)
	if errors.Is(err, ErrNotFound) || !category.IsActive {
		return Decision{}, ErrCategoryInvalid
	}
	if err != nil {
		return Decision{}, err
	}

	decision, _, err := writeDecision(ctx, q, transactionID, categoryID, OriginManual)
	return decision, err
}

// ApplyAutomatic mirrors apply_automatic_category: writes an automatic
// category decision only when the transaction has none yet, and silently
// no-ops (never returns an error a sync run would need to fail on) when
// sourceCategory is absent, unmapped, or maps to a category that no longer
// exists.
func ApplyAutomatic(ctx context.Context, q Querier, transactionID string, sourceCategory *string) error {
	if sourceCategory == nil {
		return nil
	}
	categoryID, ok := SourceCategoryMapping[*sourceCategory]
	if !ok {
		return nil
	}
	existing, err := getDecision(ctx, q, transactionID)
	if err != nil {
		return err
	}
	if existing != nil {
		return nil
	}
	if _, err := Get(ctx, q, categoryID); errors.Is(err, ErrNotFound) {
		return nil
	} else if err != nil {
		return err
	}
	_, _, err = writeDecision(ctx, q, transactionID, categoryID, OriginAutomatic)
	return err
}
