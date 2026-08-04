package httpapi_test

import (
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
)

// insertInflowTransaction mirrors insertTransaction (server_test.go) but as
// a CREDIT inflow, the only kind of transaction eligible to settle a
// receivable.
func insertInflowTransaction(t *testing.T, conn *sql.DB) string {
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
		) VALUES (?, ?, ?, ?, 'Recebimento', '42.00', '42.00', 'BRL', ?, 'POSTED', 'CREDIT', ?, 'hash', ?, ?)`,
		txID, sourceID, accountID, txID, now, rawImportID, now, now)
	return txID
}

func TestReceivableScenarioLifecycleOverHTTP(t *testing.T) {
	srv, conn := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/receivables", map[string]any{"name": "Empréstimo para Ana", "total_amount": "1200.00"})
	if resp.StatusCode != 201 {
		t.Fatalf("create receivable status = %d, want 201", resp.StatusCode)
	}
	var rec map[string]any
	decodeJSON(t, resp, &rec)
	receivableID := rec["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/receivables/"+receivableID+"/scenarios", map[string]any{"name": "Plano de recebimento"})
	if resp.StatusCode != 201 {
		t.Fatalf("create scenario status = %d, want 201", resp.StatusCode)
	}
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)
	if scenario["kind"] != "receivable_plan" || scenario["receivable_id"] != receivableID {
		t.Errorf("scenario = %+v", scenario)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/receivables/"+receivableID+"/scenarios", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("list scenarios status = %d, want 200", resp.StatusCode)
	}
	var list []map[string]any
	decodeJSON(t, resp, &list)
	if len(list) != 1 || list[0]["id"] != scenarioID {
		t.Errorf("list = %+v", list)
	}

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/generate-installments", map[string]any{
		"cadence": "mensal", "months": 3, "start_date": "2026-01-01",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("generate installments status = %d, want 201", resp.StatusCode)
	}
	var installments []map[string]any
	decodeJSON(t, resp, &installments)
	if len(installments) != 3 {
		t.Fatalf("installments = %+v, want 3", installments)
	}
	transactionID := installments[0]["id"].(string)

	txID := insertInflowTransaction(t, conn)
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/receivables/"+receivableID+"/links", map[string]any{"transaction_id": txID})
	if resp.StatusCode != 201 {
		t.Fatalf("create link status = %d, want 201", resp.StatusCode)
	}
	var link map[string]any
	decodeJSON(t, resp, &link)
	linkID := link["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions/"+transactionID+"/realizations", map[string]any{
		"receivable_link_id": linkID, "allocated_amount": "42.00",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create realization status = %d, want 201", resp.StatusCode)
	}
	var updated map[string]any
	decodeJSON(t, resp, &updated)
	realizations := updated["realizations"].([]any)
	if len(realizations) != 1 || realizations[0].(map[string]any)["receivable_link_id"] != linkID {
		t.Errorf("realizations = %+v", realizations)
	}
}

func TestRealizationRejectsBothOrNeitherLinkKind(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/receivables", map[string]any{"name": "Empréstimo", "total_amount": "1000.00"})
	var rec map[string]any
	decodeJSON(t, resp, &rec)
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/receivables/"+rec["id"].(string)+"/scenarios", map[string]any{"name": "Plano"})
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions", map[string]any{
		"description": "Parcela 1", "amount": "42.00", "projected_at": "2026-01-01",
	})
	var st map[string]any
	decodeJSON(t, resp, &st)
	transactionID := st["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions/"+transactionID+"/realizations", map[string]any{
		"allocated_amount": "42.00",
	})
	if resp.StatusCode != 422 {
		t.Fatalf("neither link kind: status = %d, want 422", resp.StatusCode)
	}
	resp.Body.Close()

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions/"+transactionID+"/realizations", map[string]any{
		"debt_link_id": "x", "receivable_link_id": "y", "allocated_amount": "42.00",
	})
	if resp.StatusCode != 422 {
		t.Fatalf("both link kinds: status = %d, want 422", resp.StatusCode)
	}
	resp.Body.Close()
}
