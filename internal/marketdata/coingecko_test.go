package marketdata

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

// newTestCoinGecko points the provider at mux, with "BTC" resolving to
// "bitcoin" unless the test registers its own /search. searches counts the
// lookups.
func newTestCoinGecko(t *testing.T, mux *http.ServeMux) (*CoinGeckoProvider, *int) {
	t.Helper()
	searches := new(int)
	search := func(w http.ResponseWriter, r *http.Request) {
		*searches++
		json.NewEncoder(w).Encode(map[string]any{"coins": []map[string]any{
			{"id": "bitcoin", "symbol": "BTC", "name": "Bitcoin", "market_cap_rank": 1},
		}})
	}
	wrapped := http.NewServeMux()
	wrapped.Handle("/", mux)
	if _, pattern := mux.Handler(&http.Request{Method: http.MethodGet, URL: mustParse(t, "/search")}); pattern == "" {
		wrapped.HandleFunc("/search", search)
	}
	server := httptest.NewServer(wrapped)
	t.Cleanup(server.Close)
	p := NewCoinGeckoProvider()
	p.baseURL = server.URL
	pointAt(p.http, server)
	return p, searches
}

func TestCoinGeckoQuoteHappyPath(t *testing.T) {
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
	p, searches := newTestCoinGecko(t, mux)

	quote, err := p.Quote(context.Background(), btc)
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if quote.Price.String() != "433155.42" || quote.Currency != "BRL" || quote.Instrument != btc {
		t.Errorf("quote = %+v, want 433155.42 BRL for BTC", quote)
	}
	if _, err := p.Quote(context.Background(), btc); err != nil {
		t.Fatalf("second Quote: %v", err)
	}
	if *searches != 1 {
		t.Errorf("searches = %d, want the ticker resolved once and remembered", *searches)
	}
}

// CoinGecko answers an id it does not recognize with HTTP 200 and an empty
// {} object rather than an error status (verified live).
func TestCoinGeckoQuoteEmptyAnswerIsNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/price", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{})
	})
	p, _ := newTestCoinGecko(t, mux)

	if _, err := p.Quote(context.Background(), btc); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestCoinGeckoUnknownTickerIsNotFoundAndNotRemembered(t *testing.T) {
	searches := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		searches++
		json.NewEncoder(w).Encode(map[string]any{"coins": []any{}})
	})
	p, _ := newTestCoinGecko(t, mux)
	p.now = func() time.Time { return coinGeckoNow }

	nope := Instrument{Market: MarketCrypto, Symbol: "NOPE"}
	if _, err := p.Quote(context.Background(), nope); !errors.Is(err, ErrNotFound) {
		t.Errorf("Quote error = %v, want ErrNotFound", err)
	}
	if _, err := p.History(context.Background(), nope, date(2026, 9, 1), date(2026, 9, 30)); !errors.Is(err, ErrNotFound) {
		t.Errorf("History error = %v, want ErrNotFound", err)
	}
	if searches != 2 {
		t.Errorf("searches = %d, want 2: a miss is asked again", searches)
	}
}

func TestCoinGeckoUnknownCoinIDIsNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/coins/bitcoin/market_chart/range", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]any{"error": "coin not found"})
	})
	p, _ := newTestCoinGecko(t, mux)
	p.now = func() time.Time { return coinGeckoNow }

	if _, err := p.History(context.Background(), btc, date(2026, 9, 1), date(2026, 9, 30)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestCoinGeckoQuoteNonPositivePrice(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/price", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"bitcoin": map[string]any{"brl": 0}})
	})
	p, _ := newTestCoinGecko(t, mux)

	if _, err := p.Quote(context.Background(), btc); err == nil {
		t.Fatal("expected an error for a non-positive price")
	}
}

func TestCoinGeckoQuoteRetriesOn429ThenSucceeds(t *testing.T) {
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
	p, _ := newTestCoinGecko(t, mux)

	quote, err := p.Quote(context.Background(), btc)
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if attempts != 2 || quote.Price.String() != "433155" {
		t.Errorf("attempts = %d, price = %s; want 2 and 433155", attempts, quote.Price)
	}
}

func TestCoinGeckoQuoteRetriesExhaustedIsRateLimited(t *testing.T) {
	attempts := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/simple/price", func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusTooManyRequests)
	})
	p, _ := newTestCoinGecko(t, mux)

	_, err := p.Quote(context.Background(), btc)
	var limited *RateLimitedError
	if !errors.As(err, &limited) || limited.Provider != ProviderCoinGecko {
		t.Fatalf("error = %v, want a *RateLimitedError from coingecko", err)
	}
	if attempts != p.http.maxAttempts {
		t.Errorf("attempts = %d, want %d", attempts, p.http.maxAttempts)
	}
}

func TestCoinGeckoSupportsOnlyCrypto(t *testing.T) {
	p := NewCoinGeckoProvider()
	if !p.Supports(MarketCrypto) || p.Supports(MarketB3) || p.Name() != ProviderCoinGecko {
		t.Error("coingecko serves crypto only")
	}
}

var coinGeckoNow = time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC)

func dayStamp(d time.Time) int64 { return d.UnixMilli() }

func TestCoinGeckoHistoryParsesDailyPoints(t *testing.T) {
	var gotQuery map[string][]string
	mux := http.NewServeMux()
	mux.HandleFunc("/coins/bitcoin/market_chart/range", func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		json.NewEncoder(w).Encode(map[string]any{"prices": [][]any{
			{dayStamp(date(2026, 8, 31)), 380000.5}, // before from
			{dayStamp(date(2026, 9, 1)), 381000.25},
			{dayStamp(date(2026, 9, 2)), 0}, // not a price
			{dayStamp(date(2026, 9, 3)), 383000},
			{dayStamp(date(2026, 9, 3).Add(6 * time.Hour)), 383500}, // second point of the same day wins
			{dayStamp(date(2026, 10, 1)), 432982.88},                // after to
		}})
	})
	p, _ := newTestCoinGecko(t, mux)
	p.now = func() time.Time { return coinGeckoNow }

	history, err := p.History(context.Background(), btc, date(2026, 9, 1), date(2026, 9, 30))
	if err != nil {
		t.Fatalf("History: %v", err)
	}

	if got := gotQuery["vs_currency"]; len(got) != 1 || got[0] != "brl" {
		t.Errorf("vs_currency = %v, want brl", got)
	}
	if got := gotQuery["interval"]; len(got) != 1 || got[0] != "daily" {
		t.Errorf("interval = %v, want daily", got)
	}
	if got, want := gotQuery["from"][0], strconv.FormatInt(date(2026, 9, 1).Unix(), 10); got != want {
		t.Errorf("from = %s, want %s", got, want)
	}
	// to is the last second of the last day, so that day's 00:00 point is in.
	if got, want := gotQuery["to"][0], strconv.FormatInt(date(2026, 10, 1).Unix()-1, 10); got != want {
		t.Errorf("to = %s, want %s", got, want)
	}

	if len(history.Prices) != 2 {
		t.Fatalf("prices = %+v, want 2026-09-01 and 2026-09-03", history.Prices)
	}
	if history.Prices[0].Price.String() != "381000.25" || history.Prices[1].Price.String() != "383500" {
		t.Errorf("prices = %v and %v, want 381000.25 and the later 383500", history.Prices[0].Price, history.Prices[1].Price)
	}
	if !history.CoveredFrom.Equal(date(2026, 9, 1)) || history.Currency != "BRL" {
		t.Errorf("CoveredFrom = %s, Currency = %q; want 2026-09-01 in BRL", history.CoveredFrom.Format(time.DateOnly), history.Currency)
	}
}

// The public API refuses anything older than 365 days (401, error_code
// 10012). The provider clamps the start instead of sending a request that is
// certain to fail, and says how far back it actually reaches.
func TestCoinGeckoHistoryClampsToThePublicLookback(t *testing.T) {
	var gotFrom string
	mux := http.NewServeMux()
	mux.HandleFunc("/coins/bitcoin/market_chart/range", func(w http.ResponseWriter, r *http.Request) {
		gotFrom = r.URL.Query().Get("from")
		json.NewEncoder(w).Encode(map[string]any{"prices": [][]any{}})
	})
	p, _ := newTestCoinGecko(t, mux)
	p.now = func() time.Time { return coinGeckoNow }

	history, err := p.History(context.Background(), btc, date(2024, 1, 1), date(2026, 9, 30))
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	earliest := date(2026, 10, 1).AddDate(0, 0, -364)
	if want := strconv.FormatInt(earliest.Unix(), 10); gotFrom != want {
		t.Errorf("from = %s, want %s (364 days before today)", gotFrom, want)
	}
	if !history.CoveredFrom.Equal(earliest) {
		t.Errorf("CoveredFrom = %s, want %s", history.CoveredFrom.Format(time.DateOnly), earliest.Format(time.DateOnly))
	}
}

func TestCoinGeckoHistoryOfOnlyTooOldDaysMakesNoRequest(t *testing.T) {
	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/coins/bitcoin/market_chart/range", func(w http.ResponseWriter, r *http.Request) { calls++ })
	p, searches := newTestCoinGecko(t, mux)
	p.now = func() time.Time { return coinGeckoNow }

	history, err := p.History(context.Background(), btc, date(2024, 1, 1), date(2024, 12, 31))
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if calls != 0 || *searches != 0 || len(history.Prices) != 0 {
		t.Errorf("calls = %d, searches = %d, prices = %v; want none: nothing that old can be served", calls, *searches, history.Prices)
	}
}

func TestCoinGeckoResolvesTheHighestRankedExactSymbol(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("query"); got != "BTC" {
			t.Errorf("query = %q, want the ticker", got)
		}
		json.NewEncoder(w).Encode(map[string]any{"coins": []map[string]any{
			{"id": "bitget-wrapped-btc", "symbol": "BGBTC", "name": "Bitget Wrapped BTC", "market_cap_rank": 291},
			{"id": "bitcoin", "symbol": "BTC", "name": "Bitcoin", "market_cap_rank": 1},
			{"id": "bitcoin-clone", "symbol": "BTC", "name": "Bitcoin Clone", "market_cap_rank": 1500},
		}})
	})
	p, _ := newTestCoinGecko(t, mux)

	id, err := p.coinID(context.Background(), btc)
	if err != nil || id != "bitcoin" {
		t.Fatalf("coinID(BTC) = %q, %v; want bitcoin", id, err)
	}
}

func TestPickCoinPrefersSymbolThenIDThenName(t *testing.T) {
	coins := []any{
		map[string]any{"id": "bitcoin-cash", "symbol": "BCH", "name": "Bitcoin Cash", "market_cap_rank": json.Number("20")},
		map[string]any{"id": "bitcoin", "symbol": "BTC", "name": "Bitcoin", "market_cap_rank": json.Number("1")},
		map[string]any{"id": "unranked-btc", "symbol": "BTC", "name": "Unranked", "market_cap_rank": nil},
	}
	for _, tc := range []struct {
		query, want string
	}{
		{"btc", "bitcoin"},               // symbol, ranked beats unranked
		{"BITCOIN", "bitcoin"},           // no symbol "BITCOIN": the id matches
		{"Bitcoin Cash", "bitcoin-cash"}, // only the name matches
		{"BCH", "bitcoin-cash"},
	} {
		got, ok := pickCoin(tc.query, coins)
		if !ok || got != tc.want {
			t.Errorf("pickCoin(%q) = %q, %v; want %q", tc.query, got, ok, tc.want)
		}
	}
	if got, ok := pickCoin("doge", coins); ok {
		t.Errorf("pickCoin(doge) = %q, want no match", got)
	}

	onlyUnranked := []any{
		map[string]any{"id": "first", "symbol": "ZZZ", "market_cap_rank": nil},
		map[string]any{"id": "second", "symbol": "ZZZ", "market_cap_rank": nil},
	}
	if got, _ := pickCoin("ZZZ", onlyUnranked); got != "first" {
		t.Errorf("pickCoin among unranked = %q, want the API's first", got)
	}
}
