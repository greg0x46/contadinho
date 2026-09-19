package httpapi_test

import (
	"net/http"
	"testing"
	"time"
)

const categorySupermercadoID = "000433b6-3094-5a9c-87df-465b70574a4b"
const categorySalarioID = "3c5a9586-2a11-556d-b014-692ed51c3997"

func TestHistoricalTimelineReturnsAValidLowestPoint(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, window := range []struct{ from, to string }{
		{"2026-08-01", "2026-08-31"},
		{"2025-01-01", "2025-12-31"},
		{"2026-08-10", "2026-08-10"},
	} {
		t.Run(window.from+"/"+window.to, func(t *testing.T) {
			resp := doJSON(t, http.MethodGet, srv.URL+"/api/timeline?reference_date=2026-09-08&from="+window.from+"&to="+window.to+"&aggregations=false", nil)
			if resp.StatusCode != 200 {
				t.Fatalf("status = %d", resp.StatusCode)
			}
			var body map[string]any
			decodeJSON(t, resp, &body)
			base := body["base"].(map[string]any)
			lowest := base["lowest_balance"].(map[string]any)
			if lowest["date"] != window.from || lowest["lowest_tier"] != "realizado" || lowest["balance"] != "0" {
				t.Fatalf("invalid historical minimum: %+v", lowest)
			}
			if base["first_negative"] != nil {
				t.Fatalf("historical window has a future warning: %v", base["first_negative"])
			}
		})
	}
}

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

	// The requested window is exactly one calendar month here, so
	// period_totals must agree with monthly_breakdown's single row.
	periodTotals := body["period_totals"].(map[string]any)
	if periodTotals["income"] != "2000.00" || periodTotals["expense"] != "100.00" || periodTotals["result"] != "1900.00" {
		t.Errorf("period_totals = %+v", periodTotals)
	}
}

// aggregations=false is what the Home dashboard sends: it plots the balance
// curve alone, so the breakdowns are pure payload for it.
func TestTimelineAggregationsFalseKeepsTheSeriesAndDropsTheBreakdowns(t *testing.T) {
	srv, conn := newTestServer(t)

	groceriesID := insertTransaction(t, conn)
	setCardDebtTransaction(t, conn, groceriesID, time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC), "100.00", "DEBIT", nil)

	var accountID string
	if err := conn.QueryRow(`SELECT account_id FROM financial_transactions WHERE id = ?`, groceriesID).Scan(&accountID); err != nil {
		t.Fatalf("account id: %v", err)
	}
	if _, err := conn.Exec(`UPDATE financial_accounts SET balance = '5000.00' WHERE id = ?`, accountID); err != nil {
		t.Fatalf("set balance: %v", err)
	}

	resp := doJSON(t, http.MethodGet,
		srv.URL+"/api/timeline?reference_date=2026-08-15&from=2026-08-01&to=2026-08-31&aggregations=false", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("get timeline status = %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	decodeJSON(t, resp, &body)

	base := body["base"].(map[string]any)
	if base["starting_balance"] != "5000.00" {
		t.Errorf("starting_balance = %v, want 5000.00", base["starting_balance"])
	}
	if points := base["points"].([]any); len(points) != 31 {
		t.Errorf("points = %d, want 31 — the series itself must not be affected", len(points))
	}
	if entries := base["entries"].([]any); len(entries) != 1 {
		t.Errorf("entries = %+v, want 1", entries)
	}
	if breakdown := body["monthly_breakdown"].([]any); len(breakdown) != 0 {
		t.Errorf("monthly_breakdown = %+v, want empty", breakdown)
	}
	if breakdown := body["category_breakdown"].([]any); len(breakdown) != 0 {
		t.Errorf("category_breakdown = %+v, want empty", breakdown)
	}
	if body["month_over_month"] != nil {
		t.Errorf("month_over_month = %+v, want null", body["month_over_month"])
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

// analysis_month is what the report's month navigator moves; reference_date
// stays pinned to today because it is the balance anchor. Before the
// parameter existed the aggregations followed reference_date, so browsing
// to a past month showed the *current* month's category breakdown under
// the past month's label.
func TestTimelineAnalysisMonthMovesAggregationsNotBalance(t *testing.T) {
	srv, conn := newTestServer(t)

	categorize := func(id string) {
		resp := doJSON(t, http.MethodPut, srv.URL+"/api/transactions/"+id+"/category",
			map[string]any{"category_id": categorySupermercadoID})
		if resp.StatusCode != 200 {
			t.Fatalf("set category status = %d, want 200", resp.StatusCode)
		}
		resp.Body.Close()
	}

	juneID := insertTransaction(t, conn)
	setCardDebtTransaction(t, conn, juneID, time.Date(2026, 6, 5, 0, 0, 0, 0, time.UTC), "100.00", "DEBIT", nil)
	categorize(juneID)
	julyID := insertTransaction(t, conn)
	setCardDebtTransaction(t, conn, julyID, time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC), "200.00", "DEBIT", nil)
	categorize(julyID)
	augustID := insertTransaction(t, conn)
	setCardDebtTransaction(t, conn, augustID, time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC), "300.00", "DEBIT", nil)
	categorize(augustID)

	var accountID string
	if err := conn.QueryRow(`SELECT account_id FROM financial_transactions WHERE id = ?`, augustID).Scan(&accountID); err != nil {
		t.Fatalf("account id: %v", err)
	}
	if _, err := conn.Exec(`UPDATE financial_accounts SET balance = '5000.00' WHERE id = ?`, accountID); err != nil {
		t.Fatalf("set balance: %v", err)
	}

	groceriesAmount := func(body map[string]any) string {
		for _, raw := range body["category_breakdown"].([]any) {
			row := raw.(map[string]any)
			if row["category_id"] == categorySupermercadoID {
				return row["amount"].(string)
			}
		}
		return ""
	}

	const window = "reference_date=2026-08-15&from=2026-06-01&to=2026-08-31"

	resp := doJSON(t, http.MethodGet, srv.URL+"/api/timeline?"+window, nil)
	var withoutParam map[string]any
	decodeJSON(t, resp, &withoutParam)
	if got := groceriesAmount(withoutParam); got != "300.00" {
		t.Errorf("category_breakdown without analysis_month = %q, want 300.00 (reference_date's month)", got)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/timeline?"+window+"&analysis_month=2026-07-01&year_over_year=true", nil)
	var july map[string]any
	decodeJSON(t, resp, &july)

	if got := groceriesAmount(july); got != "200.00" {
		t.Errorf("category_breakdown with analysis_month=2026-07 = %q, want 200.00", got)
	}
	mom := july["month_over_month"].(map[string]any)
	if mom["current"] != "-200.00" || mom["previous"] != "-100.00" {
		t.Errorf("month_over_month = %+v, want current=-200.00 previous=-100.00 (July vs June)", mom)
	}
	// The balance is anchored on reference_date, not on the month being
	// analysed — browsing to July must not rewind it.
	base := july["base"].(map[string]any)
	if base["starting_balance"] != "5000.00" {
		t.Errorf("starting_balance = %v, want 5000.00 (still anchored on reference_date)", base["starting_balance"])
	}
}

func TestTimelineRejectsInvalidAnalysisMonth(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := doJSON(t, http.MethodGet,
		srv.URL+"/api/timeline?reference_date=2026-08-15&from=2026-08-01&to=2026-08-31&analysis_month=agosto", nil)
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	resp.Body.Close()
}

// The Home dashboard's "Todo Período" option asks for these bounds instead
// of guessing a start date or requesting a decade of days to be safe.
func TestTimelineDataRangeSpansOldestTransactionToLastPlannedInstallment(t *testing.T) {
	srv, conn := newTestServer(t)

	txID := insertTransaction(t, conn)
	setCardDebtTransaction(t, conn, txID, time.Date(2024, 3, 7, 0, 0, 0, 0, time.UTC), "-42.00", "DEBIT", nil)

	resp := doJSON(t, http.MethodGet, srv.URL+"/api/timeline/range", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("get range status = %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	decodeJSON(t, resp, &body)
	if body["from"] != "2024-03-07" {
		t.Errorf("from = %v, want 2024-03-07 (the oldest transaction's day)", body["from"])
	}
	// No planned installment yet, so `to` is the current month's end — never
	// a past date, whatever the transaction history looks like.
	now := time.Now().UTC()
	monthEnd := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).AddDate(0, 1, -1).Format("2006-01-02")
	if body["to"] != monthEnd {
		t.Errorf("to = %v, want %s (this month's end)", body["to"], monthEnd)
	}

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios", map[string]any{"name": "Notebook"})
	if resp.StatusCode != 201 {
		t.Fatalf("create scenario status = %d, want 201", resp.StatusCode)
	}
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	lastInstallment := time.Date(now.Year()+2, 6, 15, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenario["id"].(string)+"/transactions", map[string]any{
		"description": "Parcela final", "amount": "-300.00", "projected_at": lastInstallment,
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create scenario transaction status = %d, want 201", resp.StatusCode)
	}
	resp.Body.Close()

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/timeline/range", nil)
	decodeJSON(t, resp, &body)
	if body["to"] != lastInstallment {
		t.Errorf("to = %v, want %s (the last planned installment)", body["to"], lastInstallment)
	}
	if body["from"] != "2024-03-07" {
		t.Errorf("from = %v, want 2024-03-07", body["from"])
	}
}

// investment_transfer_kind on a reconciled bank line is whatever the timeline
// decided from the line's classification — the handler never re-derives it
// from the amount's sign.
func TestTimelineReconciledBankLinesExposeInvestmentTransferKind(t *testing.T) {
	srv, conn := newTestServer(t)

	depositID := insertTransaction(t, conn)
	setCardDebtTransaction(t, conn, depositID, time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC), "1000.00", "DEBIT", nil)
	withdrawalID := insertTransaction(t, conn)
	setCardDebtTransaction(t, conn, withdrawalID, time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC), "300.00", "CREDIT", nil)

	now := time.Now().UTC().Format(time.RFC3339)
	mustExec := func(query string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(query, args...); err != nil {
			t.Fatalf("exec %q: %v", query, err)
		}
	}
	mustExec(`INSERT INTO investment_accounts (id,name,kind,currency_code,created_at,updated_at) VALUES ('manual','Manual','manual','BRL',?,?)`, now, now)
	mustExec(`INSERT INTO investment_operations (id,account_id,kind,occurred_on,amount,created_at,updated_at) VALUES ('deposit','manual','deposit','2026-08-05','1000',?,?)`, now, now)
	mustExec(`INSERT INTO investment_operations (id,account_id,kind,occurred_on,amount,created_at,updated_at) VALUES ('withdrawal','manual','withdrawal','2026-08-12','300',?,?)`, now, now)
	mustExec(`INSERT INTO investment_reconciliations (id,operation_id,financial_transaction_id,amount,created_at) VALUES ('link-deposit','deposit',?,'1000',?)`, depositID, now)
	mustExec(`INSERT INTO investment_reconciliations (id,operation_id,financial_transaction_id,amount,created_at) VALUES ('link-withdrawal','withdrawal',?,'300',?)`, withdrawalID, now)

	resp := doJSON(t, http.MethodGet,
		srv.URL+"/api/timeline?reference_date=2026-08-15&from=2026-08-01&to=2026-08-31", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("get timeline status = %d, want 200", resp.StatusCode)
	}
	var body map[string]any
	decodeJSON(t, resp, &body)

	kinds := map[string]any{}
	for _, raw := range body["base"].(map[string]any)["entries"].([]any) {
		entry := raw.(map[string]any)
		if entry["source"] == "real" {
			kinds[entry["source_ref_id"].(string)] = entry["investment_transfer_kind"]
		}
	}
	if kinds[depositID] != "deposit" || kinds[withdrawalID] != "withdrawal" {
		t.Fatalf("investment_transfer_kind by entry = %+v", kinds)
	}
	august := body["monthly_breakdown"].([]any)[0].(map[string]any)
	if august["investment_contributions"] != "1000" || august["investment_withdrawals"] != "300" {
		t.Errorf("august investment buckets = %+v", august)
	}
}
