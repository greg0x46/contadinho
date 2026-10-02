package marketdata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestBrapi(t *testing.T, mux *http.ServeMux, token string) *BrapiProvider {
	t.Helper()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	p := NewBrapiProvider(func(context.Context) string { return token })
	p.baseURL = server.URL
	pointAt(p.anonymous, server)
	pointAt(p.authenticated, server)
	return p
}

func brapiQuote(price any) map[string]any {
	return map[string]any{"results": []map[string]any{{"symbol": "PETR4", "regularMarketPrice": price}}}
}

func TestBrapiQuoteHappyPath(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/PETR4", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Errorf("Authorization = %q, want Bearer test-token", got)
		}
		json.NewEncoder(w).Encode(brapiQuote(38.29))
	})
	p := newTestBrapi(t, mux, "test-token")

	quote, err := p.Quote(context.Background(), petr4)
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if quote.Price.String() != "38.29" || quote.Currency != "BRL" || quote.Instrument != petr4 {
		t.Errorf("quote = %+v, want 38.29 BRL for PETR4", quote)
	}
}

func TestBrapiQuoteWithoutTokenOmitsAuthorizationHeader(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/PETR4", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Errorf("Authorization = %q, want empty", got)
		}
		json.NewEncoder(w).Encode(brapiQuote(38.29))
	})
	p := newTestBrapi(t, mux, "")
	if _, err := p.Quote(context.Background(), petr4); err != nil {
		t.Fatalf("Quote: %v", err)
	}

	p.token = nil
	if _, err := p.Quote(context.Background(), petr4); err != nil {
		t.Fatalf("Quote without a token func: %v", err)
	}
}

// The token lives in the app's settings and can be set while the process
// runs: it is read per request, and picks the limiter that goes with it.
func TestBrapiReadsTheTokenOnEveryRequest(t *testing.T) {
	var seen []string
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/PETR4", func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
		json.NewEncoder(w).Encode(brapiQuote(38.29))
	})
	p := newTestBrapi(t, mux, "")
	token := ""
	p.token = func(context.Context) string { return token }

	p.Quote(context.Background(), petr4)
	token = "fresh"
	p.Quote(context.Background(), petr4)

	if len(seen) != 2 || seen[0] != "" || seen[1] != "Bearer fresh" {
		t.Errorf("Authorization headers = %q, want none and then the new token", seen)
	}
	if client, _ := p.client(context.Background()); client != p.authenticated {
		t.Error("a request with a token must draw from the token limiter")
	}
	token = ""
	if client, _ := p.client(context.Background()); client != p.anonymous {
		t.Error("a request without a token must draw from the anonymous limiter")
	}
	if NewBrapiProvider(nil).anonymous.limiter != brapiAnonymousLimiter || NewBrapiProvider(nil).authenticated.limiter != brapiTokenLimiter {
		t.Error("the production clients must share the process-wide brapi limiters")
	}
}

// brapi leaves a ticker it does not know out of results rather than
// erroring; a 404 means the same.
func TestBrapiUnknownSymbolIsNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/NAOEX3", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"results": []any{}})
	})
	mux.HandleFunc("/quote/SUMIU3", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{"error": true, "message": "Não encontramos a ação SUMIU3"})
	})
	p := newTestBrapi(t, mux, "")
	p.now = func() time.Time { return brapiNow }

	for _, symbol := range []string{"NAOEX3", "SUMIU3"} {
		instrument := Instrument{Market: MarketB3, Symbol: symbol}
		if _, err := p.Quote(context.Background(), instrument); !errors.Is(err, ErrNotFound) {
			t.Errorf("Quote(%s) error = %v, want ErrNotFound", symbol, err)
		}
		if _, err := p.History(context.Background(), instrument, date(2026, 9, 1), date(2026, 9, 30)); !errors.Is(err, ErrNotFound) {
			t.Errorf("History(%s) error = %v, want ErrNotFound", symbol, err)
		}
	}
}

// TestBrapiQuoteMissingTokenIsSurfacedAsError mirrors the real API's live
// response for a ticker outside the small unauthenticated allow-list: 401
// with code MISSING_TOKEN. It is a failure, not "not found": with a token the
// ticker would be served.
func TestBrapiQuoteMissingTokenIsSurfacedAsError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/AAPL34", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]any{
			"error": true, "message": "Token de autenticação não fornecido", "code": "MISSING_TOKEN",
		})
	})
	p := newTestBrapi(t, mux, "")

	_, err := p.Quote(context.Background(), Instrument{Market: MarketB3, Symbol: "AAPL34"})
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want a failure that is not ErrNotFound", err)
	}
}

func TestBrapiQuoteNonPositivePrice(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/PETR4", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(brapiQuote(-1))
	})
	p := newTestBrapi(t, mux, "")

	if _, err := p.Quote(context.Background(), petr4); err == nil {
		t.Fatal("expected an error for a non-positive price")
	}
}

func TestBrapiQuoteRetriesOn429ThenSucceeds(t *testing.T) {
	attempts := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/PETR4", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		json.NewEncoder(w).Encode(brapiQuote(38.29))
	})
	p := newTestBrapi(t, mux, "")

	if _, err := p.Quote(context.Background(), petr4); err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestBrapiQuoteRetriesExhaustedIsUnavailable(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/PETR4", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	p := newTestBrapi(t, mux, "")

	if _, err := p.Quote(context.Background(), petr4); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want ErrUnavailable once retries are exhausted", err)
	}
}

func TestBrapiSupportsOnlyB3(t *testing.T) {
	p := NewBrapiProvider(nil)
	if !p.Supports(MarketB3) || p.Supports(MarketCrypto) || p.Name() != ProviderBrapi {
		t.Error("brapi serves B3 only")
	}
}

// brapiNow is the clock of the history tests: midday UTC on 2026-10-01, which
// is still the 1st in São Paulo.
var brapiNow = time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC)

// sessionStamp is how brapi dates a session: São Paulo midnight, in unix seconds.
func sessionStamp(year int, month time.Month, day int) int64 {
	return time.Date(year, month, day, 0, 0, 0, 0, time.FixedZone("BRT", -3*3600)).Unix()
}

func historyRow(stamp int64, closeValue, adjusted any) map[string]any {
	return map[string]any{"date": stamp, "open": 1, "high": 1, "low": 1, "close": closeValue, "volume": 10, "adjustedClose": adjusted}
}

func TestBrapiHistoryAsksForTheSmallestRangeThatReachesTheFirstDay(t *testing.T) {
	var ranges []string
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/PETR4", func(w http.ResponseWriter, r *http.Request) {
		ranges = append(ranges, r.URL.Query().Get("range")+"/"+r.URL.Query().Get("interval"))
		json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{
			"symbol": "PETR4",
			"historicalDataPrice": []map[string]any{
				historyRow(sessionStamp(2026, 8, 14), 36.10, 36.10), // before from
				historyRow(sessionStamp(2026, 8, 17), 38.25, 37.20),
				historyRow(sessionStamp(2026, 9, 30), 49.77, 49.77),
				historyRow(sessionStamp(2026, 10, 1), 50.00, 50.00), // after to
			},
		}}})
	})
	p := newTestBrapi(t, mux, "")
	p.now = func() time.Time { return brapiNow }

	history, err := p.History(context.Background(), petr4, date(2026, 8, 15), date(2026, 9, 30))
	if err != nil {
		t.Fatalf("History: %v", err)
	}

	// 47 days back (+2 of margin) needs 3 months, not 1 month and not a year.
	if len(ranges) != 1 || ranges[0] != "3mo/1d" {
		t.Errorf("requests = %v, want a single 3mo/1d", ranges)
	}
	if len(history.Prices) != 2 {
		t.Fatalf("prices = %+v, want only the two sessions inside [from, to]", history.Prices)
	}
	first := history.Prices[0]
	if !first.Day.Equal(date(2026, 8, 17)) {
		t.Errorf("first day = %s, want 2026-08-17", first.Day.Format(time.DateOnly))
	}
	// The close, never the dividend-adjusted close.
	if first.Price.String() != "38.25" {
		t.Errorf("first price = %s, want the close 38.25 (adjustedClose is 37.20)", first.Price)
	}
	if !history.Prices[1].Day.Equal(date(2026, 9, 30)) || history.Prices[1].Price.String() != "49.77" {
		t.Errorf("last = %+v, want 2026-09-30 at 49.77", history.Prices[1])
	}
	if !history.CoveredFrom.Equal(date(2026, 8, 15)) || history.Currency != "BRL" {
		t.Errorf("CoveredFrom = %s, Currency = %q; want the requested start 2026-08-15 in BRL",
			history.CoveredFrom.Format(time.DateOnly), history.Currency)
	}
}

func TestBrapiHistorySkipsRowsWithoutAUsablePrice(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/PETR4", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{
			"historicalDataPrice": []map[string]any{
				historyRow(sessionStamp(2026, 9, 28), nil, nil),
				historyRow(sessionStamp(2026, 9, 29), 0, 0),
				historyRow(sessionStamp(2026, 9, 30), 49.77, 49.77),
			},
		}}})
	})
	p := newTestBrapi(t, mux, "")
	p.now = func() time.Time { return brapiNow }

	history, err := p.History(context.Background(), petr4, date(2026, 9, 25), date(2026, 9, 30))
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history.Prices) != 1 || history.Prices[0].Price.String() != "49.77" {
		t.Fatalf("prices = %+v, want only the row with a real close", history.Prices)
	}
}

// A plan that does not serve the depth asked for refuses the request; the
// provider then asks for less and reports how far back it actually reaches,
// so the caller can record what was asked and stop asking for the rest.
func TestBrapiHistoryFallsBackToAShorterRangeWhenThePlanRefuses(t *testing.T) {
	var ranges []string
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/BOVA11", func(w http.ResponseWriter, r *http.Request) {
		requested := r.URL.Query().Get("range")
		ranges = append(ranges, requested)
		if requested != "3mo" {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]any{"error": true, "message": "plan limit"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{
			"historicalDataPrice": []map[string]any{historyRow(sessionStamp(2026, 9, 30), 130.5, 130.5)},
		}}})
	})
	p := newTestBrapi(t, mux, "token")
	p.now = func() time.Time { return brapiNow }

	// 242 days back: a year would be asked first.
	history, err := p.History(context.Background(), Instrument{Market: MarketB3, Symbol: "BOVA11"}, date(2026, 2, 1), date(2026, 9, 30))
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if want := []string{"1y", "6mo", "3mo"}; len(ranges) != 3 || ranges[0] != want[0] || ranges[1] != want[1] || ranges[2] != want[2] {
		t.Errorf("ranges tried = %v, want %v", ranges, want)
	}
	if len(history.Prices) != 1 {
		t.Fatalf("prices = %+v, want the one the 3mo answer holds", history.Prices)
	}
	// 3mo reaches 92 days before 2026-10-01.
	if want := date(2026, 7, 1); !history.CoveredFrom.Equal(want) {
		t.Errorf("CoveredFrom = %s, want %s: the answer does not go back to the requested start",
			history.CoveredFrom.Format(time.DateOnly), want.Format(time.DateOnly))
	}
}

func TestBrapiHistoryDoesNotFallBackOnAMissingTokenOrAnOutage(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusServiceUnavailable} {
		calls := 0
		mux := http.NewServeMux()
		mux.HandleFunc("/quote/BOVA11", func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.WriteHeader(status)
			json.NewEncoder(w).Encode(map[string]any{"error": true, "code": "MISSING_TOKEN"})
		})
		p := newTestBrapi(t, mux, "")
		p.now = func() time.Time { return brapiNow }

		if _, err := p.History(context.Background(), Instrument{Market: MarketB3, Symbol: "BOVA11"}, date(2026, 2, 1), date(2026, 9, 30)); err == nil {
			t.Fatalf("status %d: expected the error to surface", status)
		}
		if want := p.anonymous.maxAttempts; status >= 500 && calls != want {
			t.Errorf("status %d: calls = %d, want %d retries of one range", status, calls, want)
		} else if status < 500 && calls != 1 {
			t.Errorf("status %d: calls = %d, want 1: a shorter range cannot fix a missing token", status, calls)
		}
	}
}

func TestBrapiHistoryWithoutHistoryRowsIsAnError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/quote/PETR4", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"results": []map[string]any{{"symbol": "PETR4"}}})
	})
	p := newTestBrapi(t, mux, "")
	p.now = func() time.Time { return brapiNow }

	if _, err := p.History(context.Background(), petr4, date(2026, 9, 1), date(2026, 9, 30)); err == nil {
		t.Fatal("expected an error when brapi returns no history for the ticker")
	}
}
