package payables_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"contadinho-go/internal/db"
	"contadinho-go/internal/money"
	"contadinho-go/internal/payables"
	"contadinho-go/internal/scenarios"
	"contadinho-go/internal/transactions"
)

func dec(t *testing.T, s string) decimal.Decimal {
	t.Helper()
	d, err := decimal.NewFromString(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

type fixture struct {
	t           *testing.T
	conn        *sql.DB
	sourceID    string
	rawImportID string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

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

func (f *fixture) addAccount(currencyCode string) string {
	f.t.Helper()
	id := uuid.NewString()
	now := db.FormatTime(time.Now())
	f.exec(`INSERT INTO financial_accounts (
			id, source_id, external_id, currency_code, current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, 'hash', ?, ?)`, id, f.sourceID, id, currencyCode, f.rawImportID, now, now)
	return id
}

type txSpec struct {
	AccountID      string
	Description    string
	Amount         string
	CurrencyCode   string
	MovementType   string
	ProviderStatus string
	OccurredAt     *time.Time
	// NoAccountCurrencyAmount skips amount_in_account_currency so
	// select_effective_money falls back to the (Amount, CurrencyCode) pair
	// instead of preferring the account's own currency — needed to actually
	// exercise a non-BRL effective-money transaction when the account
	// itself is BRL.
	NoAccountCurrencyAmount bool
}

func (f *fixture) addTransaction(spec txSpec) string {
	f.t.Helper()
	id := uuid.NewString()
	now := db.FormatTime(time.Now())
	var occurredAt any
	if spec.OccurredAt != nil {
		occurredAt = db.FormatTime(*spec.OccurredAt)
	}
	var amountInAccountCurrency any = spec.Amount
	if spec.NoAccountCurrencyAmount {
		amountInAccountCurrency = nil
	}
	f.exec(`INSERT INTO financial_transactions (
			id, source_id, account_id, external_id, description, amount, amount_in_account_currency,
			currency_code, occurred_at, provider_status, movement_type, current_raw_import_id,
			normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'hash', ?, ?)`,
		id, f.sourceID, spec.AccountID, id, spec.Description, spec.Amount, amountInAccountCurrency,
		spec.CurrencyCode, occurredAt, spec.ProviderStatus, spec.MovementType, f.rawImportID, now, now)
	return id
}

// settlingSpec returns a txSpec for a transaction that moves money in the
// direction kind requires (outflow for a debt, inflow for a receivable).
func settlingSpec(acc string, amount string) map[payables.Kind]txSpec {
	return map[payables.Kind]txSpec{
		payables.KindDebt:       {AccountID: acc, Description: "Pagamento", Amount: "-" + amount, CurrencyCode: "BRL", MovementType: "DEBIT", ProviderStatus: "POSTED"},
		payables.KindReceivable: {AccountID: acc, Description: "Recebimento", Amount: amount, CurrencyCode: "BRL", MovementType: "CREDIT", ProviderStatus: "POSTED"},
	}
}

var bothKinds = []payables.Kind{payables.KindDebt, payables.KindReceivable}

func TestCreateGetUpdateDeletePayable(t *testing.T) {
	for _, kind := range bothKinds {
		t.Run(string(kind), func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()

			p, err := payables.Create(ctx, f.conn, kind, "Cartão de crédito", dec(t, "1000.00"), dec(t, "600.00"))
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if p.Kind != kind {
				t.Errorf("Kind = %s, want %s", p.Kind, kind)
			}
			if !p.StartingSettledAmount.Equal(dec(t, "400.00")) {
				t.Errorf("StartingSettledAmount = %s, want 400.00 (total - initial remaining)", p.StartingSettledAmount)
			}

			got, err := payables.Get(ctx, f.conn, p.ID)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if got.Name != p.Name || got.Kind != kind {
				t.Errorf("Get() = %+v", got)
			}

			updated, err := payables.Update(ctx, f.conn, p.ID, "Novo nome", dec(t, "2000.00"))
			if err != nil {
				t.Fatalf("Update: %v", err)
			}
			if updated.Name != "Novo nome" || !updated.TotalAmount.Equal(dec(t, "2000.00")) {
				t.Errorf("Update() = %+v", updated)
			}
			if updated.Kind != kind {
				t.Errorf("Kind changed by Update: %s", updated.Kind)
			}
			// starting_settled_amount is never edited by Update.
			if !updated.StartingSettledAmount.Equal(dec(t, "400.00")) {
				t.Errorf("StartingSettledAmount changed by Update: %s", updated.StartingSettledAmount)
			}

			if err := payables.Delete(ctx, f.conn, p.ID); err != nil {
				t.Fatalf("Delete: %v", err)
			}
			if _, err := payables.Get(ctx, f.conn, p.ID); !errors.Is(err, payables.ErrNotFound) {
				t.Errorf("Get after delete: err = %v, want ErrNotFound", err)
			}
		})
	}
}

func TestUpdateDeleteUnknownPayable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := payables.Update(ctx, f.conn, "unknown", "x", dec(t, "1.00")); !errors.Is(err, payables.ErrNotFound) {
		t.Errorf("Update: err = %v, want ErrNotFound", err)
	}
	if err := payables.Delete(ctx, f.conn, "unknown"); !errors.Is(err, payables.ErrNotFound) {
		t.Errorf("Delete: err = %v, want ErrNotFound", err)
	}
}

func TestListFiltersByKind(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	debt, _ := payables.Create(ctx, f.conn, payables.KindDebt, "Dívida", dec(t, "100.00"), dec(t, "100.00"))
	rec, _ := payables.Create(ctx, f.conn, payables.KindReceivable, "Recebível", dec(t, "100.00"), dec(t, "100.00"))

	all, err := payables.List(ctx, f.conn, nil)
	if err != nil {
		t.Fatalf("List(nil): %v", err)
	}
	if len(all) != 2 {
		t.Errorf("List(nil) len = %d, want 2", len(all))
	}

	debtKind := payables.KindDebt
	debtsOnly, err := payables.List(ctx, f.conn, &debtKind)
	if err != nil {
		t.Fatalf("List(debt): %v", err)
	}
	if len(debtsOnly) != 1 || debtsOnly[0].ID != debt.ID {
		t.Errorf("List(debt) = %+v, want just %s", debtsOnly, debt.ID)
	}

	recKind := payables.KindReceivable
	recsOnly, err := payables.List(ctx, f.conn, &recKind)
	if err != nil {
		t.Fatalf("List(receivable): %v", err)
	}
	if len(recsOnly) != 1 || recsOnly[0].ID != rec.ID {
		t.Errorf("List(receivable) = %+v, want just %s", recsOnly, rec.ID)
	}
}

func TestCreateLinkEligibleTransaction(t *testing.T) {
	for _, kind := range bothKinds {
		t.Run(string(kind), func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			acc := f.addAccount("BRL")
			txID := f.addTransaction(settlingSpec(acc, "500.00")[kind])
			p, _ := payables.Create(ctx, f.conn, kind, "Pendência", dec(t, "1000.00"), dec(t, "1000.00"))

			result, err := payables.CreateLink(ctx, f.conn, kind, p.ID, txID)
			if err != nil {
				t.Fatalf("CreateLink: %v", err)
			}
			if result.Status != payables.StatusCreated || result.Link == nil {
				t.Fatalf("result = %+v", result)
			}
			if !result.Link.LinkedAmount.Equal(dec(t, "500.00")) {
				t.Errorf("LinkedAmount = %s, want 500.00 (abs value)", result.Link.LinkedAmount)
			}

			updated, err := payables.Get(ctx, f.conn, p.ID)
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			summary, err := payables.Summarize(ctx, f.conn, updated)
			if err != nil {
				t.Fatalf("Summarize: %v", err)
			}
			if len(summary.Links) != 1 {
				t.Fatalf("len(summary.Links) = %d, want 1", len(summary.Links))
			}
			if !summary.Links[0].EffectiveAmount.Equal(dec(t, "500.00")) {
				t.Errorf("EffectiveAmount = %s, want 500.00", summary.Links[0].EffectiveAmount)
			}
			if !summary.Settled.Equal(dec(t, "500.00")) {
				t.Errorf("Settled = %s, want 500.00", summary.Settled)
			}
		})
	}
}

func TestSettledCountsOneRealTransactionAcrossMultiplePlanAllocations(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.addAccount("BRL")
	txID := f.addTransaction(txSpec{
		AccountID: account, Description: "Pagamento conjunto", Amount: "-100.00",
		CurrencyCode: "BRL", MovementType: "DEBIT", ProviderStatus: "POSTED",
	})
	payable, err := payables.Create(ctx, f.conn, payables.KindDebt, "Dívida", dec(t, "1000.00"), dec(t, "1000.00"))
	if err != nil {
		t.Fatalf("payables.Create: %v", err)
	}
	primary, err := scenarios.CreateScenario(ctx, f.conn, scenarios.KindDebtPlan, "Plano contábil", &payable.ID)
	if err != nil {
		t.Fatalf("Create primary scenario: %v", err)
	}
	if !primary.IsAccountingSource {
		t.Fatal("first payable plan must be the accounting source")
	}
	simulation, err := scenarios.CreateScenario(ctx, f.conn, scenarios.KindDebtPlan, "Simulação", &payable.ID)
	if err != nil {
		t.Fatalf("Create simulation scenario: %v", err)
	}
	if simulation.IsAccountingSource {
		t.Fatal("second payable plan must not become the accounting source")
	}
	link, err := payables.CreateLink(ctx, f.conn, payables.KindDebt, payable.ID, txID)
	if err != nil || link.Link == nil {
		t.Fatalf("CreateLink: result=%+v err=%v", link, err)
	}
	first, err := scenarios.CreateScenarioTransaction(ctx, f.conn, primary.ID, "Parcela A", dec(t, "60.00"), time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatalf("Create first installment: %v", err)
	}
	second, err := scenarios.CreateScenarioTransaction(ctx, f.conn, simulation.ID, "Parcela B", dec(t, "40.00"), time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatalf("Create second installment: %v", err)
	}
	if _, err := scenarios.CreateRealization(ctx, f.conn, first.ID, &link.Link.ID, dec(t, "60.00")); err != nil {
		t.Fatalf("allocate first installment: %v", err)
	}
	if _, err := scenarios.CreateRealization(ctx, f.conn, second.ID, &link.Link.ID, dec(t, "40.00")); err != nil {
		t.Fatalf("allocate second installment: %v", err)
	}

	current, err := payables.Summarize(ctx, f.conn, payable)
	if err != nil {
		t.Fatalf("Summarize: %v", err)
	}
	if !current.Settled.Equal(dec(t, "100.00")) || !current.Remaining.Equal(dec(t, "900.00")) {
		t.Fatalf("summary after split allocation = %+v, want settled 100 and remaining 900", current)
	}

	if _, err := scenarios.SetActive(ctx, f.conn, primary.ID, false); err != nil {
		t.Fatalf("deactivate accounting scenario: %v", err)
	}
	afterDeactivation, err := payables.Summarize(ctx, f.conn, payable)
	if err != nil {
		t.Fatalf("Summarize after deactivation: %v", err)
	}
	if !afterDeactivation.Settled.Equal(current.Settled) || !afterDeactivation.Remaining.Equal(current.Remaining) {
		t.Fatalf("deactivation changed payable summary: before=%+v after=%+v", current, afterDeactivation)
	}
}

func TestCreateLinkRejectsIneligibleTransaction(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("BRL")

	inflowTx := f.addTransaction(txSpec{AccountID: acc, Description: "Salario", Amount: "1000.00", CurrencyCode: "BRL", MovementType: "CREDIT", ProviderStatus: "POSTED"})
	debt, _ := payables.Create(ctx, f.conn, payables.KindDebt, "Cartão", dec(t, "1000.00"), dec(t, "1000.00"))
	result, err := payables.CreateLink(ctx, f.conn, payables.KindDebt, debt.ID, inflowTx)
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if result.Status != payables.StatusIneligible || result.Reason == nil || *result.Reason != payables.ReasonNotOutflow {
		t.Errorf("result = %+v, want ineligible/not_outflow", result)
	}

	outflowTx := f.addTransaction(txSpec{AccountID: acc, Description: "Compra", Amount: "-1000.00", CurrencyCode: "BRL", MovementType: "DEBIT", ProviderStatus: "POSTED"})
	rec, _ := payables.Create(ctx, f.conn, payables.KindReceivable, "Empréstimo", dec(t, "1000.00"), dec(t, "1000.00"))
	result, err = payables.CreateLink(ctx, f.conn, payables.KindReceivable, rec.ID, outflowTx)
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if result.Status != payables.StatusIneligible || result.Reason == nil || *result.Reason != payables.ReasonNotInflow {
		t.Errorf("result = %+v, want ineligible/not_inflow", result)
	}
}

func TestCreateLinkConflictsOnAlreadyLinked(t *testing.T) {
	for _, kind := range bothKinds {
		t.Run(string(kind), func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			acc := f.addAccount("BRL")
			txID := f.addTransaction(settlingSpec(acc, "100.00")[kind])
			p1, _ := payables.Create(ctx, f.conn, kind, "Pendência 1", dec(t, "1000.00"), dec(t, "1000.00"))
			p2, _ := payables.Create(ctx, f.conn, kind, "Pendência 2", dec(t, "1000.00"), dec(t, "1000.00"))

			if _, err := payables.CreateLink(ctx, f.conn, kind, p1.ID, txID); err != nil {
				t.Fatalf("first CreateLink: %v", err)
			}
			result, err := payables.CreateLink(ctx, f.conn, kind, p2.ID, txID)
			if err != nil {
				t.Fatalf("second CreateLink: %v", err)
			}
			if result.Status != payables.StatusConflict {
				t.Errorf("status = %s, want conflict", result.Status)
			}
		})
	}
}

// TestPayableTransactionLinksUniqueAcrossKinds is new relative to the
// pre-merge debts/receivables packages: with a single
// payable_transaction_links table (UNIQUE on transaction_id), a transaction
// can never be linked to a debt AND a receivable at once, tightening what
// were previously two independent per-table UNIQUE constraints (debt_id,
// transaction_id) and (receivable_id, transaction_id) into one global
// constraint. Exercised directly at the schema level since Go-level
// eligibility can't reach this path (a transaction's classification is
// mutually exclusive between outflow and inflow, so it can only ever be
// EligibilityForLink-eligible for one kind to begin with).
func TestPayableTransactionLinksUniqueAcrossKinds(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("BRL")
	txID := f.addTransaction(txSpec{AccountID: acc, Amount: "-100.00", CurrencyCode: "BRL", MovementType: "DEBIT", ProviderStatus: "POSTED"})
	debt, _ := payables.Create(ctx, f.conn, payables.KindDebt, "Dívida", dec(t, "1000.00"), dec(t, "1000.00"))
	rec, _ := payables.Create(ctx, f.conn, payables.KindReceivable, "Recebível", dec(t, "1000.00"), dec(t, "1000.00"))

	if _, err := payables.CreateLink(ctx, f.conn, payables.KindDebt, debt.ID, txID); err != nil {
		t.Fatalf("CreateLink to debt: %v", err)
	}
	now := db.FormatTime(time.Now())
	_, err := f.conn.ExecContext(ctx, `INSERT INTO payable_transaction_links (id, payable_id, transaction_id, linked_amount, linked_at) VALUES (?, ?, ?, '100.00', ?)`,
		uuid.NewString(), rec.ID, txID, now)
	if err == nil {
		t.Error("expected UNIQUE violation linking an already-linked transaction to a second payable")
	}
}

func TestCreateLinkUnknownPayableOrTransaction(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("BRL")
	txID := f.addTransaction(txSpec{AccountID: acc, Amount: "-100.00", CurrencyCode: "BRL", MovementType: "DEBIT", ProviderStatus: "POSTED"})
	p, _ := payables.Create(ctx, f.conn, payables.KindDebt, "Dívida", dec(t, "1000.00"), dec(t, "1000.00"))

	result, err := payables.CreateLink(ctx, f.conn, payables.KindDebt, "unknown-payable", txID)
	if err != nil || result.Status != payables.StatusPayableNotFound {
		t.Errorf("result=%+v err=%v, want payable_not_found", result, err)
	}
	result, err = payables.CreateLink(ctx, f.conn, payables.KindDebt, p.ID, "unknown-tx")
	if err != nil || result.Status != payables.StatusTransactionNotFound {
		t.Errorf("result=%+v err=%v, want transaction_not_found", result, err)
	}
}

func TestDeleteLinkScopedToPayable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("BRL")
	txID := f.addTransaction(txSpec{AccountID: acc, Amount: "-100.00", CurrencyCode: "BRL", MovementType: "DEBIT", ProviderStatus: "POSTED"})
	p1, _ := payables.Create(ctx, f.conn, payables.KindDebt, "Dívida 1", dec(t, "1000.00"), dec(t, "1000.00"))
	p2, _ := payables.Create(ctx, f.conn, payables.KindDebt, "Dívida 2", dec(t, "1000.00"), dec(t, "1000.00"))
	result, _ := payables.CreateLink(ctx, f.conn, payables.KindDebt, p1.ID, txID)

	found, err := payables.DeleteLink(ctx, f.conn, p2.ID, result.Link.ID)
	if err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}
	if found {
		t.Error("should not find the link under the wrong payable")
	}

	found, err = payables.DeleteLink(ctx, f.conn, p1.ID, result.Link.ID)
	if err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}
	if !found {
		t.Error("should find and delete the link under the correct payable")
	}
}

func TestUnlinkIfPresentTriggeredByIgnoringTransaction(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("BRL")
	txID := f.addTransaction(txSpec{AccountID: acc, Amount: "-100.00", CurrencyCode: "BRL", MovementType: "DEBIT", ProviderStatus: "POSTED"})
	p, _ := payables.Create(ctx, f.conn, payables.KindDebt, "Dívida", dec(t, "1000.00"), dec(t, "1000.00"))
	if _, err := payables.CreateLink(ctx, f.conn, payables.KindDebt, p.ID, txID); err != nil {
		t.Fatalf("CreateLink: %v", err)
	}

	if _, err := transactions.SetInclusion(ctx, f.conn, txID, money.Ignored, transactions.InclusionOriginManual, nil, nil, payables.UnlinkIfPresent); err != nil {
		t.Fatalf("SetInclusion: %v", err)
	}

	var count int
	f.conn.QueryRow(`SELECT COUNT(*) FROM payable_transaction_links WHERE transaction_id = ?`, txID).Scan(&count)
	if count != 0 {
		t.Errorf("link should be removed once the transaction is ignored, count = %d", count)
	}
}

func TestUnlinkIfPresentDropsGenericPayableAllocation(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	account := f.addAccount("BRL")
	txID := f.addTransaction(txSpec{AccountID: account, Description: "Pagamento", Amount: "-100.00", CurrencyCode: "BRL", MovementType: "DEBIT", ProviderStatus: "POSTED"})
	p, err := payables.Create(ctx, f.conn, payables.KindDebt, "Dívida", dec(t, "500.00"), dec(t, "500.00"))
	if err != nil {
		t.Fatalf("Create payable: %v", err)
	}
	plan, err := scenarios.CreateScenario(ctx, f.conn, scenarios.KindDebtPlan, "Plano", &p.ID)
	if err != nil {
		t.Fatalf("Create scenario: %v", err)
	}
	link, err := payables.CreateLink(ctx, f.conn, payables.KindDebt, p.ID, txID)
	if err != nil || link.Link == nil {
		t.Fatalf("CreateLink: result=%+v err=%v", link, err)
	}
	planned, err := scenarios.CreateScenarioTransaction(ctx, f.conn, plan.ID, "Parcela", dec(t, "100.00"), time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), nil)
	if err != nil {
		t.Fatalf("Create scenario transaction: %v", err)
	}
	if _, err := scenarios.CreateRealization(ctx, f.conn, planned.ID, &link.Link.ID, dec(t, "100.00")); err != nil {
		t.Fatalf("Create realization: %v", err)
	}

	if err := payables.UnlinkIfPresent(ctx, f.conn, txID); err != nil {
		t.Fatalf("UnlinkIfPresent: %v", err)
	}
	var count int
	if err := f.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM scenario_realizations
		WHERE relation_type = 'allocation' AND transaction_id = ?`, txID).Scan(&count); err != nil {
		t.Fatalf("count generic allocations: %v", err)
	}
	if count != 0 {
		t.Fatalf("generic allocation should be removed with its payable link, count = %d", count)
	}
}

func TestListEligibleTransactionsExcludesLinkedIgnoredAndNonBRL(t *testing.T) {
	for _, kind := range bothKinds {
		t.Run(string(kind), func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			acc := f.addAccount("BRL")
			now := time.Now().UTC()

			specs := settlingSpec(acc, "10.00")
			spec := specs[kind]
			spec.OccurredAt = &now
			eligible := f.addTransaction(spec)

			linkedSpec := specs[kind]
			linkedSpec.Description = "Ja vinculada"
			linkedSpec.OccurredAt = &now
			linked := f.addTransaction(linkedSpec)

			usdSpec := specs[kind]
			usdSpec.Description = "Moeda estrangeira"
			usdSpec.CurrencyCode = "USD"
			usdSpec.OccurredAt = &now
			usdSpec.NoAccountCurrencyAmount = true
			usdTx := f.addTransaction(usdSpec)

			ignoredSpec := specs[kind]
			ignoredSpec.Description = "Ignorada"
			ignoredSpec.OccurredAt = &now
			ignoredTx := f.addTransaction(ignoredSpec)

			p, _ := payables.Create(ctx, f.conn, kind, "Pendência", dec(t, "1000.00"), dec(t, "1000.00"))
			if _, err := payables.CreateLink(ctx, f.conn, kind, p.ID, linked); err != nil {
				t.Fatalf("CreateLink: %v", err)
			}
			if _, err := transactions.SetInclusion(ctx, f.conn, ignoredTx, money.Ignored, transactions.InclusionOriginManual, nil, nil, nil); err != nil {
				t.Fatalf("SetInclusion: %v", err)
			}

			rows, err := payables.ListEligibleTransactions(ctx, f.conn, kind, nil, 20)
			if err != nil {
				t.Fatalf("ListEligibleTransactions: %v", err)
			}
			ids := make(map[string]bool)
			for _, r := range rows {
				ids[r.ID] = true
			}
			if !ids[eligible] {
				t.Error("eligible transaction should be listed")
			}
			if ids[linked] {
				t.Error("already-linked transaction should be excluded")
			}
			if ids[usdTx] {
				t.Error("non-BRL transaction should be excluded")
			}
			if ids[ignoredTx] {
				t.Error("ignored transaction should be excluded")
			}
		})
	}
}

func TestListEligibleTransactionsSearchFiltersByDescription(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("BRL")
	now := time.Now().UTC()
	f.addTransaction(txSpec{AccountID: acc, Description: "Mercado Livre", Amount: "-10.00", CurrencyCode: "BRL", MovementType: "DEBIT", ProviderStatus: "POSTED", OccurredAt: &now})
	f.addTransaction(txSpec{AccountID: acc, Description: "Uber", Amount: "-20.00", CurrencyCode: "BRL", MovementType: "DEBIT", ProviderStatus: "POSTED", OccurredAt: &now})

	search := "mercado"
	rows, err := payables.ListEligibleTransactions(ctx, f.conn, payables.KindDebt, &search, 20)
	if err != nil {
		t.Fatalf("ListEligibleTransactions: %v", err)
	}
	if len(rows) != 1 || *rows[0].Description != "Mercado Livre" {
		t.Errorf("rows = %+v", rows)
	}
}

// backdatePayable overrides a payable's created_at, which payables.Create
// always stamps with the real wall-clock time — tests that reconstruct a
// payable's state on a synthetic past day need it to actually predate that
// day, or the "did not exist yet" rule zeroes it out.
func (f *fixture) backdatePayable(payableID string, createdAt time.Time) {
	f.t.Helper()
	f.exec(`UPDATE payables SET created_at = ? WHERE id = ?`, db.FormatTime(createdAt), payableID)
}
