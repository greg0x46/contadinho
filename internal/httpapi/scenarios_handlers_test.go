package httpapi_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"
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

func TestScenarioTransactionStatusReflectsProjectedDate(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "Financiamento", "total_amount": "1200.00"})
	var debt map[string]any
	decodeJSON(t, resp, &debt)
	debtID := debt["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/scenarios", map[string]any{"name": "Plano"})
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)

	past := time.Now().UTC().AddDate(0, 0, -5).Format("2006-01-02")
	future := time.Now().UTC().AddDate(0, 0, 5).Format("2006-01-02")

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions", map[string]any{
		"description": "Parcela atrasada", "amount": "100.00", "projected_at": past,
	})
	var overdue map[string]any
	decodeJSON(t, resp, &overdue)
	if overdue["status"] != "atrasada" {
		t.Errorf("overdue status = %v, want atrasada", overdue["status"])
	}

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions", map[string]any{
		"description": "Parcela futura", "amount": "100.00", "projected_at": future,
	})
	var upcoming map[string]any
	decodeJSON(t, resp, &upcoming)
	if upcoming["status"] != "projetada" {
		t.Errorf("upcoming status = %v, want projetada", upcoming["status"])
	}
}

func TestRealizationAllocatesDebtLinkToInstallmentAndUpdatesStatus(t *testing.T) {
	srv, conn := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "Financiamento", "total_amount": "1200.00"})
	var debt map[string]any
	decodeJSON(t, resp, &debt)
	debtID := debt["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/scenarios", map[string]any{"name": "Plano"})
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions", map[string]any{
		"description": "Parcela 1", "amount": "42.00", "projected_at": "2026-01-01",
	})
	var st map[string]any
	decodeJSON(t, resp, &st)
	transactionID := st["id"].(string)
	if st["status"] != "atrasada" {
		t.Fatalf("initial status = %v, want atrasada", st["status"])
	}

	txID := insertTransaction(t, conn)
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/links", map[string]any{"transaction_id": txID})
	if resp.StatusCode != 201 {
		t.Fatalf("create link status = %d, want 201", resp.StatusCode)
	}
	var link map[string]any
	decodeJSON(t, resp, &link)
	linkID := link["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions/"+transactionID+"/realizations", map[string]any{
		"debt_link_id": linkID, "allocated_amount": "42.00",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create realization status = %d, want 201", resp.StatusCode)
	}
	var updated map[string]any
	decodeJSON(t, resp, &updated)
	if updated["status"] != "paga" {
		t.Errorf("status after full realization = %v, want paga", updated["status"])
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/scenarios/"+scenarioID, nil)
	var detail map[string]any
	decodeJSON(t, resp, &detail)
	txs := detail["transactions"].([]any)
	if len(txs) != 1 || txs[0].(map[string]any)["status"] != "paga" {
		t.Errorf("scenario detail transactions = %+v", txs)
	}
}

func TestRealizationRejectsDebtLinkFromAnotherDebt(t *testing.T) {
	srv, conn := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "Dívida A", "total_amount": "1000.00"})
	var debtA map[string]any
	decodeJSON(t, resp, &debtA)
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "Dívida B", "total_amount": "1000.00"})
	var debtB map[string]any
	decodeJSON(t, resp, &debtB)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtA["id"].(string)+"/scenarios", map[string]any{"name": "Plano A"})
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions", map[string]any{
		"description": "Parcela 1", "amount": "42.00", "projected_at": "2026-01-01",
	})
	var st map[string]any
	decodeJSON(t, resp, &st)
	transactionID := st["id"].(string)

	txID := insertTransaction(t, conn)
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtB["id"].(string)+"/links", map[string]any{"transaction_id": txID})
	var link map[string]any
	decodeJSON(t, resp, &link)
	linkID := link["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions/"+transactionID+"/realizations", map[string]any{
		"debt_link_id": linkID, "allocated_amount": "42.00",
	})
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestDeleteRealizationRevertsStatus(t *testing.T) {
	srv, conn := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "Financiamento", "total_amount": "1200.00"})
	var debt map[string]any
	decodeJSON(t, resp, &debt)
	debtID := debt["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/scenarios", map[string]any{"name": "Plano"})
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions", map[string]any{
		"description": "Parcela 1", "amount": "42.00", "projected_at": "2026-01-01",
	})
	var st map[string]any
	decodeJSON(t, resp, &st)
	transactionID := st["id"].(string)

	txID := insertTransaction(t, conn)
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/links", map[string]any{"transaction_id": txID})
	var link map[string]any
	decodeJSON(t, resp, &link)
	linkID := link["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions/"+transactionID+"/realizations", map[string]any{
		"debt_link_id": linkID, "allocated_amount": "42.00",
	})
	resp.Body.Close()

	// The create-realization response is the updated scenario_transaction,
	// not the realization itself, so its id isn't exposed there — read it
	// straight from the (deterministically singular) row created above.
	var storedRealizationID string
	if err := conn.QueryRow(`SELECT id FROM scenario_transaction_realizations WHERE scenario_transaction_id = ?`, transactionID).Scan(&storedRealizationID); err != nil {
		t.Fatalf("query realization id: %v", err)
	}

	resp = doJSON(t, http.MethodDelete, srv.URL+"/api/scenarios/"+scenarioID+"/transactions/"+transactionID+"/realizations/"+storedRealizationID, nil)
	if resp.StatusCode != 204 {
		t.Fatalf("delete realization status = %d, want 204", resp.StatusCode)
	}
	resp.Body.Close()

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/scenarios/"+scenarioID, nil)
	var detail map[string]any
	decodeJSON(t, resp, &detail)
	txs := detail["transactions"].([]any)
	if txs[0].(map[string]any)["status"] != "atrasada" {
		t.Errorf("status after deleting realization = %v, want atrasada", txs[0].(map[string]any)["status"])
	}
}

func TestAccumulatedDeviationSumsOnlyDueInstallments(t *testing.T) {
	srv, conn := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "Financiamento", "total_amount": "1200.00"})
	var debt map[string]any
	decodeJSON(t, resp, &debt)
	debtID := debt["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/scenarios", map[string]any{"name": "Plano"})
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)
	if scenario["accumulated_deviation"] != "0.00" {
		t.Errorf("accumulated_deviation on creation = %v, want 0.00", scenario["accumulated_deviation"])
	}

	past := time.Now().UTC().AddDate(0, 0, -10).Format("2006-01-02")
	future := time.Now().UTC().AddDate(0, 0, 10).Format("2006-01-02")

	// Due, paid only in part: contributes 100.00 - 60.00 = 40.00 to the deviation.
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions", map[string]any{
		"description": "Parcela vencida", "amount": "100.00", "projected_at": past,
	})
	var overdueTx map[string]any
	decodeJSON(t, resp, &overdueTx)
	overdueID := overdueTx["id"].(string)

	// Not due yet: must NOT contribute to the deviation, however unpaid.
	doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions", map[string]any{
		"description": "Parcela futura", "amount": "300.00", "projected_at": future,
	}).Body.Close()

	txID := insertTransaction(t, conn)
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/links", map[string]any{"transaction_id": txID})
	var link map[string]any
	decodeJSON(t, resp, &link)
	linkID := link["id"].(string)

	doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/transactions/"+overdueID+"/realizations", map[string]any{
		"debt_link_id": linkID, "allocated_amount": "60.00",
	}).Body.Close()

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/scenarios/"+scenarioID, nil)
	var detail map[string]any
	decodeJSON(t, resp, &detail)
	if detail["accumulated_deviation"] != "40.00" {
		t.Errorf("accumulated_deviation = %v, want 40.00", detail["accumulated_deviation"])
	}
}

func TestReadjustReplacesUnallocatedInstallments(t *testing.T) {
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
		"cadence": "mensal", "months": 3, "start_date": "2026-09-01",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("generate status = %d, want 201", resp.StatusCode)
	}
	resp.Body.Close()

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/readjust", map[string]any{"strategy": "redistribuir"})
	if resp.StatusCode != 200 {
		t.Fatalf("readjust status = %d, want 200", resp.StatusCode)
	}
	var readjusted []map[string]any
	decodeJSON(t, resp, &readjusted)
	if len(readjusted) != 3 {
		t.Fatalf("len(readjusted) = %d, want 3", len(readjusted))
	}
	sum := 0.0
	for _, ins := range readjusted {
		var amount float64
		fmt.Sscanf(ins["amount"].(string), "%f", &amount)
		sum += amount
	}
	if sum < 1199.99 || sum > 1200.01 {
		t.Errorf("sum after readjust = %v, want ~1200.00", sum)
	}
}

func TestReadjustRejectsInvalidStrategyOverHTTP(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "Financiamento", "total_amount": "1200.00"})
	var debt map[string]any
	decodeJSON(t, resp, &debt)
	debtID := debt["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/scenarios", map[string]any{"name": "Plano"})
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/readjust", map[string]any{"strategy": "bogus"})
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
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
		"cadence": "mensal", "months": 3, "start_date": "2026-09-01",
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
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/generate-installments", map[string]any{"cadence": "mensal", "months": 2})
	if resp.StatusCode != 409 {
		t.Fatalf("second generate status = %d, want 409", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestGenerateInstallmentsWithWeeklyCadenceAndInstallmentAmount(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "Financiamento", "total_amount": "1000.00"})
	var debt map[string]any
	decodeJSON(t, resp, &debt)
	debtID := debt["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/scenarios", map[string]any{"name": "Plano"})
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/generate-installments", map[string]any{
		"cadence": "semanal", "installment_amount": "400.00", "start_date": "2026-01-01",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
	}
	var created []map[string]any
	decodeJSON(t, resp, &created)
	// ceil(1000/400) = 3 installments: 400.00, 400.00, 200.00.
	if len(created) != 3 {
		t.Fatalf("len(created) = %d, want 3", len(created))
	}
	if created[0]["amount"] != "400.00" || created[2]["amount"] != "200.00" {
		t.Errorf("created = %+v", created)
	}
	wantDates := []string{"2026-01-01", "2026-01-08", "2026-01-15"}
	for i, ins := range created {
		if ins["projected_at"] != wantDates[i] {
			t.Errorf("created[%d].projected_at = %v, want %s", i, ins["projected_at"], wantDates[i])
		}
	}
}

func TestGenerateInstallmentsRejectsInvalidCadenceOverHTTP(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "Financiamento", "total_amount": "1200.00"})
	var debt map[string]any
	decodeJSON(t, resp, &debt)
	debtID := debt["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/scenarios", map[string]any{"name": "Plano"})
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/generate-installments", map[string]any{"cadence": "bogus", "months": 3})
	if resp.StatusCode != 422 {
		t.Fatalf("status = %d, want 422", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestGenerateInstallmentsRejectsBothOrNeitherOfMonthsAndAmount(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/debts", map[string]any{"name": "Financiamento", "total_amount": "1200.00"})
	var debt map[string]any
	decodeJSON(t, resp, &debt)
	debtID := debt["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/debts/"+debtID+"/scenarios", map[string]any{"name": "Plano"})
	var scenario map[string]any
	decodeJSON(t, resp, &scenario)
	scenarioID := scenario["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/generate-installments", map[string]any{"cadence": "mensal"})
	if resp.StatusCode != 422 {
		t.Fatalf("neither given: status = %d, want 422", resp.StatusCode)
	}
	resp.Body.Close()

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/scenarios/"+scenarioID+"/generate-installments", map[string]any{
		"cadence": "mensal", "months": 3, "installment_amount": "400.00",
	})
	if resp.StatusCode != 422 {
		t.Fatalf("both given: status = %d, want 422", resp.StatusCode)
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
