package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/shopspring/decimal"
)

// BrapiProvider prices B3 instruments through brapi.dev's quote endpoint.
//
// Endpoint verified live while building this provider (brapi.dev/docs
// plus a real request against the production API): GET /api/quote/{tickers}
// returns {"results":[{"symbol":"PETR4","regularMarketPrice":<price>,...}]},
// and with ?range=..&interval=1d also results[0].historicalDataPrice.
//
// The token is optional: brapi's free unauthenticated tier only serves a
// small fixed allow-list of tickers (PETR4/MGLU3/VALE3/ITUB4 as observed
// live); any other ticker without a token answers 401 with code
// MISSING_TOKEN. This provider does not hardcode that allow-list — an
// unauthenticated request outside it simply surfaces the 401 as an error
// like any other rejection.
type BrapiProvider struct {
	baseURL string
	token   func(context.Context) string
	// Without a token brapi allows 20 requests a minute per IP; with one
	// there is no per-minute cap, only the plan's monthly quota and its
	// concurrency, so the two cases draw from different limiters (see
	// limiter.go) and the client is picked per request.
	anonymous     *httpClient
	authenticated *httpClient
	now           func() time.Time
}

// NewBrapiProvider builds a provider against the real brapi.dev API. token is
// read on every request, because it lives in the app's settings and may be
// set or changed while the process runs; nil, or an empty answer, means the
// request goes out unauthenticated.
func NewBrapiProvider(token func(context.Context) string) *BrapiProvider {
	return &BrapiProvider{
		baseURL:       "https://brapi.dev/api",
		token:         token,
		anonymous:     newHTTPClient(ProviderBrapi, brapiAnonymousLimiter),
		authenticated: newHTTPClient(ProviderBrapi, brapiTokenLimiter),
		now:           time.Now,
	}
}

func (p *BrapiProvider) Name() string { return ProviderBrapi }

func (p *BrapiProvider) Supports(market Market) bool { return market == MarketB3 }

// client is the HTTP client for one request and the headers it carries.
func (p *BrapiProvider) client(ctx context.Context) (*httpClient, map[string]string) {
	token := ""
	if p.token != nil {
		token = p.token(ctx)
	}
	if token == "" {
		return p.anonymous, nil
	}
	return p.authenticated, map[string]string{"Authorization": "Bearer " + token}
}

// get fetches one brapi endpoint and returns results[0]. A 404, or a ticker
// left out of results (how brapi answers for one it does not know), is
// ErrNotFound.
func (p *BrapiProvider) get(ctx context.Context, client *httpClient, headers map[string]string, symbol, query string) (map[string]any, error) {
	endpoint := p.baseURL + "/quote/" + url.PathEscape(symbol) + query
	var payload map[string]any
	if err := client.getJSON(ctx, endpoint, headers, &payload); err != nil {
		if statusCode(err) == 404 {
			return nil, fmt.Errorf("brapi: %s: %w", symbol, ErrNotFound)
		}
		return nil, fmt.Errorf("brapi: %w", err)
	}
	results, ok := payload["results"].([]any)
	if !ok || len(results) == 0 {
		return nil, fmt.Errorf("brapi: no quote returned for %q: %w", symbol, ErrNotFound)
	}
	entry, ok := results[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("brapi: unexpected result shape for %q", symbol)
	}
	return entry, nil
}

func (p *BrapiProvider) Quote(ctx context.Context, instrument Instrument) (Quote, error) {
	symbol := brapiSymbol(instrument)
	if instrument.Market != MarketB3 || symbol == "" {
		return Quote{}, fmt.Errorf("brapi: cannot price %s", instrument)
	}
	client, headers := p.client(ctx)
	entry, err := p.get(ctx, client, headers, symbol, "")
	if err != nil {
		return Quote{}, err
	}
	raw, ok := entry["regularMarketPrice"]
	if !ok || raw == nil {
		return Quote{}, fmt.Errorf("brapi: %q has no regularMarketPrice", symbol)
	}
	num, ok := raw.(json.Number)
	if !ok {
		return Quote{}, fmt.Errorf("brapi: %q price is not a number", symbol)
	}
	price, err := decimal.NewFromString(string(num))
	if err != nil {
		return Quote{}, fmt.Errorf("brapi: %q price is invalid: %w", symbol, err)
	}
	if !price.IsPositive() {
		return Quote{}, fmt.Errorf("brapi: %q price is not positive: %s", symbol, price.String())
	}
	return Quote{Instrument: instrument, Price: price, Currency: "BRL"}, nil
}

// brapiRanges is the ladder of values brapi's relative `range` parameter
// accepts, with how many calendar days each reaches back. brapi has no
// absolute start date: a history is always "the last N", so the smallest
// range that reaches the first day needed is the cheapest honest request.
var brapiRanges = []struct {
	name string
	days int
}{
	{"5d", 5}, {"1mo", 31}, {"3mo", 92}, {"6mo", 183}, {"1y", 366}, {"2y", 731}, {"5y", 1827},
	{"max", 0}, // no bound
}

// History returns the daily closes between from and to.
//
// The range asked for is the smallest that reaches from. Plans differ in how
// far back they serve (the Free plan: 3 months, Startup: 1 year, Pro: more
// than 10), and the API does not say which plan a token belongs to, so when a
// range is refused with a client error the next shorter one is tried and
// CoveredFrom reports how far back the answer actually goes — the caller
// records what was asked and never retries a depth the plan will not serve.
func (p *BrapiProvider) History(ctx context.Context, instrument Instrument, from, to time.Time) (History, error) {
	symbol := brapiSymbol(instrument)
	if instrument.Market != MarketB3 || symbol == "" {
		return History{}, fmt.Errorf("brapi: cannot price %s", instrument)
	}
	today := saoPauloDay(p.now())
	from, to = day(from), day(to)
	// Two days of margin: "5d" is five calendar days, which on a weekend
	// holds as few as three trading sessions.
	needed := int(today.Sub(from).Hours()/24) + 2
	index := len(brapiRanges) - 1
	for i, candidate := range brapiRanges {
		if candidate.days != 0 && candidate.days >= needed {
			index = i
			break
		}
	}

	client, headers := p.client(ctx)
	var lastErr error
	for ; index >= 0; index-- {
		chosen := brapiRanges[index]
		entry, err := p.get(ctx, client, headers, symbol, "?range="+chosen.name+"&interval=1d")
		if err != nil {
			// A plan refusing the depth answers with some other 4xx; a
			// missing token or an unknown ticker is not fixed by asking for
			// less.
			if status := statusCode(err); status >= 400 && status < 500 && status != 401 && status != 404 && index > 0 {
				lastErr = err
				continue
			}
			return History{}, err
		}
		prices, err := parseBrapiHistory(entry, symbol, from, to)
		if err != nil {
			return History{}, err
		}
		covered := from
		if chosen.days != 0 {
			if reach := today.AddDate(0, 0, -chosen.days); reach.After(covered) {
				covered = reach
			}
		}
		return History{Instrument: instrument, Prices: prices, Currency: "BRL", CoveredFrom: covered}, nil
	}
	return History{}, lastErr
}

func parseBrapiHistory(entry map[string]any, symbol string, from, to time.Time) ([]DailyPrice, error) {
	rows, ok := entry["historicalDataPrice"].([]any)
	if !ok || len(rows) == 0 {
		return nil, fmt.Errorf("brapi: no history returned for %q", symbol)
	}
	byDay := map[time.Time]decimal.Decimal{}
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		stamp, ok := row["date"].(json.Number)
		if !ok {
			continue
		}
		seconds, err := stamp.Int64()
		if err != nil {
			continue
		}
		// The close is the day's price; adjustedClose also folds in later
		// dividends and splits, which would price a past day at a number the
		// market never quoted and break quantity × price against the ledger.
		closeValue, ok := row["close"].(json.Number)
		if !ok {
			continue
		}
		price, err := decimal.NewFromString(string(closeValue))
		if err != nil || !price.IsPositive() {
			continue
		}
		// brapi stamps each session at São Paulo midnight.
		d := saoPauloDay(time.Unix(seconds, 0))
		if d.Before(from) || d.After(to) {
			continue
		}
		byDay[d] = price
	}
	return sortedPrices(byDay), nil
}
