package quotes

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/investments"
	"contadinho-go/internal/marketdata"
)

// AutoQuoteMarker prefixes Notes on every valuation operation this package
// writes. A second run the same day (or a process restart) recognizes its
// own row by this prefix and updates it in place instead of duplicating it;
// a Notes value that does NOT start with it means a human entered or
// corrected a valuation today, which this job must never overwrite.
const AutoQuoteMarker = investments.AutoQuoteMarker

// Summary totals one RefreshAll run, logged as a single line so a stalled
// connector — or a run that quietly did nothing — is visible without a new
// audit table. valued_on on the affected positions (already shown in the
// positions table) is the passive per-asset signal; this is the per-run one.
type Summary struct {
	AssetsConsidered int
	PricesFetched    int
	PriceFailures    int
	// PricesDeferred counts assets left for a later run because their source
	// asked to be left alone (a 429 with Retry-After, an exhausted window).
	// It is not a failure: nothing is wrong with the asset.
	PricesDeferred   int
	PositionsCreated int
	PositionsUpdated int
	PositionsSkipped int
}

// RefreshAll prices every manual position of every asset that has a quote
// source configured, writing (or refreshing) one valuation operation dated
// today per position. It is built entirely on internal/investments' public
// API — ListAssets, ListPositions, ListOperations, CreateOperation,
// UpdateOperation — and never runs SQL against its tables directly, so
// schema ownership of investment_assets/investment_operations stays there.
//
// Each asset is identified by its market and canonical symbol. If no symbol
// was stored, the ticker is parsed locally.
//
// A single asset's market data failure is logged and does not block the
// others. Rate limited requests are counted as deferred.
func RefreshAll(ctx context.Context, conn *sql.DB, service *marketdata.Service, today time.Time) (Summary, error) {
	return refreshSpot(ctx, conn, service, today, false)
}

// RefreshMissing is RefreshAll for the assets that have no price for today
// from their source yet. It is what runs when a position or an asset was just
// saved: a newly quoted asset gets today's value now instead of at the next
// scheduled run, and an asset the daily run already priced costs no request.
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
	byAsset := map[string][]investments.Position{}
	for _, position := range positions {
		if position.AssetID == nil {
			continue
		}
		byAsset[*position.AssetID] = append(byAsset[*position.AssetID], position)
	}

	// A source that asked to be left alone is skipped for the rest of the
	// run; the next run (or the gap scan) picks its assets up again.
	for _, asset := range assets {
		if asset.QuoteSource == nil || *asset.QuoteSource == "" {
			continue
		}
		holdings := byAsset[asset.ID]
		if len(holdings) == 0 {
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
		summary.PricesFetched++
		storeSpotPrice(ctx, conn, &summary, asset, holdings, quote.Provider, quote.Price, today)
	}

	log.Printf(
		"quote_refresh_completed assets=%d fetched=%d failures=%d deferred=%d created=%d updated=%d skipped=%d",
		summary.AssetsConsidered, summary.PricesFetched, summary.PriceFailures, summary.PricesDeferred,
		summary.PositionsCreated, summary.PositionsUpdated, summary.PositionsSkipped,
	)
	return summary, nil
}

// pricedToday reports whether the asset's series already holds a price for
// today written by source.
func pricedToday(ctx context.Context, conn *sql.DB, assetID string, today time.Time) (bool, error) {
	quotes, err := investments.ListAssetQuotes(ctx, conn, assetID, &today)
	if err != nil {
		return false, err
	}
	if len(quotes) == 0 {
		return false, nil
	}
	last := quotes[len(quotes)-1]
	return last.QuotedOn.Equal(today) && marketdata.IsProvider(last.Source), nil
}

// storeSpotPrice records today's price of an asset in its price series and as
// a valuation on every position of it that holds units, tallying the outcome
// in summary.
func storeSpotPrice(ctx context.Context, conn *sql.DB, summary *Summary, asset investments.Asset, holdings []investments.Position, source string, price decimal.Decimal, today time.Time) {
	// The asset's price series is what rendimento over a period is derived
	// from; the valuation operations below stay the manual ledger's own
	// record of the same quote.
	if err := investments.UpsertAssetQuote(ctx, conn, investments.AssetQuote{
		AssetID: asset.ID, QuotedOn: today, Price: price, Source: source,
	}); err != nil {
		log.Printf("quote_store_failed asset_id=%s source=%s: %v", asset.ID, source, err)
	}

	for _, position := range holdings {
		// A position with no operations at all (just created, nothing
		// bought into it yet) replays with no ledger state at all, so
		// applyLedgerState never runs and Closed stays false even
		// though Quantity is zero — ListPositions' default Closed
		// filter does not catch that case. Guard on quantity directly
		// so neither that nor a properly closed (sold-to-zero,
		// Closed=true) position ever gets a valuation written.
		if !position.Quantity.IsPositive() {
			continue
		}
		outcome, err := refreshPosition(ctx, conn, position, source, price, today)
		if err != nil {
			log.Printf("quote_refresh_position_failed position_id=%s asset_id=%s: %v", position.ID, asset.ID, err)
			continue
		}
		switch outcome {
		case refreshCreated:
			summary.PositionsCreated++
		case refreshUpdated:
			summary.PositionsUpdated++
		case refreshSkipped:
			summary.PositionsSkipped++
		}
	}
}

type refreshOutcome int

const (
	refreshCreated refreshOutcome = iota
	refreshUpdated
	refreshSkipped
)

// refreshPosition writes today's automatic valuation for one position,
// deduping against whatever valuation (if any) already exists for it today.
// Amount is always a total (quantity × price), never a unit price — the
// same convention every manual valuation in internal/investments follows;
// the unit price is derived only for display by positionLedger.valuationUnitPrice.
func refreshPosition(ctx context.Context, conn *sql.DB, position investments.Position, source string, price decimal.Decimal, today time.Time) (refreshOutcome, error) {
	ops, err := investments.ListOperations(ctx, conn, investments.OperationFilter{
		PositionID: &position.ID, From: &today, To: &today,
	})
	if err != nil {
		return refreshSkipped, err
	}
	var existing *investments.Operation
	for i := range ops {
		if ops[i].Kind == investments.OperationValuation {
			existing = &ops[i]
			break
		}
	}

	amount := position.Quantity.Mul(price)
	notes := formatAutoQuoteNotes(source, price, time.Now())
	input := investments.OperationInput{
		AccountID:  position.AccountID,
		PositionID: &position.ID,
		Kind:       investments.OperationValuation,
		OccurredOn: today,
		Amount:     amount,
		Notes:      &notes,
	}

	if existing == nil {
		if _, err := investments.CreateOperation(ctx, conn, input); err != nil {
			return refreshSkipped, err
		}
		return refreshCreated, nil
	}
	if existing.Notes == nil || !strings.HasPrefix(*existing.Notes, AutoQuoteMarker) {
		log.Printf("quote_refresh_position_skipped position_id=%s reason=human_valuation_today", position.ID)
		return refreshSkipped, nil
	}
	if _, err := investments.UpdateOperation(ctx, conn, existing.ID, input); err != nil {
		return refreshSkipped, err
	}
	return refreshUpdated, nil
}

func formatAutoQuoteNotes(source string, price decimal.Decimal, at time.Time) string {
	return fmt.Sprintf("%s conector=%s preço=%s em %s", AutoQuoteMarker, source, price.StringFixed(2), at.Format(time.RFC3339))
}
