package httpapi_test

import (
	"database/sql"
	"testing"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
)

// insertAccountWithInstitution mirrors insertAccount but lets a test pin the
// raw provider institution and the connection's manual label — the two
// inputs applyInstitutionName resolves between.
func insertAccountWithInstitution(t *testing.T, conn *sql.DB, institution string, label *string) string {
	t.Helper()
	now := db.FormatTime(time.Now())
	sourceID := uuid.NewString()
	accountID := uuid.NewString()
	syncRunID := uuid.NewString()
	rawImportID := uuid.NewString()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(query, args...); err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
	}
	exec(`INSERT INTO data_sources (id, provider, external_item_id, display_name, label, created_at, updated_at)
		VALUES (?, 'pluggy', ?, ?, ?, ?, ?)`, sourceID, sourceID, institution, label, now, now)
	exec(`INSERT INTO sync_runs (id, source_id, status, started_at, finished_at)
		VALUES (?, ?, 'completed', ?, ?)`, syncRunID, sourceID, now, now)
	exec(`INSERT INTO raw_imports (
			id, sync_run_id, source_id, scope, page_sequence, request_attempt,
			request_method, request_path, http_status, response_headers, payload,
			payload_sha256, received_at
		) VALUES (?, ?, ?, 'accounts', 1, 1, 'GET', '/x', 200, '{}', x'00', 'sha', ?)`,
		rawImportID, syncRunID, sourceID, now)
	exec(`INSERT INTO financial_accounts (
			id, source_id, external_id, institution, name, account_type, balance, currency_code,
			current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, 'Conta', 'BANK', '100.00', 'BRL', ?, 'hash', ?, ?)`,
		accountID, sourceID, accountID, institution, rawImportID, now, now)
	return accountID
}

func TestGetAccountHidesPluggyProxyConnectorAsInstitution(t *testing.T) {
	srv, conn := newTestServer(t)
	accountID := insertAccountWithInstitution(t, conn, "MeuPluggy", nil)

	account := getAccountJSON(t, srv.URL, accountID)
	if account["institution"] != "MeuPluggy" {
		t.Errorf("institution = %v, want the raw provider value preserved", account["institution"])
	}
	if account["institution_name"] != nil {
		t.Errorf("institution_name = %v, want nil", account["institution_name"])
	}
}

func TestGetAccountPrefersConnectionLabelForInstitutionName(t *testing.T) {
	srv, conn := newTestServer(t)
	accountID := insertAccountWithInstitution(t, conn, "MeuPluggy", strPtr("Itaú"))

	account := getAccountJSON(t, srv.URL, accountID)
	if account["institution_name"] != "Itaú" {
		t.Errorf("institution_name = %v, want %q", account["institution_name"], "Itaú")
	}
}

func TestGetAccountKeepsRealInstitutionName(t *testing.T) {
	srv, conn := newTestServer(t)
	accountID := insertAccountWithInstitution(t, conn, "Nu Pagamentos S.A. - Instituição de Pagamento", nil)

	account := getAccountJSON(t, srv.URL, accountID)
	if account["institution_name"] != "Nu Pagamentos S.A. - Instituição de Pagamento" {
		t.Errorf("institution_name = %v, want the real institution unchanged", account["institution_name"])
	}
}
