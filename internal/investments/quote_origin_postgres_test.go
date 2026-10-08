package investments_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	"contadinho-go/internal/db"
	"contadinho-go/internal/investments"
)

// newPostgresLedgerFixture is a ledgerFixture on a freshly migrated, private
// schema of the Postgres at CONTADINHO_TEST_POSTGRES_DSN, so the quote
// precedence SQL (ON CONFLICT ... DO UPDATE ... WHERE origin = ..., DO NOTHING)
// is exercised on the production dialect. It skips without that variable. The
// schema is dropped when the test ends, leaving the other tests' tables alone.
func newPostgresLedgerFixture(t *testing.T) *ledgerFixture {
	t.Helper()
	base := os.Getenv("CONTADINHO_TEST_POSTGRES_DSN")
	if base == "" {
		t.Skip("CONTADINHO_TEST_POSTGRES_DSN not set")
	}
	raw, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { raw.Close() })
	schema := "quote_origin_" + fmt.Sprintf("%x", uuid.New())
	if _, err := raw.Exec(`CREATE SCHEMA ` + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { raw.Exec(`DROP SCHEMA ` + schema + ` CASCADE`) })

	u, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	conn, err := db.Open(u.String())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })

	f := &ledgerFixture{t: t, conn: conn, ctx: context.Background(), sourceID: uuid.NewString(), rawImportID: uuid.NewString()}
	now := db.FormatTime(time.Now())
	f.exec(`INSERT INTO data_sources (id, provider, external_item_id, created_at, updated_at)
		VALUES (?, 'pluggy', 'item-1', ?, ?)`, f.sourceID, now, now)
	syncRunID := uuid.NewString()
	f.exec(`INSERT INTO sync_runs (id, source_id, status, started_at, finished_at)
		VALUES (?, ?, 'completed', ?, ?)`, syncRunID, f.sourceID, now, now)
	f.exec(`INSERT INTO raw_imports (
			id, sync_run_id, source_id, scope, page_sequence, request_attempt,
			request_method, request_path, http_status, response_headers, payload,
			payload_sha256, received_at
		) VALUES (?, ?, ?, 'accounts', 1, 1, 'GET', '/x', 200, '{}', ?, 'sha', ?)`,
		f.rawImportID, syncRunID, f.sourceID, []byte{0}, now)
	account, err := investments.CreateAccount(f.ctx, conn, investments.AccountInput{Name: "Corretora"})
	if err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	f.custodyID = account.ID
	return f
}

// UpsertConnectorQuotes on Postgres: a market price replaces another market
// price (any provider) and never a sync or issue price; it refuses origins
// other than market, and a refused batch leaves nothing behind.
func TestPostgresUpsertConnectorQuotesReplacesMarketPricesAndSparesTheRest(t *testing.T) {
	f := newPostgresLedgerFixture(t)
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

	// A refused quote in the batch rolls the whole transaction back, including
	// the valid ones before it.
	err = investments.UpsertConnectorQuotes(f.ctx, f.conn, []investments.AssetQuote{
		marketQuote(assetID, "yahoo", day(6), "50.00"),
		{AssetID: assetID, QuotedOn: day(9), Price: dec("1"), Source: "pluggy", Origin: investments.QuoteOriginSync},
	})
	if !errors.Is(err, investments.ErrInvalidInput) {
		t.Fatalf("batch with a sync origin = %v, want ErrInvalidInput", err)
	}
	after := f.quotes(assetID)
	if len(after) != len(want) || after["2026-09-06"].Source != "brapi" || after["2026-09-06"].Price.String() != "41.5" {
		t.Errorf("a refused batch changed the stored prices: %+v", after)
	}

	// Same-day update of one's own market price: price and source follow the
	// latest market writer, and the origin stays market.
	if err := investments.UpsertConnectorQuotes(f.ctx, f.conn, []investments.AssetQuote{marketQuote(assetID, "coingecko", day(6), "43.00")}); err != nil {
		t.Fatal(err)
	}
	if q := f.quotes(assetID)["2026-09-06"]; q.Source != "coingecko" || q.Price.String() != "43" || q.Origin != investments.QuoteOriginMarket {
		t.Errorf("2026-09-06 = %+v, want coingecko/market at 43", q)
	}
}

// UpsertAssetQuote on Postgres replaces whatever is stored for the day, from
// any origin to any origin, and takes source, price, origin and raw_import_id
// (including clearing it) with it.
func TestPostgresUpsertAssetQuoteReplacesAnyOrigin(t *testing.T) {
	f := newPostgresLedgerFixture(t)
	assetID := f.assetOf(f.addPosition(f.custodyID, "Petrobras PN"))

	origins := []struct {
		origin investments.QuoteOrigin
		source string
	}{
		{investments.QuoteOriginSync, investments.QuoteSourcePluggy},
		{investments.QuoteOriginIssue, investments.QuoteSourceIssue},
		{investments.QuoteOriginMarket, "yahoo"},
	}
	n := 0
	for _, from := range origins {
		for _, to := range origins {
			n++
			on := day(n)
			first := investments.AssetQuote{AssetID: assetID, QuotedOn: on, Price: dec("10"), Source: from.source, Origin: from.origin, RawImportID: &f.rawImportID}
			if err := investments.UpsertAssetQuote(f.ctx, f.conn, first); err != nil {
				t.Fatalf("%s first write: %v", from.origin, err)
			}
			second := investments.AssetQuote{AssetID: assetID, QuotedOn: on, Price: dec("20.5"), Source: to.source + "-2", Origin: to.origin}
			if err := investments.UpsertAssetQuote(f.ctx, f.conn, second); err != nil {
				t.Fatalf("%s over %s: %v", to.origin, from.origin, err)
			}
			q := f.quotes(assetID)[on.Format(investments.DateLayout)]
			if q.Source != second.Source || q.Origin != to.origin || q.Price.String() != "20.5" || q.RawImportID != nil {
				t.Errorf("%s over %s = %+v, want %s/%s at 20.5 with no raw import", to.origin, from.origin, q, second.Source, to.origin)
			}
		}
	}
	if got := f.quotes(assetID); len(got) != n {
		t.Errorf("%d quotes stored, want one per day (%d)", len(got), n)
	}

	// The raw import reference is stored when given.
	with := investments.AssetQuote{AssetID: assetID, QuotedOn: day(20), Price: dec("1"), Source: "pluggy", Origin: investments.QuoteOriginSync, RawImportID: &f.rawImportID}
	if err := investments.UpsertAssetQuote(f.ctx, f.conn, with); err != nil {
		t.Fatal(err)
	}
	if q := f.quotes(assetID)["2026-09-20"]; q.RawImportID == nil || *q.RawImportID != f.rawImportID {
		t.Errorf("raw import = %v, want %s", q.RawImportID, f.rawImportID)
	}

	// An unknown origin is refused before reaching the CHECK.
	bad := with
	bad.QuotedOn, bad.Origin = day(21), "guess"
	if err := investments.UpsertAssetQuote(f.ctx, f.conn, bad); !errors.Is(err, investments.ErrInvalidInput) {
		t.Errorf("unknown origin = %v, want ErrInvalidInput", err)
	}
}

// insertAssetQuoteIfAbsent (reached through RecordSyncedQuotes) on Postgres:
// the title's purchase PU is an issue price that fills only a day nothing
// else priced, and the sync's own price for the as-of day is always replaced.
func TestPostgresRecordSyncedQuotesIssuePriceOnlyFillsGaps(t *testing.T) {
	f := newPostgresLedgerFixture(t)
	holding := lca() // bought 2024-09-06 for 200 -> PU 1
	asset, err := investments.ResolveSyncedAsset(f.ctx, f.conn, holding)
	if err != nil {
		t.Fatalf("ResolveSyncedAsset: %v", err)
	}

	f.recordQuote(holding, "2026-09-28T00:03:22Z", "258.6")
	got := f.quotes(asset.ID)
	if q := got["2024-09-06"]; q.Source != investments.QuoteSourceIssue || q.Origin != investments.QuoteOriginIssue || q.Price.String() != "1" {
		t.Errorf("purchase day = %+v, want the issue PU of 1", q)
	}
	if q := got["2026-09-27"]; q.Source != investments.QuoteSourcePluggy || q.Origin != investments.QuoteOriginSync || q.Price.String() != "1.293" {
		t.Errorf("as-of day = %+v, want a sync price of 1.293 from pluggy", q)
	}

	// A later snapshot: the sync's as-of price for the same day is replaced,
	// the issue PU is left exactly as it was (DO NOTHING, no error).
	f.recordQuote(holding, "2026-09-28T00:03:22Z", "300")
	got = f.quotes(asset.ID)
	if len(got) != 2 {
		t.Fatalf("quotes = %+v, want two days", got)
	}
	if q := got["2026-09-27"]; q.Price.String() != "1.5" || q.Origin != investments.QuoteOriginSync {
		t.Errorf("as-of day = %+v, want the sync's 1.5", q)
	}
	if q := got["2024-09-06"]; q.Source != investments.QuoteSourceIssue || q.Price.String() != "1" {
		t.Errorf("purchase day = %+v, want the issue PU kept", q)
	}

	// Days already priced by a market provider or by the sync are not
	// displaced by the title's PU.
	for i, existing := range []struct {
		origin investments.QuoteOrigin
		source string
	}{{investments.QuoteOriginMarket, "yahoo"}, {investments.QuoteOriginSync, investments.QuoteSourcePluggy}} {
		other := lca()
		other.Code, other.ExternalID = strRef(fmt.Sprintf("24I0125015%d", i)), fmt.Sprintf("lca-other-%d", i)
		otherAsset, err := investments.ResolveSyncedAsset(f.ctx, f.conn, other)
		if err != nil {
			t.Fatalf("ResolveSyncedAsset: %v", err)
		}
		if err := investments.UpsertAssetQuote(f.ctx, f.conn, investments.AssetQuote{
			AssetID: otherAsset.ID, QuotedOn: calendarDay("2024-09-06"), Price: dec("1.05"), Source: existing.source, Origin: existing.origin,
		}); err != nil {
			t.Fatal(err)
		}
		f.recordQuote(other, "2026-09-28T00:03:22Z", "258.6")
		if q := f.quotes(otherAsset.ID)["2024-09-06"]; q.Origin != existing.origin || q.Source != existing.source || q.Price.String() != "1.05" {
			t.Errorf("%s price on the purchase day = %+v, want it kept: an issue PU only fills gaps", existing.origin, q)
		}
	}
}
