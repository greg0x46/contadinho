package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
)

func TestManualTransactionLifecycleOverHTTP(t *testing.T) {
	srv, conn := newTestServer(t)
	seed := insertAccount(t, conn, "BANK", "Conta corrente", nil, nil)

	create := map[string]any{
		"account_id":  seed.accountID,
		"description": "Almoço em dinheiro",
		"amount":      "-45.50",
		"occurred_at": "2026-03-01",
		"category_id": nil,
	}
	resp := doJSON(t, http.MethodPost, srv.URL+"/api/transactions", create)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", resp.StatusCode)
	}
	var item map[string]any
	decodeJSON(t, resp, &item)
	if item["origin"] != "manual" {
		t.Errorf("origin = %v, want manual", item["origin"])
	}
	if item["classification"] != "outflow" {
		t.Errorf("classification = %v, want outflow", item["classification"])
	}
	id, _ := item["id"].(string)
	if id == "" {
		t.Fatalf("missing id in response: %+v", item)
	}

	// An unknown account is rejected with 404, not a generic failure.
	badCreate := map[string]any{
		"account_id": "does-not-exist", "description": "x", "amount": "1", "occurred_at": "2026-03-01",
	}
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/transactions", badCreate)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown account create status = %d, want 404", resp.StatusCode)
	}

	// A zero amount is invalid input, not a domain error.
	zeroCreate := map[string]any{
		"account_id": seed.accountID, "description": "x", "amount": "0", "occurred_at": "2026-03-01",
	}
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/transactions", zeroCreate)
	resp.Body.Close()
	if resp.StatusCode != 422 {
		t.Errorf("zero amount create status = %d, want 422", resp.StatusCode)
	}

	// A failure after the base insert must roll the whole creation back.
	invalidCategoryCreate := map[string]any{
		"account_id": seed.accountID, "description": "não deve persistir", "amount": "1",
		"occurred_at": "2026-03-01", "category_id": uuid.NewString(),
	}
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/transactions", invalidCategoryCreate)
	resp.Body.Close()
	if resp.StatusCode != 422 {
		t.Errorf("invalid category create status = %d, want 422", resp.StatusCode)
	}
	var leaked int
	if err := conn.QueryRow(`SELECT COUNT(*) FROM financial_transactions WHERE description = 'não deve persistir'`).Scan(&leaked); err != nil {
		t.Fatalf("count rolled-back transaction: %v", err)
	}
	if leaked != 0 {
		t.Errorf("failed create left %d transaction(s), want 0", leaked)
	}

	// Category assignment already goes through the existing, origin-agnostic
	// endpoint and just works.
	var categories []map[string]any
	resp = doJSON(t, http.MethodGet, srv.URL+"/api/categories", nil)
	decodeJSON(t, resp, &categories)
	categoryID := categories[0]["id"].(string)
	updatedCategoryID := categories[1]["id"].(string)
	resp = doJSON(t, http.MethodPut, srv.URL+"/api/transactions/"+id+"/category", map[string]string{"category_id": categoryID})
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("category status = %d, want 200", resp.StatusCode)
	}

	// Editing changes the row.
	update := map[string]any{
		"account_id": seed.accountID, "description": "Almoço corrigido", "amount": "-50.00",
		"occurred_at": "2026-03-02", "category_id": updatedCategoryID,
	}
	resp = doJSON(t, http.MethodPut, srv.URL+"/api/transactions/"+id, update)
	if resp.StatusCode != 200 {
		t.Fatalf("update status = %d, want 200", resp.StatusCode)
	}
	var updated map[string]any
	decodeJSON(t, resp, &updated)
	if updated["description"] != "Almoço corrigido" {
		t.Errorf("description = %v, want Almoço corrigido", updated["description"])
	}
	updatedCategory, _ := updated["internal_category"].(map[string]any)
	if updatedCategory["id"] != updatedCategoryID {
		t.Errorf("category = %v, want %s", updatedCategory["id"], updatedCategoryID)
	}

	// A synced transaction cannot be edited or deleted through these routes.
	syncedID := insertTransaction(t, conn)
	resp = doJSON(t, http.MethodPut, srv.URL+"/api/transactions/"+syncedID, update)
	resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Errorf("edit synced status = %d, want 409", resp.StatusCode)
	}
	resp = doJSON(t, http.MethodDelete, srv.URL+"/api/transactions/"+syncedID, nil)
	resp.Body.Close()
	if resp.StatusCode != 409 {
		t.Errorf("delete synced status = %d, want 409", resp.StatusCode)
	}

	// Deleting the manual one actually removes it.
	resp = doJSON(t, http.MethodDelete, srv.URL+"/api/transactions/"+id, nil)
	if resp.StatusCode != http.StatusNoContent {
		var problem map[string]any
		decodeJSON(t, resp, &problem)
		t.Fatalf("delete status = %d, want 204: %+v", resp.StatusCode, problem)
	}
	resp.Body.Close()
	resp = doJSON(t, http.MethodPut, srv.URL+"/api/transactions/"+id+"/inclusion", map[string]string{"state": "ignored"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("post-delete status = %d, want 404", resp.StatusCode)
	}
}
