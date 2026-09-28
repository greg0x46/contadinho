package quotes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/shopspring/decimal"
)

// CoinGeckoConnector prices crypto assets through CoinGecko's public,
// unauthenticated "simple price" endpoint — verified live while building
// this connector: GET api.coingecko.com/api/v3/simple/price?ids=bitcoin&
// vs_currencies=brl returns {"bitcoin":{"brl":<price>}}; an unknown id
// returns 200 with an empty {} object rather than an error status.
//
// symbol must be a CoinGecko *coin id* (e.g. "bitcoin"), not a ticker —
// CoinGecko's ticker symbols are ambiguous across coins, but ids are unique.
// This is exactly what investment_assets.quote_symbol must store for a
// coingecko-sourced asset.
type CoinGeckoConnector struct {
	baseURL string
	http    *httpClient
}

// NewCoinGeckoConnector builds a connector against the real CoinGecko API.
// No API key is needed or used.
func NewCoinGeckoConnector() *CoinGeckoConnector {
	return &CoinGeckoConnector{baseURL: "https://api.coingecko.com/api/v3", http: newHTTPClient()}
}

func (c *CoinGeckoConnector) FetchPrice(ctx context.Context, symbol string) (decimal.Decimal, error) {
	if symbol == "" {
		return decimal.Decimal{}, fmt.Errorf("coingecko: empty symbol")
	}
	endpoint := c.baseURL + "/simple/price?ids=" + url.QueryEscape(symbol) + "&vs_currencies=brl"
	payload, err := c.http.getJSON(ctx, endpoint, nil)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("coingecko: %w", err)
	}
	entry, ok := payload[symbol].(map[string]any)
	if !ok {
		return decimal.Decimal{}, fmt.Errorf("coingecko: no quote returned for %q", symbol)
	}
	raw, ok := entry["brl"]
	if !ok || raw == nil {
		return decimal.Decimal{}, fmt.Errorf("coingecko: %q has no brl price", symbol)
	}
	num, ok := raw.(json.Number)
	if !ok {
		return decimal.Decimal{}, fmt.Errorf("coingecko: %q price is not a number", symbol)
	}
	price, err := decimal.NewFromString(string(num))
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("coingecko: %q price is invalid: %w", symbol, err)
	}
	if !price.IsPositive() {
		return decimal.Decimal{}, fmt.Errorf("coingecko: %q price is not positive: %s", symbol, price.String())
	}
	return price, nil
}
