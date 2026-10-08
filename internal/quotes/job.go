package quotes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/greg0x46/julius/internal/investments"
	"github.com/greg0x46/julius/internal/marketdata"
)

// Summary totals one RefreshAll run, logged as a single line so a stalled
// provider — or a run that quietly did nothing — is visible without a new
// audit table. valued_on on the affected positions (already shown in the
// positions table) is the passive per-asset signal; this is the per-run one.
type Summary struct {
	AssetsConsidered int
	PricesFetched    int
	PriceFailures    int
	// PricesDeferred counts assets left for a later run because their source
	// asked to be left alone (a 429 with Retry-After, an exhausted window).
	// It is not a failure: nothing is wrong with the asset.
	PricesDeferred int
}

// RefreshAll writes today's price of every asset that has a quote source
// configured and at least one manual position holding units into the asset's
// price series. It writes no operation: a position's value is derived from the
// series on read (see investments.ListPositions). It is built entirely on
// internal/investments' public API — ListAssets, ListPositions,
// UpsertConnectorQuotes — and never runs SQL against its tables directly, so
// schema ownership of investment_assets/investment_asset_quotes stays there.
//
// Each asset is identified by its market and canonical symbol. If no symbol
// was stored, the ticker is parsed locally.
//
// A single asset's market data failure is logged and does not block the
// others. Rate limited requests are counted as deferred.
func RefreshAll(ctx context.Context, conn *sql.DB, service *marketdata.Service, today time.Time) (Summary, error) {
	return refreshSpot(ctx, conn, service, today, false)
}

// RefreshMissing is RefreshAll for the assets that have no market price for
// today yet. It is what runs when a position or an asset was just saved: a
// newly quoted asset gets today's price now instead of at the next scheduled
// run, and an asset the daily run already priced costs no request.
func RefreshMissing(ctx context.Context, conn *sql.DB, service *marketdata.Service, today time.Time) (Summary, error) {
	return refreshSpot(ctx, conn, service, today, true)
}

func refreshSpot(ctx context.Context, conn *sql.DB, service *marketdata.Service, today time.Time, onlyMissing bool) (Summary, error) {
	var summary Summary
	today = investments.Day(today)

	assets, err := investments.ListAssets(ctx, conn)
	if err != nil {
		return summary, fmt.Errorf("list assets: %w", err)
	}

	manualSource := string(investments.PositionSourceManual)
	positions, err := investments.ListPositions(ctx, conn, investments.PositionFilter{Source: &manualSource})
	if err != nil {
		return summary, fmt.Errorf("list manual positions: %w", err)
	}
	// A position with no operations at all (just created, nothing bought into
	// it yet) is not Closed even though its quantity is zero, so the quantity
	// is what says whether an asset has any units worth pricing.
	held := map[string]bool{}
	for _, position := range positions {
		if position.AssetID != nil && position.Quantity.IsPositive() {
			held[*position.AssetID] = true
		}
	}

	// A source that asked to be left alone is skipped for the rest of the
	// run; the next run (or the gap scan) picks its assets up again.
	for _, asset := range assets {
		if asset.QuoteSource == nil || *asset.QuoteSource == "" || !held[asset.ID] {
			continue
		}
		if onlyMissing {
			priced, err := pricedToday(ctx, conn, asset.ID, today)
			if err != nil {
				return summary, fmt.Errorf("asset %s quotes: %w", asset.ID, err)
			}
			if priced {
				continue
			}
		}
		summary.AssetsConsidered++

		instrument, err := assetInstrument(asset)
		var quote marketdata.Quote
		if err == nil {
			quote, err = service.Quote(ctx, instrument)
		}
		if err != nil {
			var limited *marketdata.RateLimitedError
			if errors.As(err, &limited) {
				summary.PricesDeferred++
				log.Printf("quote_refresh_deferred asset_id=%s market=%s retry_in=%s", asset.ID, *asset.QuoteSource, limited.RetryAfter.Round(time.Second))
				continue
			}
			summary.PriceFailures++
			log.Printf("quote_fetch_failed asset_id=%s instrument=%s: %v", asset.ID, instrument, err)
			continue
		}
		// The market price is stored as a connector quote, not a plain upsert:
		// it must not overwrite a price the Pluggy sync or an issue PU already
		// holds for the day.
		if err := investments.UpsertConnectorQuotes(ctx, conn, []investments.AssetQuote{{
			AssetID: asset.ID, QuotedOn: today, Price: quote.Price, Source: quote.Provider,
			Origin: investments.QuoteOriginMarket,
		}}); err != nil {
			summary.PriceFailures++
			log.Printf("quote_store_failed asset_id=%s source=%s: %v", asset.ID, quote.Provider, err)
			continue
		}
		summary.PricesFetched++
	}

	log.Printf(
		"quote_refresh_completed assets=%d fetched=%d failures=%d deferred=%d",
		summary.AssetsConsidered, summary.PricesFetched, summary.PriceFailures, summary.PricesDeferred,
	)
	return summary, nil
}

// pricedToday reports whether the asset's series already holds a market price
// for today: one from a market data provider, whichever it was.
func pricedToday(ctx context.Context, conn *sql.DB, assetID string, today time.Time) (bool, error) {
	quotes, err := investments.ListAssetQuotes(ctx, conn, assetID, &today)
	if err != nil {
		return false, err
	}
	if len(quotes) == 0 {
		return false, nil
	}
	last := quotes[len(quotes)-1]
	return last.QuotedOn.Equal(today) && last.Origin == investments.QuoteOriginMarket, nil
}
