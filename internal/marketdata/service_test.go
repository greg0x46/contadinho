package marketdata

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// fakeProvider answers with whatever the test sets and counts the calls.
type fakeProvider struct {
	name         string
	markets      []Market
	quote        func(context.Context, Instrument) (Quote, error)
	history      func(context.Context, Instrument, time.Time, time.Time) (History, error)
	quoteCalls   int
	historyCalls int
}

func (f *fakeProvider) Name() string { return f.name }

func (f *fakeProvider) Supports(market Market) bool {
	for _, m := range f.markets {
		if m == market {
			return true
		}
	}
	return false
}

func (f *fakeProvider) Quote(ctx context.Context, instrument Instrument) (Quote, error) {
	f.quoteCalls++
	return f.quote(ctx, instrument)
}

func (f *fakeProvider) History(ctx context.Context, instrument Instrument, from, to time.Time) (History, error) {
	f.historyCalls++
	return f.history(ctx, instrument, from, to)
}

func pricedAt(price string) func(context.Context, Instrument) (Quote, error) {
	return func(context.Context, Instrument) (Quote, error) {
		return Quote{Price: decimal.RequireFromString(price), Currency: "BRL"}, nil
	}
}

func failingQuote(err error) func(context.Context, Instrument) (Quote, error) {
	return func(context.Context, Instrument) (Quote, error) { return Quote{}, err }
}

func failingHistory(err error) func(context.Context, Instrument, time.Time, time.Time) (History, error) {
	return func(context.Context, Instrument, time.Time, time.Time) (History, error) { return History{}, err }
}

func historyOf(prices ...DailyPrice) func(context.Context, Instrument, time.Time, time.Time) (History, error) {
	return func(_ context.Context, _ Instrument, from, _ time.Time) (History, error) {
		return History{Prices: prices, Currency: "BRL", CoveredFrom: from}, nil
	}
}

func dp(d time.Time, price string) DailyPrice {
	return DailyPrice{Day: d, Price: decimal.RequireFromString(price)}
}

func newTestService(providers ...Provider) (*Service, *time.Time) {
	s := New(providers...)
	now := time.Date(2026, 10, 1, 15, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	return s, &now
}

func TestServiceFallsBackToTheNextProvider(t *testing.T) {
	primary := &fakeProvider{name: "primary", markets: []Market{MarketB3},
		quote:   failingQuote(fmt.Errorf("primary: %w", ErrUnavailable)),
		history: failingHistory(fmt.Errorf("primary: %w", ErrUnavailable))}
	secondary := &fakeProvider{name: "secondary", markets: []Market{MarketB3},
		quote: pricedAt("38.29"), history: historyOf(dp(date(2026, 9, 29), "38"))}
	s, now := newTestService(primary, secondary)

	quote, err := s.Quote(context.Background(), petr4)
	if err != nil {
		t.Fatalf("Quote: %v", err)
	}
	if quote.Provider != "secondary" || quote.Instrument != petr4 || !quote.At.Equal(*now) || quote.Price.String() != "38.29" {
		t.Errorf("quote = %+v, want 38.29 from secondary stamped now", quote)
	}

	history, err := s.History(context.Background(), petr4, date(2026, 9, 1), date(2026, 9, 30))
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if history.Provider != "secondary" || !history.Fallback || history.Instrument != petr4 || len(history.Prices) != 1 {
		t.Errorf("history = %+v, want secondary's, marked as a fallback", history)
	}
}

func TestServiceFirstProviderIsNotAFallback(t *testing.T) {
	primary := &fakeProvider{name: "primary", markets: []Market{MarketB3}, history: historyOf(dp(date(2026, 9, 29), "38"))}
	secondary := &fakeProvider{name: "secondary", markets: []Market{MarketB3}}
	s, _ := newTestService(primary, secondary)

	history, err := s.History(context.Background(), petr4, date(2026, 9, 1), date(2026, 9, 30))
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if history.Provider != "primary" || history.Fallback || secondary.historyCalls != 0 {
		t.Errorf("history = %+v, secondary asked %d times; want primary's, not a fallback", history, secondary.historyCalls)
	}
}

// Fallback is about the chain for the instrument's market: a provider that
// does not serve the market is neither asked nor counted.
func TestServiceOnlyAsksProvidersThatSupportTheMarket(t *testing.T) {
	b3Only := &fakeProvider{name: "b3only", markets: []Market{MarketB3}}
	crypto := &fakeProvider{name: "crypto", markets: []Market{MarketCrypto},
		quote: pricedAt("433155.42"), history: historyOf(dp(date(2026, 9, 29), "430000"))}
	s, _ := newTestService(b3Only, crypto)

	quote, err := s.Quote(context.Background(), btc)
	if err != nil || quote.Provider != "crypto" {
		t.Fatalf("Quote = %+v, %v; want crypto's", quote, err)
	}
	history, err := s.History(context.Background(), btc, date(2026, 9, 1), date(2026, 9, 30))
	if err != nil || history.Fallback {
		t.Fatalf("History = %+v, %v; want the first crypto provider, not a fallback", history, err)
	}
	if b3Only.quoteCalls+b3Only.historyCalls != 0 {
		t.Error("a provider that does not serve crypto was asked")
	}
}

func TestServiceWithoutAProviderForTheMarket(t *testing.T) {
	s, _ := newTestService(&fakeProvider{name: "b3only", markets: []Market{MarketB3}})

	if _, err := s.Quote(context.Background(), btc); !errors.Is(err, ErrNoProvider) {
		t.Errorf("Quote error = %v, want ErrNoProvider", err)
	}
	if _, err := s.History(context.Background(), btc, date(2026, 9, 1), date(2026, 9, 30)); !errors.Is(err, ErrNoProvider) {
		t.Errorf("History error = %v, want ErrNoProvider", err)
	}
}

func TestServiceNotFoundOnlyWhenNoProviderKnowsTheInstrument(t *testing.T) {
	a := &fakeProvider{name: "a", markets: []Market{MarketB3}, quote: failingQuote(fmt.Errorf("a: %w", ErrNotFound))}
	b := &fakeProvider{name: "b", markets: []Market{MarketB3}, quote: failingQuote(fmt.Errorf("b: %w", ErrNotFound))}
	s, _ := newTestService(a, b)

	_, err := s.Quote(context.Background(), petr4)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
	if !strings.Contains(err.Error(), "b3:PETR4") || !strings.Contains(err.Error(), "a: ") || !strings.Contains(err.Error(), "b: ") {
		t.Errorf("error = %q, want the instrument and every provider's reason", err)
	}

	// One provider merely rate limited: it might know the instrument.
	b.quote = failingQuote(fmt.Errorf("b: %w", &RateLimitedError{Provider: "b", RetryAfter: time.Minute}))
	_, err = s.Quote(context.Background(), petr4)
	if errors.Is(err, ErrNotFound) {
		t.Errorf("error = %v, must not claim not found when a provider did not answer", err)
	}
	var limited *RateLimitedError
	if !errors.As(err, &limited) || limited.Provider != "b" {
		t.Errorf("error = %v, want the *RateLimitedError reachable for deferral", err)
	}
	if !strings.Contains(err.Error(), "a: instrument not found") {
		t.Errorf("error = %q, want a's reason kept in the message", err)
	}
}

func TestServiceStopsTheChainWhenTheCallerGivesUp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	primary := &fakeProvider{name: "primary", markets: []Market{MarketB3},
		quote: func(context.Context, Instrument) (Quote, error) {
			cancel()
			return Quote{}, fmt.Errorf("primary: %w", ErrUnavailable)
		}}
	secondary := &fakeProvider{name: "secondary", markets: []Market{MarketB3}, quote: pricedAt("1")}
	s, _ := newTestService(primary, secondary)

	if _, err := s.Quote(ctx, petr4); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if secondary.quoteCalls != 0 {
		t.Error("the next provider was asked after the caller gave up")
	}
}

func TestServiceReusesAQuoteForTenMinutes(t *testing.T) {
	provider := &fakeProvider{name: "p", markets: []Market{MarketB3}, quote: pricedAt("38.29")}
	s, now := newTestService(provider)

	first, _ := s.Quote(context.Background(), petr4)
	*now = now.Add(9 * time.Minute)
	second, err := s.Quote(context.Background(), petr4)
	if err != nil || provider.quoteCalls != 1 || !second.At.Equal(first.At) {
		t.Fatalf("calls = %d, second = %+v, %v; want the cached quote with its original At", provider.quoteCalls, second, err)
	}

	*now = now.Add(2 * time.Minute)
	third, err := s.Quote(context.Background(), petr4)
	if err != nil || provider.quoteCalls != 2 || !third.At.Equal(*now) {
		t.Fatalf("calls = %d, third = %+v, %v; want a fresh quote after ten minutes", provider.quoteCalls, third, err)
	}

	// Another instrument is its own entry.
	s.Quote(context.Background(), Instrument{Market: MarketB3, Symbol: "VALE3"})
	if provider.quoteCalls != 3 {
		t.Errorf("calls = %d, want VALE3 fetched on its own", provider.quoteCalls)
	}
}

func TestServiceDoesNotRememberFailures(t *testing.T) {
	provider := &fakeProvider{name: "p", markets: []Market{MarketB3}, quote: failingQuote(fmt.Errorf("p: %w", ErrNotFound))}
	s, _ := newTestService(provider)

	s.Quote(context.Background(), petr4)
	provider.quote = pricedAt("38.29")
	if quote, err := s.Quote(context.Background(), petr4); err != nil || quote.Price.String() != "38.29" {
		t.Fatalf("Quote = %+v, %v; want the provider asked again", quote, err)
	}
}

// A provider breaking the contract is treated as failing, and the next one
// is asked.
func TestServiceRejectsBadProviderOutput(t *testing.T) {
	for name, bad := range map[string]func(context.Context, Instrument) (Quote, error){
		"zero":     pricedAt("0"),
		"negative": pricedAt("-1"),
		"currency": func(context.Context, Instrument) (Quote, error) {
			return Quote{Price: decimal.RequireFromString("10"), Currency: "USD"}, nil
		},
	} {
		broken := &fakeProvider{name: "broken", markets: []Market{MarketB3}, quote: bad}
		good := &fakeProvider{name: "good", markets: []Market{MarketB3}, quote: pricedAt("38.29")}
		s, _ := newTestService(broken, good)
		if quote, err := s.Quote(context.Background(), petr4); err != nil || quote.Provider != "good" {
			t.Errorf("%s: Quote = %+v, %v; want good's", name, quote, err)
		}
	}

	broken := &fakeProvider{name: "broken", markets: []Market{MarketB3},
		history: historyOf(dp(date(2026, 9, 2), "10"), dp(date(2026, 9, 3), "0"))}
	s, _ := newTestService(broken)
	if _, err := s.History(context.Background(), petr4, date(2026, 9, 1), date(2026, 9, 30)); err == nil {
		t.Error("History accepted a zero price")
	}
}

func TestServiceNormalizesHistory(t *testing.T) {
	provider := &fakeProvider{name: "p", markets: []Market{MarketB3},
		history: func(context.Context, Instrument, time.Time, time.Time) (History, error) {
			return History{Currency: "BRL", Prices: []DailyPrice{
				dp(date(2026, 9, 30), "3"),
				dp(date(2026, 8, 31), "1"), // before from
				dp(date(2026, 9, 1), "2"),
				dp(date(2026, 10, 1), "4"), // after to
			}}, nil
		}}
	s, _ := newTestService(provider)

	history, err := s.History(context.Background(), petr4, date(2026, 9, 1).Add(15*time.Hour), date(2026, 9, 30))
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	want := []DailyPrice{dp(date(2026, 9, 1), "2"), dp(date(2026, 9, 30), "3")}
	if len(history.Prices) != len(want) {
		t.Fatalf("prices = %v, want %v", history.Prices, want)
	}
	for i := range want {
		if !history.Prices[i].Day.Equal(want[i].Day) || !history.Prices[i].Price.Equal(want[i].Price) {
			t.Errorf("price %d = %v, want %v", i, history.Prices[i], want[i])
		}
	}
	if !history.CoveredFrom.Equal(date(2026, 9, 1)) {
		t.Errorf("CoveredFrom = %s, want from when the provider leaves it unset", history.CoveredFrom)
	}
}

func TestParseProviders(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  []string
	}{
		{"", []string{"yahoo", "brapi", "coingecko"}},
		{"   ", []string{"yahoo", "brapi", "coingecko"}},
		{"brapi,coingecko", []string{"brapi", "coingecko"}},
		{" CoinGecko , YAHOO ", []string{"coingecko", "yahoo"}},
		{"yahoo", []string{"yahoo"}},
	} {
		got, err := ParseProviders(tc.value)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("ParseProviders(%q) = %v, %v; want %v", tc.value, got, err, tc.want)
		}
	}

	_, err := ParseProviders("yahoo,alphavantage")
	if err == nil || err.Error() != `CONTADINHO_QUOTES_PROVIDERS: fonte desconhecida "alphavantage" (use yahoo, brapi, coingecko)` {
		t.Errorf("unknown provider error = %v", err)
	}
	if _, err := ParseProviders("yahoo,brapi,Yahoo"); err == nil || !strings.Contains(err.Error(), "repetida") {
		t.Errorf("duplicate provider error = %v", err)
	}
}

func TestIsProvider(t *testing.T) {
	for _, name := range []string{"yahoo", "brapi", "coingecko"} {
		if !IsProvider(name) {
			t.Errorf("IsProvider(%q) = false", name)
		}
	}
	for _, name := range []string{"", "manual", "Yahoo", "pluggy"} {
		if IsProvider(name) {
			t.Errorf("IsProvider(%q) = true", name)
		}
	}
}

func TestNewDefaultBuildsTheProvidersInOrder(t *testing.T) {
	names := func(s *Service) []string {
		var out []string
		for _, p := range s.providers {
			out = append(out, p.Name())
		}
		return out
	}

	s, err := NewDefault(Config{})
	if err != nil || !reflect.DeepEqual(names(s), []string{"yahoo", "brapi", "coingecko"}) {
		t.Fatalf("default = %v, %v", names(s), err)
	}
	s, err = NewDefault(Config{Providers: []string{"coingecko", "brapi"}})
	if err != nil || !reflect.DeepEqual(names(s), []string{"coingecko", "brapi"}) {
		t.Fatalf("configured = %v, %v", names(s), err)
	}
	if _, err := NewDefault(Config{Providers: []string{"nope"}}); err == nil {
		t.Error("NewDefault accepted an unknown provider")
	}

	token := func(context.Context) string { return "secret" }
	s, _ = NewDefault(Config{Providers: []string{"brapi"}, BrapiToken: token})
	if _, headers := s.providers[0].(*BrapiProvider).client(context.Background()); headers["Authorization"] != "Bearer secret" {
		t.Errorf("brapi headers = %v, want the configured token", headers)
	}
}
