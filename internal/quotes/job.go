package quotes

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/investments"
)

// AutoQuoteMarker prefixes Notes on every valuation operation this package
// writes. A second run the same day (or a process restart) recognizes its
// own row by this prefix and updates it in place instead of duplicating it;
// a Notes value that does NOT start with it means a human entered or
// corrected a valuation today, which this job must never overwrite.
const AutoQuoteMarker = "[cotação automática]"

// Summary totals one RefreshAll run, logged as a single line so a stalled
// connector — or a run that quietly did nothing — is visible without a new
// audit table. valued_on on the affected positions (already shown in the
// positions table) is the passive per-asset signal; this is the per-run one.
type Summary struct {
	AssetsConsidered int
	PricesFetched    int
	PriceFailures    int
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
// A single asset's connector failure is logged and does not block the
// others (log.Printf, matching this codebase's existing non-Pluggy
// background-task error handling — deliberately no new audit/failure
// table).
func RefreshAll(ctx context.Context, conn *sql.DB, registry Registry, today time.Time) (Summary, error) {
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

	for _, asset := range assets {
		if asset.QuoteSource == nil || asset.QuoteSymbol == nil || *asset.QuoteSource == "" || *asset.QuoteSymbol == "" {
			continue
		}
		holdings := byAsset[asset.ID]
		if len(holdings) == 0 {
			continue
		}
		summary.AssetsConsidered++

		connector, ok := registry[*asset.QuoteSource]
		if !ok {
			log.Printf("quote_refresh_unknown_connector asset_id=%s source=%s", asset.ID, *asset.QuoteSource)
			continue
		}
		price, err := connector.FetchPrice(ctx, *asset.QuoteSymbol)
		if err != nil {
			summary.PriceFailures++
			log.Printf("quote_fetch_failed asset_id=%s source=%s symbol=%s: %v", asset.ID, *asset.QuoteSource, *asset.QuoteSymbol, err)
			continue
		}
		summary.PricesFetched++
		// The asset's price series is what rendimento over a period is
		// derived from; the valuation operations below stay the manual
		// ledger's own record of the same quote.
		if err := investments.UpsertAssetQuote(ctx, conn, investments.AssetQuote{
			AssetID: asset.ID, QuotedOn: today, Price: price, Source: *asset.QuoteSource,
		}); err != nil {
			log.Printf("quote_store_failed asset_id=%s source=%s: %v", asset.ID, *asset.QuoteSource, err)
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
			outcome, err := refreshPosition(ctx, conn, position, *asset.QuoteSource, price, today)
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

	log.Printf(
		"quote_refresh_completed assets=%d fetched=%d failures=%d created=%d updated=%d skipped=%d",
		summary.AssetsConsidered, summary.PricesFetched, summary.PriceFailures,
		summary.PositionsCreated, summary.PositionsUpdated, summary.PositionsSkipped,
	)
	return summary, nil
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
