package quotes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"contadinho-go/internal/investments"
	"contadinho-go/internal/marketdata"
)

// HistorySummary totals one RefreshHistory run, logged as a single line like
// Summary is.
type HistorySummary struct {
	AssetsConsidered int
	// HistoriesFetched is the number of assets whose history was requested;
	// an asset whose covered range already reaches both
	// ends costs no request at all.
	HistoriesFetched int
	PricesStored     int
	Failures         int
	// Deferred counts assets left for a later run because providers asked
	// to be left alone. Nothing is lost: what is missing is still missing, so
	// the next run asks again.
	Deferred int
}

// assetInstrument parses the asset's market and stored symbol (or ticker)
// without a provider request or database write.
func assetInstrument(asset investments.Asset) (marketdata.Instrument, error) {
	if asset.QuoteSource == nil || *asset.QuoteSource == "" {
		return marketdata.Instrument{}, fmt.Errorf("asset has no quote market")
	}
	market, ok := marketdata.ParseMarket(*asset.QuoteSource)
	if !ok {
		return marketdata.Instrument{}, fmt.Errorf("unknown quote market %q", *asset.QuoteSource)
	}
	if asset.QuoteSymbol != nil && strings.TrimSpace(*asset.QuoteSymbol) != "" {
		return marketdata.ParseInstrument(market, *asset.QuoteSymbol)
	}
	if asset.Ticker == nil || strings.TrimSpace(*asset.Ticker) == "" {
		return marketdata.Instrument{}, fmt.Errorf("asset has neither a quote symbol nor a ticker")
	}
	return marketdata.ParseInstrument(market, *asset.Ticker)
}

// missingRange is the one range of days to ask for, given the
// first day the asset is held, the last day that is already over, and what
// has been asked before. It is the whole backfill strategy in one function:
// a range that is already covered costs nothing, a position registered with
// an earlier date than before extends the covered range backwards, a run that
// was missed extends it forwards, and both at once are one request.
func missingRange(heldFrom, yesterday time.Time, covered *investments.QuoteCoverage) (from, to time.Time, needed bool) {
	if heldFrom.After(yesterday) {
		return time.Time{}, time.Time{}, false
	}
	if covered == nil {
		return heldFrom, yesterday, true
	}
	needBackward := heldFrom.Before(covered.From)
	needForward := covered.To.Before(yesterday)
	switch {
	case needBackward && needForward:
		return heldFrom, yesterday, true
	case needBackward:
		return heldFrom, covered.From.AddDate(0, 0, -1), true
	case needForward:
		return covered.To.AddDate(0, 0, 1), yesterday, true
	default:
		return time.Time{}, time.Time{}, false
	}
}

// RefreshHistory fills in the price series of every asset that has a quote
// source, from the day a position first held it up to yesterday (today's
// price is RefreshAll's). It is driven by what is missing, not by what
// changed: it looks at each asset's earliest holding and at the range already
// asked for its instrument, so the same call repairs a position registered with
// a past date, a purchase date corrected to an earlier one, a source or symbol
// that was switched, and the days a stopped process missed — and is a no-op,
// with no request, when there is nothing to repair.
//
// One request per asset covers its whole missing range through the service.
func RefreshHistory(ctx context.Context, conn *sql.DB, service *marketdata.Service, today time.Time) (HistorySummary, error) {
	var summary HistorySummary
	today = investments.Day(today)
	yesterday := today.AddDate(0, 0, -1)

	assets, err := investments.ListAssets(ctx, conn)
	if err != nil {
		return summary, fmt.Errorf("list assets: %w", err)
	}

	for _, asset := range assets {
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		if asset.QuoteSource == nil || *asset.QuoteSource == "" {
			continue
		}
		market := *asset.QuoteSource

		heldFrom, err := investments.AssetHeldFrom(ctx, conn, asset.ID)
		if err != nil {
			return summary, fmt.Errorf("asset %s held from: %w", asset.ID, err)
		}
		if heldFrom == nil || heldFrom.After(yesterday) {
			continue
		}
		summary.AssetsConsidered++

		instrument, err := assetInstrument(asset)
		if err != nil {
			summary.Failures++
			log.Printf("quote_history_symbol_failed asset_id=%s market=%s: %v", asset.ID, market, err)
			continue
		}

		covered, err := investments.GetQuoteCoverage(ctx, conn, asset.ID)
		if err != nil {
			return summary, fmt.Errorf("asset %s coverage: %w", asset.ID, err)
		}
		if covered != nil && (covered.Source != market || covered.Symbol != instrument.Symbol) {
			covered = nil // asked of something else: the answer no longer applies
		}
		from, to, needed := missingRange(*heldFrom, yesterday, covered)
		if !needed {
			continue
		}

		history, err := service.History(ctx, instrument, from, to)
		if err != nil {
			if deferred(&summary, market, asset.ID, err) {
				continue
			}
			summary.Failures++
			log.Printf("quote_history_failed asset_id=%s instrument=%s from=%s to=%s: %v",
				asset.ID, instrument, from.Format(investments.DateLayout), to.Format(investments.DateLayout), err)
			continue
		}
		summary.HistoriesFetched++
		if history.CoveredFrom.After(to) {
			// The fallback's entire available history starts after this
			// range. It cannot establish coverage for any requested day.
			log.Printf("quote_history_limited asset_id=%s market=%s provider=%s symbol=%s asked_from=%s served_from=%s fallback=%t",
				asset.ID, market, history.Provider, instrument.Symbol, from.Format(investments.DateLayout), history.CoveredFrom.Format(investments.DateLayout), history.Fallback)
			continue
		}

		quotes := make([]investments.AssetQuote, 0, len(history.Prices))
		for _, price := range history.Prices {
			if price.Day.Before(from) || price.Day.After(to) {
				continue
			}
			quotes = append(quotes, investments.AssetQuote{
				AssetID: asset.ID, QuotedOn: price.Day, Price: price.Price, Source: history.Provider,
				Origin: investments.QuoteOriginMarket,
			})
		}
		if err := investments.UpsertConnectorQuotes(ctx, conn, quotes); err != nil {
			summary.Failures++
			log.Printf("quote_history_store_failed asset_id=%s provider=%s: %v", asset.ID, history.Provider, err)
			continue
		}
		summary.PricesStored += len(quotes)

		// What was asked is recorded, not what was obtained: a source that
		// will not serve the older days would otherwise be asked for them on
		// every run forever.
		coverageFrom := from
		if history.Fallback && history.CoveredFrom.After(from) {
			coverageFrom = history.CoveredFrom
		}
		newCoverage := mergeCoverage(asset.ID, market, instrument.Symbol, coverageFrom, to, covered)
		if covered != nil && coverageFrom.After(covered.To.AddDate(0, 0, 1)) {
			// A single interval cannot include the unserved gap before this
			// fallback's prices. Keep the earlier interval and retry the gap.
			newCoverage = *covered
		}
		if err := investments.SaveQuoteCoverage(ctx, conn, newCoverage); err != nil {
			summary.Failures++
			log.Printf("quote_coverage_store_failed asset_id=%s market=%s: %v", asset.ID, market, err)
			continue
		}
		if history.CoveredFrom.After(from) {
			log.Printf("quote_history_limited asset_id=%s market=%s provider=%s symbol=%s asked_from=%s served_from=%s fallback=%t",
				asset.ID, market, history.Provider, instrument.Symbol, from.Format(investments.DateLayout), history.CoveredFrom.Format(investments.DateLayout), history.Fallback)
		}
		log.Printf("quote_history_backfilled asset_id=%s market=%s provider=%s symbol=%s from=%s to=%s prices=%d",
			asset.ID, market, history.Provider, instrument.Symbol, from.Format(investments.DateLayout), to.Format(investments.DateLayout), len(quotes))
	}

	log.Printf("quote_history_completed assets=%d fetched=%d prices=%d failures=%d deferred=%d",
		summary.AssetsConsidered, summary.HistoriesFetched, summary.PricesStored, summary.Failures, summary.Deferred)
	return summary, nil
}

// deferred counts a rate limited request; the service handles providers.
func deferred(summary *HistorySummary, market, assetID string, err error) bool {
	var limited *marketdata.RateLimitedError
	if !errors.As(err, &limited) {
		return false
	}
	summary.Deferred++
	log.Printf("quote_history_deferred asset_id=%s market=%s retry_in=%s", assetID, market, limited.RetryAfter.Round(time.Second))
	return true
}

// mergeCoverage is the covered range after [from, to] was asked. The ranges
// asked always touch the old one (backfill extends it at an end), so the
// result is their union; a stale or absent range is simply replaced.
func mergeCoverage(assetID, source, symbol string, from, to time.Time, previous *investments.QuoteCoverage) investments.QuoteCoverage {
	merged := investments.QuoteCoverage{AssetID: assetID, Source: source, Symbol: symbol, From: from, To: to}
	if previous == nil {
		return merged
	}
	if previous.From.Before(merged.From) {
		merged.From = previous.From
	}
	if previous.To.After(merged.To) {
		merged.To = previous.To
	}
	return merged
}
