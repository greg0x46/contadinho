// Package quotes prices manual investment positions Pluggy never reports —
// Bitcoin being the starting case, since no Open Banking connection ever
// syncs it. Synced positions (financial_investments, ValuationBasis =
// ProviderBalance) already carry a provider quote and are untouched by this
// package.
//
// It depends only on internal/investments' public API (ListAssets,
// ListPositions, ListOperations, CreateOperation/UpdateOperation) plus
// internal/settings for the optional brapi token — it never runs SQL
// against investments' own tables, so that package keeps sole ownership of
// its schema.
package quotes

import (
	"context"

	"github.com/shopspring/decimal"
)

// Connector fetches the current unit price of one instrument, in BRL, from
// a single external quote provider.
type Connector interface {
	// FetchPrice returns the current unit price of symbol in BRL. It must
	// reject a zero, negative or unparseable price as an error rather than
	// returning it, and must never round the price through float64 — decode
	// the provider's JSON with json.Decoder.UseNumber() and parse with
	// decimal.NewFromString, mirroring internal/pluggy/mapping.go.
	FetchPrice(ctx context.Context, symbol string) (decimal.Decimal, error)
}

// Registry maps investment_assets.quote_source to the connector that serves
// it. The key space is exactly the set IsKnownConnector recognizes.
type Registry map[string]Connector

// Source keys stored in investment_assets.quote_source. CoinGecko's key
// doubles as the symbol convention documented on CoinGeckoConnector: the
// asset's quote_symbol must be a CoinGecko coin id (e.g. "bitcoin"), not a
// ticker.
const (
	SourceCoinGecko = "coingecko"
	SourceBrapi     = "brapi"
)

// knownConnectors is the source of truth for every quote_source key this
// build recognizes, independent of whether a live Registry has actually been
// constructed. internal/httpapi validates a request's quote_source against
// this list alone, with no network config (an http.Client, a brapi token)
// in reach.
var knownConnectors = map[string]bool{
	SourceCoinGecko: true,
	SourceBrapi:     true,
}

// IsKnownConnector reports whether key names a connector this build
// supports. It says nothing about whether that connector can currently
// serve a request (e.g. brapi's token might still be unset) — only that the
// key itself is one internal/quotes knows how to route.
func IsKnownConnector(key string) bool {
	return knownConnectors[key]
}

// NewDefaultRegistry builds the production registry. CoinGecko needs no
// credential; brapiToken may be empty, which BrapiConnector treats as
// "send the request unauthenticated" rather than as an error.
func NewDefaultRegistry(brapiToken string) Registry {
	return Registry{
		SourceCoinGecko: NewCoinGeckoConnector(),
		SourceBrapi:     NewBrapiConnector(brapiToken),
	}
}
