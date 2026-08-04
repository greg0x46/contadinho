package httpapi_test

import (
	"database/sql"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
)

// insertInvestment inserts the minimal sync-schema chain plus one
// financial_investments row directly, so HTTP tests can exercise the read
// endpoint without a full sync pipeline.
func insertInvestment(t *testing.T, conn *sql.DB) string {
	t.Helper()
	now := db.FormatTime(time.Now())
	sourceID, syncRunID, rawImportID, investmentID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(query, args...); err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
	}
	exec(`INSERT INTO data_sources (id, provider, external_item_id, display_name, created_at, updated_at)
		VALUES (?, 'pluggy', ?, 'Banco Exemplo', ?, ?)`, sourceID, sourceID, now, now)
	exec(`INSERT INTO sync_runs (id, source_id, status, started_at, finished_at)
		VALUES (?, ?, 'completed', ?, ?)`, syncRunID, sourceID, now, now)
	exec(`INSERT INTO raw_imports (
			id, sync_run_id, source_id, scope, page_sequence, request_attempt,
			request_method, request_path, http_status, response_headers, payload,
			payload_sha256, received_at
		) VALUES (?, ?, ?, 'investments', 1, 1, 'GET', '/x', 200, '{}', x'00', 'sha', ?)`,
		rawImportID, syncRunID, sourceID, now)
	exec(`INSERT INTO financial_investments (
			id, source_id, external_id, investment_type, name, balance, currency_code,
			current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, 'MUTUAL_FUND', 'Fundo XYZ', '1000.50', 'BRL', ?, 'hash', ?, ?)`,
		investmentID, sourceID, investmentID, rawImportID, now, now)
	return investmentID
}

// insertInvestmentTransaction inserts a financial_investment_transactions
// row (plus the raw_import it references) for an investment created by
// insertInvestment, so yield tests can build a buy/sell history.
func insertInvestmentTransaction(t *testing.T, conn *sql.DB, investmentID, movementType, amount string) {
	t.Helper()
	now := db.FormatTime(time.Now())
	var sourceID, syncRunID string
	if err := conn.QueryRow(`SELECT source_id FROM financial_investments WHERE id = ?`, investmentID).Scan(&sourceID); err != nil {
		t.Fatalf("lookup source_id: %v", err)
	}
	if err := conn.QueryRow(`SELECT sync_run_id FROM raw_imports WHERE id = (
		SELECT current_raw_import_id FROM financial_investments WHERE id = ?)`, investmentID).Scan(&syncRunID); err != nil {
		t.Fatalf("lookup sync_run_id: %v", err)
	}
	var pageSequence int
	if err := conn.QueryRow(`SELECT count(*) FROM raw_imports WHERE sync_run_id = ? AND scope = 'investment_transactions'`, syncRunID).Scan(&pageSequence); err != nil {
		t.Fatalf("count raw_imports: %v", err)
	}
	pageSequence++
	rawImportID := uuid.NewString()
	if _, err := conn.Exec(`INSERT INTO raw_imports (
			id, sync_run_id, source_id, scope, page_sequence, request_attempt,
			request_method, request_path, http_status, response_headers, payload,
			payload_sha256, received_at
		) VALUES (?, ?, ?, 'investment_transactions', ?, 1, 'GET', '/x', 200, '{}', x'00', 'sha', ?)`,
		rawImportID, syncRunID, sourceID, pageSequence, now); err != nil {
		t.Fatalf("insert raw_import: %v", err)
	}
	transactionID := uuid.NewString()
	if _, err := conn.Exec(`INSERT INTO financial_investment_transactions (
			id, source_id, investment_id, external_id, movement_type, quantity, value, amount,
			occurred_at, trade_date, current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, '1', '1.00', ?, ?, ?, ?, 'hash', ?, ?)`,
		transactionID, sourceID, investmentID, transactionID, movementType, amount, now, now, rawImportID, now, now); err != nil {
		t.Fatalf("insert investment transaction: %v", err)
	}
}

func TestListInvestmentsReturnsSyncedHoldings(t *testing.T) {
	srv, conn := newTestServer(t)
	investmentID := insertInvestment(t, conn)

	resp, err := http.Get(srv.URL + "/api/investments")
	if err != nil {
		t.Fatalf("GET /api/investments: %v", err)
	}
	var investments []map[string]any
	decodeJSON(t, resp, &investments)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(investments) != 1 {
		t.Fatalf("len(investments) = %d, want 1", len(investments))
	}
	got := investments[0]
	if got["id"] != investmentID {
		t.Errorf("id = %v, want %v", got["id"], investmentID)
	}
	if got["name"] != "Fundo XYZ" || got["balance"] != "1000.50" || got["source_display_name"] != "Banco Exemplo" {
		t.Errorf("investment = %+v", got)
	}
}

func TestListInvestmentsReturnsEmptyArrayNotNull(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/api/investments")
	if err != nil {
		t.Fatalf("GET /api/investments: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if string(body) != "[]\n" {
		t.Errorf("body = %q, want an empty JSON array", body)
	}
}

func TestGetInvestmentReturnsHolding(t *testing.T) {
	srv, conn := newTestServer(t)
	investmentID := insertInvestment(t, conn)

	resp, err := http.Get(srv.URL + "/api/investments/" + investmentID)
	if err != nil {
		t.Fatalf("GET /api/investments/{id}: %v", err)
	}
	var got map[string]any
	decodeJSON(t, resp, &got)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got["id"] != investmentID || got["name"] != "Fundo XYZ" {
		t.Errorf("investment = %+v", got)
	}
}

func TestGetInvestmentReturns404WhenMissing(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/api/investments/" + uuid.NewString())
	if err != nil {
		t.Fatalf("GET /api/investments/{id}: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}

func TestListInvestmentTransactionsReturnsHistory(t *testing.T) {
	srv, conn := newTestServer(t)
	investmentID := insertInvestment(t, conn)
	insertInvestmentTransaction(t, conn, investmentID, "BUY", "1000.00")

	resp, err := http.Get(srv.URL + "/api/investments/" + investmentID + "/transactions")
	if err != nil {
		t.Fatalf("GET /api/investments/{id}/transactions: %v", err)
	}
	var got []map[string]any
	decodeJSON(t, resp, &got)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if len(got) != 1 || got[0]["movement_type"] != "BUY" || got[0]["amount"] != "1000.00" {
		t.Errorf("transactions = %+v", got)
	}
}

// Regression test: Pluggy's investments.amount field for renda fixa holdings
// turned out to be quantity × value (current gross mark value), not the
// invested principal, so balance - amount produced a false negative yield on
// healthy CDBs. The yield must come from the actual buy/sell history instead.
func TestListInvestmentsComputesYieldFromTransactionHistory(t *testing.T) {
	srv, conn := newTestServer(t)
	investmentID := insertInvestment(t, conn) // balance = 1000.50, amount_profit unset
	insertInvestmentTransaction(t, conn, investmentID, "BUY", "1200.00")
	insertInvestmentTransaction(t, conn, investmentID, "SELL", "300.00")
	// net contributed = 1200.00 - 300.00 = 900.00; yield = balance - net = 100.50

	resp, err := http.Get(srv.URL + "/api/investments")
	if err != nil {
		t.Fatalf("GET /api/investments: %v", err)
	}
	var investments []map[string]any
	decodeJSON(t, resp, &investments)
	if resp.StatusCode != http.StatusOK || len(investments) != 1 {
		t.Fatalf("status = %d, investments = %+v", resp.StatusCode, investments)
	}
	got := investments[0]
	if got["yield_value"] != "100.50" || got["yield_source"] != "calculado" {
		t.Errorf("yield_value = %v, yield_source = %v, want 100.50/calculado", got["yield_value"], got["yield_source"])
	}
}

func TestListInvestmentsPrefersProviderAmountProfitOverHistory(t *testing.T) {
	srv, conn := newTestServer(t)
	investmentID := insertInvestment(t, conn)
	insertInvestmentTransaction(t, conn, investmentID, "BUY", "1200.00")
	if _, err := conn.Exec(`UPDATE financial_investments SET amount_profit = '42.00' WHERE id = ?`, investmentID); err != nil {
		t.Fatalf("set amount_profit: %v", err)
	}

	resp, err := http.Get(srv.URL + "/api/investments")
	if err != nil {
		t.Fatalf("GET /api/investments: %v", err)
	}
	var investments []map[string]any
	decodeJSON(t, resp, &investments)
	got := investments[0]
	if got["yield_value"] != "42.00" || got["yield_source"] != "informado" {
		t.Errorf("yield_value = %v, yield_source = %v, want 42.00/informado", got["yield_value"], got["yield_source"])
	}
}

func TestListInvestmentsOmitsYieldWithoutProfitOrHistory(t *testing.T) {
	srv, conn := newTestServer(t)
	insertInvestment(t, conn)

	resp, err := http.Get(srv.URL + "/api/investments")
	if err != nil {
		t.Fatalf("GET /api/investments: %v", err)
	}
	var investments []map[string]any
	decodeJSON(t, resp, &investments)
	got := investments[0]
	if got["yield_value"] != nil || got["yield_source"] != nil {
		t.Errorf("yield_value = %v, yield_source = %v, want both nil", got["yield_value"], got["yield_source"])
	}
}

// Regression test: an investment whose only captured history is a
// SELL/REDEMPTION (the original BUY was never synced — closed position from
// before the sync window, or the provider's history is otherwise partial)
// must not report the entire redemption as yield. Reported in production as
// a CDB with balance=0 and a single SELL of 11370.20 showing "Rendimento:
// R$ 11.370,20" instead of "Não disponível".
func TestListInvestmentsOmitsCalculatedYieldWhenHistoryHasNoInflow(t *testing.T) {
	srv, conn := newTestServer(t)
	investmentID := insertInvestment(t, conn)
	if _, err := conn.Exec(`UPDATE financial_investments SET balance = '0' WHERE id = ?`, investmentID); err != nil {
		t.Fatalf("set balance: %v", err)
	}
	insertInvestmentTransaction(t, conn, investmentID, "SELL", "11370.20")

	resp, err := http.Get(srv.URL + "/api/investments")
	if err != nil {
		t.Fatalf("GET /api/investments: %v", err)
	}
	var investments []map[string]any
	decodeJSON(t, resp, &investments)
	if resp.StatusCode != http.StatusOK || len(investments) != 1 {
		t.Fatalf("status = %d, investments = %+v", resp.StatusCode, investments)
	}
	got := investments[0]
	if got["yield_value"] != nil || got["yield_source"] != nil {
		t.Errorf("yield_value = %v, yield_source = %v, want both nil", got["yield_value"], got["yield_source"])
	}
}

func TestListInvestmentTransactionsReturns404WhenInvestmentMissing(t *testing.T) {
	srv, _ := newTestServer(t)

	resp, err := http.Get(srv.URL + "/api/investments/" + uuid.NewString() + "/transactions")
	if err != nil {
		t.Fatalf("GET /api/investments/{id}/transactions: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
}
