package httpapi_test

import (
	"fmt"
	"net/http"
	"testing"
)

// TestRecurringScenarioLifecycleOverHTTP walks the whole recurring feature
// through its canonical routes: the commitment is created as a Scenario, its
// occurrences are listed and reconciled by that same scenario id, the generic
// planned-transaction endpoint sees the same decision, and deleting it takes
// the schedule and the decision with it.
func TestRecurringScenarioLifecycleOverHTTP(t *testing.T) {
	srv, conn := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/categories", map[string]any{
		"name": "Aluguel", "kind": "expense", "icon": "home", "color": "#495057",
	})
	var category map[string]any
	decodeJSON(t, resp, &category)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/recurring-scenarios", map[string]any{
		"name": "Aluguel", "kind": "expense", "amount": "1500.00",
		"category_id": category["id"], "account_id": nil, "cadence": "monthly",
		"day_of_month": 5, "month_of_year": nil, "start_date": "2026-01-01",
		"end_date": nil, "is_active": true,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", resp.StatusCode)
	}
	var commitment map[string]any
	decodeJSON(t, resp, &commitment)
	scenarioID := commitment["id"].(string)

	// The same id addresses it as a plain Scenario.
	resp = doJSON(t, http.MethodGet, srv.URL+"/api/scenarios/"+scenarioID, nil)
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	if scenario["kind"] != "recurring" || scenario["name"] != "Aluguel" {
		t.Fatalf("scenario = %+v, want a recurring scenario named Aluguel", scenario)
	}

	txID := insertTransaction(t, conn)
	resp = doJSON(t, http.MethodPut,
		fmt.Sprintf("%s/api/scenarios/%s/occurrences/2026-02-05/reconciliation", srv.URL, scenarioID),
		map[string]any{"state": "linked", "transaction_id": txID})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reconcile status = %d, want 200", resp.StatusCode)
	}
	var occurrence map[string]any
	decodeJSON(t, resp, &occurrence)
	if occurrence["status"] != "reconciled" || occurrence["origin"] != "manual" {
		t.Fatalf("occurrence = %+v, want reconciled/manual", occurrence)
	}

	// The generic projection reads the very same row.
	resp = doJSON(t, http.MethodGet,
		fmt.Sprintf("%s/api/scenarios/%s/planned-transactions?from=2026-02-01&to=2026-02-28", srv.URL, scenarioID), nil)
	var planned []map[string]any
	decodeJSON(t, resp, &planned)
	if len(planned) != 1 {
		t.Fatalf("planned = %+v, want one occurrence", planned)
	}
	if planned[0]["realized"] != true || planned[0]["realization_origin"] != "manual" {
		t.Errorf("planned occurrence = %+v, want realized by a manual decision", planned[0])
	}

	resp = doJSON(t, http.MethodDelete, srv.URL+"/api/recurring-scenarios/"+scenarioID, nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", resp.StatusCode)
	}
	var schedules, decisions int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM scenario_recurring_schedules WHERE scenario_id = ?`, scenarioID).Scan(&schedules); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(
		`SELECT COUNT(*) FROM scenario_realizations WHERE relation_type = 'reconciliation' AND scenario_id = ?`, scenarioID,
	).Scan(&decisions); err != nil {
		t.Fatal(err)
	}
	if schedules != 0 || decisions != 0 {
		t.Errorf("after delete: %d schedules and %d decisions survived", schedules, decisions)
	}
}
