package timeline

import (
	"github.com/shopspring/decimal"
)

func reportingAmount(e Entry) decimal.Decimal {
	if e.ReportableAmount != nil {
		return *e.ReportableAmount
	}
	return e.Amount
}

// PeriodTotals is the income/expense reading of a whole window, not
// bucketed by calendar month.
type PeriodTotals struct {
	Income  decimal.Decimal
	Expense decimal.Decimal // positive magnitude, not signed
	Result  decimal.Decimal // Income - Expense
}

// TotalsForPeriod sums Income and Expense across every entry in the series
// on the reportingAmount basis (a transfer between the user's own accounts,
// or a settled credit-card entry's re-dated purchase, reports as zero — see
// TestBuildSeriesTransferMovesCashButIsNeitherIncomeNorExpense). It exists
// because a window a caller cares about — a custom range, "últimos 30
// dias", "todo o período" — is rarely a whole number of calendar months.
//
// This deliberately walks series.Entries, not series.Points: Points is
// filtered by cashEntries for the balance walk alone (see its doc comment)
// and would silently drop a settled credit-card purchase's spend with no
// substitute, since the bill payment that replaces it on the balance curve
// carries a transfer-kind category and reports as zero itself.
func TotalsForPeriod(series Series) PeriodTotals {
	var totals PeriodTotals
	for _, e := range series.Entries {
		amount := reportingAmount(e)
		if amount.IsPositive() {
			totals.Income = totals.Income.Add(amount)
		} else {
			totals.Expense = totals.Expense.Add(amount.Neg())
		}
	}
	totals.Result = totals.Income.Sub(totals.Expense)
	return totals
}
