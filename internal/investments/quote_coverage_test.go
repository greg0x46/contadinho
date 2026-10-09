package investments_test

import (
	"errors"
	"testing"
	"time"

	"github.com/greg0x46/julius/internal/investments"
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

// quote stores a price the way its source's writer would: Pluggy's is a sync
// price, an issue PU an issue price, anything else a market data provider's.
func (f *ledgerFixture) quote(assetID, source string, on time.Time, price string) {
	f.t.Helper()
	origin := investments.QuoteOriginMarket
	switch source {
	case investments.QuoteSourcePluggy:
		origin = investments.QuoteOriginSync
	case investments.QuoteSourceIssue:
		origin = investments.QuoteOriginIssue
	}
	err := investments.UpsertAssetQuote(f.ctx, f.conn, investments.AssetQuote{
		AssetID: assetID, QuotedOn: on, Price: dec(price), Source: source, Origin: origin,
	})
	if err != nil {
		f.t.Fatalf("UpsertAssetQuote: %v", err)
	}
}

// marketQuote is a market data provider's price for the connector writer.
func marketQuote(assetID, source string, on time.Time, price string) investments.AssetQuote {
	return investments.AssetQuote{
		AssetID: assetID, QuotedOn: on, Price: dec(price), Source: source, Origin: investments.QuoteOriginMarket,
	}
}

func (f *ledgerFixture) quotes(assetID string) map[string]investments.AssetQuote {
	f.t.Helper()
	list, err := investments.ListAssetQuotes(f.ctx, f.conn, assetID, nil)
	if err != nil {
		f.t.Fatalf("ListAssetQuotes: %v", err)
	}
	byDay := map[string]investments.AssetQuote{}
	for _, quote := range list {
		byDay[quote.QuotedOn.Format(investments.DateLayout)] = quote
	}
	return byDay
}

func (f *ledgerFixture) buy(positionID string, on time.Time, amount, quantity string) {
	f.t.Helper()
	f.create(investments.OperationInput{
		AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationBuy,
		OccurredOn: on, Amount: dec(amount), Quantity: decRef(quantity),
	})
}

func (f *ledgerFixture) valuation(positionID string, on time.Time, amount string) {
	f.t.Helper()
	f.create(investments.OperationInput{
		AccountID: f.custodyID, PositionID: &positionID, Kind: investments.OperationValuation,
		OccurredOn: on, Amount: dec(amount),
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

// Precedence is stated on the origin, not on the writer's name: a market data
// provider replaces another market price, whichever provider wrote it, and
// never a price the sync observed or one derived from the title.
func TestUpsertConnectorQuotesReplacesMarketPricesAndSparesTheRest(t *testing.T) {
	f := newLedgerFixture(t)
	assetID := f.assetOf(f.addPosition(f.custodyID, "Petrobras PN"))

	f.quote(assetID, investments.QuoteSourcePluggy, day(5), "40.00") // observed by the provider
	f.quote(assetID, "yahoo", day(6), "41.00")                       // the daily run's spot
	f.quote(assetID, investments.QuoteSourceIssue, day(7), "38.00")  // the PU the title was bought at

	err := investments.UpsertConnectorQuotes(f.ctx, f.conn, []investments.AssetQuote{
		marketQuote(assetID, "brapi", day(5), "99.00"), // must not replace the sync's
		marketQuote(assetID, "brapi", day(6), "41.50"), // fallback close replaces Yahoo spot
		marketQuote(assetID, "brapi", day(7), "97.00"), // must not replace the issue price
		marketQuote(assetID, "brapi", day(8), "42.00"), // new
	})
	if err != nil {
		t.Fatalf("UpsertConnectorQuotes: %v", err)
	}

	want := map[string]struct {
		source, price string
		origin        investments.QuoteOrigin
	}{
		"2026-09-05": {investments.QuoteSourcePluggy, "40", investments.QuoteOriginSync},
		"2026-09-06": {"brapi", "41.5", investments.QuoteOriginMarket},
		"2026-09-07": {investments.QuoteSourceIssue, "38", investments.QuoteOriginIssue},
		"2026-09-08": {"brapi", "42", investments.QuoteOriginMarket},
	}
	got := f.quotes(assetID)
	if len(got) != len(want) {
		t.Fatalf("quotes = %+v, want %d days", got, len(want))
	}
	for day, w := range want {
		if q := got[day]; q.Source != w.source || q.Price.String() != w.price || q.Origin != w.origin {
			t.Errorf("%s: %s/%s at %s, want %s/%s at %s", day, q.Source, q.Origin, q.Price, w.source, w.origin, w.price)
		}
	}
}

// What the sync observes replaces whatever is stored for the day, a market
// price included, and takes the origin with it.
func TestSyncedQuoteReplacesAMarketPriceOfTheSameDay(t *testing.T) {
	f := newLedgerFixture(t)
	assetID := f.assetOf(f.addPosition(f.custodyID, "Petrobras PN"))
	if err := investments.UpsertConnectorQuotes(f.ctx, f.conn, []investments.AssetQuote{marketQuote(assetID, "yahoo", day(5), "41.00")}); err != nil {
		t.Fatalf("UpsertConnectorQuotes: %v", err)
	}

	f.quote(assetID, investments.QuoteSourcePluggy, day(5), "40.00")
	if q := f.quotes(assetID)["2026-09-05"]; q.Source != investments.QuoteSourcePluggy || q.Origin != investments.QuoteOriginSync || q.Price.String() != "40" {
		t.Fatalf("quote = %+v, want the sync's 40", q)
	}

	// And a market price cannot take the day back.
	if err := investments.UpsertConnectorQuotes(f.ctx, f.conn, []investments.AssetQuote{marketQuote(assetID, "yahoo", day(5), "41.00")}); err != nil {
		t.Fatalf("UpsertConnectorQuotes: %v", err)
	}
	if q := f.quotes(assetID)["2026-09-05"]; q.Origin != investments.QuoteOriginSync || q.Price.String() != "40" {
		t.Fatalf("quote = %+v, want the sync's 40 kept", q)
	}
}

// A snapshot records the price it observed as a sync price and a title's
// purchase PU as an issue price, which only fills a day nothing else priced.
func TestRecordSyncedQuotesStampsOriginsAndLetsTheIssuePriceOnlyFillGaps(t *testing.T) {
	f := newLedgerFixture(t)
	holding := lca() // bought 2024-09-06 for 200 → PU 1
	asset, err := investments.ResolveSyncedAsset(f.ctx, f.conn, holding)
	if err != nil {
		t.Fatalf("ResolveSyncedAsset: %v", err)
	}

	f.recordQuote(holding, "2026-09-28T00:03:22Z", "258.6")
	got := f.quotes(asset.ID)
	if q := got["2024-09-06"]; q.Source != investments.QuoteSourceIssue || q.Origin != investments.QuoteOriginIssue || q.Price.String() != "1" {
		t.Errorf("purchase day = %+v, want the issue PU of 1", q)
	}
	// 00:03Z on the 28th is still the 27th on the provider's wall clock.
	if q := got["2026-09-27"]; q.Source != investments.QuoteSourcePluggy || q.Origin != investments.QuoteOriginSync {
		t.Errorf("as-of day = %+v, want a sync price from pluggy", q)
	}

	// A market price already stored on another title's purchase day is not
	// displaced by that title's PU.
	other := lca()
	other.Code, other.ExternalID = strRef("24I01250143"), "lca-2"
	otherAsset, err := investments.ResolveSyncedAsset(f.ctx, f.conn, other)
	if err != nil {
		t.Fatalf("ResolveSyncedAsset: %v", err)
	}
	if err := investments.UpsertConnectorQuotes(f.ctx, f.conn, []investments.AssetQuote{
		marketQuote(otherAsset.ID, "yahoo", calendarDay("2024-09-06"), "1.05"),
	}); err != nil {
		t.Fatalf("UpsertConnectorQuotes: %v", err)
	}
	f.recordQuote(other, "2026-09-28T00:03:22Z", "258.6")
	if q := f.quotes(otherAsset.ID)["2024-09-06"]; q.Origin != investments.QuoteOriginMarket || q.Price.String() != "1.05" {
		t.Errorf("purchase day = %+v, want the market price kept: an issue PU only fills gaps", q)
	}
}

func TestQuoteWritersRefuseAMissingOrWrongOrigin(t *testing.T) {
	f := newLedgerFixture(t)
	assetID := f.assetOf(f.addPosition(f.custodyID, "Petrobras PN"))
	noOrigin := investments.AssetQuote{AssetID: assetID, QuotedOn: day(5), Price: dec("40"), Source: "pluggy"}

	if err := investments.UpsertAssetQuote(f.ctx, f.conn, noOrigin); !errors.Is(err, investments.ErrInvalidInput) {
		t.Errorf("UpsertAssetQuote without origin = %v, want ErrInvalidInput", err)
	}
	unknown := noOrigin
	unknown.Origin = "guess"
	if err := investments.UpsertAssetQuote(f.ctx, f.conn, unknown); !errors.Is(err, investments.ErrInvalidInput) {
		t.Errorf("UpsertAssetQuote with an unknown origin = %v, want ErrInvalidInput", err)
	}
	if err := investments.UpsertConnectorQuotes(f.ctx, f.conn, []investments.AssetQuote{noOrigin}); !errors.Is(err, investments.ErrInvalidInput) {
		t.Errorf("UpsertConnectorQuotes without origin = %v, want ErrInvalidInput", err)
	}
	// A connector writes market prices; it cannot claim to be the sync.
	synced := noOrigin
	synced.Origin = investments.QuoteOriginSync
	if err := investments.UpsertConnectorQuotes(f.ctx, f.conn, []investments.AssetQuote{synced}); !errors.Is(err, investments.ErrInvalidInput) {
		t.Errorf("UpsertConnectorQuotes as the sync = %v, want ErrInvalidInput", err)
	}
	zero := marketQuote(assetID, "brapi", day(8), "0")
	if err := investments.UpsertConnectorQuotes(f.ctx, f.conn, []investments.AssetQuote{zero}); !errors.Is(err, investments.ErrInvalidInput) {
		t.Errorf("a zero price: error = %v, want ErrInvalidInput", err)
	}
	if err := investments.UpsertConnectorQuotes(f.ctx, f.conn, nil); err != nil {
		t.Errorf("no quotes: error = %v, want nil", err)
	}
	if got := f.quotes(assetID); len(got) != 0 {
		t.Errorf("quotes = %+v, want none stored by the refused writes", got)
	}
}

func TestLatestAssetQuoteIsTheNewestDayWhateverTheOrderWritten(t *testing.T) {
	f := newLedgerFixture(t)
	assetID := f.assetOf(f.addPosition(f.custodyID, "Petrobras PN"))

	if got, err := investments.LatestAssetQuote(f.ctx, f.conn, assetID); err != nil || got != nil {
		t.Fatalf("LatestAssetQuote on an empty series = %+v, %v; want nil", got, err)
	}
	f.quote(assetID, "brapi", day(8), "120")
	f.quote(assetID, investments.QuoteSourcePluggy, day(5), "110")
	got, err := investments.LatestAssetQuote(f.ctx, f.conn, assetID)
	if err != nil || got == nil {
		t.Fatalf("LatestAssetQuote = %+v, %v", got, err)
	}
	if !got.QuotedOn.Equal(day(8)) || got.Price.String() != "120" || got.Source != "brapi" || got.Origin != investments.QuoteOriginMarket {
		t.Fatalf("latest = %+v, want the 8th's market price", got)
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

	// The person says what it is worth on the 8th, the same day as the feed's
	// quote: on a tie the typed valuation wins.
	f.valuation(positionID, day(8), "700")
	if got := f.yield(positionID, nil, "2026-09-08"); got.Value.String() != "100" {
		t.Fatalf("with a typed valuation = %s, want 100 (700 − 600), not the feed's 120 × 6", got.Value)
	}
}

// The latest statement about the price wins. A valuation typed after the last
// quote keeps valuing the holding on the days that follow, until a newer quote
// arrives.
func TestPositionYieldOfAQuotedHoldingKeepsANewerValuationUntilANewerQuote(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "Petrobras PN")
	f.deposit("1000", day(1))
	f.buy(positionID, day(2), "600", "6")
	assetID := f.quoteFrom(positionID, "b3", "PETR4")
	f.quote(assetID, "brapi", day(2), "100")
	f.quote(assetID, "brapi", day(5), "110")
	f.valuation(positionID, day(8), "720") // newer than the 5th's quote

	for _, to := range []string{"2026-09-06", "2026-09-07"} { // the 5th's quote still prices these
		if got := f.yield(positionID, nil, to); got.Value.String() != "60" {
			t.Fatalf("until %s = %s, want 60 (6 × 110 − 600)", to, got.Value)
		}
	}
	for _, to := range []string{"2026-09-08", "2026-09-09", "2026-09-10"} {
		if got := f.yield(positionID, nil, to); got.Value.String() != "120" {
			t.Fatalf("until %s = %s, want 120 (the typed 720 − 600), not the older quote's 6 × 110", to, got.Value)
		}
	}

	f.quote(assetID, "brapi", day(12), "130")
	if got := f.yield(positionID, nil, "2026-09-11"); got.Value.String() != "120" {
		t.Fatalf("the day before the new quote = %s, want the valuation's 120", got.Value)
	}
	if got := f.yield(positionID, nil, "2026-09-12"); got.Value.String() != "180" {
		t.Fatalf("the day of the new quote = %s, want 180 (6 × 130 − 600)", got.Value)
	}
}

// A valuation older than the quote yields to it: the quote is the later
// statement about the price.
func TestPositionYieldOfAQuotedHoldingLetsANewerQuoteReplaceAnOlderValuation(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "Petrobras PN")
	f.deposit("1000", day(1))
	f.buy(positionID, day(2), "600", "6")
	assetID := f.quoteFrom(positionID, "b3", "PETR4")
	f.quote(assetID, "brapi", day(2), "100")
	f.valuation(positionID, day(5), "650")
	f.quote(assetID, "brapi", day(8), "120")

	for _, to := range []string{"2026-09-05", "2026-09-06", "2026-09-07"} {
		if got := f.yield(positionID, nil, to); got.Value.String() != "50" {
			t.Fatalf("until %s = %s, want 50 (the typed 650 − 600)", to, got.Value)
		}
	}
	if got := f.yield(positionID, nil, "2026-09-08"); got.Value.String() != "120" {
		t.Fatalf("on the newer quote's day = %s, want 120 (6 × 120 − 600)", got.Value)
	}
}

// The manual series rounds a quoted value to cents, like a synced holding's.
func TestPositionYieldOfAQuotedManualHoldingRoundsToCents(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "Petrobras PN")
	f.deposit("1000", day(1))
	f.buy(positionID, day(2), "300", "3")
	assetID := f.quoteFrom(positionID, "b3", "PETR4")
	f.quote(assetID, "brapi", day(5), "100.0033") // 3 × 100.0033 = 300.0099

	if got := f.yield(positionID, nil, "2026-09-05"); got.EndValue.String() != "300.01" || got.Value.String() != "0.01" {
		t.Fatalf("end value = %s, yield = %s; want 300.01 and 0.01", got.EndValue, got.Value)
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

// A quote more than ten days old prices nothing, so the yield is unavailable.
// It does not fall back to a typed valuation that is older than the quote:
// that would answer with a statement the quote has already replaced.
func TestPositionYieldOfAQuotedHoldingIsUnavailableOnceTheQuoteGoesStale(t *testing.T) {
	f := newLedgerFixture(t)
	positionID := f.addPosition(f.custodyID, "Petrobras PN")
	f.deposit("1000", day(1))
	f.buy(positionID, day(2), "600", "6")
	assetID := f.quoteFrom(positionID, "b3", "PETR4")
	f.quote(assetID, "brapi", day(2), "100")
	f.valuation(positionID, day(3), "650")
	f.quote(assetID, "brapi", day(5), "120") // newer than the valuation

	// Ten days later the quote still prices the day.
	if got := f.yield(positionID, nil, "2026-09-15"); got.Value.String() != "120" {
		t.Fatalf("ten days after the quote = %s, want 120 (6 × 120 − 600)", got.Value)
	}
	// Eleven days later it no longer does, and the older valuation is not asked.
	_, err := investments.PositionYield(f.ctx, f.conn, positionID, nil, calendarDay("2026-09-16"))
	var unavailable *investments.YieldUnavailableError
	if !errors.As(err, &unavailable) || unavailable.Reason != investments.YieldReasonNoPrice {
		t.Fatalf("eleven days after the quote = %v, want the yield unavailable for lack of a price", err)
	}
}
