package pluggy

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeWriter records every envelope it's asked to persist, standing in for
// a real raw_imports-backed writer.
type fakeWriter struct {
	envelopes []RawResponseEnvelope
}

func (w *fakeWriter) Write(envelope RawResponseEnvelope) (string, error) {
	w.envelopes = append(w.envelopes, envelope)
	return fmt.Sprintf("raw-%d", len(w.envelopes)), nil
}

func newTestAdapter(t *testing.T, mux *http.ServeMux) (*Adapter, *fakeWriter) {
	t.Helper()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	writer := &fakeWriter{}
	cfg := DefaultConfig()
	cfg.BaseURL = server.URL
	cfg.ItemID = "item-1"
	cfg.MaxAttempts = 3
	adapter := NewAdapter(cfg, writer, server.Client())
	adapter.sleep = func(time.Duration) {} // no real waiting in tests
	return adapter, writer
}

func authHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"apiKey": "test-key"})
}

func TestAdapterGetSourceHappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/items/item-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-KEY") != "test-key" {
			t.Errorf("missing api key header")
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id": "item-1", "status": "UPDATED", "executionStatus": "SUCCESS",
			"connector": map[string]string{"name": "Banco Exemplo"},
		})
	})
	adapter, writer := newTestAdapter(t, mux)

	source, rawImportID, err := adapter.GetSource(context.Background())
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if source.InstitutionName == nil || *source.InstitutionName != "Banco Exemplo" {
		t.Errorf("InstitutionName = %v", source.InstitutionName)
	}
	if !source.SafeProducts["ACCOUNTS"] || !source.SafeProducts["TRANSACTIONS"] {
		t.Errorf("SafeProducts = %v", source.SafeProducts)
	}
	if rawImportID == "" {
		t.Error("expected a raw import id")
	}
	if len(writer.envelopes) != 1 {
		t.Fatalf("len(envelopes) = %d, want 1", len(writer.envelopes))
	}
}

func TestAdapterGetSourceRejectsLoginError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/items/item-1", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"id": "item-1", "status": "LOGIN_ERROR"})
	})
	adapter, _ := newTestAdapter(t, mux)

	_, _, err := adapter.GetSource(context.Background())
	provErr, ok := err.(*ProviderError)
	if !ok || provErr.Code != "item_invalid_credentials" {
		t.Errorf("err = %v, want item_invalid_credentials", err)
	}
}

func TestAdapterAuthRejectsInvalidCredentials(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	adapter, _ := newTestAdapter(t, mux)

	_, _, err := adapter.GetSource(context.Background())
	provErr, ok := err.(*ProviderError)
	if !ok || provErr.Code != "invalid_provider_credentials" {
		t.Errorf("err = %v, want invalid_provider_credentials", err)
	}
}

func TestAdapterRetriesOn429ThenSucceeds(t *testing.T) {
	attempts := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/accounts", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"id": "acc-1", "name": "Conta", "currencyCode": "BRL"}},
		})
	})
	adapter, _ := newTestAdapter(t, mux)

	page, err := adapter.GetAccounts(context.Background())
	if err != nil {
		t.Fatalf("GetAccounts: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
	if len(page.Accounts) != 1 || page.Accounts[0].ExternalID != "acc-1" {
		t.Errorf("Accounts = %+v", page.Accounts)
	}
}

func TestAdapterExhaustsRetriesOnPersistentRateLimit(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/accounts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
	})
	adapter, _ := newTestAdapter(t, mux)

	_, err := adapter.GetAccounts(context.Background())
	provErr, ok := err.(*ProviderError)
	if !ok || provErr.Code != "provider_rate_limited" {
		t.Errorf("err = %v, want provider_rate_limited", err)
	}
}

func TestAdapterIteratesTransactionPagesUntilNoCursor(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/v2/transactions", func(w http.ResponseWriter, r *http.Request) {
		after := r.URL.Query().Get("after")
		if after == "" {
			json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{{"id": "tx-1", "accountId": "acc-1", "amount": -1.0}},
				"next":    "https://api.pluggy.ai/v2/transactions?accountId=acc-1&after=page2",
			})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"id": "tx-2", "accountId": "acc-1", "amount": -2.0}},
			"next":    nil,
		})
	})
	adapter, _ := newTestAdapter(t, mux)

	var seen []string
	err := adapter.IterTransactionPages(context.Background(), "acc-1", func(page TransactionsPage) error {
		for _, tx := range page.Transactions {
			seen = append(seen, tx.ExternalID)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("IterTransactionPages: %v", err)
	}
	if len(seen) != 2 || seen[0] != "tx-1" || seen[1] != "tx-2" {
		t.Errorf("seen = %v, want [tx-1 tx-2]", seen)
	}
}

func TestAdapterGetInvestmentsHappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/investments", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"id": "inv-1", "name": "Fundo XYZ", "balance": 1000.50}},
		})
	})
	adapter, writer := newTestAdapter(t, mux)

	page, err := adapter.GetInvestments(context.Background())
	if err != nil {
		t.Fatalf("GetInvestments: %v", err)
	}
	if len(page.Investments) != 1 || page.Investments[0].ExternalID != "inv-1" {
		t.Errorf("Investments = %+v", page.Investments)
	}
	if len(writer.envelopes) != 1 || writer.envelopes[0].Scope != ScopeInvestments {
		t.Errorf("envelopes = %+v", writer.envelopes)
	}
}

func TestAdapterGetInvestmentTransactionsHappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/investments/inv-1/transactions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"id": "invtx-1", "investmentId": "inv-1", "type": "BUY", "amount": 500.00}},
		})
	})
	adapter, writer := newTestAdapter(t, mux)

	page, err := adapter.GetInvestmentTransactions(context.Background(), "inv-1")
	if err != nil {
		t.Fatalf("GetInvestmentTransactions: %v", err)
	}
	if len(page.Transactions) != 1 || page.Transactions[0].ExternalID != "invtx-1" {
		t.Errorf("Transactions = %+v", page.Transactions)
	}
	if len(writer.envelopes) != 1 || writer.envelopes[0].Scope != ScopeInvestmentTransactions {
		t.Errorf("envelopes = %+v", writer.envelopes)
	}
}

func TestAdapterGetBillsPaginatesByPageNumber(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/bills", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("accountId") != "acc-1" {
			t.Errorf("accountId = %q, want acc-1", r.URL.Query().Get("accountId"))
		}
		switch r.URL.Query().Get("page") {
		case "1":
			json.NewEncoder(w).Encode(map[string]any{
				"page": 1, "totalPages": 2,
				"results": []map[string]any{{"id": "bill-1", "dueDate": "2026-03-10T00:00:00Z"}},
			})
		case "2":
			json.NewEncoder(w).Encode(map[string]any{
				"page": 2, "totalPages": 2,
				"results": []map[string]any{{"id": "bill-2", "dueDate": "2026-04-10T00:00:00Z"}},
			})
		default:
			t.Errorf("unexpected page %q", r.URL.Query().Get("page"))
		}
	})
	adapter, writer := newTestAdapter(t, mux)

	page, err := adapter.GetBills(context.Background(), "acc-1")
	if err != nil {
		t.Fatalf("GetBills: %v", err)
	}
	if len(page.Bills) != 2 || page.Bills[0].ExternalID != "bill-1" || page.Bills[1].ExternalID != "bill-2" {
		t.Errorf("Bills = %+v", page.Bills)
	}
	if len(writer.envelopes) != 2 || writer.envelopes[0].Scope != ScopeBills {
		t.Errorf("envelopes = %+v", writer.envelopes)
	}
}

func TestAdapterRejectsRepeatedCursor(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/auth", authHandler)
	mux.HandleFunc("/v2/transactions", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{{"id": "tx-1", "accountId": "acc-1", "amount": -1.0}},
			"next":    "https://api.pluggy.ai/v2/transactions?accountId=acc-1&after=same",
		})
	})
	adapter, _ := newTestAdapter(t, mux)

	err := adapter.IterTransactionPages(context.Background(), "acc-1", func(TransactionsPage) error { return nil })
	provErr, ok := err.(*ProviderError)
	if !ok || provErr.Code != "invalid_provider_payload" {
		t.Errorf("err = %v, want invalid_provider_payload for a repeated cursor", err)
	}
}
