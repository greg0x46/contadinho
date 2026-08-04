package receivables_test

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
	"contadinho-go/internal/receivables"
	"contadinho-go/internal/transactions"
)

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

func TestCreateGetUpdateDeleteReceivable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	rec, err := receivables.Create(ctx, f.conn, "Empréstimo para Ana", dec(t, "1000.00"), dec(t, "600.00"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !rec.StartingReceivedAmount.Equal(dec(t, "400.00")) {
		t.Errorf("StartingReceivedAmount = %s, want 400.00 (total - initial remaining)", rec.StartingReceivedAmount)
	}

	got, err := receivables.Get(ctx, f.conn, rec.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name != rec.Name {
		t.Errorf("Get() = %+v", got)
	}

	updated, err := receivables.Update(ctx, f.conn, rec.ID, "Novo nome", dec(t, "2000.00"))
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Name != "Novo nome" || !updated.TotalAmount.Equal(dec(t, "2000.00")) {
		t.Errorf("Update() = %+v", updated)
	}
	// starting_received_amount is never edited by Update.
	if !updated.StartingReceivedAmount.Equal(dec(t, "400.00")) {
		t.Errorf("StartingReceivedAmount changed by Update: %s", updated.StartingReceivedAmount)
	}

	if err := receivables.Delete(ctx, f.conn, rec.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := receivables.Get(ctx, f.conn, rec.ID); !errors.Is(err, receivables.ErrNotFound) {
		t.Errorf("Get after delete: err = %v, want ErrNotFound", err)
	}
}

func TestUpdateDeleteUnknownReceivable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, err := receivables.Update(ctx, f.conn, "unknown", "x", dec(t, "1.00")); !errors.Is(err, receivables.ErrNotFound) {
		t.Errorf("Update: err = %v, want ErrNotFound", err)
	}
	if err := receivables.Delete(ctx, f.conn, "unknown"); !errors.Is(err, receivables.ErrNotFound) {
		t.Errorf("Delete: err = %v, want ErrNotFound", err)
	}
}

func TestCreateLinkEligibleTransaction(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("BRL")
	txID := f.addTransaction(txSpec{AccountID: acc, Description: "Recebimento", Amount: "500.00", CurrencyCode: "BRL", MovementType: "CREDIT", ProviderStatus: "POSTED"})
	rec, _ := receivables.Create(ctx, f.conn, "Empréstimo", dec(t, "1000.00"), dec(t, "1000.00"))

	result, err := receivables.CreateLink(ctx, f.conn, rec.ID, txID)
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if result.Status != receivables.StatusCreated || result.Link == nil {
		t.Fatalf("result = %+v", result)
	}
	if !result.Link.LinkedAmount.Equal(dec(t, "500.00")) {
		t.Errorf("LinkedAmount = %s, want 500.00 (abs value)", result.Link.LinkedAmount)
	}

	updatedRec, err := receivables.Get(ctx, f.conn, rec.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	links, err := receivables.Links(ctx, f.conn, updatedRec.ID)
	if err != nil {
		t.Fatalf("Links: %v", err)
	}
	if len(links) != 1 {
		t.Fatalf("len(links) = %d, want 1", len(links))
	}
	amt, err := receivables.LinkEffectiveAmount(ctx, f.conn, links[0].TransactionID)
	if err != nil {
		t.Fatalf("LinkEffectiveAmount: %v", err)
	}
	received := receivables.ReceivedAmount(updatedRec.StartingReceivedAmount, []decimal.Decimal{amt})
	if !received.Equal(dec(t, "500.00")) {
		t.Errorf("ReceivedAmount = %s, want 500.00", received)
	}
}

func TestCreateLinkRejectsIneligibleTransaction(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("BRL")
	outflowTx := f.addTransaction(txSpec{AccountID: acc, Description: "Compra", Amount: "-1000.00", CurrencyCode: "BRL", MovementType: "DEBIT", ProviderStatus: "POSTED"})
	rec, _ := receivables.Create(ctx, f.conn, "Empréstimo", dec(t, "1000.00"), dec(t, "1000.00"))

	result, err := receivables.CreateLink(ctx, f.conn, rec.ID, outflowTx)
	if err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if result.Status != receivables.StatusIneligible || result.Reason == nil || *result.Reason != receivables.ReasonNotInflow {
		t.Errorf("result = %+v, want ineligible/not_inflow", result)
	}
}

func TestCreateLinkConflictsOnAlreadyLinked(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("BRL")
	txID := f.addTransaction(txSpec{AccountID: acc, Amount: "100.00", CurrencyCode: "BRL", MovementType: "CREDIT", ProviderStatus: "POSTED"})
	rec1, _ := receivables.Create(ctx, f.conn, "Recebível 1", dec(t, "1000.00"), dec(t, "1000.00"))
	rec2, _ := receivables.Create(ctx, f.conn, "Recebível 2", dec(t, "1000.00"), dec(t, "1000.00"))

	if _, err := receivables.CreateLink(ctx, f.conn, rec1.ID, txID); err != nil {
		t.Fatalf("first CreateLink: %v", err)
	}
	result, err := receivables.CreateLink(ctx, f.conn, rec2.ID, txID)
	if err != nil {
		t.Fatalf("second CreateLink: %v", err)
	}
	if result.Status != receivables.StatusConflict {
		t.Errorf("status = %s, want conflict", result.Status)
	}
}

func TestCreateLinkUnknownReceivableOrTransaction(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("BRL")
	txID := f.addTransaction(txSpec{AccountID: acc, Amount: "100.00", CurrencyCode: "BRL", MovementType: "CREDIT", ProviderStatus: "POSTED"})
	rec, _ := receivables.Create(ctx, f.conn, "Recebível", dec(t, "1000.00"), dec(t, "1000.00"))

	result, err := receivables.CreateLink(ctx, f.conn, "unknown-receivable", txID)
	if err != nil || result.Status != receivables.StatusReceivableNotFound {
		t.Errorf("result=%+v err=%v, want receivable_not_found", result, err)
	}
	result, err = receivables.CreateLink(ctx, f.conn, rec.ID, "unknown-tx")
	if err != nil || result.Status != receivables.StatusTransactionNotFound {
		t.Errorf("result=%+v err=%v, want transaction_not_found", result, err)
	}
}

func TestDeleteLinkScopedToReceivable(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("BRL")
	txID := f.addTransaction(txSpec{AccountID: acc, Amount: "100.00", CurrencyCode: "BRL", MovementType: "CREDIT", ProviderStatus: "POSTED"})
	rec1, _ := receivables.Create(ctx, f.conn, "Recebível 1", dec(t, "1000.00"), dec(t, "1000.00"))
	rec2, _ := receivables.Create(ctx, f.conn, "Recebível 2", dec(t, "1000.00"), dec(t, "1000.00"))
	result, _ := receivables.CreateLink(ctx, f.conn, rec1.ID, txID)

	found, err := receivables.DeleteLink(ctx, f.conn, rec2.ID, result.Link.ID)
	if err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}
	if found {
		t.Error("should not find the link under the wrong receivable")
	}

	found, err = receivables.DeleteLink(ctx, f.conn, rec1.ID, result.Link.ID)
	if err != nil {
		t.Fatalf("DeleteLink: %v", err)
	}
	if !found {
		t.Error("should find and delete the link under the correct receivable")
	}
}

func TestUnlinkIfPresentTriggeredByIgnoringTransaction(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("BRL")
	txID := f.addTransaction(txSpec{AccountID: acc, Amount: "100.00", CurrencyCode: "BRL", MovementType: "CREDIT", ProviderStatus: "POSTED"})
	rec, _ := receivables.Create(ctx, f.conn, "Recebível", dec(t, "1000.00"), dec(t, "1000.00"))
	if _, err := receivables.CreateLink(ctx, f.conn, rec.ID, txID); err != nil {
		t.Fatalf("CreateLink: %v", err)
	}

	if _, err := transactions.SetInclusion(ctx, f.conn, txID, money.Ignored, transactions.InclusionOriginManual, nil, nil, receivables.UnlinkIfPresent); err != nil {
		t.Fatalf("SetInclusion: %v", err)
	}

	var count int
	f.conn.QueryRow(`SELECT COUNT(*) FROM receivable_transaction_links WHERE transaction_id = ?`, txID).Scan(&count)
	if count != 0 {
		t.Errorf("link should be removed once the transaction is ignored, count = %d", count)
	}
}

func TestListEligibleTransactionsExcludesLinkedIgnoredAndNonBRL(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("BRL")
	now := time.Now().UTC()

	eligible := f.addTransaction(txSpec{AccountID: acc, Description: "Recebimento X", Amount: "10.00", CurrencyCode: "BRL", MovementType: "CREDIT", ProviderStatus: "POSTED", OccurredAt: &now})
	linked := f.addTransaction(txSpec{AccountID: acc, Description: "Ja vinculada", Amount: "20.00", CurrencyCode: "BRL", MovementType: "CREDIT", ProviderStatus: "POSTED", OccurredAt: &now})
	usdTx := f.addTransaction(txSpec{AccountID: acc, Description: "Recebimento USD", Amount: "30.00", CurrencyCode: "USD", MovementType: "CREDIT", ProviderStatus: "POSTED", OccurredAt: &now, NoAccountCurrencyAmount: true})
	ignoredTx := f.addTransaction(txSpec{AccountID: acc, Description: "Ignorada", Amount: "40.00", CurrencyCode: "BRL", MovementType: "CREDIT", ProviderStatus: "POSTED", OccurredAt: &now})

	rec, _ := receivables.Create(ctx, f.conn, "Recebível", dec(t, "1000.00"), dec(t, "1000.00"))
	if _, err := receivables.CreateLink(ctx, f.conn, rec.ID, linked); err != nil {
		t.Fatalf("CreateLink: %v", err)
	}
	if _, err := transactions.SetInclusion(ctx, f.conn, ignoredTx, money.Ignored, transactions.InclusionOriginManual, nil, nil, nil); err != nil {
		t.Fatalf("SetInclusion: %v", err)
	}
	_ = usdTx

	rows, err := receivables.ListEligibleTransactions(ctx, f.conn, nil, 20)
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
}

func TestListEligibleTransactionsSearchFiltersByDescription(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	acc := f.addAccount("BRL")
	now := time.Now().UTC()
	f.addTransaction(txSpec{AccountID: acc, Description: "Mercado Livre", Amount: "10.00", CurrencyCode: "BRL", MovementType: "CREDIT", ProviderStatus: "POSTED", OccurredAt: &now})
	f.addTransaction(txSpec{AccountID: acc, Description: "Uber", Amount: "20.00", CurrencyCode: "BRL", MovementType: "CREDIT", ProviderStatus: "POSTED", OccurredAt: &now})

	search := "mercado"
	rows, err := receivables.ListEligibleTransactions(ctx, f.conn, &search, 20)
	if err != nil {
		t.Fatalf("ListEligibleTransactions: %v", err)
	}
	if len(rows) != 1 || *rows[0].Description != "Mercado Livre" {
		t.Errorf("rows = %+v", rows)
	}
}
