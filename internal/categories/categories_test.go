package categories_test

import (
	"context"
	"database/sql"
	"errors"
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
	now := db.FormatTime(time.Now())
	sourceID := uuid.NewString()
	syncRunID := uuid.NewString()
	rawImportID := uuid.NewString()
	accountID := uuid.NewString()
	txID := uuid.NewString()

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
	exec(`INSERT INTO financial_transactions (
			id, source_id, account_id, external_id, source_category, current_raw_import_id,
			normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, 'hash', ?, ?)`,
		txID, sourceID, accountID, txID, sourceCategory, rawImportID, now, now)
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
