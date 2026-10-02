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

func TestInvestmentAssetQuoteMarketRoundTrip(t *testing.T) {
	srv, _ := newTestServer(t)
	created := investmentCall(t, srv, http.MethodPost, "/api/investment-assets", map[string]any{
		"name": "Bitcoin", "ticker": "btc-usd", "asset_type": "Criptoativo", "currency_code": "BRL", "quote_source": "crypto",
	}, http.StatusCreated)
	assertInvestmentKeys(t, "asset", created, investmentAssetResponseKeys)
	if created["quote_source"] != "crypto" || created["quote_symbol"] != "BTC" {
		t.Fatalf("created = %+v", created)
	}
	id := investmentID(t, created)
	items := investmentItems(t, srv, "/api/investment-assets")
	if len(items) != 1 || items[0]["quote_symbol"] != "BTC" {
		t.Fatalf("items = %+v", items)
	}
	updated := investmentCall(t, srv, http.MethodPut, "/api/investment-assets/"+id, map[string]any{
		"name": "Bitcoin", "ticker": "btc", "asset_type": "Criptoativo", "currency_code": "BRL", "quote_source": "crypto",
	}, http.StatusOK)
	if updated["quote_symbol"] != "BTC" {
		t.Fatalf("updated = %+v", updated)
	}
	cleared := investmentCall(t, srv, http.MethodPut, "/api/investment-assets/"+id, map[string]any{
		"name": "Bitcoin", "ticker": "BTC", "asset_type": "Criptoativo", "currency_code": "BRL",
	}, http.StatusOK)
	if cleared["quote_source"] != nil || cleared["quote_symbol"] != nil {
		t.Fatalf("cleared = %+v", cleared)
	}
}

func TestInvestmentAssetCanonicalizesMarketSymbols(t *testing.T) {
	for _, tc := range []struct{ market, ticker, symbol, want string }{
		{"b3", "petr4.sa", "", "PETR4"},
		{"b3", "PETR4", "bvmf:vale3", "VALE3"},
		{"crypto", "BTC", "", "BTC"},
		{"crypto", "BTC", "btc-usd", "BTC"},
	} {
		srv, _ := newTestServer(t)
		body := map[string]any{"name": "Ativo " + tc.want, "ticker": tc.ticker, "asset_type": "Ação", "currency_code": "BRL", "quote_source": tc.market}
		if tc.symbol != "" {
			body["quote_symbol"] = tc.symbol
		}
		got := investmentCall(t, srv, http.MethodPost, "/api/investment-assets", body, http.StatusCreated)
		if got["quote_symbol"] != tc.want {
			t.Errorf("%+v: got %+v", tc, got)
		}
	}
}

func TestInvestmentAssetRejectsInvalidMarketSymbols(t *testing.T) {
	srv, _ := newTestServer(t)
	for _, tc := range []struct{ market, ticker, symbol string }{
		{"other", "PETR4", ""},
		{"b3", "BTC", ""},
		{"b3", "", ""},
		{"crypto", "BTC", "bit coin"},
	} {
		body := map[string]any{"name": "Ativo", "ticker": tc.ticker, "asset_type": "Ação", "currency_code": "BRL", "quote_source": tc.market}
		if tc.symbol != "" {
			body["quote_symbol"] = tc.symbol
		}
		if got := investmentProblem(t, srv, http.MethodPost, "/api/investment-assets", body, http.StatusBadRequest); got != "invalid-investment-input" {
			t.Errorf("%+v: %s", tc, got)
		}
	}
}
