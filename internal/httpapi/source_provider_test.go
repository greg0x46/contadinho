package httpapi_test

import (
	"net/http"
	"testing"
)

func TestTransactionQuerySourceProviderOverHTTP(t *testing.T) {
	srv, conn := newTestServer(t)
	txID := insertTransaction(t, conn)
	if _, err := conn.Exec(`UPDATE data_sources SET provider='file' WHERE id=(SELECT source_id FROM financial_transactions WHERE id=?)`, txID); err != nil {
		t.Fatal(err)
	}
	body := map[string]any{"timezone": "UTC", "group_by": "none", "page": 1, "page_size": 50,
		"filters": map[string]any{"source_provider": "file"}}
	resp := doJSON(t, http.MethodPost, srv.URL+"/api/transactions/query", body)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("file query status = %d", resp.StatusCode)
	}
	var result struct {
		Items []struct {
			ID             string  `json:"id"`
			SourceProvider *string `json:"source_provider"`
		} `json:"items"`
	}
	decodeJSON(t, resp, &result)
	if len(result.Items) != 1 || result.Items[0].ID != txID || result.Items[0].SourceProvider == nil || *result.Items[0].SourceProvider != "file" {
		t.Fatalf("file query items = %+v", result.Items)
	}
	body["filters"] = map[string]any{"source_provider": "pluggy"}
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/transactions/query", body)
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("pluggy query status = %d", resp.StatusCode)
	}
	decodeJSON(t, resp, &result)
	if len(result.Items) != 0 {
		t.Fatalf("pluggy query returned file item: %+v", result.Items)
	}
	body["filters"] = map[string]any{"source_provider": "unknown"}
	resp = doJSON(t, http.MethodPost, srv.URL+"/api/transactions/query", body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid provider status = %d", resp.StatusCode)
	}
}
