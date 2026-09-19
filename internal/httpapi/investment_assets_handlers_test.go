package httpapi_test

import (
	"net/http"
	"testing"
)

var investmentAssetResponseKeys = []string{
	"id", "name", "ticker", "asset_type", "currency_code", "created_at", "updated_at",
}

func TestInvestmentAssetLifecycleOverHTTP(t *testing.T) {
	srv, _ := newTestServer(t)

	if body := investmentRawBody(t, srv, "/api/investment-assets"); body != `{"items":[]}` {
		t.Fatalf("empty asset list = %s, want an items envelope with an empty array", body)
	}

	created := investmentCall(t, srv, http.MethodPost, "/api/investment-assets", map[string]any{
		"name": "Petrobras PN", "ticker": "PETR4", "asset_type": "Ação", "currency_code": "brl",
	}, http.StatusCreated)
	assertInvestmentKeys(t, "asset", created, investmentAssetResponseKeys)
	id := investmentID(t, created)
	if created["currency_code"] != "BRL" || created["ticker"] != "PETR4" {
		t.Fatalf("created asset = %+v", created)
	}

	updated := investmentCall(t, srv, http.MethodPut, "/api/investment-assets/"+id, map[string]any{
		"name": "Petróleo Brasileiro PN", "ticker": "PETR4", "asset_type": "Ação", "currency_code": "BRL",
	}, http.StatusOK)
	if updated["name"] != "Petróleo Brasileiro PN" {
		t.Errorf("updated name = %v", updated["name"])
	}

	items := investmentItems(t, srv, "/api/investment-assets")
	if len(items) != 1 || items[0]["id"] != id {
		t.Fatalf("listed assets = %+v", items)
	}

	problem := investmentProblem(t, srv, http.MethodPost, "/api/investment-assets", map[string]any{
		"name": "Outra Petrobras", "ticker": "petr-4", "asset_type": "Ação", "currency_code": "BRL",
	}, http.StatusConflict)
	if problem != "investment-asset-already-exists" {
		t.Errorf("duplicate problem = %q", problem)
	}

	account := createCustodyAccount(t, srv, "Corretora")
	position := investmentCall(t, srv, http.MethodPost, "/api/investment-positions", map[string]any{
		"account_id": investmentID(t, account), "asset_id": id, "name": "", "asset_type": "Ação",
	}, http.StatusCreated)
	problem = investmentProblem(t, srv, http.MethodDelete, "/api/investment-assets/"+id, nil, http.StatusConflict)
	if problem != "investment-asset-has-positions" {
		t.Errorf("in-use problem = %q", problem)
	}

	investmentCall(t, srv, http.MethodDelete, "/api/investment-positions/"+investmentID(t, position), nil, http.StatusNoContent)
	investmentCall(t, srv, http.MethodDelete, "/api/investment-assets/"+id, nil, http.StatusNoContent)
	if got := investmentItems(t, srv, "/api/investment-assets"); len(got) != 0 {
		t.Fatalf("assets after delete = %+v", got)
	}
}

func TestInvestmentAssetValidationAndMissingRecords(t *testing.T) {
	srv, _ := newTestServer(t)

	problem := investmentProblem(t, srv, http.MethodPost, "/api/investment-assets", map[string]any{
		"name": "", "ticker": nil, "asset_type": "Ação", "currency_code": "BRL",
	}, http.StatusBadRequest)
	if problem != "invalid-investment-input" {
		t.Errorf("invalid problem = %q", problem)
	}

	problem = investmentProblem(t, srv, http.MethodDelete,
		"/api/investment-assets/33333333-3333-4333-8333-333333333333", nil, http.StatusNotFound)
	if problem != "investment-asset-not-found" {
		t.Errorf("missing problem = %q", problem)
	}
}
