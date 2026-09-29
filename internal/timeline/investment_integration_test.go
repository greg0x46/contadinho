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

// The bank credit contains both principal and profit, net of withheld costs.
// Its cash movement must survive while reporting uses the four components.
func TestDetailedRedemptionReplacesOnlyItsBankParcel(t *testing.T) {
	f := newFixture(t)
	bankID := f.addAccount("1500")
	id := f.addTransaction(txn{AccountID: bankID, Amount: "1125", OccurredAt: date(t, "2026-09-10")})
	now := db.FormatTime(time.Now())
	f.exec(`INSERT INTO investment_accounts (id,name,kind,currency_code,created_at,updated_at) VALUES ('manual','Manual','manual','BRL',?,?)`, now, now)
	f.exec(`INSERT INTO investment_operations (id,account_id,kind,occurred_on,amount,principal_amount,income_amount,fees,taxes,created_at,updated_at)
        VALUES ('redemption','manual','redemption','2026-09-09','1075','1000','100','5','20',?,?)`, now, now)
	params := timeline.BuildParams{From: date(t, "2026-09-01"), To: date(t, "2026-09-30"), ReferenceDate: date(t, "2026-09-17")}
	assertReport := func(income string) timeline.Series {
		t.Helper()
		series, err := timeline.BuildSeries(context.Background(), f.conn, params)
		if err != nil {
			t.Fatal(err)
		}
		monthly := timeline.MonthlyBreakdown(series)
		if len(monthly) != 1 || monthly[0].Income.String() != income ||
			monthly[0].Expense.String() != "25" || monthly[0].InvestmentWithdrawals.String() != "1000" {
			t.Fatalf("monthly = %+v", monthly)
		}
		if series.Points[0].Balance.String() != "375" || series.Points[len(series.Points)-1].Balance.String() != "1500" {
			t.Fatalf("bank curve changed: %+v", series.Points)
		}
		return series
	}
	// Unlinked records are independent until the user identifies the match.
	assertReport("1225")
	f.exec(`INSERT INTO investment_reconciliations (id,operation_id,financial_transaction_id,amount,created_at)
        VALUES ('link','redemption',?,'1075',?)`, id, now)
	series := assertReport("150") // 100 income + the unmatched 50 of the bank line.
	for _, e := range series.Entries {
		if e.Source == timeline.SourceReal && e.SourceRefID == id &&
			(e.Amount.String() != "1125" || e.ReportableAmount == nil || e.ReportableAmount.String() != "50" || !e.InvestmentTransferAmount.IsZero()) {
			t.Fatalf("bank entry = %+v", e)
		}
	}
	params.AccountIDs = []string{bankID}
	assertReport("150") // Filters retain the replacement attached to this bank.
	params.AccountIDs = nil
	f.exec(`DELETE FROM investment_reconciliations WHERE id='link'`)
	assertReport("1225")
}
