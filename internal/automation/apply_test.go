package automation_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/automation"
	"contadinho-go/internal/db"
	"contadinho-go/internal/money"
	"contadinho-go/internal/transactions"
)

// fixture mirrors the minimal sync-schema chain other packages' tests use,
// plus a helper to add accounts/transactions with the fields matching cares
// about (description, credit_card_metadata, account name/institution).
type fixture struct {
	t           *testing.T
	conn        *sql.DB
	sourceID    string
	rawImportID string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	conn := newTestDB(t)
	f := &fixture{t: t, conn: conn, sourceID: uuid.NewString(), rawImportID: uuid.NewString()}
	now := db.FormatTime(time.Now())
	f.exec(`INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at)
		VALUES (?, 'pluggy', 'item-1', ?, ?)`, f.sourceID, now, now)
	syncRunID := uuid.NewString()
	f.exec(`INSERT INTO sync_runs (id, source_id, status, started_at, finished_at)
		VALUES (?, ?, 'completed', ?, ?)`, syncRunID, f.sourceID, now, now)
	f.exec(`INSERT INTO raw_imports (
			id, sync_run_id, source_id, scope, page_sequence, request_attempt,
			request_method, request_path, http_status, response_headers, payload,
			payload_sha256, received_at
		) VALUES (?, ?, ?, 'transactions', 1, 1, 'GET', '/x', 200, '{}', x'00', 'sha', ?)`,
		f.rawImportID, syncRunID, f.sourceID, now)
	return f
}

func (f *fixture) exec(query string, args ...any) {
	f.t.Helper()
	if _, err := f.conn.Exec(query, args...); err != nil {
		f.t.Fatalf("exec %q: %v", query, err)
	}
}

func (f *fixture) addAccount(name, institution string) string {
	f.t.Helper()
	id := uuid.NewString()
	now := db.FormatTime(time.Now())
	f.exec(`INSERT INTO financial_accounts (
			id, source_id, external_id, name, institution, current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, 'hash', ?, ?)`, id, f.sourceID, id, name, institution, f.rawImportID, now, now)
	return id
}

func (f *fixture) addTransaction(accountID, description, creditCardMetadata string) string {
	f.t.Helper()
	id := uuid.NewString()
	now := db.FormatTime(time.Now())
	var metadata any
	if creditCardMetadata != "" {
		metadata = creditCardMetadata
	}
	f.exec(`INSERT INTO financial_transactions (
			id, source_id, account_id, external_id, description, credit_card_metadata,
			provider_status, movement_type, current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, 'POSTED', 'DEBIT', ?, 'hash', ?, ?)`,
		id, f.sourceID, accountID, id, description, metadata, f.rawImportID, now, now)
	return id
}

func TestApplyToNewTransactionIgnoresOnFirstMatchingRule(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("Conta Corrente", "Banco Exemplo")
	txID := f.addTransaction(acc, "Transferencia enviada", "")

	if _, err := automation.Create(ctx, f.conn, automation.Write{
		Name: "Ignorar transferências", IsActive: true, LogicOperator: automation.LogicAnd,
		Conditions: []automation.Condition{{Field: automation.FieldDescription, Operator: automation.OperatorContains, Value: "transferencia"}},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := automation.ApplyToNewTransaction(ctx, f.conn, txID, nil); err != nil {
		t.Fatalf("ApplyToNewTransaction: %v", err)
	}

	var state, origin string
	if err := f.conn.QueryRow(`SELECT state, origin FROM transaction_inclusion_decisions WHERE transaction_id = ?`, txID).Scan(&state, &origin); err != nil {
		t.Fatalf("query decision: %v", err)
	}
	if state != string(money.Ignored) || origin != string(transactions.InclusionOriginRule) {
		t.Errorf("state=%s origin=%s", state, origin)
	}
}

func TestApplyToNewTransactionNoRuleMatchesLeavesConsidered(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("Conta Corrente", "Banco Exemplo")
	txID := f.addTransaction(acc, "Mercado XYZ", "")

	if _, err := automation.Create(ctx, f.conn, automation.Write{
		Name: "Ignorar transferências", IsActive: true, LogicOperator: automation.LogicAnd,
		Conditions: []automation.Condition{{Field: automation.FieldDescription, Operator: automation.OperatorContains, Value: "transferencia"}},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := automation.ApplyToNewTransaction(ctx, f.conn, txID, nil); err != nil {
		t.Fatalf("ApplyToNewTransaction: %v", err)
	}

	var count int
	f.conn.QueryRow(`SELECT COUNT(*) FROM transaction_inclusion_decisions WHERE transaction_id = ?`, txID).Scan(&count)
	if count != 0 {
		t.Errorf("expected no inclusion decision when no rule matches, got %d", count)
	}
}

func TestApplyToNewTransactionMatchesCardMetadata(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("Cartão", "Banco Exemplo")
	txID := f.addTransaction(acc, "Compra qualquer", `{"cardNumber":"**** 4321"}`)

	if _, err := automation.Create(ctx, f.conn, automation.Write{
		Name: "Ignorar cartão final 4321", IsActive: true, LogicOperator: automation.LogicAnd,
		Conditions: []automation.Condition{{Field: automation.FieldCard, Operator: automation.OperatorContains, Value: "4321"}},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := automation.ApplyToNewTransaction(ctx, f.conn, txID, nil); err != nil {
		t.Fatalf("ApplyToNewTransaction: %v", err)
	}

	var state string
	if err := f.conn.QueryRow(`SELECT state FROM transaction_inclusion_decisions WHERE transaction_id = ?`, txID).Scan(&state); err != nil {
		t.Fatalf("query decision: %v", err)
	}
	if state != string(money.Ignored) {
		t.Errorf("state = %s, want ignored", state)
	}
}

func TestApplyToNewTransactionSkipsWhenNoActiveRules(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("Conta", "Banco")
	txID := f.addTransaction(acc, "Qualquer coisa", "")

	if err := automation.ApplyToNewTransaction(ctx, f.conn, txID, nil); err != nil {
		t.Fatalf("ApplyToNewTransaction: %v", err)
	}
	var count int
	f.conn.QueryRow(`SELECT COUNT(*) FROM transaction_inclusion_decisions WHERE transaction_id = ?`, txID).Scan(&count)
	if count != 0 {
		t.Errorf("count = %d, want 0", count)
	}
}

func TestManualDecisionBlocksLaterRuleApplication(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("Conta", "Banco")
	txID := f.addTransaction(acc, "Transferencia enviada", "")

	// Setting state to "considered" with no prior decision is a no-op (it's
	// already the implicit default, so nothing is persisted) — a real
	// manual-origin "considered" row only exists once the user has toggled
	// it away from and back to considered. That's what actually protects a
	// transaction from a later rule, so recreate that sequence here.
	if _, err := transactions.SetInclusion(ctx, f.conn, txID, money.Ignored, transactions.InclusionOriginManual, nil, nil, nil); err != nil {
		t.Fatalf("SetInclusion (ignore): %v", err)
	}
	if _, err := transactions.SetInclusion(ctx, f.conn, txID, money.Considered, transactions.InclusionOriginManual, nil, nil, nil); err != nil {
		t.Fatalf("SetInclusion (un-ignore): %v", err)
	}

	if _, err := automation.Create(ctx, f.conn, automation.Write{
		Name: "Ignorar transferências", IsActive: true, LogicOperator: automation.LogicAnd,
		Conditions: []automation.Condition{{Field: automation.FieldDescription, Operator: automation.OperatorContains, Value: "transferencia"}},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := automation.ApplyToNewTransaction(ctx, f.conn, txID, nil); err != nil {
		t.Fatalf("ApplyToNewTransaction: %v", err)
	}

	var state, origin string
	f.conn.QueryRow(`SELECT state, origin FROM transaction_inclusion_decisions WHERE transaction_id = ?`, txID).Scan(&state, &origin)
	if state != string(money.Considered) || origin != string(transactions.InclusionOriginManual) {
		t.Errorf("state=%s origin=%s, want considered/manual (unchanged)", state, origin)
	}
}

func TestApplyRetroactivelyCountsMatchedAndActuallyChanged(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("Conta", "Banco")
	tx1 := f.addTransaction(acc, "Transferencia 1", "")
	tx2 := f.addTransaction(acc, "Transferencia 2", "")
	tx3 := f.addTransaction(acc, "Mercado", "")

	// tx1 is already manually protected (see the comment in
	// TestManualDecisionBlocksLaterRuleApplication for why a bare "set to
	// considered" wouldn't actually persist a manual decision): it should
	// count as matched but not as newly ignored, since the manual decision
	// blocks the rule.
	if _, err := transactions.SetInclusion(ctx, f.conn, tx1, money.Ignored, transactions.InclusionOriginManual, nil, nil, nil); err != nil {
		t.Fatalf("SetInclusion (ignore): %v", err)
	}
	if _, err := transactions.SetInclusion(ctx, f.conn, tx1, money.Considered, transactions.InclusionOriginManual, nil, nil, nil); err != nil {
		t.Fatalf("SetInclusion (un-ignore): %v", err)
	}

	rule, err := automation.Create(ctx, f.conn, automation.Write{
		Name: "Ignorar transferências", IsActive: false, LogicOperator: automation.LogicAnd,
		Conditions: []automation.Condition{{Field: automation.FieldDescription, Operator: automation.OperatorContains, Value: "transferencia"}},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	result, err := automation.ApplyRetroactively(ctx, f.conn, rule.ID, nil)
	if err != nil {
		t.Fatalf("ApplyRetroactively: %v", err)
	}
	if result.Matched != 2 {
		t.Errorf("Matched = %d, want 2 (tx1 and tx2)", result.Matched)
	}
	if result.Ignored != 1 {
		t.Errorf("Ignored = %d, want 1 (only tx2 actually changed)", result.Ignored)
	}

	var tx2State string
	f.conn.QueryRow(`SELECT state FROM transaction_inclusion_decisions WHERE transaction_id = ?`, tx2).Scan(&tx2State)
	if tx2State != string(money.Ignored) {
		t.Errorf("tx2 state = %s, want ignored", tx2State)
	}
	var tx3Count int
	f.conn.QueryRow(`SELECT COUNT(*) FROM transaction_inclusion_decisions WHERE transaction_id = ?`, tx3).Scan(&tx3Count)
	if tx3Count != 0 {
		t.Errorf("tx3 (non-matching) should have no decision, got %d", tx3Count)
	}
}

func TestApplyRetroactivelyUnknownRuleIsANoOp(t *testing.T) {
	f := newFixture(t)
	result, err := automation.ApplyRetroactively(context.Background(), f.conn, "unknown", nil)
	if err != nil {
		t.Fatalf("ApplyRetroactively: %v", err)
	}
	if result.Matched != 0 || result.Ignored != 0 {
		t.Errorf("result = %+v, want zero", result)
	}
}

func TestNewTransactionHookMatchesSyncNewTransactionHookSignature(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("Conta", "Banco")
	txID := f.addTransaction(acc, "Transferencia enviada", "")

	if _, err := automation.Create(ctx, f.conn, automation.Write{
		Name: "Ignorar transferências", IsActive: true, LogicOperator: automation.LogicAnd,
		Conditions: []automation.Condition{{Field: automation.FieldDescription, Operator: automation.OperatorContains, Value: "transferencia"}},
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	var hookCalled bool
	onIgnored := func(ctx context.Context, q transactions.Querier, transactionID string) error {
		hookCalled = true
		return nil
	}
	hook := automation.NewTransactionHook(onIgnored)
	if err := hook(ctx, f.conn, txID, acc); err != nil {
		t.Fatalf("hook: %v", err)
	}
	if !hookCalled {
		t.Error("onIgnored hook should have been invoked when a rule ignores the transaction")
	}
}
