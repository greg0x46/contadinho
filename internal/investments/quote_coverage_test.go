package investments_test

import (
	"errors"
	"testing"
	"time"

	"contadinho-go/internal/investments"
)

func (f *ledgerFixture) assetOf(positionID string) string {
	f.t.Helper()
	var assetID string
	if err := f.conn.QueryRow(`SELECT asset_id FROM investment_positions WHERE id = ?`, positionID).Scan(&assetID); err != nil {
		f.t.Fatalf("asset of position: %v", err)
	}
	return assetID
}

// quoteFrom configures the market and canonical symbol on an asset.
func (f *ledgerFixture) quoteFrom(positionID, source, symbol string) string {
	f.t.Helper()
	assetID := f.assetOf(positionID)
	f.exec(`UPDATE investment_assets SET quote_source = ?, quote_symbol = ? WHERE id = ?`, source, symbol, assetID)
	return assetID
}

func (f *ledgerFixture) quote(assetID, source string, on time.Time, price string) {
	f.t.Helper()
	err := investments.UpsertAssetQuote(f.ctx, f.conn, investments.AssetQuote{
		AssetID: assetID, QuotedOn: on, Price: dec(price), Source: source,
	})
	if err != nil {
		f.t.Fatalf("UpsertAssetQuote: %v", err)
	}
}

func (f *ledgerFixture) buy(positionID string, on time.Time, amount, quantity string) {
	f.t.Helper()
	f.create(investments.OperationInput{
		AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationBuy,
		OccurredOn: on, Amount: dec(amount), Quantity: decRef(quantity),
	})
}

func (f *ledgerFixture) valuation(positionID string, on time.Time, amount string, notes *string) {
	f.t.Helper()
	f.create(investments.OperationInput{
		AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationValuation,
		OccurredOn: on, Amount: dec(amount), Notes: notes,
	})
}

func TestQuoteCoverageRoundTrip(t *testing.T) {
	f := newLedgerFixture(t)
	assetID := f.assetOf(f.addPosition(f.custodyID, "Petrobras PN"))

	if got, err := investments.GetQuoteCoverage(f.ctx, f.conn, assetID); err != nil || got != nil {
		t.Fatalf("GetQuoteCoverage before any = %+v, %v; want nil", got, err)
	}

	coverage := investments.QuoteCoverage{AssetID: assetID, Source: "b3", Symbol: "PETR4", From: day(2), To: day(20)}
	if err := investments.SaveQuoteCoverage(f.ctx, f.conn, coverage); err != nil {
		t.Fatalf("SaveQuoteCoverage: %v", err)
	}
	got, err := investments.GetQuoteCoverage(f.ctx, f.conn, assetID)
	if err != nil || got == nil {
		t.Fatalf("GetQuoteCoverage = %+v, %v", got, err)
	}
	if got.Source != "b3" || got.Symbol != "PETR4" || !got.From.Equal(day(2)) || !got.To.Equal(day(20)) {
		t.Errorf("coverage = %+v, want what was saved", got)
	}
	if !got.Contains(day(5), day(20)) || got.Contains(day(1), day(20)) || got.Contains(day(5), day(21)) {
		t.Errorf("Contains is wrong for %+v", got)
	}

	// Saving again replaces it rather than adding a second one.
	coverage.From = day(1)
	if err := investments.SaveQuoteCoverage(f.ctx, f.conn, coverage); err != nil {
		t.Fatalf("SaveQuoteCoverage (replace): %v", err)
	}
	if got, _ := investments.GetQuoteCoverage(f.ctx, f.conn, assetID); !got.From.Equal(day(1)) {
		t.Errorf("coverage from = %s, want it replaced by 2026-09-01", got.From.Format(investments.DateLayout))
	}

	for name, bad := range map[string]investments.QuoteCoverage{
		"no asset":       {Source: "b3", Symbol: "PETR4", From: day(1), To: day(2)},
		"no source":      {AssetID: assetID, Symbol: "PETR4", From: day(1), To: day(2)},
		"no symbol":      {AssetID: assetID, Source: "brapi", From: day(1), To: day(2)},
		"inverted range": {AssetID: assetID, Source: "b3", Symbol: "PETR4", From: day(5), To: day(2)},
	} {
		if err := investments.SaveQuoteCoverage(f.ctx, f.conn, bad); !errors.Is(err, investments.ErrInvalidInput) {
			t.Errorf("%s: SaveQuoteCoverage = %v, want ErrInvalidInput", name, err)
		}
	}
}

func TestAssetHeldFromIsTheFirstDayUnitsCameIn(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "Petrobras PN")
	assetID := f.assetOf(positionID)

	if got, err := investments.AssetHeldFrom(f.ctx, f.conn, assetID); err != nil || got != nil {
		t.Fatalf("AssetHeldFrom with no operations = %v, %v; want nil", got, err)
	}

	f.deposit("1000", day(1)) // cash is not the asset
	if got, _ := investments.AssetHeldFrom(f.ctx, f.conn, assetID); got != nil {
		t.Fatalf("AssetHeldFrom with only a cash deposit = %v, want nil", got)
	}

	f.buy(positionID, day(4), "300", "3")
	f.buy(positionID, day(9), "300", "3")
	got, err := investments.AssetHeldFrom(f.ctx, f.conn, assetID)
	if err != nil || got == nil || !got.Equal(day(4)) {
		t.Fatalf("AssetHeldFrom = %v, %v; want the first purchase, 2026-09-04", got, err)
	}

	// A correction that moves the first purchase earlier moves it too.
	f.buy(positionID, day(2), "100", "1")
	if got, _ := investments.AssetHeldFrom(f.ctx, f.conn, assetID); !got.Equal(day(2)) {
		t.Errorf("AssetHeldFrom after an earlier purchase = %s, want 2026-09-02", got.Format(investments.DateLayout))
	}
}

func TestUpsertConnectorQuotesReplacesItsOwnPricesAndSparesOtherWriters(t *testing.T) {
	f := newLedgerFixture(t)
	assetID := f.assetOf(f.addPosition(f.custodyID, "Petrobras PN"))

	f.quote(assetID, investments.QuoteSourcePluggy, day(5), "40.00") // observed by the provider
	f.quote(assetID, "yahoo", day(6), "41.00")                       // the daily run's spot

	err := investments.UpsertConnectorQuotes(f.ctx, f.conn, []investments.AssetQuote{
		{AssetID: assetID, QuotedOn: day(5), Price: dec("99.00"), Source: "brapi"}, // must not replace pluggy's
		{AssetID: assetID, QuotedOn: day(6), Price: dec("41.50"), Source: "brapi"}, // fallback close replaces Yahoo spot
		{AssetID: assetID, QuotedOn: day(7), Price: dec("42.00"), Source: "brapi"}, // new
	})
	if err != nil {
		t.Fatalf("UpsertConnectorQuotes: %v", err)
	}

	quotes, err := investments.ListAssetQuotes(f.ctx, f.conn, assetID, nil)
	if err != nil {
		t.Fatalf("ListAssetQuotes: %v", err)
	}
	want := []struct{ source, price string }{
		{investments.QuoteSourcePluggy, "40"}, {"brapi", "41.5"}, {"brapi", "42"},
	}
	if len(quotes) != len(want) {
		t.Fatalf("quotes = %+v, want %d days", quotes, len(want))
	}
	for i, w := range want {
		if quotes[i].Source != w.source || quotes[i].Price.String() != w.price {
			t.Errorf("day %d: %s at %s, want %s at %s", i, quotes[i].Source, quotes[i].Price, w.source, w.price)
		}
	}

	bad := []investments.AssetQuote{{AssetID: assetID, QuotedOn: day(8), Price: dec("0"), Source: "brapi"}}
	if err := investments.UpsertConnectorQuotes(f.ctx, f.conn, bad); !errors.Is(err, investments.ErrInvalidInput) {
		t.Errorf("a zero price: error = %v, want ErrInvalidInput", err)
	}
	if err := investments.UpsertConnectorQuotes(f.ctx, f.conn, nil); err != nil {
		t.Errorf("no quotes: error = %v, want nil", err)
	}
}

// A holding the user asked to be priced automatically is valued on any day
// from the units held then and the price that day, so a position registered
// with a past date has a rendimento for any period inside the history that was
// backfilled — not only for the days a valuation operation happens to exist.
func TestPositionYieldOfAQuotedManualHoldingUsesThePriceSeries(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "Petrobras PN")
	f.deposit("1000", day(1))
	f.buy(positionID, day(2), "600", "6") // 6 units at 100
	assetID := f.quoteFrom(positionID, "b3", "PETR4")
	f.quote(assetID, "brapi", day(2), "100")
	f.quote(assetID, "brapi", day(5), "110")
	f.quote(assetID, "brapi", day(8), "120")
	f.quote(assetID, "brapi", day(10), "130")

	// 6 × 130 − 6 × 100 invested.
	if got := f.yield(positionID, nil, "2026-09-10"); got.Value.String() != "180" {
		t.Fatalf("since inception = %s, want 180", got.Value)
	}
	// Any period inside the history, with no valuation operation on its ends:
	// 6 × 120 − 6 × 110.
	from := day(5)
	period := f.yield(positionID, &from, "2026-09-08")
	if period.Partial || period.Value.String() != "60" {
		t.Fatalf("5th→8th = %s (partial %v), want 60 over exactly that period", period.Value, period.Partial)
	}
}

func TestPositionYieldOfAQuotedHoldingFollowsTheUnitsHeldEachDay(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "Petrobras PN")
	f.deposit("2000", day(1))
	f.buy(positionID, day(2), "600", "6") // 6 units at 100
	f.buy(positionID, day(6), "660", "6") // 6 more at 110
	assetID := f.quoteFrom(positionID, "b3", "PETR4")
	f.quote(assetID, "brapi", day(2), "100")
	f.quote(assetID, "brapi", day(5), "105")
	f.quote(assetID, "brapi", day(6), "110")
	f.quote(assetID, "brapi", day(9), "120")

	// Before the second purchase 6 units are held, after it 12.
	// V(9th) = 12 × 120 = 1440; invested 600 + 660 = 1260.
	if got := f.yield(positionID, nil, "2026-09-09"); got.Value.String() != "180" {
		t.Fatalf("since inception = %s, want 180", got.Value)
	}
	// 5th→9th: V(9th) − V(5th) − the purchase on the 6th = 1440 − 630 − 660.
	from := day(5)
	if got := f.yield(positionID, &from, "2026-09-09"); got.Value.String() != "150" {
		t.Fatalf("5th→9th = %s, want 150", got.Value)
	}
}

func TestPositionYieldOfAQuotedHoldingLetsAPersonsValuationWin(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "Petrobras PN")
	f.deposit("1000", day(1))
	f.buy(positionID, day(2), "600", "6")
	assetID := f.quoteFrom(positionID, "b3", "PETR4")
	f.quote(assetID, "brapi", day(2), "100")
	f.quote(assetID, "brapi", day(8), "120")

	// The person says what it is worth on the 8th, whatever the feed says.
	f.valuation(positionID, day(8), "700", nil)
	if got := f.yield(positionID, nil, "2026-09-08"); got.Value.String() != "100" {
		t.Fatalf("with a typed valuation = %s, want 100 (700 − 600), not the feed's 120 × 6", got.Value)
	}

	// An automatic valuation is the job's own record of the same quote and
	// never outranks the series.
	auto := investments.AutoQuoteMarker + " conector=brapi"
	f.valuation(positionID, day(9), "999", &auto)
	if got := f.yield(positionID, nil, "2026-09-09"); got.Value.String() != "120" {
		t.Fatalf("with an automatic valuation = %s, want 120 (6 × 120 − 600): the 8th's quote still prices the 9th", got.Value)
	}
}

func TestPositionYieldOfAQuotedHoldingValuesASoldOutPositionAtZero(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "Petrobras PN")
	f.deposit("1000", day(1))
	f.buy(positionID, day(2), "600", "6")
	f.create(investments.OperationInput{
		AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationSell,
		OccurredOn: day(8), Amount: dec("720"), Quantity: decRef("6"),
	})
	assetID := f.quoteFrom(positionID, "b3", "PETR4")
	f.quote(assetID, "brapi", day(2), "100")
	f.quote(assetID, "brapi", day(8), "120")
	f.quote(assetID, "brapi", day(9), "500") // a price after it was all sold must not resurrect it

	// Realized: sold for 720, cost 600.
	if got := f.yield(positionID, nil, "2026-09-09"); got.Value.String() != "120" {
		t.Fatalf("realized = %s, want 120", got.Value)
	}
}

// An asset with no quote source keeps being valued only by the ledger's own
// valuations: whatever prices happen to sit in its series are not its
// holder's to be priced by.
func TestPositionYieldOfAnUnquotedManualHoldingIgnoresThePriceSeries(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "Petrobras PN")
	f.deposit("1000", day(1))
	f.buy(positionID, day(2), "600", "6")
	f.quote(f.assetOf(positionID), "brapi", day(8), "120") // no quote_source on the asset

	_, err := investments.PositionYield(f.ctx, f.conn, positionID, nil, calendarDay("2026-09-08"))
	var unavailable *investments.YieldUnavailableError
	if !errors.As(err, &unavailable) {
		t.Fatalf("PositionYield = %v, want it unavailable: nothing but cost ever valued this holding", err)
	}
}
