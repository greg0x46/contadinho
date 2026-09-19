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
	monthly := timeline.MonthlyBreakdown(series)
	if len(monthly) != 1 || monthly[0].Expense.String() != "60" || monthly[0].Income.String() != "100" || monthly[0].InvestmentContributions.String() != "1000" {
		t.Fatalf("monthly = %+v", monthly)
	}
	if series.Points[0].Balance.String() != "1550" || series.Points[len(series.Points)-1].Balance.String() != "500" {
		t.Fatalf("cash curve changed: %+v", series.Points)
	}
	params.AccountIDs = []string{bankID}
	filtered, err := timeline.BuildSeries(context.Background(), f.conn, params)
	if err != nil {
		t.Fatal(err)
	}
	monthly = timeline.MonthlyBreakdown(filtered)
	if len(monthly) != 1 || monthly[0].Expense.String() != "50" || !monthly[0].Income.IsZero() {
		t.Fatalf("bank filter leaked investment income/cost: %+v", monthly)
	}
}

// A reconciled bank line's transfer direction comes from the bank line's
// classification, decided once in realEntries; MonthlyBreakdown and the HTTP
// DTO only read InvestmentTransferKind, never Amount's sign.
func TestReconciledBankLinesCarryTheInvestmentTransferKind(t *testing.T) {
	f := newFixture(t)
	bankID := f.addAccount("500")
	depositTx := f.addTransaction(txn{AccountID: bankID, Amount: "-1000", OccurredAt: date(t, "2026-09-10")})
	withdrawalTx := f.addTransaction(txn{AccountID: bankID, Amount: "300", OccurredAt: date(t, "2026-09-12")})
	now := db.FormatTime(time.Now())
	f.exec(`INSERT INTO investment_accounts (id,name,kind,currency_code,created_at,updated_at) VALUES ('manual','Manual','manual','BRL',?,?)`, now, now)
	f.exec(`INSERT INTO investment_operations (id,account_id,kind,occurred_on,amount,created_at,updated_at) VALUES ('deposit','manual','deposit','2026-09-10','1000',?,?)`, now, now)
	f.exec(`INSERT INTO investment_operations (id,account_id,kind,occurred_on,amount,created_at,updated_at) VALUES ('withdrawal','manual','withdrawal','2026-09-12','300',?,?)`, now, now)
	f.exec(`INSERT INTO investment_reconciliations (id,operation_id,financial_transaction_id,amount,created_at) VALUES ('link-deposit','deposit',?,'1000',?)`, depositTx, now)
	f.exec(`INSERT INTO investment_reconciliations (id,operation_id,financial_transaction_id,amount,created_at) VALUES ('link-withdrawal','withdrawal',?,'300',?)`, withdrawalTx, now)
	params := timeline.BuildParams{From: date(t, "2026-09-01"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-09-17")}
	series, err := timeline.BuildSeries(context.Background(), f.conn, params)
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, e := range series.Entries {
		if e.Source == timeline.SourceReal {
			kinds[e.SourceRefID] = e.InvestmentTransferKind
		}
	}
	if kinds[depositTx] != "deposit" || kinds[withdrawalTx] != "withdrawal" {
		t.Fatalf("real entry kinds = %+v", kinds)
	}
	monthly := timeline.MonthlyBreakdown(series)
	if len(monthly) != 1 || monthly[0].InvestmentContributions.String() != "1000" || monthly[0].InvestmentWithdrawals.String() != "300" || !monthly[0].Income.IsZero() || !monthly[0].Expense.IsZero() {
		t.Fatalf("monthly = %+v", monthly)
	}
}
