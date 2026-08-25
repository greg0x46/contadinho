package httpapi_test

import (
	"net/http"
	"testing"
)

func TestRecurringCommitmentLifecycleOverHTTP(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/categories", map[string]any{
		"name": "Aluguel", "kind": "expense", "icon": "home", "color": "#495057",
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create category status = %d, want 201", resp.StatusCode)
	}
	var category map[string]any
	decodeJSON(t, resp, &category)
	categoryID := category["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/recurring-scenarios", map[string]any{
		"name": "Aluguel", "kind": "expense", "amount": "1500.00",
		"category_id": categoryID, "cadence": "monthly", "day_of_month": 5, "start_date": "2026-01-01",
		"is_active": true,
	})
	if resp.StatusCode != 201 {
		t.Fatalf("create commitment status = %d, want 201", resp.StatusCode)
	}
	var created map[string]any
	decodeJSON(t, resp, &created)
	id := created["id"].(string)
	if created["amount"] != "1500.00" || created["day_of_month"] != float64(5) {
		t.Errorf("created = %+v", created)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/recurring-scenarios", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("list status = %d, want 200", resp.StatusCode)
	}
	var list []map[string]any
	decodeJSON(t, resp, &list)
	if len(list) != 1 || list[0]["id"] != id {
		t.Errorf("list = %+v", list)
	}

	resp = doJSON(t, http.MethodPut, srv.URL+"/api/recurring-scenarios/"+id, map[string]any{
		"name": "Aluguel", "kind": "expense", "amount": "1600.00",
		"category_id": categoryID, "cadence": "monthly", "day_of_month": 10, "start_date": "2026-01-01",
		"is_active": true,
	})
	if resp.StatusCode != 200 {
		t.Fatalf("update status = %d, want 200", resp.StatusCode)
	}
	var updated map[string]any
	decodeJSON(t, resp, &updated)
	if updated["amount"] != "1600.00" || updated["day_of_month"] != float64(10) {
		t.Errorf("updated = %+v", updated)
	}

	resp = doJSON(t, http.MethodPatch, srv.URL+"/api/recurring-scenarios/"+id, map[string]any{"is_active": false})
	if resp.StatusCode != 200 {
		t.Fatalf("set active status = %d, want 200", resp.StatusCode)
	}
	var paused map[string]any
	decodeJSON(t, resp, &paused)
	if paused["is_active"] != false {
		t.Errorf("expected paused commitment, got %+v", paused)
	}

	resp = doJSON(t, http.MethodDelete, srv.URL+"/api/recurring-scenarios/"+id, nil)
	if resp.StatusCode != 204 {
		t.Fatalf("delete status = %d, want 204", resp.StatusCode)
	}

	resp = doJSON(t, http.MethodGet, srv.URL+"/api/recurring-scenarios", nil)
	decodeJSON(t, resp, &list)
	if len(list) != 0 {
		t.Errorf("expected empty list after delete, got %+v", list)
	}
}

func TestCreateRecurringCommitmentValidatesAnnualRequiresMonth(t *testing.T) {
	srv, _ := newTestServer(t)

	resp := doJSON(t, http.MethodPost, srv.URL+"/api/categories", map[string]any{
		"name": "IPVA", "kind": "expense", "icon": "car", "color": "#495057",
	})
	var category map[string]any
	decodeJSON(t, resp, &category)
	categoryID := category["id"].(string)

	resp = doJSON(t, http.MethodPost, srv.URL+"/api/recurring-scenarios", map[string]any{
		"name": "IPVA", "kind": "expense", "amount": "800.00", "category_id": categoryID,
		"cadence": "annual", "day_of_month": 15, "start_date": "2026-01-01", "is_active": true,
	})
	if resp.StatusCode != 422 {
		t.Fatalf("expected 422 for annual cadence without month_of_year, got %d", resp.StatusCode)
	}
}
