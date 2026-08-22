package httpapi_test

import (
	"net/http"
	"testing"
	"time"
)

const categorySupermercadoID = "000433b6-3094-5a9c-87df-465b70574a4b"
const categorySalarioID = "3c5a9586-2a11-556d-b014-692ed51c3997"

func TestTimelineOverHTTP(t *testing.T) {
	srv, conn := newTestServer(t)

	groceriesID := insertTransaction(t, conn)
	setCardDebtTransaction(t, conn, groceriesID, time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC), "100.00", "DEBIT", nil)
	resp := doJSON(t, http.MethodPut, srv.URL+"/api/transactions/"+groceriesID+"/category",
		map[string]any{"category_id": categorySupermercadoID})
	if resp.StatusCode != 200 {
		t.Fatalf("set category status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	salaryID := insertTransaction(t, conn)
	setCardDebtTransaction(t, conn, salaryID, time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC), "2000.00", "CREDIT", nil)
	resp = doJSON(t, http.MethodPut, srv.URL+"/api/transactions/"+salaryID+"/category",
		map[string]any{"category_id": categorySalarioID})
	if resp.StatusCode != 200 {
		t.Fatalf("set category status = %d, want 200", resp.StatusCode)
	}
	resp.Body.Close()

	var accountID string
	if err := conn.QueryRow(`SELECT account_id FROM financial_transactions WHERE id = ?`, groceriesID).Scan(&accountID); err != nil {
		t.Fatalf("account id: %v", err)
	}
	if _, err := conn.Exec(`UPDATE financial_accounts SET balance = '5000.00' WHERE id = ?`, accountID); err != nil {
		t.Fatalf("set balance: %v", err)
	}

	resp = doJSON(t, http.MethodGet,
		srv.URL+"/api/timeline?reference_date=2026-08-15&from=2026-08-01&to=2026-08-31", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("get timeline status = %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	decodeJSON(t, resp, &body)

	base := body["base"].(map[string]any)
	if base["starting_balance"] != "5000.00" {
		t.Errorf("starting_balance = %v, want 5000.00", base["starting_balance"])
	}
	entries := base["entries"].([]any)
	if len(entries) != 2 {
		t.Fatalf("entries = %+v, want 2", entries)
	}

	// Reconciliation: sum of category_breakdown[].Amount for the expense
	// side must equal the sum of the real Entries in the same category —
	// no presentation layer is allowed to recompute this independently.
	categoryBreakdown := body["category_breakdown"].([]any)
	var groceriesImpact string
	for _, raw := range categoryBreakdown {
		row := raw.(map[string]any)
		if row["category_id"] == categorySupermercadoID {
			groceriesImpact = row["amount"].(string)
		}
	}
	if groceriesImpact != "100.00" {
		t.Errorf("category_breakdown Supermercado amount = %q, want 100.00 (matching the entry's magnitude)", groceriesImpact)
	}

	monthlyBreakdown := body["monthly_breakdown"].([]any)
	if len(monthlyBreakdown) != 1 {
		t.Fatalf("monthly_breakdown = %+v, want 1 month", monthlyBreakdown)
	}
	august := monthlyBreakdown[0].(map[string]any)
	if august["income"] != "2000.00" || august["expense"] != "100.00" || august["result"] != "1900.00" {
		t.Errorf("august summary = %+v", august)
	}
}

func TestTimelineWithScenarioIDsReturnsSimulationAndImpacts(t *testing.T) {
	srv, conn := newTestServer(t)

	txID := insertTransaction(t, conn)
	// Pin occurred_at before reference_date, the way every other timeline
	// test here does: insertTransaction defaults to time.Now(), and a real
	// transaction landing after the reference date would shift the base
	// series' final balance, making the scenario no longer "the only source
	// of difference" this test asserts on.
	setCardDebtTransaction(t, conn, txID, time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC), "-42.00", "DEBIT", nil)
	var accountID string
	if err := conn.QueryRow(`SELECT account_id FROM financial_transactions WHERE id = ?`, txID).Scan(&accountID); err != nil {
		t.Fatalf("account id: %v", err)
	}
	if _, err := conn.Exec(`UPDATE financial_accounts SET balance = '5000.00' WHERE id = ?`, accountID); err != nil {
		t.Fatalf("set balance: %v", err)
	}

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/scenarios", map[string]any{"name": "Viagem"})
	if resp.StatusCode != 201 {
		t.Fatalf("create scenario status = %d, want 201", resp.StatusCode)
	}
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions", map[string]any{
		"description": "Passagem", "amount": "-300.00", "projected_at": "2026-08-20",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create scenario transaction status = %d, want 201", resp.StatusCode)
	}
	resp.Body.Close()

	// Without scenario_ids: simulation is null, no impacts.
	resp = doJSON(t, http.MethodGet,
		srv.URL+"/api/timeline?reference_date=2026-08-15&from=2026-08-01&to=2026-08-31", nil)
	var withoutScenario map[string]any
	decodeJSON(t, resp, &withoutScenario)
	if withoutScenario["simulation"] != nil {
		t.Errorf("simulation = %v, want nil", withoutScenario["simulation"])
	}
	if impacts := withoutScenario["scenario_impacts"].([]any); len(impacts) != 0 {
		t.Errorf("scenario_impacts = %+v, want empty", impacts)
	}

	// With scenario_ids: simulation reflects the scenario, and the impact
	// equals simulation.final_balance - base.final_balance (-300 here,
	// since the scenario is the only source of difference).
	resp = doJSON(t, http.MethodGet,
		srv.URL+"/api/timeline?reference_date=2026-08-15&from=2026-08-01&to=2026-08-31&scenario_ids="+scenarioID, nil)
	var withScenario map[string]any
	decodeJSON(t, resp, &withScenario)
	simulation := withScenario["simulation"].(map[string]any)
	simulationPoints := simulation["points"].([]any)
	basePoints := withScenario["base"].(map[string]any)["points"].([]any)
	simulationFinal := simulationPoints[len(simulationPoints)-1].(map[string]any)["balance"].(string)
	baseFinal := basePoints[len(basePoints)-1].(map[string]any)["balance"].(string)
	if simulationFinal != "4700.00" || baseFinal != "5000.00" {
		t.Errorf("simulationFinal = %q, baseFinal = %q, want 4700.00 and 5000.00", simulationFinal, baseFinal)
	}

	impacts := withScenario["scenario_impacts"].([]any)
	if len(impacts) != 1 {
		t.Fatalf("scenario_impacts = %+v, want 1", impacts)
	}
	impact := impacts[0].(map[string]any)
	if impact["scenario_id"] != scenarioID || impact["delta"] != "-300.00" {
		t.Errorf("impact = %+v, want delta -300.00", impact)
	}
}

func TestTimelineMonthOverMonthWithoutPreviousMonthIsNull(t *testing.T) {
	srv, conn := newTestServer(t)
	groceriesID := insertTransaction(t, conn)
	setCardDebtTransaction(t, conn, groceriesID, time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC), "100.00", "DEBIT", nil)

	resp := doJSON(t, http.MethodGet,
		srv.URL+"/api/timeline?reference_date=2026-08-15&from=2026-08-01&to=2026-08-31", nil)
	var body map[string]any
	decodeJSON(t, resp, &body)
	if body["month_over_month"] != nil {
		t.Errorf("month_over_month = %v, want nil (no July data at all)", body["month_over_month"])
	}
}

func TestTimelineMonthOverMonthWithPreviousMonth(t *testing.T) {
	srv, conn := newTestServer(t)
	julyID := insertTransaction(t, conn)
	setCardDebtTransaction(t, conn, julyID, time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC), "200.00", "CREDIT", nil)
	augustID := insertTransaction(t, conn)
	setCardDebtTransaction(t, conn, augustID, time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC), "300.00", "CREDIT", nil)

	resp := doJSON(t, http.MethodGet,
		srv.URL+"/api/timeline?reference_date=2026-08-15&from=2026-07-01&to=2026-08-31", nil)
	var body map[string]any
	decodeJSON(t, resp, &body)
	mom := body["month_over_month"].(map[string]any)
	if mom["current"] != "300.00" || mom["previous"] != "200.00" {
		t.Errorf("month_over_month = %+v, want current=300.00 previous=200.00", mom)
	}
}

func TestTimelineYearOverYearOnlyComputedWhenRequested(t *testing.T) {
	srv, conn := newTestServer(t)
	txID := insertTransaction(t, conn)
	setCardDebtTransaction(t, conn, txID, time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC), "100.00", "CREDIT", nil)

	resp := doJSON(t, http.MethodGet,
		srv.URL+"/api/timeline?reference_date=2026-08-15&from=2026-01-01&to=2026-12-31", nil)
	var withoutFlag map[string]any
	decodeJSON(t, resp, &withoutFlag)
	if withoutFlag["year_over_year"] != nil {
		t.Errorf("year_over_year without the flag = %v, want nil", withoutFlag["year_over_year"])
	}

	resp = doJSON(t, http.MethodGet,
		srv.URL+"/api/timeline?reference_date=2026-08-15&from=2026-01-01&to=2026-12-31&year_over_year=true", nil)
	var withFlag map[string]any
	decodeJSON(t, resp, &withFlag)
	// No 2025 data exists, so even with the flag it must stay nil rather
	// than compare against a false zero.
	if withFlag["year_over_year"] != nil {
		t.Errorf("year_over_year with no 2025 data = %v, want nil", withFlag["year_over_year"])
	}
}

func TestTimelineCategoryEvolutionOnlyComputedWhenRequested(t *testing.T) {
	srv, conn := newTestServer(t)
	groceriesID := insertTransaction(t, conn)
	setCardDebtTransaction(t, conn, groceriesID, time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC), "100.00", "DEBIT", nil)
	resp := doJSON(t, http.MethodPut, srv.URL+"/api/transactions/"+groceriesID+"/category",
		map[string]any{"category_id": categorySupermercadoID})
	resp.Body.Close()

	resp = doJSON(t, http.MethodGet,
		srv.URL+"/api/timeline?reference_date=2026-08-15&from=2026-08-01&to=2026-08-31", nil)
	var withoutParam map[string]any
	decodeJSON(t, resp, &withoutParam)
	if withoutParam["category_evolution"] != nil {
		t.Errorf("category_evolution without the param = %v, want nil", withoutParam["category_evolution"])
	}

	resp = doJSON(t, http.MethodGet,
		srv.URL+"/api/timeline?reference_date=2026-08-15&from=2026-08-01&to=2026-08-31&category_evolution_id="+categorySupermercadoID, nil)
	var withParam map[string]any
	decodeJSON(t, resp, &withParam)
	evolution := withParam["category_evolution"].([]any)
	if len(evolution) != 1 {
		t.Fatalf("category_evolution = %+v, want 1 month", evolution)
	}
	august := evolution[0].(map[string]any)
	if august["amount"] != "100.00" {
		t.Errorf("category_evolution august amount = %v, want 100.00", august["amount"])
	}
}

func TestTimelineRejectsMissingParams(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := doJSON(t, http.MethodGet, srv.URL+"/api/timeline", nil)
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestTimelineRejectsToBeforeFrom(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := doJSON(t, http.MethodGet,
		srv.URL+"/api/timeline?reference_date=2026-08-15&from=2026-08-31&to=2026-08-01", nil)
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	resp.Body.Close()
}
