package httpapi_test

import (
	"context"
	"database/sql"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/greg0x46/julius/internal/investments"
)

// insertSyncedEquity turns a row made by insertInvestment into a listed share
// with a code, so it resolves to its own catalog asset.
func insertSyncedEquity(t *testing.T, conn *sql.DB, name, code, quantity, balance string) string {
	t.Helper()
	id := insertInvestment(t, conn)
	if _, err := conn.Exec(`UPDATE financial_investments
		SET investment_type = 'EQUITY', name = ?, code = ?, quantity = ?, balance = ? WHERE id = ?`,
		name, code, quantity, balance, id); err != nil {
		t.Fatalf("update investment: %v", err)
	}
	return id
}

// quoteToday gives the asset behind code a price today, creating the asset the
// way a sync would.
func quoteToday(t *testing.T, conn *sql.DB, name, code, price string) string {
	t.Helper()
	ctx := context.Background()
	holding := investments.SyncedHolding{
		ExternalID: uuid.NewString(), Name: &name, Code: &code, InvestmentType: strPtr("EQUITY"),
	}
	asset, err := investments.ResolveSyncedAsset(ctx, conn, holding)
	if err != nil {
		t.Fatalf("ResolveSyncedAsset: %v", err)
	}
	if err := investments.UpsertAssetQuote(ctx, conn, investments.AssetQuote{
		AssetID: asset.ID, QuotedOn: investments.ProviderDay(time.Now()), Price: decimal.RequireFromString(price),
		Source: investments.QuoteSourcePluggy, Origin: investments.QuoteOriginSync,
	}); err != nil {
		t.Fatalf("UpsertAssetQuote: %v", err)
	}
	return asset.ID
}

func countRows(t *testing.T, conn *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatalf("count %q: %v", query, err)
	}
	return n
}

func listInvestmentsByName(t *testing.T, baseURL string) map[string]map[string]any {
	t.Helper()
	resp, err := testGet(t, baseURL+"/api/investments")
	if err != nil {
		t.Fatalf("GET /api/investments: %v", err)
	}
	var list []map[string]any
	decodeJSON(t, resp, &list)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/investments status = %d, want 200 (body %+v)", resp.StatusCode, list)
	}
	byName := map[string]map[string]any{}
	for _, item := range list {
		byName[item["name"].(string)] = item
	}
	return byName
}

func wantYield(t *testing.T, item map[string]any, value, source string) {
	t.Helper()
	if item["yield_value"] != value || item["yield_source"] != source {
		t.Errorf("%v: yield_value = %v, yield_source = %v, want %s/%s",
			item["name"], item["yield_value"], item["yield_source"], value, source)
	}
}

// Yield is read on every page load: it must price a holding from the series
// that exists, never create the asset of one that has none yet.
func TestReadingInvestmentsDoesNotCreateAssets(t *testing.T) {
	srv, conn := newTestServer(t)
	priced := insertSyncedEquity(t, conn, "Ação A", "AAA3", "10", "1100")
	insertInvestmentMovement(t, conn, priced, "BUY", "inflow", "1000", strPtr("10"))
	quoteToday(t, conn, "Ação A", "AAA3", "110")
	unpriced := insertSyncedEquity(t, conn, "Ação B", "BBB3", "5", "650")
	insertInvestmentMovement(t, conn, unpriced, "BUY", "inflow", "500", strPtr("5"))

	assetsBefore := countRows(t, conn, `SELECT COUNT(*) FROM investment_assets`)
	quotesBefore := countRows(t, conn, `SELECT COUNT(*) FROM investment_asset_quotes`)

	byName := listInvestmentsByName(t, srv.URL)
	// A: priced from its series. B: no asset, so the movement history nets.
	wantYield(t, byName["Ação A"], "100.00", "calculado")
	wantYield(t, byName["Ação B"], "150", "calculado")

	for _, id := range []string{priced, unpriced} {
		resp, err := testGet(t, srv.URL+"/api/investments/"+id)
		if err != nil {
			t.Fatalf("GET /api/investments/{id}: %v", err)
		}
		var got map[string]any
		decodeJSON(t, resp, &got)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/investments/%s status = %d, want 200", id, resp.StatusCode)
		}
		want := map[string]string{priced: "100.00", unpriced: "150"}[id]
		wantYield(t, got, want, "calculado")
	}

	if got := countRows(t, conn, `SELECT COUNT(*) FROM investment_assets`); got != assetsBefore {
		t.Errorf("investment_assets = %d after reading, want %d", got, assetsBefore)
	}
	if got := countRows(t, conn, `SELECT COUNT(*) FROM investment_assets WHERE canonical_key = 'ticker:BBB3'`); got != 0 {
		t.Errorf("the asset of the unpriced holding was created by a read")
	}
	if got := countRows(t, conn, `SELECT COUNT(*) FROM investment_asset_quotes`); got != quotesBefore {
		t.Errorf("investment_asset_quotes = %d after reading, want %d", got, quotesBefore)
	}
}

// Yield is display data. A holding whose series cannot be computed falls back
// to its movement history, as it did before price series existed, and neither
// fails the list nor touches the other holdings.
func TestListInvestmentsFallsBackToHistoryWhenTheSeriesCannotBeComputed(t *testing.T) {
	srv, conn := newTestServer(t)

	healthy := insertSyncedEquity(t, conn, "Ação A", "AAA3", "10", "1100")
	insertInvestmentMovement(t, conn, healthy, "BUY", "inflow", "1000", strPtr("10"))
	quoteToday(t, conn, "Ação A", "AAA3", "110")

	// A movement whose date is garbage: the series cannot be built.
	badDate := insertSyncedEquity(t, conn, "Ação B", "BBB3", "5", "650")
	insertInvestmentMovement(t, conn, badDate, "BUY", "inflow", "500", strPtr("5"))
	quoteToday(t, conn, "Ação B", "BBB3", "130")
	if _, err := conn.Exec(`UPDATE financial_investment_transactions SET trade_date = 'not-a-date' WHERE investment_id = ?`, badDate); err != nil {
		t.Fatalf("corrupt trade_date: %v", err)
	}

	// A corrupt stored price: the asset's series cannot be read, with and
	// without a history to fall back on.
	badPrice := insertSyncedEquity(t, conn, "Ação C", "CCC3", "4", "700")
	insertInvestmentMovement(t, conn, badPrice, "BUY", "inflow", "500", strPtr("4"))
	assetC := quoteToday(t, conn, "Ação C", "CCC3", "175")
	insertSyncedEquity(t, conn, "Ação D", "DDD3", "4", "700") // no movements to fall back on
	assetD := quoteToday(t, conn, "Ação D", "DDD3", "175")
	for _, assetID := range []string{assetC, assetD} {
		if _, err := conn.Exec(`UPDATE investment_asset_quotes SET price = 'abc' WHERE asset_id = ?`, assetID); err != nil {
			t.Fatalf("corrupt price: %v", err)
		}
	}

	byName := listInvestmentsByName(t, srv.URL)
	if len(byName) != 4 {
		t.Fatalf("listed %d holdings, want 4", len(byName))
	}
	wantYield(t, byName["Ação A"], "100.00", "calculado")
	wantYield(t, byName["Ação B"], "150", "calculado")
	wantYield(t, byName["Ação C"], "200", "calculado")
	if got := byName["Ação D"]; got["yield_value"] != nil || got["yield_unavailable_reason"] != "sem_historico" {
		t.Errorf("Ação D: yield_value = %v, reason = %v, want none/sem_historico", got["yield_value"], got["yield_unavailable_reason"])
	}

	// The single-holding endpoint degrades the same way.
	for id, want := range map[string]string{badDate: "150", badPrice: "200"} {
		resp, err := testGet(t, srv.URL+"/api/investments/"+id)
		if err != nil {
			t.Fatalf("GET /api/investments/{id}: %v", err)
		}
		var got map[string]any
		decodeJSON(t, resp, &got)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET /api/investments/%s status = %d, want 200", id, resp.StatusCode)
		}
		wantYield(t, got, want, "calculado")
	}
}
