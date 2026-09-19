package timeline

import (
	"sort"
	"time"

	"github.com/shopspring/decimal"

	"contadinho-go/internal/investments"
)

// MonthSummary is one row of MonthlyBreakdown, the monthly evolution chart's
// data source.
type MonthSummary struct {
	Month                   time.Time // first day of the month, UTC
	Income                  decimal.Decimal
	Expense                 decimal.Decimal // positive magnitude, not signed
	Result                  decimal.Decimal // Income - Expense
	InvestmentContributions decimal.Decimal
	InvestmentWithdrawals   decimal.Decimal
}

func reportingAmount(e Entry) decimal.Decimal {
	if e.ReportableAmount != nil {
		return *e.ReportableAmount
	}
	return e.Amount
}

func monthKey(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

// MonthlyBreakdown produces one MonthSummary per calendar month that has at
// least one entry in series, ordered chronologically.
func MonthlyBreakdown(series Series) []MonthSummary {
	byMonth := map[time.Time]*MonthSummary{}
	var months []time.Time
	for _, e := range series.Entries {
		m := monthKey(e.Date)
		summary, ok := byMonth[m]
		if !ok {
			summary = &MonthSummary{Month: m}
			byMonth[m] = summary
			months = append(months, m)
		}
		amount := reportingAmount(e)
		if amount.IsPositive() {
			summary.Income = summary.Income.Add(amount)
		} else {
			summary.Expense = summary.Expense.Add(amount.Neg())
		}
		// Direction is decided once, where the entry is built (see
		// realEntries and BuildSeries); an entry without a kind moves no
		// investment money.
		switch e.InvestmentTransferKind {
		case string(investments.OperationDeposit):
			summary.InvestmentContributions = summary.InvestmentContributions.Add(e.InvestmentTransferAmount)
		case string(investments.OperationWithdrawal):
			summary.InvestmentWithdrawals = summary.InvestmentWithdrawals.Add(e.InvestmentTransferAmount)
		}
	}
	sort.Slice(months, func(i, j int) bool { return months[i].Before(months[j]) })
	out := make([]MonthSummary, len(months))
	for i, m := range months {
		s := *byMonth[m]
		s.Result = s.Income.Sub(s.Expense)
		out[i] = s
	}
	return out
}

// PeriodTotals is the whole-window equivalent of one MonthSummary row: the
// same Income/Expense/Result, just not bucketed by calendar month.
type PeriodTotals struct {
	Income  decimal.Decimal
	Expense decimal.Decimal // positive magnitude, not signed
	Result  decimal.Decimal // Income - Expense
}

// TotalsForPeriod sums Income and Expense across every entry in the series,
// the same reportingAmount basis as MonthlyBreakdown (a transfer between the
// user's own accounts, or a settled credit-card entry's re-dated purchase,
// counts here exactly as it does there — see
// TestBuildSeriesTransferMovesCashButIsNeitherIncomeNorExpense). It exists
// because a window a caller cares about — a custom range, "últimos 30
// dias", "todo o período" — is rarely a whole number of calendar months, so
// summing MonthlyBreakdown rows would double-count or clip at the edges.
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

// CategoryImpact is one row of CategoryBreakdown.
type CategoryImpact struct {
	CategoryID   *string
	CategoryName string          // "Sem categoria" when CategoryID is nil
	Amount       decimal.Decimal // positive magnitude of expenses in the category
	Percentage   decimal.Decimal // Amount / total expenses of the month, 0 when the month has no expenses
}

const noCategoryName = "Sem categoria"

// CategoryBreakdown sums series' outflow entries in month by category,
// ordered by descending absolute impact. "Sem categoria" always appears,
// even at zero, so it's never silently dropped from the totals.
func CategoryBreakdown(series Series, month time.Time) []CategoryImpact {
	m := monthKey(month)
	amounts := map[string]decimal.Decimal{}
	names := map[string]string{}
	order := []string{noCategoryName}
	amounts[noCategoryName] = decimal.Zero
	names[noCategoryName] = noCategoryName

	total := decimal.Zero
	for _, e := range series.Entries {
		amount := reportingAmount(e)
		if !monthKey(e.Date).Equal(m) || !amount.IsNegative() {
			continue
		}
		magnitude := amount.Neg()
		key := noCategoryName
		name := noCategoryName
		if e.CategoryID != nil {
			key = *e.CategoryID
			name = e.CategoryName
		}
		if _, seen := amounts[key]; !seen {
			order = append(order, key)
		}
		amounts[key] = amounts[key].Add(magnitude)
		names[key] = name
		total = total.Add(magnitude)
	}

	out := make([]CategoryImpact, 0, len(order))
	for _, key := range order {
		amount := amounts[key]
		var categoryID *string
		if key != noCategoryName {
			id := key
			categoryID = &id
		}
		percentage := decimal.Zero
		if total.IsPositive() {
			percentage = amount.Div(total).Mul(decimal.NewFromInt(100))
		}
		out = append(out, CategoryImpact{
			CategoryID:   categoryID,
			CategoryName: names[key],
			Amount:       amount,
			Percentage:   percentage,
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Amount.GreaterThan(out[j].Amount) })
	return out
}

// MonthAmount is one row of CategoryEvolution.
type MonthAmount struct {
	Month  time.Time
	Amount decimal.Decimal // positive magnitude
}

// CategoryEvolution sums series' outflow entries for one category (nil
// means "Sem categoria"), one row per calendar month that has at least one
// matching entry — the same Series CategoryBreakdown already computed, just
// grouped differently, so the total for any given month always reconciles
// with that month's CategoryBreakdown row for the same category (no second
// query, no divergent rounding).
func CategoryEvolution(series Series, categoryID *string) []MonthAmount {
	byMonth := map[time.Time]decimal.Decimal{}
	var months []time.Time
	for _, e := range series.Entries {
		amount := reportingAmount(e)
		if !amount.IsNegative() {
			continue
		}
		matches := (categoryID == nil && e.CategoryID == nil) ||
			(categoryID != nil && e.CategoryID != nil && *categoryID == *e.CategoryID)
		if !matches {
			continue
		}
		m := monthKey(e.Date)
		if _, seen := byMonth[m]; !seen {
			months = append(months, m)
		}
		byMonth[m] = byMonth[m].Add(amount.Neg())
	}
	sort.Slice(months, func(i, j int) bool { return months[i].Before(months[j]) })
	out := make([]MonthAmount, len(months))
	for i, m := range months {
		out[i] = MonthAmount{Month: m, Amount: byMonth[m]}
	}
	return out
}
