package categories_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"contadinho-go/internal/categories"
	"contadinho-go/internal/money"
)

const seededTransferCategory = "533d9187-99b6-542b-a2f3-6eb9cbb299ce" // Transferência entre Contas Próprias

// TestApplyAutomaticCategorizesSamePersonTransfer is the whole point of
// mapping the label: the assigned category must be transfer-kind, because
// that kind is what money.Eligibility keys on to drop the transaction from
// income/expense totals.
func TestApplyAutomaticCategorizesSamePersonTransfer(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()

	label := categories.SamePersonTransferLabel
	txID := insertTransaction(t, conn, &label)

	if err := categories.ApplyAutomatic(ctx, conn, txID, &label); err != nil {
		t.Fatalf("ApplyAutomatic: %v", err)
	}

	d, err := getDecision(t, conn, txID)
	if err != nil {
		t.Fatalf("read decision: %v", err)
	}
	if d.categoryID != seededTransferCategory || d.origin != "automatic" {
		t.Fatalf("decision = %+v, want the seeded transfer category assigned automatically", d)
	}
	category, err := categories.Get(ctx, conn, d.categoryID)
	if err != nil {
		t.Fatalf("Get assigned category: %v", err)
	}
	if category.Kind != money.Transfer {
		t.Errorf("assigned category kind = %q, want %q", category.Kind, money.Transfer)
	}
}

func TestBackfillAutomaticCategorizesAlreadySyncedTransactions(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()

	label := categories.SamePersonTransferLabel
	accountID, rawImportID := newTestAccount(t, conn)
	first := insertTransactionOnAccount(t, conn, accountID, rawImportID, label, nil, nil)
	second := insertTransactionOnAccount(t, conn, accountID, rawImportID, label, nil, nil)
	// Another label entirely: the backfill addresses one source category, so
	// nothing else may be swept along with it.
	unrelated := insertTransactionOnAccount(t, conn, accountID, rawImportID, "Groceries", nil, nil)

	applied, err := categories.BackfillAutomatic(ctx, conn, label)
	if err != nil {
		t.Fatalf("BackfillAutomatic: %v", err)
	}
	if applied != 2 {
		t.Errorf("applied = %d, want 2", applied)
	}
	for _, id := range []string{first, second} {
		d, err := getDecision(t, conn, id)
		if err != nil {
			t.Fatalf("read decision for %s: %v", id, err)
		}
		if d.categoryID != seededTransferCategory {
			t.Errorf("decision for %s = %+v, want the transfer category", id, d)
		}
	}
	if _, err := getDecision(t, conn, unrelated); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("the Groceries transaction was touched, err = %v", err)
	}

	// Idempotent: the startup hook re-runs this on every boot.
	applied, err = categories.BackfillAutomatic(ctx, conn, label)
	if err != nil {
		t.Fatalf("BackfillAutomatic (second run): %v", err)
	}
	if applied != 0 {
		t.Errorf("second run applied = %d, want 0", applied)
	}
}

// TestBackfillAutomaticKeepsExistingDecisions guards the one thing a
// retroactive pass must never do: a user who already filed one of these
// transfers under a category of their own keeps it.
func TestBackfillAutomaticKeepsExistingDecisions(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()

	label := categories.SamePersonTransferLabel
	accountID, rawImportID := newTestAccount(t, conn)
	manual := insertTransactionOnAccount(t, conn, accountID, rawImportID, label, nil, nil)
	untouched := insertTransactionOnAccount(t, conn, accountID, rawImportID, label, nil, nil)

	if _, err := categories.AssignManual(ctx, conn, manual, seededExpenseCategory); err != nil {
		t.Fatalf("AssignManual: %v", err)
	}

	applied, err := categories.BackfillAutomatic(ctx, conn, label)
	if err != nil {
		t.Fatalf("BackfillAutomatic: %v", err)
	}
	if applied != 1 {
		t.Errorf("applied = %d, want 1 — only the uncategorized transfer", applied)
	}

	d, err := getDecision(t, conn, manual)
	if err != nil {
		t.Fatalf("read decision: %v", err)
	}
	if d.categoryID != seededExpenseCategory || d.origin != "manual" {
		t.Errorf("backfill overwrote a manual decision: %+v", d)
	}
	if d2, err := getDecision(t, conn, untouched); err != nil || d2.categoryID != seededTransferCategory {
		t.Errorf("decision = %+v, err = %v, want the transfer category", d2, err)
	}
}
