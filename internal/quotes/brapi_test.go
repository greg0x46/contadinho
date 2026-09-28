package quotes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestBrapi(t *testing.T, mux *http.ServeMux, token string) *BrapiConnector {
	t.Helper()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	c := NewBrapiConnector(token)
	c.baseURL = server.URL
	c.http.client = server.Client()
	c.http.sleep = func(time.Duration) {}
	c.http.backoff = time.Millisecond
	return c
}

func TestBrapiFetchPriceHappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/PETR4", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"symbol": "PETR4", "regularMarketPrice": 38.29}},
		})
	})
	connector := newTestBrapi(t, mux, "test-token")

	price, err := connector.FetchPrice(context.Background(), "PETR4")
	if err != nil {
		t.Fatalf("FetchPrice: %v", err)
	}
	if price.String() != "38.29" {
		t.Errorf("price = %s, want 38.29", price.String())
	}
}

func TestBrapiFetchPriceWithoutTokenOmitsAuthorizationHeader(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/PETR4", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want empty", got)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"symbol": "PETR4", "regularMarketPrice": 38.29}},
		})
	})
	connector := newTestBrapi(t, mux, "")

	if _, err := connector.FetchPrice(context.Background(), "PETR4"); err != nil {
		t.Fatalf("FetchPrice: %v", err)
	}
}

// TestBrapiFetchPriceUnknownSymbol mirrors brapi's documented behavior for a
// ticker that simply does not exist: it is omitted from results rather than
// erroring.
func TestBrapiFetchPriceUnknownSymbol(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/NAOEXISTE9", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	})
	connector := newTestBrapi(t, mux, "")

	if _, err := connector.FetchPrice(context.Background(), "NAOEXISTE9"); err == nil {
		t.Fatal("expected an error for an unknown symbol")
	}
}

// TestBrapiFetchPriceMissingTokenIsSurfacedAsError mirrors the real API's
// live response (verified while building this connector) for a ticker
// outside the small unauthenticated allow-list: 401 with code MISSING_TOKEN.
// This connector hardcodes no allow-list — it just surfaces the 401.
func TestBrapiFetchPriceMissingTokenIsSurfacedAsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/AAPL34", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": true, "message": "Token de autenticação não fornecido", "code": "MISSING_TOKEN",
		})
	})
	connector := newTestBrapi(t, mux, "")

	if _, err := connector.FetchPrice(context.Background(), "AAPL34"); err == nil {
		t.Fatal("expected an error for a 401 response")
	}
}

func TestBrapiFetchPriceNonPositivePrice(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/PETR4", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"symbol": "PETR4", "regularMarketPrice": -1}},
		})
	})
	connector := newTestBrapi(t, mux, "")

	if _, err := connector.FetchPrice(context.Background(), "PETR4"); err == nil {
		t.Fatal("expected an error for a non-positive price")
	}
}

func TestBrapiFetchPriceRetriesOn429ThenSucceeds(t *testing.T) {
	attempts := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/PETR4", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"symbol": "PETR4", "regularMarketPrice": 38.29}},
		})
	})
	connector := newTestBrapi(t, mux, "")

	if _, err := connector.FetchPrice(context.Background(), "PETR4"); err != nil {
		t.Fatalf("FetchPrice: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestBrapiFetchPriceRetriesExhausted(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/PETR4", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	connector := newTestBrapi(t, mux, "")

	if _, err := connector.FetchPrice(context.Background(), "PETR4"); err == nil {
		t.Fatal("expected an error once retries are exhausted")
	}
}
