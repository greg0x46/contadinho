package quotes

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/shopspring/decimal"
)

// BrapiConnector prices B3 tickers held manually (outside any synced Open
// Banking connection) through brapi.dev's quote endpoint.
//
// Endpoint verified live while building this connector (brapi.dev/docs plus
// a real request against the production API): GET /api/quote/{tickers}
// returns {"results":[{"symbol":"PETR4","regularMarketPrice":<price>,...}]}
// — this package was originally briefed against a /api/v2/stocks/quote path
// with the price nested under results[0].data.regularMarketPrice, which
// does not exist on the real service; see the final report for the request
// that confirmed the correct shape.
//
// token is optional: brapi's free unauthenticated tier only serves a small
// fixed allow-list of tickers (PETR4/MGLU3/VALE3/ITUB4 as observed live);
// any other ticker without a token answers 401 with code MISSING_TOKEN. This
// connector does not hardcode that allow-list — an unauthenticated request
// outside it simply surfaces the 401 as an error like any other rejection.
type BrapiConnector struct {
	baseURL string
	token   string
	http    *httpClient
}

// NewBrapiConnector builds a connector against the real brapi.dev API.
// token may be empty; FetchPrice then omits the Authorization header.
func NewBrapiConnector(token string) *BrapiConnector {
	return &BrapiConnector{baseURL: "https://brapi.dev/api", token: token, http: newHTTPClient()}
}

func (c *BrapiConnector) FetchPrice(ctx context.Context, symbol string) (decimal.Decimal, error) {
	if symbol == "" {
		return decimal.Decimal{}, fmt.Errorf("brapi: empty symbol")
	}
	endpoint := c.baseURL + "/quote/" + url.PathEscape(symbol)
	var headers map[string]string
	if c.token != "" {
		headers = map[string]string{"Authorization": "Bearer " + c.token}
	}
	payload, err := c.http.getJSON(ctx, endpoint, headers)
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("brapi: %w", err)
	}
	results, ok := payload["results"].([]any)
	if !ok || len(results) == 0 {
		return decimal.Decimal{}, fmt.Errorf("brapi: no quote returned for %q", symbol)
	}
	entry, ok := results[0].(map[string]any)
	if !ok {
		return decimal.Decimal{}, fmt.Errorf("brapi: unexpected result shape for %q", symbol)
	}
	raw, ok := entry["regularMarketPrice"]
	if !ok || raw == nil {
		return decimal.Decimal{}, fmt.Errorf("brapi: %q has no regularMarketPrice", symbol)
	}
	num, ok := raw.(json.Number)
	if !ok {
		return decimal.Decimal{}, fmt.Errorf("brapi: %q price is not a number", symbol)
	}
	price, err := decimal.NewFromString(string(num))
	if err != nil {
		return decimal.Decimal{}, fmt.Errorf("brapi: %q price is invalid: %w", symbol, err)
	}
	if !price.IsPositive() {
		return decimal.Decimal{}, fmt.Errorf("brapi: %q price is not positive: %s", symbol, price.String())
	}
	return price, nil
}
