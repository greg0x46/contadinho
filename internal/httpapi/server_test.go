package httpapi_test

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
	"contadinho-go/internal/httpapi"
	"contadinho-go/internal/settings"
)

const testPassword = "correct horse battery staple"

func newTestServerWithSession(t *testing.T) (*httptest.Server, *sql.DB, *settings.Session) {
	t.Helper()
	conn, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	frontend := fstest.MapFS{
		"index.html":    {Data: []byte("<html>spa</html>")},
		"assets/app.js": {Data: []byte("console.log(1)")},
	}
	session := settings.NewSession()
	srv := httptest.NewServer(httpapi.NewServer(conn, fs.FS(frontend), session))
	t.Cleanup(srv.Close)
	return srv, conn, session
}

// newTestServer returns a server that is already configured and unlocked —
// what every test exercising categories/transactions/debts/automation/
// sync-runs behavior wants, since it isn't testing the lock gate itself
// (see newLockedTestServer, TestSetupAndUnlockFlow, and TestLockGate* for
// that). It bypasses the HTTP /api/setup handler and calls settings.Setup
// directly, so — deliberately — it does NOT set any pluggy.* config values;
// tests that need those (e.g. pluggy.item_id) still set them explicitly.
func newTestServer(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()
	srv, conn, session := newTestServerWithSession(t)
	key, err := settings.Setup(context.Background(), conn, testPassword)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	session.Unlock(key)
	return srv, conn
}

// newLockedTestServer returns a server in its fresh, never-configured state.
func newLockedTestServer(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()
	srv, conn, _ := newTestServerWithSession(t)
	return srv, conn
}

// insertTransaction inserts the minimal sync-schema chain plus one
// financial_transactions row directly, so HTTP tests can exercise
// inclusion/category endpoints without a full sync pipeline (phase 4).
func insertTransaction(t *testing.T, conn *sql.DB) string {
	t.Helper()
	now := db.FormatTime(time.Now())
	sourceID, syncRunID, rawImportID, accountID, txID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(query, args...); err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
	}
	exec(`INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at)
		VALUES (?, 'pluggy', ?, ?, ?)`, sourceID, sourceID, now, now)
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
			id, source_id, account_id, external_id, description, amount, amount_in_account_currency,
			currency_code, occurred_at, provider_status, movement_type, current_raw_import_id,
			normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, 'Mercado', '-42.00', '-42.00', 'BRL', ?, 'POSTED', 'DEBIT', ?, 'hash', ?, ?)`,
		txID, sourceID, accountID, txID, now, rawImportID, now, now)
	return txID
}

func doJSON(t *testing.T, method, url string, body any) *http.Response {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

func TestLockGateRejectsAPIWhileLocked(t *testing.T) {
	srv, _ := newLockedTestServer(t)

	locked := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/categories"},
		{http.MethodPost, "/api/transactions/query"},
		{http.MethodGet, "/api/debts"},
		{http.MethodGet, "/api/automation-rules"},
		{http.MethodGet, "/api/sync-runs"},
	}
	for _, tc := range locked {
		resp := doJSON(t, tc.method, srv.URL+tc.path, nil)
		resp.Body.Close()
		if resp.StatusCode != http.StatusLocked {
			t.Errorf("%s %s while locked: status = %d, want 423", tc.method, tc.path, resp.StatusCode)
		}
	}

	exempt := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/setup/status"},
		{http.MethodGet, "/health"},
	}
	for _, tc := range exempt {
		resp := doJSON(t, tc.method, srv.URL+tc.path, nil)
		resp.Body.Close()
		if resp.StatusCode == http.StatusLocked {
			t.Errorf("%s %s should not be gated by the lock, got 423", tc.method, tc.path)
		}
	}

	// Once configured and unlocked, the same route that was 423 must work.
	resp := doJSON(t, http.MethodPost, srv.URL+"/api/setup", map[string]string{
		"password": testPassword, "pluggy_client_id": "cid",
		"pluggy_client_secret": "csecret", "pluggy_item_id": "item-1",
	})
	resp.Body.Close()
	if resp.StatusCode != 201 {
		t.Fatalf("setup status = %d, want 201", resp.StatusCode)
	}
	resp = doJSON(t, http.MethodGet, srv.URL+"/api/categories", nil)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("categories after setup: status = %d, want 200", resp.StatusCode)
	}
}

func TestSetupAndUnlockFlow(t *testing.T) {
	srv, _ := newLockedTestServer(t)

	resp := doJSON(t, http.MethodGet, srv.URL+"/api/setup/status", nil)
	var status map[string]any
	decodeJSON(t, resp, &status)
	if status["configured"] != false || status["unlocked"] != false {
		t.Fatalf("initial status = %+v, want configured=false unlocked=false", status)
	}

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/setup", map[string]string{
		"password": "correct horse battery staple", "pluggy_client_id": "cid",
		"pluggy_client_secret": "csecret", "pluggy_item_id": "item-1",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("setup status = %d, want 201", resp.StatusCode)
	}
	decodeJSON(t, resp, &status)
	if status["configured"] != true || status["unlocked"] != true {
		t.Errorf("post-setup status = %+v", status)
	}

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/setup", map[string]string{"password": "another password", "pluggy_client_id": "a", "pluggy_client_secret": "b", "pluggy_item_id": "c"})
	resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Errorf("second setup status = %d, want 409", resp.StatusCode)
	}

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/unlock", map[string]string{"password": "wrong"})
	resp.Body.Close()
	if resp.StatusCode != 401 {
		t.Errorf("wrong password unlock status = %d, want 401", resp.StatusCode)
	}

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/unlock", map[string]string{"password": "correct horse battery staple"})
	if resp.StatusCode != 200 {
		t.Fatalf("unlock status = %d, want 200", resp.StatusCode)
	}
	decodeJSON(t, resp, &status)
	if status["unlocked"] != true {
		t.Errorf("unlock result = %+v", status)
	}
}

func TestSyncRunLifecycleOverHTTP(t *testing.T) {
	srv, conn := newTestServer(t)

	// No item_id configured yet: creating a run should fail clearly rather
	// than silently creating a data_source with an empty external_item_id.
	resp := doJSON(t, http.MethodPost, srv.URL+"/api/sync-runs", nil)
	resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Fatalf("unconfigured create status = %d, want 409", resp.StatusCode)
	}

	if err := settings.Set(context.Background(), conn, "pluggy.item_id", "item-1", false, nil); err != nil {
		t.Fatalf("Set item_id: %v", err)
	}

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/sync-runs", nil)
	if resp.StatusCode != 202 {
		t.Fatalf("create status = %d, want 202", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc == "" {
		t.Error("expected a Location header")
	}
	var created map[string]any
	decodeJSON(t, resp, &created)
	runID := created["id"].(string)
	if created["status"] != "in_progress" {
		t.Errorf("status = %v, want in_progress", created["status"])
	}

	// A second run while the first is still in_progress must conflict.
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/sync-runs", nil)
	var conflict map[string]any
	decodeJSON(t, resp, &conflict)
	if resp.StatusCode != 409 || conflict["active_sync_run_id"] != runID {
		t.Errorf("second create: status=%d body=%+v", resp.StatusCode, conflict)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/sync-runs", nil)
	var list []map[string]any
	decodeJSON(t, resp, &list)
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/sync-runs/"+runID, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("get status = %d, want 200", resp.StatusCode)
	}
	var detail map[string]any
	decodeJSON(t, resp, &detail)
	if detail["id"] != runID {
		t.Errorf("detail id = %v, want %s", detail["id"], runID)
	}
	if _, ok := detail["failures"].([]any); !ok {
		t.Errorf("detail should include a failures array, got %+v", detail)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/sync-runs/does-not-exist", nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("unknown run status = %d, want 404", resp.StatusCode)
	}
}

func TestAutomationRuleLifecycleOverHTTP(t *testing.T) {
	srv, conn := newTestServer(t)
	txID := insertTransaction(t, conn)
	if _, err := conn.Exec(`UPDATE financial_transactions SET description = 'Transferencia enviada' WHERE id = ?`, txID); err != nil {
		t.Fatalf("update description: %v", err)
	}

	body := map[string]any{
		"name": "Ignorar transferências", "is_active": true, "logic_operator": "and",
		"conditions":          []map[string]string{{"field": "description", "operator": "contains", "value": "transferencia"}},
		"apply_retroactively": true,
	}
	resp := doJSON(t, http.MethodPost, srv.URL+"/api/automation-rules", body)
	if resp.StatusCode != 201 {
		t.Fatalf("create status = %d, want 201", resp.StatusCode)
	}
	var created map[string]any
	decodeJSON(t, resp, &created)
	rule := created["rule"].(map[string]any)
	ruleID := rule["id"].(string)
	retro := created["retroactive_apply"].(map[string]any)
	if retro["matched"].(float64) != 1 || retro["ignored"].(float64) != 1 {
		t.Errorf("retroactive_apply = %+v, want matched=1 ignored=1", retro)
	}

	var state string
	conn.QueryRow(`SELECT state FROM transaction_inclusion_decisions WHERE transaction_id = ?`, txID).Scan(&state)
	if state != "ignored" {
		t.Errorf("state = %s, want ignored", state)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/automation-rules", nil)
	var list []map[string]any
	decodeJSON(t, resp, &list)
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}

	resp = doJSON(t, http.MethodPatch, srv.URL+"/api/automation-rules/"+ruleID, map[string]bool{"is_active": false})
	if resp.StatusCode != 200 {
		t.Fatalf("patch status = %d, want 200", resp.StatusCode)
	}
	var patched map[string]any
	decodeJSON(t, resp, &patched)
	if patched["is_active"] != false {
		t.Errorf("is_active = %v, want false", patched["is_active"])
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/automation-rules/condition-options", nil)
	var options map[string]any
	decodeJSON(t, resp, &options)
	if _, ok := options["accounts"]; !ok {
		t.Errorf("expected an accounts key, got %+v", options)
	}

	resp = doJSON(t, http.MethodDelete, srv.URL+"/api/automation-rules/"+ruleID, nil)
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Errorf("delete status = %d, want 204", resp.StatusCode)
	}

	resp = doJSON(t, http.MethodPatch, srv.URL+"/api/automation-rules/"+ruleID, map[string]bool{"is_active": true})
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("patch after delete status = %d, want 404", resp.StatusCode)
	}
}

func TestCreateAutomationRuleRejectsEmptyConditions(t *testing.T) {
	srv, _ := newTestServer(t)
	body := map[string]any{"name": "Regra vazia", "is_active": true, "logic_operator": "and", "conditions": []map[string]string{}}
	resp := doJSON(t, http.MethodPost, srv.URL+"/api/automation-rules", body)
	resp.Body.Close()
	if resp.StatusCode != 422 {
		t.Errorf("status = %d, want 422", resp.StatusCode)
	}
}

func TestDebtLifecycleOverHTTP(t *testing.T) {
	srv, conn := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "Cartão", "total_amount": "1000.00", "initial_remaining_amount": "600.00"})
	if resp.StatusCode != 201 {
		t.Fatalf("create status = %d, want 201", resp.StatusCode)
	}
	var created map[string]any
	decodeJSON(t, resp, &created)
	debtID := created["id"].(string)
	if created["starting_paid_amount"] != "400.00" || created["status"] != "open" || created["link_count"].(float64) != 0 {
		t.Errorf("created = %+v", created)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/debts", nil)
	var list []map[string]any
	decodeJSON(t, resp, &list)
	if len(list) != 1 {
		t.Fatalf("len(list) = %d, want 1", len(list))
	}

	resp = doJSON(t, http.MethodPut, srv.URL+"/api/debts/"+debtID, map[string]any{"name": "Cartão renomeado", "total_amount": "2000.00"})
	if resp.StatusCode != 200 {
		t.Fatalf("update status = %d, want 200", resp.StatusCode)
	}
	var updated map[string]any
	decodeJSON(t, resp, &updated)
	if updated["name"] != "Cartão renomeado" || updated["total_amount"] != "2000.00" {
		t.Errorf("updated = %+v", updated)
	}

	// Link an eligible outflow transaction.
	txID := insertTransaction(t, conn)
	if _, err := conn.Exec(`UPDATE financial_transactions SET amount = '-150.00', amount_in_account_currency = '-150.00', currency_code = 'BRL' WHERE id = ?`, txID); err != nil {
		t.Fatalf("update transaction: %v", err)
	}
	if _, err := conn.Exec(`UPDATE financial_accounts SET currency_code = 'BRL' WHERE id = (SELECT account_id FROM financial_transactions WHERE id = ?)`, txID); err != nil {
		t.Fatalf("update account: %v", err)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/debts/eligible-transactions", nil)
	var eligible []map[string]any
	decodeJSON(t, resp, &eligible)
	if len(eligible) != 1 || eligible[0]["id"] != txID {
		t.Fatalf("eligible = %+v, want just %s", eligible, txID)
	}

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/links", map[string]string{"transaction_id": txID})
	if resp.StatusCode != 201 {
		t.Fatalf("create link status = %d, want 201", resp.StatusCode)
	}
	var link map[string]any
	decodeJSON(t, resp, &link)
	linkID := link["id"].(string)
	if link["linked_amount"] != "150.00" {
		t.Errorf("linked_amount = %v, want 150.00", link["linked_amount"])
	}

	// Linking the same transaction again must conflict.
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/links", map[string]string{"transaction_id": txID})
	resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Errorf("re-link status = %d, want 409", resp.StatusCode)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/debts/"+debtID, nil)
	var detail map[string]any
	decodeJSON(t, resp, &detail)
	if detail["paid_amount"] != "550.00" { // 400 starting + 150 linked
		t.Errorf("paid_amount = %v, want 550.00", detail["paid_amount"])
	}
	if detail["link_count"].(float64) != 1 {
		t.Errorf("link_count = %v, want 1", detail["link_count"])
	}
	links := detail["links"].([]any)
	if len(links) != 1 {
		t.Fatalf("links = %+v", links)
	}

	// Ignoring the transaction must unlink it (via debts.UnlinkIfPresent).
	resp = doJSON(t, http.MethodPut, srv.URL+"/api/transactions/"+txID+"/inclusion", map[string]string{"state": "ignored"})
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("ignore status = %d, want 200", resp.StatusCode)
	}
	resp = doJSON(t, http.MethodGet, srv.URL+"/api/debts/"+debtID, nil)
	decodeJSON(t, resp, &detail)
	if detail["link_count"].(float64) != 0 {
		t.Errorf("link_count after ignoring = %v, want 0", detail["link_count"])
	}

	resp = doJSON(t, http.MethodDelete, srv.URL+"/api/debts/"+debtID+"/links/"+linkID, nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("delete already-removed link status = %d, want 404", resp.StatusCode)
	}

	resp = doJSON(t, http.MethodDelete, srv.URL+"/api/debts/"+debtID, nil)
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Errorf("delete debt status = %d, want 204", resp.StatusCode)
	}
	resp = doJSON(t, http.MethodGet, srv.URL+"/api/debts/"+debtID, nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("get deleted debt status = %d, want 404", resp.StatusCode)
	}
}

func TestDebtTotalOwedOverHTTP(t *testing.T) {
	srv, conn := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "Financiamento", "total_amount": "1000.00", "initial_remaining_amount": "600.00"})
	if resp.StatusCode != 201 {
		t.Fatalf("create debt status = %d, want 201", resp.StatusCode)
	}
	resp.Body.Close()

	// "Parcelas futuras (cartão)" is the sum of financial_accounts.balance
	// across every CREDIT account — Pluggy already computes that balance
	// authoritatively (it reflects payments applied, fees, everything), so
	// this doesn't depend on any transaction's provider_status or on bill
	// closing dates. A transaction inserted here just to give the account a
	// row to exist through must NOT influence the total on its own.
	creditTxID := insertTransaction(t, conn)
	var creditAccountID string
	if err := conn.QueryRow(`SELECT account_id FROM financial_transactions WHERE id = ?`, creditTxID).Scan(&creditAccountID); err != nil {
		t.Fatalf("account id: %v", err)
	}
	if _, err := conn.Exec(`UPDATE financial_accounts SET account_type = 'CREDIT', balance = '5535.84' WHERE id = ?`, creditAccountID); err != nil {
		t.Fatalf("set credit account balance: %v", err)
	}

	// A non-CREDIT account's balance must not count toward future installments.
	bankTxID := insertTransaction(t, conn)
	var bankAccountID string
	if err := conn.QueryRow(`SELECT account_id FROM financial_transactions WHERE id = ?`, bankTxID).Scan(&bankAccountID); err != nil {
		t.Fatalf("account id: %v", err)
	}
	if _, err := conn.Exec(`UPDATE financial_accounts SET balance = '999999.00' WHERE id = ?`, bankAccountID); err != nil {
		t.Fatalf("set bank account balance: %v", err)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/debts/total-owed", nil)
	var total map[string]any
	decodeJSON(t, resp, &total)
	if total["remaining_debts_total"] != "600.00" {
		t.Errorf("remaining_debts_total = %v, want 600.00", total["remaining_debts_total"])
	}
	if total["future_installments_total"] != "5535.84" {
		t.Errorf("future_installments_total = %v, want 5535.84", total["future_installments_total"])
	}
	if total["total_owed"] != "6135.84" {
		t.Errorf("total_owed = %v, want 6135.84", total["total_owed"])
	}
	if total["currency_code"] != "BRL" {
		t.Errorf("currency_code = %v, want BRL", total["currency_code"])
	}
}

func TestSpendingByCategoryOverHTTP(t *testing.T) {
	srv, conn := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/categories", map[string]string{"name": "Mercado", "kind": "expense"})
	var category map[string]any
	decodeJSON(t, resp, &category)
	categoryID := category["id"].(string)

	categorizedTxID := insertTransaction(t, conn)
	resp = doJSON(t, http.MethodPut, srv.URL+"/api/transactions/"+categorizedTxID+"/category", map[string]string{"category_id": categoryID})
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("assign category status = %d, want 200", resp.StatusCode)
	}

	uncategorizedTxID := insertTransaction(t, conn)
	if _, err := conn.Exec(`UPDATE financial_transactions SET amount = '-10.00', amount_in_account_currency = '-10.00' WHERE id = ?`, uncategorizedTxID); err != nil {
		t.Fatalf("update transaction: %v", err)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/transactions/spending-by-category?timezone=UTC", nil)
	var body map[string]any
	decodeJSON(t, resp, &body)
	if body["currency_code"] != "BRL" {
		t.Errorf("currency_code = %v, want BRL", body["currency_code"])
	}
	if body["total"] != "52.00" { // -42.00 categorized + -10.00 uncategorized
		t.Errorf("total = %v, want 52.00", body["total"])
	}
	items := body["items"].([]any)
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2: %+v", len(items), items)
	}
	first := items[0].(map[string]any)
	if first["category_id"] != categoryID || first["category_name"] != "Mercado" || first["amount"] != "42.00" {
		t.Errorf("items[0] = %+v", first)
	}
	second := items[1].(map[string]any)
	if second["category_id"] != nil || second["category_name"] != "Sem categoria" || second["amount"] != "10.00" {
		t.Errorf("items[1] = %+v", second)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/transactions/spending-by-category?timezone=Not/AZone", nil)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("invalid timezone status = %d, want 400", resp.StatusCode)
	}
}

func TestCreateDebtValidation(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "", "total_amount": "100.00"})
	resp.Body.Close()
	if resp.StatusCode != 422 {
		t.Errorf("empty name status = %d, want 422", resp.StatusCode)
	}
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "x", "total_amount": "-1.00"})
	resp.Body.Close()
	if resp.StatusCode != 422 {
		t.Errorf("non-positive amount status = %d, want 422", resp.StatusCode)
	}
}

func TestHealth(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := doJSON(t, http.MethodGet, srv.URL+"/health", nil)
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestSPAFallbackAndAssetsAndUnknownAPI(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodGet, srv.URL+"/transactions", nil)
	resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("SPA fallback: status=%d content-type=%s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/assets/app.js", nil)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("asset: status=%d", resp.StatusCode)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/does-not-exist", nil)
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("unknown api: status=%d, want 404", resp.StatusCode)
	}
}

func TestCategoryLifecycleOverHTTP(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/categories", map[string]string{"name": "Assinaturas", "kind": "expense"})
	if resp.StatusCode != 201 {
		t.Fatalf("create status = %d, want 201", resp.StatusCode)
	}
	var created map[string]any
	decodeJSON(t, resp, &created)
	id := created["id"].(string)

	resp = doJSON(t, http.MethodPatch, srv.URL+"/api/categories/"+id, map[string]any{"is_active": false})
	if resp.StatusCode != 200 {
		t.Fatalf("update status = %d, want 200", resp.StatusCode)
	}
	var updated map[string]any
	decodeJSON(t, resp, &updated)
	if updated["is_active"] != false {
		t.Errorf("is_active = %v, want false", updated["is_active"])
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/categories", nil)
	var list []map[string]any
	decodeJSON(t, resp, &list)
	if len(list) != 29 { // 28 seeded + 1 created
		t.Errorf("len(list) = %d, want 29", len(list))
	}
}

func TestTransactionInclusionAndCategoryOverHTTP(t *testing.T) {
	srv, conn := newTestServer(t)
	txID := insertTransaction(t, conn)

	resp := doJSON(t, http.MethodPut, srv.URL+"/api/transactions/"+txID+"/inclusion", map[string]string{"state": "ignored"})
	if resp.StatusCode != 200 {
		t.Fatalf("inclusion status = %d, want 200", resp.StatusCode)
	}
	var inclusion map[string]any
	decodeJSON(t, resp, &inclusion)
	if inclusion["state"] != "ignored" || inclusion["transaction_id"] != txID {
		t.Errorf("inclusion result = %+v", inclusion)
	}

	resp = doJSON(t, http.MethodPut, srv.URL+"/api/transactions/does-not-exist/inclusion", map[string]string{"state": "ignored"})
	resp.Body.Close()
	if resp.StatusCode != 404 {
		t.Errorf("unknown transaction inclusion status = %d, want 404", resp.StatusCode)
	}

	var categories []map[string]any
	resp = doJSON(t, http.MethodGet, srv.URL+"/api/categories", nil)
	decodeJSON(t, resp, &categories)
	categoryID := categories[0]["id"].(string)

	resp = doJSON(t, http.MethodPut, srv.URL+"/api/transactions/"+txID+"/category", map[string]string{"category_id": categoryID})
	if resp.StatusCode != 200 {
		t.Fatalf("category status = %d, want 200", resp.StatusCode)
	}
	var category map[string]any
	decodeJSON(t, resp, &category)
	if category["category_id"] != categoryID || category["origin"] != "manual" {
		t.Errorf("category result = %+v", category)
	}

	resp = doJSON(t, http.MethodPut, srv.URL+"/api/transactions/"+txID+"/category", map[string]string{"category_id": "not-a-uuid"})
	resp.Body.Close()
	if resp.StatusCode != 422 {
		t.Errorf("invalid category status = %d, want 422", resp.StatusCode)
	}
}

func TestQueryTransactionsOverHTTP(t *testing.T) {
	srv, conn := newTestServer(t)
	txID := insertTransaction(t, conn)

	var categories []map[string]any
	resp := doJSON(t, http.MethodGet, srv.URL+"/api/categories", nil)
	decodeJSON(t, resp, &categories)
	var categoryID string
	for _, c := range categories {
		if c["kind"] == "expense" {
			categoryID = c["id"].(string)
			break
		}
	}
	resp = doJSON(t, http.MethodPut, srv.URL+"/api/transactions/"+txID+"/category", map[string]string{"category_id": categoryID})
	resp.Body.Close()

	body := map[string]any{
		"timezone": "UTC", "group_by": "none", "page": 1, "page_size": 50,
		"filters": map[string]any{
			"date_from": nil, "date_to": nil, "description": nil, "account_id": nil,
			"institution": nil, "category_id": nil, "classification": nil,
			"provider_status": nil, "amount_min": nil, "amount_max": nil, "uncategorized": nil,
		},
	}
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/transactions/query", body)
	if resp.StatusCode != 200 {
		t.Fatalf("query status = %d, want 200", resp.StatusCode)
	}
	var result map[string]any
	decodeJSON(t, resp, &result)
	items, _ := result["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	totals, _ := result["totals"].([]any)
	if len(totals) != 1 {
		t.Fatalf("len(totals) = %d, want 1", len(totals))
	}
	total := totals[0].(map[string]any)
	if total["outflow"] != "42.00" {
		t.Errorf("outflow = %v, want 42.00", total["outflow"])
	}

	badBody := map[string]any{"timezone": "Not/AZone", "group_by": "none", "page": 1, "page_size": 50,
		"filters": map[string]any{
			"date_from": nil, "date_to": nil, "description": nil, "account_id": nil,
			"institution": nil, "category_id": nil, "classification": nil,
			"provider_status": nil, "amount_min": nil, "amount_max": nil, "uncategorized": nil,
		},
	}
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/transactions/query", badBody)
	resp.Body.Close()
	if resp.StatusCode != 400 {
		t.Errorf("invalid timezone status = %d, want 400", resp.StatusCode)
	}
}
