package timeline

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/investments"
)

func TestInvestmentTransfersDoNotBecomeMonthlySpending(t *testing.T) {
	day := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	zero, residual := decimal.Zero, decimal.NewFromInt(-50)
	// Kind is what realEntries/BuildSeries set; the aggregation never reads
	// Amount's sign to decide the direction.
	deposit, withdrawal := string(investments.OperationDeposit), string(investments.OperationWithdrawal)
	series := Series{Entries: []Entry{
		{Date: day, Amount: decimal.NewFromInt(-1000), ReportableAmount: &zero, InvestmentTransferAmount: decimal.NewFromInt(1000), InvestmentTransferKind: deposit},
		{Date: day, Amount: decimal.NewFromInt(-250), ReportableAmount: &residual, InvestmentTransferAmount: decimal.NewFromInt(200), InvestmentTransferKind: deposit},
		{Date: day, Amount: decimal.NewFromInt(300), ReportableAmount: &zero, InvestmentTransferAmount: decimal.NewFromInt(300), InvestmentTransferKind: withdrawal},
		// An entry that carries no kind moves no investment money, whatever
		// its sign — it must not land in either bucket.
		{Date: day, Amount: decimal.NewFromInt(-40), ReportableAmount: &zero},
	}}
	monthly := MonthlyBreakdown(series)
	if len(monthly) != 1 || monthly[0].Expense.String() != "50" || !monthly[0].Income.IsZero() || monthly[0].InvestmentContributions.String() != "1200" || monthly[0].InvestmentWithdrawals.String() != "300" {
		t.Fatalf("monthly = %+v", monthly)
	}
	categories := CategoryBreakdown(series, day)
	if len(categories) != 1 || categories[0].Amount.String() != "50" {
		t.Fatalf("categories = %+v", categories)
	}
	evolution := CategoryEvolution(series, nil)
	if len(evolution) != 1 || evolution[0].Amount.String() != "50" {
		t.Fatalf("evolution = %+v", evolution)
	}
	// The real ledger remains intact for balance reconstruction.
	if series.Entries[0].Amount.String() != "-1000" || series.Entries[1].Amount.String() != "-250" {
		t.Fatal("cash amounts changed")
	}
}
