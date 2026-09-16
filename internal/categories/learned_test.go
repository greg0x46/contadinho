package categories_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"testing"
	"time"

	"contadinho-go/internal/categories"
	"contadinho-go/internal/db"
	"contadinho-go/internal/money"
)

// learnedFixture is one account with a hand-categorized "reference"
// transaction, the shape every ApplyLearned test starts from.
type learnedFixture struct {
	conn        *sql.DB
	accountID   string
	rawImportID string
}

func newLearnedFixture(t *testing.T) learnedFixture {
	t.Helper()
	conn := newTestDB(t)
	accountID, rawImportID := newTestAccount(t, conn)
	return learnedFixture{conn: conn, accountID: accountID, rawImportID: rawImportID}
}

// insert adds a transaction with the given description and movement_type
// ("" leaves it NULL, as some providers do).
func (f learnedFixture) insert(t *testing.T, description, movementType string) string {
	t.Helper()
	id := insertTransactionOnAccount(t, f.conn, f.accountID, f.rawImportID, "", &description, nil)
	if movementType != "" {
		if _, err := f.conn.Exec(`UPDATE financial_transactions SET movement_type = ? WHERE id = ?`, movementType, id); err != nil {
			t.Fatalf("set movement_type: %v", err)
		}
	}
	return id
}

// manual hand-categorizes id and pins the decision's changed_at so tests
// can order references deterministically.
func (f learnedFixture) manual(t *testing.T, id, categoryID string, changedAt time.Time) {
	t.Helper()
	if _, err := categories.AssignManual(context.Background(), f.conn, id, categoryID); err != nil {
		t.Fatalf("AssignManual: %v", err)
	}
	if _, err := f.conn.Exec(`UPDATE transaction_category_decisions SET changed_at = ? WHERE transaction_id = ?`,
		db.FormatTime(changedAt), id); err != nil {
		t.Fatalf("pin changed_at: %v", err)
	}
}

func (f learnedFixture) apply(t *testing.T, id string) bool {
	t.Helper()
	changed, err := categories.ApplyLearned(context.Background(), f.conn, id)
	if err != nil {
		t.Fatalf("ApplyLearned: %v", err)
	}
	return changed
}

func (f learnedFixture) expectDecision(t *testing.T, id, categoryID, origin string) {
	t.Helper()
	got, err := getDecision(t, f.conn, id)
	if err != nil {
		t.Fatalf("getDecision(%s): %v", id, err)
	}
	if got.categoryID != categoryID || got.origin != origin {
		t.Errorf("decision = %+v, want category=%s origin=%s", got, categoryID, origin)
	}
}

func (f learnedFixture) expectNoDecision(t *testing.T, id string) {
	t.Helper()
	if _, err := getDecision(t, f.conn, id); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected no decision for %s, err = %v", id, err)
	}
}

func TestApplyLearnedNoReferenceIsNoop(t *testing.T) {
	f := newLearnedFixture(t)
	id := f.insert(t, "Padaria Rv", "DEBIT")
	if f.apply(t, id) {
		t.Errorf("changed = true without any manual decision")
	}
	f.expectNoDecision(t, id)
}

func TestApplyLearnedCopiesManualDecisionOfSimilarTransaction(t *testing.T) {
	f := newLearnedFixture(t)
	ref := f.insert(t, "Padaria Rv", "DEBIT")
	f.manual(t, ref, seededExpenseCategory, time.Now())

	// Case and surrounding whitespace differ; the key is the same.
	id := f.insert(t, "  PADARIA RV ", "DEBIT")
	if !f.apply(t, id) {
		t.Fatalf("changed = false")
	}
	f.expectDecision(t, id, seededExpenseCategory, "learned")

	var events int
	if err := f.conn.QueryRow(`SELECT count(*) FROM transaction_category_events WHERE transaction_id = ? AND origin = 'learned'`, id).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 1 {
		t.Errorf("learned events = %d, want 1", events)
	}

	// Idempotent: a second pass changes nothing.
	if f.apply(t, id) {
		t.Errorf("second ApplyLearned reported a change")
	}
}

func TestApplyLearnedRequiresSameMovementType(t *testing.T) {
	f := newLearnedFixture(t)
	ref := f.insert(t, "Pix Fulano", "CREDIT")
	f.manual(t, ref, seededExpenseCategory, time.Now())

	debit := f.insert(t, "Pix Fulano", "DEBIT")
	f.apply(t, debit)
	f.expectNoDecision(t, debit)

	nullType := f.insert(t, "Pix Fulano", "")
	f.apply(t, nullType)
	f.expectNoDecision(t, nullType)

	credit := f.insert(t, "Pix Fulano", "CREDIT")
	f.apply(t, credit)
	f.expectDecision(t, credit, seededExpenseCategory, "learned")
}

func TestApplyLearnedPrecedence(t *testing.T) {
	f := newLearnedFixture(t)
	ctx := context.Background()
	other, err := categories.Create(ctx, f.conn, "Outra", money.Expense, "tag", "#000000")
	if err != nil {
		t.Fatal(err)
	}
	ref := f.insert(t, "Loja Z", "DEBIT")
	f.manual(t, ref, seededExpenseCategory, time.Now())

	// Overrides an automatic decision.
	auto := f.insert(t, "Loja Z", "DEBIT")
	src := "Groceries"
	if err := categories.ApplyAutomatic(ctx, f.conn, auto, &src); err != nil {
		t.Fatal(err)
	}
	f.expectDecision(t, auto, seededExpenseCategory, "automatic")
	// Force a different automatic category so the override is observable.
	if _, err := f.conn.Exec(`UPDATE transaction_category_decisions SET category_id = ? WHERE transaction_id = ?`, other.ID, auto); err != nil {
		t.Fatal(err)
	}
	if !f.apply(t, auto) {
		t.Errorf("learned did not override automatic")
	}
	f.expectDecision(t, auto, seededExpenseCategory, "learned")

	// Never touches a manual decision.
	manual := f.insert(t, "Loja Z", "DEBIT")
	f.manual(t, manual, other.ID, time.Now().Add(-time.Hour))
	f.apply(t, manual)
	f.expectDecision(t, manual, other.ID, "manual")

	// Never touches a rule decision.
	ruled := f.insert(t, "Loja Z", "DEBIT")
	if _, _, err := categories.ApplyRule(ctx, f.conn, ruled, other.ID); err != nil {
		t.Fatal(err)
	}
	f.apply(t, ruled)
	f.expectDecision(t, ruled, other.ID, "rule")

	// A rule applied afterwards overrides a learned decision.
	learned := f.insert(t, "Loja Z", "DEBIT")
	f.apply(t, learned)
	f.expectDecision(t, learned, seededExpenseCategory, "learned")
	if _, changed, err := categories.ApplyRule(ctx, f.conn, learned, other.ID); err != nil || !changed {
		t.Fatalf("ApplyRule over learned: changed=%v err=%v", changed, err)
	}
	f.expectDecision(t, learned, other.ID, "rule")
}

func TestApplyLearnedMostRecentManualDecisionWins(t *testing.T) {
	f := newLearnedFixture(t)
	ctx := context.Background()
	newer, err := categories.Create(ctx, f.conn, "Mais Recente", money.Expense, "tag", "#000000")
	if err != nil {
		t.Fatal(err)
	}
	old := f.insert(t, "Mercado Q", "DEBIT")
	f.manual(t, old, seededExpenseCategory, time.Now().Add(-48*time.Hour))
	recent := f.insert(t, "Mercado Q", "DEBIT")
	f.manual(t, recent, newer.ID, time.Now().Add(-time.Hour))

	id := f.insert(t, "Mercado Q", "DEBIT")
	f.apply(t, id)
	f.expectDecision(t, id, newer.ID, "learned")
}

func TestApplyLearnedSkipsInactiveCategory(t *testing.T) {
	f := newLearnedFixture(t)
	ctx := context.Background()
	c, err := categories.Create(ctx, f.conn, "Temporária", money.Expense, "tag", "#000000")
	if err != nil {
		t.Fatal(err)
	}
	ref := f.insert(t, "Loja W", "DEBIT")
	f.manual(t, ref, c.ID, time.Now())
	inactive := false
	if _, err := categories.Update(ctx, f.conn, c.ID, nil, &inactive, nil, nil); err != nil {
		t.Fatal(err)
	}

	id := f.insert(t, "Loja W", "DEBIT")
	f.apply(t, id)
	f.expectNoDecision(t, id)
}

func TestApplyLearnedMatchesAcrossInstallmentsAndCascades(t *testing.T) {
	f := newLearnedFixture(t)
	// Reference: parcela 2/12 of an earlier purchase, categorized by hand.
	ref := f.insert(t, "Jim.Com* 50450362 Kau 2/12", "DEBIT")
	f.manual(t, ref, seededExpenseCategory, time.Now())

	// A new purchase at the same merchant, split in 3, arrives.
	var installments [3]string
	for i := 1; i <= 3; i++ {
		description := fmt.Sprintf("Jim.Com* 50450362 Kau %d/3", i)
		metadata := installmentMetadata(t, "1234", i, 3)
		installments[i-1] = insertTransactionOnAccount(t, f.conn, f.accountID, f.rawImportID, "", &description, &metadata)
		if _, err := f.conn.Exec(`UPDATE financial_transactions SET movement_type = 'DEBIT' WHERE id = ?`, installments[i-1]); err != nil {
			t.Fatal(err)
		}
	}
	f.apply(t, installments[0])
	for _, id := range installments {
		f.expectDecision(t, id, seededExpenseCategory, "learned")
	}
}

func TestApplyLearnedPreservesExistingSiblingDecisions(t *testing.T) {
	for _, origin := range []string{"manual", "rule", "learned"} {
		t.Run(origin, func(t *testing.T) {
			f := newLearnedFixture(t)
			ctx := context.Background()
			description := "Loja 1/3"
			metadata := installmentMetadata(t, "1234", 1, 3)
			old := insertTransactionOnAccount(t, f.conn, f.accountID, f.rawImportID, "", &description, &metadata)
			switch origin {
			case "manual":
				f.manual(t, old, seededExpenseCategory, time.Now().Add(-time.Hour))
			case "rule":
				if _, _, err := categories.ApplyRule(ctx, f.conn, old, seededExpenseCategory); err != nil {
					t.Fatal(err)
				}
			case "learned":
				ref := f.insert(t, "Loja", "")
				f.manual(t, ref, seededExpenseCategory, time.Now().Add(-time.Hour))
				f.apply(t, old)
			}
			other, err := categories.Create(ctx, f.conn, "Outra", money.Expense, "tag", "#000000")
			if err != nil {
				t.Fatal(err)
			}
			ref := f.insert(t, "Loja", "")
			f.manual(t, ref, other.ID, time.Now())
			description = "Loja 2/3"
			metadata = installmentMetadata(t, "1234", 2, 3)
			fresh := insertTransactionOnAccount(t, f.conn, f.accountID, f.rawImportID, "", &description, &metadata)
			if !f.apply(t, fresh) {
				t.Fatal("new installment was not categorized")
			}
			f.expectDecision(t, fresh, other.ID, "learned")
			f.expectDecision(t, old, seededExpenseCategory, origin)
			var events int
			if err := f.conn.QueryRow(`SELECT count(*) FROM transaction_category_events WHERE transaction_id = ?`, old).Scan(&events); err != nil {
				t.Fatal(err)
			}
			if events != 1 {
				t.Errorf("existing installment events = %d, want 1", events)
			}
		})
	}
}

func TestApplyLearnedPromotesSameAutomaticCategory(t *testing.T) {
	f := newLearnedFixture(t)
	ref := f.insert(t, "Loja", "DEBIT")
	f.manual(t, ref, seededExpenseCategory, time.Now())
	id := f.insert(t, "Loja", "DEBIT")
	source := "Groceries"
	if err := categories.ApplyAutomatic(context.Background(), f.conn, id, &source); err != nil {
		t.Fatal(err)
	}
	if !f.apply(t, id) {
		t.Error("origin promotion did not report a change")
	}
	f.expectDecision(t, id, seededExpenseCategory, "learned")
	if f.apply(t, id) {
		t.Error("second application reported a change")
	}
	var revision, events int
	var previous, resulting, origin string
	if err := f.conn.QueryRow(`SELECT revision, previous_category_id, resulting_category_id, origin
		FROM transaction_category_events WHERE transaction_id = ? ORDER BY revision DESC LIMIT 1`, id).
		Scan(&revision, &previous, &resulting, &origin); err != nil {
		t.Fatal(err)
	}
	if revision != 2 || previous != seededExpenseCategory || resulting != seededExpenseCategory || origin != "learned" {
		t.Errorf("unexpected promotion event: revision=%d previous=%s resulting=%s origin=%s", revision, previous, resulting, origin)
	}
	if err := f.conn.QueryRow(`SELECT count(*) FROM transaction_category_events WHERE transaction_id = ?`, id).Scan(&events); err != nil {
		t.Fatal(err)
	}
	if events != 2 {
		t.Errorf("events = %d, want 2", events)
	}
}
