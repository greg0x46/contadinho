package ledger_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/greg0x46/julius/internal/automation"
	"github.com/greg0x46/julius/internal/categories"
	"github.com/greg0x46/julius/internal/db"
	"github.com/greg0x46/julius/internal/ledger"
	"github.com/greg0x46/julius/internal/money"
	"github.com/greg0x46/julius/internal/transactions"
)

type fixture struct {
	t         *testing.T
	conn      *sql.DB
	accountID string
	ignored   []string
	svc       *ledger.Service
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	f := &fixture{t: t, conn: conn}
	sourceID, runID, rawID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	now := db.FormatTime(time.Now())
	f.exec(`INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at)
		VALUES (?, 'pluggy', 'item-1', ?, ?)`, sourceID, now, now)
	f.exec(`INSERT INTO sync_runs (id, source_id, status, started_at, finished_at)
		VALUES (?, ?, 'completed', ?, ?)`, runID, sourceID, now, now)
	f.exec(`INSERT INTO raw_imports (
			id, sync_run_id, source_id, scope, page_sequence, request_attempt,
			request_method, request_path, http_status, response_headers, payload,
			payload_sha256, received_at
		) VALUES (?, ?, ?, 'transactions', 1, 1, 'GET', '/x', 200, '{}', x'00', 'sha', ?)`,
		rawID, runID, sourceID, now)
	f.accountID = uuid.NewString()
	f.exec(`INSERT INTO financial_accounts (
			id, source_id, external_id, name, institution, currency_code,
			current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, 'Conta', 'Banco', 'BRL', ?, 'hash', ?, ?)`,
		f.accountID, sourceID, f.accountID, rawID, now, now)

	f.svc = &ledger.Service{DB: conn, OnIgnored: func(_ context.Context, _ transactions.Querier, id string) error {
		f.ignored = append(f.ignored, id)
		return nil
	}}
	return f
}

func (f *fixture) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.conn.Exec(query, args...); err != nil {
		f.t.Fatalf("exec: %v", err)
	}
}

func (f *fixture) category(name string) string {
	f.t.Helper()
	c, err := categories.Create(context.Background(), f.conn, name, money.Expense, "", "")
	if err != nil {
		f.t.Fatalf("categories.Create: %v", err)
	}
	return c.ID
}

func (f *fixture) rule(write automation.Write) {
	f.t.Helper()
	if _, err := automation.Create(context.Background(), f.conn, write); err != nil {
		f.t.Fatalf("automation.Create: %v", err)
	}
}

func (f *fixture) countTransactions() int {
	f.t.Helper()
	var n int
	if err := f.conn.QueryRow(`SELECT COUNT(*) FROM financial_transactions`).Scan(&n); err != nil {
		f.t.Fatalf("count: %v", err)
	}
	return n
}

// origins returns the origin of every category event for a transaction, oldest
// first. Decisions that a later pass skips leave no event, so this history is
// what proves the order the passes ran in.
func (f *fixture) origins(id string) []string {
	f.t.Helper()
	rows, err := f.conn.Query(`SELECT origin FROM transaction_category_events WHERE transaction_id = ? ORDER BY revision`, id)
	if err != nil {
		f.t.Fatalf("origins: %v", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var o string
		if err := rows.Scan(&o); err != nil {
			f.t.Fatalf("scan: %v", err)
		}
		out = append(out, o)
	}
	return out
}

func assertOrigins(t *testing.T, got []string, want ...string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("category event origins = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("category event origins = %v, want %v", got, want)
		}
	}
}

func (f *fixture) input(description string) transactions.ManualInput {
	return transactions.ManualInput{
		AccountID:   f.accountID,
		Description: description,
		Amount:      decimal.RequireFromString("-30.00"),
		OccurredAt:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.Local),
	}
}

func containsRule(description string, actions ...automation.ActionWrite) automation.Write {
	return automation.Write{
		Name: "rule " + description, IsActive: true, LogicOperator: automation.LogicOr,
		Conditions: []automation.Condition{
			{Field: automation.FieldDescription, Operator: automation.OperatorContains, Value: description},
		},
		Actions: actions,
	}
}

func strPtr(s string) *string { return &s }

func TestCreateManualAppliesLearnedCategoryFromPastManualDecision(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mercado := f.category("Mercado")

	if _, err := f.svc.CreateManual(ctx, f.input("Padaria do Zé"), &mercado); err != nil {
		t.Fatalf("first CreateManual: %v", err)
	}
	item, err := f.svc.CreateManual(ctx, f.input("Padaria do Zé"), nil)
	if err != nil {
		t.Fatalf("second CreateManual: %v", err)
	}
	if item.InternalCategory == nil || item.InternalCategory.ID != mercado {
		t.Fatalf("InternalCategory = %+v, want learned %s", item.InternalCategory, mercado)
	}
	if item.InternalCategory.Origin != "learned" {
		t.Errorf("Origin = %q, want learned", item.InternalCategory.Origin)
	}
}

func TestCreateManualRuleOverridesLearnedCategory(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mercado, lazer := f.category("Mercado"), f.category("Lazer")

	if _, err := f.svc.CreateManual(ctx, f.input("Cinema Centro"), &mercado); err != nil {
		t.Fatalf("seed CreateManual: %v", err)
	}
	f.rule(containsRule("cinema", automation.ActionWrite{Type: automation.ActionSetCategory, CategoryID: &lazer}))

	item, err := f.svc.CreateManual(ctx, f.input("Cinema Centro"), nil)
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}
	if item.InternalCategory == nil || item.InternalCategory.ID != lazer {
		t.Fatalf("InternalCategory = %+v, want rule category %s", item.InternalCategory, lazer)
	}
	// learned must run before the rule: if the rule went first, the learned
	// pass would be skipped and leave no event.
	assertOrigins(t, f.origins(item.ID), "learned", "rule")
}

func TestCreateManualExplicitCategorySkipsLearnedPass(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mercado, saude := f.category("Mercado"), f.category("Saúde")

	if _, err := f.svc.CreateManual(ctx, f.input("Pilates"), &mercado); err != nil {
		t.Fatalf("seed CreateManual: %v", err)
	}
	item, err := f.svc.CreateManual(ctx, f.input("Pilates"), &saude)
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}
	assertOrigins(t, f.origins(item.ID), "manual")
}

func TestCreateManualExplicitCategoryBeatsRule(t *testing.T) {
	f := newFixture(t)
	lazer, saude := f.category("Lazer"), f.category("Saúde")
	f.rule(containsRule("academia", automation.ActionWrite{Type: automation.ActionSetCategory, CategoryID: &lazer}))

	item, err := f.svc.CreateManual(context.Background(), f.input("Academia mensal"), &saude)
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}
	if item.InternalCategory == nil || item.InternalCategory.ID != saude {
		t.Fatalf("InternalCategory = %+v, want explicit %s", item.InternalCategory, saude)
	}
	if item.InternalCategory.Origin != "manual" {
		t.Errorf("Origin = %q, want manual", item.InternalCategory.Origin)
	}
	// the rule must run first and be overridden: if the explicit category went
	// first, the rule would skip the manual decision and leave no event.
	assertOrigins(t, f.origins(item.ID), "rule", "manual")
}

func TestCreateManualRuleWithInactiveCategoryFailsWithoutBlamingInput(t *testing.T) {
	f := newFixture(t)
	lazer := f.category("Lazer")
	f.rule(containsRule("streaming", automation.ActionWrite{Type: automation.ActionSetCategory, CategoryID: &lazer}))
	inactive := false
	if _, err := categories.Update(context.Background(), f.conn, lazer, nil, &inactive, nil, nil); err != nil {
		t.Fatalf("deactivate category: %v", err)
	}

	_, err := f.svc.CreateManual(context.Background(), f.input("Streaming mensal"), nil)
	if !errors.Is(err, categories.ErrCategoryInvalid) {
		t.Fatalf("err = %v, want ErrCategoryInvalid from the rule pass", err)
	}
	if n := f.countTransactions(); n != 0 {
		t.Errorf("transactions persisted = %d, want 0 after rollback", n)
	}
}

func TestCreateManualInvalidCategoryRollsBack(t *testing.T) {
	f := newFixture(t)

	_, err := f.svc.CreateManual(context.Background(), f.input("Qualquer"), strPtr(uuid.NewString()))
	if !errors.Is(err, categories.ErrCategoryInvalid) {
		t.Fatalf("err = %v, want ErrCategoryInvalid", err)
	}
	if n := f.countTransactions(); n != 0 {
		t.Errorf("transactions persisted = %d, want 0 after rollback", n)
	}
}

func TestCreateManualMissingAccount(t *testing.T) {
	f := newFixture(t)
	in := f.input("Qualquer")
	in.AccountID = uuid.NewString()

	_, err := f.svc.CreateManual(context.Background(), in, nil)
	if !errors.Is(err, transactions.ErrAccountNotFound) {
		t.Fatalf("err = %v, want ErrAccountNotFound", err)
	}
}

func TestCreateManualIgnoreRuleRunsOnIgnoredHook(t *testing.T) {
	f := newFixture(t)
	f.rule(containsRule("transferencia", automation.ActionWrite{Type: automation.ActionIgnore}))

	item, err := f.svc.CreateManual(context.Background(), f.input("Transferencia propria"), nil)
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}
	if item.Inclusion.State != money.Ignored {
		t.Errorf("Inclusion.State = %q, want ignored", item.Inclusion.State)
	}
	if len(f.ignored) != 1 || f.ignored[0] != item.ID {
		t.Errorf("OnIgnored calls = %v, want [%s]", f.ignored, item.ID)
	}
}

func TestUpdateManualAssignsCategory(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	mercado := f.category("Mercado")

	created, err := f.svc.CreateManual(ctx, f.input("Feira"), nil)
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}
	in := f.input("Feira livre")
	updated, err := f.svc.UpdateManual(ctx, created.ID, in, &mercado)
	if err != nil {
		t.Fatalf("UpdateManual: %v", err)
	}
	if updated.Description == nil || *updated.Description != "Feira livre" {
		t.Errorf("Description = %v, want Feira livre", updated.Description)
	}
	if updated.InternalCategory == nil || updated.InternalCategory.ID != mercado {
		t.Errorf("InternalCategory = %+v, want %s", updated.InternalCategory, mercado)
	}
}

func TestUpdateManualInvalidCategoryRollsBackFieldChanges(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	created, err := f.svc.CreateManual(ctx, f.input("Feira"), nil)
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}
	_, err = f.svc.UpdateManual(ctx, created.ID, f.input("Renomeada"), strPtr(uuid.NewString()))
	if !errors.Is(err, categories.ErrCategoryInvalid) {
		t.Fatalf("err = %v, want ErrCategoryInvalid", err)
	}
	item, found, err := transactions.GetItem(ctx, f.conn, created.ID)
	if err != nil || !found {
		t.Fatalf("GetItem: found=%v err=%v", found, err)
	}
	if item.Description == nil || *item.Description != "Feira" {
		t.Errorf("Description = %v, want unchanged Feira", item.Description)
	}
}

func TestUpdateAndDeleteRejectSyncedTransaction(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	var sourceID, rawID string
	if err := f.conn.QueryRow(`SELECT source_id, current_raw_import_id FROM financial_accounts WHERE id = ?`, f.accountID).Scan(&sourceID, &rawID); err != nil {
		t.Fatalf("lookup: %v", err)
	}
	syncedID := uuid.NewString()
	now := db.FormatTime(time.Now())
	f.exec(`INSERT INTO financial_transactions (
			id, source_id, account_id, external_id, description,
			provider_status, movement_type, current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, 'Sincronizada', 'POSTED', 'DEBIT', ?, 'hash', ?, ?)`,
		syncedID, sourceID, f.accountID, syncedID, rawID, now, now)

	if _, err := f.svc.UpdateManual(ctx, syncedID, f.input("x"), nil); !errors.Is(err, transactions.ErrNotManual) {
		t.Errorf("UpdateManual err = %v, want ErrNotManual", err)
	}
	if err := f.svc.DeleteManual(ctx, syncedID); !errors.Is(err, transactions.ErrNotManual) {
		t.Errorf("DeleteManual err = %v, want ErrNotManual", err)
	}
}

func TestUpdateAndDeleteUnknownTransaction(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if _, err := f.svc.UpdateManual(ctx, uuid.NewString(), f.input("x"), nil); !errors.Is(err, transactions.ErrTransactionNotFound) {
		t.Errorf("UpdateManual err = %v, want ErrTransactionNotFound", err)
	}
	if err := f.svc.DeleteManual(ctx, uuid.NewString()); !errors.Is(err, transactions.ErrTransactionNotFound) {
		t.Errorf("DeleteManual err = %v, want ErrTransactionNotFound", err)
	}
}

func TestDeleteManualRemovesTheRow(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	created, err := f.svc.CreateManual(ctx, f.input("Descartável"), nil)
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}
	if err := f.svc.DeleteManual(ctx, created.ID); err != nil {
		t.Fatalf("DeleteManual: %v", err)
	}
	if _, found, err := transactions.GetItem(ctx, f.conn, created.ID); err != nil || found {
		t.Errorf("GetItem after delete: found=%v err=%v, want not found", found, err)
	}
}
