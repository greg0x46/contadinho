package marketdata

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

// fakeYahoo serves captured chart responses by symbol and records what was
// asked of it.
type fakeYahoo struct {
	t        *testing.T
	bodies   map[string][]byte // symbol → response body
	statuses map[string]int    // symbol → status, when not 200
	calls    map[string]int
	queries  map[string][]url.Values
}

func (f *fakeYahoo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if got := r.Header.Get("User-Agent"); got != yahooUserAgent {
		f.t.Errorf("User-Agent = %q, want the browser one", got)
	}
	symbol := strings.TrimPrefix(r.URL.Path, "/")
	f.calls[symbol]++
	f.queries[symbol] = append(f.queries[symbol], r.URL.Query())
	if status := f.statuses[symbol]; status != 0 {
		w.WriteHeader(status)
	}
	body, ok := f.bodies[symbol]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		body = fixture(f.t, "yahoo_notfound.json")
	}
	w.Write(body)
}

func newTestYahoo(t *testing.T, fixtures map[string]string) (*YahooProvider, *fakeYahoo, *time.Time) {
	t.Helper()
	fake := &fakeYahoo{t: t, bodies: map[string][]byte{}, statuses: map[string]int{},
		calls: map[string]int{}, queries: map[string][]url.Values{}}
	for symbol, name := range fixtures {
		fake.bodies[symbol] = fixture(t, name)
	}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	p := NewYahooProvider()
	p.baseURL = server.URL
	pointAt(p.http, server)
	now := time.Date(2026, 10, 1, 23, 10, 0, 0, time.UTC)
	p.now = func() time.Time { return now }
	return p, fake, &now
}

var (
	petr4 = Instrument{Market: MarketB3, Symbol: "PETR4"}
	btc   = Instrument{Market: MarketCrypto, Symbol: "BTC"}
	eth   = Instrument{Market: MarketCrypto, Symbol: "ETH"}
)

func TestYahooQuoteB3IsTheRegularMarketPrice(t *testing.T) {
	p, fake, _ := newTestYahoo(t, map[string]string{
		"PETR4.SA":  "yahoo_petr4_spot.json",
		"HGLG11.SA": "yahoo_hglg11_spot.json",
		"AAPL34.SA": "yahoo_aapl34_spot.json",
	})

	for symbol, want := range map[string]string{"PETR4": "49.77", "HGLG11": "146.99", "AAPL34": "86.24"} {
		quote, err := p.Quote(context.Background(), Instrument{Market: MarketB3, Symbol: symbol})
		if err != nil {
			t.Fatalf("Quote(%s): %v", symbol, err)
		}
		if quote.Price.String() != want || quote.Currency != "BRL" {
			t.Errorf("Quote(%s) = %s %s, want %s BRL", symbol, quote.Price, quote.Currency, want)
		}
	}
	query := fake.queries["PETR4.SA"][0]
	if query.Get("range") != "1d" || query.Get("interval") != "1d" {
		t.Errorf("query = %v, want range=1d&interval=1d", query)
	}
}

func TestYahooQuoteCryptoIsTheDollarPriceAtTheCachedRate(t *testing.T) {
	p, fake, now := newTestYahoo(t, map[string]string{
		"BTC-USD": "yahoo_btcusd_spot.json",
		"ETH-USD": "yahoo_ethusd_spot.json",
		"BRL=X":   "yahoo_brlx_spot.json",
	})

	quote, err := p.Quote(context.Background(), btc)
	if err != nil {
		t.Fatalf("Quote(BTC): %v", err)
	}
	// 84659.34 USD × 5.2225 BRL/USD = 442133.403…, kept to cents.
	if quote.Price.String() != "442133.4" || quote.Currency != "BRL" {
		t.Errorf("BTC = %s %s, want 442133.40 BRL", quote.Price, quote.Currency)
	}
	quote, err = p.Quote(context.Background(), eth)
	if err != nil {
		t.Fatalf("Quote(ETH): %v", err)
	}
	if quote.Price.String() != "14096" {
		t.Errorf("ETH = %s, want 2699.09 × 5.2225 = 14096.00", quote.Price)
	}
	if fake.calls["BRL=X"] != 1 || fake.calls["BTC-USD"] != 1 || fake.calls["ETH-USD"] != 1 {
		t.Errorf("calls = %v, want one rate lookup shared by both coins", fake.calls)
	}

	*now = now.Add(11 * time.Minute)
	if _, err := p.Quote(context.Background(), btc); err != nil {
		t.Fatalf("Quote(BTC) later: %v", err)
	}
	if fake.calls["BRL=X"] != 2 {
		t.Errorf("rate lookups = %d, want a fresh one once the cached rate is stale", fake.calls["BRL=X"])
	}
}

func TestYahooQuoteRefusesAnotherCurrency(t *testing.T) {
	p, _, _ := newTestYahoo(t, map[string]string{"PETR4.SA": "yahoo_btcusd_spot.json"})

	_, err := p.Quote(context.Background(), petr4)
	if err == nil || errors.Is(err, ErrNotFound) || !strings.Contains(err.Error(), `"USD"`) {
		t.Fatalf("error = %v, want a currency mismatch", err)
	}
}

func TestYahooQuoteCryptoRefusesARateInAnotherCurrency(t *testing.T) {
	p, _, _ := newTestYahoo(t, map[string]string{"BTC-USD": "yahoo_btcusd_spot.json", "BRL=X": "yahoo_btcusd_spot.json"})

	if _, err := p.Quote(context.Background(), btc); err == nil {
		t.Fatal("expected an error when the rate is not in BRL")
	}
}

func TestYahooUnknownSymbolIsNotFound(t *testing.T) {
	p, fake, _ := newTestYahoo(t, map[string]string{"BRL=X": "yahoo_brlx_spot.json"})

	if _, err := p.Quote(context.Background(), Instrument{Market: MarketB3, Symbol: "NAOEX3"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("b3 error = %v, want ErrNotFound", err)
	}
	if _, err := p.Quote(context.Background(), Instrument{Market: MarketCrypto, Symbol: "NOPE"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("crypto error = %v, want ErrNotFound", err)
	}
	if _, err := p.History(context.Background(), petr4, date(2026, 9, 1), date(2026, 9, 30)); !errors.Is(err, ErrNotFound) {
		t.Errorf("history error = %v, want ErrNotFound", err)
	}
	if fake.calls["NOPE-USD"] != 1 || fake.calls["BRL=X"] != 0 {
		t.Errorf("calls = %v, want the rate left alone for a coin that does not exist", fake.calls)
	}
}

// Yahoo not having the exchange rate says nothing about the coin, so it must
// not let the service conclude the coin does not exist.
func TestYahooMissingRateIsNotNotFound(t *testing.T) {
	p, _, _ := newTestYahoo(t, map[string]string{"BTC-USD": "yahoo_btcusd_spot.json"})

	_, err := p.Quote(context.Background(), btc)
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want a failure that is not ErrNotFound", err)
	}
}

func TestYahooRateLimitAndOutage(t *testing.T) {
	p, fake, _ := newTestYahoo(t, map[string]string{"PETR4.SA": "yahoo_petr4_spot.json"})
	fake.statuses["PETR4.SA"] = http.StatusServiceUnavailable
	if _, err := p.Quote(context.Background(), petr4); !errors.Is(err, ErrUnavailable) {
		t.Errorf("5xx error = %v, want ErrUnavailable", err)
	}

	fake.statuses["PETR4.SA"] = http.StatusTooManyRequests
	_, err := p.Quote(context.Background(), petr4)
	var limited *RateLimitedError
	if !errors.As(err, &limited) || limited.Provider != ProviderYahoo {
		t.Errorf("429 error = %v, want a *RateLimitedError from yahoo", err)
	}
}

func TestYahooOtherClientErrorIsAPlainFailure(t *testing.T) {
	p, fake, _ := newTestYahoo(t, map[string]string{"PETR4.SA": "yahoo_notfound.json"})
	fake.statuses["PETR4.SA"] = http.StatusBadRequest

	_, err := p.Quote(context.Background(), petr4)
	if err == nil || errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnavailable) {
		t.Fatalf("error = %v, want a plain failure", err)
	}
}

func TestYahooMalformedBodyIsAnError(t *testing.T) {
	p, fake, _ := newTestYahoo(t, nil)
	fake.bodies["PETR4.SA"] = []byte(`{"chart":{"result":[{"meta":{"currency":"BRL"},"timestamp":[1],"indicators":{"quote":[{"close":[]}]}}]}}`)
	if _, err := p.History(context.Background(), petr4, date(2026, 9, 1), date(2026, 9, 30)); err == nil {
		t.Error("expected an error for timestamps without closes")
	}
	fake.bodies["PETR4.SA"] = []byte(`{"chart":{"result":[{"meta":{"currency":"BRL"}}]}}`)
	if _, err := p.Quote(context.Background(), petr4); err == nil {
		t.Error("expected an error for a chart without a price")
	}
	fake.bodies["PETR4.SA"] = []byte(`not json`)
	if _, err := p.Quote(context.Background(), petr4); err == nil {
		t.Error("expected an error for a body that is not JSON")
	}
}

func TestYahooHistoryB3(t *testing.T) {
	p, fake, _ := newTestYahoo(t, map[string]string{"PETR4.SA": "yahoo_petr4_history.json"})

	history, err := p.History(context.Background(), petr4, date(2026, 9, 18), date(2026, 9, 25))
	if err != nil {
		t.Fatalf("History: %v", err)
	}

	query := fake.queries["PETR4.SA"][0]
	if got, want := query.Get("period1"), strconv.FormatInt(date(2026, 9, 17).Unix(), 10); got != want {
		t.Errorf("period1 = %s, want %s (a day before from)", got, want)
	}
	if got, want := query.Get("period2"), strconv.FormatInt(date(2026, 9, 27).Unix(), 10); got != want {
		t.Errorf("period2 = %s, want %s (two days after to)", got, want)
	}
	if query.Get("interval") != "1d" || query.Has("range") {
		t.Errorf("query = %v, want daily bars over the period", query)
	}

	// The fixture runs 17 to 28 September; the 17th and 28th are outside.
	want := []struct {
		day   time.Time
		price string
	}{
		{date(2026, 9, 18), "48.5"},
		{date(2026, 9, 21), "48"},
		{date(2026, 9, 22), "48.35"},
		{date(2026, 9, 23), "49.6"},
		{date(2026, 9, 24), "49.26"}, // 49.2599983215332 on the wire
		{date(2026, 9, 25), "48.07"},
	}
	if len(history.Prices) != len(want) {
		t.Fatalf("prices = %v, want %d sessions", history.Prices, len(want))
	}
	for i, w := range want {
		got := history.Prices[i]
		if !got.Day.Equal(w.day) || got.Price.String() != w.price {
			t.Errorf("price %d = %s %s, want %s %s", i, got.Day.Format(time.DateOnly), got.Price, w.day.Format(time.DateOnly), w.price)
		}
	}
	if !history.CoveredFrom.Equal(date(2026, 9, 18)) || history.Currency != "BRL" {
		t.Errorf("CoveredFrom = %s, Currency = %s; want from and BRL", history.CoveredFrom, history.Currency)
	}
}

// Bars are dated in the exchange's zone, null and zero closes leave the day
// unpriced, and the close is read — never the dividend-adjusted close.
func TestYahooHistoryB3DatesBarsInTheExchangeZoneAndSkipsMissingCloses(t *testing.T) {
	p, fake, _ := newTestYahoo(t, nil)
	stamp := func(t time.Time) string { return strconv.FormatInt(t.Unix(), 10) }
	fake.bodies["PETR4.SA"] = []byte(`{"chart":{"result":[{"meta":{"currency":"BRL","exchangeTimezoneName":"America/Sao_Paulo","gmtoffset":-10800,"priceHint":2},` +
		`"timestamp":[` + stamp(time.Date(2026, 9, 22, 13, 0, 0, 0, time.UTC)) + `,` +
		stamp(time.Date(2026, 9, 24, 2, 0, 0, 0, time.UTC)) + `,` + // 23:00 on the 23rd in São Paulo
		stamp(time.Date(2026, 9, 24, 13, 0, 0, 0, time.UTC)) + `,` +
		stamp(time.Date(2026, 9, 25, 13, 0, 0, 0, time.UTC)) + `],` +
		`"indicators":{"quote":[{"close":[48.349998474121094,49.599998474121094,null,0]}],"adjclose":[{"adjclose":[40.1,40.2,40.3,40.4]}]}}],"error":null}}`)

	history, err := p.History(context.Background(), petr4, date(2026, 9, 21), date(2026, 9, 25))
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history.Prices) != 2 {
		t.Fatalf("prices = %v, want the 22nd and the 23rd only", history.Prices)
	}
	if !history.Prices[1].Day.Equal(date(2026, 9, 23)) || history.Prices[1].Price.String() != "49.6" {
		t.Errorf("second = %+v, want 49.60 on 2026-09-23 (São Paulo date of 02:00Z on the 24th)", history.Prices[1])
	}
	if history.Prices[0].Price.String() != "48.35" {
		t.Errorf("first = %s, want the close 48.35, not adjclose", history.Prices[0].Price)
	}
}

// Crypto trades every day, the dollar does not: a weekend is converted at
// the Friday rate. The rate's bars are stamped 23:00Z, the London midnight
// that opens the next day, and are dated in London.
func TestYahooHistoryCryptoCarriesTheRateOverTheWeekend(t *testing.T) {
	p, fake, _ := newTestYahoo(t, map[string]string{
		"BTC-USD": "yahoo_btcusd_history.json",
		"BRL=X":   "yahoo_brlx_history.json",
	})

	history, err := p.History(context.Background(), btc, date(2026, 9, 18), date(2026, 9, 28))
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	for _, symbol := range []string{"BTC-USD", "BRL=X"} {
		query := fake.queries[symbol][0]
		if got, want := query.Get("period1"), strconv.FormatInt(date(2026, 9, 11).Unix(), 10); got != want {
			t.Errorf("%s period1 = %s, want %s (a week before from)", symbol, got, want)
		}
		if got, want := query.Get("period2"), strconv.FormatInt(date(2026, 9, 30).Unix(), 10); got != want {
			t.Errorf("%s period2 = %s, want %s", symbol, got, want)
		}
	}
	if len(history.Prices) != 11 {
		t.Fatalf("prices = %v, want every day from the 18th to the 28th", history.Prices)
	}
	byDay := map[time.Time]string{}
	for _, price := range history.Prices {
		byDay[price.Day] = price.Price.StringFixed(2)
	}
	for _, tc := range []struct {
		day  time.Time
		want string
		why  string
	}{
		{date(2026, 9, 18), "414547.17", "80901.46 × Friday's 5.1241"},
		{date(2026, 9, 19), "416249.50", "81233.68 × Friday's 5.1241"},
		{date(2026, 9, 20), "415782.85", "81142.61 × Friday's 5.1241"},
		{date(2026, 9, 21), "445199.58", "86602.91 × Monday's 5.1407"},
		{date(2026, 9, 26), "438111.68", "84406.45 × Friday's 5.1905"},
		{date(2026, 9, 27), "438379.72", "84458.09 × Friday's 5.1905"},
		{date(2026, 9, 28), "433169.79", "83502.61 × Monday's 5.1875"},
	} {
		if got := byDay[tc.day]; got != tc.want {
			t.Errorf("%s = %s, want %s (%s)", tc.day.Format(time.DateOnly), got, tc.want, tc.why)
		}
	}
	if !history.CoveredFrom.Equal(date(2026, 9, 18)) || history.Currency != "BRL" {
		t.Errorf("CoveredFrom = %s, Currency = %s", history.CoveredFrom, history.Currency)
	}
}

// A day before the first known rate cannot be converted and stays unpriced;
// the rate's open current-day bar (close null) is ignored.
func TestYahooHistoryCryptoSkipsDaysWithoutARate(t *testing.T) {
	p, _, _ := newTestYahoo(t, map[string]string{
		"BTC-USD": "yahoo_btcusd_history.json",
		"BRL=X":   "yahoo_brlx_spot.json", // rates from the 28th on
	})

	history, err := p.History(context.Background(), btc, date(2026, 9, 26), date(2026, 9, 29))
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history.Prices) != 2 {
		t.Fatalf("prices = %v, want only the 28th and the 29th", history.Prices)
	}
	if got := history.Prices[1]; !got.Day.Equal(date(2026, 9, 29)) || got.Price.StringFixed(2) != "436751.59" {
		t.Errorf("29th = %+v, want 83622.43 × 5.2229 = 436751.59", got)
	}
}
