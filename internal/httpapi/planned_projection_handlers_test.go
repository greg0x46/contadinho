package httpapi_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestPlannedTransactionRealizationSuppressesProjectionOverHTTP(t *testing.T) {
	srv, conn := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/scenarios", map[string]any{"name": "Viagem"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create scenario status = %d, want 201", resp.StatusCode)
	}
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions", map[string]any{
		"description": "Passagem", "amount": "-100.00", "projected_at": "2026-09-01",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create planned transaction status = %d, want 201", resp.StatusCode)
	}

	plannedURL := fmt.Sprintf("%s/api/scenarios/%s/planned-transactions?from=2026-01-01&to=2026-12-31", srv.URL, scenarioID)
	resp = doJSON(t, http.MethodGet, plannedURL, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list planned transactions status = %d, want 200", resp.StatusCode)
	}
	var planned []map[string]any
	decodeJSON(t, resp, &planned)
	if len(planned) != 1 {
		t.Fatalf("planned transactions = %+v, want one", planned)
	}
	eventKey := planned[0]["event_key"].(string)

	txID := insertTransaction(t, conn)
	resp = doJSON(t, http.MethodPut, fmt.Sprintf("%s/api/scenarios/%s/planned-transactions/%s/realization", srv.URL, scenarioID, eventKey), map[string]any{
		"state": "linked", "transaction_id": txID,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("realize planned transaction status = %d, want 200", resp.StatusCode)
	}
	var realization map[string]any
	decodeJSON(t, resp, &realization)
	if realization["relation_type"] != "allocation" || realization["state"] != "linked" {
		t.Fatalf("realization = %+v", realization)
	}

	resp = doJSON(t, http.MethodGet, plannedURL, nil)
	var realized []map[string]any
	decodeJSON(t, resp, &realized)
	if len(realized) != 1 || realized[0]["realized"] != true {
		t.Fatalf("realized planned transactions = %+v", realized)
	}

	resp = doJSON(t, http.MethodGet, fmt.Sprintf(
		"%s/api/timeline?reference_date=2026-08-01&from=2026-01-01&to=2026-12-31&scenario_ids=%s",
		srv.URL, scenarioID), nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("timeline status = %d, want 200", resp.StatusCode)
	}
	var timeline map[string]any
	decodeJSON(t, resp, &timeline)
	simulation, ok := timeline["simulation"].(map[string]any)
	if !ok {
		t.Fatalf("timeline simulation = %+v", timeline["simulation"])
	}
	entries, ok := simulation["entries"].([]any)
	if !ok {
		t.Fatalf("timeline simulation entries = %+v", simulation["entries"])
	}
	for _, raw := range entries {
		entry := raw.(map[string]any)
		if entry["source"] == "cenario" {
			t.Fatalf("realized planned event still appears in timeline: %+v", entry)
		}
	}
	if len(entries) == 0 {
		t.Fatal("real transaction disappeared from timeline after realization")
	}
}

func TestScenarioActivationControlsDefaultTimelineOnly(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := doJSON(t, http.MethodPost, srv.URL+"/api/recurring-scenarios", map[string]any{
		"name": "Aluguel", "kind": "expense", "amount": "100.00",
		"category_id": "000433b6-3094-5a9c-87df-465b70574a4b", "account_id": nil,
		"cadence": "monthly", "day_of_month": 15, "month_of_year": nil,
		"start_date": "2026-01-01", "end_date": nil, "is_active": true,
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create recurring status = %d, want 201", resp.StatusCode)
	}
	var commitment map[string]any
	decodeJSON(t, resp, &commitment)
	scenarioID := commitment["id"].(string)

	timelineURL := srv.URL + "/api/timeline?reference_date=2026-08-01&from=2026-08-01&to=2026-08-31"
	containsRecurring := func() bool {
		resp := doJSON(t, http.MethodGet, timelineURL, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("timeline status = %d, want 200", resp.StatusCode)
		}
		var payload map[string]any
		decodeJSON(t, resp, &payload)
		base := payload["base"].(map[string]any)
		entries := base["entries"].([]any)
		for _, raw := range entries {
			if raw.(map[string]any)["source"] == "recorrente" {
				return true
			}
		}
		return false
	}
	if !containsRecurring() {
		t.Fatal("active recurring scenario did not appear in default timeline")
	}

	resp = doJSON(t, http.MethodPatch, srv.URL+"/api/scenarios/"+scenarioID, map[string]any{"is_active": false})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("deactivate scenario status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
	if containsRecurring() {
		t.Fatal("inactive recurring scenario still appeared in default timeline")
	}

	resp = doJSON(t, http.MethodPatch, srv.URL+"/api/scenarios/"+scenarioID, map[string]any{"is_active": true})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("reactivate scenario status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()
	if !containsRecurring() {
		t.Fatal("reactivated recurring scenario did not return to default timeline")
	}
}
