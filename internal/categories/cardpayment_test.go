package categories_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/categories"
	"contadinho-go/internal/db"
)

// insertCardPaymentLeg is insertTransactionOnAccount plus the three
// provider fields isCardPaymentTransaction needs, which the shared helper
// (categories_test.go) doesn't carry — added here rather than extending it,
// since other tests depend on its current shape.
func insertCardPaymentLeg(t *testing.T, conn *sql.DB, accountID, rawImportID, movementType, sourceCategory, sourceCategoryID, additionalInfo string) string {
	t.Helper()
	now := db.FormatTime(time.Now())
	txID := uuid.NewString()
	nullable := func(s string) any {
		if s == "" {
			return nil
		}
		return s
	}
	if _, err := conn.Exec(`INSERT INTO financial_transactions (
			id, source_id, account_id, external_id, description,
			movement_type, source_category, source_category_id, operation_type_additional_info,
			current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, (SELECT source_id FROM financial_accounts WHERE id = ?), ?, ?, 'Transação', ?, ?, ?, ?, ?, 'hash', ?, ?)`,
		txID, accountID, accountID, txID,
		nullable(movementType), nullable(sourceCategory), nullable(sourceCategoryID), nullable(additionalInfo),
		rawImportID, now, now,
	); err != nil {
		t.Fatalf("insert card payment leg: %v", err)
	}
	return txID
}

// TestApplyAutomaticCardPaymentCategorizesBankLeg is the whole point of
// recognizing the bank-debit leg: the assigned category must be the seeded
// transfer-kind one, so money.MovedCash counts it in balance reconstruction
// while money.Eligibility keeps it out of income/expense totals.
func TestApplyAutomaticCardPaymentCategorizesBankLeg(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	accountID, rawImportID := newTestAccount(t, conn)
	txID := insertCardPaymentLeg(t, conn, accountID, rawImportID, "DEBIT", "", "", "PAGAMENTO_FATURA")

	if err := categories.ApplyAutomaticCardPayment(ctx, conn, txID, "DEBIT", "", "", "PAGAMENTO_FATURA"); err != nil {
		t.Fatalf("ApplyAutomaticCardPayment: %v", err)
	}

	d, err := getDecision(t, conn, txID)
	if err != nil {
		t.Fatalf("read decision: %v", err)
	}
	if d.categoryID != categories.CardPaymentCategoryID || d.origin != "automatic" {
		t.Fatalf("decision = %+v, want the card payment category assigned automatically", d)
	}
}

// TestApplyAutomaticCardPaymentCategorizesCardLeg covers the other leg,
// matched by either the numeric source_category_id or the label.
func TestApplyAutomaticCardPaymentCategorizesCardLeg(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	accountID, rawImportID := newTestAccount(t, conn)

	byID := insertCardPaymentLeg(t, conn, accountID, rawImportID, "CREDIT", "", "05100000", "")
	if err := categories.ApplyAutomaticCardPayment(ctx, conn, byID, "CREDIT", "", "05100000", ""); err != nil {
		t.Fatalf("ApplyAutomaticCardPayment (by id): %v", err)
	}
	if d, err := getDecision(t, conn, byID); err != nil || d.categoryID != categories.CardPaymentCategoryID {
		t.Fatalf("decision = %+v, err = %v, want the card payment category", d, err)
	}

	byLabel := insertCardPaymentLeg(t, conn, accountID, rawImportID, "CREDIT", "Credit card payment", "", "")
	if err := categories.ApplyAutomaticCardPayment(ctx, conn, byLabel, "CREDIT", "Credit card payment", "", ""); err != nil {
		t.Fatalf("ApplyAutomaticCardPayment (by label): %v", err)
	}
	if d, err := getDecision(t, conn, byLabel); err != nil || d.categoryID != categories.CardPaymentCategoryID {
		t.Fatalf("decision = %+v, err = %v, want the card payment category", d, err)
	}
}

// TestApplyAutomaticCardPaymentIgnoresUnrelatedTransactions guards the one
// thing that would make this rule dangerous: an ordinary card purchase is
// also a DEBIT, and an ordinary card refund/cashback is also a CREDIT — the
// exact additional-info/source_category_id values must not match either.
func TestApplyAutomaticCardPaymentIgnoresUnrelatedTransactions(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	accountID, rawImportID := newTestAccount(t, conn)

	purchase := insertCardPaymentLeg(t, conn, accountID, rawImportID, "DEBIT", "", "", "")
	if err := categories.ApplyAutomaticCardPayment(ctx, conn, purchase, "DEBIT", "", "", ""); err != nil {
		t.Fatalf("ApplyAutomaticCardPayment: %v", err)
	}
	if _, err := getDecision(t, conn, purchase); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("an ordinary purchase got a decision, err = %v", err)
	}

	otherCredit := insertCardPaymentLeg(t, conn, accountID, rawImportID, "CREDIT", "Cashback", "", "")
	if err := categories.ApplyAutomaticCardPayment(ctx, conn, otherCredit, "CREDIT", "Cashback", "", ""); err != nil {
		t.Fatalf("ApplyAutomaticCardPayment: %v", err)
	}
	if _, err := getDecision(t, conn, otherCredit); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("an unrelated credit got a decision, err = %v", err)
	}
}

func TestBackfillAutomaticCardPaymentCategorizesAlreadySyncedTransactions(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	accountID, rawImportID := newTestAccount(t, conn)

	bankLeg := insertCardPaymentLeg(t, conn, accountID, rawImportID, "DEBIT", "", "", "PAGAMENTO_FATURA")
	cardLeg := insertCardPaymentLeg(t, conn, accountID, rawImportID, "CREDIT", "", "05100000", "")
	unrelated := insertCardPaymentLeg(t, conn, accountID, rawImportID, "DEBIT", "", "", "")

	applied, err := categories.BackfillAutomaticCardPayment(ctx, conn)
	if err != nil {
		t.Fatalf("BackfillAutomaticCardPayment: %v", err)
	}
	if applied != 2 {
		t.Errorf("applied = %d, want 2", applied)
	}
	for _, id := range []string{bankLeg, cardLeg} {
		d, err := getDecision(t, conn, id)
		if err != nil {
			t.Fatalf("read decision for %s: %v", id, err)
		}
		if d.categoryID != categories.CardPaymentCategoryID {
			t.Errorf("decision for %s = %+v, want the card payment category", id, d)
		}
	}
	if _, err := getDecision(t, conn, unrelated); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("the unrelated transaction was touched, err = %v", err)
	}

	// Idempotent: the startup hook re-runs this on every boot.
	applied, err = categories.BackfillAutomaticCardPayment(ctx, conn)
	if err != nil {
		t.Fatalf("BackfillAutomaticCardPayment (second run): %v", err)
	}
	if applied != 0 {
		t.Errorf("second run applied = %d, want 0", applied)
	}
}

// TestBackfillAutomaticCardPaymentKeepsExistingDecisions guards the same
// thing TestBackfillAutomaticKeepsExistingDecisions guards for the
// same-person-transfer backfill: a leg the user already categorized by hand
// keeps that decision.
func TestBackfillAutomaticCardPaymentKeepsExistingDecisions(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	accountID, rawImportID := newTestAccount(t, conn)

	manual := insertCardPaymentLeg(t, conn, accountID, rawImportID, "DEBIT", "", "", "PAGAMENTO_FATURA")
	untouched := insertCardPaymentLeg(t, conn, accountID, rawImportID, "CREDIT", "", "05100000", "")

	if _, err := categories.AssignManual(ctx, conn, manual, seededExpenseCategory); err != nil {
		t.Fatalf("AssignManual: %v", err)
	}

	applied, err := categories.BackfillAutomaticCardPayment(ctx, conn)
	if err != nil {
		t.Fatalf("BackfillAutomaticCardPayment: %v", err)
	}
	if applied != 1 {
		t.Errorf("applied = %d, want 1 — only the uncategorized leg", applied)
	}

	d, err := getDecision(t, conn, manual)
	if err != nil {
		t.Fatalf("read decision: %v", err)
	}
	if d.categoryID != seededExpenseCategory || d.origin != "manual" {
		t.Errorf("backfill overwrote a manual decision: %+v", d)
	}
	if d2, err := getDecision(t, conn, untouched); err != nil || d2.categoryID != categories.CardPaymentCategoryID {
		t.Errorf("decision = %+v, err = %v, want the card payment category", d2, err)
	}
}
