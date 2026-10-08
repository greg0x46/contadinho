package ledger

import (
	"context"
	"database/sql"
	"errors"

	"github.com/greg0x46/julius/internal/categories"
	"github.com/greg0x46/julius/internal/transactions"
)

// ErrItemUnavailable is returned when a committed lançamento cannot be read back.
var ErrItemUnavailable = errors.New("transaction unavailable after commit")

// Service owns the lançamento manual flows (see
// .specs/contextos/transacoes/reference.md). Domain errors from the underlying
// packages are returned unchanged so callers can match them with errors.Is.
type Service struct {
	DB        *sql.DB
	OnIgnored transactions.OnIgnoredHook // nil is a valid no-op
}

// CreateManual inserts a lançamento the user authors by hand and runs the same
// automation pass a sync would, so an existing rule categorizes or ignores a
// manual entry exactly like a synced one. An explicit categoryID is applied
// last, so the user's own choice always wins. Everything commits or rolls back
// together.
func (s *Service) CreateManual(ctx context.Context, in transactions.ManualInput, categoryID *string) (transactions.Item, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return transactions.Item{}, err
	}
	defer tx.Rollback()

	id, err := transactions.CreateManual(ctx, tx, in)
	if err != nil {
		return transactions.Item{}, err
	}
	if err := ApplyNewTransactionDecisions(ctx, tx, id, categoryID, s.OnIgnored); err != nil {
		return transactions.Item{}, err
	}
	if err := tx.Commit(); err != nil {
		return transactions.Item{}, err
	}
	return s.item(ctx, id)
}

// UpdateManual edits a lançamento manual's core fields and, when supplied, its
// category in the same database transaction.
func (s *Service) UpdateManual(ctx context.Context, id string, in transactions.ManualInput, categoryID *string) (transactions.Item, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return transactions.Item{}, err
	}
	defer tx.Rollback()

	if err := transactions.UpdateManual(ctx, tx, id, in); err != nil {
		return transactions.Item{}, err
	}
	if categoryID != nil {
		if _, err := categories.AssignManualWithQuerier(ctx, tx, id, *categoryID); err != nil {
			return transactions.Item{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return transactions.Item{}, err
	}
	return s.item(ctx, id)
}

// DeleteManual soft-deletes a lançamento manual.
func (s *Service) DeleteManual(ctx context.Context, id string) error {
	return transactions.DeleteManual(ctx, s.DB, id, s.OnIgnored)
}

func (s *Service) item(ctx context.Context, id string) (transactions.Item, error) {
	item, found, err := transactions.GetItem(ctx, s.DB, id)
	if err != nil {
		return transactions.Item{}, err
	}
	if !found {
		return transactions.Item{}, ErrItemUnavailable
	}
	return item, nil
}
