package quotes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// newTestCoinGecko mirrors internal/pluggy/client_test.go's newTestAdapter:
// point the connector at an httptest server and strip out real retry
// waiting, so a retry test runs instantly.
func newTestCoinGecko(t *testing.T, mux *http.ServeMux) *CoinGeckoConnector {
	t.Helper()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	c := NewCoinGeckoConnector()
	c.baseURL = server.URL
	c.http.client = server.Client()
	c.http.sleep = func(time.Duration) {}
	c.http.backoff = time.Millisecond
	return c
}

func TestCoinGeckoFetchPriceHappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/price", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("ids"); got != "bitcoin" {
			t.Errorf("ids = %q, want bitcoin", got)
		}
		if got := r.URL.Query().Get("vs_currencies"); got != "brl" {
			t.Errorf("vs_currencies = %q, want brl", got)
		}
		json.NewEncoder(w).Encode(map[string]any{"bitcoin": map[string]any{"brl": 433155.42}})
	})
	connector := newTestCoinGecko(t, mux)

	price, err := connector.FetchPrice(context.Background(), "bitcoin")
	if err != nil {
		t.Fatalf("FetchPrice: %v", err)
	}
	if price.String() != "433155.42" {
		t.Errorf("price = %s, want 433155.42", price.String())
	}
}

// TestCoinGeckoFetchPriceUnknownSymbol mirrors CoinGecko's real behavior for
// an id it does not recognize: HTTP 200 with an empty {} object rather than
// an error status (verified live against api.coingecko.com while building
// this connector).
func TestCoinGeckoFetchPriceUnknownSymbol(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/price", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{})
	})
	connector := newTestCoinGecko(t, mux)

	if _, err := connector.FetchPrice(context.Background(), "not-a-real-coin"); err == nil {
		t.Fatal("expected an error for an unknown symbol")
	}
}

func TestCoinGeckoFetchPriceNonPositivePrice(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/price", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"bitcoin": map[string]any{"brl": 0}})
	})
	connector := newTestCoinGecko(t, mux)

	if _, err := connector.FetchPrice(context.Background(), "bitcoin"); err == nil {
		t.Fatal("expected an error for a non-positive price")
	}
}

func TestCoinGeckoFetchPriceRetriesOn429ThenSucceeds(t *testing.T) {
	attempts := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/price", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"bitcoin": map[string]any{"brl": 433155}})
	})
	connector := newTestCoinGecko(t, mux)

	price, err := connector.FetchPrice(context.Background(), "bitcoin")
	if err != nil {
		t.Fatalf("FetchPrice: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
	if price.String() != "433155" {
		t.Errorf("price = %s, want 433155", price.String())
	}
}

func TestCoinGeckoFetchPriceRetriesExhausted(t *testing.T) {
	attempts := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/price", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusTooManyRequests)
	})
	connector := newTestCoinGecko(t, mux)

	if _, err := connector.FetchPrice(context.Background(), "bitcoin"); err == nil {
		t.Fatal("expected an error once retries are exhausted")
	}
	if attempts != connector.http.maxAttempts {
		t.Errorf("attempts = %d, want %d", attempts, connector.http.maxAttempts)
	}
}
