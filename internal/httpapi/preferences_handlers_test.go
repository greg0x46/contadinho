package httpapi_test

import (
	"net/http"
	"testing"
)

func TestPreferencesLifecycleOverHTTP(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodGet, srv.URL+"/api/preferences", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("get status = %d, want 200", resp.StatusCode)
	}
	var got map[string]any
	decodeJSON(t, resp, &got)
	if got["transactions_period_basis"] != "occurred_at" {
		t.Errorf("default transactions_period_basis = %v, want occurred_at", got["transactions_period_basis"])
	}

	resp = doJSON(t, http.MethodPut, srv.URL+"/api/preferences", map[string]string{"transactions_period_basis": "paid_at"})
	if resp.StatusCode != 200 {
		t.Fatalf("put status = %d, want 200", resp.StatusCode)
	}
	decodeJSON(t, resp, &got)
	if got["transactions_period_basis"] != "paid_at" {
		t.Errorf("updated transactions_period_basis = %v, want paid_at", got["transactions_period_basis"])
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/preferences", nil)
	decodeJSON(t, resp, &got)
	if got["transactions_period_basis"] != "paid_at" {
		t.Errorf("persisted transactions_period_basis = %v, want paid_at", got["transactions_period_basis"])
	}

	resp = doJSON(t, http.MethodPut, srv.URL+"/api/preferences", map[string]string{"transactions_period_basis": "nonsense"})
	if resp.StatusCode != 422 {
		t.Errorf("invalid value status = %d, want 422", resp.StatusCode)
	}
}
