package marketdata

import (
	"regexp"
	"strings"
)

// The user gives a market and a symbol; how each provider wants the
// instrument spelled is this package's problem, not theirs. This file is the
// one place those rules live. ParseInstrument canonicalizes what was typed so
// two spellings of one instrument ("petr4", "PETR4.SA", "BVMF:PETR4") are the
// same request and a misspelling is refused when the asset is saved rather
// than failing silently on the first quote run. The per-provider spellings
// below are derived from the canonical symbol offline; what needs the network
// (a CoinGecko coin id from a ticker) is the provider's own business.

// Markets lists every market this build knows, in the order a person would
// pick from.
func Markets() []Market { return []Market{MarketB3, MarketCrypto} }

// ParseMarket reads a stored or submitted market name. Only the exact
// lower-case names are markets; anything else is reported as unknown so the
// caller can refuse it.
func ParseMarket(s string) (Market, bool) {
	switch market := Market(strings.TrimSpace(s)); market {
	case MarketB3, MarketCrypto:
		return market, true
	default:
		return "", false
	}
}

// b3Ticker is a B3 instrument code: four letters or digits, one or two
// digits for the class (PETR4, BOVA11, B3SA3), and an optional "F" for the
// fractional market (PETR4F).
var b3Ticker = regexp.MustCompile(`^[A-Z0-9]{4}[0-9]{1,2}F?$`)

// exchangePrefixes are how other platforms spell the exchange before a
// ticker ("BVMF:PETR4" on Google Finance, "B3:PETR4" on TradingView).
var exchangePrefixes = []string{"BVMF:", "BMFBOVESPA:", "BOVESPA:", "B3:"}

// cryptoTicker is a crypto asset's usual ticker (BTC, ETH, USDC, 1INCH). It
// is deliberately loose: there is no registry to check it against offline,
// and a ticker no provider knows is reported as not found on the first quote.
var cryptoTicker = regexp.MustCompile(`^[A-Z0-9]{2,15}$`)

// cryptoPairSuffixes are the quote currencies people paste along with the
// ticker, because that is how exchanges and Yahoo show it ("BTC-USD",
// "BTC/BRL"). The pair is not part of the instrument: prices always come
// back in BRL.
var cryptoPairSuffixes = []string{"BRL", "USD", "USDT", "EUR"}

// ParseInstrument turns what a person typed as a symbol into the canonical
// Instrument of market. For B3 that is the upper-case ticker with any
// exchange prefix or ".SA" suffix removed; for crypto the upper-case ticker
// with any fiat pair removed. A symbol that cannot be one is a *SymbolError
// whose reason is written for that person.
func ParseInstrument(market Market, raw string) (Instrument, error) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	switch market {
	case MarketB3:
		for _, prefix := range exchangePrefixes {
			value = strings.TrimPrefix(value, prefix)
		}
		value = strings.TrimSuffix(value, ".SA")
		if !b3Ticker.MatchString(value) {
			return Instrument{}, &SymbolError{Market: market, Input: raw,
				Reason: "esperado um código da B3, como PETR4, BOVA11 ou B3SA3"}
		}
	case MarketCrypto:
		for _, suffix := range cryptoPairSuffixes {
			if base, ok := strings.CutSuffix(value, "-"+suffix); ok {
				value = base
				break
			}
			if base, ok := strings.CutSuffix(value, "/"+suffix); ok {
				value = base
				break
			}
		}
		if !cryptoTicker.MatchString(value) {
			return Instrument{}, &SymbolError{Market: market, Input: raw,
				Reason: "esperado o código da criptomoeda, como BTC ou ETH"}
		}
	default:
		return Instrument{}, &SymbolError{Market: market, Input: raw, Reason: "mercado de cotação desconhecido"}
	}
	return Instrument{Market: market, Symbol: value}, nil
}

// yahooSymbol is how Yahoo spells an instrument: B3 codes carry the ".SA"
// exchange suffix, and crypto is the pair against the dollar, because Yahoo
// no longer lists BRL crypto pairs (BTC-BRL answers 404) — the provider
// converts with USD/BRL itself.
func yahooSymbol(instrument Instrument) string {
	switch instrument.Market {
	case MarketB3:
		return instrument.Symbol + ".SA"
	case MarketCrypto:
		return instrument.Symbol + "-USD"
	default:
		return instrument.Symbol
	}
}

// yahooUSDBRL is Yahoo's USD→BRL exchange rate (BRL per dollar).
const yahooUSDBRL = "BRL=X"

// brapiSymbol is how brapi spells a B3 instrument: the canonical ticker as is.
func brapiSymbol(instrument Instrument) string { return instrument.Symbol }
