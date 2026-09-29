package syncsvc_test

import (
	"context"
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

// Every investments payload the syncs stored is replayed into the price
// series once, and holdings stored before issue_date existed get it filled.
func TestBackfillAssetQuotesRebuildsTheSeriesFromStoredPayloadsOnce(t *testing.T) {
	conn := newTestConn(t)
	ctx := context.Background()
	sourceID, syncRunID := newSyncRun(t, conn)
	now := time.Now()
	for i, payload := range []string{
		investmentsPayload("2026-08-31T06:25:30.000Z", "256.29"),
		investmentsPayload("2026-09-28T00:03:22.000Z", "258.6"),
	} {
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
			'258.6', '200', '258.6', '200', '2026-09-28T00:03:22.000000000Z', 'raw-b', 'hash', ?, ?)`,
		sourceID, stamp, stamp); err != nil {
		t.Fatalf("insert investment: %v", err)
	}

	processed, err := syncsvc.BackfillAssetQuotes(ctx, conn)
	if err != nil || processed != 2 {
		t.Fatalf("BackfillAssetQuotes = %d, %v; want 2 snapshots", processed, err)
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
