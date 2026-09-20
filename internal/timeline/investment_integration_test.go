package timeline_test

import (
	"context"
	"testing"
	"time"

	"contadinho-go/internal/db"
	"contadinho-go/internal/timeline"
)

func TestInvestmentReportingKeepsBankCurveAndAvoidsDoubleContributions(t *testing.T) {
	f := newFixture(t)
	bankID := f.addAccount("500")
	id := f.addTransaction(txn{AccountID: bankID, Amount: "-1050", OccurredAt: date(t, "2026-09-10")})
	now := db.FormatTime(time.Now())
	f.exec(`INSERT INTO investment_accounts (id,name,kind,currency_code,created_at,updated_at) VALUES ('manual','Manual','manual','BRL',?,?)`, now, now)
	f.exec(`INSERT INTO investment_operations (id,account_id,kind,occurred_on,amount,created_at,updated_at) VALUES ('deposit','manual','deposit','2026-09-10','1000',?,?)`, now, now)
	f.exec(`INSERT INTO investment_operations (id,account_id,kind,occurred_on,amount,created_at,updated_at) VALUES ('income','manual','income','2026-09-11','100',?,?)`, now, now)
	f.exec(`INSERT INTO investment_operations (id,account_id,kind,occurred_on,amount,created_at,updated_at) VALUES ('fee','manual','fee','2026-09-12','10',?,?)`, now, now)
	f.exec(`INSERT INTO investment_reconciliations (id,operation_id,financial_transaction_id,amount,created_at) VALUES ('link','deposit',?,'1000',?)`, id, now)
	params := timeline.BuildParams{From: date(t, "2026-09-01"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-09-17")}
	series, err := timeline.BuildSeries(context.Background(), f.conn, params)
	if err != nil {
		t.Fatal(err)
	}
	totals := timeline.TotalsForPeriod(series)
	if totals.Expense.String() != "60" || totals.Income.String() != "100" {
		t.Fatalf("totals = %+v", totals)
	}
	if series.Points[0].Balance.String() != "1550" || series.Points[len(series.Points)-1].Balance.String() != "500" {
		t.Fatalf("cash curve changed: %+v", series.Points)
	}
	params.AccountIDs = []string{bankID}
	filtered, err := timeline.BuildSeries(context.Background(), f.conn, params)
	if err != nil {
		t.Fatal(err)
	}
	totals = timeline.TotalsForPeriod(filtered)
	if totals.Expense.String() != "50" || !totals.Income.IsZero() {
		t.Fatalf("bank filter leaked investment income/cost: %+v", totals)
	}
}
