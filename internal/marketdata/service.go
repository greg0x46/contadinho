package marketdata

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// Provider names, stored as provenance on quotes and histories.
const (
	ProviderYahoo     = "yahoo"
	ProviderBrapi     = "brapi"
	ProviderCoinGecko = "coingecko"
)

// knownProviders is every provider this build has, in the default order.
var knownProviders = []string{ProviderYahoo, ProviderBrapi, ProviderCoinGecko}

// DefaultProviders is the provider order used when none is configured:
// Yahoo first, because it serves every market and the whole history without
// a key; brapi and CoinGecko behind it, for when Yahoo fails or does not
// know an instrument.
const DefaultProviders = "yahoo,brapi,coingecko"

// IsProvider reports whether name is a provider this build has. Callers use
// it to recognize what a market data provider wrote, as opposed to a price
// that came from somewhere else.
func IsProvider(name string) bool {
	for _, known := range knownProviders {
		if name == known {
			return true
		}
	}
	return false
}

// quoteTTL is how long a served quote is reused. A burst of refreshes (a
// page load, a run over many positions of one instrument) costs one request;
// ten minutes is still current for a daily valuation.
const quoteTTL = 10 * time.Minute

// Service answers market data requests from an ordered list of providers.
// For each request it tries, in order, the providers that support the
// instrument's market, and the first good answer wins; a failure is logged
// and the next one is asked. It is safe for concurrent use.
type Service struct {
	providers []Provider
	now       func() time.Time

	mu     sync.Mutex
	quotes map[Instrument]Quote
}

// New builds a Service that tries providers in the order given.
func New(providers ...Provider) *Service {
	return &Service{providers: providers, now: time.Now, quotes: map[Instrument]Quote{}}
}

// chain is the providers that serve market, in configured order.
func (s *Service) chain(instrument Instrument) ([]Provider, error) {
	var chain []Provider
	for _, provider := range s.providers {
		if provider.Supports(instrument.Market) {
			chain = append(chain, provider)
		}
	}
	if len(chain) == 0 {
		return nil, fmt.Errorf("%s: %w", instrument, ErrNoProvider)
	}
	return chain, nil
}

// Quote returns the current price of instrument in BRL, stamped with the
// provider that served it and when. A quote served in the last ten minutes
// is returned again as it was; a failure is never remembered.
func (s *Service) Quote(ctx context.Context, instrument Instrument) (Quote, error) {
	s.mu.Lock()
	cached, ok := s.quotes[instrument]
	s.mu.Unlock()
	if ok && s.now().Sub(cached.At) < quoteTTL {
		return cached, nil
	}

	chain, err := s.chain(instrument)
	if err != nil {
		return Quote{}, err
	}
	var failures []error
	for i, provider := range chain {
		quote, err := provider.Quote(ctx, instrument)
		if err == nil {
			err = checkQuote(quote)
		}
		if err == nil {
			quote.Instrument = instrument
			quote.Provider = provider.Name()
			quote.At = s.now()
			s.mu.Lock()
			s.quotes[instrument] = quote
			s.mu.Unlock()
			return quote, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Quote{}, ctxErr
		}
		failures = append(failures, err)
		logFallback(chain, i, instrument, "quote", err)
	}
	return Quote{}, chainFailure(instrument, failures)
}

// History returns the daily prices of instrument in BRL for the closed range
// [from, to], stamped with the provider that served it. Fallback says the
// first provider for the market did not, which matters to a caller reading
// CoveredFrom: a short coverage may be the fallback's limit, not the
// instrument's.
//
// There is no cache here: the caller persists what it gets along with its
// coverage, and that is the durable cache.
func (s *Service) History(ctx context.Context, instrument Instrument, from, to time.Time) (History, error) {
	from, to = day(from), day(to)
	chain, err := s.chain(instrument)
	if err != nil {
		return History{}, err
	}
	var failures []error
	for i, provider := range chain {
		history, err := provider.History(ctx, instrument, from, to)
		if err == nil {
			history, err = checkHistory(history, from, to)
		}
		if err == nil {
			history.Instrument = instrument
			history.Provider = provider.Name()
			history.Fallback = i > 0
			return history, nil
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return History{}, ctxErr
		}
		failures = append(failures, err)
		logFallback(chain, i, instrument, "history", err)
	}
	return History{}, chainFailure(instrument, failures)
}

// logFallback records that a provider failed and the next one is being
// asked. The last provider's failure is the caller's to report.
func logFallback(chain []Provider, i int, instrument Instrument, op string, err error) {
	if i == len(chain)-1 {
		return
	}
	log.Printf("marketdata_fallback provider=%s instrument=%s op=%s: %v", chain[i].Name(), instrument, op, err)
}

// checkQuote holds a provider to the contract, whatever it says it did.
func checkQuote(quote Quote) error {
	if !quote.Price.IsPositive() {
		return fmt.Errorf("price is not positive: %s", quote.Price)
	}
	if quote.Currency != "BRL" {
		return fmt.Errorf("price in %q, want BRL", quote.Currency)
	}
	return nil
}

// checkHistory holds a provider to the contract: every price positive and in
// BRL, one per day, in order, within [from, to], and a coverage that says
// something.
func checkHistory(history History, from, to time.Time) (History, error) {
	if history.Currency != "BRL" {
		return History{}, fmt.Errorf("history in %q, want BRL", history.Currency)
	}
	byDay := map[time.Time]decimal.Decimal{}
	for _, price := range history.Prices {
		if !price.Price.IsPositive() {
			return History{}, fmt.Errorf("price on %s is not positive: %s", price.Day.Format(time.DateOnly), price.Price)
		}
		d := day(price.Day)
		if d.Before(from) || d.After(to) {
			continue
		}
		byDay[d] = price.Price
	}
	history.Prices = sortedPrices(byDay)
	if history.CoveredFrom.IsZero() || history.CoveredFrom.Before(from) {
		history.CoveredFrom = from
	}
	return history, nil
}

// chainError is every provider's failure for one request. It unwraps to all
// of them, so errors.As still finds a *RateLimitedError a caller defers on.
type chainError struct {
	instrument Instrument
	errs       []error
}

func (e *chainError) Error() string {
	parts := make([]string, len(e.errs))
	for i, err := range e.errs {
		parts[i] = err.Error()
	}
	return e.instrument.String() + ": " + strings.Join(parts, "; ")
}

func (e *chainError) Unwrap() []error { return e.errs }

// chainFailure is the error for a request no provider could answer. Only
// when every provider said it does not know the instrument is the answer
// ErrNotFound: if one of them merely failed, that one might know it, and
// "not found" would be a claim nobody made.
func chainFailure(instrument Instrument, failures []error) error {
	allNotFound := true
	for _, err := range failures {
		if !errors.Is(err, ErrNotFound) {
			allNotFound = false
		}
	}
	if !allNotFound {
		for i, err := range failures {
			if errors.Is(err, ErrNotFound) {
				failures[i] = errors.New(err.Error())
			}
		}
	}
	return &chainError{instrument: instrument, errs: failures}
}

// ParseProviders reads CONTADINHO_QUOTES_PROVIDERS: provider names separated
// by commas, in the order they are tried. Empty means DefaultProviders. A
// provider left out is disabled.
func ParseProviders(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		value = DefaultProviders
	}
	var names []string
	for _, field := range strings.Split(value, ",") {
		if name := strings.ToLower(strings.TrimSpace(field)); name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return ParseProviders(DefaultProviders)
	}
	if err := checkProviderNames(names); err != nil {
		return nil, err
	}
	return names, nil
}

func checkProviderNames(names []string) error {
	seen := map[string]bool{}
	for _, name := range names {
		if !IsProvider(name) {
			return fmt.Errorf("CONTADINHO_QUOTES_PROVIDERS: fonte desconhecida %q (use %s)", name, strings.Join(knownProviders, ", "))
		}
		if seen[name] {
			return fmt.Errorf("CONTADINHO_QUOTES_PROVIDERS: fonte %q repetida", name)
		}
		seen[name] = true
	}
	return nil
}

// Config is what the production Service is built from.
type Config struct {
	// Providers is the order providers are tried in, as ParseProviders
	// returns it. Empty means DefaultProviders.
	Providers []string
	// BrapiToken reads the brapi token for each request; nil or an empty
	// answer means brapi is asked without one.
	BrapiToken func(context.Context) string
}

// NewDefault builds the production Service against the real providers.
func NewDefault(cfg Config) (*Service, error) {
	names := cfg.Providers
	if len(names) == 0 {
		names, _ = ParseProviders(DefaultProviders)
	}
	if err := checkProviderNames(names); err != nil {
		return nil, err
	}
	providers := make([]Provider, 0, len(names))
	for _, name := range names {
		switch name {
		case ProviderYahoo:
			providers = append(providers, NewYahooProvider())
		case ProviderBrapi:
			providers = append(providers, NewBrapiProvider(cfg.BrapiToken))
		case ProviderCoinGecko:
			providers = append(providers, NewCoinGeckoProvider())
		}
	}
	return New(providers...), nil
}

// day is a calendar date at UTC midnight, the way internal/investments
// stores days (investments.Day). It reads the date in t's own location.
func day(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// saoPauloZone is the wall clock Brazilian provider instants are read in,
// mirroring internal/investments' providerZone: a fixed -3h, since Brazil
// has had no daylight saving since 2019.
var saoPauloZone = time.FixedZone("BRT", -3*3600)

// saoPauloDay is the calendar day an instant belongs to in Brazil
// (investments.ProviderDay).
func saoPauloDay(t time.Time) time.Time { return day(t.In(saoPauloZone)) }

// sortedPrices turns one price per day into a list in day order.
func sortedPrices(byDay map[time.Time]decimal.Decimal) []DailyPrice {
	prices := make([]DailyPrice, 0, len(byDay))
	for d, price := range byDay {
		prices = append(prices, DailyPrice{Day: d, Price: price})
	}
	sort.Slice(prices, func(i, j int) bool { return prices[i].Day.Before(prices[j].Day) })
	return prices
}
