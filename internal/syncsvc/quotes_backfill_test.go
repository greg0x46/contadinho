package syncsvc_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"contadinho-go/internal/db"
	"contadinho-go/internal/investments"
	"contadinho-go/internal/pluggy"
	"contadinho-go/internal/syncsvc"
)

// investmentsPayload is a Pluggy /investments response as raw_imports keeps
// it, with the LCA priced as it was on asOf.
func investmentsPayload(asOf, amount string) string {
	return `{"results":[{"id":"lca-1","name":"LCA - BANCO VOTORANTIM S.A.","type":"FIXED_INCOME",
		"subtype":"LCA","code":"24I01250142","currencyCode":"BRL","balance":` + amount + `,
		"quantity":200,"amount":` + amount + `,"amountOriginal":200,"value":1.29,
		"issuerCNPJ":"59.588.111/0001-03","issueDate":"2024-09-06T03:00:00.000Z",
		"purchaseDate":"2024-09-06T03:00:00.000Z","dueDate":"2028-09-05T03:00:00.000Z",
		"date":"` + asOf + `"}]}`
}

// storeLCA stores the payloads as raw_imports, oldest first, and the LCA they
// describe as it stood after the last one, still without the title terms.
func storeLCA(t *testing.T, conn *sql.DB, payloads ...string) {
	t.Helper()
	sourceID, syncRunID := newSyncRun(t, conn)
	now := time.Now()
	for i, payload := range payloads {
		if _, err := conn.Exec(`INSERT INTO raw_imports (
				id, sync_run_id, source_id, scope, page_sequence, request_attempt,
				request_method, request_path, http_status, response_headers, payload,
				payload_sha256, received_at
			) VALUES (?, ?, ?, 'investments', ?, 1, 'GET', '/investments', 200, '{}', ?, 'sha', ?)`,
			"raw-"+string(rune('a'+i)), syncRunID, sourceID, i+1, []byte(payload),
			db.FormatTime(now.Add(time.Duration(i)*time.Minute))); err != nil {
			t.Fatalf("insert raw_import: %v", err)
		}
	}
	stamp := db.FormatTime(now)
	if _, err := conn.Exec(`INSERT INTO financial_investments (
			id, source_id, external_id, name, code, investment_type, subtype, currency_code, balance, quantity,
			amount, amount_original, as_of_date, current_raw_import_id, normalized_hash, created_at, updated_at
		) VALUES ('fi-lca', ?, 'lca-1', 'LCA - BANCO VOTORANTIM S.A.', '24I01250142', 'FIXED_INCOME', 'LCA', 'BRL',
			'258.6', '200', '258.6', '200', '2026-09-28T00:03:22.000000000Z', ?, 'hash', ?, ?)`,
		sourceID, "raw-"+string(rune('a'+len(payloads)-1)), stamp, stamp); err != nil {
		t.Fatalf("insert investment: %v", err)
	}
}

// Every investments payload the syncs stored is replayed into the price
// series once, and holdings stored before issue_date existed get it filled.
func TestBackfillAssetQuotesRebuildsTheSeriesFromStoredPayloadsOnce(t *testing.T) {
	conn := newTestConn(t)
	ctx := context.Background()
	storeLCA(t, conn,
		investmentsPayload("2026-08-31T06:25:30.000Z", "256.29"),
		investmentsPayload("2026-09-28T00:03:22.000Z", "258.6"),
	)

	processed, err := syncsvc.BackfillAssetQuotes(ctx, conn)
	if err != nil || processed != 2 {
		t.Fatalf("BackfillAssetQuotes = %d, %v; want 2 snapshots", processed, err)
	}
	// The rebuilt series says what each price is: the snapshots' own prices
	// are sync prices, the purchase PU derived from the title is an issue price.
	origins := map[string]int{}
	rows, err := conn.Query(`SELECT origin, COUNT(*) FROM investment_asset_quotes GROUP BY origin`)
	if err != nil {
		t.Fatalf("read origins: %v", err)
	}
	for rows.Next() {
		var origin string
		var count int
		if err := rows.Scan(&origin, &count); err != nil {
			t.Fatalf("scan origins: %v", err)
		}
		origins[origin] = count
	}
	rows.Close()
	if origins[string(investments.QuoteOriginSync)] != 2 || origins[string(investments.QuoteOriginIssue)] != 1 || len(origins) != 2 {
		t.Fatalf("origins = %v, want 2 sync and 1 issue", origins)
	}
	var issueDate string
	if err := conn.QueryRow(`SELECT issue_date FROM financial_investments WHERE id = 'fi-lca'`).Scan(&issueDate); err != nil {
		t.Fatalf("issue_date not filled: %v", err)
	}
	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	result, err := investments.PositionYield(ctx, conn, "fi-lca", &from, time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC))
	if err != nil || result.Value.String() != "2.31" {
		t.Fatalf("September yield = %v, %v; want 2.31", result.Value, err)
	}

	again, err := syncsvc.BackfillAssetQuotes(ctx, conn)
	if err != nil || again != 0 {
		t.Fatalf("second BackfillAssetQuotes = %d, %v; want a no-op", again, err)
	}
}

// Holdings are parsed by the same mapping a live sync uses.
func TestParseInvestmentsPayloadReadsTheTitleTerms(t *testing.T) {
	snapshots, err := pluggy.ParseInvestmentsPayload([]byte(investmentsPayload("2026-09-28T00:03:22.000Z", "258.6")))
	if err != nil || len(snapshots) != 1 {
		t.Fatalf("ParseInvestmentsPayload = %v, %v", snapshots, err)
	}
	s := snapshots[0]
	if s.IssueDate == nil || s.PurchaseDate == nil || s.IssuerCNPJ == nil || *s.IssuerCNPJ != "59.588.111/0001-03" {
		t.Fatalf("title terms not mapped: %+v", s)
	}
}

// The rebuild is gated on a price a sync wrote, not on any price: a database
// whose series holds only a market quote or a purchase PU has not been rebuilt.
func TestBackfillAssetQuotesStillRunsWhenTheSeriesHoldsNoSyncedPrice(t *testing.T) {
	conn := newTestConn(t)
	storeLCA(t, conn, investmentsPayload("2026-09-28T00:03:22.000Z", "258.6"))
	if _, err := conn.Exec(`INSERT INTO investment_assets
		(id, canonical_key, name, asset_type, currency_code, created_at, updated_at)
		VALUES ('asset-x', 'name:acao:petr4', 'Petrobras PN', 'Ação', 'BRL', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`); err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	for day, kind := range map[string][2]string{"2026-09-01": {"yahoo", "market"}, "2026-09-02": {"issue", "issue"}} {
		if _, err := conn.Exec(`INSERT INTO investment_asset_quotes
			(asset_id, quoted_on, price, source, origin, created_at, updated_at)
			VALUES ('asset-x', ?, '10', ?, ?, '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`, day, kind[0], kind[1]); err != nil {
			t.Fatalf("insert quote: %v", err)
		}
	}

	processed, err := syncsvc.BackfillAssetQuotes(context.Background(), conn)
	if err != nil || processed != 1 {
		t.Fatalf("BackfillAssetQuotes = %d, %v; want the stored snapshot rebuilt", processed, err)
	}
}
