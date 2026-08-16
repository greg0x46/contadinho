package timeline

import (
	"sort"
	"time"

	"github.com/shopspring/decimal"
)

// MonthSummary is one row of MonthlyBreakdown, the monthly evolution chart's
// data source.
type MonthSummary struct {
	Month   time.Time // first day of the month, UTC
	Income  decimal.Decimal
	Expense decimal.Decimal // positive magnitude, not signed
	Result  decimal.Decimal // Income - Expense
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
		if e.Amount.IsPositive() {
			summary.Income = summary.Income.Add(e.Amount)
		} else {
			summary.Expense = summary.Expense.Add(e.Amount.Neg())
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
		if !monthKey(e.Date).Equal(m) || e.Amount.IsPositive() {
			continue
		}
		magnitude := e.Amount.Neg()
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
		if e.Amount.IsPositive() {
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
		byMonth[m] = byMonth[m].Add(e.Amount.Neg())
	}
	sort.Slice(months, func(i, j int) bool { return months[i].Before(months[j]) })
	out := make([]MonthAmount, len(months))
	for i, m := range months {
		out[i] = MonthAmount{Month: m, Amount: byMonth[m]}
	}
	return out
}
