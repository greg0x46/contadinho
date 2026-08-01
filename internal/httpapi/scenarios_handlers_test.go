package httpapi_test

import (
	"fmt"
	"net/http"
	"testing"
)

func TestScenarioLifecycleOverHTTP(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "Financiamento", "total_amount": "1200.00"})
	if resp.StatusCode != 201 {
		t.Fatalf("create debt status = %d, want 201", resp.StatusCode)
	}
	var debt map[string]any
	decodeJSON(t, resp, &debt)
	debtID := debt["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/scenarios", map[string]any{"name": "Plano de pagamento"})
	if resp.StatusCode != 201 {
		t.Fatalf("create scenario status = %d, want 201", resp.StatusCode)
	}
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)
	if scenario["kind"] != "debt_plan" || scenario["debt_id"] != debtID {
		t.Errorf("scenario = %+v", scenario)
	}
	if txs, ok := scenario["transactions"].([]any); !ok || len(txs) != 0 {
		t.Errorf("scenario transactions = %+v, want empty", scenario["transactions"])
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/debts/"+debtID+"/scenarios", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("list scenarios status = %d, want 200", resp.StatusCode)
	}
	var list []map[string]any
	decodeJSON(t, resp, &list)
	if len(list) != 1 || list[0]["id"] != scenarioID {
		t.Errorf("list = %+v", list)
	}

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions", map[string]any{
		"description": "Parcela 1", "amount": "400.00", "projected_at": "2026-09-01", "category": "Dívidas",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create scenario transaction status = %d, want 201", resp.StatusCode)
	}
	var st map[string]any
	decodeJSON(t, resp, &st)
	transactionID := st["id"].(string)
	if st["amount"] != "400.00" || st["projected_at"] != "2026-09-01" || st["category"] != "Dívidas" {
		t.Errorf("scenario transaction = %+v", st)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/scenarios/"+scenarioID, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("get scenario status = %d, want 200", resp.StatusCode)
	}
	var detail map[string]any
	decodeJSON(t, resp, &detail)
	txs, ok := detail["transactions"].([]any)
	if !ok || len(txs) != 1 {
		t.Fatalf("detail transactions = %+v", detail["transactions"])
	}

	resp = doJSON(t, http.MethodPut, srv.URL+"/api/scenarios/"+scenarioID+"/transactions/"+transactionID, map[string]any{
		"description": "Parcela 1 (editada)", "amount": "450.00", "projected_at": "2026-09-05", "category": nil,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("update scenario transaction status = %d, want 200", resp.StatusCode)
	}
	var updated map[string]any
	decodeJSON(t, resp, &updated)
	if updated["amount"] != "450.00" || updated["category"] != nil {
		t.Errorf("updated = %+v", updated)
	}

	resp = doJSON(t, http.MethodDelete, srv.URL+"/api/scenarios/"+scenarioID+"/transactions/"+transactionID, nil)
	if resp.StatusCode != 204 {
		t.Fatalf("delete scenario transaction status = %d, want 204", resp.StatusCode)
	}
	resp.Body.Close()

	resp = doJSON(t, http.MethodDelete, srv.URL+"/api/scenarios/"+scenarioID, nil)
	if resp.StatusCode != 204 {
		t.Fatalf("delete scenario status = %d, want 204", resp.StatusCode)
	}
	resp.Body.Close()

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/scenarios/"+scenarioID, nil)
	if resp.StatusCode != 404 {
		t.Fatalf("get deleted scenario status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestGenerateInstallmentsCreatesMonthlyPlanFromRemainingAmount(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "Financiamento", "total_amount": "1200.00"})
	var debt map[string]any
	decodeJSON(t, resp, &debt)
	debtID := debt["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/scenarios", map[string]any{"name": "Plano"})
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/generate-installments", map[string]any{
		"months": 3, "start_date": "2026-09-01",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var created []map[string]any
	decodeJSON(t, resp, &created)
	if len(created) != 3 {
		t.Fatalf("len(created) = %d, want 3", len(created))
	}
	sum := 0.0
	for _, ins := range created {
		var amount float64
		if _, err := fmt.Sscanf(ins["amount"].(string), "%f", &amount); err != nil {
			t.Fatalf("parse amount: %v", err)
		}
		sum += amount
	}
	if sum < 1199.99 || sum > 1200.01 {
		t.Errorf("sum of generated installments = %v, want ~1200.00", sum)
	}

	// A second call must not silently duplicate installments.
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/generate-installments", map[string]any{"months": 2})
	if resp.StatusCode != 409 {
		t.Fatalf("second generate status = %d, want 409", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestCreateScenarioRejectsUnknownDebt(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts/unknown/scenarios", map[string]any{"name": "Plano"})
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestCreateScenarioTransactionRejectsInvalidBody(t *testing.T) {
	srv, _ := newTestServer(t)
	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "Financiamento", "total_amount": "1200.00"})
	var debt map[string]any
	decodeJSON(t, resp, &debt)
	debtID := debt["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/scenarios", map[string]any{"name": "Plano"})
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions", map[string]any{
		"description": "", "amount": "400.00", "projected_at": "2026-09-01",
	})
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	resp.Body.Close()

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions", map[string]any{
		"description": "Parcela", "amount": "-1.00", "projected_at": "2026-09-01",
	})
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	resp.Body.Close()

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions", map[string]any{
		"description": "Parcela", "amount": "400.00", "projected_at": "not-a-date",
	})
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	resp.Body.Close()
}
