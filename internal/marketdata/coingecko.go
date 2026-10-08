package marketdata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// CoinGeckoProvider prices crypto assets through CoinGecko's public,
// unauthenticated API — verified live while building this
// provider: GET api.coingecko.com/api/v3/simple/price?ids=bitcoin&
// vs_currencies=brl returns {"bitcoin":{"brl":<price>}}; an unknown id
// returns 200 with an empty {} object rather than an error status.
//
// CoinGecko does not take tickers: its ticker symbols are ambiguous across
// coins, ids are unique. The instrument's ticker ("BTC") is turned into a
// coin id ("bitcoin") through /search the first time it is priced, and that
// answer is kept for the life of the process (see coinID).
type CoinGeckoProvider struct {
	baseURL string
	http    *httpClient
	now     func() time.Time

	idsMu sync.Mutex
	ids   map[string]string // ticker → coin id
}

// NewCoinGeckoProvider builds a provider against the real CoinGecko API. No
// API key is needed or used.
func NewCoinGeckoProvider() *CoinGeckoProvider {
	return &CoinGeckoProvider{
		baseURL: "https://api.coingecko.com/api/v3",
		http:    newHTTPClient(ProviderCoinGecko, coingeckoLimiter),
		now:     time.Now,
		ids:     map[string]string{},
	}
}

func (p *CoinGeckoProvider) Name() string { return ProviderCoinGecko }

func (p *CoinGeckoProvider) Supports(market Market) bool { return market == MarketCrypto }

// get fetches one endpoint, reporting a 404 (an unknown coin id on the
// /coins routes) as ErrNotFound.
func (p *CoinGeckoProvider) get(ctx context.Context, endpoint string, out any) error {
	if err := p.http.getJSON(ctx, endpoint, nil, out); err != nil {
		if statusCode(err) == 404 {
			return fmt.Errorf("coingecko: %w: %w", ErrNotFound, err)
		}
		return fmt.Errorf("coingecko: %w", err)
	}
	return nil
}

func (p *CoinGeckoProvider) Quote(ctx context.Context, instrument Instrument) (Quote, error) {
	id, err := p.coinID(ctx, instrument)
	if err != nil {
		return Quote{}, err
	}
	var payload map[string]any
	if err := p.get(ctx, p.baseURL+"/simple/price?ids="+url.QueryEscape(id)+"&vs_currencies=brl", &payload); err != nil {
		return Quote{}, err
	}
	entry, ok := payload[id].(map[string]any)
	if !ok {
		return Quote{}, fmt.Errorf("coingecko: no quote returned for %q: %w", id, ErrNotFound)
	}
	raw, ok := entry["brl"]
	if !ok || raw == nil {
		return Quote{}, fmt.Errorf("coingecko: %q has no brl price", id)
	}
	num, ok := raw.(json.Number)
	if !ok {
		return Quote{}, fmt.Errorf("coingecko: %q price is not a number", id)
	}
	price, err := decimal.NewFromString(string(num))
	if err != nil {
		return Quote{}, fmt.Errorf("coingecko: %q price is invalid: %w", id, err)
	}
	if !price.IsPositive() {
		return Quote{}, fmt.Errorf("coingecko: %q price is not positive: %s", id, price.String())
	}
	return Quote{Instrument: instrument, Price: price, Currency: "BRL"}, nil
}

// coinGeckoLookbackDays is how far back the public API serves history: a
// request reaching further is refused with 401 and error_code 10012 ("Public
// API users are limited to querying historical data within the past 365
// days"). Older days cannot be priced without a paid plan.
const coinGeckoLookbackDays = 365

// History returns one price per UTC day between from and to, from
// /coins/{id}/market_chart/range with interval=daily (any range, one point
// per day at 00:00 UTC). A range that starts before the public lookback is
// clamped to it, and CoveredFrom says so.
func (p *CoinGeckoProvider) History(ctx context.Context, instrument Instrument, from, to time.Time) (History, error) {
	today := day(p.now().UTC())
	earliest := today.AddDate(0, 0, -(coinGeckoLookbackDays - 1))
	start, end := day(from), day(to)
	if start.Before(earliest) {
		start = earliest
	}
	if end.Before(start) {
		// Everything asked for is older than the API serves.
		return History{Instrument: instrument, Currency: "BRL", CoveredFrom: start}, nil
	}
	id, err := p.coinID(ctx, instrument)
	if err != nil {
		return History{}, err
	}

	endpoint := fmt.Sprintf("%s/coins/%s/market_chart/range?vs_currency=brl&interval=daily&from=%d&to=%d",
		p.baseURL, url.PathEscape(id), start.Unix(), end.AddDate(0, 0, 1).Unix()-1)
	var payload map[string]any
	if err := p.get(ctx, endpoint, &payload); err != nil {
		return History{}, err
	}
	rows, ok := payload["prices"].([]any)
	if !ok {
		return History{}, fmt.Errorf("coingecko: no history returned for %q", id)
	}
	byDay := map[time.Time]decimal.Decimal{}
	for _, raw := range rows {
		pair, ok := raw.([]any)
		if !ok || len(pair) != 2 {
			continue
		}
		stamp, ok := pair[0].(json.Number)
		if !ok {
			continue
		}
		millis, err := stamp.Int64()
		if err != nil {
			continue
		}
		value, ok := pair[1].(json.Number)
		if !ok {
			continue
		}
		price, err := decimal.NewFromString(string(value))
		if err != nil || !price.IsPositive() {
			continue
		}
		d := day(time.UnixMilli(millis).UTC())
		if d.Before(start) || d.After(end) {
			continue
		}
		byDay[d] = price
	}
	return History{Instrument: instrument, Prices: sortedPrices(byDay), Currency: "BRL", CoveredFrom: start}, nil
}

// coinID finds the coin id for the instrument's ticker through /search,
// whose coins come back ordered by market cap. A ticker is not unique on
// CoinGecko ("BTC" is also a dozen wrapped and bridged tokens), so the match
// is exact on the symbol and the highest ranked coin wins; the symbol may
// also be the id or the name itself ("BITCOIN" for "bitcoin"), tried after
// the ticker.
//
// The answer is kept for the life of the process, and the first answer
// stays: ranks move, and the coin must not change under a position's
// history halfway through a run. No coin found is ErrNotFound, and is not
// remembered, so a coin listed later is found on a later run.
func (p *CoinGeckoProvider) coinID(ctx context.Context, instrument Instrument) (string, error) {
	if instrument.Market != MarketCrypto || strings.TrimSpace(instrument.Symbol) == "" {
		return "", fmt.Errorf("coingecko: cannot price %s", instrument)
	}
	ticker := instrument.Symbol
	p.idsMu.Lock()
	id, ok := p.ids[ticker]
	p.idsMu.Unlock()
	if ok {
		return id, nil
	}

	var payload map[string]any
	if err := p.get(ctx, p.baseURL+"/search?query="+url.QueryEscape(ticker), &payload); err != nil {
		return "", err
	}
	coins, _ := payload["coins"].([]any)
	id, ok = pickCoin(ticker, coins)
	if !ok {
		return "", fmt.Errorf("coingecko: no coin found for %q: %w", ticker, ErrNotFound)
	}

	p.idsMu.Lock()
	defer p.idsMu.Unlock()
	if earlier, ok := p.ids[ticker]; ok {
		return earlier, nil
	}
	p.ids[ticker] = id
	return id, nil
}

// pickCoin chooses among /search results, which arrive ordered by market cap
// (unranked coins last). Exact symbol first, then exact id, then exact name.
func pickCoin(query string, coins []any) (string, bool) {
	wanted := strings.ToLower(strings.TrimSpace(query))
	bestBySymbol, bestByID, bestByName := "", "", ""
	bestRank := -1
	for _, raw := range coins {
		coin, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := coin["id"].(string)
		if id == "" {
			continue
		}
		if symbol, _ := coin["symbol"].(string); strings.ToLower(symbol) == wanted {
			// Ranked coins beat unranked ones; among the ranked the lowest
			// rank wins, and ties keep the API's own order.
			rank := coinRank(coin["market_cap_rank"])
			if bestBySymbol == "" || (rank >= 0 && (bestRank < 0 || rank < bestRank)) {
				bestBySymbol, bestRank = id, rank
			}
		}
		if strings.ToLower(id) == wanted && bestByID == "" {
			bestByID = id
		}
		if name, _ := coin["name"].(string); strings.ToLower(name) == wanted && bestByName == "" {
			bestByName = id
		}
	}
	for _, candidate := range []string{bestBySymbol, bestByID, bestByName} {
		if candidate != "" {
			return candidate, true
		}
	}
	return "", false
}

// coinRank reads market_cap_rank, -1 when the coin is unranked.
func coinRank(raw any) int {
	number, ok := raw.(json.Number)
	if !ok {
		return -1
	}
	rank, err := strconv.Atoi(string(number))
	if err != nil || rank <= 0 {
		return -1
	}
	return rank
}
