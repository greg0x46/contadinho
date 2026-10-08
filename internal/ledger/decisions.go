// Package ledger holds application-level flows that coordinate several domain
// engines (transactions, categories, automation) inside one database
// transaction. HTTP handlers and importers call it so the ordering rules live
// in one place.
package ledger

import (
	"context"

	"contadinho-go/internal/automation"
	"contadinho-go/internal/categories"
	"contadinho-go/internal/transactions"
)

// ApplyNewTransactionDecisions runs the category passes for a freshly inserted
// lançamento, using q's transaction. Same precedence as the sync path
// (syncsvc.upsertTransaction): a learned category from a past manual decision
// goes first so a matching rule can still override it; an explicit categoryID
// is a manual decision and overrides both. nil categoryID means none was chosen.
func ApplyNewTransactionDecisions(ctx context.Context, q transactions.Querier, id string, categoryID *string, onIgnored transactions.OnIgnoredHook) error {
	if categoryID == nil {
		if _, err := categories.ApplyLearned(ctx, q, id); err != nil {
			return err
		}
	}
	if err := automation.ApplyToNewTransactionWithQuerier(ctx, q, id, onIgnored); err != nil {
		return err
	}
	if categoryID != nil {
		if _, err := categories.AssignManualWithQuerier(ctx, q, id, *categoryID); err != nil {
			return err
		}
	}
	return nil
}
