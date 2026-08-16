package timeline_test

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
)

// fixture mirrors internal/transactions' unexported one — package-private
// there, so timeline's tests need their own minimal copy of the sync-schema
// chain (data_sources -> sync_runs -> raw_imports) financial_accounts/
// financial_transactions require as foreign keys.
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

func (f *fixture) addAccount(balance string) string {
	f.t.Helper()
	id := uuid.NewString()
	now := db.FormatTime(time.Now())
	f.exec(`INSERT INTO financial_accounts (
			id, source_id, external_id, name, balance, currency_code,
			current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, 'Conta corrente', ?, 'BRL', ?, 'hash', ?, ?)`,
		id, f.sourceID, id, balance, f.rawImportID, now, now)
	return id
}

type txn struct {
	AccountID          string
	Amount             string // signed: positive credit, negative debit — mirrors Pluggy's own sign convention
	OccurredAt         time.Time
	CategoryID         *string
	CreditCardMetadata *string // raw Pluggy credit_card_metadata JSON, e.g. `{"billId":"..."}`
}

func (f *fixture) addTransaction(tx txn) string {
	f.t.Helper()
	id := uuid.NewString()
	now := db.FormatTime(time.Now())
	movementType := "DEBIT"
	magnitude := tx.Amount
	if strings.HasPrefix(tx.Amount, "-") {
		magnitude = strings.TrimPrefix(tx.Amount, "-")
	} else {
		movementType = "CREDIT"
	}
	f.exec(`INSERT INTO financial_transactions (
			id, source_id, account_id, external_id, description, amount,
			amount_in_account_currency, currency_code, occurred_at, provider_status,
			movement_type, credit_card_metadata, current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, 'Transação', ?, ?, 'BRL', ?, 'POSTED', ?, ?, ?, 'hash', ?, ?)`,
		id, f.sourceID, tx.AccountID, id, magnitude, magnitude, db.FormatTime(tx.OccurredAt), movementType, tx.CreditCardMetadata, f.rawImportID, now, now)
	if tx.CategoryID != nil {
		f.exec(`INSERT INTO transaction_category_decisions (transaction_id, category_id, revision, changed_at, origin)
			VALUES (?, ?, 1, ?, 'manual')`, id, *tx.CategoryID, now)
		f.exec(`INSERT INTO transaction_category_events (id, transaction_id, revision, previous_category_id, resulting_category_id, changed_at, origin)
			VALUES (?, ?, 1, NULL, ?, ?, 'manual')`, uuid.NewString(), id, *tx.CategoryID, now)
	}
	return id
}

// addCreditCardAccount mirrors addAccount but sets account_type = 'CREDIT',
// the signal timeline.realEntries uses to project the transaction's cash
// impact onto its bill's due date instead of occurred_at.
func (f *fixture) addCreditCardAccount(balance string) string {
	f.t.Helper()
	id := f.addAccount(balance)
	f.exec(`UPDATE financial_accounts SET account_type = 'CREDIT' WHERE id = ?`, id)
	return id
}

// addBill inserts a financial_bills row so ProjectedEntryDate can resolve a
// transaction's billId (or forecast month) to a real due date.
func (f *fixture) addBill(accountID, externalID, dueDate string) {
	f.t.Helper()
	id := uuid.NewString()
	now := db.FormatTime(time.Now())
	f.exec(`INSERT INTO financial_bills (
			id, source_id, account_id, external_id, due_date,
			current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, 'hash', ?, ?)`,
		id, f.sourceID, accountID, externalID, db.FormatTime(date(f.t, dueDate)), f.rawImportID, now, now)
}

func date(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

const (
	categorySupermercado = "000433b6-3094-5a9c-87df-465b70574a4b" // expense
	categorySalario      = "3c5a9586-2a11-556d-b014-692ed51c3997" // income
)
