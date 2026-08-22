package httpapi_test

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"contadinho-go/internal/db"
)

// reconciliationFixture is the pairing every test here needs: a category, an
// expense commitment on day 5, and a server already unlocked. The commitment
// deliberately has NO automation rule — these tests exercise the manual
// override path, and a rule would make it ambiguous which layer reconciled.
type reconciliationFixture struct {
	t          *testing.T
	url        string
	conn       *sql.DB
	categoryID string
	id         string
}

func newReconciliationFixture(t *testing.T) *reconciliationFixture {
	t.Helper()
	srv, conn := newTestServer(t)
	f := &reconciliationFixture{t: t, url: srv.URL, conn: conn}

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/categories", map[string]any{
		"name": "Aluguel", "kind": "expense", "icon": "home", "color": "#495057",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create category status = %d, want 201", resp.StatusCode)
	}
	var category map[string]any
	decodeJSON(t, resp, &category)
	f.categoryID = category["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/recurring-commitments", map[string]any{
		"name": "Aluguel", "kind": "expense", "amount": "1500.00",
		"category_id": f.categoryID, "cadence": "monthly", "day_of_month": 5,
		"start_date": "2026-01-01", "is_active": true,
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create commitment status = %d, want 201", resp.StatusCode)
	}
	var commitment map[string]any
	decodeJSON(t, resp, &commitment)
	f.id = commitment["id"].(string)
	return f
}

func (f *reconciliationFixture) occurrencesURL(from, to string) string {
	return fmt.Sprintf("%s/api/recurring-commitments/%s/occurrences?from=%s&to=%s", f.url, f.id, from, to)
}

func (f *reconciliationFixture) reconciliationURL(date string) string {
	return fmt.Sprintf("%s/api/recurring-commitments/%s/occurrences/%s/reconciliation", f.url, f.id, date)
}

// addTransaction inserts one transaction on occurredAt. The movement type
// follows the amount's sign, because classification comes from movement_type
// (money.Classify) and not from the sign — a negative amount tagged CREDIT
// would read as an inflow, which is the wrong fixture for an expense.
func (f *reconciliationFixture) addTransaction(amount, occurredAt string) string {
	f.t.Helper()
	movementType := "CREDIT"
	if strings.HasPrefix(amount, "-") {
		movementType = "DEBIT"
	}
	now := db.FormatTime(time.Now())
	sourceID, syncRunID, rawImportID, accountID, txID :=
		uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec := func(query string, args ...any) {
		f.t.Helper()
		if _, err := f.conn.Exec(query, args...); err != nil {
			f.t.Fatalf("exec %q: %v", query, err)
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
			id, source_id, external_id, name, currency_code, current_raw_import_id, normalized_hash,
			created_at, updated_at
		) VALUES (?, ?, ?, 'Conta Corrente', 'BRL', ?, 'hash', ?, ?)`,
		accountID, sourceID, accountID, rawImportID, now, now)
	exec(`INSERT INTO financial_transactions (
			id, source_id, account_id, external_id, description, amount, amount_in_account_currency,
			currency_code, occurred_at, provider_status, movement_type, current_raw_import_id,
			normalized_hash, created_at, updated_at
		) VALUES (?, ?, ?, ?, 'ALUGUEL IMOBILIARIA', ?, ?, 'BRL', ?, 'POSTED', ?, ?, 'hash', ?, ?)`,
		txID, sourceID, accountID, txID, amount, amount,
		db.FormatTime(mustParseDay(f.t, occurredAt)), movementType, rawImportID, now, now)
	return txID
}

func mustParseDay(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		t.Fatalf("parse %q: %v", value, err)
	}
	return parsed
}

func TestListOccurrencesReportsUnreconciledSchedule(t *testing.T) {
	f := newReconciliationFixture(t)

	resp := doJSON(t, http.MethodGet, f.occurrencesURL("2026-01-01", "2026-03-31"), nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var occurrences []map[string]any
	decodeJSON(t, resp, &occurrences)

	if len(occurrences) != 3 {
		t.Fatalf("got %d occurrences, want 3", len(occurrences))
	}
	// Newest first: the occurrence a user comes to check is the recent one.
	if occurrences[0]["date"] != "2026-03-05" || occurrences[2]["date"] != "2026-01-05" {
		t.Errorf("unexpected ordering: %+v", occurrences)
	}
	for _, occurrence := range occurrences {
		if occurrence["status"] != "unreconciled" || occurrence["origin"] != nil {
			t.Errorf("occurrence = %+v, want unreconciled with no origin (no rule, no override)", occurrence)
		}
		if occurrence["expected_amount"] != "1500.00" {
			t.Errorf("expected_amount = %v, want 1500.00", occurrence["expected_amount"])
		}
	}
}

func TestReconcileOccurrenceManuallyThenDetachThenRestore(t *testing.T) {
	f := newReconciliationFixture(t)
	txID := f.addTransaction("-1490.00", "2026-02-06")

	resp := doJSON(t, http.MethodPut, f.reconciliationURL("2026-02-05"), map[string]any{
		"state": "linked", "transaction_id": txID,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("link status = %d, want 200", resp.StatusCode)
	}
	var linked map[string]any
	decodeJSON(t, resp, &linked)
	if linked["status"] != "reconciled" || linked["origin"] != "manual" {
		t.Fatalf("linked = %+v, want reconciled/manual", linked)
	}
	transaction, ok := linked["transaction"].(map[string]any)
	if !ok || transaction["id"] != txID {
		t.Fatalf("linked transaction = %+v, want %s attached", linked["transaction"], txID)
	}
	if transaction["description"] != "ALUGUEL IMOBILIARIA" || transaction["account_name"] != "Conta Corrente" {
		t.Errorf("transaction display fields = %+v", transaction)
	}
	// The value is reported as a magnitude, matching how the payables
	// candidate DTO reports one.
	if money, ok := transaction["effective_money"].(map[string]any); !ok || money["value"] != "1490.00" {
		t.Errorf("effective_money = %+v, want magnitude 1490.00", transaction["effective_money"])
	}

	// Detaching replaces the link — one row, one decision.
	resp = doJSON(t, http.MethodPut, f.reconciliationURL("2026-02-05"), map[string]any{"state": "detached"})
	if resp.StatusCode != 200 {
		t.Fatalf("detach status = %d, want 200", resp.StatusCode)
	}
	var detached map[string]any
	decodeJSON(t, resp, &detached)
	if detached["status"] != "detached" || detached["transaction"] != nil {
		t.Fatalf("detached = %+v, want detached with no transaction", detached)
	}

	// "Voltar ao automático" forgets the decision entirely.
	resp = doJSON(t, http.MethodDelete, f.reconciliationURL("2026-02-05"), nil)
	if resp.StatusCode != 204 {
		t.Fatalf("delete status = %d, want 204", resp.StatusCode)
	}
	resp = doJSON(t, http.MethodGet, f.occurrencesURL("2026-02-01", "2026-02-28"), nil)
	var occurrences []map[string]any
	decodeJSON(t, resp, &occurrences)
	if len(occurrences) != 1 || occurrences[0]["status"] != "unreconciled" {
		t.Errorf("after restore = %+v, want a plain unreconciled occurrence", occurrences)
	}
}

func TestDeleteReconciliationWithoutADecisionIs404(t *testing.T) {
	f := newReconciliationFixture(t)

	resp := doJSON(t, http.MethodDelete, f.reconciliationURL("2026-02-05"), nil)
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// An occurrence has no identity beyond its date, so a date the schedule never
// produces would create an override no read would ever resolve.
func TestReconcileRejectsADateThatIsNotAnOccurrence(t *testing.T) {
	f := newReconciliationFixture(t)
	txID := f.addTransaction("-1490.00", "2026-02-06")

	resp := doJSON(t, http.MethodPut, f.reconciliationURL("2026-02-17"), map[string]any{
		"state": "linked", "transaction_id": txID,
	})
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
}

func TestReconcileRejectsAMalformedDate(t *testing.T) {
	f := newReconciliationFixture(t)

	resp := doJSON(t, http.MethodPut, f.reconciliationURL("05-02-2026"), map[string]any{"state": "detached"})
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
}

func TestReconcileRejectsAnInflowForAnExpenseCommitment(t *testing.T) {
	f := newReconciliationFixture(t)
	inflow := f.addTransaction("1490.00", "2026-02-06")

	resp := doJSON(t, http.MethodPut, f.reconciliationURL("2026-02-05"), map[string]any{
		"state": "linked", "transaction_id": inflow,
	})
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	var problem map[string]any
	decodeJSON(t, resp, &problem)
	if problem["type"] != "ineligible-transaction" && problem["title"] != "Transação inelegível" {
		t.Errorf("problem = %+v", problem)
	}
}

// One transaction settles at most one occurrence, or the same money would
// suppress two projections.
func TestReconcileRejectsATransactionAnotherOccurrenceClaims(t *testing.T) {
	f := newReconciliationFixture(t)
	txID := f.addTransaction("-1490.00", "2026-02-06")

	if resp := doJSON(t, http.MethodPut, f.reconciliationURL("2026-02-05"), map[string]any{
		"state": "linked", "transaction_id": txID,
	}); resp.StatusCode != 200 {
		t.Fatalf("first link status = %d, want 200", resp.StatusCode)
	}
	resp := doJSON(t, http.MethodPut, f.reconciliationURL("2026-03-05"), map[string]any{
		"state": "linked", "transaction_id": txID,
	})
	if resp.StatusCode != 409 {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
}

// Re-linking the same transaction to the same occurrence is a replacement,
// not a conflict — otherwise a double submit from the UI would fail.
func TestReconcileTheSameTransactionToTheSameOccurrenceIsIdempotent(t *testing.T) {
	f := newReconciliationFixture(t)
	txID := f.addTransaction("-1490.00", "2026-02-06")

	for attempt := 1; attempt <= 2; attempt++ {
		resp := doJSON(t, http.MethodPut, f.reconciliationURL("2026-02-05"), map[string]any{
			"state": "linked", "transaction_id": txID,
		})
		if resp.StatusCode != 200 {
			t.Fatalf("attempt %d status = %d, want 200", attempt, resp.StatusCode)
		}
	}
}

func TestReconcileRejectsAnUnknownState(t *testing.T) {
	f := newReconciliationFixture(t)

	resp := doJSON(t, http.MethodPut, f.reconciliationURL("2026-02-05"), map[string]any{"state": "maybe"})
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
}

func TestReconcileRejectsAMissingTransaction(t *testing.T) {
	f := newReconciliationFixture(t)

	resp := doJSON(t, http.MethodPut, f.reconciliationURL("2026-02-05"), map[string]any{
		"state": "linked", "transaction_id": uuid.NewString(),
	})
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestListCandidatesRanksTheClosestTransactionFirst(t *testing.T) {
	f := newReconciliationFixture(t)
	near := f.addTransaction("-1450.00", "2026-02-04")
	far := f.addTransaction("-1500.00", "2026-02-25")

	resp := doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/recurring-commitments/%s/occurrences/2026-02-05/candidates", f.url, f.id), nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var candidates []map[string]any
	decodeJSON(t, resp, &candidates)
	if len(candidates) != 2 {
		t.Fatalf("got %d candidates, want 2: %+v", len(candidates), candidates)
	}
	if candidates[0]["id"] != near {
		t.Errorf("first candidate = %v, want the one nearest the occurrence (%s)", candidates[0]["id"], near)
	}
	if candidates[1]["id"] != far {
		t.Errorf("second candidate = %v, want %s", candidates[1]["id"], far)
	}
}

func TestListCandidatesExcludesAlreadyReconciledTransactions(t *testing.T) {
	f := newReconciliationFixture(t)
	taken := f.addTransaction("-1450.00", "2026-02-04")
	free := f.addTransaction("-1500.00", "2026-02-06")

	if resp := doJSON(t, http.MethodPut, f.reconciliationURL("2026-03-05"), map[string]any{
		"state": "linked", "transaction_id": taken,
	}); resp.StatusCode != 200 {
		t.Fatalf("link status = %d, want 200", resp.StatusCode)
	}

	resp := doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/recurring-commitments/%s/occurrences/2026-02-05/candidates", f.url, f.id), nil)
	var candidates []map[string]any
	decodeJSON(t, resp, &candidates)
	if len(candidates) != 1 || candidates[0]["id"] != free {
		t.Errorf("candidates = %+v, want only the unclaimed %s", candidates, free)
	}
}

func TestListCandidatesFiltersBySearch(t *testing.T) {
	f := newReconciliationFixture(t)
	f.addTransaction("-1450.00", "2026-02-04")

	resp := doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/recurring-commitments/%s/occurrences/2026-02-05/candidates?search=imobiliaria", f.url, f.id), nil)
	var matching []map[string]any
	decodeJSON(t, resp, &matching)
	if len(matching) != 1 {
		t.Errorf("got %d matches for a substring of the description, want 1", len(matching))
	}

	resp = doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/recurring-commitments/%s/occurrences/2026-02-05/candidates?search=supermercado", f.url, f.id), nil)
	var none []map[string]any
	decodeJSON(t, resp, &none)
	if len(none) != 0 {
		t.Errorf("got %d matches for an absent term, want 0", len(none))
	}
}

func TestOccurrenceEndpointsRejectAnUnknownCommitment(t *testing.T) {
	srv, _ := newTestServer(t)
	missing := uuid.NewString()

	for _, target := range []struct {
		method string
		url    string
		body   any
	}{
		{http.MethodGet, srv.URL + "/api/recurring-commitments/" + missing + "/occurrences", nil},
		{http.MethodGet, srv.URL + "/api/recurring-commitments/" + missing + "/occurrences/2026-02-05/candidates", nil},
		{http.MethodPut, srv.URL + "/api/recurring-commitments/" + missing + "/occurrences/2026-02-05/reconciliation", map[string]any{"state": "detached"}},
		{http.MethodDelete, srv.URL + "/api/recurring-commitments/" + missing + "/occurrences/2026-02-05/reconciliation", nil},
	} {
		resp := doJSON(t, target.method, target.url, target.body)
		if resp.StatusCode != 404 {
			t.Errorf("%s %s status = %d, want 404", target.method, target.url, resp.StatusCode)
		}
	}
}

// A paused commitment produces no occurrences at all, so the 422 has to
// explain that rather than claim the date is wrong.
func TestReconcileRejectsAPausedCommitment(t *testing.T) {
	f := newReconciliationFixture(t)
	if resp := doJSON(t, http.MethodPatch, f.url+"/api/recurring-commitments/"+f.id,
		map[string]any{"is_active": false}); resp.StatusCode != 200 {
		t.Fatalf("pause status = %d, want 200", resp.StatusCode)
	}

	resp := doJSON(t, http.MethodPut, f.reconciliationURL("2026-02-05"), map[string]any{"state": "detached"})
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	var problem map[string]any
	decodeJSON(t, resp, &problem)
	if problem["title"] != "Compromisso pausado" {
		t.Errorf("problem = %+v, want the paused explanation", problem)
	}
}

// The occurrence a transaction settles is discoverable from the transaction
// side too — the drawer's entry point.
func TestTransactionReconciliationReportsCurrentAndOptions(t *testing.T) {
	f := newReconciliationFixture(t)
	txID := f.addTransaction("-1490.00", "2026-02-06")

	resp := doJSON(t, http.MethodGet, f.url+"/api/transactions/"+txID+"/reconciliation", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var before map[string]any
	decodeJSON(t, resp, &before)
	if before["current"] != nil {
		t.Errorf("current = %+v, want nil before any reconciliation", before["current"])
	}
	options := before["options"].([]any)
	if len(options) == 0 {
		t.Fatal("expected nearby occurrences to be offered")
	}
	// Nearest occurrence first: the transaction is on 2026-02-06, day 5 is
	// the schedule.
	first := options[0].(map[string]any)
	if first["occurrence_date"] != "2026-02-05" || first["commitment_id"] != f.id {
		t.Errorf("first option = %+v, want the 2026-02-05 occurrence", first)
	}

	if resp := doJSON(t, http.MethodPut, f.reconciliationURL("2026-02-05"), map[string]any{
		"state": "linked", "transaction_id": txID,
	}); resp.StatusCode != 200 {
		t.Fatalf("link status = %d, want 200", resp.StatusCode)
	}

	resp = doJSON(t, http.MethodGet, f.url+"/api/transactions/"+txID+"/reconciliation", nil)
	var after map[string]any
	decodeJSON(t, resp, &after)
	current, ok := after["current"].(map[string]any)
	if !ok {
		t.Fatalf("current = %+v, want the linked occurrence", after["current"])
	}
	if current["occurrence_date"] != "2026-02-05" || current["origin"] != "manual" {
		t.Errorf("current = %+v", current)
	}
	for _, option := range after["options"].([]any) {
		if option.(map[string]any)["occurrence_date"] == "2026-02-05" {
			t.Error("the occurrence this transaction already settles must not also be offered")
		}
	}
}

// An expense commitment can never be settled by an inflow, so it must not
// even be offered.
func TestTransactionReconciliationOffersOnlyMatchingDirections(t *testing.T) {
	f := newReconciliationFixture(t)
	inflow := f.addTransaction("1490.00", "2026-02-06")

	resp := doJSON(t, http.MethodGet, f.url+"/api/transactions/"+inflow+"/reconciliation", nil)
	var body map[string]any
	decodeJSON(t, resp, &body)
	if len(body["options"].([]any)) != 0 {
		t.Errorf("options = %+v, want none for an inflow against an expense commitment", body["options"])
	}
}

func TestTransactionReconciliationRejectsAnUnknownTransaction(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodGet, srv.URL+"/api/transactions/"+uuid.NewString()+"/reconciliation", nil)
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// Ignoring a transaction excludes it from the totals, so it can no longer be
// what carries a commitment's money — the same cleanup payable links get.
func TestIgnoringATransactionDropsItsReconciliation(t *testing.T) {
	f := newReconciliationFixture(t)
	txID := f.addTransaction("-1490.00", "2026-02-06")

	if resp := doJSON(t, http.MethodPut, f.reconciliationURL("2026-02-05"), map[string]any{
		"state": "linked", "transaction_id": txID,
	}); resp.StatusCode != 200 {
		t.Fatalf("link status = %d, want 200", resp.StatusCode)
	}
	if resp := doJSON(t, http.MethodPut, f.url+"/api/transactions/"+txID+"/inclusion",
		map[string]any{"state": "ignored"}); resp.StatusCode != 200 {
		t.Fatalf("ignore status = %d, want 200", resp.StatusCode)
	}

	resp := doJSON(t, http.MethodGet, f.occurrencesURL("2026-02-01", "2026-02-28"), nil)
	var occurrences []map[string]any
	decodeJSON(t, resp, &occurrences)
	if len(occurrences) != 1 || occurrences[0]["status"] != "unreconciled" {
		t.Errorf("occurrences = %+v, want the link dropped by the ignore hook", occurrences)
	}
}

// The Recorrências view and the report must agree about the same fact: an
// ignored transaction is excluded from the totals, so the automation rule
// must not match it here either.
func TestListOccurrencesIgnoresTransactionsExcludedFromTotals(t *testing.T) {
	f := newReconciliationFixture(t)
	txID := f.addTransaction("-1490.00", "2026-02-06")

	if resp := doJSON(t, http.MethodPut, f.url+"/api/transactions/"+txID+"/inclusion",
		map[string]any{"state": "ignored"}); resp.StatusCode != 200 {
		t.Fatalf("ignore status = %d, want 200", resp.StatusCode)
	}

	resp := doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/recurring-commitments/%s/occurrences/2026-02-05/candidates", f.url, f.id), nil)
	var candidates []map[string]any
	decodeJSON(t, resp, &candidates)
	if len(candidates) != 0 {
		t.Errorf("candidates = %+v, want none: an ignored transaction settles nothing", candidates)
	}
}

func TestTransactionReconciliationOffersNothingForAnIgnoredTransaction(t *testing.T) {
	f := newReconciliationFixture(t)
	txID := f.addTransaction("-1490.00", "2026-02-06")

	if resp := doJSON(t, http.MethodPut, f.url+"/api/transactions/"+txID+"/inclusion",
		map[string]any{"state": "ignored"}); resp.StatusCode != 200 {
		t.Fatalf("ignore status = %d, want 200", resp.StatusCode)
	}

	resp := doJSON(t, http.MethodGet, f.url+"/api/transactions/"+txID+"/reconciliation", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	decodeJSON(t, resp, &body)
	if body["current"] != nil || len(body["options"].([]any)) != 0 {
		t.Errorf("body = %+v, want nothing offered for an ignored transaction", body)
	}
}
