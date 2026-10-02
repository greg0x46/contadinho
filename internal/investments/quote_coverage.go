package investments

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/db"
	"contadinho-go/internal/money"
)

// AutoQuoteMarker prefixes Notes on every valuation operation internal/quotes
// writes. A valuation without it was typed by a person, and rendimento never
// lets a connector's price override one on the same day.
const AutoQuoteMarker = "[cotação automática]"

// QuoteCoverage is the interval of days an asset's price series has already
// been asked for a market and canonical symbol. A day inside it with no quote is a day the market did not
// trade, not a day nobody fetched.
type QuoteCoverage struct {
	AssetID string
	Source  string
	Symbol  string
	From    time.Time
	To      time.Time
}

// Contains reports whether the whole closed interval [from, to] is covered.
func (c QuoteCoverage) Contains(from, to time.Time) bool {
	return !Day(from).Before(c.From) && !Day(to).After(c.To)
}

// GetQuoteCoverage returns the asset's covered interval, or nil when no
// market history has been asked for it yet.
func GetQuoteCoverage(ctx context.Context, q Querier, assetID string) (*QuoteCoverage, error) {
	var (
		coverage QuoteCoverage
		from, to string
	)
	err := q.QueryRowContext(ctx,
		`SELECT asset_id, source, symbol, covered_from, covered_to FROM investment_asset_quote_coverage WHERE asset_id = ?`,
		assetID).Scan(&coverage.AssetID, &coverage.Source, &coverage.Symbol, &from, &to)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if coverage.From, err = parseDate(from); err != nil {
		return nil, err
	}
	if coverage.To, err = parseDate(to); err != nil {
		return nil, err
	}
	return &coverage, nil
}

// SaveQuoteCoverage replaces the asset's covered interval.
func SaveQuoteCoverage(ctx context.Context, q Querier, coverage QuoteCoverage) error {
	if coverage.AssetID == "" || coverage.Source == "" || coverage.Symbol == "" ||
		coverage.From.IsZero() || coverage.To.IsZero() || coverage.To.Before(coverage.From) {
		return ErrInvalidInput
	}
	_, err := q.ExecContext(ctx, `
		INSERT INTO investment_asset_quote_coverage (asset_id, source, symbol, covered_from, covered_to, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (asset_id) DO UPDATE SET
			source = excluded.source, symbol = excluded.symbol,
			covered_from = excluded.covered_from, covered_to = excluded.covered_to,
			updated_at = excluded.updated_at`,
		coverage.AssetID, coverage.Source, coverage.Symbol,
		formatDate(coverage.From), formatDate(coverage.To), db.FormatTime(time.Now()))
	return err
}

// UpsertConnectorQuotes stores market price history in one transaction. A
// market close replaces a spot from any market data provider, including when
// fallback changes the provider. Pluggy quotes and issue prices stay intact.
func UpsertConnectorQuotes(ctx context.Context, conn *sql.DB, quotes []AssetQuote) error {
	if len(quotes) == 0 {
		return nil
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := db.FormatTime(time.Now())
	for _, quote := range quotes {
		if quote.AssetID == "" || quote.Source == "" || !quote.Price.IsPositive() {
			return ErrInvalidInput
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO investment_asset_quotes (asset_id, quoted_on, price, source, raw_import_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, NULL, ?, ?)
			ON CONFLICT (asset_id, quoted_on) DO UPDATE SET
				price = excluded.price, source = excluded.source, updated_at = excluded.updated_at
			WHERE investment_asset_quotes.source NOT IN (?, ?)`,
			quote.AssetID, formatDate(quote.QuotedOn), money.CanonicalDecimal(quote.Price), quote.Source, now, now, QuoteSourcePluggy, QuoteSourceIssue); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// AssetHeldFrom is the first day any manual position of the asset took units
// in (opening balance, purchase or transfer in): the earliest day the asset
// needs a price. Nil when no position holds it.
func AssetHeldFrom(ctx context.Context, q Querier, assetID string) (*time.Time, error) {
	var first sql.NullString
	err := q.QueryRowContext(ctx, `
		SELECT MIN(o.occurred_on)
		FROM investment_operations o
		JOIN investment_positions p ON p.id = o.position_id
		WHERE p.asset_id = ? AND o.kind IN ('initial_balance', 'buy', 'transfer_in')`,
		assetID).Scan(&first)
	if err != nil {
		return nil, err
	}
	if !first.Valid {
		return nil, nil
	}
	day, err := parseDate(first.String)
	if err != nil {
		return nil, err
	}
	return &day, nil
}

// quantityTimeline is how many units a position held at the end of each day
// it moved, from its own operations: what a price on a later day multiplies.
type quantityTimeline struct {
	days []time.Time
	held []decimal.Decimal
}

func newQuantityTimeline(events []flowEvent) quantityTimeline {
	var timeline quantityTimeline
	running := decimal.Zero
	for _, event := range events {
		running = running.Add(event.quantity)
		if n := len(timeline.days); n > 0 && timeline.days[n-1].Equal(event.day) {
			timeline.held[n-1] = running
			continue
		}
		timeline.days = append(timeline.days, event.day)
		timeline.held = append(timeline.held, running)
	}
	return timeline
}

// at is the quantity held at the end of day d.
func (t quantityTimeline) at(d time.Time) decimal.Decimal {
	held := decimal.Zero
	for i, day := range t.days {
		if day.After(d) {
			break
		}
		held = t.held[i]
	}
	return held
}
