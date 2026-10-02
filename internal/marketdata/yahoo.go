package marketdata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// YahooProvider prices B3 instruments and crypto through Yahoo Finance's
// public chart endpoint — the one the yfinance ecosystem uses, which needs no
// key, cookie or crumb, only a browser-like User-Agent:
//
//	GET query1.finance.yahoo.com/v8/finance/chart/{symbol}?range=1d&interval=1d
//	GET .../chart/{symbol}?period1={unix}&period2={unix}&interval=1d
//
// returns {"chart":{"result":[{"meta":{...},"timestamp":[...],"indicators":
// {"quote":[{"close":[...]}],"adjclose":[...]}}],"error":null}}. Verified
// live while building this provider:
//
//   - B3 stocks, units, FIIs, ETFs, BDRs and the fractional market all answer
//     as {TICKER}.SA in BRL, daily bars stamped 13:00Z (10:00 in São Paulo).
//   - An unknown symbol is a 404 whose body says {"code":"Not Found"}.
//   - There are no BRL crypto pairs any more (BTC-BRL is a 404), so crypto
//     is {TICKER}-USD (daily bars at 00:00Z) times USD/BRL ("BRL=X", bars at
//     London midnight, no weekends, and a current-day bar whose close may
//     still be null).
//   - Closes are float32 noise (49.2599983215332): they are rounded to the
//     instrument's meta.priceHint decimals, which is how Yahoo itself
//     displays them (2 for B3 and BTC, 4 for BRL=X).
//   - close is split-adjusted but not dividend-adjusted, the same meaning
//     brapi's close has. adjclose also folds in later dividends, which would
//     price a past day at a number the market never quoted and break
//     quantity × price against the ledger, so it is never read.
//
// The endpoint is unofficial and may change shape without notice; that is
// why it is one provider among several rather than the only one.
type YahooProvider struct {
	baseURL string
	http    *httpClient
	now     func() time.Time

	// The USD/BRL spot rate is shared by every crypto quote, so it is kept
	// for fxTTL: a run over N crypto assets costs N+1 requests, not 2N.
	fxTTL time.Duration
	fxMu  sync.Mutex
	fx    decimal.Decimal
	fxAt  time.Time
}

// yahooUserAgent is a desktop browser's. The chart endpoint answers 429 to
// clients that announce themselves as scripts.
const yahooUserAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0 Safari/537.36"

// NewYahooProvider builds a provider against the real Yahoo Finance API.
func NewYahooProvider() *YahooProvider {
	return &YahooProvider{
		baseURL: "https://query1.finance.yahoo.com/v8/finance/chart",
		http:    newHTTPClient(ProviderYahoo, yahooLimiter),
		now:     time.Now,
		fxTTL:   10 * time.Minute,
	}
}

func (p *YahooProvider) Name() string { return ProviderYahoo }

func (p *YahooProvider) Supports(market Market) bool {
	return market == MarketB3 || market == MarketCrypto
}

// Quote is meta.regularMarketPrice: the last trade while the market is open,
// the close after it shuts. Crypto is its dollar price converted at the
// current USD/BRL rate.
func (p *YahooProvider) Quote(ctx context.Context, instrument Instrument) (Quote, error) {
	switch instrument.Market {
	case MarketB3:
		chart, err := p.chart(ctx, yahooSymbol(instrument), "range=1d&interval=1d", "BRL")
		if err != nil {
			return Quote{}, err
		}
		price, err := chart.spot()
		if err != nil {
			return Quote{}, err
		}
		return Quote{Instrument: instrument, Price: price, Currency: "BRL"}, nil
	case MarketCrypto:
		chart, err := p.chart(ctx, yahooSymbol(instrument), "range=1d&interval=1d", "USD")
		if err != nil {
			return Quote{}, err
		}
		usd, err := chart.spot()
		if err != nil {
			return Quote{}, err
		}
		rate, err := p.usdBRL(ctx)
		if err != nil {
			return Quote{}, err
		}
		return Quote{Instrument: instrument, Price: chart.toBRL(usd, rate), Currency: "BRL"}, nil
	default:
		return Quote{}, fmt.Errorf("yahoo: market %q not supported", instrument.Market)
	}
}

// usdBRL is the current USD/BRL rate, from cache when it is fresh enough.
// The lock is held through the fetch so a burst of crypto quotes asks once.
func (p *YahooProvider) usdBRL(ctx context.Context) (decimal.Decimal, error) {
	p.fxMu.Lock()
	defer p.fxMu.Unlock()
	if !p.fxAt.IsZero() && p.now().Sub(p.fxAt) < p.fxTTL {
		return p.fx, nil
	}
	chart, err := p.fxChart(ctx, "range=1d&interval=1d")
	if err != nil {
		return decimal.Decimal{}, err
	}
	rate, err := chart.spot()
	if err != nil {
		return decimal.Decimal{}, err
	}
	p.fx, p.fxAt = rate, p.now()
	return rate, nil
}

// History returns the daily closes between from and to. Yahoo serves the
// whole life of an instrument, so CoveredFrom is always from.
//
// The request is padded on both sides (a day before, two after) because
// period2 is exclusive and a bar's instant is not its day: a B3 session is
// stamped 13:00Z and an FX one at London midnight, the previous UTC day.
// Each bar is dated in the exchange's own time zone and the padding is cut
// off again.
func (p *YahooProvider) History(ctx context.Context, instrument Instrument, from, to time.Time) (History, error) {
	from, to = day(from), day(to)
	switch instrument.Market {
	case MarketB3:
		chart, err := p.chart(ctx, yahooSymbol(instrument), periodQuery(from.AddDate(0, 0, -1), to.AddDate(0, 0, 2)), "BRL")
		if err != nil {
			return History{}, err
		}
		var prices []DailyPrice
		for _, bar := range chart.dailyCloses() {
			if !bar.Day.Before(from) && !bar.Day.After(to) {
				prices = append(prices, bar)
			}
		}
		return History{Instrument: instrument, Prices: prices, Currency: "BRL", CoveredFrom: from}, nil
	case MarketCrypto:
		// A week before from, so the first days have a rate to carry
		// forward even across a long FX holiday weekend.
		query := periodQuery(from.AddDate(0, 0, -7), to.AddDate(0, 0, 2))
		chart, err := p.chart(ctx, yahooSymbol(instrument), query, "USD")
		if err != nil {
			return History{}, err
		}
		fxChart, err := p.fxChart(ctx, query)
		if err != nil {
			return History{}, err
		}
		rates := fxChart.dailyCloses()
		var prices []DailyPrice
		next := 0 // rates[:next] are at or before the current day
		for _, bar := range chart.dailyCloses() {
			for next < len(rates) && !rates[next].Day.After(bar.Day) {
				next++
			}
			if bar.Day.Before(from) || bar.Day.After(to) {
				continue
			}
			if next == 0 {
				// No rate on or before this day: it cannot be converted.
				continue
			}
			// Crypto trades every day and FX does not: a weekend or holiday
			// is priced at the last rate before it.
			prices = append(prices, DailyPrice{Day: bar.Day, Price: chart.toBRL(bar.Price, rates[next-1].Price)})
		}
		return History{Instrument: instrument, Prices: prices, Currency: "BRL", CoveredFrom: from}, nil
	default:
		return History{}, fmt.Errorf("yahoo: market %q not supported", instrument.Market)
	}
}

func periodQuery(start, end time.Time) string {
	return fmt.Sprintf("period1=%d&period2=%d&interval=1d", start.Unix(), end.Unix())
}

// fxChart fetches USD/BRL. Yahoo not knowing its own exchange rate says
// nothing about the instrument being priced, so a 404 here is reported as a
// plain failure rather than ErrNotFound.
func (p *YahooProvider) fxChart(ctx context.Context, query string) (yahooChart, error) {
	chart, err := p.chart(ctx, yahooUSDBRL, query, "BRL")
	if errors.Is(err, ErrNotFound) {
		return yahooChart{}, fmt.Errorf("yahoo: USD/BRL rate unavailable: %v", err)
	}
	return chart, err
}

// yahooResponse is the part of the chart response this provider reads.
// Prices are json.Number so they never pass through float64; a null close
// decodes as the empty json.Number.
type yahooResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency             string      `json:"currency"`
				RegularMarketPrice   json.Number `json:"regularMarketPrice"`
				ExchangeTimezoneName string      `json:"exchangeTimezoneName"`
				GMTOffset            int         `json:"gmtoffset"`
				PriceHint            *int32      `json:"priceHint"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close []json.Number `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

// yahooChart is one chart response, checked and ready to read.
type yahooChart struct {
	symbol    string
	marketRaw json.Number
	decimals  int32
	location  *time.Location
	stamps    []int64
	closes    []json.Number
}

// chart fetches symbol's chart and checks it is quoted in currency: a
// symbol that suddenly answers in another currency must not be read as BRL.
func (p *YahooProvider) chart(ctx context.Context, symbol, query, currency string) (yahooChart, error) {
	endpoint := p.baseURL + "/" + url.PathEscape(symbol) + "?" + query
	var payload yahooResponse
	err := p.http.getJSON(ctx, endpoint, map[string]string{"User-Agent": yahooUserAgent}, &payload)
	if err != nil {
		if statusCode(err) == 404 {
			return yahooChart{}, fmt.Errorf("yahoo: %s: %w", symbol, ErrNotFound)
		}
		return yahooChart{}, fmt.Errorf("yahoo: %s: %w", symbol, err)
	}
	if e := payload.Chart.Error; e != nil {
		if e.Code == "Not Found" {
			return yahooChart{}, fmt.Errorf("yahoo: %s: %w", symbol, ErrNotFound)
		}
		return yahooChart{}, fmt.Errorf("yahoo: %s: %s: %s", symbol, e.Code, e.Description)
	}
	if len(payload.Chart.Result) == 0 {
		return yahooChart{}, fmt.Errorf("yahoo: %s: %w", symbol, ErrNotFound)
	}
	result := payload.Chart.Result[0]
	meta := result.Meta
	if meta.Currency != currency {
		return yahooChart{}, fmt.Errorf("yahoo: %s is quoted in %q, want %s", symbol, meta.Currency, currency)
	}
	chart := yahooChart{symbol: symbol, marketRaw: meta.RegularMarketPrice, decimals: 2, stamps: result.Timestamp}
	if meta.PriceHint != nil && *meta.PriceHint >= 0 && *meta.PriceHint <= 12 {
		chart.decimals = *meta.PriceHint
	}
	if len(result.Indicators.Quote) > 0 {
		chart.closes = result.Indicators.Quote[0].Close
	}
	if len(chart.closes) != len(chart.stamps) {
		return yahooChart{}, fmt.Errorf("yahoo: %s: %d timestamps for %d closes", symbol, len(chart.stamps), len(chart.closes))
	}
	// The exchange's zone dates the bars. Its current UTC offset is only a
	// fallback for a system without that zone in its tz database: it is
	// right for every bar outside daylight-saving changes.
	chart.location, err = time.LoadLocation(meta.ExchangeTimezoneName)
	if err != nil || meta.ExchangeTimezoneName == "" {
		chart.location = time.FixedZone("yahoo"+strconv.Itoa(meta.GMTOffset), meta.GMTOffset)
	}
	return chart, nil
}

// price reads one of the chart's numbers, rounded to its display precision.
// ok is false for a null or unparseable value; a zero or negative one is an
// error.
func (c yahooChart) price(raw json.Number) (decimal.Decimal, bool, error) {
	if raw == "" {
		return decimal.Decimal{}, false, nil
	}
	value, err := decimal.NewFromString(string(raw))
	if err != nil {
		return decimal.Decimal{}, false, nil
	}
	value = value.Round(c.decimals)
	if !value.IsPositive() {
		return decimal.Decimal{}, false, fmt.Errorf("yahoo: %s price is not positive: %s", c.symbol, value)
	}
	return value, true, nil
}

// spot is meta.regularMarketPrice.
func (c yahooChart) spot() (decimal.Decimal, error) {
	price, ok, err := c.price(c.marketRaw)
	if err != nil {
		return decimal.Decimal{}, err
	}
	if !ok {
		return decimal.Decimal{}, fmt.Errorf("yahoo: %s has no regularMarketPrice", c.symbol)
	}
	return price, nil
}

// dailyCloses dates every bar in the exchange's zone and keeps its close,
// sorted by day. Bars without a usable close (null, zero) are skipped — the
// day stays unpriced rather than wrong — and when two bars fall on one day
// the later wins.
func (c yahooChart) dailyCloses() []DailyPrice {
	byDay := map[time.Time]decimal.Decimal{}
	for i, stamp := range c.stamps {
		price, ok, err := c.price(c.closes[i])
		if !ok || err != nil {
			continue
		}
		byDay[day(time.Unix(stamp, 0).In(c.location))] = price
	}
	return sortedPrices(byDay)
}

// toBRL converts a dollar price of this chart's instrument at rate, keeping
// at least cents and as many decimals as the instrument is quoted with (a
// coin worth fractions of a cent keeps its precision).
func (c yahooChart) toBRL(usd, rate decimal.Decimal) decimal.Decimal {
	return usd.Mul(rate).Round(max(2, c.decimals))
}
