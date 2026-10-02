// Package marketdata is the one door to external market data. Callers ask
// for an Instrument — a market and the symbol that market knows it by — and
// get back a normalized Quote or History in BRL, stamped with the provider
// that served it. Which providers exist, in what order they are tried, how
// each one spells the instrument and how its errors look are this package's
// concern and nobody else's.
//
// The rest of the app never names a provider to ask for data. A provider
// name only shows up as provenance: on a Quote, a History, and the rows
// internal/quotes stores from them.
//
// Adding a provider is a new type implementing Provider plus a case in
// NewDefault; adding a market is a Market constant, its canonical symbol
// rule in ParseInstrument, and Supports on the providers that serve it.
// Other kinds of data (dividends, corporate events, FX, ...) are new methods
// on the providers that have them, exposed through Service the same way
// Quote and History are.
package marketdata

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Market is where an instrument trades, and so what its symbol means. It is
// what investment_assets.quote_source stores.
type Market string

const (
	// MarketB3 is the Brazilian exchange: stocks, units, FIIs, ETFs and BDRs,
	// spelled by their B3 code (PETR4, BOVA11, HGLG11, AAPL34).
	MarketB3 Market = "b3"
	// MarketCrypto is crypto assets, spelled by their usual ticker (BTC,
	// ETH) and priced in BRL.
	MarketCrypto Market = "crypto"
)

// Instrument is a tradable thing independent of any provider. Symbol is the
// canonical spelling ParseInstrument produces ("PETR4", "BTC"), never a
// provider's own ("PETR4.SA", "bitcoin", "BTC-USD").
type Instrument struct {
	Market Market
	Symbol string
}

func (i Instrument) String() string { return string(i.Market) + ":" + i.Symbol }

// Quote is the current unit price of an instrument.
type Quote struct {
	Instrument Instrument
	Price      decimal.Decimal
	Currency   string
	// Provider is the name of the provider that served the price.
	Provider string
	// At is when the price was obtained.
	At time.Time
}

// DailyPrice is one day's closing unit price. Day is a calendar date at UTC
// midnight (investments.Day), in the market's own calendar.
type DailyPrice struct {
	Day   time.Time
	Price decimal.Decimal
}

// History is what was obtained for a closed range of days. CoveredFrom is
// the earliest day the answer speaks for: the start of the range asked for,
// or later when the provider would not serve that far back. A day inside
// the covered range with no entry in Prices is a day the market did not
// trade.
type History struct {
	Instrument  Instrument
	Prices      []DailyPrice
	Currency    string
	CoveredFrom time.Time
	// Provider is the name of the provider that served the history.
	Provider string
	// Fallback is true when the history came from a provider other than the
	// first one configured for the market — the preferred provider failed,
	// so a short CoveredFrom may only be this provider's limit.
	Fallback bool
}

// Provider is one external source of market data. Implementations translate
// an Instrument into their own symbol, fetch, and normalize the answer: prices
// in BRL as decimal.Decimal (never rounded through float64), zero or negative
// prices rejected, days in the market's calendar. They classify failures with
// the errors below so Service can decide what to do next.
type Provider interface {
	// Name is the stable identifier stored as provenance ("yahoo").
	Name() string
	// Supports reports whether the provider can serve instruments of market
	// at all; it says nothing about one particular symbol.
	Supports(market Market) bool
	// Quote returns the current price. Provider and At are filled in by
	// Service.
	Quote(ctx context.Context, instrument Instrument) (Quote, error)
	// History returns the daily prices of the closed range [from, to]. It is
	// never asked for the current day: today's price comes from Quote.
	// Provider and Fallback are filled in by Service.
	History(ctx context.Context, instrument Instrument, from, to time.Time) (History, error)
}

// ErrNotFound means the provider does not know the instrument. Another
// provider may still know it.
var ErrNotFound = errors.New("instrument not found")

// ErrUnavailable means the provider could not answer right now: a timeout, a
// network failure, a 5xx. Asking again later may work.
var ErrUnavailable = errors.New("provider unavailable")

// ErrNoProvider means no enabled provider supports the instrument's market.
var ErrNoProvider = errors.New("no provider enabled for market")

// RateLimitedError says a provider asked to be left alone, and for how long.
// It is not a failure of the instrument: the lookup is simply postponed.
type RateLimitedError struct {
	Provider   string
	RetryAfter time.Duration
}

func (e *RateLimitedError) Error() string {
	return fmt.Sprintf("%s: rate limited, retry in %s", e.Provider, e.RetryAfter.Round(time.Second))
}

// SymbolError is a symbol the market cannot be asked for. The message is
// written for the person who typed it, in Portuguese, because it comes back
// to them as the reason a save was refused.
type SymbolError struct {
	Market Market
	Input  string
	Reason string
}

func (e *SymbolError) Error() string {
	return fmt.Sprintf("símbolo %q inválido para %s: %s", e.Input, e.Market, e.Reason)
}
