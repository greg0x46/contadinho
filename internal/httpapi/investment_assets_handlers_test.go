package httpapi_test

import (
	"net/http"
	"testing"
)

var investmentAssetResponseKeys = []string{
	"id", "name", "ticker", "asset_type", "currency_code", "quote_source", "quote_symbol", "created_at", "updated_at",
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

// TestInvestmentAssetQuoteSourceRoundTrip covers the fields this feature
// adds to the asset CRUD endpoints: a connector name and the symbol to ask
// it for round-trip through create, update (including clearing both back to
// null) and list; an unknown connector key is rejected the same way any
// other invalid asset input is.
func TestInvestmentAssetQuoteSourceRoundTrip(t *testing.T) {
	srv, _ := newTestServer(t)

	created := investmentCall(t, srv, http.MethodPost, "/api/investment-assets", map[string]any{
		"name": "Bitcoin", "ticker": nil, "asset_type": "Criptoativo", "currency_code": "BRL",
		"quote_source": "coingecko", "quote_symbol": "bitcoin",
	}, http.StatusCreated)
	assertInvestmentKeys(t, "asset", created, investmentAssetResponseKeys)
	if created["quote_source"] != "coingecko" || created["quote_symbol"] != "bitcoin" {
		t.Fatalf("created asset = %+v", created)
	}
	id := investmentID(t, created)

	items := investmentItems(t, srv, "/api/investment-assets")
	if len(items) != 1 || items[0]["quote_source"] != "coingecko" || items[0]["quote_symbol"] != "bitcoin" {
		t.Fatalf("listed assets = %+v", items)
	}

	updated := investmentCall(t, srv, http.MethodPut, "/api/investment-assets/"+id, map[string]any{
		"name": "Bitcoin", "ticker": nil, "asset_type": "Criptoativo", "currency_code": "BRL",
		"quote_source": "brapi", "quote_symbol": "BTC11",
	}, http.StatusOK)
	if updated["quote_source"] != "brapi" || updated["quote_symbol"] != "BTC11" {
		t.Fatalf("updated asset = %+v", updated)
	}

	cleared := investmentCall(t, srv, http.MethodPut, "/api/investment-assets/"+id, map[string]any{
		"name": "Bitcoin", "ticker": nil, "asset_type": "Criptoativo", "currency_code": "BRL",
	}, http.StatusOK)
	if cleared["quote_source"] != nil || cleared["quote_symbol"] != nil {
		t.Fatalf("cleared asset = %+v", cleared)
	}

	problem := investmentProblem(t, srv, http.MethodPost, "/api/investment-assets", map[string]any{
		"name": "Ethereum", "ticker": nil, "asset_type": "Criptoativo", "currency_code": "BRL",
		"quote_source": "not-a-real-connector", "quote_symbol": "ethereum",
	}, http.StatusBadRequest)
	if problem != "invalid-investment-input" {
		t.Errorf("unknown connector problem = %q", problem)
	}

	problem = investmentProblem(t, srv, http.MethodPost, "/api/investment-assets", map[string]any{
		"name": "Ethereum", "ticker": nil, "asset_type": "Criptoativo", "currency_code": "BRL",
		"quote_source": "coingecko",
	}, http.StatusBadRequest)
	if problem != "invalid-investment-input" {
		t.Errorf("quote_source without quote_symbol problem = %q", problem)
	}
}
