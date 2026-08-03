package categories_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/categories"
	"contadinho-go/internal/db"
	"contadinho-go/internal/money"
)

const seededExpenseCategory = "000433b6-3094-5a9c-87df-465b70574a4b" // Supermercado

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// insertTransaction creates the minimal sync-schema chain plus one
// financial_transactions row, so decision tests have a real transaction_id
// to reference.
func insertTransaction(t *testing.T, conn *sql.DB, sourceCategory *string) string {
	t.Helper()
	accountID, rawImportID := newTestAccount(t, conn)
	var sourceCategoryValue string
	if sourceCategory != nil {
		sourceCategoryValue = *sourceCategory
	}
	return insertTransactionOnAccount(t, conn, accountID, rawImportID, sourceCategoryValue, nil, nil)
}

// newTestAccount creates the minimal sync-schema chain (data source, sync
// run, raw import, financial account) shared by every transaction inserted
// against it, returning the account id and the raw_import id new
// transactions on that account should reference.
func newTestAccount(t *testing.T, conn *sql.DB) (accountID, rawImportID string) {
	t.Helper()
	now := db.FormatTime(time.Now())
	sourceID := uuid.NewString()
	syncRunID := uuid.NewString()
	rawImportID = uuid.NewString()
	accountID = uuid.NewString()

	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(query, args...); err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
	}
	exec(`INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at)
		VALUES (?, 'pluggy', 'item-1', ?, ?)`, sourceID, now, now)
	exec(`INSERT INTO sync_runs (id, source_id, status, started_at, finished_at)
		VALUES (?, ?, 'completed', ?, ?)`, syncRunID, sourceID, now, now)
	exec(`INSERT INTO raw_imports (
			id, sync_run_id, source_id, scope, page_sequence, request_attempt,
			request_method, request_path, http_status, response_headers, payload,
			payload_sha256, received_at
		) VALUES (?, ?, ?, 'transactions', 1, 1, 'GET', '/x', 200, '{}', x'00', 'sha', ?)`,
		rawImportID, syncRunID, sourceID, now)
	exec(`INSERT INTO financial_accounts (
			id, source_id, external_id, currency_code, current_raw_import_id, normalized_hash,
			created_at, updated_at
		) VALUES (?, ?, ?, 'BRL', ?, 'hash', ?, ?)`, accountID, sourceID, accountID, rawImportID, now, now)
	return accountID, rawImportID
}

// insertTransactionOnAccount inserts one financial_transactions row on an
// existing account, optionally with description and credit_card_metadata
// (used for installment-grouping tests, where several transactions must
// share one account).
func insertTransactionOnAccount(t *testing.T, conn *sql.DB, accountID, rawImportID, sourceCategory string, description, creditCardMetadata *string) string {
	t.Helper()
	now := db.FormatTime(time.Now())
	txID := uuid.NewString()
	var sourceCategoryArg any
	if sourceCategory != "" {
		sourceCategoryArg = sourceCategory
	}
	if _, err := conn.Exec(`INSERT INTO financial_transactions (
			id, source_id, account_id, external_id, description, credit_card_metadata,
			source_category, current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, (SELECT source_id FROM financial_accounts WHERE id = ?), ?, ?, ?, ?, ?, ?, 'hash', ?, ?)`,
		txID, accountID, accountID, txID, description, creditCardMetadata, sourceCategoryArg, rawImportID, now, now,
	); err != nil {
		t.Fatalf("insert transaction: %v", err)
	}
	return txID
}

func TestCreateUpdateGetList(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()

	c, err := categories.Create(ctx, conn, "Assinaturas", money.Expense)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !c.IsActive {
		t.Errorf("new category should be active")
	}

	got, err := categories.Get(ctx, conn, c.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != "Assinaturas" || got.Kind != money.Expense {
		t.Errorf("Get() = %+v", got)
	}

	newName := "Assinaturas digitais"
	inactive := false
	updated, err := categories.Update(ctx, conn, c.ID, &newName, &inactive)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != newName || updated.IsActive {
		t.Errorf("Update() = %+v", updated)
	}

	all, err := categories.List(ctx, conn)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	// 28 seeded + 1 created.
	if len(all) != 29 {
		t.Errorf("len(List()) = %d, want 29", len(all))
	}

	if _, err := categories.Get(ctx, conn, uuid.NewString()); !errors.Is(err, categories.ErrNotFound) {
		t.Errorf("Get(unknown) error = %v, want ErrNotFound", err)
	}
}

func TestAssignManualOverridesAnyPriorDecision(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	txID := insertTransaction(t, conn, nil)

	other, err := categories.Create(ctx, conn, "Outra", money.Expense)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	d1, err := categories.AssignManual(ctx, conn, txID, seededExpenseCategory)
	if err != nil {
		t.Fatalf("AssignManual: %v", err)
	}
	if d1.CategoryID != seededExpenseCategory || d1.Revision != 1 || d1.Origin != categories.OriginManual {
		t.Errorf("first assignment = %+v", d1)
	}

	d2, err := categories.AssignManual(ctx, conn, txID, other.ID)
	if err != nil {
		t.Fatalf("AssignManual (override): %v", err)
	}
	if d2.CategoryID != other.ID || d2.Revision != 2 {
		t.Errorf("override assignment = %+v, want category=%s revision=2", d2, other.ID)
	}
}

func TestAssignManualRejectsUnknownTransactionOrCategory(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	txID := insertTransaction(t, conn, nil)

	if _, err := categories.AssignManual(ctx, conn, uuid.NewString(), seededExpenseCategory); !errors.Is(err, categories.ErrTransactionNotFound) {
		t.Errorf("unknown transaction: err = %v, want ErrTransactionNotFound", err)
	}
	if _, err := categories.AssignManual(ctx, conn, txID, uuid.NewString()); !errors.Is(err, categories.ErrCategoryInvalid) {
		t.Errorf("unknown category: err = %v, want ErrCategoryInvalid", err)
	}

	inactive := false
	custom, _ := categories.Create(ctx, conn, "Inativa", money.Expense)
	if _, err := categories.Update(ctx, conn, custom.ID, nil, &inactive); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, err := categories.AssignManual(ctx, conn, txID, custom.ID); !errors.Is(err, categories.ErrCategoryInvalid) {
		t.Errorf("inactive category: err = %v, want ErrCategoryInvalid", err)
	}
}

// installmentMetadata builds the credit_card_metadata JSON Pluggy attaches
// to one parcela of a card purchase.
func installmentMetadata(t *testing.T, cardNumber string, installmentNumber, totalInstallments int) string {
	t.Helper()
	return fmt.Sprintf(
		`{"cardNumber":%q,"installmentNumber":%d,"totalInstallments":%d}`,
		cardNumber, installmentNumber, totalInstallments,
	)
}

func TestAssignManualCascadesAcrossInstallments(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	accountID, rawImportID := newTestAccount(t, conn)

	var installments [3]string
	for i := 1; i <= 3; i++ {
		description := fmt.Sprintf("Loja X %d/3", i)
		metadata := installmentMetadata(t, "1234", i, 3)
		installments[i-1] = insertTransactionOnAccount(t, conn, accountID, rawImportID, "", &description, &metadata)
	}
	unrelatedDescription := "Outra Compra"
	unrelated := insertTransactionOnAccount(t, conn, accountID, rawImportID, "", &unrelatedDescription, nil)

	// Assigning the middle installment should cascade to every parcela of
	// the same purchase, and only to those.
	d, err := categories.AssignManual(ctx, conn, installments[1], seededExpenseCategory)
	if err != nil {
		t.Fatalf("AssignManual: %v", err)
	}
	if d.TransactionID != installments[1] || d.CategoryID != seededExpenseCategory {
		t.Errorf("returned decision = %+v", d)
	}

	for _, txID := range installments {
		got, err := getDecision(t, conn, txID)
		if err != nil {
			t.Fatalf("getDecision(%s): %v", txID, err)
		}
		if got.categoryID != seededExpenseCategory || got.origin != "manual" {
			t.Errorf("installment %s decision = %+v, want category=%s manual", txID, got, seededExpenseCategory)
		}
	}

	if _, err := getDecision(t, conn, unrelated); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("unrelated transaction got a decision, err = %v", err)
	}
}

func TestAssignManualDoesNotCrossMatchDifferentPurchases(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	accountID, rawImportID := newTestAccount(t, conn)

	descriptionA := "Loja Y 1/2"
	metadataA := installmentMetadata(t, "1111", 1, 2)
	txA := insertTransactionOnAccount(t, conn, accountID, rawImportID, "", &descriptionA, &metadataA)

	// Same base description and totalInstallments, but a different card:
	// coincidence, not the same purchase.
	descriptionB := "Loja Y 2/2"
	metadataB := installmentMetadata(t, "2222", 2, 2)
	txB := insertTransactionOnAccount(t, conn, accountID, rawImportID, "", &descriptionB, &metadataB)

	if _, err := categories.AssignManual(ctx, conn, txA, seededExpenseCategory); err != nil {
		t.Fatalf("AssignManual: %v", err)
	}

	if _, err := getDecision(t, conn, txA); err != nil {
		t.Fatalf("getDecision(txA): %v", err)
	}
	if _, err := getDecision(t, conn, txB); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("txB should not have been cascaded into, err = %v", err)
	}
}

func TestApplyAutomaticOnlyWhenUnassigned(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()

	mapped := "Groceries" // maps to categorySupermercado in mapping.go
	txID := insertTransaction(t, conn, &mapped)

	if err := categories.ApplyAutomatic(ctx, conn, txID, &mapped); err != nil {
		t.Fatalf("ApplyAutomatic: %v", err)
	}
	got, err := categories.Get(ctx, conn, categories.SourceCategoryMapping[mapped])
	if err != nil {
		t.Fatalf("Get mapped category: %v", err)
	}
	_ = got

	d, err := getDecision(t, conn, txID)
	if err != nil {
		t.Fatalf("read decision: %v", err)
	}
	if d.categoryID != categories.SourceCategoryMapping[mapped] || d.origin != "automatic" {
		t.Errorf("decision = %+v", d)
	}

	// A manual override must not be clobbered by a later automatic pass.
	other, _ := categories.Create(ctx, conn, "Manual override", money.Expense)
	if _, err := categories.AssignManual(ctx, conn, txID, other.ID); err != nil {
		t.Fatalf("AssignManual: %v", err)
	}
	if err := categories.ApplyAutomatic(ctx, conn, txID, &mapped); err != nil {
		t.Fatalf("ApplyAutomatic (should no-op): %v", err)
	}
	d2, err := getDecision(t, conn, txID)
	if err != nil {
		t.Fatalf("read decision: %v", err)
	}
	if d2.categoryID != other.ID {
		t.Errorf("automatic pass overwrote manual decision: %+v", d2)
	}
}

func TestApplyAutomaticNoOpsWithoutMapping(t *testing.T) {
	conn := newTestDB(t)
	ctx := context.Background()
	unmapped := "Some Unmapped Label"
	txID := insertTransaction(t, conn, &unmapped)

	if err := categories.ApplyAutomatic(ctx, conn, txID, &unmapped); err != nil {
		t.Fatalf("ApplyAutomatic: %v", err)
	}
	if _, err := getDecision(t, conn, txID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("expected no decision row, err = %v", err)
	}
}

type rawDecision struct {
	categoryID string
	origin     string
}

func getDecision(t *testing.T, conn *sql.DB, txID string) (rawDecision, error) {
	t.Helper()
	var d rawDecision
	err := conn.QueryRow(
		`SELECT category_id, origin FROM transaction_category_decisions WHERE transaction_id = ?`, txID,
	).Scan(&d.categoryID, &d.origin)
	return d, err
}
